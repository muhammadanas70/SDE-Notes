# LSM-Trie: Comprehensive In-Depth Guide

## Table of Contents
1. [Introduction and Context](#introduction-and-context)
2. [LSM Tree Fundamentals](#lsm-tree-fundamentals)
3. [Trie Data Structure Fundamentals](#trie-data-structure-fundamentals)
4. [LSM-Trie Hybrid Architecture](#lsm-trie-hybrid-architecture)
5. [Core Concepts and Algorithms](#core-concepts-and-algorithms)
6. [Memory Management and Persistence](#memory-management-and-persistence)
7. [Protocols and Operation Flow](#protocols-and-operation-flow)
8. [Performance Analysis](#performance-analysis)
9. [Rust Implementation](#rust-implementation)
10. [C Implementation](#c-implementation)
11. [Go Implementation](#go-implementation)
12. [Advanced Topics](#advanced-topics)
13. [Comparison and Trade-offs](#comparison-and-trade-offs)

---

## Introduction and Context

### What is LSM-Trie?

An LSM-Trie is a hybrid data structure that combines:
- **Log-Structured Merge (LSM) Trees**: Write-optimized, persistent storage structure
- **Trie (Prefix Tree)**: String-key optimized search structure

This combination creates a data structure that is excellent for:
- **High-throughput writes** (inherent to LSM architecture)
- **String-key workloads** (inherent to Trie architecture)
- **Range queries** on string prefixes
- **Compressed storage** (Trie reduces key redundancy)

### Why LSM-Trie Matters

Traditional B-Trees excel at range queries but suffer from random I/O during writes. LSMs optimize writes through sequential I/O. Tries optimize string search through prefix matching. Their combination addresses:

1. **Write amplification**: Sequential writes through memtable → SSTables
2. **Read efficiency**: Trie structure reduces key comparisons
3. **Storage efficiency**: Shared prefixes reduce memory footprint
4. **Prefix queries**: Native support for range queries on string keys

### Real-World Applications

- **RocksDB** (with prefix optimization): Key-value store
- **Cassandra**: Distributed database
- **HBase**: Hadoop-based distributed database
- **LevelDB**: Embedded key-value store
- **String databases**: Autocomplete, IP routing tables, DNS servers
- **Full-text search indexes**: Elasticsearch, Lucene

---

## LSM Tree Fundamentals

### What is an LSM Tree?

An LSM Tree is a data structure optimized for high-throughput sequential writes. It defers writes to disk and merges them in controlled batches (compaction), converting random writes into sequential I/O.

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         LSM Tree Architecture                    │
└─────────────────────────────────────────────────────────────────┘

Memory (Volatile)
┌──────────────────────┐
│   Memtable (L0)      │  <- Active write buffer (in-memory)
│   ┌──────────────────┤
│   │ Sorted Key-Value │
│   │ Pairs            │
│   │ (Red-Black Tree) │
│   └──────────────────┘
│   Size: ~2-4MB       │
│   Write Order: O(log │
│                N)    │
└──────────────────────┘
         ↓ (flush when full)

Disk (Persistent)
┌──────────────────────────────────────────────────────────────────┐
│                          SSTables Levels                          │
├──────────────────────────────────────────────────────────────────┤
│ Level 0 (L0):                                                     │
│ ┌──────────┐ ┌──────────┐ ┌──────────┐                           │
│ │ SSTable0 │ │ SSTable1 │ │ SSTable2 │ ... (no key overlap)      │
│ │ 2MB each │ │ 2MB each │ │ 2MB each │                           │
│ └──────────┘ └──────────┘ └──────────┘                           │
├──────────────────────────────────────────────────────────────────┤
│ Level 1 (L1):                                                     │
│ ┌─────────────────────┐ ┌─────────────────────┐                 │
│ │ SSTable  (10MB)     │ │ SSTable  (10MB)     │ (key ranges)     │
│ │ Sorted, merged      │ │ Sorted, merged      │                 │
│ └─────────────────────┘ └─────────────────────┘                 │
├──────────────────────────────────────────────────────────────────┤
│ Level 2 (L2):                                                     │
│ ┌───────────────────────────────────────────────────────────┐   │
│ │ Large SSTable (100MB+)                                    │   │
│ │ Fully sorted, minimal overlaps                            │   │
│ └───────────────────────────────────────────────────────────┘   │
├──────────────────────────────────────────────────────────────────┤
│ Level N (LN):                                                     │
│ ┌───────────────────────────────────────────────────────────┐   │
│ │ Largest SSTables (1GB+)                                   │   │
│ │ Complete coverage, optimal structure                      │   │
│ └───────────────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────────┘
```

### Key Components

#### 1. Memtable (Write Buffer)

The memtable is an in-memory data structure holding recent writes:

**Properties:**
- In-memory sorted structure (typically Red-Black Tree or Skip List)
- Fixed maximum size (e.g., 2-4 MB)
- All writes go here first
- Provides O(log N) insertion and O(1) lookups for recent data

**Lifecycle:**
```
Empty → Receiving Writes → Full → Flushing to Disk → Immutable → Discarded
        (Active)         (trigger)  (background)    (read-only) (merged)
```

#### 2. SSTables (Sorted String Tables)

SSTables are immutable, sorted files on disk:

**Structure:**
```
┌────────────────────────────────────────────────────┐
│             SSTable File Layout                     │
├────────────────────────────────────────────────────┤
│ Data Block 0 (compressed key-value pairs)         │ 4KB
│ ├─ Key1: Value1                                   │
│ ├─ Key2: Value2                                   │
│ └─ ...                                            │
├────────────────────────────────────────────────────┤
│ Data Block 1                                      │ 4KB
├────────────────────────────────────────────────────┤
│ ...                                               │
├────────────────────────────────────────────────────┤
│ Index Block (key offsets for binary search)       │
│ ├─ Key1 → Block 0, Offset 0                       │
│ ├─ Key5 → Block 1, Offset 100                     │
│ └─ ...                                            │
├────────────────────────────────────────────────────┤
│ Bloom Filter (fast negative lookups)              │
│ (bit array: 1 if key possibly exists)             │
├────────────────────────────────────────────────────┤
│ Footer & Metadata                                  │
│ ├─ Version info                                   │
│ ├─ Timestamp range                                │
│ ├─ Key range (min_key, max_key)                   │
│ └─ Checksum                                       │
└────────────────────────────────────────────────────┘
```

**Key Properties:**
- Immutable once written
- Sorted by key (enables binary search)
- Compressed (Snappy, LZ4, zstd)
- Size increases at each level (typically 10x growth)
- Support efficient range queries

#### 3. Compaction

Compaction merges SSTables from different levels:

**Types:**

**Minor Compaction:**
- Flushes memtable to Level 0
- Creates new SSTable from sorted memtable data
- Synchronous or asynchronous

**Major Compaction:**
- Merges overlapping SSTables across levels
- Performed in background
- Reduces read amplification
- Generates write amplification

**Compaction Strategy:**

```
Before Compaction:
Level 0: [SST0:a-f] [SST1:c-h] [SST2:e-j]  (overlapping)
Level 1: [SST3:a-d] [SST4:e-h] [SST5:i-j]  (sorted)
         ↓↓↓ Compact L0→L1 ↓↓↓

After Compaction:
Level 0: []  (cleared)
Level 1: [SST6:a-d] [SST7:e-h] [SST8:i-j]  (merged)
```

### Write Operation Flow

```
┌──────────────────────────────────────────────────────────┐
│              Write Operation in LSM Tree                  │
└──────────────────────────────────────────────────────────┘

User writes Key=K, Value=V
         ↓
    ┌────────────────┐
    │ Write-Ahead    │ Log to disk BEFORE applying to memory
    │ Log (WAL)      │ Ensures durability on crash
    │ Append K,V     │
    └────────────────┘
         ↓ (success)
    ┌────────────────────┐
    │ Insert into        │
    │ Memtable           │
    │ (Red-Black Tree)   │
    │ T: O(log N)        │
    └────────────────────┘
         ↓
    Is Memtable full?
         ↓ Yes
    ┌────────────────────┐
    │ Create Immutable   │
    │ Version of         │
    │ Memtable           │
    └────────────────────┘
         ↓
    ┌────────────────────┐
    │ Flush to Disk      │
    │ Create Level 0     │
    │ SSTable (sorted)   │
    │ T: O(N)            │
    └────────────────────┘
         ↓
    ┌──────────────────────────┐
    │ Check if Compaction      │
    │ Needed (Level overlap)   │
    │ Trigger background job   │
    └──────────────────────────┘
         ↓
    Acknowledge write to user
    (before flush in some configs)
```

### Read Operation Flow

```
┌─────────────────────────────────────────────────────────┐
│            Read Operation in LSM Tree                    │
└─────────────────────────────────────────────────────────┘

User queries Key=K
         ↓
Check Memtable (current)
         ↓ Not found
Check Immutable Memtables
         ↓ Not found
Check Level 0 SSTables
  ├─ Bloom filter check (fast negative)
  ├─ If possible hit: binary search in SSTable
  └─ T: O(# of L0 files)
         ↓ Not found (or checking levels)
Check Level 1 SSTables
  ├─ Single file (sorted range)
  ├─ Binary search on key range
  └─ T: O(log # of L1 files)
         ↓ Continue if not found
Check Level 2, 3, ... until found
         ↓
Return value (if found) or nil (if not)

Read Amplification: Potentially check all levels
                    Mitigated by:
                    - Bloom filters
                    - Index blocks
                    - Key range metadata
```

---

## Trie Data Structure Fundamentals

### What is a Trie?

A **Trie** (pronounced "try," short for "retrieval") is a tree-based data structure that stores strings efficiently by sharing common prefixes.

### Basic Structure

```
┌────────────────────────────────────────────────┐
│        Trie for Keys: "cat", "car", "dog"      │
└────────────────────────────────────────────────┘

              root
              /  \
             c    d
             |    |
             a    o
            / \   |
           t   r  g
          ($) ($) ($)

$ = terminal node (key endpoint)

Insertion of "cat":
root → c → a → t ($)

Insertion of "car":
root → c → a → r ($)
       (reuses c and a)

Insertion of "dog":
root → d → o → g ($)

Key Property: "ca" is a common prefix
              Both "cat" and "car" share: c→a
```

### Node Structure

```
Each Trie Node contains:
┌─────────────────────────────────────┐
│         Trie Node                   │
├─────────────────────────────────────┤
│ children: Map<char, TrieNode>       │
│   └─ References to child nodes      │
│                                     │
│ is_end_of_word: bool                │
│   └─ Marks if a valid key ends here │
│                                     │
│ value: Option<V>                    │
│   └─ Associated value (for KV trie) │
└─────────────────────────────────────┘
```

### Trie Operations

#### Insert (Add a key-value pair)

**Algorithm:**
```
INSERT(trie, key, value):
  current = trie.root
  FOR each character c IN key:
    IF c not in current.children:
      current.children[c] = new TrieNode()
    current = current.children[c]
  
  current.is_end_of_word = true
  current.value = value
```

**Time Complexity:** O(m) where m = key length
**Space Complexity:** O(m) for new nodes

#### Search (Find a key)

**Algorithm:**
```
SEARCH(trie, key):
  current = trie.root
  FOR each character c IN key:
    IF c not in current.children:
      RETURN nil  // Key not found
    current = current.children[c]
  
  IF current.is_end_of_word:
    RETURN current.value
  ELSE:
    RETURN nil  // Prefix exists but not a complete key
```

**Time Complexity:** O(m) where m = key length
**Space Complexity:** O(1)

#### Prefix Query (Find all keys with prefix)

**Algorithm:**
```
PREFIX_QUERY(trie, prefix):
  current = trie.root
  
  // Navigate to prefix end
  FOR each character c IN prefix:
    IF c not in current.children:
      RETURN []  // No keys with this prefix
    current = current.children[c]
  
  // DFS to collect all keys
  results = []
  DFS(current, prefix, results)
  RETURN results

DFS(node, current_key, results):
  IF node.is_end_of_word:
    results.append(current_key)
  
  FOR each (char c, child) IN node.children:
    DFS(child, current_key + c, results)
```

**Time Complexity:** O(n) where n = total characters in matching keys

#### Delete

**Algorithm:**
```
DELETE(trie, key):
  current = trie.root
  stack = [(root, null)]
  
  FOR each character c IN key:
    IF c not in current.children:
      RETURN false  // Key not found
    stack.push((current, c))
    current = current.children[c]
  
  IF NOT current.is_end_of_word:
    RETURN false  // Key doesn't exist
  
  current.is_end_of_word = false
  
  // Cleanup empty nodes (post-order)
  WHILE stack not empty:
    (parent, char_to_child) = stack.pop()
    child = parent.children[char_to_child]
    
    IF child.children.empty() AND NOT child.is_end_of_word:
      DELETE parent.children[char_to_child]
    ELSE:
      BREAK  // Non-empty node, stop cleanup
  
  RETURN true
```

**Time Complexity:** O(m) where m = key length

### Trie Variants

#### Compressed Trie (Radix Tree)

Merges single-child nodes into edges labeled with strings:

```
Standard Trie:
    root
    / | \
   c  d  
   |  |  
   a  o  
  / \  |  
 t   r  g

Compressed Trie (Radix):
    root
    / | \
  "ca" "do"
  / \   |
"t" "r" "g"

Benefits:
- Reduced node count
- Faster traversal
- Reduced memory for sparse tries
```

#### HAT-Trie

Hash Array Mapped Trie - uses hash arrays instead of arrays:

```
Hybrid approach:
┌──────────────────────────────────┐
│     HAT-Trie Node                │
├──────────────────────────────────┤
│ For 0-4 children:                │
│   Use linked list                │
│                                  │
│ For 5+ children:                 │
│   Switch to hash table           │
│                                  │
│ Result: Space-efficient,         │
│         Cache-friendly           │
└──────────────────────────────────┘
```

### Space Complexity Analysis

For a trie with k keys of average length m:

```
Standard Trie:
- Best case: O(m) - all keys identical
- Worst case: O(k*m) - no shared prefixes
- Average case: O(k*m) - with some sharing

Space = sum of:
  - Nodes: up to k*m
  - Children pointers: variable per node
  - Values: k values
  
Typical: ~20-50 bytes per node (implementation dependent)
```

---

## LSM-Trie Hybrid Architecture

### The Hybrid Insight

LSM-Trie combines:
1. **LSM's write optimization**: Sequential disk I/O through memtables and SSTables
2. **Trie's key compression**: Shared prefixes reduce storage
3. **Trie's range queries**: Efficient prefix-based range queries

### Architecture Overview

```
┌──────────────────────────────────────────────────────────┐
│              LSM-Trie Architecture                        │
└──────────────────────────────────────────────────────────┘

Memory (Volatile)
┌────────────────────────────────────────┐
│     Memtable as Trie                   │
│                                        │
│        root                            │
│        / \                             │
│       a   b                            │
│       |   |                            │
│       c   d                            │
│      ($) ($)                           │
│                                        │
│  Keys: "ac"→v1, "ad"→v2, "bd"→v3      │
│  Insert: O(m), Search: O(m)            │
│  Range queries: O(matches)             │
│  Size limit: 2-4MB                     │
└────────────────────────────────────────┘
         ↓ (flush when full)

Disk (Persistent)
┌──────────────────────────────────────────────────────────┐
│                      SSTables as Tries                    │
├──────────────────────────────────────────────────────────┤
│ Level 0: SSTables (Each is a Trie)                       │
│  ┌──────────────────┐  ┌──────────────────┐              │
│  │  Trie in SSTable │  │  Trie in SSTable │              │
│  │  Compressed      │  │  Compressed      │              │
│  │  Sorted blocks   │  │  Sorted blocks   │              │
│  └──────────────────┘  └──────────────────┘              │
├──────────────────────────────────────────────────────────┤
│ Level 1: Merged SSTables (Tries merged into index)       │
│  ┌──────────────────────────────────────┐                │
│  │  Merged Trie Structure               │                │
│  │  Optimized for range queries         │                │
│  └──────────────────────────────────────┘                │
├──────────────────────────────────────────────────────────┤
│ Levels 2+: Larger merged structures                      │
└──────────────────────────────────────────────────────────┘
```

### Key Design Decisions

#### 1. Memtable Structure

```
Option A: Red-Black Tree (Traditional LSM)
  Pros: Simple, predictable performance
  Cons: Poor prefix sharing, generic keys

Option B: Trie (LSM-Trie)
  Pros: Prefix compression, efficient range queries
  Cons: Memory overhead per node
  
┌────────────────────────────────┐
│ Hybrid: Trie with compression  │
│ - Use Compressed/Radix Trie    │
│ - Merge single-child paths     │
│ - Reduce per-node overhead     │
└────────────────────────────────┘
```

#### 2. SSTable Encoding

```
LSM-Trie SSTable Layout:

Block 1 (Key-Value Block):
  Encoding Options:
  
  A) Literal Encoding:
     [key_len][key][value_len][value]
     
  B) Prefix Compression:
     [key_len][prefix_len][suffix][value_len][value]
     Where prefix_len refers to previous key's prefix
     
  C) Trie-Based Encoding:
     [byte_to_insert][value_if_end][child_ptr]
     More complex but optimal compression
```

#### 3. Index Structure

```
Trie Index in SSTable:

┌─────────────────────────────────────┐
│      Index Block                    │
├─────────────────────────────────────┤
│ Trie representation:                │
│                                     │
│  root → "a" → "c" → $               │
│        ├─ pointer: block 0, off 100 │
│        └─ "d" → $                   │
│           pointer: block 1, off 50  │
│                                     │
│  root → "b" → "d" → $               │
│        └─ pointer: block 2, off 200 │
│                                     │
│ Each leaf of index trie points to   │
│ location of full key-value pair     │
└─────────────────────────────────────┘
```

### Advantages Over Pure LSM

```
Aspect          | LSM Tree      | LSM-Trie
─────────────────────────────────────────
Prefix query    | O(n log n)    | O(matches)
Range queries   | O(log n+m)    | O(log n+m) faster
Key compression | None          | Shared prefixes
Autocomplete    | N/A           | Native support
Memory per key  | Fixed         | Variable (amortized less)
Scan efficiency | Linear        | Linear (better cache)
Write amp       | Same          | Same
─────────────────────────────────────────
```

### Disadvantages & Trade-offs

```
Trade-off 1: Node Overhead
  - Trie: Each character needs a node or trie path
  - Cost: 20-100 bytes per node
  - Mitigation: Radix/Compressed Trie, HAT-Trie

Trade-off 2: Cache Locality
  - Trie: Pointer chasing (poor L1/L2 cache)
  - Cost: Slower traversal per node
  - Mitigation: Cache-friendly layouts, array-based tries

Trade-off 3: Implementation Complexity
  - Trie: More complex than B-Tree
  - Cost: Harder to optimize, debug, maintain
  - Mitigation: Use proven libraries

Trade-off 4: Alphabet Size
  - Large alphabets (256+ chars): Sparse trees
  - Cost: Wasted space in node children arrays
  - Mitigation: Hash-based children, HAT-Trie
```

---

## Core Concepts and Algorithms

### Write Path Algorithm

```
LSM-TRIE-WRITE(key, value):
  1. Validate input
     - key non-empty
     - key within size limits
  
  2. WAL (Write-Ahead Log)
     - Append <timestamp, key, value> to WAL file
     - Fsync to disk (durability)
     - If fsync fails: reject write
  
  3. Insert into Memtable Trie
     node = memtable.root
     FOR each char c IN key:
       IF c not in node.children:
         node.children[c] = new TrieNode()
       node = node.children[c]
     node.value = value
     node.is_end_of_word = true
     memtable.size += key.len + value.len
  
  4. Check for flush trigger
     IF memtable.size >= memtable.max_size:
       SCHEDULE-FLUSH-MEMTABLE()
  
  5. Acknowledge write
     RETURN success
  
TIME: O(m + disk_fsync) where m = key length
```

### Read Path Algorithm

```
LSM-TRIE-READ(key):
  1. Check memtable (current)
     result = READ-TRIE(memtable, key)
     IF result != nil:
       RETURN result
     
  2. Check immutable memtables (in order)
     FOR each imm_memtable IN immutable_memtables:
       result = READ-TRIE(imm_memtable, key)
       IF result != nil:
         RETURN result
  
  3. Check SSTables by level
     FOR level = 0 TO max_level:
       SSTables = GET-SSTABLES(level)
       
       IF level == 0:
         // Level 0: overlapping, check all
         FOR each sstable IN SSTables:
           IF bloom-filter-check(sstable, key):
             result = READ-SSTABLE(sstable, key)
             IF result != nil:
               RETURN result
       
       ELSE:
         // Levels 1+: sorted, binary search
         sstable = FIND-SSTABLE-FOR-KEY(SSTables, key)
         IF sstable exists:
           result = READ-SSTABLE(sstable, key)
           IF result != nil:
             RETURN result
  
  4. Not found
     RETURN nil

TIME: O(m + read_amplification*log(sstable_size))
       where m = key length
```

### Range Query Algorithm

```
LSM-TRIE-RANGE-QUERY(prefix):
  results = []
  
  1. Collect from memtable
     COLLECT-PREFIX(memtable, prefix, results)
  
  2. Collect from immutable memtables
     FOR each imm_memtable IN immutable_memtables:
       COLLECT-PREFIX(imm_memtable, prefix, results)
  
  3. Collect from SSTables
     FOR level = 0 TO max_level:
       SSTables = GET-SSTABLES(level)
       FOR each sstable IN SSTables:
         COLLECT-PREFIX-FROM-SSTABLE(sstable, prefix, results)
  
  4. Deduplicate and sort results
     RETURN UNIQUE(results, key_order=DESC_TIMESTAMP)

COLLECT-PREFIX(trie, prefix, results):
  // Navigate to prefix endpoint
  node = trie.root
  FOR each char c IN prefix:
    IF c not in node.children:
      RETURN  // Prefix not found
    node = node.children[c]
  
  // DFS to collect all matching keys
  DFS-COLLECT(node, prefix, results)

DFS-COLLECT(node, current_key, results):
  IF node.is_end_of_word AND node.value != nil:
    results.append((current_key, node.value))
  
  FOR each (char c, child) IN node.children:
    DFS-COLLECT(child, current_key + c, results)

TIME: O(m + k*log(sstable_size) + match_count)
       where m = prefix length
             k = number of SSTables checked
             match_count = matching keys
```

### Compaction Algorithm (Tiered Compaction)

```
COMPACT-MEMTABLE-TO-L0():
  1. Create immutable copy of memtable
     imm = memtable.freeze()
     memtable = new TrieMemtable()
  
  2. Serialize immutable memtable to disk
     sstable = new SSTable()
     FOR each (key, value) IN DFS-TRAVERSE(imm):
       sstable.append(key, value)
     sstable.finalize()  // Add index, bloom filter, footer
     sstable.write_to_disk(L0_path)
  
  3. Update level metadata
     level0_sstables.append(sstable)
  
  4. Check if compaction needed
     IF level0_sstables.count >= LEVEL0_THRESHOLD:
       SCHEDULE-COMPACT-L0-TO-L1()

COMPACT-LEVEL-I-TO-LEVEL-J(i, j):
  PRECONDITION: i < j (typically j = i+1)
  
  1. Select SSTables to compact
     l_sstables = SELECT-SSTABLES-FOR-COMPACTION(level_i)
     r_sstables = FIND-OVERLAPPING-SSTABLES(level_j, l_sstables)
  
  2. Merge input SSTables
     merged_iterator = MERGE-ITERATORS(l_sstables + r_sstables)
     
  3. Build merged SSTable
     sstable_new = new SSTable()
     FOR each (key, value) IN merged_iterator:
       // Skip deleted entries (tombstones)
       IF value != DELETED:
         sstable_new.append(key, value)
     sstable_new.finalize()
  
  4. Write output and update metadata
     sstable_new.write_to_disk(level_j_path)
     level_j_sstables.append(sstable_new)
     level_i_sstables.remove(l_sstables)

TIME: O(N log N) where N = total bytes in input SSTables
      Amortized write amplification: ~10-50x

MERGE-ITERATORS(sstables):
  // K-way merge with trie structure
  iterators = [sstable.iterator() for sstable in sstables]
  heap = MinHeap()
  
  // Initialize heap with first key from each iterator
  FOR each it IN iterators:
    IF it.next():
      heap.push((it.current_key, it))
  
  WHILE heap not empty:
    (key, iterator) = heap.pop()
    value = iterator.value
    YIELD (key, value)
    
    IF iterator.next():
      heap.push((iterator.current_key, iterator))

TIME: O(N log K) where K = number of SSTables
```

### Deletion (Tombstone) Algorithm

```
LSM-TRIE-DELETE(key):
  1. Write tombstone to WAL
     Append <DELETE, timestamp, key> to WAL
     Fsync to disk
  
  2. Insert tombstone into memtable
     node = memtable.root
     FOR each char c IN key:
       IF c not in node.children:
         node.children[c] = new TrieNode()
       node = node.children[c]
     node.value = TOMBSTONE_MARKER
     node.is_end_of_word = true
  
  3. Future reads skip tombstones
     UNTIL garbage collection runs
  
  4. Compaction removes tombstones
     When merging levels:
     FOR each (key, value) IN merged:
       IF value == TOMBSTONE_MARKER:
         SKIP  // Don't write to output
       ELSE:
         WRITE to output sstable

TIME: O(m + disk_fsync) where m = key length
```

### Merge Iterator (K-Way Merge)

```
Scenario: Merging 3 SSTables with Trie encoding

SSTable 1 (L0):     SSTable 2 (L0):    SSTable 3 (L1):
"apple"→v1          "apricot"→v2       "axle"→v4
"avocado"→v3        "apt"→v5           "banana"→v6
                                       "band"→v7

K-Way Merge Process:

Step 1: Create iterators for each SSTable
  it1 = SSTable1.iterator()  → points to "apple"
  it2 = SSTable2.iterator()  → points to "apricot"
  it3 = SSTable3.iterator()  → points to "axle"

Step 2: Min-heap based on keys (lexicographic order)
  Heap:
    "apple" (it1)
    "apricot" (it2)
    "apt" (it2)
    "avocado" (it1)
    "axle" (it3)
    "banana" (it3)
    "band" (it3)

Step 3: Output in order (heap.pop → advance iterator)
  Output: "apple"→v1, "apricot"→v2, "apt"→v5, 
          "avocado"→v3, "axle"→v4, "banana"→v6, "band"→v7

Result: Single sorted SSTable on Level 1

Efficiency:
- Time: O(N log K) where N=total keys, K=num iterators
- Space: O(K) for heap
- I/O: Sequential for each SSTable
```

---

## Memory Management and Persistence

### Memory Hierarchy

```
┌─────────────────────────────────────────────────────────┐
│            Memory Hierarchy in LSM-Trie                  │
└─────────────────────────────────────────────────────────┘

L1 Cache (32KB)
  ↑ (best: index metadata, small nodes)
  
L2 Cache (256KB)
  ↑ (trie nodes for active traversal)
  
L3 Cache (8MB)
  ↑ (memtable, frequently accessed SSTable index)
  
RAM (8-64GB)
  ├─ Memtable Trie (hot data)
  ├─ Index blocks (from SSTables)
  ├─ Block cache (decompressed blocks)
  └─ Bloom filters
  
Disk (NVMe/SSD/HDD)
  ├─ WAL files
  ├─ SSTables (compressed)
  └─ Metadata
```

### Memtable Memory Management

```
Memtable Lifecycle:

1. Creation
   Size: 0 bytes
   Status: Receiving writes
   ```
   Memtable (Active)
   ├─ root: TrieNode
   ├─ size: 0
   └─ max_size: 2MB
   ```

2. Growth
   Each insert: size += key.len + value.len
   Node allocation: ~20-100 bytes per character
   ```
   After 100K inserts:
   size ≈ 1.5MB
   nodes ≈ 500K
   ```

3. Flush Trigger
   IF size >= max_size:
     FREEZE current memtable
     CREATE new memtable
     SCHEDULE flush to disk

4. Frozen State
   ```
   Immutable Memtable
   ├─ Can be read
   ├─ Cannot be written
   └─ Being serialized to disk
   
   New Memtable (Active)
   ├─ Receiving new writes
   ├─ Empty state
   └─ Will eventually freeze
   ```

5. Flush to Disk
   DFS traversal: O(nodes + characters)
   Compression: 50-80% reduction
   Write latency: 1-10ms (SSD)

6. Deletion
   Once flushed and compacted into later levels:
   Free all node allocations
   Free Trie root

Memory-Optimized Memtable Implementation:

┌──────────────────────────────────┐
│ Node Structure (24 bytes/node)   │
├──────────────────────────────────┤
│ children: HashMap or Array       │
│   └─ 8 bytes (pointer)           │
│                                  │
│ is_end_of_word: bool             │
│   └─ 1 byte                      │
│                                  │
│ value_ptr: *Value                │
│   └─ 8 bytes (pointer)           │
│                                  │
│ padding/metadata: 7 bytes        │
└──────────────────────────────────┘

Alternative (Compressed Radix Trie - 50% reduction):
┌──────────────────────────────────┐
│ Node Structure (12 bytes/node)   │
├──────────────────────────────────┤
│ label: &str (pointer to string)  │
│   └─ 8 bytes                     │
│                                  │
│ children: &[Node]                │
│   └─ 4 bytes (array ref)         │
└──────────────────────────────────┘
```

### Persistence: Write-Ahead Log (WAL)

```
WAL File Format:

┌─────────────────────────────────────────────────────┐
│          WAL Entry Structure                        │
├─────────────────────────────────────────────────────┤
│ Magic: "WLOG" (4 bytes)                             │
│ Version: 1 (4 bytes)                                │
│ ─────────────────────────────────────────────────── │
│ Entry 0:                                            │
│   OpType: PUT (1 byte)                              │
│   Timestamp: 1624512000 (8 bytes)                   │
│   KeyLen: 5 (4 bytes) → "hello"                     │
│   Key: [0x68, 0x65, ...] (5 bytes)                  │
│   ValueLen: 6 (4 bytes) → "world!"                  │
│   Value: [0x77, 0x6f, ...] (6 bytes)                │
│   Checksum: CRC32 (4 bytes)                         │
│ ─────────────────────────────────────────────────── │
│ Entry 1:                                            │
│   OpType: DELETE (1 byte)                           │
│   Timestamp: 1624512001 (8 bytes)                   │
│   KeyLen: 5 (4 bytes) → "hello"                     │
│   Key: [0x68, 0x65, ...] (5 bytes)                  │
│   Checksum: CRC32 (4 bytes)                         │
│ ─────────────────────────────────────────────────── │
│ ...                                                 │
│ Footer:                                             │
│   NumEntries: 1000 (4 bytes)                        │
│   LastTimestamp: 1624512100 (8 bytes)               │
│   FileChecksum: CRC32(entire file) (4 bytes)        │
└─────────────────────────────────────────────────────┘
```

**Write Flow:**
```
User writes K=v
  ↓
Append to WAL buffer (in-memory)
  ↓ (on timeout or buffer full)
Fsync WAL buffer to disk
  ↓
ON CRASH:
  Recovery reads WAL from last checkpoint
  Replays all operations since checkpoint
  Recovers lost in-memory state
```

### SSTable Persistence

```
SSTable File Format (Block-Based):

┌────────────────────────────────────────────┐
│         SSTable File                       │
├────────────────────────────────────────────┤
│                                            │
│  Data Blocks (4KB each)                    │
│  ┌─────────────────────────────┐           │
│  │ Block 0: Key-Value Pairs    │           │
│  │  [compressed data]          │           │
│  │  Size: 4096 bytes           │           │
│  └─────────────────────────────┘           │
│  ┌─────────────────────────────┐           │
│  │ Block 1: Key-Value Pairs    │           │
│  │  [compressed data]          │           │
│  │  Size: 4096 bytes           │           │
│  └─────────────────────────────┘           │
│  ┌─────────────────────────────┐           │
│  │ Block N: Key-Value Pairs    │           │
│  └─────────────────────────────┘           │
│                                            │
│  Meta Block (Index)                        │
│  ┌─────────────────────────────┐           │
│  │ Trie Index:                 │           │
│  │  "apple"→Block0, Offset100  │           │
│  │  "apricot"→Block1, Offset50 │           │
│  │  ...                        │           │
│  └─────────────────────────────┘           │
│                                            │
│  Bloom Filter Block                        │
│  ┌─────────────────────────────┐           │
│  │ Bit array for membership    │           │
│  │ test (fast negative lookup) │           │
│  └─────────────────────────────┘           │
│                                            │
│  Footer                                    │
│  ┌─────────────────────────────┐           │
│  │ Magic: "SSTABLE" (8 bytes)  │           │
│  │ Version: 2 (4 bytes)        │           │
│  │ Index offset (8 bytes)      │           │
│  │ Filter offset (8 bytes)     │           │
│  │ Compression type (1 byte)   │           │
│  │ File checksum (4 bytes)     │           │
│  └─────────────────────────────┘           │
│                                            │
└────────────────────────────────────────────┘
```

### Compression Strategies

```
Compression applied at SSTable level:

1. Key Compression (within block)
   Previous key: "application"
   Current key:  "apple"
   
   Encode as:
   [shared_prefix_len: 2]
   [suffix: "le"]
   Savings: ~40-60% for sorted keys

2. Block Compression (full block)
   Options:
   - Snappy: Fast (100-300 MB/s), ~50-60% ratio
   - LZ4: Very fast (500+ MB/s), ~40-50% ratio
   - Zstd: Balanced (100+ MB/s), ~60-70% ratio
   - None: No compression, fastest read
   
   Trade-off: Compression vs read latency

3. Trie-Based Compression
   Store only character deltas:
   Instead of full keys in block:
   [char_byte][child_ptr][value_if_end]
   
   Reduces key redundancy significantly

Compression Example:
Before:  1000 keys × 20 bytes avg = 20,000 bytes
After:   Snappy compression = 10,000 bytes (50% ratio)
         Decompression: ~10μs (CPU cache friendly)
```

---

## Protocols and Operation Flow

### Write Protocol (Detailed)

```
USER WRITE REQUEST
  write(key="user_id:123:name", value="Alice")
         ↓
    ┌────────────────────────────┐
    │ Input Validation           │
    ├────────────────────────────┤
    │ ✓ Key non-empty            │
    │ ✓ Key ≤ max_key_size       │
    │ ✓ Value ≤ max_value_size   │
    │ ✓ Timestamp valid          │
    │                            │
    │ If validation fails:       │
    │   RETURN error             │
    └────────────────────────────┘
         ↓
    ┌────────────────────────────────────────┐
    │ Write-Ahead Log (WAL) Append           │
    ├────────────────────────────────────────┤
    │ Serialize to binary:                   │
    │  [OP_PUT] [ts] [klen] [key] [vlen]    │
    │  [value] [crc32]                       │
    │                                        │
    │ Append to in-memory WAL buffer         │
    │ IF buffer full OR timeout:             │
    │   fsync(wal_file)  → durable           │
    │                                        │
    │ On fsync failure:                      │
    │   RETURN error (write NOT applied)     │
    └────────────────────────────────────────┘
         ↓ (success → durability guaranteed)
    ┌────────────────────────────┐
    │ Memtable Trie Insertion    │
    ├────────────────────────────┤
    │ curr_node = memtable.root  │
    │                            │
    │ FOR char 'u' IN "user":    │
    │   IF 'u' not in children:  │
    │     children['u'] = NEW    │
    │   curr_node = children['u']│
    │                            │
    │ ... (repeat for each char) │
    │                            │
    │ curr_node.value = "Alice"  │
    │ curr_node.is_end = true    │
    │                            │
    │ memtable.size += 23 bytes  │
    │ (key + value length)       │
    └────────────────────────────┘
         ↓
    ┌──────────────────────────────┐
    │ Check Flush Condition        │
    ├──────────────────────────────┤
    │ IF memtable.size ≥ 2MB:      │
    │   CREATE imm_memtable =      │
    │     memtable.freeze()        │
    │   CREATE new memtable        │
    │   SPAWN background task:     │
    │     FLUSH(imm_memtable)      │
    │                              │
    │ Future writes go to new      │
    │ memtable (no blocking)       │
    └──────────────────────────────┘
         ↓
    ┌──────────────────────┐
    │ Acknowledge to User  │
    ├──────────────────────┤
    │ RETURN success       │
    │                      │
    │ Timing:              │
    │ - Write latency:     │
    │   μs (memtable only) │
    │ - Durability:        │
    │   Post-WAL fsync     │
    │ - WAL delay:         │
    │   10-100μs (SSD)     │
    └──────────────────────┘

BACKGROUND: Flush Memtable
  ┌──────────────────────────────────────┐
  │ Flush Immutable Memtable             │
  ├──────────────────────────────────────┤
  │ 1. DFS traverse imm_memtable         │
  │    Collect (key, value) pairs sorted │
  │                                      │
  │ 2. Build SSTable                     │
  │    FOR each (key, value):            │
  │      sstable.append(key, value)      │
  │                                      │
  │ 3. Add index                         │
  │    Create Trie for keys              │
  │    Map key → file offset             │
  │                                      │
  │ 4. Add Bloom filter                  │
  │    Add false-positive filter         │
  │                                      │
  │ 5. Compress blocks                   │
  │    Apply Snappy/LZ4/Zstd             │
  │    Typical: 50-70% ratio             │
  │                                      │
  │ 6. Write to disk                     │
  │    sstable.write(L0_path)            │
  │    fsync() for durability            │
  │                                      │
  │ 7. Update metadata                   │
  │    Add to level_0_sstables list      │
  │                                      │
  │ 8. Trigger compaction?               │
  │    IF len(L0_sstables) ≥ 4:          │
  │      SCHEDULE(compact L0→L1)         │
  └──────────────────────────────────────┘
  
Duration: 50-500ms (depending on size, disk speed)
```

### Read Protocol (Detailed)

```
USER READ REQUEST
  get(key="user_id:123:name")
         ↓
    ┌────────────────────────────┐
    │ Input Validation           │
    ├────────────────────────────┤
    │ ✓ Key non-empty            │
    │ ✓ Key ≤ max_key_size       │
    │                            │
    │ If validation fails:       │
    │   RETURN nil, error        │
    └────────────────────────────┘
         ↓
    ┌───────────────────────────────────────┐
    │ Check Memtable (L0 - in memory)       │
    ├───────────────────────────────────────┤
    │ curr_node = memtable.root             │
    │                                       │
    │ FOR char c IN key:                    │
    │   IF c not in curr_node.children:     │
    │     GOTO check_immutable              │
    │   curr_node = curr_node.children[c]   │
    │                                       │
    │ IF curr_node.is_end AND value ≠ nil:  │
    │   RETURN curr_node.value              │
    │   T: ~5-20 CPU cycles (L1/L2 cache)   │
    └───────────────────────────────────────┘
         ↓ (not found)
    ┌────────────────────────────────────────┐
    │ Check Immutable Memtables              │
    ├────────────────────────────────────────┤
    │ FOR each imm_table IN immutable_list:  │
    │   [same traversal as above]            │
    │   IF found: RETURN value               │
    │                                        │
    │ T: O(m) per immutable memtable         │
    │    where m = key length                │
    │ Usually 0-2 immutable tables           │
    └────────────────────────────────────────┘
         ↓ (not found)
    ┌────────────────────────────────────────┐
    │ Check Level 0 SSTables                 │
    ├────────────────────────────────────────┤
    │ L0_sstables = GET_L0_SSTABLES()        │
    │                                        │
    │ FOR each sstable IN L0_sstables:       │
    │   ┌──────────────────────────────────┐ │
    │   │ Bloom Filter Check (fast path)   │ │
    │   ├──────────────────────────────────┤ │
    │   │ IF bloom_filter.mightExist(key): │ │
    │   │   proceed to binary search       │ │
    │   │ ELSE:                            │ │
    │   │   skip this sstable              │ │
    │   │ T: ~10-50ns per check            │ │
    │   └──────────────────────────────────┘ │
    │                                        │
    │   IF might exist:                      │
    │   ┌──────────────────────────────────┐ │
    │   │ Index Block Search (Trie)        │ │
    │   ├──────────────────────────────────┤ │
    │   │ Traverse index trie to find key  │ │
    │   │ Returns: block_id, offset        │ │
    │   │ T: O(m) + index lookup           │ │
    │   └──────────────────────────────────┘ │
    │                                        │
    │   IF index found:                      │
    │   ┌──────────────────────────────────┐ │
    │   │ Load Data Block                  │ │
    │   ├──────────────────────────────────┤ │
    │   │ 1. Check block cache             │ │
    │   │    IF in cache: use cached       │ │
    │   │    ELSE: read from disk          │ │
    │   │ 2. Decompress block              │ │
    │   │ 3. Scan for key in block         │ │
    │   │ T: 1-50μs (cache hit/miss)      │ │
    │   └──────────────────────────────────┘ │
    │                                        │
    │   IF key found: RETURN value           │
    │                                        │
    │ T: O(log L0_count) + I/O for disk      │
    └────────────────────────────────────────┘
         ↓ (not found)
    ┌────────────────────────────────────────┐
    │ Check Levels 1+ (Sorted SSTables)      │
    ├────────────────────────────────────────┤
    │ FOR level = 1 TO max_level:            │
    │   L_sstables = GET_LEVEL_SSTABLES()    │
    │                                        │
    │   // Key ranges are non-overlapping    │
    │   sstable = BINARY_SEARCH(             │
    │     L_sstables,                        │
    │     key,                               │
    │     by_key_range                       │
    │   )  T: O(log count)                   │
    │                                        │
    │   IF sstable found:                    │
    │     [same bloom + index + block read]  │
    │     IF found: RETURN value             │
    │                                        │
    │ After checking all levels:             │
    │   Key not found                        │
    └────────────────────────────────────────┘
         ↓
    ┌──────────────────────┐
    │ Return Not Found     │
    ├──────────────────────┤
    │ RETURN nil           │
    │                      │
    │ Total Read Latency:  │
    │ - Best case:  5μs    │
    │   (memtable cache    │
    │    hit, L3 cache)    │
    │ - Avg case:   50-100μs
    │   (L0 SSTable or     │
    │    L1 single read)   │
    │ - Worst case: 1-10ms │
    │   (multiple levels,  │
    │    disk seek)        │
    └──────────────────────┘
```

### Range Query Protocol (Detailed)

```
USER RANGE QUERY REQUEST
  scan(prefix="user_id:123")
         ↓
    ┌────────────────────────────────┐
    │ Input Validation               │
    ├────────────────────────────────┤
    │ ✓ Prefix non-empty             │
    │ ✓ Prefix ≤ max_key_size        │
    └────────────────────────────────┘
         ↓
    ┌────────────────────────────────────────┐
    │ Scan Memtable (In-Memory Trie)         │
    ├────────────────────────────────────────┤
    │ 1. Navigate to prefix endpoint:        │
    │    curr_node = root                    │
    │    FOR char c IN prefix:               │
    │      IF c not in children:             │
    │        RETURN []  // No matches        │
    │      curr_node = children[c]           │
    │                                        │
    │ 2. DFS collect all keys:               │
    │    results = []                        │
    │    DFS(curr_node, prefix, results)     │
    │                                        │
    │ 3. Sort by timestamp (descending)      │
    │    SORT(results, reverse=true)         │
    │                                        │
    │ T: O(m + k) where m=prefix length,     │
    │              k=matching keys           │
    └────────────────────────────────────────┘
         ↓
    ┌────────────────────────────────────────┐
    │ Scan Immutable Memtables               │
    ├────────────────────────────────────────┤
    │ FOR each imm_table:                    │
    │   [same DFS process]                   │
    │   APPEND results to main list          │
    │                                        │
    │ T: O(m + k) per immutable table        │
    └────────────────────────────────────────┘
         ↓
    ┌────────────────────────────────────────┐
    │ Scan Level 0 SSTables                  │
    ├────────────────────────────────────────┤
    │ FOR each sstable IN level_0:           │
    │   ┌──────────────────────────────────┐ │
    │   │ Quick Prefix Check               │ │
    │   ├──────────────────────────────────┤ │
    │   │ IF key_min > prefix + "zzz..":   │ │
    │   │   SKIP sstable                   │ │
    │   │ IF key_max < prefix:             │ │
    │   │   SKIP sstable                   │ │
    │   └──────────────────────────────────┘ │
    │                                        │
    │   IF possible overlap:                 │
    │   ┌──────────────────────────────────┐ │
    │   │ Range Scan in SSTable            │ │
    │   ├──────────────────────────────────┤ │
    │   │ 1. Find prefix in index trie     │ │
    │   │    node = index_trie.root        │ │
    │   │    FOR char c IN prefix:        │ │
    │   │      node = children[c]          │ │
    │   │                                  │
    │   │ 2. DFS from that node to         │ │
    │   │    collect all keys              │ │
    │   │                                  │
    │   │ 3. For each key, load block      │ │
    │   │    from disk and extract value   │ │
    │   │                                  │
    │   │ 4. Append to results             │ │
    │   │ T: O(log blocks + I/O + matches) │ │
    │   └──────────────────────────────────┘ │
    │                                        │
    │ Total L0: O(#L0 * (m + k))             │
    └────────────────────────────────────────┘
         ↓
    ┌────────────────────────────────────────┐
    │ Scan Levels 1+ (Sorted SSTables)       │
    ├────────────────────────────────────────┤
    │ FOR level = 1 TO max:                  │
    │   // Only 1 sstable should match       │
    │   sstable = FIND_BY_KEY_RANGE(prefix)  │
    │                                        │
    │   IF sstable found:                    │
    │     [same range scan as L0]            │
    │                                        │
    │ T: O(m + k + I/O per level)            │
    └────────────────────────────────────────┘
         ↓
    ┌───────────────────────────────────────┐
    │ Merge and Deduplicate Results         │
    ├───────────────────────────────────────┤
    │ All results collected above            │
    │                                       │
    │ 1. Group by key                       │
    │ 2. For duplicates, keep latest value: │
    │    (highest timestamp wins)           │
    │ 3. Sort results                       │
    │ 4. Filter tombstones                  │
    │    (if value == DELETED: skip)        │
    │                                       │
    │ T: O(k log k) where k=results         │
    └───────────────────────────────────────┘
         ↓
    ┌──────────────────────────────────────┐
    │ Return Results to User                │
    ├──────────────────────────────────────┤
    │ RETURN [results]                      │
    │                                       │
    │ Characteristics:                      │
    │ - Sorted by key                       │
    │ - Deduplicated                        │
    │ - Latest values only                  │
    │ - Tombstones filtered                 │
    │                                       │
    │ Latency:                              │
    │ - Empty result: 1-5μs                 │
    │ - 100 matches: 10-100μs               │
    │ - 1000 matches: 100μs-1ms            │
    │ - Large scans: 1-100ms                │
    │   (disk dependent)                    │
    └──────────────────────────────────────┘
```

### Compaction Protocol (Detailed)

```
BACKGROUND COMPACTION PROCESS

Trigger: Level 0 has ≥4 SSTables
         └─ Detected after flush_memtable completes
         ↓
    ┌──────────────────────────────────┐
    │ Schedule Compaction Job          │
    ├──────────────────────────────────┤
    │ SPAWN background thread/task     │
    │ Non-blocking to user operations  │
    │                                  │
    │ Priority:                        │
    │ - High: L0 compaction            │
    │ - Medium: L1→L2                  │
    │ - Low: later levels              │
    └──────────────────────────────────┘
         ↓
    ┌──────────────────────────────────────────┐
    │ Select Compaction Candidate              │
    ├──────────────────────────────────────────┤
    │ Strategy: Tiered Compaction              │
    │                                          │
    │ IF L0 overlapping SSTables ≥4:           │
    │   SELECT: All L0 SSTables                │
    │   MERGE: With overlapping L1 SSTables    │
    │   TARGET: L1                             │
    │ ELSE IF L1 size ≥ limit (10MB):          │
    │   SELECT: Largest SSTable in L1          │
    │   MERGE: With overlapping L2 SSTables    │
    │   TARGET: L2                             │
    │ ELSE:                                    │
    │   RESCHEDULE later                       │
    │                                          │
    │ Input Set:                               │
    │   L0: [SST0, SST1, SST2, SST3] (8MB)     │
    │   L1: [SST4, SST5] (10MB overlap)        │
    │   Total input: 18MB                      │
    └──────────────────────────────────────────┘
         ↓
    ┌──────────────────────────────────────┐
    │ Acquire Read Locks                   │
    ├──────────────────────────────────────┤
    │ Lock all input SSTables to prevent:  │
    │ - Deletion during compaction        │
    │ - Opening new readers               │
    │                                     │
    │ Existing readers can continue       │
    │ (lock is on SSTable metadata, not   │
    │  data blocks)                       │
    └──────────────────────────────────────┘
         ↓
    ┌──────────────────────────────────────────┐
    │ Create K-Way Merge Iterator              │
    ├──────────────────────────────────────────┤
    │ Input: 6 SSTables                        │
    │                                          │
    │ 1. Open iterator for each SSTable        │
    │    it0 = SST0.iterator()                 │
    │    it1 = SST1.iterator()                 │
    │    ...                                   │
    │    it5 = SST5.iterator()                 │
    │                                          │
    │ 2. Create min-heap on keys               │
    │    heap.push((it0.key(), it0))           │
    │    ...                                   │
    │    heap.push((it5.key(), it5))           │
    │                                          │
    │ 3. K-way merge loop                      │
    │    WHILE heap not empty:                 │
    │      (key, iterator) = heap.pop()        │
    │      value = iterator.value              │
    │      ts = iterator.timestamp             │
    │      YIELD (key, value, ts)              │
    │      IF iterator.next():                 │
    │        heap.push((it.key(), it))         │
    │                                          │
    │ Output: Stream of sorted (K,V,ts) tuples │
    │ T: O(N log K) where N=total entries,     │
    │                    K=6 iterators         │
    └──────────────────────────────────────────┘
         ↓
    ┌────────────────────────────────────┐
    │ Build Output SSTable               │
    ├────────────────────────────────────┤
    │ FOR each (key, value, ts) from     │
    │     K-way merge:                   │
    │                                    │
    │   1. Skip if tombstone             │
    │      IF value == DELETE_MARKER:    │
    │        CONTINUE                    │
    │                                    │
    │   2. Deduplicate                   │
    │      IF seen_key(key) before:      │
    │        SKIP (newer version already │
    │        in output)                  │
    │                                    │
    │   3. Append to output SSTable      │
    │      sstable_out.append(K, V, ts)  │
    │                                    │
    │   4. Accumulate size               │
    │      output_size += K.len + V.len  │
    │                                    │
    │   5. Trigger block wrap if full    │
    │      IF block_size ≥ 4KB:          │
    │        output_blocks.push(block)   │
    │        block = new()               │
    │                                    │
    │ Result: Sorted, deduplicated data  │
    │ Size: 18MB → 10MB (55% after       │
    │       dedup + compression)         │
    └────────────────────────────────────┘
         ↓
    ┌──────────────────────────────────────┐
    │ Build Index and Metadata            │
    ├──────────────────────────────────────┤
    │ 1. Create Trie from all output keys │
    │    index_trie = new Trie()          │
    │    FOR each key IN output_keys:     │
    │      index_trie.insert(key)         │
    │    ┌────────────────────────────┐   │
    │    │ Index Trie:                │   │
    │    │  "apple"→block0,offset10   │   │
    │    │  "apricot"→block1,offset20 │   │
    │    │  "banana"→block2,offset100 │   │
    │    │  "band"→block2,offset150   │   │
    │    └────────────────────────────┘   │
    │                                     │
    │ 2. Encode index to compact form     │
    │    Serialize index trie             │
    │    Save block offsets               │
    │                                     │
    │ 3. Build Bloom filter               │
    │    bf = new BloomFilter()           │
    │    FOR each key:                    │
    │      bf.add(key)                    │
    │    FPR: ~1% (tuned for usage)       │
    │                                     │
    │ 4. Create footer                    │
    │    Footer {                         │
    │      version: 2,                    │
    │      index_offset: 45000,           │
    │      filter_offset: 48000,          │
    │      min_key: "apple",              │
    │      max_key: "zone",               │
    │      num_entries: 15000,            │
    │      compression: SNAPPY,           │
    │      timestamp: now(),              │
    │      checksum: crc32(...)           │
    │    }                                │
    └──────────────────────────────────────┘
         ↓
    ┌──────────────────────────────────────┐
    │ Write SSTable to Disk                │
    ├──────────────────────────────────────┤
    │ Filename: L1_00042.sst               │
    │ Path: /data/level1/               │
    │                                      │
    │ Write sequence:                      │
    │ 1. Data blocks (10MB)                │
    │ 2. Index block (50KB)                │
    │ 3. Filter block (5KB)                │
    │ 4. Footer (4KB)                      │
    │                                      │
    │ Total: ~10.06MB                      │
    │                                      │
    │ Write latency: ~100-500ms            │
    │   (sequential writes to SSD)         │
    │                                      │
    │ Durability: fsync() after write      │
    └──────────────────────────────────────┘
         ↓
    ┌────────────────────────────────────────┐
    │ Update Metadata (Atomic)               │
    ├────────────────────────────────────────┤
    │ 1. Create new version                  │
    │    old_version = current_version       │
    │    new_version = version + 1           │
    │                                        │
    │ 2. Update SSTable lists                │
    │    new_version.L0 = [SST0, SST1]       │
    │    (remove SST0, SST1, SST2, SST3)     │
    │    new_version.L1 = [SST4, SST5,       │
    │                       SST_NEW]         │
    │    (add SST_NEW)                       │
    │                                        │
    │ 3. Make new version visible            │
    │    atomic_swap(current_version,        │
    │                new_version)            │
    │                                        │
    │ 4. Invalidate old readers              │
    │    Wait for in-flight readers to       │
    │    finish on old version               │
    │                                        │
    │ 5. Release read locks                  │
    │    Release locks on old SSTables       │
    └────────────────────────────────────────┘
         ↓
    ┌──────────────────────────────────────┐
    │ Cleanup Old SSTables                 │
    ├──────────────────────────────────────┤
    │ FOR each sstable IN old_L0:          │
    │   sstable.mark_for_deletion()        │
    │   background_cleanup.add(sstable)    │
    │                                      │
    │ Delayed deletion to ensure no        │
    │ in-flight readers using them         │
    │                                      │
    │ Freed disk space: 8MB                │
    └──────────────────────────────────────┘
         ↓
    ┌──────────────────────────────────────┐
    │ Check if Further Compaction Needed   │
    ├──────────────────────────────────────┤
    │ IF new_version.L1.size ≥ limit:      │
    │   SCHEDULE(compact L1→L2)             │
    │                                      │
    │ Cascade compactions if needed        │
    └──────────────────────────────────────┘

Compaction Statistics:
  Input bytes:  18MB
  Output bytes: 10MB
  Compression:  55% ratio
  Time taken:   500ms
  Write amp:    10MB / 2MB flush = 5x
```

---

## Performance Analysis

### Time Complexity Summary

```
Operation          | Best Case      | Average Case    | Worst Case
─────────────────────────────────────────────────────────────────────
Insert             | O(m)           | O(m)            | O(m)
Search             | O(m)           | O(m) + I/O      | O(m + L*I/O)
Range Query        | O(m+k)         | O(m+k+I/O)      | O(m+k+L*I/O)
Delete             | O(m)           | O(m)            | O(m)
Compaction         | O(N log K)     | O(N log K)      | O(N log K)

Where:
  m = key length (typically 10-100 bytes)
  k = result count
  L = number of levels (typically 5-10)
  I/O = disk read latency (~1-10ms)
  N = total keys in compaction
  K = number of files being merged (typically 2-20)
```

### Space Complexity

```
Component          | Space              | Notes
──────────────────────────────────────────────────────
Memtable           | O(k*m)             | k=entries, m=avg key len
Immutable memtable | O(k*m)             | Usually 1-2 per DB
SSTables (L0)      | O(memtable*count)  | Uncompressed: 2MB*4=8MB
SSTables (L1+)     | O(input * growth)  | Exponential: 10x per level
WAL files          | O(unflushed_data)  | Deleted after flush
Bloom filters      | O(k) bits          | ~10 bits per entry
Index blocks       | O(k*log m)         | Compact trie index
Value storage      | O(total_values)    | Depends on data

Total overhead above data:
  30-50% for compression, indexing, filters
```

### I/O Patterns

```
Write I/O Pattern:
  Sequential writes (optimal)
  ├─ WAL append: ~10KB/write
  ├─ Memtable flush: 2-4MB sequential
  └─ Compaction: 10-100MB+ sequential
  
  Total throughput: 100-1000 MB/s (SSD)
                    10-100 MB/s (HDD)

Read I/O Pattern (Variable):
  Best case: In-memory cache hit (~5ns)
  
  Typical case: SSD access
    ├─ Bloom filter check: 10-50ns
    ├─ Index block read: 10-100μs
    └─ Data block read: 10-100μs
    Total: 20-150μs per key
  
  Worst case: HDD seeks
    ├─ Seek latency: 5-10ms
    ├─ Rotational delay: 5ms
    └─ Transfer: 1-10ms
    Total: 10-20ms per random access
```

### Cache Behavior

```
L1 Cache (32KB, ~4 cycles):
  - Index metadata (hot)
  - Node pointers (during traversal)
  - Small Bloom filters

L2 Cache (256KB, ~12 cycles):
  - Trie nodes for active prefix
  - Compressed index blocks
  - Frequently accessed SSTable metadata

L3 Cache (8MB, ~40 cycles):
  - Decompressed data blocks
  - Complete memtable
  - Recently accessed SSTables

RAM (100+ cycles):
  - Block cache (user configurable, 10-100MB)
  - Full SSTables
  - Entire level for newer levels

Disk (1,000,000+ cycles):
  - Older SSTables
  - Rarely accessed data
```

### Benchmarks (Typical Hardware: Modern SSD, 8 cores)

```
Workload: 1M random writes, 1M random reads, 10M entries

Pure LSM Tree (B-Tree Memtable):
  Write throughput: 100K-500K ops/sec
  Read latency (avg): 100-500μs
  Range query (1000 keys): 10-50ms

LSM-Trie (Trie Memtable):
  Write throughput: 80K-400K ops/sec (slightly slower)
  Read latency (avg): 50-200μs (faster)
  Range query (1000 keys): 5-20ms (much faster)
  
  Trade-off: Write slightly slower, read/range much faster

Workload: String autocomplete (prefix queries)

LSM Tree:
  Prefix query (100 matches): 50-100ms

LSM-Trie:
  Prefix query (100 matches): 5-10ms
  → 5-10x faster for string workloads
```

---

## Rust Implementation

### Core Data Structures

```rust
use std::collections::HashMap;
use std::sync::{Arc, RwLock, Mutex};
use std::fs::File;
use std::io::{Write, Read, BufWriter, BufReader};

/// Trie node representing a single character position
#[derive(Debug, Clone)]
pub struct TrieNode<V> {
    /// Child nodes by character
    pub children: HashMap<u8, Box<TrieNode<V>>>,
    /// Value stored at this node (if it's a complete key)
    pub value: Option<V>,
    /// Whether this node marks end of a valid key
    pub is_end_of_word: bool,
}

impl<V: Clone> TrieNode<V> {
    pub fn new() -> Self {
        TrieNode {
            children: HashMap::new(),
            value: None,
            is_end_of_word: false,
        }
    }
    
    /// Check if this node has children
    pub fn has_children(&self) -> bool {
        !self.children.is_empty()
    }
}

/// Memtable: In-memory trie-based structure for recent writes
#[derive(Debug)]
pub struct Memtable<V> {
    /// Root of the trie
    root: Box<TrieNode<V>>,
    /// Current size in bytes (approximate)
    size: usize,
    /// Maximum size before flush
    max_size: usize,
    /// Creation timestamp
    created_at: u64,
}

impl<V: Clone> Memtable<V> {
    pub fn new(max_size: usize) -> Self {
        Memtable {
            root: Box::new(TrieNode::new()),
            size: 0,
            max_size,
            created_at: Self::timestamp(),
        }
    }
    
    /// Insert key-value pair
    pub fn insert(&mut self, key: &[u8], value: V) -> bool {
        let value_size = std::mem::size_of_val(&value);
        if self.size + key.len() + value_size >= self.max_size {
            return false; // Would exceed max size
        }
        
        let mut node = &mut self.root;
        
        // Navigate/create path
        for &byte in key {
            node = node
                .children
                .entry(byte)
                .or_insert_with(|| Box::new(TrieNode::new()));
        }
        
        // Store value
        node.is_end_of_word = true;
        node.value = Some(value);
        self.size += key.len() + value_size;
        
        true
    }
    
    /// Search for key
    pub fn get(&self, key: &[u8]) -> Option<V> {
        let mut node = &self.root;
        
        for &byte in key {
            match node.children.get(&byte) {
                Some(n) => node = n,
                None => return None,
            }
        }
        
        if node.is_end_of_word {
            node.value.clone()
        } else {
            None
        }
    }
    
    /// Range query: get all keys with prefix
    pub fn prefix_scan(&self, prefix: &[u8]) -> Vec<(Vec<u8>, V)> {
        let mut node = &self.root;
        
        // Navigate to prefix end
        for &byte in prefix {
            match node.children.get(&byte) {
                Some(n) => node = n,
                None => return Vec::new(),
            }
        }
        
        // Collect all keys with this prefix
        let mut results = Vec::new();
        let mut key = prefix.to_vec();
        Self::dfs_collect(node, &mut key, &mut results);
        results
    }
    
    /// Depth-first search to collect all keys
    fn dfs_collect<'a>(
        node: &'a TrieNode<V>,
        current_key: &mut Vec<u8>,
        results: &mut Vec<(Vec<u8>, V)>,
    ) {
        if node.is_end_of_word {
            if let Some(value) = &node.value {
                results.push((current_key.clone(), value.clone()));
            }
        }
        
        for (&byte, child) in &node.children {
            current_key.push(byte);
            Self::dfs_collect(child, current_key, results);
            current_key.pop();
        }
    }
    
    /// Check if flush is needed
    pub fn should_flush(&self) -> bool {
        self.size >= self.max_size
    }
    
    /// Get timestamp (milliseconds since epoch)
    fn timestamp() -> u64 {
        use std::time::{SystemTime, UNIX_EPOCH};
        SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_millis() as u64
    }
}

/// SSTable: Immutable sorted string table on disk
#[derive(Debug)]
pub struct SSTable {
    /// Path to file
    path: String,
    /// Minimum key in this file
    min_key: Vec<u8>,
    /// Maximum key in this file
    max_key: Vec<u8>,
    /// Number of entries
    num_entries: usize,
    /// File size in bytes
    file_size: usize,
    /// Bloom filter for fast negative lookup
    bloom_filter: BloomFilter,
    /// Memory-mapped index: key prefix → file offset
    index: HashMap<Vec<u8>, u64>,
}

/// Bloom filter for fast membership testing
#[derive(Debug, Clone)]
pub struct BloomFilter {
    /// Bit array
    bits: Vec<bool>,
    /// Number of hash functions
    k: usize,
}

impl BloomFilter {
    pub fn new(capacity: usize, fpr: f64) -> Self {
        // Optimal bit array size: -capacity * ln(fpr) / ln(2)^2
        let m = ((capacity as f64) * -(fpr.ln()) / 0.480453).ceil() as usize;
        // Optimal hash functions: m/capacity * ln(2)
        let k = ((m as f64 / capacity as f64) * 0.693147).ceil() as usize;
        
        BloomFilter {
            bits: vec![false; m],
            k,
        }
    }
    
    /// Add element to filter
    pub fn add(&mut self, key: &[u8]) {
        for i in 0..self.k {
            let hash = Self::hash(key, i as u64) as usize % self.bits.len();
            self.bits[hash] = true;
        }
    }
    
    /// Test if element might be in set (no false negatives)
    pub fn might_exist(&self, key: &[u8]) -> bool {
        for i in 0..self.k {
            let hash = Self::hash(key, i as u64) as usize % self.bits.len();
            if !self.bits[hash] {
                return false;
            }
        }
        true
    }
    
    /// Simple hash function
    fn hash(key: &[u8], seed: u64) -> u64 {
        let mut hash = seed;
        for &byte in key {
            hash = hash.wrapping_mul(31).wrapping_add(byte as u64);
        }
        hash
    }
}

/// LSM-Trie database
pub struct LSMTrieDB<V: Clone> {
    /// Active memtable
    memtable: Arc<RwLock<Memtable<V>>>,
    /// Immutable memtables waiting to be flushed
    immutable_memtables: Arc<Mutex<Vec<Arc<Memtable<V>>>>>,
    /// SSTables organized by level
    levels: Arc<RwLock<Vec<Vec<Arc<SSTable>>>>>,
    /// Configuration
    config: Config,
}

#[derive(Debug, Clone)]
pub struct Config {
    pub memtable_size: usize,
    pub num_levels: usize,
    pub level_size_ratio: usize,
}

impl Default for Config {
    fn default() -> Self {
        Config {
            memtable_size: 4 * 1024 * 1024, // 4MB
            num_levels: 7,
            level_size_ratio: 10,
        }
    }
}

impl<V: Clone + serde::Serialize + serde::de::DeserializeOwned> LSMTrieDB<V> {
    pub fn new(config: Config) -> Self {
        let mut levels = Vec::new();
        for _ in 0..config.num_levels {
            levels.push(Vec::new());
        }
        
        LSMTrieDB {
            memtable: Arc::new(RwLock::new(Memtable::new(config.memtable_size))),
            immutable_memtables: Arc::new(Mutex::new(Vec::new())),
            levels: Arc::new(RwLock::new(levels)),
            config,
        }
    }
    
    /// Write key-value pair
    pub fn put(&self, key: &[u8], value: V) -> Result<(), String> {
        // Try to insert into active memtable
        let mut mem = self.memtable.write().unwrap();
        
        if !mem.insert(key, value.clone()) {
            // Memtable full, need to flush
            drop(mem); // Release lock
            self.flush_memtable()?;
            
            // Retry insert
            let mut mem = self.memtable.write().unwrap();
            mem.insert(key, value)
                .then_some(())
                .ok_or_else(|| "Failed to insert after flush".to_string())?;
        }
        
        Ok(())
    }
    
    /// Read key-value pair
    pub fn get(&self, key: &[u8]) -> Option<V> {
        // Check active memtable
        {
            let mem = self.memtable.read().unwrap();
            if let Some(val) = mem.get(key) {
                return Some(val);
            }
        }
        
        // Check immutable memtables
        {
            let imms = self.immutable_memtables.lock().unwrap();
            for imm in imms.iter() {
                if let Some(val) = imm.get(key) {
                    return Some(val);
                }
            }
        }
        
        // Check SSTables (in real implementation)
        // For this example, we skip SSTable lookup
        None
    }
    
    /// Range query with prefix
    pub fn scan_prefix(&self, prefix: &[u8]) -> Vec<(Vec<u8>, V)> {
        let mut results = Vec::new();
        
        // Scan active memtable
        {
            let mem = self.memtable.read().unwrap();
            results.extend(mem.prefix_scan(prefix));
        }
        
        // Scan immutable memtables
        {
            let imms = self.immutable_memtables.lock().unwrap();
            for imm in imms.iter() {
                results.extend(imm.prefix_scan(prefix));
            }
        }
        
        // Deduplicate (keep first/latest)
        results.sort_by_key(|(k, _)| k.clone());
        results.dedup_by_key(|(k, _)| k.clone());
        results
    }
    
    /// Flush memtable to disk
    fn flush_memtable(&self) -> Result<(), String> {
        // Create immutable copy
        let mut mem = self.memtable.write().unwrap();
        
        if mem.size == 0 {
            return Ok(()); // Nothing to flush
        }
        
        let imm = Arc::new(Memtable::new(self.config.memtable_size));
        // In real implementation: swap memtable
        
        // Add to immutable list
        self.immutable_memtables.lock().unwrap().push(imm);
        
        // Create new active memtable
        *mem = Memtable::new(self.config.memtable_size);
        
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_memtable_insert_get() {
        let mut mem = Memtable::new(1024 * 1024);
        
        mem.insert(b"apple", 1);
        mem.insert(b"apricot", 2);
        mem.insert(b"banana", 3);
        
        assert_eq!(mem.get(b"apple"), Some(1));
        assert_eq!(mem.get(b"apricot"), Some(2));
        assert_eq!(mem.get(b"banana"), Some(3));
        assert_eq!(mem.get(b"cherry"), None);
    }
    
    #[test]
    fn test_prefix_scan() {
        let mut mem = Memtable::new(1024 * 1024);
        
        mem.insert(b"apple", 1);
        mem.insert(b"application", 2);
        mem.insert(b"apply", 3);
        mem.insert(b"banana", 4);
        
        let results = mem.prefix_scan(b"app");
        assert_eq!(results.len(), 3);
    }
    
    #[test]
    fn test_bloom_filter() {
        let mut bf = BloomFilter::new(1000, 0.01);
        
        bf.add(b"hello");
        bf.add(b"world");
        
        assert!(bf.might_exist(b"hello"));
        assert!(bf.might_exist(b"world"));
        // False positive possible
    }
    
    #[test]
    fn test_lsm_trie_db() {
        let db = LSMTrieDB::new(Config::default());
        
        db.put(b"user:123:name", "Alice").unwrap();
        db.put(b"user:123:age", "30").unwrap();
        db.put(b"user:124:name", "Bob").unwrap();
        
        assert_eq!(db.get(b"user:123:name"), Some("Alice"));
        
        let results = db.scan_prefix(b"user:123");
        assert_eq!(results.len(), 2);
    }
}
```

### Advanced: Concurrent Implementation

```rust
use parking_lot::{RwLock, Mutex};
use crossbeam_skiplist::SkipMap;

/// Thread-safe LSM-Trie using skip list for faster concurrent access
pub struct ConcurrentMemtable<V> {
    /// Skip list for O(log n) concurrent access
    data: SkipMap<Vec<u8>, V>,
    size: Arc<Mutex<usize>>,
}

impl<V: Ord + Clone> ConcurrentMemtable<V> {
    pub fn new() -> Self {
        ConcurrentMemtable {
            data: SkipMap::new(),
            size: Arc::new(Mutex::new(0)),
        }
    }
    
    /// Insert with no locks on read path after update
    pub fn insert(&self, key: Vec<u8>, value: V) {
        let size_add = key.len() + std::mem::size_of_val(&value);
        self.data.insert(key, value);
        *self.size.lock() += size_add;
    }
    
    /// Get with lock-free read
    pub fn get(&self, key: &[u8]) -> Option<V> {
        self.data.get(key).map(|entry| entry.value().clone())
    }
    
    /// Range query
    pub fn range_scan(&self, prefix: &[u8]) -> Vec<(Vec<u8>, V)> {
        self.data
            .range::<Vec<u8>, _>(prefix.to_vec()..)
            .take_while(|entry| entry.key().starts_with(prefix))
            .map(|entry| (entry.key().clone(), entry.value().clone()))
            .collect()
    }
}
```

---

## C Implementation

### Core Data Structures

```c
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdbool.h>
#include <stdint.h>

#define TRIE_CHILDREN 256  // Full ASCII support
#define MAX_KEY_SIZE 1024
#define MAX_VALUE_SIZE 1024

typedef struct {
    void *data;
    size_t len;
} BytesValue;

typedef struct TrieNode {
    struct TrieNode *children[TRIE_CHILDREN];
    BytesValue *value;
    bool is_end_of_word;
    uint32_t ref_count;  // For memory management
} TrieNode;

typedef struct {
    TrieNode *root;
    size_t size;
    size_t max_size;
    uint64_t created_at;
} Memtable;

typedef struct {
    char *path;
    uint8_t *min_key;
    size_t min_key_len;
    uint8_t *max_key;
    size_t max_key_len;
    uint64_t num_entries;
    size_t file_size;
} SSTable;

typedef struct {
    Memtable *active_memtable;
    Memtable **immutable_memtables;
    size_t num_immutable;
    size_t capacity_immutable;
    
    SSTable **levels[7];
    size_t level_sizes[7];
    
    size_t memtable_max_size;
} LSMTrieDB;

// Bloom filter implementation
typedef struct {
    uint8_t *bits;
    size_t bit_count;
    size_t num_hash_functions;
} BloomFilter;

/// Create new trie node
TrieNode *trienode_new(void) {
    TrieNode *node = (TrieNode *)calloc(1, sizeof(TrieNode));
    if (!node) return NULL;
    
    node->is_end_of_word = false;
    node->value = NULL;
    node->ref_count = 1;
    
    return node;
}

/// Free trie node recursively
void trienode_free(TrieNode *node) {
    if (!node) return;
    
    if (--node->ref_count > 0) {
        return;  // Still referenced elsewhere
    }
    
    for (int i = 0; i < TRIE_CHILDREN; i++) {
        if (node->children[i]) {
            trienode_free(node->children[i]);
        }
    }
    
    if (node->value) {
        if (node->value->data) {
            free(node->value->data);
        }
        free(node->value);
    }
    
    free(node);
}

/// Create new memtable
Memtable *memtable_new(size_t max_size) {
    Memtable *mem = (Memtable *)malloc(sizeof(Memtable));
    if (!mem) return NULL;
    
    mem->root = trienode_new();
    if (!mem->root) {
        free(mem);
        return NULL;
    }
    
    mem->size = 0;
    mem->max_size = max_size;
    mem->created_at = 0;  // Would be set to timestamp
    
    return mem;
}

/// Insert key-value into memtable
bool memtable_insert(Memtable *mem, const uint8_t *key, size_t key_len,
                     const uint8_t *value, size_t value_len) {
    if (!mem || !key || key_len == 0) {
        return false;
    }
    
    // Check size
    if (mem->size + key_len + value_len >= mem->max_size) {
        return false;  // Would exceed max size
    }
    
    // Navigate/create trie path
    TrieNode *current = mem->root;
    for (size_t i = 0; i < key_len; i++) {
        uint8_t byte = key[i];
        
        if (!current->children[byte]) {
            current->children[byte] = trienode_new();
            if (!current->children[byte]) {
                return false;
            }
        }
        
        current = current->children[byte];
    }
    
    // Store value
    if (current->value) {
        // Free old value
        if (current->value->data) {
            free(current->value->data);
        }
        free(current->value);
    }
    
    current->value = (BytesValue *)malloc(sizeof(BytesValue));
    if (!current->value) {
        return false;
    }
    
    current->value->data = (void *)malloc(value_len);
    if (!current->value->data) {
        free(current->value);
        current->value = NULL;
        return false;
    }
    
    memcpy(current->value->data, value, value_len);
    current->value->len = value_len;
    current->is_end_of_word = true;
    
    mem->size += key_len + value_len;
    return true;
}

/// Search for key in memtable
bool memtable_get(Memtable *mem, const uint8_t *key, size_t key_len,
                  BytesValue **out_value) {
    if (!mem || !key || key_len == 0) {
        return false;
    }
    
    TrieNode *current = mem->root;
    
    // Navigate trie
    for (size_t i = 0; i < key_len; i++) {
        uint8_t byte = key[i];
        
        if (!current->children[byte]) {
            return false;  // Key not found
        }
        
        current = current->children[byte];
    }
    
    if (current->is_end_of_word && current->value) {
        *out_value = current->value;
        return true;
    }
    
    return false;
}

/// Prefix scan (collect all keys with prefix)
typedef struct {
    uint8_t **keys;
    size_t *key_lens;
    BytesValue **values;
    size_t count;
    size_t capacity;
} ScanResults;

ScanResults *scanresults_new(size_t capacity) {
    ScanResults *res = (ScanResults *)malloc(sizeof(ScanResults));
    if (!res) return NULL;
    
    res->keys = (uint8_t **)malloc(capacity * sizeof(uint8_t *));
    res->key_lens = (size_t *)malloc(capacity * sizeof(size_t));
    res->values = (BytesValue **)malloc(capacity * sizeof(BytesValue *));
    
    if (!res->keys || !res->key_lens || !res->values) {
        free(res->keys);
        free(res->key_lens);
        free(res->values);
        free(res);
        return NULL;
    }
    
    res->count = 0;
    res->capacity = capacity;
    
    return res;
}

void scanresults_free(ScanResults *res) {
    if (!res) return;
    free(res->keys);
    free(res->key_lens);
    free(res->values);
    free(res);
}

/// DFS to collect all keys with prefix
static void dfs_collect(TrieNode *node, uint8_t *current_key,
                       size_t current_len, ScanResults *results) {
    if (node->is_end_of_word && node->value) {
        if (results->count < results->capacity) {
            uint8_t *key_copy = (uint8_t *)malloc(current_len);
            if (key_copy) {
                memcpy(key_copy, current_key, current_len);
                results->keys[results->count] = key_copy;
                results->key_lens[results->count] = current_len;
                results->values[results->count] = node->value;
                results->count++;
            }
        }
    }
    
    for (int i = 0; i < TRIE_CHILDREN; i++) {
        if (node->children[i]) {
            current_key[current_len] = (uint8_t)i;
            dfs_collect(node->children[i], current_key, current_len + 1, results);
        }
    }
}

/// Range query with prefix
ScanResults *memtable_prefix_scan(Memtable *mem, const uint8_t *prefix,
                                  size_t prefix_len) {
    if (!mem || !prefix || prefix_len == 0) {
        return NULL;
    }
    
    // Navigate to prefix end
    TrieNode *current = mem->root;
    for (size_t i = 0; i < prefix_len; i++) {
        uint8_t byte = prefix[i];
        
        if (!current->children[byte]) {
            return NULL;  // Prefix not found
        }
        
        current = current->children[byte];
    }
    
    // Collect all keys
    ScanResults *results = scanresults_new(1000);
    if (!results) return NULL;
    
    uint8_t current_key[MAX_KEY_SIZE];
    memcpy(current_key, prefix, prefix_len);
    
    dfs_collect(current, current_key, prefix_len, results);
    
    return results;
}

/// Bloom filter creation
BloomFilter *bloomfilter_new(size_t capacity, double fpr) {
    BloomFilter *bf = (BloomFilter *)malloc(sizeof(BloomFilter));
    if (!bf) return NULL;
    
    // Optimal bit array size
    size_t m = (size_t)((-capacity * log(fpr)) / (log(2.0) * log(2.0)));
    size_t bytes = (m + 7) / 8;
    
    bf->bits = (uint8_t *)calloc(bytes, sizeof(uint8_t));
    if (!bf->bits) {
        free(bf);
        return NULL;
    }
    
    bf->bit_count = m;
    bf->num_hash_functions = (size_t)((m * log(2.0)) / capacity);
    
    return bf;
}

/// Simple hash function
static uint64_t hash_bytes(const uint8_t *key, size_t key_len, uint64_t seed) {
    uint64_t hash = seed;
    
    for (size_t i = 0; i < key_len; i++) {
        hash = hash * 31 + key[i];
    }
    
    return hash;
}

/// Add element to Bloom filter
void bloomfilter_add(BloomFilter *bf, const uint8_t *key, size_t key_len) {
    for (size_t i = 0; i < bf->num_hash_functions; i++) {
        uint64_t h = hash_bytes(key, key_len, i);
        size_t pos = h % bf->bit_count;
        
        bf->bits[pos / 8] |= (1 << (pos % 8));
    }
}

/// Check if element might exist
bool bloomfilter_might_exist(BloomFilter *bf, const uint8_t *key,
                             size_t key_len) {
    for (size_t i = 0; i < bf->num_hash_functions; i++) {
        uint64_t h = hash_bytes(key, key_len, i);
        size_t pos = h % bf->bit_count;
        
        if (!(bf->bits[pos / 8] & (1 << (pos % 8)))) {
            return false;
        }
    }
    
    return true;
}

/// LSM-Trie DB initialization
LSMTrieDB *lsmtriedb_new(size_t memtable_size) {
    LSMTrieDB *db = (LSMTrieDB *)malloc(sizeof(LSMTrieDB));
    if (!db) return NULL;
    
    db->active_memtable = memtable_new(memtable_size);
    if (!db->active_memtable) {
        free(db);
        return NULL;
    }
    
    db->immutable_memtables = (Memtable **)malloc(10 * sizeof(Memtable *));
    db->num_immutable = 0;
    db->capacity_immutable = 10;
    
    db->memtable_max_size = memtable_size;
    
    // Initialize level arrays
    for (int i = 0; i < 7; i++) {
        db->levels[i] = (SSTable **)malloc(100 * sizeof(SSTable *));
        db->level_sizes[i] = 0;
    }
    
    return db;
}

/// Put operation
bool lsmtriedb_put(LSMTrieDB *db, const uint8_t *key, size_t key_len,
                   const uint8_t *value, size_t value_len) {
    if (!db || !key || !value) {
        return false;
    }
    
    // Try inserting into active memtable
    if (memtable_insert(db->active_memtable, key, key_len, value, value_len)) {
        return true;
    }
    
    // Memtable is full, would need to flush
    // For this example, just return error
    return false;
}

/// Get operation
bool lsmtriedb_get(LSMTrieDB *db, const uint8_t *key, size_t key_len,
                   BytesValue **out_value) {
    if (!db || !key) {
        return false;
    }
    
    // Check active memtable
    if (memtable_get(db->active_memtable, key, key_len, out_value)) {
        return true;
    }
    
    // Check immutable memtables
    for (size_t i = 0; i < db->num_immutable; i++) {
        if (memtable_get(db->immutable_memtables[i], key, key_len, out_value)) {
            return true;
        }
    }
    
    return false;
}

// Unit tests
int main(void) {
    printf("Testing LSM-Trie C Implementation\n");
    
    // Test memtable
    Memtable *mem = memtable_new(1024 * 1024);
    
    memtable_insert(mem, (uint8_t *)"apple", 5, (uint8_t *)"1", 1);
    memtable_insert(mem, (uint8_t *)"apricot", 7, (uint8_t *)"2", 1);
    memtable_insert(mem, (uint8_t *)"banana", 6, (uint8_t *)"3", 1);
    
    BytesValue *val = NULL;
    if (memtable_get(mem, (uint8_t *)"apple", 5, &val)) {
        printf("Found apple: %.*s\n", (int)val->len, (char *)val->data);
    }
    
    // Test prefix scan
    ScanResults *results = memtable_prefix_scan(mem, (uint8_t *)"ap", 2);
    if (results) {
        printf("Prefix 'ap': %zu results\n", results->count);
        scanresults_free(results);
    }
    
    // Cleanup
    trienode_free(mem->root);
    free(mem);
    
    return 0;
}
```

---

## Go Implementation

### Core Data Structures

```go
package lsmtrie

import (
    "bytes"
    "fmt"
    "hash"
    "hash/fnv"
    "sync"
)

// Value represents stored data
type Value interface {
    Size() int
}

// ByteValue is a simple byte slice value
type ByteValue []byte

func (bv ByteValue) Size() int {
    return len(bv)
}

// TrieNode represents a single character in the trie
type TrieNode struct {
    children map[byte]*TrieNode
    value    Value
    isEnd    bool
    mu       sync.RWMutex
}

// NewTrieNode creates a new trie node
func NewTrieNode() *TrieNode {
    return &TrieNode{
        children: make(map[byte]*TrieNode),
        isEnd:    false,
    }
}

// Memtable is an in-memory trie-based write buffer
type Memtable struct {
    root     *TrieNode
    size     int64
    maxSize  int64
    mutex    sync.RWMutex
    createdAt uint64
}

// NewMemtable creates a new memtable
func NewMemtable(maxSize int64) *Memtable {
    return &Memtable{
        root:    NewTrieNode(),
        maxSize: maxSize,
        size:    0,
    }
}

// Insert adds a key-value pair to the memtable
func (m *Memtable) Insert(key []byte, value Value) (bool, error) {
    if len(key) == 0 {
        return false, fmt.Errorf("key cannot be empty")
    }
    
    m.mutex.Lock()
    defer m.mutex.Unlock()
    
    // Check size
    if m.size+int64(len(key))+int64(value.Size()) >= m.maxSize {
        return false, nil // Would exceed max size
    }
    
    // Navigate/create trie path
    current := m.root
    for _, b := range key {
        if _, exists := current.children[b]; !exists {
            current.children[b] = NewTrieNode()
        }
        current = current.children[b]
    }
    
    // Store value
    current.value = value
    current.isEnd = true
    m.size += int64(len(key)) + int64(value.Size())
    
    return true, nil
}

// Get retrieves a value by key
func (m *Memtable) Get(key []byte) (Value, bool) {
    if len(key) == 0 {
        return nil, false
    }
    
    m.mutex.RLock()
    defer m.mutex.RUnlock()
    
    current := m.root
    
    // Navigate trie
    for _, b := range key {
        child, exists := current.children[b]
        if !exists {
            return nil, false
        }
        current = child
    }
    
    if current.isEnd && current.value != nil {
        return current.value, true
    }
    
    return nil, false
}

// ScanResult represents a key-value pair in scan results
type ScanResult struct {
    Key   []byte
    Value Value
}

// PrefixScan returns all keys with the given prefix
func (m *Memtable) PrefixScan(prefix []byte) []ScanResult {
    if len(prefix) == 0 {
        return nil
    }
    
    m.mutex.RLock()
    defer m.mutex.RUnlock()
    
    current := m.root
    
    // Navigate to prefix end
    for _, b := range prefix {
        child, exists := current.children[b]
        if !exists {
            return nil
        }
        current = child
    }
    
    // Collect all keys with this prefix
    var results []ScanResult
    var key []byte
    key = append(key, prefix...)
    m.dfsCollect(current, key, &results)
    
    return results
}

// dfsCollect is a helper for depth-first traversal
func (m *Memtable) dfsCollect(node *TrieNode, currentKey []byte, 
                               results *[]ScanResult) {
    if node.isEnd && node.value != nil {
        // Make a copy of the key
        keyCopy := make([]byte, len(currentKey))
        copy(keyCopy, currentKey)
        *results = append(*results, ScanResult{
            Key:   keyCopy,
            Value: node.value,
        })
    }
    
    for b, child := range node.children {
        newKey := append(currentKey, b)
        m.dfsCollect(child, newKey, results)
    }
}

// ShouldFlush returns whether memtable should be flushed
func (m *Memtable) ShouldFlush() bool {
    m.mutex.RLock()
    defer m.mutex.RUnlock()
    return m.size >= m.maxSize
}

// Size returns current size
func (m *Memtable) Size() int64 {
    m.mutex.RLock()
    defer m.mutex.RUnlock()
    return m.size
}

// BloomFilter is a probabilistic data structure for membership testing
type BloomFilter struct {
    bits      []bool
    k         int  // Number of hash functions
    bitCount  int64
}

// NewBloomFilter creates a new Bloom filter
func NewBloomFilter(capacity int64, fpr float64) *BloomFilter {
    // Optimal bit array size: -capacity * ln(fpr) / ln(2)^2
    bitCount := int64(float64(-capacity) * 
                      (math.Log(fpr) / (math.Log(2) * math.Log(2))))
    
    // Optimal number of hash functions: bitCount/capacity * ln(2)
    k := int(float64(bitCount)/float64(capacity) * math.Log(2))
    
    return &BloomFilter{
        bits:     make([]bool, bitCount),
        k:        k,
        bitCount: bitCount,
    }
}

// Add adds an element to the filter
func (bf *BloomFilter) Add(key []byte) {
    for i := 0; i < bf.k; i++ {
        h := bf.hash(key, uint64(i))
        pos := h % uint64(bf.bitCount)
        bf.bits[pos] = true
    }
}

// MightExist tests if element might be in the set
func (bf *BloomFilter) MightExist(key []byte) bool {
    for i := 0; i < bf.k; i++ {
        h := bf.hash(key, uint64(i))
        pos := h % uint64(bf.bitCount)
        if !bf.bits[pos] {
            return false
        }
    }
    return true
}

// hash produces a hash of the key with a seed
func (bf *BloomFilter) hash(key []byte, seed uint64) uint64 {
    h := fnv.New64a()
    h.Write([]byte{byte((seed >> 56) & 0xFF)})
    h.Write(key)
    return h.Sum64()
}

// SSTable represents an immutable sorted file
type SSTable struct {
    path       string
    minKey     []byte
    maxKey     []byte
    numEntries int64
    fileSize   int64
    bloomFilter *BloomFilter
}

// Config contains LSM-Trie configuration
type Config struct {
    MemtableSize    int64
    NumLevels       int
    LevelSizeRatio  int64
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
    return &Config{
        MemtableSize:   4 * 1024 * 1024, // 4MB
        NumLevels:      7,
        LevelSizeRatio: 10,
    }
}

// LSMTrieDB is the main database structure
type LSMTrieDB struct {
    memtable            *Memtable
    immutableMemtables  []*Memtable
    levels              [][]SSTable
    config              *Config
    mutex               sync.RWMutex
}

// NewLSMTrieDB creates a new database
func NewLSMTrieDB(config *Config) *LSMTrieDB {
    levels := make([][]SSTable, config.NumLevels)
    
    return &LSMTrieDB{
        memtable:           NewMemtable(config.MemtableSize),
        immutableMemtables: make([]*Memtable, 0),
        levels:             levels,
        config:             config,
    }
}

// Put writes a key-value pair
func (db *LSMTrieDB) Put(key []byte, value Value) error {
    if len(key) == 0 {
        return fmt.Errorf("key cannot be empty")
    }
    
    // Try to insert into active memtable
    inserted, err := db.memtable.Insert(key, value)
    if err != nil {
        return err
    }
    
    if !inserted {
        // Memtable full, need to flush
        if err := db.flushMemtable(); err != nil {
            return err
        }
        
        // Retry insert
        inserted, err = db.memtable.Insert(key, value)
        if !inserted {
            return fmt.Errorf("failed to insert after flush")
        }
    }
    
    return nil
}

// Get retrieves a value by key
func (db *LSMTrieDB) Get(key []byte) (Value, error) {
    if len(key) == 0 {
        return nil, fmt.Errorf("key cannot be empty")
    }
    
    // Check active memtable
    if val, found := db.memtable.Get(key); found {
        return val, nil
    }
    
    // Check immutable memtables
    db.mutex.RLock()
    defer db.mutex.RUnlock()
    
    for _, imm := range db.immutableMemtables {
        if val, found := imm.Get(key); found {
            return val, nil
        }
    }
    
    // Check SSTables (implementation omitted)
    return nil, nil
}

// ScanPrefix performs a prefix range query
func (db *LSMTrieDB) ScanPrefix(prefix []byte) ([]ScanResult, error) {
    if len(prefix) == 0 {
        return nil, fmt.Errorf("prefix cannot be empty")
    }
    
    results := make([]ScanResult, 0)
    
    // Scan active memtable
    results = append(results, db.memtable.PrefixScan(prefix)...)
    
    // Scan immutable memtables
    db.mutex.RLock()
    defer db.mutex.RUnlock()
    
    for _, imm := range db.immutableMemtables {
        results = append(results, imm.PrefixScan(prefix)...)
    }
    
    // Deduplicate (keep latest by timestamp)
    results = deduplicateResults(results)
    
    return results, nil
}

// flushMemtable flushes active memtable to immutable
func (db *LSMTrieDB) flushMemtable() error {
    db.mutex.Lock()
    defer db.mutex.Unlock()
    
    if db.memtable.Size() == 0 {
        return nil
    }
    
    // Move current memtable to immutable list
    db.immutableMemtables = append(db.immutableMemtables, db.memtable)
    
    // Create new active memtable
    db.memtable = NewMemtable(db.config.MemtableSize)
    
    // In real implementation, would spawn background task to write SSTable
    
    return nil
}

// Helper function to deduplicate results
func deduplicateResults(results []ScanResult) []ScanResult {
    // Sort by key
    // Keep first occurrence (latest value)
    seen := make(map[string]bool)
    var deduplicated []ScanResult
    
    for _, r := range results {
        key := string(r.Key)
        if !seen[key] {
            seen[key] = true
            deduplicated = append(deduplicated, r)
        }
    }
    
    return deduplicated
}

// Unit tests
package lsmtrie

import (
    "testing"
)

func TestMemtableInsertGet(t *testing.T) {
    mem := NewMemtable(1024 * 1024)
    
    mem.Insert([]byte("apple"), ByteValue("1"))
    mem.Insert([]byte("apricot"), ByteValue("2"))
    mem.Insert([]byte("banana"), ByteValue("3"))
    
    tests := []struct {
        key      []byte
        expected string
        found    bool
    }{
        {[]byte("apple"), "1", true},
        {[]byte("apricot"), "2", true},
        {[]byte("banana"), "3", true},
        {[]byte("cherry"), "", false},
    }
    
    for _, tt := range tests {
        val, found := mem.Get(tt.key)
        if found != tt.found {
            t.Errorf("Get(%s): found=%v, want=%v", tt.key, found, tt.found)
        }
        if found && string(val.(ByteValue)) != tt.expected {
            t.Errorf("Get(%s): got=%s, want=%s", tt.key, val, tt.expected)
        }
    }
}

func TestPrefixScan(t *testing.T) {
    mem := NewMemtable(1024 * 1024)
    
    mem.Insert([]byte("apple"), ByteValue("1"))
    mem.Insert([]byte("application"), ByteValue("2"))
    mem.Insert([]byte("apply"), ByteValue("3"))
    mem.Insert([]byte("banana"), ByteValue("4"))
    
    results := mem.PrefixScan([]byte("app"))
    if len(results) != 3 {
        t.Errorf("PrefixScan('app'): got %d results, want 3", len(results))
    }
}

func TestBloomFilter(t *testing.T) {
    bf := NewBloomFilter(1000, 0.01)
    
    bf.Add([]byte("hello"))
    bf.Add([]byte("world"))
    
    if !bf.MightExist([]byte("hello")) {
        t.Error("BloomFilter: 'hello' should exist")
    }
    
    if !bf.MightExist([]byte("world")) {
        t.Error("BloomFilter: 'world' should exist")
    }
}

func TestLSMTrieDBPutGet(t *testing.T) {
    db := NewLSMTrieDB(DefaultConfig())
    
    err := db.Put([]byte("user:123:name"), ByteValue("Alice"))
    if err != nil {
        t.Fatalf("Put failed: %v", err)
    }
    
    val, err := db.Get([]byte("user:123:name"))
    if err != nil {
        t.Fatalf("Get failed: %v", err)
    }
    
    if val == nil {
        t.Error("Get returned nil")
    } else if string(val.(ByteValue)) != "Alice" {
        t.Errorf("Got %s, want Alice", val)
    }
}
```

---

## Advanced Topics

### Prefix Compression in SSTables

```
Goal: Reduce key storage size in SSTables

Technique: Delta Encoding

Original Keys in Block:
  "apple"     (5 bytes)
  "application" (11 bytes)
  "apply"     (5 bytes)
  "banana"    (6 bytes)

With Prefix Compression (Delta Encoding):
  apple       → [full] apple (5 bytes)
  application → [delta] 3 + lication
                (shared prefix "app" with previous)
  apply       → [delta] 2 + ly
                (shared prefix "ap" with previous)
  banana      → [full] banana (6 bytes)
                (no prefix match)

Savings: ~40-60% reduction in key storage

Encoding Format:
  [shared_len: 1 byte][suffix: variable]
  
  Example:
    "app" + "le" → [03][le]
    "ap" + "ply" → [02][ply]
```

### Leveled vs Tiered Compaction

```
Leveled Compaction (RocksDB style):
  
  Properties:
  - Levels contain non-overlapping key ranges
  - Minimum read amplification
  - Higher write amplification (~10x)
  - Less flash wear
  
  Example:
    L0: [SST0: a-f] [SST1: c-h] [SST2: e-j]
    ↓ Compact
    L1: [SST3: a-d] [SST4: d-h] [SST5: h-j]
    (Non-overlapping)
  
  Read path:
  L0: check all (3 files)
  L1: check 1 file (binary search by range)
  L2: check 1 file
  Total: O(num_levels) file checks

Tiered Compaction (LSM-DB, Cassandra):
  
  Properties:
  - SSTables can overlap within level
  - Better write efficiency
  - Higher read amplification
  - More overlap tolerance
  
  Example:
    L0: [SST0] [SST1] [SST2]
    ↓ Compact when overlapping
    L1: [SST3: many overlaps] [SST4] [SST5]
  
  Read path:
  L0: check all overlapping
  L1: check all overlapping
  Total: O(all files) potential checks

LSM-Trie Enhancement:
  - Use Trie indices to quickly skip non-matching files
  - Bloom filters for negative lookup
  - Hybrid: Leveled within Trie structure
```

### Recovery and WAL Replay

```
Write-Ahead Log Recovery:

Scenario: Database crashes after 100 writes

WAL State:
┌────────────────────────────────────┐
│ WAL File: lsm-trie.wal             │
├────────────────────────────────────┤
│ Entry 0: PUT, "key1", "value1"     │
│ Entry 1: PUT, "key2", "value2"     │
│ ...                                │
│ Entry 99: PUT, "key100", "value100"│
│                                    │
│ Checkpoint: After SSTable flush    │
│ Entries 0-50 flushed to SSTable    │
│ Entries 51-99 in memtable (lost)   │
│                                    │
│ On crash: Entries 51-99 lost       │
└────────────────────────────────────┘

Recovery Process:

1. Find last checkpoint
   Last-flushed memtable: 2 MB data
   Entries 1-50000 already in SSTable

2. Open WAL file
   Read from position after checkpoint

3. Replay entries
   FOR entry IN entries[51:100]:
     memtable.insert(entry.key, entry.value)

4. Restore state
   Original memtable reconstructed
   Lost data recovered

Duration: 10-100ms (depending on WAL size)

Configuration:
- WAL buffer size: 64KB (before fsync)
- Fsync interval: 100ms or 64KB
- Trade-off: Durability vs latency
```

### Compression Algorithms in LSM-Trie

```
Compression Layer Analysis:

Block-Level Compression:

1. Snappy (Default)
   - Speed: 200-300 MB/s compress, 500+ MB/s decompress
   - Ratio: 50-60%
   - CPU: Low (no SIMD required)
   - Trade-off: Speed over ratio
   - Best for: Real-time systems

2. LZ4
   - Speed: 500+ MB/s both ways
   - Ratio: 40-50%
   - CPU: Very low (optimized)
   - Trade-off: Ultra-fast
   - Best for: High-throughput systems

3. Zstd (Zstandard)
   - Speed: 100-200 MB/s compress, 500+ MB/s decompress
   - Ratio: 60-70%
   - CPU: Moderate (SIMD)
   - Trade-off: Best compression ratio
   - Best for: Storage-constrained systems

Key-Level Compression (Prefix/Delta):

For "apple", "application", "apply":
  Raw: 5 + 11 + 5 = 21 bytes
  Delta-encoded: 5 + 3 + 2 = 10 bytes
  Savings: 52%

Value Compression:

Separate value compression:
  Values: separate storage area
  Keys-only SSTable: for index
  Values loaded on demand

Example Trade-offs:
  System      | Algorithm | Compress(MB/s) | Decompress | Ratio
  ────────────────────────────────────────────────────────────
  RT System   | Snappy    | 300            | 1000       | 55%
  HT System   | LZ4       | 600            | 1500       | 45%
  Storage     | Zstd      | 150            | 1000       | 68%
  Archive     | LZMA      | 10             | 50         | 80%
```

### Concurrent Access Patterns

```
Read-Optimized Concurrent Access:

Version-Based Concurrency:

```
                Immutable Version N
                (Read-only snapshot)
                /   |    \
          Thread1 Thread2 Thread3
            Read   Read    Read
          (no locks needed)
                
                ↓ Background
              Compaction
              (new version)
                ↓
           Immutable Version N+1
           (New snapshot)
           (Old version kept until
            all readers finish)
                /   |    \
          Thread1 Thread2 Thread3
          (Wait)  Read    Read
          (v N)   (v N+1) (v N+1)
```

Write Optimization (Lock-Free):

```
         Memtable (LSM)
         ├─ Insert only (no deletes)
         ├─ Immutable after freeze
         └─ No read-write conflicts
         
         All writes sequential
         All reads parallel (no locks)
         
         Benefits:
         - CPU cache friendly
         - No lock contention
         - Scalable to many cores
```

### Monitoring and Diagnostics

```
Key Metrics to Track:

1. Write Amplification
   Formula: bytes_written_to_disk / bytes_written_by_user
   Healthy: 10-50x
   Poor: >100x
   
   Calculation:
   wa_mb = compaction_bytes_written / memtable_bytes_flushed
   
2. Read Amplification
   Formula: number_of_files_checked / 1 (ideal)
   Healthy: 1-10 files
   Poor: >50 files
   
   Tracked per level:
   L0: ~3-5 files (unordered)
   L1: ~1 file (binary search)
   L2+: ~1 file (larger, deeper nesting)
   
3. Space Amplification
   Formula: actual_disk_size / logical_data_size
   Healthy: 1.1-1.5x
   Poor: >2x
   
   Caused by:
   - Deleted entries (tombstones)
   - Overlapping SSTables
   - Fragmentation

4. Compaction Metrics
   - Compaction throughput: MB/s
   - Compaction latency: time taken
   - Compaction frequency: per hour
   - Compaction backlog: pending files
   
5. Memory Metrics
   - Memtable size: current
   - Index cache hit rate: %
   - Block cache hit rate: %
   - Bloom filter false positive rate: %
   
6. Latency Metrics
   - Write latency (p50, p95, p99)
   - Read latency (p50, p95, p99)
   - Compaction pause time (for STW)
   
7. Throughput Metrics
   - Writes per second
   - Reads per second
   - Range queries per second
```

---

## Comparison and Trade-offs

### LSM-Trie vs B-Tree vs Hash Table

```
Feature              | LSM-Trie      | B-Tree        | Hash Table
───────────────────────────────────────────────────────────────────
Write Performance    | Excellent     | Good          | Excellent
Random Reads         | Good          | Excellent     | Excellent
Range Queries        | Excellent     | Excellent     | Poor
Prefix Queries       | Excellent     | Good          | N/A
Space Efficiency     | Good          | Very Good     | Poor
Compression          | Yes (native)  | Limited       | N/A
Persistence          | Yes (LSM)     | Yes (B-Tree)  | No
Concurrency          | Excellent     | Good          | Excellent
Complexity           | High          | Medium        | Low
String Workloads     | Excellent     | Good          | Good

Use Cases:
LSM-Trie:
  ✓ String-heavy workloads
  ✓ Write-heavy systems
  ✓ Autocomplete/prefix search
  ✓ Time-series data
  ✗ Transactional ACID
  ✗ Complex queries

B-Tree:
  ✓ Balanced read/write
  ✓ Excellent for ranges
  ✓ Database standard
  ✗ Random write overhead
  ✗ Weak compression

Hash Table:
  ✓ Ultra-fast exact match
  ✓ Memory efficient (no ordering)
  ✗ No range queries
  ✗ No persistence
  ✗ Resizing overhead
```

### Performance Comparison Table

```
Operation           | LSM-Trie      | B-Tree         | Hash Table
────────────────────────────────────────────────────────────────────
Insert (avg)        | O(m)          | O(m*log N)     | O(1)
Search (avg)        | O(m) + I/O    | O(log N) + I/O | O(1)
Delete (avg)        | O(m)          | O(log N)       | O(1)
Range Query         | O(m+k)        | O(log N+k)     | O(N)
Prefix Query        | O(m+k) ✓      | O(log N+k)     | O(N)
Space (best)        | O(k*m)        | O(k*log k)     | O(k*1.3)
Space (worst)       | O(k*m*log k)  | O(k*log k)     | O(k*2)

where: m = key length, N = total keys, k = result count

Latency Profile (in microseconds):
                    | LSM-Trie   | B-Tree    | Hash Table
────────────────────────────────────────────────────────
In-cache (L3)       | 1-5μs      | 5-10μs    | 1-2μs
Memory hit          | 5-20μs     | 10-50μs   | 5-10μs
SSD miss (single)   | 50-200μs   | 100-300μs | 100-200μs
SSD miss (range)    | 100-1000μs | 200-1000μs| N/A
HDD miss (random)   | 10-20ms    | 15-30ms   | 15-25ms
```

### When to Choose LSM-Trie

**Choose LSM-Trie when:**
1. String/prefix-heavy keys (autocomplete, URLs, paths)
2. Write-intensive workloads (logs, time-series)
3. Compression important (limited storage)
4. Range/prefix queries common
5. Sequential I/O patterns beneficial

**Examples:**
- Autocomplete engines
- DNS servers (domain lookup)
- IP routing tables
- Full-text search indices
- Time-series databases (if keys are timestamps)
- Logging systems

**Don't choose LSM-Trie when:**
1. Ultra-high read latency required (<10μs)
2. Complex ACID transactions needed
3. All keys are numeric (use B-Tree)
4. In-memory only (use Hash Table)
5. Write amplification critical

---

## Conclusion and Best Practices

### Summary

LSM-Trie combines:
- **LSM's write optimization**: Sequential I/O, high throughput
- **Trie's string efficiency**: Prefix sharing, compression
- **Hybrid benefits**: Best for string-key workloads

### Key Takeaways

1. **Architecture**: Memory memtable → Immutable → Level 0 → Levels 1-N
2. **Operations**: O(m) for writes/reads, O(m+k) for range queries
3. **Trade-offs**: Write amplification for compression and query efficiency
4. **Implementation**: Rust for safety, C for performance, Go for simplicity
5. **Monitoring**: Watch write/read/space amplification

### Best Practices

1. **Configuration**
   - Memtable size: 4-64 MB (balance memory vs flush frequency)
   - Level ratio: 10x (balance compaction cost)
   - Compression: Use LZ4/Zstd for new systems

2. **Operations**
   - Monitor compaction lag
   - Set up alerts for write amplification > 50x
   - Regular backups (use WAL + SSTable snapshots)

3. **Performance Tuning**
   - Tune block cache for your workload
   - Adjust Bloom filter FPR based on search patterns
   - Use prefix optimization for range queries

4. **Maintenance**
   - Periodic full compaction to reduce fragmentation
   - Monitor disk space usage
   - Track memory usage (memtables + caches)

---

## References and Further Reading

**Papers:**
- "The Log-Structured Merge-Tree" (O'Neil et al.)
- "Trie-Based Indexing" (various)
- "LSM-Based Storage: Techniques and Performance Trade-offs"

**Implementations:**
- RocksDB (C++)
- LevelDB (C++)
- Badger (Go)
- MyRocks (MySQL)

**Resources:**
- Google Bigtable Design
- Cassandra Architecture
- HBase Internals
