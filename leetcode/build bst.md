package main

import "fmt"

// TreeNode is LeetCode's standard definition for a binary tree node.
type TreeNode struct {
	Val   int
	Left  *TreeNode
	Right *TreeNode
}

// BuildTree constructs the binary tree recursively from an array representation.
// It uses the standard complete binary tree property:
// Left Child = 2 * index + 1
// Right Child = 2 * index + 2
func BuildTree(nums []*int, index int) *TreeNode {
	// Base case: if the index is out of bounds or the node is null (nil)
	if index >= len(nums) || nums[index] == nil {
		return nil
	}

	// Create the current node
	node := &TreeNode{
		Val: *nums[index],
	}

	// Recursively build the left and right subtrees
	node.Left = BuildTree(nums, 2*index+1)
	node.Right = BuildTree(nums, 2*index+2)

	return node
}

// ptr is a helper function to easily create pointers to integers inline.
func ptr(v int) *int {
	return &v
}

func main() {
	// Example 1 Input from LeetCode 98: root = [2,1,3]
	input1 := []*int{ptr(2), ptr(1), ptr(3)}
	root1 := BuildTree(input1, 0)
	
	// Example 2 Input from LeetCode 98: root = [5,1,4,null,null,3,6]
	input2 := []*int{ptr(5), ptr(1), ptr(4), nil, nil, ptr(3), ptr(6)}
	root2 := BuildTree(input2, 0)

	// You can now pass root1 or root2 into your isValidBST function.
	fmt.Printf("Tree 1 Root: %d\n", root1.Val)
	fmt.Printf("Tree 2 Root: %d\n", root2.Val)
}

Here is the recursive template in Go to build a binary tree for local testing.

Since LeetCode inputs include `null` values for missing nodes, the standard Go workaround is to use a slice of integer pointers (`[]*int`), where `nil` represents a `null` node.

This template uses the standard array-to-tree recursive formula where the left child of index `i` is at `2*i + 1` and the right child is at `2*i + 2`.

### Go Recursive Tree Builder Template

```go
package main

import "fmt"

// TreeNode is LeetCode's standard definition for a binary tree node.
type TreeNode struct {
	Val   int
	Left  *TreeNode
	Right *TreeNode
}

// BuildTree constructs the binary tree recursively from an array representation.
// It uses the standard complete binary tree property:
// Left Child = 2 * index + 1
// Right Child = 2 * index + 2
func BuildTree(nums []*int, index int) *TreeNode {
	// Base case: if the index is out of bounds or the node is null (nil)
	if index >= len(nums) || nums[index] == nil {
		return nil
	}

	// Create the current node
	node := &TreeNode{
		Val: *nums[index],
	}

	// Recursively build the left and right subtrees
	node.Left = BuildTree(nums, 2*index+1)
	node.Right = BuildTree(nums, 2*index+2)

	return node
}

// ptr is a helper function to easily create pointers to integers inline.
func ptr(v int) *int {
	return &v
}

func main() {
	// Example 1 Input from LeetCode 98: root = [2,1,3]
	input1 := []*int{ptr(2), ptr(1), ptr(3)}
	root1 := BuildTree(input1, 0)
	
	// Example 2 Input from LeetCode 98: root = [5,1,4,null,null,3,6]
	input2 := []*int{ptr(5), ptr(1), ptr(4), nil, nil, ptr(3), ptr(6)}
	root2 := BuildTree(input2, 0)

	// You can now pass root1 or root2 into your isValidBST function.
	fmt.Printf("Tree 1 Root: %d\n", root1.Val)
	fmt.Printf("Tree 2 Root: %d\n", root2.Val)
}

```

### Note on LeetCode's Input Format

LeetCode visually truncates the `null` children of `null` nodes in their array representation to save space. If you are building a deeply unbalanced tree locally using this mathematical `2*i + 1` recursive approach, you may need to manually pad your local input arrays with extra `nil` values to ensure the elements line up with the correct indices.

You have excellent intuition! You are completely right to call this out, and it's a very common point of confusion when transitioning from textbook data structures to LeetCode problems.

In a real-world scenario, if you were building a Binary Search Tree (BST) from scratch, you would use a strict **BST Insertion Algorithm** (where numbers smaller than the root go left, and numbers larger go right).

However, for **LeetCode 98: Validate Binary Search Tree**, we cannot use a real BST builder. Here is why:

---

### 1. The Goal is to Detect "Broken" Trees

The entire point of this LeetCode problem is to write a function that can look at a tree and tell us if it is a **valid** BST or an **invalid** BST.

LeetCode passes inputs that are intentionally broken to test your code. For example, look at Example 2 from the problem:

