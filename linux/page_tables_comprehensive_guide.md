# Comprehensive Guide to Page Tables: Building Your Mental Model

**Audience:** System programmers, kernel engineers, cloud security specialists  
**Focus:** Virtual memory translation, kernel implementation, architecture details, production concerns

---

## Table of Contents

1. [Part 1: Foundational Concepts](#part-1-foundational-concepts)
2. [Part 2: Hardware Architecture](#part-2-hardware-architecture)
3. [Part 3: Linux Kernel Implementation](#part-3-linux-kernel-implementation)
4. [Part 4: Code Deep Dives](#part-4-code-deep-dives)
5. [Part 5: Performance, Security, and Debugging](#part-5-performance-security-and-debugging)
6. [Part 6: Real-World Scenarios](#part-6-real-world-scenarios)

---

# Part 1: Foundational Concepts

## 1.1 Why Virtual Memory Exists

### The Problem We're Solving

Before virtual memory, processes accessed **physical memory directly**. This created cascading problems:

1. **Fragmentation**: Process A allocates 1MB, then exits. Process B can't use that 1MB gap if it needs contiguous memory.
2. **Address Space Conflicts**: Two processes both assume they own address 0x1000. One overwrites the other.
3. **Process Isolation Failure**: One buggy process corrupts another's memory without restriction.
4. **No Transparency**: Processes must know where they're actually placed in physical memory.
5. **Memory Protection Impossible**: No enforced boundary between kernel and user space.

### The Solution: Virtual Memory

Each process has its own **virtual address space** (0x0 to 2^N bytes). The CPU and memory management unit (MMU) transparently translate virtual addresses (VAs) to physical addresses (PAs).

```
Process A (Virtual)    Process B (Virtual)      Physical Memory
0x0-0x1000            0x0-0x1000              0x0-0x1000
  [Code]                [Code]    ────────┐    [Page A Code]
0x1000-0x2000         0x1000-0x2000       │    0x1000-0x2000
  [Data]                [Data]    ────┐   │    [Page B Code]
                                       └──┼──► 0x2000-0x3000
                                           └──► [Page A Data]
                                          etc.
```

Each process sees a **clean 0x0 start** and a full address space, completely isolated.

---

## 1.2 Core Terminology and Concepts

### Virtual Address (VA) vs Physical Address (PA)

- **Virtual Address**: What the CPU issues when it executes `mov rax, [0x7fff1234]`
- **Physical Address**: Where the actual data sits in RAM

### Pages and Frames

- **Page**: A fixed-size unit of virtual memory (typically 4KB, but can be 2MB, 1GB)
- **Page Frame**: A fixed-size unit of physical memory

**Why fixed size?** Simpler translation, smaller lookup structures, less fragmentation.

### Page Offset

In a 4KB page system (2^12 bytes per page):

```
Virtual Address Layout (64-bit x86-64):
┌─────────────────────────────────────────┬──────────────┐
│    Page Number / Virtual Page Index     │ Page Offset  │
│           (52 bits)                     │  (12 bits)   │
├─────────────────────────────────────────┼──────────────┤
│         Translated by MMU               │  Passed      │
│                                         │  Through     │
│              (PTE)                      │  Unchanged   │
└─────────────────────────────────────────┴──────────────┘

If VA = 0x7fff1234:
  - Page offset = 0x234 (12 bits: address & 0xFFF)
  - Virtual page index = 0x7fff1 (upper bits: address >> 12)
```

The **page offset is never translated**—it's the same in the physical address. Only the page number is translated.

### Page Table Entry (PTE)

A PTE is a data structure that says: "Virtual page N is stored in physical frame M, and here are the access permissions."

```
Physical Frame Number          Permission Bits
        |                              |
        v                              v
┌─────────────────┬─────────┬─────────────────────┐
│                 │ P | W | X │ U | D │ A │Valid │
│ PFN (40-50 bits)│ │ │ │ │ │ │ │
└─────────────────┴─────────┴─────────────────────┘
    Physical address base      Permissions

P = Present (valid entry)
W = Writable
X = Executable (NX=No-eXecute bit if not set)
U = User-accessible (1=user, 0=kernel-only)
D = Dirty (page written to)
A = Accessed (page read/written)
```

---

## 1.3 The Translation Workflow

### Step-by-Step Address Translation

When the CPU executes `mov rax, [VA]`:

1. **Extract page offset**: `page_offset = VA & 0xFFF` (for 4KB pages)
2. **Extract page index**: `vpn = VA >> 12`
3. **Perform page table walk**: Use VPN to look up PTE in page tables
4. **Check PTE validity**: Is the page present? Does the access match permissions?
   - **Page Not Present**: Page fault (page on disk, swap, or unmapped)
   - **Permission Violation**: Protection fault (e.g., write to read-only page)
5. **Extract Physical Frame Number**: `pfn = PTE & 0xFFFFF000` (extract high bits)
6. **Compute PA**: `PA = (pfn << 12) | page_offset = (pfn << 12) | (VA & 0xFFF)`
7. **Access Memory**: CPU issues actual read/write to PA

### Why Page Offset Doesn't Need Translation

The offset within a page is **identical** in both virtual and physical addresses. If VA=0x7fff1234 points to page index 0x7fff and offset 0x234, and that page is in physical frame 0x1a, then:

```
PA = (0x1a << 12) | 0x234 = 0x1a000 | 0x234 = 0x1a234
```

The offset 0x234 stays the same.

---

## 1.4 Page Fault Handling

A **page fault** occurs when:

1. A PTE has `P=0` (present bit is 0)
2. Access violates permissions (write to read-only, user-space access to kernel-only, execute to non-executable)
3. Reserved bits set incorrectly

### Page Fault Handler Flow (Simplified)

```
CPU tries VA access
  │
  ├─→ MMU walks page tables, checks PTE
  │
  └─→ PTE valid and permissions match?
       │
       ├─→ YES: Access proceeds
       │
       └─→ NO: Exception raised
            │
            └─→ Kernel Page Fault Handler
                  │
                  ├─→ Is page on disk (swapped)? Load it
                  ├─→ Is page never allocated? Allocate, zero it, populate PTE
                  ├─→ Is permission violation? Kill process (segfault)
                  └─→ After handling, resume instruction
```

This is how lazy allocation works: processes allocate memory without immediately consuming physical pages. When they access an unmapped page, the handler allocates it on demand.

---

## 1.5 Multi-Level Page Tables: The Core Insight

### The Problem with Flat Page Tables

With 64-bit addresses and 4KB pages, a flat single-level page table would need:

```
Address space size = 2^48 bytes (for user-space on x86-64, canonical addressing)
Page size = 2^12 bytes (4KB)
Number of pages = 2^48 / 2^12 = 2^36 = 68 billion page table entries

If each PTE is 8 bytes:
Single-level table size = 68 billion × 8 = 544 TB per process!

Even kernel running thousands of processes: impossible.
```

### The Multi-Level Solution

Instead of one massive table, use a **tree of tables**:

```
Virtual Address (x86-64, 4-level paging):
┌──────┬──────┬──────┬──────┬──────────┐
│ L4   │ L3   │ L2   │ L1   │ Offset   │
│ (9b) │ (9b) │ (9b) │ (9b) │ (12b)    │
└──────┴──────┴──────┴──────┴──────────┘
   ↓      ↓      ↓      ↓
 PML4   PDPT   PD    PT   (Physical page)


PML4 (Page Map Level 4) - points to PDPT tables
  ↓
PDPT (Page Directory Pointer Table) - points to PD tables
  ↓
PD (Page Directory) - points to PT tables
  ↓
PT (Page Table) - points to actual physical pages
```

Each level has 512 entries (9 bits), so:
- PML4: 512 entries → covers 512 * 512GB = 256TB per level
- PDPT: 512 entries → covers 512 * 1GB = 512GB per level
- PD: 512 entries → covers 512 * 2MB = 1GB per level
- PT: 512 entries → covers 512 * 4KB = 2MB per level

**Key advantage**: Only allocate table pages when needed. If a VA range is unmapped, don't allocate any PT, PD, or PDPT entries for it.

### Multi-Level Table Walk (Example x86-64)

For VA = `0x00007fff12345678`:

```
Step 1: Extract indices from VA
  PML4 index = (VA >> 39) & 0x1FF = (0x7fff... >> 39) & 0x1FF = 0xF
  PDPT index = (VA >> 30) & 0x1FF = 0x1C
  PD index   = (VA >> 21) & 0x1FF = 0x1A
  PT index   = (VA >> 12) & 0x1FF = 0x1AD
  Offset     = VA & 0xFFF = 0x678

Step 2: Walk the tree
  1. Load PML4 from CR3 register (root table)
  2. Access PML4[0xF] → get PDPT table address
  3. Access PDPT[0x1C] → get PD table address
  4. Access PD[0x1A] → get PT table address
  5. Access PT[0x1AD] → get PTE with physical frame number
  6. Compute PA = (PFN << 12) | 0x678

Step 3: Access physical address
  RAM[PA] ← data (if write) or data ← RAM[PA] (if read)
```

**Important:** Each level walk is another memory access. This is why TLBs (Translation Lookaside Buffers) exist—to cache PTEs.

---

## 1.6 Huge Pages (2MB, 1GB)

### Standard vs Huge Pages

Standard 4KB pages:
```
2MB range uses 512 PTEs (512 × 4KB)
Each PTE is 8 bytes
Overhead: 512 × 8 = 4KB to map 2MB
```

2MB huge pages:
```
One PTE in PD directly points to 2MB physical frame
No PT level needed
Overhead: 8 bytes to map 2MB
Reduction: 512× less TLB pressure, 4× fewer table walks
```

### When PT Entry Becomes Huge Page

In x86-64 PD, if a PTE has the `PS` bit (Page Size) set to 1, that PTE directly points to a **2MB physical frame**, not a PT table.

```
Standard 4KB mapping (PD entry):
┌──────────────────┬─────────┐
│ PT Table Address │ PS=0 │ ... │
└──────────────────┴─────────┘

Huge 2MB mapping (PD entry):
┌──────────────────┬─────────┐
│ 2MB Frame Addr   │ PS=1 │ ... │
└──────────────────┴─────────┘
```

Similarly, PDPT with PS=1 can map 1GB directly.

---

# Part 2: Hardware Architecture

## 2.1 x86-64 Paging in Detail

### CR3 Register: Root of the Tree

The **CR3 register** (Control Register 3) contains the physical base address of the PML4 table.

```c
// When a process context switches:
// 1. Save current process's CR3 value
// 2. Load new process's CR3 value into CR3 register
// This immediately switches the address space

asm volatile("mov %0, %%cr3" : : "r" (new_pml4_pa));
```

**Critical**: Each process has its own PML4 (and often its own page table tree), but the kernel portion (high addresses) is typically shared across all processes.

### x86-64 PTE Format (Canonical)

```
Bits 63:52  Reserved (must be 0 for canonical addresses)
Bits 51:12  Physical Page Frame Number (52-bit physical address support)
Bits 11:9   Available for OS use
Bit  8      Global (G) - TLB not flushed on CR3 change if set
Bit  7      Page Size (PS) - if 1, maps 2MB (PD) or 1GB (PDPT)
Bit  6      Dirty (D) - set by CPU if page written to
Bit  5      Accessed (A) - set by CPU if page accessed
Bit  4      Cache Disabled (CD) - if 1, disable caching for this page
Bit  3      Write-Through (WT) - if 1, write updates immediately to memory
Bit  2      User/Supervisor (U/S) - 1=user-space, 0=kernel-only
Bit  1      Write (W) - 1=writable, 0=read-only
Bit  0      Present (P) - 1=page in memory, 0=not in memory (page fault)

Example PTE for a user-space, writable, present page:
0x00000000_12345f07
  │         │      │
  │         │      └─ P=1 (present), W=1 (writable), U=1 (user)
  │         │         A=1 (accessed), D=1 (dirty)
  │         └─────── Physical frame 0x12345
  └───────────────── 52 bits of zeros (canonical)
```

### TLB: Translation Lookaside Buffer

The **TLB** is a hardware cache of PTE entries. Without it, every memory access would require 4 table walks (L4→L3→L2→L1), multiplying memory latency.

```
Modern CPU TLB structure:
┌────────────────────────────────┐
│  L1 Instruction TLB (32-128 entries) │
│  L1 Data TLB       (64-256 entries)  │
│  L2 Unified TLB    (512-4K entries)  │
│  L3 Unified TLB    (varies, on chip) │
└────────────────────────────────┘

TLB Entry (associative cache):
┌──────────┬──────────┬─────────────┐
│   VPN    │   PFN    │ Permissions │
└──────────┴──────────┴─────────────┘

On access:
1. Check L1 TLB: VA hits? Use cached PFN → PA immediately
2. Miss: Check L2, L3 TLBs
3. Miss: Walk page tables (expensive)
4. Insert new PTE into TLB (evict LRU entry)

TLB Invalidation:
- invlpg VA: Invalidate TLB entry for specific VA
- mov cr3, rbx: Invalidate entire TLB (expensive, 100s of cycles)
- mov cr8, rax: Partial flush based on ASID (some CPUs)
```

---

## 2.2 ARM64 Paging

ARM64 uses similar hierarchical paging but with architectural differences.

### ARM64 Translation Granule Options

ARM allows configurable page sizes and table levels:

```
Translation Granule (TG) Options:
- 4KB granule: 9-bit indices per level, up to 4 levels
- 16KB granule: 11-bit indices per level, up to 4 levels
- 64KB granule: 16-bit indices per level, up to 3 levels

Typical 4KB, 3-level configuration:
VA: │ L1 (9b) │ L2 (9b) │ L3 (9b) │ Offset (12b) │
     │ PGD    │ PMD    │ PTE    │

PGD (Page Global Directory) → PMD (Page Middle Directory) → PTE
```

### ARM64 Descriptor Format (PTE)

```
Bits 63:52  Reserved
Bits 51:12  Physical Address
Bits 11:10  Shareability (SH)
Bit  9      Accessed (AF)
Bit  8      Not Global (nG)
Bits 7:6    Contiguous (affects TLB packing)
Bits 5:4    Memory Attributes (MAIR index)
Bit  3      Execute Never (XN)
Bit  2      Access Permission (AP[1])
Bit  1      Access Permission (AP[0])
Bit  0      Valid

AP[1:0] encoding:
00 = read-write (privileged)
01 = read-write (all)
10 = read-only (privileged)
11 = read-only (all)
```

### ARM64 ASID (Address Space ID)

ARM64 supports **ASID** (Address Space Identifier), allowing the TLB to cache entries for multiple processes simultaneously without global flush.

```
TTBR0_EL1 (Translation Table Base Register):
┌────────────┬──────────┬─────────────────────┐
│ ASID (16b) │ Reserved │ Table Base (48b)    │
└────────────┴──────────┴─────────────────────┘

When switching processes:
- Write new ASID and table base to TTBR0_EL1
- TLB entries tagged with old ASID automatically isolated
- No expensive global flush needed
- Efficiency: 10-100× TLB miss reduction in context switches
```

---

## 2.3 Real Example: Address Translation Simulation

### Scenario: x86-64 User-Space Access

```
Physical memory layout (simplified):
┌──────────────────┐
│ Kernel code/data │ 0xffffffff80000000 - 0xffffffffffffffff
├──────────────────┤
│ User-space data  │ (scattered based on page tables)
├──────────────────┤
│ Physical RAM     │ 0x0 - 0x100000000 (4GB)
└──────────────────┘

Process: User application tries to read 8 bytes at VA = 0x00007fff12345678

Step 1: Extract indices
  PML4 idx = (0x00007fff12345678 >> 39) & 0x1FF = 0x0F
  PDPT idx = (0x00007fff12345678 >> 30) & 0x1FF = 0x1C
  PD idx   = (0x00007fff12345678 >> 21) & 0x1FF = 0x1A
  PT idx   = (0x00007fff12345678 >> 12) & 0x1FF = 0x1AD
  Offset   = 0x00007fff12345678 & 0xFFF = 0x678

Step 2: CR3 contains PML4 physical address
  CR3 = 0x40000000 (example)

Step 3: Walk
  Read PML4[0x0F] from PA 0x40000000 + (0x0F * 8) = 0x40000078
    → Returns PDPT table PA: 0x41000000
  
  Read PDPT[0x1C] from PA 0x41000000 + (0x1C * 8) = 0x410000E0
    → Returns PD table PA: 0x42000000
  
  Read PD[0x1A] from PA 0x42000000 + (0x1A * 8) = 0x420000D0
    → Returns PT table PA: 0x43000000
  
  Read PT[0x1AD] from PA 0x43000000 + (0x1AD * 8) = 0x43000D68
    → Returns PTE: 0x0000000050000f07

Step 4: Parse PTE
  PFN = (0x0000000050000f07 >> 12) & 0xFFFFFFFFFFFFF = 0x50000
  P = 1 (present)
  U = 1 (user-space access allowed)
  W = 1 (writable)
  A = 1 (accessed)

Step 5: Translate VA to PA
  PA = (PFN << 12) | Offset = (0x50000 << 12) | 0x678 = 0x50000678

Step 6: CPU accesses RAM at 0x50000678, reads 8 bytes

Step 7: TLB is updated: VA[0x7fff123xxxxx] → PA[0x50000xxxxx]
```

This tree walk has a **4 memory accesses** latency:
- Access 1: PML4 lookup (~100 cycles)
- Access 2: PDPT lookup (~100 cycles)
- Access 3: PD lookup (~100 cycles)
- Access 4: PT lookup → gets PFN
- Access 5: Actual data read

**Total: 500+ cycles if all misses.** With TLB hit: ~3 cycles. This is why TLB is critical.

---

# Part 3: Linux Kernel Implementation

## 3.1 Linux Virtual Address Space Layout

### x86-64 User/Kernel Space Separation

```
┌────────────────────────────────────┐
│ 0xffffffffffffffff                 │
│                                    │
│ ─ Kernel-only space ─              │
│                                    │
│ 0xffff800000000000                 │ ← Kernel base
├────────────────────────────────────┤
│ 0x00007fffffffffff                 │
│                                    │
│ ─ User-space ─                     │
│ [Program, libs, stack, heap, mmap] │
│                                    │
│ 0x0000000000000000                 │
└────────────────────────────────────┘

Canonical form: highest bit is sign-extended
For 48-bit virtual addresses:
  Bits 63-47 must all be the same (0 for user, 1 for kernel)
  Invalid: 0x0000800000000000 - 0xffff7fffffffffff (huge gap)
```

### Linux Process Virtual Address Map

```
Higher addresses:
┌──────────────────────────┐
│ Kernel Mappings          │ 0xffffffff80000000+
├──────────────────────────┤
│ [Not mapped]             │ (canonical hole)
├──────────────────────────┤
│ Stack (grows down)       │ [top of user space]
├──────────────────────────┤
│ [Memory-mapped region]   │ mmap'd files, shared memory
├──────────────────────────┤
│ Heap (grows up)          │ malloc'd memory
├──────────────────────────┤
│ .bss section (uninitialized) │
├──────────────────────────┤
│ .data section (initialized) │
├──────────────────────────┤
│ .text section (code)     │
├──────────────────────────┤
│ ELF header               │
└──────────────────────────┘
Lower addresses: 0x0
```

---

## 3.2 Linux Kernel Page Table Data Structures

### mm_struct: Process Memory Context

Every process has a `struct mm_struct` tracking all memory management state:

```c
// From include/linux/mm_types.h (simplified)
struct mm_struct {
    struct {
        struct vm_area_struct *mmap;     // Linked list of VMA's
        struct rb_root mm_rb;            // Red-black tree of VMA's
        u64 vmacache_seqnum;             // VMA cache serializer
        unsigned long (*get_unmapped_area) (struct file *filp,
            unsigned long addr, unsigned long len,
            unsigned long pgoff, unsigned long flags);
        unsigned long mmap_base;         // base address for mmap()
        unsigned long task_size;         // max user-space VA
        unsigned long highest_vm_end;    // highest VMA end

        pgd_t *pgd;                      // Pointer to PML4 (root page table)
        atomic_t mm_users;               // Reference count
        atomic_t mm_count;               // Reference count
        int map_count;                   // Number of VMA's

        spinlock_t page_table_lock;      // Protect page tables
        struct rw_semaphore mmap_lock;   // Protect mm_struct

        struct list_head mmlist;         // All mm_struct's linked
        unsigned long hiwater_rss;       // Peak RSS usage
        unsigned long hiwater_vm;        // Peak VM usage

        unsigned long total_vm;          // Total pages mapped
        unsigned long locked_vm;         // mlocked pages
        unsigned long pinned_vm;         // pinned pages
        unsigned long data_vm;           // Executable data
        unsigned long exec_vm;           // Stack + heap
        unsigned long stack_vm;          // Stack pages

        unsigned long arg_lock;          // Protect arg pages
        unsigned long start_code, end_code;
        unsigned long start_data, end_data;
        unsigned long start_brk, brk;
        unsigned long start_stack;
        unsigned long arg_start, arg_end;
        unsigned long env_start, env_end;

        unsigned long saved_auxv[AT_VECTOR_SIZE]; // Auxiliary values

        struct {
        // OOM killer fields
        } membarrier_state;

        // ... Many more fields for performance, security, ...
};
```

### vm_area_struct: Virtual Memory Area

Each contiguous region of a process's address space is a VMA:

```c
// From include/linux/mm_types.h (simplified)
struct vm_area_struct {
    struct mm_struct *vm_mm;        // Back-pointer to process mm
    unsigned long vm_start;         // Start address (inclusive)
    unsigned long vm_end;           // End address (exclusive)

    struct vm_area_struct *vm_next; // Ordered by address
    struct vm_area_struct *vm_prev;

    struct rb_node vm_rb;           // RB-tree node in mm_struct
    unsigned long rb_subtree_gap;   // Optimization for gap finding

    struct list_head anon_vma_chain; // Link to anon_vma
    struct anon_vma *anon_vma;      // For reverse mapping
    struct vm_operations_struct *vm_ops; // Callbacks (mmap, munmap, etc)
    unsigned long vm_pgoff;         // Offset in file (for file-backed)

    struct file *vm_file;           // File backing this region (if any)
    void *vm_private_data;          // Filesystem-specific data

    struct swap_extent *swap_extent;// For swap management
    struct mempolicy *vm_policy;    // NUMA policy
    struct vm_userfaultfd_ctx vm_userfaultfd_ctx;

    // VM flags (permissions and behavior)
    unsigned long vm_flags;
    // #define VM_READ         0x00000001
    // #define VM_WRITE        0x00000002
    // #define VM_EXEC         0x00000004
    // #define VM_SHARED       0x00000008
    // #define VM_MAYREAD      0x00000010
    // #define VM_MAYWRITE     0x00000020
    // #define VM_MAYEXEC      0x00000040
    // #define VM_MAYSHARE     0x00000080
    // #define VM_GROWSDOWN    0x00000100
    // #define VM_UFFD_MISSING 0x00010000
    // ... many more ...
};
```

### Key Insight: VMAs ≠ Page Tables

**Critical distinction:**
- **VMAs (mm_struct)**: Logical memory regions with a single set of permissions and properties
- **Page Tables (pgd)**: Physical mapping from virtual addresses to physical frames

A single VMA might span multiple page table entries (all with the same permissions), or conversely, permissions can change mid-VMA (e.g., code vs data in the same file mapping).

---

## 3.3 Page Table Walk: Kernel Functions

### Walking Page Tables in Kernel Code

The kernel provides macros and functions to navigate page tables without hardware:

```c
// include/asm-generic/pgtable.h
// These are standard accessors:

// Get PML4 entry (top level)
pml4_t *pml4 = (pml4_t *)__va(current->mm->pgd);
pml4_t pml4_entry = pml4[pml4_index(va)];

// Check if entry is present
if (!pml4_present(pml4_entry)) {
    // Page table entry not present
    return -EFAULT;
}

// Get next level (PDPT)
pgd_t *pgd = (pgd_t *)__va(pml4_val(pml4_entry) & PAGE_MASK);
pgd_t pgd_entry = pgd[pgd_index(va)];

// Similar for PD and PT levels
pud_t *pud = (pud_t *)__va(pgd_val(pgd_entry) & PAGE_MASK);
pud_t pud_entry = pud[pud_index(va)];

pmd_t *pmd = (pmd_t *)__va(pud_val(pud_entry) & PAGE_MASK);
pmd_t pmd_entry = pmd[pmd_index(va)];

pte_t *pte = (pte_t *)__va(pmd_val(pmd_entry) & PAGE_MASK);
pte_t pte_entry = pte[pte_index(va)];

// Extract physical address from PTE
unsigned long pfn = pte_pfn(pte_entry);
unsigned long pa = pfn_to_phys(pfn) | (va & ~PAGE_MASK);
```

### Kernel Abstraction Layers

To support different architectures (x86-64, ARM64, RISC-V, PowerPC), Linux uses **multi-level abstractions**:

```
Architecture-specific (arch/x86_64/include/asm/pgtable.h):
  pte_t, pmd_t, pud_t, pgd_t (types)
  pte_index(), pmd_index(), pud_index(), pgd_index() (macros)
  pte_offset_map(), pmd_offset(), pud_offset(), pgd_offset() (functions)
  
Generic layer (include/asm-generic/pgtable.h):
  pte_present(), pte_write(), pte_dirty(), pte_accessed()
  pte_mkdirty(), pte_mkyoung(), pte_mkwrite()
  
High-level API (mm/pgtable-generic.c):
  ptep_get(), ptep_get_and_clear(), set_pte()
  pte_unmap(), pte_offset_map_lock()
```

---

## 3.4 Kernel Page Fault Handler

### Exception Path

When a page fault occurs:

```
CPU Exception (#PF)
  │
  ├─→ Exception handler (arch-specific, e.g., arch/x86/mm/fault.c)
  │
  ├─→ do_page_fault() / do_user_addr_fault()
  │   Extracts VA from CR2 register, gets error code
  │
  ├─→ __handle_mm_fault()
  │   │
  │   ├─→ Walk page tables
  │   │
  │   ├─→ Determine fault type:
  │   │   ├─→ Page not present: do_anonymous_page(), do_fault()
  │   │   ├─→ Write to COW page: do_cow_fault()
  │   │   ├─→ Permission violation: kill process
  │   │
  │   └─→ Allocate page, populate PTE, return VM_FAULT_NOPAGE
  │
  └─→ Return to user-space instruction that faulted
      Instruction retries automatically
```

### Simplified do_page_fault() Flow (x86-64)

```c
// arch/x86/mm/fault.c (heavily simplified)
__do_page_fault(struct pt_regs *regs, unsigned long error_code) {
    unsigned long address = read_cr2();  // VA that caused fault
    
    // Classify the fault
    int is_user = user_mode(regs);
    int is_write = error_code & PFERR_WRITE_MASK;
    int is_fetch = error_code & PFERR_FETCH_MASK;
    int is_present = error_code & PFERR_PRESENT_MASK;
    
    // Basic checks
    if (address < PAGE_OFFSET) {
        // Null pointer dereference or similar
        bad_area_nosemaphore(regs, error_code, address);
        return;
    }
    
    struct mm_struct *mm = current->mm;
    
    // Find the VMA containing this address
    struct vm_area_struct *vma = find_vma(mm, address);
    
    if (!vma) {
        // Address not in any VMA → segmentation fault
        bad_area(regs, error_code, address);
        return;
    }
    
    // Check if access type matches VMA permissions
    if (is_write && !(vma->vm_flags & VM_WRITE)) {
        // Write to read-only region
        bad_area_access_error(regs, error_code, address, vma);
        return;
    }
    
    if (is_fetch && !(vma->vm_flags & VM_EXEC)) {
        // Execute on non-executable region
        bad_area_access_error(regs, error_code, address, vma);
        return;
    }
    
    // Handle the fault
    handle_mm_fault(vma, address, flags);
}
```

### handle_mm_fault() Core Logic

```c
// mm/memory.c (simplified)
vm_fault_t handle_mm_fault(struct vm_area_struct *vma,
                           unsigned long address,
                           unsigned int flags) {
    struct mm_struct *mm = vma->vm_mm;
    
    // Allocate or update page tables as needed
    p4d_t *p4d = p4d_alloc(mm, pgd, address);
    pud_t *pud = pud_alloc(mm, p4d, address);
    pmd_t *pmd = pmd_alloc(mm, pud, address);
    pte_t *pte = pte_alloc_map(mm, pmd, address);
    
    if (!pte_none(*pte)) {
        // Page table entry already exists
        if (!pte_present(*pte)) {
            // Entry not present: page is swapped, demand paged, etc
            handle_pte_fault(mm, vma, address, pte, flags);
        } else {
            // Entry is present: might be COW, permission change, etc
            handle_pte_fault(mm, vma, address, pte, flags);
        }
    } else {
        // Page table entry is empty: first access to this VA
        if (vma->vm_ops && vma->vm_ops->fault) {
            // File-backed: call mmap handler
            return vma->vm_ops->fault(vma, vmf);
        } else {
            // Anonymous (heap/stack): allocate a zero page
            return do_anonymous_page(vmf);
        }
    }
}
```

---

## 3.5 Core Page Table Operations

### Allocating a Page Table Level

When a process first accesses a virtual address, intermediate page tables must exist:

```c
// mm/memory.c

pud_t * __pud_alloc(struct mm_struct *mm, pgd_t *pgd, unsigned long address) {
    // PUD = Page Upper Directory (level above PMD)
    
    pud_t *new;
    
    // Allocate a page for the new PUD table
    new = (pud_t *)get_zeroed_page(GFP_KERNEL);
    if (!new)
        return NULL;
    
    // Insert into parent (PGD)
    // Must use spinlock: page table walk might happen concurrently
    spin_lock(&mm->page_table_lock);
    
    // Double-check: another CPU might have allocated it
    if (!pgd_none(*pgd)) {
        // Already allocated
        free_page((unsigned long)new);
        new = pud_offset(pgd, address);
    } else {
        // Set PGD entry to point to new PUD table
        pgd_populate(mm, pgd, new);
    }
    
    spin_unlock(&mm->page_table_lock);
    return new;
}

// Similar functions exist:
// - pmd_alloc() for PMD (next level down)
// - pte_alloc() for PTE (bottom level)
```

### Setting a Page Table Entry

After allocation, PTEs are actually set:

```c
// mm/memory.c

static inline void set_pte_at(struct mm_struct *mm,
                              unsigned long addr,
                              pte_t *ptep,
                              pte_t pte) {
    // Low-level setter: actually writes PTE to memory
    ptep->pte = pte.pte;
    
    // If PTE is marked as present and not global, invalidate TLB
    if (pte_present(pte) && !(pte_flags(pte) & _PAGE_GLOBAL)) {
        flush_tlb_fix_spurious_fault(mm, addr);
    }
}

// Higher-level wrapper
void set_pte(pte_t *ptep, pte_t pte) {
    // Update page tables with full checking
    ptep->pte = pte_val(pte);
}
```

### Handling Write-Protect / Copy-on-Write (CoW)

When a process forks, child and parent initially share the same physical pages (read-only). When one tries to write, a page fault occurs:

```c
// mm/memory.c - simplified

vm_fault_t do_wp_page(struct vm_fault *vmf) {
    struct page *page = vmf->page;
    
    // Count references to this page
    if (page_count(page) == 1) {
        // Only this process references it: safe to write directly
        pte_t entry = pte_mkwrite(vmf->orig_pte);
        set_pte_at(vma->vm_mm, vmf->address, vmf->pte, entry);
        
        flush_tlb_fix_spurious_fault(vma->vm_mm, vmf->address);
        return VM_FAULT_WRITE;
    } else {
        // Other processes reference it: must copy
        struct page *new_page = alloc_page(GFP_HIGHUSER_MOVABLE);
        copy_user_highpage(new_page, page, vmf->address, vma);
        
        // Decrement refcount on old page
        put_page(page);
        
        // Point this PTE to new (writable) page
        pte_t entry = mk_pte(new_page, vma->vm_page_prot);
        entry = pte_mkwrite(entry);
        set_pte_at(vma->vm_mm, vmf->address, vmf->pte, entry);
        
        // Update page tracking
        page_add_new_anon_rmap(new_page, vma, vmf->address, false);
        
        return VM_FAULT_WRITE;
    }
}
```

---

# Part 4: Code Deep Dives

## 4.1 Walking Page Tables in C (Kernel Context)

### Manual Page Table Walk

```c
// Example: Walk page tables to find physical address
// This is what happens "under the hood" during translation

#include <linux/mm.h>
#include <linux/sched.h>
#include <asm/pgalloc.h>

// Simplified walk that demonstrates the structure
unsigned long walk_page_tables(struct mm_struct *mm, unsigned long va) {
    pgd_t *pgd_entry;
    p4d_t *p4d_entry;
    pud_t *pud_entry;
    pmd_t *pmd_entry;
    pte_t *pte_entry;
    unsigned long pa = 0;

    // Get root page table (PML4)
    pgd_entry = pgd_offset(mm, va);
    if (pgd_none(*pgd_entry) || pgd_bad(*pgd_entry)) {
        pr_info("PGD not present for VA %lx\n", va);
        return 0;
    }

    // Level 1: PGD → P4D (on systems with 5-level paging)
    // On x86-64, this level doesn't exist functionally, but Linux abstracts it
    p4d_entry = p4d_offset(pgd_entry, va);
    if (p4d_none(*p4d_entry) || p4d_bad(*p4d_entry)) {
        pr_info("P4D not present for VA %lx\n", va);
        return 0;
    }

    // Level 2: P4D → PUD
    pud_entry = pud_offset(p4d_entry, va);
    if (pud_none(*pud_entry) || pud_bad(*pud_entry)) {
        pr_info("PUD not present for VA %lx\n", va);
        return 0;
    }

    // Check for 1GB huge page
    if (pud_huge(*pud_entry)) {
        unsigned long pud_pfn = pud_pfn(*pud_entry);
        unsigned long pud_base = pfn_to_phys(pud_pfn);
        pa = pud_base + (va & ~PUD_MASK);
        pr_info("Found 1GB huge page: PA %lx\n", pa);
        return pa;
    }

    // Level 3: PUD → PMD
    pmd_entry = pmd_offset(pud_entry, va);
    if (pmd_none(*pmd_entry) || pmd_bad(*pmd_entry)) {
        pr_info("PMD not present for VA %lx\n", va);
        return 0;
    }

    // Check for 2MB huge page
    if (pmd_huge(*pmd_entry)) {
        unsigned long pmd_pfn = pmd_pfn(*pmd_entry);
        unsigned long pmd_base = pfn_to_phys(pmd_pfn);
        pa = pmd_base + (va & ~PMD_MASK);
        pr_info("Found 2MB huge page: PA %lx\n", pa);
        return pa;
    }

    // Level 4: PMD → PTE
    pte_entry = pte_offset_map(pmd_entry, va);
    if (!pte_entry || pte_none(*pte_entry)) {
        pr_info("PTE not present for VA %lx\n", va);
        pte_unmap(pte_entry);
        return 0;
    }

    if (!pte_present(*pte_entry)) {
        pr_info("Page not in memory (swapped or demand paged) for VA %lx\n", va);
        pte_unmap(pte_entry);
        return 0;
    }

    // Extract physical frame number and compute PA
    unsigned long pte_pfn = pte_pfn(*pte_entry);
    unsigned long pte_base = pfn_to_phys(pte_pfn);
    pa = pte_base + (va & ~PAGE_MASK);

    pr_info("VA %lx → PA %lx (PFN %lx, offset %lx)\n",
            va, pa, pte_pfn, va & ~PAGE_MASK);

    pte_unmap(pte_entry);
    return pa;
}
```

### Protecting a Page (Setting Read-Only)

```c
// Example: Make a page read-only and track access

int protect_page_readonly(struct mm_struct *mm, unsigned long va) {
    pgd_t *pgd;
    p4d_t *p4d;
    pud_t *pud;
    pmd_t *pmd;
    pte_t *pte, pte_entry, pte_new;
    spinlock_t *ptl;
    int ret = 0;

    // Acquire lock (per-page-table-level in newer kernels)
    pte = pte_offset_map_lock(mm, pmd_offset(pud_offset(p4d_offset(
            pgd_offset(mm, va), va), va), va), va, &ptl);

    if (!pte) {
        pr_err("Failed to get PTE for VA %lx\n", va);
        return -EFAULT;
    }

    pte_entry = *pte;

    // Check if page is present
    if (!pte_present(pte_entry)) {
        pr_warn("Page not present at VA %lx\n", va);
        ret = -EFAULT;
        goto out;
    }

    // Check if already read-only
    if (!pte_write(pte_entry)) {
        pr_info("Page already read-only at VA %lx\n", va);
        goto out;
    }

    // Create new PTE with write bit cleared
    pte_new = pte_wrprotect(pte_entry);

    // Update page table
    set_pte_at(mm, va, pte, pte_new);

    // Invalidate TLB entry for this VA
    flush_tlb_page(vma, va);  // Requires VMA pointer in real code

    pr_info("Page at VA %lx is now read-only\n", va);

out:
    pte_unmap_unlock(pte, ptl);
    return ret;
}
```

---

## 4.2 Page Table Implementation in Rust

Rust's type safety makes page table code cleaner and safer. Here's a pedagogical example:

```rust
// pedagogical_pagetable.rs
// A simplified, educational implementation of page table walking in Rust

use core::ptr;

// Architecture-specific constants
const PAGE_SIZE: usize = 4096;
const PAGE_OFFSET_BITS: usize = 12;
const PAGE_OFFSET_MASK: usize = PAGE_SIZE - 1;

// PML4, PDPT, PD, PT have 512 entries each (9 bits per level)
const ENTRIES_PER_TABLE: usize = 512;
const INDEX_BITS: usize = 9;
const INDEX_MASK: usize = (1 << INDEX_BITS) - 1;

// Level descriptions
#[repr(usize)]
enum PageTableLevel {
    Level4 = 39, // PML4
    Level3 = 30, // PDPT
    Level2 = 21, // PD
    Level1 = 12, // PT
}

// PTE permission bits
#[derive(Debug, Clone, Copy)]
struct PTEFlags {
    present: bool,        // Bit 0
    write: bool,          // Bit 1
    user: bool,           // Bit 2
    write_through: bool,  // Bit 3
    cache_disabled: bool, // Bit 4
    accessed: bool,       // Bit 5
    dirty: bool,          // Bit 6
    huge: bool,           // Bit 7 (Page Size)
    global: bool,         // Bit 8
}

impl PTEFlags {
    fn from_raw(val: u64) -> Self {
        PTEFlags {
            present: (val & 0x001) != 0,
            write: (val & 0x002) != 0,
            user: (val & 0x004) != 0,
            write_through: (val & 0x008) != 0,
            cache_disabled: (val & 0x010) != 0,
            accessed: (val & 0x020) != 0,
            dirty: (val & 0x040) != 0,
            huge: (val & 0x080) != 0,
            global: (val & 0x100) != 0,
        }
    }

    fn to_raw(&self) -> u64 {
        let mut val: u64 = 0;
        if self.present { val |= 0x001; }
        if self.write { val |= 0x002; }
        if self.user { val |= 0x004; }
        if self.write_through { val |= 0x008; }
        if self.cache_disabled { val |= 0x010; }
        if self.accessed { val |= 0x020; }
        if self.dirty { val |= 0x040; }
        if self.huge { val |= 0x080; }
        if self.global { val |= 0x100; }
        val
    }
}

// Page Table Entry: 64-bit with physical address and flags
#[derive(Debug, Clone, Copy)]
struct PageTableEntry {
    value: u64,
}

impl PageTableEntry {
    fn new(pfn: u64, flags: PTEFlags) -> Self {
        let pa = pfn << PAGE_OFFSET_BITS;
        let flags_val = flags.to_raw();
        PageTableEntry {
            value: pa | flags_val,
        }
    }

    fn pfn(&self) -> u64 {
        (self.value >> PAGE_OFFSET_BITS) & ((1u64 << 40) - 1)
    }

    fn flags(&self) -> PTEFlags {
        PTEFlags::from_raw(self.value & 0xFFF)
    }

    fn address(&self) -> u64 {
        self.pfn() << PAGE_OFFSET_BITS
    }

    fn is_present(&self) -> bool {
        self.flags().present
    }

    fn is_huge(&self) -> bool {
        self.flags().huge
    }
}

// A page table level (PML4, PDPT, PD, or PT)
#[repr(C, align(4096))]
struct PageTable {
    entries: [PageTableEntry; ENTRIES_PER_TABLE],
}

impl PageTable {
    fn index_for_level(va: u64, level: PageTableLevel) -> usize {
        ((va >> (level as usize)) & INDEX_MASK as u64) as usize
    }

    fn get(&self, index: usize) -> PageTableEntry {
        if index >= ENTRIES_PER_TABLE {
            panic!("Index out of bounds");
        }
        self.entries[index]
    }

    fn set(&mut self, index: usize, entry: PageTableEntry) {
        if index >= ENTRIES_PER_TABLE {
            panic!("Index out of bounds");
        }
        self.entries[index] = entry;
    }
}

// The core page table walker
struct PageTableWalker {
    pml4_pa: u64,
}

impl PageTableWalker {
    fn new(pml4_pa: u64) -> Self {
        PageTableWalker { pml4_pa }
    }

    /// Walk page tables to translate VA → PA
    /// Returns (physical_address, page_size)
    /// page_size indicates if 4KB, 2MB (huge), or 1GB (huge) page was used
    fn translate(&self, va: u64) -> Result<(u64, &'static str), &'static str> {
        // Step 1: Get PML4
        let pml4_va = self.phys_to_virt(self.pml4_pa);
        let pml4 = unsafe { &*(pml4_va as *const PageTable) };

        let pml4_idx = PageTable::index_for_level(va, PageTableLevel::Level4);
        let pml4_entry = pml4.get(pml4_idx);

        if !pml4_entry.is_present() {
            return Err("PML4 entry not present");
        }

        // Step 2: Get PDPT
        let pdpt_pa = pml4_entry.address();
        let pdpt_va = self.phys_to_virt(pdpt_pa);
        let pdpt = unsafe { &*(pdpt_va as *const PageTable) };

        let pdpt_idx = PageTable::index_for_level(va, PageTableLevel::Level3);
        let pdpt_entry = pdpt.get(pdpt_idx);

        if !pdpt_entry.is_present() {
            return Err("PDPT entry not present");
        }

        // Check for 1GB huge page
        if pdpt_entry.is_huge() {
            let pa = (pdpt_entry.pfn() << PAGE_OFFSET_BITS) | (va & 0x3FFFFFFF);
            return Ok((pa, "1GB"));
        }

        // Step 3: Get PD
        let pd_pa = pdpt_entry.address();
        let pd_va = self.phys_to_virt(pd_pa);
        let pd = unsafe { &*(pd_va as *const PageTable) };

        let pd_idx = PageTable::index_for_level(va, PageTableLevel::Level2);
        let pd_entry = pd.get(pd_idx);

        if !pd_entry.is_present() {
            return Err("PD entry not present");
        }

        // Check for 2MB huge page
        if pd_entry.is_huge() {
            let pa = (pd_entry.pfn() << PAGE_OFFSET_BITS) | (va & 0x1FFFFF);
            return Ok((pa, "2MB"));
        }

        // Step 4: Get PT
        let pt_pa = pd_entry.address();
        let pt_va = self.phys_to_virt(pt_pa);
        let pt = unsafe { &*(pt_va as *const PageTable) };

        let pt_idx = PageTable::index_for_level(va, PageTableLevel::Level1);
        let pte = pt.get(pt_idx);

        if !pte.is_present() {
            return Err("PTE not present");
        }

        // Extract PA: (PFN << 12) | offset
        let pa = (pte.pfn() << PAGE_OFFSET_BITS) | (va & PAGE_OFFSET_MASK as u64);
        Ok((pa, "4KB"))
    }

    /// Convert physical address to kernel virtual address
    /// This is a simplified mapping: PA + KERNEL_BASE
    fn phys_to_virt(&self, pa: u64) -> u64 {
        const KERNEL_BASE: u64 = 0xffff800000000000;
        KERNEL_BASE + pa
    }

    /// Convert kernel virtual address to physical
    fn virt_to_phys(&self, va: u64) -> u64 {
        const KERNEL_BASE: u64 = 0xffff800000000000;
        va - KERNEL_BASE
    }
}

// ============================================================================
// Example usage and testing
// ============================================================================

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_pte_flags() {
        let flags = PTEFlags {
            present: true,
            write: true,
            user: true,
            write_through: false,
            cache_disabled: false,
            accessed: true,
            dirty: true,
            huge: false,
            global: false,
        };

        let raw = flags.to_raw();
        assert_eq!(raw & 0x001, 0x001); // Present
        assert_eq!(raw & 0x002, 0x002); // Write
        assert_eq!(raw & 0x004, 0x004); // User

        let recovered = PTEFlags::from_raw(raw);
        assert_eq!(recovered.present, true);
        assert_eq!(recovered.write, true);
        assert_eq!(recovered.user, true);
    }

    #[test]
    fn test_pte_address_extraction() {
        let pfn: u64 = 0x12345;
        let flags = PTEFlags {
            present: true,
            write: true,
            user: false,
            write_through: false,
            cache_disabled: false,
            accessed: true,
            dirty: true,
            huge: false,
            global: false,
        };

        let pte = PageTableEntry::new(pfn, flags);
        assert_eq!(pte.pfn(), pfn);
        assert_eq!(pte.address(), pfn << PAGE_OFFSET_BITS);
        assert_eq!(pte.flags().present, true);
        assert_eq!(pte.flags().write, true);
    }
}
```

---

## 4.3 Rust: Safe Abstraction for Page Table Walking

A production-grade Rust module with safety guarantees:

```rust
// safe_pagetable_walker.rs
// Production-quality page table walker with error handling

use core::fmt;

#[derive(Debug, Clone)]
pub enum PageWalkError {
    InvalidAddress { va: u64, reason: &'static str },
    PageNotPresent { va: u64, level: u32 },
    PermissionDenied { va: u64, access_type: &'static str },
    AddressNotMapped { va: u64 },
}

impl fmt::Display for PageWalkError {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        match self {
            PageWalkError::InvalidAddress { va, reason } =>
                write!(f, "Invalid address 0x{:x}: {}", va, reason),
            PageWalkError::PageNotPresent { va, level } =>
                write!(f, "Page table level {} not present for 0x{:x}", level, va),
            PageWalkError::PermissionDenied { va, access_type } =>
                write!(f, "Permission denied ({}) for address 0x{:x}", access_type, va),
            PageWalkError::AddressNotMapped { va } =>
                write!(f, "Address 0x{:x} not mapped", va),
        }
    }
}

/// Safe result type for page table operations
pub type PageWalkResult<T> = Result<T, PageWalkError>;

/// Represents a translated address with metadata
#[derive(Debug, Clone, Copy)]
pub struct TranslatedAddress {
    pub physical: u64,
    pub page_size: PageSize,
    pub writable: bool,
    pub executable: bool,
    pub user_accessible: bool,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum PageSize {
    Small,  // 4KB
    Large,  // 2MB
    Huge,   // 1GB
}

impl fmt::Display for PageSize {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        match self {
            PageSize::Small => write!(f, "4KB"),
            PageSize::Large => write!(f, "2MB"),
            PageSize::Huge => write!(f, "1GB"),
        }
    }
}

/// Validates that a virtual address is canonically correct
/// For 48-bit VA: bits 63-47 must all be 0 or all be 1
fn is_canonical_address(va: u64) -> bool {
    let bit_47 = (va >> 47) & 1;
    let bits_63_48 = va >> 48;

    // All high bits should match bit 47
    if bit_47 == 0 {
        bits_63_48 == 0
    } else {
        bits_63_48 == 0xFFFF
    }
}

/// Safe page table walker
pub struct SafePageTableWalker {
    pml4_physical_address: u64,
}

impl SafePageTableWalker {
    pub fn new(pml4_pa: u64) -> PageWalkResult<Self> {
        // Validate PML4 address is physical
        if pml4_pa & 0xFFF != 0 {
            return Err(PageWalkError::InvalidAddress {
                va: pml4_pa,
                reason: "PML4 address must be page-aligned",
            });
        }

        Ok(SafePageTableWalker {
            pml4_physical_address: pml4_pa,
        })
    }

    /// Translate virtual address, checking all permissions
    pub fn translate_with_access_check(
        &self,
        va: u64,
        access: AccessType,
    ) -> PageWalkResult<TranslatedAddress> {
        // Validate canonical form
        if !is_canonical_address(va) {
            return Err(PageWalkError::InvalidAddress {
                va,
                reason: "Non-canonical address",
            });
        }

        let result = self.translate(va)?;

        // Check access permissions
        match access {
            AccessType::Read => {
                // Reads always allowed if page is present
                Ok(result)
            }
            AccessType::Write => {
                if !result.writable {
                    return Err(PageWalkError::PermissionDenied {
                        va,
                        access_type: "write to read-only",
                    });
                }
                Ok(result)
            }
            AccessType::Execute => {
                if !result.executable {
                    return Err(PageWalkError::PermissionDenied {
                        va,
                        access_type: "execute on non-executable",
                    });
                }
                Ok(result)
            }
        }
    }

    /// Core translation without access check
    pub fn translate(&self, va: u64) -> PageWalkResult<TranslatedAddress> {
        if !is_canonical_address(va) {
            return Err(PageWalkError::InvalidAddress {
                va,
                reason: "Non-canonical address",
            });
        }

        // In a real implementation, we would:
        // 1. Walk PML4 via CR3 (kernel manages)
        // 2. Check PDE, PMD, PTE in sequence
        // 3. Handle huge pages at each level
        // 4. Extract flags

        // Pseudo-code representation:
        // let pml4 = get_page_table(self.pml4_physical_address)?;
        // let pml4_entry = pml4[pml4_index(va)];
        // ... continue walk ...

        // Placeholder implementation for pedagogical purposes
        Err(PageWalkError::AddressNotMapped { va })
    }
}

#[derive(Debug, Clone, Copy)]
pub enum AccessType {
    Read,
    Write,
    Execute,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_canonical_addresses() {
        // User-space (low) canonical
        assert!(is_canonical_address(0x0000000000000000));
        assert!(is_canonical_address(0x00007fffffffffff));

        // Non-canonical (hole)
        assert!(!is_canonical_address(0x0000800000000000));
        assert!(!is_canonical_address(0xffff7fffffffffff));

        // Kernel-space (high) canonical
        assert!(is_canonical_address(0xffff800000000000));
        assert!(is_canonical_address(0xffffffffffffffff));
    }

    #[test]
    fn test_walker_creation() {
        // Valid PML4
        let result = SafePageTableWalker::new(0x1000);
        assert!(result.is_ok());

        // Unaligned PML4
        let result = SafePageTableWalker::new(0x1001);
        assert!(result.is_err());
    }
}
```

---

# Part 5: Performance, Security, and Debugging

## 5.1 TLB Performance Implications

### TLB Hit Rate Impact on Performance

The Translation Lookaside Buffer is the critical performance lever:

```
Scenario: Loop over 1 million integers

Case 1: Perfect TLB hits
  Memory access = ~3-5 cycles (L1 cache)
  1,000,000 × 5 = 5,000,000 cycles

Case 2: 95% TLB hits, 5% TLB misses requiring page walk
  Hit time: 5 cycles
  Miss time: 100 cycles (page walk + L3 lookup)
  Time = (0.95 × 5) + (0.05 × 100) = 4.75 + 5 = 9.75 cycles per access
  1,000,000 × 9.75 = 9,750,000 cycles (1.95× slower!)

Case 3: Large working set, TLB thrashing (30% miss rate)
  Hit time: 5 cycles
  Miss time: 100 cycles
  Time = (0.70 × 5) + (0.30 × 100) = 3.5 + 30 = 33.5 cycles per access
  1,000,000 × 33.5 = 33,500,000 cycles (6.7× slower!)
```

### Strategies to Improve TLB Hit Rate

1. **Huge Pages**: Reduce address range → fewer TLB entries needed
   ```
   4KB pages: 2MB range needs 512 TLB entries
   2MB pages: Same 2MB range needs 1 TLB entry
   Benefit: 512× reduction in TLB pressure
   ```

2. **Memory Locality**: Concentrate access patterns
   ```
   Bad: Random access across 1GB (TLB thrashing)
   Good: Sequential access in 64MB (fits in TLB)
   ```

3. **Page Clustering**: Group related data
   ```
   Before: Thread 1 accesses [0x1000], Thread 2 accesses [0x1001000]
           Each needs separate PTE, TLB entry
   
   After: Thread 1 uses [0x1000-0x2000], Thread 2 uses [0x2000-0x3000]
          Both fit in same 2MB huge page
   ```

### Measuring TLB Performance

```bash
# Linux: Use perf to measure TLB misses
perf stat -e dTLB-loads,dTLB-load-misses,iTLB-loads,iTLB-load-misses \
    ./your_program

# Output example:
#  100,234,567 dTLB-loads      # Data TLB lookups
#     2,345,678 dTLB-load-misses # (2.3% miss rate - excellent)
#     50,123,456 iTLB-loads      # Instruction TLB lookups
#        234,567 iTLB-load-misses # (0.47% miss rate - excellent)

# Page walk cycles
perf stat -e cycles,cache-references,cache-misses,tlb-loads,tlb-load-misses \
    ./your_program
```

---

## 5.2 Security Implications

### Meltdown / Spectre and Page Table Isolation (PTI)

**The Attack**: Even with SMAP/SMEP, a speculative load can leak kernel memory via timing.

```
Vulnerable code (pre-Meltdown mitigation):
  mov rax, [rax]        // Speculatively load kernel memory (faults, but too late)
  mov rcx, [rax + rbx]  // Speculative: try to access leaked address
  
The CPU speculatively executes the second line, causing a cache access pattern
that reveals the kernel memory contents through timing side-channels.
```

**PTI Solution**: Keep separate page tables

```
Normal (vulnerable):
  User-space page tables contain all addresses (including kernel)
  Kernel-space page tables contain all addresses
  
After PTI:
  User-space page tables: only user VAs mapped + minimal kernel entry points
  Kernel-space page tables: everything
  
On syscall/exception:
  Switch CR3 from user PML4 to kernel PML4
  Kernel code runs in separate address space, inaccessible to speculation
```

**Kernel Code Inspection**:

```c
// include/asm-generic/pgtable.h

// PKEY (Protection Key) encoding for fine-grained access control
#define _PAGE_PKEY_BIT0         (1 << _PAGE_PKEY0)
#define _PAGE_PKEY_BIT1         (1 << _PAGE_PKEY1)
#define _PAGE_PKEY_BIT2         (1 << _PAGE_PKEY2)

// PKU (Protection Key User) - user-space has 16 keys, each with R/W/X bits
// Kernel uses PKEY_ACCESS_EXEC (0) for most allocations

// Memory protection key assignment
pte_t pte_mkpkey(pte_t pte, unsigned int key) {
    if (key == 0)
        return pte;
    return __pte(pte_val(pte) | (key << _PAGE_PKEY_BIT0));
}
```

### SMEP/SMAP: Supervisor Mode Execution/Access Prevention

- **SMEP**: Kernel cannot execute user-space code
- **SMAP**: Kernel cannot access user-space data without explicit permission (`stac`/`clac`)

```c
// Example: Secure memcpy from user-space

#include <asm/uaccess.h>

// UNSAFE: Can be exploited if CR3 trick exposes kernel data
unsigned long unsafe_copy(void *kernel_buf, const void *user_buf, size_t len) {
    memcpy(kernel_buf, user_buf, len);  // Kernel reading user memory
    return len;
}

// SAFE: With SMAP protection
unsigned long safe_copy(void *kernel_buf, const void *user_buf, size_t len) {
    unsigned long ret;
    
    // Enable access to user-space temporarily
    stac();
    
    // Now the copy is allowed
    ret = copy_from_user(kernel_buf, user_buf, len);
    
    // Disable access (automatic on context switch, but good to be explicit)
    clac();
    
    return ret;
}
```

---

## 5.3 Debugging Page Table Issues

### Kernel Module: Inspect Page Tables

```c
// debug_pte.c - Kernel module to dump page tables for any virtual address

#include <linux/module.h>
#include <linux/kernel.h>
#include <linux/fs.h>
#include <linux/proc_fs.h>
#include <linux/seq_file.h>
#include <linux/sched.h>
#include <asm/pgtable.h>

MODULE_LICENSE("GPL");
MODULE_AUTHOR("Debug Team");

// Read VA from /proc/debug_pte
static int debug_pte_read(struct seq_file *m, void *v) {
    unsigned long va = (unsigned long)v;
    struct mm_struct *mm = current->mm;
    
    pgd_t *pgd;
    p4d_t *p4d;
    pud_t *pud;
    pmd_t *pmd;
    pte_t *pte;
    
    seq_printf(m, "=== Page Table Walk for VA 0x%lx ===\n", va);
    
    if (!is_canonical_address(va)) {
        seq_printf(m, "ERROR: Non-canonical address\n");
        return 0;
    }
    
    // PGD (L4)
    pgd = pgd_offset(mm, va);
    seq_printf(m, "PGD[%lu] @ %px = 0x%016llx\n",
        pgd_index(va), pgd, (unsigned long long)pgd_val(*pgd));
    
    if (pgd_none(*pgd) || pgd_bad(*pgd)) {
        seq_printf(m, "  [PGDIR NOT PRESENT]\n");
        return 0;
    }
    
    // P4D (L3, usually folded in x86-64)
    p4d = p4d_offset(pgd, va);
    seq_printf(m, "P4D[%lu] @ %px = 0x%016llx\n",
        p4d_index(va), p4d, (unsigned long long)p4d_val(*p4d));
    
    if (p4d_none(*p4d) || p4d_bad(*p4d)) {
        seq_printf(m, "  [P4D NOT PRESENT]\n");
        return 0;
    }
    
    // PUD (L2)
    pud = pud_offset(p4d, va);
    seq_printf(m, "PUD[%lu] @ %px = 0x%016llx\n",
        pud_index(va), pud, (unsigned long long)pud_val(*pud));
    
    if (pud_none(*pud) || pud_bad(*pud)) {
        seq_printf(m, "  [PUD NOT PRESENT]\n");
        return 0;
    }
    
    if (pud_huge(*pud)) {
        unsigned long pa = pud_pfn(*pud) << PAGE_SHIFT;
        seq_printf(m, "  [1GB HUGE PAGE at 0x%lx]\n", pa | (va & ~PUD_MASK));
        seq_printf(m, "  Flags: Present=%d, Write=%d, User=%d\n",
            pud_present(*pud), pud_write(*pud), pud_user(*pud));
        return 0;
    }
    
    // PMD (L1)
    pmd = pmd_offset(pud, va);
    seq_printf(m, "PMD[%lu] @ %px = 0x%016llx\n",
        pmd_index(va), pmd, (unsigned long long)pmd_val(*pmd));
    
    if (pmd_none(*pmd) || pmd_bad(*pmd)) {
        seq_printf(m, "  [PMD NOT PRESENT]\n");
        return 0;
    }
    
    if (pmd_huge(*pmd)) {
        unsigned long pa = pmd_pfn(*pmd) << PAGE_SHIFT;
        seq_printf(m, "  [2MB HUGE PAGE at 0x%lx]\n", pa | (va & ~PMD_MASK));
        seq_printf(m, "  Flags: Present=%d, Write=%d, User=%d, Accessed=%d, Dirty=%d\n",
            pmd_present(*pmd), pmd_write(*pmd), pmd_user(*pmd),
            pmd_young(*pmd), pmd_dirty(*pmd));
        return 0;
    }
    
    // PTE (L0)
    pte = pte_offset_map(pmd, va);
    seq_printf(m, "PTE[%lu] @ %px = 0x%016llx\n",
        pte_index(va), pte, (unsigned long long)pte_val(*pte));
    
    if (!pte_present(*pte)) {
        seq_printf(m, "  [PAGE NOT PRESENT]\n");
        seq_printf(m, "  This page may be:\n");
        seq_printf(m, "    - Swapped out\n");
        seq_printf(m, "    - Demand-paged\n");
        seq_printf(m, "    - Never allocated\n");
        pte_unmap(pte);
        return 0;
    }
    
    // Decode PTE
    {
        unsigned long pa = pte_pfn(*pte) << PAGE_SHIFT;
        pa |= (va & ~PAGE_MASK);
        
        seq_printf(m, "  Physical Address: 0x%lx\n", pa);
        seq_printf(m, "  Flags:\n");
        seq_printf(m, "    Present=%d, Write=%d, User=%d\n",
            pte_present(*pte), pte_write(*pte), pte_user(*pte));
        seq_printf(m, "    Accessed=%d, Dirty=%d, Global=%d\n",
            pte_young(*pte), pte_dirty(*pte), pte_global(*pte));
    }
    
    pte_unmap(pte);
    return 0;
}

static int debug_pte_open(struct inode *inode, struct file *file) {
    return single_open(file, debug_pte_read, file->private_data);
}

static const struct proc_ops debug_pte_ops = {
    .proc_open = debug_pte_open,
    .proc_read = seq_read,
    .proc_lseek = seq_lseek,
    .proc_release = single_release,
};

static int __init debug_pte_init(void) {
    proc_create("debug_pte", 0644, NULL, &debug_pte_ops);
    pr_info("debug_pte module loaded. Read /proc/debug_pte\n");
    return 0;
}

static void __exit debug_pte_exit(void) {
    remove_proc_entry("debug_pte", NULL);
    pr_info("debug_pte module unloaded\n");
}

module_init(debug_pte_init);
module_exit(debug_pte_exit);
```

**Usage**:

```bash
# Compile
gcc -c debug_pte.c -I/lib/modules/$(uname -r)/build/include

# Insert
insmod debug_pte.ko

# Read (will show page tables for a test address)
cat /proc/debug_pte

# Output:
# === Page Table Walk for VA 0x7ffff6dd1000 ===
# PGD[0xf] @ 0xffffc9000f2f4f38 = 0x00000000410f8063
# P4D[0x1c] @ 0xffffc9000410f000 = 0x00000000410f7063
# PUD[0x1a] @ 0xffffc9000410f000 = 0x00000000410f6063
# PMD[0x1ad] @ 0xffffc9000410f000 = 0x0000000050001f67
#   [2MB HUGE PAGE at 0x50000000]
#   Flags: Present=1, Write=1, User=1, Accessed=1, Dirty=1
```

---

## 5.4 Production Monitoring with perf and ftrace

### TLB and Cache Misses

```bash
# Overview: Cache and TLB misses
perf stat -e cache-references,cache-misses,dTLB-loads,dTLB-load-misses \
    -u your_user_app -- ./application arg1 arg2

# Sampling: Record an event log for analysis
perf record -e dTLB-load-misses -c 10000 ./application arg1 arg2
perf report  # Interactive analysis

# Flamegraph: Identify hotspots
perf record -F 99 ./application
perf script > out.perf
./stackcollapse-perf.pl out.perf > out.folded
./flamegraph.pl out.folded > out.svg
```

### Kernel Page Fault Tracing

```bash
# Trace all page faults in real-time
trace-cmd record -e page_fault_user \
    -F "address > 0x7fff00000000" \
    ./application

# Or with ftrace
echo "p:pf_trace do_page_fault address=%di" > /sys/kernel/debug/tracing/kprobes
echo 1 > /sys/kernel/debug/tracing/events/kprobes/pf_trace/enable
cat /sys/kernel/debug/tracing/trace_pipe | head -20
```

---

# Part 6: Real-World Scenarios

## 6.1 Scenario: Cloud Workload Memory Isolation (Security)

### Problem

Two tenants' VMs on the same host:
- Tenant A (cryptocurrency exchange): Handles private keys in memory
- Tenant B (attacker): Runs speculative side-channel attack

### Threat Model

1. Tenant B executes Spectre gadget to leak Tenant A's memory through timing
2. Tenant B uses page table tricks to infer Tenant A's physical memory layout

### Solution: Nested/Extended Page Tables

```
Guest OS sees virtual addresses (GVAs)
  GVA → GPA (Guest Physical Address)
    [Guest Page Tables]

Hypervisor maps guest physical to host physical
  GPA → HPA (Host Physical Address)
    [Extended Page Tables / EPT / NPT]

Attacker's view:
  Can only guess GVA → GPA mappings (in guest)
  Cannot directly see HPA (protected by EPT)
  
Protection:
  Even if guest kernel is compromised, host memory is protected
  Each guest has separate EPTP (EPT Pointer in CR3), fully isolated
```

**Architecture**:

```
Guest A (Tenant)          Guest B (Tenant)
├─ GVA range 0-2GB        ├─ GVA range 0-2GB
│  └─ Page tables         │  └─ Page tables
│     (Accessible)        │     (Accessible)
│                         │
└─ GPA range 0-1GB        └─ GPA range 1-2GB
   (Guest's perception      (Guest's perception
    of physical memory)      of physical memory)


Hypervisor Layer:
┌────────────────────────────────────────┐
│ Extended Page Tables (EPT)             │
├──────────────────┬──────────────────┤
│ GPA 0-1GB → HPA  │ GPA 1-2GB → HPA  │
│  at 0xf000_0000  │  at 0x8000_0000  │
└────────────────────────────────────────┘

Physically isolated:
Guest A uses HPA 0xf000_0000-0xf3ff_ffff
Guest B uses HPA 0x8000_0000-0x83ff_ffff
Even if Guest B escapes GPA → HPA, it can't cross HPA boundary.
```

---

## 6.2 Scenario: Kernel Module Page Table Corruption Detection

### Problem

A faulty kernel module overwrites PTEs, corrupting mappings. This causes random memory corruptions in user-space applications.

### Detection Strategy

```c
// Kernel module: Monitor page table integrity

#include <linux/module.h>
#include <linux/kernel.h>
#include <linux/timer.h>
#include <asm/pgtable.h>

static struct timer_list check_timer;

// Hash page table every N seconds, detect changes
static void check_page_tables(struct timer_list *t) {
    struct task_struct *p;
    unsigned long pml4_pa, pml4_hash;
    
    rcu_read_lock();
    for_each_process(p) {
        if (!p->mm) continue;  // Skip kernel threads
        
        pml4_pa = __pa(p->mm->pgd);
        
        // Hash the PML4 table
        pml4_hash = crc32(0, (void *)p->mm->pgd, PAGE_SIZE);
        
        // Compare with last known hash
        if (pml4_hash != expected_hash[p->pid]) {
            pr_alert("ALERT: PML4 modified for PID %d! PA: 0x%lx\n",
                p->pid, pml4_pa);
            
            // Dump modified entries
            for (int i = 0; i < ENTRIES_PER_TABLE; i++) {
                if (expected_pml4[i] != p->mm->pgd[i]) {
                    pr_alert("  Entry [%d]: was 0x%016llx, now 0x%016llx\n",
                        i, expected_pml4[i], pgd_val(p->mm->pgd[i]));
                }
            }
        }
        
        expected_hash[p->pid] = pml4_hash;
    }
    rcu_read_unlock();
    
    // Schedule next check
    mod_timer(t, jiffies + HZ);  // Check every second
}

static int __init integrity_check_init(void) {
    timer_setup(&check_timer, check_page_tables, 0);
    mod_timer(&check_timer, jiffies + HZ);
    return 0;
}

module_init(integrity_check_init);
```

---

## 6.3 Scenario: Memory Hotplug and Page Table Expansion

### Problem

A server added 128GB of new DRAM. OS must expand page tables to address it.

### Solution

Linux supports memory hotplug:

```
Original address space:
┌──────────────────────┐
│ Kernel (high)        │ 0xffff800000000000+
├──────────────────────┤
│ User VAs             │ 0x0-0x7fffffffffff
└──────────────────────┘
Max addressable: 2^48 bytes

After adding 128GB:
├─ Identify new memory at BIOS/ACPI
├─ Kernel notifies memory management: add 128GB
├─ Page tables extended (already supported in x86-64)
├─ Zones updated: new memory placed in NUMA zones
├─ Buddy allocator tracks new free pages
└─ Applications can now request more (if mmap succeeds)

No PML4 modification needed (already have virtual space),
just buddy allocator state changes.
```

---

## 6.4 Scenario: Performance Tuning with Huge Pages

### Initial State: 4KB Pages Everywhere

```
Application: In-memory database with 256GB working set
Problem: Each 256GB / 4KB = 64M pages

TLB: 512 entries (typical L2 TLB on modern Intel)
Hit rate: 512 / 64M = 0.0008% → essentially all TLB misses
Page walk cycles per access: ~100 cycles on average
Performance: 25% loss from memory latency alone
```

### Solution: Huge Pages for Hot Data

```bash
# Enable 2MB huge pages
echo 2048 > /proc/sys/vm/nr_hugepages

# Allocate memory with huge pages
app_using_huge_pages > /tmp/app.log &
PID=$!

# Verify: app should use 2MB pages for hot regions
grep -i huge /proc/$PID/smaps | head -20

# Expected output:
# Size:              2097152 kB    (2MB huge page)
# Rss:               2097152 kB    (all resident)
# PSS:               2097152 kB    (no sharing)
# Shared_Clean:            0 kB
# Shared_Dirty:            0 kB
# Private_Clean: 2097152 kB
# Private_Dirty:          0 kB
# Referenced:      2097152 kB    (accessed)
# Anonymous:       2097152 kB
# AnonHugePages:   2097152 kB ← Uses huge pages!
```

### Result

```
Before (4KB pages):
  Working set: 256GB = 64M pages
  TLB capacity: 512 entries
  TLB miss ratio: ~99.9%
  Avg latency: 100 cycles per TLB miss
  Performance: Baseline

After (2MB huge pages):
  Working set: 256GB = 128k pages (512× reduction)
  TLB capacity: 512 entries
  TLB miss ratio: ~0.1% (fits in TLB!)
  Avg latency: ~3 cycles (TLB hits)
  Performance: 5-10× faster for memory-bound workload
```

---

## 6.5 Scenario: Kernel Address Space Layout Randomization (KASLR)

### Problem

Kernel base address is predictable, allowing ROP gadget chains.

### Solution: Randomize PML4 Entries

```
Classic kernel:
  Kernel code: 0xffffffff80000000 (always the same)
  Kernel data: 0xffffffff8xxxx000 (always the same)
  Attacker knows exact ROP gadget addresses

KASLR-enabled kernel:
  Kernel randomized to: 0xffffffff80000000 + random_offset
  With 40-bit randomization: 2^40 possible locations
  Attacker cannot predict gadget addresses
  Each boot uses different kernel base
```

**Implementation**:

```c
// During early boot: calculate randomoffset

unsigned long kaslr_offset = get_random_long() & 0xFFFFF000;  // 4KB aligned
unsigned long kernel_base = 0xffffffff80000000 + kaslr_offset;

// All kernel references use: kernel_base
//   Text:  kernel_base + 0x000000 (code)
//   Data:  kernel_base + 0x400000 (rwdata)
//   BSS:   kernel_base + 0x500000 (uninitialized)

// PML4 entries updated:
pgd_val(kernel_pgd[0xff]) = __pa(kernel_pagetables) | PGDIR_ENTRY_FLAGS;
//                            ^ Points to actual page tables at randomized location
```

---

## Summary: Mental Model Building

### Key Insights Learned

1. **Page tables are trees, not arrays**
   - Multi-level prevents massive allocations
   - Lazy allocation only pays for used address ranges
   - Trade-off: 4 memory accesses per translation (TLB hides this)

2. **Permission bits are enforced by MMU, not software**
   - Present, Writable, Executable, User/Kernel are hardware-checked
   - Violations → automatic exceptions → kernel fault handler
   - No slow permission checking per access

3. **TLB is the critical performance lever**
   - Miss rate determines 50-80% of memory performance
   - Huge pages → 100-512× TLB efficiency gain
   - Address locality matters more than total memory size

4. **Virtual address space is abundant**
   - 48-bit user space = 256TB per process (never runs out)
   - Kernel uses fixed high addresses (shared across processes)
   - Canonical form prevents gaps exploitation

5. **Context switching is expensive**
   - Must flush TLB (CR3 change) = 100-1000 cycles
   - ASID (ARM64) reduces cost by caching multi-process TLB
   - Processes with high context-switch rates lose 10-40% performance

6. **Kernel memory is invisible to user-space**
   - SMEP/SMAP/PTI/pkeys provide isolation layers
   - Even with kernel exploits, user data protected by separate tables
   - Speculative attacks require multiple layers to cross boundary

---

**Next Steps**: Choose a specific scenario (TLB optimization, security hardening, memory profiling, kernel module development) and dig deeper with actual workloads on your development VM.

