# DMAable Memory: Complete Comprehensive Guide

## Table of Contents

1. [Fundamental Concepts](#fundamental-concepts)
2. [Physical Memory Architecture](#physical-memory-architecture)
3. [DMA Mechanism and Operation](#dma-mechanism-and-operation)
4. [IOMMU and Address Translation](#iommu-and-address-translation)
5. [Cache Coherency](#cache-coherency)
6. [Linux Kernel Implementation](#linux-kernel-implementation)
7. [Memory Alignment and Padding](#memory-alignment-and-padding)
8. [C Implementation Guide](#c-implementation-guide)
9. [Rust Implementation Guide](#rust-implementation-guide)
10. [Real-World Scenarios](#real-world-scenarios)
11. [Debugging and Performance](#debugging-and-performance)
12. [Advanced Topics](#advanced-topics)

---

## Fundamental Concepts

### What is DMA?

DMA (Direct Memory Access) is a hardware mechanism that allows peripheral devices to access system memory **without CPU intervention**. This bypasses the CPU entirely for data movement operations, enabling:

- **High throughput**: Devices can move data at hardware speed (limited only by bus bandwidth)
- **Reduced CPU overhead**: CPU is freed from repetitive memory copy operations
- **Parallelism**: CPU and I/O can operate concurrently

### Why DMAable Memory?

Regular application memory may be:
1. **Paged out**: Physical page may not be in RAM (swapped to disk)
2. **Shared with COW (Copy-on-Write)**: Page may be duplicated, breaking DMA semantics
3. **Non-contiguous in physical space**: DMA devices expect contiguous physical addresses
4. **Movable**: Kernel can relocate it for memory compaction

DMAable memory is **pinned, physically contiguous, and cache-coherent** with the device.

### Key Terminology

| Term | Meaning |
|------|---------|
| **VA (Virtual Address)** | Address seen by CPU and application |
| **PA (Physical Address)** | Actual address on memory bus |
| **DMA Address** | Address device uses (may differ from PA in virtualized systems) |
| **IOVA (I/O Virtual Address)** | Virtual address space managed by IOMMU |
| **Pinned Memory** | Pages locked in physical RAM, cannot be swapped or moved |
| **Coherency** | Consistency between CPU cache and device's view of memory |

---

## Physical Memory Architecture

### System Memory Layout (x86-64)

```
Physical Memory Space (System View)
┌─────────────────────────────────────────────────────────┐
│                  High Address (>4GB)                    │
│                                                         │
│                  Kernel Managed                         │
│                  (kernel heap, modules)                 │
│                                                         │
├─────────────────────────────────────────────────────────┤
│              NUMA Node 1 (if multi-socket)              │
│              Managed by Buddy Allocator                 │
│                                                         │
│  ┌─────────────────────────────────────────────────┐   │
│  │ Page 0: struct page metadata (Linux)            │   │
│  │ Page 1: User Memory (may be in app space)       │   │
│  │ Page 2: DMA Buffer (pinned, contiguous PA)      │   │ ← DMAable
│  │ Page 3: DMA Buffer (contiguous with Page 2)     │   │ ← DMAable
│  │ ...                                             │   │
│  │ Page N: Kernel Buffer (cache-coherent)          │   │
│  └─────────────────────────────────────────────────┘   │
│                                                         │
├─────────────────────────────────────────────────────────┤
│              NUMA Node 0 (local)                         │
│              Similar structure                          │
│                                                         │
├─────────────────────────────────────────────────────────┤
│           DMA Zone (24-bit addressing devices)          │
│           (ISA, old PCI devices)                        │
│                                                         │
├─────────────────────────────────────────────────────────┤
│                 Low Address (first 1MB)                 │
│                 (BIOS, bootloader)                      │
└─────────────────────────────────────────────────────────┘

Linux organizes physical memory into zones:
- ZONE_DMA:       0 - 16MB    (for ISA and legacy devices)
- ZONE_DMA32:     16MB - 4GB  (for 32-bit DMA capable devices)
- ZONE_NORMAL:    4GB+        (normal kernel memory)
- ZONE_HIGHMEM:   (32-bit systems, permanently mapped memory window)
- ZONE_MOVABLE:   (hotpluggable memory, no kernel allocations)
```

### struct page: Memory Metadata

Every page of physical RAM has a `struct page` (Linux kernel):

```c
struct page {
    unsigned long flags;           // PG_locked, PG_uptodate, PG_dirty, etc.
    struct address_space *mapping; // associated inode/file (if pagecache)
    atomic_t _mapcount;            // page table references
    atomic_t _refcount;            // reference count (pin count)
    
    union {
        struct list_head lru;      // LRU cache list
        struct {                   // For slab allocator
            void *freelist;
            struct kmem_cache *slab_cache;
        };
    };
    
    unsigned long private;         // driver-specific data
};
```

**For DMA buffers**, Linux maintains:
- `_refcount` > 0: Page cannot be freed
- `flags & PG_locked`: Page cannot be swapped or migrated
- Virtual address mapping in `mapping` (if applicable)

---

## DMA Mechanism and Operation

### Request-Grant-Transfer Cycle

```
┌─────────────────────────────────────────────────────────────────┐
│ Application/Driver Process                                       │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
                    ┌──────────────────┐
                    │  1. Setup        │
                    │  DMA descriptors │  (allocate DMA buffer, fill with data)
                    │  Register addr   │  (provide physical address to device)
                    │  + size          │
                    └──────────────────┘
                              │
                              ▼
                    ┌──────────────────┐
                    │  2. Issue Cmd    │
                    │  Ring doorbell   │  (write to device register)
                    │  (device IRQ OK) │
                    └──────────────────┘
                              │
                              ▼ (CPU can context switch now)
                    ┌──────────────────┐
                    │  3. Device DMA   │
                    │  Transfers Data  │  (hardware moves memory)
                    │  (bus arbitration)
                    │  (cache coherency)
                    │  (address xlation)
                    └──────────────────┘
                              │
                              ▼
                    ┌──────────────────┐
                    │  4. Interrupt    │
                    │  Device signals  │  (MSI-X, edge-triggered)
                    │  completion      │
                    └──────────────────┘
                              │
                              ▼
                    ┌──────────────────┐
                    │  5. ISR Handler  │  (interrupt service routine)
                    │  Read completion │  (ACK device, read descriptor)
                    │  status          │
                    └──────────────────┘
                              │
                              ▼
                    ┌──────────────────┐
                    │  6. Process      │
                    │  data / reuse    │
                    │  buffer          │
                    └──────────────────┘
```

### DMA Channel Architecture

Modern devices use **queue-based DMA** with descriptors:

```
Device Memory (PCIe BAR):
┌─────────────────────────────────────────────────────┐
│ TX Queue (Transmit)                                 │
│ ┌─────────────────────────────────────────────────┐ │
│ │ Index 0: Descriptor                             │ │
│ │   .addr = 0x1000000 (physical)                  │ │
│ │   .len = 4096 bytes                             │ │
│ │   .flags = OWN (owned by device)                │ │
│ │   .status = 0                                   │ │
│ │                                                 │ │
│ │ Index 1: Descriptor                             │ │
│ │   .addr = 0x1001000 (physical, contiguous)      │ │
│ │   .len = 4096 bytes                             │ │
│ │   .flags = OWN | EOP (end of packet)            │ │
│ │   .status = 0                                   │ │
│ │                                                 │ │
│ │ Index 2: Descriptor (available)                 │ │
│ │   .flags = 0 (driver owns)                      │ │
│ ├─────────────────────────────────────────────────┤ │
│ │ Head Pointer: 1 (device reading from index 1)   │ │
│ │ Tail Pointer: 2 (driver writes next at index 2) │ │
│ └─────────────────────────────────────────────────┘ │
│                                                     │
│ RX Queue (Receive)                                  │
│ ┌─────────────────────────────────────────────────┐ │
│ │ Index 0: Descriptor                             │ │
│ │   .addr = 0x2000000 (physical, pre-allocated)   │ │
│ │   .len = 2048 bytes (buffer size)               │ │
│ │   .flags = OWN (device can write to buffer)     │ │
│ │   .status = 0                                   │ │
│ │   .actual_len = 0 (filled by device)            │ │
│ │                                                 │ │
│ │ Index 1: Descriptor                             │ │
│ │   .addr = 0x2000800 (physical, contiguous)      │ │
│ │   .len = 2048 bytes                             │ │
│ │   .flags = OWN                                  │ │
│ │   .status = 0                                   │ │
│ │   .actual_len = 0                               │ │
│ └─────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────┘

System RAM:
┌──────────────────────────────────────┐
│ TX Buffer (0x1000000, 8KB contiguous)│
│ ├─────────────────┬─────────────────┤
│ │ Page 0 (4KB)    │ Page 1 (4KB)     │ ← physically contiguous
│ └─────────────────┴─────────────────┘
│
│ RX Buffer (0x2000000, 4KB each)      │
│ ├──────────┬──────────┤              │
│ │ Page 2   │ Page 3   │              │ ← must be contiguous
│ └──────────┴──────────┘              │
└──────────────────────────────────────┘
```

### Ownership Model

```
Owner | Can Write | Can Read | Notes
------|-----------|----------|-------
CPU   | Yes       | Yes      | Before DMA starts
Dev   | Yes       | Yes      | While .flags & OWN is set
CPU   | No        | No       | While device owns buffer
CPU   | Yes       | Yes      | After device clears OWN flag

Violation: CPU writes while device owns = data corruption
Violation: CPU reads stale data = inconsistency

Sequence:
1. CPU fills TX buffer with data
2. CPU sets descriptor .flags = OWN  ← CPU releases
3. Device reads, processes, writes status ← Device owns
4. Device clears .flags &= ~OWN      ← Device releases
5. CPU reads status, processes RX    ← CPU owns again
```

---

## IOMMU and Address Translation

### Why IOMMU?

In systems with IOMMU (Intel VT-d, AMD-Vi):

```
Without IOMMU:
┌────────────────────────────────────────────┐
│ Threat: Malicious device can DMA to any PA │
│ (privilege escalation, data theft)          │
│                                             │
│ Device sees PA directly:                    │
│ Device.DMA(PA=0x12345678, len=4KB)         │
│   ├─ Can read/write ANY physical address   │
│   └─ No isolation                          │
└────────────────────────────────────────────┘

With IOMMU:
┌────────────────────────────────────────────┐
│ Translation engine controls device access   │
│                                             │
│ Device issues:                              │
│ Device.DMA(IOVA=0x80000000, len=4KB)      │ ← Device virtual addr
│   │                                         │
│   ▼                                         │
│ IOMMU translates:                          │
│ 0x80000000 → PA 0x12345678 ✓ (allowed)    │
│ 0x81000000 → denied (fault)                │
│                                             │
│ Protection domain: per-device address space│
└────────────────────────────────────────────┘
```

### IOMMU Page Tables

```
Device's IOVA Space (managed by IOMMU):
┌─────────────────────────────────────────────────────┐
│ IOVA = 0x80000000                                    │
│ ┌─────────────────────────────────────────────────┐ │
│ │ Level 1 (Root): PML4 table (4KB, 512 entries)   │ │
│ │ IOVA bits [47:39] = index into Level 1           │ │
│ │                                                  │ │
│ │ [0] → Level 2 table PA=0x10000                   │ │
│ │ [1] → Level 2 table PA=0x11000                   │ │
│ │ [256] → points to our device's allocated region │ │
│ │ [257..511] → unmapped (would fault)              │ │
│ └─────────────────────────────────────────────────┘ │
│         │                                            │
│         ▼                                            │
│ ┌─────────────────────────────────────────────────┐ │
│ │ Level 2 (PDPT): 4KB, 512 entries                 │ │
│ │ IOVA bits [38:30] = index into Level 2           │ │
│ │                                                  │ │
│ │ [0] → Level 3 table PA=0x20000                   │ │
│ │ [1] → Level 3 table PA=0x21000                   │ │
│ └─────────────────────────────────────────────────┘ │
│         │                                            │
│         ▼                                            │
│ ┌─────────────────────────────────────────────────┐ │
│ │ Level 3 (PDT): 4KB, 512 entries                  │ │
│ │ IOVA bits [29:21] = index into Level 3           │ │
│ │                                                  │
│ │ [0] → Level 4 table PA=0x30000                   │ │
│ │ [1] → Level 4 table PA=0x31000                   │ │
│ └─────────────────────────────────────────────────┘ │
│         │                                            │
│         ▼                                            │
│ ┌─────────────────────────────────────────────────┐ │
│ │ Level 4 (PT): 4KB, 512 entries                   │ │
│ │ IOVA bits [20:12] = index into Level 4           │ │
│ │                                                  │ │
│ │ [0] → PA=0x12345000, P=1, W=1, Read=1, Write=1  │ │
│ │ [1] → PA=0x12346000, P=1, W=1, Read=1, Write=1  │ │
│ │ [2] → unmapped (P=0)                             │ │
│ └─────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────┘

Final IOVA → PA mapping:
0x80000000 (IOVA)
  = PDPT[0].PA + PDT[0].PA + PT[0].PA
  = 0x12345000 (PA)
```

### Linux IOMMU API

```
Device wants to access:  Bus Address (IOVA)
                              │
                              ▼
            ┌──────────────────────────────────┐
            │ iommu_map(domain, iova, phys,    │
            │           size, prot)            │
            │ - Creates translation entry      │
            │ - Allocates page tables if needed│
            └──────────────────────────────────┘
                              │
                              ▼
            DMA happens at device level:
            Device accesses IOVA → IOMMU translates to PA

            ┌──────────────────────────────────┐
            │ iommu_unmap(domain, iova, size)  │
            │ - Invalidates translation        │
            │ - Flushes IOTLB                  │
            └──────────────────────────────────┘
```

---

## Cache Coherency

### The Coherency Problem

```
CPU View                Device View
┌──────────────┐        ┌──────────────┐
│ L3 Cache:    │        │ Device Reads │
│ 0x1000 = 42  │        │ PA 0x1000:   │ 
│              │        │ sees 99 (old)│ ← STALE DATA!
├──────────────┤        └──────────────┘
│ RAM:         │
│ 0x1000 = 99  │
└──────────────┘
```

**Why?** CPU caches the line, device bypasses cache and reads directly from RAM. When CPU updates the cached line, device doesn't see the change.

### Cache Coherency Solutions

#### 1. Write-Back with Explicit Flush

```
CPU updates data:
RAM: value = 0x1000 = 99     (in cache, dirty)

Before DMA:
clflush(&value)              (explicit flush to RAM)
OR
clflushopt(&value)           (optimized, still synchronous)

Then: value in RAM = 99, Device sees correct value
```

#### 2. Write-Through Caching

```
CPU writes → immediately goes to RAM
└─ Slower (every write hits RAM)
└─ Guarantees coherency
└─ Used by memory-mapped I/O regions (WC: Write-Combining)
```

#### 3. Hardware Coherency (Snooping)

```
CPU Cache         System Interconnect       Device
┌──────────┐                             ┌──────────┐
│ L1 Cache │────┐                    ┌───│ DMA Eng  │
│ L2 Cache │    │                    │   └──────────┘
│ L3 Cache │    │                    │
└──────────┘    │  ┌──────────────┐  │
                ├─→│ Cache        ├──┤
                │  │ Coherency    │  │
                │  │ Engine       │  │
                │  │ (snoop)      │  │
                │  │ - Invalidates│  │
                │  │ - Writes back│  │
                │  └──────────────┘  │
                │                    │
                └─────────────────────┘

Device reads address 0x1000:
Cache controller snoops bus
If CPU has dirty line, flushes it first
Device guaranteed to see latest value
```

#### 4. No-Snoop Devices

```
Device claims: "I never read memory you write to"
Device claims: "You never read memory I write to"

Used by GPU VRAM transfer engines
└─ GPU writes VRAM directly (CPU never reads)
└─ Device marks NO_SNOOP in PCIe transactions
└─ Avoids coherency overhead (speeds up ~20%)
```

### Linux Kernel Coherency Management

```c
// Memory region for DMA:
enum dma_data_direction {
    DMA_BIDIRECTIONAL,      // CPU and device both read/write
    DMA_TO_DEVICE,          // CPU writes, device reads only
    DMA_FROM_DEVICE,        // Device writes, CPU reads only
    DMA_NONE,               // (rarely used)
};

// Corresponding cache handling:
DMA_TO_DEVICE:
    ├─ writeback (flush cache to RAM)
    └─ device sees latest CPU data

DMA_FROM_DEVICE:
    ├─ (no action before)
    └─ invalidate (after, discard cache lines)
    └─ CPU sees device's writes

DMA_BIDIRECTIONAL:
    ├─ writeback (before: flush CPU updates)
    └─ invalidate (after: discard old cache lines)
```

---

## Linux Kernel Implementation

### Memory Zones and Allocators

```
Linux Memory Zones:
┌──────────────────────────────────────┐
│ Zone (area of physical memory)       │
├──────────────────────────────────────┤
│ ZONE_DMA:     0 - 16MB               │ ISA, legacy devices
│ ZONE_DMA32:   16MB - 4GB             │ 32-bit DMA devices
│ ZONE_NORMAL:  4GB - end              │ Normal kernel use
│ ZONE_HIGHMEM: (32-bit only)          │ Permanently mapped
│ ZONE_MOVABLE: (hotpluggable, if)     │ Can be removed
└──────────────────────────────────────┘

Each zone has:
struct zone {
    struct free_area free_area[MAX_ORDER];  // Buddy allocator
    unsigned long    managed_pages;
    unsigned long    present_pages;
    spinlock_t       lock;
    struct per_cpu_pageset *pageset;
    // ... and more
};
```

### Buddy Allocator (Foundation of DMA Buffers)

The Linux kernel uses the **Buddy Allocator** for page-level allocation:

```
Free memory represented as powers of 2:

Order 0: Single 4KB page
Order 1: 2 × 4KB = 8KB (consecutive)
Order 2: 4 × 4KB = 16KB (consecutive)
Order 3: 8 × 4KB = 32KB (consecutive)
...
Order N: 2^N × 4KB pages

Allocation of 12KB:
1. Need 3 pages = need at least Order 2 (4 pages)
2. Allocate Order 2 → returns 4 consecutive pages
3. Return 3 pages to user, internal fragmentation = 4KB

Free areas tracked as linked lists:

free_area[0] → (4KB) → (4KB) → (4KB) → NULL  (fragmented)
free_area[1] → (8KB) → NULL                   (one contiguous pair)
free_area[2] → (16KB) → NULL                  (one contiguous quad)
free_area[3] → NULL                           (no 8-page blocks)

Allocation strategy:
alloc_pages_node(node, flags, order):
    ├─ Try allocate from free_area[order]
    ├─ If empty, try free_area[order+1]
    │  ├─ Split into two buddies
    │  ├─ Return one, add other to free_area[order]
    └─ Continue until success or ENOMEM

Free strategy (coalescing):
free_pages(page, order):
    ├─ Mark as free
    ├─ Check buddy (page ^ (1 << order)) if free
    ├─ If buddy free, coalesce (merge) to higher order
    └─ Repeat recursively
```

### DMA Allocator: dma_alloc_coherent()

```c
// Generic DMA allocation (works across x86, ARM, etc.)
void *dma_alloc_coherent(struct device *dev, size_t size,
                         dma_addr_t *dma_handle, gfp_t gfp);

Under the hood:
1. Determine NUMA node (usually from device.numa_node)
2. Determine zone constraints:
   - Device max DMA address determines zone
   - 32-bit device → ZONE_DMA32
   - 64-bit device → ZONE_NORMAL

3. Allocate pages:
   pages = alloc_pages_exact_node(node, __GFP_ZERO | GFP_DMA, size)
   ├─ Allocates contiguous pages (physically)
   ├─ __GFP_ZERO: zeros the memory
   └─ GFP_DMA: restrict to DMA zone

4. Set page attributes:
   for each page:
       set_page_dma()     ← kernel-internal marking
       set_page_private() ← store metadata
       get_page()         ← increment refcount (pin)

5. Map to CPU virtual address:
   if (nocache):
       vaddr = vmap(pages, nocache, pgprot_writecombine())
   else:
       vaddr = page_address(pages[0])  # Already mapped in kernel space

6. Compute DMA address:
   *dma_handle = virt_to_phys(vaddr)
   
   Or if IOMMU present:
   *dma_handle = iommu_map_single(dev, phys, size, direction)
   ├─ Creates IOMMU translation
   └─ Returns IOVA (device sees IOVA, not phys)

7. Return vaddr to driver
   ├─ CPU accesses via vaddr
   ├─ Device accesses via *dma_handle
   └─ Both point to same physical memory
```

### Implementation Detail: struct page tracking

```c
// In dma_alloc_coherent, each allocated page:
struct page *pg = alloc_pages(...);

pg->_refcount = 2;     // Once for allocation, once for DMA
pg->flags |= PG_locked;   // Prevent swap/migration
set_bit(PG_dma, &pg->flags);  // Mark as DMA memory (implementation detail)

// When driver holds reference:
get_page(pg);           // Increment refcount

// Before freeing:
put_page(pg);           // Decrement refcount
// When refcount reaches 0, page returned to free list
```

### Free-path: dma_free_coherent()

```c
void dma_free_coherent(struct device *dev, size_t size,
                       void *vaddr, dma_addr_t dma_handle);

Under the hood:
1. Unmap from IOMMU (if present):
   if (has_iommu):
       iommu_unmap(dev, dma_handle, size)
       iommu_tlb_flush(dev)  ← invalidate IOTLB

2. Unmap from CPU:
   vunmap(vaddr)  ← only if vmap was used

3. Release pages:
   for each page:
       clear_bit(PG_locked, &pg->flags)  ← allow migration
       put_page(pg)
       
4. Return to buddy allocator:
   __free_pages(pg, order)
   ├─ Mark as free
   ├─ Coalesce with buddies
   └─ Add back to free_area[order]
```

### dma_alloc_coherent() vs vmalloc()

```
dma_alloc_coherent()    vmalloc()
┌────────────────────┐  ┌────────────────────┐
│ Physically contig  │  │ Virtually contig   │
│ Can be pinned      │  │ May be paged out   │
│ No cache-flush     │  │ Need cache ops     │
│ Device can access  │  │ Device CANNOT      │
│ Supports IOMMU     │  │ Must map manually  │
│ Slower allocation  │  │ Faster allocation  │
│ Limited by order   │  │ Limited by VM addr │
│ space available    │  │ space available    │
│ (max contiguous)   │  │ (max vm space)     │
│ (few MB typical)   │  │ (GBs possible)     │
└────────────────────┘  └────────────────────┘
```

---

## Memory Alignment and Padding

### Why Alignment Matters for DMA

```
Misaligned DMA Access:
└─ Device memory accesses are atomic per-word
└─ Crossing cache line boundary requires two transfers
└─ Misaligned access to certain mappings may fault

Best practice: align to cache line (64 bytes on modern x86)

struct dma_buffer {
    uint32_t command;      // offset 0
    uint16_t flags;        // offset 4
    uint16_t reserved;     // offset 6
    uint64_t dma_addr;     // offset 8 ← misaligned if not padded!
    
    // What CPU sees:
    // 0   1   2   3   4   5   6   7   8   9  10  11
    // +---+---+---+---+---+---+---+---+---+---+---+---+
    // | cmd  | flags | r | dma_addr ...
    //       ↑               ↑ crosses cache boundary!
};

Fixed: Use explicit padding and __attribute__((aligned(...)))

struct dma_buffer {
    uint32_t command;      // offset 0
    uint16_t flags;        // offset 4
    uint16_t reserved;     // offset 6
    uint64_t dma_addr;     // offset 8 (naturally aligned)
    
} __attribute__((aligned(64)));  ← align whole struct to cache line
```

### L1 and L2 Padding

```
L1 Cache:  64 bytes/line (modern x86)
L2 Cache:  Same, 64 bytes/line
L3 Cache:  Same, 64 bytes/line

Single cache line:
┌──────────────────────────────────────┐
│ Offset 0-63: one cache line (64B)    │
│                                      │
│ CPU core A reads offset 0            │
│ CPU core B reads offset 32           │
│ → Both cores load SAME cache line    │
│ → False sharing: ping-pong on CHB    │
│ → Serialization, ~200 cycle latency  │
└──────────────────────────────────────┘

Fix: Pad to separate cache lines
struct dma_buffer {
    uint64_t field_a;
    char padding[56];          ← fill to 64 bytes
} __attribute__((aligned(64)));
```

### Cache Line Alignment in C

```c
// Modern technique: use ALIGN macro or _Alignas

#define CACHE_LINE_SIZE 64

// C11 style:
struct dma_buffer {
    uint32_t cmd;
    uint32_t status;
    uint64_t addr;
} _Alignas(CACHE_LINE_SIZE);

// Or inline:
_Alignas(64) struct dma_buffer buf;

// Or kernel style:
struct dma_buffer {
    uint32_t cmd;
    uint32_t status;
    uint64_t addr;
} __attribute__((aligned(64)));

// Verify at compile time:
_Static_assert(sizeof(struct dma_buffer) <= 64,
               "struct dma_buffer exceeds cache line!");
```

---

## C Implementation Guide

### Basic DMA Buffer Allocation (Character Device)

```c
// dma_char_driver.c
#include <linux/module.h>
#include <linux/cdev.h>
#include <linux/device.h>
#include <linux/dma-mapping.h>
#include <linux/slab.h>
#include <linux/uaccess.h>

#define DMA_BUFFER_SIZE (64 * 1024)  // 64KB
#define CACHE_LINE      64

struct dma_buffer {
    struct device *dev;
    
    // Allocated memory
    void *cpu_addr;        // CPU-accessible virtual address
    dma_addr_t dma_addr;   // Device-accessible bus address
    size_t size;
    
    // Metadata for synchronization
    enum dma_data_direction direction;
    bool is_coherent;
} _Alignas(CACHE_LINE);

static int dma_driver_open(struct inode *inode, struct file *file)
{
    struct dma_buffer *buf = kzalloc(sizeof(*buf), GFP_KERNEL);
    if (!buf)
        return -ENOMEM;
    
    file->private_data = buf;
    return 0;
}

static int dma_driver_release(struct inode *inode, struct file *file)
{
    struct dma_buffer *buf = file->private_data;
    
    // If DMA was used and is still in flight, must synchronize first
    if (buf->dma_addr && buf->is_coherent) {
        dma_sync_single_for_cpu(buf->dev, buf->dma_addr, 
                                buf->size, buf->direction);
    }
    
    if (buf->cpu_addr) {
        dma_free_coherent(buf->dev, buf->size,
                         buf->cpu_addr, buf->dma_addr);
    }
    
    kfree(buf);
    return 0;
}

static ssize_t dma_driver_write(struct file *file, const char __user *ubuf,
                               size_t count, loff_t *offset)
{
    struct dma_buffer *buf = file->private_data;
    
    if (count > DMA_BUFFER_SIZE)
        count = DMA_BUFFER_SIZE;
    
    // Step 1: Allocate DMA-safe buffer
    if (!buf->cpu_addr) {
        buf->size = count;
        buf->direction = DMA_TO_DEVICE;
        
        // Allocate from DMA zone, must be contiguous
        buf->cpu_addr = dma_alloc_coherent(buf->dev, buf->size,
                                          &buf->dma_addr, GFP_KERNEL);
        if (!buf->cpu_addr)
            return -ENOMEM;
        
        buf->is_coherent = true;
    }
    
    // Step 2: Copy from user space to DMA buffer
    if (copy_from_user(buf->cpu_addr, ubuf, count)) {
        return -EFAULT;
    }
    
    // Step 3: Synchronize cache for DMA (flush CPU cache)
    // In coherent allocation, this is implicit, but explicit is safer:
    dma_sync_single_for_device(buf->dev, buf->dma_addr, count,
                              buf->direction);
    
    // Step 4: Issue DMA to device (implementation specific)
    // device_write_dma(device, buf->dma_addr, count);
    
    return count;
}

static ssize_t dma_driver_read(struct file *file, char __user *ubuf,
                              size_t count, loff_t *offset)
{
    struct dma_buffer *buf = file->private_data;
    
    if (!buf->cpu_addr || !buf->dma_addr)
        return 0;  // No DMA buffer yet
    
    // Step 1: Invalidate CPU cache (discard stale lines)
    dma_sync_single_for_cpu(buf->dev, buf->dma_addr, buf->size,
                           DMA_FROM_DEVICE);
    
    // Step 2: Copy from DMA buffer to user space
    if (copy_to_user(ubuf, buf->cpu_addr, buf->size))
        return -EFAULT;
    
    return buf->size;
}

static const struct file_operations dma_fops = {
    .open    = dma_driver_open,
    .release = dma_driver_release,
    .read    = dma_driver_read,
    .write   = dma_driver_write,
};
```

### Advanced: Ring Buffer with Descriptor Chains

```c
// dma_ring_buffer.c - Circular queue for high-throughput DMA

#include <linux/module.h>
#include <linux/dma-mapping.h>
#include <linux/spinlock.h>

#define RING_SIZE 256

struct dma_descriptor {
    uint64_t addr;
    uint32_t length;
    uint16_t flags;
#define DESC_OWN_DEVICE    (1 << 15)  // Device owns this descriptor
#define DESC_END_OF_PACKET (1 << 14)  // End of packet
    uint16_t status;  // Set by device upon completion
};

// Ensure descriptor is exactly 16 bytes and cache-aligned
_Static_assert(sizeof(struct dma_descriptor) == 16, "Descriptor size");

struct dma_ring {
    struct dma_descriptor *descriptors;
    dma_addr_t desc_dma;
    
    void **data_buffers;           // Pointers to actual data
    dma_addr_t *data_dma_addrs;    // DMA addresses of data
    
    uint32_t head;                 // Driver reads from head (device writes)
    uint32_t tail;                 // Driver writes at tail
    
    spinlock_t lock;
    struct device *dev;
    enum dma_data_direction direction;
    
} _Alignas(CACHE_LINE);

static int dma_ring_init(struct dma_ring *ring, struct device *dev,
                        size_t data_size, enum dma_data_direction dir)
{
    ring->dev = dev;
    ring->direction = dir;
    ring->head = 0;
    ring->tail = 0;
    spin_lock_init(&ring->lock);
    
    // Allocate descriptor table (must be DMA-coherent)
    ring->descriptors = dma_alloc_coherent(dev,
                                          RING_SIZE * sizeof(struct dma_descriptor),
                                          &ring->desc_dma, GFP_KERNEL);
    if (!ring->descriptors)
        return -ENOMEM;
    
    memset(ring->descriptors, 0, RING_SIZE * sizeof(struct dma_descriptor));
    
    // Allocate data buffers (one per descriptor slot)
    ring->data_buffers = kcalloc(RING_SIZE, sizeof(void*), GFP_KERNEL);
    ring->data_dma_addrs = kcalloc(RING_SIZE, sizeof(dma_addr_t), GFP_KERNEL);
    
    if (!ring->data_buffers || !ring->data_dma_addrs)
        goto err_free;
    
    for (int i = 0; i < RING_SIZE; i++) {
        ring->data_buffers[i] = dma_alloc_coherent(dev, data_size,
                                                  &ring->data_dma_addrs[i],
                                                  GFP_KERNEL);
        if (!ring->data_buffers[i])
            goto err_free;
    }
    
    return 0;
    
err_free:
    // Cleanup on error (omitted for brevity)
    return -ENOMEM;
}

static void dma_ring_enqueue(struct dma_ring *ring, const void *data, size_t len)
{
    unsigned long flags;
    uint32_t next_tail;
    struct dma_descriptor *desc;
    
    spin_lock_irqsave(&ring->lock, flags);
    
    next_tail = (ring->tail + 1) % RING_SIZE;
    
    // Ring full?
    if (next_tail == ring->head) {
        spin_unlock_irqrestore(&ring->lock, flags);
        return;  // Would need to drop packet or wait
    }
    
    // Prepare descriptor
    desc = &ring->descriptors[ring->tail];
    memcpy(ring->data_buffers[ring->tail], data, len);
    
    // Synchronize: flush CPU cache → device will see data
    dma_sync_single_for_device(ring->dev,
                              ring->data_dma_addrs[ring->tail],
                              len, ring->direction);
    
    // Write descriptor (device will see this)
    wmb();  // Memory barrier: ensure data written before descriptor
    
    desc->addr = ring->data_dma_addrs[ring->tail];
    desc->length = len;
    desc->flags = DESC_OWN_DEVICE;
    
    if (next_tail == ring->head) {
        desc->flags |= DESC_END_OF_PACKET;
    }
    
    wmb();  // Ensure device sees descriptor update
    ring->tail = next_tail;
    
    // Ring doorbell (device-specific, example):
    // writel(ring->tail, device->doorbell_register);
    
    spin_unlock_irqrestore(&ring->lock, flags);
}

static int dma_ring_dequeue(struct dma_ring *ring, void *output, size_t *len)
{
    unsigned long flags;
    struct dma_descriptor *desc;
    
    spin_lock_irqsave(&ring->lock, flags);
    
    if (ring->head == ring->tail) {
        spin_unlock_irqrestore(&ring->lock, flags);
        return 0;  // Ring empty
    }
    
    desc = &ring->descriptors[ring->head];
    
    // Is device still processing?
    rmb();  // Read barrier: ensure we see latest status
    if (desc->flags & DESC_OWN_DEVICE) {
        spin_unlock_irqrestore(&ring->lock, flags);
        return 0;  // Device still owns, not complete
    }
    
    // Device released the descriptor
    *len = desc->status & 0xFFFF;  // Extract actual length
    
    // Invalidate CPU cache: discard stale lines from device write
    dma_sync_single_for_cpu(ring->dev,
                           ring->data_dma_addrs[ring->head],
                           *len, ring->direction);
    
    memcpy(output, ring->data_buffers[ring->head], *len);
    
    // Mark as consumed
    desc->flags &= ~DESC_OWN_DEVICE;
    ring->head = (ring->head + 1) % RING_SIZE;
    
    spin_unlock_irqrestore(&ring->lock, flags);
    return 1;  // Data available
}
```

### Handling Non-Coherent Devices

```c
// For devices that don't support hardware cache coherency
// (e.g., some ARM SoCs, older processors)

enum {
    DMA_MEM_WRITE = 1,  // We wrote, device will read
    DMA_MEM_READ = 2,   // Device wrote, we will read
};

struct non_coherent_dma {
    void *cpu_addr;
    dma_addr_t dma_addr;
    size_t size;
    int op;
};

void sync_before_dma(struct non_coherent_dma *buf)
{
    // We wrote data, cache has dirty lines
    // Device will read from main memory, not cache
    // Must flush cache to main memory
    
    #ifdef CONFIG_ARM
    // ARM: L2 cache flush
    __cpuc_flush_dcache_area(buf->cpu_addr, buf->size);
    #elif defined(CONFIG_X86)
    // x86: clflush each cache line (or wbinvd if no WBINVD allowed)
    unsigned long addr = (unsigned long)buf->cpu_addr;
    for (; addr < (unsigned long)buf->cpu_addr + buf->size;
         addr += CACHE_LINE_SIZE) {
        clflush((void*)addr);
    }
    #endif
}

void sync_after_dma(struct non_coherent_dma *buf)
{
    // Device wrote data, we will read from cache
    // Cache has stale lines (device updated main memory)
    // Must invalidate cache so we see device's updates
    
    #ifdef CONFIG_ARM
    // ARM: Invalidate cache range
    __cpuc_flush_dcache_area(buf->cpu_addr, buf->size);
    #elif defined(CONFIG_X86)
    // x86: Flush + invalidate (clflush does both)
    unsigned long addr = (unsigned long)buf->cpu_addr;
    for (; addr < (unsigned long)buf->cpu_addr + buf->size;
         addr += CACHE_LINE_SIZE) {
        clflush((void*)addr);
    }
    #endif
}

void use_non_coherent_dma(struct non_coherent_dma *buf)
{
    // Write to buffer
    memset(buf->cpu_addr, 0xAA, buf->size);
    
    // Synchronize before device reads
    sync_before_dma(buf);
    
    // Issue DMA command to device
    // device_dma_read(buf->dma_addr, buf->size);
    
    // Wait for completion (interrupt or polling)
    // while (!device_dma_done());
    
    // Synchronize after device writes
    sync_after_dma(buf);
    
    // Now read from buffer
    uint8_t *data = buf->cpu_addr;
    // process data...
}
```

---

## Rust Implementation Guide

### Basic Safe Wrapper

```rust
// dma_safe.rs
use core::ptr::NonNull;
use core::marker::PhantomData;

/// A wrapper around DMA-allocated memory
/// Ensures alignment, pinning, and proper lifetime management
pub struct DmaBuffer<T: ?Sized> {
    ptr: NonNull<T>,
    dma_addr: u64,
    size: usize,
    
    // Phantom data: T is invariant in this struct
    // (can't coerce types unsafely)
    _phantom: PhantomData<T>,
}

impl<T: ?Sized> DmaBuffer<T> {
    /// Allocate a new DMA buffer for a contiguous type T
    /// SAFETY: Must be called with valid device pointer
    ///         Device lifetime must outlive buffer lifetime
    pub unsafe fn new(dev: *mut core::ffi::c_void, size: usize) -> Option<Self> {
        extern "C" {
            fn dma_alloc_coherent(
                dev: *mut core::ffi::c_void,
                size: usize,
                dma_handle: *mut u64,
                flags: u32,
            ) -> *mut core::ffi::c_void;
        }
        
        const GFP_KERNEL: u32 = 0xd0;
        
        let mut dma_addr = 0u64;
        let cpu_addr = dma_alloc_coherent(dev, size, &mut dma_addr, GFP_KERNEL);
        
        NonNull::new(cpu_addr as *mut T).map(|ptr| DmaBuffer {
            ptr,
            dma_addr,
            size,
            _phantom: PhantomData,
        })
    }
    
    /// Get the CPU virtual address
    #[inline]
    pub fn cpu_addr(&self) -> *mut T {
        self.ptr.as_ptr()
    }
    
    /// Get the device-visible bus address
    #[inline]
    pub fn dma_addr(&self) -> u64 {
        self.dma_addr
    }
    
    /// Get the size in bytes
    #[inline]
    pub fn size(&self) -> usize {
        self.size
    }
    
    /// Synchronize for device access (flush cache)
    pub unsafe fn sync_for_device(&self) {
        extern "C" {
            fn dma_sync_single_for_device(
                dev: *mut core::ffi::c_void,
                addr: u64,
                size: usize,
                direction: u32,
            );
        }
        const DMA_TO_DEVICE: u32 = 1;
        
        dma_sync_single_for_device(core::ptr::null_mut(), self.dma_addr, self.size, DMA_TO_DEVICE);
    }
    
    /// Synchronize for CPU access (invalidate cache)
    pub unsafe fn sync_for_cpu(&self) {
        extern "C" {
            fn dma_sync_single_for_cpu(
                dev: *mut core::ffi::c_void,
                addr: u64,
                size: usize,
                direction: u32,
            );
        }
        const DMA_FROM_DEVICE: u32 = 2;
        
        dma_sync_single_for_cpu(core::ptr::null_mut(), self.dma_addr, self.size, DMA_FROM_DEVICE);
    }
}

impl<T: ?Sized> Drop for DmaBuffer<T> {
    fn drop(&mut self) {
        unsafe {
            extern "C" {
                fn dma_free_coherent(
                    dev: *mut core::ffi::c_void,
                    size: usize,
                    cpu_addr: *mut core::ffi::c_void,
                    dma_addr: u64,
                );
            }
            
            dma_free_coherent(core::ptr::null_mut(), self.size, self.ptr.as_ptr() as _, self.dma_addr);
        }
    }
}

// SAFETY: DmaBuffer is Send if T is Send
//         The underlying allocation is pinned
unsafe impl<T: Send + ?Sized> Send for DmaBuffer<T> {}

// SAFETY: DmaBuffer is Sync if T is Sync
unsafe impl<T: Sync + ?Sized> Sync for DmaBuffer<T> {}

// Example usage:
pub fn example() {
    unsafe {
        let buf: DmaBuffer<[u8; 4096]> = DmaBuffer::new(core::ptr::null_mut(), 4096)
            .expect("allocation failed");
        
        // Write to CPU address
        core::ptr::write_bytes(buf.cpu_addr(), 0, 4096);
        
        // Flush for device
        buf.sync_for_device();
        
        // Device reads at buf.dma_addr()
        
        // After device write, invalidate cache
        buf.sync_for_cpu();
        
        // Read from CPU address (sees device's updates)
    }
}
```

### Ring Buffer in Rust

```rust
// dma_ring.rs
use alloc::vec::Vec;
use core::cell::UnsafeCell;
use core::marker::PhantomPinned;
use core::pin::Pin;
use core::sync::atomic::{AtomicU32, Ordering};

#[repr(C)]
pub struct DmaDescriptor {
    pub addr: u64,
    pub length: u32,
    pub flags: u16,
    pub status: u16,
}

const DESC_OWN_DEVICE: u16 = 1 << 15;
const DESC_END_OF_PACKET: u16 = 1 << 14;
const RING_SIZE: usize = 256;

pub struct DmaRing {
    descriptors: DmaBuffer<[DmaDescriptor; RING_SIZE]>,
    data_buffers: Vec<DmaBuffer<[u8; 4096]>>,
    
    head: AtomicU32,
    tail: AtomicU32,
    
    _pin: PhantomPinned,
}

impl DmaRing {
    pub fn new() -> Result<Self, ()> {
        unsafe {
            let desc_buf = DmaBuffer::new(core::ptr::null_mut(), RING_SIZE * core::mem::size_of::<DmaDescriptor>())
                .ok_or(())?;
            
            let mut data_buffers = Vec::new();
            for _ in 0..RING_SIZE {
                data_buffers.push(DmaBuffer::new(core::ptr::null_mut(), 4096).ok_or(())?);
            }
            
            Ok(DmaRing {
                descriptors: desc_buf,
                data_buffers,
                head: AtomicU32::new(0),
                tail: AtomicU32::new(0),
                _pin: PhantomPinned,
            })
        }
    }
    
    pub fn enqueue(&mut self, data: &[u8]) -> Result<(), ()> {
        let head = self.head.load(Ordering::Acquire);
        let mut tail = self.tail.load(Ordering::Acquire);
        let next_tail = (tail + 1) % RING_SIZE as u32;
        
        if next_tail == head {
            return Err(()); // Ring full
        }
        
        let idx = tail as usize;
        unsafe {
            let dst = core::slice::from_raw_parts_mut(
                self.data_buffers[idx].cpu_addr() as *mut u8,
                4096,
            );
            
            if data.len() > 4096 {
                return Err(());
            }
            
            dst[..data.len()].copy_from_slice(data);
            self.data_buffers[idx].sync_for_device();
            
            // Write barrier: ensure data visible before descriptor
            core::sync::atomic::compiler_fence(Ordering::Release);
            
            let desc = &mut (*self.descriptors.cpu_addr())[idx];
            desc.addr = self.data_buffers[idx].dma_addr();
            desc.length = data.len() as u32;
            desc.flags = DESC_OWN_DEVICE;
            
            if next_tail == head {
                desc.flags |= DESC_END_OF_PACKET;
            }
        }
        
        core::sync::atomic::compiler_fence(Ordering::Release);
        self.tail.store(next_tail, Ordering::Release);
        
        Ok(())
    }
    
    pub fn dequeue(&mut self, output: &mut [u8]) -> Result<usize, ()> {
        let mut head = self.head.load(Ordering::Acquire);
        let tail = self.tail.load(Ordering::Acquire);
        
        if head == tail {
            return Err(()); // Ring empty
        }
        
        unsafe {
            let desc = &(*self.descriptors.cpu_addr())[head as usize];
            
            // Read barrier: ensure we see device updates
            core::sync::atomic::compiler_fence(Ordering::Acquire);
            
            if desc.flags & DESC_OWN_DEVICE != 0 {
                return Err(()); // Device still processing
            }
            
            let len = (desc.status & 0xFFFF) as usize;
            
            self.data_buffers[head as usize].sync_for_cpu();
            
            let src = core::slice::from_raw_parts(
                self.data_buffers[head as usize].cpu_addr() as *const u8,
                len.min(output.len()),
            );
            
            output[..src.len()].copy_from_slice(src);
            
            head = (head + 1) % RING_SIZE as u32;
            self.head.store(head, Ordering::Release);
            
            Ok(len)
        }
    }
}

// Example usage (Rust driver context):
pub fn rust_driver_example() -> Result<(), ()> {
    let mut ring = DmaRing::new()?;
    
    // Send data to device
    let data = b"Hello, Device!";
    ring.enqueue(data)?;
    
    // Wait for device (would normally be interrupt-driven)
    // Wait for completion...
    
    // Receive from device
    let mut output = [0u8; 256];
    let received_len = ring.dequeue(&mut output)?;
    
    // Use output[..received_len]
    Ok(())
}
```

### Memory Ordering in Rust DMA

```rust
// dma_ordering.rs
use core::sync::atomic::Ordering;

/// Rust provides atomics but not volatile for structs
/// For DMA, we need volatile semantics

pub struct VolatileDescriptor {
    addr: core::cell::UnsafeCell<u64>,
    flags: core::cell::UnsafeCell<u32>,
}

impl VolatileDescriptor {
    #[inline]
    pub fn read_addr(&self) -> u64 {
        unsafe { core::ptr::read_volatile(self.addr.get()) }
    }
    
    #[inline]
    pub fn write_addr(&self, val: u64) {
        unsafe { core::ptr::write_volatile(self.addr.get(), val) }
    }
    
    #[inline]
    pub fn read_flags(&self) -> u32 {
        unsafe { core::ptr::read_volatile(self.flags.get()) }
    }
    
    #[inline]
    pub fn write_flags(&self, val: u32) {
        unsafe { core::ptr::write_volatile(self.flags.get(), val) }
    }
}

/// Helper: ensure all previous writes are visible
/// (memory barrier for ordering)
pub fn dma_write_barrier() {
    // Acquire semantics: all previous writes are before this point
    core::sync::atomic::compiler_fence(Ordering::Release);
}

/// Helper: ensure we see all previous device writes
pub fn dma_read_barrier() {
    // Release semantics: all subsequent reads see updates from before
    core::sync::atomic::compiler_fence(Ordering::Acquire);
}

/// Correct pattern for DMA descriptor ownership transfer:
///
/// CPU to Device:
/// 1. Write data
/// 2. dma_write_barrier()
/// 3. Set OWN bit in descriptor
/// 4. Ring doorbell
///
/// Device to CPU:
/// 1. dma_read_barrier()
/// 2. Check if device cleared OWN bit
/// 3. Read data from buffer

pub fn transfer_to_device(desc: &VolatileDescriptor, data: &[u8], dma_addr: u64) {
    // 1. Write data to buffer (before barrier)
    // (done outside this function)
    
    // 2. Memory barrier
    dma_write_barrier();
    
    // 3. Set descriptor (device sees this after barrier)
    desc.write_addr(dma_addr);
    desc.write_flags(0x8000); // OWN bit set
    
    // Doorbell write (device-specific)
}

pub fn transfer_from_device(desc: &VolatileDescriptor) -> Option<usize> {
    // 1. Memory barrier (see device updates)
    dma_read_barrier();
    
    // 2. Check ownership
    let flags = desc.read_flags();
    if (flags & 0x8000) != 0 {
        return None; // Device still owns
    }
    
    // 3. Read data (now valid)
    let len = desc.read_addr() as usize;
    
    Some(len)
}
```

---

## Real-World Scenarios

### Scenario 1: Ethernet NIC Driver

```
Real hardware: Intel 82599 10GbE NIC

Memory layout for RX ring:
┌────────────────────────────────┐
│ Descriptor Ring (DMA-coherent) │
│ ├─ Descriptor 0: RX buffer 0   │
│ ├─ Descriptor 1: RX buffer 1   │
│ └─ ...                          │
│                                │
│ RX Buffers (one per descriptor)│
│ ├─ Buffer 0: 2048 bytes        │
│ ├─ Buffer 1: 2048 bytes        │
│ └─ ...                          │
└────────────────────────────────┘

Operations:

1. RX Setup:
   for i = 0 to RING_SIZE-1:
       alloc page (contiguous)
       buf[i] = page virtual address
       buf_dma[i] = page physical address
       desc[i].addr = buf_dma[i]
       desc[i].length = PAGE_SIZE
       desc[i].flags = 0  (CPU owns initially)

2. Enable RX:
   Write RDBAL (descriptor ring base low)
   Write RDBAH (descriptor ring base high)
   Write RDLEN (descriptor ring length)
   Set RXDCTL.ENABLE

3. Device receives packet:
   for each packet:
       device.dma_write(desc[RDT].addr, packet)
       set desc[RDT].STATUS = DD (descriptor done)
       increment RDT register

4. Driver polls/interrupt:
   if desc[RDH].STATUS & DD:
       invalidate_cache(buf[RDH])
       process_packet(buf[RDH])
       refill: allocate new buffer, update desc[RDT]

5. RX path (driver -> kernel):
   netif_receive_skb(skb)
       -> TCP/IP stack processes packet
```

C implementation skeleton:

```c
#define RX_RING_SIZE 256

struct rx_ring {
    struct rx_desc *desc;      // DMA coherent
    dma_addr_t desc_dma;
    
    struct sk_buff **skbs;     // Software ring of skb pointers
    void **buffers;
    dma_addr_t *buffer_dmas;
    
    uint32_t next_to_use;      // Next descriptor driver will fill
    uint32_t next_to_clean;    // Next descriptor to process
};

static int setup_rx(struct net_device *dev) {
    struct rx_ring *rx = &driver->rx;
    
    // Allocate descriptors
    rx->desc = dma_alloc_coherent(&dev->dev,
                                 RX_RING_SIZE * sizeof(struct rx_desc),
                                 &rx->desc_dma, GFP_KERNEL);
    
    // Allocate buffers
    for (int i = 0; i < RX_RING_SIZE; i++) {
        rx->buffers[i] = dma_alloc_coherent(&dev->dev, 2048,
                                           &rx->buffer_dmas[i], GFP_KERNEL);
        
        rx->desc[i].addr = rx->buffer_dmas[i];
        rx->desc[i].length = 2048;
        rx->desc[i].flags = 0;  // CPU owns
    }
    
    // Program hardware
    writel(rx->desc_dma & 0xFFFFFFFF, RDBAL);
    writel(rx->desc_dma >> 32, RDBAH);
    writel(RX_RING_SIZE * sizeof(struct rx_desc), RDLEN);
    writel(RX_RING_SIZE - 1, RDT);  // Hardware can fill up to RDT
    
    return 0;
}

static int rx_poll(struct napi_struct *napi, int budget) {
    struct rx_ring *rx = container_of(napi, struct rx_ring, napi);
    int packets = 0;
    
    while (packets < budget) {
        uint32_t next = rx->next_to_clean;
        struct rx_desc *desc = &rx->desc[next];
        
        // Check if device is done with this descriptor
        rmb();  // Read barrier
        if (!(desc->flags & DD))
            break;  // Device still processing
        
        // Invalidate cache: device wrote to this buffer
        dma_sync_single_for_cpu(&dev->dev, rx->buffer_dmas[next],
                               2048, DMA_FROM_DEVICE);
        
        // Create skb and pass to stack
        struct sk_buff *skb = netdev_alloc_skb(netdev, 2048);
        memcpy(skb_put(skb, desc->length), rx->buffers[next], desc->length);
        
        netif_receive_skb(skb);
        packets++;
        
        // Clear descriptor status
        desc->flags &= ~DD;
        
        // Allocate new buffer (replace consumed one)
        void *new_buf = dma_alloc_coherent(&dev->dev, 2048,
                                          &rx->buffer_dmas[next], GFP_KERNEL);
        rx->buffers[next] = new_buf;
        desc->addr = rx->buffer_dmas[next];
        
        // Update hardware tail pointer
        rx->next_to_clean = (next + 1) % RX_RING_SIZE;
        writel(rx->next_to_clean, RDT);
    }
    
    if (packets < budget) {
        napi_complete(napi);
        writel(EIMS_RX, EIMS);  // Re-enable interrupts
    }
    
    return packets;
}
```

### Scenario 2: GPU VRAM Transfer

```
GPU (separate address space, no-snoop capable):

Host Memory (CPU-visible):
┌──────────────────────────┐
│ DMA Transfer Buffer      │
│ (pinned, coherent)       │
│ ├─ 256MB allocation      │
│ ├─ Single contiguous PA  │
│ └─ CPU address: 0x800...│
│                          │
│ Purpose: Hold staging    │
│ data before GPU xfer     │
└──────────────────────────┘

GPU Memory (GPU-visible):
┌──────────────────────────┐
│ VRAM                     │
│ (PCIe BAR2, aperture)    │
│ ├─ Compute results       │
│ ├─ Texture maps          │
│ └─ Framebuffer           │
└──────────────────────────┘

Transfer path (CPU → GPU):
1. CPU writes to staging buffer (standard memory)
2. clflush() entire buffer (no-snoop: device won't snoop)
3. GPU DMA engine reads from PA (direct access, no IOMMU)
4. GPU writes to VRAM

Transfer path (GPU → CPU):
1. GPU writes to VRAM
2. GPU issues write fence
3. CPU clflush() + clflush_opt() to discard stale lines
4. CPU reads staging buffer

Why "no-snoop"?
GPU writes to VRAM, never reads host memory
GPU DMA marked with PCIe No-Snoop attribute
CPU can't snoop GPU VRAM (separate address space)
└─ Each side caches independently
└─ Explicit flushes required
└─ Performance: avoids cache coherency traffic (~20% faster)
```

Rust example:

```rust
pub struct GpuTransfer {
    host_buffer: DmaBuffer<[u8; 256 * 1024 * 1024]>,
    gpu_mmio: usize,  // GPU aperture address
}

impl GpuTransfer {
    pub fn upload_to_gpu(&mut self, data: &[u8], gpu_offset: u64) -> Result<(), ()> {
        if data.len() > 256 * 1024 * 1024 {
            return Err(());
        }
        
        // 1. Copy to host staging buffer
        unsafe {
            let staging = core::slice::from_raw_parts_mut(
                self.host_buffer.cpu_addr() as *mut u8,
                256 * 1024 * 1024,
            );
            staging[..data.len()].copy_from_slice(data);
            
            // 2. Flush cache (no-snoop device)
            for addr in (0..data.len()).step_by(64) {
                core::arch::x86_64::_mm_clflush(
                    (staging.as_ptr() as usize + addr) as *const _
                );
            }
            
            // 3. Issue GPU DMA command
            let cmd = GpuCommand {
                src_addr: self.host_buffer.dma_addr(),
                dst_offset: gpu_offset,
                length: data.len() as u32,
                direction: 1, // Host → GPU
            };
            
            // Write to GPU command queue
            self.issue_gpu_cmd(cmd);
        }
        
        Ok(())
    }
    
    pub fn download_from_gpu(&mut self, gpu_offset: u64, size: usize) -> Result<Vec<u8>, ()> {
        if size > 256 * 1024 * 1024 {
            return Err(());
        }
        
        unsafe {
            // 1. Issue GPU DMA (GPU → Host)
            let cmd = GpuCommand {
                src_offset: gpu_offset,
                dst_addr: self.host_buffer.dma_addr(),
                length: size as u32,
                direction: 2, // GPU → Host
            };
            
            self.issue_gpu_cmd(cmd);
            
            // 2. Wait for GPU (would be interrupt or polling)
            // self.wait_gpu_completion();
            
            // 3. Invalidate cache (discard stale host lines)
            for addr in (0..size).step_by(64) {
                core::arch::x86_64::_mm_clflush(
                    (self.host_buffer.cpu_addr() as usize + addr) as *const _
                );
            }
            
            // 4. Copy from staging to output
            let staging = core::slice::from_raw_parts(
                self.host_buffer.cpu_addr() as *const u8,
                size,
            );
            
            let mut output = Vec::new();
            output.extend_from_slice(staging);
            
            Ok(output)
        }
    }
    
    fn issue_gpu_cmd(&self, cmd: GpuCommand) {
        // Write to GPU command register at aperture
        unsafe {
            let cmd_ptr = (self.gpu_mmio + 0x1000) as *mut u32;
            core::ptr::write_volatile(cmd_ptr, cmd.src_offset as u32);
            core::ptr::write_volatile(cmd_ptr.add(1), cmd.dst_offset as u32);
            core::ptr::write_volatile(cmd_ptr.add(2), cmd.length);
            core::ptr::write_volatile(cmd_ptr.add(3), cmd.direction);
        }
    }
}
```

---

## Debugging and Performance

### Kernel Debugging: Finding DMA Issues

```bash
# 1. Check DMA zone allocation pressure
cat /proc/buddyinfo
# Output: Node 0, zone      DMA      DMA32    Normal
#            0          1        2      3        10

# Large fragmentation = allocation failures likely

# 2. Monitor page allocations
cat /proc/vmstat | grep dma
dma_alloc_failed  0              # Failures to allocate
dma_free          12345          # Freed allocations

# 3. Trace DMA operations (ftrace/tracepoints)
cd /sys/kernel/debug/tracing

# Enable DMA mapping tracepoints
echo 1 > events/dma_fence/enable
echo 1 > events/iommu/enable

# Record trace
echo > trace
# ... run workload ...
cat trace > /tmp/dma_trace.txt

# Analyze
grep dma_alloc /tmp/dma_trace.txt | head -20

# 4. Check IOMMU status (Intel VT-d)
dmesg | grep -i iommu
# Should see:
# DMAR: IOMMU 0: reg_base_addr fed90000 ver 1:0 cap d2078c206f0 ecap f010da
# DMAR: RMRR base: 0x000000dd800000 end: 0x000000df7fffff

# 5. Check device IOMMU groups (important for passthrough)
cat /sys/kernel/iommu_groups/0/devices/0000:00:01.0

# 6. Monitor cache line bouncing
perf stat -e LLC-load-misses,LLC-stores,LLC-store-misses workload

# 7. Check NUMA locality
numactl --hardware
cat /proc/zoneinfo | grep -A 5 "Node 0"

# 8. Verify alignment of DMA buffers
gdb> print sizeof(struct dma_buffer)
$1 = 64  # Good: cache-aligned

# 9. Check if pages are pinned
cat /proc/meminfo | grep Mlocked
# Should include DMA buffer allocations

# 10. Trace page migrations (would fail for pinned pages)
echo 'p:page_migrate migrate_misfit_req' > /sys/kernel/debug/tracing/kprobe_events
```

### Performance Analysis

```bash
# Bandwidth measurement
# Test coherent DMA throughput

#include <stdio.h>
#include <time.h>
#include <sys/ioctl.h>

#define DMA_SIZE (1024 * 1024 * 256)  // 256MB

struct timespec start, end;

clock_gettime(CLOCK_MONOTONIC, &start);

// Issue DMA write (device → CPU)
ioctl(device_fd, IOCTL_DMA_READ, DMA_SIZE);

clock_gettime(CLOCK_MONOTONIC, &end);

double elapsed = (end.tv_sec - start.tv_sec) +
                (end.tv_nsec - start.tv_nsec) / 1e9;
double bandwidth = (DMA_SIZE / (1024*1024)) / elapsed;

printf("DMA bandwidth: %.2f MB/s\n", bandwidth);

// Expected:
// - PCIe 3.0 x16: ~12 GB/s
// - PCIe 4.0 x16: ~24 GB/s
// - PCIe 5.0 x16: ~48 GB/s

# Latency measurement
# Use CPU TSC to measure device response time

unsigned long tsc_start = rdtsc();
device_trigger_interrupt();
unsigned long tsc_end = rdtsc();

// Subtract TSC to CPU cycles
unsigned long cycles = tsc_end - tsc_start;
double latency_us = cycles / (cpu_ghz * 1000);

printf("Interrupt latency: %.2f us\n", latency_us);

// Expected: 1-10 microseconds (depending on interrupt type)
```

### Common DMA Bugs

```c
// Bug 1: Forgetting to flush cache
void bad_dma_write(void) {
    struct dma_buffer buf;
    buf.ptr[0] = 42;
    // BUG: CPU cache has dirty line, device sees old value
    device_dma_read(&buf.dma_addr);
    // Fix:
    dma_sync_single_for_device(&dev, buf.dma_addr, buf.size, DMA_TO_DEVICE);
}

// Bug 2: Using wrong virtual address
void bad_dma_addr(void) {
    void *vaddr = dma_alloc_coherent(..., &dma_addr);
    
    // BUG: Using vaddr when device can only see dma_addr
    write_device_register((unsigned long)vaddr);
    
    // Fix:
    write_device_register(dma_addr);
}

// Bug 3: Reading while device is still writing
void bad_dma_read(void) {
    while (!device_done());  // Wait for device...
    // BUG: No cache invalidation, reads stale value
    uint32_t result = *(uint32_t*)buf.cpu_addr;
    
    // Fix:
    dma_sync_single_for_cpu(&dev, buf.dma_addr, buf.size, DMA_FROM_DEVICE);
    result = *(uint32_t*)buf.cpu_addr;
}

// Bug 4: Freeing while DMA in flight
void bad_dma_free(void) {
    device_dma_read(buf.dma_addr);
    dma_free_coherent(..., buf.cpu_addr, buf.dma_addr);  // BUG: immediate free
    
    // Fix:
    device_dma_read(buf.dma_addr);
    wait_for_completion(&dma_done);
    dma_free_coherent(..., buf.cpu_addr, buf.dma_addr);
}

// Bug 5: Assuming cache coherency
void bad_non_coherent(void) {
    memcpy(buffer, data, size);
    // BUG: On ARM, device doesn't snoop CPU cache
    device_dma_read(dma_addr);  // Device sees garbage
    
    // Fix:
    memcpy(buffer, data, size);
    __cpuc_flush_dcache_area(buffer, size);  // ARM-specific
    device_dma_read(dma_addr);
}
```

---

## Advanced Topics

### Scatter-Gather DMA

For non-contiguous buffers:

```c
// Scatter-gather descriptor
struct scatterlist {
    unsigned long page_link;  // Pointer to page + flags
    unsigned int offset;      // Offset within page
    unsigned int length;      // Bytes in this entry
    dma_addr_t dma_address;   // DMA address (filled by sg_map)
};

// Map sg list for DMA
int sg_count = dma_map_sg(dev, sg_list, nents, DMA_TO_DEVICE);

// Now each sg[i].dma_address is device-visible
for (int i = 0; i < sg_count; i++) {
    device_dma_queue(sg[i].dma_address, sg[i].length);
}

// Unmap
dma_unmap_sg(dev, sg_list, nents, DMA_TO_DEVICE);
```

### SWIOTLB (Software I/O TLB)

For devices that can't reach certain memory:

```c
// Problem: Device has limited DMA address space
// e.g., 32-bit device, but system has > 4GB RAM

// Solution: SWIOTLB (Linux kernel feature)
// - Allocates pool in addressable memory (< 4GB)
// - Copies to/from addressable pool automatically

// When mapping DMA:
dma_map_single(dev, vaddr, size, DMA_TO_DEVICE);
├─ Check: can device reach phys_addr?
├─ If NO: allocate from SWIOTLB pool
│  ├─ Copy data to pool
│  └─ Return pool's DMA address
└─ If YES: return phys DMA address directly

// Performance impact: extra copy (~10-20% slower)
// Used for: legacy 32-bit devices, embedded systems

// Check SWIOTLB status:
cat /sys/kernel/debug/swiotlb
```

### DMA with IOMMU and Device Passthrough

```c
// When device is passed to VM (KVM/QEMU):

// Host kernel:
struct iommu_domain *domain = iommu_domain_alloc();
iommu_attach_device(domain, pci_device);

// VM guest wants to DMA:
// Guest's GPA (guest physical address)
//   ↓ (VM page table)
// HPA (host physical address)
//   ↓ (IOMMU)
// Device PA (actual physical)

// IOMMU page table setup:
for each guest page:
    hpa = vm.translate(gpa)
    iommu_map(domain, gpa, hpa, PAGE_SIZE)
    // Device sees GPA directly (translation to HPA by IOMMU)

// Isolation: Device cannot DMA outside guest's address space
// Protection: Malicious guest can't escape VM via device DMA
```

### DPDK (Data Plane Development Kit) Approach

```c
// DPDK pre-allocates huge pages for performance:
// - Huge pages: 2MB or 1GB (large TLB entries)
// - Reduced IOMMU overhead (fewer page table lookups)
// - Reduced cache misses (fewer PTW failures)

// DPDK allocation:
struct rte_mempool *pool = rte_mempool_create(
    "mbufs", 256, 
    sizeof(struct rte_mbuf),
    0, 0, NULL, NULL, 
    alloc_func,  // Custom allocator using hugetlb
    free_func, 0, 0
);

// Hugetlb backing:
// /sys/kernel/mm/transparent_hugepage/hpage_pmd_size
// /proc/meminfo: HugePages_Total, HugePages_Free

// Benefits:
// - 2MB pages: 256 × fewer IOMMU entries
// - 1GB pages: 512 × fewer IOMMU entries
// - Reduced IOTLB misses
// - Better TLB hit rate
```

### Lock-Free DMA Ring Buffers

```c
// Non-blocking enqueue/dequeue for multi-producer/consumer

struct lfq_ring {
    char *descriptors;          // Aligned to cache line
    volatile uint64_t head;     // Modified by dequeue
    char _pad1[CACHE_LINE - 8];
    volatile uint64_t tail;     // Modified by enqueue
    char _pad2[CACHE_LINE - 8];
};

// Producer (CPU A) enqueues without lock
void enqueue(struct lfq_ring *ring, void *item) {
    uint64_t next = (ring->tail + 1) % RING_SIZE;
    
    // Load-acquire: see any previous state
    uint64_t current_head = __atomic_load_n(&ring->head, __ATOMIC_ACQUIRE);
    
    if (next == current_head) {
        return;  // Ring full, drop packet
    }
    
    // Store item
    ring->descriptors[ring->tail] = item;
    
    // Release-store: make visible to other cores
    __atomic_store_n(&ring->tail, next, __ATOMIC_RELEASE);
}

// Consumer (CPU B) dequeues without lock
int dequeue(struct lfq_ring *ring, void **item) {
    // Load-acquire
    uint64_t current_tail = __atomic_load_n(&ring->tail, __ATOMIC_ACQUIRE);
    uint64_t current_head = ring->head;
    
    if (current_head == current_tail) {
        return 0;  // Ring empty
    }
    
    // Read item
    *item = ring->descriptors[current_head];
    
    // Release-store
    __atomic_store_n(&ring->head, (current_head + 1) % RING_SIZE, __ATOMIC_RELEASE);
    
    return 1;
}

// Key insights:
// - Head modified only by consumer
// - Tail modified only by producer
// - Padding separates them to different cache lines (no false sharing)
// - Atomic loads/stores + memory ordering semantics
// - Lock-free: no mutex, no spinlock overhead
```

---

## Summary

### Key Takeaways

1. **DMA bypasses CPU**: Direct device ↔ memory transfers, no CPU overhead
2. **Physical Contiguity**: Devices see physical addresses, must be contiguous
3. **Cache Coherency**: Explicit synchronization needed (clflush, dma_sync)
4. **IOMMU**: Optional hardware isolation, translates device VA to physical
5. **Alignment**: Cache line (64B) alignment prevents false sharing
6. **Pinning**: Memory must be locked, not swapped or migrated
7. **Ownership**: Clear semantics: who can read/write at any time
8. **Ordering**: Memory barriers (wmb, rmb) for descriptor hand-off
9. **NUMA**: Allocate from correct node for locality
10. **Debugging**: trace-cmd, perf, /proc/buddyinfo, gdb + kgdb

### Mental Model

```
Think of DMA as:
    
    CPU Memory Space          Device Memory Space
    (virtual, cached)         (physical, no cache)
         ↓ VA                       ↓ PA/IOVA
    ┌─────────────┐           ┌──────────────┐
    │ Application │           │   Device     │
    │   Buffer    │ ← DMA ──→ │   Buffer     │
    └─────────────┘           └──────────────┘
         ↑                          ↑
         └──────────────────────────┘
         Data flow (no CPU intervention)
         
    Synchronization points:
    1. Before device reads: flush cache (writeback)
    2. Before CPU reads: invalidate cache (discard stale)
    3. Ownership: device <→ CPU (clear barriers)
    4. Completion: interrupt or polling
```

### Performance Checklist

- [ ] Allocate from correct NUMA node (locality)
- [ ] Use DMA zone appropriate for device (DMA32 vs NORMAL)
- [ ] Align to cache lines (64B minimum)
- [ ] Batch operations (reduce interrupt overhead)
- [ ] Use coherent allocations (implicit cache ops)
- [ ] Monitor IOMMU efficiency (watch IOTLB misses)
- [ ] Pre-allocate (avoid allocation latency)
- [ ] Use huge pages for DPDK-style workloads
- [ ] Profile with perf (see cache misses, IOMMU overhead)
- [ ] Test under load (fragmentation affects allocation)

---

## References and Further Reading

- Linux Kernel DMA API: `<linux/dma-mapping.h>`
- Intel VT-d Specification (IOMMU)
- PCIe Specifications (coherency, MSI-X)
- ARM SMMU Architecture (arm-smmu.c)
- Cachegrind / Perf for profiling
- LKML discussions on DMA synchronization
- Real driver implementations: `drivers/net/ethernet/intel/ixgbe/`

---

**This guide provides the foundation for understanding and implementing DMA operations correctly and efficiently. The key is mental clarity on address spaces, cache coherency, and ownership semantics.**

