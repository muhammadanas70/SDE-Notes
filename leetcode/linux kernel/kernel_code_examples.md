# Linux Kernel DSA: Code Examples & References

## Where to Find These Data Structures in Linux Kernel Source

### 1. HASH TABLES & HASH FUNCTIONS

#### Kernel Location: `include/linux/hash.h`, `lib/hash.c`

**Hash Table Implementation (hlist):**
```c
// Kernel's hash list - doubly linked list optimized for hashing
// From: include/linux/list.h

struct hlist_node {
    struct hlist_node *next, **pprev;  // pprev is pointer to pointer for removal in O(1)
};

struct hlist_head {
    struct hlist_node *first;
};

// Hash bucket with head
struct hlist_head hashtable[HASH_SIZE];

// Example: Process hash table
// From: kernel/pid.c - Process ID management
struct pid_namespace {
    struct hlist_head *pid_hash;
    // ...
};
```

**Hash Functions:**
```c
// Full hash function from kernel
// From: lib/hash.c

#define GOLDEN_RATIO_PRIME_32 UINT32_C(0x9e3779b9)
#define GOLDEN_RATIO_PRIME_64 UINT64_C(0x9e3779b97f4a7c15)

static inline u32 hash_32(u32 val, unsigned int bits)
{
    return (val * GOLDEN_RATIO_PRIME_32) >> (32 - bits);
}

static inline u64 hash_64(u64 val, unsigned int bits)
{
    return (val * GOLDEN_RATIO_PRIME_64) >> (64 - bits);
}
```

**LRU Cache in Kernel (Page Cache):**
```c
// From: include/linux/fs.h, mm/filemap.c
// Page cache uses LRU list for eviction

struct address_space {
    struct radix_tree_root page_tree;     // Radix tree for quick lookup
    spinlock_t tree_lock;
    atomic_t i_mmap_writable;
    // ...
};

// LRU replacement implemented via:
// - lru_list in page structure
// - lru_cache_add() when page accessed
// - shrink_lru_memcg_list() for eviction
```

**Dentry Cache (filesystem):**
```c
// From: fs/dcache.c
// Hash table for quick directory entry lookup

struct dentry {
    struct hlist_node d_hash;             // Hash table entry
    struct dentry *d_parent;              // Parent directory
    struct qstr d_name;                   // Name
    struct inode *d_inode;                // Associated inode
    // ...
};

// Hash lookup:
// d_hash_and_lookup(parent, name)
```

**Socket Hash Table:**
```c
// From: net/ipv4/inet_hashtables.c
// TCP/UDP socket management

struct inet_hashinfo {
    struct inet_ehash_bucket *ehash;      // Established hash table
    struct inet_bind_hashbucket *bhash;   // Bind hash table
    // ...
};

// Problem mapping: LeetCode 1 (Two Sum) → Hash-based socket lookup
```

**Related LeetCode Problems:**
- LeetCode 1: Two Sum → Hash lookup for process by PID
- LeetCode 146: LRU Cache → Page cache replacement
- LeetCode 706: Design HashMap → Implement dentry cache
- LeetCode 359: Logger Rate Limiter → Token bucket in kernel

---

### 2. LINKED LISTS & QUEUES

#### Kernel Location: `include/linux/list.h`, `kernel/sched/`

**Core Linked List Structure:**
```c
// From: include/linux/list.h
// Circular doubly linked list - most fundamental kernel structure

struct list_head {
    struct list_head *next, *prev;
};

#define LIST_HEAD_INIT(name) { &(name), &(name) }
#define LIST_HEAD(name) struct list_head name = LIST_HEAD_INIT(name)

// Macros for safe operations
#define list_for_each(pos, head) \
    for (pos = (head)->next; pos != (head); pos = pos->next)

#define list_for_each_safe(pos, n, head) \
    for (pos = (head)->next, n = pos->next; pos != (head); \
         pos = n, n = pos->next)

// Example: Process task list
list_for_each(p, &current->children) {
    struct task_struct *child = list_entry(p, struct task_struct, sibling);
    // Process child
}
```

