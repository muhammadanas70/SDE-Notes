# Linux Kernel DSA: Concept Map + Practice Problems

> Every data structure in this guide is *actually used* in the Linux kernel source tree.
> Problems are drawn from LeetCode, CSES, SPOJ, HackerRank, Codeforces, GeeksForGeeks, and AtCoder.

**Legend**
```
[LC-N]   → LeetCode problem N          (leetcode.com/problems/)
[CSES]   → cses.fi/problemset/task/N
[SPOJ]   → spoj.com/problems/
[HR]     → HackerRank
[CF]     → Codeforces
[GFG]    → GeeksForGeeks
[AC]     → AtCoder
★        → closest / most direct analog to the actual kernel construct
💀       → Hard
🟡       → Medium
🟢       → Easy
```

---

## PART 1 — Master Reference Table

```
+──────────────────────────────+────────────────────────────────+──────────────────────────────────────────────────+
│ Kernel Subsystem             │ Kernel Source File(s)          │ Data Structure / Algorithm                       │
+──────────────────────────────+────────────────────────────────+──────────────────────────────────────────────────+
│ CFS Scheduler                │ kernel/sched/fair.c            │ Red-Black Tree (vruntime-keyed)                  │
│ RT Scheduler                 │ kernel/sched/rt.c              │ Bitmap Priority Array + Linked List              │
│ Deadline Scheduler           │ kernel/sched/deadline.c        │ Red-Black Tree (deadline-keyed)                  │
│ Timer Wheel                  │ kernel/time/timer_wheel.c      │ Hierarchical Timing Wheel (bucket array + lists) │
│ Buddy Allocator              │ mm/page_alloc.c                │ Free-List Array indexed by order (power-of-2)    │
│ VMA Management               │ mm/mmap.c, mm/interval_tree.c  │ Interval Tree + Red-Black Tree                   │
│ Page Cache                   │ mm/filemap.c                   │ XArray (radix tree) + LRU list                   │
│ Slab Allocator               │ mm/slab.c, mm/slub.c           │ Linked Free-List + Kmem Cache                    │
│ IDR / IDA (fd, pid)          │ lib/idr.c                      │ Radix Tree over integer keys                     │
│ VFS dcache                   │ fs/dcache.c                    │ Hash Table (chaining) + LRU                      │
│ VFS icache                   │ fs/inode.c                     │ Hash Table (chaining) + LRU                      │
│ ext4 directory index (htree) │ fs/ext4/dir.c, htree.c         │ Hash B-Tree (HTree)                              │
│ Btrfs                        │ fs/btrfs/ctree.c               │ Copy-on-Write B+ Tree                            │
│ XFS                          │ fs/xfs/libxfs/xfs_btree.c      │ B+ Tree (multi-level)                            │
│ FIB / Route Lookup           │ net/ipv4/fib_trie.c            │ LC-Trie (level-compressed trie, LPM)             │
│ IPv6 Routing                 │ net/ipv6/ip6_fib.c             │ Radix Tree                                       │
│ Conntrack / Netfilter        │ net/netfilter/nf_conntrack*.c  │ Hash Table (RCU-protected)                       │
│ TCP Receive Buffer           │ net/ipv4/tcp_input.c           │ Sliding Window + Ring Buffer (sk_buff)           │
│ Packet Queue (sk_buff)       │ include/linux/skbuff.h         │ Doubly Linked List                               │
│ eBPF: Hash Map               │ kernel/bpf/hashtab.c           │ Hash Table + LRU variant                         │
│ eBPF: LPM Trie Map           │ kernel/bpf/lpm_trie.c          │ Longest Prefix Match Trie                        │
│ eBPF: Ring Buffer            │ kernel/bpf/ringbuf.c           │ Lock-free Ring Buffer                            │
│ eBPF: Queue / Stack Map      │ kernel/bpf/queue_stack_maps.c  │ Ring Buffer (queue/stack semantics)              │
│ io_uring                     │ io_uring/io_uring.c            │ Ring Buffer (SQ + CQ pair)                       │
│ Block I/O (deadline)         │ block/mq-deadline.c            │ Red-Black Tree (deadline-keyed)                  │
│ kfifo                        │ include/linux/kfifo.h          │ Power-of-2 Ring Buffer (lock-free single prod)   │
│ Lockdep                      │ kernel/locking/lockdep.c       │ Directed Graph + DFS Cycle Detection             │
│ RCU                          │ kernel/rcu/tree.c              │ Lock-free Linked List + Quiescent State tracking │
│ Epoll                        │ fs/eventpoll.c                 │ Red-Black Tree + Wait Queue                      │
│ lib/rbtree.c                 │ lib/rbtree.c                   │ Red-Black Tree (generic)                         │
│ lib/radix-tree.c / XArray    │ lib/radix-tree.c, lib/xarray.c │ Radix Tree / XArray                              │
│ include/linux/list.h         │ include/linux/list.h           │ Circular Doubly Linked List (intrusive)          │
│ lib/bitmap.c                 │ lib/bitmap.c                   │ Bitmap (SIMD-accelerated bit arrays)             │
│ lib/sort.c                   │ lib/sort.c                     │ Heapsort (in-place, cache-aware)                 │
│ lib/bsearch.c                │ lib/bsearch.c                  │ Binary Search                                    │
│ Module Loader                │ kernel/module/main.c           │ Topological Sort (dependency graph)              │
│ Cgroup Hierarchy             │ kernel/cgroup/cgroup.c         │ Tree + DFS traversal                             │
│ Device Tree (OF)             │ drivers/of/base.c              │ Rooted Tree + BFS/DFS                            │
│ dm-verity                    │ drivers/md/dm-verity.c         │ Merkle Tree                                      │
+──────────────────────────────+────────────────────────────────+──────────────────────────────────────────────────+
```

---

## PART 2 — Problems by DSA Concept

---

### 2.1 Red-Black Tree

**Kernel Context**
- `kernel/sched/fair.c` — CFS picks the task with minimum `vruntime`; leftmost node in the RB-tree always wins O(1) pick via `rb_first()`.
- `mm/mmap.c` — VMAs stored by start address; range overlap search uses augmented RB-tree (max end-addr per subtree).
- `fs/eventpoll.c` — epoll fd set stored in RB-tree for O(log N) insert/delete.
- `block/mq-deadline.c` — I/O requests stored by deadline in RB-tree.

**Why RB-Tree?** Guaranteed O(log N) insert/delete/lookup with bounded rotation count (≤2 for insert, ≤3 for delete). No balance factor storage needed (just 1-bit color). Augmentation (tracking subtree max) is straightforward.

**Foundational Problems**
| # | Problem | Platform | Difficulty | Kernel Relevance |
|---|---------|----------|------------|-----------------|
| 1 | Validate Binary Search Tree | LC-98 | 🟡 | Core BST invariant, basis of RB-tree |
| 2 | Insert into a Binary Search Tree | LC-701 | 🟢 | `rb_insert_color()` concept |
| 3 | Delete Node in a BST | LC-450 | 🟡 | `rb_erase()` concept |
| 4 | Kth Smallest Element in a BST | LC-230 | 🟡 | In-order traversal, CFS pick_next |
| 5 | Balance a BST | LC-1382 | 🟡 | Rebalancing after operations |
| 6 | LCA of a Binary Search Tree | LC-235 | 🟡 | Ancestor queries on BST |
| 7 | Self Balancing Tree (Insertion) | HR | 🟡 | Direct BST height tracking |

**Augmented / Advanced (VMA interval augmentation)**
| # | Problem | Platform | Difficulty | Kernel Relevance |
|---|---------|----------|------------|-----------------|
| 8 | Count of Smaller Numbers After Self | LC-315 ★ | 💀 | Augmented RB-tree rank queries; same as VMA search |
| 9 | Count of Range Sum | LC-327 | 💀 | Range queries on BST structure |
| 10 | Reverse Pairs | LC-493 | 💀 | Merge sort / BST order-statistic tree |
| 11 | The Skyline Problem | LC-218 | 💀 | Multiset-based sweep, augmented BST |

---

### 2.2 Interval Tree

**Kernel Context**
- `mm/interval_tree.c` — Stores VMAs as `[vm_start, vm_end)` intervals; overlap search is O(log N + k).
- `include/linux/interval_tree_generic.h` — Generic augmented RB-tree macro for interval trees.
- The `INTERVAL_TREE_DEFINE()` macro in `include/linux/interval_tree_generic.h` generates the whole interval tree from a base RB-tree.

**Why Interval Tree?** Finding all VMAs that overlap `[query_start, query_end)` is O(log N) average, critical for `mmap()`, `munmap()`, `mprotect()`, and page-fault handling.

```
  Interval Tree (VMAs):
  
       [0x1000, 0x3000)        ← RB node, max_end = 0x8000 (subtree)
      /                  \
  [0x400, 0x1000)      [0x5000, 0x8000)
  max_end=0x1000       max_end=0x8000
  
  Query [0x600, 0x1500): start at root
  - root.max_end (0x8000) >= 0x600 → explore left
  - [0x400, 0x1000) overlaps? start<1500 && end>600 → YES
  - right child: start(0x5000) > 0x1500 → prune
```

