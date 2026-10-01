# The Complete Mental Model for DSA: How, Where, and When to Apply Logic

> *"The goal is not to memorize solutions — it is to build a thinking framework that generates solutions."*

---

## Table of Contents

1. [The Core Mental Model: Before Any Code](#1-the-core-mental-model-before-any-code)
2. [Understanding the Problem Space First](#2-understanding-the-problem-space-first)
3. [The Decision Hierarchy: What to Write First](#3-the-decision-hierarchy-what-to-write-first)
4. [Loops: When, Where, and What Kind](#4-loops-when-where-and-what-kind)
5. [Conditions: When, Where, and What Kind](#5-conditions-when-where-and-what-kind)
6. [The Loop-Condition Relationship](#6-the-loop-condition-relationship)
7. [Keywords and Special Constructs](#7-keywords-and-special-constructs)
8. [Thinking Through Data Structures](#8-thinking-through-data-structures)
9. [Pattern Recognition: The 12 Core Patterns](#9-pattern-recognition-the-12-core-patterns)
10. [The Five-Phase Solving Process](#10-the-five-phase-solving-process)
11. [Where to Start: A Complete Decision Framework](#11-where-to-start-a-complete-decision-framework)
12. [Dry Running: The Most Underrated Skill](#12-dry-running-the-most-underrated-skill)
13. [Edge Cases: Systematic Thinking](#13-edge-cases-systematic-thinking)
14. [Complete Problem Walkthroughs](#14-complete-problem-walkthroughs)
15. [Language-Specific Idioms: C, Go, Rust](#15-language-specific-idioms-c-go-rust)

---

## 1. The Core Mental Model: Before Any Code

Before writing a single line of code, you need a mental model. A mental model is the internal picture you form of how data moves, transforms, and gets checked. Without it, you write random code and hope it works. With it, you write deliberate code that you can trace and debug.

### The Four Questions You Must Answer First

Every DSA problem can be broken down by answering these four questions in order:

**Question 1: What is the input and what is the output?**

Be completely explicit. If the input is an array, ask: sorted or unsorted? Can it have duplicates? What are the value ranges? What is the size range? If the output is a number, ask: is it an index, a count, a value, a boolean? Can it be negative?

**Question 2: What is the relationship between input and output?**

How does the output depend on the input? Is each output element derived from one input element (mapping)? Is the output derived from comparing multiple input elements (selection)? Is the output built by combining all input elements (aggregation)? Is the output a rearrangement of input elements (sorting/permuting)?

**Question 3: What decisions need to be made and when?**

Every algorithm makes decisions. A decision is a branch point: "do this OR do that." When does each decision happen? Before processing? During processing each element? After comparing two elements?

**Question 4: What repeats and what doesn't?**

Loops exist because something needs to happen more than once. What is the unit of repetition? Is it: each element, each pair of elements, each window of elements, each level of a tree, each neighbor in a graph?

### Building Your Mental Picture

Once you answer these questions, form a picture. Literally visualize the data. If the input is `[3, 1, 4, 1, 5, 9, 2, 6]`, picture it as boxes. Picture a pointer moving through it. Picture two pointers. Picture it being split in half. Picture values going into a bucket. This visualization is not optional — it is what separates programmers who guess from programmers who think.

---

## 2. Understanding the Problem Space First

### Classifying the Problem

Before choosing any structure or algorithm, classify what the problem is asking:

**Existence problems** — Does something exist in the data? (binary search, hash lookup)
**Count problems** — How many times does something occur? (frequency counting, sliding window)
**Optimization problems** — What is the best value? (DP, greedy, divide and conquer)
**Construction problems** — Build something: a path, a sequence, a structure. (backtracking, BFS, DP)
**Transformation problems** — Rearrange or modify the data. (sorting, two pointers, in-place)
**Detection problems** — Is the data in some property: a cycle, a palindrome, sorted? (Floyd's, two pointers)

### Understanding Constraints

Constraints tell you which algorithm is acceptable. This is the single most important skill in competitive DSA.

If `n ≤ 10`: Any algorithm works. Even O(n!) is fine. Try all permutations.
If `n ≤ 20`: Bitmask DP, backtracking with pruning.
If `n ≤ 100`: O(n³) is fine. Triple nested loops.
If `n ≤ 1000`: O(n²) is fine. Double nested loops.
If `n ≤ 10⁵`: O(n log n) required. Sorting, binary search, segment trees.
If `n ≤ 10⁶`: O(n) required. Linear scan, hash maps, two pointers.
If `n ≤ 10⁹`: O(log n) required. Binary search, math formulas.

This is not a rule to memorize. It is a derivation: a modern computer does roughly 10⁸ simple operations per second. If n = 10⁵ and you use O(n²), that is 10¹⁰ operations — 100 seconds. Not acceptable. So you derive that you need O(n log n) or better.

---

## 3. The Decision Hierarchy: What to Write First

When you sit down to code a solution, there is a natural hierarchy of decisions. Getting this hierarchy wrong is why beginners write tangled code.

### Level 1: The Outermost Structure

The outermost structure of your function defines the fundamental shape of the algorithm. Ask: is this problem solved by:

- A single pass through the data (one loop)?
- Multiple passes (multiple loops sequentially)?
- A recursive decomposition (recursive calls)?
- A state machine (loop with state variable)?
- A nested traversal (loop inside loop)?

This decision comes first because everything else nests inside it.

### Level 2: The Iteration Strategy

Inside that outermost structure, how do you move through the data?

- One pointer moving left to right?
- Two pointers, one from each end, moving toward each other?
- Two pointers, both starting left, one fast and one slow?
- A window of fixed or variable size sliding across?
- An index going in steps (every 2nd, every half, dividing each time)?

### Level 3: The Decision Points

Inside the iteration, what decisions do you make at each step?

- Compare current element to a target (if/else)?
- Compare current element to the previous (if/else)?
- Compare two pointers' values (if/else/swap)?
- Check membership in a set (if hash_set contains)?
- Check a running invariant (while invariant holds)?

### Level 4: The State Updates

After each decision, what state changes?

- Increment a counter?
- Update a maximum or minimum?
- Move a pointer?
- Add to a running sum?
- Push or pop from a stack?
- Update a DP table cell?

### Why This Hierarchy Matters

When you write code bottom-up (starting with the inner condition before knowing the outer loop), you often end up with code that has the wrong structure. The inner condition depends on what the outer loop is doing. If the outer loop is wrong, the inner condition cannot save it.

Always work top-down: outer structure → iteration strategy → decision points → state updates.

---

## 4. Loops: When, Where, and What Kind

### When Do You Need a Loop?

You need a loop when the problem requires visiting, processing, or comparing more than one element, and the number of elements is not fixed at compile time (i.e., it depends on the input size n).

You do NOT need a loop when: the answer can be computed directly from the input with a formula; you only ever look at one element; or the "repetition" is handled by recursion.

### The Four Fundamental Loop Shapes

#### Shape 1: The Linear Scan Loop

**When to use it:** You need to visit every element exactly once, in order.

**Mental trigger:** "For each element in the collection, do something."

```c
// C
for (int i = 0; i < n; i++) {
    // process arr[i]
}
```

```go
// Go
for i := 0; i < n; i++ {
    // process arr[i]
}
// or idiomatic range loop:
for i, v := range arr {
    // i is index, v is value
}
```

```rust
// Rust
for i in 0..n {
    // process arr[i]
}
// or iterator style:
for (i, v) in arr.iter().enumerate() {
    // i is index, v is reference to value
}
```

**Where to put the condition inside this loop:** Your condition goes inside the loop body. If the condition decides whether to process the element, it goes at the top of the body (guard clause). If the condition updates state based on the element's value, it goes after any access of the element.

#### Shape 2: The While-Condition Loop

**When to use it:** You don't know how many steps you'll take — you stop when a condition is met, not when an index reaches a bound. Classic uses: two pointers meeting, binary search converging, Newton's method converging, reading until EOF.

**Mental trigger:** "Keep going until something is true."

```c
// C - two pointer meeting
int left = 0, right = n - 1;
while (left < right) {
    // process arr[left] and arr[right]
    left++;
    right--;
}
```

```go
// Go
left, right := 0, n-1
for left < right {
    // process arr[left] and arr[right]
    left++
    right--
}
```

```rust
// Rust
let mut left = 0;
let mut right = n - 1;
while left < right {
    // process arr[left] and arr[right]
    left += 1;
    right -= 1;
}
```

**Critical insight about the loop condition:** The `while` condition is a sentinel. It expresses the invariant that must hold for the loop to continue. If you are confused about what goes in the `while` condition versus inside the body, ask: "What must be true for it to even make sense to run this loop's body?" That answer goes in the `while` condition. Everything else — the decisions about what to do in each step — goes inside the body.

**Common mistake:** Putting business logic in the `while` condition. Do not write `while (arr[i] != target && i < n && doSomeProcessing())`. The condition should be simple: a membership check, a comparison between two values, a boolean flag.

#### Shape 3: The Nested Loop

**When to use it:** You need to consider all pairs, all combinations, compare each element with every other element, or process a 2D structure.

**Mental trigger:** "For each element i, do something involving every j."

```c
// C - all pairs O(n²)
for (int i = 0; i < n; i++) {
    for (int j = i + 1; j < n; j++) {
        // process pair (arr[i], arr[j])
    }
}
```

```go
// Go
for i := 0; i < n; i++ {
    for j := i + 1; j < n; j++ {
        // process pair (arr[i], arr[j])
    }
}
```

```rust
// Rust
for i in 0..n {
    for j in (i+1)..n {
        // process pair (arr[i], arr[j])
    }
}
```

**Where to start inner loop:** This is a very common source of confusion. Ask yourself: "Should j start from 0, from i, from i+1, or from somewhere else?"

- Start j from 0: You want to compare i against ALL other elements, including those before it.
- Start j from i: You want each pair (i, j) where j >= i (includes same element).
- Start j from i+1: You want each pair exactly once, no same-element pairs.
- Start j from some computed position: You have a more complex relationship.

**Critical insight:** Never start the inner loop from scratch (0) when you only want pairs you haven't seen before. That doubles your work and often produces wrong results.

#### Shape 4: The Do-While / Post-Condition Loop

**When to use it:** You need to execute the body at least once before checking the condition. Classic uses: reading input until valid, processing a node before checking if it has a next, implementing a hash collision resolution that must probe at least once.

```c
// C (the only language with do-while as a native construct)
do {
    // execute at least once
    read_input(&val);
} while (val < 0);  // repeat if invalid
```

```go
// Go - simulate with for loop
for {
    readInput(&val)
    if val >= 0 {
        break
    }
}
```

```rust
// Rust - simulate with loop
loop {
    val = read_input();
    if val >= 0 {
        break;
    }
}
```

### Loop Boundary Errors: The Most Common Bug

The two classic boundary errors are off-by-one at the start and off-by-one at the end.

**Off-by-one at the end:** Should the condition be `i < n` or `i <= n-1`? These are identical. But `i <= n` is wrong — it goes one past the end. The rule: for a 0-indexed array of size n, valid indices are 0 through n-1. Your loop bound should exclude n.

**Off-by-one at the start:** When do you initialize? If you need to compare `arr[i]` with `arr[i-1]`, you must start i at 1, not 0, or you will access `arr[-1]`.

**The "one element past" sentinel:** In two-pointer problems on sorted arrays, what happens when `left == right`? Do you process that case or not? This depends on the problem. For finding pairs, `left < right` is correct (you don't want a pair of the same element). For finding a single element, `left <= right` is correct (you want to process the case where both pointers are on the same element).

---

## 5. Conditions: When, Where, and What Kind

### When Do You Need a Condition?

You need a condition when the behavior of your algorithm depends on the value of data, not just its position. Loops handle position. Conditions handle value.

If the same operation is performed regardless of value, you don't need a condition. If different operations are performed based on value, you need a condition.

### The Four Condition Locations

#### Location 1: As the Loop Termination Condition

This condition controls whether the loop continues. It is not about the data's value in the business sense — it is about whether it is still valid to keep looping.

Examples of valid loop conditions:
- `i < n` — am I still within bounds?
- `left < right` — have my two pointers crossed?
- `low <= high` — has binary search converged?
- `node != NULL` — have I reached the end of a linked list?

The loop condition is about the algorithm's continuation, not the algorithm's logic.

#### Location 2: As a Guard Clause at the Top of the Loop Body

A guard clause is an early exit or skip. It says: "If this element doesn't meet my criteria, skip it and move on."

```c
// C - skip negative numbers
for (int i = 0; i < n; i++) {
    if (arr[i] < 0) continue;  // guard clause - skip
    // process arr[i] knowing it is non-negative
    sum += arr[i];
}
```

```go
// Go
for i := 0; i < n; i++ {
    if arr[i] < 0 {
        continue
    }
    sum += arr[i]
}
```

```rust
// Rust
for &v in arr.iter() {
    if v < 0 { continue; }
    sum += v;
}
```

**When to use guard clauses:** When most elements are processed the same way, but some special elements should be skipped or handled differently, put the special case check at the top as a guard. This keeps the main logic unindented and clean.

#### Location 3: As Business Logic Inside the Loop Body

This is the primary decision: given the current element(s), what should happen?

```c
// C - find maximum and track its index
int max_val = arr[0];
int max_idx = 0;
for (int i = 1; i < n; i++) {
    if (arr[i] > max_val) {    // business logic condition
        max_val = arr[i];      // update state
        max_idx = i;           // update state
    }
}
```

```go
// Go
maxVal, maxIdx := arr[0], 0
for i := 1; i < n; i++ {
    if arr[i] > maxVal {
        maxVal = arr[i]
        maxIdx = i
    }
}
```

```rust
// Rust
let mut max_val = arr[0];
let mut max_idx = 0;
for i in 1..arr.len() {
    if arr[i] > max_val {
        max_val = arr[i];
        max_idx = i;
    }
}
```

**The key insight:** Business logic conditions live INSIDE the loop body, not in the loop condition. The loop condition only controls termination.

#### Location 4: After the Loop

Post-loop conditions handle the result that the loop produced. Classic examples: after binary search, check if you actually found the target. After finding a potential answer, validate it.

```c
// C - binary search, then validate
int low = 0, high = n - 1;
while (low <= high) {
    int mid = low + (high - low) / 2;
    if (arr[mid] == target) return mid;
    else if (arr[mid] < target) low = mid + 1;
    else high = mid - 1;
}
return -1;  // post-loop: target not found
```

```go
// Go
low, high := 0, n-1
for low <= high {
    mid := low + (high-low)/2
    if arr[mid] == target {
        return mid
    } else if arr[mid] < target {
        low = mid + 1
    } else {
        high = mid - 1
    }
}
return -1
```

```rust
// Rust
let (mut low, mut high) = (0i32, n as i32 - 1);
while low <= high {
    let mid = low + (high - low) / 2;
    if arr[mid as usize] == target {
        return mid as usize;
    } else if arr[mid as usize] < target {
        low = mid + 1;
    } else {
        high = mid - 1;
    }
}
// post-loop: not found
```

### if vs if-else vs if-else-if vs switch

#### Use `if` alone (no else):

When the condition is a side effect or state update that either happens or doesn't. The algorithm continues the same way regardless.

```c
if (arr[i] > max) max = arr[i];   // update max if bigger; continue regardless
```

#### Use `if-else`:

When two different paths must be taken — one or the other, never both, never neither. This expresses mutual exclusivity.

```c
if (arr[left] + arr[right] == target) {
    return found_result;
} else if (arr[left] + arr[right] < target) {
    left++;
} else {
    right--;
}
```

**Mental model for if-else:** Think of it as a fork in the road. You MUST take one path. The algorithm's correctness depends on exactly one branch executing.

#### Use `if` with early return vs else:

In functions that return, you can often replace if-else with if-return, making the else implicit. This reduces nesting and improves readability.

```c
// Deep nesting - avoid
if (condition) {
    // 20 lines of code
} else {
    // 20 lines of code
}

// Flat - prefer
if (condition) {
    // handle case A
    return result_a;
}
// handle case B (implicit else)
return result_b;
```

#### Use `switch` (or match in Rust):

When you have a discrete enumeration of cases — not ranges, not complex conditions, but specific values.

```c
// C - switch is for discrete values
switch (node->type) {
    case NODE_LEAF:
        return node->value;
    case NODE_ADD:
        return evaluate(node->left) + evaluate(node->right);
    case NODE_MUL:
        return evaluate(node->left) * evaluate(node->right);
    default:
        return -1;
}
```

```go
// Go - switch is more powerful, can handle expressions
switch {
case x < 0:
    return "negative"
case x == 0:
    return "zero"
default:
    return "positive"
}
```

```rust
// Rust - match is exhaustive and powerful
match node.kind {
    NodeKind::Leaf => node.value,
    NodeKind::Add  => evaluate(&node.left) + evaluate(&node.right),
    NodeKind::Mul  => evaluate(&node.left) * evaluate(&node.right),
}
```

**Key insight about switch/match vs if-else:** If you find yourself writing `if x == 1 ... else if x == 2 ... else if x == 3 ...`, that is a switch in disguise. Use switch/match. It is not just cleaner — it also allows the compiler to generate jump tables, which are faster.

---

## 6. The Loop-Condition Relationship

This section addresses exactly the confusion you described: "where to apply condition — in loop conditions or conditional statements?"

### The Fundamental Rule

**The loop condition controls WHEN to iterate.**
**The if-condition controls WHAT to do in each iteration.**

These are different concerns. Mixing them creates bugs and confusion.

### Common Wrong Patterns and Their Corrections

#### Wrong Pattern 1: Business logic in the loop condition

```c
// WRONG - mixing concerns
while (i < n && arr[i] != target && do_some_update(&state)) {
    i++;
}
```

The problem: the loop condition is doing three things. You can't reason about it clearly.

```c
// RIGHT - separate concerns
while (i < n) {
    if (arr[i] == target) break;
    do_some_update(&state);
    i++;
}
```

Now the loop condition does one thing (bounds check). The if-condition does one thing (target check). The update does one thing.

#### Wrong Pattern 2: Loop control inside a deeply nested if

```c
// WRONG - hard to see the loop structure
for (int i = 0; i < n; i++) {
    if (arr[i] > 0) {
        if (arr[i] % 2 == 0) {
            if (arr[i] < 100) {
                sum += arr[i];
            }
        }
    }
}
```

```c
// RIGHT - flatten with guard clauses
for (int i = 0; i < n; i++) {
    if (arr[i] <= 0)    continue;
    if (arr[i] % 2 != 0) continue;
    if (arr[i] >= 100)  continue;
    sum += arr[i];
}
```

The loop structure is now clearly visible. Each guard clause eliminates one class of elements.

#### Wrong Pattern 3: Redundant condition checking

```c
// WRONG - arr[left] + arr[right] computed twice
while (left < right) {
    if (arr[left] + arr[right] == target) {
        return true;
    }
    if (arr[left] + arr[right] < target) {
        left++;
    }
    if (arr[left] + arr[right] > target) {  // already know it's not == or <, so this is always true
        right--;
    }
}
```

```c
// RIGHT - use if-else-if to express mutual exclusivity
while (left < right) {
    int sum = arr[left] + arr[right];
    if (sum == target)      return true;
    else if (sum < target)  left++;
    else                    right--;
}
```

### The Loop-Condition Decision Tree

When you are deciding whether to put logic in the loop condition or inside the body, ask:

1. Does this logic determine whether to run another iteration at all? → Loop condition
2. Does this logic determine what to do WITHIN an iteration? → Inside body
3. Does this logic skip the current element? → Guard clause (if + continue) inside body
4. Does this logic exit the loop early? → if + break inside body (or restructure to while)

---

## 7. Keywords and Special Constructs

### break

**When to use:** You have found what you are looking for, or a condition is met that makes further iteration meaningless. Break exits the innermost loop immediately.

**Where to use:** Inside a loop, always wrapped in an if-condition. You almost never have an unconditional break (that would mean the loop never runs more than once — just remove the loop).

```c
// C - find first occurrence
int result = -1;
for (int i = 0; i < n; i++) {
    if (arr[i] == target) {
        result = i;
        break;  // found it, no need to continue
    }
}
```

```go
// Go - same pattern
result := -1
for i := 0; i < n; i++ {
    if arr[i] == target {
        result = i
        break
    }
}
```

```rust
// Rust - break can return a value from a loop
let result = 'search: loop {
    for (i, &v) in arr.iter().enumerate() {
        if v == target {
            break 'search i as i32;
        }
    }
    break 'search -1;
};
```

**Nested loops and break:** In C and Go, `break` only breaks the innermost loop. If you have nested loops and need to break out of both, use a flag variable or a goto (in C) or labeled break (in Go and Rust).

```go
// Go - labeled break
outer:
for i := 0; i < n; i++ {
    for j := 0; j < m; j++ {
        if matrix[i][j] == target {
            break outer  // breaks out of both loops
        }
    }
}
```

```rust
// Rust - labeled break
'outer: for i in 0..n {
    for j in 0..m {
        if matrix[i][j] == target {
            break 'outer;
        }
    }
}
```

### continue

**When to use:** The current element doesn't satisfy the criteria and should be skipped, but the loop should continue with the next element.

**Where to use:** At the top of the loop body, as a guard clause. Putting it in the middle of the body is a red flag — it means your logic flow is non-linear.

```c
// C - process only even numbers
for (int i = 0; i < n; i++) {
    if (arr[i] % 2 != 0) continue;  // skip odd
    // process even number
    result[count++] = arr[i];
}
```

```go
// Go
for _, v := range arr {
    if v%2 != 0 {
        continue
    }
    result = append(result, v)
}
```

```rust
// Rust - idiomatic: use filter instead
let result: Vec<i32> = arr.iter().filter(|&&v| v % 2 == 0).copied().collect();
// Or with explicit continue:
let mut result = Vec::new();
for &v in arr.iter() {
    if v % 2 != 0 { continue; }
    result.push(v);
}
```

### return (from inside a loop)

**When to use:** The answer is known and there is nothing left to do. Return exits the function entirely — more aggressive than break.

**Where to use:** Inside a condition inside a loop. Same pattern as break, but when the loop is the entire function body.

```c
// C - return as soon as answer is found
bool has_duplicate(int *arr, int n) {
    int seen[10001] = {0};
    for (int i = 0; i < n; i++) {
        if (seen[arr[i]]) return true;   // found duplicate, done
        seen[arr[i]] = 1;
    }
    return false;  // no duplicate found
}
```

```go
// Go
func hasDuplicate(arr []int) bool {
    seen := make(map[int]bool)
    for _, v := range arr {
        if seen[v] {
            return true
        }
        seen[v] = true
    }
    return false
}
```

```rust
// Rust
fn has_duplicate(arr: &[i32]) -> bool {
    let mut seen = std::collections::HashSet::new();
    for &v in arr {
        if !seen.insert(v) {
            return true;
        }
    }
    false
}
```

### goto (C only)

**When to use:** Breaking out of multiple nested loops cleanly. This is one of the few legitimate uses of goto. Do NOT use goto for any other purpose — it makes code unmaintainable.

```c
// C - legitimate goto use: breaking nested loops
for (int i = 0; i < n; i++) {
    for (int j = 0; j < m; j++) {
        if (matrix[i][j] == target) {
            found_i = i;
            found_j = j;
            goto done;  // jump out of both loops
        }
    }
}
done:
// continue here after breaking both loops
```

---

## 8. Thinking Through Data Structures

The data structure you choose determines what operations are cheap and what operations are expensive. Your algorithm must be designed around cheap operations.

### When to Use Each Structure

**Array:**
- You need O(1) random access by index.
- The size is fixed or known ahead of time.
- Cache performance matters (arrays are contiguous in memory).
- You are implementing sorting, two pointers, binary search.

**Hash Map / Hash Set:**
- You need O(1) lookup by value, not by index.
- You are counting frequencies.
- You are checking membership ("have I seen this before?").
- You are storing a mapping from one value to another.
- Mental trigger: "Does X exist in my data?" → hash set. "How many times did X appear?" → hash map.

**Stack:**
- You need to process things in reverse order.
- You need to match opening/closing pairs (parentheses, brackets).
- You need to track "what was I doing before this?" (function calls, DFS).
- Mental trigger: "I need to undo" or "I need to process in LIFO order."

**Queue:**
- You need to process things in the order they arrived (FIFO).
- You are doing BFS (level-by-level traversal).
- Mental trigger: "Process in the order encountered."

**Heap / Priority Queue:**
- You need the minimum or maximum element quickly.
- You need to repeatedly extract the smallest/largest.
- Mental trigger: "Give me the next best/worst element."

**Tree (BST, Trie, Segment Tree):**
- BST: You need sorted order AND fast insert/delete.
- Trie: You are working with strings and need prefix searches.
- Segment Tree: You need range queries with point updates.

**Graph (adjacency list/matrix):**
- The problem involves relationships between entities.
- You need to find paths, connected components, shortest routes.

---

## 9. Pattern Recognition: The 12 Core Patterns

Pattern recognition is the skill of seeing a new problem and recognizing it as an instance of a known pattern. Here are the 12 patterns you need, with the mental triggers that activate them.

### Pattern 1: Two Pointers

**Mental trigger:** Sorted array. Looking for a pair, triplet, or window. Moving in from both ends, or one fast one slow.

**When both pointers start from opposite ends:**
The array is sorted. You want to find if any pair sums to a target. You move left pointer right when the sum is too small (increase it), and right pointer left when the sum is too large (decrease it). They meet in the middle.

```c
// C - two sum in sorted array
bool two_sum(int *arr, int n, int target) {
    int left = 0, right = n - 1;
    while (left < right) {
        int sum = arr[left] + arr[right];
        if (sum == target) return true;
        else if (sum < target) left++;
        else right--;
    }
    return false;
}
```

```go
// Go
func twoSum(arr []int, target int) bool {
    left, right := 0, len(arr)-1
    for left < right {
        sum := arr[left] + arr[right]
        if sum == target {
            return true
        } else if sum < target {
            left++
        } else {
            right--
        }
    }
    return false
}
```

```rust
// Rust
fn two_sum(arr: &[i32], target: i32) -> bool {
    let (mut left, mut right) = (0, arr.len() - 1);
    while left < right {
        let sum = arr[left] + arr[right];
        if sum == target { return true; }
        else if sum < target { left += 1; }
        else { right -= 1; }
    }
    false
}
```

**When both pointers start from the same end (slow/fast):**
Finding cycles (Floyd's algorithm), finding the middle of a linked list, removing duplicates in place.

```c
// C - remove duplicates from sorted array in place
int remove_duplicates(int *arr, int n) {
    if (n == 0) return 0;
    int slow = 0;  // slow pointer: position to write next unique
    for (int fast = 1; fast < n; fast++) {  // fast pointer: scans for new unique values
        if (arr[fast] != arr[slow]) {
            slow++;
            arr[slow] = arr[fast];
        }
    }
    return slow + 1;  // new length
}
```

```go
// Go
func removeDuplicates(arr []int) int {
    if len(arr) == 0 { return 0 }
    slow := 0
    for fast := 1; fast < len(arr); fast++ {
        if arr[fast] != arr[slow] {
            slow++
            arr[slow] = arr[fast]
        }
    }
    return slow + 1
}
```

```rust
// Rust
fn remove_duplicates(arr: &mut Vec<i32>) -> usize {
    if arr.is_empty() { return 0; }
    let mut slow = 0;
    for fast in 1..arr.len() {
        if arr[fast] != arr[slow] {
            slow += 1;
            arr[slow] = arr[fast];
        }
    }
    slow + 1
}
```

### Pattern 2: Sliding Window

**Mental trigger:** Subarray or substring. Contiguous. Optimizing something (max sum, min length, containing all characters).

**Fixed-size window:**

```c
// C - maximum sum subarray of size k
int max_sum_window(int *arr, int n, int k) {
    int window_sum = 0;
    for (int i = 0; i < k; i++) window_sum += arr[i];  // build first window
    int max_sum = window_sum;
    for (int i = k; i < n; i++) {
        window_sum += arr[i] - arr[i - k];  // slide: add new, remove old
        if (window_sum > max_sum) max_sum = window_sum;
    }
    return max_sum;
}
```

```go
// Go
func maxSumWindow(arr []int, k int) int {
    windowSum := 0
    for i := 0; i < k; i++ { windowSum += arr[i] }
    maxSum := windowSum
    for i := k; i < len(arr); i++ {
        windowSum += arr[i] - arr[i-k]
        if windowSum > maxSum { maxSum = windowSum }
    }
    return maxSum
}
```

```rust
// Rust
fn max_sum_window(arr: &[i32], k: usize) -> i32 {
    let mut window_sum: i32 = arr[..k].iter().sum();
    let mut max_sum = window_sum;
    for i in k..arr.len() {
        window_sum += arr[i] - arr[i - k];
        if window_sum > max_sum { max_sum = window_sum; }
    }
    max_sum
}
```

**Variable-size window:** Two pointers, left and right. Expand right until condition is violated. Shrink from left until condition is satisfied again.

```c
// C - minimum subarray length with sum >= target
int min_subarray_len(int *arr, int n, int target) {
    int left = 0, current_sum = 0;
    int min_len = n + 1;  // sentinel "not found"
    for (int right = 0; right < n; right++) {
        current_sum += arr[right];  // expand window
        while (current_sum >= target) {  // shrink window while valid
            int len = right - left + 1;
            if (len < min_len) min_len = len;
            current_sum -= arr[left];
            left++;
        }
    }
    return min_len == n + 1 ? 0 : min_len;
}
```

```go
// Go
func minSubarrayLen(arr []int, target int) int {
    left, currentSum, minLen := 0, 0, len(arr)+1
    for right := 0; right < len(arr); right++ {
        currentSum += arr[right]
        for currentSum >= target {
            if l := right - left + 1; l < minLen { minLen = l }
            currentSum -= arr[left]
            left++
        }
    }
    if minLen == len(arr)+1 { return 0 }
    return minLen
}
```

```rust
// Rust
fn min_subarray_len(arr: &[i32], target: i32) -> usize {
    let (mut left, mut current_sum) = (0, 0);
    let mut min_len = arr.len() + 1;
    for right in 0..arr.len() {
        current_sum += arr[right];
        while current_sum >= target {
            let len = right - left + 1;
            if len < min_len { min_len = len; }
            current_sum -= arr[left];
            left += 1;
        }
    }
    if min_len == arr.len() + 1 { 0 } else { min_len }
}
```

### Pattern 3: Binary Search

**Mental trigger:** Sorted data. Search space is halved at each step. O(log n). Can also apply to answer-space (searching for the answer value itself, not an index).

**Classic binary search:**

```c
// C
int binary_search(int *arr, int n, int target) {
    int low = 0, high = n - 1;
    while (low <= high) {
        int mid = low + (high - low) / 2;  // avoid overflow
        if (arr[mid] == target) return mid;
        else if (arr[mid] < target) low = mid + 1;
        else high = mid - 1;
    }
    return -1;
}
```

```go
// Go
func binarySearch(arr []int, target int) int {
    low, high := 0, len(arr)-1
    for low <= high {
        mid := low + (high-low)/2
        if arr[mid] == target { return mid }
        if arr[mid] < target { low = mid + 1 } else { high = mid - 1 }
    }
    return -1
}
```

```rust
// Rust
fn binary_search(arr: &[i32], target: i32) -> Option<usize> {
    let (mut low, mut high) = (0usize, arr.len());
    while low < high {
        let mid = low + (high - low) / 2;
        if arr[mid] == target { return Some(mid); }
        if arr[mid] < target { low = mid + 1; } else { high = mid; }
    }
    None
}
```

**Why `mid = low + (high - low) / 2` and not `(low + high) / 2`:** Because `(low + high)` can overflow if both are large integers. The subtraction avoids this. This is a classic gotcha.

**Binary search on the answer:** Some problems ask "what is the minimum value of X such that condition Y holds?" If the condition is monotone (once it holds for X, it holds for all larger X), you can binary search on the answer space.

### Pattern 4: Fast and Slow Pointers (Floyd's Cycle Detection)

**Mental trigger:** Linked list. Does it have a cycle? Where does the cycle start? Find the middle of the list.

```c
// C - detect cycle in linked list
typedef struct Node { int val; struct Node *next; } Node;

bool has_cycle(Node *head) {
    Node *slow = head, *fast = head;
    while (fast != NULL && fast->next != NULL) {
        slow = slow->next;          // move one step
        fast = fast->next->next;    // move two steps
        if (slow == fast) return true;  // they meet → cycle
    }
    return false;
}
```

```go
// Go
type Node struct { Val int; Next *Node }

func hasCycle(head *Node) bool {
    slow, fast := head, head
    for fast != nil && fast.Next != nil {
        slow = slow.Next
        fast = fast.Next.Next
        if slow == fast { return true }
    }
    return false
}
```

```rust
// Rust - with raw pointers (simplified with Rc/RefCell in practice)
// Conceptual representation:
fn has_cycle(list: &[usize]) -> bool {
    // list[i] = next index, 0 = null
    if list.is_empty() { return false; }
    let (mut slow, mut fast) = (0, 0);
    loop {
        if list[fast] == 0 || list[list[fast]] == 0 { return false; }
        slow = list[slow];
        fast = list[list[fast]];
        if slow == fast { return true; }
    }
}
```

### Pattern 5: Merge Intervals

**Mental trigger:** List of intervals. Overlapping. Sort by start, then merge.

```c
// C - merge overlapping intervals
void merge_intervals(int intervals[][2], int n, int result[][2], int *result_n) {
    // assume sorted by start
    *result_n = 0;
    for (int i = 0; i < n; i++) {
        if (*result_n == 0 || result[*result_n - 1][1] < intervals[i][0]) {
            // no overlap: add new interval
            result[*result_n][0] = intervals[i][0];
            result[*result_n][1] = intervals[i][1];
            (*result_n)++;
        } else {
            // overlap: extend the last interval's end
            if (intervals[i][1] > result[*result_n - 1][1])
                result[*result_n - 1][1] = intervals[i][1];
        }
    }
}
```

```go
// Go
func mergeIntervals(intervals [][]int) [][]int {
    if len(intervals) == 0 { return nil }
    // sort by start (assume sorted here for clarity)
    result := [][]int{intervals[0]}
    for _, iv := range intervals[1:] {
        last := result[len(result)-1]
        if iv[0] <= last[1] {
            if iv[1] > last[1] { last[1] = iv[1] }
        } else {
            result = append(result, iv)
        }
    }
    return result
}
```

```rust
// Rust
fn merge_intervals(intervals: &mut Vec<[i32; 2]>) -> Vec<[i32; 2]> {
    intervals.sort_by_key(|iv| iv[0]);
    let mut result: Vec<[i32; 2]> = vec![];
    for &iv in intervals.iter() {
        if let Some(last) = result.last_mut() {
            if iv[0] <= last[1] {
                last[1] = last[1].max(iv[1]);
                continue;
            }
        }
        result.push(iv);
    }
    result
}
```

### Pattern 6: Prefix Sums

**Mental trigger:** Range sum queries. "Sum of subarray from i to j." Preprocessing for O(1) queries.

The insight: if `prefix[i]` = sum of `arr[0..i-1]`, then sum of `arr[l..r]` = `prefix[r+1] - prefix[l]`.

```c
// C
void build_prefix(int *arr, int n, int *prefix) {
    prefix[0] = 0;
    for (int i = 0; i < n; i++)
        prefix[i + 1] = prefix[i] + arr[i];
}

int range_sum(int *prefix, int l, int r) {
    return prefix[r + 1] - prefix[l];
}
```

```go
// Go
func buildPrefix(arr []int) []int {
    prefix := make([]int, len(arr)+1)
    for i, v := range arr { prefix[i+1] = prefix[i] + v }
    return prefix
}
func rangeSum(prefix []int, l, r int) int { return prefix[r+1] - prefix[l] }
```

```rust
// Rust
fn build_prefix(arr: &[i32]) -> Vec<i32> {
    let mut prefix = vec![0i32; arr.len() + 1];
    for (i, &v) in arr.iter().enumerate() { prefix[i+1] = prefix[i] + v; }
    prefix
}
fn range_sum(prefix: &[i32], l: usize, r: usize) -> i32 { prefix[r+1] - prefix[l] }
```

### Pattern 7: BFS (Breadth-First Search)

**Mental trigger:** Shortest path in an unweighted graph or grid. Level-by-level traversal. Find minimum steps.

```c
// C - BFS on grid (shortest path)
#include <stdbool.h>
int bfs(int grid[][100], int rows, int cols, int sr, int sc, int er, int ec) {
    int queue[10000][2];
    bool visited[100][100] = {false};
    int head = 0, tail = 0;
    queue[tail][0] = sr; queue[tail][1] = sc; tail++;
    visited[sr][sc] = true;
    int dist = 0;
    int dx[] = {0, 0, 1, -1};
    int dy[] = {1, -1, 0, 0};
    while (head < tail) {
        int level_size = tail - head;
        for (int i = 0; i < level_size; i++) {
            int r = queue[head][0], c = queue[head][1]; head++;
            if (r == er && c == ec) return dist;
            for (int d = 0; d < 4; d++) {
                int nr = r + dx[d], nc = c + dy[d];
                if (nr >= 0 && nr < rows && nc >= 0 && nc < cols
                    && !visited[nr][nc] && grid[nr][nc] == 0) {
                    visited[nr][nc] = true;
                    queue[tail][0] = nr; queue[tail][1] = nc; tail++;
                }
            }
        }
        dist++;
    }
    return -1;
}
```

```go
// Go
func bfsGrid(grid [][]int, sr, sc, er, ec int) int {
    rows, cols := len(grid), len(grid[0])
    visited := make([][]bool, rows)
    for i := range visited { visited[i] = make([]bool, cols) }
    queue := [][2]int{{sr, sc}}
    visited[sr][sc] = true
    dirs := [][2]int{{0,1},{0,-1},{1,0},{-1,0}}
    dist := 0
    for len(queue) > 0 {
        for size := len(queue); size > 0; size-- {
            r, c := queue[0][0], queue[0][1]
            queue = queue[1:]
            if r == er && c == ec { return dist }
            for _, d := range dirs {
                nr, nc := r+d[0], c+d[1]
                if nr >= 0 && nr < rows && nc >= 0 && nc < cols && !visited[nr][nc] && grid[nr][nc] == 0 {
                    visited[nr][nc] = true
                    queue = append(queue, [2]int{nr, nc})
                }
            }
        }
        dist++
    }
    return -1
}
```

### Pattern 8: DFS / Backtracking

**Mental trigger:** Explore all possibilities. Find all permutations, subsets, paths. Pruning invalid states early.

```c
// C - generate all permutations
void permute(int *arr, int n, int start, int result[][100], int *count) {
    if (start == n) {
        for (int i = 0; i < n; i++) result[*count][i] = arr[i];
        (*count)++;
        return;
    }
    for (int i = start; i < n; i++) {
        int tmp = arr[start]; arr[start] = arr[i]; arr[i] = tmp;  // swap
        permute(arr, n, start + 1, result, count);                  // recurse
        tmp = arr[start]; arr[start] = arr[i]; arr[i] = tmp;  // undo swap (backtrack)
    }
}
```

```go
// Go
func permute(arr []int, start int, result *[][]int) {
    if start == len(arr) {
        tmp := make([]int, len(arr))
        copy(tmp, arr)
        *result = append(*result, tmp)
        return
    }
    for i := start; i < len(arr); i++ {
        arr[start], arr[i] = arr[i], arr[start]
        permute(arr, start+1, result)
        arr[start], arr[i] = arr[i], arr[start]  // backtrack
    }
}
```

```rust
// Rust
fn permute(arr: &mut Vec<i32>, start: usize, result: &mut Vec<Vec<i32>>) {
    if start == arr.len() {
        result.push(arr.clone());
        return;
    }
    for i in start..arr.len() {
        arr.swap(start, i);
        permute(arr, start + 1, result);
        arr.swap(start, i);  // backtrack
    }
}
```

### Pattern 9: Dynamic Programming

**Mental trigger:** Overlapping subproblems + optimal substructure. "What is the maximum/minimum/count of ways?" The answer to a larger problem depends on answers to smaller problems.

**The thinking process for DP:**
1. Define the subproblem: what does `dp[i]` mean?
2. Write the recurrence: how does `dp[i]` relate to smaller `dp` values?
3. Identify base cases.
4. Determine the order of computation (bottom-up).

```c
// C - longest increasing subsequence (LIS)
int lis(int *arr, int n) {
    int dp[n];
    for (int i = 0; i < n; i++) dp[i] = 1;  // base: each element alone
    int max_len = 1;
    for (int i = 1; i < n; i++) {
        for (int j = 0; j < i; j++) {
            if (arr[j] < arr[i]) {         // can extend
                if (dp[j] + 1 > dp[i])    // recurrence
                    dp[i] = dp[j] + 1;
            }
        }
        if (dp[i] > max_len) max_len = dp[i];
    }
    return max_len;
}
```

```go
// Go
func lis(arr []int) int {
    n := len(arr)
    dp := make([]int, n)
    for i := range dp { dp[i] = 1 }
    maxLen := 1
    for i := 1; i < n; i++ {
        for j := 0; j < i; j++ {
            if arr[j] < arr[i] && dp[j]+1 > dp[i] {
                dp[i] = dp[j] + 1
            }
        }
        if dp[i] > maxLen { maxLen = dp[i] }
    }
    return maxLen
}
```

```rust
// Rust
fn lis(arr: &[i32]) -> usize {
    let n = arr.len();
    let mut dp = vec![1usize; n];
    let mut max_len = 1;
    for i in 1..n {
        for j in 0..i {
            if arr[j] < arr[i] && dp[j] + 1 > dp[i] {
                dp[i] = dp[j] + 1;
            }
        }
        if dp[i] > max_len { max_len = dp[i]; }
    }
    max_len
}
```

### Pattern 10: Monotonic Stack

**Mental trigger:** "Next greater element," "previous smaller element," span problems, histogram problems. The stack maintains a sequence that is monotonically increasing or decreasing.

```c
// C - next greater element for each index
void next_greater(int *arr, int n, int *result) {
    int stack[n];  // stores indices
    int top = -1;
    for (int i = 0; i < n; i++) result[i] = -1;  // default: no greater element
    for (int i = 0; i < n; i++) {
        while (top >= 0 && arr[stack[top]] < arr[i]) {
            result[stack[top]] = arr[i];  // arr[i] is the next greater for stack[top]
            top--;
        }
        stack[++top] = i;
    }
}
```

```go
// Go
func nextGreater(arr []int) []int {
    n := len(arr)
    result := make([]int, n)
    for i := range result { result[i] = -1 }
    stack := []int{}  // stores indices
    for i, v := range arr {
        for len(stack) > 0 && arr[stack[len(stack)-1]] < v {
            result[stack[len(stack)-1]] = v
            stack = stack[:len(stack)-1]
        }
        stack = append(stack, i)
    }
    return result
}
```

```rust
// Rust
fn next_greater(arr: &[i32]) -> Vec<i32> {
    let n = arr.len();
    let mut result = vec![-1i32; n];
    let mut stack: Vec<usize> = vec![];
    for (i, &v) in arr.iter().enumerate() {
        while let Some(&top) = stack.last() {
            if arr[top] < v {
                result[top] = v;
                stack.pop();
            } else { break; }
        }
        stack.push(i);
    }
    result
}
```

### Pattern 11: Union-Find (Disjoint Set Union)

**Mental trigger:** Connected components. "Are X and Y connected?" "Merge groups." Kruskal's MST.

```c
// C - Union-Find with path compression and union by rank
int parent[100001], rank_arr[100001];

void init(int n) {
    for (int i = 0; i <= n; i++) { parent[i] = i; rank_arr[i] = 0; }
}

int find(int x) {
    if (parent[x] != x) parent[x] = find(parent[x]);  // path compression
    return parent[x];
}

void unite(int x, int y) {
    int rx = find(x), ry = find(y);
    if (rx == ry) return;
    if (rank_arr[rx] < rank_arr[ry]) { int t = rx; rx = ry; ry = t; }
    parent[ry] = rx;
    if (rank_arr[rx] == rank_arr[ry]) rank_arr[rx]++;
}
```

```go
// Go
type DSU struct { parent, rank []int }

func NewDSU(n int) *DSU {
    d := &DSU{make([]int, n), make([]int, n)}
    for i := range d.parent { d.parent[i] = i }
    return d
}
func (d *DSU) Find(x int) int {
    if d.parent[x] != x { d.parent[x] = d.Find(d.parent[x]) }
    return d.parent[x]
}
func (d *DSU) Union(x, y int) {
    rx, ry := d.Find(x), d.Find(y)
    if rx == ry { return }
    if d.rank[rx] < d.rank[ry] { rx, ry = ry, rx }
    d.parent[ry] = rx
    if d.rank[rx] == d.rank[ry] { d.rank[rx]++ }
}
```

```rust
// Rust
struct DSU { parent: Vec<usize>, rank: Vec<usize> }

impl DSU {
    fn new(n: usize) -> Self { DSU { parent: (0..n).collect(), rank: vec![0; n] } }
    fn find(&mut self, x: usize) -> usize {
        if self.parent[x] != x { self.parent[x] = self.find(self.parent[x]); }
        self.parent[x]
    }
    fn union(&mut self, x: usize, y: usize) {
        let (rx, ry) = (self.find(x), self.find(y));
        if rx == ry { return; }
        if self.rank[rx] < self.rank[ry] { self.parent[rx] = ry; }
        else if self.rank[rx] > self.rank[ry] { self.parent[ry] = rx; }
        else { self.parent[ry] = rx; self.rank[rx] += 1; }
    }
}
```

### Pattern 12: Divide and Conquer

**Mental trigger:** The problem can be split into independent subproblems of the same type. Merge sort, quick sort, finding maximum in an array recursively.

```c
// C - merge sort
void merge(int *arr, int l, int m, int r) {
    int n1 = m - l + 1, n2 = r - m;
    int left[n1], right[n2];
    for (int i = 0; i < n1; i++) left[i] = arr[l + i];
    for (int i = 0; i < n2; i++) right[i] = arr[m + 1 + i];
    int i = 0, j = 0, k = l;
    while (i < n1 && j < n2)
        arr[k++] = (left[i] <= right[j]) ? left[i++] : right[j++];
    while (i < n1) arr[k++] = left[i++];
    while (j < n2) arr[k++] = right[j++];
}

void merge_sort(int *arr, int l, int r) {
    if (l >= r) return;
    int m = l + (r - l) / 2;
    merge_sort(arr, l, m);
    merge_sort(arr, m + 1, r);
    merge(arr, l, m, r);
}
```

```go
// Go
func mergeSort(arr []int) []int {
    if len(arr) <= 1 { return arr }
    mid := len(arr) / 2
    left := mergeSort(arr[:mid])
    right := mergeSort(arr[mid:])
    return mergeSorted(left, right)
}
func mergeSorted(l, r []int) []int {
    result := make([]int, 0, len(l)+len(r))
    for len(l) > 0 && len(r) > 0 {
        if l[0] <= r[0] { result = append(result, l[0]); l = l[1:] } else { result = append(result, r[0]); r = r[1:] }
    }
    return append(append(result, l...), r...)
}
```

```rust
// Rust
fn merge_sort(arr: &mut [i32]) {
    let n = arr.len();
    if n <= 1 { return; }
    let mid = n / 2;
    let mut left = arr[..mid].to_vec();
    let mut right = arr[mid..].to_vec();
    merge_sort(&mut left);
    merge_sort(&mut right);
    let (mut i, mut j, mut k) = (0, 0, 0);
    while i < left.len() && j < right.len() {
        if left[i] <= right[j] { arr[k] = left[i]; i += 1; } else { arr[k] = right[j]; j += 1; }
        k += 1;
    }
    while i < left.len() { arr[k] = left[i]; i += 1; k += 1; }
    while j < right.len() { arr[k] = right[j]; j += 1; k += 1; }
}
```

---

## 10. The Five-Phase Solving Process

This is the process to follow for every single DSA problem.

### Phase 1: Read and Restate (5 minutes)

Read the problem. Then close it and restate it in your own words. If you cannot, you do not understand the problem. Do NOT start coding in this phase. Common mistake: starting to code immediately because the problem "looks familiar." Problems that look familiar often have crucial differences.

### Phase 2: Work Small Examples by Hand (5 minutes)

Take the smallest meaningful example. Work through it manually, step by step. Write down what you do. These manual steps are your algorithm — you are discovering your algorithm by doing it, not thinking abstractly about it.

For `[3, 1, 4, 1, 5, 9]`, finding duplicates:
- See 3. Haven't seen it. Remember 3.
- See 1. Haven't seen it. Remember 1.
- See 4. Haven't seen it. Remember 4.
- See 1. HAVE seen it. → Found duplicate: 1.

This manual process tells you: you need to "remember" what you've seen. That means you need a set. That means hash set. The data structure choice fell out of the manual process.

### Phase 3: Identify the Pattern (3 minutes)

Match the problem to one of the 12 core patterns. Ask:
- Is the data sorted? → binary search or two pointers
- Is the problem about subarrays/substrings? → sliding window or prefix sums
- Is the problem about paths/connectivity? → BFS or DFS
- Is the problem about optimal decisions across a sequence? → DP
- Is the problem asking for all possibilities? → backtracking

### Phase 4: Write Pseudocode First (5 minutes)

Do not write real code yet. Write pseudocode. Pseudocode forces you to articulate the algorithm without getting bogged down in syntax. It should be specific enough that translating to real code is mechanical.

```
function solve(arr, target):
    sort arr
    left = 0, right = len(arr) - 1
    while left < right:
        sum = arr[left] + arr[right]
        if sum == target: return (left, right)
        if sum < target: left += 1
        else: right -= 1
    return not found
```

### Phase 5: Code, Test, Optimize (10+ minutes)

Now write the actual code. After writing, test on your small examples first (they should pass immediately since you worked them by hand). Then test on edge cases. Only then try to submit or optimize.

---

## 11. Where to Start: A Complete Decision Framework

This section directly addresses your main question: "Where to start? Loop or condition? Condition in loop or in conditional statement?"

### The Starting Point Decision Tree

```
START HERE
│
├── Does the problem need to look at more than one element?
│   │
│   ├── NO → No loop needed. Use direct formula or single access.
│   │
│   └── YES → You need a loop. Now ask:
│       │
│       ├── Do I know exactly how many iterations I need?
│       │   │
│       │   ├── YES → Use for loop with fixed bound.
│       │   └── NO  → Use while loop with convergence condition.
│       │
│       ├── Do I need to compare each element with all others?
│       │   │
│       │   └── YES → Nested loop. Outer: i from 0 to n. Inner: j from i+1 to n.
│       │
│       └── Do I need to look at a contiguous window?
│           │
│           └── YES → Sliding window. One loop, two pointers.
│
After choosing loop structure, ask:
│
├── Is the operation on each element always the same?
│   │
│   ├── YES → No if-condition needed inside loop.
│   └── NO  → Need if-condition inside loop.
│       │
│       ├── Does the condition SKIP this element? → if + continue (guard clause)
│       ├── Does the condition CHOOSE between two operations? → if-else
│       ├── Does the condition EXIT early? → if + break (or return)
│       └── Does the condition UPDATE state differently? → if-else or if-else-if
│
After loop(s), ask:
│
└── Is the result computed incrementally during the loop?
    │
    ├── YES → Return or output the accumulated result.
    └── NO  → One final computation or check after the loop.
```

### Concrete Starting Point Rules

**Rule 1: Start with the data traversal, not the decision.**

The first thing to write is the loop that visits the data. Don't think about what you're checking yet — just establish how you move through the data.

```c
// Step 1: establish traversal
for (int i = 0; i < n; i++) {
    // what you do here comes in step 2
}
```

**Rule 2: Establish state variables before the loop.**

State variables are things that change as you loop and that you need to remember across iterations. Declare and initialize them before the loop starts.

```c
int max_so_far = INT_MIN;  // state variable
int current_sum = 0;       // state variable
for (int i = 0; i < n; i++) {
    // use and update state variables
}
```

**Rule 3: Write the update rule inside the loop before the condition.**

When confused about where to put a condition, first write what ALWAYS happens (the unconditional update), then add the condition around it.

```c
// wrong order of thinking
for (int i = 0; i < n; i++) {
    if (???) {
        current_sum += arr[i];  // I know this happens, but when?
    }
}

// right order of thinking
for (int i = 0; i < n; i++) {
    current_sum += arr[i];  // first: what always happens
    // then: what is the condition that changes behavior?
    if (current_sum > max_so_far) max_so_far = current_sum;
    if (current_sum < 0) current_sum = 0;
}
```

**Rule 4: Condition in loop vs condition in conditional statement — the governing rule:**

Use a condition IN the loop termination (`while (condition)`) when the condition determines whether to run another full iteration. Use a condition INSIDE the loop body (`if (condition)`) when the condition determines what to do in the current iteration.

The two are not interchangeable. Confusing them is the most common logical error beginners make.

---

## 12. Dry Running: The Most Underrated Skill

Dry running means tracing through your code manually, step by step, with a specific input. It is the single most important debugging skill. Most bugs become obvious during a dry run.

### How to Dry Run Properly

1. Draw a table with columns: loop variable, all state variables, array contents if modified.
2. Fill in one row per iteration.
3. At each conditional branch, write down which branch is taken and why.
4. Compare the final state to the expected output.

**Example: Dry run of remove duplicates**

Input: `[1, 1, 2, 3, 3]`, n = 5

| fast | arr[fast] | arr[slow] | slow | action |
|------|-----------|-----------|------|--------|
| 1    | 1         | 1         | 0    | equal, skip |
| 2    | 2         | 1         | 0    | different → slow=1, arr[1]=2 |
| 3    | 3         | 2         | 1    | different → slow=2, arr[2]=3 |
| 4    | 3         | 3         | 2    | equal, skip |

Result: `[1, 2, 3, ...]`, new length = slow + 1 = 3. ✓

### What to Look for During Dry Runs

- Off-by-one errors in loop bounds (does the loop run one too many or too few times?)
- Pointer/index crossing (do two pointers pass each other when they shouldn't?)
- Uninitialized state (is a variable used before it is set?)
- Wrong update order (is state updated before or after it should be used?)
- Missing edge cases (what happens on the first iteration? the last?)

---

## 13. Edge Cases: Systematic Thinking

Edge cases are inputs that lie at the boundary of normal behavior. Always check these before declaring your solution correct.

### The Universal Edge Case Checklist

**Size edge cases:**
- Empty input (n = 0)
- Single element (n = 1)
- Two elements (n = 2)
- Maximum size (n = constraint maximum)

**Value edge cases:**
- All elements the same
- All elements sorted ascending
- All elements sorted descending (reverse sorted)
- All elements negative
- Mix of positive and negative
- Presence of zeros
- INT_MAX and INT_MIN (overflow risk)

**Structure edge cases (for linked lists, trees, graphs):**
- Empty structure (null head, null root)
- Single node
- Linear chain (no branching in a tree)
- Complete binary tree
- Disconnected graph (for BFS/DFS)
- Graph with a cycle

**Problem-specific edge cases:**
- For target-sum problems: target is negative, target is zero, target is larger than all elements.
- For substring problems: empty string, string of length 1, all characters the same.
- For binary search: target is smaller than all elements, target is larger than all elements, target is not present.

### Testing Strategy

Always test in this order:
1. The example from the problem statement (should pass trivially if you understood it).
2. Your hand-worked small example.
3. Edge cases from the checklist above.
4. A large stress test (to catch performance issues).

---

## 14. Complete Problem Walkthroughs

### Walkthrough 1: Maximum Subarray (Kadane's Algorithm)

**Problem:** Given an array of integers, find the contiguous subarray with the largest sum.

**Phase 1 (Restate):** I have an array. I need to find a contiguous block of elements (at least one) that has the largest possible sum.

**Phase 2 (Small example):** `[-2, 1, -3, 4, -1, 2, 1, -5, 4]`
- Starting fresh: try starting subarray at each position.
- At index 3 (value 4): sum = 4.
- Extend to index 4 (value -1): sum = 3. Still better than restarting (which would give -1).
- Extend to index 5 (value 2): sum = 5.
- Extend to index 6 (value 1): sum = 6. This seems to be the max.
- The key insight: if my current running sum goes negative, I should restart from the next element (starting fresh is better than carrying a negative burden).

**Phase 3 (Pattern):** This is a DP pattern. `current_sum[i] = max(arr[i], current_sum[i-1] + arr[i])`. The subproblem is: what is the maximum sum of a subarray ending at index i?

**Phase 4 (Pseudocode):**
```
current_sum = arr[0]
max_sum = arr[0]
for i from 1 to n-1:
    current_sum = max(arr[i], current_sum + arr[i])
    max_sum = max(max_sum, current_sum)
return max_sum
```

**Phase 5 (Implementation):**

```c
// C
int max_subarray(int *arr, int n) {
    int current = arr[0];
    int max_sum = arr[0];
    for (int i = 1; i < n; i++) {
        if (current < 0) current = arr[i];  // restart
        else current += arr[i];             // extend
        if (current > max_sum) max_sum = current;
    }
    return max_sum;
}
```

```go
// Go
func maxSubarray(arr []int) int {
    current, maxSum := arr[0], arr[0]
    for _, v := range arr[1:] {
        if current < 0 { current = v } else { current += v }
        if current > maxSum { maxSum = current }
    }
    return maxSum
}
```

```rust
// Rust
fn max_subarray(arr: &[i32]) -> i32 {
    let (mut current, mut max_sum) = (arr[0], arr[0]);
    for &v in &arr[1..] {
        current = if current < 0 { v } else { current + v };
        if current > max_sum { max_sum = current; }
    }
    max_sum
}
```

**Where is the condition?** Inside the loop body. It decides whether to extend or restart the current subarray. This is a business logic decision, not a loop control decision.

**Dry run:**

| i | arr[i] | current | max_sum | action |
|---|--------|---------|---------|--------|
| 0 | -2     | -2      | -2      | init   |
| 1 | 1      | 1       | 1       | restart (current was -2 < 0) |
| 2 | -3     | -2      | 1       | extend |
| 3 | 4      | 4       | 4       | restart (current was -2 < 0) |
| 4 | -1     | 3       | 4       | extend |
| 5 | 2      | 5       | 5       | extend |
| 6 | 1      | 6       | 6       | extend |
| 7 | -5     | 1       | 6       | extend |
| 8 | 4      | 5       | 6       | extend |

Answer: 6. ✓

---

### Walkthrough 2: Valid Parentheses

**Problem:** Given a string of parentheses `()[]{}`, determine if it is valid. Valid means every open bracket has a matching close bracket in the right order.

**Phase 2 (Manual):** `"({[]})"`
- See `(`: open, remember it.
- See `{`: open, remember it. Stack: `(`, `{`.
- See `[`: open, remember it. Stack: `(`, `{`, `[`.
- See `]`: close. Match with top of stack `[`. ✓ Pop. Stack: `(`, `{`.
- See `}`: close. Match with top of stack `{`. ✓ Pop. Stack: `(`.
- See `)`: close. Match with top of stack `(`. ✓ Pop. Stack empty.
- Valid.

**Phase 3 (Pattern):** Stack. Open brackets push onto stack. Close brackets pop and verify match.

```c
// C
#include <stdbool.h>
bool is_valid(char *s) {
    int n = strlen(s);
    char stack[n + 1];
    int top = -1;
    for (int i = 0; s[i]; i++) {
        char c = s[i];
        if (c == '(' || c == '[' || c == '{') {
            stack[++top] = c;  // push
        } else {
            if (top < 0) return false;  // nothing to match
            char open = stack[top--];   // pop
            if (c == ')' && open != '(') return false;
            if (c == ']' && open != '[') return false;
            if (c == '}' && open != '{') return false;
        }
    }
    return top == -1;  // stack must be empty
}
```

```go
// Go
func isValid(s string) bool {
    stack := []rune{}
    match := map[rune]rune{')': '(', ']': '[', '}': '{'}
    for _, c := range s {
        if c == '(' || c == '[' || c == '{' {
            stack = append(stack, c)
        } else {
            if len(stack) == 0 || stack[len(stack)-1] != match[c] {
                return false
            }
            stack = stack[:len(stack)-1]
        }
    }
    return len(stack) == 0
}
```

```rust
// Rust
fn is_valid(s: &str) -> bool {
    let mut stack = Vec::new();
    for c in s.chars() {
        match c {
            '(' | '[' | '{' => stack.push(c),
            ')' => if stack.pop() != Some('(') { return false; },
            ']' => if stack.pop() != Some('[') { return false; },
            '}' => if stack.pop() != Some('{') { return false; },
            _   => {}
        }
    }
    stack.is_empty()
}
```

**Where are the conditions?**
- One condition is in the else-branch: when we see a closing bracket, check if the stack is empty (guard clause) and check if the top matches.
- Post-loop condition: `top == -1` (stack must be empty — unmatched opens would remain).

---

### Walkthrough 3: Number of Islands (BFS/DFS on Grid)

**Problem:** Given a 2D grid of '1' (land) and '0' (water), count the number of islands. An island is a group of connected '1's.

**Phase 2 (Manual):** 
```
1 1 0 0 0
1 1 0 0 0
0 0 1 0 0
0 0 0 1 1
```
- See '1' at (0,0): island #1. Mark all connected '1's as visited.
- (0,0), (0,1), (1,0), (1,1) all connected → marked.
- See '0' at (0,2): skip.
- See '1' at (2,2): island #2. Mark (2,2).
- See '1' at (3,3): island #3. Mark (3,3), (3,4).
- Total: 3 islands.

**Phase 3 (Pattern):** DFS/BFS on a grid. Outer loop visits all cells. When an unvisited land cell is found, start BFS/DFS to mark all connected land.

```c
// C
void dfs(char **grid, int rows, int cols, int r, int c) {
    if (r < 0 || r >= rows || c < 0 || c >= cols || grid[r][c] != '1') return;
    grid[r][c] = '0';  // mark visited by changing to water
    dfs(grid, rows, cols, r + 1, c);
    dfs(grid, rows, cols, r - 1, c);
    dfs(grid, rows, cols, r, c + 1);
    dfs(grid, rows, cols, r, c - 1);
}

int num_islands(char **grid, int rows, int cols) {
    int count = 0;
    for (int r = 0; r < rows; r++) {
        for (int c = 0; c < cols; c++) {
            if (grid[r][c] == '1') {   // found unvisited land
                count++;               // new island
                dfs(grid, rows, cols, r, c);  // mark all connected land
            }
        }
    }
    return count;
}
```

```go
// Go
func numIslands(grid [][]byte) int {
    rows, cols := len(grid), len(grid[0])
    var dfs func(r, c int)
    dfs = func(r, c int) {
        if r < 0 || r >= rows || c < 0 || c >= cols || grid[r][c] != '1' { return }
        grid[r][c] = '0'
        dfs(r+1, c); dfs(r-1, c); dfs(r, c+1); dfs(r, c-1)
    }
    count := 0
    for r := 0; r < rows; r++ {
        for c := 0; c < cols; c++ {
            if grid[r][c] == '1' { count++; dfs(r, c) }
        }
    }
    return count
}
```

```rust
// Rust
fn num_islands(grid: &mut Vec<Vec<char>>) -> i32 {
    let (rows, cols) = (grid.len(), grid[0].len());
    fn dfs(grid: &mut Vec<Vec<char>>, r: i32, c: i32, rows: i32, cols: i32) {
        if r < 0 || r >= rows || c < 0 || c >= cols || grid[r as usize][c as usize] != '1' { return; }
        grid[r as usize][c as usize] = '0';
        dfs(grid, r+1, c, rows, cols); dfs(grid, r-1, c, rows, cols);
        dfs(grid, r, c+1, rows, cols); dfs(grid, r, c-1, rows, cols);
    }
    let mut count = 0;
    for r in 0..rows as i32 {
        for c in 0..cols as i32 {
            if grid[r as usize][c as usize] == '1' { count += 1; dfs(grid, r, c, rows as i32, cols as i32); }
        }
    }
    count
}
```

**Where are the conditions?**
- In the DFS function: the base case condition at the top. This is a guard clause — if the current cell is out of bounds or already water, return. This is the "recursion termination condition" — analogous to the loop termination condition.
- In the outer nested loops: `if (grid[r][c] == '1')` — this is a business logic condition inside a loop that triggers counting and marking.

---

## 15. Language-Specific Idioms: C, Go, Rust

### C: Low-Level Control

C gives you maximum control and maximum responsibility. Every algorithm is transparent — you see every memory access.

**Key C idioms for DSA:**

```c
// Swap without temp variable (integer XOR swap)
a ^= b; b ^= a; a ^= b;  // works but confusing; use temp for clarity

// Dynamic array with realloc
int *arr = malloc(n * sizeof(int));
arr = realloc(arr, new_n * sizeof(int));  // grow array

// Stack implementation using array
int stack[MAX_SIZE];
int top = -1;
stack[++top] = value;  // push
int val = stack[top--]; // pop

// Queue using circular buffer
int queue[MAX_SIZE];
int head = 0, tail = 0;
queue[tail % MAX_SIZE] = value; tail++;  // enqueue
int val = queue[head % MAX_SIZE]; head++; // dequeue

// Avoid INT_MIN/INT_MAX for initialization
int min_val = arr[0];  // initialize to first element, not INT_MAX
int max_val = arr[0];

// String as char array
char s[] = "hello";
int len = strlen(s);  // O(n) — call once, store result
```

**C: When to use which loop type:**

```c
// for loop: index-based traversal with known bounds
for (int i = 0; i < n; i++) { }

// while loop: condition-based, unknown number of iterations
while (left < right) { }

// do-while: must execute at least once
do { read_input(); } while (!valid());

// Pointer-based loop (faster than index in some cases)
int *ptr = arr;
int *end = arr + n;
while (ptr < end) {
    // *ptr instead of arr[i]
    ptr++;
}
```

### Go: Clean and Idiomatic

Go has one loop keyword (`for`) that handles all loop shapes. Its standard library provides slices, maps, and channels.

```go
// Go range loop idioms
for i, v := range slice { }      // index and value
for i := range slice { }         // index only
for _, v := range slice { }      // value only
for k, v := range myMap { }      // map key-value
for i, c := range "hello" { }    // string: i is byte index, c is rune

// Slice tricks
slice = append(slice, elem)           // push
slice = slice[:len(slice)-1]          // pop (stack)
slice = slice[1:]                     // dequeue (inefficient for large queues)
slice = append(slice[0:i], slice[i+1:]...) // delete element at i

// Go does not have a while keyword; use for
for condition { }    // while loop
for { }              // infinite loop (use with break)

// Multiple return values for error handling
if val, ok := myMap[key]; ok {
    // key exists, val is valid
}

// defer for cleanup (useful in complex algorithms with early returns)
func process() {
    defer cleanup()  // called when function returns, regardless of which return
    // ...
}
```

### Rust: Safety and Performance

Rust's ownership system prevents entire classes of bugs (use-after-free, double-free, data races). The learning curve is steep but the guarantees are worth it.

```rust
// Rust iterator chains (functional style)
let sum: i32 = arr.iter().sum();
let max = arr.iter().max().unwrap();
let filtered: Vec<i32> = arr.iter().filter(|&&x| x > 0).copied().collect();
let doubled: Vec<i32> = arr.iter().map(|&x| x * 2).collect();

// Two useful iterator adapters for DSA
arr.iter().enumerate()           // (index, value) pairs
arr.windows(k)                   // overlapping windows of size k
arr.chunks(k)                    // non-overlapping chunks of size k

// Rust's match for exhaustive conditions
match value {
    0        => println!("zero"),
    1..=9    => println!("single digit"),
    10..=99  => println!("double digit"),
    _        => println!("large"),
}

// Option and Result handling
if let Some(val) = maybe_val { /* val is unwrapped */ }
let val = maybe_val.unwrap_or(default);
let val = maybe_val.unwrap_or_else(|| compute_default());

// Vec as stack
let mut stack: Vec<i32> = Vec::new();
stack.push(val);
let top = stack.pop();         // returns Option<i32>
let top_ref = stack.last();    // peek without removing

// HashMap
use std::collections::HashMap;
let mut freq: HashMap<i32, i32> = HashMap::new();
*freq.entry(key).or_insert(0) += 1;  // frequency counting idiom

// BinaryHeap (max-heap by default)
use std::collections::BinaryHeap;
let mut heap = BinaryHeap::new();
heap.push(val);
let max = heap.pop();  // Option<T>
// For min-heap, use Reverse wrapper
use std::cmp::Reverse;
let mut min_heap = BinaryHeap::new();
min_heap.push(Reverse(val));
let min = min_heap.pop().map(|Reverse(v)| v);
```

### Comparison: Same Algorithm in All Three Languages

**Kadane's Algorithm — Annotated Comparison**

```c
// C: Explicit, procedural, manual bounds
int max_subarray(int *arr, int n) {
    int current = arr[0];    // must handle empty case manually before calling
    int max_sum = arr[0];
    for (int i = 1; i < n; i++) {
        current = (arr[i] > current + arr[i]) ? arr[i] : current + arr[i];
        if (current > max_sum) max_sum = current;
    }
    return max_sum;
}
```

```go
// Go: Compact, clear syntax, no manual memory
func maxSubarray(arr []int) int {
    current, maxSum := arr[0], arr[0]
    for _, v := range arr[1:] {  // range with slice syntax is elegant
        if v > current+v { current = v } else { current += v }
        if current > maxSum { maxSum = current }
    }
    return maxSum
}
```

```rust
// Rust: Ownership guarantees correctness; functional style available
fn max_subarray(arr: &[i32]) -> i32 {
    arr[1..].iter().fold(
        (arr[0], arr[0]),  // (current, max_sum)
        |(current, max_sum), &v| {
            let new_current = current.max(0) + v;  // restart if current < 0
            (new_current, max_sum.max(new_current))
        }
    ).1  // extract max_sum from the tuple
}
// Or imperative style (clearer for learning):
fn max_subarray_imperative(arr: &[i32]) -> i32 {
    let (mut current, mut max_sum) = (arr[0], arr[0]);
    for &v in &arr[1..] {
        current = v.max(current + v);
        max_sum = max_sum.max(current);
    }
    max_sum
}
```

---

## Summary: The Mental Model in One Page

**Before coding:**
- Classify the problem (existence, count, optimization, construction, transformation, detection).
- Check constraints → determine acceptable time complexity → choose algorithm class.
- Work a small example by hand → let the manual process reveal the algorithm.
- Match to a pattern: two pointers, sliding window, binary search, BFS, DFS, DP, stack, union-find.

**When coding:**
- Write the outermost structure first (which kind of loop? recursive or iterative?).
- Establish state variables before the loop.
- Write what always happens in the loop body first. Then add conditions around it.
- Loop condition = when to keep going. Body condition = what to do this iteration.
- Guard clauses (if + continue) at top of body for elements to skip.
- Early exit (if + break or return) when the answer is found.

**Loop starting points:**
- Sorting/searching in a sorted array → start with while loop, two pointers.
- Process every element → start with for loop, index 0 to n-1.
- Finding pairs → nested for loop, inner starts at i+1.
- Contiguous subarray → for loop with two pointers as sliding window.
- Tree/graph traversal → recursive DFS or iterative BFS with queue.
- Optimal across overlapping subproblems → DP with for loop bottom-up.

**Condition placement:**
- Always try to put complex conditions INSIDE the loop body, not in the loop header.
- Use if-else when exactly one of two paths must be taken.
- Use if alone when the action is optional (one path does something, the other does nothing).
- Use switch/match when you have a finite enumeration of discrete cases.
- Post-loop condition for validation and final result.

**Debugging:**
- Dry run on a small example. Build a table of variable values per iteration.
- Test edge cases: empty, single element, all same, all negative, sorted/reverse-sorted.
- Check boundary conditions: `<` vs `<=`, `i` vs `i-1`, starting index 0 vs 1.
```