**Process Queue Management:**
```c
// From: kernel/sched/core.c
// Task scheduling queues

struct task_struct {
    struct list_head run_list;            // Runqueue entry
    struct list_head tasks;               // All processes list
    struct list_head children;            // Child processes
    struct list_head sibling;             // Sibling process
    // ...
};

// Runqueue: Process scheduling
struct rq {
    struct list_head cfs_tasks;           // CFS scheduler queue
    // ...
};

// Process list traversal:
#define for_each_process(p) \
    for (p = &init_task ; (p = next_task(p)) != &init_task ; )
```

**sk_buff (Network Packet Buffer):**
```c
// From: include/linux/skbuff.h
// Most complex kernel linked list structure

struct sk_buff {
    struct sk_buff          *next,
                            *prev;
    struct sk_buff_head     *list;
    struct sock             *sk;
    struct net_device       *dev;
    
    /* Linked data */
    unsigned char           *head,
                            *data,
                            *tail,
                            *end;
    unsigned int            len,
                            data_len;
    // ...
};

// sk_buff allocation from pool (buddy allocator integration)
struct sk_buff *__alloc_skb(unsigned int size, gfp_t priority,
                            int flags, int node);

// Queue operations:
void skb_queue_tail(struct sk_buff_head *list, struct sk_buff *newsk);
void skb_unlink(struct sk_buff *skb, struct sk_buff_head *list);
struct sk_buff *skb_dequeue(struct sk_buff_head *list);
```

**Doubly Linked List with Hash (hlist):**
```c
// From: include/linux/list.h
// Used for hash tables where collision chains need removal in O(1)

struct hlist_head {
    struct hlist_node *first;
};

struct hlist_node {
    struct hlist_node *next, **pprev;    // pprev: pointer to pointer!
};

// Why pprev is pointer-to-pointer?
// Allows in-place removal without traversing to previous node
// Normal list:    prev->next = next;  (need to find prev)
// hlist:          *pprev = next;      (direct removal)

// Deletion:
static inline void hlist_del(struct hlist_node *n)
{
    struct hlist_node *next = n->next;
    struct hlist_node **pprev = n->pprev;
    *pprev = next;                      // Remove in O(1)!
    if (next)
        next->pprev = pprev;
}
```

**Related LeetCode Problems:**
- LeetCode 206: Reverse Linked List → Task list reversal
- LeetCode 141: Linked List Cycle → Deadlock detection
- LeetCode 142: Linked List Cycle II → Find circular wait
- LeetCode 138: Copy List with Random → Fork process copy (clone_task_struct)
- LeetCode 21: Merge Two Sorted → Merge runqueues
- LeetCode 148: Sort List → Sort process by priority

---

### 3. RED-BLACK TREES

#### Kernel Location: `include/linux/rbtree.h`, `lib/rbtree.c`

**Red-Black Tree Structure:**
```c
// From: include/linux/rbtree.h

struct rb_node {
    unsigned long  rb_parent_color;       // Parent + color bit-packed
    struct rb_node *rb_right;
    struct rb_node *rb_left;
} __aligned(sizeof(long));

struct rb_root {
    struct rb_node *rb_node;
};

// Get color: (node->rb_parent_color & 1)
// Get parent: (node->rb_parent_color & ~3)

#define RB_RED      0
#define RB_BLACK    1

// Insert: rb_link_node(), rb_insert_color()
// Erase:  rb_erase()
// Search: rb_search_node()
```

**CFS Scheduler (Completely Fair Scheduler) - Red-Black Tree:**
```c
// From: kernel/sched/fair.c
// Uses RB-tree for O(log n) process selection

struct cfs_rq {
    struct rb_root      tasks_timeline;   // RB-tree of runnable tasks
    struct rb_node      *rb_leftmost;     // Leftmost node for quick min-selection
    // ...
};

struct sched_entity {
    struct rb_node      run_node;         // Node in CFS tree
    u64                 vruntime;         // Virtual runtime (key)
    // ...
};

// Find next task to run:
// struct sched_entity *se = __pick_first_entity(cfs_rq);
// Leftmost task in RB-tree has minimum vruntime
```