| # | Problem | Platform | Difficulty | Kernel Relevance |
|---|---------|----------|------------|-----------------|
| 1 | Merge Intervals | LC-56 | 🟡 | `mmap()` VMA coalescing after adjacent maps |
| 2 | Insert Interval | LC-57 | 🟡 | `do_mmap()` inserting new VMA |
| 3 | Meeting Rooms II | LC-253 | 🟡 | Peak overlap count (page fault pressure) |
| 4 | Data Stream as Disjoint Intervals | LC-352 | 💀 | Online VMA insertion + merge |
| 5 | Range Module ★ | LC-715 | 💀 | **Best LC analog to interval tree** — add/remove/query intervals |
| 6 | My Calendar III | LC-732 | 💀 | Segment tree w/ lazy or interval tree — booking overlap count |
| 7 | Minimum Interval to Include Each Query | LC-1851 | 💀 | Offline interval query, close to mprotect range queries |
| 8 | Count Integers in Intervals | LC-2276 | 💀 | Online interval union, direct VMA union tracking |
| 9 | Falling Squares | LC-699 | 💀 | Interval merge with height tracking |
| 10 | My Calendar I | LC-729 | 🟡 | Non-overlapping interval insert |
| 11 | My Calendar II | LC-731 | 🟡 | At most 2 overlaps — page sharing limit concept |
| 12 | Interval Tree (insert + overlap query) | GFG | 🟡 | Direct implementation exercise |

---

### 2.3 Radix Tree / XArray / Trie

**Kernel Context**
- `lib/radix-tree.c`, `lib/xarray.c` — Page cache: maps `page_index` (u64) → `struct page*`. XArray is the modern replacement (v4.20+). Lookup is O(k) where k = log_{64}(max_index) ≈ 6–11 levels.
- `lib/idr.c` — Maps integer IDs (file descriptors, PIDs, IPC IDs) → pointers. Built on radix tree.
- `net/ipv6/ip6_fib.c` — IPv6 route table: Patricia trie / compressed radix trie.
- `kernel/bpf/lpm_trie.c` — `BPF_MAP_TYPE_LPM_TRIE`: stores IP prefixes for XDP routing decisions.

**Why Radix/Trie?** O(k) lookup/insert (k = key length, independent of N). Space-efficient path compression. Prefix queries are native. Radix trees excel for sparse integer → pointer mappings.

```
  Radix Tree (XArray) — Page Cache:
  
  Root (64-slot)
  ├── slot[0] → internal node (64-slot)
  │   ├── slot[0] → page @ index 0
  │   └── slot[1] → page @ index 1
  └── slot[1] → internal node
      └── slot[0] → page @ index 64
  
  LPM Trie — BPF_MAP_TYPE_LPM_TRIE:
  
  10.0.0.0/8    → route A
  10.128.0.0/9  → route B    (more specific, wins LPM)
  192.168.1.0/24 → route C
  
  Binary Trie:
  0 → ...
  1 → 0 (10.x.x.x matched) → ...
       1 → 0 (10.0.0.0/8 terminates here)
           ...
```

**Trie Foundations**
| # | Problem | Platform | Difficulty | Kernel Relevance |
|---|---------|----------|------------|-----------------|
| 1 | Implement Trie (Prefix Tree) ★ | LC-208 | 🟡 | Core trie structure — XArray node concept |
| 2 | Design Add and Search Words DS | LC-211 | 🟡 | Wildcard search — like glob in sysfs paths |
| 3 | Map Sum Pairs | LC-677 | 🟡 | Prefix aggregation |
| 4 | Replace Words | LC-648 | 🟡 | Prefix replacement — routing table compression |
| 5 | Stream of Characters | LC-1032 | 💀 | Aho-Corasick on trie — multi-pattern match |
| 6 | Prefix and Suffix Search | LC-745 | 💀 | Compound trie — like multi-key radix tree |
| 7 | Implement Magic Dictionary | LC-676 | 🟡 | Approximate trie search |
| 8 | Word Search II | LC-212 | 💀 | Trie + DFS — file path glob expansion |

**Binary Trie (LPM / XOR)**
| # | Problem | Platform | Difficulty | Kernel Relevance |
|---|---------|----------|------------|-----------------|
| 9 | Maximum XOR of Two Numbers ★ | LC-421 | 🟡 | **Binary trie = BPF LPM trie structure** |
| 10 | Count Pairs With XOR in a Range | LC-1803 | 💀 | Range query on binary trie |
| 11 | Longest Prefix Matching (Routing Tables) | GFG | 🟡 | **Direct IP routing LPM implementation** |
| 12 | Compressed Trie / Patricia Trie | GFG | 🟡 | Path-compressed trie = kernel's fib_trie |
| 13 | String Hashing / Pattern Matching | CSES | 🟡 | Trie-based pattern matching in kernel ftrace |

---

### 2.4 Hash Table

**Kernel Context**
- `fs/dcache.c` — dentry cache: `dcache_hash()` → `hlist_bl_head` chains. Up to millions of dentries, hash table prevents O(N) path lookups.
- `fs/inode.c` — inode cache: `inode_hash()` → `hlist_head` chains.
- `net/netfilter/nf_conntrack_core.c` — Connection tracking: 5-tuple (src/dst IP, port, proto) → conntrack entry.
- `include/linux/hashtable.h` — Generic macro-based hash table with `hlist` chains.
- `kernel/pid.c` — PID → task mapping via hash table.

**Why Hash Table?** O(1) average lookup/insert. The kernel uses open-addressing for some cases and chaining (hlist) for most. Load factor and hash function choice are critical for worst-case behavior.

| # | Problem | Platform | Difficulty | Kernel Relevance |
|---|---------|----------|------------|-----------------|
| 1 | Design HashSet | LC-705 | 🟢 | Understand basic chaining/open addressing |
| 2 | Design HashMap ★ | LC-706 | 🟢 | `DEFINE_HASHTABLE()` from include/linux/hashtable.h |
| 3 | Insert Delete GetRandom O(1) | LC-380 | 🟡 | Hash + array trick — inode cache with O(1) random evict |
| 4 | Insert Delete GetRandom O(1) Duplicates | LC-381 | 💀 | Multi-value hash entry — conntrack multipath |
| 5 | Maximum Frequency Stack ★ | LC-895 | 💀 | Hash + frequency map — slab per-cpu cache structures |
| 6 | All O`one Data Structure | LC-432 | 💀 | Hash + DLL — LRU + frequency combined (dcache evict) |
| 7 | Encode and Decode TinyURL | LC-535 | 🟡 | Hash function design — `jhash()` in kernel |
| 8 | Contacts | HR | 🟡 | Trie / Hash — autocomplete (sysfs path lookup) |
| 9 | No Prefix Set | HR | 🟡 | Prefix collision detection in hash tables |
| 10 | Pairs | HR | 🟡 | Hash set lookup — O(1) conntrack reverse lookup |

---

### 2.5 LRU / LFU Cache

**Kernel Context**
- `mm/workingset.c`, `mm/vmscan.c` — Page frame reclaim: LRU lists (active/inactive anon + file). The kernel maintains 5 LRU lists per NUMA node; `kswapd` evicts from the tail.
- `fs/dcache.c` — LRU dentry eviction: `dentry_unused` list + hash table.
- `fs/inode.c` — inode LRU: `sb_mnt_count`, `inode_lru` list.

**Why LRU?** Temporal locality: recently used pages are more likely to be reused. Hash table provides O(1) lookup; doubly linked list provides O(1) move-to-front and tail eviction.

```
  Linux Page Cache LRU (simplified):
  
  [Active List head] ←→ pg3 ←→ pg1 ←→ [Active List tail]
  [Inactive List head] ←→ pg5 ←→ pg2 ←→ [Inactive List tail]
                                              ↑
                                         kswapd evicts from here
  
  Hash table: pfn/index → list node (O(1) lookup)
```

| # | Problem | Platform | Difficulty | Kernel Relevance |
|---|---------|----------|------------|-----------------|
| 1 | LRU Cache ★ | LC-146 | 🟡 | **EXACT** — page cache eviction with hash+DLL |
| 2 | LFU Cache ★ | LC-460 | 💀 | Frequency-based eviction — eBPF `BPF_MAP_TYPE_LRU_HASH` uses an enhanced version |
| 3 | All O`one Data Structure | LC-432 | 💀 | Ordered frequency with O(1) min/max — dcache scoring |
| 4 | Design Most Recently Used Queue | LC-1756 | 🟡 | MRU queue structure |
| 5 | LRU Cache Implementation | GFG | 🟡 | Step-by-step implementation with hash+DLL |
| 6 | Cache Eviction Simulation | GFG | 🟡 | Multi-policy simulation |

---

### 2.6 Ring Buffer (Circular Queue)

**Kernel Context**
- `include/linux/kfifo.h` — `kfifo`: lock-free single-producer/single-consumer ring buffer. Power-of-2 size lets mask replace modulo. Used in serial drivers, input subsystem, USB.
- `io_uring/io_uring.c` — Submission Queue (SQ) + Completion Queue (CQ): two independent ring buffers shared between kernel and userspace via mmap. Zero-copy I/O path.
- `kernel/bpf/ringbuf.c` — `BPF_MAP_TYPE_RINGBUF`: lock-free, variable-length record ring buffer. Shared via mmap for zero-copy from BPF to userspace.
- `kernel/events/ring_buffer.c` — perf ring buffer for sampling events.
- `drivers/net/` — NIC Rx/Tx descriptor rings (e.g., `igb_ring`, `ixgbe_ring`).

**Why Ring Buffer?** O(1) enqueue/dequeue, cache-friendly (contiguous memory), power-of-2 size enables branchless wrap-around with bitmask. No dynamic allocation in the hot path.

```
  kfifo layout (power-of-2 = 8 slots):
  
  head=3, tail=7, mask=0x7
  
  [0]  [1]  [2] [A] [B] [C] [D]  [7]
                 ↑               ↑
               head=3          tail=7
  
  enqueue: buf[tail & mask] = item; tail++;
  dequeue: item = buf[head & mask]; head++;
  empty: head == tail
  full:  (tail - head) == size
```

