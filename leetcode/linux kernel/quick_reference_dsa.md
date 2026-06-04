# Quick Reference: Essential DSA Problems for Linux Kernel

## Must-Do Problems by Category (Top 60)

### Hash Tables & Hashing (6 problems)
| Problem | Platform | Difficulty | Kernel Use |
|---------|----------|-----------|-----------|
| Two Sum | LeetCode 1 | Easy | Hash lookup |
| LRU Cache | LeetCode 146 | Medium | Page cache |
| Group Anagrams | LeetCode 49 | Medium | Hash functions |
| Design HashMap | LeetCode 706 | Easy | Hash table |
| Valid Sudoku | InterviewBit | Medium | Hash validation |
| Logger Rate Limiter | LeetCode 359 | Easy | Rate limiting |

### Linked Lists (8 problems)
| Problem | Platform | Difficulty | Kernel Use |
|---------|----------|-----------|-----------|
| Reverse Linked List | LeetCode 206 | Easy | Memory structures |
| Merge Two Sorted Lists | LeetCode 21 | Easy | Queue merging |
| Linked List Cycle | LeetCode 141 | Easy | Deadlock detection |
| Linked List Cycle II | LeetCode 142 | Medium | Cycle entry point |
| Copy List with Random Pointer | LeetCode 138 | Medium | Process copy |
| Remove Nth Node | LeetCode 19 | Medium | Node deletion |
| Partition List | LeetCode 86 | Medium | Queue organization |
| Sort List | LeetCode 148 | Medium | Merge sort |

### Binary Search Trees (10 problems)
| Problem | Platform | Difficulty | Kernel Use |
|---------|----------|-----------|-----------|
| Validate BST | LeetCode 98 | Medium | Tree validation |
| Binary Tree Paths | LeetCode 257 | Easy | Path enumeration |
| Invert Binary Tree | LeetCode 226 | Easy | Tree manipulation |
| Lowest Common Ancestor | LeetCode 236 | Medium | Process hierarchy |
| Delete Node in BST | LeetCode 450 | Medium | Node removal |
| Serialize/Deserialize BST | LeetCode 449 | Hard | Tree persistence |
| Convert BST to GST | LeetCode 538 | Medium | Tree traversal |
| Kth Smallest in BST | LeetCode 230 | Medium | Ordered access |
| Balanced Binary Tree | LeetCode 110 | Easy | Balance checking |
| Binary Tree Level Order | LeetCode 102 | Medium | BFS traversal |

### Graphs & Topological Sort (8 problems)
| Problem | Platform | Difficulty | Kernel Use |
|---------|----------|-----------|-----------|
| Course Schedule | LeetCode 207 | Medium | Dependency resolution |
| Course Schedule II | LeetCode 210 | Medium | Task ordering |
| Clone Graph | LeetCode 133 | Medium | Process cloning |
| Network Delay Time | LeetCode 743 | Medium | Dijkstra's algorithm |
| Alien Dictionary | LeetCode 269 | Hard | Topological sort |
| Connected Components | CodeSignal | Medium | Graph connectivity |
| Number of Islands | Custom | Medium | Region counting |
| All Nodes Distance K | LeetCode 863 | Medium | Tree radius |

### Priority Queues & Heaps (6 problems)
| Problem | Platform | Difficulty | Kernel Use |
|---------|----------|-----------|-----------|
| Merge k Sorted Lists | LeetCode 23 | Hard | Scheduler merge |
| Kth Largest Element | LeetCode 215 | Medium | Heap operations |
| Find Median | LeetCode 295 | Hard | Dual heap |
| Top K Frequent | LeetCode 347 | Medium | Top items |
| Last Stone Weight | LeetCode 1046 | Easy | Min heap |
| Design Twitter | LeetCode 355 | Medium | Priority feed |