**Virtual Memory Area Tree (VMA):**
```c
// From: include/linux/mm_types.h, mm/rbtree.c
// Memory regions organized in RB-tree for quick lookup

struct vm_area_struct {
    unsigned long vm_start;               // Start address
    unsigned long vm_end;                 // End address
    struct rb_node vm_rb;                 // RB-tree node
    struct vm_area_struct *vm_next;       // Linked list (optimization)
    // ...
};

struct mm_struct {
    struct rb_root mm_rb;                 // Root of VMA tree
    // ...
};

// Operations:
struct vm_area_struct *find_vma(struct mm_struct *mm, unsigned long addr);
// Time: O(log n) thanks to RB-tree
```

**File Tree (RB-tree variant - Interval Tree):**
```c
// From: mm/interval_tree.c
// Used for interval queries in VM

struct interval_tree_node {
    struct rb_node rb;
    unsigned long start;
    unsigned long last;                   // Inclusive
    // ...
};

// Typical use: Find all overlapping intervals
// Used for VMA lookup: which memory region contains address X?
```

**Related LeetCode Problems:**
- LeetCode 98: Validate BST → Validate RB-tree structure
- LeetCode 236: Lowest Common Ancestor → Find common VMA
- LeetCode 230: Kth Smallest → Find Kth VMA
- LeetCode 450: Delete Node in BST → Remove VMA
- LeetCode 449: Serialize/Deserialize → Tree persistence

---

### 4. GRAPHS & TOPOLOGY

#### Kernel Location: `kernel/`, `drivers/`

**Process Hierarchy Tree:**
```c
// From: include/linux/sched.h

struct task_struct {
    struct task_struct *parent;           // Parent process
    struct list_head children;            // Child processes (node in graph)
    struct list_head sibling;             // Siblings
    // ...
};

// Process tree is a graph where:
// - Nodes = processes
// - Edges = parent-child relationships
// - Root = init (PID 1)
```

**Device Dependency Graph:**
```c
// From: drivers/base/core.c
// Device driver dependencies form a DAG

struct device {
    struct device_node *node;
    struct list_head links;               // Dependency edges
    // ...
};

// Topological sort used for device probe order
// Ensures drivers loaded in correct dependency order
```

**Interrupt Controller Hierarchy:**
```c
// From: kernel/irq/, include/linux/irqdesc.h

struct irq_desc {
    struct irq_chip *chip;                // Interrupt operations
    struct irqaction *action;             // Handler list
    // ...
};

// IRQ dependency graph:
// - Physical IRQs at leaves
// - GPIO IRQs depend on GPIO chip
// - GPIO chip depends on main controller
// - DFS used for handler invocation
```

**Network Routing Table (DAG):**
```c
// From: net/ipv4/route.c
// Routing decision is graph traversal

struct fib_node {
    struct hlist_node fn_hash;
    struct fib_node *fn_parent;
    // ...
};

// Longest prefix match = graph search with specific key
```

**Related LeetCode Problems:**
- LeetCode 207: Course Schedule → Detect circular dependencies
- LeetCode 210: Course Schedule II → Topological sort for probe order
- LeetCode 133: Clone Graph → Fork process tree
- LeetCode 743: Network Delay Time → Interrupt propagation time
- LeetCode 269: Alien Dictionary → Boot order discovery

---

### 5. PRIORITY QUEUES & HEAPS

#### Kernel Location: `kernel/sched/`, `kernel/time/`

**Runqueue Priority Ordering:**
```c
// From: kernel/sched/sched.h

struct rq {
    struct cfs_rq cfs;                    // Fair scheduling queue
    struct rt_rq  rt;                     // Real-time queue
    // Uses RB-tree + priority bucket array
    // NOT traditional heap, but priority-queue concept
};

// Task selection by priority:
// 1. Check real-time tasks (FIFO, RR)
// 2. Then check CFS tasks (vruntime-based)
```

**Timer Queue (Heap-like):**
```c
// From: kernel/time/timer.c
// Timer management with heap-like structure

struct timer_list {
    struct list_head entry;
    unsigned long expires;                // Expiration time (key)
    void (*function)(unsigned long);
    unsigned long data;
    struct timer_base *base;
    // ...
};

// Not pure heap, but cascading timer wheels (more efficient)
// Offers O(1) insertion with proper bucketing

struct timer_base {
    spinlock_t lock;
    struct timer_list *running_timer;
    unsigned long clk;
    unsigned long next_timer;
    // ...
};
```