| # | Problem | Platform | Difficulty | Kernel Relevance |
|---|---------|----------|------------|-----------------|
| 1 | Design Circular Queue ★ | LC-622 | 🟡 | Direct `kfifo` analog (fixed-size, enqueue/dequeue) |
| 2 | Design Circular Deque ★ | LC-641 | 🟡 | Bidirectional kfifo — USB bulk transfer rings |
| 3 | Design Front Middle Back Queue | LC-1670 | 🟡 | Complex deque structure |
| 4 | Design Hit Counter | LC-362 (Premium) | 🟡 | Time-windowed ring buffer — perf event sampling window |
| 5 | Queue Using Two Stacks | HR | 🟢 | Buffer management fundamentals |
| 6 | Deque Implementations | HR | 🟢 | Double-ended ring buffer |

---

### 2.7 Skip List

**Kernel Context**
- Not in mainline Linux kernel (kernel uses RB-trees instead).
- Used in LevelDB (RocksDB) which is used alongside kernel-space tools.
- Understanding skip lists deepens probabilistic data structure intuition relevant to BPF verifier analysis.

| # | Problem | Platform | Difficulty | Kernel Relevance |
|---|---------|----------|------------|-----------------|
| 1 | Design Skiplist ★ | LC-1206 | 💀 | Only direct LC skip list problem; implement search/add/erase |

---

### 2.8 Priority Queue / Heap

**Kernel Context**
- `kernel/sched/rt.c` — RT scheduler uses a bitmap-indexed priority array (140 levels), which is an O(1) priority queue with bounded N. `sched_find_first_bit()` for O(1) dequeue.
- `kernel/time/hrtimer.c` — High-resolution timers stored in a timerqueue (augmented RB-tree), not a raw heap — but the semantics are identical to a min-heap.
- `kernel/sched/deadline.c` — EDF (Earliest Deadline First): always run the task with the nearest absolute deadline → min-heap by deadline.
- `block/mq-deadline.c` — I/O request deadline scheduler: prioritizes requests approaching their deadline.
- `net/core/sock.c` — Socket priority queues.

**Why Heap?** O(log N) insert/extract-min. When "always process the most urgent item" is required (deadlines, timers, RT tasks), a heap or heap-like structure is the natural choice.

