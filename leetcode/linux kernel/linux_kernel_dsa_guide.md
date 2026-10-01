# Linux Kernel DSA Problems & Concepts Guide

## Overview
This guide maps Data Structures and Algorithms problems to their Linux kernel applications. Problems are sourced from LeetCode, HackerRank, CodeSignal, InterviewBit, and others.

---

## 1. HASH TABLES / HASH FUNCTIONS

**Kernel Use Cases:**
- Process tables (task_struct lookup)
- Page cache management
- Inode cache (dentry cache)
- Network socket tables
- Hash-based synchronization primitives

### Recommended Problems:

#### LeetCode
- **LeetCode 1** - Two Sum (Hash map basics)
- **LeetCode 49** - Group Anagrams (Hash function design)
- **LeetCode 146** - LRU Cache (Cache eviction like page cache)
- **LeetCode 155** - Min Stack (O(1) operations)
- **LeetCode 202** - Happy Number (Cycle detection with hash)
- **LeetCode 242** - Valid Anagram (Hash frequency)
- **LeetCode 347** - Top K Frequent Elements (K most used items)
- **LeetCode 359** - Logger Rate Limiter (Rate limiting in kernel)
- **LeetCode 380** - Insert Delete GetRandom O(1) (Hash + Array hybrid)
- **LeetCode 449** - Serialize and Deserialize BST (Tree hashing)
- **LeetCode 705** - Design HashSet
- **LeetCode 706** - Design HashMap
- **LeetCode 1396** - Design Underground System (State tracking)

#### HackerRank
- Hash Tables: Ransom Note
- Hash Tables: Ice Cream Parlor
- Hash Tables: Two Strings
- Count Triplets

#### CodeSignal
- firstDuplicate
- isCryptSolution
- containsDuplicate

#### InterviewBit
- Valid Sudoku
- Copy List with Random Pointer
- Longest Substring Without Repeating Characters

---

## 2. LINKED LISTS

**Kernel Use Cases:**
- Process queue management
- Memory allocation linked lists (buddy allocator)
- Event queue structures
- DMA descriptor chains
- Task scheduling queues

### Recommended Problems:

#### LeetCode
- **LeetCode 2** - Add Two Numbers (List manipulation)
- **LeetCode 19** - Remove Nth Node From End of List
- **LeetCode 21** - Merge Two Sorted Lists (Process merging)
- **LeetCode 23** - Merge k Sorted Lists (Priority queue like scheduling)
- **LeetCode 25** - Reverse Nodes in k-Group
- **LeetCode 61** - Rotate List
- **LeetCode 82** - Remove Duplicates from Sorted List II
- **LeetCode 86** - Partition List (Kernel process categorization)
- **LeetCode 92** - Reverse Linked List II
- **LeetCode 109** - Convert Sorted List to Binary Search Tree
- **LeetCode 138** - Copy List with Random Pointer (Process descriptor copy)
- **LeetCode 141** - Linked List Cycle (Deadlock detection)
- **LeetCode 142** - Linked List Cycle II (Find cycle entry - kernel lock analysis)
- **LeetCode 143** - Reorder List
- **LeetCode 148** - Sort List (Merge sort on linked structures)
- **LeetCode 160** - Intersection of Two Linked Lists (Memory sharing)
- **LeetCode 203** - Remove Linked List Elements
- **LeetCode 206** - Reverse Linked List
- **LeetCode 234** - Palindrome Linked List
- **LeetCode 328** - Odd Even Linked List (Queue organization)
- **LeetCode 430** - Flatten a Multilevel Doubly Linked List

#### HackerRank
- Insert a Node at Specific Position
- Delete Duplicate Nodes
- Reverse a Doubly Linked List
- Find the Merge Point

#### CodeSignal
- removeKFromList
- rearrangeLastN
- isListPalindrome

---

## 3. TREES & TREE VARIANTS

**Kernel Use Cases:**
- **Red-Black Trees**: Virtual memory management, process priority, filesystems
- **B-Trees**: Filesystem indices (ext4, btrfs)
- **Radix Trees**: Page cache, memory management
- **Tries**: Routing tables, DNS caching
- **Binary Search Trees**: Interval trees for memory regions