**High-Resolution Timer:**
```c
// From: kernel/time/hrtimer.c

struct hrtimer {
    struct rb_node node;                  // In red-black tree (heap-like)
    ktime_t expires;                      // Expiration time
    // ...
};

struct hrtimer_clock_base {
    struct rb_root active;                // RB-tree of active timers
    struct rb_node *first;
    ktime_t offset;
    // ...
};

// Efficient heap via RB-tree
// Find next timer to expire: O(log n) via tree traversal
```

**Block I/O Scheduler (Priority):**
```c
// From: block/bfq-iosched.c
// I/O priority scheduling

struct bfq_sched_data {
    struct rb_root service_tree;          // Priority tree
    // ...
};

// Higher priority requests scheduled first
// Implementation uses weighted RB-tree (not traditional heap)
```

**Related LeetCode Problems:**
- LeetCode 23: Merge k Sorted Lists → Merge priority queues
- LeetCode 215: Kth Largest → Priority selection
- LeetCode 295: Find Median → Dual priority queue
- LeetCode 347: Top K Frequent → Heap of elements
- LeetCode 703: Kth Largest in Stream → Online priority queue

---

### 6. TRIE & RADIX TREE

#### Kernel Location: `lib/radix-tree.c`, `net/`

**Radix Tree (Compressed Trie):**
```c
// From: include/linux/radix-tree.h
// Used for associative arrays with integer keys

struct radix_tree_root {
    unsigned int height;
    gfp_t gfp_mask;
    struct radix_tree_node *rnode;
};

struct radix_tree_node {
    unsigned char shift;
    unsigned char offset;
    unsigned char count;
    unsigned char exceptional;
    struct radix_tree_node *parent;
    void *slots[RADIX_TREE_MAP_SIZE];     // Child nodes or values
    // ...
};

// Page cache uses radix tree:
// Key = file offset, Value = page pointer
struct address_space {
    struct radix_tree_root page_tree;     // Page cache lookup O(log n)
    // ...
};
```

**IP Routing Table (Trie-like):**
```c
// From: net/ipv4/fib_trie.c
// Longest prefix matching using trie structure

struct trie_node {
    struct trie_node *kv[2];              // Binary trie (0/1)
    void *value;                          // Route info
    // ...
};

// Longest prefix match:
// Traverse trie following IP bits left-to-right
// O(k) where k = prefix length
```

**Related LeetCode Problems:**
- LeetCode 208: Implement Trie → Routing table lookup
- LeetCode 211: Add and Search Words → DNS cache
- LeetCode 648: Replace Words → Device name matching
- LeetCode 677: Map Sum Pairs → File offset mapping

---

### 7. BIT MANIPULATION & BITMAPS

#### Kernel Location: `include/linux/bitmap.h`, `include/asm/bitops.h`

**Bitmap Operations:**
```c
// From: include/linux/bitmap.h
// Core bitmap operations for flags

#define BITS_PER_LONG       (BITS_PER_BYTE * sizeof(long))

static inline void set_bit(int nr, volatile unsigned long *addr)
{
    unsigned long mask = BIT_MASK(nr);
    unsigned long *p = ((unsigned long *)addr) + BIT_WORD(nr);
    *p  |= mask;
}

static inline void clear_bit(int nr, volatile unsigned long *addr)
{
    unsigned long mask = BIT_MASK(nr);
    unsigned long *p = ((unsigned long *)addr) + BIT_WORD(nr);
    *p &= ~mask;
}

static inline int test_bit(int nr, const volatile unsigned long *addr)
{
    return 1UL & (addr[BIT_WORD(nr)] >> (nr & (BITS_PER_LONG-1)));
}
```

**Process Memory Maps (Bitmaps):**
```c
// From: mm/page_alloc.c
// Page allocation bitmap

typedef struct {
    unsigned long bits[DIV_ROUND_UP(NR_PAGEFLAGS, BITS_PER_LONG)];
} pageflags_t;

struct page {
    unsigned long flags;                  // Page flags as bitmap
    // Individual bits:
    // bit 0: PG_locked
    // bit 1: PG_error
    // bit 2: PG_referenced
    // bit 3: PG_uptodate
    // ...
};

// Efficient flag checking:
if (test_bit(PG_locked, &page->flags)) {
    // Page is locked
}

// Set flag atomically:
set_bit(PG_dirty, &page->flags);
```

