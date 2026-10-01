# Height-Balanced Binary Search Trees: A Complete In-Depth Guide

---

## Table of Contents

1. [Foundations: Binary Trees and BSTs](#1-foundations-binary-trees-and-bsts)
2. [The Problem with Unbalanced BSTs](#2-the-problem-with-unbalanced-bsts)
3. [What Is a Height-Balanced BST?](#3-what-is-a-height-balanced-bst)
4. [Tree Height, Depth, and Balance Factor](#4-tree-height-depth-and-balance-factor)
5. [Rotations: The Core Primitive](#5-rotations-the-core-primitive)
6. [AVL Trees](#6-avl-trees)
7. [Red-Black Trees](#7-red-black-trees)
8. [AVL vs Red-Black Trees: Deep Comparison](#8-avl-vs-red-black-trees-deep-comparison)
9. [Other Balanced BST Variants](#9-other-balanced-bst-variants)
10. [Complexity Analysis](#10-complexity-analysis)
11. [Memory Layout and Cache Behavior](#11-memory-layout-and-cache-behavior)
12. [Go Implementation: AVL Tree](#12-go-implementation-avl-tree)
13. [C Implementation: Red-Black Tree](#13-c-implementation-red-black-tree)
14. [Rust Implementation: AVL Tree](#14-rust-implementation-avl-tree)
15. [Real-World Usage and Standard Libraries](#15-real-world-usage-and-standard-libraries)
16. [Mental Models and Intuition Building](#16-mental-models-and-intuition-building)

---

## 1. Foundations: Binary Trees and BSTs

### 1.1 Binary Tree

A **binary tree** is a rooted tree where every node has at most two children, referred to as the **left child** and the **right child**.

```
        A              <- root (depth 0)
       / \
      B   C            <- depth 1
     / \   \
    D   E   F          <- depth 2
```

Formal definition of a node:

```
Node {
    key   : comparable value
    left  : pointer to Node | null
    right : pointer to Node | null
    parent: pointer to Node | null  (optional, but used in many algorithms)
}
```

### 1.2 Binary Search Tree (BST)

A BST adds an **ordering invariant** on top of a binary tree:

> For every node N:
> - All keys in N's **left** subtree are **strictly less than** N.key
> - All keys in N's **right** subtree are **strictly greater than** N.key
> - No duplicate keys (in a basic BST; duplicates can be handled with a count field)

```
          8
         / \
        3   10
       / \    \
      1   6    14
         / \   /
        4   7 13
```

This invariant means that an **in-order traversal** (left → root → right) always produces a sorted sequence:

```
In-order: 1, 3, 4, 6, 7, 8, 10, 13, 14
```

### 1.3 Core BST Operations

**Search:**
Compare the target with the current node. Go left if smaller, right if larger, stop if equal.

**Insert:**
Search for the position where the key would be, then place it there as a new leaf.

**Delete (3 cases):**
- Node is a leaf → remove directly.
- Node has one child → replace node with that child.
- Node has two children → find in-order successor (smallest key in right subtree), copy its key to current node, then delete the successor node (which has at most one child).

---

## 2. The Problem with Unbalanced BSTs

### 2.1 Worst-Case Degeneration

If you insert keys in sorted order (1, 2, 3, 4, 5, ...) into a plain BST, you get a **right-skewed tree** (effectively a linked list):

```
1
 \
  2
   \
    3
     \
      4
       \
        5
```

Inserting 10 elements in sorted order:

```
Before balancing (sorted insert: 1..10):

1
 \
  2
   \
    3
     \
      4
       \
        5
         \
          6
           \
            7
             \
              8
               \
                9
                 \
                 10

Height = 9 (n-1 for n nodes)
Search for 10: 10 comparisons (O(n))
```

Similarly, inserting in reverse order produces a left-skewed tree.

### 2.2 The Core Problem: Height Determines Performance

All BST operation costs are O(h) where h is the **height** of the tree:

| Tree shape        | Height h     | Search cost  |
|-------------------|-------------|--------------|
| Perfect binary    | log₂(n)     | O(log n)     |
| Random insertions | ~1.39 log₂n | O(log n)     |
| Sorted insertions | n - 1       | O(n)         |

For n = 1,000,000 nodes:
- Balanced: ~20 comparisons
- Degenerate: ~1,000,000 comparisons

This is a **catastrophic** difference in practice. Self-balancing trees guarantee height stays O(log n) no matter the insertion order.

---

## 3. What Is a Height-Balanced BST?

A **height-balanced BST** (also called a **self-balancing BST**) is a BST that automatically keeps its height O(log n) after every insert and delete by performing **structural rebalancing** (via rotations or color changes).

The key guarantee: for any tree with n nodes, height h satisfies:

```
h ≤ c · log₂(n)     for some small constant c
```

Different balanced BST types use different definitions of "balanced":

| Type           | Balance definition                                      | Height bound     |
|----------------|---------------------------------------------------------|------------------|
| AVL tree       | |height(left) - height(right)| ≤ 1 for every node     | ≤ 1.44 log₂(n)  |
| Red-Black tree | No path from root to leaf is more than 2× another      | ≤ 2 log₂(n+1)   |
| Weight-balanced| |size(left) - size(right)| bounded by weight ratio   | O(log n)         |
| B-tree (order m)| All leaves at same depth; nodes have m/2..m children  | O(log_m n)       |

---

## 4. Tree Height, Depth, and Balance Factor

### 4.1 Definitions

**Depth of a node**: number of edges from the root to that node. Root has depth 0.

**Height of a node**: number of edges on the longest downward path from that node to a leaf. A leaf has height 0. A null node has height -1 (by convention).

**Height of a tree**: height of the root node.

```
          8              height=3, depth=0
         / \
        3   10           height=2, depth=1    height=1, depth=1
       / \    \
      1   6    14        height=0,1,1 at depth=2
         / \   /
        4   7 13         height=0 at depth=3

Height of node 3:
  - left subtree (node 1) has height 0
  - right subtree (node 6) has height 1
  - height(3) = 1 + max(0, 1) = 2
```

Height formula:

```
height(null) = -1
height(node) = 1 + max(height(node.left), height(node.right))
```

### 4.2 Balance Factor

The **balance factor (BF)** of a node is:

```
BF(node) = height(node.left) - height(node.right)
```

Examples:

```
   5          BF = height(left=3) - height(right=7)
  / \            = 1 - 1 = 0  (perfectly balanced)
 3   7
 
   5          BF = height(left=3) - height(right=null)
  /              = 0 - (-1) = 1  (left-heavy by 1)
 3
 
   5          BF = height(left=3 subtree of depth 2) - height(right=null)
  /              = 2 - (-1) = 3  (IMBALANCED, needs rotation)
 3
  \
   4
```

In an AVL tree, every node must have BF ∈ {-1, 0, +1}. Any node with |BF| ≥ 2 is **imbalanced** and must be fixed.

---

## 5. Rotations: The Core Primitive

Rotations are the fundamental structural operations used to rebalance trees. A rotation:
- Changes the shape of the tree
- **Preserves the BST ordering invariant**
- Runs in O(1) time (just pointer manipulation)

There are two basic rotations and two composite ones.

### 5.1 Right Rotation (RotateRight)

Applied when a node is **left-heavy** (left subtree is too tall).

```
Before right rotation at node y:

        y
       / \
      x   C
     / \
    A   B

After right rotation at y:

      x
     / \
    A   y
       / \
      B   C
```

Why the BST invariant is preserved:
- A < x < B < y < C — this ordering is maintained before and after.

Step-by-step pointer changes:
```
1. x = y.left
2. y.left = x.right    (B moves from x's right to y's left)
3. x.right = y         (y becomes x's right child)
4. update parent pointers
5. update heights of y first, then x (bottom-up)
```

### 5.2 Left Rotation (RotateLeft)

Applied when a node is **right-heavy** (right subtree is too tall). Mirror image of right rotation.

```
Before left rotation at node x:

    x
   / \
  A   y
     / \
    B   C

After left rotation at x:

        y
       / \
      x   C
     / \
    A   B
```

### 5.3 Left-Right Rotation (Double Rotation)

Also called **LR rotation**. Used when left child is right-heavy.

```
Initial state (z is unbalanced, BF = +2, left child y has BF = -1):

        z
       / \
      y   D
     / \
    A   x
       / \
      B   C

Step 1: Left rotate y

        z
       / \
      x   D
     / \
    y   C
   / \
  A   B

Step 2: Right rotate z

        x
       / \
      y   z
     / \ / \
    A  B C  D
```

### 5.4 Right-Left Rotation (Double Rotation)

Also called **RL rotation**. Used when right child is left-heavy. Mirror of LR rotation.

```
Initial state (z is unbalanced, BF = -2, right child y has BF = +1):

    z
   / \
  A   y
     / \
    x   D
   / \
  B   C

Step 1: Right rotate y

    z
   / \
  A   x
     / \
    B   y
       / \
      C   D

Step 2: Left rotate z

        x
       / \
      z   y
     / \ / \
    A  B C  D
```

### 5.5 Which Rotation to Apply?

```
Imbalanced node N has BF = +2 (left-heavy):
  - If N.left has BF >= 0: simple right rotation at N
  - If N.left has BF < 0:  left-right double rotation

Imbalanced node N has BF = -2 (right-heavy):
  - If N.right has BF <= 0: simple left rotation at N
  - If N.right has BF > 0:  right-left double rotation
```

ASCII summary:

```
Case 1: Left-Left (BF=+2, left child BF>=0) → Right Rotate
      z (+2)          y (0)
     /        →      / \
    y (+1)          x   z
   /
  x

Case 2: Right-Right (BF=-2, right child BF<=0) → Left Rotate
  z (-2)               y (0)
    \       →         / \
     y (-1)          z   x
       \
        x

Case 3: Left-Right (BF=+2, left child BF=-1) → Left Rotate left child, then Right Rotate
      z (+2)          z (+2)           x (0)
     /        →      /        →       / \
    y (-1)          x (+1)           y   z
      \            /
       x          y

Case 4: Right-Left (BF=-2, right child BF=+1) → Right Rotate right child, then Left Rotate
  z (-2)         z (-2)              x (0)
    \     →        \       →        / \
     y (+1)         x (-1)         z   y
    /                 \
   x                   y
```

---

## 6. AVL Trees

Named after Georgy **A**delson-**V**elsky and Evgenii **L**andis (1962), AVL trees were the first self-balancing BST data structure invented.

### 6.1 AVL Invariant

Every node stores its **height** (or equivalently, its balance factor). After every insert/delete:

1. Update heights of all affected nodes (bottom-up from the changed node to the root).
2. For each node, check if |BF| > 1.
3. If imbalanced, apply the appropriate rotation.
4. Continue up to the root.

### 6.2 Node Structure

```
AVLNode {
    key    : int
    height : int          // height of subtree rooted here
    left   : *AVLNode
    right  : *AVLNode
    // parent pointer optional; some implementations use recursive stack
}
```

Storing height (not BF) is preferred because it lets you recompute BF in O(1) at any time: `BF = height(left) - height(right)`.

### 6.3 AVL Insertion

```
Pseudocode:

insert(node, key):
    // Standard BST insert
    if node == null:
        return newNode(key)
    if key < node.key:
        node.left = insert(node.left, key)
    else if key > node.key:
        node.right = insert(node.right, key)
    else:
        return node  // duplicate, ignore

    // Update height
    node.height = 1 + max(height(node.left), height(node.right))

    // Get balance factor
    bf = balanceFactor(node)

    // Left-Left case
    if bf > 1 and key < node.left.key:
        return rotateRight(node)

    // Right-Right case
    if bf < -1 and key > node.right.key:
        return rotateLeft(node)

    // Left-Right case
    if bf > 1 and key > node.left.key:
        node.left = rotateLeft(node.left)
        return rotateRight(node)

    // Right-Left case
    if bf < -1 and key < node.right.key:
        node.right = rotateRight(node.right)
        return rotateLeft(node)

    return node
```

### 6.4 AVL Insertion Example (Step-by-Step)

Insert keys: 10, 20, 30, 40, 50, 25

```
Step 1: Insert 10

   10(h=0)

Step 2: Insert 20

   10(h=1)
     \
     20(h=0)
   BF(10) = 0 - (-1) = ... wait, let's use height properly.
   height(10) = 1 + max(-1, 0) = 1
   BF(10) = height(left=null=-1) - height(right=20=0) = -1-0 = -1. OK.

Step 3: Insert 30

   10(h=2)
     \
     20(h=1)
       \
       30(h=0)
   
   BF(20) = -1 - 0 = -1. OK.
   BF(10) = -1 - 1 = -2. IMBALANCED! Right-Right case.
   
   Apply left rotation at 10:

     20(h=1)
    /  \
  10    30
  (h=0) (h=0)
   BF(20) = 0. Balanced.

Step 4: Insert 40

       20(h=2)
      /  \
    10    30(h=1)
            \
            40(h=0)
   BF(20) = 0-1 = -1. OK.

Step 5: Insert 50

       20(h=3)
      /  \
    10    30(h=2)
            \
            40(h=1)
              \
              50(h=0)
   
   BF(30) = -1-0 = -1. OK.
   BF(20) = 0-2 = -2. IMBALANCED! Right-Right case.
   
   Apply left rotation at 20:

        30(h=2)
       /  \
     20    40(h=1)
    /  \     \
  10   (null) 50
  BF(30) = 1-1 = 0. Balanced.
  
   Wait, let's be precise:

        30
       /  \
      20   40
     /       \
    10        50
   
   h(10)=0, h(20)=1, h(50)=0, h(40)=1, h(30)=2
   BF(30) = 1-1 = 0. Balanced.

Step 6: Insert 25

        30
       /  \
      20   40
     /  \    \
    10   25   50
   
   h(25)=0, h(10)=0, h(20)=1, h(50)=0, h(40)=1, h(30)=2
   BF(20) = 0-0 = 0. OK.
   BF(30) = 1-1 = 0. OK.
   
   Final tree:
   
        30
       /  \
      20   40
     /  \    \
    10   25   50
```

### 6.5 AVL Deletion

Deletion is more complex than insertion because rebalancing can propagate all the way to the root (whereas insertion only requires at most one rotation).

```
Pseudocode:

delete(node, key):
    if node == null:
        return null
    
    if key < node.key:
        node.left = delete(node.left, key)
    else if key > node.key:
        node.right = delete(node.right, key)
    else:
        // Found the node to delete
        if node.left == null:
            return node.right
        else if node.right == null:
            return node.left
        else:
            // Two children: replace with in-order successor
            successor = minNode(node.right)
            node.key = successor.key
            node.right = delete(node.right, successor.key)
    
    // Update height
    node.height = 1 + max(height(node.left), height(node.right))
    
    // Rebalance (same 4 cases as insertion)
    bf = balanceFactor(node)
    
    // Left-Left
    if bf > 1 and balanceFactor(node.left) >= 0:
        return rotateRight(node)
    // Left-Right
    if bf > 1 and balanceFactor(node.left) < 0:
        node.left = rotateLeft(node.left)
        return rotateRight(node)
    // Right-Right
    if bf < -1 and balanceFactor(node.right) <= 0:
        return rotateLeft(node)
    // Right-Left
    if bf < -1 and balanceFactor(node.right) > 0:
        node.right = rotateRight(node.right)
        return rotateLeft(node)
    
    return node
```

### 6.6 AVL Height Bound Proof (Fibonacci Relationship)

Let N(h) = minimum number of nodes in an AVL tree of height h.

```
N(-1) = 0   (empty tree)
N(0)  = 1   (single node)
N(h)  = 1 + N(h-1) + N(h-2)   for h >= 1
```

This is similar to the Fibonacci recurrence. If F(n) is the nth Fibonacci number:

```
N(h) = F(h+3) - 1
```

Since F(n) ≈ φⁿ/√5 where φ = (1+√5)/2 ≈ 1.618:

```
n ≥ N(h) ≈ φ^(h+2)/√5
h ≤ log_φ(n√5) - 2
h ≤ log_φ(n) + constant
h ≤ 1.44 log₂(n)
```

So AVL trees guarantee height ≤ 1.44 log₂(n). The worst-case AVL tree (minimum-node trees of each height) looks like:

```
h=0:  •        (1 node)
h=1:  •        (2 nodes)
      / \
     •   (null)... wait, h=1 min is:
     
      •
     /
    •

h=2:  •        (4 nodes: 1 + N(1) + N(0) = 1+2+1 = 4)
     / \
    •   •
   /
  •

h=3:  •         (7 nodes: 1 + N(2) + N(1) = 1+4+2 = 7)
     / \
    •   •
   / \   \
  •   •   •
 /
•
```

---

## 7. Red-Black Trees

Red-Black (RB) trees were invented by Rudolf Bayer (1972) as "symmetric binary B-trees," then popularized by Sedgewick and others. They are the most commonly used balanced BST in practice.

### 7.1 Red-Black Properties

Every node is colored either **RED** or **BLACK**. A Red-Black tree satisfies:

1. **Color property**: Every node is red or black.
2. **Root property**: The root is black.
3. **Leaf property**: All null leaves (NIL sentinel nodes) are black.
4. **Red property** (no double-red): If a node is red, both its children are black. (No two consecutive red nodes on any path.)
5. **Black-height property**: For any node, all paths from that node to descendant null leaves contain the **same number of black nodes**. This common count is called the **black-height** of the node.

```
Red-Black tree example (R=red, B=black):

              7(B)
            /      \
          3(R)      18(R)
         /   \      /   \
       2(B)  4(B) 11(B) 19(B)
      /    \         \
   1(R)  NIL(B)    14(R)
```

Black-height of this tree = 2 (counting black nodes on path from root to any NIL, not counting root itself in some definitions).

### 7.2 Why Red-Black Trees Work

The black-height property enforces that all root-to-leaf paths have the same number of black nodes. The red property prevents two consecutive red nodes. Therefore:

- Shortest root-to-leaf path: all black nodes → length = black-height (bh)
- Longest root-to-leaf path: alternating red and black → length = 2 × bh

So the tree height h ≤ 2 × bh. Since every subtree of 2^bh - 1 nodes fits in bh levels of all-black nodes:

```
n ≥ 2^bh - 1
bh ≤ log₂(n+1)
h ≤ 2 × log₂(n+1) = O(log n)
```

### 7.3 Node Structure

```
RBNode {
    key    : int
    color  : RED | BLACK
    left   : *RBNode
    right  : *RBNode
    parent : *RBNode    // parent pointer is REQUIRED in RB trees
}

// A special NIL sentinel node is used instead of null:
NIL = RBNode { color: BLACK, left: NIL, right: NIL, parent: NIL }
```

Using a sentinel NIL node (instead of null) simplifies the code significantly: you never have to null-check before accessing a node's color.

### 7.4 Red-Black Insertion

Insertion always adds a **RED** node. This potentially violates the red property (double-red) but never the black-height property. Fixing double-red is then done via **recoloring** and **rotations**.

```
insert(tree, key):
    z = newNode(key, RED)
    // Standard BST insert to find position
    y = NIL
    x = tree.root
    while x != NIL:
        y = x
        if z.key < x.key: x = x.left
        else:              x = x.right
    z.parent = y
    if y == NIL:
        tree.root = z
    elif z.key < y.key:
        y.left = z
    else:
        y.right = z
    z.left = NIL
    z.right = NIL
    
    insertFixup(tree, z)
```

**Insert fixup — 3 cases** (and their mirrors for left/right symmetry):

Assume z is red and z.parent is red (double-red violation). Let uncle = z's parent's sibling.

```
Case 1: Uncle is RED
   
   Recolor: parent → BLACK, uncle → BLACK, grandparent → RED
   Move z up to grandparent and continue.
   
        G(B)            G(R)   ← might violate if G's parent is also red
       / \      →      / \
     P(R)  U(R)      P(B)  U(B)
    /                /
   Z(R)             Z(R)

Case 2: Uncle is BLACK, z is a right child of P which is a left child of G
   
   Left rotate P, making Z the new "P":
   
        G(B)            G(B)
       / \      →      / \
     P(R)  U(B)      Z(R)  U(B)
       \              /
       Z(R)          P(R)
   
   Now we have Case 3.

Case 3: Uncle is BLACK, z is a left child of P which is a left child of G
   
   Recolor P → BLACK, G → RED. Right rotate G.
   
        G(B)            P(B)
       / \      →      / \
     P(R)  U(B)      Z(R)  G(R)
    /                         \
   Z(R)                       U(B)
```

Cases 4, 5, 6 are mirrors of 1, 2, 3 for the right subtree.

### 7.5 Red-Black Deletion

Deletion is the most complex operation. It removes a node and then fixes any violations using a fixup procedure. The key insight:

- If we delete a RED node, no black-height violation occurs.
- If we delete a BLACK node (or replace a node with a BLACK one), we get a black-height deficit that must be fixed.

The fixup introduces the concept of a **"double-black"** node — a conceptual placeholder meaning "this position is owed one extra black node."

**Delete fixup — 4 cases** (for when the "extra black" is on the left; mirror for right):

Let x = double-black node, w = x's sibling.

```
Case 1: w is RED
   Recolor w → BLACK, x.parent → RED. Left rotate x.parent.
   This transforms to Case 2, 3, or 4.

Case 2: w is BLACK, w's both children are BLACK
   Recolor w → RED. Move double-black up to x.parent.
   If x.parent was RED, color it BLACK and done.
   If x.parent was BLACK, it becomes double-black, recurse.

Case 3: w is BLACK, w's left child is RED, w's right child is BLACK
   Recolor w → RED, w.left → BLACK. Right rotate w.
   Now transforms to Case 4.

Case 4: w is BLACK, w's right child is RED
   Set w's color = x.parent's color.
   Set x.parent → BLACK. Set w.right → BLACK.
   Left rotate x.parent.
   Done. Double-black resolved.
```

### 7.6 Visualizing a Red-Black Tree Insertion Sequence

Insert: 1, 2, 3, 4, 5, 6, 7

```
Insert 1:
  1(B)  ← root is always black

Insert 2:
  1(B)
    \
    2(R)

Insert 3:
  Before fix: 2(R) is right child of 1(B), 1's right child 2(R) has right child 3(R) → double red
  
  Right-Right case (uncle=NIL=BLACK, Case 3):
  Recolor: 1→RED, 2→BLACK. Left rotate 1.
  
      2(B)
     / \
   1(R) 3(R)

Insert 4:
      2(B)
     / \
   1(R) 3(R)
           \
           4(R)   ← double red: 3(R) → 4(R), uncle=1(R)
  
  Case 1 (uncle 1 is RED): recolor 1→B, 3→B, 2→R→but 2 is root so 2→B
  
      2(B)
     / \
   1(B) 3(B)
           \
           4(R)

Insert 5:
      2(B)
     / \
   1(B) 3(B)
           \
           4(R)
              \
              5(R)  ← double red: 4(R)→5(R), uncle=NIL=BLACK
  
  Right-Right (Case 3 mirror): recolor 3→R, 4→B. Left rotate 3.
  Then fix up... let me trace carefully.
  
  z=5(R), p=4(R), g=3(B), uncle=NIL(B). Case 3 (uncle BLACK, z and p are right children).
  Color p(4) = BLACK, g(3) = RED. Left rotate g(3).
  
      2(B)
     / \
   1(B) 4(B)
        / \
      3(R) 5(R)

Insert 6:
      2(B)
     / \
   1(B) 4(B)
        / \
      3(R) 5(R)
              \
              6(R)  ← double red: 5(R)→6(R), uncle=3(R)
  
  Case 1 (uncle 3 is RED): recolor 5→B, 3→B, 4→R. Now 4(R), parent=2(B). OK.
  
      2(B)
     / \
   1(B) 4(R)
        / \
      3(B) 5(B)
              \
              6(R)

Insert 7:
      2(B)
     / \
   1(B) 4(R)
        / \
      3(B) 5(B)
              \
              6(R)
                 \
                 7(R)  ← double red: 6(R)→7(R), uncle=NIL(B)
  
  Case 3 (uncle BLACK, RR): color 5(B)→R... wait.
  z=7, p=6(R), g=5(B), uncle=NIL(B). Right-Right. Color p→BLACK(6→B), g→RED(5→R). Left rotate g(5).
  
      2(B)
     / \
   1(B) 4(R)
        / \
      3(B) 6(B)
           / \
         5(R) 7(R)
  
  Now 4(R) is child of 2(B), so no double red here. But we need to check: is 4 the root? No. Is 2(B)'s parent null? Yes, 2 is root. No double-red between 4(R) and 2(B). OK.
  
  Final tree:
  
        2(B)
       /    \
     1(B)   4(R)
            / \
          3(B) 6(B)
               / \
             5(R) 7(R)
```

---

## 8. AVL vs Red-Black Trees: Deep Comparison

### 8.1 Structural Difference

AVL is **more strictly balanced**. The height of an AVL tree is bounded by 1.44 log₂n. Red-Black allows up to 2 log₂n. This means:

- AVL trees are "shorter" and have faster lookup.
- Red-Black trees allow slightly more imbalance in exchange for fewer restructuring operations on insert/delete.

### 8.2 Rotation Counts

| Operation | AVL (worst case)    | Red-Black (worst case) |
|-----------|---------------------|------------------------|
| Insert    | 1 rotation          | 2 rotations            |
| Delete    | O(log n) rotations  | 3 rotations            |

For **insertion**, AVL does at most 1 rotation and then stops propagating up. Red-Black does at most 2 rotations but may need O(log n) recolorings that travel up.

For **deletion**, AVL may need O(log n) rotations propagating all the way up. Red-Black needs at most 3 rotations (but recolorings may travel up).

### 8.3 Write-Heavy vs Read-Heavy

```
Scenario: database index with frequent reads, rare writes
→ Prefer AVL (tighter height = faster search)

Scenario: in-memory map with frequent inserts and deletes
→ Prefer Red-Black (fewer rotations on average = faster writes)
```

### 8.4 Implementation Complexity

AVL is conceptually simpler (4 rotation cases, easy to understand) but requires storing heights. Red-Black is more complex (6 fixup cases × 2 symmetries = 12 sub-cases for deletion) but only requires 1 bit of color per node.

### 8.5 Memory

```
AVL node: key(8B) + height(4B) + left(8B) + right(8B) + [parent(8B)] = 28-36 bytes
RB node:  key(8B) + color(1B, often padded to 4-8B) + left(8B) + right(8B) + parent(8B) = 33-40 bytes
```

Both are similar in practice due to alignment padding.

### 8.6 Summary Table

| Aspect               | AVL                    | Red-Black              |
|----------------------|------------------------|------------------------|
| Height bound         | 1.44 log₂n             | 2 log₂(n+1)            |
| Search performance   | Slightly faster        | Slightly slower        |
| Insert rotations     | ≤ 1                    | ≤ 2                    |
| Delete rotations     | O(log n)               | ≤ 3                    |
| Recolorings (insert) | N/A                    | O(log n)               |
| Implementation       | Simpler                | Complex                |
| Used in              | Linux VFS, some DBs    | Linux CFS, C++ std::map|
| Best for             | Read-heavy workloads   | Write-heavy workloads  |

---

## 9. Other Balanced BST Variants

### 9.1 B-Trees

B-trees are balanced search trees where each node can have **multiple keys and multiple children** (not just 2). A B-tree of order m means each node has between ⌈m/2⌉ and m children (except root).

```
B-tree of order 3 (2-3 tree):

            [10 | 20]
           /    |    \
        [5|7]  [12|15]  [25|30]
```

B-trees are optimized for **disk-based storage** where reading a large block (page) at once is cheap, but the number of disk seeks matters. By storing many keys per node, B-trees minimize tree height and thus disk accesses. Used in databases (MySQL InnoDB, PostgreSQL) and file systems (ext4, NTFS, HFS+).

### 9.2 B+ Trees

A variant of B-trees where:
- Internal nodes only store keys (for routing), not values.
- All values are stored in the **leaf nodes**.
- Leaf nodes are **linked in a doubly linked list** for efficient range queries.

```
B+ Tree (order 3):

Internal:      [10 | 20]
              /    |    \
Leaves: [5,7,8]→[10,12,15]→[20,25,30]
```

This is the dominant structure in relational database indexes.

### 9.3 Splay Trees

A self-adjusting BST that **splays** (moves to the root) any accessed node via a sequence of rotations. No explicit balance information stored.

Key insight: recently accessed keys are near the root → good for workloads with **temporal locality**.

Splay tree operations: zig (single rotation), zig-zig, zig-zag.

```
Accessing node X in a splay tree:
1. Standard BST search to find X.
2. Splay X to the root using rotation sequences.
3. X is now at the root.
```

Amortized O(log n) for all operations, but worst-case O(n) per operation.

### 9.4 Treaps

A randomized BST where each node has:
- A **key** (BST ordering on keys)
- A **priority** (randomly assigned; max-heap ordering on priorities)

Result: the tree structure is a random BST, which has expected height O(log n) with high probability.

```
Treap node:
  key=5, priority=91   (root: highest priority)
       /              \
  key=3, pri=72     key=8, pri=55
     /                 \
 key=1, pri=30       key=10, pri=42
```

No rotations needed to maintain BST order (keys handle that). Only priority-triggered rotations needed to restore heap order after insert/delete.

### 9.5 Weight-Balanced Trees

Balance criterion is based on **subtree size** rather than height:

```
For every node, |size(left) - size(right)| ≤ α × size(node)
for some constant α ∈ (0, 1)
```

Weight-balanced trees support order-statistics efficiently (rank, select) because subtree sizes are already stored.

### 9.6 Skip Lists (Not a Tree, but Often a BST Alternative)

Skip lists are a probabilistic data structure using multiple layers of linked lists:

```
Level 3: head ────────────────────────────────────── 50 ─── tail
Level 2: head ──────────── 20 ─────────────── 50 ─── tail
Level 1: head ─── 10 ─── 20 ─── 30 ─── 40 ─── 50 ─── tail
```

O(log n) expected time for search/insert/delete. Used in Redis (sorted sets) and LevelDB/RocksDB (MemTable).

---

## 10. Complexity Analysis

### 10.1 Time Complexity

| Operation | BST (avg) | BST (worst) | AVL (worst) | RB (worst) |
|-----------|-----------|-------------|-------------|------------|
| Search    | O(log n)  | O(n)        | O(log n)    | O(log n)   |
| Insert    | O(log n)  | O(n)        | O(log n)    | O(log n)   |
| Delete    | O(log n)  | O(n)        | O(log n)    | O(log n)   |
| Min/Max   | O(log n)  | O(n)        | O(log n)    | O(log n)   |
| Successor | O(log n)  | O(n)        | O(log n)    | O(log n)   |

### 10.2 Space Complexity

| Structure | Space per node | Overhead  |
|-----------|---------------|-----------|
| Plain BST | O(1)          | key + 2 ptrs |
| AVL       | O(1)          | + height (4 bytes) |
| Red-Black | O(1)          | + color (1 bit, usually 1 byte) |
| B-tree(m) | O(m)          | m keys + m+1 ptrs per node |

Total space: O(n) for all BST variants.

### 10.3 Rotation Counts (Important for Practical Performance)

```
AVL Insert: At most 1 rotation (single or double)
  - After one rotation, the subtree height is restored. No need to continue up.
  
AVL Delete: At most O(log n) rotations
  - Each rotation may decrease height of a subtree, requiring more rotations above.

RB Insert: At most 2 rotations + O(log n) recolorings
  - Recolorings are cheap (just bit flips), but they propagate up.
  - At most 2 structural rotations regardless of tree size.

RB Delete: At most 3 rotations + O(log n) recolorings
  - Again, structural changes are bounded by a small constant.
```

### 10.4 Amortized Analysis

For a sequence of n operations on a Red-Black tree, the **total** number of rotations across all operations is O(n) — amortized O(1) rotation per operation. This makes Red-Black trees excellent for scenarios where you're doing many operations.

For AVL trees, the amortized rotation count is also O(1) per insert, but O(log n) per delete in the worst case (though in practice, amortized over many operations, it's closer to O(1) per delete too).

---

## 11. Memory Layout and Cache Behavior

### 11.1 Node Size and Cache Lines

A typical x86-64 cache line is 64 bytes. An AVL or RB tree node with key(8) + pointers(3×8=24) + height or color(4) ≈ 36 bytes. This means ~1.7 nodes fit per cache line.

```
Cache line (64 bytes):
[  node A (36 bytes)  | node B (partial, 28 bytes) | padding ]
```

For a tree with 1 million nodes, a random access pattern (typical for BST traversal) has a very low cache hit rate. Every pointer dereference to a child node is likely a **cache miss**.

### 11.2 Cache-Optimized BST Layouts

**Van Emde Boas (VEB) layout**: Store nodes in memory following a recursive BFS-like layout where subtrees are placed contiguously. This gives cache-oblivious performance.

```
BFS (level-order) layout:

Tree:       1
           / \
          2   3
         / \ / \
        4  5 6  7

Memory: [1, 2, 3, 4, 5, 6, 7]  ← accessing root and children often hits same cache line
```

**VEB layout** for a tree of 7 nodes:

```
Top half:  1
          / \
         2   3

Bottom quarters (left):    Bottom quarters (right):
    4  5                       6  7

Memory: [1, 2, 3, 4, 5, 6, 7] same as BFS for this small tree,
but VEB recursion gives better spatial locality for large trees.
```

### 11.3 B-Trees and Cache Efficiency

B-trees are specifically designed to match the disk or cache page size. A B-tree of order m with large m (e.g., m=64 or m=128) can store many keys per node, fitting within a single cache line or disk block:

```
B-tree page (4KB disk block, ~40-byte records): can hold ~100 keys per node
Tree height for 1M records: log_100(1,000,000) ≈ 3 levels
```

This is why B-trees dominate database index structures.

---

## 12. Go Implementation: AVL Tree

```go
package avl

// AVL tree implementation in Go
// Supports generic keys via interface{} for simplicity;
// production code should use generics (Go 1.18+).

// ============================================================
// Node and Tree structures
// ============================================================

type Node struct {
    Key    int
    Val    interface{}
    height int
    left   *Node
    right  *Node
}

type AVL struct {
    root *Node
    size int
}

// ============================================================
// Height helpers
// ============================================================

func height(n *Node) int {
    if n == nil {
        return -1
    }
    return n.height
}

func max(a, b int) int {
    if a > b {
        return a
    }
    return b
}

func updateHeight(n *Node) {
    n.height = 1 + max(height(n.left), height(n.right))
}

func balanceFactor(n *Node) int {
    if n == nil {
        return 0
    }
    return height(n.left) - height(n.right)
}

// ============================================================
// Rotations
// ============================================================

// rotateRight performs a right rotation around y:
//
//       y                x
//      / \              / \
//     x   C    -->     A   y
//    / \                  / \
//   A   B                B   C
//
func rotateRight(y *Node) *Node {
    x := y.left
    B := x.right

    // Perform rotation
    x.right = y
    y.left = B

    // Update heights (y first, it's now lower in the tree)
    updateHeight(y)
    updateHeight(x)

    return x // x is now the root of this subtree
}

// rotateLeft performs a left rotation around x:
//
//     x                  y
//    / \                / \
//   A   y    -->       x   C
//      / \            / \
//     B   C          A   B
//
func rotateLeft(x *Node) *Node {
    y := x.right
    B := y.left

    // Perform rotation
    y.left = x
    x.right = B

    // Update heights
    updateHeight(x)
    updateHeight(y)

    return y
}

// ============================================================
// Rebalancing
// ============================================================

// rebalance checks if the subtree rooted at n is unbalanced and
// applies the appropriate rotation to fix it. Returns the new root.
func rebalance(n *Node) *Node {
    updateHeight(n)
    bf := balanceFactor(n)

    // Left-heavy
    if bf > 1 {
        if balanceFactor(n.left) < 0 {
            // Left-Right case: rotate left child left first
            n.left = rotateLeft(n.left)
        }
        // Left-Left case
        return rotateRight(n)
    }

    // Right-heavy
    if bf < -1 {
        if balanceFactor(n.right) > 0 {
            // Right-Left case: rotate right child right first
            n.right = rotateRight(n.right)
        }
        // Right-Right case
        return rotateLeft(n)
    }

    return n // already balanced
}

// ============================================================
// Insert
// ============================================================

func insertNode(n *Node, key int, val interface{}) *Node {
    if n == nil {
        return &Node{Key: key, Val: val}
    }
    if key < n.Key {
        n.left = insertNode(n.left, key, val)
    } else if key > n.Key {
        n.right = insertNode(n.right, key, val)
    } else {
        // Key already exists: update value
        n.Val = val
        return n
    }
    return rebalance(n)
}

func (t *AVL) Insert(key int, val interface{}) {
    oldSize := t.size
    t.root = insertNode(t.root, key, val)
    // Rough size tracking (doesn't handle duplicates perfectly here)
    if t.root != nil && oldSize == t.size {
        t.size++
    }
}

// ============================================================
// Search
// ============================================================

func searchNode(n *Node, key int) *Node {
    if n == nil {
        return nil
    }
    if key == n.Key {
        return n
    }
    if key < n.Key {
        return searchNode(n.left, key)
    }
    return searchNode(n.right, key)
}

// Search returns the value associated with key, or nil if not found.
func (t *AVL) Search(key int) (interface{}, bool) {
    node := searchNode(t.root, key)
    if node == nil {
        return nil, false
    }
    return node.Val, true
}

// ============================================================
// Min / Max
// ============================================================

func minNode(n *Node) *Node {
    for n.left != nil {
        n = n.left
    }
    return n
}

func maxNode(n *Node) *Node {
    for n.right != nil {
        n = n.right
    }
    return n
}

func (t *AVL) Min() (int, bool) {
    if t.root == nil {
        return 0, false
    }
    return minNode(t.root).Key, true
}

func (t *AVL) Max() (int, bool) {
    if t.root == nil {
        return 0, false
    }
    return maxNode(t.root).Key, true
}

// ============================================================
// Delete
// ============================================================

func deleteNode(n *Node, key int) *Node {
    if n == nil {
        return nil
    }
    if key < n.Key {
        n.left = deleteNode(n.left, key)
    } else if key > n.Key {
        n.right = deleteNode(n.right, key)
    } else {
        // Found the node to delete
        if n.left == nil {
            return n.right
        }
        if n.right == nil {
            return n.left
        }
        // Two children: replace with in-order successor
        succ := minNode(n.right)
        n.Key = succ.Key
        n.Val = succ.Val
        n.right = deleteNode(n.right, succ.Key)
    }
    return rebalance(n)
}

func (t *AVL) Delete(key int) {
    t.root = deleteNode(t.root, key)
    t.size--
}

// ============================================================
// In-order traversal (returns sorted keys)
// ============================================================

func inOrder(n *Node, result *[]int) {
    if n == nil {
        return
    }
    inOrder(n.left, result)
    *result = append(*result, n.Key)
    inOrder(n.right, result)
}

func (t *AVL) InOrder() []int {
    result := make([]int, 0, t.size)
    inOrder(t.root, &result)
    return result
}

// ============================================================
// Size and Height
// ============================================================

func (t *AVL) Size() int { return t.size }

func (t *AVL) Height() int { return height(t.root) }

// ============================================================
// Verify AVL invariant (for testing)
// ============================================================

func verify(n *Node) bool {
    if n == nil {
        return true
    }
    bf := balanceFactor(n)
    if bf > 1 || bf < -1 {
        return false
    }
    expectedH := 1 + max(height(n.left), height(n.right))
    if n.height != expectedH {
        return false
    }
    return verify(n.left) && verify(n.right)
}

func (t *AVL) Verify() bool { return verify(t.root) }

// ============================================================
// Example usage (main package can call these):
//
// tree := &AVL{}
// tree.Insert(10, "ten")
// tree.Insert(5, "five")
// tree.Insert(20, "twenty")
// tree.Insert(3, "three")
// tree.Insert(7, "seven")
//
// val, ok := tree.Search(5)    // "five", true
// fmt.Println(tree.InOrder())  // [3 5 7 10 20]
// fmt.Println(tree.Height())   // 2
// tree.Delete(5)
// fmt.Println(tree.InOrder())  // [3 7 10 20]
// ============================================================
```

---

## 13. C Implementation: Red-Black Tree

```c
/*
 * Red-Black Tree Implementation in C
 *
 * Uses a sentinel NIL node for cleaner null handling.
 * Supports integer keys with void* values.
 */

#include <stdio.h>
#include <stdlib.h>
#include <assert.h>

/* ============================================================
 * Types and constants
 * ============================================================ */

typedef enum { BLACK = 0, RED = 1 } Color;

typedef struct RBNode {
    int            key;
    void          *val;
    Color          color;
    struct RBNode *left;
    struct RBNode *right;
    struct RBNode *parent;
} RBNode;

typedef struct {
    RBNode *root;
    RBNode *nil;   /* sentinel nil node */
    int     size;
} RBTree;

/* ============================================================
 * Initialization
 * ============================================================ */

RBTree *rb_create(void) {
    RBTree *t = malloc(sizeof(RBTree));

    /* The sentinel nil: black, points to itself */
    t->nil        = malloc(sizeof(RBNode));
    t->nil->color  = BLACK;
    t->nil->left   = t->nil;
    t->nil->right  = t->nil;
    t->nil->parent = t->nil;
    t->nil->key    = 0;
    t->nil->val    = NULL;

    t->root = t->nil;
    t->size = 0;
    return t;
}

static RBNode *new_node(RBTree *t, int key, void *val) {
    RBNode *n  = malloc(sizeof(RBNode));
    n->key     = key;
    n->val     = val;
    n->color   = RED;
    n->left    = t->nil;
    n->right   = t->nil;
    n->parent  = t->nil;
    return n;
}

/* ============================================================
 * Rotations
 * ============================================================ */

/*
 * Left rotation around node x:
 *
 *    x                y
 *   / \              / \
 *  A   y    -->     x   C
 *     / \          / \
 *    B   C        A   B
 */
static void rotate_left(RBTree *t, RBNode *x) {
    RBNode *y = x->right;

    x->right = y->left;           /* turn y's left subtree into x's right subtree */
    if (y->left != t->nil)
        y->left->parent = x;

    y->parent = x->parent;        /* link x's parent to y */
    if (x->parent == t->nil)
        t->root = y;
    else if (x == x->parent->left)
        x->parent->left = y;
    else
        x->parent->right = y;

    y->left  = x;                 /* put x on y's left */
    x->parent = y;
}

/*
 * Right rotation around node y:
 *
 *       y             x
 *      / \           / \
 *     x   C  -->    A   y
 *    / \               / \
 *   A   B             B   C
 */
static void rotate_right(RBTree *t, RBNode *y) {
    RBNode *x = y->left;

    y->left = x->right;
    if (x->right != t->nil)
        x->right->parent = y;

    x->parent = y->parent;
    if (y->parent == t->nil)
        t->root = x;
    else if (y == y->parent->right)
        y->parent->right = x;
    else
        y->parent->left = x;

    x->right  = y;
    y->parent = x;
}

/* ============================================================
 * Insert fixup
 * ============================================================ */

/*
 * Fix red-black properties after insertion.
 * z is the newly inserted (red) node.
 *
 * Invariant: z is red. z->parent may also be red (violation).
 */
static void insert_fixup(RBTree *t, RBNode *z) {
    while (z->parent->color == RED) {
        if (z->parent == z->parent->parent->left) {
            /* Parent is a LEFT child */
            RBNode *uncle = z->parent->parent->right;

            if (uncle->color == RED) {
                /* Case 1: uncle is red — recolor and move up */
                z->parent->color          = BLACK;
                uncle->color              = BLACK;
                z->parent->parent->color  = RED;
                z = z->parent->parent;
            } else {
                if (z == z->parent->right) {
                    /* Case 2: z is a right child — rotate to Case 3 */
                    z = z->parent;
                    rotate_left(t, z);
                }
                /* Case 3: z is a left child */
                z->parent->color         = BLACK;
                z->parent->parent->color = RED;
                rotate_right(t, z->parent->parent);
            }
        } else {
            /* Parent is a RIGHT child (mirror of above) */
            RBNode *uncle = z->parent->parent->left;

            if (uncle->color == RED) {
                /* Case 1 mirror */
                z->parent->color          = BLACK;
                uncle->color              = BLACK;
                z->parent->parent->color  = RED;
                z = z->parent->parent;
            } else {
                if (z == z->parent->left) {
                    /* Case 2 mirror */
                    z = z->parent;
                    rotate_right(t, z);
                }
                /* Case 3 mirror */
                z->parent->color         = BLACK;
                z->parent->parent->color = RED;
                rotate_left(t, z->parent->parent);
            }
        }
    }
    t->root->color = BLACK; /* Root must always be black */
}

/* ============================================================
 * Insert
 * ============================================================ */

void rb_insert(RBTree *t, int key, void *val) {
    RBNode *z = new_node(t, key, val);
    RBNode *y = t->nil;
    RBNode *x = t->root;

    /* Standard BST insertion to find position */
    while (x != t->nil) {
        y = x;
        if (z->key < x->key)
            x = x->left;
        else if (z->key > x->key)
            x = x->right;
        else {
            /* Key exists: update value, free new node */
            x->val = val;
            free(z);
            return;
        }
    }

    z->parent = y;
    if (y == t->nil)
        t->root = z;
    else if (z->key < y->key)
        y->left = z;
    else
        y->right = z;

    t->size++;
    insert_fixup(t, z);
}

/* ============================================================
 * Search
 * ============================================================ */

static RBNode *search_node(RBTree *t, int key) {
    RBNode *x = t->root;
    while (x != t->nil) {
        if (key == x->key)
            return x;
        else if (key < x->key)
            x = x->left;
        else
            x = x->right;
    }
    return NULL;
}

void *rb_search(RBTree *t, int key) {
    RBNode *n = search_node(t, key);
    return n ? n->val : NULL;
}

/* ============================================================
 * Transplant helper for deletion
 *
 * Replaces subtree rooted at u with subtree rooted at v.
 * ============================================================ */

static void transplant(RBTree *t, RBNode *u, RBNode *v) {
    if (u->parent == t->nil)
        t->root = v;
    else if (u == u->parent->left)
        u->parent->left = v;
    else
        u->parent->right = v;
    v->parent = u->parent; /* Even if v is nil, nil->parent is updated */
}

/* ============================================================
 * Delete fixup
 * ============================================================ */

/*
 * Fix red-black properties after deletion.
 * x is the node that has "extra black" (double-black).
 */
static void delete_fixup(RBTree *t, RBNode *x) {
    while (x != t->root && x->color == BLACK) {
        if (x == x->parent->left) {
            RBNode *w = x->parent->right; /* sibling */

            if (w->color == RED) {
                /* Case 1: sibling is red */
                w->color          = BLACK;
                x->parent->color  = RED;
                rotate_left(t, x->parent);
                w = x->parent->right;
            }

            if (w->left->color == BLACK && w->right->color == BLACK) {
                /* Case 2: sibling's children both black */
                w->color = RED;
                x = x->parent;
            } else {
                if (w->right->color == BLACK) {
                    /* Case 3: sibling's right child is black */
                    w->left->color = BLACK;
                    w->color       = RED;
                    rotate_right(t, w);
                    w = x->parent->right;
                }
                /* Case 4: sibling's right child is red */
                w->color          = x->parent->color;
                x->parent->color  = BLACK;
                w->right->color   = BLACK;
                rotate_left(t, x->parent);
                x = t->root; /* Done */
            }
        } else {
            /* Mirror for right side */
            RBNode *w = x->parent->left;

            if (w->color == RED) {
                /* Case 1 mirror */
                w->color          = BLACK;
                x->parent->color  = RED;
                rotate_right(t, x->parent);
                w = x->parent->left;
            }

            if (w->right->color == BLACK && w->left->color == BLACK) {
                /* Case 2 mirror */
                w->color = RED;
                x = x->parent;
            } else {
                if (w->left->color == BLACK) {
                    /* Case 3 mirror */
                    w->right->color = BLACK;
                    w->color        = RED;
                    rotate_left(t, w);
                    w = x->parent->left;
                }
                /* Case 4 mirror */
                w->color          = x->parent->color;
                x->parent->color  = BLACK;
                w->left->color    = BLACK;
                rotate_right(t, x->parent);
                x = t->root;
            }
        }
    }
    x->color = BLACK;
}

/* ============================================================
 * Delete
 * ============================================================ */

static RBNode *tree_minimum(RBTree *t, RBNode *x) {
    while (x->left != t->nil)
        x = x->left;
    return x;
}

void rb_delete(RBTree *t, int key) {
    RBNode *z = search_node(t, key);
    if (!z) return;

    RBNode *y = z;
    RBNode *x;
    Color   y_original_color = y->color;

    if (z->left == t->nil) {
        x = z->right;
        transplant(t, z, z->right);
    } else if (z->right == t->nil) {
        x = z->left;
        transplant(t, z, z->left);
    } else {
        /* Two children: find in-order successor */
        y               = tree_minimum(t, z->right);
        y_original_color = y->color;
        x               = y->right;

        if (y->parent == z) {
            x->parent = y; /* even if x is nil */
        } else {
            transplant(t, y, y->right);
            y->right         = z->right;
            y->right->parent = y;
        }
        transplant(t, z, y);
        y->left         = z->left;
        y->left->parent = y;
        y->color        = z->color;
    }

    free(z);
    t->size--;

    if (y_original_color == BLACK)
        delete_fixup(t, x);
}

/* ============================================================
 * In-order traversal
 * ============================================================ */

static void inorder_helper(RBTree *t, RBNode *n,
                            void (*visit)(int, void *)) {
    if (n == t->nil) return;
    inorder_helper(t, n->left, visit);
    visit(n->key, n->val);
    inorder_helper(t, n->right, visit);
}

void rb_inorder(RBTree *t, void (*visit)(int, void *)) {
    inorder_helper(t, t->root, visit);
}

/* ============================================================
 * Verify red-black invariants (for testing)
 * ============================================================ */

static int verify_helper(RBTree *t, RBNode *n, int *black_height) {
    if (n == t->nil) {
        *black_height = 0;
        return 1;
    }

    /* Red node must have black children */
    if (n->color == RED) {
        if (n->left->color == RED || n->right->color == RED)
            return 0;
    }

    int lbh, rbh;
    if (!verify_helper(t, n->left, &lbh))  return 0;
    if (!verify_helper(t, n->right, &rbh)) return 0;

    /* Black heights must match */
    if (lbh != rbh) return 0;

    *black_height = lbh + (n->color == BLACK ? 1 : 0);
    return 1;
}

int rb_verify(RBTree *t) {
    if (t->root->color != BLACK) return 0;
    int bh;
    return verify_helper(t, t->root, &bh);
}

/* ============================================================
 * Cleanup
 * ============================================================ */

static void free_nodes(RBTree *t, RBNode *n) {
    if (n == t->nil) return;
    free_nodes(t, n->left);
    free_nodes(t, n->right);
    free(n);
}

void rb_destroy(RBTree *t) {
    free_nodes(t, t->root);
    free(t->nil);
    free(t);
}

/* ============================================================
 * Example usage:
 *
 * RBTree *t = rb_create();
 * rb_insert(t, 10, "ten");
 * rb_insert(t, 5,  "five");
 * rb_insert(t, 20, "twenty");
 *
 * char *v = rb_search(t, 5);   // "five"
 *
 * void print_kv(int k, void *v) { printf("%d=%s\n", k, (char*)v); }
 * rb_inorder(t, print_kv);     // sorted output
 *
 * rb_delete(t, 5);
 * assert(rb_verify(t));
 * rb_destroy(t);
 * ============================================================ */
```

---

## 14. Rust Implementation: AVL Tree

```rust
//! AVL Tree implementation in Rust.
//!
//! Uses an arena-less recursive box-pointer approach.
//! The type parameter K must be Ord + Clone.

use std::cmp::Ordering;
use std::fmt;

// ============================================================
// Node type
// ============================================================

#[derive(Debug)]
struct Node<K: Ord, V> {
    key:    K,
    val:    V,
    height: i32,
    left:   Option<Box<Node<K, V>>>,
    right:  Option<Box<Node<K, V>>>,
}

impl<K: Ord, V> Node<K, V> {
    fn new(key: K, val: V) -> Box<Self> {
        Box::new(Node {
            key,
            val,
            height: 0,
            left: None,
            right: None,
        })
    }
}

// ============================================================
// Height helpers
// ============================================================

fn height<K: Ord, V>(node: &Option<Box<Node<K, V>>>) -> i32 {
    node.as_ref().map_or(-1, |n| n.height)
}

fn update_height<K: Ord, V>(node: &mut Box<Node<K, V>>) {
    node.height = 1 + height(&node.left).max(height(&node.right));
}

fn balance_factor<K: Ord, V>(node: &Box<Node<K, V>>) -> i32 {
    height(&node.left) - height(&node.right)
}

// ============================================================
// Rotations
//
// Rust's ownership model makes in-place mutation of tree
// structure tricky. We use Option<Box<Node>> and replace
// nodes by taking ownership.
// ============================================================

/// Right rotation:
///
///       y                x
///      / \              / \
///     x   C    -->     A   y
///    / \                  / \
///   A   B                B   C
///
fn rotate_right<K: Ord, V>(mut y: Box<Node<K, V>>) -> Box<Node<K, V>> {
    let mut x = y.left.take().expect("rotate_right: no left child");
    let b = x.right.take();      // B subtree

    y.left = b;                  // B becomes y's left child
    update_height(&mut y);

    x.right = Some(y);           // y becomes x's right child
    update_height(&mut x);

    x                            // x is the new root of this subtree
}

/// Left rotation:
///
///     x                  y
///    / \                / \
///   A   y    -->       x   C
///      / \            / \
///     B   C          A   B
///
fn rotate_left<K: Ord, V>(mut x: Box<Node<K, V>>) -> Box<Node<K, V>> {
    let mut y = x.right.take().expect("rotate_left: no right child");
    let b = y.left.take();       // B subtree

    x.right = b;                 // B becomes x's right child
    update_height(&mut x);

    y.left = Some(x);            // x becomes y's left child
    update_height(&mut y);

    y                            // y is the new root of this subtree
}

// ============================================================
// Rebalancing
// ============================================================

fn rebalance<K: Ord, V>(mut node: Box<Node<K, V>>) -> Box<Node<K, V>> {
    update_height(&mut node);
    let bf = balance_factor(&node);

    if bf > 1 {
        // Left-heavy
        if balance_factor(node.left.as_ref().unwrap()) < 0 {
            // Left-Right case: rotate left child left first
            let left = node.left.take().unwrap();
            node.left = Some(rotate_left(left));
        }
        // Left-Left case (or after LR fix)
        return rotate_right(node);
    }

    if bf < -1 {
        // Right-heavy
        if balance_factor(node.right.as_ref().unwrap()) > 0 {
            // Right-Left case: rotate right child right first
            let right = node.right.take().unwrap();
            node.right = Some(rotate_right(right));
        }
        // Right-Right case (or after RL fix)
        return rotate_left(node);
    }

    node
}

// ============================================================
// Insert
// ============================================================

fn insert_node<K: Ord, V>(
    node: Option<Box<Node<K, V>>>,
    key: K,
    val: V,
) -> Box<Node<K, V>> {
    match node {
        None => Node::new(key, val),
        Some(mut n) => {
            match key.cmp(&n.key) {
                Ordering::Less => {
                    let left = n.left.take();
                    n.left = Some(insert_node(left, key, val));
                }
                Ordering::Greater => {
                    let right = n.right.take();
                    n.right = Some(insert_node(right, key, val));
                }
                Ordering::Equal => {
                    n.val = val; // update existing
                    return n;
                }
            }
            rebalance(n)
        }
    }
}

// ============================================================
// Min node extraction (used in delete)
// ============================================================

/// Remove and return the minimum node from the subtree.
/// Returns (min_node, new_subtree_root).
fn remove_min<K: Ord, V>(
    node: Box<Node<K, V>>,
) -> (Box<Node<K, V>>, Option<Box<Node<K, V>>>) {
    match node.left {
        None => {
            // This node IS the minimum
            let right = node.right;
            (node, right) // right becomes the new subtree (might be None)
        }
        Some(_) => {
            let mut n = node;
            let left = n.left.take().unwrap();
            let (min_node, new_left) = remove_min(left);
            n.left = new_left;
            let rebalanced = rebalance(n);
            (min_node, Some(rebalanced))
        }
    }
}

// ============================================================
// Delete
// ============================================================

fn delete_node<K: Ord, V>(
    node: Option<Box<Node<K, V>>>,
    key: &K,
) -> Option<Box<Node<K, V>>> {
    let mut n = node?;

    match key.cmp(&n.key) {
        Ordering::Less => {
            let left = n.left.take();
            n.left = delete_node(left, key);
        }
        Ordering::Greater => {
            let right = n.right.take();
            n.right = delete_node(right, key);
        }
        Ordering::Equal => {
            // Node to delete found
            return match (n.left.take(), n.right.take()) {
                (None, right)   => right,          // 0 or 1 child
                (left, None)    => left,
                (left, right) => {
                    // Two children: replace with in-order successor
                    let (mut succ, new_right) = remove_min(right.unwrap());
                    succ.left  = left;
                    succ.right = new_right;
                    Some(rebalance(succ))
                }
            };
        }
    }

    Some(rebalance(n))
}

// ============================================================
// Search
// ============================================================

fn search_node<'a, K: Ord, V>(
    node: &'a Option<Box<Node<K, V>>>,
    key: &K,
) -> Option<&'a V> {
    let n = node.as_ref()?;
    match key.cmp(&n.key) {
        Ordering::Equal   => Some(&n.val),
        Ordering::Less    => search_node(&n.left, key),
        Ordering::Greater => search_node(&n.right, key),
    }
}

// ============================================================
// In-order traversal
// ============================================================

fn inorder<K: Ord, V, F: FnMut(&K, &V)>(
    node: &Option<Box<Node<K, V>>>,
    f: &mut F,
) {
    if let Some(n) = node {
        inorder(&n.left, f);
        f(&n.key, &n.val);
        inorder(&n.right, f);
    }
}

// ============================================================
// Public AVL Tree struct
// ============================================================

pub struct AVLTree<K: Ord, V> {
    root: Option<Box<Node<K, V>>>,
    size: usize,
}

impl<K: Ord, V> AVLTree<K, V> {
    pub fn new() -> Self {
        AVLTree { root: None, size: 0 }
    }

    pub fn insert(&mut self, key: K, val: V) {
        // Determine if this is an update or new insert
        let is_new = search_node(&self.root, &key).is_none();
        let root = self.root.take();
        self.root = Some(insert_node(root, key, val));
        if is_new {
            self.size += 1;
        }
    }

    pub fn search(&self, key: &K) -> Option<&V> {
        search_node(&self.root, key)
    }

    pub fn delete(&mut self, key: &K) {
        let exists = self.search(key).is_some();
        if exists {
            let root = self.root.take();
            self.root = delete_node(root, key);
            self.size -= 1;
        }
    }

    pub fn len(&self) -> usize {
        self.size
    }

    pub fn is_empty(&self) -> bool {
        self.size == 0
    }

    pub fn height(&self) -> i32 {
        height(&self.root)
    }

    /// Returns all keys in sorted order.
    pub fn keys(&self) -> Vec<&K> {
        let mut result = Vec::with_capacity(self.size);
        inorder(&self.root, &mut |k, _| result.push(k));
        result
    }

    /// Returns all (key, value) pairs in sorted key order.
    pub fn iter(&self) -> Vec<(&K, &V)> {
        let mut result = Vec::with_capacity(self.size);
        inorder(&self.root, &mut |k, v| result.push((k, v)));
        result
    }

    /// Verifies the AVL invariant for all nodes. O(n).
    pub fn verify(&self) -> bool {
        fn check<K: Ord, V>(node: &Option<Box<Node<K, V>>>) -> bool {
            if let Some(n) = node {
                let bf = balance_factor(n);
                if bf.abs() > 1 { return false; }
                let expected_h = 1 + height(&n.left).max(height(&n.right));
                if n.height != expected_h { return false; }
                return check(&n.left) && check(&n.right);
            }
            true
        }
        check(&self.root)
    }
}

impl<K: Ord + fmt::Display, V: fmt::Display> AVLTree<K, V> {
    /// Print a readable (sideways) ASCII representation of the tree.
    pub fn print(&self) {
        fn print_node<K: Ord + fmt::Display, V: fmt::Display>(
            node: &Option<Box<Node<K, V>>>,
            prefix: &str,
            is_left: bool,
        ) {
            if let Some(n) = node {
                println!(
                    "{}{}[{}:{} h={}]",
                    prefix,
                    if is_left { "├── " } else { "└── " },
                    n.key,
                    n.val,
                    n.height
                );
                let new_prefix = format!("{}{}", prefix, if is_left { "│   " } else { "    " });
                print_node(&n.left, &new_prefix, true);
                print_node(&n.right, &new_prefix, false);
            }
        }
        if let Some(n) = &self.root {
            println!("[{}:{} h={}]", n.key, n.val, n.height);
            print_node(&n.left, "", true);
            print_node(&n.right, "", false);
        } else {
            println!("(empty)");
        }
    }
}

impl<K: Ord, V> Default for AVLTree<K, V> {
    fn default() -> Self { Self::new() }
}

// ============================================================
// Tests
// ============================================================

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_basic_insert_search() {
        let mut tree: AVLTree<i32, &str> = AVLTree::new();
        tree.insert(10, "ten");
        tree.insert(5,  "five");
        tree.insert(20, "twenty");
        tree.insert(3,  "three");
        tree.insert(7,  "seven");

        assert_eq!(tree.search(&5),  Some(&"five"));
        assert_eq!(tree.search(&10), Some(&"ten"));
        assert_eq!(tree.search(&99), None);
        assert!(tree.verify());
        assert_eq!(tree.len(), 5);
    }

    #[test]
    fn test_sorted_insert_stays_balanced() {
        let mut tree: AVLTree<i32, i32> = AVLTree::new();
        // Worst-case for plain BST: sorted insert
        for i in 0..100 {
            tree.insert(i, i * i);
        }
        assert!(tree.verify());
        // Height should be O(log n), not n
        assert!(tree.height() <= 15, "height {} too large", tree.height());
        assert_eq!(tree.len(), 100);
    }

    #[test]
    fn test_inorder_sorted() {
        let mut tree: AVLTree<i32, ()> = AVLTree::new();
        let vals = vec![5, 3, 7, 1, 4, 6, 8, 2];
        for v in &vals {
            tree.insert(*v, ());
        }
        let keys: Vec<i32> = tree.keys().into_iter().copied().collect();
        let mut expected = vals.clone();
        expected.sort();
        assert_eq!(keys, expected);
    }

    #[test]
    fn test_delete() {
        let mut tree: AVLTree<i32, &str> = AVLTree::new();
        for (k, v) in [(5,"a"),(3,"b"),(7,"c"),(1,"d"),(4,"e"),(6,"f"),(8,"g")] {
            tree.insert(k, v);
        }
        tree.delete(&3); // node with two children
        assert_eq!(tree.search(&3), None);
        assert!(tree.verify());
        assert_eq!(tree.len(), 6);

        tree.delete(&5); // root
        assert!(tree.verify());
    }

    #[test]
    fn test_string_keys() {
        let mut tree: AVLTree<String, i32> = AVLTree::new();
        tree.insert("banana".to_string(), 2);
        tree.insert("apple".to_string(), 1);
        tree.insert("cherry".to_string(), 3);

        assert_eq!(tree.search(&"apple".to_string()), Some(&1));
        let keys: Vec<&String> = tree.keys();
        assert_eq!(keys[0].as_str(), "apple");
        assert_eq!(keys[1].as_str(), "banana");
        assert_eq!(keys[2].as_str(), "cherry");
    }
}

/*
 * Usage example (in main.rs or a binary):
 *
 * fn main() {
 *     let mut tree: AVLTree<i32, &str> = AVLTree::new();
 *     for (k, v) in [(30,"thirty"),(20,"twenty"),(40,"forty"),
 *                    (10,"ten"),(25,"twenty-five"),(35,"thirty-five")] {
 *         tree.insert(k, v);
 *     }
 *     tree.print();
 *     // [30:thirty h=2]
 *     // ├── [20:twenty h=1]
 *     // │   ├── [10:ten h=0]
 *     // │   └── [25:twenty-five h=0]
 *     // └── [40:forty h=1]
 *     //     ├── [35:thirty-five h=0]
 *     //     └── (empty)
 *
 *     println!("Keys: {:?}", tree.keys());
 *     println!("Height: {}", tree.height());
 *     tree.delete(&20);
 *     println!("After deleting 20, valid: {}", tree.verify());
 * }
 */
```

---

## 15. Real-World Usage and Standard Libraries

### 15.1 Linux Kernel

The Linux kernel uses Red-Black trees extensively via `<linux/rbtree.h>`:

- **Completely Fair Scheduler (CFS)**: tasks are stored in an RB tree keyed by virtual runtime. The leftmost node (lowest runtime) is the next task to run.
- **Virtual Memory Areas (VMAs)**: each process's memory segments are stored in an RB tree for fast lookup by address.
- **Epoll**: file descriptors tracked in RB trees for O(log n) registration/removal.

### 15.2 C++ Standard Library

`std::map` and `std::set` are specified to use balanced BSTs (typically Red-Black trees in all major implementations: GCC libstdc++, LLVM libc++, MSVC STL).

```
std::map<int, std::string> m;
m[1] = "one";   // O(log n) insert
m[2] = "two";
auto it = m.find(1);  // O(log n) search
```

### 15.3 Java

`java.util.TreeMap` and `java.util.TreeSet` use Red-Black trees.

### 15.4 Go Standard Library

Go's `sort` package uses introsort. For ordered maps, Go currently (1.21+) offers `golang.org/x/exp/maps` and community-maintained BST packages, but the standard library uses hash maps (no built-in ordered map). Projects like `google/btree` provide B-tree implementations.

### 15.5 Rust Standard Library

`std::collections::BTreeMap` and `std::collections::BTreeSet` use B-trees (not binary trees), which are better for modern CPU cache performance.

### 15.6 Databases

- **MySQL InnoDB**: B+ trees for all indexes.
- **PostgreSQL**: B-trees for standard indexes; also supports hash, GiST (generalized search trees), GIN (inverted indexes).
- **SQLite**: B+ trees.
- **Redis**: AVL trees for sorted sets internally in some older versions; now uses skip lists for sorted sets (simpler concurrent implementation).
- **RocksDB/LevelDB**: B-trees for SSTable indexes; skip lists for in-memory MemTable.

---

## 16. Mental Models and Intuition Building

### 16.1 The "Gravity" Mental Model for Rotations

Think of an imbalanced node as a column that's **leaning** to one side. A rotation "pivots" the column around its middle child, bringing it back upright.

```
Right-heavy tree (leaning right):

   1              2
    \     →      / \
     2           1   3
      \
       3

The column [1, 2, 3] is leaning right. The pivot point is 2.
After rotation, 2 is the new center with 1 and 3 as symmetric children.
```

### 16.2 The "Bucket of Water" Model for Balance Factor

Imagine the tree as a mobile (hanging sculpture). Each node is a peg holding two sub-mobiles. The balance factor tells you how much the peg is tilted. An AVL tree keeps every peg within ±1 tilt. A Red-Black tree allows some pegs to be at ±2 but no peg can be consecutive with another heavily-tilted peg.

### 16.3 The "Bank Account" Model for Amortized Complexity

Think of rotations as expensive operations that cost "tokens." A splay tree or amortized analysis argument works like a bank account:

- Cheap operations (e.g., recolorings) "save" tokens.
- Expensive operations (rotations) "spend" tokens.
- Over a long sequence, the average cost per operation is O(log n) even if individual operations are sometimes O(1) and sometimes O(log n).

### 16.4 Why O(log n) Is Magical for Large n

```
n = 1 billion nodes (like a large database):

Balanced BST height:  log₂(1,000,000,000) ≈ 30
Degenerate BST:       1,000,000,000 comparisons

30 comparisons vs 1 billion. This is the difference between
a blink and a multi-hour wait.
```

### 16.5 The "Invariant Maintenance" Principle

The core principle of all balanced BSTs is: **maintain the invariant cheaply after every modification**.

- AVL: invariant is "BF ∈ {-1,0,1} at every node". Maintained by at most 1 rotation per insert.
- Red-Black: invariant is "no double-red + equal black-heights." Maintained by recoloring (cheap) + at most 3 rotations.

This is a general principle in data structure design: define a strong invariant, then prove you can restore it cheaply after any operation.

### 16.6 Thinking About Rotations as Key Insight

A rotation is **not about balance at all** — it's about **rearranging the tree while preserving the BST ordering invariant**. Balance only determines *when* to apply a rotation and *which direction*. The rotation itself is a pure pointer manipulation that:

1. Takes 3 nodes (grandparent, parent, child) and a subtree structure.
2. Rearranges them into an equivalent subtree where the BST property is maintained.
3. Adjusts heights/colors.

Once you internalize that a rotation preserves ordering while changing structure, the rest of balanced BST theory becomes mechanical.

### 16.7 When to Use Each Structure

```
Use AVL when:
├── Search-heavy workloads (more reads than writes)
├── You need the tightest possible height guarantee
└── Predictable per-operation worst-case time matters

Use Red-Black when:
├── Write-heavy workloads (many inserts/deletes)
├── You want O(1) amortized rotations (good for persistent/functional trees)
└── Implementing language runtime features (schedulers, allocators)

Use B-tree when:
├── Data doesn't fit in RAM (disk-based storage)
├── Cache efficiency is critical
└── Range queries are common

Use Skip list when:
├── Concurrent access needed (simpler lock-based or lock-free design)
└── Probabilistic guarantees are acceptable

Use Treap when:
├── You want simple, correct randomized balancing
└── The implementation simplicity matters more than worst-case guarantees
```

### 16.8 Common Bugs and Pitfalls

**Off-by-one in height**: Height of null is -1, not 0. Getting this wrong causes cascading incorrect balance factors.

**Forgetting to update heights after rotations**: Always update the child's height before the parent's height (bottom-up).

**Parent pointers in Red-Black trees**: Forgetting to update `parent` pointers in transplant, rotations, and fixup procedures causes dangling references.

**NIL sentinel initialization**: The NIL node must have `color = BLACK`. Checking `if (n == NULL)` instead of `if (n == t->nil)` causes crashes.

**Deletion in AVL with two children**: Must re-balance along the path from the deleted node's original position, not just from where you replaced the key.

**Rust borrow checker**: AVL/RB trees with parent pointers are notoriously hard in Rust's ownership model. The `Box<Node>` approach (no parent pointers) is the idiomatic choice; using `Rc<RefCell<Node>>` enables parent pointers but at the cost of runtime borrow checking.

---

## Appendix A: Height-Minimal AVL Trees (Fibonacci Trees)

The worst-case AVL tree (minimum nodes for a given height) is called a **Fibonacci tree** because the node counts follow a Fibonacci-like recurrence:

```
T(0):     •          (1 node, height 0)

T(1):     •          (2 nodes, height 1)
         /
        •

T(2):     •          (4 nodes, height 2)
         / \
        •   •
       /
      •

T(3):        •       (7 nodes, height 3)
            / \
           •   •
          / \   \
         •   •   •
        /
       •
```

T(h) = T(h-1) + T(h-2) + 1 (root + left subtree of height h-1 + right subtree of height h-2)

---

## Appendix B: Black-Height Examples

```
Tree (R=red, B=black):

            7(B)           <- black-height from here: 2
           /    \
         3(R)   18(R)      <- black-height: 2 (R doesn't count)
        /   \   /   \
      2(B) 4(B) 11(B) 19(B) <- black-height: 1
      /              \
    1(R)            14(R)  <- black-height: 1 (R)
   /                /
 NIL(B)           NIL(B)   <- black-height: 0 (NIL sentinel)

Verify black property:
Path: 7→3→2→1→NIL: B,R,B,R,B = 3 black nodes (7,2,NIL... counting NIL depends on convention)
Path: 7→18→19→NIL: B,R,B,B  = 3 black nodes
Path: 7→3→4→NIL:  B,R,B,B  = 3 black nodes
All paths: same black count. ✓
```

---

## Appendix C: Quick Reference Card

```
╔══════════════════════════════════════════════════════════╗
║           HEIGHT-BALANCED BST QUICK REFERENCE            ║
╠══════════════════════════════════════════════════════════╣
║ height(null)  = -1                                       ║
║ height(node)  = 1 + max(h(left), h(right))               ║
║ BF(node)      = h(left) - h(right)                       ║
║ AVL invariant: |BF| ≤ 1 for all nodes                    ║
║ RB invariant:  no 2 consec reds; equal black-heights     ║
╠══════════════════════════════════════════════════════════╣
║ ROTATION CASES (for node N with |BF| = 2):               ║
║   BF=+2, left.BF ≥ 0  → rotateRight(N)                  ║
║   BF=+2, left.BF < 0  → rotateLeft(N.left), rotRight(N) ║
║   BF=-2, right.BF ≤ 0 → rotateLeft(N)                   ║
║   BF=-2, right.BF > 0 → rotateRight(N.right), rotLeft(N)║
╠══════════════════════════════════════════════════════════╣
║ COMPLEXITY (n = number of nodes):                        ║
║   Search, Insert, Delete: O(log n) guaranteed            ║
║   AVL height ≤ 1.44 log₂(n)                             ║
║   RB height  ≤ 2 log₂(n+1)                              ║
╠══════════════════════════════════════════════════════════╣
║ INSERT rotations:  AVL ≤ 1,  RB ≤ 2                     ║
║ DELETE rotations:  AVL O(log n), RB ≤ 3                  ║
╚══════════════════════════════════════════════════════════╝
```