* **Input Array:** `[5, 1, 4, null, null, 3, 6]`

If we visualize this exact array as a standard binary tree based on position, it looks like this:

```text
       5
      / \
     1   4
        / \
       3   6

```

*(Notice that `3` is in the right subtree of `5`, which violates the BST rule because 3 is less than 5! This tree is invalid.)*

---

### 2. Why a Real BST Builder Fails Us Here

If we used a real, rule-following BST insertion function to build a tree from those same numbers `[5, 1, 4, 3, 6]`, the algorithm would automatically "fix" the tree for us. It would place the `3` to the left of the `4`, creating a perfectly valid BST:

```text
       5
      / \
     1   4
        / \
       3*  6   <-- A real BST builder puts 3 here, making it valid!

```

If our tree builder automatically fixes the errors, then our `isValidBST` function will *always* receive a perfect tree, and we will never be able to test if our code can catch the invalid ones.

---

### 3. What the Template Actually Does

The template I gave you is a **Level-Order / Complete Binary Tree Builder**. It doesn't care about the *values* of the numbers at all. It only cares about their *position* in the array.

It maps indices mathematically:

* The left child of index `i` is always at `2*i + 1`
* The right child of index `i` is always at `2*i + 2`

> **Summary:** You are 100% correct that this is a custom builder. It is a "blind" binary tree builder designed specifically to recreate LeetCode's literal array format—flaws and all—so that you have a broken tree to test your validation logic against!

Here is the code for both approaches. Look closely at the core decision-making step in each function—that is where the magic (and the difference) happens.

---

### 1. The Blind Custom Builder (Position-Based)

This builder **ignores the value** of the number. It forces the node into a specific spot in the tree based purely on its index in the array.

```go
// BuildBlindTree places nodes strictly by their mathematical array position.
func BuildBlindTree(nums []*int, index int) *TreeNode {
	// 1. Base Case: Out of bounds or explicit 'null' placeholder
	if index >= len(nums) || nums[index] == nil {
		return nil
	}

	// 2. Create the node exactly where the array dictates
	node := &TreeNode{Val: *nums[index]}

	// 3. CORE DIFFERENCE: Math decides the path, NOT the value!
	node.Left = BuildBlindTree(nums, 2*index+1)
	node.Right = BuildBlindTree(nums, 2*index+2)

	return node
}

```

---

### 2. The Real BST Builder (Value-Based)

This builder **ignores the array index** once it starts placing a node. Instead, it compares the new value against existing node values to find its rightful, rule-abiding home.

```go
// InsertBST inserts a single value based on strict BST rules.
func InsertBST(root *TreeNode, val int) *TreeNode {
	// 1. Base Case: Found an empty spot, drop the new node here
	if root == nil {
		return &TreeNode{Val: val}
	}

	// 2. CORE DIFFERENCE: The VALUE decides the path, NOT math!
	if val < root.Val {
		// Smaller values ALWAYS go left
		root.Left = InsertBST(root.Left, val)
	} else {
		// Larger or equal values ALWAYS go right
		root.Right = InsertBST(root.Right, val)
	}

	return root
}

// BuildRealBST loops through a raw integer slice to build a valid BST.
func BuildRealBST(nums []int) *TreeNode {
	var root *TreeNode
	for _, val := range nums {
		root = InsertBST(root, val)
	}
	return root
}

```

---

### Key Differences Explained

[Image comparing Binary Search Tree insertion and complete binary tree array mapping]