**CPU Affinity (cpumask_t):**
```c
// From: include/linux/cpumask.h

typedef struct cpumask {
    unsigned long bits[BITS_TO_LONGS(NR_CPUS)];
} cpumask_t;

// Which CPUs a task runs on:
struct task_struct {
    cpumask_t cpus_allowed;               // Bitmap of CPUs
    // ...
};

// Check if task can run on CPU 3:
if (cpumask_test_cpu(3, &task->cpus_allowed)) {
    // Can run on CPU 3
}
```

**Memory Region Bitmap:**
```c
// From: mm/buddy.c
// Free page bitmap in buddy allocator

struct zone {
    DECLARE_BITMAP(flags, ZONES_PER_NODE);
    struct free_area free_area[MAX_ORDER];
    // ...
};

// Find first set/clear bit:
// unsigned long pos = find_first_bit(bitmap, size);
```

**Related LeetCode Problems:**
- LeetCode 191: Number of 1 Bits → popcount(cpumask)
- LeetCode 231: Power of Two → Check CPU number validity
- LeetCode 136: Single Number → XOR flags for page state
- LeetCode 461: Hamming Distance → CPU mask difference
- LeetCode 371: Sum of Two Integers → Bitmap arithmetic

---

### 8. MEMORY MANAGEMENT ALGORITHMS

#### Kernel Location: `mm/`, `include/linux/`

**Buddy Allocator:**
```c
// From: mm/page_alloc.c
// Binary buddy allocator for fast allocation/deallocation

struct free_area {
    struct list_head free_list[MIGRATE_TYPES];
    unsigned long nr_free;
};

struct zone {
    struct free_area free_area[MAX_ORDER + 1];  // Order 0 to MAX_ORDER
    // ...
};

// Allocation:
// 1. Find free block of order >= requested
// 2. Split recursively until exact size
// 3. Return address

// Deallocation:
// 1. Free block at order
// 2. Try to coalesce with buddy
// 3. Recursively merge upwards
```

**Slab Allocator:**
```c
// From: mm/slab.c
// Object pool allocator for frequently-used structures

struct kmem_cache {
    struct list_head list;
    unsigned long colour;
    unsigned long colour_off;
    struct kmem_cache_node *node[MAX_NUMNODES];
    // ...
};

struct slab {
    struct list_head list;
    unsigned long colouroff;
    void *s_mem;                          // Slab memory start
    unsigned int inuse;                   // Objects in use
    // ...
};

// Allocation: 
// - Quick: return from free objects list
// - If no free: allocate new slab

// Deallocation:
// - Return to free list
// - If slab empty: possibly free slab
```

**LRU Page Replacement:**
```c
// From: mm/page_alloc.c, mm/lru.c

enum lru_list {
    LRU_INACTIVE_ANON = LRU_BASE,
    LRU_ACTIVE_ANON = LRU_BASE + 1,
    LRU_INACTIVE_FILE = LRU_BASE + 2,
    LRU_ACTIVE_FILE = LRU_BASE + 3,
    LRU_UNEVICTABLE = LRU_BASE + 4,
    NR_LRU_LISTS
};

struct lruvec {
    struct list_head lists[NR_LRU_LISTS];
    struct zone_reclaim_stat reclaim_stat;
    // ...
};

// When memory pressure:
// - Scan LRU lists from INACTIVE
// - Reclaim least recently used pages
// - Write dirty pages back to disk
```

**Related LeetCode Problems:**
- LeetCode 146: LRU Cache → Page replacement
- LeetCode 460: LFU Cache → Frequency-based eviction
- LeetCode 42: Trapping Rain Water → Memory layout optimization

---

### 9. SYNCHRONIZATION & LOCKING

#### Kernel Location: `include/linux/mutex.h`, `kernel/locking/`