### 3A. Binary Search Trees / AVL Trees

#### LeetCode
- **LeetCode 98** - Validate Binary Search Tree
- **LeetCode 100** - Same Tree
- **LeetCode 101** - Symmetric Tree
- **LeetCode 102** - Binary Tree Level Order Traversal (BFS like process listing)
- **LeetCode 103** - Binary Tree Zigzag Level Order Traversal
- **LeetCode 104** - Maximum Depth of Binary Tree
- **LeetCode 105** - Construct Binary Tree from Preorder and Inorder
- **LeetCode 106** - Construct Binary Tree from Inorder and Postorder
- **LeetCode 110** - Balanced Binary Tree (Like maintaining balanced VM structures)
- **LeetCode 111** - Minimum Depth of Binary Tree
- **LeetCode 112** - Path Sum
- **LeetCode 113** - Path Sum II
- **LeetCode 114** - Flatten Binary Tree to Linked List
- **LeetCode 116** - Populating Next Right Pointers
- **LeetCode 124** - Binary Tree Maximum Path Sum
- **LeetCode 129** - Sum Root to Leaf Numbers
- **LeetCode 144** - Binary Tree Preorder Traversal
- **LeetCode 145** - Binary Tree Postorder Traversal
- **LeetCode 173** - Binary Search Tree Iterator
- **LeetCode 199** - Binary Tree Right Side View
- **LeetCode 222** - Count Complete Tree Nodes
- **LeetCode 226** - Invert Binary Tree
- **LeetCode 230** - Kth Smallest Element in a BST
- **LeetCode 235** - Lowest Common Ancestor of BST
- **LeetCode 236** - Lowest Common Ancestor of Binary Tree (Process relationship)
- **LeetCode 250** - Count Univalue Subtrees
- **LeetCode 255** - Verify Preorder Sequence in BST
- **LeetCode 257** - Binary Tree Paths
- **LeetCode 270** - Closest Binary Search Tree Value
- **LeetCode 285** - Inorder Successor in BST
- **LeetCode 297** - Serialize and Deserialize Binary Tree
- **LeetCode 314** - Binary Tree Vertical Order Traversal
- **LeetCode 333** - Largest Values in Each Tree Row
- **LeetCode 450** - Delete Node in a BST (Like removing process)
- **LeetCode 501** - Find Mode in Binary Search Tree
- **LeetCode 530** - Minimum Absolute Difference in BST
- **LeetCode 538** - Convert BST to Greater Sum Tree
- **LeetCode 543** - Diameter of Binary Tree
- **LeetCode 617** - Merge Two Binary Trees
- **LeetCode 623** - Add One Row to Tree
- **LeetCode 653** - Two Sum IV - Input is a BST
- **LeetCode 700** - Search in a Binary Search Tree
- **LeetCode 701** - Insert into a Binary Search Tree
- **LeetCode 783** - Minimum Distance Between BST Nodes

### 3B. Red-Black Trees / Self-Balancing Trees

#### LeetCode / Coding Challenge Sites
- **LeetCode 1157** - Online Majority Element in Subarray (TreeMap-like)
- **LeetCode 707** - Design Linked List (Can use balanced tree concepts)
- **Interval Tree Problems** (Not direct LeetCode but used in kernels):
  - Interval scheduling
  - Memory region overlap detection

#### HackerRank
- Self-Balancing Tree concept problems
- Range Query problems

#### CodeSignal
- bstWeightedSearch
- findSmallestDifference

### 3C. Trie / Prefix Trees

#### LeetCode
- **LeetCode 208** - Implement Trie (Prefix Tree)
- **LeetCode 211** - Design Add and Search Words Data Structure
- **LeetCode 212** - Word Search II (Trie-based)
- **LeetCode 472** - Concatenated Words
- **LeetCode 648** - Replace Words (Trie for quick lookup)
- **LeetCode 677** - Map Sum Pairs
- **LeetCode 1032** - Stream of Characters (Trie for pattern matching)

#### HackerRank
- No Prefix Set (Trie basics)
- Contacts (Trie for prefix search)

#### InterviewBit
- Shortest Unique Prefix
- Dictionary Trie
- Palindrome Pairs