| Feature | Blind Custom Builder | Real BST Builder |
| --- | --- | --- |
| **The Driver** | Driven by **Array Index** (`index`). | Driven by **Node Value** (`val`). |
| **Tree Shape** | Matches the literal sequence of the array (even if it's a broken/invalid BST). | Guarantees a valid BST structure every single time. |
| **Array Input Format** | Requires `nil` (`null`) pointers to represent skipped or empty branches. | Takes a clean, flat slice of standard integers (`[]int`). No `nil` placeholders required. |
| **Why use it?** | **For Testing:** Recreates broken trees so you can test if your validator code actually catches errors. | **For Production:** Used when you need an actual functioning, searchable BST data structure. |

A Binary Search Tree (BST) is a foundational data structure in computer science because it turns linear searching into a highly efficient logarithmic guessing game.

Here are all the fundamental, structural, and behavioral properties of a Binary Search Tree.

---

### 1. The Core Ordering Property (The Golden Rule)

For any given node (let's call it the **Root** node) in the tree:

* **Left Subtree Property:** The values of all the nodes in the left subtree must be **strictly less than** the value of the root node.
* **Right Subtree Property:** The values of all the nodes in the right subtree must be **strictly greater than** the value of the root node.
* **Recursive Property:** This rule must apply flawlessly to *every single sub-node* down the entire tree, not just the top root.

> **Note on Duplicates:** Traditionally, a strict BST does not allow duplicate values. However, if duplicates are allowed by a specific implementation, they must be handled consistently (e.g., *always* placed in the left subtree, or *always* placed in the right).

---

### 2. The Traversal Property (The In-Order Trick)

If you perform an **In-Order Traversal** (visiting the Left subtree, then the Root, then the Right subtree) on a valid BST, the nodes will always be visited in **strictly ascending, sorted order**.

* If your tree contains `[5, 1, 4, 3, 6]` and an in-order traversal outputs `1, 3, 4, 5, 6`, you have a valid BST.
* *Why this matters for LeetCode 98:* This property is a highly popular way to validate a BST! If you run an in-order traversal and the numbers aren't perfectly sorted, the tree is invalid.

---

### 3. Structural Properties

* **Binary Nature:** Each node can have a maximum of two children: a left child and a right child (hence, *binary*).
* **Acyclic:** It is a tree, meaning it contains no cycles or loops.
* **Single Root:** There is exactly one top-level node from which all other nodes descend.

---

### 4. Computational & Time Complexity Properties

The efficiency of a BST depends entirely on how "balanced" it is.

| Operation | Average Case (Balanced Tree) | Worst Case (Skewed Tree / Linked List) |
| --- | --- | --- |
| **Search** | $O(\log n)$ | $O(n)$ |
| **Insertion** | $O(\log n)$ | $O(n)$ |
| **Deletion** | $O(\log n)$ | $O(n)$ |

* **The $O(\log n)$ Magic:** In a balanced tree, every time you move down a level (left or right), you throw away half of the remaining possibilities. This is identical to how binary search works on an array.
* **The $O(n)$ Nightmare:** If you insert numbers in sorted order (like `1, 2, 3, 4, 5`), the BST turns into a straight line (a skewed tree). At this point, it behaves like a standard linked list, losing all its speed advantages. This limitation is what inspired self-balancing trees like AVL and Red-Black trees.

---

Are you planning to use the **In-Order Traversal** approach to solve LeetCode 98, or are you looking to use a **Min/Max boundary constraint** approach?

Based on the level-order mapping, here is exactly how that array looks as a binary tree in ASCII:

```text
       5
      / \
     1   4
        / \
       3   6

```

### Breaking down the levels:

* **Level 0 (Root):** `5`
* **Level 1:** `1` (left child of 5) and `4` (right child of 5)
* **Level 2:** The `null, null` in the array mean the `1` has no children. The next values, `3` and `6`, become the left and right children of `4`.

### Why this fails the BST validation:

Even though `3` is correctly to the left of `4`, it is residing in the **right subtree** of the main root `5`. Because $3 < 5$, it violates the golden rule that *everything* to the right of `5` must be greater than `5`.

If we take those exact same numbers—`1, 3, 4, 5, 6`—there are a few ways to arrange them into a **correct, valid BST**.

Here are the two most common ways to see it:

### 1. The "Real BST Builder" Result

If you fed the sequence `[5, 1, 4, 3, 6]` into the strict, rule-following BST insertion algorithm we looked at earlier, it would automatically place the `3` and the `4` in their correct legal positions.

It would look like this:

```text
       5
      / \
     1   6
      \
       4
      /
     3

```

**Why this is 100% valid:**

* **Check Root 5:** Everything in its left subtree (`1, 4, 3`) is strictly less than 5. Everything in its right subtree (`6`) is greater than 5.
* **Check Node 1:** Everything in its right subtree (`4, 3`) is greater than 1.
* **Check Node 4:** Its left child (`3`) is less than 4.

---

### 2. The "Perfectly Balanced" Result

If you wanted to build the most efficient, perfectly balanced tree using those numbers, you would put the middle number (`4`) at the very top as the root.

It would look like this:

```text
       4
     /   \
    1     5
     \     \
      3     6

```

**Why this is 100% valid:**

* **Check Root 4:** Left side (`1, 3`) is less than 4; right side (`5, 6`) is greater than 4.
* **Check Node 1:** Its right child (`3`) is greater than 1.
* **Check Node 5:** Its right child (`6`) is greater than 5.

### Summary of the Difference

In the invalid LeetCode input, `3` was forced to be a child of `4` while staying on the right side of `5` (which broke the rule). In both correct trees above, **`3` has been moved to the left side of `5**`, where it rightfully belongs because $3 < 5$.

Now that you see what a valid vs. invalid tree looks like, do you want to talk about how your recursion logic can track these upper and lower boundaries to catch the invalid nodes?