### Bit Manipulation (6 problems)
| Problem | Platform | Difficulty | Kernel Use |
|---------|----------|-----------|-----------|
| Number of 1 Bits | LeetCode 191 | Easy | popcount() |
| Power of Two | LeetCode 231 | Easy | Flag checking |
| Single Number | LeetCode 136 | Easy | XOR operations |
| Reverse Bits | LeetCode 190 | Easy | Bit reversal |
| Sum of Two Integers | LeetCode 371 | Medium | Bitwise addition |
| Hamming Distance | LeetCode 461 | Easy | Bit differences |

### Searching & Sorting (8 problems)
| Problem | Platform | Difficulty | Kernel Use |
|---------|----------|-----------|-----------|
| Search Rotated Sorted Array | LeetCode 33 | Medium | Table search |
| Find First & Last Position | LeetCode 34 | Medium | Range finding |
| Find Peak Element | LeetCode 162 | Medium | Peak detection |
| Meeting Rooms II | LeetCode 253 | Medium | Scheduling |
| Merge Intervals | LeetCode 56 | Medium | Interval merging |
| Insert Interval | LeetCode 57 | Hard | Interval insertion |
| Kth Largest in Stream | LeetCode 703 | Easy | Online k-th |
| Find K Closest Elements | LeetCode 658 | Medium | Range query |

### String Algorithms (6 problems)
| Problem | Platform | Difficulty | Kernel Use |
|---------|----------|-----------|-----------|
| Longest Substring No Repeat | LeetCode 3 | Medium | Buffer ops |
| Implement strStr() | LeetCode 28 | Easy | String matching |
| Repeated Substring Pattern | LeetCode 459 | Medium | KMP algorithm |
| Regular Expression Match | LeetCode 10 | Hard | Pattern matching |
| Shortest Palindrome | LeetCode 214 | Hard | KMP variant |
| Validate IP Address | LeetCode 468 | Medium | Address parsing |

### Dynamic Programming (6 problems)
| Problem | Platform | Difficulty | Kernel Use |
|---------|----------|-----------|-----------|
| Longest Increasing Subsequence | LeetCode 300 | Medium | Ordering |
| Coin Change | LeetCode 322 | Medium | Resource alloc |
| Edit Distance | LeetCode 72 | Medium | String ops |
| House Robber | LeetCode 198 | Easy | Optimization |
| Best Time to Buy Stock | Custom | Easy | Decision making |
| Trapping Rain Water | LeetCode 42 | Hard | Optimization |

### Tries & Advanced Trees (4 problems)
| Problem | Platform | Difficulty | Kernel Use |
|---------|----------|-----------|-----------|
| Implement Trie | LeetCode 208 | Medium | Prefix tree |
| Add & Search Words | LeetCode 211 | Medium | Pattern search |
| Word Search II | LeetCode 212 | Hard | Trie-based search |
| Shortest Unique Prefix | InterviewBit | Medium | Trie traversal |

### Concurrency (4 problems)
| Problem | Platform | Difficulty | Kernel Use |
|---------|----------|-----------|-----------|
| Print FooBar | LeetCode 1115 | Medium | Synchronization |
| Building H2O | LeetCode 1117 | Medium | Semaphore |
| Print Zero Even Odd | LeetCode 1116 | Medium | Mutex |
| Dining Philosophers | LeetCode 1226 | Medium | Deadlock |

---

## Study Plan: 60-Day Challenge

### Week 1-2: Foundations (Hash + Linked Lists)
- Day 1-3: LeetCode 1, 242, 347 (Hash basics)
- Day 4-5: LeetCode 206, 21, 19 (List basics)
- Day 6-7: LeetCode 146 (LRU)
- Day 8-14: Review + implement variants

### Week 3-4: Trees (BST + Traversals)
- Day 15-17: LeetCode 98, 100, 104 (BST basics)
- Day 18-20: LeetCode 236, 230, 450 (BST operations)
- Day 21-22: LeetCode 102, 145 (Traversals)
- Day 23-28: Serialize, delete, convert problems

