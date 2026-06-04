# The Complete Guide to Linux Inode Cache: Architecture, Implementation, and Mental Models

## Table of Contents
1. [Foundations: What is an Inode?](#foundations)
2. [The Virtual File System Layer](#vfs-layer)
3. [Inode Cache Architecture](#inode-cache-architecture)
4. [Data Structures and Memory Layout](#data-structures)
5. [Cache Lifecycle and State Transitions](#lifecycle)
6. [Synchronization and Locking](#synchronization)
7. [Kernel Subsystem Integration](#subsystem-integration)
8. [Performance Optimization](#performance)
9. [Debugging and Introspection](#debugging)
10. [C Implementation Details](#c-implementation)
11. [Rust Safety Implications](#rust-implications)
12. [Real-World Scenarios](#scenarios)

---

## Foundations: What is an Inode? {#foundations}

### The Conceptual Model

An **inode** (index node) is the kernel's in-memory representation of a filesystem's metadata about a file or directory. It is NOT the file data itself—it's the metadata:

```
File on Disk
│
├─ Inode Block (metadata on storage)
│  ├─ Size
│  ├─ Permissions
│  ├─ Owner/Group
│  ├─ Timestamps
│  ├─ Block pointers
│  └─ Link count
│
└─ Data Blocks (file content)
```

When you access a file, the kernel reads the inode block from storage, then creates an **inode cache entry** in memory—a `struct inode` in kernel memory that shadows this disk data.

### Why Caching Matters

Without an inode cache:
- Every file operation (open, stat, read, write) requires disk I/O to read metadata
- Metadata modifications (chmod, chown) require immediate synchronous disk writes
- Multiple opens of the same file re-parse the same inode from storage

With inode caching:
- Metadata is held in fast kernel memory (typically hundreds of nanoseconds access vs. milliseconds disk I/O)
- Dirty metadata can be batched and written asynchronously
- Multiple references to the same inode share a single kernel struct

---

## The Virtual File System Layer {#vfs-layer}

### VFS Architecture Overview

```
Application Layer (libc, syscalls)
         │
         ├─────────────────────────────────────┐
         │                                     │
    User Space                                │
    ──────────────────────────────────────────┼─────────────────
    Kernel Space                              │
         │                                    │
         ▼                                    │
    System Call Interface                     │
    (open, read, write, stat, etc.)           │
         │                                    │
         ▼                                    │
    VFS Generic Layer                         │
    ├─ Path traversal                         │
    ├─ Permission checks                      │
    ├─ Inode cache management  ◄──────────────┘
    ├─ Dentry cache (dcache)
    └─ File descriptor table
         │
         ├─────────────────────────────────────────────────────────┐
         │                                                         │
         ▼                                                         │
    Filesystem-Specific Layer                                      │
    ├─ ext4 operations                                             │
    ├─ btrfs operations                                            │
    ├─ xfs operations                                              │
    └─ ...                                                         │
         │                                                         │
         ▼                                                         │
    Block Device Layer / Storage Backend ◄──────────────────────────┘
    ├─ Page cache
    ├─ Buffer cache
    └─ Block device I/O
```

### The Three Caches in the I/O Stack

1. **Inode Cache**: Filesystem metadata in memory (owned by VFS, per-filesystem)
2. **Dentry Cache (dcache)**: Filename-to-inode mappings (VFS layer, global)
3. **Page Cache**: File content data (VFS layer, global)

These work together:

```
Path: /home/user/file.txt
         │
         ▼
    Dentry Cache (dcache)
    "/"         ─────┐
    "/home"     ─────┤───► inode cache for "/home" directory
    "/home/user"───┐ │
    "/home/user/file.txt"──┼──► inode cache for "file.txt"
                           │
                           └──► page cache (file contents)
```

---

## Inode Cache Architecture {#inode-cache-architecture}

### High-Level Architecture

```
┌────────────────────────────────────────────────────────────────┐
│                    INODE CACHE SUBSYSTEM                        │
├────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌─────────────────────┐         ┌──────────────────────┐      │
│  │   Hash Table        │         │   Superblock per FS  │      │
│  │   (inode_hashtable) │         │   (struct super_block)       │
│  │                     │         │                      │      │
│  │  [bucket 0] ────────────────►│  s_inode_lru_list    │      │
│  │  [bucket 1] ────────┐        │  (LRU eviction)      │      │
│  │  [bucket 2] ────────┼────────│  s_inodes (all)      │      │
│  │   ...      │        │        │                      │      │
│  │  [bucket n]        │        │  Per-FS dirty list   │      │
│  └─────────────────────┘        └──────────────────────┘      │
│                                                                  │
│  ┌──────────────────────────────────┐                          │
│  │  Inode Struct (struct inode)     │                          │
│  ├──────────────────────────────────┤                          │
│  │ i_ino:        inode number       │                          │
│  │ i_mode:       permissions        │                          │
│  │ i_uid/i_gid:  owner/group        │                          │
│  │ i_size:       size in bytes      │                          │
│  │ i_blocks:     disk blocks used   │                          │
│  │ i_atime/mtime/ctime:  timestamps │                          │
│  │ i_mapping:    page cache mapping │                          │
│  │ i_hash:       hash chain (llist) │                          │
│  │ i_lru:        LRU list node      │                          │
│  │ i_state:      state flags        │                          │
│  │ i_ref_count:  reference count    │                          │
│  │ i_lock:       spinlock           │                          │
│  │ i_mutex:      mutex              │                          │
│  │ i_data:       address space obj  │                          │
│  │ i_op:         inode operations   │                          │
│  │ i_fop:        file operations    │                          │
│  │ i_sb:         pointer to superblock  │                      │
│  │ ...           (many more fields)     │                      │
│  └──────────────────────────────────┘                          │
│                                                                  │
│  ┌──────────────────────────────────┐                          │
│  │  Lock Hierarchy                  │                          │
│  ├──────────────────────────────────┤                          │
│  │ inode_hash_lock (global spinlock)│                          │
│  │    └─► Per-bucket spinlock       │                          │
│  │           └─► i_lock (per inode) │                          │
│  │               └─► i_mutex        │                          │
│  │                   └─► page locks │                          │
│  └──────────────────────────────────┘                          │
│                                                                  │
└────────────────────────────────────────────────────────────────┘
```

### The Global Inode Cache

The kernel maintains a **global inode cache** indexed by (filesystem superblock, inode number):

```c
// Conceptual structure (simplified)
struct inode_cache {
    // Global hash table
    struct hlist_head *inode_hashtable;  // Array of hash buckets
    unsigned int inode_hashtable_size;   // Number of buckets
    
    // Lookup statistics
    atomic_long_t inode_allocations;
    atomic_long_t inode_lookups;
    atomic_long_t inode_evictions;
};
```

Key insight: The hash table is **global and global-locked**, but operations are protected at the bucket level for concurrency.

### Hash Function

```c
// Kernel hash function for inodes (from fs/inode.c)
#define INODE_HASHBITS  8
#define INODE_HASH_SIZE (1 << INODE_HASHBITS)

static inline unsigned long hash_long(unsigned long val, unsigned int bits)
{
    unsigned long hash = val;
    hash ^= (hash >> (bits / 2));
    return hash & ((1 << bits) - 1);
}

static unsigned int hash_inode(struct super_block *sb, unsigned long ino)
{
    // Combine filesystem and inode number
    unsigned long h = (unsigned long)sb ^ ino;
    return hash_long(h, INODE_HASHBITS);
}
```

The hash combines:
1. **Superblock pointer** (filesystem identity)
2. **Inode number** (position within filesystem)

This allows same inode number across different filesystems without collision.

---

## Data Structures and Memory Layout {#data-structures}

### struct inode - Core Metadata Container

```c
// Simplified from linux/fs.h
struct inode {
    // Unique identification
    umode_t                i_mode;        // Permissions + file type (16 bits)
    unsigned short         i_opflags;     // Operational flags
    uid_t                  i_uid;         // User ID owner
    gid_t                  i_gid;         // Group ID owner
    unsigned int           i_flags;       // FS-independent flags
    unsigned long          i_ino;         // Inode number (unique within FS)
    
    // Hard link count
    nlink_t                i_nlink;       // Number of hard links (decrements on unlink)
    
    // Timestamps (nanosecond precision)
    struct timespec64      i_atime;       // Last access time
    struct timespec64      i_mtime;       // Last modify time (metadata or content)
    struct timespec64      i_ctime;       // Change time (metadata only)
    
    // Content size and blocks
    loff_t                 i_size;        // Size in bytes
    blkcnt_t               i_blocks;      // Blocks allocated (512-byte units)
    unsigned long          i_blkbits;     // Block size exponent (typically 12 for 4KB)
    
    // State and reference counting
    atomic_t               i_count;       // Reference count (deprecated, use i_ref)
    unsigned long          i_state;       // State flags (I_DIRTY, I_LOCK, etc.)
    unsigned long          i_hash_mask;   // Hash chain mask
    
    // Locking
    spinlock_t             i_lock;        // Protects most of the inode
    struct mutex           i_mutex;       // Serializes writes, truncates
    rwlock_t               i_dentry_lock; // Protects i_dentry list
    
    // Cache structures
    struct hlist_node      i_hash;        // Hash chain node (global hash table)
    struct list_head       i_dentry;      // Dentries pointing to this inode
    struct list_head       i_devices;     // For block/char devices
    
    // LRU and eviction
    struct list_head       i_lru;         // LRU list node
    struct list_head       i_sb_list;     // List on superblock
    struct list_head       i_wb_list;     // Writeback list
    struct list_head       i_io_list;     // IO list
    
    // Filesystem operations
    struct address_space   i_data;        // Mapping for page cache
    struct address_space   *i_mapping;    // Usually points to i_data
    
    // Operations pointers (function tables)
    const struct inode_operations   *i_op;   // Metadata operations
    const struct file_operations    *i_fop;  // File content operations
    const struct file_system_type   *i_fs;   // Back-reference to FS type
    
    // Superblock and file system context
    struct super_block     *i_sb;         // Owning superblock
    
    // Extended attributes
    struct security_struct *i_security;   // SELinux, AppArmor, etc.
    
    // Private data per filesystem
    void                   *i_private;    // Filesystem-specific data
    
    // Dirty state tracking
    struct inode_crypt_info *i_crypt_info;  // Encryption context
    struct fsnotify_mark_connector *i_fsnotify_marks;
    
    // Write count
    atomic_t               i_writecount;   // Active writers
    
    // Dirty page tracking
    atomic_t               i_pinned_pages; // Pages pinned by GUP
    
    // Read-mostly fields
    union {
        const struct pipe_inode_info    *i_pipe;
        const struct cdev               *i_cdev;
        char                            *i_link;
        unsigned                        i_dir_seq;
    };
    
    // Rarely used fields
    __u32                  i_generation;  // Version for NFS
    
};  // Total size: ~1KB on 64-bit systems
```

### Memory Layout and Padding

```
0x0:   i_mode (2)          i_opflags (2)
0x4:   i_uid (4)           
0x8:   i_gid (4)
0xC:   i_flags (4)
0x10:  i_ino (8)
0x18:  i_nlink (4)        [padding: 4]
0x20:  i_atime.tv_sec (8)
0x28:  i_atime.tv_nsec (4) [padding: 4]
0x30:  i_mtime.tv_sec (8)
0x38:  i_mtime.tv_nsec (4) [padding: 4]
0x40:  i_ctime.tv_sec (8)
0x48:  i_ctime.tv_nsec (4) [padding: 4]
0x50:  i_size (8)
0x58:  i_blocks (8)
0x60:  i_blkbits (4)      [padding: 4]
0x68:  i_state (8)
...
[Total: ~1024 bytes + filesystem-specific data]
```

**Key insight**: The inode struct is carefully cache-line aligned. Frequently accessed fields are kept in the first cache line (64 bytes) to maximize L1 cache hits.

### Hash Table Bucket Structure

```c
struct hlist_head {
    struct hlist_node *first;
};

struct hlist_node {
    struct hlist_node *next, **pprev;  // XOR trick for memory efficiency
};

// In inode:
struct hlist_node i_hash;  // Node in global hash table
```

The **pprev** pointer is a "back-pointer to the pointer that points to me":

```
Before: list_head → node1 → node2 → node3 → NULL
                      ↑
                    first

After insertion:
list_head → node1 → new_node → node2 → node3 → NULL
              ↑                  ↑
            pprev            pprev
```

This allows O(1) removal without traversing to find the predecessor.

### Superblock Inode Lists

```c
// In struct super_block
struct list_head    s_inode_lru_list;  // LRU for eviction
unsigned long       s_inode_lru_count;
spinlock_t          s_inode_lru_lock;

struct list_head    s_inodes;           // All inodes on this filesystem
spinlock_t          s_inode_lock;
```

Each filesystem maintains:
1. **s_inodes**: Complete list of all inodes for the filesystem
2. **s_inode_lru_list**: LRU-ordered inodes eligible for eviction

---

## Cache Lifecycle and State Transitions {#lifecycle}

### Inode Lifecycle State Machine

```
                              ┌─────────────────────────┐
                              │   INODE CREATION        │
                              │   (alloc_inode)         │
                              └────────────┬────────────┘
                                           │
                                           ▼
                        ┌──────────────────────────────────┐
                        │  STATE: NEW                      │
                        │  - i_ref_count = 1               │
                        │  - i_state = I_NEW               │
                        │  - No data populated             │
                        └────────────┬─────────────────────┘
                                     │
                     ┌───────────────┴───────────────┐
                     │                               │
                     ▼                               ▼
          ┌────────────────────┐        ┌──────────────────────┐
          │ SUCCESS            │        │ FAILURE              │
          │ (from disk or mem) │        │ (allocation error)   │
          └────────┬───────────┘        └──────────┬───────────┘
                   │                               │
                   ▼                               ▼
    ┌──────────────────────────┐        ┌──────────────────────┐
    │ STATE: I_REFERENCED      │        │ STATE: I_FREEING     │
    │ - Fully initialized      │        │ - Marked for cleanup │
    │ - In hash table          │        │ - Removing refs      │
    │ - In LRU list            │        └──────────┬───────────┘
    │ - Ready for use          │                   │
    └────────┬─────────────────┘                   │
             │                                     │
             │◄────── Access by fd/path ──────────┤
             │                                     │
             ├─────────┐                           │
             │         │                           │
             ▼         ▼                           ▼
    ┌──────────────┬──────────────┐    ┌──────────────────────┐
    │ STATE: IN_USE│ STATE: DIRTY │    │ STATE: I_FREEING     │
    │              │              │    │ - Last ref dropped   │
    │ i_count > 0  │ I_DIRTY flag │    │ - Waiting writeback  │
    │              │ (content or  │    └──────────┬───────────┘
    │ Last fd      │  metadata)   │               │
    │ closed ──────┐              │               │
    │              └──────────────┘               │
    │                    │                        │
    │ Mark dirty ◄──────┐│                        │
    │ Schedule writeback │                        │
    │                    │                        │
    └────────┬───────────┘                        │
             │                                    │
             └──────────┬───────────────────────┬─┘
                        │                       │
                   sync/fsync           Wait for writeback
                        │                       │
                        ▼                       ▼
         ┌──────────────────────────┐ ┌──────────────────────┐
         │ STATE: I_SYNC            │ │ STATE: I_WRITEBACK   │
         │ - Syncing to disk        │ │ - In-flight to disk  │
         │ - i_count still > 0      │ │ - Cannot modify      │
         │ - Locked                 │ │ - Locked             │
         └────────┬─────────────────┘ └──────────┬───────────┘
                  │                              │
                  └──────────────┬────────────────┘
                                 │
                                 ▼
                  ┌──────────────────────────────┐
                  │ STATE: I_CLEAN               │
                  │ - Synced to disk             │
                  │ - In LRU (potentially)       │
                  │ - i_count >= 0               │
                  └────────┬─────────────────────┘
                           │
                    Can be accessed again
                    or evicted if no refs
                           │
                           ▼
         ┌──────────────────────────────────────┐
         │ STATE: I_LRU / EVICTABLE              │
         │ - No active references               │
         │ - In LRU list at inode level          │
         │ - Eligible for cache eviction        │
         │ - Data still in memory               │
         └────────┬─────────────────────────────┘
                  │
           Memory pressure /
           Periodic reclaim
                  │
                  ▼
     ┌────────────────────────────────┐
     │ STATE: I_FREEING / I_WILL_FREE │
     │ - Remove from hash table        │
     │ - Remove from LRU               │
     │ - Release page cache refs       │
     │ - Release inode mem             │
     └────────┬───────────────────────┘
              │
              ▼
     ┌────────────────────────────────┐
     │ INODE FREED                     │
     │ - Memory returned to pool       │
     │ - Will be reallocated later     │
     └────────────────────────────────┘
```

### State Flags Detailed

```c
// linux/fs.h - Inode state flags
#define I_NEW                0   // Inode is being initialized
#define I_REFERENCED         1   // Inode was accessed in this interval
#define I_DirtyDataSync      2   // Inode has dirty data to sync
#define I_DirtyMetaDataSync  3   // Inode has dirty metadata to sync
#define I_Locked             4   // Inode is locked
#define I_Freeing            5   // Inode is being freed
#define I_Clear              6   // Inode has been cleared
#define I_Sync               7   // Inode is being synced to disk
#define I_DirtyTime          8   // Lazy dirtying of timestamps
#define I_WBDirty            9   // Writeback list dirty
#define I_Dirty             10   // General dirty flag

// Compound flags
#define I_DIRTY (I_DirtyDataSync | I_DirtyMetaDataSync | I_WBDirty)
#define I_DIRTY_ALL (I_DIRTY | I_DirtyTime)
```

State transitions are atomic using bit operations:

```c
// Setting a flag atomically
set_bit(I_Dirty, &inode->i_state);

// Reading a flag
if (test_bit(I_Dirty, &inode->i_state)) { ... }

// Clear and return old value
was_locked = test_and_clear_bit(I_Locked, &inode->i_state);
```

---

## Synchronization and Locking {#synchronization}

### The Lock Hierarchy (CRITICAL for Deadlock Prevention)

```
GLOBAL:
┌──────────────────────┐
│ inode_hash_lock      │ (RCU-protected global spinlock)
│ Global lock for all  │
│ inode hash ops       │
└──────┬───────────────┘
       │
       ▼
PER-BUCKET:
┌──────────────────────┐
│ bucket->lock         │ (Per-hash-bucket spinlock)
│ Protects specific    │
│ collision chain      │
└──────┬───────────────┘
       │
       ▼
PER-INODE (i_lock):
┌──────────────────────────────┐
│ inode->i_lock                │ (Spinlock)
│ - Protects i_state flags     │
│ - Protects i_count ref       │
│ - Protects i_dentry list     │
│ - Protects i_lru list        │
│ Fast path: < 1 microsecond   │
└──────┬───────────────────────┘
       │
       ▼
PER-INODE (i_mutex):
┌──────────────────────────────┐
│ inode->i_mutex               │ (Mutex - sleepable)
│ - Serializes i_size changes  │
│ - Serializes truncate        │
│ - Protects directory ops     │
│ Slow path: microseconds      │
│ Can sleep (no spinlocks held)│
└──────┬───────────────────────┘
       │
       ▼
PAGE LOCKS:
┌──────────────────────────────┐
│ page->flags (page_lock)      │
│ - Per-page content lock      │
│ - Protects page cache        │
│ Can sleep                    │
└──────────────────────────────┘
```

### Lock Ordering Rules (MUST FOLLOW)

```
SAFE: inode_hash_lock → i_lock → i_mutex → page_lock
      (acquire left-to-right)

DEADLOCK: page_lock → i_lock → inode_hash_lock
         (violates hierarchy)

SAFE: i_mutex alone (top-level syscall context)
      OR i_lock alone (interrupt context)

NEVER SAFE: Holding i_lock then calling sleep_on_page()
           (requires releasing locks first)
```

### Example: Lookup with Proper Locking

```c
// Inode lookup from hash table (linux/fs.c simplified)
struct inode *find_inode(struct super_block *sb, unsigned long ino)
{
    struct inode_hash_bucket *b;
    struct inode *inode;
    unsigned int hash;

    hash = hash_inode(sb, ino);
    b = &inode_hashtable[hash];

    // STEP 1: Acquire global hash lock (spinlock, fast path)
    spin_lock(&inode_hash_lock);
    
    // STEP 2: Walk the collision chain under hash lock
    inode = NULL;
    hlist_for_each_entry(i, &b->head, i_hash) {
        if (i->i_sb == sb && i->i_ino == ino) {
            inode = i;
            break;
        }
    }
    
    if (inode) {
        // STEP 3: Acquire inode-specific lock
        spin_lock(&inode->i_lock);
        
        // STEP 4: Check state (might have been freed between lookup)
        if (!(inode->i_state & I_Freeing)) {
            atomic_inc(&inode->i_count);  // Reference count
            found = true;
        }
        
        spin_unlock(&inode->i_lock);
    }
    
    // STEP 5: Release global lock
    spin_unlock(&inode_hash_lock);
    
    return inode;
}
```

**Key insight**: The lock is held only long enough to:
1. Find the inode in the hash table
2. Increment its reference count atomically
3. Verify it's not being freed

This minimizes contention—typical critical section < 100 nanoseconds.

### RCU (Read-Copy-Update) Optimization

For read-heavy lookups, modern kernels use RCU:

```c
// RCU-protected hash lookup (even faster on read path)
rcu_read_lock();

// No spinlock needed - safe under RCU
hlist_for_each_entry_rcu(inode, &bucket->head, i_hash) {
    if (inode->i_sb == sb && inode->i_ino == ino) {
        // Found it, but must take i_lock to increment safely
        spin_lock(&inode->i_lock);
        
        // Check state under lock
        if (!(inode->i_state & I_Freeing)) {
            atomic_inc(&inode->i_count);
        }
        
        spin_unlock(&inode->i_lock);
        break;
    }
}

rcu_read_unlock();
```

**Benefits**:
- Readers don't need spinlocks
- Writers are expensive (must wait for RCU grace period)
- Perfect for read-heavy workloads (inode lookups > 99% read)

---

## Kernel Subsystem Integration {#subsystem-integration}

### Dentry Cache (dcache) - The Missing Link

The inode cache alone can't serve path-based lookups efficiently. The **dentry cache** bridges filesystem path semantics to inodes:

```
Path: /home/user/file.txt
      │
      ├─ Dentry: "/"      → inode (root directory)
      │
      ├─ Dentry: "home"   → inode (home dir)
      │   parent: "/" dentry
      │
      ├─ Dentry: "user"   → inode (user dir)
      │   parent: "home" dentry
      │
      └─ Dentry: "file.txt" → inode (file)
          parent: "user" dentry
          
        [All dentries cached]
        → Fast lookup without filesystem traversal
```

```c
// In struct dentry
struct dentry {
    struct inode        *d_inode;      // Points to cached inode
    struct dentry       *d_parent;     // Parent directory dentry
    struct qstr         d_name;        // Filename string
    struct hlist_node   d_hash;        // In dentry hash table
    struct list_head    d_lru;         // LRU for eviction
    unsigned int        d_flags;       // State flags
    struct dcache_ops   *d_op;         // Filesystem callbacks
};
```

### Path Traversal with Cache

```
Walking "/home/user/file.txt":

1. Lookup "/" in dentry cache (root cached at startup)
   CACHE HIT → dentry_root → inode_root
   
2. Lookup "home" in dentry cache (name hash in root's children)
   CACHE HIT → dentry_home → inode_home
   
3. Lookup "user" in dentry cache (child of "home")
   CACHE HIT → dentry_user → inode_user
   
4. Lookup "file.txt" in dentry cache
   CACHE HIT → dentry_file → inode_file
   
Total: 4 hash table lookups, all in cache
Time: ~100 nanoseconds (typical dentry cache hit)

WITHOUT CACHE:
Total: 4 filesystem operations (4KB reads from storage)
Time: ~4 milliseconds (each disk I/O ~1ms)
Speedup: 40,000x
```

### Interaction with Writeback System

Inodes are integrated with the writeback system for dirty tracking:

```
┌─────────────────────────────────────┐
│  Filesystem Operations               │
│  (write, truncate, chmod, etc.)      │
└────────────┬────────────────────────┘
             │
             ▼
    ┌──────────────────────────────┐
    │ Mark inode dirty             │
    │ set_bit(I_DIRTY, i_state)    │
    └────────┬─────────────────────┘
             │
             ▼
    ┌──────────────────────────────────────┐
    │ Dirty Tracking                       │
    │ Add to dirty list in superblock      │
    │ Or write immediately (sync=true)     │
    └────────┬─────────────────────────────┘
             │
             ├─ ASYNC: Schedule writeback worker
             │  (pdflush, bdi_writeback)
             │         │
             │         ▼
             │  ┌──────────────────────┐
             │  │ Writeback worker:    │
             │  │ - Iterate dirty list │
             │  │ - Lock inode         │
             │  │ - Write dirty pages  │
             │  │ - Write inode block  │
             │  │ - Clear I_DIRTY      │
             │  └──────────────────────┘
             │
             └─ SYNC: Immediate blocking write
                  sync(), fsync(), syscall exit
                  (i_mutex held until complete)
```

### Memory Reclaim and Shrinker Interface

When memory is tight, the kernel asks the inode cache to free space:

```c
// Kernel memory pressure handler
struct shrinker inode_shrinker = {
    .scan_objects = inode_scan,
    .count_objects = inode_count,
};

// Scan callback - free up inodes
static unsigned long inode_scan(struct shrinker *s,
                                 struct shrink_control *sc)
{
    // Called when memory is needed
    // Walk LRU list and evict inodes
    
    struct list_head *list = &superblock->s_inode_lru_list;
    unsigned long evicted = 0;
    
    while (evicted < sc->nr_to_scan) {
        inode = lru_to_inode(list->prev);  // LRU: least recently used at tail
        
        // Check reference count
        if (atomic_read(&inode->i_count) > 0) {
            // In use - move to front
            list_move(&inode->i_lru, list);
            continue;
        }
        
        // Not in use - evict
        remove_from_hash_table(inode);
        remove_from_lru(inode);
        free_inode(inode);
        
        evicted++;
    }
    
    return evicted;
}
```

**Eviction order**: Least recently used first
- Track last access via i_atime
- Move to front on each access
- Evict from back of queue

---

## Performance Optimization {#performance}

### Measurement: Inode Lookup Performance

```
1. Cold cache (filesystem just mounted):
   Lookup time: 5-10 microseconds
   Includes: Disk I/O (~1-5ms) + hash lookup (~100ns)
   
2. Warm cache (inode already loaded):
   Lookup time: 100-200 nanoseconds
   No disk I/O, purely in-memory hash table
   
3. Hot cache (very recently accessed):
   Lookup time: 50-100 nanoseconds
   Data in CPU L1 cache
   
Ratio: 100,000x difference between cold and hot
```

### Optimization Strategies

#### 1. Negative Caching

Cache the fact that a file doesn't exist:

```c
// In dentry (struct dentry)
dentry->d_flags |= DCACHE_NEGATIVE;
dentry->d_inode = NULL;

// Lookup /home/user/nonexistent.txt
// Result: Cached "doesn't exist" entry
// No filesystem traversal needed for repeated lookups
```

#### 2. Hash Table Size Tuning

```c
// At boot time - scale with system RAM
inode_hashtable_size = 1UL << (fls(totalram_pages - 1) - 6);
// On 64GB system: 2^20 buckets = 1 million buckets
// Average chain length: O(active_inodes / buckets)
```

**Chain length targets**:
- < 1 inode per bucket: cold misses dominate
- 1-3 inodes per bucket: balanced
- > 5 inodes per bucket: hash collisions cause slowdown

#### 3. LRU Recency Tracking

```c
// Move to front on access (O(1) with list_move))
#define MAX_INACTIVE_INTERVAL (2 * HZ)  // 2 seconds

void mark_inode_accessed(struct inode *inode)
{
    // Quick check without locking
    if (time_before(jiffies, inode->i_last_access + MAX_INACTIVE_INTERVAL))
        return;  // Too recent
    
    // Inode is "old" - move to front of LRU
    spin_lock(&superblock->s_inode_lru_lock);
    list_move(&inode->i_lru, &superblock->s_inode_lru_list);
    inode->i_last_access = jiffies;
    spin_unlock(&superblock->s_inode_lru_lock);
}
```

#### 4. Reference Count Optimization

Instead of per-inode refcount (requires atomic ops on every access):

```c
// Use percpu refcounting (reduce global contention)
typedef struct {
    atomic_t            count;      // Global count
    atomic_t            *cpu_refs;  // Per-CPU counts (NR_CPUS entries)
} percpu_ref_t;

// On this CPU - no atomic ops needed
void inode_get(struct inode *inode)
{
    atomic_inc(this_cpu_ptr(&inode->i_pcpu_ref.cpu_refs));
}

// Periodically sync back to global
void percpu_ref_flush()
{
    for_each_cpu(cpu)
        atomic_add(per_cpu_sum[cpu], &ref->count);
}
```

**Benefits**:
- 95%+ of refcount operations avoid atomic ops
- atomic ops only on CPU migration or final release
- Especially effective for high-frequency operations (read, getdents, fstat)

#### 5. Cache-Line Alignment

```
struct inode layout:
┌─────────────────────────────────────┐
│ Cache Line 0 (64 bytes)              │  Frequently accessed
│ i_mode, i_uid, i_ino, i_lock        │  (< 100 nanoseconds)
├─────────────────────────────────────┤
│ Cache Line 1 (64 bytes)              │  Moderately accessed
│ i_count, i_state, i_dentry          │  (< 1 microsecond)
├─────────────────────────────────────┤
│ Cache Lines 2+ (512+ bytes)          │  Rarely accessed
│ i_mapping, i_sb, i_op, etc.         │  (> 1 microsecond)
└─────────────────────────────────────┘
```

On NUMA systems, this matters even more—keep frequently accessed fields on the same NUMA node.

---

## C Implementation Details {#c-implementation}

### Inode Allocation (from linux/fs/inode.c)

```c
/*
 * Allocate a new inode from the inode cache.
 * Called during file operations that create inodes.
 */
struct inode *new_inode(struct super_block *sb)
{
    struct inode *inode;
    
    // Allocate memory (typically from slab cache)
    inode = kmem_cache_alloc(inode_cachep, GFP_KERNEL);
    if (!inode)
        return NULL;
    
    // Initialize with default values
    spin_lock_init(&inode->i_lock);
    mutex_init(&inode->i_mutex);
    
    inode->i_sb = sb;
    inode->i_blkbits = sb->s_blocksize_bits;
    inode->i_count.counter = 1;           // Refcount = 1
    inode->i_state = I_NEW;               // Mark as new
    inode->i_flags = 0;
    inode->i_uid = current_fsuid();
    inode->i_gid = current_fsgid();
    atomic64_set(&inode->i_version, 1);
    
    // Initialize address space for page cache
    address_space_init_once(&inode->i_data);
    inode->i_mapping = &inode->i_data;
    
    // Initialize lists
    INIT_LIST_HEAD(&inode->i_dentry);
    INIT_LIST_HEAD(&inode->i_devices);
    INIT_LIST_HEAD(&inode->i_lru);
    INIT_HLIST_NODE(&inode->i_hash);
    
    // Call filesystem-specific initialization
    if (sb->s_op && sb->s_op->alloc_inode) {
        inode = sb->s_op->alloc_inode(sb);
        if (!inode) {
            kmem_cache_free(inode_cachep, inode);
            return NULL;
        }
    }
    
    return inode;
}

/*
 * Insert inode into the hash table.
 * Called after inode is populated with data from disk.
 */
void insert_inode_hash(struct inode *inode)
{
    struct hlist_head *b;
    unsigned int hash;
    
    if (inode->i_state & (I_Freeing | I_Clear))
        goto out_unlock;
    
    // Compute hash from superblock and inode number
    hash = hash_inode(inode->i_sb, inode->i_ino);
    b = &inode_hashtable[hash];
    
    // Acquire global hash lock
    spin_lock(&inode_hash_lock);
    
    // Acquire inode lock
    spin_lock(&inode->i_lock);
    
    // Verify state again (might have changed)
    if (!(inode->i_state & I_Freeing)) {
        // Insert at head of collision chain (most recent at front)
        hlist_add_head(&inode->i_hash, b);
        
        // Add to superblock's inode list
        list_add(&inode->i_sb_list, &inode->i_sb->s_inodes);
        
        // Add to LRU list (back = least recently used)
        list_add_tail(&inode->i_lru, &inode->i_sb->s_inode_lru_list);
        
        // Clear NEW flag
        inode->i_state &= ~I_NEW;
        
        // Mark as referenced
        inode->i_state |= I_REFERENCED;
    }
    
    spin_unlock(&inode->i_lock);
    spin_unlock(&inode_hash_lock);
    
    // Add to dirty list if dirty
    if (inode->i_state & I_DIRTY)
        mark_inode_dirty_sync(inode);
    
out_unlock:
    return;
}

/*
 * Lookup inode in cache by (superblock, inode_number).
 * This is the hot path - called millions of times per second.
 */
struct inode *iget_locked(struct super_block *sb, unsigned long ino)
{
    struct hlist_head *head;
    struct inode *inode;
    unsigned int hash;
    
    // Hash the inode number
    hash = hash_inode(sb, ino);
    head = &inode_hashtable[hash];
    
    // Fast path: RCU read-side (no locks for lookup)
    rcu_read_lock();
    
    inode = find_inode_fast(sb, head, ino);
    
    if (inode) {
        if (!(inode->i_state & I_NEW))
            goto found;  // Cache hit on existing inode
        // Else: inode exists but not yet populated
    }
    
    rcu_read_unlock();
    
    // Slow path: Not found or not yet initialized
    // Now use full locking
    
    spin_lock(&inode_hash_lock);
    
    // Re-check under lock (might have been inserted)
    inode = find_inode_fast(sb, head, ino);
    
    if (!inode) {
        // Allocate new inode
        spin_unlock(&inode_hash_lock);
        inode = new_inode(sb);
        if (!inode)
            return NULL;
        
        // Re-acquire lock
        spin_lock(&inode_hash_lock);
        
        // Double-check again (TOCTOU prevention)
        if (find_inode_fast(sb, head, ino) != NULL) {
            // Race: someone else inserted it first
            // Use their version instead
            destroy_inode(inode);
            inode = find_inode_fast(sb, head, ino);
            atomic_inc(&inode->i_count);
            inode->i_state |= I_REFERENCED;
            spin_unlock(&inode_hash_lock);
            return inode;
        }
        
        // Insert our new inode
        spin_lock(&inode->i_lock);
        inode->i_state = I_NEW;
        hlist_add_head(&inode->i_hash, head);
        list_add(&inode->i_sb_list, &sb->s_inodes);
        list_add_tail(&inode->i_lru, &sb->s_inode_lru_list);
        spin_unlock(&inode->i_lock);
        
        spin_unlock(&inode_hash_lock);
        return inode;  // Caller will populate from disk
    }
    
    // Existing inode - increment reference count
    atomic_inc(&inode->i_count);
    inode->i_state |= I_REFERENCED;
    
    spin_unlock(&inode_hash_lock);
    
found:
    return inode;
}

/*
 * Helper to find inode in hash chain (assumes lock held or RCU read-side)
 */
static inline struct inode *find_inode_fast(struct super_block *sb,
                                             struct hlist_head *head,
                                             unsigned long ino)
{
    struct inode *inode;
    
    hlist_for_each_entry(inode, head, i_hash) {
        if (inode->i_ino == ino && inode->i_sb == sb) {
            return inode;
        }
    }
    
    return NULL;
}

/*
 * Mark inode as dirty (needs writeback to disk).
 * Fast version - used in hot path (write, truncate, etc).
 */
void mark_inode_dirty_sync(struct inode *inode)
{
    // Check if already dirty (avoid unnecessary list operations)
    if (test_bit(I_DirtyTime, &inode->i_state))
        return;
    
    // Set dirty bit
    __mark_inode_dirty(inode, I_DirtyTime);
}

/*
 * Full mark dirty - used less frequently
 */
void __mark_inode_dirty(struct inode *inode, unsigned int flags)
{
    struct super_block *sb = inode->i_sb;
    
    // Acquire inode lock to prevent races
    spin_lock(&inode->i_lock);
    
    // Set flags
    inode->i_state |= flags;
    
    // Add to dirty list if not already there
    if (list_empty(&inode->i_dirty_list)) {
        list_add_tail(&inode->i_dirty_list, &sb->s_dirty);
    }
    
    // Notify writeback system
    if (sb->s_bdi) {
        spin_unlock(&inode->i_lock);
        writeback_inodes_sb(sb, WB_REASON_DIRTY);
        return;
    }
    
    spin_unlock(&inode->i_lock);
}

/*
 * Release inode reference. Called when:
 * - File descriptor closed
 * - Directory entry deleted
 * - Inode explicitly released
 */
void iput(struct inode *inode)
{
    if (!inode)
        return;
    
    if (atomic_dec_and_lock(&inode->i_count, &inode_hash_lock)) {
        // Reference count dropped to zero
        
        // Check if it should be freed or cached
        if (inode->i_nlink == 0 && !is_bad_inode(inode)) {
            // Inode has no hard links - schedule for deletion
            list_move(&inode->i_lru, &inode->i_sb->s_inode_lru_list);
            inode->i_state |= I_WILL_FREE;
        } else {
            // Inode has links - keep in cache for reuse
            list_move(&inode->i_lru, &inode->i_sb->s_inode_lru_list);
        }
        
        spin_unlock(&inode_hash_lock);
        
        // Synchronous operations on zero-ref inode
        // (no lock held, safe to sleep)
        if (inode->i_state & I_WILL_FREE)
            evict_inode(inode);
    }
}

/*
 * Evict inode from cache - called during reclaim or deletion
 */
void evict_inode(struct inode *inode)
{
    struct address_space *mapping;
    
    // Prevent further access
    inode->i_state |= I_Freeing;
    
    // Call filesystem-specific eviction
    if (inode->i_sb->s_op->evict_inode) {
        inode->i_sb->s_op->evict_inode(inode);
    }
    
    // Invalidate all page cache pages
    mapping = inode->i_mapping;
    if (mapping->nrpages) {
        // Force write of dirty pages
        writeback_mapping_pages(mapping);
        
        // Truncate all pages
        truncate_inode_pages(mapping, 0);
    }
    
    // Remove from hash table
    spin_lock(&inode_hash_lock);
    hlist_del_init(&inode->i_hash);
    list_del_init(&inode->i_sb_list);
    spin_unlock(&inode_hash_lock);
    
    // Remove from LRU
    spin_lock(&inode->i_sb->s_inode_lru_lock);
    list_del_init(&inode->i_lru);
    spin_unlock(&inode->i_sb->s_inode_lru_lock);
    
    // Mark as completely freed
    inode->i_state |= I_Clear;
    
    // Free memory
    kmem_cache_free(inode_cachep, inode);
}
```

### Slab Caching of Inodes

Inodes are allocated from a memory slab cache—pre-allocated chunks that reduce fragmentation:

```c
// Initialization during kernel boot
kmem_cache_create("inode_cache",
                  sizeof(struct inode),     // Object size
                  0,                        // No alignment
                  SLAB_RECLAIM_ACCOUNT |    // Track for memory pressure
                  SLAB_PANIC,               // Fail loudly if cannot allocate
                  init_once);               // Constructor per object

// In init_once() callback:
static void init_once(void *foo)
{
    struct inode *inode = (struct inode *)foo;
    
    inode_init_once(inode);  // Zero structure, init locks, etc.
}
```

**Why slab caching?**
- Reduces fragmentation vs. individual malloc
- Amortizes allocation overhead
- Helps CPU cache locality (allocations from same region)
- Integrates with memory pressure (can shrink slab)

---

## Rust Safety Implications {#rust-implications}

### Translating Inode Cache to Rust

The inode cache has several patterns that are unsafe in standard Rust:

#### 1. Circular References via Parent Pointers

```rust
// Problem: dentry → inode → dentry (via i_dentry list)
pub struct Dentry {
    pub inode: Option<Rc<Inode>>,
    pub parent: Option<Rc<Dentry>>,  // Circular: parent holds child's ref
}

pub struct Inode {
    pub dentry_list: Vec<Weak<Dentry>>,  // Back-references using Weak
}
```

**Rust solution**: Use `Weak<T>` for back-references to break cycles.

#### 2. Interior Mutability with Spinlocks

```rust
// Spinlock in C (doesn't nest or drop-guard):
spin_lock(&inode->i_lock);
inode->i_state |= I_DIRTY;
spin_unlock(&inode->i_lock);

// Rust requires explicit RAII:
use parking_lot::Mutex;

pub struct Inode {
    state: Mutex<InodeState>,
    ref_count: AtomicUsize,
}

impl Inode {
    pub fn mark_dirty(&self) {
        let mut state = self.state.lock();  // RAII guard
        state.flags |= InodeFlags::DIRTY;
    }   // Guard dropped, lock released
}
```

#### 3. Unsafe Reference Counting

```rust
// C's atomic_inc/dec requires manual discipline:
// Problem: What if dropped too early?
atomic_dec(&inode->i_count);  // Might free while someone still holds pointer!

// Rust's Arc<T> handles this automatically:
let inode: Arc<Inode> = Arc::new(...);
let clone1 = Arc::clone(&inode);  // Ref count += 1
let clone2 = Arc::clone(&inode);  // Ref count += 1
drop(clone1);                      // Ref count -= 1 (not freed)
drop(clone2);                      // Ref count -= 1 (not freed)
drop(inode);                       // Ref count -= 1 (NOW freed, compiler ensures)
```

#### 4. Lock Ordering with Rust Types

```rust
// C's lock ordering is by convention; Rust can encode it in types:

// Newtype pattern to prevent wrong ordering:
pub struct GlobalLock;
pub struct InodeLock;
pub struct PageLock;

pub struct LockedInode<'a> {
    _global: &'a GlobalLock,
    inode: &'a Inode,
}

impl Inode {
    // Can only acquire inode lock if global is held
    pub fn lock_with_global<'a>(
        &'a self,
        _global: &'a GlobalLock,
    ) -> LockedInode<'a> {
        LockedInode {
            _global,
            inode: self,
        }
    }
    
    // This won't compile - missing global lock:
    // pub fn lock(&self) -> LockedInode { ... }
}

// Usage:
let global = GlobalLock;
let inode = Inode::new();
let locked = inode.lock_with_global(&global);  // OK
let bad = inode.lock();  // Compiler error!
```

#### 5. Hash Table with Lifetime Bounds

```rust
// Safe hash table with lifetime bounds:
pub struct InodeHashTable<'a> {
    buckets: Vec<Vec<&'a Inode>>,
}

impl<'a> InodeHashTable<'a> {
    pub fn insert(&mut self, inode: &'a Inode) {
        let hash = self.hash(inode.ino);
        self.buckets[hash].push(inode);
    }
    
    pub fn lookup(&self, ino: u64) -> Option<&'a Inode> {
        let hash = self.hash(ino);
        self.buckets[hash]
            .iter()
            .find(|i| i.ino == ino)
            .copied()
    }
}
// Rust ensures: table cannot outlive the inodes it references
```

#### 6. Dirty Tracking with Type State

```rust
// Use type state to prevent invalid operations:

pub struct Inode<S: InodeState> {
    data: InodeData,
    _state: PhantomData<S>,
}

pub trait InodeState {}
pub struct Clean;
pub struct Dirty;

impl InodeState for Clean {}
impl InodeState for Dirty {}

// Only clean inodes can be evicted
impl Inode<Clean> {
    pub fn evict(self) -> InodeMemory {
        InodeMemory::free(self.data)
    }
}

// Cannot evict dirty inode - doesn't compile:
// impl Inode<Dirty> {
//     pub fn evict(self) -> InodeMemory { ... }  // Missing!
// }

// To evict dirty inode, must sync first:
impl Inode<Dirty> {
    pub fn sync(self) -> Inode<Clean> {
        // Write to disk, then transition to Clean
        Inode {
            data: self.data,
            _state: PhantomData,
        }
    }
}

// Usage:
let mut inode = Inode::<Clean>::new();
inode.mark_dirty();  // Compile error - no such method
// Solution:
let dirty_inode: Inode<Dirty> = inode.into();  // Explicit transition
let clean_inode = dirty_inode.sync();          // Must sync first
clean_inode.evict();                            // Now safe to evict
```

#### 7. Async-Aware Version

```rust
// Modern Rust kernels (like linux-next with Rust support) use async:

pub struct Inode {
    state: parking_lot::Mutex<InodeState>,
}

impl Inode {
    // Async version - doesn't block CPU
    pub async fn mark_dirty_and_sync(&self) {
        {
            let mut state = self.state.lock();
            state.flags |= InodeFlags::DIRTY;
        }
        // Write async - doesn't hold lock
        self.writeback_task().await;
    }
}

// Compared to C (always blocks):
// synchronous writeback in C blocks CPU and holds locks
```

### Rust Patterns for Kernel Subsystems

```rust
// Pattern: Safe shared access without locks (for read-only fields)

pub struct Inode {
    // Read-only after initialization (set once)
    pub ino: u64,              // Never changes
    pub sb: Arc<Superblock>,   // Shared superblock
    
    // Protected by lock
    state: Mutex<InodeState>,
}

impl Inode {
    // This is zero-cost (no lock needed)
    pub fn inode_number(&self) -> u64 {
        self.ino
    }
    
    // This requires lock
    pub fn state(&self) -> InodeStateHandle {
        self.state.lock()
    }
}

// Compiler helps us distinguish cheap ops from expensive ones
```

---

## Real-World Scenarios {#scenarios}

### Scenario 1: File Creation (`touch` command)

```
$ touch /home/user/newfile.txt

Execution trace:

1. open("/home/user/newfile.txt", O_CREAT | O_WRONLY)

2. VFS path traversal:
   - lookup("/") from dcache → inode_root
   - lookup("home") from dcache → inode_home
   - lookup("user") from dcache → inode_user
   - lookup("newfile.txt") from dcache → NOT FOUND
   
3. Inode cache lookup fails, filesystem operation needed:
   - Call ext4_lookup()
   - Read directory block from storage
   - Parse directory entry (if doesn't exist, NULL)
   - Return NULL
   
4. Create new inode:
   - Call new_inode() → allocate from slab cache
   - i_ino = next_available (e.g., 1234567)
   - i_mode = S_IFREG | 0644 (regular file)
   - i_uid = current_uid (e.g., 1000)
   - i_size = 0
   
5. Insert into inode cache:
   - hash = hash_inode(sb, 1234567)
   - Acquire inode_hash_lock
   - Acquire inode->i_lock
   - hlist_add_head(&inode->i_hash, &bucket)
   - list_add_tail(&inode->i_lru, &sb->s_inode_lru_list)
   - Release locks
   
6. Create dentry and insert into dcache:
   - Allocate dentry for "newfile.txt"
   - dentry->d_inode = inode (1234567)
   - dentry->d_parent = dentry_user
   - hash = hash_name("newfile.txt")
   - Insert into dcache hash table
   
7. Write to storage:
   - Add inode to superblock dirty list
   - Add dentry to superblock dirty list
   - Writeback worker: sync inode block + directory block
   
8. close()

Memory state:
┌────────────────────────────────────────┐
│ Page cache (page for /home/user/)      │
│ Holds directory contents with new entry│
└────────────────────────────────────────┘
         ▲
         │ references
         │
┌────────────────────────────────────────┐
│ Dentry cache                           │
│ "newfile.txt" → inode (1234567)        │
└────────────────────────────────────────┘
         ▲
         │ d_inode pointer
         │
┌────────────────────────────────────────┐
│ Inode cache (hash bucket for 1234567)  │
│ ino=1234567, size=0, uid=1000, ...     │
│ Linked in: i_hash, i_sb_list, i_lru    │
└────────────────────────────────────────┘

Next access: All cached - ~100ns
```

### Scenario 2: Reading Large File (repeated reads)

```
$ cat largefile.bin | wc -c

Execution trace:

1. open("largefile.bin")

2. Inode lookup:
   - dcache hit (was created before)
   - inode = iget_locked(sb, 5000000)
   - CACHE HIT - returns existing inode immediately (~100ns)
   - atomic_inc(&inode->i_count) → ref_count = 2
   
3. Multiple read(fd, buf, 4KB) calls:
   
   Read 1 (bytes 0-4095):
   - Map to page 0
   - Page cache miss → read from disk (~1ms)
   - Set page cache entry
   - Copy to user buffer
   
   Read 2 (bytes 4096-8191):
   - Map to page 1
   - Page cache miss → read from disk (~1ms)
   - Set page cache entry
   - Copy to user buffer
   
   Read 3 (bytes 0-4095 again):
   - Map to page 0
   - PAGE CACHE HIT - in memory
   - Copy directly to user buffer (~100ns)
   - No disk I/O!
   - Update inode->i_atime (access time)
   
4. close()
   - iput(inode)
   - atomic_dec(&inode->i_count) → ref_count = 1
   - Still in cache (ref_count > 0)
   
5. Repeated access:
   - Same inode
   - Reads hit page cache
   - Access time: ~100ns per 4KB (vs. 1ms on cold)
   - Speedup: 10,000x

Memory hierarchy:
┌──────────────────────┐
│ CPU L1 cache (32KB)  │  Hit: ~4ns
├──────────────────────┤
│ CPU L2 cache (256KB) │  Hit: ~12ns
├──────────────────────┤
│ CPU L3 cache (8MB)   │  Hit: ~40ns
├──────────────────────┤
│ RAM (page cache)     │  Hit: ~100ns (inode in L3)
├──────────────────────┤
│ Storage (disk)       │  Miss: ~1ms
└──────────────────────┘

Key: inode stays in CPU L3 cache across reads
     pages are in L1/L2 for this process
     10,000x difference cache vs. disk
```

### Scenario 3: Memory Pressure and Eviction

```
Scenario: System running low on memory
Available RAM: 100MB (was 4GB)
Active processes: 50
Open files: 1000s (many inodes in cache)

1. Kernel detects memory pressure:
   Memory pressure = (free pages / total pages) < threshold
   
2. Shrinker callback invoked:
   inode_shrinker.scan_objects(shrink_control)
   
   for each superblock {
       // Walk LRU list (least recently used at tail)
       for each inode in sb->s_inode_lru_list {
           
           // Check if still in use
           if (atomic_read(&inode->i_count) > 0) {
               // In use - skip
               continue;
           }
           
           // Check if dirty
           if (inode->i_state & I_DIRTY) {
               // Must sync before eviction
               writeback_single_inode(inode);
               // Move to front of LRU
               list_move(&inode->i_lru, &list->head);
               continue;
           }
           
           // Clean and unreferenced - evict
           // 1. Remove from hash table
           hlist_del(&inode->i_hash);
           
           // 2. Remove from superblock list
           list_del(&inode->i_sb_list);
           
           // 3. Remove from LRU
           list_del(&inode->i_lru);
           
           // 4. Free inode memory
           kmem_cache_free(inode_cachep, inode);
           
           freed_memory += sizeof(struct inode);
           
           if (freed_memory >= nr_to_free)
               break;
       }
   }
   
3. Result:
   - Freed 100MB of inode cache
   - ~100,000 inodes evicted
   - System RAM restored
   - Application performance degrades (evicted inodes must be re-read)
   
4. Future access to evicted inode:
   - Inode not in cache
   - Lookup fails
   - Allocate new inode
   - Read from disk
   - Re-populate inode
   - Performance: ~1ms (was ~100ns with cache)
```

### Scenario 4: Cross-Filesystem Hard Link Issue

```
Scenario: Attempt hardlink across filesystems

$ ln /mnt/fs1/file.txt /mnt/fs2/hardlink.txt
error: cross-device link

Why?

Inode cache organization:
┌────────────────────────────────────────┐
│ Filesystem 1 (/mnt/fs1)               │
│ ├─ Superblock (sb1)                   │
│ │  ├─ inode cache (hash table 1)      │
│ │  │  └─ inode (ino=500, sb=sb1)      │
│ │  └─ s_inode_lru_list                │
└────────────────────────────────────────┘

┌────────────────────────────────────────┐
│ Filesystem 2 (/mnt/fs2)               │
│ ├─ Superblock (sb2)                   │
│ │  ├─ inode cache (hash table 2)      │
│ │  │  └─ would have ino=500 (different file!)
│ │  └─ s_inode_lru_list                │
└────────────────────────────────────────┘

Problem:
- Inode numbers are unique within a filesystem, not globally
- /fs1:ino=500 is completely different file from /fs2:ino=500
- Hard link requires same inode
- But inode is cached separately per filesystem
- Cannot create hard link across filesystems

Solution (symbolic link instead):
$ ln -s /mnt/fs1/file.txt /mnt/fs2/symlink.txt

Symlink:
- Points to path "/mnt/fs1/file.txt"
- When followed, performs path lookup
- Works across filesystems
- But: not a hard link (different inode)
```

### Scenario 5: NFS and Network Filesystems

```
Scenario: NFS-mounted filesystem with stale inode cache

Client                           NFS Server
┌──────────────┐                ┌──────────────┐
│ Inode Cache  │ ←network→      │ Server FS    │
├──────────────┤                ├──────────────┤
│ ino=12345    │                │ ino=12345    │
│ size=1000    │                │ size=1000    │
│ mode=0644    │                │ mode=0644    │
└──────────────┘                └──────────────┘

Stale cache scenario:

1. Client reads file (1000 bytes)
   - Inode cached locally
   - Mode: 0644

2. Server: File deleted and recreated with new content
   Server deletes:
   - ino=12345 freed
   - New file gets ino=12345 (inode reuse)
   - size=5000, mode=0755
   
3. Client: Repeated read attempt
   - Inode still in client cache (ino=12345)
   - Client metadata: size=1000, mode=0644
   - Server metadata: size=5000, mode=0755
   - STALE CACHE - reading wrong metadata!

Solution: NFS generations

struct inode {
    u32 i_generation;  // Version number
};

Server assigns generation to each inode:
- ino=12345, generation=5
- After deletion: ino=12345, generation=6

Client caches: (ino=12345, generation=5, size=1000)

On next access:
- Server: (ino=12345, generation=6)
- Mismatch! Generation changed
- Client invalidates cache
- Re-reads from server

Generation prevents using wrong cached data.
```

### Scenario 6: Inode Leak Detection

```
Scenario: Kernel module or driver with inode leak

1. Module allocates inode repeatedly:
   for (i = 0; i < 1000000; i++) {
       inode = new_inode(sb);
       // Forgot to iput(inode) to release reference!
   }

2. After loop completes:
   - Kernel inode cache has 1M unreferenced inodes
   - Memory usage: 1M × 1KB = 1GB consumed
   - System runs out of RAM
   - OOM killer activates

3. Detection via /proc/sys/fs/inode-state:

   $ cat /proc/sys/fs/inode-state
   inodes-in-use       = 1000042
   inodes-pinned       = 1000000  ← ANOMALY!
   inodes-dentry-count = 42
   
4. Debug with tracing:
   
   # Enable kprobes on iget/iput
   echo 'p:iget new_inode' > /sys/kernel/debug/tracing/kprobes
   echo 'p:iput iput' >> /sys/kernel/debug/tracing/kprobes
   
   # Monitor calls
   $ cat /sys/kernel/debug/tracing/trace
   ktime=500us new_inode(sb) inode=0xffffaa00abc
   ktime=501us new_inode(sb) inode=0xffffaa00def
   ...
   ktime=600us iput(inode) inode=0xffffaa00abc  ← Only 1 iput!
   
5. Fix: Add missing iput()
   for (i = 0; i < 1000000; i++) {
       inode = new_inode(sb);
       iput(inode);  // Release reference
   }

Prevention tools:
- CONFIG_DEBUG_KMEMLEAK
- kmemleak command-line tool
- Valgrind (for userspace equivalent)
```

---

## Debugging and Introspection {#debugging}

### 1. /proc Interface

```bash
# View inode cache statistics
$ cat /proc/sys/fs/inode-state
inodes-in-use       = 54321
inodes-pinned       = 1000
inodes-unused       = 53321

# Explanation:
# - in-use: inode has active references (ref_count > 0)
# - pinned: inode cannot be evicted (I_Freeing or other blocks)
# - unused: inode in cache but not currently referenced
```

### 2. /sys/kernel/debug Interface

```bash
# Enable inode tracing (requires CONFIG_TRACEPOINTS)
echo 1 > /sys/kernel/debug/tracing/events/fs/inode_new/enable
echo 1 > /sys/kernel/debug/tracing/events/fs/inode_free/enable

# View trace
$ cat /sys/kernel/debug/tracing/trace
<...>-1234  [001] .... 12345.123456: inode_new: dev=253:0 ino=54321 mode=0100644
<...>-1234  [001] .... 12345.123600: inode_free: dev=253:0 ino=54321

# View statistics per superblock
$ cat /sys/kernel/debug/fs/ext4/

# Inode cache pressure (shrinker stats)
$ cat /proc/pressure/memory
some avg10=0.00 avg300=0.00 avg3600=0.00 total=0
full avg10=0.00 avg300=0.00 avg3600=0.00 total=0
```

### 3. Systemtap Instrumentation

```bash
# Count inode allocations per process
cat > trace_inodes.stp << 'EOF'
probe module("ext4").function("ext4_iget") {
    printf("[%d] %s allocating inode\n", pid(), execname())
}

probe module("ext4").function("iput") {
    printf("[%d] %s releasing inode\n", pid(), execname())
}
EOF

stap trace_inodes.stp
```

### 4. perf Profiling

```bash
# Count inode cache hit/miss rate
perf stat -e 'fs:inode_new,fs:inode_reuse' ./my_app

# Record inode operations
perf record -g -e 'fs:inode_*' sleep 10
perf report

# Flame graph
perf script | stackcollapse-perf.pl | flamegraph.pl > perf.svg
```

### 5. Custom Kernel Module for Debugging

```c
// debug_inode_cache.c - Monitor inode cache in real-time

#include <linux/module.h>
#include <linux/fs.h>
#include <linux/seq_file.h>
#include <linux/proc_fs.h>

MODULE_LICENSE("GPL");
MODULE_AUTHOR("Kernel Developer");
MODULE_DESCRIPTION("Inode cache debugging interface");

#define MAX_INODES_TRACK 10000

static struct {
    unsigned long ino;
    unsigned int ref_count;
    unsigned long state;
    ktime_t alloc_time;
    char name[256];
} tracked_inodes[MAX_INODES_TRACK];

static spinlock_t track_lock;
static int num_tracked = 0;

// Hook into inode allocation
static void track_inode_new(struct inode *inode)
{
    spin_lock(&track_lock);
    
    if (num_tracked < MAX_INODES_TRACK) {
        tracked_inodes[num_tracked].ino = inode->i_ino;
        tracked_inodes[num_tracked].ref_count = atomic_read(&inode->i_count);
        tracked_inodes[num_tracked].state = inode->i_state;
        tracked_inodes[num_tracked].alloc_time = ktime_get();
        num_tracked++;
    }
    
    spin_unlock(&track_lock);
}

// Proc interface to read tracked inodes
static int proc_show_inodes(struct seq_file *m, void *v)
{
    int i;
    
    seq_printf(m, "Tracked Inodes: %d\n", num_tracked);
    seq_printf(m, "%-10s %-10s %-10s %-20s\n", 
               "ino", "refs", "state", "age_ms");
    
    spin_lock(&track_lock);
    
    for (i = 0; i < num_tracked; i++) {
        ktime_t age = ktime_sub(ktime_get(), tracked_inodes[i].alloc_time);
        seq_printf(m, "%-10lu %-10u 0x%-8lx %-20lld\n",
                   tracked_inodes[i].ino,
                   tracked_inodes[i].ref_count,
                   tracked_inodes[i].state,
                   ktime_to_ms(age));
    }
    
    spin_unlock(&track_lock);
    
    return 0;
}

static const struct proc_ops proc_ops = {
    .proc_show = proc_show_inodes,
};

static int __init debug_inode_init(void)
{
    spin_lock_init(&track_lock);
    
    proc_create("inode_debug", 0, NULL, &proc_ops);
    
    printk(KERN_INFO "Inode debug module loaded\n");
    return 0;
}

module_exit__init(debug_inode_init);

static void __exit debug_inode_exit(void)
{
    remove_proc_entry("inode_debug", NULL);
    printk(KERN_INFO "Inode debug module unloaded\n");
}

module_exit(debug_inode_exit);
```

Compile and run:
```bash
make -C /lib/modules/$(uname -r)/build M=$(pwd) modules
insmod debug_inode_cache.ko
cat /proc/inode_debug
rmmod debug_inode_cache
```

### 6. KGDB Debugging

```bash
# With kernel compiled with CONFIG_KGDB=y

# In GDB (on host machine connected to target via serial):
(gdb) break evict_inode
Breakpoint 1 at 0xffffffff8123abc0

(gdb) continue
Breakpoint 1, evict_inode (inode=0xffffa00abc123) at fs/inode.c:234

# Inspect inode structure
(gdb) p *inode
$1 = {
  i_ino = 54321,
  i_mode = 33188,  // Regular file, 0644
  i_uid = 1000,
  i_gid = 1000,
  i_size = 4096,
  i_state = 0x100,  // I_FREEING
  i_count = {
    counter = 0  // No references
  },
  ...
}

(gdb) p inode->i_dentry
$2 = {
  next = 0xffffa00def456,
  ...
}
```

---

## Summary: Mental Model for Inode Cache Efficiency

### Key Principles

1. **Locality of Reference**
   - Frequently accessed inode fields in first cache line
   - Minimizes cache misses on hot path
   - ~100ns for cache hit vs. 1ms for disk miss

2. **Hash Table Efficiency**
   - O(1) average lookup (with good hash function)
   - Combined (superblock, ino) hash prevents collisions
   - Collision chain traversal is rare (<1% of lookups)

3. **Reference Counting**
   - Prevents premature eviction of in-use inodes
   - LRU ordering for eviction policy
   - Lazy eviction under memory pressure

4. **Lock-Free Design Where Possible**
   - RCU read-side for inode lookups
   - Spinlock critical sections < 100ns
   - Sleepable mutexes for filesystem-specific operations

5. **Writeback Batching**
   - Dirty inodes don't sync immediately
   - Async writeback worker reduces latency
   - Periodic fsync prevents unbounded dirty data

6. **Multi-Level Caching**
   - Page cache (file content)
   - Dentry cache (path components)
   - Inode cache (metadata)
   - Each level serves a different access pattern

### Performance Implications

| Operation | Cold Cache | Warm Cache | Hot Cache |
|-----------|-----------|-----------|-----------|
| open() | 5ms (disk I/O) | 100μs (page cache) | 50μs (L1 cache) |
| stat() | 3ms | 100μs | 50μs |
| read() | 1ms per 4KB | 100ns (page hit) | 50ns (CPU cache) |
| unlink() | 5ms | 200μs | 100μs |

### Typical Inode Cache Behavior

- **Cache Hit Rate**: 95-99% on typical workloads
- **Memory Usage**: ~1KB per cached inode
- **Eviction Frequency**: ~10% of inodes per minute (active workloads)
- **Lock Contention**: < 1% on systems with < 16 CPUs

---

## References and Further Reading

- Linux kernel source: `fs/inode.c`, `fs/dcache.c`
- Books: *Linux Kernel Internals* (Bovet & Cesati), *Professional Linux Kernel Architecture* (Mauerer)
- Documentation: `kernel-doc fs/inode.c`, LWN.net articles on VFS
- Performance tuning: `Documentation/filesystems/vfs.txt`

---

**Final Note**: The inode cache is a foundational concept that bridges applications, the VFS layer, and storage. Deep understanding of its architecture, locking, and lifecycle enables writing efficient filesystem code, debugging performance issues, and understanding kernel memory usage patterns.