**Mutex (Mutual Exclusion Lock):**
```c
// From: include/linux/mutex.h

struct mutex {
    atomic_long_t owner;                  // Owner task pointer
    spinlock_t wait_lock;                 // Protect wait list
    struct list_head wait_list;           // Waiters queue
    // ...
};

// Lock: atomic_cmpxchg to set owner
// If conflict: add to wait_list
// Unlock: Remove from wait_list, wake next
```

**Semaphore (Counter-based):**
```c
// From: include/linux/semaphore.h

struct semaphore {
    raw_spinlock_t lock;
    unsigned int count;                   // Resource count
    struct list_head wait_list;           // Waiting processes
};

// Down: Decrement count, block if 0
// Up: Increment count, wake waiter
```

**Read-Write Lock:**
```c
// From: include/linux/rwlock.h

typedef struct {
    arch_rwlock_t raw_lock;
    unsigned int magic, cpu;
    unsigned int owner_cpu;
    void *owner;
    // ...
} rwlock_t;

// Multiple readers OR one writer
// Used for page cache, filesystem metadata
```

**Spinlock (Busy-wait):**
```c
// From: include/linux/spinlock.h

typedef struct spinlock {
    union {
        struct raw_spinlock rlock;
    } ____cacheline_aligned_in_smp;
} spinlock_t;

// For short critical sections
// CPU spins waiting for lock (no context switch)
```

**Related LeetCode Problems:**
- LeetCode 1115: Print FooBar → Mutex coordination
- LeetCode 1117: Building H2O → Semaphore pattern
- LeetCode 1226: Dining Philosophers → Deadlock avoidance
- LeetCode 1242: Web Crawler → Lock coordination

---

## Quick Reference: Problems → Kernel Code

| Problem | Kernel Code | Location |
|---------|-------------|----------|
| Hash lookup (Two Sum) | Process table lookup | kernel/pid.c |
| LRU Cache | Page cache eviction | mm/page_alloc.c |
| Linked List Cycle | Deadlock detection | kernel/locking/ |
| Validate BST | VMA tree validation | mm/mmap.c |
| Course Schedule | Device probe order | drivers/base/ |
| Merge k Lists | Runqueue merging | kernel/sched/core.c |
| Kth Largest | Priority selection | kernel/sched/fair.c |
| Number of 1 Bits | CPU affinity check | include/linux/cpumask.h |
| Trie | IP routing | net/ipv4/fib_trie.c |
| Implement Mutex | Lock coordination | kernel/locking/mutex.c |
| Buddy Allocator | Memory allocation | mm/page_alloc.c |
| Dijkstra | Interrupt latency | kernel/irq/ |
| sk_buff | Network buffers | include/linux/skbuff.h |

---

## Study Strategy Using Kernel Source

### Step 1: Solve DSA Problem
```
Example: LeetCode 146 (LRU Cache)
- Implement data structure
- Understand algorithm
- Test edge cases
```

### Step 2: Find Kernel Usage
```
grep -r "lru" kernel/ | grep -i cache
Search: mm/lru.c, mm/page_alloc.c
```

### Step 3: Understand Kernel Context
```c
// How does kernel use LRU?
- Multiple lists for different page types
- Atomic operations for thread safety
- Memory pressure triggers reclamation
- Writeback for dirty pages
```

### Step 4: Adapt for Kernel
```c
// Kernel-specific considerations:
- Use kernel memory allocators (kmalloc, vmalloc)
- Atomic operations for synchronization
- Handle lock contention
- Optimize for cache locality
```

---

## Key Takeaways

1. **Hash Tables**: Everywhere for O(1) lookups (process table, inode cache, socket table)
2. **Linked Lists**: Fundamental structure (task queues, sk_buff, wait lists)
3. **Red-Black Trees**: Scheduling (CFS), VM management (VMA), I/O scheduling
4. **Graphs**: Process hierarchy, device dependencies, interrupt routing
5. **Priority Queues**: Task scheduling, timer management, I/O scheduling
6. **Tries/Radix Trees**: Page cache, IP routing, device tree
7. **Bit Manipulation**: Flags, bitmaps, CPU affinity, memory regions
8. **Heaps/Sorting**: Memory layout, priority ordering
9. **Synchronization**: Mutex, semaphore, spinlock implementation
10. **Memory Algorithms**: Buddy allocator, slab allocator, LRU replacement

---

