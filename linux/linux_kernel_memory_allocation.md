# Linux Kernel Memory Allocation: A Complete Deep-Dive Guide

> **Goal:** Build a world-class mental model of how the Linux kernel manages, allocates, and
> frees memory — from raw hardware pages up through high-level allocator APIs.
> Every concept is defined from first principles before it is used.

---

## Table of Contents

1. [Foundational Concepts](#1-foundational-concepts)
   - 1.1 Physical vs Virtual Memory
   - 1.2 Pages — The Atom of Memory
   - 1.3 The Memory Map: `struct page`
   - 1.4 Zones — Classifying Physical Memory
   - 1.5 Virtual Address Space Layout (x86-64)
2. [The Buddy System — Page Allocator](#2-the-buddy-system--page-allocator)
   - 2.1 What Problem Does It Solve?
   - 2.2 How Buddy Works — Step by Step
   - 2.3 Fragmentation and the Buddy's Weakness
   - 2.4 Page-Level API in C
3. [The Slab Allocator — Object Cache](#3-the-slab-allocator--object-cache)
   - 3.1 The Fragmentation Problem Slab Solves
   - 3.2 Slab Architecture
   - 3.3 SLUB — The Modern Slab
   - 3.4 Slab API in C
4. [General-Purpose Allocators](#4-general-purpose-allocators)
   - 4.1 `kmalloc` Family
   - 4.2 `kzalloc` and `kcalloc`
   - 4.3 `kfree`
   - 4.4 Size Classes and kmalloc Internals
5. [Large-Block & Virtual Allocators](#5-large-block--virtual-allocators)
   - 5.1 `vmalloc` — Virtually Contiguous
   - 5.2 `kvmalloc` — The Smart Hybrid
   - 5.3 `vfree` and `kvfree`
6. [GFP Flags — Allocation Behavior Control](#6-gfp-flags--allocation-behavior-control)
   - 6.1 What GFP Means
   - 6.2 Flag Deep Dive
   - 6.3 Choosing the Right Flag
7. [Memory Reclaim and Pressure](#7-memory-reclaim-and-pressure)
8. [Per-CPU Allocators](#8-per-cpu-allocators)
9. [DMA and Coherent Memory](#9-dma-and-coherent-memory)
10. [Complete Architecture Diagram](#10-complete-architecture-diagram)
11. [C Implementation Examples](#11-c-implementation-examples)
12. [Rust in the Linux Kernel](#12-rust-in-the-linux-kernel)
13. [Mental Models and Expert Intuition](#13-mental-models-and-expert-intuition)
14. [API Quick Reference Table](#14-api-quick-reference-table)

---

## 1. Foundational Concepts

Before touching a single API, you must own the mental model of how memory is structured
in the kernel. Every allocator decision flows from this foundation.

---

### 1.1 Physical vs Virtual Memory

**Physical Memory** is the actual RAM chips soldered onto your motherboard.
The CPU accesses these chips through a memory bus. Each byte has a unique
**Physical Address (PA)** — a raw number representing its location in silicon.

**Virtual Memory** is an abstraction the OS creates. Every running process — including
the kernel itself — sees its own **Virtual Address Space (VAS)**: a flat array of
addresses that do *not* correspond directly to physical addresses.

The hardware component that translates virtual → physical addresses is called the
**MMU (Memory Management Unit)**, using data structures called **Page Tables**.

```
  Process A sees:            Process B sees:           Physical RAM:
  ┌─────────────────┐        ┌─────────────────┐       ┌─────────────────┐
  │ VA: 0x0000_1000 │        │ VA: 0x0000_1000 │       │ PA: 0x0010_0000 │ ← A's page
  │ VA: 0x0000_2000 │        │ VA: 0x0000_2000 │       │ PA: 0x0020_0000 │ ← B's page
  └─────────────────┘        └─────────────────┘       │ PA: 0x0030_0000 │ ← kernel
                                                        └─────────────────┘
          │                          │
          └──────── MMU (Page Tables) translates VA → PA ──────────┘
```

**Why this matters for kernel allocators:**
- `kmalloc()` returns a **virtual address** in kernel space.
- The memory it points to is **physically contiguous** — critical for DMA.
- `vmalloc()` also returns a virtual address but the pages may be **physically scattered**.

---

### 1.2 Pages — The Atom of Memory

The kernel does **not** manage individual bytes when dealing with physical memory.
It manages **pages**.

> **Page:** The smallest unit of physical memory the OS tracks. On x86-64 Linux,
> the default page size is **4 KiB (4096 bytes)**. This is defined as `PAGE_SIZE`.

Why pages? Because the MMU's hardware translation works in page-sized granules.
A page table entry (PTE) maps one virtual page to one physical page frame.

```
Physical RAM organized as page frames:

  ┌──────────┬──────────┬──────────┬──────────┬──────────┐
  │ Frame 0  │ Frame 1  │ Frame 2  │ Frame 3  │ Frame 4  │
  │  4 KiB   │  4 KiB   │  4 KiB   │  4 KiB   │  4 KiB   │
  └──────────┴──────────┴──────────┴──────────┴──────────┘
  PA:0x0000  PA:0x1000  PA:0x2000  PA:0x3000  PA:0x4000
```

The kernel tracks every physical page frame with a **`struct page`** (see 1.3).

**Page Order:** When allocating multiple pages, the kernel uses the concept of **order**.
Order `n` means `2^n` contiguous pages.

```
Order 0  =  2^0 = 1 page  =   4 KiB
Order 1  =  2^1 = 2 pages =   8 KiB
Order 2  =  2^2 = 4 pages =  16 KiB
Order 3  =  2^3 = 8 pages =  32 KiB
...
Order 11 =  2^11 = 2048 pages = 8 MiB   (MAX_ORDER on most systems)
```

---

### 1.3 The Memory Map: `struct page`

Every physical page frame in the system has a corresponding `struct page` in kernel
memory. This array is called the **mem_map** or **memory map**.

```c
// Simplified view of struct page (linux/mm_types.h)
// Actual struct uses unions heavily to save space
struct page {
    unsigned long flags;          // Page state: dirty, locked, uptodate, etc.
    atomic_t      _refcount;      // How many entities reference this page
    atomic_t      _mapcount;      // How many page table entries point here
    struct list_head lru;         // LRU list linkage (for page reclaim)
    struct address_space *mapping;// If file-backed, points to inode's mapping
    pgoff_t       index;          // Offset within the mapping (file position)
    void          *virtual;       // Kernel virtual address (if mapped)
    // ... many more fields via unions for different use cases
};
```

**Key insight:** On a machine with 4 GiB of RAM, there are `4 GiB / 4 KiB = 1,048,576`
page frames, so the mem_map contains ~1 million `struct page` entries.
Each `struct page` is roughly 64 bytes → the mem_map itself uses ~64 MiB of RAM.

**PFN (Page Frame Number):** A simple integer index into the mem_map array.
`PFN = physical_address >> PAGE_SHIFT` where `PAGE_SHIFT = 12` (log2 of 4096).

Conversion macros:
```c
// PA → struct page
struct page *page = pfn_to_page(pa >> PAGE_SHIFT);

// struct page → virtual address
void *vaddr = page_address(page);

// virtual address → struct page
struct page *page = virt_to_page(vaddr);

// struct page → physical address
phys_addr_t pa = page_to_phys(page);
```

---

### 1.4 Zones — Classifying Physical Memory

Not all physical RAM is equal. Legacy hardware, DMA constraints, and address space
limits mean the kernel partitions RAM into **zones**:

```
Physical Address Space (x86-64, simplified):

  0 MiB ──────────────────────────────────────────────────
  │  ZONE_DMA                                             │
  │  0 – 16 MiB                                          │
  │  For old ISA devices that can only address 24-bit PA  │
  16 MiB ─────────────────────────────────────────────────
  │  ZONE_DMA32                                           │
  │  16 MiB – 4 GiB                                      │
  │  For 32-bit DMA devices on 64-bit systems             │
  4 GiB ──────────────────────────────────────────────────
  │  ZONE_NORMAL                                          │
  │  4 GiB – end of RAM                                  │
  │  General-purpose kernel memory (modern systems use    │
  │  this zone almost exclusively)                        │
  End ─────────────────────────────────────────────────────

  ZONE_HIGHMEM: only on 32-bit x86 (no 64-bit equivalent)
  ZONE_MOVABLE: pages that can be migrated (for memory hotplug)
```

Each zone maintains its **own free page lists** (the buddy system, see §2).
When you pass `GFP_DMA` to an allocator, you're telling it "allocate from ZONE_DMA".

---

### 1.5 Virtual Address Space Layout (x86-64)

On x86-64 Linux, the 64-bit virtual address space is split between user and kernel:

```
Virtual Address Space (x86-64, 48-bit canonical addresses):

  0xFFFF_FFFF_FFFF_FFFF ──────────────────────────────────
  │                                                       │
  │  Kernel Virtual Address Space (128 TiB)              │
  │                                                       │
  │  0xFFFF_8000_0000_0000 ─────────────────────────────  │
  │  │ Direct Map (physmem linearly mapped here)        │  │
  │  │ Every physical page has a VA here via +OFFSET    │  │
  │  │ kmalloc() returns addresses from this region     │  │
  │  0xFFFF_C000_0000_0000 ─────────────────────────────  │
  │  │ vmalloc region (virtually contiguous mappings)   │  │
  │  │ vmalloc() returns addresses from here            │  │
  │  0xFFFF_E000_0000_0000 ─────────────────────────────  │
  │  │ Kernel image, modules                            │  │
  │                                                       │
  0xFFFF_7FFF_FFFF_FFFF ──────────────────────────────────
  (non-canonical hole — accessing here = hardware fault)
  0x0000_8000_0000_0000 ──────────────────────────────────
  │                                                       │
  │  User Virtual Address Space (128 TiB)                │
  │  0x0 → stack, heap, mmap, code, data segments       │
  │                                                       │
  0x0000_0000_0000_0000 ──────────────────────────────────
```

**Critical insight:** The kernel's "direct map" means every physical page is
permanently mapped in kernel VA space with a fixed offset. This is why `kmalloc`
is so fast — there's no need to set up new page table entries. The mapping already
exists from boot.

---

## 2. The Buddy System — Page Allocator

The Buddy System is the **lowest-level physical memory allocator** in the Linux kernel.
All higher-level allocators (Slab, kmalloc, vmalloc) ultimately call the buddy system
to get pages.

---

### 2.1 What Problem Does It Solve?

The core challenges of physical page allocation:

1. **Allocation:** Given a request for `2^n` contiguous pages, find them quickly.
2. **Freeing:** Return pages and merge adjacent free blocks to reduce fragmentation.
3. **External Fragmentation:** The phenomenon where you have enough total free pages
   but no single contiguous block large enough to satisfy a request.

The Buddy System elegantly solves all three with a single data structure.

---

### 2.2 How Buddy Works — Step by Step

The system maintains **11 free lists** (orders 0 through 10), each containing
blocks of `2^order` contiguous pages.

```
Buddy System Free Lists (per zone):

  Order 0 (1 page):    [page_A] → [page_C] → [page_F] → NULL
  Order 1 (2 pages):   [page_B,B+1] → NULL
  Order 2 (4 pages):   [page_D,D+1,D+2,D+3] → NULL
  Order 3 (8 pages):   NULL
  ...
  Order 10 (1024 pg):  NULL
```

#### Allocation: Splitting

Request: allocate 1 page (Order 0).

```
Step 1: Check Order 0 list → empty.
Step 2: Check Order 1 list → has a block of 2 pages.
Step 3: SPLIT the Order-1 block into two Order-0 blocks.
        → Give one to the caller.
        → Place the other ("the buddy") on the Order-0 free list.

Before:                          After:
  Order 0: []                    Order 0: [page_B+1]   ← buddy goes here
  Order 1: [page_B, page_B+1]   Order 1: []
                                 Caller gets: page_B
```

If Order 1 was also empty, the kernel checks Order 2, splits it into two Order-1
blocks, places one on Order-1 free list, then proceeds to split that one, and so on.

#### Freeing: Coalescing (Merging)

When a page (or block) is freed, the kernel checks if its **buddy** is also free.
Two blocks are buddies if they differ only in one bit of their PFN.

```
Buddy Formula: buddy_pfn = pfn XOR (1 << order)

Example: freeing a page at PFN 4 (order 0):
  buddy_pfn = 4 XOR (1 << 0) = 4 XOR 1 = 5

  Check: is PFN 5 in the Order-0 free list? 
  YES → Merge them into an Order-1 block at PFN 4.
  Now check Order-1 buddy: 4 XOR (1 << 1) = 4 XOR 2 = 6
  Is PFN 6 free at Order 1? YES → Merge into Order-2 block at PFN 4.
  Continue until no more merging is possible.
```

**Detailed visual walkthrough:**

```
Initial State (8 pages, all free, merged to Order-3):
  Order 3: [PFN 0..7]
  Order 0: [], Order 1: [], Order 2: []

Allocate 1 page (Order 0):
  Split Order-3 → two Order-2 blocks
  Order 3: []
  Order 2: [PFN 0..3], [PFN 4..7]
  Split first Order-2 → two Order-1 blocks
  Order 2: [PFN 4..7]
  Order 1: [PFN 0..1], [PFN 2..3]
  Split first Order-1 → two Order-0 blocks
  Order 1: [PFN 2..3], [PFN 4..7 at Order 2]
  Order 0: [PFN 1]
  → Return PFN 0 to caller

Allocate another 1 page:
  Order 0 has PFN 1 → Return PFN 1 directly
  Order 0: []

Free PFN 0:
  Buddy of PFN 0 at Order 0 = PFN 0 XOR 1 = PFN 1
  Is PFN 1 free? YES → Merge PFN 0+1 into Order-1 block
  Order 0: []
  Order 1: [PFN 0..1], [PFN 2..3]
  Buddy of PFN 0 at Order 1 = PFN 0 XOR 2 = PFN 2
  Is [PFN 2..3] free at Order 1? YES → Merge into Order-2
  Order 1: []
  Order 2: [PFN 0..3], [PFN 4..7]
  Buddy of PFN 0 at Order 2 = PFN 0 XOR 4 = PFN 4
  Is [PFN 4..7] free at Order 2? YES → Merge into Order-3
  Order 3: [PFN 0..7]   ← fully coalesced!
```

---

### 2.3 Fragmentation and the Buddy's Weakness

The buddy system handles **external fragmentation** well (free blocks merge back).
But it suffers from **internal fragmentation**: if you need 5 pages, you get 8
(rounded up to next power of 2), wasting 3 pages.

Modern Linux adds **anti-fragmentation heuristics**:

```
Page Migration Types (MIGRATE_TYPES):

  MIGRATE_UNMOVABLE   ← kernel data, page tables (cannot be moved)
  MIGRATE_MOVABLE     ← user process pages (can be migrated, good for defrag)
  MIGRATE_RECLAIMABLE ← file cache (can be discarded and reloaded)

Each free list entry is tagged with migration type.
This prevents movable and unmovable pages from interleaving,
reducing fragmentation over time.
```

**Compaction:** The kernel can run a compaction daemon that migrates movable pages
to consolidate free memory into larger contiguous blocks.

---

### 2.4 Page-Level API in C

```c
#include <linux/gfp.h>      // GFP flags
#include <linux/mm.h>        // struct page, page_address()

/* ─────────────────────────────────────────────────────────────
 * alloc_pages(gfp_mask, order)
 *
 * Allocates 2^order contiguous pages from the buddy system.
 * Returns a pointer to struct page of the FIRST page in the block,
 * or NULL on failure.
 *
 * gfp_mask: Controls allocation behavior (see §6).
 * order:    Power-of-2 exponent. 0 = 1 page, 1 = 2 pages, etc.
 * ──────────────────────────────────────────────────────────── */
struct page *alloc_pages(gfp_t gfp_mask, unsigned int order);

/* ─────────────────────────────────────────────────────────────
 * __get_free_pages(gfp_mask, order)
 *
 * Like alloc_pages() but returns the VIRTUAL ADDRESS (unsigned long)
 * of the first page instead of struct page *.
 * This virtual address is in the kernel's direct map region.
 * Returns 0 (not NULL) on failure.
 * ──────────────────────────────────────────────────────────── */
unsigned long __get_free_pages(gfp_t gfp_mask, unsigned int order);

/* Convenience wrappers for common cases: */
#define alloc_page(gfp_mask)          alloc_pages(gfp_mask, 0)
#define __get_free_page(gfp_mask)     __get_free_pages(gfp_mask, 0)
#define get_zeroed_page(gfp_mask)     __get_free_pages((gfp_mask)|__GFP_ZERO, 0)

/* ─────────────────────────────────────────────────────────────
 * free_pages(addr, order)
 *
 * Returns pages to the buddy system.
 * addr: the virtual address returned by __get_free_pages().
 * order: MUST match the order used during allocation.
 *
 * free_page(addr) is the order-0 convenience macro.
 * ──────────────────────────────────────────────────────────── */
void free_pages(unsigned long addr, unsigned int order);

/* For struct page * returned by alloc_pages(): */
void __free_pages(struct page *page, unsigned int order);
```

**Example — Kernel Driver Using Page Allocator:**

```c
// ============================================================
// example_page_alloc.c
// Demonstrates raw page allocation in a kernel module.
// ============================================================
#include <linux/module.h>
#include <linux/mm.h>
#include <linux/gfp.h>
#include <linux/string.h>

static int __init page_alloc_example_init(void)
{
    struct page *page;
    unsigned long vaddr;
    void         *mapped;

    // ── Allocate 1 page (4 KiB) using struct page * interface ─
    page = alloc_page(GFP_KERNEL);
    if (!page) {
        pr_err("alloc_page failed\n");
        return -ENOMEM;
    }

    // Convert struct page → kernel virtual address
    mapped = page_address(page);
    pr_info("Allocated page at phys=0x%llx virt=%p\n",
            (u64)page_to_phys(page), mapped);

    // Use the page
    memset(mapped, 0xAB, PAGE_SIZE);
    pr_info("First byte: 0x%02x\n", *(unsigned char *)mapped);

    // Free via struct page *
    __free_page(page);

    // ── Allocate 8 pages (32 KiB, order 3) using virtual address ─
    vaddr = __get_free_pages(GFP_KERNEL | __GFP_ZERO, 3);  // order 3 = 8 pages
    if (!vaddr) {
        pr_err("__get_free_pages(order=3) failed\n");
        return -ENOMEM;
    }

    pr_info("Got %lu KiB at vaddr=0x%lx\n", (1UL << 3) * PAGE_SIZE / 1024, vaddr);

    // Free — order MUST match
    free_pages(vaddr, 3);

    return 0;
}

static void __exit page_alloc_example_exit(void)
{
    pr_info("Module unloaded\n");
}

module_init(page_alloc_example_init);
module_exit(page_alloc_example_exit);
MODULE_LICENSE("GPL");
```

---

## 3. The Slab Allocator — Object Cache

### 3.1 The Fragmentation Problem Slab Solves

Imagine a device driver that creates and destroys `struct my_device` thousands of
times per second. Using the buddy system directly would:

1. Allocate a full page (4 KiB) for a struct that is 128 bytes → 97% waste.
2. Every allocation/free cycle would initialize and tear down the struct from scratch
   (zero fields, initialize spinlocks, etc.) → CPU waste.
3. Each freed struct might leave tiny unusable gaps → **internal fragmentation**.

The **Slab Allocator** solves all three:
- Packs many objects of the same type into a single page.
- Keeps freed objects **warm** (pre-initialized, ready to reuse).
- Each cache is dedicated to one object type → zero fragmentation within a cache.

---

### 3.2 Slab Architecture

```
kmem_cache (the "cache descriptor"):
┌────────────────────────────────────────────────────────────────┐
│ name: "my_device"                                              │
│ object_size: 128 bytes                                         │
│ align: 8 bytes                                                 │
│ ctor: my_device_init()  ← called when slab is first created   │
│                                                                │
│  ┌─────────────────────────────────────────────────────────┐  │
│  │  Slab 1 (one or more pages)                             │  │
│  │  ┌──────┬──────┬──────┬──────┬──────┬──────┬──────┐   │  │
│  │  │ obj0 │ obj1 │ obj2 │ obj3 │ obj4 │ obj5 │ obj6 │   │  │
│  │  │ FREE │ USED │ FREE │ USED │ FREE │ FREE │ USED │   │  │
│  │  └──────┴──────┴──────┴──────┴──────┴──────┴──────┘   │  │
│  │  freelist: → obj0 → obj2 → obj4 → obj5 → NULL           │  │
│  └─────────────────────────────────────────────────────────┘  │
│                                                                │
│  ┌─────────────────────────────────────────────────────────┐  │
│  │  Slab 2 (completely full — no free objects)             │  │
│  │  ┌──────┬──────┬──────┬──────┬──────┬──────┬──────┐   │  │
│  │  │ USED │ USED │ USED │ USED │ USED │ USED │ USED │   │  │
│  │  └──────┴──────┴──────┴──────┴──────┴──────┴──────┘   │  │
│  └─────────────────────────────────────────────────────────┘  │
│                                                                │
│  ┌─────────────────────────────────────────────────────────┐  │
│  │  Slab 3 (completely free — candidate for return to buddy)│  │
│  └─────────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────────┘
```

**Three slab states:**
- **Partial:** Has both free and used objects → allocations come from here first.
- **Full:** All objects in use → no allocations possible from here.
- **Empty:** All objects free → candidate to be returned to the buddy system.

**Object Layout within a Slab:**

```
One slab page (4 KiB = 4096 bytes):

┌────────────┬────────────┬────────────┬────────────┬──────────┐
│  slab_hdr  │   obj[0]   │   obj[1]   │   obj[2]   │   ...    │
│  (metadata)│  128 bytes │  128 bytes │  128 bytes │          │
└────────────┴────────────┴────────────┴────────────┴──────────┘
   ↑ contains: freelist ptr, used count, next slab ptr

Coloring: objects are offset slightly to distribute cache line usage.
```

---

### 3.3 SLUB — The Modern Slab

Linux originally had three slab implementations: SLAB, SLUB, SLOB.
Since Linux 2.6.23, **SLUB** is the default. Key differences:

```
SLAB  → Classic implementation, complex, per-cache metadata.
SLUB  → Simpler, fewer queues, stores metadata IN the freelist itself.
SLOB  → Minimal/tiny system allocator (removed in Linux 6.4).

SLUB optimizations:
  1. Per-CPU partial slabs → lock-free fast path for most allocations.
  2. Freelist poison → detects use-after-free bugs (when DEBUG enabled).
  3. Red zones → detects buffer overflows (when DEBUG enabled).
  4. No per-object metadata → more objects fit per slab.
```

**SLUB Per-CPU Cache (fast path):**

```
CPU 0:                           CPU 1:
┌────────────────────────┐       ┌────────────────────────┐
│ kmem_cache_cpu         │       │ kmem_cache_cpu         │
│  freelist → obj → obj  │       │  freelist → obj → obj  │
│  page    → current slab│       │  page    → current slab│
└──────────┬─────────────┘       └────────────────────────┘
           │ miss (freelist empty)
           ▼
┌──────────────────────────┐
│ Per-CPU partial list      │
│ (slabs with free objects) │
└──────────┬────────────────┘
           │ empty
           ▼
┌──────────────────────────┐
│ Node partial list (NUMA)  │
└──────────┬────────────────┘
           │ empty
           ▼
┌──────────────────────────┐
│ Buddy system (new slab)   │
└──────────────────────────┘
```

---

### 3.4 Slab API in C

```c
#include <linux/slab.h>        // kmem_cache_create, kmem_cache_alloc, etc.

/* ─────────────────────────────────────────────────────────────
 * kmem_cache_create(name, size, align, flags, ctor)
 *
 * Creates a new object cache.
 *
 * name:  String identifier (shown in /proc/slabinfo).
 * size:  Size of each object in bytes.
 * align: Required alignment (0 = use default alignment).
 * flags: Cache behavior flags (SLAB_HWCACHE_ALIGN, SLAB_POISON, etc.).
 * ctor:  Optional constructor, called once when a new slab is created.
 *        Called with each NEW object (not on every alloc/free).
 *
 * Returns NULL on failure.
 * ──────────────────────────────────────────────────────────── */
struct kmem_cache *kmem_cache_create(const char *name, unsigned int size,
                                     unsigned int align, slab_flags_t flags,
                                     void (*ctor)(void *));

/* ─────────────────────────────────────────────────────────────
 * kmem_cache_alloc(cache, flags)
 *
 * Allocate one object from the cache.
 * Returns a pointer to the object, or NULL on failure.
 * The object is NOT zero-initialized (reuses prior state).
 * ──────────────────────────────────────────────────────────── */
void *kmem_cache_alloc(struct kmem_cache *cache, gfp_t flags);

/* ─────────────────────────────────────────────────────────────
 * kmem_cache_zalloc(cache, flags)  [helper macro]
 *
 * Like kmem_cache_alloc() but zeroes the object before returning.
 * Prefer this when the prior state of a recycled object is unsafe.
 * ──────────────────────────────────────────────────────────── */
static inline void *kmem_cache_zalloc(struct kmem_cache *cache, gfp_t flags)
{
    return kmem_cache_alloc(cache, flags | __GFP_ZERO);
}

/* ─────────────────────────────────────────────────────────────
 * kmem_cache_free(cache, ptr)
 *
 * Returns an object to its cache.
 * ptr MUST have been allocated from cache — mixing caches is a bug.
 * The object is placed on the freelist (kept "warm").
 * ──────────────────────────────────────────────────────────── */
void kmem_cache_free(struct kmem_cache *cache, void *ptr);

/* ─────────────────────────────────────────────────────────────
 * kmem_cache_destroy(cache)
 *
 * Destroys the entire cache, returning all slabs to the buddy system.
 * Must be called when the cache is no longer needed (module unload).
 * WARNING: All objects MUST be freed before calling this.
 * ──────────────────────────────────────────────────────────── */
void kmem_cache_destroy(struct kmem_cache *cache);
```

**Important Slab Flags:**

```c
SLAB_HWCACHE_ALIGN  // Align objects to hardware cache line size (performance)
SLAB_POISON         // Fill freed objects with poison pattern (debug: detect UAF)
SLAB_RED_ZONE       // Add red zones around objects (debug: detect overflows)
SLAB_ACCOUNT        // Account memory to memcg (memory control groups)
SLAB_PANIC          // Panic if cache creation fails (for critical caches)
SLAB_TYPESAFE_BY_RCU // Safe to dereference object under RCU read lock
```

**Complete Slab Example — Network Packet Descriptor Cache:**

```c
// ============================================================
// slab_example.c
// Models how the kernel manages a high-frequency object type.
// ============================================================
#include <linux/module.h>
#include <linux/slab.h>
#include <linux/init.h>

// Our object type (e.g., a packet descriptor)
struct packet_desc {
    u32  src_ip;
    u32  dst_ip;
    u16  src_port;
    u16  dst_port;
    u8   protocol;
    u8   flags;
    u16  length;
    u32  checksum;
    u8   data[64];   // inline payload
};

// The global cache (one per object type, typically a module global)
static struct kmem_cache *packet_cache;

// Constructor: called once per object when a new slab is allocated.
// Good place to initialize fields that never change between uses
// (e.g., a spinlock, a list_head, a fixed protocol version).
static void packet_desc_ctor(void *obj)
{
    struct packet_desc *p = (struct packet_desc *)obj;
    // Initialize invariant fields only
    p->protocol = 0xFF;  // "unset" sentinel
    p->flags    = 0;
}

static int __init slab_example_init(void)
{
    struct packet_desc *pkt1, *pkt2, *pkt3;

    // Create the cache
    packet_cache = kmem_cache_create(
        "packet_desc",              // name in /proc/slabinfo
        sizeof(struct packet_desc), // object size
        0,                          // alignment: use SLUB default
        SLAB_HWCACHE_ALIGN |        // align to CPU cache line (usually 64B)
        SLAB_POISON,                // poison freed objects (for debug)
        packet_desc_ctor            // constructor
    );

    if (!packet_cache) {
        pr_err("Failed to create packet_cache\n");
        return -ENOMEM;
    }

    pr_info("Cache created. Object size: %u, Slab size: %u\n",
            kmem_cache_size(packet_cache),
            (unsigned int)kmem_cache_size(packet_cache));

    // Allocate objects — FAST PATH (per-CPU freelist, no locking)
    pkt1 = kmem_cache_alloc(packet_cache, GFP_KERNEL);
    pkt2 = kmem_cache_alloc(packet_cache, GFP_KERNEL);
    pkt3 = kmem_cache_alloc(packet_cache, GFP_KERNEL);

    if (!pkt1 || !pkt2 || !pkt3) {
        pr_err("Allocation failed\n");
        goto cleanup;
    }

    // Use objects
    pkt1->src_ip  = 0xC0A80001;  // 192.168.0.1
    pkt1->dst_ip  = 0xC0A80002;  // 192.168.0.2
    pkt1->src_port = 1234;
    pkt1->dst_port = 80;
    pkt1->length   = 74;

    pr_info("pkt1 at %p: src=%pI4h dst=%pI4h\n",
            pkt1, &pkt1->src_ip, &pkt1->dst_ip);

    // Free objects back to cache (NOT to buddy system yet!)
    kmem_cache_free(packet_cache, pkt1);
    kmem_cache_free(packet_cache, pkt2);

cleanup:
    if (pkt3) kmem_cache_free(packet_cache, pkt3);

    // Destroy cache — returns slabs to buddy system
    // WARNING: Only call after ALL objects are freed
    kmem_cache_destroy(packet_cache);

    return 0;
}

static void __exit slab_example_exit(void) { }

module_init(slab_example_init);
module_exit(slab_example_exit);
MODULE_LICENSE("GPL");
```

---

## 4. General-Purpose Allocators

### 4.1 `kmalloc` Family

`kmalloc()` is the kernel's equivalent of userspace `malloc()`. It is built on
top of the Slab allocator using a set of **pre-created size-class caches**.

```c
#include <linux/slab.h>

/* ─────────────────────────────────────────────────────────────
 * kmalloc(size, flags)
 *
 * Allocates 'size' bytes of physically contiguous kernel memory.
 *
 * GUARANTEES:
 *   - The returned memory is physically contiguous (can be used for DMA).
 *   - The virtual address is in the kernel direct map region.
 *   - Returns NULL on failure (unless __GFP_NOFAIL is set).
 *   - Content is NOT zeroed.
 *
 * LIMITS:
 *   - Efficient for sizes up to ~128 KiB (order 5).
 *   - For larger sizes, use vmalloc() or alloc_pages().
 *   - Actual allocation is rounded up to the next size class.
 *
 * Returns: kernel virtual address, or NULL on failure.
 * ──────────────────────────────────────────────────────────── */
void *kmalloc(size_t size, gfp_t flags);

/* Array-safe variant — checks for integer overflow in n*size */
void *kmalloc_array(size_t n, size_t size, gfp_t flags);
```

**How kmalloc routes to a slab cache:**

```
Request: kmalloc(200, GFP_KERNEL)

Size classes (power-of-2 caches, exact names from /proc/slabinfo):
  kmalloc-8       kmalloc-16      kmalloc-32      kmalloc-64
  kmalloc-128     kmalloc-192     kmalloc-256     kmalloc-512
  kmalloc-1024    kmalloc-2048    kmalloc-4096    kmalloc-8192

Step 1: Find the smallest size class >= 200 bytes → kmalloc-256
Step 2: Call kmem_cache_alloc(kmalloc-256 cache, GFP_KERNEL)
Step 3: Return pointer to 256-byte slab object

Internal fragmentation: 256 - 200 = 56 bytes wasted.
External fragmentation: zero (slab handles it).
```

Note: `kmalloc-192` exists to avoid the large waste between kmalloc-128 and kmalloc-256
for objects in the 128-192 byte range.

---

### 4.2 `kzalloc` and `kcalloc`

```c
/* ─────────────────────────────────────────────────────────────
 * kzalloc(size, flags)
 *
 * Identical to kmalloc() but ZEROES the allocated memory before
 * returning. Use this when uninitialized memory is a security risk
 * or when you need zero-initialization for correctness.
 *
 * Implemented as: kmalloc(size, flags | __GFP_ZERO)
 * ──────────────────────────────────────────────────────────── */
static inline void *kzalloc(size_t size, gfp_t flags)
{
    return kmalloc(size, flags | __GFP_ZERO);
}

/* ─────────────────────────────────────────────────────────────
 * kcalloc(n, size, flags)
 *
 * Allocates an array of 'n' elements, each 'size' bytes, zeroed.
 * CRITICAL ADVANTAGE over kzalloc(n*size, flags):
 *   kcalloc checks for integer overflow in (n * size).
 *   If n*size overflows size_t, returns NULL instead of
 *   allocating a dangerously small buffer.
 * ──────────────────────────────────────────────────────────── */
static inline void *kcalloc(size_t n, size_t size, gfp_t flags)
{
    return kmalloc_array(n, size, flags | __GFP_ZERO);
}
```

**Security rule:** Prefer `kzalloc` over `kmalloc` for buffers that will be
copied to userspace. Uninitialized kernel memory exposed to userspace is a
classic information-disclosure vulnerability (kernel data leak).

---

### 4.3 `kfree`

```c
/* ─────────────────────────────────────────────────────────────
 * kfree(ptr)
 *
 * Frees memory allocated by kmalloc(), kzalloc(), or kcalloc().
 * Passing NULL is safe — kfree(NULL) is a no-op.
 *
 * NEVER:
 *   - Double-free (call kfree twice on same pointer).
 *   - Free a vmalloc() pointer with kfree() (use vfree()).
 *   - Free a slab-cache pointer with kfree() (use kmem_cache_free()).
 *   - Access memory after kfree() (use-after-free).
 *
 * GOOD PRACTICE after freeing:
 *   kfree(ptr);
 *   ptr = NULL;  // Prevents accidental use-after-free
 * ──────────────────────────────────────────────────────────── */
void kfree(const void *ptr);

/* kfree_sensitive: zeroes memory before freeing (for crypto keys, passwords) */
void kfree_sensitive(const void *ptr);
```

---

### 4.4 Size Classes and kmalloc Internals

```
kmalloc internal routing (simplified):

kmalloc(size, gfp)
    │
    ├── size == 0 → return ZERO_SIZE_PTR (not NULL, but not dereferenceable)
    │
    ├── size > KMALLOC_MAX_CACHE_SIZE (128 KiB by default)
    │   └── Falls through to alloc_pages() directly (order calculated from size)
    │
    └── size <= KMALLOC_MAX_CACHE_SIZE
        ├── index = kmalloc_index(size)  ← finds the right size class
        └── return kmem_cache_alloc(kmalloc_caches[type][index], gfp)

KMALLOC_MAX_CACHE_SIZE is typically:
  32-bit systems: 128 KiB
  64-bit systems: 8 MiB (but allocations this large are rare)
```

---

## 5. Large-Block & Virtual Allocators

### 5.1 `vmalloc` — Virtually Contiguous

`vmalloc()` solves a specific problem: you need a large buffer (say, 8 MiB), but
the system has been running for hours and its physical memory is fragmented. The
buddy system cannot find 2048 contiguous pages, but it has plenty of 1-page or
4-page blocks scattered around.

```c
#include <linux/vmalloc.h>

/* ─────────────────────────────────────────────────────────────
 * vmalloc(size)
 *
 * Allocates 'size' bytes of memory contiguous in VIRTUAL address
 * space but potentially non-contiguous in PHYSICAL address space.
 *
 * How it works:
 *   1. Allocates individual pages (or small blocks) from the buddy system.
 *   2. Finds a free range in the vmalloc virtual address region.
 *   3. Maps all the pages into that virtual range using the kernel's
 *      page tables (each page gets its own PTE).
 *   4. Returns the start of the virtual range.
 *
 * COST vs kmalloc:
 *   - Slower: requires setting up page table mappings.
 *   - TLB pressure: each page has its own TLB entry (not contiguous in PA).
 *   - Cannot be used for DMA (physical addresses are scattered).
 *   - No size limit beyond available VM space (and available pages).
 *
 * Returns: kernel virtual address, or NULL on failure.
 * ──────────────────────────────────────────────────────────── */
void *vmalloc(unsigned long size);

/* vmalloc_to_page: Convert a vmalloc virtual address to struct page *  */
struct page *vmalloc_to_page(const void *vmalloc_addr);

/* vmalloc_to_pfn: Convert vmalloc VA to PFN */
unsigned long vmalloc_to_pfn(const void *vmalloc_addr);
```

**Virtual memory layout of a vmalloc allocation:**

```
vmalloc area (in kernel VA space):

  VA: 0xFFFF_C000_0000_0000
  ├─ [vmalloc region 1: 4 MiB]
  │   VA pages: 0xC000_0000 → 0xC000_0000 + 4MiB (contiguous in VA)
  │   PA pages: PFN 5, PFN 891, PFN 23, PFN 10002 ... (scattered in PA)
  │   Page table: 1024 PTEs, each pointing to a different PFN
  │
  ├─ [guard page] ← 1 unmapped page between regions (catches overflows)
  │
  ├─ [vmalloc region 2: 2 MiB]
  │   VA pages: contiguous, PA pages: scattered
  │
  VA: 0xFFFF_E000_0000_0000
```

---

### 5.2 `kvmalloc` — The Smart Hybrid

```c
/* ─────────────────────────────────────────────────────────────
 * kvmalloc(size, flags)
 *
 * "Best effort" allocator — tries physically contiguous first,
 * falls back to virtually contiguous if that fails.
 *
 * Algorithm:
 *   1. Try kmalloc(size, flags | __GFP_NOWARN | __GFP_NORETRY)
 *      (no retry, no warning — we handle failure ourselves)
 *   2. If kmalloc succeeds → return the pointer (physically contiguous).
 *   3. If kmalloc fails AND size > PAGE_SIZE → try vmalloc(size).
 *   4. Return whichever succeeded, or NULL if both fail.
 *
 * WHY USE THIS?
 *   - You want physical contiguity when possible (better performance).
 *   - But you don't REQUIRE it (no DMA, no hardware constraints).
 *   - You want to succeed even under memory pressure.
 *
 * kvmalloc_array: Array variant with overflow check.
 * kvzalloc:       Zero-initialized variant.
 * ──────────────────────────────────────────────────────────── */
void *kvmalloc(size_t size, gfp_t flags);
void *kvmalloc_array(size_t n, size_t size, gfp_t flags);
static inline void *kvzalloc(size_t size, gfp_t flags)
{
    return kvmalloc(size, flags | __GFP_ZERO);
}
```

**Decision flowchart for choosing between kmalloc, vmalloc, kvmalloc:**

```
Do you need DMA (device memory access)?
    YES → Use kmalloc() (physically contiguous required)
    NO  ↓
Is the size > 128 KiB?
    NO  → Use kmalloc() (efficient for small sizes)
    YES ↓
Is physical contiguity required for ANY reason?
    YES → Use alloc_pages() (and manage pages yourself)
    NO  ↓
Do you want to succeed even under memory pressure?
    YES → Use kvmalloc() (tries kmalloc, falls back to vmalloc)
    NO  → Use vmalloc() (directly, no wasted kmalloc attempt)
```

---

### 5.3 `vfree` and `kvfree`

```c
/* ─────────────────────────────────────────────────────────────
 * vfree(addr)
 *
 * Frees memory allocated by vmalloc(), vzalloc(), vmalloc_user(), etc.
 * Unmaps the virtual address range and returns individual pages
 * to the buddy system.
 *
 * Passing NULL is safe (no-op).
 * NEVER use kfree() on vmalloc() memory (and vice versa).
 * ──────────────────────────────────────────────────────────── */
void vfree(const void *addr);

/* ─────────────────────────────────────────────────────────────
 * kvfree(addr)
 *
 * Safe counterpart to kvmalloc().
 * Determines if the pointer came from kmalloc or vmalloc
 * and calls the appropriate free function.
 *
 * How it determines origin:
 *   is_vmalloc_addr(addr) checks if VA is in the vmalloc region.
 *   TRUE  → call vfree(addr)
 *   FALSE → call kfree(addr)
 * ──────────────────────────────────────────────────────────── */
void kvfree(const void *addr);
```

**Large allocation example — DMA buffer vs. large software buffer:**

```c
// ============================================================
// large_alloc_example.c
// Demonstrates when to use each large-allocation strategy.
// ============================================================
#include <linux/module.h>
#include <linux/slab.h>
#include <linux/vmalloc.h>
#include <linux/mm.h>

#define BUFFER_SIZE (4 * 1024 * 1024)  // 4 MiB

static int __init large_alloc_init(void)
{
    void *kmalloc_buf;
    void *vmalloc_buf;
    void *kv_buf;

    // ── Case 1: DMA-capable buffer (must be physically contiguous) ──
    // This will FAIL on a fragmented system for large sizes.
    // Use GFP_DMA32 for 32-bit DMA devices.
    kmalloc_buf = kmalloc(64 * 1024, GFP_KERNEL | GFP_DMA32);
    if (!kmalloc_buf) {
        pr_warn("kmalloc DMA buffer failed (system fragmented?)\n");
    } else {
        pr_info("DMA buffer: virt=%p phys=%pa (contiguous)\n",
                kmalloc_buf, &virt_to_phys(kmalloc_buf));
        kfree(kmalloc_buf);
    }

    // ── Case 2: Large software buffer (no DMA needed) ──
    // vmalloc succeeds even if PA is fragmented.
    vmalloc_buf = vmalloc(BUFFER_SIZE);
    if (!vmalloc_buf) {
        pr_err("vmalloc(%d MiB) failed\n", BUFFER_SIZE >> 20);
        return -ENOMEM;
    }
    pr_info("vmalloc buffer: virt=%p (4 MiB, VA-contiguous)\n", vmalloc_buf);

    // Individual page physical addresses (scattered):
    {
        struct page *p0 = vmalloc_to_page(vmalloc_buf);
        struct page *p1 = vmalloc_to_page(vmalloc_buf + PAGE_SIZE);
        pr_info("  page0 PA=0x%llx, page1 PA=0x%llx (likely NOT adjacent)\n",
                (u64)page_to_phys(p0), (u64)page_to_phys(p1));
    }

    memset(vmalloc_buf, 0, BUFFER_SIZE);  // Can use normally via VA
    vfree(vmalloc_buf);

    // ── Case 3: Best-effort — use kvmalloc ──
    kv_buf = kvmalloc(BUFFER_SIZE, GFP_KERNEL);
    if (!kv_buf) {
        pr_err("kvmalloc failed\n");
        return -ENOMEM;
    }
    pr_info("kvmalloc: virt=%p is_vmalloc=%d\n",
            kv_buf, is_vmalloc_addr(kv_buf));
    kvfree(kv_buf);  // kvfree automatically picks kfree or vfree

    return 0;
}

static void __exit large_alloc_exit(void) { }

module_init(large_alloc_init);
module_exit(large_alloc_exit);
MODULE_LICENSE("GPL");
```

---

## 6. GFP Flags — Allocation Behavior Control

### 6.1 What GFP Means

**GFP = Get Free Pages.** Every kernel memory allocation function accepts a `gfp_t`
bitmask that controls:

1. **Which memory zone** to allocate from (normal, DMA, DMA32).
2. **How hard to try** when memory is scarce (retry, wait for reclaim, panic).
3. **Whether the allocator can sleep** (waiting for I/O, swapping pages out).
4. **Special behaviors** (zeroing, accounting, movability).

---

### 6.2 Flag Deep Dive

```c
/* ═══════════════════════════════════════════════════════════
 * ZONE MODIFIERS — where to allocate from
 * ═══════════════════════════════════════════════════════════ */

__GFP_DMA
    // Allocate from ZONE_DMA (0–16 MiB on x86).
    // For legacy ISA devices with 24-bit DMA.
    // Avoid unless you have a specific 24-bit DMA device.

__GFP_DMA32
    // Allocate from ZONE_DMA32 (0–4 GiB).
    // For 32-bit DMA-capable PCI devices on 64-bit systems.
    // More common than __GFP_DMA.

__GFP_HIGHMEM
    // Allow allocation from ZONE_HIGHMEM (32-bit x86 only).
    // On 64-bit, HIGHMEM doesn't exist — flag is ignored.

/* ═══════════════════════════════════════════════════════════
 * RECLAIM BEHAVIOR — what to do when memory is scarce
 * ═══════════════════════════════════════════════════════════ */

__GFP_IO
    // Allow page-out I/O to free memory.
    // Enables writing dirty pages to disk to reclaim RAM.
    // Don't use in contexts where I/O would deadlock.

__GFP_FS
    // Allow filesystem operations during reclaim.
    // Don't use in filesystem code paths (risk of recursion deadlock).

__GFP_RECLAIM  (= __GFP_DIRECT_RECLAIM | __GFP_KSWAPD_RECLAIM)
    // Allow memory reclaim (both direct and via kswapd daemon).

__GFP_DIRECT_RECLAIM
    // The calling process will synchronously reclaim memory.
    // This CAN SLEEP (the process blocks until memory is freed).

__GFP_KSWAPD_RECLAIM
    // Wake up kswapd daemon to reclaim memory asynchronously.
    // Does not sleep by itself.

__GFP_RETRY_MAYFAIL
    // Retry allocation several times before giving up.
    // Returns NULL on persistent failure (doesn't panic).
    // For large allocations where failure is tolerable.

__GFP_NOFAIL
    // NEVER return NULL — loop forever until memory is available.
    // Use only for allocations that are truly unrecoverable on failure.
    // Risk: can lock up the system if memory never becomes available.

__GFP_NORETRY
    // Don't retry — fail immediately if no memory is available.
    // Use when you have a fallback path (like kvmalloc's attempt).

__GFP_NOWARN
    // Don't print warning messages on allocation failure.
    // Use when you expect and handle failure (e.g., in kvmalloc).

/* ═══════════════════════════════════════════════════════════
 * BEHAVIORAL FLAGS
 * ═══════════════════════════════════════════════════════════ */

__GFP_ZERO
    // Zero the allocated memory before returning.
    // This is how kzalloc/kcalloc/get_zeroed_page work.

__GFP_ATOMIC
    // [NOT a real flag — historical alias for GFP_ATOMIC behavior]
    // Use the GFP_ATOMIC composite flag instead (see below).

__GFP_ACCOUNT
    // Charge this allocation to the current memory cgroup.
    // Important for container memory accounting.

__GFP_MOVABLE
    // The allocated pages can be migrated/compacted.
    // Use for long-lived allocations to reduce fragmentation.

__GFP_NOWAIT
    // Do not wait (sleep) under any circumstances.
    // Returns NULL immediately if allocation can't be satisfied.

/* ═══════════════════════════════════════════════════════════
 * COMPOSITE FLAGS — use these in practice
 * ═══════════════════════════════════════════════════════════ */

GFP_KERNEL
    // = __GFP_RECLAIM | __GFP_IO | __GFP_FS
    // The standard flag for normal kernel allocations.
    // CAN SLEEP (the calling process may block waiting for memory).
    // Use in process context (e.g., syscall handlers, kernel threads).
    // NOT safe in interrupt handlers or with spinlocks held.

GFP_ATOMIC
    // = __GFP_HIGH (use emergency reserves)
    // CANNOT SLEEP — returns NULL immediately if no memory is free.
    // Use ONLY in:
    //   - Interrupt handlers (IRQ, softirq, tasklet)
    //   - Code holding a spinlock
    //   - Any non-preemptible context
    // Uses a small emergency memory pool.

GFP_NOWAIT
    // Like GFP_ATOMIC but doesn't use emergency reserves.
    // Use in real-time contexts that prefer immediate failure.

GFP_NOIO
    // = __GFP_RECLAIM (but without __GFP_IO | __GFP_FS)
    // CAN SLEEP, but won't start I/O.
    // Use in storage drivers to avoid deadlocks in I/O paths.

GFP_NOFS
    // = __GFP_RECLAIM | __GFP_IO (but without __GFP_FS)
    // CAN SLEEP, allows I/O but not filesystem operations.
    // Use in filesystem code to avoid recursive deadlocks.

GFP_USER
    // For user-space memory allocations.
    // Like GFP_KERNEL + __GFP_MOVABLE.

GFP_HIGHUSER
    // Like GFP_USER + __GFP_HIGHMEM (32-bit x86 mainly).

GFP_DMA
    // = GFP_ATOMIC | __GFP_DMA
    // Atomic allocation from ZONE_DMA.

GFP_DMA32
    // = GFP_KERNEL | __GFP_DMA32
    // Sleeping allocation from ZONE_DMA32.
```

---

### 6.3 Choosing the Right Flag

```
Are you in an interrupt context?
  YES (in_interrupt() == true)
      → GFP_ATOMIC
      → Never use GFP_KERNEL here (will deadlock or BUG())

Are you in process context?
  └── Are you in a filesystem path?
      YES → GFP_NOFS   (prevents recursive filesystem locking)
      └── Does your path start I/O?
          YES → GFP_NOIO   (prevents deadlock in I/O path)
          NO  → GFP_KERNEL (standard choice)

Do you need DMA memory?
  24-bit DMA device   → GFP_KERNEL | __GFP_DMA    (rarely needed)
  32-bit DMA device   → GFP_KERNEL | __GFP_DMA32   (common for PCI)
  64-bit DMA device   → GFP_KERNEL                 (ZONE_NORMAL fine)

Can you tolerate failure?
  YES → GFP_KERNEL | __GFP_NORETRY | __GFP_NOWARN
  NO  → GFP_KERNEL | __GFP_NOFAIL   (use sparingly)

Are you debugging?
  Add __GFP_ZERO to catch use-of-uninitialized-memory bugs.
```

---

## 7. Memory Reclaim and Pressure

Understanding reclaim is essential for writing allocators that behave correctly
under pressure. This is often the hidden failure mode of kernel code.

### The kswapd Daemon

The kernel runs a daemon called **kswapd** (one per NUMA node) that runs in the
background, proactively reclaiming memory when the system approaches low-memory
thresholds.

```
Memory Watermarks (per zone):

  ┌──────────────────────────────────────┐ ← Total zone pages
  │  pages_high  (HIGH watermark)        │
  │  Zone is healthy. kswapd sleeps.     │
  ├──────────────────────────────────────┤ ← pages_low (LOW watermark)
  │  Zone is getting tight.              │
  │  kswapd wakes up and starts reclaim. │
  ├──────────────────────────────────────┤ ← pages_min (MIN watermark)
  │  CRITICAL. Direct reclaim kicks in.  │
  │  New allocations may block the       │
  │  calling process to reclaim memory.  │
  │  Emergency pool reserved for         │
  │  GFP_ATOMIC allocations.            │
  └──────────────────────────────────────┘ ← 0
```

### Direct Reclaim

When an allocation with `__GFP_DIRECT_RECLAIM` (included in `GFP_KERNEL`) fails
because memory is below `pages_min`, the kernel runs reclaim **inline** in the
calling process's context. This means `kmalloc(GFP_KERNEL)` can sleep for
milliseconds while the kernel:

1. Writes dirty page-cache pages to disk.
2. Drops clean page-cache pages.
3. Swaps out user-space anonymous pages.

```
Direct Reclaim Flow:

kmalloc(size, GFP_KERNEL)
    │
    ├── Attempt fast allocation from freelist → FAIL (below watermark)
    │
    └── __alloc_pages_slowpath()
            │
            ├── Wake kswapd
            ├── Try compaction (move movable pages)
            ├── Direct reclaim:
            │     shrink_node()
            │     ├── shrink_lruvec() → write dirty pages, drop clean
            │     └── shrink_slab()   → shrinker callbacks
            │
            └── Retry allocation → SUCCESS or OOM
                                         │
                                         └── OOM Killer selects a victim
                                             process to kill if memory
                                             cannot be freed.
```

### OOM Killer

If all reclaim attempts fail, the **OOM (Out Of Memory) Killer** is invoked.
It selects a process to kill based on an **OOM score** (considers process memory
usage, runtime, privilege level, oom_score_adj setting) and sends it SIGKILL.

```c
// You can adjust a process's OOM score from kernel code:
// Higher value = more likely to be killed.
// Range: -1000 (never kill) to +1000 (kill first)
// Set from userspace: echo 500 > /proc/<pid>/oom_score_adj
```

---

## 8. Per-CPU Allocators

One of the kernel's most powerful optimization techniques: **per-CPU variables**.
These are variables where each CPU has its own private copy, eliminating the
need for atomic operations or locking for per-CPU counters and caches.

```c
#include <linux/percpu.h>
#include <linux/percpu-defs.h>

/* ─────────────────────────────────────────────────────────────
 * Static per-CPU variables:
 * Allocated at compile time in a special .percpu section.
 * ──────────────────────────────────────────────────────────── */
DEFINE_PER_CPU(int, my_counter);       // int, one per CPU
DEFINE_PER_CPU(struct my_state, cpu_state);

// Access (must disable preemption to avoid CPU migration):
void example_static_percpu(void)
{
    preempt_disable();                           // pin to current CPU
    __this_cpu_inc(my_counter);                  // increment current CPU's copy
    int val = __this_cpu_read(my_counter);       // read current CPU's copy
    __this_cpu_write(my_counter, 42);            // write current CPU's copy
    preempt_enable();
}

/* ─────────────────────────────────────────────────────────────
 * Dynamic per-CPU allocation:
 * Allocated at runtime (module init, device probe, etc.)
 * ──────────────────────────────────────────────────────────── */
int __percpu *dynamic_counter;

// Allocate: size bytes on each CPU, with given alignment
dynamic_counter = alloc_percpu(int);
// or: alloc_percpu_gfp(int, GFP_KERNEL)
// or: __alloc_percpu(size_t size, size_t align)

// Access:
{
    int *ptr = per_cpu_ptr(dynamic_counter, smp_processor_id());
    // or: this_cpu_ptr(dynamic_counter) [with preemption disabled]
}

// Free:
free_percpu(dynamic_counter);
```

**How per-CPU memory is laid out:**

```
Per-CPU memory layout (at kernel boot):

  CPU 0 section:
  ┌─────────────────────────────────────────┐
  │ var_a[cpu=0] │ var_b[cpu=0] │ ...       │
  └─────────────────────────────────────────┘

  CPU 1 section (separate physical location):
  ┌─────────────────────────────────────────┐
  │ var_a[cpu=1] │ var_b[cpu=1] │ ...       │
  └─────────────────────────────────────────┘

  CPU N section:
  ┌─────────────────────────────────────────┐
  │ var_a[cpu=N] │ var_b[cpu=N] │ ...       │
  └─────────────────────────────────────────┘

No cache line bouncing between CPUs.
No atomic operations needed for single-CPU access.
```

---

## 9. DMA and Coherent Memory

When a device (e.g., NIC, GPU, storage controller) wants to read or write
system memory directly without involving the CPU, it uses **DMA (Direct Memory Access)**.

The problem: CPUs have caches. If the CPU writes data to an address, that data
may sit in the CPU's L1/L2 cache and not yet be in RAM. The DMA device reads
**RAM**, not the CPU cache → it reads **stale data**.

The kernel provides two DMA memory models:

### Coherent (Consistent) DMA Memory

```c
#include <linux/dma-mapping.h>

/* ─────────────────────────────────────────────────────────────
 * dma_alloc_coherent(dev, size, dma_handle, gfp)
 *
 * Allocates memory that is coherent between CPU and device.
 * "Coherent" means: no explicit cache flushing needed.
 * The hardware/firmware handles coherency (IOMMU, cache-coherent fabric).
 *
 * dev:        The device that will do DMA.
 * size:       Bytes to allocate.
 * dma_handle: OUTPUT — the DMA (device-visible) address.
 * gfp:        Allocation flags.
 *
 * Returns: CPU virtual address of the buffer.
 *          *dma_handle is set to the DMA address (bus address).
 * ──────────────────────────────────────────────────────────── */
void *dma_alloc_coherent(struct device *dev, size_t size,
                         dma_addr_t *dma_handle, gfp_t gfp);

void dma_free_coherent(struct device *dev, size_t size,
                       void *cpu_addr, dma_addr_t dma_handle);
```

**Two addresses for one buffer:**

```
┌──────────────────────────────────────────────────────────┐
│          Physical RAM                                     │
│   ┌────────────────────────────────────┐                 │
│   │     DMA buffer (e.g., 64 KiB)     │                 │
│   └────────────────────────────────────┘                 │
│             ↑                    ↑                       │
│    CPU virtual address      DMA (bus) address            │
│    (from dma_alloc_coherent) (stored in *dma_handle)     │
│                                                          │
│    CPU uses virtual address to read/write the buffer.    │
│    Device uses DMA address to read/write the buffer.     │
│    Both see the SAME data (coherent).                    │
└──────────────────────────────────────────────────────────┘
```

### Streaming (Non-Coherent) DMA

For data that flows one direction (CPU fills buffer → device reads it, or device
fills buffer → CPU reads it), streaming DMA is more efficient:

```c
// Map a kmalloc buffer for DMA (CPU → Device direction):
dma_addr_t dma_map_single(struct device *dev, void *cpu_addr,
                           size_t size, enum dma_data_direction dir);
// dir: DMA_TO_DEVICE, DMA_FROM_DEVICE, or DMA_BIDIRECTIONAL

// After device is done, unmap:
void dma_unmap_single(struct device *dev, dma_addr_t dma_addr,
                      size_t size, enum dma_data_direction dir);
```

---

## 10. Complete Architecture Diagram

```
╔══════════════════════════════════════════════════════════════════════════════╗
║           LINUX KERNEL MEMORY ALLOCATION — COMPLETE ARCHITECTURE            ║
╠══════════════════════════════════════════════════════════════════════════════╣
║                                                                              ║
║  CALLER CODE (drivers, subsystems, filesystems)                              ║
║  ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌───────────┐ ┌───────────┐  ║
║  │  kmalloc() │ │  kzalloc() │ │  kcalloc() │ │ vmalloc() │ │kvmalloc() │  ║
║  └─────┬──────┘ └─────┬──────┘ └─────┬──────┘ └─────┬─────┘ └─────┬─────┘  ║
║        │              │              │               │              │        ║
║        └──────────────┴──────────────┘               │         ┌───┘        ║
║                       │                              │         │            ║
║                       ▼                              │         │            ║
║  ┌────────────────────────────────────────────┐      │         │            ║
║  │         SLAB / SLUB Allocator              │      │         │            ║
║  │  ┌──────────┬──────────┬──────────┐        │      │         │            ║
║  │  │kmalloc-8 │kmalloc-64│kmalloc-1k│  ...   │      │         │            ║
║  │  │  cache   │  cache   │  cache   │        │      │         │            ║
║  │  └────┬─────┴─────┬────┴────┬─────┘        │      │         │            ║
║  │       │ per-CPU   │ partial │ full lists    │      │         │            ║
║  └───────┼───────────┼─────────┼──────────────┘      │         │            ║
║          │           │         │                      │         │            ║
║          └───────────┴─────────┘                      │         │            ║
║                       │                               │         │            ║
║                       │ (needs new slab)               │         │            ║
║                       ▼                               ▼         ▼            ║
║  ┌──────────────────────────────────────────────────────────────────────┐   ║
║  │                  BUDDY SYSTEM (Page Allocator)                       │   ║
║  │                                                                      │   ║
║  │  ZONE_DMA        ZONE_DMA32           ZONE_NORMAL                   │   ║
║  │  (0–16 MiB)      (0–4 GiB)            (>4 GiB on 64-bit)            │   ║
║  │  ┌──────────┐   ┌──────────┐          ┌─────────────────────────┐   │   ║
║  │  │Order 0   │   │Order 0   │          │Order 0  [pg][pg][pg]... │   │   ║
║  │  │Order 1   │   │Order 1   │          │Order 1  [pg,pg][pg,pg]  │   │   ║
║  │  │...       │   │...       │          │...                      │   │   ║
║  │  │Order 10  │   │Order 10  │          │Order 10 [1024 pages]    │   │   ║
║  │  └──────────┘   └──────────┘          └─────────────────────────┘   │   ║
║  └──────────────────────────────────────────────────────────────────────┘   ║
║                       │                                                      ║
║                       ▼                                                      ║
║  ┌──────────────────────────────────────────────────────────────────────┐   ║
║  │                  PHYSICAL RAM (struct page mem_map[])                │   ║
║  │                                                                      │   ║
║  │  [pg0][pg1][pg2][pg3][pg4][pg5]...[pgN]                             │   ║
║  │   4KiB each, tracked by struct page                                  │   ║
║  └──────────────────────────────────────────────────────────────────────┘   ║
║                                                                              ║
║  VIRTUAL ADDRESS SPACE (Kernel):                                             ║
║  ┌──────────────────────────────────────────────────────────────────────┐   ║
║  │  Direct Map Region              vmalloc Region                       │   ║
║  │  0xffff880000000000             0xffffc90000000000                   │   ║
║  │  ┌────────────────────┐         ┌─────────────────────────────────┐ │   ║
║  │  │ PA 0→N linearly    │         │ VA-contiguous, PA-scattered     │ │   ║
║  │  │ mapped (+offset)   │         │ (vmalloc pages with PTEs)       │ │   ║
║  │  │ kmalloc() lives    │         │ vmalloc() / kvmalloc() live here│ │   ║
║  │  │ here              │         └─────────────────────────────────┘ │   ║
║  │  └────────────────────┘                                              │   ║
║  └──────────────────────────────────────────────────────────────────────┘   ║
║                                                                              ║
║  RECLAIM PATH:                                                               ║
║  allocation fails → kswapd wakes → direct reclaim → OOM killer              ║
║                                                                              ║
╚══════════════════════════════════════════════════════════════════════════════╝
```

---

## 11. C Implementation Examples

### 11.1 A Complete Kernel Module — Device Driver Memory Management

```c
// ============================================================
// driver_memory_demo.c
//
// A realistic kernel module demonstrating correct usage of
// all major allocation APIs with proper error handling,
// RAII-style cleanup, and best practices.
//
// Build: make -C /lib/modules/$(uname -r)/build M=$PWD modules
// ============================================================

#include <linux/module.h>
#include <linux/kernel.h>
#include <linux/init.h>
#include <linux/slab.h>          // kmalloc, kzalloc, kmem_cache_*
#include <linux/vmalloc.h>       // vmalloc, vfree, kvmalloc, kvfree
#include <linux/mm.h>            // alloc_pages, free_pages, page_address
#include <linux/gfp.h>           // GFP_* flags
#include <linux/errno.h>
#include <linux/string.h>

MODULE_LICENSE("GPL");
MODULE_AUTHOR("Kernel Learner");
MODULE_DESCRIPTION("Memory allocation demonstration module");

/* ─────────────────────────────────────────────────────────────
 * Data structures
 * ──────────────────────────────────────────────────────────── */

// Descriptor for a network-like packet (small, frequent object)
struct my_packet {
    u32 seq_num;
    u16 payload_len;
    u8  priority;
    u8  flags;
    u8  payload[56];         // 56 bytes inline
    struct list_head list;   // For linking packets in a queue
};

// Large driver state structure (allocated once at probe time)
struct my_driver_state {
    spinlock_t       lock;
    u32              device_id;
    u32              irq_count;
    struct list_head packet_queue;
    unsigned long    stats[16];
    char             name[64];
};

/* ─────────────────────────────────────────────────────────────
 * Global: the packet slab cache
 * Created once at module load, destroyed at module unload.
 * ──────────────────────────────────────────────────────────── */
static struct kmem_cache *packet_cache __read_mostly;

/* Constructor: called once per new slab object.
 * Initialize fields that never change between alloc/free cycles. */
static void packet_ctor(void *obj)
{
    struct my_packet *pkt = obj;
    INIT_LIST_HEAD(&pkt->list);  // Initialize the list linkage
    pkt->flags = 0;
}

/* ─────────────────────────────────────────────────────────────
 * Packet allocation/deallocation helpers
 * ──────────────────────────────────────────────────────────── */

static struct my_packet *alloc_packet(u32 seq, u16 len, u8 priority)
{
    struct my_packet *pkt;

    // Fast path: allocate from per-CPU slab freelist.
    // Must NOT use GFP_KERNEL in interrupt context.
    // This function may be called from softirq → use GFP_ATOMIC.
    pkt = kmem_cache_alloc(packet_cache, GFP_ATOMIC);
    if (unlikely(!pkt))
        return NULL;

    // Initialize per-allocation fields (not done in ctor)
    pkt->seq_num     = seq;
    pkt->payload_len = len;
    pkt->priority    = priority;
    memset(pkt->payload, 0, sizeof(pkt->payload));

    return pkt;
}

static void free_packet(struct my_packet *pkt)
{
    if (!pkt)
        return;
    // Remove from any list before freeing
    list_del_init(&pkt->list);
    // Return to slab cache (object stays "warm" for future use)
    kmem_cache_free(packet_cache, pkt);
}

/* ─────────────────────────────────────────────────────────────
 * Module initialization — demonstrates all allocator types
 * ──────────────────────────────────────────────────────────── */

static int __init driver_memory_demo_init(void)
{
    struct my_driver_state *state = NULL;
    struct my_packet       *pkt1 = NULL, *pkt2 = NULL;
    void                   *large_buf = NULL;
    struct page            *raw_pages = NULL;
    void                   *raw_vaddr;
    int                     ret = 0;

    pr_info("=== Memory Allocation Demo Starting ===\n");

    /* ── 1. Create the packet slab cache ──────────────────── */
    packet_cache = kmem_cache_create(
        "my_packet",                  // name (appears in /proc/slabinfo)
        sizeof(struct my_packet),     // object size: 64 bytes (8+56)
        0,                            // align: default (SLUB picks)
        SLAB_HWCACHE_ALIGN |          // align to CPU cache line
        SLAB_POISON        |          // detect use-after-free (debug)
        SLAB_ACCOUNT,                 // account to memory cgroup
        packet_ctor                   // constructor
    );

    if (!packet_cache) {
        pr_err("Failed to create packet_cache\n");
        ret = -ENOMEM;
        goto err_out;
    }
    pr_info("[1] Slab cache 'my_packet' created, obj_size=%u\n",
            kmem_cache_size(packet_cache));

    /* ── 2. Allocate the driver state (kzalloc) ────────────── */
    // kzalloc: guaranteed zeroed, physically contiguous, process context.
    // sizeof < 4KiB → goes to kmalloc-1024 or kmalloc-512 slab.
    state = kzalloc(sizeof(*state), GFP_KERNEL);
    if (!state) {
        pr_err("Failed to allocate driver state (%zu bytes)\n",
               sizeof(*state));
        ret = -ENOMEM;
        goto err_destroy_cache;
    }
    pr_info("[2] Driver state at %p (kzalloc, %zu bytes)\n",
            state, sizeof(*state));

    // Initialize the state
    spin_lock_init(&state->lock);
    INIT_LIST_HEAD(&state->packet_queue);
    state->device_id = 0x1234ABCD;
    snprintf(state->name, sizeof(state->name), "my_device_0");

    /* ── 3. Allocate packets from slab cache ────────────────── */
    pkt1 = alloc_packet(1, 42, 5);
    pkt2 = alloc_packet(2, 100, 3);

    if (!pkt1 || !pkt2) {
        pr_err("Packet allocation failed\n");
        ret = -ENOMEM;
        goto err_free_state;
    }

    pr_info("[3] pkt1=%p seq=%u, pkt2=%p seq=%u\n",
            pkt1, pkt1->seq_num, pkt2, pkt2->seq_num);

    // Simulate a queue
    list_add_tail(&pkt1->list, &state->packet_queue);
    list_add_tail(&pkt2->list, &state->packet_queue);

    /* ── 4. Large buffer with kvmalloc ─────────────────────── */
    // 4 MiB — kvmalloc tries kmalloc first (likely fails due to
    // fragmentation/size), then falls back to vmalloc.
    large_buf = kvmalloc(4 * 1024 * 1024, GFP_KERNEL);
    if (!large_buf) {
        pr_err("kvmalloc(4 MiB) failed\n");
        ret = -ENOMEM;
        goto err_free_pkts;
    }
    pr_info("[4] Large buffer at %p (kvmalloc, 4 MiB), is_vmalloc=%d\n",
            large_buf, is_vmalloc_addr(large_buf));

    /* ── 5. Raw page allocation ─────────────────────────────── */
    // Allocate 4 contiguous pages (order 2 = 2^2 = 4 pages = 16 KiB).
    raw_pages = alloc_pages(GFP_KERNEL | __GFP_ZERO, 2);  // order 2
    if (!raw_pages) {
        pr_err("alloc_pages(order=2) failed\n");
        ret = -ENOMEM;
        goto err_free_large;
    }
    raw_vaddr = page_address(raw_pages);
    pr_info("[5] Raw pages: struct page=%p, vaddr=%p, phys=0x%llx\n",
            raw_pages, raw_vaddr, (u64)page_to_phys(raw_pages));

    /* ── Simulate work ──────────────────────────────────────── */
    memset(raw_vaddr, 0xDE, 4 * PAGE_SIZE);
    pr_info("    First byte of raw pages: 0x%02x\n",
            *(unsigned char *)raw_vaddr);

    /* ── Cleanup in REVERSE order of allocation ─────────────── */
    // This is the RAII principle: destroy in reverse order.

    __free_pages(raw_pages, 2);     // [5]
    pr_info("[5] Raw pages freed\n");

err_free_large:
    kvfree(large_buf);              // [4] — kvfree handles both kmalloc/vmalloc
    pr_info("[4] Large buffer freed\n");

err_free_pkts:
    free_packet(pkt1);              // [3]
    free_packet(pkt2);
    pr_info("[3] Packets freed\n");

err_free_state:
    kfree(state);                   // [2]
    pr_info("[2] Driver state freed\n");

err_destroy_cache:
    if (packet_cache)
        kmem_cache_destroy(packet_cache); // [1] Must be LAST
    pr_info("[1] Slab cache destroyed\n");

err_out:
    if (ret == 0)
        pr_info("=== Demo Complete (no errors) ===\n");
    else
        pr_err("=== Demo Complete (errors occurred: %d) ===\n", ret);

    return ret;  // Return 0 = module loads; nonzero = load fails
}

static void __exit driver_memory_demo_exit(void)
{
    pr_info("Module unloaded\n");
}

module_init(driver_memory_demo_init);
module_exit(driver_memory_demo_exit);
```

### 11.2 Memory Pressure — Shrinker Callback

When the system is low on memory, subsystems can register a **shrinker** to
voluntarily release their cached objects. This is how the slab layer interfaces
with the VM's reclaim machinery.

```c
// ============================================================
// shrinker_example.c
// Demonstrates registering a custom memory shrinker.
// ============================================================
#include <linux/shrinker.h>
#include <linux/mm.h>
#include <linux/atomic.h>

// A simple object cache we manage ourselves
static DEFINE_SPINLOCK(my_cache_lock);
static LIST_HEAD(my_cache_list);
static atomic_long_t my_cache_count = ATOMIC_LONG_INIT(0);

// count_objects: Tell the MM how many objects we have cached.
// Called by the shrinker framework to estimate reclaim potential.
static unsigned long my_count_objects(struct shrinker *shrinker,
                                       struct shrink_control *sc)
{
    return atomic_long_read(&my_cache_count);
}

// scan_objects: Called when the MM wants us to free some objects.
// sc->nr_to_scan: how many to free this call.
// Return: number actually freed.
static unsigned long my_scan_objects(struct shrinker *shrinker,
                                      struct shrink_control *sc)
{
    unsigned long freed = 0;
    struct my_cached_obj *obj, *tmp;

    spin_lock(&my_cache_lock);
    list_for_each_entry_safe(obj, tmp, &my_cache_list, list) {
        if (freed >= sc->nr_to_scan)
            break;
        list_del(&obj->list);
        kfree(obj);
        atomic_long_dec(&my_cache_count);
        freed++;
    }
    spin_unlock(&my_cache_lock);

    return freed;
}

// Register the shrinker at module init:
static struct shrinker my_shrinker = {
    .count_objects = my_count_objects,
    .scan_objects  = my_scan_objects,
    .seeks         = DEFAULT_SEEKS,  // relative cost of recreating objects
};

static int __init shrinker_init(void)
{
    return register_shrinker(&my_shrinker);
}

static void __exit shrinker_exit(void)
{
    unregister_shrinker(&my_shrinker);
    // ... free remaining objects ...
}
```

### 11.3 Detecting Allocation Context — Safe Flag Selection

```c
// ============================================================
// context_aware_alloc.c
//
// A helper function that chooses the correct GFP flag based
// on the calling context — a pattern used in many subsystems.
// ============================================================
#include <linux/interrupt.h>  // in_interrupt(), in_atomic()
#include <linux/slab.h>

/**
 * safe_alloc - Allocates memory with context-appropriate GFP flag.
 * @size: Number of bytes to allocate.
 *
 * Automatically selects GFP_ATOMIC for interrupt/atomic contexts
 * and GFP_KERNEL for process contexts.
 *
 * Returns: pointer to allocated memory, or NULL on failure.
 */
static void *safe_alloc(size_t size)
{
    gfp_t flags;

    if (in_interrupt() || in_atomic()) {
        // We are in an interrupt handler, softirq, tasklet,
        // or we are holding a spinlock. Cannot sleep.
        flags = GFP_ATOMIC;
    } else {
        // Process context. Can sleep while waiting for memory.
        flags = GFP_KERNEL;
    }

    return kzalloc(size, flags);
}

// Usage example in a network driver:
static irqreturn_t my_irq_handler(int irq, void *dev_id)
{
    struct rx_buffer *buf;

    // Called from IRQ context — safe_alloc correctly uses GFP_ATOMIC
    buf = safe_alloc(sizeof(*buf));
    if (!buf) {
        // GFP_ATOMIC failure is expected under memory pressure.
        // Drop the packet and increment a stats counter.
        return IRQ_HANDLED;
    }

    // ... process received data into buf ...

    kfree(buf);
    return IRQ_HANDLED;
}
```

---

## 12. Rust in the Linux Kernel

### 12.1 Background: Rust in Linux

Since Linux 6.1 (merged December 2022), the Linux kernel officially supports
Rust as a second implementation language alongside C. The goal: eliminate
entire classes of memory safety bugs (use-after-free, double-free, buffer
overflow) by leveraging Rust's ownership and borrow-checker at compile time.

Rust kernel code uses a **separate API layer** (`rust/kernel/`) that wraps the
C allocators with safe Rust abstractions. You don't call `kmalloc()` directly
from Rust — you use `Box`, `Vec`, or custom allocation types that call
the C functions underneath through FFI.

### 12.2 The Kernel Allocator Bridge

```rust
// Conceptual view of how Rust kernel allocates memory.
// Source: linux/rust/kernel/allocator.rs (simplified)

use core::alloc::{GlobalAlloc, Layout};

// The kernel's Rust allocator — a zero-size struct that
// delegates all allocations to kmalloc/kfree.
pub struct KernelAllocator;

unsafe impl GlobalAlloc for KernelAllocator {
    unsafe fn alloc(&self, layout: Layout) -> *mut u8 {
        // bindings:: is the auto-generated FFI layer (bindgen output)
        // that exposes C kernel functions to Rust.
        let ptr = bindings::krealloc(
            core::ptr::null(),           // NULL ptr = fresh allocation
            layout.size(),               // requested size
            bindings::GFP_KERNEL | bindings::__GFP_ZERO,
        );
        ptr as *mut u8
    }

    unsafe fn dealloc(&self, ptr: *mut u8, _layout: Layout) {
        bindings::kfree(ptr as *const core::ffi::c_void);
    }
}

// Register as the global allocator for all of Rust kernel code:
#[global_allocator]
static ALLOCATOR: KernelAllocator = KernelAllocator;
```

### 12.3 Box and Vec in Kernel Rust

Standard Rust `Box<T>` and `Vec<T>` work in the kernel because the global
allocator above maps them to `kmalloc/kfree`.

```rust
// ============================================================
// rust_driver_example.rs
//
// A kernel module written in Rust demonstrating memory allocation.
// This is actual kernel Rust code style.
// ============================================================

use kernel::prelude::*;          // Kernel Rust prelude
use kernel::{
    sync::Mutex,
    net::SkBuff,
};

// Module declaration (Rust equivalent of MODULE_LICENSE, MODULE_AUTHOR)
module! {
    type: RustMemDemo,
    name: "rust_mem_demo",
    author: "Kernel Learner",
    description: "Rust memory allocation demo",
    license: "GPL",
}

// Our driver state — allocated on the kernel heap via Box<T>
struct DriverState {
    device_id: u32,
    packet_count: u64,
    buffer: Vec<u8>,    // growable buffer, heap-allocated via kvmalloc
    name: String,
}

struct RustMemDemo {
    state: Mutex<DriverState>,
}

impl kernel::Module for RustMemDemo {
    fn init(_module: &'static ThisModule) -> Result<Self> {
        pr_info!("Rust memory demo: init\n");

        // Box::try_new() calls kmalloc() under the hood.
        // Returns Err(ENOMEM) instead of panicking on OOM.
        // try_new is preferred over new() in kernel code (no panic).
        let buffer = Vec::try_with_capacity(4096)?;   // pre-allocate 4 KiB

        let state = DriverState {
            device_id: 0xDEAD_BEEF,
            packet_count: 0,
            buffer,
            name: String::try_from("rust_device_0")?,
        };

        // Mutex::new wraps the state. Mutex itself is Box'd.
        Ok(RustMemDemo {
            state: Mutex::new(state),
        })
        // If this returns Ok(x), `x` is stored (heap) until module unload.
        // If Err, all allocated memory is automatically freed here
        // by Rust's Drop machinery — no manual goto-cleanup needed!
    }
}

impl Drop for RustMemDemo {
    fn drop(&mut self) {
        // Called automatically when module is unloaded.
        // All fields (state, buffer, name) are automatically freed
        // by their own Drop implementations.
        pr_info!("Rust memory demo: exit — all memory freed automatically\n");
    }
}
```

### 12.4 Rust Kernel Allocation Error Handling Pattern

```rust
// ============================================================
// The canonical pattern for fallible kernel allocation in Rust.
// No panics. Returns Result<T, Error>.
// ============================================================

use kernel::prelude::*;
use kernel::error::code::ENOMEM;

struct MyPacket {
    seq_num: u32,
    payload: Vec<u8>,
}

impl MyPacket {
    fn new(seq: u32, capacity: usize) -> Result<Self> {
        // try_with_capacity: fallible Vec allocation.
        // Returns Err(ENOMEM) rather than panicking.
        let payload = Vec::try_with_capacity(capacity)
            .map_err(|_| ENOMEM)?;  // convert to kernel error code

        Ok(MyPacket {
            seq_num: seq,
            payload,
        })
    }
}

// In kernel init code:
fn allocate_packets() -> Result {
    // Question mark (?) propagates errors up — like goto err in C.
    let pkt1 = MyPacket::new(1, 64)?;
    let pkt2 = MyPacket::new(2, 128)?;

    // Box packets for heap storage:
    let _boxed1 = Box::try_new(pkt1)?;   // fails if OOM
    let _boxed2 = Box::try_new(pkt2)?;

    // When _boxed1, _boxed2 go out of scope, Drop is called:
    // Box::drop → kfree(ptr)
    // Vec::drop → kfree(payload_ptr)
    // — automatic, no manual kfree needed.

    Ok(())
}
```

### 12.5 Rust vs C Memory Safety Comparison

```
Property                     │ C Kernel Code          │ Rust Kernel Code
─────────────────────────────┼────────────────────────┼─────────────────────────
Use-after-free               │ Possible (programmer    │ Prevented at compile time
                             │ must be careful)        │ by borrow checker
─────────────────────────────┼────────────────────────┼─────────────────────────
Double-free                  │ Possible                │ Prevented (ownership)
─────────────────────────────┼────────────────────────┼─────────────────────────
Null pointer dereference     │ Possible (returns NULL) │ Option<T> forces checking
─────────────────────────────┼────────────────────────┼─────────────────────────
Buffer overflow               │ Possible                │ Bounds checked (in safe Rust)
─────────────────────────────┼────────────────────────┼─────────────────────────
Memory leak                  │ Possible (missing kfree)│ Drop trait handles dealloc
─────────────────────────────┼────────────────────────┼─────────────────────────
GFP flag mistakes            │ Possible                │ Type-system enforced
                             │ (GFP_KERNEL in IRQ)     │ (AllocFlags types)
─────────────────────────────┼────────────────────────┼─────────────────────────
Integer overflow in size     │ Must use kcalloc        │ Checked by type system
─────────────────────────────┼────────────────────────┼─────────────────────────
Cleanup on error             │ goto err pattern        │ Drop / ? operator
─────────────────────────────┼────────────────────────┼─────────────────────────
Uninitialized memory         │ Must use kzalloc        │ Requires unsafe{} to skip
─────────────────────────────┼────────────────────────┼─────────────────────────
Performance overhead         │ Zero                    │ Minimal (zero-cost abstractions)
─────────────────────────────┼────────────────────────┼─────────────────────────
Interoperability             │ Native                  │ Via FFI bindgen layer
```

### 12.6 Implementing a Custom Allocator in Rust (Standalone/Std)

While the above is kernel-specific, here is a standalone Rust program that
**models** the buddy system concept to build intuition:

```rust
// ============================================================
// buddy_model.rs
//
// A userspace simulation of the buddy allocator concept.
// Run with: rustc buddy_model.rs && ./buddy_model
//
// This is a MENTAL MODEL — not real kernel code.
// Real kernel buddy is in mm/page_alloc.c
// ============================================================

use std::collections::HashSet;

const MAX_ORDER: usize = 4;   // orders 0..=4 → max block = 2^4 = 16 "pages"
const TOTAL_PAGES: usize = 1 << MAX_ORDER;  // 16 total pages

/// Buddy allocator model.
/// free_lists[order] = set of block start indices that are free at that order.
struct BuddyAllocator {
    free_lists: [HashSet<usize>; MAX_ORDER + 1],
    total_pages: usize,
}

impl BuddyAllocator {
    fn new(total_pages: usize) -> Self {
        assert!(total_pages.is_power_of_two(), "Total pages must be power of 2");
        let order = total_pages.trailing_zeros() as usize;
        assert!(order <= MAX_ORDER, "Too large");

        let mut alloc = BuddyAllocator {
            free_lists: Default::default(),
            total_pages,
        };
        // Initially: one big free block at the highest order
        alloc.free_lists[order].insert(0);
        alloc
    }

    /// Allocate a block of 2^order pages.
    /// Returns the starting page index, or None if unavailable.
    fn allocate(&mut self, order: usize) -> Option<usize> {
        assert!(order <= MAX_ORDER);

        // Find the smallest available order >= requested order
        let avail_order = (order..=MAX_ORDER)
            .find(|&o| !self.free_lists[o].is_empty())?;

        // Take one block from that order's free list
        let block = *self.free_lists[avail_order].iter().next().unwrap();
        self.free_lists[avail_order].remove(&block);

        // Split down to requested order
        let mut current_block = block;
        let mut current_order = avail_order;
        while current_order > order {
            current_order -= 1;
            // The buddy of current_block at current_order
            let buddy = current_block + (1 << current_order);
            // Place the buddy on the lower order's free list
            self.free_lists[current_order].insert(buddy);
        }

        println!("  ALLOC: order={}, block_start={}, size={} pages",
                 order, current_block, 1 << order);
        Some(current_block)
    }

    /// Free a block of 2^order pages starting at page_index.
    fn free(&mut self, page_index: usize, order: usize) {
        assert!(order <= MAX_ORDER);

        let mut current = page_index;
        let mut current_order = order;

        println!("  FREE:  order={}, block_start={}", order, page_index);

        // Coalesce with buddies as long as possible
        while current_order < MAX_ORDER {
            // buddy_pfn = pfn XOR (1 << order)
            let buddy = current ^ (1 << current_order);

            if self.free_lists[current_order].remove(&buddy) {
                // Buddy is free → merge!
                println!("    MERGE: [{}, {}] + [{}, {}] → order {}",
                         current, current + (1 << current_order) - 1,
                         buddy,  buddy  + (1 << current_order) - 1,
                         current_order + 1);
                // The merged block starts at the lower of the two
                current = current.min(buddy);
                current_order += 1;
            } else {
                break; // Buddy is in use, can't merge further
            }
        }

        self.free_lists[current_order].insert(current);
        println!("    Result: block_start={} placed on order={}", current, current_order);
    }

    fn print_state(&self) {
        println!("  Buddy state:");
        for order in 0..=MAX_ORDER {
            if !self.free_lists[order].is_empty() {
                let mut blocks: Vec<_> = self.free_lists[order].iter().collect();
                blocks.sort();
                println!("    Order {} ({:>2} pages): {:?}", order, 1 << order, blocks);
            }
        }
    }
}

fn main() {
    println!("=== Buddy Allocator Simulation ({} pages) ===\n", TOTAL_PAGES);

    let mut buddy = BuddyAllocator::new(TOTAL_PAGES);

    println!("Initial state:");
    buddy.print_state();

    println!("\nAllocating 1 page (order 0):");
    let a = buddy.allocate(0).expect("alloc failed");
    buddy.print_state();

    println!("\nAllocating 1 page (order 0):");
    let b = buddy.allocate(0).expect("alloc failed");
    buddy.print_state();

    println!("\nAllocating 4 pages (order 2):");
    let c = buddy.allocate(2).expect("alloc failed");
    buddy.print_state();

    println!("\nFreeing first page (a={}):", a);
    buddy.free(a, 0);
    buddy.print_state();

    println!("\nFreeing second page (b={}):", b);
    buddy.free(b, 0);   // ← should coalesce with a!
    buddy.print_state();

    println!("\nFreeing 4-page block (c={}):", c);
    buddy.free(c, 2);   // ← should trigger chain of merges
    buddy.print_state();

    println!("\nFinal state — should be fully coalesced to order {}:", MAX_ORDER);
    buddy.print_state();
}
```

**Expected output:**
```
=== Buddy Allocator Simulation (16 pages) ===

Initial state:
  Buddy state:
    Order 4 (16 pages): [0]

Allocating 1 page (order 0):
  ALLOC: order=0, block_start=0, size=1 pages
  Buddy state:
    Order 0 ( 1 pages): [1]
    Order 1 ( 2 pages): [2]
    Order 2 ( 4 pages): [4]
    Order 3 ( 8 pages): [8]

Allocating 1 page (order 0):
  ALLOC: order=0, block_start=1, size=1 pages
  Buddy state:
    Order 1 ( 2 pages): [2]
    Order 2 ( 4 pages): [4]
    Order 3 ( 8 pages): [8]

...

Freeing second page (b=1):
  FREE:  order=0, block_start=1
    MERGE: [1, 1] + [0, 0] → order 1
    MERGE: [0, 1] + [2, 3] → order 2
    MERGE: [0, 3] + [4, 7] → order 3
    MERGE: [0, 7] + [8,15] → order 4
    Result: block_start=0 placed on order=4

Final state — should be fully coalesced to order 4:
  Buddy state:
    Order 4 (16 pages): [0]   ← fully defragmented!
```

---

## 13. Mental Models and Expert Intuition

### 13.1 The Hierarchy of Speed

Internalize this hierarchy. It is the foundation of all performance reasoning:

```
Allocation Speed (fastest to slowest):

1. Per-CPU variable access          ── ~0 ns  (direct register/memory)
   __this_cpu_read(var)

2. SLUB per-CPU freelist hit        ── ~5 ns  (no lock, no atomic)
   kmalloc() fast path

3. SLUB per-CPU partial refill      ── ~20 ns (cmpxchg to steal a slab)
   kmalloc() slow path

4. kmalloc() from new slab          ── ~50 ns (calls buddy system)
   buddy system allocation

5. vmalloc()                        ── ~200 ns (page table setup)
   Maps pages into vmalloc VA range

6. alloc_pages() with compaction    ── ~1 µs  (page migration)
   System is fragmented

7. GFP_KERNEL with direct reclaim   ── ~1 ms  (I/O, swapping)
   Low memory situation

8. OOM Killer invoked               ── seconds (process termination + recovery)
```

### 13.2 The Four Questions Before Every Allocation

Before writing any allocation call, ask:

1. **How big?** < 128 KiB → kmalloc. > 128 KiB → vmalloc/kvmalloc/alloc_pages.
2. **How often?** Same type, many times → slab cache. One-off → kmalloc.
3. **What context?** Interrupt/atomic → GFP_ATOMIC. Process → GFP_KERNEL.
4. **DMA needed?** Yes → kmalloc (physically contiguous). No → vmalloc ok.

### 13.3 The RAII Pattern in C Kernel Code

C has no destructors. The kernel uses the **goto err** pattern as a manual RAII:

```c
int my_function(void)
{
    int ret;
    void *a = NULL, *b = NULL;
    struct kmem_cache *cache = NULL;

    cache = kmem_cache_create(...);
    if (!cache) { ret = -ENOMEM; goto err; }

    a = kmalloc(...);
    if (!a) { ret = -ENOMEM; goto err_destroy_cache; }

    b = vmalloc(...);
    if (!b) { ret = -ENOMEM; goto err_free_a; }

    // ... actual work ...
    return 0;

    // Error paths in REVERSE allocation order:
err_free_a:
    kfree(a);
err_destroy_cache:
    kmem_cache_destroy(cache);
err:
    return ret;
}
```

Rust eliminates this pattern entirely with `?` and `Drop`. This is one of
the strongest arguments for Rust in kernel code.

### 13.4 The "Poison" Mental Model for Debugging

The kernel's memory debugging tools use **poisoning** — filling memory with
recognizable patterns:

```
Pattern       Meaning (when seen in a crash dump)
──────────────────────────────────────────────────────────────
0x6b6b6b6b   Freed slab object (POISON_FREE in SLUB debug)
             If you read this, you have a use-after-free bug.

0x5a5a5a5a   Uninitialized slab object (POISON_INUSE)
             If you read this, uninitialized read bug.

0xdeadbeef   Deliberate sentinel by kernel developers
             Often used to mark "this should never be reached".

0x0          kzalloc/kzalloc_node/get_zeroed_page output
             Safe — kernel explicitly zeroed this.
```

Enable kernel debugging: `CONFIG_SLUB_DEBUG=y`, `CONFIG_KASAN=y` (AddressSanitizer).

### 13.5 Cognitive Chunking Model

When reading a kernel allocator call like:

```c
ptr = kmem_cache_alloc_node(cache, GFP_KERNEL | __GFP_ZERO, node_id);
```

Chunk it as:

```
kmem_cache_alloc_node(
    cache,                        ← WHAT: which object type
    GFP_KERNEL | __GFP_ZERO,      ← HOW: process context, zeroed
    node_id                       ← WHERE: which NUMA node to prefer
);
```

Your brain should instantly parse: "This is a NUMA-aware slab allocation,
sleeping is OK, result is zeroed, for a specific object type."

---

## 14. API Quick Reference Table

```
┌──────────────────────┬───────────────────────────────┬──────────────────────┬───────────────────┬──────────────────────┐
│ Function             │ Returns                        │ Physically Cont?     │ Zeroed?           │ Freed With           │
├──────────────────────┼───────────────────────────────┼──────────────────────┼───────────────────┼──────────────────────┤
│ kmalloc(sz, gfp)     │ void * (kernel VA)             │ YES                  │ NO                │ kfree()              │
│ kzalloc(sz, gfp)     │ void * (kernel VA)             │ YES                  │ YES               │ kfree()              │
│ kcalloc(n, sz, gfp)  │ void * (kernel VA)             │ YES                  │ YES               │ kfree()              │
│ kmalloc_array(n,s,g) │ void * (kernel VA)             │ YES                  │ NO                │ kfree()              │
├──────────────────────┼───────────────────────────────┼──────────────────────┼───────────────────┼──────────────────────┤
│ vmalloc(sz)          │ void * (vmalloc VA)            │ NO (VA only)         │ NO                │ vfree()              │
│ vzalloc(sz)          │ void * (vmalloc VA)            │ NO (VA only)         │ YES               │ vfree()              │
├──────────────────────┼───────────────────────────────┼──────────────────────┼───────────────────┼──────────────────────┤
│ kvmalloc(sz, gfp)    │ void * (kmalloc or vmalloc VA) │ TRY (falls back)     │ NO                │ kvfree()             │
│ kvzalloc(sz, gfp)    │ void * (kmalloc or vmalloc VA) │ TRY (falls back)     │ YES               │ kvfree()             │
│ kvmalloc_array(n,s,g)│ void * (kmalloc or vmalloc VA) │ TRY (falls back)     │ NO                │ kvfree()             │
├──────────────────────┼───────────────────────────────┼──────────────────────┼───────────────────┼──────────────────────┤
│ alloc_pages(gfp, ord)│ struct page *                  │ YES (2^ord pages)    │ Optional (__GFP_ZERO)│ __free_pages()    │
│ __get_free_pages(g,o)│ unsigned long (VA)             │ YES (2^ord pages)    │ Optional (__GFP_ZERO)│ free_pages()      │
│ alloc_page(gfp)      │ struct page *                  │ YES (1 page)         │ Optional (__GFP_ZERO)│ __free_page()     │
│ __get_free_page(gfp) │ unsigned long (VA)             │ YES (1 page)         │ Optional (__GFP_ZERO)│ free_page()       │
│ get_zeroed_page(gfp) │ unsigned long (VA)             │ YES (1 page)         │ YES               │ free_page()          │
├──────────────────────┼───────────────────────────────┼──────────────────────┼───────────────────┼──────────────────────┤
│ kmem_cache_create()  │ struct kmem_cache *            │ N/A (creates cache)  │ N/A               │ kmem_cache_destroy() │
│ kmem_cache_alloc()   │ void * (object from cache)     │ YES (slab objects)   │ NO (ctor only)    │ kmem_cache_free()    │
│ kmem_cache_zalloc()  │ void * (object from cache)     │ YES (slab objects)   │ YES               │ kmem_cache_free()    │
├──────────────────────┼───────────────────────────────┼──────────────────────┼───────────────────┼──────────────────────┤
│ dma_alloc_coherent() │ void * (CPU VA) + dma_addr_t   │ YES (required)       │ YES               │ dma_free_coherent()  │
└──────────────────────┴───────────────────────────────┴──────────────────────┴───────────────────┴──────────────────────┘
```

### GFP Flag Quick Reference

```
┌─────────────────────┬──────────────────────────────────┬────────────────┬──────────────────────────────────┐
│ Flag                │ Can Sleep?                        │ Emergency Pool?│ Use Case                         │
├─────────────────────┼──────────────────────────────────┼────────────────┼──────────────────────────────────┤
│ GFP_KERNEL          │ YES                               │ NO             │ Standard process context alloc   │
│ GFP_ATOMIC          │ NO                                │ YES            │ IRQ handlers, spinlock held      │
│ GFP_NOWAIT          │ NO                                │ NO             │ Real-time, prefers fast failure  │
│ GFP_NOIO            │ YES (no I/O)                      │ NO             │ Storage driver I/O paths         │
│ GFP_NOFS            │ YES (no FS)                       │ NO             │ Filesystem code paths            │
│ GFP_USER            │ YES                               │ NO             │ Userspace memory allocations     │
│ GFP_DMA             │ NO (atomic)                       │ YES            │ 24-bit ISA DMA                   │
│ GFP_DMA32           │ YES                               │ NO             │ 32-bit PCI DMA on 64-bit system  │
│ GFP_HIGHUSER        │ YES                               │ NO             │ User pages, HIGHMEM allowed      │
└─────────────────────┴──────────────────────────────────┴────────────────┴──────────────────────────────────┘
```

### Useful Kernel Debug Interfaces

```bash
# View all active slab caches, object counts, memory usage:
cat /proc/slabinfo

# View memory zone information (watermarks, free pages per order):
cat /proc/zoneinfo

# View buddy system free page distribution:
cat /proc/buddyinfo

# View vmalloc mappings:
cat /proc/vmallocinfo

# View overall memory statistics:
cat /proc/meminfo

# View per-process memory maps:
cat /proc/<pid>/smaps

# Enable SLUB debugging for a specific cache:
# echo 1 > /sys/kernel/slab/<cache_name>/alloc_calls

# KASAN (Kernel Address Sanitizer) — enables in kernel config:
# CONFIG_KASAN=y (AddressSanitizer: detects UAF, OOB, etc.)
# CONFIG_KFENCE=y (Kernel Electric Fence: probabilistic, low overhead)
# CONFIG_SLUB_DEBUG=y (SLUB debug: poison, red zones, track)
```

---

## Summary: The Expert's Mental Stack

```
When you see a kernel allocation call, your mind should instantly map:

kmalloc(256, GFP_KERNEL)
  ↓
[SIZE: 256 bytes]
  → Routes to kmalloc-256 slab cache
  → O(1) per-CPU freelist lookup
  → If miss: SLUB slow path → new slab from buddy

[FLAG: GFP_KERNEL]
  → Process context (can sleep)
  → Will retry under memory pressure
  → Will reclaim pages if needed
  → NOT safe in IRQ context

[RESULT: physically contiguous]
  → Valid for DMA operations
  → In kernel direct-map VA region
  → Freed with kfree()

[COMPLEXITY: O(1) amortized]
  → Slab fast path: no locks, no system calls
  → Slab miss: O(log order) buddy split

[FAILURE MODES]
  → NULL return on OOM
  → Sleep for extended time under pressure
  → Trigger OOM killer as last resort
```

This complete mental stack — from hardware page frames through buddy free lists,
through SLUB per-CPU caches, to the high-level API — is what separates a kernel
engineer who can write correct code from one who can write **correct, efficient,
and robust** code under all conditions.

---

*Guide covers Linux kernel v5.x through v6.x. Always verify against the specific
kernel version you are targeting using `linux/mm.h`, `linux/slab.h`, and
`Documentation/core-api/memory-allocation.rst`.*