| # | Problem | Platform | Difficulty | Kernel Relevance |
|---|---------|----------|------------|-----------------|
| 1 | Kth Largest Element in a Stream | LC-703 | 🟢 | Min-heap fundamentals — RT priority tracking |
| 2 | Last Stone Weight | LC-1046 | 🟢 | Max-heap — greedy with priority |
| 3 | Merge K Sorted Lists | LC-23 | 💀 | k-way heap merge — log-structured storage merge |
| 4 | Find Median from Data Stream | LC-295 | 💀 | Two-heap median — load balancer median tracking |
| 5 | The Skyline Problem | LC-218 | 💀 | Heap + sweep — timer wheel event processing |
| 6 | Top K Frequent Elements | LC-347 | 🟡 | Min-heap of size K — top K hot pages |
| 7 | Task Scheduler ★ | LC-621 | 🟡 | **CPU scheduling simulation** — CFS/RT cooldown concept |
| 8 | Course Schedule III ★ | LC-630 | 💀 | Deadline scheduling — greedy + max-heap (EDF concept) |
| 9 | Design Twitter | LC-355 | 🟡 | Heap merge of per-user timelines — I/O queue merging |
| 10 | IPO (Project Selection) | LC-502 | 💀 | Two-heap greedy — resource allocation under constraint |
| 11 | Furthest Building You Can Reach | LC-1642 | 🟡 | Greedy + min-heap — memory pressure decision |
| 12 | Minimize Deviation in Array | LC-1675 | 💀 | Heap + normalization |
| 13 | QHEAP1 | HR | 🟢 | Direct heap operations — min/max priority queue |
| 14 | Jesse and Cookies | HR | 🟡 | Greedy + min-heap |
| 15 | Find the Running Median ★ | HR | 💀 | Two-heap median — dynamic load tracking |
| 16 | Tasks and Deadlines ★ | CSES | 🟡 | **EDF deadline scheduling** — direct `SCHED_DEADLINE` analog |
| 17 | Restaurant Customers | CSES | 🟡 | Sweep + priority queue |
| 18 | Minimum Spanning Tree (Prim's) | CSES | 🟡 | Heap-based graph algorithm |

---

### 2.9 Segment Tree

**Kernel Context**
- Not directly in kernel code, but the concepts appear in:
  - `kernel/bpf/`: eBPF range-indexed array maps with per-range updates conceptually use segment-tree semantics.
  - `drivers/gpu/drm/`: GPU memory range management uses interval trees that share segment tree query patterns.
  - Tracing/perf: range-based histogram buckets.
- Segment trees are a foundational structure for building all other range query data structures.

| # | Problem | Platform | Difficulty | Kernel Relevance |
|---|---------|----------|------------|-----------------|
| 1 | Range Sum Query - Mutable ★ | LC-307 | 🟡 | Core segment tree — range query + point update |
| 2 | Range Sum Query 2D - Mutable | LC-308 | 💀 | 2D segment tree — GPU texture memory ranges |
| 3 | Range Module ★ | LC-715 | 💀 | Segment tree w/ lazy prop — interval insert/delete/query |
| 4 | My Calendar III | LC-732 | 💀 | Lazy propagation — scheduling conflict detection |
| 5 | Falling Squares | LC-699 | 💀 | Segment tree w/ coordinate compression |
| 6 | Count of Smaller Numbers After Self | LC-315 | 💀 | Merge sort / segment tree — augmented RB-tree query |
| 7 | Count of Range Sum | LC-327 | 💀 | Segment tree over prefix sums |
| 8 | Reverse Pairs | LC-493 | 💀 | Merge sort / BIT / segment tree |
| 9 | Dynamic Range Minimum Queries ★ | CSES | 🟡 | **Core segment tree problem** — point update, range min |
| 10 | Dynamic Range Sum Queries | CSES | 🟡 | Point update, range sum |
| 11 | Range Update Queries ★ | CSES | 🟡 | **Lazy propagation** — bulk memory range updates |
| 12 | Hotel Queries | CSES | 🟡 | Segment tree binary search |
| 13 | Polynomial Queries | CSES | 💀 | Complex lazy propagation |
| 14 | HORRIBLE (Horrible Queries) ★ | SPOJ | 💀 | **Lazy propagation** — canonical problem |
| 15 | GSS1 (Can You Answer These Queries I) | SPOJ | 💀 | Max subarray sum with segment tree |
| 16 | LITE (Light Switching) | SPOJ | 🟡 | Lazy flip segment tree |
| 17 | MULTQ3 | SPOJ | 🟡 | Modular segment tree queries |

---

### 2.10 Fenwick Tree (Binary Indexed Tree / BIT)

**Kernel Context**
- `kernel/events/` — perf subsystem uses counter arrays that can conceptually be BIT-queried for prefix-sum histograms.
- Bitmap with POPCOUNT: `lib/bitmap.c` + `hweight()` calls are O(N/word) prefix popcount, which BIT optimizes to O(log N).
- Used in eBPF programs for histogram summarization.

| # | Problem | Platform | Difficulty | Kernel Relevance |
|---|---------|----------|------------|-----------------|
| 1 | Range Sum Query - Mutable | LC-307 | 🟡 | BIT solution — perf counter range queries |
| 2 | Count of Smaller Numbers After Self | LC-315 | 💀 | BIT with coordinate compression |
| 3 | Count of Range Sum | LC-327 | 💀 | BIT over prefix sums |
| 4 | Reverse Pairs | LC-493 | 💀 | BIT inversion count |
| 5 | Salary Queries ★ | CSES | 🟡 | BIT + coordinate compression — classic perf histogram |
| 6 | Forest Queries | CSES | 🟡 | 2D BIT — GPU tile/region counters |
| 7 | List Removals | CSES | 🟡 | BIT for finding k-th element after removals |
| 8 | Counting Inversions | HR | 🟡 | BIT inversion count — cache line ordering analysis |
| 9 | KQUERY | SPOJ | 💀 | Offline BIT — K-th element queries |

---

### 2.11 Bitmap / Bit Operations

**Kernel Context**
- `include/linux/cpumask.h` — `cpu_mask`, `cpumask_t`: bitmaps for CPU affinity. `cpumask_set_cpu()`, `cpumask_first()`, `cpumask_next_zero()`.
- `include/linux/bitmap.h`, `lib/bitmap.c` — Generic bitmap ops: `bitmap_and()`, `bitmap_or()`, `bitmap_weight()` (POPCOUNT), `bitmap_find_next_zero_area()`.
- `mm/page_alloc.c` — Buddy allocator free-area bitmaps. `free_list[order]` tracks free blocks.
- `kernel/sched/rt.c` — `sched_find_first_bit()` over 140-bit priority bitmap → O(1) highest-priority task.
- `kernel/irq/bitmap.c` — IRQ affinity masks.
- `lib/find_bit.c` — `find_first_bit()`, `find_next_bit()`, `find_first_zero_bit()` — critical hot paths.

**Why Bitmaps?** Dense boolean arrays. SIMD-accelerated AND/OR/XOR. `find_first_bit()` uses hardware BSF/LZCNT instructions (≤64 bits per word). Cache-line-sized ops for CPU masks.

| # | Problem | Platform | Difficulty | Kernel Relevance |
|---|---------|----------|------------|-----------------|
| 1 | Number of 1 Bits ★ | LC-191 | 🟢 | `hweight32()`/`hweight64()` — cpumask population count |
| 2 | Counting Bits | LC-338 | 🟢 | `bitmap_weight()` across a range |
| 3 | Power of Two ★ | LC-231 | 🟢 | `is_power_of_2()` — buddy allocator order check |
| 4 | Reverse Bits | LC-190 | 🟢 | Bit reversal — network byte order conversions |
| 5 | Hamming Distance | LC-461 | 🟢 | XOR + popcount — cpu_mask diff |
| 6 | Single Number II | LC-137 | 🟡 | Bit counting trick |
| 7 | Bitwise AND of Numbers Range | LC-201 | 🟡 | Common prefix bits — buddy block alignment |
| 8 | Maximum XOR of Two Numbers | LC-421 | 🟡 | XOR + binary trie — `jhash()` design |
| 9 | Maximum Product of Word Lengths | LC-318 | 🟡 | Bitmask intersection — cpumask AND |
| 10 | Minimum Flips to Make a OR b Equal to c | LC-1318 | 🟡 | Bit-level diff counting |
| 11 | Sum of Two Integers | LC-371 | 🟡 | Bit arithmetic without + operator |
| 12 | Find position of only set bit | GFG | 🟢 | `ffs()` / `__builtin_ctz()` — Linux `find_first_bit()` |
| 13 | Count total set bits 1 to N | GFG | 🟡 | `bitmap_weight()` prefix sum |
| 14 | Next Power of 2 | GFG | 🟢 | `roundup_pow_of_two()` — slab/kfifo sizing |

---

### 2.12 Doubly Linked List (include/linux/list.h)

**Kernel Context**
- `include/linux/list.h` — Intrusive circular doubly linked list. Every kernel structure embeds a `struct list_head` (two pointers). Constant-overhead regardless of element size. Used in: task_struct (process list), dentry LRU, sk_buff queue, work_struct, module list, device list, etc.
- `include/linux/llist.h` — Lock-free singly linked list (CAS-based).
- Intrusive design: no allocation per node; the node is inside the object. `container_of()` recovers the containing struct.

```
  struct list_head { struct list_head *next, *prev; };
  
  [list_head] ←→ [task_struct.tasks] ←→ [task_struct.tasks] ←→ [list_head]
  (init_task)                                                    (init_task)
  
  container_of(ptr, struct task_struct, tasks) → task pointer
```

| # | Problem | Platform | Difficulty | Kernel Relevance |
|---|---------|----------|------------|-----------------|
| 1 | LRU Cache ★ | LC-146 | 🟡 | Hash + DLL — exact page cache LRU pattern |
| 2 | LFU Cache | LC-460 | 💀 | Frequency-bucketed DLL — eBPF LRU |
| 3 | All O`one Data Structure | LC-432 | 💀 | Ordered DLL with hash — dcache eviction scoring |
| 4 | Max Stack | LC-716 | 🟡 | Doubly linked stack + auxiliary max tracking |
| 5 | Sort List ★ | LC-148 | 🟡 | Merge sort on linked list — list.h intrusive sort |
| 6 | Reverse Nodes in k-Group | LC-25 | 💀 | In-place list reversal — sk_buff list reordering |
| 7 | Reverse Linked List II | LC-92 | 🟡 | Sublist reversal — packet buffer reorder |
| 8 | Merge K Sorted Lists | LC-23 | 💀 | k-way list merge — I/O completion list merge |
| 9 | Copy List with Random Pointer | LC-138 | 🟡 | Deep copy with backpointer — process clone |
| 10 | Flatten a Multilevel Doubly Linked List | LC-430 | 🟡 | Multilevel DLL — cgroup hierarchy flattening |
| 11 | Array and Simple Queries | HR | 🟡 | DLL-based rearrangement |

---

### 2.13 Topological Sort (Module Loading / Cgroup Controllers)

**Kernel Context**
- `kernel/module/main.c` — `load_module()`: kernel modules declare dependencies via `MODULE_SOFTDEP()`. On load, kernel resolves the dependency DAG and loads prerequisites first. On unload, reverse topological order.
- `kernel/cgroup/cgroup.c` — Cgroup controller initialization order follows dependency graph.
- `init/main.c` — `initcall` order: `early_initcall` → `core_initcall` → `arch_initcall` → ... → `late_initcall` is a hardcoded topological sort baked into linker sections.
- `tools/perf/`, eBPF CO-RE: BTF type dependency resolution.

| # | Problem | Platform | Difficulty | Kernel Relevance |
|---|---------|----------|------------|-----------------|
| 1 | Course Schedule ★ | LC-207 | 🟡 | Module dependency: can all modules load? (cycle check) |
| 2 | Course Schedule II ★ | LC-210 | 🟡 | Module loading order (topological ordering) |
| 3 | Alien Dictionary | LC-269 (Premium) | 💀 | Custom ordering — symbol versioning constraints |
| 4 | Minimum Height Trees | LC-310 | 🟡 | DAG centroid — dependency graph center |
| 5 | Sort Items by Groups Respecting Dependencies | LC-1203 | 💀 | Two-level topo sort — module groups with ordering |
| 6 | All Ancestors of a Node in a DAG | LC-2192 | 🟡 | Reverse dependency tracking — module `modinfo` |
| 7 | Find Eventual Safe States | LC-802 | 🟡 | DAG safe nodes — deadlock-free lock ordering |
| 8 | Course Schedule ★ | CSES | 🟡 | Canonical topo sort problem |
| 9 | Longest Flight Route | CSES | 🟡 | Longest path in DAG — critical dependency chain |
| 10 | Game Routes | CSES | 🟡 | Path count in DAG |

---

### 2.14 Cycle Detection (Lockdep)

**Kernel Context**
- `kernel/locking/lockdep.c` — **lockdep**: detects potential deadlocks at runtime. Builds a directed graph where nodes are lock classes and edges are "lock A held while acquiring lock B". Every new lock acquisition adds an edge; lockdep runs DFS to check for cycles. A cycle = potential deadlock. This is one of the most sophisticated uses of graph algorithms in any operating system.

```
  Lockdep Graph:
  
  spinlock_A → mutex_B → rwlock_C
                 ↑              ↓
              semaphore_D ←─────┘   ← CYCLE DETECTED → WARN_ON
  
  lockdep_check_circular_wait():
    DFS from each lock class
    If back-edge found → print lock dependency chain + stack trace
```

| # | Problem | Platform | Difficulty | Kernel Relevance |
|---|---------|----------|------------|-----------------|
| 1 | Course Schedule ★ | LC-207 | 🟡 | **DFS cycle detection** — direct lockdep equivalent |
| 2 | Find Eventual Safe States ★ | LC-802 | 🟡 | Nodes that can never reach a cycle — safe lock sequences |
| 3 | Redundant Connection | LC-684 | 🟡 | Union-Find cycle detection — undirected lock graph |
| 4 | Redundant Connection II | LC-685 | 💀 | Directed cycle with extra edge — lockdep directed graph |
| 5 | Round Trip (undirected cycle) | CSES | 🟡 | Undirected cycle exists? |
| 6 | Round Trip II (directed cycle) ★ | CSES | 🟡 | Directed cycle — **exact lockdep cycle check** |
| 7 | Cycle Detection in Directed Graph | GFG | 🟡 | DFS with 3-color marking — WHITE/GRAY/BLACK |
| 8 | Detect Cycle in Undirected Graph | GFG | 🟡 | Union-Find approach |
| 9 | Detect Negative Cycle (Bellman-Ford) | CF/GFG | 🟡 | Negative weight cycle — priority inversion detection |

---

### 2.15 Union-Find (Disjoint Set Union)

**Kernel Context**
- `kernel/locking/lockdep.c` — Lock class equivalence grouping: multiple `spinlock_t` instances that follow the same nesting order are placed in the same *class*. Conceptually, DSU tracks which lock instances belong to the same equivalence class.
- Network namespace merging, cgroup hierarchy equivalence checking.
- `drivers/gpu/drm/`: GPU memory region grouping.

| # | Problem | Platform | Difficulty | Kernel Relevance |
|---|---------|----------|------------|-----------------|
| 1 | Number of Provinces | LC-547 | 🟡 | DSU fundamentals — lock class grouping |
| 2 | Redundant Connection | LC-684 | 🟡 | Cycle via DSU |
| 3 | Redundant Connection II | LC-685 | 💀 | Directed graph + DSU |
| 4 | Accounts Merge ★ | LC-721 | 🟡 | Identity merging — network namespace consolidation |
| 5 | Satisfiability of Equality Equations | LC-990 | 🟡 | Equivalence classes — symbol alias resolution |
| 6 | Smallest String With Swaps | LC-1202 | 🟡 | DSU + sorting within components |
| 7 | Similar String Groups | LC-839 | 💀 | Component identification |
| 8 | Min Cost to Connect All Points | LC-1584 | 🟡 | Kruskal MST via DSU |
| 9 | Optimize Water Distribution | LC-1168 (Premium) | 💀 | DSU + virtual node MST |
| 10 | Building Teams | CSES | 🟡 | Bipartite check via DSU |
| 11 | Road Reparation | CSES | 🟡 | Kruskal + DSU — network topology repair |
| 12 | Union-Find with Rollback | CF Educational | 💀 | Persistent DSU — snapshot-and-rollback for lockdep history |

---

### 2.16 B-Tree / B+ Tree (File Systems)

**Kernel Context**
- `fs/ext4/` — `htree`: B-tree-based directory indexing. Hashes filenames, stores in multi-level tree of 4KB blocks. O(log N) file lookup in large directories.
- `fs/btrfs/ctree.c` — Copy-on-Write B+ tree for ALL metadata: inodes, extents, checksums, subvolumes. The entire filesystem is one giant B-tree forest.
- `fs/xfs/libxfs/xfs_btree.c` — Multiple B+ trees per AG (allocation group): free space, inode, refcount.
- `fs/f2fs/` — B+ tree for SIT (Segment Information Table).

**Why B+ Tree?** Block-aligned nodes fit I/O blocks (4KB). All values at leaves (B+ tree) enables efficient range scans. High fanout (hundreds of keys per node) minimizes tree depth → few disk reads per lookup.

| # | Problem | Platform | Difficulty | Kernel Relevance |
|---|---------|----------|------------|-----------------|
| 1 | Insert into a Binary Search Tree | LC-701 | 🟢 | BST insertion — basis of B-tree node insert |
| 2 | Delete Node in a BST | LC-450 | 🟡 | BST deletion — B-tree merge/borrow |
| 3 | My Calendar I | LC-729 | 🟡 | Ordered interval insert — ext4 htree insert |
| 4 | My Calendar II | LC-731 | 🟡 | Multi-level ordered structure |
| 5 | My Calendar III | LC-732 | 💀 | Range overlap count — btrfs extent tree query |
| 6 | B-Tree Insertion | GFG ★ | 🟡 | **Direct B-tree implementation** |
| 7 | B+ Tree Insertion and Search | GFG ★ | 🟡 | **Direct B+ tree** — closest to Btrfs ctree |
| 8 | B-Tree Deletion | GFG | 💀 | Merge/borrow operations |
| 9 | Range Sum Query - Mutable | LC-307 | 🟡 | Conceptual: range queries on ordered structure |
| 10 | Count of Smaller Numbers After Self | LC-315 | 💀 | Order-statistic tree — B+ tree rank queries |

---

### 2.17 Heapsort (lib/sort.c)

**Kernel Context**
- `lib/sort.c` — `sort()`: used throughout the kernel (e.g., sorting IRQ affinity tables, symbol tables in `kallsyms`, module dependency ordering). Implements heapsort for O(N log N) worst-case guaranteed, O(1) extra space, no recursion — critical for kernel stack constraints.
- `scripts/kallsyms.c` — Sorts symbol table using `lib/sort.c`.

| # | Problem | Platform | Difficulty | Kernel Relevance |
|---|---------|----------|------------|-----------------|
| 1 | Sort an Array ★ | LC-912 | 🟡 | Implement heapsort — exact `lib/sort.c` scenario |
| 2 | Kth Largest Element in an Array | LC-215 | 🟡 | Quickselect / partial heap — `lib/sort.c` + partition |
| 3 | Top K Frequent Elements | LC-347 | 🟡 | Min-heap of size K |
| 4 | Sort List | LC-148 | 🟡 | Merge sort on linked list — `list_sort()` in kernel |
| 5 | Wiggle Sort II | LC-324 | 🟡 | Partial sort — quickselect based |

---

### 2.18 Binary Search (lib/bsearch.c)

**Kernel Context**
- `lib/bsearch.c` — Generic `bsearch()`: used in IRQ chip lookup, CPU feature flag lookup, ACPI table search, symbol table lookup in ftrace/kprobes.
- `arch/x86/kernel/cpu/`: CPU feature flag sorted table lookups.
- `kernel/kallsyms.c` — `kallsyms_lookup_name()`: binary search over sorted symbol address table.

| # | Problem | Platform | Difficulty | Kernel Relevance |
|---|---------|----------|------------|-----------------|
| 1 | Binary Search ★ | LC-704 | 🟢 | `bsearch()` — `kallsyms_lookup_name()` equivalent |
| 2 | Search in Rotated Sorted Array | LC-33 | 🟡 | Rotated search — IRQ affinity range lookups |
| 3 | Find First and Last Position | LC-34 | 🟡 | Lower/upper bound — `lower_bound()` for range queries |
| 4 | Find Minimum in Rotated Sorted Array | LC-153 | 🟡 | Binary search on rotated data |
| 5 | Find Peak Element | LC-162 | 🟡 | Binary search on non-monotone function |
| 6 | Median of Two Sorted Arrays | LC-4 | 💀 | Binary search on answer — NUMA balancing thresholds |
| 7 | Capacity to Ship Packages Within D Days | LC-1011 | 🟡 | Binary search on answer — I/O bandwidth allocation |
| 8 | Split Array Largest Sum | LC-410 | 💀 | Binary search on answer — IRQ affinity band splitting |
| 9 | Koko Eating Bananas | LC-875 | 🟡 | Rate binary search — token bucket rate finding |
| 10 | Search a 2D Matrix | LC-74 | 🟡 | 2D binary search — page table multi-level lookup |
| 11 | Time-Based Key-Value Store | LC-981 | 🟡 | Binary search over timestamps — snapshot/versioned lookup |

---

### 2.19 Sliding Window / Two Pointer (TCP, Network Buffers)

**Kernel Context**
- `net/ipv4/tcp_input.c` — TCP receive window: tracks `rcv_wnd` (window size), `rcv_nxt` (next expected), `rcv_wup` (last ACKed). Sliding window controls how much data sender can have in flight.
- `net/ipv4/tcp_output.c` — Congestion window (`snd_cwnd`): exponentially grown, then additively increased. AIMD algorithm is a sliding window controller.
- `net/core/skbuff.c` — sk_buff data pointer + length: `skb_pull()`, `skb_push()` slide a window over a contiguous buffer.
- `kernel/events/ring_buffer.c` — perf ring buffer read/write positions (head/tail) implement a sliding window over events.

| # | Problem | Platform | Difficulty | Kernel Relevance |
|---|---------|----------|------------|-----------------|
| 1 | Sliding Window Maximum ★ | LC-239 | 💀 | **TCP receive window max tracking** — monotonic deque |
| 2 | Minimum Window Substring | LC-76 | 💀 | Minimum covering window — packet coalescing |
| 3 | Permutation in String | LC-567 | 🟡 | Fixed-size window — packet sequence detection |
| 4 | Find All Anagrams in a String | LC-438 | 🟡 | Sliding pattern match |
| 5 | Longest Substring Without Repeating Characters | LC-3 | 🟡 | Variable window — duplicate packet detection |
| 6 | Longest Subarray with Abs Diff ≤ Limit | LC-1438 | 🟡 | Monotonic deque — TCP window bounds |
| 7 | Max Consecutive Ones III | LC-1004 | 🟡 | Window with budget — packet drop budget |
| 8 | Fruit Into Baskets | LC-904 | 🟡 | 2-type sliding window |
| 9 | Fraudulent Activity Notifications ★ | HR | 🟡 | Sliding window + counting sort — TCP RENO detection |
| 10 | Minimum Size Subarray Sum | LC-209 | 🟡 | Two pointer — MTU-constrained segment sizing |

---

### 2.20 Monotonic Stack / Deque

**Kernel Context**
- Monotonic stacks appear in kernel code implicitly in scheduler preemption decisions and in XDP program analysis (verifier enforces structured control flow, which the verifier tracks with a stack of states).
- `net/sched/sch_sfq.c` — Stochastic Fair Queuing: monotone queue for fairness.
- `arch/x86/kernel/`: stack unwinding — literal call stack, but with monotone depth properties.

| # | Problem | Platform | Difficulty | Kernel Relevance |
|---|---------|----------|------------|-----------------|
| 1 | Largest Rectangle in Histogram ★ | LC-84 | 💀 | Monotonic stack — memory fragmentation analysis |
| 2 | Maximal Rectangle | LC-85 | 💀 | 2D monotonic stack |
| 3 | Trapping Rain Water | LC-42 | 🟡 | Two-pointer / monotonic stack — memory pressure |
| 4 | Sliding Window Maximum | LC-239 | 💀 | Monotonic deque — TCP window max |
| 5 | Sum of Subarray Minimums | LC-907 | 🟡 | Contribution counting + monotonic stack |
| 6 | Sum of Subarray Ranges | LC-2104 | 🟡 | Extension of LC-907 |
| 7 | Daily Temperatures | LC-739 | 🟡 | Next greater element — next preemption point |
| 8 | Final Prices With a Special Discount | LC-1475 | 🟢 | Next smaller element |

---

### 2.21 Graph Algorithms (Lockdep, Cgroup, Device Tree)

**Kernel Context**
- **BFS**: `kernel/cgroup/cgroup.c` — cgroup_for_each_live_child() traverses cgroup tree by level.
- **DFS**: `kernel/locking/lockdep.c` — cycle detection. `drivers/of/base.c` — device tree node enumeration.
- **Dijkstra**: Not directly in kernel, but NUMA topology path distances (`arch/x86/mm/numa.c`) use shortest-path semantics.
- **SCC (Strongly Connected Components)**: lockdep safe-order analysis.
- **BFS on grid**: `mm/compaction.c` — memory compaction scans PFN ranges.

| # | Problem | Platform | Difficulty | Kernel Relevance |
|---|---------|----------|------------|-----------------|
| 1 | Number of Islands | LC-200 | 🟡 | BFS/DFS — NUMA node island detection |
| 2 | Word Ladder | LC-127 | 💀 | BFS shortest path — module dependency shortest path |
| 3 | Pacific Atlantic Water Flow | LC-417 | 🟡 | Multi-source BFS — interrupt affinity propagation |
| 4 | 01 Matrix | LC-542 | 🟡 | Multi-source BFS — NUMA distance matrix |
| 5 | Shortest Path in Binary Matrix | LC-1091 | 🟡 | BFS grid — memory map traversal |
| 6 | Network Delay Time (Dijkstra) | LC-743 | 🟡 | NUMA inter-node latency minimum path |
| 7 | Path With Minimum Effort | LC-1631 | 🟡 | Dijkstra variant — PCIe bus route optimization |
| 8 | Cheapest Flights Within K Stops | LC-787 | 🟡 | Bellman-Ford / BFS |
| 9 | Critical Connections in a Network | LC-1192 | 💀 | Tarjan bridges — network single-point-of-failure |
| 10 | Strongly Connected Components (Tarjan/Kosaraju) | GFG ★ | 💀 | **lockdep SCC analysis** — equivalent lock class groups |
| 11 | Bipartite Graph Check | LC-785 | 🟡 | 2-colorability — lock ordering constraint satisfaction |
| 12 | Shortest Routes I (Dijkstra) | CSES | 🟡 | Canonical Dijkstra implementation |
| 13 | Shortest Routes II (Floyd-Warshall) | CSES | 🟡 | All-pairs — NUMA distance table computation |
| 14 | Cycle Finding (Bellman-Ford) | CSES | 🟡 | Negative cycle — priority inversion in RT scheduling |
| 15 | Giant Pizza (2-SAT) | CSES | 💀 | 2-SAT via SCC — lock ordering 2-SAT reduction |

---

### 2.22 Tree Algorithms (Cgroup Hierarchy / Device Tree / Sysfs)

**Kernel Context**
- `kernel/cgroup/cgroup.c` — cgroup tree: CSS (cgroup subsystem state) propagation uses DFS post-order traversal (`cgroup_apply_control()`).
- `drivers/of/base.c` — Open Firmware device tree: `of_find_node_by_name()`, `for_each_child_of_node()` — BFS and DFS traversals.
- `fs/sysfs/`, `lib/kobject.c` — kobject hierarchy: parent→child tree exposed via sysfs.
- Tree DP appears in cgroup resource accounting (aggregate CPU/memory usage from leaves upward).

| # | Problem | Platform | Difficulty | Kernel Relevance |
|---|---------|----------|------------|-----------------|
| 1 | Binary Tree Level Order Traversal | LC-102 | 🟡 | BFS — cgroup_for_each_live_child() |
| 2 | Binary Tree Postorder Traversal | LC-145 | 🟡 | Post-order — cgroup teardown (children before parent) |
| 3 | Diameter of Binary Tree | LC-543 | 🟢 | Tree diameter — longest device chain |
| 4 | Maximum Depth of Binary Tree | LC-104 | 🟢 | Tree depth — cgroup nesting depth limit (max 16) |
| 5 | Lowest Common Ancestor | LC-236 | 🟡 | LCA — cgroup common ancestor for resource accounting |
| 6 | Path Sum II | LC-113 | 🟡 | Root-to-leaf path — sysfs path enumeration |
| 7 | Serialize and Deserialize Binary Tree | LC-297 | 💀 | DT blob serialization — device tree DTB format |
| 8 | Count Good Nodes in Binary Tree | LC-1448 | 🟡 | DFS with running max — cgroup watermark propagation |
| 9 | Binary Tree Cameras | LC-968 | 💀 | Tree DP (greedy) — optimal cgroup monitoring placement |
| 10 | Subordinates ★ | CSES | 🟢 | Subtree size — cgroup population count |
| 11 | Tree Diameter ★ | CSES | 🟡 | Tree diameter — longest kernel symbol dependency chain |
| 12 | Tree Distances I | CSES | 🟡 | Distance from each node — NUMA distance propagation |
| 13 | Company Queries I & II (LCA) | CSES | 🟡 | LCA with binary lifting — deep cgroup hierarchy lookup |
| 14 | Distance Queries (LCA) | CSES | 🟡 | LCA-based distance |

---

### 2.23 Buddy System (mm/page_alloc.c)

**Kernel Context**
- `mm/page_alloc.c` — Buddy allocator: free memory organized as free_area[0..MAX_ORDER-1]. `alloc_pages(order)` finds free block of `2^order` pages; coalesces adjacent ("buddy") blocks on free. Order = 0 to 10 (4KB to 4MB blocks).

```
  Buddy System Free Lists:
  
  order 0:  [4KB] [4KB]
  order 1:  [8KB]
  order 2:  [16KB] [16KB]
  
  alloc(order=1):
    take from order-1 list → 8KB block
  
  alloc(order=0) when order-0 is empty:
    split order-1 block → two 4KB buddies
    return one, put other in order-0 free list
  
  free(4KB at addr X, order=0):
    compute buddy addr: X XOR (1 << (order * PAGE_SHIFT))
    if buddy is free: coalesce, promote to order-1
    repeat up the order chain
```

| # | Problem | Platform | Difficulty | Kernel Relevance |
|---|---------|----------|------------|-----------------|
| 1 | Power of Two | LC-231 | 🟢 | `is_power_of_2()` — buddy order check |
| 2 | Merge Intervals | LC-56 | 🟡 | Free block coalescing — `__free_one_page()` |
| 3 | Insert Interval | LC-57 | 🟡 | Inserting freed block into free list |
| 4 | Range Module | LC-715 | 💀 | Interval union/split — exact buddy alloc/free |
| 5 | Falling Squares | LC-699 | 💀 | Block splitting and stacking |
| 6 | Avoid Flood in The City | LC-1488 | 🟡 | Allocate/free lake slots — buddy free pool reuse |
| 7 | Segment Tree with Lazy Propagation | SPOJ HORRIBLE | 💀 | Range split/merge operations — buddy coalesce |
| 8 | Memory Allocator Simulation | GFG | 🟡 | Direct buddy allocator simulation |

---

### 2.24 Timer Wheel (kernel/time/timer_wheel.c)

**Kernel Context**
- `kernel/time/timer_wheel.c` (older: `kernel/time/timer.c`) — Hierarchical Timer Wheel: 5 cascading levels of buckets. Level 0: 256 buckets × 1 tick. Level 1: 64 buckets × 256 ticks. Etc. Timer insert: O(1). Timer fire: O(1) amortized (cascade is rare). Used for `mod_timer()`, `timer_list`, `delayed_work`.

```
  Hierarchical Timer Wheel:
  
  Level 0: [0][1][2]...[255]   ← 256 buckets, 1ms/bucket
  Level 1: [0][1]...[63]       ← 64 buckets, 256ms/bucket
  Level 2: [0][1]...[63]       ← 64 buckets, 16s/bucket
  Level 3: ...
  
  Timer fires at tick T:
  - index = T & 0xFF → Level-0 bucket
  - If Level-0 just wrapped: cascade Level-1 into Level-0
```

| # | Problem | Platform | Difficulty | Kernel Relevance |
|---|---------|----------|------------|-----------------|
| 1 | Design Circular Queue | LC-622 | 🟡 | Single-level wheel bucket — timer_wheel level 0 |
| 2 | Design Circular Deque | LC-641 | 🟡 | Bidirectional bucket operations |
| 3 | Time Based Key-Value Store | LC-981 | 🟡 | Timestamp-indexed lookup — timer expiry check |
| 4 | Task Scheduler | LC-621 | 🟡 | Cooldown period / re-fire delay — `mod_timer()` |
| 5 | Design a Food Rating System | LC-2353 | 🟡 | Priority + time-based operations |

---

## PART 3 — Platform-Indexed Problem List

### LeetCode (by Problem Number)

```
LC-3    Longest Substring Without Repeating Characters     🟡  Sliding Window
LC-4    Median of Two Sorted Arrays                        💀  Binary Search
LC-23   Merge K Sorted Lists                               💀  Heap / DLL
LC-25   Reverse Nodes in k-Group                           💀  Linked List
LC-33   Search in Rotated Sorted Array                     🟡  Binary Search
LC-34   Find First and Last Position                       🟡  Binary Search
LC-42   Trapping Rain Water                                🟡  Monotonic Stack
LC-56   Merge Intervals                                    🟡  Interval Tree
LC-57   Insert Interval                                    🟡  Interval Tree
LC-74   Search a 2D Matrix                                 🟡  Binary Search
LC-76   Minimum Window Substring                           💀  Sliding Window
LC-84   Largest Rectangle in Histogram                     💀  Monotonic Stack
LC-85   Maximal Rectangle                                  💀  Monotonic Stack
LC-92   Reverse Linked List II                             🟡  Linked List
LC-98   Validate Binary Search Tree                        🟡  BST / RB-Tree
LC-102  Binary Tree Level Order Traversal                  🟡  BFS / Tree
LC-104  Maximum Depth of Binary Tree                       🟢  Tree DFS
LC-113  Path Sum II                                        🟡  Tree DFS
LC-127  Word Ladder                                        💀  Graph BFS
LC-136  Single Number                                      🟢  Bit Manipulation
LC-137  Single Number II                                   🟡  Bit Manipulation
LC-138  Copy List with Random Pointer                      🟡  Linked List
LC-145  Binary Tree Postorder Traversal                    🟢  Tree DFS
LC-146  LRU Cache                  ★★★                    🟡  LRU / Hash + DLL
LC-148  Sort List                                          🟡  Linked List / Merge Sort
LC-153  Find Minimum in Rotated Sorted Array               🟡  Binary Search
LC-155  Min Stack                                          🟡  Stack
LC-162  Find Peak Element                                  🟡  Binary Search
LC-190  Reverse Bits                                       🟢  Bit Manipulation
LC-191  Number of 1 Bits                                   🟢  Bitmap / popcount
LC-200  Number of Islands                                  🟡  Graph BFS/DFS
LC-201  Bitwise AND of Numbers Range                       🟡  Bit Manipulation
LC-207  Course Schedule            ★★★                    🟡  Topo Sort / Cycle Detection
LC-208  Implement Trie             ★★★                    🟡  Trie / Radix Tree
LC-209  Minimum Size Subarray Sum                          🟡  Sliding Window
LC-211  Design Add and Search Words                        🟡  Trie
LC-212  Word Search II                                     💀  Trie + DFS
LC-215  Kth Largest Element                                🟡  Heap / Quickselect
LC-218  The Skyline Problem                                💀  Heap / Segment Tree
LC-230  Kth Smallest in BST                                🟡  BST
LC-231  Power of Two                                       🟢  Bit Manipulation / Buddy
LC-235  LCA of BST                                        🟡  BST
LC-236  LCA of Binary Tree                                🟡  Tree
LC-239  Sliding Window Maximum     ★★                     💀  Monotonic Deque / Sliding Window
LC-253  Meeting Rooms II                                   🟡  Interval Tree / Heap
LC-269  Alien Dictionary (Premium)                         💀  Topological Sort
LC-295  Find Median from Data Stream                       💀  Heap (two heaps)
LC-297  Serialize/Deserialize Binary Tree                  💀  Tree BFS/DFS
LC-307  Range Sum Query - Mutable                          🟡  Segment Tree / BIT
LC-308  Range Sum Query 2D - Mutable                       💀  2D Segment Tree
LC-310  Minimum Height Trees                               🟡  Topological Sort
LC-315  Count of Smaller Numbers After Self                💀  BIT / Segment Tree / Merge Sort
LC-318  Maximum Product of Word Lengths                    🟡  Bitmask
LC-324  Wiggle Sort II                                     🟡  Quickselect
LC-327  Count of Range Sum                                 💀  Segment Tree / BIT
LC-338  Counting Bits                                      🟢  Bit Manipulation
LC-347  Top K Frequent Elements                            🟡  Heap
LC-352  Data Stream as Disjoint Intervals                  💀  Interval Tree
LC-355  Design Twitter                                     🟡  Heap
LC-371  Sum of Two Integers                                🟡  Bit Arithmetic
LC-380  Insert Delete GetRandom O(1)                       🟡  Hash Table
LC-381  Insert Delete GetRandom O(1) Duplicates            💀  Hash Table
LC-410  Split Array Largest Sum                            💀  Binary Search on Answer
LC-417  Pacific Atlantic Water Flow                        🟡  Graph BFS
LC-421  Maximum XOR of Two Numbers    ★★                  🟡  Binary Trie (LPM concept)
LC-430  Flatten Multilevel DLL                             🟡  Linked List
LC-432  All O`one Data Structure   ★★                     💀  Hash + DLL (dcache eviction)
LC-438  Find All Anagrams                                  🟡  Sliding Window
LC-450  Delete Node in BST                                 🟡  BST / RB-Tree
LC-460  LFU Cache                  ★★                     💀  LFU / Hash + DLL
LC-461  Hamming Distance                                   🟢  Bit Manipulation
LC-493  Reverse Pairs                                      💀  Merge Sort / BIT / Segment Tree
LC-502  IPO                                                💀  Heap (two heaps)
LC-535  Encode and Decode TinyURL                          🟡  Hash Function Design
LC-542  01 Matrix                                          🟡  BFS
LC-543  Diameter of Binary Tree                            🟢  Tree DFS
LC-547  Number of Provinces                                🟡  Union-Find
LC-567  Permutation in String                              🟡  Sliding Window
LC-621  Task Scheduler              ★★                    🟡  CPU Scheduling / Heap
LC-622  Design Circular Queue       ★★★                   🟡  Ring Buffer (kfifo)
LC-630  Course Schedule III         ★★                    💀  Deadline Scheduling / Heap
LC-641  Design Circular Deque                              🟡  Ring Buffer
LC-648  Replace Words                                      🟡  Trie
LC-676  Implement Magic Dictionary                         🟡  Trie
LC-677  Map Sum Pairs                                      🟡  Trie
LC-684  Redundant Connection                               🟡  Union-Find / Cycle Detection
LC-685  Redundant Connection II                            💀  Union-Find / Directed Graph
LC-699  Falling Squares                                    💀  Segment Tree / Interval
LC-701  Insert into BST                                    🟢  BST / RB-Tree
LC-703  Kth Largest Element in Stream                      🟢  Heap
LC-704  Binary Search                                      🟢  Binary Search
LC-705  Design HashSet                                     🟢  Hash Table
LC-706  Design HashMap                                     🟢  Hash Table
LC-716  Max Stack                                          🟡  Stack + DLL
LC-721  Accounts Merge                                     🟡  Union-Find
LC-729  My Calendar I                                      🟡  Interval / BST
LC-731  My Calendar II                                     🟡  Interval Tree
LC-732  My Calendar III            ★★                     💀  Segment Tree / Interval Tree
LC-739  Daily Temperatures                                 🟡  Monotonic Stack
LC-745  Prefix and Suffix Search                           💀  Trie
LC-785  Is Graph Bipartite                                 🟡  BFS / DSU
LC-787  Cheapest Flights K Stops                           🟡  BFS / Bellman-Ford
LC-802  Find Eventual Safe States                          🟡  Graph / Cycle Detection
LC-839  Similar String Groups                              💀  Union-Find
LC-875  Koko Eating Bananas                                🟡  Binary Search on Answer
LC-895  Maximum Frequency Stack                            💀  Hash + Frequency Map
LC-904  Fruit Into Baskets                                 🟡  Sliding Window
LC-907  Sum of Subarray Minimums                           🟡  Monotonic Stack
LC-912  Sort an Array              ★★                     🟡  Heapsort (lib/sort.c)
LC-968  Binary Tree Cameras                                💀  Tree DP
LC-981  Time Based Key-Value Store                         🟡  Binary Search / Hash
LC-990  Satisfiability of Equality Equations               🟡  Union-Find
LC-1004 Max Consecutive Ones III                           🟡  Sliding Window
LC-1011 Capacity to Ship Packages                          🟡  Binary Search on Answer
LC-1032 Stream of Characters                               💀  Trie (Aho-Corasick)
LC-1046 Last Stone Weight                                  🟢  Heap
LC-1091 Shortest Path in Binary Matrix                     🟡  BFS
LC-1192 Critical Connections in a Network                  💀  Tarjan Bridges
LC-1202 Smallest String With Swaps                         🟡  Union-Find
LC-1203 Sort Items by Groups                               💀  Topological Sort (2-level)
LC-1206 Design Skiplist            ★★★                   💀  Skip List
LC-1318 Minimum Flips to Make a OR b = c                  🟡  Bit Manipulation
LC-1382 Balance a BST                                      🟡  BST
LC-1438 Longest Subarray Abs Diff ≤ Limit                  🟡  Monotonic Deque
LC-1448 Count Good Nodes in Binary Tree                    🟡  Tree DFS
LC-1475 Final Prices With Special Discount                 🟢  Monotonic Stack
LC-1584 Min Cost to Connect All Points                     🟡  MST / Union-Find
LC-1631 Path With Minimum Effort                           🟡  Dijkstra / Binary Search
LC-1642 Furthest Building You Can Reach                    🟡  Heap
LC-1670 Design Front Middle Back Queue                     🟡  Deque
LC-1675 Minimize Deviation in Array                        💀  Heap
LC-1756 Design Most Recently Used Queue                    🟡  MRU Queue
LC-1803 Count Pairs With XOR in Range                      💀  Binary Trie
LC-1851 Minimum Interval to Include Each Query             💀  Interval Tree / Heap
LC-2104 Sum of Subarray Ranges                             🟡  Monotonic Stack
LC-2192 All Ancestors of Node in DAG                       🟡  Topological Sort
LC-2276 Count Integers in Intervals                        💀  Interval Tree
LC-2353 Design a Food Rating System                        🟡  Hash + Heap
```

---

### CSES Problem Set (cses.fi)

```
Category           Problem                          DSA Concept
──────────────────────────────────────────────────────────────────────────────
Range Queries      Dynamic Range Sum Queries        Segment Tree / BIT
Range Queries      Dynamic Range Minimum Queries ★  Segment Tree (point upd, range min)
Range Queries      Range Update Queries          ★  Lazy Propagation Segment Tree
Range Queries      Hotel Queries                    Segment Tree Binary Search
Range Queries      List Removals                    BIT (k-th element removal)
Range Queries      Salary Queries                ★  BIT + Coordinate Compression
Range Queries      Forest Queries                   2D BIT
Range Queries      Polynomial Queries               Complex Lazy Propagation
Range Queries      Range Update and Sums            Lazy Propagation (dual)
──────────────────────────────────────────────────────────────────────────────
Trees              Subordinates                     Subtree Size (DFS)
Trees              Tree Diameter                ★  Tree BFS/DFS
Trees              Tree Distances I & II            Rerooting DP
Trees              Company Queries I (LCA)          Binary Lifting
Trees              Company Queries II (LCA)         Binary Lifting + depth
Trees              Distance Queries                 LCA + distance
Trees              Tree Matching                    Tree DP
──────────────────────────────────────────────────────────────────────────────
Graph Algorithms   Shortest Routes I                Dijkstra
Graph Algorithms   Shortest Routes II               Floyd-Warshall
Graph Algorithms   High Score                       Bellman-Ford (longest path)
Graph Algorithms   Cycle Finding                ★  Bellman-Ford negative cycle
Graph Algorithms   Round Trip                    ★  Undirected Cycle Detection
Graph Algorithms   Round Trip II                ★  Directed Cycle Detection (lockdep)
Graph Algorithms   Course Schedule              ★  Topological Sort (module deps)
Graph Algorithms   Longest Flight Route             DAG Longest Path
Graph Algorithms   Game Routes                      DAG Path Count
Graph Algorithms   Giant Pizza                      2-SAT via SCC
──────────────────────────────────────────────────────────────────────────────
Sorting/Searching  Restaurant Customers             Sweep + Priority Queue
Sorting/Searching  Tasks and Deadlines          ★  Deadline Scheduling (EDF)
Sorting/Searching  Josephus Problem I & II          Circular Queue / Order Statistics
──────────────────────────────────────────────────────────────────────────────
Connected Comp     Building Teams                   Bipartite Check (DSU)
Connected Comp     Road Reparation                  MST (Kruskal + DSU)
Connected Comp     Planets Queries I & II           Binary Lifting on functional graph
```

---

### SPOJ Problems

```
Problem Code   Problem Name                      DSA Concept
──────────────────────────────────────────────────────────────────────────────
HORRIBLE    ★  Horrible Queries                  Lazy Propagation Segment Tree
GSS1           Can You Answer These Queries I    Segment Tree (max subarray)
GSS3           Can You Answer These Queries III  Segment Tree (max subarray, updates)
LITE           Light Switching                   Lazy Flip Segment Tree
MULTQ3         Multiples of 3                    Segment Tree (mod counting)
KQUERY         K-Query                           BIT + Offline Sorting
DQUERY         D-Query                           Mo's Algorithm
NKMOU          Codeforces-style Interval         Segment Tree
ARST           All-Round Segment Tree            Lazy Propagation
```

---

### HackerRank Problems

```
Domain              Problem                        DSA Concept
──────────────────────────────────────────────────────────────────────────────
Data Structures     Contacts                    ★  Trie (prefix count)
Data Structures     No Prefix Set                  Trie (conflict detection)
Data Structures     Self Balancing Tree         ★  BST (height tracking)
Data Structures     Array and Simple Queries       DLL-based rotation
Data Structures     Cube Summation                 3D BIT
Data Structures     Castle on the Grid             BFS grid
──────────────────────────────────────────────────────────────────────────────
Algorithms          Jesse and Cookies              Min-Heap
Algorithms          QHEAP1                      ★  Heap operations (insert/delete/min)
Algorithms          Find the Running Median     ★  Two-Heap median stream
Algorithms          Fraudulent Activity Notifications  Sliding Window + Counting Sort
Algorithms          Minimum Average Waiting Time   Greedy + Heap (SJF scheduling)
Algorithms          Angry Professor                Sorting + Threshold counting
```

---

### GeeksForGeeks (Direct Implementations)

```
Topic                        Problem / Article                   Kernel Relevance
──────────────────────────────────────────────────────────────────────────────
Red-Black Tree               RB-Tree Insertion                   lib/rbtree.c
Red-Black Tree               RB-Tree Deletion                    lib/rbtree.c
B-Tree                    ★  B-Tree Insertion + Search           ext4 htree
B+ Tree                   ★  B+ Tree Insertion + Search          Btrfs / XFS ctree
Patricia Trie / Radix Trie★  Compressed Trie Implementation      net/ipv4/fib_trie.c
Segment Tree                 Lazy Propagation Tutorial           Range management
Skip List                    Skip List Implementation            Probabilistic DS
LRU Cache                    LRU using Hash + DLL                mm/vmscan.c
Buddy System              ★  Buddy Memory Allocation Simulation  mm/page_alloc.c
Slab Allocator            ★  Slab Allocator Design               mm/slub.c
Memory Pool                  Memory Pool Implementation          Kmem Cache
Bloom Filter                 Bloom Filter Implementation         BPF_MAP_TYPE_BLOOM_FILTER
Trie (LPM)                ★  Longest Prefix Matching (routing)   net/ipv4/fib_trie.c
Interval Tree             ★  Interval Tree Insert + Search       mm/interval_tree.c
Union-Find                   Path Compression + Union by Rank    lockdep class grouping
Topological Sort             Kahn's + DFS based                  kernel/module/main.c
Dijkstra                     Dijkstra with Adjacency List        NUMA topology
Tarjan SCC                ★  Strongly Connected Components       lockdep safe classes
Detect Cycle (Directed)   ★  DFS 3-color cycle detection        lockdep deadlock check
```

---

## PART 4 — Kernel-Specific Design Problems

These are "design" style problems that don't exist on competitive platforms but directly model kernel subsystems. Implement them from scratch:

```
1. Design a Buddy Allocator
   ─────────────────────────
   API: alloc(order) → addr; free(addr, order)
   Use: array of free_area[0..MAX_ORDER-1], each a doubly linked list of blocks.
   Key: buddy address = addr XOR (1 << (order + PAGE_SHIFT))
   Map to: mm/page_alloc.c

2. Design a Hierarchical Timer Wheel
   ────────────────────────────────
   API: add_timer(fn, delay_ticks); tick()
   Use: 5-level cascading wheel, each level 64–256 buckets.
   Map to: kernel/time/timer_wheel.c

3. Design a Lock Dependency Checker (Lockdep Mini)
   ─────────────────────────────────────────────
   API: acquire(lock_class); release(lock_class)
   Use: directed graph of (A→B) = "A held while B acquired"
   Detect: cycle via DFS after each new edge
   Map to: kernel/locking/lockdep.c

4. Design an io_uring Ring Buffer Pair
   ─────────────────────────────────
   API: submit(sqe); complete(cqe); flush()
   Use: two power-of-2 ring buffers (SQ + CQ) with head/tail indices
   Key: SQPOLL — kernel thread consumes SQ without syscall
   Map to: io_uring/io_uring.c

5. Design an LPM (Longest Prefix Match) Trie
   ─────────────────────────────────────────
   API: insert(prefix, prefix_len, value); lookup(ip) → value
   Use: binary trie, each bit of the IP address is a branch
   Key: on lookup, track last matching node with a stored prefix
   Map to: kernel/bpf/lpm_trie.c, net/ipv4/fib_trie.c

6. Design a Slab Allocator (Mini SLUB)
   ────────────────────────────────────
   API: kmem_cache_create(size); kmem_cache_alloc(); kmem_cache_free()
   Use: per-cpu free lists, slab pages, object reuse without page allocator
   Map to: mm/slub.c

7. Design a CFS Scheduler Simulator
   ──────────────────────────────
   API: add_task(pid, weight); remove_task(pid); pick_next() → pid; update_vruntime(pid, runtime_ns)
   Use: RB-tree keyed by vruntime; vruntime += runtime_ns * NICE_0_LOAD / weight
   Key: leftmost node = next to run (lowest vruntime)
   Map to: kernel/sched/fair.c
```

---

## PART 5 — Recommended Study Sequence

```
Phase 1: Foundations (Weeks 1–3)
─────────────────────────────────
LC-191, LC-231, LC-338, LC-461     → Bitmap / bit ops (lib/bitmap.c)
LC-704, LC-33, LC-34, LC-162       → Binary search (lib/bsearch.c)
LC-622, LC-641                      → Ring buffer (kfifo)
LC-98, LC-701, LC-450, LC-230       → BST basics → RB-tree intuition
LC-146, LC-460                      → LRU / LFU (page cache)
LC-705, LC-706                      → Hash table (dcache / conntrack)

Phase 2: Core Kernel Structures (Weeks 4–7)
────────────────────────────────────────────
LC-208, LC-421, GFG Patricia Trie  → Trie / LPM trie (FIB, eBPF)
LC-56, LC-57, LC-715, LC-352       → Interval tree (VMA)
LC-207, LC-210                      → Topological sort (module loader)
LC-684, LC-685, CSES Round Trip II → Cycle detection (lockdep)
LC-547, LC-721, LC-990             → Union-Find (lock classes)
LC-239, LC-76, LC-3                → Sliding window (TCP)
LC-621, LC-630, HR Tasks+Deadlines → Scheduling / deadline heap

Phase 3: Advanced (Weeks 8–12)
────────────────────────────────
CSES Dynamic Range + Range Update  → Segment tree + lazy propagation
SPOJ HORRIBLE, GSS1                → Lazy segment tree (hard)
LC-307, LC-315, LC-493             → BIT / merge sort
LC-432, LC-895                     → Advanced hash + DLL
LC-1206                            → Skip list
LC-218, LC-295                     → Dual-heap problems
GFG RB-Tree, B+ Tree               → Direct implementations
LC-1192, GFG Tarjan SCC            → Advanced graph (lockdep deep)

Phase 4: Design Problems
─────────────────────────
Implement: Buddy Allocator
Implement: LPM Trie (binary, with prefix tracking)
Implement: Mini LRU Cache matching Linux 5-list model
Implement: io_uring ring buffer pair
Implement: CFS simulator (RB-tree + vruntime)
Implement: Lockdep mini (directed graph + DFS cycle detection)
```

---

## PART 6 — Quick Reference: Problem → Kernel Construct

```
If you're reading this kernel code...      Solve these problems first:
──────────────────────────────────────────────────────────────────────────────
kernel/sched/fair.c (CFS)                 LC-98, LC-315, LC-230, LC-621, LC-630
mm/page_alloc.c (buddy)                   LC-231, LC-56, LC-715, GFG Buddy Sim
mm/mmap.c (VMA / interval_tree)           LC-56, LC-57, LC-715, LC-352, LC-2276
mm/vmscan.c (LRU eviction)                LC-146, LC-460, LC-432
fs/dcache.c (dentry cache)                LC-706, LC-146, LC-432
net/ipv4/fib_trie.c (routing LPM)         LC-208, LC-421, GFG LPM Routing
kernel/bpf/lpm_trie.c (eBPF LPM)         LC-208, LC-421, LC-1803, Design LPM
kernel/bpf/ringbuf.c (BPF ring buf)       LC-622, LC-641
io_uring/io_uring.c (SQ/CQ rings)         LC-622, LC-641, Design io_uring
kernel/locking/lockdep.c (deadlock)       LC-207, CSES Round Trip II, GFG Tarjan
include/linux/list.h (intrusive DLL)      LC-146, LC-148, LC-25, LC-432
lib/sort.c (heapsort)                     LC-912, LC-215, LC-148
lib/bsearch.c (binary search)             LC-704, LC-33, LC-34, LC-981
lib/rbtree.c (RB-tree)                    LC-98, LC-701, LC-450, LC-315
lib/xarray.c / radix-tree.c              LC-208, LC-421, LC-677, LC-1032
kernel/module/main.c (module loader)      LC-207, LC-210, LC-1203
kernel/time/timer_wheel.c                LC-622, LC-621, Design Timer Wheel
fs/btrfs/ctree.c (B+ tree)               GFG B+ Tree, LC-729, LC-731, LC-732
net/ipv4/tcp_input.c (TCP window)         LC-239, LC-76, LC-567, LC-3
kernel/cgroup/cgroup.c (cgroup tree)      LC-102, LC-145, LC-236, CSES Subordinates
```

---

*Last updated: 2026. Problems verified across LeetCode, CSES, SPOJ, HackerRank, GeeksForGeeks.*
*★★★ = critical / foundational. ★★ = highly recommended. ★ = direct analog.*