### 3D. Segment Trees / Fenwick Trees

#### LeetCode
- **LeetCode 307** - Range Sum Query - Mutable (Segment tree use case)
- **LeetCode 308** - Range Sum Query 2D - Mutable
- **LeetCode 315** - Count of Smaller Numbers After Self
- **LeetCode 493** - Reverse Pairs (Fenwick tree)
- **LeetCode 1649** - Create Sorted Array Through Instructions (Fenwick tree)
- **LeetCode 1738** - Find Kth Largest XOR Coordinate Value

#### HackerRank
- Dynamic Array (Segment tree-like queries)

---

## 4. GRAPHS & GRAPH ALGORITHMS

**Kernel Use Cases:**
- Process dependencies & deadlock detection
- Interrupt controller dependency graphs
- Network routing
- Device driver dependencies
- Task scheduling DAGs (Directed Acyclic Graphs)
- Memory access patterns (cache graphs)

### Recommended Problems:

#### LeetCode
- **LeetCode 97** - Interleaving String (DP on graph)
- **LeetCode 127** - Word Ladder (BFS shortest path)
- **LeetCode 130** - Surrounded Regions (DFS/BFS)
- **LeetCode 133** - Clone Graph (Graph traversal)
- **LeetCode 207** - Course Schedule (Topological sort - dependency resolution)
- **LeetCode 208** - Course Schedule II (Topological sort - task ordering)
- **LeetCode 261** - Graph Valid Tree
- **LeetCode 269** - Alien Dictionary (Topological sort)
- **LeetCode 286** - Walls and Gates (BFS like interrupt propagation)
- **LeetCode 310** - Minimum Height Trees (Tree center finding)
- **LeetCode 323** - Number of Connected Components (Graph connectivity)
- **LeetCode 743** - Network Delay Time (Dijkstra's algorithm)
- **LeetCode 765** - Couples Holding Hands (Graph cycle detection)
- **LeetCode 815** - Bus Routes (Graph traversal)
- **LeetCode 847** - Shortest Path Visiting All Nodes (TSP variant)
- **LeetCode 863** - All Nodes Distance K in Binary Tree
- **LeetCode 997** - Find the Celebrity (Graph relationship)
- **LeetCode 1059** - All Paths from Source Lead to Destination
- **LeetCode 1203** - Sort Items by Groups Respecting Dependencies (Multi-level topological sort)
- **LeetCode 1245** - Tree Diameter
- **LeetCode 1334** - Find the City With the Smallest Number of Neighbors at a Threshold Distance (Floyd-Warshall)

#### HackerRank
- Breadth First Search: Shortest Reach
- Depth First Search
- Connected Components (Kruskal's Algorithm)
- Minimum MST: Kruskal's Algorithm
- Prim's Algorithm

#### CodeSignal
- hasPath
- connectedComponents
- largestComponentSize

#### InterviewBit
- Clone Binary Tree
- Reconstruct Itinerary
- Alien Dictionary
- Pacific Atlantic Water Flow
- Number of Islands

---

## 5. PRIORITY QUEUES / HEAPS

**Kernel Use Cases:**
- CPU scheduling (runqueue with priorities)
- I/O scheduling
- Timer management
- Network packet scheduling
- Memory page reclamation priority

### Recommended Problems:

#### LeetCode
- **LeetCode 23** - Merge k Sorted Lists (Heap for scheduling)
- **LeetCode 215** - Kth Largest Element in an Array
- **LeetCode 264** - Ugly Number II (Min heap generation)
- **LeetCode 295** - Find Median from Data Stream (Dual heap)
- **LeetCode 313** - Super Ugly Number
- **LeetCode 347** - Top K Frequent Elements (Heap for top items)
- **LeetCode 355** - Design Twitter (Priority queue for feed ordering)
- **LeetCode 692** - Top K Frequent Words
- **LeetCode 703** - Kth Largest Element in a Stream (Heap maintenance)
- **LeetCode 857** - Minimum Cost to Hire K Workers
- **LeetCode 1046** - Last Stone Weight (Min heap operations)
- **LeetCode 1157** - Online Majority Element in Subarray
- **LeetCode 1383** - Maximum Performance of a Team (Greedy + heap)
- **LeetCode 1675** - Minimize Deviation in Array

#### HackerRank
- QHEAP1 (Heap operations)
- Find the Running Median (Heap for median)
- Jesse and Cookies (Min heap)

#### CodeSignal
- findLargestValue
- nthSmallest

#### InterviewBit
- Magician and Chocolates (Heap greedy)
- Connect Ropes (Min heap)
- Merge K Sorted Lists

---

## 6. DYNAMIC PROGRAMMING

**Kernel Use Cases:**
- Memory allocation optimization
- Process scheduling decisions
- Cache replacement policies (LRU, LFU)
- Path finding in device trees
- Longest increasing subsequence (process priority ordering)

### Recommended Problems:

#### LeetCode
- **LeetCode 5** - Longest Palindromic Substring
- **LeetCode 10** - Regular Expression Matching (Pattern matching in kernel)
- **LeetCode 32** - Longest Valid Parentheses
- **LeetCode 42** - Trapping Rain Water (Optimization)
- **LeetCode 44** - Wildcard Matching
- **LeetCode 62** - Unique Paths
- **LeetCode 63** - Unique Paths II
- **LeetCode 64** - Minimum Path Sum
- **LeetCode 70** - Climbing Stairs
- **LeetCode 72** - Edit Distance (String algorithms in kernel)
- **LeetCode 87** - Scramble String
- **LeetCode 91** - Decode Ways
- **LeetCode 97** - Interleaving String
- **LeetCode 115** - Distinct Subsequences
- **LeetCode 120** - Triangle
- **LeetCode 132** - Palindrome Partitioning II
- **LeetCode 139** - Word Break
- **LeetCode 140** - Word Break II
- **LeetCode 188** - Best Time to Buy and Sell Stock IV (Resource allocation)
- **LeetCode 198** - House Robber
- **LeetCode 213** - House Robber II
- **LeetCode 221** - Maximal Square
- **LeetCode 264** - Ugly Number II
- **LeetCode 276** - Paint Fence (State transition)
- **LeetCode 279** - Perfect Squares
- **LeetCode 300** - Longest Increasing Subsequence
- **LeetCode 312** - Burst Balloons
- **LeetCode 322** - Coin Change
- **LeetCode 416** - Partition Equal Subset Sum
- **LeetCode 467** - Unique Substrings in Wraparound String
- **LeetCode 516** - Longest Palindromic Subsequence
- **LeetCode 583** - Delete Operation for Two Strings
- **LeetCode 673** - Number of Longest Increasing Subsequence
- **LeetCode 688** - Knight Probability in Chessboard (State space exploration)
- **LeetCode 1143** - Longest Common Subsequence
- **LeetCode 1312** - Minimum Insertion Steps to Make a String Palindrome

#### HackerRank
- The Coin Change Problem (Resource allocation)
- Fibonacci Modified
- Sherlock and Queries (Range updates)

#### InterviewBit
- Best Time to Buy and Sell Stock
- Jump Game Array
- Longest Arithmetic Progression

---

## 7. BIT MANIPULATION / BITWISE OPERATIONS

**Kernel Use Cases:**
- Process flags and permissions
- Memory bitmaps
- Interrupt masks
- CPU flags (EFLAGS)
- Bit-level synchronization
- Memory page flags
- Device registers

### Recommended Problems:

#### LeetCode
- **LeetCode 7** - Reverse Integer
- **LeetCode 29** - Divide Two Integers (Bitwise operations)
- **LeetCode 50** - Pow(x, n)
- **LeetCode 67** - Add Binary
- **LeetCode 137** - Single Number II
- **LeetCode 139** - Single Number III
- **LeetCode 190** - Reverse Bits
- **LeetCode 191** - Number of 1 Bits (popcount in kernel)
- **LeetCode 231** - Power of Two (Check flags)
- **LeetCode 260** - Single Number III
- **LeetCode 318** - Maximum Product of Word Lengths
- **LeetCode 319** - Bulb Switcher (Bit patterns)
- **LeetCode 320** - Generalized Abbreviation
- **LeetCode 338** - Counting Bits
- **LeetCode 371** - Sum of Two Integers (Bitwise addition)
- **LeetCode 389** - Find the Difference
- **LeetCode 393** - UTF-8 Validation (Bit pattern validation)
- **LeetCode 401** - Binary Watch
- **LeetCode 411** - Minimum Unique Word Abbreviation
- **LeetCode 421** - Maximum XOR of Two Numbers in an Array
- **LeetCode 461** - Hamming Distance
- **LeetCode 476** - Number Complement
- **LeetCode 1018** - Binary Prefix Divisible By 5

#### HackerRank
- Solve Me First
- Flipping bits
- Lonely Integer
- Maximizing XOR
- Sums of Powers

#### CodeSignal
- countBits
- largestBitmask
- rangeBitwiseAnd

#### InterviewBit
- Single Number
- Number of 1 Bits
- Reverse Bits
- Gray Code
- Palindrome Integer

---

## 8. SORTING & SEARCHING

**Kernel Use Cases:**
- Process scheduling (by priority/deadline)
- Memory region sorting
- Interrupt priority ordering
- Binary search in sorted tables
- Merge operations in memory management

### Sorting Algorithms

#### LeetCode
- **LeetCode 56** - Merge Intervals (Sort + merge)
- **LeetCode 57** - Insert Interval
- **LeetCode 148** - Sort List
- **LeetCode 164** - Maximum Gap
- **LeetCode 179** - Largest Number
- **LeetCode 252** - Meeting Rooms
- **LeetCode 253** - Meeting Rooms II (Interval scheduling)
- **LeetCode 349** - Intersection of Two Arrays
- **LeetCode 350** - Intersection of Two Arrays II
- **LeetCode 406** - Queue Reconstruction by Height
- **LeetCode 1122** - Relative Sort Array

### Binary Search

#### LeetCode
- **LeetCode 33** - Search in Rotated Sorted Array (Process table search)
- **LeetCode 34** - Find First and Last Position of Element
- **LeetCode 35** - Search Insert Position
- **LeetCode 69** - Sqrt(x)
- **LeetCode 74** - Search a 2D Matrix
- **LeetCode 153** - Find Minimum in Rotated Sorted Array
- **LeetCode 162** - Find Peak Element
- **LeetCode 240** - Search a 2D Matrix II
- **LeetCode 278** - First Bad Version (Linear scan like kernel updates)
- **LeetCode 475** - Heaters (Range coverage)
- **LeetCode 658** - Find K Closest Elements
- **LeetCode 1011** - Capacity To Ship Packages Within D Days (Binary search on answer)

#### HackerRank
- Binary Search: Ice Cream Parlor
- Simple Search

#### CodeSignal
- firstNotSmaller
- searchInSortedUnknownSizeArray

---

## 9. STRING ALGORITHMS

**Kernel Use Cases:**
- File path parsing
- Command-line argument processing
- Device name matching
- Pattern matching in logs
- String matching in buffers

### Recommended Problems:

#### LeetCode
- **LeetCode 3** - Longest Substring Without Repeating Characters
- **LeetCode 5** - Longest Palindromic Substring
- **LeetCode 8** - String to Integer (atoi)
- **LeetCode 10** - Regular Expression Matching (Pattern in kernel configs)
- **LeetCode 12** - Integer to Roman
- **LeetCode 13** - Roman to Integer
- **LeetCode 14** - Longest Common Prefix (Device name matching)
- **LeetCode 28** - Implement strStr() / Find the Index of the First Occurrence
- **LeetCode 38** - Count and Say
- **LeetCode 58** - Length of Last Word
- **LeetCode 65** - Valid Number
- **LeetCode 67** - Add Binary
- **LeetCode 125** - Valid Palindrome
- **LeetCode 151** - Reverse Words in a String
- **LeetCode 157** - Read N Characters Given Read4
- **LeetCode 158** - Read N Characters Given Read4 II
- **LeetCode 165** - Compare Version Numbers
- **LeetCode 186** - Reverse Words in a String II
- **LeetCode 214** - Shortest Palindrome
- **LeetCode 271** - Encode and Decode Strings (Serialization)
- **LeetCode 383** - Ransom Note
- **LeetCode 409** - Longest Palindrome (String composition)
- **LeetCode 434** - Number of Segments in a String
- **LeetCode 459** - Repeated Substring Pattern (KMP algorithm)
- **LeetCode 468** - Validate IP Address
- **LeetCode 520** - Detect Capital
- **LeetCode 539** - Minimum Time Difference (Time parsing)
- **LeetCode 556** - Next Greater Element III
- **LeetCode 606** - Construct String from Binary Tree (Serialization)
- **LeetCode 680** - Valid Palindrome II
- **LeetCode 686** - Repeated String Match (KMP/Boyer-Moore)
- **LeetCode 758** - Bold Words in String
- **LeetCode 791** - Custom Sort String
- **LeetCode 1044** - Longest Duplicate Substring (Suffix array / Rabin-Karp)

#### HackerRank
- Simple String Manipulation
- CamelCase
- Two Strings
- Strong Password Validation
- Sherlock and the Valid String

#### CodeSignal
- isIPv4Address
- isUnstableString
- isBeautifulString

#### InterviewBit
- Longest Palindromic Substring
- Add Binary Strings
- Multiply Strings
- ZigZag String

---

## 10. MEMORY MANAGEMENT ALGORITHMS

**Kernel Use Cases (Direct):**
- Buddy allocator
- Slab allocator
- Page replacement (LRU, Clock algorithm)
- Memory defragmentation

### Recommended Problems:

#### LeetCode
- **LeetCode 146** - LRU Cache (Page replacement)
- **LeetCode 460** - LFU Cache (Least Frequently Used - page reclamation)
- **LeetCode 1105** - Filling Bookcase Shelves (Memory layout optimization)
- **LeetCode 1157** - Online Majority Element in Subarray (Streaming memory)

#### HackerRank
- Balanced Parentheses (Stack memory allocation)

#### InterviewBit
- LRU Cache
- Largest Rectangle in Histogram

---

## 11. CONCURRENCY & SYNCHRONIZATION PRIMITIVES

**Kernel Use Cases:**
- Mutex and semaphore implementation
- Lock-free data structures
- Deadlock detection
- Race condition prevention

### Recommended Problems:

#### LeetCode
- **LeetCode 1115** - Print FooBarAlternately (Synchronization)
- **LeetCode 1116** - Print Zero Even Odd (Mutex-like primitives)
- **LeetCode 1117** - Building H2O (Semaphore-like problem)
- **LeetCode 1195** - Fizz Buzz Multithreaded (Thread coordination)
- **LeetCode 1226** - The Dining Philosophers (Deadlock classic)
- **LeetCode 1242** - Web Crawler Multithreaded (Lock coordination)
- **LeetCode 1279** - Traffic Light Controlled Intersection (State machine)
- **LeetCode 1311** - Get Watched Videos by Your Friends (Graph + synchronization)

---

## 12. SPECIAL KERNEL-SPECIFIC TOPICS

### Context Switching & Scheduling
- **LeetCode 253** - Meeting Rooms II (Process scheduling)
- **LeetCode 743** - Network Delay Time (Task deadline)
- **LeetCode 1229** - Meeting Scheduler

### Interrupt & Exception Handling
- **LeetCode 394** - Decode String (Stack-based like interrupt handlers)
- **LeetCode 402** - Remove K Digits (Priority decisions)

### Virtual Memory / Paging
- **LeetCode 146** - LRU Cache
- **LeetCode 460** - LFU Cache
- **LeetCode 2166** - Design Bitset (Page bitmaps)

### File System Operations
- **LeetCode 173** - Binary Search Tree Iterator (File tree traversal)
- **LeetCode 320** - Generalized Abbreviation (Path abbreviation)
- **LeetCode 751** - IP to CIDR (Network ranges)

---

## 13. STUDY PROGRESSION PATH

### Level 1: Fundamentals (Prerequisites)
1. Arrays & Strings (LeetCode 1-10)
2. Linked Lists (LeetCode 2, 19, 21)
3. Basic Trees (LeetCode 94, 100, 104)
4. Hash Maps (LeetCode 1, 242, 347)

### Level 2: Core Kernel DSA
1. Red-Black Tree concepts
2. Skip Lists
3. Interval Trees
4. Topological Sort (LeetCode 207, 208)
5. Dijkstra's Algorithm
6. Binary Search (LeetCode 33, 34)
7. Bit Manipulation (LeetCode 191, 231, 371)

### Level 3: Advanced Topics
1. Segment Trees / Fenwick Trees
2. Suffix Arrays / Suffix Trees
3. Trie data structures
4. Advanced Graph Algorithms (SCC, Bridges)
5. Dynamic Programming on Trees
6. Lock-free algorithms
7. Concurrent data structures

### Level 4: Kernel-Specific Deep Dive
1. sk_buff structure (linked list + memory allocation)
2. Runqueue implementation (priority queues)
3. Page cache (hash tables + trees)
4. VMA (Virtual Memory Area) trees
5. Cgroup subsystems (tree traversal)

---

## 14. RESOURCES & REFERENCES

### Online Judge Platforms
- **LeetCode**: https://leetcode.com (Best for breadth)
- **HackerRank**: https://www.hackerrank.com
- **CodeSignal**: https://codesignal.com
- **InterviewBit**: https://www.interviewbit.com
- **AtCoder**: https://atcoder.jp (Advanced algo)
- **Codeforces**: https://codeforces.com (Competitive programming)

### Kernel-Specific Resources
- **Linux Kernel Documentation**: https://www.kernel.org/doc/
- **Understanding the Linux Kernel** (Book by Daniel Bovet & Marco Cesati)
- **Linux Kernel Internals Course**: https://www.kernel.org/
- **Kernel Source Code**: https://github.com/torvalds/linux
- **LWN.net**: https://lwn.net (Kernel news & deep dives)

### Practice Strategy
1. **Solve 2-3 problems daily** from Level 1
2. **Understand kernel application** for each solved problem
3. **Implement each DS twice**: Generic + Kernel-specific variant
4. **Review kernel source code** after solving related DSA problem
5. **Join study groups** focused on kernel development

---

## 15. MAPPING TABLE: Problem → Kernel Concept

| Problem | LeetCode # | Platform | Kernel Concept |
|---------|-----------|----------|-----------------|
| Two Sum | 1 | LC | Hash table lookup |
| LRU Cache | 146 | LC | Page replacement |
| Merge k Sorted | 23 | LC | Scheduler merging |
| Course Schedule | 207 | LC | Dependency resolution |
| Single Number | 136 | LC | XOR flags |
| Valid Palindrome | 125 | LC | String parsing |
| Topological Sort | 207,208 | LC | Task ordering |
| Binary Search | 33,34 | LC | Process table search |
| Red-Black Tree | Custom | - | VM/CFS implementation |
| Linked List Cycle | 141,142 | LC | Deadlock detection |
| Binary Tree LCA | 236 | LC | Process hierarchy |
| Validate BST | 98 | LC | Interval validation |
| Graph Clone | 133 | LC | Process fork/copy |
| Number of Islands | Custom | IB | Memory region count |
| Meeting Rooms II | 253 | LC | CPU scheduling |
| Longest Substring | 3 | LC | Buffer operations |
| Read N Characters | 157,158 | LC | I/O buffering |
| Shortest Palindrome | 214 | LC | KMP/pattern matching |
| Design Hashmap | 706 | LC | Hashtable implementation |
| Implement Trie | 208 | LC | Routing lookup |

---

## Final Notes

### Why These Problems?
- **Hash Tables**: Used everywhere in kernel for O(1) lookups
- **Linked Lists**: Fundamental structure for queuing
- **Trees**: Memory management, scheduling, filesystems
- **Graphs**: Dependencies, routing, device hierarchies
- **Priority Queues**: CPU scheduling
- **Bit Manipulation**: Hardware flags, memory bitmaps
- **String Algorithms**: Path parsing, command processing
- **DP**: Optimization problems
- **Concurrency**: Synchronization primitives

### Implementation Tips
1. Always implement **thread-safe** versions of data structures
2. Consider **cache locality** during implementation
3. Handle **edge cases** (NULL pointers, boundaries)
4. Profile with **real kernel scenarios**
5. Study kernel source code after each problem

---

Good luck with your Linux kernel DSA journey! 🐧