### Week 5-6: Graphs (DFS + BFS + Topo Sort)
- Day 29-31: LeetCode 133, 207, 210 (Graph basics)
- Day 32-34: LeetCode 743, 269 (Advanced graph)
- Day 35-36: Topological sort variants
- Day 37-42: Review + implement

### Week 7-8: Heaps & Searching
- Day 43-45: LeetCode 23, 215, 347 (Heap)
- Day 46-48: LeetCode 33, 34, 162 (Binary search)
- Day 49-50: LeetCode 295, 703 (Advanced heap)
- Day 51-56: Integration problems

### Week 9: Bit + Concurrency + Special Topics
- Day 57-59: LeetCode 191, 231, 136 (Bits)
- Day 60: LeetCode 1115, 1117, 1226 (Concurrency)

---

## Problem Difficulty Progression

**Start Here (If New):**
```
Easy:
- Two Sum (1)
- Reverse Linked List (206)
- Valid Palindrome (125)
- Power of Two (231)
- Number of 1 Bits (191)
- Implement Trie (208)
```

**Intermediate (Core Knowledge):**
```
Medium:
- LRU Cache (146)
- LFU Cache (460)
- Course Schedule (207)
- Merge k Sorted Lists (23)
- Kth Largest Element (215)
- Lowest Common Ancestor (236)
- Rotate Array (189)
- Search Rotated Sorted Array (33)
```

**Advanced (Kernel Deep Dive):**
```
Hard:
- Serialize/Deserialize BST (449)
- Word Search II (212)
- Regular Expression Matching (10)
- Shortest Palindrome (214)
- Alien Dictionary (269)
- Trapping Rain Water (42)
- Edit Distance (72)
- Median of Two Sorted (4)
```

---

## Quick Links to All Problems

### By Platform
**LeetCode**: https://leetcode.com/problemset/all/
**HackerRank**: https://www.hackerrank.com/domains/data-structures
**CodeSignal**: https://codesignal.com/
**InterviewBit**: https://www.interviewbit.com/problems/

### Recommended Order of Solving
1. **Master fundamentals** (Arrays, Strings, Lists)
2. **Trees & BSTs** (Most important for kernel)
3. **Graphs** (Dependencies, scheduling)
4. **Heaps** (Scheduling queues)
5. **Advanced** (Tries, Segment Trees, Fenwick)

---

## Tips for Kernel DSA Learning

1. **After each problem**:
   - Implement the data structure from scratch
   - Search for it in Linux kernel source
   - Understand how kernel uses it differently

2. **Focus areas**:
   - Memory efficiency (kernel has constraints)
   - Cache locality (important for performance)
   - Thread safety (kernel is heavily concurrent)
   - Edge case handling (kernel needs robustness)

3. **Real kernel structures to study**:
   - `sk_buff` (networking) - Complex linked list + allocation
   - `task_struct` (processes) - Red-Black tree + hash tables
   - `page` struct (memory) - Bitmap + linked lists
   - `rbtree` (scheduler) - Red-Black tree implementation
   - `hlist` (kernel hash) - Hash list implementation

4. **Testing strategy**:
   - Write unit tests for each structure
   - Test edge cases (empty, single element, large)
   - Benchmark performance
   - Test with multiple threads

---

## Checkpoint Milestones

- [ ] **Day 7**: Complete all hash and linked list problems
- [ ] **Day 21**: Understand all tree traversals and operations
- [ ] **Day 35**: Master graph algorithms and topological sort
- [ ] **Day 49**: Comfortable with heaps and advanced searching
- [ ] **Day 60**: Can implement any DS and understand kernel usage

---

## Additional Resources

**Books:**
- "Cracking the Coding Interview" - DSA fundamentals
- "Understanding the Linux Kernel" - Kernel internals
- "The Linux Programming Interface" - System programming

**YouTube Channels:**
- Abdul Bari (Algorithms)
- William Fiset (Data Structures)
- Kunal Kushwaha (DSA)

**GitHub Repos:**
- https://github.com/torvalds/linux (Kernel source)
- https://github.com/bslatkin/effective-python (Python patterns)

