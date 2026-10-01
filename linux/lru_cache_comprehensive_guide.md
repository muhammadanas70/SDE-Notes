# LRU Cache: Comprehensive Deep-Dive Guide

## Table of Contents
1. [Fundamentals & Theory](#fundamentals--theory)
2. [Memory & Architecture](#memory--architecture)
3. [Data Structure Design](#data-structure-design)
4. [Implementation: Go](#implementation-go)
5. [Implementation: C](#implementation-c)
6. [Implementation: Rust](#implementation-rust)
7. [Linux Kernel Concepts](#linux-kernel-concepts)
8. [Performance Analysis](#performance-analysis)
9. [Thread Safety & Concurrency](#thread-safety--concurrency)
10. [Testing & Debugging](#testing--debugging)
11. [Production Considerations](#production-considerations)

---

## Fundamentals & Theory

### What is an LRU Cache?

An LRU (Least Recently Used) Cache is a data structure that:
- Stores a **fixed, bounded number of key-value pairs** (capacity constraint)
- **Evicts the least recently used item** when the cache is full and a new item must be inserted
- Provides **O(1) average-case lookup, insertion, and deletion** 
- Tracks **access recency** to determine eviction order

### Why LRU?

**The Principle**: Recent accesses are statistically more likely to be accessed again (temporal locality).

Real-world scenarios:
- CPU L1/L2/L3 caches use LRU or LRU-variants
- Virtual memory page replacement (Linux: `reclaim` system)
- Database buffer pools (InnoDB, PostgreSQL)
- CDN edge caches
- DNS resolvers
- Network packet buffers
- Kernel VFS dentry cache

### Key Invariants

An LRU cache must maintain:

1. **Capacity Invariant**: `size <= capacity`
2. **Ordering Invariant**: Items are ordered by access time (most recent at head/tail depending on implementation)
3. **Uniqueness Invariant**: Each key appears at most once
4. **Consistency Invariant**: All internal data structures stay synchronized

Violating these breaks the contract.

### Access Patterns Matter

Consider three patterns:

```
Pattern A (Uniform Random):
Access: [1, 2, 3, 4, 5, 1, 3, 2, 4]
→ LRU works well, eviction matches access patterns

Pattern B (Working Set Locality):
Access: [1, 1, 1, 2, 2, 2, 3, 3, 3] (working set size = 3)
→ LRU is optimal for bounded working sets

Pattern C (Scan Pattern):
Access: [1, 2, 3, 4, 5, 6, 7, 8] (capacity = 4)
→ LRU thrashes, every new access evicts an old one
→ Full-scan sequential workloads are worst-case for LRU
→ Alternatives: LFU, ARC, CLOCK needed here
```

---

## Memory & Architecture

### Core Challenge: Dual Data Structure Requirement

To achieve O(1) operations, we need:

1. **Fast lookup by key** → Hash table (HashMap)
2. **Fast ordering by recency** → Doubly-linked list (DLL)

These must stay **in perfect sync**.

### Architecture Diagram (High Level)

```
┌─────────────────────────────────────────────────────────────┐
│                        LRU CACHE                             │
└─────────────────────────────────────────────────────────────┘
                              │
                ┌─────────────┴──────────────┐
                │                            │
                ▼                            ▼
        ┌──────────────────┐        ┌──────────────────────┐
        │   HASH TABLE     │        │  DOUBLY-LINKED LIST  │
        ├──────────────────┤        ├──────────────────────┤
        │ O(1) lookup      │        │ O(1) reordering      │
        │ key → node ref   │        │ head = most recent   │
        │                  │        │ tail = least recent  │
        └──────────────────┘        └──────────────────────┘
                │                            │
                └────────────┬───────────────┘
                     (shared nodes)
                     Each node stores:
                     - key
                     - value
                     - prev pointer
                     - next pointer
```

### Node Structure in Memory

```
Memory Layout for a Single Cache Node:

    ┌────────────────────────────────────────┐
    │             KEY FIELD                  │  bytes 0-31   (string or u64)
    ├────────────────────────────────────────┤
    │             VALUE FIELD                │  bytes 32-63  (can be large)
    ├────────────────────────────────────────┤
    │           PREV POINTER                 │  bytes 64-71  (address)
    ├────────────────────────────────────────┤
    │           NEXT POINTER                 │  bytes 72-79  (address)
    ├────────────────────────────────────────┤
    │      REFERENCE COUNT (opt)             │  bytes 80-87  (atomic u64)
    └────────────────────────────────────────┘
    Total: ~88 bytes per node (cache-line aligned in practice)

This structure is embedded in the hash table's bucket entry.
Pointer chasing: key lookup → hash table → node → follows DLL chain.
```

### Cache Line Implications

Modern CPUs (x86-64) have 64-byte cache lines.

**Critical insight**: Pointer chasing (following linked list next/prev) causes:
- Cache miss on each node access (data is scattered in heap)
- Load latency: ~200-300 nanoseconds per miss
- Better to use **contiguous memory** (vector/array) when possible

However, we **cannot avoid pointer chasing** for O(1) reordering in linked list.

---

## Data Structure Design

### The Fundamental Trade-off

| Approach | Lookup | Reorder | Memory | Contiguity |
|----------|--------|---------|--------|-----------|
| Hash + DLL | O(1) | O(1) | ~2x | Poor |
| Hash + Array + Heap | O(1) | O(log n) | ~1.5x | Better |
| Hash + Bitmap | O(1) | O(n) | ~1x | Good |
| Cuckoo Hash | O(1) amort | O(n) | ~1.2x | Very Good |

**We choose Hash + DLL** because:
- True O(1) eviction and reordering (worst-case)
- Simplicity and debuggability
- Industry standard (Redis, Memcached, Linux kernel use this)

### Anatomy of Operations

#### GET operation
```
1. Hash table lookup: hash(key) → bucket
2. Check if collision, walk chain or probe
3. Find node N
4. Update N's timestamp/position
5. Move N to head of DLL (most recent)
6. Return N.value
```

Memory operations:
- 1 hash computation
- 1-3 pointer dereferences (hash table probe)
- 3-4 pointer updates (unlink from old position, link at head)
- Cache misses: ~2-3 per operation

#### PUT operation (key exists)
```
Same as GET, but also update value field
```

#### PUT operation (key doesn't exist, cache not full)
```
1. Allocate new node N
2. Hash table insert
3. Link N at head of DLL
4. Increment size
```

#### PUT operation (key doesn't exist, cache full)
```
1. Allocate new node N
2. Identify eviction victim: tail of DLL (least recent)
3. Remove victim from hash table
4. Remove victim from DLL
5. Free victim's memory
6. Insert N into hash table
7. Link N at head of DLL
8. Size unchanged (cache full invariant)
```

**Cost**: 1 allocation + 1 deallocation per eviction

---

## Implementation: Go

Go is ideal for this because:
- Built-in `sync.RWMutex` for thread-safe access
- `container/list` standard library (doubly-linked list)
- Garbage collection handles memory automatically
- Simple, readable syntax

### Basic Go Implementation

```go
package main

import (
	"container/list"
	"fmt"
	"sync"
)

// Node represents a single item in the LRU cache
type Node struct {
	Key   string
	Value interface{}
}

// LRUCache implements a thread-safe LRU cache
type LRUCache struct {
	mu       sync.RWMutex
	capacity int
	cache    map[string]*list.Element // key → *list.Element
	list     *list.List               // doubly-linked list for ordering
}

// NewLRUCache creates a new LRU cache with given capacity
func NewLRUCache(capacity int) *LRUCache {
	if capacity <= 0 {
		panic("capacity must be positive")
	}
	return &LRUCache{
		capacity: capacity,
		cache:    make(map[string]*list.Element),
		list:     list.New(),
	}
}

// Get retrieves value by key and updates recency
func (lru *LRUCache) Get(key string) (interface{}, bool) {
	lru.mu.Lock()
	defer lru.mu.Unlock()

	// Fast path: check if key exists
	elem, exists := lru.cache[key]
	if !exists {
		return nil, false
	}

	// Move to front (most recent)
	lru.list.MoveToFront(elem)
	return elem.Value.(*Node).Value, true
}

// Put inserts or updates a key-value pair
func (lru *LRUCache) Put(key string, value interface{}) {
	lru.mu.Lock()
	defer lru.mu.Unlock()

	// Case 1: Key already exists
	if elem, exists := lru.cache[key]; exists {
		// Update value and move to front
		elem.Value.(*Node).Value = value
		lru.list.MoveToFront(elem)
		return
	}

	// Case 2: Key doesn't exist, cache not full
	if lru.list.Len() < lru.capacity {
		node := &Node{Key: key, Value: value}
		elem := lru.list.PushFront(node)
		lru.cache[key] = elem
		return
	}

	// Case 3: Key doesn't exist, cache full → evict LRU
	// Remove tail (least recently used)
	tail := lru.list.Back()
	if tail != nil {
		lru.list.Remove(tail)
		node := tail.Value.(*Node)
		delete(lru.cache, node.Key)
	}

	// Insert new node at front
	node := &Node{Key: key, Value: value}
	elem := lru.list.PushFront(node)
	lru.cache[key] = elem
}

// Size returns current number of items
func (lru *LRUCache) Size() int {
	lru.mu.RLock()
	defer lru.mu.RUnlock()
	return lru.list.Len()
}

// Clear removes all items
func (lru *LRUCache) Clear() {
	lru.mu.Lock()
	defer lru.mu.Unlock()
	lru.cache = make(map[string]*list.Element)
	lru.list.Init()
}

// Stats returns cache statistics
func (lru *LRUCache) Stats() map[string]interface{} {
	lru.mu.RLock()
	defer lru.mu.RUnlock()
	return map[string]interface{}{
		"capacity": lru.capacity,
		"size":     lru.list.Len(),
		"usage":    float64(lru.list.Len()) / float64(lru.capacity),
	}
}

// DebugPrint prints cache contents in order (head to tail)
func (lru *LRUCache) DebugPrint() {
	lru.mu.RLock()
	defer lru.mu.RUnlock()

	fmt.Println("=== LRU Cache Contents (head=most recent, tail=least recent) ===")
	for i, elem := 1, lru.list.Front(); elem != nil; elem, i = elem.Next(), i+1 {
		node := elem.Value.(*Node)
		fmt.Printf("%d. [%s] = %v\n", i, node.Key, node.Value)
	}
	fmt.Println("==============================================================")
}
```

### Advanced Go: Concurrent Benchmarking

```go
package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Metrics tracks cache performance
type Metrics struct {
	hits      int64
	misses    int64
	evictions int64
	puts      int64
}

// InstrumentedLRUCache wraps LRUCache with metrics
type InstrumentedLRUCache struct {
	*LRUCache
	metrics Metrics
}

// Get with metrics
func (ilru *InstrumentedLRUCache) Get(key string) (interface{}, bool) {
	val, ok := ilru.LRUCache.Get(key)
	if ok {
		atomic.AddInt64(&ilru.metrics.hits, 1)
	} else {
		atomic.AddInt64(&ilru.metrics.misses, 1)
	}
	return val, ok
}

// Put with metrics
func (ilru *InstrumentedLRUCache) Put(key string, value interface{}) {
	// Check if eviction will happen
	ilru.mu.RLock()
	willEvict := lru.list.Len() >= lru.capacity && !lru.cache[key]
	ilru.mu.RUnlock()

	ilru.LRUCache.Put(key, value)

	if willEvict {
		atomic.AddInt64(&ilru.metrics.evictions, 1)
	}
	atomic.AddInt64(&ilru.metrics.puts, 1)
}

// PrintMetrics displays performance statistics
func (ilru *InstrumentedLRUCache) PrintMetrics() {
	hits := atomic.LoadInt64(&ilru.metrics.hits)
	misses := atomic.LoadInt64(&ilru.metrics.misses)
	total := hits + misses

	var hitRate float64
	if total > 0 {
		hitRate = float64(hits) / float64(total) * 100
	}

	fmt.Printf("Cache Performance:\n")
	fmt.Printf("  Hits:      %d\n", hits)
	fmt.Printf("  Misses:    %d\n", misses)
	fmt.Printf("  Hit Rate:  %.2f%%\n", hitRate)
	fmt.Printf("  Evictions: %d\n", atomic.LoadInt64(&ilru.metrics.evictions))
	fmt.Printf("  Puts:      %d\n", atomic.LoadInt64(&ilru.metrics.puts))
}

// Benchmark concurrent reads/writes
func BenchmarkConcurrentAccess(b *testing.B) {
	cache := NewLRUCache(1000)
	numGoroutines := 10
	opsPerGoroutine := b.N / numGoroutines

	b.ResetTimer()
	var wg sync.WaitGroup

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				key := fmt.Sprintf("key_%d_%d", id, i%100)
				cache.Put(key, i)
				cache.Get(key)
			}
		}(g)
	}

	wg.Wait()
}
```

### Key Design Decisions in Go Implementation

1. **Mutex Granularity**: Single RWMutex per cache
   - Trade-off: Simple but limits concurrent reads (Go's sync.RWMutex allows concurrent reads)
   - Production: Use per-shard locking for scaling

2. **Memory Efficiency**: Hash map + linked list = ~2x overhead
   - Each entry needs: key (string), value (interface{}), list node pointers
   - `interface{}` adds 16 bytes (type pointer + data pointer)

3. **Garbage Collection**: Go's GC handles evicted nodes
   - Benefit: No manual memory management
   - Cost: GC pauses on large evictions
   - Mitigation: Batch evictions, use memory pools

---

## Implementation: C

C requires manual memory management, making the implementation more complex but revealing low-level details.

### Core Data Structures

```c
#ifndef LRU_CACHE_H
#define LRU_CACHE_H

#include <stdint.h>
#include <string.h>
#include <stdlib.h>
#include <pthread.h>
#include <stdbool.h>

/* Cache node in doubly-linked list */
typedef struct cache_node {
    char *key;                    // Heap-allocated key
    void *value;                  // User-provided value pointer
    size_t value_size;           // Size of value for deep copy
    
    struct cache_node *prev;     // Previous in LRU order
    struct cache_node *next;     // Next in LRU order
    
    uint64_t access_count;       // For statistics
    uint64_t last_access_time;   // Timestamp in nanoseconds
} cache_node_t;

/* Hash table entry (separate chaining for collisions) */
typedef struct hash_entry {
    cache_node_t *node;          // Pointer to actual node in DLL
    struct hash_entry *next;     // Collision chain
} hash_entry_t;

/* LRU Cache structure */
typedef struct {
    hash_entry_t *hash_table;    // Hash table for O(1) lookup
    uint32_t table_size;         // Size of hash table (prime number)
    uint32_t table_mask;         // For faster modulo operation
    
    cache_node_t *head;          // Most recently used
    cache_node_t *tail;          // Least recently used
    
    uint32_t capacity;           // Maximum number of items
    uint32_t size;               // Current number of items
    
    uint64_t total_accesses;     // Statistics
    uint64_t total_evictions;    // Statistics
    
    pthread_rwlock_t lock;       // Read-write lock
} lru_cache_t;

/* Function declarations */
lru_cache_t *lru_cache_create(uint32_t capacity);
void lru_cache_destroy(lru_cache_t *cache);
bool lru_cache_get(lru_cache_t *cache, const char *key, void **out_value);
bool lru_cache_put(lru_cache_t *cache, const char *key, void *value, size_t value_size);
bool lru_cache_delete(lru_cache_t *cache, const char *key);
uint32_t lru_cache_size(lru_cache_t *cache);
void lru_cache_clear(lru_cache_t *cache);

#endif // LRU_CACHE_H
```

### Implementation

```c
#include "lru_cache.h"
#include <stdio.h>
#include <time.h>

/* Murmur3 hash function (fast, widely used) */
static uint32_t murmurhash3(const char *key, uint32_t table_size) {
    uint32_t h = 0x9e3779b9;
    for (size_t i = 0; key[i] != '\0'; i++) {
        h ^= key[i];
        h = (h << 13) | (h >> 19);
        h *= 0x85ebca6b;
    }
    return h % table_size;
}

/* Helper: Get current time in nanoseconds */
static uint64_t get_time_ns(void) {
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return ts.tv_sec * 1000000000ULL + ts.tv_nsec;
}

/* Create new cache */
lru_cache_t *lru_cache_create(uint32_t capacity) {
    if (capacity == 0) {
        return NULL;
    }

    lru_cache_t *cache = (lru_cache_t *)malloc(sizeof(lru_cache_t));
    if (!cache) return NULL;

    /* Choose hash table size (next power of 2 for efficiency) */
    uint32_t table_size = 16;
    while (table_size < capacity * 4) {
        table_size *= 2;
    }

    cache->hash_table = (hash_entry_t *)calloc(table_size, sizeof(hash_entry_t));
    if (!cache->hash_table) {
        free(cache);
        return NULL;
    }

    cache->table_size = table_size;
    cache->table_mask = table_size - 1;
    cache->capacity = capacity;
    cache->size = 0;
    cache->head = NULL;
    cache->tail = NULL;
    cache->total_accesses = 0;
    cache->total_evictions = 0;

    if (pthread_rwlock_init(&cache->lock, NULL) != 0) {
        free(cache->hash_table);
        free(cache);
        return NULL;
    }

    return cache;
}

/* Move node to head (most recent) */
static void move_to_head(lru_cache_t *cache, cache_node_t *node) {
    if (node == cache->head) {
        return;  // Already at head
    }

    /* Unlink from current position */
    if (node->prev) {
        node->prev->next = node->next;
    }
    if (node->next) {
        node->next->prev = node->prev;
    }
    if (node == cache->tail) {
        cache->tail = node->prev;
    }

    /* Link at head */
    node->prev = NULL;
    node->next = cache->head;
    if (cache->head) {
        cache->head->prev = node;
    }
    cache->head = node;

    if (!cache->tail) {
        cache->tail = node;
    }
}

/* Remove node from list and hash table */
static void evict_node(lru_cache_t *cache, cache_node_t *node) {
    /* Unlink from DLL */
    if (node->prev) {
        node->prev->next = node->next;
    } else {
        cache->head = node->next;
    }

    if (node->next) {
        node->next->prev = node->prev;
    } else {
        cache->tail = node->prev;
    }

    /* Remove from hash table */
    uint32_t index = murmurhash3(node->key, cache->table_size);
    hash_entry_t *entry = &cache->hash_table[index];
    hash_entry_t *prev_entry = NULL;

    while (entry && entry->node != node) {
        prev_entry = entry;
        entry = entry->next;
    }

    if (entry) {
        if (prev_entry) {
            prev_entry->next = entry->next;
            free(entry);  // Free collision chain entry
        } else {
            if (entry->next) {
                hash_entry_t *temp = entry->next;
                entry->node = temp->node;
                entry->next = temp->next;
                free(temp);
            } else {
                entry->node = NULL;
            }
        }
    }

    /* Free node resources */
    free(node->key);
    free(node->value);
    free(node);

    cache->size--;
    cache->total_evictions++;
}

/* GET operation */
bool lru_cache_get(lru_cache_t *cache, const char *key, void **out_value) {
    if (!cache || !key || !out_value) {
        return false;
    }

    pthread_rwlock_rdlock(&cache->lock);

    uint32_t index = murmurhash3(key, cache->table_size);
    hash_entry_t *entry = &cache->hash_table[index];

    while (entry && entry->node) {
        if (strcmp(entry->node->key, key) == 0) {
            /* Found! Update timestamp but don't reorder under read lock */
            *out_value = entry->node->value;
            cache->total_accesses++;
            
            pthread_rwlock_unlock(&cache->lock);

            /* Reorder needs write lock */
            pthread_rwlock_wrlock(&cache->lock);
            entry->node->last_access_time = get_time_ns();
            entry->node->access_count++;
            move_to_head(cache, entry->node);
            pthread_rwlock_unlock(&cache->lock);

            return true;
        }
        entry = entry->next;
    }

    pthread_rwlock_unlock(&cache->lock);
    return false;
}

/* PUT operation */
bool lru_cache_put(lru_cache_t *cache, const char *key, void *value, size_t value_size) {
    if (!cache || !key) {
        return false;
    }

    pthread_rwlock_wrlock(&cache->lock);

    uint32_t index = murmurhash3(key, cache->table_size);
    hash_entry_t *entry = &cache->hash_table[index];

    /* Check if key already exists */
    while (entry && entry->node) {
        if (strcmp(entry->node->key, key) == 0) {
            /* Update existing value */
            free(entry->node->value);
            entry->node->value = malloc(value_size);
            if (!entry->node->value) {
                pthread_rwlock_unlock(&cache->lock);
                return false;
            }
            memcpy(entry->node->value, value, value_size);
            entry->node->value_size = value_size;
            entry->node->last_access_time = get_time_ns();
            move_to_head(cache, entry->node);
            
            pthread_rwlock_unlock(&cache->lock);
            return true;
        }
        entry = entry->next;
    }

    /* Key doesn't exist - create new node */
    cache_node_t *new_node = (cache_node_t *)malloc(sizeof(cache_node_t));
    if (!new_node) {
        pthread_rwlock_unlock(&cache->lock);
        return false;
    }

    new_node->key = (char *)malloc(strlen(key) + 1);
    new_node->value = malloc(value_size);

    if (!new_node->key || !new_node->value) {
        free(new_node->key);
        free(new_node->value);
        free(new_node);
        pthread_rwlock_unlock(&cache->lock);
        return false;
    }

    strcpy(new_node->key, key);
    memcpy(new_node->value, value, value_size);
    new_node->value_size = value_size;
    new_node->access_count = 1;
    new_node->last_access_time = get_time_ns();
    new_node->prev = NULL;
    new_node->next = NULL;

    /* Check if cache is full */
    if (cache->size >= cache->capacity) {
        /* Evict tail (least recently used) */
        if (cache->tail) {
            evict_node(cache, cache->tail);
        }
    }

    /* Insert into hash table */
    entry = &cache->hash_table[index];
    if (entry->node == NULL) {
        entry->node = new_node;
    } else {
        /* Collision: add to chain */
        hash_entry_t *new_entry = (hash_entry_t *)malloc(sizeof(hash_entry_t));
        if (!new_entry) {
            free(new_node->key);
            free(new_node->value);
            free(new_node);
            pthread_rwlock_unlock(&cache->lock);
            return false;
        }
        new_entry->node = new_node;
        new_entry->next = entry->next;
        entry->next = new_entry;
    }

    /* Insert at head of DLL */
    move_to_head(cache, new_node);
    cache->size++;

    pthread_rwlock_unlock(&cache->lock);
    return true;
}

/* DELETE operation */
bool lru_cache_delete(lru_cache_t *cache, const char *key) {
    if (!cache || !key) {
        return false;
    }

    pthread_rwlock_wrlock(&cache->lock);

    uint32_t index = murmurhash3(key, cache->table_size);
    hash_entry_t *entry = &cache->hash_table[index];

    while (entry && entry->node) {
        if (strcmp(entry->node->key, key) == 0) {
            cache_node_t *node = entry->node;
            evict_node(cache, node);
            pthread_rwlock_unlock(&cache->lock);
            return true;
        }
        entry = entry->next;
    }

    pthread_rwlock_unlock(&cache->lock);
    return false;
}

/* Get current size */
uint32_t lru_cache_size(lru_cache_t *cache) {
    if (!cache) return 0;
    pthread_rwlock_rdlock(&cache->lock);
    uint32_t size = cache->size;
    pthread_rwlock_unlock(&cache->lock);
    return size;
}

/* Clear all entries */
void lru_cache_clear(lru_cache_t *cache) {
    if (!cache) return;

    pthread_rwlock_wrlock(&cache->lock);

    cache_node_t *current = cache->head;
    while (current) {
        cache_node_t *next = current->next;
        free(current->key);
        free(current->value);
        free(current);
        current = next;
    }

    for (uint32_t i = 0; i < cache->table_size; i++) {
        hash_entry_t *entry = cache->hash_table[i].next;
        while (entry) {
            hash_entry_t *next = entry->next;
            free(entry);
            entry = next;
        }
        cache->hash_table[i].node = NULL;
        cache->hash_table[i].next = NULL;
    }

    cache->head = NULL;
    cache->tail = NULL;
    cache->size = 0;

    pthread_rwlock_unlock(&cache->lock);
}

/* Destroy cache */
void lru_cache_destroy(lru_cache_t *cache) {
    if (!cache) return;

    lru_cache_clear(cache);
    pthread_rwlock_destroy(&cache->lock);
    free(cache->hash_table);
    free(cache);
}
```

### Memory Safety Considerations in C

```c
/* Common pitfalls and how to avoid them */

/* PITFALL 1: Use-after-free */
// WRONG:
cache_node_t *node = cache->head;
evict_node(cache, node);
printf("%s\n", node->key);  // node is freed!

// RIGHT: Store key before eviction
char key_copy[256];
strcpy(key_copy, cache->head->key);
evict_node(cache, cache->head);
printf("%s\n", key_copy);

/* PITFALL 2: Double-free in collision chain */
// The hash_entry_t linked list can cause double-frees
// Solution: Separate allocation for overflow entries

/* PITFALL 3: Memory leak on allocation failure */
// Always check malloc return and unwind properly
new_node = malloc(...);
if (!new_node) {
    // Must still hold lock before unlocking
    pthread_rwlock_unlock(&cache->lock);
    return false;
}

/* PITFALL 4: Pointer validity across lock release */
// Never assume a pointer is valid after releasing lock
// Another thread might have evicted that node
```

### C Testing

```c
#include <assert.h>

void test_lru_basic(void) {
    lru_cache_t *cache = lru_cache_create(3);
    assert(cache != NULL);
    assert(lru_cache_size(cache) == 0);

    int val1 = 100, val2 = 200, val3 = 300, val4 = 400;

    /* Fill cache */
    assert(lru_cache_put(cache, "key1", &val1, sizeof(int)));
    assert(lru_cache_put(cache, "key2", &val2, sizeof(int)));
    assert(lru_cache_put(cache, "key3", &val3, sizeof(int)));
    assert(lru_cache_size(cache) == 3);

    /* Access key1 to make it recent */
    int *retrieved = NULL;
    assert(lru_cache_get(cache, "key1", (void **)&retrieved));
    assert(*retrieved == 100);

    /* key2 is now LRU, so it should be evicted */
    assert(lru_cache_put(cache, "key4", &val4, sizeof(int)));
    assert(lru_cache_size(cache) == 3);
    assert(!lru_cache_get(cache, "key2", (void **)&retrieved));

    /* Verify order: head = key4, tail = key3 */
    assert(strcmp(cache->head->key, "key4") == 0);
    assert(strcmp(cache->tail->key, "key3") == 0);

    lru_cache_destroy(cache);
    printf("✓ test_lru_basic passed\n");
}
```

---

## Implementation: Rust

Rust's ownership system makes LRU cache implementation elegant and safe. No manual memory management needed, but we must reason about ownership.

```rust
use std::collections::HashMap;
use std::cell::RefCell;
use std::rc::Rc;
use std::ptr::NonNull;
use std::sync::{Arc, RwLock};

/// Node in doubly-linked list
#[derive(Debug)]
struct Node<K, V> {
    key: K,
    value: V,
    prev: Option<NonNull<Node<K, V>>>,
    next: Option<NonNull<Node<K, V>>>,
}

impl<K, V> Node<K, V> {
    fn new(key: K, value: V) -> Self {
        Node {
            key,
            value,
            prev: None,
            next: None,
        }
    }
}

/// Thread-safe LRU Cache
pub struct LRUCache<K: Eq + std::hash::Hash + Clone, V: Clone> {
    cache: Arc<RwLock<LRUCacheInner<K, V>>>,
}

struct LRUCacheInner<K, V> {
    map: HashMap<K, NonNull<Node<K, V>>>,
    head: Option<NonNull<Node<K, V>>>,
    tail: Option<NonNull<Node<K, V>>>,
    capacity: usize,
    size: usize,
    metrics: CacheMetrics,
}

#[derive(Default, Debug)]
struct CacheMetrics {
    hits: u64,
    misses: u64,
    evictions: u64,
}

impl<K: Eq + std::hash::Hash + Clone, V: Clone> LRUCache<K, V> {
    pub fn new(capacity: usize) -> Self {
        if capacity == 0 {
            panic!("Capacity must be positive");
        }

        LRUCache {
            cache: Arc::new(RwLock::new(LRUCacheInner {
                map: HashMap::new(),
                head: None,
                tail: None,
                capacity,
                size: 0,
                metrics: CacheMetrics::default(),
            })),
        }
    }

    pub fn get(&self, key: &K) -> Option<V> {
        let mut inner = self.cache.write().unwrap();
        
        if let Some(node_ptr) = inner.map.get(key) {
            // SAFETY: node_ptr is valid as long as it's in the map
            unsafe {
                let node = node_ptr.as_mut().unwrap();
                let value = node.value.clone();
                
                // Move to head (most recent)
                inner.move_to_head(*node_ptr);
                
                inner.metrics.hits += 1;
                Some(value)
            }
        } else {
            inner.metrics.misses += 1;
            None
        }
    }

    pub fn put(&self, key: K, value: V) {
        let mut inner = self.cache.write().unwrap();

        // Case 1: Key exists, update value and move to head
        if let Some(node_ptr) = inner.map.get(&key) {
            unsafe {
                let node = node_ptr.as_mut().unwrap();
                node.value = value;
                inner.move_to_head(*node_ptr);
            }
            return;
        }

        // Case 2: Key doesn't exist, cache not full
        if inner.size < inner.capacity {
            let node = Box::into_raw(Box::new(Node::new(key.clone(), value)));
            let node_ptr = NonNull::new(node).unwrap();
            
            inner.insert_at_head(node_ptr);
            inner.map.insert(key, node_ptr);
            inner.size += 1;
            return;
        }

        // Case 3: Key doesn't exist, cache full → evict LRU
        if let Some(tail_ptr) = inner.tail {
            unsafe {
                let tail_node = tail_ptr.as_ref().unwrap();
                let evicted_key = tail_node.key.clone();
                inner.map.remove(&evicted_key);
                inner.remove_node(tail_ptr);
                let _ = Box::from_raw(tail_ptr.as_ptr());
            }
            inner.metrics.evictions += 1;
        }

        // Insert new node at head
        let node = Box::into_raw(Box::new(Node::new(key.clone(), value)));
        let node_ptr = NonNull::new(node).unwrap();
        inner.insert_at_head(node_ptr);
        inner.map.insert(key, node_ptr);
        inner.size = inner.capacity;
    }

    pub fn size(&self) -> usize {
        self.cache.read().unwrap().size
    }

    pub fn capacity(&self) -> usize {
        self.cache.read().unwrap().capacity
    }

    pub fn metrics(&self) -> (u64, u64, u64) {
        let inner = self.cache.read().unwrap();
        (inner.metrics.hits, inner.metrics.misses, inner.metrics.evictions)
    }

    pub fn clear(&self) {
        let mut inner = self.cache.write().unwrap();
        inner.clear();
    }
}

impl<K, V> LRUCacheInner<K, V> {
    /// Move node to head of list
    unsafe fn move_to_head(&mut self, node_ptr: NonNull<Node<K, V>>) {
        let node = node_ptr.as_mut().unwrap();
        
        if Some(node_ptr) == self.head {
            return;  // Already at head
        }

        // Unlink from current position
        if let Some(prev) = node.prev {
            prev.as_mut().unwrap().next = node.next;
        } else {
            self.head = node.next;
        }

        if let Some(next) = node.next {
            next.as_mut().unwrap().prev = node.prev;
        } else {
            self.tail = node.prev;
        }

        // Link at head
        node.prev = None;
        node.next = self.head;
        
        if let Some(head) = self.head {
            head.as_mut().unwrap().prev = Some(node_ptr);
        }
        self.head = Some(node_ptr);

        if self.tail.is_none() {
            self.tail = Some(node_ptr);
        }
    }

    /// Insert node at head
    unsafe fn insert_at_head(&mut self, node_ptr: NonNull<Node<K, V>>) {
        let node = node_ptr.as_mut().unwrap();
        node.prev = None;
        node.next = self.head;

        if let Some(head) = self.head {
            head.as_mut().unwrap().prev = Some(node_ptr);
        }

        self.head = Some(node_ptr);

        if self.tail.is_none() {
            self.tail = Some(node_ptr);
        }
    }

    /// Remove node from list
    unsafe fn remove_node(&mut self, node_ptr: NonNull<Node<K, V>>) {
        let node = node_ptr.as_ref().unwrap();

        if let Some(prev) = node.prev {
            prev.as_mut().unwrap().next = node.next;
        } else {
            self.head = node.next;
        }

        if let Some(next) = node.next {
            next.as_mut().unwrap().prev = node.prev;
        } else {
            self.tail = node.prev;
        }
    }

    /// Clear all entries
    fn clear(&mut self) {
        let mut current = self.head;
        while let Some(node_ptr) = current {
            unsafe {
                let node = node_ptr.as_ref().unwrap();
                current = node.next;
                let _ = Box::from_raw(node_ptr.as_ptr());
            }
        }
        self.map.clear();
        self.head = None;
        self.tail = None;
        self.size = 0;
    }
}

impl<K, V> Drop for LRUCacheInner<K, V> {
    fn drop(&mut self) {
        self.clear();
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_basic_operations() {
        let cache: LRUCache<String, i32> = LRUCache::new(2);

        cache.put("a".to_string(), 1);
        assert_eq!(cache.get(&"a".to_string()), Some(1));
        
        cache.put("b".to_string(), 2);
        assert_eq!(cache.size(), 2);
        
        cache.put("c".to_string(), 3);  // Should evict "b"
        assert_eq!(cache.size(), 2);
        assert_eq!(cache.get(&"b".to_string()), None);

        let (hits, misses, evictions) = cache.metrics();
        assert_eq!(hits, 1);
        assert_eq!(misses, 1);
        assert_eq!(evictions, 1);
    }

    #[test]
    fn test_lru_order() {
        let cache: LRUCache<String, String> = LRUCache::new(3);

        cache.put("a".to_string(), "val_a".to_string());
        cache.put("b".to_string(), "val_b".to_string());
        cache.put("c".to_string(), "val_c".to_string());

        // Access 'a' to make it recent
        assert_eq!(cache.get(&"a".to_string()), Some("val_a".to_string()));

        // Now 'b' should be LRU
        cache.put("d".to_string(), "val_d".to_string());

        assert_eq!(cache.get(&"b".to_string()), None);
        assert_eq!(cache.get(&"a".to_string()), Some("val_a".to_string()));
        assert_eq!(cache.get(&"c".to_string()), Some("val_c".to_string()));
        assert_eq!(cache.get(&"d".to_string()), Some("val_d".to_string()));
    }
}

fn main() {
    let cache: LRUCache<String, String> = LRUCache::new(2);

    cache.put("user:1".to_string(), "Alice".to_string());
    cache.put("user:2".to_string(), "Bob".to_string());

    println!("Get user:1: {:?}", cache.get(&"user:1".to_string()));
    println!("Get user:3: {:?}", cache.get(&"user:3".to_string()));

    cache.put("user:3".to_string(), "Charlie".to_string());

    println!("Size: {}", cache.size());
    let (hits, misses, evictions) = cache.metrics();
    println!("Hits: {}, Misses: {}, Evictions: {}", hits, misses, evictions);
}
```

### Rust Safety Guarantees

```rust
/* Why Rust's LRU is safer than C */

// 1. No manual deallocation
let cache = LRUCache::new(100);
let cloned = cache.clone();  // Arc means multiple owners, safe sharing
// Drops when no longer referenced → no memory leak

// 2. Borrow checker prevents data races
let cache = LRUCache::new(10);
let r1 = cache.get(&"key1");  // Borrows immutably
let r2 = cache.get(&"key2");  // Can do multiple reads
// let w = cache.put("key3", 3);  // ERROR: cannot mutate while borrowed

// 3. Raw pointer use is isolated and documented
unsafe {
    let node = node_ptr.as_mut().unwrap();  // Explicitly marks unsafe
}
// Unsafe block is reviewable and auditable

// 4. Type safety prevents key collisions
cache.put("name", "Alice");  // Type is String
// cache.put(42, "Alice");  // ERROR: type mismatch
```

---

## Linux Kernel Concepts

### VFS Dentry Cache

The Linux kernel uses LRU for the dentry cache (directory entry cache).

```c
/* From linux/fs/dcache.c (simplified) */

struct dentry {
    struct hlist_bl_node d_hash;     // Hash table node
    struct dentry *d_parent;         // Parent directory
    struct qstr d_name;              // File name
    struct inode *d_inode;           // Associated inode
    
    struct list_head d_lru;          // LRU chain (kernel uses macro DCACHE_LRU_LIST)
    unsigned long d_time;            // Last access time
    unsigned int d_flags;            // Flags (DCACHE_LRU_LIST indicates in LRU)
};

/* D-cache is split into 'hot' and 'cold' lists for performance */
struct dcache_lru {
    struct list_head list;           // actual LRU list
    spinlock_t lock;
};

static struct dcache_lru dentry_lru[NR_CPUS];  // Per-CPU for scalability

/* LRU eviction in kernel */
void dentry_lru_del(struct dentry *dentry) {
    // Remove from LRU list when used
    list_del(&dentry->d_lru);
}

void dentry_lru_add(struct dentry *dentry) {
    // Add to tail (least recent)
    list_add_tail(&dentry->d_lru, &dentry_lru->list);
}

void dentry_lru_move_tail(struct dentry *dentry) {
    // Move to tail (most recent in kernel's design)
    list_move_tail(&dentry->d_lru, &dentry_lru->list);
}

/* Shrink (evict) LRU entries under memory pressure */
int dcache_lru_shrink(struct shrinker *shrink, struct shrink_control *sc) {
    struct list_head *list;
    struct dentry *dentry, *tmp;
    int nr_shrunk = 0;
    
    spin_lock(&dentry_lru->lock);
    
    list_for_each_entry_safe(dentry, tmp, &dentry_lru->list, d_lru) {
        if (nr_shrunk >= sc->nr_to_scan) break;
        
        if (d_try_lock(dentry)) {
            /* Check if still has references */
            if (dentry->d_count != 0) {
                d_unlock(dentry);
                continue;
            }
            
            /* Safe to evict */
            list_del(&dentry->d_lru);
            prune_one_dentry(dentry);
            nr_shrunk++;
        }
    }
    
    spin_unlock(&dentry_lru->lock);
    return nr_shrunk;
}
```

### Page Cache (Buffer Cache)

The kernel uses LRU for the page cache (`mm/page_io.c`):

```c
/* Simplified Linux page LRU structure */

struct zone {
    spinlock_t lru_lock;
    struct list_head list[NR_LRU_LISTS];  // Multiple lists:
    // list[LRU_INACTIVE_ANON]
    // list[LRU_ACTIVE_ANON]
    // list[LRU_INACTIVE_FILE]
    // list[LRU_ACTIVE_FILE]
    
    unsigned long nr_scanned[NR_LRU_LISTS];
};

/* Adding page to LRU */
void lru_cache_add(struct page *page) {
    struct lruvec *lruvec;
    
    lruvec = mem_cgroup_lru_add_list(page, lru);
    list_add_tail(&page->lru, &lruvec->lists[lru]);
    
    __mod_zone_page_state(zone, NR_LRU_BASE + lru, 1);
}

/* Marking page as accessed (move to most recent) */
void mark_page_accessed(struct page *page) {
    if (!PageActive(page) && !PageUnevictable(page) &&
        PageReferenced(page)) {
        
        /* Move from inactive to active list */
        SetPageActive(page);
        list_move(&page->lru, &lruvec->lists[lru_type + LRU_ACTIVE]);
    }
    
    ClearPageReferenced(page);
    SetPageReferenced(page);
}

/* Eviction under memory pressure (kswapd / direct reclaim) */
static unsigned long shrink_inactive_list(unsigned long nr_to_scan,
                                          struct lruvec *lruvec,
                                          struct scan_control *sc) {
    struct list_head *head = &lruvec->lists[LRU_INACTIVE_FILE];
    unsigned long nr_scanned = 0;
    
    spin_lock_irq(&lruvec->lock);
    
    while (nr_scanned < nr_to_scan) {
        struct page *page = list_first_entry(head, struct page, lru);
        
        /* Check if page can be evicted */
        if (page_referenced(page)) {
            /* Still hot, move to active list */
            list_move(&page->lru, &lruvec->lists[LRU_ACTIVE_FILE]);
            continue;
        }
        
        /* Cold page, evict */
        list_del(&page->lru);
        
        if (page_mapped(page) && mapping_exitable(page->mapping)) {
            /* Try to unmap from page tables */
            if (try_to_unmap(page, TTU_UNMAP)) {
                /* Free the page */
                __remove_mapping(page->mapping, page);
                nr_scanned++;
            }
        }
    }
    
    spin_unlock_irq(&lruvec->lock);
    return nr_scanned;
}
```

### CLOCK Replacement Algorithm (Alternative)

The kernel also implements CLOCK algorithm in some places (more efficient than LRU for sequential workloads):

```c
/* CLOCK algorithm: uses a reference bit instead of full ordering */

struct page {
    unsigned int referenced : 1;  // Reference bit (CLOCK hand)
};

void page_lru_clock_scan(struct zone *zone, unsigned int nr_to_scan) {
    /* Global "hand" pointer */
    static struct page *clock_hand = NULL;
    
    int scanned = 0;
    
    while (scanned < nr_to_scan) {
        struct page *page = clock_hand;
        
        if (!page) {
            page = zone->page_list;
            clock_hand = page;
        }
        
        if (page->referenced) {
            /* Give it another chance */
            page->referenced = 0;
        } else {
            /* Evict */
            free_page(page);
        }
        
        clock_hand = page->next;
        scanned++;
    }
}
```

### Why Kernel LRU is Complex

```
kernel/mm/vmscan.c is ~2000 lines because:

1. NUMA awareness: Different LRU lists per node
2. Memory cgroups: Separate LRU per cgroup (control group)
3. Reclaim types: Synchronous vs async, file vs anon
4. Shrinker callbacks: Multiple subsystems register shrink functions
5. Lock contention: Uses per-zone, per-node locks, not global
6. Swap coordination: Decides when to swap pages to disk
7. Page flags: Tracks referenced, dirty, writeback, locked status
8. Interrupt safety: Called from interrupt handlers

Lesson: Production systems need complexity for:
- Scalability (multi-socket, multi-node)
- Fairness (no starvation)
- Observability (metrics, tracing)
- Configurability (tuning knobs)
```

---

## Performance Analysis

### Time Complexity

| Operation | Best | Average | Worst |
|-----------|------|---------|-------|
| GET | O(1) | O(1) | O(n)* |
| PUT (exists) | O(1) | O(1) | O(n)* |
| PUT (new, not full) | O(1) | O(1) | O(n)* |
| PUT (new, full) | O(1) | O(1) | O(n)* |
| DELETE | O(1) | O(1) | O(n)* |

*O(n) occurs only with hash collision chains. With proper hash table sizing and hash function, collision chains are O(1) expected length.

### Space Complexity

```
Base: O(capacity)

Per entry overhead:
- Hash table entry: ~24 bytes (pointer, collision chain)
- List node: ~40 bytes (key, value, prev, next pointers)
- String key: ~variable (UTF-8 bytes + null terminator)
- User value: ~variable

Example (assuming 64-bit pointers, 32-byte keys, 64-byte values):
Cache entry: 24 + 40 + 32 + 64 = 160 bytes
For 10,000 items: ~1.6 MB pure data + overhead
Actual heap usage: ~2-3x due to allocator fragmentation
```

### Cache Line Considerations

```
Modern x86-64: 64-byte cache line

Optimal layout for hot path (GET):
    ┌─────────────────┐
    │ Hash bucket ptr │ 8 bytes
    ├─────────────────┤
    │ Key hash        │ 8 bytes  <- Cache line 0
    ├─────────────────┤
    │ Next in chain   │ 8 bytes
    ├─────────────────┤
    │ Node ptr        │ 8 bytes
    ├─────────────────┤
    │ Value (hot)     │ 32 bytes <- Part of cache line
    └─────────────────┘
    Cache misses in GET operation: ~1 (for hash table probe)

Pessimal layout:
    ┌─────────────────────────────────────┐
    │ Hash table (in L1)                  │
    └─────────────────────────────────────┘
              64-byte jump
    ┌─────────────────────────────────────┐
    │ Node in heap (cold, cache miss)    │
    └─────────────────────────────────────┘

Lesson: Allocator fragmentation causes cache misses.
Solution: Memory pool allocator for nodes (custom alloc)
```

### Benchmark: Uniform vs Skewed Workloads

```
Workload A: Uniform random (all keys equally likely)
- Hit rate: ~90% (capacity = 1000, key space = 10000)
- Evictions: ~100K (for 1M operations)
- LRU optimal

Workload B: Zipf distribution (80/20 rule)
- Hit rate: ~95% (top 100 keys = 80% of requests)
- Evictions: ~50K
- LRU nearly optimal

Workload C: Sequential scan (working set > capacity)
- Hit rate: ~1% (never hits same key twice)
- Evictions: ~1M (one per operation)
- LRU worst-case
- Better: CLOCK, ARC (Adaptive Replacement Cache)
```

---

## Thread Safety & Concurrency

### Single-Lock Approach (Simple, Contention)

```go
// Go example with single RWMutex
type LRUCache struct {
    mu sync.RWMutex
    // ... fields ...
}

func (lru *LRUCache) Get(key string) (interface{}, bool) {
    lru.mu.RLock()  // Multiple readers OK
    defer lru.mu.RUnlock()
    // ... find and return ...
}

func (lru *LRUCache) Put(key string, val interface{}) {
    lru.mu.Lock()   // Exclusive access
    defer lru.mu.Unlock()
    // ... insert/update ...
}

// Problem: High contention under concurrent load
// All writers block readers
```

### Sharded Approach (Scalable)

```go
type ShardedLRUCache struct {
    shards []*LRUCache
    mask   uint32  // num_shards - 1
}

func (sc *ShardedLRUCache) getShard(key string) *LRUCache {
    hash := hashFunction(key)
    return sc.shards[hash & sc.mask]
}

func (sc *ShardedLRUCache) Get(key string) (interface{}, bool) {
    shard := sc.getShard(key)
    return shard.Get(key)
}

func (sc *ShardedLRUCache) Put(key string, val interface{}) {
    shard := sc.getShard(key)
    shard.Put(key, val)
}

// Benefit: N shards = N independent locks
// 16 shards: 16x less contention
// Trade-off: Slightly worse cache locality (data scattered across shards)
```

### Lock-Free Approach (Complex, High Performance)

```go
// Using atomic operations instead of locks
type LockFreeLRUNode struct {
    key      string
    value    interface{}
    next     atomic.Pointer[LockFreeLRUNode]
    prev     atomic.Pointer[LockFreeLRUNode]
}

// Challenges:
// 1. CAS (Compare-And-Swap) loops for reordering
// 2. ABA problem (node freed and reallocated with same address)
// 3. Hazard pointers or epoch-based reclamation needed for safety

// Not recommended unless:
// - Extreme contention measured empirically
// - Lock-free data structure library available
// - Team experienced with lock-free programming
```

### Real-World Contention Analysis

```
Single-lock LRU under load:

CPU 0: holds lock ─────────────────────
CPU 1: waiting ─┐
CPU 2: waiting ─┤
CPU 3: waiting ─┤  Lock holder = 1, waiting = 3 (75% waiting!)
CPU 4: waiting ─┤
...

With 16-shard LRU:

CPU 0: lock shard 5 ──────
CPU 1: lock shard 12 ─────
CPU 2: lock shard 3 ──┐
CPU 3: waiting        │  Much better distribution
CPU 4: lock shard 8 ──┤
...

Rule of thumb: Use sharding if:
- >8 concurrent threads accessing cache
- Cache hit rate <90% (more time in critical section)
- Measured contention >50%
```

---

## Testing & Debugging

### Unit Tests (Comprehensive)

```rust
#[cfg(test)]
mod tests {
    use super::*;

    /// Test 1: Basic insert and retrieve
    #[test]
    fn test_basic_get_put() {
        let cache: LRUCache<String, i32> = LRUCache::new(2);
        
        cache.put("a".to_string(), 1);
        assert_eq!(cache.get(&"a".to_string()), Some(1));
        assert_eq!(cache.size(), 1);
    }

    /// Test 2: Eviction when full
    #[test]
    fn test_eviction_when_full() {
        let cache: LRUCache<String, i32> = LRUCache::new(2);
        
        cache.put("a".to_string(), 1);
        cache.put("b".to_string(), 2);
        assert_eq!(cache.size(), 2);
        
        cache.put("c".to_string(), 3);  // Should evict "a"
        assert_eq!(cache.size(), 2);
        assert_eq!(cache.get(&"a".to_string()), None);
        assert_eq!(cache.get(&"c".to_string()), Some(3));
    }

    /// Test 3: Update existing key doesn't evict
    #[test]
    fn test_update_existing_key() {
        let cache: LRUCache<String, i32> = LRUCache::new(2);
        
        cache.put("a".to_string(), 1);
        cache.put("b".to_string(), 2);
        
        cache.put("a".to_string(), 10);  // Update, not insert
        assert_eq!(cache.size(), 2);
        assert_eq!(cache.get(&"a".to_string()), Some(10));
        assert_eq!(cache.get(&"b".to_string()), Some(2));
    }

    /// Test 4: LRU order is correct
    #[test]
    fn test_lru_order() {
        let cache: LRUCache<String, i32> = LRUCache::new(3);
        
        cache.put("a".to_string(), 1);
        cache.put("b".to_string(), 2);
        cache.put("c".to_string(), 3);
        
        // Access 'a' to make it most recent
        cache.get(&"a".to_string());
        
        // Insert 'd' → should evict 'b' (LRU)
        cache.put("d".to_string(), 4);
        
        assert_eq!(cache.get(&"b".to_string()), None);
        assert_eq!(cache.get(&"a".to_string()), Some(1));
    }

    /// Test 5: Multiple accesses affect order
    #[test]
    fn test_multiple_accesses() {
        let cache: LRUCache<String, i32> = LRUCache::new(3);
        
        cache.put("a".to_string(), 1);
        cache.put("b".to_string(), 2);
        cache.put("c".to_string(), 3);
        
        // Access order: b, a, c, a
        cache.get(&"b".to_string());
        cache.get(&"a".to_string());
        cache.get(&"c".to_string());
        cache.get(&"a".to_string());  // 'a' is now most recent
        
        // LRU is 'b'
        cache.put("d".to_string(), 4);
        assert_eq!(cache.get(&"b".to_string()), None);
    }

    /// Test 6: Edge case - capacity of 1
    #[test]
    fn test_capacity_one() {
        let cache: LRUCache<String, i32> = LRUCache::new(1);
        
        cache.put("a".to_string(), 1);
        assert_eq!(cache.get(&"a".to_string()), Some(1));
        
        cache.put("b".to_string(), 2);
        assert_eq!(cache.get(&"a".to_string()), None);
        assert_eq!(cache.get(&"b".to_string()), Some(2));
    }

    /// Test 7: Metrics accuracy
    #[test]
    fn test_metrics() {
        let cache: LRUCache<String, i32> = LRUCache::new(2);
        
        cache.put("a".to_string(), 1);
        cache.get(&"a".to_string());  // hit
        cache.get(&"b".to_string());  // miss
        
        let (hits, misses, _) = cache.metrics();
        assert_eq!(hits, 1);
        assert_eq!(misses, 1);
    }

    /// Test 8: Concurrent access (stress test)
    #[test]
    fn test_concurrent_access() {
        use std::sync::Arc;
        use std::thread;

        let cache = Arc::new(LRUCache::new(100));
        let mut handles = vec![];

        for i in 0..10 {
            let cache_clone = Arc::clone(&cache);
            let handle = thread::spawn(move || {
                for j in 0..100 {
                    let key = format!("key_{}_{}", i, j % 50);
                    cache_clone.put(key.clone(), j);
                    let _ = cache_clone.get(&key);
                }
            });
            handles.push(handle);
        }

        for handle in handles {
            handle.join().unwrap();
        }

        assert!(cache.size() > 0);
    }
}
```

### Property-Based Testing

```rust
use quickcheck::{Prop, TestResult, quickcheck};

/// Property: Putting N unique items in capacity N cache keeps all items
fn prop_capacity_invariant(values: Vec<i32>) -> TestResult {
    if values.is_empty() {
        return TestResult::discard();
    }

    let cache: LRUCache<i32, i32> = LRUCache::new(values.len());
    
    for &val in &values {
        cache.put(val, val);
    }

    // All items should still be there (no eviction)
    TestResult::from_bool(
        values.iter().all(|&v| cache.get(&v).is_some())
    )
}

/// Property: Eviction only happens when capacity exceeded
fn prop_eviction_condition(ops: Vec<(String, i32)>) -> TestResult {
    let capacity = 10;
    let cache: LRUCache<String, i32> = LRUCache::new(capacity);
    
    let mut unique_keys = std::collections::HashSet::new();
    for (key, val) in &ops {
        unique_keys.insert(key.clone());
        cache.put(key.clone(), *val);
    }

    let size = cache.size();
    TestResult::from_bool(size <= capacity && size == unique_keys.len().min(capacity))
}

#[test]
fn test_properties() {
    quickcheck(prop_capacity_invariant as fn(Vec<i32>) -> TestResult);
    quickcheck(prop_eviction_condition as fn(Vec<(String, i32)>) -> TestResult);
}
```

### Manual Debugging in Linux with gdb

```bash
# Compile with debug symbols
gcc -g -O0 -pthread lru_cache.c test_lru.c -o test_lru

# Run under gdb
gdb ./test_lru

# Set breakpoint on eviction
(gdb) break evict_node
(gdb) run

# When breakpoint hits, inspect state
(gdb) print cache->size
(gdb) print cache->capacity
(gdb) print *cache->head
(gdb) print *cache->tail

# Walk the linked list
(gdb) define show_list
> set $node = $arg0
> while $node != 0
>   printf "Node: key=%s, value=%p\n", $node->key, $node->value
>   set $node = $node->next
> end
> end

(gdb) show_list cache->head

# Set conditional breakpoint
(gdb) break evict_node if cache->size > 100

# View memory layout
(gdb) x/16xg &cache->hash_table[0]  // 16 8-byte values from hash table
```

### Valgrind Memory Check

```bash
# Check for memory leaks
valgrind --leak-check=full --show-leak-kinds=all ./test_lru

# Helgrind: thread safety checker
valgrind --tool=helgrind ./test_lru

# DRD: another thread checker (sometimes better)
valgrind --tool=drd ./test_lru
```

### Perf Profiling

```bash
# Record CPU time spent in each function
perf record -g ./test_lru
perf report

# Trace cache misses
perf stat -e cache-references,cache-misses,cycles,instructions ./test_lru

# Generate flame graph
perf record -F 99 ./test_lru
perf script | stackcollapse-perf.pl | flamegraph.pl > out.svg
```

---

## Production Considerations

### 1. Hash Function Quality

```c
/* Bad hash function: sequential keys collide */
uint32_t bad_hash(const char *key) {
    return (uint32_t)(key[0]);  // Only uses first char!
}

/* Good hash: MurmurHash3 (used by Redis, Java HashMap) */
uint32_t murmurhash3(const char *key, uint32_t seed) {
    uint32_t h = seed;
    const uint8_t *data = (const uint8_t *)key;
    
    while (*data) {
        h ^= *data++;
        h = ((h << 13) | (h >> 19)) * 0x5bd1e995;
    }
    
    h ^= h >> 16;
    return h;
}

/* Lesson: Poor hash → long collision chains → O(n) operations */
```

### 2. Eviction Policy Under Memory Pressure

```go
// Pattern: Batch evictions during backpressure
type LRUCache struct {
    // ... fields ...
    evictionThreshold float64  // 0.9 = evict when 90% full
    batchEvictSize    int      // How many to evict at once
}

func (lru *LRUCache) ensureCapacity() {
    if float64(lru.size) / float64(lru.capacity) > lru.evictionThreshold {
        // Evict batch instead of one at a time
        lru.evictBatch(lru.batchEvictSize)
    }
}

func (lru *LRUCache) evictBatch(count int) {
    for i := 0; i < count && lru.size > 0; i++ {
        tail := lru.list.Back()
        if tail != nil {
            lru.evictNode(tail)
        }
    }
}

// Benefit: Reduces lock contention (fewer critical sections)
// Trade-off: Slightly reduced hit rate (more evictions)
```

### 3. Observability & Metrics

```go
type LRUCacheMetrics struct {
    HitRate      prometheus.Gauge
    EvictionRate prometheus.Gauge
    Size         prometheus.Gauge
    Capacity     prometheus.Gauge
    P50Latency   prometheus.Histogram  // 50th percentile
    P99Latency   prometheus.Histogram  // 99th percentile
}

func (lru *LRUCache) recordGet(duration time.Duration, hit bool) {
    if hit {
        lru.metrics.HitRate.Inc()
    } else {
        lru.metrics.HitRate.Add(-1)  // Hit rate = hits / (hits + misses)
    }
    lru.metrics.P50Latency.Observe(duration.Seconds())
    lru.metrics.P99Latency.Observe(duration.Seconds())
}

// In production, export to Prometheus
// Then alert on:
// - Hit rate < 70% (cache ineffective)
// - P99 latency > 10ms (contention issue)
// - Eviction rate too high (capacity too small)
```

### 4. Configuration Tuning

```
LRU Cache Tuning Guide:

Parameter: Capacity
- Too small: High eviction rate, low hit rate
- Too large: Memory wasted, slow eviction
- Recommendation: Size for 80-90% hit rate

Parameter: Hash table size
- Rule: hash_table_size = capacity * 4 to 8
- Larger = fewer collisions but more memory
- Smaller = faster eviction but chain walking

Parameter: Lock strategy
- Single lock: Simple, good for <4 cores
- Sharding: Use for 8+ cores, high contention
- Lock-free: Only if contention measured >80%

Monitoring:
- Watch hit_rate metric over time
- If declining: Access pattern changed, need resize
- If stable: Cache is tuned well
```

### 5. Persistence & Snapshots

```go
// Serialize cache to disk
func (lru *LRUCache) SaveToFile(filename string) error {
    lru.mu.RLock()
    defer lru.mu.RUnlock()

    f, err := os.Create(filename)
    if err != nil {
        return err
    }
    defer f.Close()

    encoder := json.NewEncoder(f)

    // Walk list from head (most recent)
    var items []map[string]interface{}
    for elem := lru.list.Front(); elem != nil; elem = elem.Next() {
        node := elem.Value.(*Node)
        items = append(items, map[string]interface{}{
            "key":   node.Key,
            "value": node.Value,
        })
    }

    return encoder.Encode(items)
}

// Load from disk
func (lru *LRUCache) LoadFromFile(filename string) error {
    f, err := os.Open(filename)
    if err != nil {
        return err
    }
    defer f.Close()

    decoder := json.NewDecoder(f)
    var items []map[string]interface{}
    if err := decoder.Decode(&items); err != nil {
        return err
    }

    lru.mu.Lock()
    defer lru.mu.Unlock()

    for _, item := range items {
        lru.Put(item["key"].(string), item["value"])
    }

    return nil
}

// Use case: Redis RDB snapshots, cache warmup on restart
```

---

## Summary: Mental Model

```
┌─────────────────────────────────────────────────────────────┐
│              LRU CACHE MENTAL MODEL                         │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Core Insight: O(1) = Hash Table + Doubly-Linked List     │
│                                                             │
│  ┌─────────────────┐              ┌─────────────────┐     │
│  │  Hash Table     │─┐            │  DLL (Ordering) │     │
│  │ (Fast Lookup)   │ │  ┌────────▶│ (Fast Reorder)  │     │
│  └─────────────────┘ │  │         └─────────────────┘     │
│         ▲            │  │                ▲                 │
│         │            └──┴────────────────┘                 │
│         └─────── Must Stay Synchronized ───────────────────│
│                                                             │
│  Operations:                                               │
│  - GET: O(1) lookup + O(1) reorder to head                │
│  - PUT: O(1) lookup + O(1) insert at head                 │
│  - Evict: O(1) remove tail + O(1) delete from hash      │
│                                                             │
│  Key Trade-offs:                                           │
│  1. Memory: 2-3x overhead (pointers, hash table)          │
│  2. Cache lines: Pointer chasing causes misses            │
│  3. Concurrency: Locks needed for consistency             │
│                                                             │
│  Best For:                                                 │
│  ✓ Temporal locality (recent = likely again)             │
│  ✓ Bounded working set                                    │
│  ✗ Sequential scan (full-table walks)                     │
│  ✗ Heavy write-only (no temporal locality)               │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## Further Reading

- **Linux Kernel VFS**: `linux/fs/dcache.c`, `mm/vmscan.c`
- **Redis Implementation**: https://github.com/redis/redis (dictType, ziplist)
- **Memcached**: https://github.com/memcached/memcached
- **Paper: ARC Cache** (Adaptive Replacement): Megiddo & Modha, 2003
- **Paper: LRU Under Contention**: Dice et al., 2015
- **Book**: "The Linux Programming Interface" by Michael Kerrisk (Chapter 50)

