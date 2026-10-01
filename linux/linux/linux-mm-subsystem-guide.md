# The Linux Memory Management (MM) Subsystem — A Complete Guide

> Scope: physical memory management, virtual memory, page tables, allocators
> (buddy, slab/slub, vmalloc), page cache, reclaim, swap, THP, NUMA, cgroups,
> OOM killer, and the emerging Rust-for-Linux MM bindings — with C and Rust
> code, and ASCII architecture diagrams.

---

## Table of Contents

1. [Big Picture: Why MM Is Shaped This Way](#1-big-picture)
2. [Physical Memory Representation](#2-physical-memory-representation)
3. [Nodes, Zones, and the Buddy Allocator](#3-nodes-zones-and-the-buddy-allocator)
4. [Boot-Time Memory Initialization](#4-boot-time-memory-initialization)
5. [Virtual Memory: Address Spaces and VMAs](#5-virtual-memory-address-spaces-and-vmas)
6. [Page Tables and Address Translation](#6-page-tables-and-address-translation)
7. [The Page Fault Path](#7-the-page-fault-path)
8. [mmap() Internals](#8-mmap-internals)
9. [Kernel Object Allocators: SLAB / SLUB / SLOB](#9-kernel-object-allocators-slabslubslob)
10. [kmalloc vs vmalloc vs kvmalloc](#10-kmalloc-vs-vmalloc-vs-kvmalloc)
11. [The Page Cache and Address Spaces](#11-the-page-cache-and-address-spaces)
12. [LRU Lists and Reclaim (kswapd, direct reclaim)](#12-lru-lists-and-reclaim)
13. [Swap Subsystem](#13-swap-subsystem)
14. [Transparent Huge Pages (THP)](#14-transparent-huge-pages-thp)
15. [The OOM Killer](#15-the-oom-killer)
16. [NUMA and NUMA Balancing](#16-numa-and-numa-balancing)
17. [Memory Control Groups (memcg)](#17-memory-control-groups-memcg)
18. [Rust for Linux: MM Bindings](#18-rust-for-linux-mm-bindings)
19. [Observability & Debugging](#19-observability--debugging)
20. [End-to-End Architecture Diagram](#20-end-to-end-architecture-diagram)
21. [Glossary](#21-glossary)

---

## 1. Big Picture

Linux MM has one job stated simply: **map physical RAM to virtual address
spaces, safely, efficiently, and under memory pressure, without the rest of
the kernel or userspace ever needing to know exactly where a byte physically
lives.**

Everything else — buddy allocator, slab, page cache, swap, THP, cgroups — is
an optimization or a policy layered on top of two foundational facts:

1. Physical memory is a flat array of fixed-size frames (**pages**, usually
   4 KiB on most architectures, though huge pages exist at 2 MiB/1 GiB on
   x86_64).
2. Virtual memory lets every process believe it owns the entire address
   space, via **page tables** that the MMU walks on every memory access.

```
                     ┌────────────────────────────────────────┐
                     │              USERSPACE                 │
                     │  process A        process B             │
                     │  0x0000..0x7fff   0x0000..0x7fff         │
                     └─────────┬───────────────┬────────────────┘
                               │ virtual addr  │ virtual addr
                               ▼               ▼
                     ┌────────────────────────────────────────┐
                     │         MMU  (walks page tables)         │
                     └─────────┬───────────────┬────────────────┘
                               ▼               ▼
                     ┌────────────────────────────────────────┐
                     │        PHYSICAL RAM  (page frames)        │
                     │  [pfn 0][pfn 1][pfn 2] ... [pfn N]        │
                     └────────────────────────────────────────┘
```

Design pressures that shape the code you'll read in `mm/`:

- **Speed**: page faults and allocations happen millions of times/sec;
  allocators must be mostly lock-free / per-CPU.
- **Fragmentation control**: buddy allocator + migration types keep large
  contiguous blocks available for huge pages / DMA.
- **NUMA-awareness**: allocate memory close to the CPU that will use it.
- **Reclaim under pressure**: when RAM is scarce, evict clean pages, swap out
  dirty anonymous pages, or kill something (OOM).
- **Isolation**: cgroups must be able to cap and account memory per
  container/workload.

---

## 2. Physical Memory Representation

### 2.1 `struct page` — the atom of physical memory

Every physical page frame (4 KiB unit) in the system has exactly one
`struct page` describing it, stored in a giant array called `mem_map` (or,
on sparse/discontiguous layouts, looked up via `vmemmap`).

```c
/* simplified from include/linux/mm_types.h */
struct page {
    unsigned long flags;        /* PG_locked, PG_dirty, PG_lru, PG_slab... */

    union {
        struct {                /* page cache / anonymous page */
            struct list_head lru;      /* LRU linkage for reclaim */
            struct address_space *mapping;
            pgoff_t index;              /* offset in mapping */
            unsigned long private;
        };
        struct {                /* slab allocator (SLUB) */
            struct kmem_cache *slab_cache;
            void *freelist;
            unsigned long counters;
        };
        struct {                /* compound / huge page tail */
            unsigned long compound_head;
        };
    };

    atomic_t _refcount;         /* reference count, page freed at 0 */
    atomic_t _mapcount;         /* how many PTEs map this page */

#ifdef CONFIG_NUMA
    int nid_or_something;       /* node this page belongs to (folded into flags) */
#endif
};
```

`struct page` is intentionally packed via unions because with millions of
pages on a large machine, **every extra byte costs megabytes** system-wide
(a 64 GiB machine has 16M pages at 4 KiB each → 16M `struct page`s).

Key fields to internalize:

| Field | Meaning |
|---|---|
| `flags` | Bitmap: `PG_locked`, `PG_uptodate`, `PG_dirty`, `PG_lru`, `PG_slab`, `PG_reserved`, `PG_swapbacked`, etc. Also encodes the **zone** and **node** in the high bits. |
| `_refcount` | Number of kernel references. Page is freed to the buddy allocator when it hits 0. |
| `_mapcount` | Number of page-table entries (PTEs) currently pointing at this page — used for reclaim decisions (shared vs private). |
| `lru` | List node linking the page into one of the per-zone/per-memcg LRU lists (active/inactive, anon/file). |
| `mapping` | If page-cache-backed, the `address_space` (inode/file) it belongs to; low bit set means "this is anonymous, points to `anon_vma`" instead. |

### 2.2 The Folio (post-5.16 rework)

Since Linux 5.16, `struct folio` was introduced to replace ad-hoc "compound
page" handling. A folio represents **one or more physically contiguous
pages treated as a single unit** (e.g., the page cache now allocates
folios, which can be larger than one page — improving batching, reducing
per-page overhead for large files).

```c
struct folio {
    /* first struct page fields, unioned identically */
    struct page page;
    /* ... folio-specific accessors wrap page[] internally */
};

/* Typical modern page-cache code (post-folio rework): */
struct folio *folio = filemap_alloc_folio(gfp, order);
folio_attach_private(folio, data);
folio_put(folio);           /* replaces put_page() in new code */
```

Why this mattered: before folios, a 2 MiB file-backed mapping was 512
separate `struct page` objects that code had to iterate and reason about
individually (is this a tail page? head page? compound?). A folio makes
"this is one memory object of N pages" an explicit, type-checked concept —
eliminating a whole class of head/tail page bugs.

### 2.3 PFN vs. virtual kernel address vs. `struct page`

Three ways to name the same physical page, with cheap conversions:

```
        pfn_to_page(pfn)              page_to_pfn(page)
  PFN  ───────────────────►  struct page  ───────────────────►  PFN
   │                                                              │
   │ __pa(virt) / __va(phys)                                     │
   ▼                                                              ▼
Physical addr  ◄─────────────────────────────────────────  kernel virtual addr
                     (only for the linear/direct map region)
```

- **PFN** (Page Frame Number) = `physical_address >> PAGE_SHIFT`.
- The kernel maps *all* physical RAM (on 64-bit) into a **direct/linear
  map** (`PAGE_OFFSET` + physical address = kernel virtual address), so
  `__va()`/`__pa()` are just arithmetic, no page-table walk needed.
- `struct page *` is found via `pfn_to_page()`, either direct array
  indexing (`mem_map[pfn]`, FLATMEM) or through `vmemmap` (SPARSEMEM,
  virtually-mapped memmap — the common case today because RAM has holes).

---

## 3. Nodes, Zones, and the Buddy Allocator

### 3.1 NUMA nodes

On a NUMA machine, RAM is partitioned into **nodes**, one per physical
memory controller/socket. `struct pglist_data` (aka `pg_data_t`) describes
one node.

```
Node 0 (CPU socket 0)              Node 1 (CPU socket 1)
┌───────────────────────┐          ┌───────────────────────┐
│ pg_data_t node_data[0] │          │ pg_data_t node_data[1] │
│  ├─ zone DMA           │          │  ├─ zone DMA           │
│  ├─ zone DMA32         │          │  ├─ zone DMA32         │
│  ├─ zone NORMAL        │          │  ├─ zone NORMAL        │
│  └─ zone MOVABLE       │          │  └─ zone MOVABLE       │
└───────────────────────┘          └───────────────────────┘
        │  local access is fast         │
        └──────── QPI/UPI/Infinity Fabric interconnect (slower) ────────┘
```

On a UMA (single-socket) box, there's just node 0 with the same structure.

### 3.2 Zones

Within a node, physical memory is split into **zones**, because different
hardware/software constraints apply to different physical ranges:

| Zone | Why it exists |
|---|---|
| `ZONE_DMA` | Legacy ISA DMA can only address the first 16 MiB. |
| `ZONE_DMA32` | Some 32-bit-only DMA devices can address up to 4 GiB. |
| `ZONE_NORMAL` | Directly mapped, no restriction — the workhorse zone. |
| `ZONE_HIGHMEM` | (32-bit only) RAM not permanently mapped into kernel VA space; needs `kmap()`. Irrelevant on 64-bit. |
| `ZONE_MOVABLE` | Pages here are guaranteed migratable — used for memory hot-unplug and to keep contiguous regions available for huge pages. |

```c
/* simplified include/linux/mmzone.h */
struct zone {
    unsigned long watermark[NR_WMARK];  /* WMARK_MIN/LOW/HIGH */
    long lowmem_reserve[MAX_NR_ZONES];
    struct pglist_data *zone_pgdat;
    struct per_cpu_pages __percpu *per_cpu_pageset;  /* per-CPU freelist cache */

    unsigned long zone_start_pfn;
    unsigned long spanned_pages;
    unsigned long present_pages;

    struct free_area free_area[MAX_ORDER];  /* the buddy allocator's heart */

    seqlock_t span_seqlock;
    unsigned long flags;
    spinlock_t lock;
};
```

### 3.3 The Buddy Allocator

The buddy allocator is the **base allocator for physical page frames**
(everything else — slab, vmalloc, page cache — ultimately calls down into
it). It organizes free memory as power-of-two-sized blocks ("orders"),
`MAX_ORDER` traditionally 11 (so up to 2¹⁰ = 1024 pages = 4 MiB per block on
x86_64 with 4 KiB pages; modern kernels have tuned this).

```
free_area[0]  -> list of free  1-page   (2^0)  blocks
free_area[1]  -> list of free  2-page   (2^1)  blocks
free_area[2]  -> list of free  4-page   (2^2)  blocks
...
free_area[10] -> list of free 1024-page (2^10) blocks
```

**Allocation** (`alloc_pages(gfp, order)`): search `free_area[order]`; if
empty, go to `order+1`, take a block, **split it in half**, keep one half,
put the other ("buddy") half back on `free_area[order]`.

**Freeing**: check if the block's *buddy* (the block that would merge with
it to reform the parent) is also free; if so, **merge** and repeat one
order up. This is where the name comes from — pages are freed together with
their "buddy."

```
Order 3 block (8 pages) split for an order-1 (2-page) allocation:

 [========= 8 pages, order 3 =========]
        split
 [==4 pages==][==4 pages==]                 order 2, one kept one queued
        split (on the kept half)
 [==2==][==2==][==4 pages==]                 order 1, order 1, order 2
    ^ allocated  ^ buddy, stays on free_area[1]
```

```c
/* mm/page_alloc.c (conceptually simplified) */
static inline void expand(struct zone *zone, struct page *page,
                           int low, int high, struct free_area *area)
{
    unsigned long size = 1 << high;
    while (high > low) {
        area--;
        high--;
        size >>= 1;
        list_add(&page[size].lru, &area->free_list[MIGRATE_MOVABLE]);
        area->nr_free++;
        set_page_order(&page[size], high);
    }
}

struct page *__rmqueue_smallest(struct zone *zone, unsigned int order,
                                 int migratetype)
{
    for (unsigned int current_order = order;
         current_order < MAX_ORDER; ++current_order) {
        struct free_area *area = &zone->free_area[current_order];
        struct page *page = list_first_entry_or_null(
                &area->free_list[migratetype], struct page, lru);
        if (!page)
            continue;
        list_del(&page->lru);
        rmv_page_order(page);
        area->nr_free--;
        expand(zone, page, order, current_order, area);
        return page;
    }
    return NULL;  /* zone exhausted at this order — triggers reclaim/compaction */
}
```

### 3.4 Migration types (fighting fragmentation)

Within each order's free list, blocks are further bucketed by
**migratetype** so that unmovable kernel allocations don't get scattered
throughout memory and prevent large contiguous regions (needed for THP,
hugetlbfs, hot-unplug) from ever forming:

- `MIGRATE_UNMOVABLE` — kernel data structures that can never move (most
  slab allocations, page tables).
- `MIGRATE_MOVABLE` — user pages, page cache — can be migrated by
  `mm/compaction.c` or `migrate_pages()`.
- `MIGRATE_RECLAIMABLE` — e.g., some slab caches — can be dropped and
  refetched rather than truly moved.
- `MIGRATE_CMA` — Contiguous Memory Allocator reserved regions.

### 3.5 Watermarks and kswapd wake-up

Each zone has three watermarks: `WMARK_MIN`, `WMARK_LOW`, `WMARK_HIGH`.

```
 free pages
     │
     │  ┌───────────────┐  above HIGH: healthy, no reclaim pressure
     │  ├───────────────┤  HIGH
     │  │  kswapd active │  between LOW and HIGH: kswapd reclaims asynchronously
     │  ├───────────────┤  LOW    <-- crossing this wakes kswapd
     │  │ direct reclaim │  between MIN and LOW: allocator itself may reclaim
     │  ├───────────────┤  MIN
     │  │   OOM territory│  below MIN: allocation fails → direct reclaim harder,
     │  └───────────────┘             eventually OOM killer
```

- Free pages drop below `WMARK_LOW` → `kswapd` (per-node kernel thread) is
  woken to reclaim asynchronously, in the background, without blocking the
  allocating process.
- Free pages drop below `WMARK_MIN` while an allocation is in flight → the
  allocating process itself performs **direct reclaim** synchronously
  (`try_to_free_pages()`), which is why allocations can suddenly get slow
  under pressure.

### 3.6 GFP flags — the "how" of every allocation

```c
struct page *p = alloc_pages(GFP_KERNEL, 0);          /* order 0, may sleep */
struct page *p2 = alloc_pages(GFP_ATOMIC, 0);          /* cannot sleep, for IRQ/softirq context */
struct page *p3 = alloc_pages(GFP_KERNEL | __GFP_ZERO, 2); /* 4 pages, zeroed */
struct page *p4 = alloc_pages(GFP_HIGHUSER_MOVABLE, 0);    /* for userspace, movable */
```

| Flag | Meaning |
|---|---|
| `GFP_KERNEL` | Normal kernel allocation, may sleep, may trigger reclaim. |
| `GFP_ATOMIC` | Cannot sleep (interrupt/softirq context); dips into emergency reserves. |
| `GFP_NOWAIT` | Similar to atomic but doesn't touch emergency reserves. |
| `GFP_NOFS` | May sleep/reclaim but must not re-enter the filesystem (used inside FS code to avoid deadlock). |
| `GFP_NOIO` | May sleep/reclaim but must not issue I/O. |
| `__GFP_ZERO` | Zero the returned memory. |
| `__GFP_HIGHMEM` | Allocation may come from `ZONE_HIGHMEM` (32-bit). |
| `__GFP_MOVABLE` | Hint that this allocation can be migrated later. |
| `__GFP_NOFAIL` | Never return NULL — keep retrying/reclaiming forever (dangerous, rare). |

---

## 4. Boot-Time Memory Initialization

Before the buddy allocator exists, the kernel still needs to allocate
memory (for page tables, `struct page` arrays, etc). This is `memblock`
("boot-time bootmem allocator's successor"):

```
Boot sequence (simplified):

 firmware (E820 / EFI memory map / devicetree)
        │
        ▼
 memblock_add()   -- register all usable physical RAM ranges
        │
        ▼
 memblock_reserve() -- carve out kernel image, initrd, firmware tables
        │
        ▼
 memblock_alloc()  -- early allocations (page tables, struct page[])
        │
        ▼
 paging_init() / zone_sizes_init() -- set up zones per node
        │
        ▼
 mem_init()  -- hand remaining free memblock regions to the BUDDY allocator
        │
        ▼
 buddy allocator live; memblock APIs still usable until "memblock" is torn down
        │
        ▼
 kmem_cache_init() -- bring up SLAB/SLUB on top of the buddy allocator
        │
        ▼
 vmalloc_init(), page cache init, etc.
```

```c
/* Roughly what happens in mm/memblock.c consumers */
void __init setup_arch_memory(void)
{
    memblock_add(0x0, 0x40000000);              /* found 1 GiB of RAM at 0 */
    memblock_reserve(kernel_start, kernel_size); /* don't hand out the kernel image */
    /* ... */
    void *pgtbl = memblock_alloc(PAGE_SIZE, PAGE_SIZE); /* early page table page */
}
```

`memblock` is deliberately dumb (a simple sorted array of regions) because
at this point in boot there is no slab allocator, no locking
infrastructure, sometimes not even full page tables yet.

---

## 5. Virtual Memory: Address Spaces and VMAs

### 5.1 `mm_struct` — one per process address space

```c
/* simplified include/linux/mm_types.h */
struct mm_struct {
    struct maple_tree mm_mt;        /* VMAs, indexed by address range (since 6.1) */
    unsigned long mmap_base;        /* base of the mmap region */
    unsigned long task_size;        /* size of address space */
    pgd_t *pgd;                     /* root of the page table tree */

    atomic_t mm_users;              /* # of threads/tasks sharing this mm */
    atomic_t mm_count;              /* # of "lazy" references (kernel threads) */

    unsigned long total_vm;         /* total pages mapped */
    unsigned long locked_vm;        /* pages locked (mlock) */
    unsigned long pinned_vm;        /* pages pinned (get_user_pages) */

    struct rw_semaphore mmap_lock;  /* protects the VMA tree */

    struct list_head mmlist;
};
```

**Note:** Prior to Linux 6.1, VMAs were stored in a red-black tree
(`rb_root`) plus a linked list. As of 6.1 the kernel switched to a
**maple tree** (`struct maple_tree`), an RCU-safe B-tree variant designed
specifically for this workload — it supports lock-free, range-based lookups
under RCU which enabled a huge follow-on project: **per-VMA locking**
(taking a fine-grained lock on a single VMA instead of the whole-mm
`mmap_lock` for page faults), which significantly reduces contention on
highly-threaded processes.

### 5.2 `vm_area_struct` (VMA) — one per contiguous mapped region

```c
struct vm_area_struct {
    unsigned long vm_start, vm_end;    /* [start, end) virtual address range */
    struct mm_struct *vm_mm;
    pgprot_t vm_page_prot;             /* RWX + cache attrs, arch-encoded */
    unsigned long vm_flags;            /* VM_READ, VM_WRITE, VM_EXEC, VM_SHARED... */

    struct file *vm_file;              /* non-NULL for file-backed mappings */
    unsigned long vm_pgoff;            /* offset into vm_file, in PAGE_SIZE units */

    const struct vm_operations_struct *vm_ops; /* fault(), open(), close() callbacks */
    struct anon_vma *anon_vma;         /* reverse mapping for anonymous memory */
};
```

A process's address space is a set of non-overlapping VMAs:

```
0x0000000000400000  ┌────────────┐
                     │  .text     │  r-xp   file-backed (the binary)
0x0000000000401000  ├────────────┤
                     │  .data/.bss│  rw-p   file-backed / anon
0x0000000000600000  ├────────────┤
                     │   heap     │  rw-p   anonymous, grows via brk()/mmap()
                     │     ↓      │
                     ├────────────┤
                     │  (unmapped)│
                     ├────────────┤
                     │     ↑      │
                     │   mmap     │  shared libs, mmap()'d files, anon mmap
                     │   region   │
0x00007ffc00000000  ├────────────┤
                     │   stack    │  rw-p   anonymous, grows down
                     │     ↓      │
0x00007fffffffffff  └────────────┘
```

`cat /proc/<pid>/maps` shows exactly this list.

### 5.3 Reverse mapping (rmap)

Given a physical page, the kernel frequently needs to answer: **"which
page tables map this page?"** — essential for reclaim (must unmap before
freeing/swapping) and for `fork()`'s copy-on-write bookkeeping.

- **File-backed pages**: walk the `address_space`'s interval tree of VMAs
  that map that file+offset (`i_mmap` tree).
- **Anonymous pages**: each anonymous VMA has an `anon_vma`, and anonymous
  pages point back to it via `page->mapping` (with the low bit set as a
  tag distinguishing "this is an anon_vma pointer, not an address_space").

```
 struct page (anon)                     struct anon_vma
 ┌────────────────┐   page->mapping    ┌───────────────────┐
 │ mapping (tagged)│ ─────────────────► │ root anon_vma       │
 │ index            │                    │ rb_root of anon_vma_chain│
 └────────────────┘                    └──────────┬─────────┘
                                                     │ (fork creates a child anon_vma
                                                     │  chained to the parent — this is
                                                     ▼  how COW-shared pages track *all*
                                          VMA in process B         processes mapping them)
```

---

## 6. Page Tables and Address Translation

### 6.1 x86_64: a 4-level (or 5-level) radix tree

```
Virtual Address (48-bit canonical, 4-level paging):

 63        48 47      39 38      30 29      21 20      12 11         0
┌────────────┬──────────┬──────────┬──────────┬──────────┬───────────┐
│ sign-extend │  PGD idx │  PUD idx │  PMD idx │  PTE idx │  offset    │
│  (16 bits)  │ (9 bits) │ (9 bits) │ (9 bits) │ (9 bits) │ (12 bits)  │
└────────────┴──────────┴──────────┴──────────┴──────────┴───────────┘
                   │            │           │          │
                   ▼            ▼           ▼          ▼
                 PGD    →     PUD    →    PMD    →    PTE   →  physical page + offset
              (pgd_t*)     (pud_t*)    (pmd_t*)    (pte_t*)
              512 entries  512 entries 512 entries 512 entries
              each covers  each covers each covers each maps
              512 GiB      1 GiB       2 MiB       4 KiB
```

Kernel names map 1:1 to hardware levels:

| Level | Kernel type | Covers | Linux name |
|---|---|---|---|
| 4 | `pgd_t` | 512 GiB/entry | Page Global Directory |
| 3 | `pud_t` | 1 GiB/entry | Page Upper Directory |
| 2 | `pmd_t` | 2 MiB/entry | Page Middle Directory |
| 1 | `pte_t` | 4 KiB/entry | Page Table Entry |

With **5-level paging** (`CONFIG_X86_5LEVEL`, needed for >128 TiB address
spaces), a `p4d_t` level is inserted between PGD and PUD. On kernels that
support both, the code uses a folding trick: if 5-level isn't active,
`p4d_t` operations become no-ops that pass straight through — the *generic*
MM code is always written against the full 5-level API, letting the
architecture "fold" unused levels away at compile/runtime.

```
5-level: PGD → P4D → PUD → PMD → PTE   (57-bit virtual addresses, 128 PiB)
4-level: PGD → PUD → PMD → PTE          (48-bit virtual addresses, 256 TiB)
```

### 6.2 A PTE, bit by bit (x86_64)

```
 63  62        52 51                  12 11  9 8 7 6 5 4 3 2 1 0
┌───┬────────────┬──────────────────────┬──────┬─┬─┬─┬─┬─┬─┬─┬─┬───┐
│NX │  (avail)   │   Physical Page PFN   │avail │G│PS│D│A│PCD│PWT│U/S│R/W│P│
└───┴────────────┴──────────────────────┴──────┴─┴─┴─┴─┴─┴─┴─┴─┴───┘
```

| Bit | Name | Meaning |
|---|---|---|
| 0 | Present (P) | 1 = valid mapping. If 0, the rest of the entry is free for the OS to stash info (e.g., swap PTE encodes a swap slot here). |
| 1 | Read/Write | 1 = writable. |
| 2 | User/Supervisor | 1 = accessible from ring 3 (userspace). |
| 5 | Accessed (A) | Set by hardware on access; cleared by the kernel to implement LRU-ish aging without extra instructions. |
| 6 | Dirty (D) | Set by hardware on write; the kernel checks this to know if a page must be written back before reclaim. |
| 63 | NX | No-eXecute — enforced by the CPU, critical for W^X security policies. |

The **Accessed** and **Dirty** bits are the hardware-assisted foundation of
Linux's page reclaim/LRU heuristics — the kernel doesn't need a trap on
every memory access to know "was this touched recently," it just
periodically scans and clears the Accessed bit (this is literally what
"the clock algorithm" / Linux's LRU approximation does).

### 6.3 TLB and its invalidation cost

The MMU caches translations in the **Translation Lookaside Buffer (TLB)**.
Every time the kernel changes a PTE that might be cached, it must issue a
**TLB shootdown** (`flush_tlb_*()`), which on multi-core systems means an
IPI (inter-processor interrupt) to every CPU that might have that mapping
cached — a genuinely expensive operation, which is why the kernel batches
unmap operations (`tlb_gather_mmu()` / `tlb_finish_mmu()`) instead of
flushing per-PTE.

```c
/* mm/mmu_gather.c pattern used throughout unmap paths */
struct mmu_gather tlb;
tlb_gather_mmu(&tlb, mm);
unmap_page_range(&tlb, vma, start, end, NULL);  /* batch up PTEs to clear */
tlb_finish_mmu(&tlb);                            /* ONE flush for the whole batch */
```

### 6.4 Walking a page table by hand (C)

```c
/* Simplified illustrative version of what handle_mm_fault() / follow_page()
 * does internally to translate a virtual address, generic 4-level. */
static struct page *walk_page_table(struct mm_struct *mm, unsigned long addr)
{
    pgd_t *pgd = pgd_offset(mm, addr);
    if (pgd_none(*pgd) || pgd_bad(*pgd))
        return NULL;

    p4d_t *p4d = p4d_offset(pgd, addr);
    if (p4d_none(*p4d) || p4d_bad(*p4d))
        return NULL;

    pud_t *pud = pud_offset(p4d, addr);
    if (pud_none(*pud) || pud_bad(*pud))
        return NULL;
    if (pud_large(*pud))                       /* 1 GiB huge page */
        return pud_page(*pud) + ((addr >> PAGE_SHIFT) & (PTRS_PER_PUD_PAGE - 1));

    pmd_t *pmd = pmd_offset(pud, addr);
    if (pmd_none(*pmd) || pmd_bad(*pmd))
        return NULL;
    if (pmd_large(*pmd))                       /* 2 MiB huge page (THP) */
        return pmd_page(*pmd) + ((addr >> PAGE_SHIFT) & (PTRS_PER_PMD_PAGE - 1));

    pte_t *pte = pte_offset_map(pmd, addr);
    if (!pte || !pte_present(*pte)) {
        if (pte) pte_unmap(pte);
        return NULL;
    }

    struct page *page = pte_page(*pte);
    pte_unmap(pte);
    return page;
}
```

---

## 7. The Page Fault Path

A page fault is **not** always an error — it's the primary mechanism by
which Linux implements lazy allocation, copy-on-write, demand paging, and
swap-in.

```
                     CPU accesses a virtual address
                                │
                       MMU walk fails (not present,
                       protection violation, etc.)
                                │
                                ▼
                  #PF exception → do_page_fault() (arch/x86/mm/fault.c)
                                │
                                ▼
                  find_vma(mm, address)  -- which VMA (if any) covers this addr?
                                │
                 ┌──────────────┼───────────────────┐
                 │ no VMA covers│                    │ VMA found
                 ▼ this address                      ▼
          SIGSEGV (real segfault)         handle_mm_fault(vma, address, flags)
                                                       │
                     ┌─────────────────────────────────┼─────────────────────────┐
                     ▼                                 ▼                         ▼
              PTE not present                  PTE present, write         PTE present, write
              (never touched)                  fault, page is             fault, COW needed
                     │                          already writable          (refcount > 1,
                     ▼                          (shouldn't normally        e.g. after fork)
        anonymous?  ┌──────────┐               happen — stale TLB)              │
       ┌─────┴─────┐│ file-backed?             │                                ▼
       ▼            │└──────────┘        flush_tlb, retry                do_wp_page():
 do_anonymous_page()▼                                                    alloc new page,
 alloc zeroed page,  filemap_fault():                                    copy contents,
 map PTE             read page from page cache                          remap PTE writable,
                     (or issue readahead I/O                            drop refcount on
                      if not cached), map PTE                           original page
```

```c
/* Simplified core of handle_pte_fault() (mm/memory.c) */
static vm_fault_t handle_pte_fault(struct vm_fault *vmf)
{
    if (!vmf->pte) {
        if (vma_is_anonymous(vmf->vma))
            return do_anonymous_page(vmf);
        else
            return do_fault(vmf);          /* file-backed: ->fault() callback */
    }

    if (!pte_present(vmf->orig_pte))
        return do_swap_page(vmf);          /* PTE encodes a swap slot */

    if (vmf->flags & FAULT_FLAG_WRITE) {
        if (!pte_write(vmf->orig_pte))
            return do_wp_page(vmf);        /* copy-on-write */
    }

    /* Just needed to update Accessed/Dirty bits and TLB. */
    entry = pte_mkyoung(vmf->orig_pte);
    ptep_set_access_flags(vmf->vma, vmf->address, vmf->pte, entry,
                            vmf->flags & FAULT_FLAG_WRITE);
    return 0;
}
```

### 7.1 Copy-on-Write (COW) in detail

`fork()` doesn't copy memory. It copies **page tables**, marks every
private-writable PTE read-only in *both* parent and child, and increments
`_refcount`/`_mapcount` on the underlying physical pages. The actual copy
happens lazily, only for pages either process later writes to:

```
Before fork():                     After fork(), before any write:

 Parent PTE ──► Page X (refcount=1) Parent PTE (RO) ──┐
                                                       ├──► Page X (refcount=2)
                                     Child PTE (RO) ───┘

After child writes to Page X:

 Parent PTE (RO) ──► Page X (refcount=1)
 Child PTE (RW)  ──► Page X' (new copy, refcount=1)
```

```c
/* Simplified do_wp_page() */
static vm_fault_t do_wp_page(struct vm_fault *vmf)
{
    struct page *old_page = vm_normal_page(vmf->vma, vmf->address, vmf->orig_pte);

    if (page_mapcount(old_page) == 1) {
        /* We're the only mapper -- no need to copy, just make writable */
        wp_page_reuse(vmf);
        return 0;
    }

    /* Shared: allocate a fresh page and copy */
    struct page *new_page = alloc_page_vma(GFP_HIGHUSER_MOVABLE, vmf->vma, vmf->address);
    copy_user_highpage(new_page, old_page, vmf->address, vmf->vma);

    entry = mk_pte(new_page, vmf->vma->vm_page_prot);
    entry = maybe_mkwrite(pte_mkdirty(entry), vmf->vma);
    set_pte_at(vmf->vma->vm_mm, vmf->address, vmf->pte, entry);
    page_add_new_anon_rmap(new_page, vmf->vma, vmf->address);

    put_page(old_page);       /* drop our reference */
    return 0;
}
```

---

## 8. mmap() Internals

```
 userspace: mmap(NULL, len, PROT_READ|PROT_WRITE, MAP_PRIVATE|MAP_ANONYMOUS, -1, 0)
                     │
                     ▼ syscall
              sys_mmap → ksys_mmap_pgoff → vm_mmap_pgoff → do_mmap()
                     │
                     ▼
        get_unmapped_area()   -- find a free virtual address range
                     │
                     ▼
        mmap_region()
          ├─ allocate & initialize vm_area_struct
          ├─ if file-backed: vma->vm_file = file; call f_op->mmap()
          ├─ insert VMA into mm->mm_mt (maple tree)
          └─ NOTE: no physical pages allocated yet, no PTEs installed!
                     │
                     ▼
           returns virtual address to userspace
                     │
        (later, on first access) ──► PAGE FAULT ──► do_anonymous_page()/filemap_fault()
                                       actually backs the memory
```

This laziness is the point: `mmap(MAP_ANONYMOUS, 1 GiB)` returns instantly
and costs nothing until pages are actually touched — Linux **overcommits**
virtual address space aggressively (tunable via
`/proc/sys/vm/overcommit_memory`).

```c
/* User-facing example: mapping a file, private, copy-on-write */
#include <sys/mman.h>
#include <fcntl.h>
#include <unistd.h>

int fd = open("data.bin", O_RDONLY);
size_t len = 4096 * 100;
void *p = mmap(NULL, len, PROT_READ, MAP_PRIVATE, fd, 0);
if (p == MAP_FAILED) { perror("mmap"); return 1; }

/* Touching p[0] here triggers filemap_fault() -> page cache lookup/readahead */
volatile char c = ((char *)p)[0];

munmap(p, len);
close(fd);
```

---

## 9. Kernel Object Allocators: SLAB / SLUB / SLOB

The buddy allocator only hands out whole pages (4 KiB+). Kernel code
constantly needs small, fixed-size objects (a `struct task_struct`, an
`inode`, a `dentry`, a network `skbuff`) — allocating a full page per
64-byte object would waste massively and be slow. **Slab allocators** sit on
top of the buddy allocator, carving pages into pools of same-sized objects.

```
                     alloc_pages() / __get_free_pages()
                                 │
                                 ▼
                 ┌───────────────────────────────┐
                 │        BUDDY ALLOCATOR          │
                 └───────────────┬───────────────┘
                                 │  hands out whole pages ("slabs")
                                 ▼
                 ┌───────────────────────────────┐
                 │     SLUB / SLAB / SLOB           │  <- object allocators
                 └───────────────┬───────────────┘
                                 │  hands out fixed-size objects
              ┌──────────────────┼───────────────────┐
              ▼                  ▼                    ▼
      kmem_cache "task_struct" kmem_cache "inode"  kmalloc-64, kmalloc-128, ...
```

### 9.1 SLUB (the default since ~2.6.23, "Unqueued Slab Allocator")

SLUB is what's actually compiled into virtually every modern distro kernel.
Its design goals: minimize per-CPU queue bookkeeping (unlike old SLAB),
minimize metadata overhead, and be simple enough to reason about.

```c
/* include/linux/slub_def.h, simplified */
struct kmem_cache {
    struct kmem_cache_cpu __percpu *cpu_slab;   /* per-CPU active slab + freelist */
    unsigned long flags;
    int size;                 /* object size including padding/red-zoning */
    int object_size;          /* real object size requested */
    int offset;                /* where the "next free object" pointer lives */
    struct kmem_cache_node *node[MAX_NUMNODES]; /* per-node partial-slab lists */
    const char *name;
    struct list_head list;    /* global list of all kmem_caches */
};

struct kmem_cache_cpu {
    void **freelist;           /* singly-linked list of free objects, lock-free */
    struct page *page;         /* the slab page currently being carved from */
    unsigned long tid;         /* transaction ID, for lock-free CAS-based fast path */
};
```

**Fast path allocation** (the common case) is genuinely lock-free — it's a
`this_cpu_cmpxchg` on the per-CPU freelist pointer:

```c
/* Simplified kmem_cache_alloc() fast path — mm/slub.c */
static __always_inline void *slab_alloc(struct kmem_cache *s, gfp_t gfp)
{
redo:
    struct kmem_cache_cpu *c = raw_cpu_ptr(s->cpu_slab);
    void *object = c->freelist;
    unsigned long tid = c->tid;

    if (unlikely(!object || !node_match(c->page, numa_node_id())))
        return __slab_alloc(s, gfp, NUMA_NO_NODE); /* slow path: get new slab */

    void *next_object = get_freepointer_safe(s, object);

    /* Lock-free swap: if nobody else touched this CPU's freelist since we
     * read it (tid unchanged), install next_object as the new head. */
    if (unlikely(!this_cpu_cmpxchg_double(
            s->cpu_slab->freelist, s->cpu_slab->tid,
            object, tid,
            next_object, next_tid(tid)))) {
        goto redo;
    }

    return object;
}
```

Each free object, while unallocated, stores a pointer to the *next* free
object right inside its own memory — a classic **intrusive free list**, so
the allocator needs zero extra metadata memory per free object.

```
Slab page carved into fixed-size objects (freelist threaded through them):

┌────────┬────────┬────────┬────────┬────────┬────────┐
│ obj 0   │ obj 1   │ obj 2   │ obj 3   │ obj 4   │ obj 5   │
│ [next]──┼───────► │ [next]──┼───────► │  (used)  │ [next]──┼─► NULL
│  (free) │ (used)  │  (free) │ (used)  │          │  (free) │
└────────┴────────┴────────┴────────┴────────┴────────┘
     ▲
 c->freelist (per-CPU head pointer)
```

### 9.2 `kmem_cache_create()` — defining your own cache

```c
/* Typical driver/subsystem pattern for a dedicated object cache */
static struct kmem_cache *my_request_cache;

struct my_request {
    struct list_head list;
    u64 id;
    char payload[256];
};

static int __init my_module_init(void)
{
    my_request_cache = kmem_cache_create(
        "my_request",
        sizeof(struct my_request),
        0,                              /* align: 0 = use default */
        SLAB_HWCACHE_ALIGN | SLAB_PANIC, /* flags */
        NULL);                          /* constructor, optional */
    if (!my_request_cache)
        return -ENOMEM;
    return 0;
}

static struct my_request *alloc_request(void)
{
    return kmem_cache_alloc(my_request_cache, GFP_KERNEL);
}

static void free_request(struct my_request *req)
{
    kmem_cache_free(my_request_cache, req);
}

static void __exit my_module_exit(void)
{
    kmem_cache_destroy(my_request_cache);
}
```

### 9.3 SLAB vs SLUB vs SLOB (historical, for context)

| Allocator | Status | Design |
|---|---|---|
| **SLAB** | Legacy, removed in Linux 6.5+ | Per-CPU *queues/arrays* of objects (like Solaris's original slab paper), complex, good for tiny embedded caches historically but more overhead/complexity than SLUB warranted. |
| **SLUB** | Default today | Simpler metadata, per-CPU single active slab + lock-free freelist, better cache-line behavior, debugging support (poisoning, red zones) built in. |
| **SLOB** | Removed in Linux 6.4 | Simple List Of Blocks — a first-fit allocator for tiny embedded systems where the RAM cost of any per-object metadata was unacceptable. |

### 9.4 Debugging features

```
# Build with CONFIG_SLUB_DEBUG, then boot with:
slub_debug=FZPU

  F = sanity checks (Fault injection points)
  Z = red zoning (detect buffer overrun writes just past the object)
  P = poisoning (fill freed memory with 0x6b so use-after-free reads look wrong)
  U = user tracking (record the alloc/free call stack per object)
```

```bash
# Inspect live caches
$ sudo slabtop
  OBJS ACTIVE  USE OBJ SIZE  SLABS OBJ/SLAB CACHE SIZE NAME
 45200  44800  99%    0.19K   1130       40       4520K dentry
 38912  38912 100%    1.00K   1216       32     38912K task_struct
```

---

## 10. kmalloc vs vmalloc vs kvmalloc

| API | Backing | Physically contiguous? | Virtually contiguous? | Typical use |
|---|---|---|---|---|
| `kmalloc(size, gfp)` | SLUB `kmalloc-N` caches (power-of-2 sizes) | **Yes** | Yes | Small objects, DMA buffers, anything a device might DMA into. |
| `vmalloc(size)` | Individual pages from the buddy allocator, stitched together via a dedicated page table region | **No** | **Yes** | Large allocations where physical contiguity isn't needed (e.g., module loading, large driver buffers). |
| `kvmalloc(size, gfp)` | Tries `kmalloc` first, falls back to `vmalloc` if the allocation is large / physically-contiguous alloc fails | Sometimes | Yes | General-purpose "just give me memory, I don't care how" for large-ish sizes. |

```c
/* kmalloc: fast, physically contiguous, size-class rounded up */
void *buf = kmalloc(1024, GFP_KERNEL);     /* served by kmalloc-1024 cache */
kfree(buf);

/* vmalloc: for big allocations (e.g., several MB) where physical
 * contiguity isn't required — costs a page-table setup + TLB entries */
void *big = vmalloc(16 * 1024 * 1024);      /* 16 MiB, NOT physically contiguous */
vfree(big);

/* kvmalloc: the modern "don't make me think about it" API */
void *data = kvmalloc(size, GFP_KERNEL);
kvfree(data);
```

### 10.1 Why `vmalloc` is slower

```
kmalloc: one alloc_pages() call, contiguous, mapped via the existing
         direct/linear map — ZERO extra page table work.

vmalloc: 1. Reserve a range in the dedicated vmalloc VA region
            (separate from the linear map: VMALLOC_START..VMALLOC_END)
         2. Allocate N individual (non-contiguous) physical pages
         3. Explicitly build PTEs mapping each page into the reserved
            virtual range  <-- extra work + extra TLB pressure
         4. Access pattern: same as any virtual address, but TLB misses
            are more likely (many random physical pages, not the huge-page-
            backed linear map)
```

```
Virtual address space layout (x86_64, illustrative, not to scale):

0xffff888000000000 ─┬─ Direct/linear map (all physical RAM, 1:1 offset)
                      │      __va(pa) = pa + PAGE_OFFSET, no page walk needed
                      │      for the mapping to exist -- it's prebuilt at boot
0xffffc90000000000 ─┼─ vmalloc area  (VMALLOC_START..VMALLOC_END)
                      │      individually mapped, non-contiguous physical backing
0xffffea0000000000 ─┼─ vmemmap (struct page array, virtually mapped)
0xffffffff80000000 ─┴─ kernel text/data (the kernel image itself)
```

### 10.2 `kmalloc` size classes

```c
/* Rough list of kmalloc caches SLUB creates at boot (mm/slab_common.c) */
kmalloc-8,  kmalloc-16,  kmalloc-32,  kmalloc-64,  kmalloc-96,
kmalloc-128, kmalloc-192, kmalloc-256, kmalloc-512, kmalloc-1k,
kmalloc-2k, kmalloc-4k, kmalloc-8k, ... up to kmalloc-order-N for large sizes
```

`kmalloc(50, GFP_KERNEL)` gets rounded up and served from `kmalloc-64` —
meaning **kmalloc always has internal fragmentation** up to the next power
of two (this is a deliberate speed/simplicity tradeoff vs. a general-purpose
`malloc`-style allocator that tries to minimize waste).

---

## 11. The Page Cache and Address Spaces

The page cache is the layer that caches file contents (and, since Linux
2.6, unifies with how anonymous/swap pages are handled) in RAM, keyed by
`(struct address_space *, pgoff_t)`.

```c
/* simplified include/linux/fs.h */
struct address_space {
    struct inode *host;
    struct xarray i_pages;        /* the actual cache: radix/xarray of folios,
                                      keyed by page offset within the file */
    unsigned long nrpages;
    const struct address_space_operations *a_ops;  /* ->readpage, ->writepage... */
    struct rb_root_cached i_mmap; /* interval tree of VMAs mapping this file */
};
```

```
             read()/write() syscalls              mmap()'d access (page fault)
                     │                                        │
                     ▼                                        ▼
              generic_file_read_iter()                 filemap_fault()
                     │                                        │
                     └───────────────┬────────────────────────┘
                                     ▼
                     address_space->i_pages  (xarray keyed by pgoff_t)
                     ┌───┬───┬───┬───┬───┬───┬───┬───┐
                     │ 0 │ 1 │ 2 │ - │ 4 │ - │ - │ 7 │   (- = not cached)
                     └───┴───┴───┴───┴───┴───┴───┴───┘
                        │               │  cache miss
                        │ cache hit     ▼
                        │        ->readpage()/->readahead()
                        │        issues real I/O to the block device,
                        │        inserts new folio into i_pages
                        ▼
                 copy to userspace / map into VMA
```

### 11.1 Readahead

The kernel predicts sequential access patterns and prefetches:

```c
/* Conceptual: mm/readahead.c decides how much to prefetch based on
 * observed access pattern (sequential vs random) */
void page_cache_sync_readahead(struct address_space *mapping,
                                 struct file_ra_state *ra,
                                 struct file *filp,
                                 pgoff_t offset, unsigned long req_size)
{
    if (!ra->ra_pages)
        return;

    if (blk_migration_or_random_pattern_detected(ra, offset)) {
        ra->size = req_size;             /* conservative: just what was asked */
    } else {
        ra->size = min(ra->size * 2, ra->ra_pages); /* ramp up window */
        ra->async_size = ra->size / 4;    /* trigger next readahead early */
    }
    do_page_cache_ra(filp, ra, offset, ra->size);
}
```

### 11.2 Writeback

Dirty page-cache pages (written via `write()` or a writable `mmap`) aren't
flushed to disk synchronously — they're marked `PG_dirty` and written back
later by per-device flusher threads (`kworker/flush-<dev>`), governed by
`/proc/sys/vm/dirty_ratio` and `dirty_background_ratio`.

```
   write() syscall                  time passes / memory pressure /
        │                            dirty_expire_centisecs elapses
        ▼                                        │
  mark folio dirty                                ▼
  (PG_dirty set,                         writeback kernel thread
   added to dirty                        (wb_workfn -> do_writepages)
   list, NOT written                              │
   to disk yet)                                    ▼
                                          ->writepage()/->writepages()
                                          issues real block I/O,
                                          clears PG_dirty on completion
```

---

## 12. LRU Lists and Reclaim

### 12.1 The multi-generational-ish LRU (classic 5-list model)

Each memory zone (in practice, each memcg × node) maintains up to 5 LRU
lists:

```
       LRU_INACTIVE_ANON   LRU_ACTIVE_ANON     (anonymous / heap / stack pages)
       LRU_INACTIVE_FILE   LRU_ACTIVE_FILE     (page-cache / file-backed pages)
       LRU_UNEVICTABLE                          (mlocked, ramfs, etc — never reclaimed)
```

```
   new page ──► tail of INACTIVE list
                        │
                referenced again (Accessed bit set, or explicit mark)?
                        │ yes                          │ no, and reclaim scans it
                        ▼                                ▼
                promoted to ACTIVE list           reclaimed: write back if dirty
                (head)                            (for file) or swap out (for anon),
                                                    then freed to the buddy allocator
        ┌───────────────────────┐
        │  ACTIVE   │ (recently used, protected from reclaim)
        ├───────────────────────┤
        │  INACTIVE │ (reclaim candidates, scanned first)
        └───────────────────────┘
        page demoted from ACTIVE → INACTIVE if not referenced for a while,
        this is Linux's approximation of "true" LRU (which would be too
        expensive to maintain exactly at this scale)
```

Since Linux 6.1, an optional **Multi-Generational LRU (MGLRU)** replaces
this simple active/inactive split with several numbered generations,
giving reclaim a much more precise picture of "how recently was this
touched" at low overhead — enable via
`echo y > /sys/kernel/mm/lru_gen/enabled`. Conceptually it's the same
"protect what's hot, evict what's cold" idea, generalized from 2 buckets to
N.

### 12.2 kswapd — background reclaim

```c
/* Simplified mm/vmscan.c logic */
static int kswapd(void *p)
{
    pg_data_t *pgdat = (pg_data_t *)p;

    for (;;) {
        wait_event_freezable(pgdat->kswapd_wait,
                              kswapd_should_run(pgdat));

        balance_pgdat(pgdat);   /* the real work: reclaim until watermarks OK */
    }
}

static void balance_pgdat(pg_data_t *pgdat)
{
    for (int order = pgdat->kswapd_max_order; order >= 0; order--) {
        for_each_zone_in_node(pgdat, zone) {
            if (zone_watermark_ok(zone, order, high_wmark_pages(zone)))
                continue; /* this zone is fine */

            shrink_node(pgdat, &sc);  /* reclaim pages from LRU lists */
        }
    }
}
```

### 12.3 Direct reclaim — the unlucky synchronous path

If an allocation can't be satisfied and `kswapd` hasn't kept up, the
allocating process itself reclaims:

```c
/* mm/page_alloc.c, simplified */
struct page *__alloc_pages_slowpath(gfp_t gfp_mask, unsigned int order, ...)
{
    wake_all_kswapds(order, gfp_mask, ...);

retry:
    page = get_page_from_freelist(gfp_mask, order, alloc_flags, ac);
    if (page)
        goto got_page;

    if (can_direct_reclaim) {
        progress = __perform_reclaim(gfp_mask, order, ac);  /* SYNCHRONOUS reclaim,
                                                                 the caller pays the cost */
        if (progress)
            goto retry;
    }

    if (should_compact_retry(...))
        goto retry;  /* try defragmenting instead of just reclaiming */

    /* Still nothing? Last resort. */
    page = __alloc_pages_may_oom(gfp_mask, order, ac);
    ...
}
```

### 12.4 `shrinker`s — reclaiming non-page-cache kernel memory

Subsystems that cache kernel objects (dentries, inodes, filesystem
metadata caches) register a **shrinker** callback so the reclaim path can
ask them to give memory back too, not just the LRU page lists:

```c
/* Typical shrinker registration pattern (fs/dcache.c style) */
static unsigned long dentry_shrink_count(struct shrinker *shrink,
                                          struct shrink_control *sc)
{
    return dentry_stat.nr_unused;
}

static unsigned long dentry_shrink_scan(struct shrinker *shrink,
                                         struct shrink_control *sc)
{
    return prune_dcache_sb(sc->nr_to_scan);  /* actually free some dentries */
}

static struct shrinker dcache_shrinker = {
    .count_objects = dentry_shrink_count,
    .scan_objects  = dentry_shrink_scan,
    .seeks = DEFAULT_SEEKS,
};

void __init dcache_init(void)
{
    register_shrinker(&dcache_shrinker, "dcache");
}
```

---

## 13. Swap Subsystem

Swap extends usable "memory" by moving cold **anonymous** pages (heap,
stack — pages with no backing file) out to a swap device/file, freeing the
physical page for reuse. (File-backed pages don't need swap — they're just
dropped, since the file itself is the backing store; if dirty, they're
written back to the file instead.)

```
   anonymous page, INACTIVE_ANON, chosen for reclaim
                     │
                     ▼
         get_swap_page()  -- allocate a slot in the swap area
                     │
                     ▼
       swap_writepage() -- write the page's content out to the swap device
                     │
                     ▼
   PTE is rewritten: Present bit cleared, remaining bits encode the
   swap type + offset instead of a PFN
                     │
                     ▼
        physical page freed back to the buddy allocator
                     │
      (later) process touches that virtual address again
                     │
                     ▼
              PAGE FAULT -- do_swap_page()
                     │
                     ▼
   look up the swap slot encoded in the (non-present) PTE, read the
   page back in from the swap device, install a fresh PTE
```

### 13.1 Swap PTE encoding

```
 Present PTE (bit 0 = 1):
  [ ... PFN ... ][flags][R/W][U/S][P=1]

 Swapped-out PTE (bit 0 = 0):
  [ swap_offset ][ swap_type ][    unused    ][   ][   ][P=0]
       (the same 64 bits, entirely repurposed since the hardware
        ignores everything except bit 0 when Present=0)
```

```c
/* Simplified do_swap_page() (mm/memory.c) */
static vm_fault_t do_swap_page(struct vm_fault *vmf)
{
    swp_entry_t entry = pte_to_swp_entry(vmf->orig_pte);

    struct page *page = lookup_swap_cache(entry, vmf->vma, vmf->address);
    if (!page) {
        /* Not even in the swap cache -- read it from the swap device */
        page = swap_readpage(entry, vmf->vma, vmf->address);
        if (!page)
            return VM_FAULT_OOM;
    }

    pte_t pte = mk_pte(page, vmf->vma->vm_page_prot);
    set_pte_at(vmf->vma->vm_mm, vmf->address, vmf->pte, pte);
    swap_free(entry);            /* drop the swap slot reference */
    page_add_anon_rmap(page, vmf->vma, vmf->address, false);

    return 0;
}
```

### 13.2 Setting up swap

```bash
# Traditional swap partition
sudo mkswap /dev/sda5
sudo swapon /dev/sda5

# Swap file (common on cloud instances / laptops)
sudo fallocate -l 4G /swapfile
sudo chmod 600 /swapfile
sudo mkswap /swapfile
sudo swapon /swapfile

# Tune how aggressively the kernel swaps vs drops page cache (0-200, default 60)
sudo sysctl vm.swappiness=10
```

### 13.3 zswap / zram — compressed swap in RAM

Rather than writing to a (slow) block device, `zswap` compresses
cold anonymous pages and keeps them in a RAM pool; `zram` goes further and
creates an entire compressed block device in RAM to swap to. Both trade CPU
cycles for effectively more usable memory — extremely common on
memory-constrained Android/embedded and even desktop Linux today.

```bash
# zram: create a 2GB compressed RAM-backed swap device
echo 2G > /sys/block/zram0/disksize
mkswap /dev/zram0
swapon -p 100 /dev/zram0
```

---

## 14. Transparent Huge Pages (THP)

Normal pages are 4 KiB. Using huge pages (2 MiB, or 1 GiB with `hugetlbfs`)
reduces the number of page-table entries and TLB pressure dramatically for
large working sets. **THP** does this *automatically*, without the
application needing to use `hugetlbfs` explicitly.

```
Without huge pages, mapping 2 MiB of memory needs:
  512 × 4 KiB PTEs  → 512 TLB entries needed to cover the region

With a THP (one 2 MiB page):
  1 × PMD entry marked "huge" (PS bit set) → 1 TLB entry covers the same region
```

```
                anon fault / khugepaged background scan
                                │
                                ▼
                  is this VMA eligible? (madvise/always mode,
                  size + alignment allows a 2 MiB-aligned region)
                                │ yes
                                ▼
              alloc_pages(GFP_TRANSHUGE, order=9)  -- 512 contiguous pages
                                │
                                ▼
          install ONE PMD entry with the PS (page size) bit set,
          instead of 512 individual PTEs
                                │
                                ▼
              if later only part of the huge page is touched
              differently (e.g. partial munmap, COW on one subpage) →
              PAGE SPLIT: PMD huge entry broken back into 512 normal PTEs
```

```c
/* mm/huge_memory.c, simplified fault handler */
vm_fault_t do_huge_pmd_anonymous_page(struct vm_fault *vmf)
{
    if (!transparent_hugepage_enabled(vmf->vma))
        return VM_FAULT_FALLBACK;   /* fall back to normal 4K path */

    struct page *page = alloc_hugepage_vma(GFP_TRANSHUGE, vmf->vma,
                                             haddr, HPAGE_PMD_ORDER);
    if (!page)
        return VM_FAULT_FALLBACK;   /* couldn't get a contiguous 2MiB block --
                                        common under fragmentation; falls
                                        back gracefully to 4K pages */

    clear_huge_page(page, haddr, HPAGE_PMD_NR);
    pmd_t entry = mk_huge_pmd(page, vmf->vma->vm_page_prot);
    set_pmd_at(vmf->vma->vm_mm, haddr, vmf->pmd, entry);
    return 0;
}
```

```bash
# Check/set THP mode
cat /sys/kernel/mm/transparent_hugepage/enabled
# always [madvise] never
echo always | sudo tee /sys/kernel/mm/transparent_hugepage/enabled

# Explicit hugetlbfs (guaranteed huge pages, reserved at boot, never split)
echo 512 | sudo tee /proc/sys/vm/nr_hugepages   # reserve 512 x 2MiB = 1GiB
```

`khugepaged` is a background kernel thread that scans existing mappings and
opportunistically *promotes* runs of regular pages into a THP if they
happen to be physically contiguous and suitably aligned, without waiting
for a fresh fault.

---

## 15. The OOM Killer

When memory is exhausted and reclaim/swap/compaction have all failed to
free enough, the kernel must pick a victim process to kill rather than let
the whole system deadlock or thrash to a halt.

```c
/* mm/oom_kill.c, badness scoring (heavily simplified) */
long oom_badness(struct task_struct *p, unsigned long totalpages)
{
    if (is_global_init(p) || (p->flags & PF_KTHREAD))
        return LONG_MIN;                     /* never kill init or kernel threads */

    long points = get_mm_rss(p->mm) + get_mm_swap_entries(p->mm);
    points += p->mm->total_vm / 8;            /* virtual memory used, minor weight */

    /* oom_score_adj (-1000 to +1000) lets userspace bias the decision --
     * e.g. sshd sets -1000 ("never kill me"), a sacrificial batch job
     * might set +500 */
    points += points * p->signal->oom_score_adj / 1000;

    return points;
}
```

```
  Allocation fails after: buddy exhausted at all watermarks
       + direct reclaim made no progress
       + compaction couldn't create a contiguous block
       + (memcg case) cgroup limit hit and cgroup reclaim also failed
                                │
                                ▼
                    out_of_memory() is invoked
                                │
                                ▼
             scan all eligible processes, compute oom_badness()
                                │
                                ▼
           pick the highest-scoring process (usually: biggest RSS
           consumer, unless oom_score_adj says otherwise)
                                │
                                ▼
                send SIGKILL to that process (and its thread group)
                                │
                                ▼
           its memory is freed as it exits → allocation can proceed
```

```bash
# Protect a critical process from the OOM killer
echo -1000 | sudo tee /proc/$(pgrep sshd | head -1)/oom_score_adj

# Watch it happen live
dmesg -T | grep -i 'killed process'
journalctl -k | grep -i oom
```

**cgroup-aware OOM**: with cgroup v2, an OOM inside a memory-limited
cgroup (`memory.max` exceeded) is scoped to that cgroup — the kernel first
tries to kill something *inside* the offending cgroup rather than picking
a victim system-wide.

---

## 16. NUMA and NUMA Balancing

### 16.1 Allocation policy

```c
#include <numaif.h>

/* Bind this allocation to node 0 explicitly */
unsigned long nodemask = 1UL << 0;
mbind(addr, length, MPOL_BIND, &nodemask, 2, 0);

/* Or let the kernel interleave across all nodes (good for large,
 * uniformly-accessed shared structures) */
set_mempolicy(MPOL_INTERLEAVE, &all_nodes_mask, max_node + 1);
```

```bash
# Inspect NUMA topology
numactl --hardware

# Run a process pinned to node 0's CPUs and memory
numactl --cpunodebind=0 --membind=0 ./my_program
```

### 16.2 Automatic NUMA balancing

If a process's memory ends up on a different node than the CPU it's mostly
running on (common after the scheduler migrates a thread), Linux can
detect and fix this automatically:

```
  kernel periodically unmaps some of the task's pages (marks PTEs
  "PROT_NONE" but not actually swapped -- a "NUMA hinting fault" trap)
                     │
                     ▼
     next access to that page → minor fault → do_numa_page()
                     │
                     ▼
     compare: which node is this page on? which node is this
     thread currently running on?
                     │
              mismatch found
                     ▼
     migrate_misplaced_page(): copy the page to the local node,
     update the PTE to point at the new physical location
                     │
                     ▼
  over time, hot pages "follow" the threads that use them
```

```bash
cat /proc/sys/kernel/numa_balancing        # 1 = enabled (default on NUMA HW)
cat /proc/<pid>/numa_maps                  # see per-VMA node distribution
```

---

## 17. Memory Control Groups (memcg)

cgroups let the kernel enforce **per-group** memory limits and accounting
— the mechanism underlying container memory limits (Docker/Kubernetes
`--memory`, `resources.limits.memory`).

```
                     cgroup v2 hierarchy
                                │
             /sys/fs/cgroup/mygroup/
             ├── memory.max        (hard limit — OOM inside this cgroup if exceeded)
             ├── memory.high       (soft limit — throttled + reclaimed, not killed)
             ├── memory.low        (protection — reclaim avoids this if possible)
             ├── memory.current    (current usage)
             ├── memory.stat       (detailed breakdown: anon, file, kernel, slab...)
             └── memory.events     (low, high, max, oom, oom_kill counters)
```

```bash
# Create a cgroup, cap it at 500 MiB, run a process inside it
sudo mkdir /sys/fs/cgroup/mygroup
echo 500M | sudo tee /sys/fs/cgroup/mygroup/memory.max
echo $$ | sudo tee /sys/fs/cgroup/mygroup/cgroup.procs
# this shell (and children) are now capped at 500 MiB
```

Each `struct page` that belongs to user memory is charged to exactly one
memcg (`page->memcg_data`), and every LRU list is actually maintained
**per memcg × per node**, not just per node — reclaim under a memcg limit
only scans that cgroup's own LRU lists.

```c
/* Simplified memcg charging path, called from most page-allocation sites
 * that hand memory to userspace (mm/memcontrol.c) */
int mem_cgroup_charge(struct folio *folio, struct mm_struct *mm, gfp_t gfp)
{
    struct mem_cgroup *memcg = get_mem_cgroup_from_mm(mm);

    if (page_counter_try_charge(&memcg->memory, folio_nr_pages(folio)))
        goto charged;

    /* Over limit: try reclaiming within this memcg first */
    if (try_to_free_mem_cgroup_pages(memcg, folio_nr_pages(folio), gfp))
        goto retry_charge;

    /* Still over limit -- trigger a memcg-scoped OOM kill */
    mem_cgroup_out_of_memory(memcg, gfp, folio_order(folio));
    return -ENOMEM;

charged:
    folio->memcg_data = (unsigned long)memcg;
    return 0;
}
```

---

## 18. Rust for Linux: MM Bindings

Since roughly Linux 6.1+, the **Rust for Linux** project has been adding
Rust support to the kernel, including safe abstractions over allocation
APIs. This isn't a rewrite of `mm/` — the core buddy/slab/page-table
machinery remains C — but Rust kernel modules and increasingly some
subsystems get safe, idiomatic wrappers over those C primitives via the
`kernel` crate.

### 18.1 The core allocation traits

```rust
// rust/kernel/alloc.rs (conceptual, based on upstream design)
// The kernel crate exposes GFP flags and typed allocators mirroring C's
// kmalloc/vmalloc/kvmalloc, but wired into Rust's `Allocator` trait so
// standard collections (Vec, Box) can use kernel allocation directly.

use kernel::alloc::{flags, Flags};

pub struct Kmalloc;
pub struct Vmalloc;
pub struct KVmalloc;

impl Kmalloc {
    /// Allocate `layout`-compatible memory via the kernel's kmalloc().
    /// Returns a raw NonNull<u8> or an AllocError, never panics.
    pub unsafe fn alloc(layout: Layout, flags: Flags) -> Result<NonNull<[u8]>, AllocError>;
}
```

### 18.2 `KBox<T>` — a kernel-allocated `Box`

```rust
// Using the kernel-provided Box-like type, backed by kmalloc, with
// fallible allocation (kernel code MUST handle allocation failure --
// there's no unwinding-based OOM abort like userspace Rust's Box).
use kernel::alloc::{flags::GFP_KERNEL, KBox};
use kernel::prelude::*;

struct Request {
    id: u64,
    payload: [u8; 256],
}

fn make_request(id: u64) -> Result<KBox<Request>> {
    // try_new returns Result, not Option/panic -- allocation failure is
    // an ordinary, handled error path, exactly like C's "if (!p) return -ENOMEM;"
    let req = KBox::new(
        Request { id, payload: [0; 256] },
        GFP_KERNEL,
    )?;
    Ok(req)
}
```

### 18.3 `KVec<T>` — kernel Vec with explicit fallible growth

```rust
use kernel::alloc::{flags::GFP_KERNEL, KVec};
use kernel::prelude::*;

fn collect_ids(count: usize) -> Result<KVec<u64>> {
    let mut v = KVec::new();
    for i in 0..count {
        // push tries to grow the backing allocation and returns Result;
        // in userspace Rust this would silently abort on OOM instead.
        v.push(i as u64, GFP_KERNEL)?;
    }
    Ok(v)
}
```

### 18.4 A minimal Rust kernel module allocating memory

```rust
// rust/samples/rust_mm_example.rs — illustrative, follows the shape of
// real samples in samples/rust/ upstream.
use kernel::prelude::*;
use kernel::alloc::{flags::GFP_KERNEL, KBox};

module! {
    type: MmExample,
    name: "mm_example",
    author: "Example",
    description: "Demonstrates kernel allocation from Rust",
    license: "GPL",
}

struct MmExample {
    buffer: KBox<[u8; 4096]>,
}

impl kernel::Module for MmExample {
    fn init(_module: &'static ThisModule) -> Result<Self> {
        // Equivalent to: buf = kmalloc(4096, GFP_KERNEL); if (!buf) return -ENOMEM;
        let buffer = KBox::new([0u8; 4096], GFP_KERNEL)?;
        pr_info!("mm_example: allocated {} bytes via kmalloc\n", buffer.len());
        Ok(MmExample { buffer })
    }
}

impl Drop for MmExample {
    fn drop(&mut self) {
        // KBox's Drop impl calls kfree() automatically -- no explicit
        // free needed, and it's impossible to forget (unlike raw C kfree()).
        pr_info!("mm_example: freeing buffer\n");
    }
}
```

### 18.5 Why this matters for MM specifically

The Rust-for-Linux MM story is deliberately narrow and pragmatic:

- **Not replacing `mm/`**: the buddy allocator, SLUB, page tables, reclaim
  — all remain C. Rewriting core MM in Rust is not an active near-term
  goal; it's enormous, deeply performance-tuned, and touches every
  architecture's low-level code.
- **What Rust *does* target**: (1) safe wrappers so *driver and subsystem*
  code written in Rust can allocate memory without manual `kfree`
  bookkeeping and without the class of use-after-free/double-free bugs
  that plague C allocation call sites; (2) fallible-by-construction
  allocation (`Result`-returning, not panic/abort on OOM) because kernel
  code must always handle allocation failure gracefully, unlike most
  userspace Rust.
- **Ongoing area**: `Allocator` trait plumbing for `Vec`/`Box`-equivalents
  (`KVec`, `KBox`), and increasingly, safe abstractions for specific
  subsystems (e.g., DMA-coherent allocations for Rust device drivers) as
  more of the driver tree gains Rust support.

---

## 19. Observability & Debugging

```bash
# System-wide memory summary
free -h
cat /proc/meminfo

# Per-zone watermarks & free page counts by order
cat /proc/buddyinfo
cat /proc/zoneinfo

# Per-process memory maps and RSS/PSS breakdown
cat /proc/<pid>/maps
cat /proc/<pid>/smaps_rollup

# Slab cache usage
sudo slabtop
cat /proc/slabinfo

# Live reclaim/compaction/fault event counters
cat /proc/vmstat | egrep 'pgfault|pgmajfault|pgscan|pgsteal|compact'

# Trace page allocation call sites (great for finding leaks)
echo 1 | sudo tee /proc/sys/vm/oom_dump_tasks

# ftrace / perf for MM hot paths
sudo perf record -e 'kmem:*' -a sleep 5
sudo perf script

# BPF-based, very granular (requires bpftrace)
sudo bpftrace -e 'kprobe:__alloc_pages { @[comm] = count(); }'
```

```c
/* Common /proc/meminfo fields and what they actually mean */
MemTotal:        physical RAM installed
MemFree:         truly unused pages (buddy allocator free lists)
MemAvailable:    MemFree + reclaimable cache -- "realistic" free estimate
Buffers/Cached:  page cache (file-backed) -- reclaimable under pressure
Active/Inactive: LRU list sizes (split further into (anon)/(file) variants)
Dirty:           page-cache pages waiting to be written back
Slab:            SLUB/SLAB object cache memory (SReclaimable + SUnreclaim)
```

---

## 20. End-to-End Architecture Diagram

```
┌──────────────────────────────────────────────────────────────────────────┐
│                              USERSPACE                                     │
│   malloc()/free()  read()/write()  mmap()/munmap()  numactl/cgroups        │
└───────────────────────────────────┬────────────────────────────────────────┘
                                     │ syscalls
┌───────────────────────────────────▼────────────────────────────────────────┐
│                          VIRTUAL MEMORY LAYER                              │
│  mm_struct ── maple tree of vm_area_struct (VMAs) ── mmap_lock/per-VMA lock │
│  page fault handler (handle_mm_fault): anon / file / COW / swap-in         │
└───────────┬───────────────────────────────────────────────┬────────────────┘
            │                                                 │
┌───────────▼────────────────┐               ┌────────────────▼───────────────┐
│      PAGE TABLES             │               │        PAGE CACHE               │
│  PGD→P4D→PUD→PMD→PTE walk    │               │  address_space (xarray of       │
│  Accessed/Dirty bits, THP,   │               │  folios), readahead, writeback   │
│  TLB shootdown batching      │               └────────────────┬───────────────┘
└───────────┬────────────────┘                                 │
            │                                                   │
┌───────────▼───────────────────────────────────────────────────▼────────────┐
│                       PHYSICAL MEMORY LAYER                                │
│   struct page[] (vmemmap) ── LRU lists (active/inactive × anon/file × memcg)│
│   kswapd (async reclaim) ── direct reclaim ── compaction ── OOM killer      │
└───────────┬───────────────────────────────────────────────┬────────────────┘
            │                                                 │
┌───────────▼────────────────┐               ┌────────────────▼───────────────┐
│  SLUB / kmalloc              │               │  vmalloc                       │
│  fixed-size kernel objects,  │               │  large, non-contiguous kernel   │
│  per-CPU lock-free freelists │               │  allocations, dedicated VA range │
└───────────┬────────────────┘               └────────────────┬───────────────┘
            │                                                   │
┌───────────▼───────────────────────────────────────────────────▼────────────┐
│                            BUDDY ALLOCATOR                                 │
│   per-node, per-zone free_area[order] free lists, migratetype buckets      │
└───────────┬──────────────────────────────────────────────────────────────┘
            │
┌───────────▼──────────────────────────────────────────────────────────────┐
│                    PHYSICAL RAM  (page frames, per NUMA node)              │
│      Node 0: [DMA][DMA32][NORMAL][MOVABLE]   Node 1: [DMA][DMA32]...       │
└──────────────────────────────────────────────────────────────────────────┘

        Cross-cutting concerns woven through every layer above:
        ┌────────────────────────────────────────────────────────────┐
        │  memcg accounting & limits   │  swap (anon reclaim target)  │
        │  NUMA policy & balancing     │  compaction (defragmentation)│
        └────────────────────────────────────────────────────────────┘
```

---

## 21. Glossary

| Term | Definition |
|---|---|
| **PFN** | Page Frame Number — `physical_addr >> PAGE_SHIFT`. |
| **VMA** | `vm_area_struct` — one contiguous virtual address range in a process. |
| **folio** | A physically-contiguous group of one or more pages treated as a unit (post-5.16 page cache/anon rework). |
| **rmap** | Reverse mapping — from a physical page back to the PTE(s)/VMA(s) mapping it. |
| **buddy allocator** | Power-of-two page-frame allocator; the base of all physical memory allocation. |
| **slab/SLUB** | Object allocator layered on the buddy allocator, for fixed-size kernel objects. |
| **watermark** | Per-zone free-memory thresholds (`MIN`/`LOW`/`HIGH`) governing when reclaim triggers. |
| **kswapd** | Per-node kernel thread doing background/asynchronous reclaim. |
| **direct reclaim** | Synchronous reclaim performed by the allocating process itself, when kswapd hasn't kept up. |
| **THP** | Transparent Huge Pages — automatic use of 2 MiB pages to reduce TLB pressure. |
| **COW** | Copy-on-Write — deferred copying of shared pages until one side writes. |
| **memcg** | Memory cgroup — per-group memory accounting/limiting. |
| **OOM killer** | Last-resort mechanism that kills a process to free memory when all else fails. |
| **TLB shootdown** | Cross-CPU invalidation of stale cached virtual→physical translations. |
| **GFP flags** | "Get Free Page" flags describing allocation context/constraints (sleep? reclaim? zone?). |
| **memblock** | The boot-time-only allocator used before the buddy allocator is initialized. |

---

### Suggested reading order for building a mental model

1. §1 (big picture) → §2–3 (physical memory + buddy) → §6 (page tables) →
   §5 (VMAs) → §7–8 (fault path + mmap) — this gives you the full
   "virtual address to physical byte" story end to end.
2. §9–10 (slab/vmalloc) — how the kernel allocates *its own* memory on top
   of the same buddy allocator.
3. §11–13 (page cache, reclaim, swap) — what happens under memory
   pressure.
4. §14–17 (THP, OOM, NUMA, cgroups) — the policy layers that make Linux MM
   production-grade for real, mixed, multi-tenant workloads.
5. §18 (Rust) — how the newest driver/subsystem code is starting to
   interact with all of the above safely.
