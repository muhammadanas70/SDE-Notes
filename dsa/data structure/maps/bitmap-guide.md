# The Bitmap: A Complete Engineering Guide

*Bit arrays, bitsets, bitmaps — theory, hardware, Linux kernel internals, and
production-grade implementations in C, Rust, and Go.*

---

## Table of Contents

1. [What a Bitmap Actually Is](#1-what-a-bitmap-actually-is)
2. [The Mental Model: Bits, Words, and Addressing](#2-the-mental-model-bits-words-and-addressing)
3. [Core Bitwise Operations](#3-core-bitwise-operations)
4. [Bitmap Primitive Operations](#4-bitmap-primitive-operations)
5. [Memory Layout, Alignment, and Cache Behavior](#5-memory-layout-alignment-and-cache-behavior)
6. [Concurrency: Atomics, Ordering, and Lock-Free Bit Ops](#6-concurrency-atomics-ordering-and-lock-free-bit-ops)
7. [Variants: Fixed, Dynamic, Sparse, Compressed](#7-variants-fixed-dynamic-sparse-compressed)
8. [Linux Kernel Bitmap Subsystem](#8-linux-kernel-bitmap-subsystem)
9. [Real Kernel Architectures Using Bitmaps](#9-real-kernel-architectures-using-bitmaps)
10. [Userspace Systems Using Bitmaps](#10-userspace-systems-using-bitmaps)
11. [C Implementation](#11-c-implementation)
12. [Rust Implementation](#12-rust-implementation)
13. [Go Implementation](#13-go-implementation)
14. [Complexity, Performance, and When Not to Use a Bitmap](#14-complexity-performance-and-when-not-to-use-a-bitmap)
15. [Mental Model Summary](#15-mental-model-summary)

---

## 1. What a Bitmap Actually Is

A **bitmap** (also called a **bit array**, **bitset**, or **bit vector**) is a data
structure that stores **one bit of state per element of a domain**, packed
contiguously in memory. If you have N things and each thing needs a single
yes/no fact recorded about it, a bitmap stores that fact in `N` bits instead of
`N` bytes, `N` words, or `N` objects.

Two words matter more than any others in this guide: **domain** and **packing**.

- **Domain** — the bitmap does not store values; it stores *presence/absence,
  set/unset, allocated/free, dirty/clean* for a fixed, indexable universe.
  Index 37 in the bitmap means "the 37th thing in some external, ordered
  domain" — a page frame number, a process ID, an inode number, a TCP flag,
  a CPU number, a day of the month.
- **Packing** — 8 bits share one byte, 64 bits share one 64-bit word on a
  modern machine. This is the entire reason bitmaps exist: word-level CPU
  instructions (AND, OR, XOR, shifts, `popcount`, `bsf`/`bsr`) turn an
  operation over 64 boolean values into **one instruction** instead of 64
  branches.

This single property — turning per-element work into per-word work — is why
bitmaps appear at every layer of a real operating system: physical memory
allocation, CPU scheduling, filesystem block/inode allocation, process ID
allocation, network packet classification, permission checks, signal masks,
and much more. Once you see "a fixed universe of N discrete things, each with
one bit of state" you should immediately think "bitmap."

### What a bitmap is *not*

- It is **not** a general associative container. You cannot store arbitrary
  values in it — only 1 bit per index (unless you build a multi-bit-per-slot
  variant, covered in §7).
- It is **not** automatically sparse-efficient. A bitmap over a domain of 2^32
  elements is 512 MiB whether 3 bits or 3 billion bits are set. Sparse
  domains need compression (§7) or a different structure (hash set, tree).
- It is **not** a substitute for a `bool[]` array conceptually — it *is* a
  `bool[]` array, just packed 8×/64× denser and operated on at word
  granularity.

---

## 2. The Mental Model: Bits, Words, and Addressing

Think of a bitmap as one long ribbon of bits, then chunked into
machine-word-sized tiles for storage:

```
Logical bit index:    0   1   2   3   4   5   6   7   8   9  ...  63  64  65 ...
                       +---+---+---+---+---+---+---+---+---+---+ ... +---+---+
Ribbon (conceptual):   | 0 | 1 | 1 | 0 | 1 | 0 | 0 | 1 | 0 | 1 | ... | 1 | 0 |...
                       +---+---+---+---+---+---+---+---+---+---+ ... +---+---+

Physical storage (64-bit words, little-endian bit order within a word):

        word[0]  (bits 0..63)             word[1]  (bits 64..127)
   MSB                              LSB
   63 ......................... 1  0        127 ..................... 65 64
  +--------------------------------+       +--------------------------------+
  | ... 1  0  0  1  0  1  0  1  1  0|       | ...          0          1      |
  +--------------------------------+       +--------------------------------+
                                bit 0 is the least-significant bit of word[0]
```

### Index arithmetic

Given a logical bit index `i` and a word size `W` (typically 32 or 64):

```
word_index   = i / W        (i >> log2(W)   — a shift, not a divide)
bit_in_word  = i % W        (i &  (W - 1)    — a mask, not a modulo)
mask         = 1  <<  bit_in_word
```

Every single bitmap operation — set, clear, test, toggle — reduces to this
same three-line arithmetic followed by one bitwise instruction on
`storage[word_index]`. This is the entire "trick" of a bitmap; everything
else in this document is refinement, hardening, and application.

```
storage[] = [ word0 ][ word1 ][ word2 ][ word3 ] ...
              64 bits  64 bits  64 bits  64 bits

bit i = 130  →  word_index = 130 / 64 = 2
                bit_in_word = 130 % 64 = 2
                → touch bit 2 of storage[2]
```

### Bit ordering conventions (a real source of bugs)

There are two independent ordering questions, and conflating them causes
real-world interoperability bugs:

1. **Byte order** (endianness) — which byte of a multi-byte word is stored
   first in memory (little-endian x86/ARM vs big-endian).
2. **Bit numbering within a word** — LSB0 (bit 0 = least significant bit,
   used by Linux kernel bitmaps, x86 `BT`/`BTS` instructions) vs MSB0 (bit 0
   = most significant bit, used in some networking RFCs and hardware
   datasheets).

Linux kernel bitmaps are **LSB0-numbered**, stored in **native machine word
order** (`unsigned long[]`), which is why `find_first_bit()` on x86 can be a
single `BSF` (bit-scan-forward) instruction. Network protocol bit diagrams in
RFCs are almost always **MSB0** — bit 0 is drawn as the leftmost, most
significant bit of the octet. When you translate an RFC packet-header
bitfield into a kernel bitmap, this mismatch is a classic off-by-mirror bug.

---

## 3. Core Bitwise Operations

These are the six instructions a bitmap is built from. On virtually all
architectures each is a single-cycle ALU op.

| Operation | Symbol | Effect on bit `b` | Typical use |
|---|---|---|---|
| AND | `&` | keeps `b` only if both operands have it | intersection, masking off bits |
| OR | `\|` | sets `b` if either operand has it | union, setting a bit |
| XOR | `^` | sets `b` if exactly one operand has it | toggle, difference, checksums |
| NOT | `~` | flips every bit | complement of a set |
| Left shift | `<<` | moves bits toward higher index | building a mask, `1 << n` |
| Right shift | `>>` | moves bits toward lower index | extracting a field |

### Set-theoretic reading of a bitmap

If bitmap `A` represents set `S_A` and bitmap `B` represents set `S_B` over
the same domain:

```
A & B   = S_A ∩ S_B   (intersection)
A | B   = S_A ∪ S_B   (union)
A ^ B   = S_A Δ S_B   (symmetric difference)
~A      = complement of S_A within the domain
A & ~B  = S_A - S_B   (set difference / "A but not B")
```

This is precisely how the kernel computes things like "which CPUs are online
**and** in this cgroup's cpuset" (`cpumask_and`), or how a packet classifier
computes "which ACL rules match **all** of these five header-field bitmaps"
(bitmap intersection across dimensions — the essence of bitmap-based packet
classification algorithms like BV/ABV, §10).

### Derived bit-twiddling primitives

Built from the six ops above, these appear constantly in kernel and systems
code:

```
Isolate lowest set bit:        x & (-x)              (two's complement trick)
Clear lowest set bit:          x & (x - 1)            (used in popcount loops)
Check power of two:            x && !(x & (x - 1))
Round up to next power of two: see clz-based formulas
Create mask of low n bits:     (1UL << n) - 1
Sign/no-branch min:            uses shift + xor tricks (rarely needed today;
                                CMOV/branchless intrinsics preferred)
```

---

## 4. Bitmap Primitive Operations

Every bitmap library, whether it is 40 lines of C in the kernel or a crate on
crates.io, converges on the same primitive operation set. Learn these once;
everything else (§7–§13) is these primitives applied to a domain.

### 4.1 `set_bit(bitmap, i)`
```
storage[i / W]  |=  (1 << (i % W))
```
OR-ing in a 1 never clears a neighboring bit — this is why OR is the "set"
operation: it is idempotent and only ever turns bits on.

### 4.2 `clear_bit(bitmap, i)`
```
storage[i / W]  &=  ~(1 << (i % W))
```
AND-ing with an all-ones-except-position-i mask clears exactly one bit.

### 4.3 `test_bit(bitmap, i)`
```
return (storage[i / W]  >>  (i % W))  &  1
```
Shift the target bit down to position 0, mask off everything else.

### 4.4 `toggle_bit(bitmap, i)`
```
storage[i / W]  ^=  (1 << (i % W))
```
XOR flips exactly one bit regardless of its previous state — no branch
needed, which is why it is preferred over "read, negate, write" in hot paths.

### 4.5 `find_first_set_bit` / `find_first_zero_bit`

Naively this is a loop over every bit. Real implementations instead loop over
**words**, and use a hardware instruction to jump straight to the answer
within the first non-zero (or non-all-ones) word:

- x86: `BSF` (bit scan forward), `TZCNT` (count trailing zeros)
- ARM64: `RBIT` + `CLZ`, or `CLZ` directly for find-highest
- Compiler intrinsics: `__builtin_ctzl`, `__builtin_clzl` (GCC/Clang),
  `_BitScanForward` (MSVC), `u64::trailing_zeros()` (Rust),
  `bits.TrailingZeros64()` (Go)

```
find_first_zero_bit(bitmap):
    for word_index in 0 .. num_words:
        w = storage[word_index]
        if w != ALL_ONES:                 # this word has a free bit
            inverted = ~w
            bit = trailing_zeros(inverted) # hardware instruction, O(1)
            return word_index * W + bit
    return NOT_FOUND
```

This is the single most important optimization in every real bitmap
implementation: **skip whole words at a time**, and let one CPU instruction
resolve the winning bit within the word that matters. This is exactly how
Linux's page allocator and inode/block allocators avoid an O(N) bit-by-bit
scan over gigabytes of memory.

### 4.6 `popcount` (Hamming weight — count of set bits)

Used for "how many things are allocated", "how many CPUs are in this mask",
checksums, error-correction codes, and as a building block for rank/select
structures in compressed bitmaps (§7.3).

```
Naive (Kernighan's bit-counting trick):
    count = 0
    while x != 0:
        x = x & (x - 1)     # clears the lowest set bit each iteration
        count += 1
    # loop runs popcount(x) times, not word-width times — output-sensitive
```

Modern hardware has a dedicated instruction: x86 `POPCNT`, ARM64 `CNT` (per
byte, then horizontally summed), exposed as `__builtin_popcountll`,
`u64::count_ones()` in Rust, `bits.OnesCount64()` in Go. Always prefer the
intrinsic — it is a single cycle versus a data-dependent loop.

### 4.7 `find_next_bit(bitmap, start)`

The workhorse of *iterating a bitmap* — "give me every allocated page frame",
"give me every CPU in this mask" — without scanning bits you've already
visited:

```
find_next_set_bit(bitmap, start):
    word_index = start / W
    bit_offset = start % W
    w = storage[word_index] >> bit_offset       # mask off already-visited bits
    if w != 0:
        return start + trailing_zeros(w)
    for word_index in word_index+1 .. num_words:
        if storage[word_index] != 0:
            return word_index * W + trailing_zeros(storage[word_index])
    return NOT_FOUND
```

Kernel code almost never writes a raw `for (i = 0; i < n; i++) if
(test_bit(i))` loop — it uses `for_each_set_bit(i, bitmap, size)`, which is
exactly this word-skipping `find_next_bit` under the hood (see §8.4).

---

## 5. Memory Layout, Alignment, and Cache Behavior

A bitmap's entire performance story is about **cache lines**, not
instructions — the ALU ops above are all sub-nanosecond; the cost is getting
the right word into L1 cache.

```
                              64-byte cache line
        +----------------------------------------------------------+
        |  word0 | word1 | word2 | word3 | word4 | word5 | word6 |w7|
        +----------------------------------------------------------+
           8B       8B      8B      8B      8B      8B      8B    8B

        One cache line of a bitmap covers 64 bytes * 8 bits/byte
        = 512 logical bits.
```

Consequences that matter in real systems code:

- **Sequential scans are cheap.** `find_next_bit` walking forward touches
  each cache line once and then does 512 bits of work per line — this is
  why bitmap allocators outperform linked-list-of-free-blocks allocators
  under contention and cache pressure.
- **Random access across a huge bitmap is expensive.** A 1 GiB bitmap
  covering an 8 GiB address space at page (4 KiB) granularity is 256 KiB —
  fits in L2 on most modern CPUs, but a *terabyte*-scale server's page
  bitmap can be tens of MiB, meaning random bit tests thrash cache. This is
  one reason the kernel uses multi-level structures (buddy allocator free
  lists per order, radix trees, `xarray`) rather than one flat bitmap over
  all of physical memory (§9.1).
- **False sharing.** If two independent, frequently-updated flags land in
  the *same word* (e.g., bit 3 = "CPU 3 busy", bit 4 = "CPU 4 busy") two
  cores writing "adjacent" bits will bounce the same 64-bit word, and by
  extension the same 64-byte cache line, between their caches via MESI —
  even though logically they touch disjoint bits. This is why per-CPU
  kernel data (`percpu` variables) is deliberately **not** a shared bitmap
  indexed by CPU number for hot-path flags; each CPU gets its own cache-line-
  aligned data instead, and shared bitmaps are reserved for genuinely
  cross-CPU state that must be visible everywhere (like `cpu_online_mask`),
  accepting the sharing cost because updates are rare (hotplug events).
- **Alignment.** Kernel bitmap storage is always declared as an array of
  `unsigned long` (`DECLARE_BITMAP`, §8.1) — never `char[]` — specifically
  so that word-at-a-time ops are naturally aligned and the compiler can
  emit single-instruction loads/stores instead of byte-assembly code.

```
Cache-line-conscious layout for a per-CPU "busy" flag (WRONG vs RIGHT):

WRONG (shared word, false sharing):
   storage[0] bit0=CPU0 bit1=CPU1 bit2=CPU2 bit3=CPU3 ...
   → every CPU's flag update dirties the same cache line for all CPUs.

RIGHT (kernel's actual approach for hot per-CPU flags):
   percpu_flag[CPU0] -> own cache line
   percpu_flag[CPU1] -> own cache line
   percpu_flag[CPU2] -> own cache line
   (bitmaps like cpu_online_mask remain a single bitmap because they are
    read far more often than written, and writes are rare hotplug events —
    the trade-off is deliberate, not accidental.)
```

---

## 6. Concurrency: Atomics, Ordering, and Lock-Free Bit Ops

Multiple CPUs frequently need to set/clear/test bits in the *same* bitmap
concurrently — page allocation bitmaps, CPU masks, IDA/IDR ID allocators.
Plain `|=`/`&=` is a **read-modify-write** and is *not* atomic; two CPUs
racing on the same word can both read the old value, both compute their own
update, and one write clobbers the other ("lost update").

### 6.1 Atomic bit operations

Every real implementation exposes atomic variants built on hardware
read-modify-write instructions:

- x86: `LOCK BTS` (bit test and set), `LOCK BTR` (bit test and reset), `LOCK
  CMPXCHG` for compare-and-swap loops
- ARM64: load-linked/store-conditional (`LDXR`/`STXR`) or, on newer cores,
  native atomic instructions (`LDSET`, `LDCLR` — atomic bit-set/clear)
- Semantics: `test_and_set_bit(i)` atomically sets bit `i` **and** returns
  its *previous* value in one indivisible step — this is the primitive that
  lets two threads race to "claim" the same resource (e.g., the same page
  frame) and know, without a separate lock, which one won.

```
Thread A                          Thread B
--------                          --------
test_and_set_bit(42)              test_and_set_bit(42)
   reads bit 42 = 0                  reads bit 42 = 1   (A's write is visible)
   sets bit 42 = 1                   sets bit 42 = 1 (no-op, already set)
   returns 0  → "I won, it was free"  returns 1  → "someone else already has it"
```

Without the atomicity, both threads could observe `0`, both believe they won,
and both proceed to use the same page frame — a real, historically-exploited
class of race condition in allocators.

### 6.2 Memory ordering

An atomic bit op guarantees *that bit's* update is indivisible, but says
nothing by itself about the visibility ordering of *other* memory the thread
touched before/after. Kernel and lock-free code must pair bit operations with
explicit memory barriers or use ordering-annotated atomics
(`atomic_fetch_or_acquire`, `release`, etc. in C11/C++20; `Ordering::Acquire`
/`Release`/`SeqCst` in Rust; `atomic.CompareAndSwap` + happens-before rules in
Go) so that, e.g., "bit says page is allocated" is guaranteed to be visible
*before* another CPU can observe writes into that page's contents.

### 6.3 Lock-free patterns built on atomic bit ops

- **CAS retry loop** for multi-bit updates (set several bits as one logical
  transaction): read word, compute new word, `compare_exchange(old, new)`,
  retry on failure. This is the general pattern when a single hardware bit
  instruction isn't enough because you need to change more than one bit
  atomically as a unit.
- **Per-word spinlock avoidance**: because a whole word (64 bits) is the
  atomic unit, some designs deliberately assign disjoint "ownership ranges"
  of a bitmap per CPU/thread to avoid CAS contention entirely (see `percpu`
  IDA batches in the kernel — each CPU pre-reserves a private range of IDs
  from a shared ID space to avoid hitting the shared bitmap on every
  allocation).

---

## 7. Variants: Fixed, Dynamic, Sparse, Compressed

### 7.1 Fixed-size bitmap

Size known at compile time. `DECLARE_BITMAP(name, 256)` in C, `[u64; 4]` in
Rust, `[8]uint32` in Go. Zero allocation overhead, used whenever the domain
size is a compile-time constant (CPU count ceiling, signal count, page flags
per page).

### 7.2 Dynamic bitmap

Size known only at runtime — number of inodes in a freshly-mounted
filesystem, number of online CPUs detected at boot, number of connections in
a connection tracker. Backed by a heap-allocated word array plus a stored bit
length; must support **growth** (realloc + zero-extend) if the domain can
expand (e.g., CPU hotplug growing `nr_cpu_ids`).

### 7.3 Sparse / compressed bitmaps

When the domain is huge (2^32+) but only a small fraction of bits are ever
set, a flat bitmap wastes enormous memory. Three families of solution exist,
each a real production technique:

**a) Run-length encoding (RLE) bitmaps** — store alternating run-lengths of
0s and 1s instead of raw bits. Excellent when data is clustered (e.g., "all
inodes 0–9999 allocated, then a long gap"). Classic academic form:
**WAH (Word-Aligned Hybrid)** and **EWAH (Enhanced WAH)** compress runs of
all-zero or all-one *words*, keeping "literal" (mixed) words uncompressed —
this preserves most of the O(1)-per-word operation speed of a plain bitmap
while compressing long runs, and is used in analytics databases (e.g.
historically in Apache Hive/Druid bitmap indexes) for column value
membership.

**b) Roaring bitmaps** — partition the 32-bit domain into 2^16 "chunks" of
2^16 elements each. Each chunk is stored using whichever of three
representations is smallest for its actual density: a plain 8 KiB bitmap
(dense chunks), a sorted array of 16-bit values (sparse chunks), or a
run-length list (bursty chunks). This adaptive per-chunk choice is why
Roaring bitmaps dominate in modern search engines and analytics engines
(Lucene/Elasticsearch postings lists, ClickHouse, Pilosa, Spark) — they get
near-flat-bitmap speed on dense regions and near-hash-set memory efficiency
on sparse regions, in the *same* structure, automatically.

```
Roaring bitmap conceptual layout for a 32-bit domain:

  high 16 bits select a "container"; low 16 bits index within it

  key=0x0000 -> [ARRAY container]   {3, 57, 1090, ...}      (sparse: few bits)
  key=0x0001 -> [BITMAP container]  8192 bytes, dense bits   (dense: many bits)
  key=0x0002 -> [RUN container]     [(start=0,len=4000), ...] (long runs)
  key=0x0003 -> (absent — no bits set anywhere in this 65536-range)
```

**c) Multi-bit-per-slot "bitmaps"** — technically not 1-bit-per-element, but
the same packing philosophy applied to small fixed-width fields (2-bit page
color, 4-bit reference count nibble) — sometimes called "bit-packed arrays."
Used when you need more than boolean state but still want dense, cache-
friendly, word-aligned storage (e.g., 2-bit-per-page "age" counters in some
page-replacement approximations).

### 7.4 Bloom filters (bitmap + hashing)

A **Bloom filter** is a bitmap of size `m` combined with `k` independent hash
functions. "Insert x" sets the `k` bits `hash_1(x)...hash_k(x)`; "query x"
checks whether all `k` of those bits are set. It never has false negatives
but can have false positives, trading a small, tunable error rate for
massively sub-linear memory versus a full hash set. This is a bitmap
application, not a bitmap variant per se, but it belongs in the mental model
because it is the single most common "bitmap + hashing" building block in
distributed systems (SSTable existence checks in LSM-tree databases like
RocksDB/Cassandra, network packet de-duplication, spell-checkers, and
per-flow existence checks in some DDoS-mitigation designs).

```
Bloom filter insert("192.168.1.1"):

   hash1("192.168.1.1") -> bit 7    ─┐
   hash2("192.168.1.1") -> bit 41   ─┼─> set all three bits to 1
   hash3("192.168.1.1") -> bit 88   ─┘

   bitmap: ...0 0 0 0 0 0 0 [1] 0 ... 0 [1] 0 ... 0 [1] 0 ...
                            bit7          bit41        bit88

Query("10.0.0.5"): if hash1/2/3 land on bits that are ALL already 1
                    → "possibly present" (may be a false positive)
                    if ANY of the three bits is 0
                    → "definitely absent" (no false negatives, guaranteed)
```

---

## 8. Linux Kernel Bitmap Subsystem

The kernel's bitmap API lives primarily in `include/linux/bitmap.h`,
`include/linux/bitops.h`, and architecture-specific `arch/*/include/asm/
bitops.h` for the hardware-accelerated primitives. This is the single most
battle-tested bitmap implementation in existence — billions of devices run
it continuously.

### 8.1 Declaring a bitmap

```c
#include <linux/bitmap.h>

#define MY_DOMAIN_SIZE  1024

/* Expands to: unsigned long name[BITS_TO_LONGS(bits)]; */
DECLARE_BITMAP(my_bitmap, MY_DOMAIN_SIZE);
```

`BITS_TO_LONGS(n)` computes `DIV_ROUND_UP(n, BITS_PER_LONG)` — the number of
`unsigned long` words needed to hold `n` bits, rounding up so a
non-word-multiple bit count still gets fully covered (with unused high bits
in the final word left as padding, which the API is careful to keep zeroed
where semantics depend on it, e.g. `find_first_zero_bit` must not "find" a
padding bit past the real domain).

`BITS_PER_LONG` is 32 or 64 depending on architecture word size — this is
precisely why kernel bitmaps are declared as arrays of `unsigned long` and
not `uint8_t`: the whole point is to operate at native register width.

### 8.2 Core kernel bitmap API (representative, not exhaustive)

```c
void set_bit(int nr, volatile unsigned long *addr);
void clear_bit(int nr, volatile unsigned long *addr);
int  test_bit(int nr, const volatile unsigned long *addr);
int  test_and_set_bit(int nr, volatile unsigned long *addr);   /* atomic */
int  test_and_clear_bit(int nr, volatile unsigned long *addr); /* atomic */

unsigned long find_first_bit(const unsigned long *addr, unsigned long size);
unsigned long find_first_zero_bit(const unsigned long *addr, unsigned long size);
unsigned long find_next_bit(const unsigned long *addr, unsigned long size,
                             unsigned long offset);

void bitmap_zero(unsigned long *dst, unsigned int nbits);
void bitmap_fill(unsigned long *dst, unsigned int nbits);
void bitmap_and(unsigned long *dst, const unsigned long *s1,
                 const unsigned long *s2, unsigned int nbits);
void bitmap_or (unsigned long *dst, const unsigned long *s1,
                 const unsigned long *s2, unsigned int nbits);
int  bitmap_weight(const unsigned long *src, unsigned int nbits); /* popcount */
int  bitmap_empty(const unsigned long *src, unsigned int nbits);
int  bitmap_full (const unsigned long *src, unsigned int nbits);
```

Two crucial design details:

- **`volatile` and explicit atomics** — `test_and_set_bit` is defined
  per-architecture using `LOCK BTS` (x86) or LL/SC loops (ARM), guaranteeing
  atomicity across CPUs; plain `set_bit`/`clear_bit` are atomic *with
  respect to concurrent bit ops on the same word* on most architectures but
  callers must still reason about ordering relative to *other* memory
  accesses (§6.2) — the kernel documents which barriers each variant
  implies.
- **Non-atomic variants exist too** — `__set_bit`, `__clear_bit`,
  `__test_and_set_bit` (double-underscore prefix is the kernel's naming
  convention for "no locking, caller must already hold appropriate
  protection") — used when the caller already holds a spinlock or otherwise
  knows there's no concurrent access, to avoid the cost of a locked
  instruction on every single-threaded update.

### 8.3 `for_each_set_bit` and friends

```c
#define for_each_set_bit(bit, addr, size) \
    for ((bit) = find_first_bit((addr), (size)); \
         (bit) < (size); \
         (bit) = find_next_bit((addr), (size), (bit) + 1))
```

This macro is how the kernel iterates "every set bit" efficiently — every
usage of "for every online CPU", "for every allocated block in this group",
"for every raised signal" ultimately expands to this word-skipping loop, not
a naive bit-by-bit `for (i = 0; i < size; i++)`.

### 8.4 `cpumask_t` and `nodemask_t`

CPU affinity, online/offline/present/active CPU tracking, and NUMA node
sets are, under the hood, exactly the bitmap primitives above with a
type-safe wrapper (`struct cpumask { unsigned long bits[...]; }`) and a
dedicated API (`cpumask_set_cpu`, `cpumask_test_cpu`, `cpumask_and`,
`for_each_cpu`) so that CPU-mask code cannot accidentally be passed a
generic bitmap of the wrong size, and so that on kernels built for a fixed,
small `NR_CPUS` the compiler can specialize the bitmap operations into a
single machine word (`unsigned long` on a ≤64-CPU kernel build is one word —
`cpumask_set_cpu` compiles to one instruction).

```
struct cpumask (conceptual, for NR_CPUS = 128, so 2 words of 64 bits):

  word0: CPU63 ................................... CPU0
  word1: CPU127................................... CPU64

cpumask_set_cpu(70, mask)
   -> word_index = 70/64 = 1, bit = 70%64 = 6
   -> mask->bits[1] |= (1UL << 6)
```

### 8.5 IDR / IDA — ID allocation on top of bitmaps

The kernel needs to hand out small integer IDs constantly (file descriptors
historically, POSIX timer IDs, some device minor numbers) — **IDA** (ID
Allocator, the simpler, integer-only sibling of **IDR**, which maps IDs to
pointers via a radix tree) is fundamentally "find the lowest clear bit,
atomically claim it, extend the underlying bitmap if all words are full."
Modern IDA implementation uses an **XArray** of per-range bitmap chunks with
a **per-CPU percpu allocation cache** (§6.3) precisely so that ID allocation
under heavy concurrent load doesn't serialize every CPU on one shared
bitmap's cache line.

---

## 9. Real Kernel Architectures Using Bitmaps

### 9.1 Physical page frame allocation — the buddy allocator

Linux does **not** use one giant flat bitmap over all of physical memory for
its primary allocator (that idea, "bitmap allocator," exists as a *simpler*
teaching/embedded technique — see the contrast below) — it uses the **buddy
system**: an array of free-lists, one per allocation order (order 0 = 1
page, order 1 = 2 pages, ... order 10 = 1024 pages), and a **1-bit-per-buddy-
pair** bitmap at each order recording only whether a given buddy pair is
"both free, and thus mergeable" — not whether each individual page is free
(that's tracked by list membership, not a bitmap, in modern Linux; classic
textbook buddy-allocator descriptions use a bitmap for this "is my buddy
also free" bit, which is the part of the design that actually is a bitmap).

```
Buddy allocator free-list + coalescing-bit structure (conceptual):

Order 0 (4 KiB pages):   free_list -> [pg7]->[pg12]->[pg13]->NULL
Order 1 (8 KiB, 2 pgs):  free_list -> [pg8-9]->NULL
Order 2 (16 KiB, 4 pgs): free_list -> [pg20-23]->[pg100-103]->NULL
...
Order 10 (4 MiB, 1024pgs): free_list -> [pg0-1023]->NULL

Each order's "buddy bitmap" bit records: "is my buddy block
ALSO on the free list at this order?" — one bit per (block, order) pair.

  page pair (12,13) at order0:  bit = 1  → both free → can COALESCE
                                            into one order-1 block
  page pair (8,9)   at order1:  bit = 0  → buddy (10,11) not free
                                            → cannot coalesce further

           order 2  [ 8 . . . 11 ]            <- would-be merged block
                        ↑ merge ↑
           order 1  [ 8 . 9 ]  [10 . 11]      <- 8-9 free, 10-11 NOT free
                       bit=1     (buddy not free, bit stays 0)
           order 0  [8][9][10][11]
```

Contrast: many **embedded / simpler kernels** and the classic **"bitmap page
allocator"** design (also used historically, and still used in some
allocators like early Linux `mm/bootmem.c` for early-boot memory before the
buddy allocator is initialized) *do* use one flat bit per physical page:

```
bootmem-style flat page bitmap:

  bit i = 0  →  page frame i is FREE
  bit i = 1  →  page frame i is ALLOCATED / reserved

  storage: [ word0 ][ word1 ][ word2 ] ... one bit per 4 KiB page frame

  Allocating N contiguous pages = find_next_zero_bit N times in a row
  (a contiguous run of zero bits), then set_bit on each — this is
  literally §4.5's find_first_zero_bit applied at scale, and it is
  exactly why early-boot memory allocation before the buddy allocator
  and its per-order freelists are initialized falls back to this
  simpler, flat-bitmap design (mm/bootmem.c / the earlier "bootmem
  allocator", superseded functionally by memblock which itself uses
  sorted region arrays rather than a bitmap — the historical bitmap-
  based bootmem is the clean pedagogical example even where current
  kernels have moved to memblock's array-based approach).
```

### 9.2 Filesystem block and inode allocation (ext2/ext3/ext4)

This is the canonical, most-quoted real-world bitmap architecture, and it
maps almost one-to-one onto everything in §4:

```
ext4 disk layout (simplified) — one "block group" of many:

+----------+----------+-------------+-------------+------------------+
| Superblk | GroupDesc| Block Bitmap| Inode Bitmap|  Inode Table | Data Blocks... |
+----------+----------+-------------+-------------+------------------+
                            1 block      1 block     N blocks

Block Bitmap  (1 bit per DATA BLOCK in this group):
   bit i = 0 → block i is FREE
   bit i = 1 → block i is IN USE

Inode Bitmap  (1 bit per INODE SLOT in this group):
   bit i = 0 → inode i is FREE
   bit i = 1 → inode i is ALLOCATED

Allocating a new file's data block:
   1. find_next_zero_bit(block_bitmap, group_size)   <- §4.5, this exact op
   2. atomically test_and_set_bit() to claim it       <- §6.1, this exact op
   3. if the whole group's bitmap is full (bitmap_full, §8.2),
      move to the next block group and retry
   4. update the group descriptor's free-block counter (a plain integer,
      cached specifically so "is this group worth trying" doesn't require
      a popcount scan on every allocation)
```

Every one of ext4's block-group allocation policies (try to keep a file's
blocks in the same group as its inode; try to extend allocations
contiguously; the "multiblock allocator", mballoc, which additionally
layers **buddy-style bitmaps per order** on top of the raw block bitmap to
speed up finding runs of N contiguous free blocks) is bitmap-scanning
optimization *on top of* this exact bit-per-block foundation. `mballoc`'s
per-order "buddy bitmaps" are the filesystem-level cousin of §9.1's page
buddy allocator, for exactly the same reason: finding a run of K contiguous
free bits by scanning single bits is too slow, so a hierarchy of "is this
2^k-aligned, 2^k-sized chunk fully free" bits is precomputed and maintained
incrementally.

### 9.3 Process/signal bitmaps

- **Signal pending masks** (`sigset_t`) — one bit per signal number (1–64),
  tested with `sigismember`, manipulated with `sigaddset`/`sigdelset` —
  conceptually identical to `test_bit`/`set_bit` over a 64-bit domain,
  because there are ≤ 64 (or 128 with real-time signals, needing 2 words)
  signal numbers.
- **`/proc/<pid>/status` `CapBnd`/`CapEff`** — Linux capability sets are
  bitmaps over the ~40 defined capabilities (`CAP_NET_ADMIN`,
  `CAP_SYS_ADMIN`, ...), tested with the exact same `test_bit` pattern in
  `security/commoncap.c`.
- **`cpuset`/cgroup `cpus`/`mems` files** — parsed into and displayed from
  `cpumask_t`/`nodemask_t` (§8.4), including the classic
  `"0-3,7,9-11"` range-list textual format that `bitmap_parselist()` and
  `bitmap_print_to_pagebuf()` convert to/from the underlying bitmap.

### 9.4 Networking: packet classification and connection tracking

- **ACL / firewall rule matching** — classic **bit-vector packet
  classification** algorithms represent, for each of the N rules in a
  ruleset, a bitmap of "which packets (by header field range) this rule
  could match" per dimension (source IP range, dest port range, protocol),
  then AND the per-dimension bitmaps together (§3's `A & B & C & ...`) to
  find which rules match a given packet in the *union of dimensions* —
  this is the Bit Vector (BV) and Aggregated Bit Vector (ABV) family of
  classification algorithms, directly exploiting word-parallel AND.
- **`iptables`/`nftables` set matching** and **XDP/eBPF map-backed
  bitmaps** (`BPF_MAP_TYPE_ARRAY` or bespoke bit-packed maps) are used in
  high-performance userspace/XDP packet filters for the same reason:
  testing "is this /24 in my blocklist" against a precomputed bitmap over
  the /24 space is a single word test instead of a hash lookup, when the
  domain is small and dense enough (e.g., a bitmap over all 65536 possible
  destination ports for a port-scan/DDoS heuristic).

---

## 10. Userspace Systems Using Bitmaps

- **Databases** — bitmap indexes for low-cardinality columns (e.g.,
  `status IN ('active','inactive','pending')`) store one bitmap per
  distinct value, each bit representing "does row i have this value" — a
  query like `status='active' AND region='EU'` becomes a single bitmap AND
  across two pre-built bitmaps, no row-by-row scan required. Roaring
  bitmaps (§7.3b) are the modern default representation for this in
  systems like ClickHouse, Pilosa, and Apache Druid.
- **Search engines** — an inverted index's "postings list" (which documents
  contain term X) is frequently represented as a bitmap or Roaring bitmap
  over document IDs rather than a plain sorted-integer list, specifically
  so that boolean queries (`term1 AND term2 AND NOT term3`) become bitmap
  AND/OR/ANDNOT operations (§3) instead of a merge-join over sorted lists.
- **Graphics/game engines** — dirty-rectangle tracking, tile "visited"
  flags in pathfinding (A*/flood fill visited-set), collision layer masks
  (`layer_mask & other_layer_mask != 0` to decide if two objects can
  collide) are all direct 1-bit-per-entity bitmap applications.
- **Version control / build systems** — a build system's "which targets are
  dirty and need rebuilding" set, when the target universe is known and
  bounded per build graph traversal, is naturally a bitmap over target
  indices for fast reachability/coloring during a topological build.
- **Bloom filters in distributed systems** (§7.4) — LSM-tree storage
  engines' per-SSTable existence check, HTTP cache "have I seen this URL"
  probabilistic pre-check before a real (expensive) lookup, and
  distributed deduplication.

---

## 11. C Implementation

Kernel-style, portable, dependency-free bit array with word-skipping
find-bit operations and both non-atomic and `<stdatomic.h>`-based atomic
variants.

```c
/* bitmap.h — a small, kernel-flavored bitmap library */
#ifndef BITMAP_H
#define BITMAP_H

#include <stdint.h>
#include <stddef.h>
#include <string.h>
#include <stdatomic.h>

typedef unsigned long bm_word_t;
#define BM_BITS_PER_WORD   (sizeof(bm_word_t) * 8)

/* number of words needed to hold `nbits` bits, rounded up */
#define BM_WORDS(nbits) (((nbits) + BM_BITS_PER_WORD - 1) / BM_BITS_PER_WORD)

/* Declare a fixed-size bitmap, mirroring DECLARE_BITMAP() */
#define BITMAP_DECLARE(name, nbits) bm_word_t name[BM_WORDS(nbits)]

static inline void bm_zero(bm_word_t *map, size_t nbits) {
    memset(map, 0, BM_WORDS(nbits) * sizeof(bm_word_t));
}

static inline void bm_fill(bm_word_t *map, size_t nbits) {
    memset(map, 0xFF, BM_WORDS(nbits) * sizeof(bm_word_t));
}

static inline void bm_set(bm_word_t *map, size_t bit) {
    map[bit / BM_BITS_PER_WORD] |= ((bm_word_t)1 << (bit % BM_BITS_PER_WORD));
}

static inline void bm_clear(bm_word_t *map, size_t bit) {
    map[bit / BM_BITS_PER_WORD] &= ~((bm_word_t)1 << (bit % BM_BITS_PER_WORD));
}

static inline int bm_test(const bm_word_t *map, size_t bit) {
    return (map[bit / BM_BITS_PER_WORD] >> (bit % BM_BITS_PER_WORD)) & 1;
}

static inline void bm_toggle(bm_word_t *map, size_t bit) {
    map[bit / BM_BITS_PER_WORD] ^= ((bm_word_t)1 << (bit % BM_BITS_PER_WORD));
}

/* Atomic test-and-set: returns the PREVIOUS value of the bit. */
static inline int bm_test_and_set_atomic(_Atomic bm_word_t *map, size_t bit) {
    bm_word_t mask = (bm_word_t)1 << (bit % BM_BITS_PER_WORD);
    bm_word_t old = atomic_fetch_or_explicit(&map[bit / BM_BITS_PER_WORD],
                                              mask, memory_order_acq_rel);
    return (old & mask) != 0;
}

static inline int bm_test_and_clear_atomic(_Atomic bm_word_t *map, size_t bit) {
    bm_word_t mask = (bm_word_t)1 << (bit % BM_BITS_PER_WORD);
    bm_word_t old = atomic_fetch_and_explicit(&map[bit / BM_BITS_PER_WORD],
                                               ~mask, memory_order_acq_rel);
    return (old & mask) != 0;
}

/* popcount over the whole map */
static inline size_t bm_weight(const bm_word_t *map, size_t nbits) {
    size_t words = BM_WORDS(nbits);
    size_t count = 0;
    for (size_t w = 0; w < words; w++) {
#if defined(__GNUC__)
        count += (size_t)__builtin_popcountl(map[w]);
#else
        bm_word_t x = map[w];
        while (x) { x &= (x - 1); count++; }
#endif
    }
    return count;
}

/* find_next_zero_bit: word-skipping, hardware trailing-zero-count based */
static inline long bm_find_next_zero(const bm_word_t *map, size_t nbits,
                                      size_t start) {
    if (start >= nbits) return -1;
    size_t word_index = start / BM_BITS_PER_WORD;
    size_t bit_offset  = start % BM_BITS_PER_WORD;

    bm_word_t w = ~map[word_index];
    w &= ~((bm_word_t)0) << bit_offset;   /* mask off bits before `start` */

    size_t words = BM_WORDS(nbits);
    while (1) {
        if (w != 0) {
#if defined(__GNUC__)
            size_t bit = (size_t)__builtin_ctzl(w);
#else
            size_t bit = 0;
            bm_word_t t = w;
            while (!(t & 1)) { t >>= 1; bit++; }
#endif
            size_t result = word_index * BM_BITS_PER_WORD + bit;
            return (result < nbits) ? (long)result : -1;
        }
        word_index++;
        if (word_index >= words) return -1;
        w = ~map[word_index];
    }
}

static inline long bm_find_next_set(const bm_word_t *map, size_t nbits,
                                     size_t start) {
    if (start >= nbits) return -1;
    size_t word_index = start / BM_BITS_PER_WORD;
    size_t bit_offset  = start % BM_BITS_PER_WORD;

    bm_word_t w = map[word_index];
    w &= ~((bm_word_t)0) << bit_offset;

    size_t words = BM_WORDS(nbits);
    while (1) {
        if (w != 0) {
#if defined(__GNUC__)
            size_t bit = (size_t)__builtin_ctzl(w);
#else
            size_t bit = 0;
            bm_word_t t = w;
            while (!(t & 1)) { t >>= 1; bit++; }
#endif
            size_t result = word_index * BM_BITS_PER_WORD + bit;
            return (result < nbits) ? (long)result : -1;
        }
        word_index++;
        if (word_index >= words) return -1;
        w = map[word_index];
    }
}

/* iterate every set bit: mirrors the kernel's for_each_set_bit() */
#define bm_for_each_set(bit, map, nbits) \
    for (long (bit) = bm_find_next_set((map), (nbits), 0); \
         (bit) != -1; \
         (bit) = bm_find_next_set((map), (nbits), (size_t)(bit) + 1))

static inline void bm_and(bm_word_t *dst, const bm_word_t *a,
                          const bm_word_t *b, size_t nbits) {
    size_t words = BM_WORDS(nbits);
    for (size_t i = 0; i < words; i++) dst[i] = a[i] & b[i];
}

static inline void bm_or(bm_word_t *dst, const bm_word_t *a,
                         const bm_word_t *b, size_t nbits) {
    size_t words = BM_WORDS(nbits);
    for (size_t i = 0; i < words; i++) dst[i] = a[i] | b[i];
}

static inline void bm_andnot(bm_word_t *dst, const bm_word_t *a,
                             const bm_word_t *b, size_t nbits) {
    size_t words = BM_WORDS(nbits);
    for (size_t i = 0; i < words; i++) dst[i] = a[i] & ~b[i];
}

#endif /* BITMAP_H */
```

### Example: a tiny bitmap-backed page-frame allocator in C

```c
#include "bitmap.h"
#include <stdio.h>

#define NUM_PAGES 4096   /* toy: 4096 "physical pages" tracked */

BITMAP_DECLARE(page_bitmap, NUM_PAGES);  /* bit=1 means ALLOCATED */

/* returns allocated page index, or -1 if out of memory */
long alloc_page(void) {
    long free_bit = bm_find_next_zero(page_bitmap, NUM_PAGES, 0);
    if (free_bit < 0) return -1;
    bm_set(page_bitmap, (size_t)free_bit);
    return free_bit;
}

void free_page(size_t page_index) {
    bm_clear(page_bitmap, page_index);
}

int main(void) {
    bm_zero(page_bitmap, NUM_PAGES);

    long a = alloc_page();
    long b = alloc_page();
    printf("allocated pages: %ld, %ld\n", a, b);
    printf("pages in use: %zu\n", bm_weight(page_bitmap, NUM_PAGES));

    free_page((size_t)a);
    printf("pages in use after free: %zu\n", bm_weight(page_bitmap, NUM_PAGES));
    return 0;
}
```

---

## 12. Rust Implementation

Rust's ownership model and `#[repr(transparent)]`/const-generics make it
natural to build a bitmap that is both zero-cost and impossible to
index-out-of-bounds without an explicit panic (no silent memory corruption
the way a raw C pointer bug could cause).

```rust
// bitmap.rs — a fixed-capacity, word-packed bit array

pub struct Bitmap<const WORDS: usize> {
    words: [u64; WORDS],
}

const BITS_PER_WORD: usize = 64;

impl<const WORDS: usize> Bitmap<WORDS> {
    pub const CAPACITY: usize = WORDS * BITS_PER_WORD;

    pub const fn new() -> Self {
        Bitmap { words: [0u64; WORDS] }
    }

    #[inline]
    fn split(bit: usize) -> (usize, usize) {
        (bit / BITS_PER_WORD, bit % BITS_PER_WORD)
    }

    pub fn set(&mut self, bit: usize) {
        let (w, b) = Self::split(bit);
        self.words[w] |= 1u64 << b;
    }

    pub fn clear(&mut self, bit: usize) {
        let (w, b) = Self::split(bit);
        self.words[w] &= !(1u64 << b);
    }

    pub fn test(&self, bit: usize) -> bool {
        let (w, b) = Self::split(bit);
        (self.words[w] >> b) & 1 == 1
    }

    pub fn toggle(&mut self, bit: usize) {
        let (w, b) = Self::split(bit);
        self.words[w] ^= 1u64 << b;
    }

    /// Hamming weight — total number of set bits.
    pub fn weight(&self) -> u32 {
        self.words.iter().map(|w| w.count_ones()).sum()
    }

    pub fn is_empty(&self) -> bool {
        self.words.iter().all(|&w| w == 0)
    }

    pub fn is_full(&self) -> bool {
        self.words.iter().all(|&w| w == u64::MAX)
    }

    /// Word-skipping find-first-zero-bit, using trailing_zeros() (TZCNT/BSF).
    pub fn find_first_zero(&self) -> Option<usize> {
        for (i, &w) in self.words.iter().enumerate() {
            if w != u64::MAX {
                let bit = (!w).trailing_zeros() as usize;
                let idx = i * BITS_PER_WORD + bit;
                if idx < Self::CAPACITY {
                    return Some(idx);
                }
            }
        }
        None
    }

    pub fn find_first_set(&self) -> Option<usize> {
        for (i, &w) in self.words.iter().enumerate() {
            if w != 0 {
                let bit = w.trailing_zeros() as usize;
                return Some(i * BITS_PER_WORD + bit);
            }
        }
        None
    }

    /// Iterator over all set bit indices — mirrors kernel's for_each_set_bit.
    pub fn iter_set_bits(&self) -> SetBitsIter<'_, WORDS> {
        SetBitsIter { map: self, word_idx: 0, cur_word: self.words.first().copied().unwrap_or(0) }
    }

    pub fn and_with(&mut self, other: &Bitmap<WORDS>) {
        for i in 0..WORDS { self.words[i] &= other.words[i]; }
    }

    pub fn or_with(&mut self, other: &Bitmap<WORDS>) {
        for i in 0..WORDS { self.words[i] |= other.words[i]; }
    }

    pub fn andnot_with(&mut self, other: &Bitmap<WORDS>) {
        for i in 0..WORDS { self.words[i] &= !other.words[i]; }
    }
}

pub struct SetBitsIter<'a, const WORDS: usize> {
    map: &'a Bitmap<WORDS>,
    word_idx: usize,
    cur_word: u64,
}

impl<'a, const WORDS: usize> Iterator for SetBitsIter<'a, WORDS> {
    type Item = usize;
    fn next(&mut self) -> Option<usize> {
        while self.cur_word == 0 {
            self.word_idx += 1;
            if self.word_idx >= WORDS {
                return None;
            }
            self.cur_word = self.map.words[self.word_idx];
        }
        let bit = self.cur_word.trailing_zeros() as usize;
        self.cur_word &= self.cur_word - 1; // clear lowest set bit (§3)
        Some(self.word_idx * BITS_PER_WORD + bit)
    }
}
```

### Lock-free atomic variant (concurrent allocator claim, §6.1)

```rust
use std::sync::atomic::{AtomicU64, Ordering};

pub struct AtomicBitmap<const WORDS: usize> {
    words: [AtomicU64; WORDS],
}

impl<const WORDS: usize> AtomicBitmap<WORDS> {
    /// Atomically claims a bit; returns true if THIS call won the race.
    pub fn test_and_set(&self, bit: usize) -> bool {
        let w = bit / 64;
        let b = bit % 64;
        let mask = 1u64 << b;
        let old = self.words[w].fetch_or(mask, Ordering::AcqRel);
        (old & mask) == 0   // true means this call set a previously-clear bit
    }

    pub fn test_and_clear(&self, bit: usize) -> bool {
        let w = bit / 64;
        let b = bit % 64;
        let mask = 1u64 << b;
        let old = self.words[w].fetch_and(!mask, Ordering::AcqRel);
        (old & mask) != 0
    }
}

/// Example: racing "allocate a free ID" across threads — mirrors kernel IDA.
pub fn claim_first_free<const WORDS: usize>(map: &AtomicBitmap<WORDS>) -> Option<usize> {
    for word_idx in 0..WORDS {
        loop {
            let cur = map.words[word_idx].load(Ordering::Acquire);
            if cur == u64::MAX { break; }        // word full, move to next
            let free_bit = (!cur).trailing_zeros();
            let mask = 1u64 << free_bit;
            // CAS loop: only succeed if no one else claimed it first.
            if map.words[word_idx]
                .compare_exchange(cur, cur | mask, Ordering::AcqRel, Ordering::Acquire)
                .is_ok()
            {
                return Some(word_idx * 64 + free_bit as usize);
            }
            // else: retry — another thread raced us; `cur` is stale, reload
        }
    }
    None
}
```

---

## 13. Go Implementation

Go's `math/bits` package exposes exactly the same hardware-backed
`TrailingZeros64`/`OnesCount64` intrinsics used in the C and Rust versions
above, compiling down to the same `TZCNT`/`BSF`/`POPCNT` instructions on
amd64.

```go
package bitmap

import (
    "math/bits"
    "sync/atomic"
)

const wordBits = 64

// Bitmap is a fixed-domain, word-packed bit array.
type Bitmap struct {
    words []uint64
    nbits int
}

func New(nbits int) *Bitmap {
    n := (nbits + wordBits - 1) / wordBits
    return &Bitmap{words: make([]uint64, n), nbits: nbits}
}

func (b *Bitmap) split(bit int) (int, uint) {
    return bit / wordBits, uint(bit % wordBits)
}

func (b *Bitmap) Set(bit int) {
    w, off := b.split(bit)
    b.words[w] |= 1 << off
}

func (b *Bitmap) Clear(bit int) {
    w, off := b.split(bit)
    b.words[w] &^= 1 << off // Go's AND-NOT operator: x &^ y == x & (^y)
}

func (b *Bitmap) Test(bit int) bool {
    w, off := b.split(bit)
    return (b.words[w]>>off)&1 == 1
}

func (b *Bitmap) Toggle(bit int) {
    w, off := b.split(bit)
    b.words[w] ^= 1 << off
}

// Weight returns the Hamming weight (popcount) of the whole bitmap.
func (b *Bitmap) Weight() int {
    total := 0
    for _, w := range b.words {
        total += bits.OnesCount64(w)
    }
    return total
}

// FindFirstZero does a word-skipping scan using hardware trailing-zero-count.
func (b *Bitmap) FindFirstZero() (int, bool) {
    for i, w := range b.words {
        if w != ^uint64(0) { // word is not all-ones, so it has a free bit
            bit := i*wordBits + bits.TrailingZeros64(^w)
            if bit < b.nbits {
                return bit, true
            }
        }
    }
    return 0, false
}

// FindNextSet mirrors the kernel's find_next_bit() for iteration.
func (b *Bitmap) FindNextSet(start int) (int, bool) {
    if start >= b.nbits {
        return 0, false
    }
    wordIdx, bitOff := b.split(start)
    w := b.words[wordIdx] &^ ((uint64(1) << bitOff) - 1) // mask off bits before start
    for {
        if w != 0 {
            bit := wordIdx*wordBits + bits.TrailingZeros64(w)
            if bit < b.nbits {
                return bit, true
            }
            return 0, false
        }
        wordIdx++
        if wordIdx >= len(b.words) {
            return 0, false
        }
        w = b.words[wordIdx]
    }
}

// ForEachSet iterates every set bit — mirrors for_each_set_bit() in C.
func (b *Bitmap) ForEachSet(fn func(bit int)) {
    bit, ok := b.FindNextSet(0)
    for ok {
        fn(bit)
        bit, ok = b.FindNextSet(bit + 1)
    }
}

func (b *Bitmap) And(other *Bitmap) {
    for i := range b.words {
        b.words[i] &= other.words[i]
    }
}

func (b *Bitmap) Or(other *Bitmap) {
    for i := range b.words {
        b.words[i] |= other.words[i]
    }
}

// AtomicBitmap is a concurrency-safe bitmap for racing ID/resource claims.
type AtomicBitmap struct {
    words []uint64 // accessed exclusively through atomic ops below
}

func NewAtomic(nbits int) *AtomicBitmap {
    n := (nbits + wordBits - 1) / wordBits
    return &AtomicBitmap{words: make([]uint64, n)}
}

// TestAndSet atomically claims a bit; returns true if this call won the race.
func (b *AtomicBitmap) TestAndSet(bit int) bool {
    w, off := bit/wordBits, uint(bit%wordBits)
    mask := uint64(1) << off
    for {
        old := atomic.LoadUint64(&b.words[w])
        if old&mask != 0 {
            return false // already claimed by someone else
        }
        if atomic.CompareAndSwapUint64(&b.words[w], old, old|mask) {
            return true // we won the race
        }
        // CAS failed: someone else changed the word concurrently — retry
    }
}

// ClaimFirstFree finds and atomically claims the lowest free bit —
// the Go analogue of Linux's IDA "allocate lowest free id" pattern.
func (b *AtomicBitmap) ClaimFirstFree() (int, bool) {
    for w := range b.words {
        for {
            cur := atomic.LoadUint64(&b.words[w])
            if cur == ^uint64(0) {
                break // word full, try next word
            }
            freeBit := bits.TrailingZeros64(^cur)
            mask := uint64(1) << freeBit
            if atomic.CompareAndSwapUint64(&b.words[w], cur, cur|mask) {
                return w*wordBits + freeBit, true
            }
            // lost the race for this word — reload and retry
        }
    }
    return 0, false
}
```

---

## 14. Complexity, Performance, and When Not to Use a Bitmap

| Operation | Naive bit-by-bit | Word-skipping bitmap |
|---|---|---|
| `set`/`clear`/`test`/`toggle` one bit | O(1) | O(1) — identical, single instruction |
| `find_first_set`/`find_first_zero` | O(N) bits | O(N/W) words, W=32 or 64 → effectively O(N/64) |
| `popcount`/weight | O(N) | O(N/W) words with `POPCNT` |
| set union/intersection/difference | O(N) bit-pairs | O(N/W) word-pairs |
| membership test in a Bloom filter | — | O(k) hash evaluations, independent of N |

**When a flat bitmap is the right tool:**
- The domain is a bounded, dense, small-to-medium set of discrete integer
  IDs (page frames within a zone, CPU numbers, inode/block numbers within a
  filesystem block group, signal numbers, capability bits).
- You need fast set-algebra (AND/OR/XOR) across many elements at once.
- You need atomic, lock-free "claim one of N slots" semantics.

**When it is the wrong tool:**
- The domain is enormous and sparsely populated (a bitmap over all possible
  IPv6 addresses to track "seen" addresses is astronomically wasteful) —
  use a hash set, a Bloom filter (§7.4), or a Roaring/compressed bitmap
  (§7.3) instead.
- You need to store more than 1 bit of information per element — use a
  packed array of small integers, or a proper array/struct.
- The domain size is unknown and unbounded, and growth would require
  frequent, expensive reallocation and rehashing of every index (e.g., an
  ever-growing set of arbitrary 64-bit hash values) — a hash set is the
  natural fit there, not a bitmap.

---

## 15. Mental Model Summary

Carry exactly these ideas forward, and every bitmap you meet in kernel code,
a database engine, or a distributed system will click into place immediately:

1. **A bitmap is "one bit of state, per element, over a fixed indexable
   domain" — packed at word granularity so the CPU can operate on 32 or 64
   elements per instruction.**
2. **Every operation reduces to `word_index = i / W`, `bit_offset = i % W`,
   then one AND/OR/XOR/shift on `storage[word_index]`.** There is no deeper
   trick beneath this.
3. **Real implementations never scan bit-by-bit.** They scan word-by-word
   and use a hardware trailing/leading-zero-count instruction to jump
   straight to the answer within the interesting word. This single
   optimization is what separates a "toy" bitmap from a kernel-grade one.
4. **Concurrent bitmaps need atomic read-modify-write instructions**
   (`test_and_set_bit`, `LOCK BTS`, `fetch_or`, `CompareAndSwap`) — plain
   `|=` on a shared word is a data race.
5. **Set algebra on bitmaps is set algebra on the represented sets** — AND
   is intersection, OR is union, ANDNOT is difference. This is exactly how
   the kernel computes CPU affinity intersections and how packet
   classifiers and bitmap-indexed databases answer boolean queries in one
   pass over words instead of rows.
6. **Dense, bounded, small-ID domains → flat bitmap. Sparse or huge domains
   → Roaring/RLE-compressed bitmap or a Bloom filter.** Picking the wrong
   one is the single most common real-world bitmap mistake (memory blowup
   from a flat bitmap over a huge sparse domain, or unnecessary complexity
   from reaching for Roaring/Bloom when a domain is small and dense).
7. **Cache-line locality dominates performance, not instruction count.**
   Sequential scans are nearly free; random access across a huge bitmap
   thrashes cache; naively sharing one bitmap word across independently-hot
   flags causes false sharing between cores.

Once these seven points are internalized as reflexes — not facts to recall,
but the first thing you reach for when you see "N discrete things, one bit
of state each" — you will recognize the pattern instantly in page
allocators, filesystem allocators, scheduler masks, permission checks,
firewall rules, and database indexes, and you will know, without having to
re-derive it, exactly which variant (flat, atomic, Roaring, Bloom) the
situation calls for.
