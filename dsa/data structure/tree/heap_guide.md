Yes — **a priority queue is commonly implemented on top of a heap**, and specifically a **min-heap** when the smallest-priority value should come out first.

The important distinction is:

* **Priority Queue** → an **abstract data type (ADT)**: defines what operations should do.
* **Min-Heap** → a **data structure** that can efficiently implement those operations.

### The relationship

For a min-priority queue:

```text
Priority Queue
      │
      ▼
   Min-Heap
      │
      ▼
Array / contiguous memory
```

A typical min-heap maintains:

```text
parent <= children
```

For an array:

```text
        2
       / \
      5   7
     / \ / \
    9  6 8  10
```

The minimum element is always at the root:

```text
heap[0] = minimum
```

So the priority-queue operations become:

| Priority Queue operation | Min-heap operation      | Complexity |
| ------------------------ | ----------------------- | ---------: |
| `insert(x)`              | sift/bubble up          | `O(log n)` |
| `min()` / `peek()`       | read root               |     `O(1)` |
| `extract_min()`          | remove root + sift down | `O(log n)` |
| `build()`                | heapify                 |     `O(n)` |

### The algorithm concepts you should learn

If you're studying this from a data-structures/algorithm perspective, I'd think about it in this order:

```text
Binary Tree
    ↓
Complete Binary Tree
    ↓
Array representation
    ↓
Heap invariant
    ↓
Sift Up
    ↓
Sift Down
    ↓
Insert / Extract
    ↓
Build Heap
    ↓
Priority Queue
    ↓
Heap Sort
```

One particularly important concept is **build-heap**.

Given:

```text
[7, 2, 9, 1, 5, 3]
```

you don't need to repeatedly insert every element (`O(n log n)`). You can construct the heap bottom-up using **sift-down**, which takes **O(n)** overall.

That's one of the places where understanding the **heap invariant** matters more than memorizing the implementation.

If you're asking because you're implementing a priority queue yourself, I'd recommend we derive **`sift_down()` and `build_heap()` from the invariants first**, rather than jumping straight into code.
