# The Complete Guide to Bit Masking, Flags, Fast Arithmetic & Bitwise Shift Operators

> A deep-dive into bit-level thinking — from hardware registers to Linux kernel internals, network subsystems, and systems programming in C, Rust, and Go.

---

## Table of Contents

1. [Mental Model: How Computers Actually Think](#1-mental-model-how-computers-actually-think)
2. [Binary Representation Deep Dive](#2-binary-representation-deep-dive)
3. [Bitwise Operators — The Full Taxonomy](#3-bitwise-operators--the-full-taxonomy)
4. [Bit Masking — Theory and Mechanics](#4-bit-masking--theory-and-mechanics)
5. [Flags — Packing State into Bits](#5-flags--packing-state-into-bits)
6. [Bitwise Shift Operators](#6-bitwise-shift-operators)
7. [Fast Arithmetic with Bit Operations](#7-fast-arithmetic-with-bit-operations)
8. [Bit Tricks and Hacks](#8-bit-tricks-and-hacks)
9. [Two's Complement and Signed Arithmetic](#9-twos-complement-and-signed-arithmetic)
10. [Linux Kernel Bit Patterns](#10-linux-kernel-bit-patterns)
11. [Network Subsystem Bit Operations](#11-network-subsystem-bit-operations)
12. [Hardware Registers and Memory-Mapped I/O](#12-hardware-registers-and-memory-mapped-io)
13. [C Implementations](#13-c-implementations)
14. [Rust Implementations](#14-rust-implementations)
15. [Go Implementations](#15-go-implementations)
16. [Advanced Patterns: Bitfields, Packed Structs, and Portability](#16-advanced-patterns-bitfields-packed-structs-and-portability)
17. [Performance Analysis and CPU-Level Mechanics](#17-performance-analysis-and-cpu-level-mechanics)
18. [Real-World Case Studies](#18-real-world-case-studies)

---

## 1. Mental Model: How Computers Actually Think

The single most important mental shift in systems programming is understanding that **a CPU does not know what a byte "means."** It only knows how to perform binary operations on bit patterns. Meaning is imposed by the programmer.

When you write `int x = 65`, the CPU stores:

```
Memory cell:
+--------+--------+--------+--------+
|00000000|00000000|00000000|01000001|
+--------+--------+--------+--------+
 byte 3    byte 2   byte 1   byte 0
         (on a little-endian 32-bit system)
```

That same bit pattern `01000001` is also the ASCII character `'A'` (65), or the float approximation of a tiny number, or a hardware register flag. The **type** in your language tells the compiler how to interpret those bits — but at runtime they're all the same electricity.

### The Register File Mental Model

```
CPU Architecture (x86-64 simplified):

 ┌─────────────────────────────────────────────────────┐
 │                    CPU Die                          │
 │                                                     │
 │  Register File (ultra-fast, on-die storage)         │
 │  ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐               │
 │  │ RAX  │ │ RBX  │ │ RCX  │ │ RDX  │  64-bit GPRs  │
 │  │64bit │ │64bit │ │64bit │ │64bit │               │
 │  └──┬───┘ └──────┘ └──────┘ └──────┘               │
 │     │                                               │
 │     ├── EAX (low 32 bits)                           │
 │     │    ├── AX (low 16 bits)                       │
 │     │    │    ├── AH (bits 15:8)                    │
 │     │    │    └── AL (bits 7:0)                     │
 │     │                                               │
 │  ┌──────────────────────────────────────────────┐   │
 │  │  RFLAGS Register (status bits)               │   │
 │  │  bit: 63...12  11  10   9   8   7   6   5   4   3   2   1   0  │
 │  │  flag:          OF  DF  IF  TF  SF  ZF  -   AF  -   PF  -   CF │
 │  │       Overflow  Dir Intr Trap Sign Zero    Aux     Parity Carry │
 │  └──────────────────────────────────────────────┘   │
 │                                                     │
 │  ALU ──── performs bitwise AND, OR, XOR, NOT, SHL, SHR, SAR     │
 └─────────────────────────────────────────────────────┘
```

Every flag in RFLAGS is a single bit. The OS, runtime, and your code read these bits after every arithmetic operation. This is not metaphor — this is how branching (`if`, loops, comparisons) physically works.

### Why Bit Operations Matter

- **Speed**: A single `AND` instruction runs in 1 CPU cycle (single cycle throughput on modern CPUs).
- **Density**: You can pack 64 boolean values into a single `uint64_t` — no struct overhead, no pointer chasing.
- **Protocol conformance**: Every network packet, filesystem inode, and hardware register uses bit fields.
- **Cache efficiency**: Bit-packed data means fewer cache lines fetched.
- **Kernel requirement**: The Linux kernel itself is full of bitmask operations in hot paths — process states, page flags, IRQ masks, socket states.

---

## 2. Binary Representation Deep Dive

### Number Systems Side by Side

```
Decimal  Binary      Hex    Octal
0        0000 0000   0x00   0o000
1        0000 0001   0x01   0o001
2        0000 0010   0x02   0o002
3        0000 0011   0x03   0o003
4        0000 0100   0x04   0o004
7        0000 0111   0x07   0o007
8        0000 1000   0x08   0o010
15       0000 1111   0x0F   0o017
16       0001 0000   0x10   0o020
127      0111 1111   0x7F   0o177
128      1000 0000   0x80   0o200
255      1111 1111   0xFF   0o377
```

### Bit Position Notation (0-indexed from LSB)

```
Byte layout (8 bits):

Position:  7    6    5    4    3    2    1    0
           │    │    │    │    │    │    │    │
Value:     128  64   32   16   8    4    2    1
           2^7  2^6  2^5  2^4  2^3  2^2  2^1  2^0

Example: 0b10110101 = 128 + 32 + 16 + 4 + 1 = 181 = 0xB5

Bit:   7   6   5   4   3   2   1   0
       ┌───┬───┬───┬───┬───┬───┬───┬───┐
       │ 1 │ 0 │ 1 │ 1 │ 0 │ 1 │ 0 │ 1 │
       └───┴───┴───┴───┴───┴───┴───┴───┘
         ↑                           ↑
        MSB                         LSB
  (Most Significant Bit)   (Least Significant Bit)
```

### 32-bit Word Layout

```
 31                16 15                 0
 ┌──────────────────┬──────────────────┐
 │   High Word (HW) │   Low Word (LW)  │
 │   bits 31..16    │   bits 15..0     │
 └──────────────────┴──────────────────┘

 31      24 23      16 15      8  7       0
 ┌─────────┬─────────┬─────────┬─────────┐
 │  Byte 3 │  Byte 2 │  Byte 1 │  Byte 0 │
 └─────────┴─────────┴─────────┴─────────┘
```

### Endianness — Critical for Network Code

```
Value: 0x0A0B0C0D (decimal: 168496141)

Little-Endian (x86, x86-64, ARM default):
Memory address: 0x1000  0x1001  0x1002  0x1003
                ┌──────┬──────┬──────┬──────┐
                │ 0x0D │ 0x0C │ 0x0B │ 0x0A │
                └──────┴──────┴──────┴──────┘
                  LSB                    MSB
(Least significant byte at lowest address)

Big-Endian (Network byte order, SPARC, old MIPS):
Memory address: 0x1000  0x1001  0x1002  0x1003
                ┌──────┬──────┬──────┬──────┐
                │ 0x0A │ 0x0B │ 0x0C │ 0x0D │
                └──────┴──────┴──────┴──────┘
                  MSB                    LSB
(Most significant byte at lowest address)
```

This is why `htons()`, `htonl()`, `ntohs()`, `ntohl()` exist — to convert between host and network byte order. The Linux kernel uses these everywhere in the networking stack.

---

## 3. Bitwise Operators — The Full Taxonomy

### 3.1 AND ( `&` )

AND outputs `1` only when **both** inputs are `1`. Everything else is `0`.

```
Truth table:
  A  B  A & B
  0  0    0
  0  1    0
  1  0    0
  1  1    1

Example: 0b11001010 & 0b00001111
  1 1 0 0 1 0 1 0    (0xCA = 202)
& 0 0 0 0 1 1 1 1    (0x0F = 15)  ← mask
─────────────────
  0 0 0 0 1 0 1 0    (0x0A = 10)
```

**Primary use**: Masking — zeroing out bits you don't care about, extracting specific bits.

### 3.2 OR ( `|` )

OR outputs `1` when **at least one** input is `1`.

```
Truth table:
  A  B  A | B
  0  0    0
  0  1    1
  1  0    1
  1  1    1

Example: 0b11001010 | 0b00110101
  1 1 0 0 1 0 1 0
| 0 0 1 1 0 1 0 1
─────────────────
  1 1 1 1 1 1 1 1    (0xFF = 255)
```

**Primary use**: Setting bits — turning specific bits ON without affecting others.

### 3.3 XOR ( `^` )

XOR (exclusive OR) outputs `1` only when inputs **differ**.

```
Truth table:
  A  B  A ^ B
  0  0    0
  0  1    1
  1  0    1
  1  1    0       ← this is what makes it "exclusive"

Example: 0b11001010 ^ 0b00001111
  1 1 0 0 1 0 1 0
^ 0 0 0 0 1 1 1 1
─────────────────
  1 1 0 0 0 1 0 1    (0xC5 = 197)

Key property: A ^ A = 0 (any value XORed with itself is zero)
             A ^ 0 = A (XOR with zero is identity)
             A ^ B ^ B = A (XOR is its own inverse)
```

**Primary uses**: Toggling bits, swapping without a temp variable, checksums, simple encryption (XOR cipher), detecting differences.

### 3.4 NOT ( `~` ) — Bitwise Complement

NOT flips every single bit.

```
Example (8-bit): ~0b11001010
  1 1 0 0 1 0 1 0    (0xCA = 202)
─────────────────
  0 0 1 1 0 1 0 1    (0x35 = 53)

General rule: ~x = -(x + 1)   [for two's complement]
              ~0 = -1
              ~1 = -2
              ~255 = -256

Warning: ~x on an unsigned type gives the bitwise complement.
         On signed types, the result is implementation-defined in C
         (but well-defined in practice on two's complement hardware,
          which is now mandated by C23).
```

**Primary use**: Creating inverse masks — `~mask` to clear bits.

### 3.5 Left Shift ( `<<` )

Shift all bits toward higher positions. Vacated low bits fill with `0`.

```
x = 0b00000011 (3)
x << 1:
Before: 0 0 0 0 0 0 1 1
         ↓ ↓ ↓ ↓ ↓ ↓ ↓ ↓  (all shift left by 1)
After:  0 0 0 0 0 1 1 0   = 6

x << 2:
Before: 0 0 0 0 0 0 1 1
After:  0 0 0 0 1 1 0 0   = 12

Overflow (bits fall off the left edge):
x = 0b10000001 (129)
x << 1:
Before: 1 0 0 0 0 0 0 1
After:  0 0 0 0 0 0 1 0   = 2  (the leading 1 was lost)
```

### 3.6 Right Shift ( `>>` )

Shift all bits toward lower positions. Behavior on the vacated high bits depends on type.

```
Logical Right Shift (unsigned types — fills with 0):
x = 0b10001000 (136 unsigned)
x >> 1:
Before: 1 0 0 0 1 0 0 0
After:  0 1 0 0 0 1 0 0   = 68

Arithmetic Right Shift (signed types — fills with sign bit):
x = 0b10001000 (-120 signed int8)
x >> 1:
Before: 1 0 0 0 1 0 0 0
After:  1 1 0 0 0 1 0 0   = -60 (sign bit extended)

This is how signed right shift implements floor division by powers of 2.
```

---

## 4. Bit Masking — Theory and Mechanics

A **bitmask** is a value used with AND, OR, or XOR to selectively manipulate specific bits in a target value. The mask itself is just a number — the *art* is choosing which bits to set in the mask.

### 4.1 The Three Fundamental Mask Operations

```
Given: value = 0b10110110
       mask  = 0b00001111  (low nibble mask)

1. READ / TEST (AND):  Does value have any of these bits set?
   value & mask = 0b00000110   ← only the masked bits survive

2. SET (OR):  Force these bits to 1:
   value | mask = 0b10111111   ← masked bits now all 1

3. CLEAR (AND with inverted mask):  Force these bits to 0:
   value & ~mask = 0b10110000  ← masked bits now all 0

4. TOGGLE (XOR):  Flip these bits:
   value ^ mask = 0b10111001   ← masked bits are toggled
```

### 4.2 Creating Masks

```
Common mask construction patterns:

Single bit mask for bit N:
  mask = 1 << N

  bit 0:  1 << 0 = 0b00000001 = 0x01
  bit 3:  1 << 3 = 0b00001000 = 0x08
  bit 7:  1 << 7 = 0b10000000 = 0x80

Range mask for bits [HIGH:LOW] (inclusive):
  Method 1: ((1 << (HIGH - LOW + 1)) - 1) << LOW

  Example: bits [5:2] → 4 bits wide starting at bit 2
  = ((1 << 4) - 1) << 2
  = (16 - 1) << 2
  = 15 << 2
  = 0b00111100

  Verification:
  bit: 7 6 5 4 3 2 1 0
       0 0 1 1 1 1 0 0   ← bits 5,4,3,2 are set ✓

  Method 2 (using GENMASK macro from Linux kernel):
  #define GENMASK(h, l) (((~0UL) >> (BITS_PER_LONG - 1 - (h))) & (~0UL << (l)))
  GENMASK(5, 2) = 0b00111100  ✓
```

### 4.3 Extracting a Bit Field

To read a multi-bit field from a register or packet header:

```
Example: Extract bits [5:2] from a byte

value = 0b10110110

Step 1: AND with mask to zero other bits
  0b10110110
& 0b00111100   (mask = GENMASK(5,2))
─────────────
  0b00110100

Step 2: Right-shift down to bit 0
  0b00110100 >> 2
= 0b00001101
= 13

So the 4-bit field at [5:2] has value 13.

One-liner in C:
  uint8_t field = (value & 0x3C) >> 2;

General formula:
  field = (value >> LOW) & ((1 << WIDTH) - 1);
  where WIDTH = HIGH - LOW + 1
```

### 4.4 Inserting a Bit Field

To write a value into specific bits of a register:

```
Step 1: Clear the target bits (AND with inverse mask)
Step 2: OR in the new value, shifted to position

value    = 0b10110110   (original)
new_bits = 0b0101       (value 5, to insert at bits [5:2])
mask     = 0b00111100   (GENMASK(5,2))

Step 1: value & ~mask
  0b10110110 & 0b11000011 = 0b10000010

Step 2: | (new_bits << 2)
  0b10000010 | (0b0101 << 2)
= 0b10000010 | 0b00010100
= 0b10010110

Result: bits [5:2] now hold 0101 (5), other bits unchanged.

C macro:
  #define FIELD_PREP(mask, val)  (((val) << __ffs(mask)) & (mask))
  #define FIELD_GET(mask, reg)   (((reg) & (mask)) >> __ffs(mask))
  (These are actual Linux kernel macros from <linux/bitfield.h>)
```

### 4.5 Testing Bits

```
Test if a single bit is set:
  if (value & (1 << N)) { /* bit N is set */ }

Test if ALL bits in mask are set:
  if ((value & mask) == mask) { /* all mask bits set */ }

Test if ANY bit in mask is set:
  if (value & mask) { /* at least one set */ }

Test if NO bits in mask are set:
  if (!(value & mask)) { /* none set */ }

Test if value has exactly one bit set (is a power of 2):
  if (value && !(value & (value - 1))) { /* power of 2 */ }

Check if a bit is zero:
  if (!(value & (1 << N))) { /* bit N is zero */ }
```

---

## 5. Flags — Packing State into Bits

Flags are a design pattern where each bit in an integer represents an independent boolean state. This is the most common use of bitmasks in real systems code.

### 5.1 Why Flags Beat Struct Fields for State

```
Naive approach (7 booleans in a struct):
struct ProcessFlags {
    bool is_running;      // 1 byte minimum (or 4 with padding)
    bool is_zombie;       // 1 byte
    bool is_stopped;      // 1 byte
    bool has_children;    // 1 byte
    bool is_traced;       // 1 byte
    bool in_signal;       // 1 byte
    bool has_pending;     // 1 byte
    // Total: 7 bytes minimum, likely 8-16 with alignment
};

Bitflag approach:
uint8_t process_flags;   // 1 byte — fits all 7 flags + 1 spare
// Atomic test-and-set possible with CPU instructions
// Can pass all flags in a single register
// Can test multiple flags with a single AND
```

### 5.2 Defining and Using Flags

```c
/* ── Flag Definitions ── */
/* Convention: use powers of 2, or 1 << N */
#define FLAG_NONE         0x00   /* 0000 0000 */
#define FLAG_READ         0x01   /* 0000 0001  (bit 0) */
#define FLAG_WRITE        0x02   /* 0000 0010  (bit 1) */
#define FLAG_EXEC         0x04   /* 0000 0100  (bit 2) */
#define FLAG_HIDDEN       0x08   /* 0000 1000  (bit 3) */
#define FLAG_SYSTEM       0x10   /* 0001 0000  (bit 4) */
#define FLAG_ARCHIVE      0x20   /* 0010 0000  (bit 5) */
#define FLAG_DIRECTORY    0x40   /* 0100 0000  (bit 6) */
#define FLAG_VOLATILE     0x80   /* 1000 0000  (bit 7) */

/* ── Operations ── */
uint8_t perms = FLAG_NONE;

/* Set a flag */
perms |= FLAG_READ;
perms |= FLAG_WRITE;
/* perms = 0b00000011 */

/* Set multiple flags at once */
perms |= (FLAG_READ | FLAG_EXEC);

/* Clear a flag */
perms &= ~FLAG_WRITE;

/* Toggle a flag */
perms ^= FLAG_EXEC;

/* Test a flag */
if (perms & FLAG_READ)   { /* readable */ }
if (!(perms & FLAG_EXEC)) { /* not executable */ }

/* Test multiple flags — ALL must be set */
if ((perms & (FLAG_READ | FLAG_WRITE)) == (FLAG_READ | FLAG_WRITE)) {
    /* both read and write are set */
}

/* Test multiple flags — ANY must be set */
if (perms & (FLAG_READ | FLAG_WRITE | FLAG_EXEC)) {
    /* at least one permission is set */
}

/* Replace all flags */
perms = FLAG_READ | FLAG_EXEC;

/* Clear all flags */
perms = 0;

/* Conditional set/clear */
/* Set FLAG_READ if condition is true, clear if false */
perms = (perms & ~FLAG_READ) | (condition ? FLAG_READ : 0);
```

### 5.3 Linux Process State Flags (Real Kernel Code)

The Linux kernel uses bitflags extensively for task states. Here's how `include/linux/sched.h` actually works conceptually:

```c
/*
 * Task state bitmask — from Linux kernel include/linux/sched.h
 * These are bit positions used with set_task_state(), etc.
 */
#define TASK_RUNNING            0x00000000
#define TASK_INTERRUPTIBLE      0x00000001  /* sleeping, wakes on signal */
#define TASK_UNINTERRUPTIBLE    0x00000002  /* sleeping, ignores signals (D state) */
#define __TASK_STOPPED          0x00000004  /* stopped by signal */
#define __TASK_TRACED           0x00000008  /* being traced by ptrace */
#define TASK_PARKED             0x00000040
#define TASK_DEAD               0x00000080
#define TASK_WAKEKILL           0x00000100  /* wake on fatal signals */
#define TASK_WAKING             0x00000200
#define TASK_NOLOAD             0x00000400
#define TASK_NEW                0x00000800
#define TASK_RTLOCK_WAIT        0x00001000

/* Composite states (OR of primitives) */
#define TASK_KILLABLE           (TASK_WAKEKILL | TASK_UNINTERRUPTIBLE)
#define TASK_STOPPED            (TASK_WAKEKILL | __TASK_STOPPED)
#define TASK_TRACED             (TASK_WAKEKILL | __TASK_TRACED)
#define TASK_IDLE               (TASK_UNINTERRUPTIBLE | TASK_NOLOAD)

/*
 * Bit layout visualization:
 * Bit: 12   11   10    9    8    7    6    5    4    3    2    1    0
 *      │    │    │    │    │    │    │    │    │    │    │    │    │
 *      RTLK NEW  NOLD WAKN WKKL DEAD PARK  -    -   TRCD STOP UINT INT
 */
```

### 5.4 Linux Page Flags (Memory Subsystem)

```c
/*
 * Page flags — from include/linux/page-flags.h
 * Each struct page has flags field (unsigned long)
 *
 * These describe the state of a physical memory page.
 */
enum pageflags {
    PG_locked,          /* bit 0:  page is locked */
    PG_referenced,      /* bit 1:  page was recently accessed */
    PG_uptodate,        /* bit 2:  page data is valid */
    PG_dirty,           /* bit 3:  page has been modified */
    PG_lru,             /* bit 4:  page is on LRU list */
    PG_active,          /* bit 5:  page is on active LRU */
    PG_workingset,      /* bit 6:  part of working set */
    PG_waiters,         /* bit 7:  threads waiting on this page */
    PG_error,           /* bit 8:  I/O error on this page */
    PG_slab,            /* bit 9:  allocated by slab allocator */
    PG_owner_priv_1,    /* bit 10: owner-specific flag */
    PG_arch_1,          /* bit 11: architecture-specific */
    PG_reserved,        /* bit 12: page reserved by hardware/firmware */
    PG_private,         /* bit 13: has private data (page->private) */
    PG_private_2,       /* bit 14: private data 2 */
    PG_writeback,       /* bit 15: page is under writeback */
    PG_head,            /* bit 16: head of compound page */
    PG_mappedtodisk,    /* bit 17: page has on-disk data */
    PG_reclaim,         /* bit 18: page about to be reclaimed */
    PG_swapbacked,      /* bit 19: backed by swap or RAM */
    PG_unevictable,     /* bit 20: page cannot be evicted */
    PG_mlocked,         /* bit 21: page is mlocked */
    /* ... more flags follow ... */
};

/* The kernel provides macros for each flag: */
/* PageLocked(page)      → tests PG_locked   */
/* SetPageLocked(page)   → sets  PG_locked   */
/* ClearPageLocked(page) → clears PG_locked  */
/* TestSetPageLocked(page) → atomic test-and-set */
```

---

## 6. Bitwise Shift Operators

### 6.1 Left Shift — `<<`

Left shift multiplies by powers of 2. Each position shifted left doubles the value.

```
x << n  =  x * 2^n   (when no overflow)

Bit animation for 5 << 3:
5 = 0b00000101

After << 1:  0b00001010  = 10
After << 2:  0b00010100  = 20
After << 3:  0b00101000  = 40

General:     5 * 2^3 = 5 * 8 = 40 ✓

Real use cases:
  1 << 10 = 1024 = 1 KB
  1 << 20 = 1048576 = 1 MB
  1 << 30 = 1073741824 = 1 GB
```

**Undefined behavior in C (important!):**
- Shifting a negative value: `(-1) << 1` — undefined behavior in C (until C23)
- Shifting by more than bit width: `1 << 32` on a 32-bit int — undefined behavior
- Shifting into the sign bit of a signed type: `1 << 31` on int32 — undefined behavior

**Safe pattern in C:**
```c
/* Always use unsigned types for shifting */
uint32_t mask = (uint32_t)1 << 31;   /* safe */
/* Or use UINT32_C macro */
uint32_t mask2 = UINT32_C(1) << 31;  /* safe */
```

### 6.2 Logical Right Shift — `>>`

For **unsigned** types, right shift fills vacated high bits with `0`. Divides by powers of 2 (integer division, truncating toward zero for unsigned).

```
x >> n  =  x / 2^n   (integer division, for unsigned)

Example: 200u >> 3
200 = 0b11001000

After >> 1:  0b01100100 = 100   (200/2 = 100)
After >> 2:  0b00110010 = 50    (200/4 = 50)
After >> 3:  0b00011001 = 25    (200/8 = 25)

Zeros fill in from the left:
0 │ 0 0 1 1 0 0 1   << those zeros were inserted
```

### 6.3 Arithmetic Right Shift — `>>`

For **signed** types (in C on two's complement systems, now standardized in C23), right shift extends the sign bit — the MSB is replicated.

```
Example: -120 >> 1
-120 in two's complement (8-bit): 0b10001000

After >> 1 (arithmetic):
Original: 1 0 0 0 1 0 0 0
Result:   1 1 0 0 0 1 0 0  = 0b11000100 = -60 (signed)
          ↑
     sign bit replicated

This correctly computes floor(-120 / 2) = -60.

But:
-1 >> 1:
-1 = 0b11111111
Result: 0b11111111 = -1    (floor(-1/2) = floor(-0.5) = -1) ✓

-3 >> 1:
-3 = 0b11111101
Result: 0b11111110 = -2    (floor(-3/2) = floor(-1.5) = -2) ✓
```

### 6.4 Rotation — The Missing Operator

C and Go don't have native bit rotation operators, but it's commonly needed and CPUs have native `ROL`/`ROR` instructions.

```
Rotate Left (circular shift):
Original: 1 0 1 1 0 0 1 0
Rotate 3: 1 0 0 1 0 1 0 1  ← bits that fall off left reappear at right

Visual:
┌─────────────────────────┐
│  1 0 1 1 0 0 1 0        │
└──►──────────────────────┘  ROL 3
     ↓
  1 0 0 1 0 1 0 1

C implementation (compiler typically optimizes to ROL instruction):
uint32_t rotl32(uint32_t x, uint32_t n) {
    return (x << n) | (x >> (32 - n));
}

uint32_t rotr32(uint32_t x, uint32_t n) {
    return (x >> n) | (x << (32 - n));
}
```

### 6.5 Shift Operator Truth Table Summary

```
Operation       Fill bit    Effect on unsigned    Effect on signed
─────────────────────────────────────────────────────────────────
x << n          0 (right)   x * 2^n (mod 2^W)    UB if overflow
x >> n          0 (left)    x / 2^n (floor)       impl-defined*
                                                  (arithmetic on most)
ROL(x,n)        wrap        circular              circular
ROR(x,n)        wrap        circular              circular

* C23 mandates two's complement and arithmetic right shift for signed types.
  In practice all modern compilers do arithmetic right shift for signed.
```

---

## 7. Fast Arithmetic with Bit Operations

### 7.1 Multiplication and Division by Powers of 2

```
Multiplication by 2^n  ←→  Left shift by n
n * 2    = n << 1
n * 4    = n << 2
n * 8    = n << 3
n * 16   = n << 4
n * 1024 = n << 10

Division by 2^n (unsigned) ←→ Right shift by n
n / 2    = n >> 1
n / 4    = n >> 2
n / 8    = n >> 3

Division by 2^n (signed) — floor division:
n / 2    = (n + (n >> 31)) >> 1   ← adjustment needed for negative

Why the adjustment for signed division?
  -7 / 2 = -3.5 → C truncates toward zero → -3
  -7 >> 1 in arithmetic = floor(-3.5) = -4  ← different!
  To match C semantics: (n >> 31) extracts sign, adds 0 for positive, 1 for negative
  This rounds negative numbers toward zero like C integer division.
```

### 7.2 Modulo by Powers of 2

```
n % (2^k) = n & (2^k - 1)    (for non-negative n)

Examples:
n % 2   = n & 1     (0b...XXXXX1: lowest bit tells even/odd)
n % 4   = n & 3     (0b...XXXXXX11)
n % 8   = n & 7     (0b...XXXXXXX111)
n % 16  = n & 15    (0b...XXXXXXXX1111)
n % 256 = n & 255   (extracts lowest byte)
n % 1024 = n & 1023

Why this works:
  n = q * 2^k + r,  where r = n % 2^k, 0 ≤ r < 2^k
  In binary: q occupies high bits, r occupies low k bits
  Masking with (2^k - 1) = 0b00...0111...1 keeps only the low k bits = r

Circular buffer index (very common pattern):
  /* Instead of: index = (index + 1) % BUFFER_SIZE; (where BUFFER_SIZE = 2^k) */
  /* Use: */
  index = (index + 1) & (BUFFER_SIZE - 1);  /* 10x faster */
```

### 7.3 Integer Average Without Overflow

```
Naive (has overflow bug):
  avg = (a + b) / 2;   ← a + b can overflow if both are large

Bit trick (no overflow):
  avg = (a & b) + ((a ^ b) >> 1);

How it works:
  a & b  : bits set in BOTH (carry bits from addition)
  a ^ b  : bits set in EXACTLY ONE (sum without carry)
  (a ^ b) >> 1: half of the "differing" bits
  Sum: common bits + half the differing bits = average

Alternative:
  avg = a + ((b - a) >> 1);   /* also avoids the overflow */
```

### 7.4 Absolute Value Without Branch

```
/* For signed 32-bit integer (two's complement) */
int abs_branchless(int x) {
    int mask = x >> 31;   /* arithmetic: all 0s if positive, all 1s if negative */
    return (x + mask) ^ mask;
}

/* How it works:
   If x ≥ 0: mask = 0x00000000
     (x + 0) ^ 0 = x ✓

   If x < 0: mask = 0xFFFFFFFF (= -1)
     (x + (-1)) ^ (-1)
     = (x - 1) ^ 0xFFFFFFFF
     = ~(x - 1)          [XOR with all-ones = bitwise NOT]
     = -x                [in two's complement: ~y = -y - 1, so ~(x-1) = -(x-1)-1 = -x] ✓
*/
```

### 7.5 Min and Max Without Branches

```c
/* Branchless min for signed integers */
int min_branchless(int a, int b) {
    return b + ((a - b) & ((a - b) >> 31));
}

/* Branchless max for signed integers */
int max_branchless(int a, int b) {
    return a - ((a - b) & ((a - b) >> 31));
}

/*
 * How min works:
 * Let d = a - b
 * d >> 31 = 0xFFFFFFFF if a < b (d is negative)
 *         = 0x00000000 if a >= b (d is non-negative)
 *
 * If a < b: b + (d & 0xFFFFFFFF) = b + d = b + (a-b) = a ✓
 * If a >= b: b + (d & 0x00000000) = b + 0 = b ✓
 */
```

### 7.6 Checking Even/Odd

```c
/* Test if n is even */
if (!(n & 1)) { /* even */ }

/* Test if n is odd */
if (n & 1) { /* odd */ }

/* The lowest bit IS the parity for integers:
   0 → 0b...000 → bit0 = 0 → even
   1 → 0b...001 → bit0 = 1 → odd
   2 → 0b...010 → bit0 = 0 → even
   ...
*/
```

### 7.7 Swap Without a Temporary Variable

```c
/* XOR swap — works for distinct memory locations */
void swap_xor(int *a, int *b) {
    *a ^= *b;  /* a = a ^ b */
    *b ^= *a;  /* b = b ^ (a^b) = a */
    *a ^= *b;  /* a = (a^b) ^ a = b */
}

/*
 * Trace through:
 * a=5 (0101), b=3 (0011)
 * Step 1: a = 0101^0011 = 0110 (6)
 * Step 2: b = 0011^0110 = 0101 (5) ✓
 * Step 3: a = 0110^0101 = 0011 (3) ✓
 *
 * WARNING: Do NOT use if a and b alias the same memory!
 * swap_xor(&x, &x) sets x to 0.
 */
```

### 7.8 Multiply by Constants — Shift-Add Combinations

```
Multiply by 3:   x*3 = (x<<1) + x
Multiply by 5:   x*5 = (x<<2) + x
Multiply by 6:   x*6 = (x<<2) + (x<<1)
Multiply by 7:   x*7 = (x<<3) - x
Multiply by 9:   x*9 = (x<<3) + x
Multiply by 10:  x*10 = (x<<3) + (x<<1)
Multiply by 12:  x*12 = (x<<3) + (x<<2)

Modern compilers do this automatically for compile-time constants,
but understanding it helps when working at assembly level or in embedded
environments without a hardware multiplier.
```

---

## 8. Bit Tricks and Hacks

### 8.1 Population Count (popcount) — Count Set Bits

```c
/* Naive O(n) per bit */
int popcount_naive(uint32_t x) {
    int count = 0;
    while (x) {
        count += x & 1;
        x >>= 1;
    }
    return count;
}

/* Kernighan's trick — O(set bits) */
int popcount_kernighan(uint32_t x) {
    int count = 0;
    while (x) {
        x &= x - 1;   /* clears lowest set bit */
        count++;
    }
    return count;
}

/* How x &= (x-1) works:
   x     = 0b10110100
   x-1   = 0b10110011  (borrows from lowest 1, flips all lower bits)
   x&(x-1)= 0b10110000  (lowest set bit is gone)
*/

/* Parallel bit counting (SWAR — SIMD Within A Register) */
uint32_t popcount_parallel(uint32_t x) {
    x = x - ((x >> 1) & 0x55555555);           /* pairs */
    x = (x & 0x33333333) + ((x >> 2) & 0x33333333); /* nibbles */
    x = (x + (x >> 4)) & 0x0F0F0F0F;           /* bytes */
    return (x * 0x01010101) >> 24;              /* sum bytes via multiply */
}

/* In practice, use compiler builtins: */
__builtin_popcount(x)   /* GCC/Clang */
/* Or CPU instruction (SSE4.2): POPCNT */
```

### 8.2 Find Lowest Set Bit

```c
/* Extract lowest set bit */
int lsb = x & (-x);   /* or x & (~x + 1) */

/* How it works:
   x     = 0b10110100
  -x     = 0b01001100  (two's complement negation)
  x & -x = 0b00000100  (only lowest set bit survives)
*/

/* Clear lowest set bit (Kernighan) */
x &= x - 1;

/* Get position of lowest set bit */
int pos = __builtin_ctz(x);   /* count trailing zeros */

/* CPU instruction: BSF (Bit Scan Forward) on x86 */
```

### 8.3 Find Highest Set Bit

```c
/* Get position of highest set bit */
int pos = 31 - __builtin_clz(x);   /* 31 - count leading zeros */
/* CPU instruction: BSR (Bit Scan Reverse) on x86, CLZ on ARM */

/* Round up to next power of 2 */
uint32_t next_pow2(uint32_t x) {
    x--;
    x |= x >> 1;
    x |= x >> 2;
    x |= x >> 4;
    x |= x >> 8;
    x |= x >> 16;
    x++;
    return x;
}

/* How it works for x = 100:
   99  = 0b01100011
   >>1: 0b01110011 (OR)
   >>2: 0b01111111 (OR, spreads bits right)
   ...
   fills all bits below the highest 1 with 1s
   Then +1 carries to next power of 2:
   0b01111111 + 1 = 0b10000000 = 128 ✓
*/
```

### 8.4 Parity Bit

```c
/* Compute parity: 0 if even number of 1s, 1 if odd */
uint32_t parity(uint32_t x) {
    x ^= x >> 16;
    x ^= x >> 8;
    x ^= x >> 4;
    x &= 0xf;
    return (0x6996 >> x) & 1;  /* lookup table in a magic constant */
}

/* Simpler (but more instructions): */
uint32_t parity_simple(uint32_t x) {
    x ^= x >> 16;
    x ^= x >> 8;
    x ^= x >> 4;
    x ^= x >> 2;
    x ^= x >> 1;
    return x & 1;
}
/* CPU instruction: POPCNT result & 1 gives parity */
```

### 8.5 Reverse Bit Order

```c
/* Reverse all 32 bits */
uint32_t reverse_bits(uint32_t x) {
    x = ((x >> 1) & 0x55555555) | ((x & 0x55555555) << 1);  /* swap pairs */
    x = ((x >> 2) & 0x33333333) | ((x & 0x33333333) << 2);  /* swap nibble halves */
    x = ((x >> 4) & 0x0F0F0F0F) | ((x & 0x0F0F0F0F) << 4);  /* swap bytes halves */
    x = ((x >> 8) & 0x00FF00FF) | ((x & 0x00FF00FF) << 8);  /* swap 16-bit halves */
    x = (x >> 16) | (x << 16);                               /* swap 32-bit halves */
    return x;
}
/* ARM has RBIT instruction for this */
```

### 8.6 Power-of-2 Tricks

```c
/* Is x a power of 2? */
bool is_pow2(uint32_t x) {
    return x && !(x & (x - 1));
}

/* Align n up to next multiple of alignment (must be power of 2) */
#define ALIGN_UP(n, align)   (((n) + (align) - 1) & ~((align) - 1))
#define ALIGN_DOWN(n, align) ((n) & ~((align) - 1))

/* Example: align 100 up to 64-byte boundary */
ALIGN_UP(100, 64) = (100 + 63) & ~63
                  = 163 & ~63
                  = 0b10100011 & 0b11000000
                  = 0b10000000
                  = 128 ✓

/* This pattern is EVERYWHERE in kernel memory allocation */
```

---

## 9. Two's Complement and Signed Arithmetic

### 9.1 How Two's Complement Works

```
Two's complement for N-bit integers:
  - Non-negative numbers [0, 2^(N-1) - 1]: same as unsigned
  - Negative numbers [-2^(N-1), -1]: bitwise complement + 1

For 8-bit signed:
  Value   Bits        Value   Bits
   127    0111 1111    -1     1111 1111
   126    0111 1110    -2     1111 1110
    ...                -3     1111 1101
     1    0000 0001   ...
     0    0000 0000   -127   1000 0001
                      -128   1000 0000  ← minimum (no positive counterpart!)

Number circle (wraps around):
     0
  -1   1
 -2     2
-3       3
 .       .
-127   127
   -128

Going counterclockwise from 0: 0, -1, -2, ..., -128
Going clockwise from 0: 0, 1, 2, ..., 127

Overflow wraps: 127 + 1 = -128 (signed overflow, UB in C but predictable)
```

### 9.2 Computing Two's Complement

```
Method 1: Flip all bits, add 1
  x = 0b00000101 (+5)
  ~x = 0b11111010
  ~x + 1 = 0b11111011 = -5 ✓

Method 2: From the right, copy bits up to and including the first 1, then flip the rest
  x = 0b00101000 (+40)
        ↑↑↑ copy these three (up to first 1 from right)
  result starts: ...1000
  remaining bits flipped: 11010...
  result: 0b11011000 = -40 ✓

Key identities:
  -x = ~x + 1 = ~(x - 1)
  -1 = 0xFF...FF (all ones)
  INT_MIN = -INT_MIN = INT_MIN (the only value equal to its own negation... and overflow!)
  ~0 = -1
```

### 9.3 Signed vs Unsigned in Bit Operations

```
Critical difference — comparison:
  (int8_t)  0b11111111 = -1
  (uint8_t) 0b11111111 = 255

  -1 < 0   → true  (signed comparison)
  255 < 0  → false (unsigned comparison)

Right shift behavior:
  (int8_t)  0b10000000 >> 1 = 0b11000000 = -64  (arithmetic, sign-extended)
  (uint8_t) 0b10000000 >> 1 = 0b01000000 =  64  (logical, zero-filled)

The C integer promotion rules + sign/unsigned mixing cause many bugs:
  /* BUG: unsigned > signed comparison */
  int len = -1;
  size_t size = 1;
  if (len < size) { /* ... */ }  /* This might NOT execute! */
  /* -1 as unsigned = SIZE_MAX, which is > 1 */
  /* Always be explicit about types in comparisons */
```

---

## 10. Linux Kernel Bit Operations

### 10.1 Atomic Bit Operations

The Linux kernel provides atomic bit operations for SMP safety (no need for locks for single-bit operations):

```c
/* From include/asm-generic/bitops/atomic.h and arch-specific variants */

/* Set bit nr in the bitmap pointed to by addr */
void set_bit(unsigned int nr, volatile unsigned long *addr);

/* Clear bit nr */
void clear_bit(unsigned int nr, volatile unsigned long *addr);

/* Atomically test and set — returns old value */
int test_and_set_bit(unsigned int nr, volatile unsigned long *addr);

/* Test if bit is set */
int test_bit(unsigned int nr, const volatile unsigned long *addr);

/* Change bit */
void change_bit(unsigned int nr, volatile unsigned long *addr);

/*
 * On x86, these compile to:
 *   set_bit   → LOCK BTS (Bit Test and Set with bus lock)
 *   clear_bit → LOCK BTR (Bit Test and Reset)
 *   test_bit  → BT  (Bit Test, no lock needed for read)
 *
 * ARM equivalents use LDREX/STREX (load-exclusive/store-exclusive)
 * for the read-modify-write cycle.
 */
```

### 10.2 Bitmaps in the Kernel

```c
/*
 * The Linux kernel has a rich bitmap API in lib/bitmap.c
 * Used for: CPU masks (cpumask_t), IRQ masks, memory node masks, etc.
 */

/* DECLARE_BITMAP creates an array of unsigned longs */
#define BITS_PER_LONG   64  /* on 64-bit systems */
#define BITS_TO_LONGS(nr) DIV_ROUND_UP(nr, BITS_PER_LONG)
#define DECLARE_BITMAP(name, bits) unsigned long name[BITS_TO_LONGS(bits)]

/* Example: bitmap for 256 CPUs */
DECLARE_BITMAP(cpu_bitmap, 256);
/* Creates: unsigned long cpu_bitmap[4] (4 * 64 = 256 bits) */

/*
 * Bitmap layout in memory:
 *
 *   word[0]                    word[1]
 * ┌────────────────────────┐ ┌────────────────────────┐
 * │ bits 63..0             │ │ bits 127..64           │
 * └────────────────────────┘ └────────────────────────┘
 *   bit0 at word[0] bit0       bit64 at word[1] bit0
 *
 * Bit N is in word[N / BITS_PER_LONG] at position N % BITS_PER_LONG
 */

/* Common bitmap operations */
bitmap_zero(dst, nbits);           /* clear all bits */
bitmap_fill(dst, nbits);           /* set all bits */
bitmap_copy(dst, src, nbits);      /* copy */
bitmap_and(dst, src1, src2, nbits);/* dst = src1 & src2 */
bitmap_or(dst, src1, src2, nbits); /* dst = src1 | src2 */
bitmap_xor(dst, src1, src2, nbits);/* dst = src1 ^ src2 */
bitmap_weight(src, nbits);         /* count set bits */
find_first_bit(addr, size);        /* index of first set bit */
find_next_bit(addr, size, offset); /* index of next set bit after offset */
```

### 10.3 cpumask — CPU Affinity and Online CPUs

```c
/*
 * cpumask_t is a bitmap of NR_CPUS bits, used everywhere for
 * scheduling, IRQ affinity, NUMA topology, etc.
 */

/* Real kernel usage patterns: */

/* Get current CPU's bit set in mask */
cpumask_set_cpu(cpu, mask);

/* Iterate over CPUs in mask */
for_each_cpu(cpu, mask) {
    /* do something for each CPU */
}

/* CPU online mask operations */
if (cpu_online(cpu)) { ... }
cpumask_copy(dst, cpu_online_mask);

/* Under the hood, for_each_cpu uses find_next_bit: */
#define for_each_cpu(cpu, mask)                          \
    for ((cpu) = cpumask_first(mask);                    \
         (cpu) < nr_cpu_ids;                             \
         (cpu) = cpumask_next((cpu), (mask)))

/* cpumask_first = find_first_bit, cpumask_next = find_next_bit */
```

### 10.4 GENMASK and Field Manipulation in the Kernel

```c
/* From include/linux/bits.h */
#define BIT(nr)         (1UL << (nr))
#define BIT_ULL(nr)     (1ULL << (nr))

#define GENMASK(h, l)   \
    (((~0UL) - (1UL << (l)) + 1) & (~0UL >> (BITS_PER_LONG - 1 - (h))))

/* From include/linux/bitfield.h */
/* FIELD_PREP: insert value into field defined by mask */
#define FIELD_PREP(_mask, _val) \
    ({                                              \
        (typeof(_mask))(((_val) << __bf_shf(_mask)) & (_mask));  \
    })

/* FIELD_GET: extract field value */
#define FIELD_GET(_mask, _reg) \
    ({                                              \
        (typeof(_mask))(((_reg) & (_mask)) >> __bf_shf(_mask));  \
    })

/* Real kernel example: PCIe Capability Register parsing */
#define PCI_EXP_LNKSTA_CLS         0x000F  /* bits [3:0]: link speed */
#define PCI_EXP_LNKSTA_NLW         0x03F0  /* bits [9:4]: negotiated width */
#define PCI_EXP_LNKSTA_LT          0x0800  /* bit  [11]:  link training */

uint16_t lnksta = read_pcie_register();
uint8_t  speed = FIELD_GET(PCI_EXP_LNKSTA_CLS, lnksta);
uint8_t  width = FIELD_GET(PCI_EXP_LNKSTA_NLW, lnksta);
bool     training = !!(lnksta & PCI_EXP_LNKSTA_LT);
```

---

## 11. Network Subsystem Bit Operations

### 11.1 IP Header Bit Layout

The IPv4 header is a masterclass in bit packing:

```
IPv4 Header (20 bytes minimum):

  0                   1                   2                   3
  0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |Version|  IHL  |Type of Service|          Total Length         |
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |         Identification        |Flags|      Fragment Offset    |
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |  Time to Live |    Protocol   |         Header Checksum       |
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |                       Source Address                          |
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |                    Destination Address                        |
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

Byte 0: Version (bits 7:4) + IHL (bits 3:0)
  0x45 = 0b01000101
         ↑↑↑↑ ↑↑↑↑
         0100 0101
         ver=4 IHL=5 (5 × 4 = 20 bytes, no options)

  Extract version: (byte0 >> 4) & 0x0F
  Extract IHL:     byte0 & 0x0F

Fragmentation word (bytes 6-7): Flags + Fragment Offset
  Bit 15: Reserved (must be 0)
  Bit 14: Don't Fragment (DF)
  Bit 13: More Fragments (MF)
  Bits 12:0: Fragment Offset (in 8-byte units)

  #define IP_RF    0x8000   /* reserved fragment flag */
  #define IP_DF    0x4000   /* don't fragment flag */
  #define IP_MF    0x2000   /* more fragments flag */
  #define IP_OFFMASK 0x1FFF /* mask for fragment offset */

  uint16_t frag_word = ntohs(iph->frag_off);  /* network byte order → host */
  bool df    = !!(frag_word & IP_DF);
  bool mf    = !!(frag_word & IP_MF);
  uint16_t offset = (frag_word & IP_OFFMASK) << 3;  /* ×8 for byte offset */
```

### 11.2 TCP Flags and State Machine

```
TCP Header Flags (offset 12-13 in TCP header):

  ┌────┬────┬────┬────┬────┬────┬────┬────┐
  │ CWR│ ECE│ URG│ ACK│ PSH│ RST│ SYN│ FIN│  byte 13
  └────┴────┴────┴────┴────┴────┴────┴────┘
    7    6    5    4    3    2    1    0

  #define TCP_FLAG_FIN  0x01   /* bit 0: finish */
  #define TCP_FLAG_SYN  0x02   /* bit 1: synchronize */
  #define TCP_FLAG_RST  0x04   /* bit 2: reset */
  #define TCP_FLAG_PSH  0x08   /* bit 3: push */
  #define TCP_FLAG_ACK  0x10   /* bit 4: acknowledgment */
  #define TCP_FLAG_URG  0x20   /* bit 5: urgent */
  #define TCP_FLAG_ECE  0x40   /* bit 6: ECN-Echo */
  #define TCP_FLAG_CWR  0x80   /* bit 7: Congestion Window Reduced */

  /* Common combinations */
  SYN packet:      flags = 0x02 = 0b00000010
  SYN-ACK packet:  flags = 0x12 = 0b00010010
  ACK packet:      flags = 0x10 = 0b00010000
  FIN-ACK packet:  flags = 0x11 = 0b00010001
  RST packet:      flags = 0x04 = 0b00000100

  /* Parse flags */
  uint8_t flags = tcp_hdr->th_flags;
  bool is_syn = !!(flags & TCP_FLAG_SYN);
  bool is_ack = !!(flags & TCP_FLAG_ACK);
  bool is_syn_ack = (flags & (TCP_FLAG_SYN | TCP_FLAG_ACK)) == 
                    (TCP_FLAG_SYN | TCP_FLAG_ACK);
```

### 11.3 Linux Socket State Flags (sk_buff and sock)

```c
/*
 * struct sk_buff — the fundamental network buffer in Linux
 * Contains numerous bit fields for packet metadata
 */

struct sk_buff {
    /* ... */
    __u8    local_df:1,   /* allow local fragmentation */
            cloned:1,     /* head may be cloned */
            ip_summed:2,  /* checksum state (2 bits!) */
            nohdr:1,      /* payload after header */
            nfctinfo:3;   /* netfilter conntrack info */

    __u8    pkt_type:3,   /* packet class: PACKET_HOST, BROADCAST, etc */
            fclone:2,     /* skbuff clone status */
            ipvs_property:1,
            peeked:1,
            nf_trace:1;

    /* ip_summed values (2-bit field): */
    /* CHECKSUM_NONE    = 0: no checksum available */
    /* CHECKSUM_UNNECESSARY = 1: HW verified checksum */
    /* CHECKSUM_COMPLETE    = 2: csum field is complete checksum */
    /* CHECKSUM_PARTIAL     = 3: csum_start set, needs completion */
    /* ... */
};

/*
 * Socket state flags (sock->sk_flags, sock->sk_state)
 * sk_state uses TCP_* values packed as unsigned char
 */

/* Socket option flags (sk_flags) */
#define SOCK_DEAD           0   /* socket has been fully disconnected */
#define SOCK_DONE           1   /* orderly disconnect in progress */
#define SOCK_URGINLINE      2   /* urgent data inline */
#define SOCK_KEEPOPEN       3   /* TCP keepalive on */
#define SOCK_LINGER         4   /* linger on close */
#define SOCK_DESTROY        5   /* socket being destroyed */
#define SOCK_BROADCAST      6   /* socket has broadcast permission */
#define SOCK_TIMESTAMP      7   /* enable timestamps */
/* Test with: sock_flag(sk, SOCK_DEAD) → test_bit(SOCK_DEAD, &sk->sk_flags) */
```

### 11.4 IP Address Manipulation

```c
/*
 * IPv4 addresses are 32-bit integers in network byte order.
 * Bit operations are essential for subnet calculations.
 */

typedef uint32_t be32;  /* big-endian 32-bit (network order) */

/* Subnet mask for /24 (255.255.255.0) in host byte order */
uint32_t mask24 = 0xFFFFFF00;  /* 1111...1111 0000 0000 */
/* Or: ~((1 << (32 - prefix)) - 1) for prefix-length notation */
/* prefix=24: ~((1<<8)-1) = ~0xFF = 0xFFFFFF00 ✓ */

/* Check if two IPs are in the same subnet */
bool same_subnet(uint32_t ip_a, uint32_t ip_b, uint32_t mask) {
    return (ip_a & mask) == (ip_b & mask);
}

/* Get network address (host bits zeroed) */
uint32_t network = ip & mask;

/* Get broadcast address (host bits all 1) */
uint32_t broadcast = (ip & mask) | ~mask;

/* Get host portion */
uint32_t host_part = ip & ~mask;

/*
 * Example: IP 192.168.10.50, mask /24
 * ip   = 0xC0A80A32  (192.168.10.50)
 * mask = 0xFFFFFF00
 *
 * network   = 0xC0A80A00  (192.168.10.0)
 * broadcast = 0xC0A80AFF  (192.168.10.255)
 * host_part = 0x00000032  (50)
 */

/* IPv6 uses 128 bits — requires 2 × uint64_t or 4 × uint32_t */
struct in6_addr {
    union {
        uint8_t  u6_addr8[16];
        uint16_t u6_addr16[8];
        uint32_t u6_addr32[4];
    } in6_u;
};
```

### 11.5 Ethernet Frame and MAC Address Bits

```
Ethernet Frame (simplified):

  ┌────────────┬────────────┬────────┬────────────────┬──────┐
  │  Dst MAC   │  Src MAC   │ EType  │    Payload     │ FCS  │
  │  6 bytes   │  6 bytes   │ 2bytes │  46-1500 bytes │ 4 B  │
  └────────────┴────────────┴────────┴────────────────┴──────┘

MAC Address bit interpretation (first byte of destination):
  Bit 0 (LSB): Multicast bit
    0 = unicast (goes to one interface)
    1 = multicast (FF:FF:FF:FF:FF:FF is broadcast)

  Bit 1: Locally Administered bit
    0 = globally unique (OUI assigned by IEEE)
    1 = locally administered (set by software)

  Check if destination is multicast:
  bool is_multicast = !!(dst_mac[0] & 0x01);

  Check if destination is broadcast (FF:FF:FF:FF:FF:FF):
  bool is_broadcast = (dst_mac[0] & dst_mac[1] & dst_mac[2] &
                       dst_mac[3] & dst_mac[4] & dst_mac[5]) == 0xFF;

EtherType values (common):
  0x0800 = IPv4
  0x0806 = ARP
  0x86DD = IPv6
  0x8100 = 802.1Q VLAN (4 bytes follow)
  0x8847 = MPLS unicast

VLAN tag (802.1Q), 4 bytes after EtherType 0x8100:
  ┌────────────┬──────────────────────────────┐
  │  0x8100    │  TCI (Tag Control Information)│
  │ (EtherType)│         2 bytes              │
  └────────────┴──────────────────────────────┘

  TCI bit layout:
  Bit 15:13 → PCP (Priority Code Point, 3 bits): 802.1p QoS priority
  Bit 12    → DEI (Drop Eligible Indicator): 0 or 1
  Bits 11:0 → VID (VLAN Identifier, 12 bits): 0-4095

  uint16_t tci = ntohs(vlan_tag->tci);
  uint8_t  pcp = (tci >> 13) & 0x07;
  bool     dei = !!(tci & 0x1000);
  uint16_t vid = tci & 0x0FFF;
```

---

## 12. Hardware Registers and Memory-Mapped I/O

### 12.1 MMIO Register Access Pattern

```
Memory-Mapped I/O (MMIO):
Hardware registers appear at specific physical addresses.
Reading/writing these addresses controls hardware.

Physical memory map (ARM SoC example):
  0x00000000 - 0x3FFFFFFF  → RAM
  0xFE200000               → GPIO registers (Raspberry Pi BCM2837)
  0xFE215000               → UART registers
  0xFE204000               → SPI registers

Register access pattern in C:
  #define GPIO_BASE   0xFE200000UL
  #define GPFSEL0     (*(volatile uint32_t *)(GPIO_BASE + 0x00))
  #define GPSET0      (*(volatile uint32_t *)(GPIO_BASE + 0x1C))
  #define GPCLR0      (*(volatile uint32_t *)(GPIO_BASE + 0x28))
  #define GPLEV0      (*(volatile uint32_t *)(GPIO_BASE + 0x34))

  /* volatile is critical: prevents compiler from caching the value */
```

### 12.2 GPIO Register Bit Fields (Raspberry Pi BCM2837)

```
GPFSEL0 — GPIO Function Select 0 (controls pins 0-9):
Each pin uses 3 bits: FSELn = bits [3n+2 : 3n]

  Bit: 31 30 29 | 28 27 26 | 25 24 23 | 22 21 20 | 19 18 17 | ...
  Pin: reserved | GPIO 9   | GPIO 8   | GPIO 7   | GPIO 6   | ...

  Values for each 3-bit field:
  000 = Input
  001 = Output
  100 = Alt function 0
  101 = Alt function 1
  110 = Alt function 2
  111 = Alt function 3
  011 = Alt function 4
  010 = Alt function 5

  Set GPIO 4 as output:
  #define GPIO4_FSEL_SHIFT  12      /* bits [14:12] for GPIO4 */
  #define GPIO4_FSEL_MASK   (0x7 << GPIO4_FSEL_SHIFT)
  #define FSEL_OUTPUT       0x1

  GPFSEL0 = (GPFSEL0 & ~GPIO4_FSEL_MASK) |
             (FSEL_OUTPUT << GPIO4_FSEL_SHIFT);

  Drive GPIO 4 high (set):
  GPSET0 = (1 << 4);   /* writing 1 to bit sets the pin */

  Drive GPIO 4 low (clear):
  GPCLR0 = (1 << 4);   /* writing 1 to bit clears the pin */

  Read GPIO 4 state:
  bool state = !!(GPLEV0 & (1 << 4));
```

### 12.3 x86 CR0 Control Register

```
x86 CR0 register (controls CPU operating mode):

  Bit  Symbol  Description
  ─────────────────────────────────────────────────────
   0    PE      Protected Mode Enable (1 = protected mode)
   1    MP      Monitor Coprocessor
   2    EM      Emulation (1 = no FPU, trap FPU instructions)
   3    TS      Task Switched (FPU context lazy save)
   4    ET      Extension Type (always 1 on modern CPUs)
   5    NE      Numeric Error (FPU error reporting)
  16    WP      Write Protect (prevent kernel writing to RO user pages)
  18    AM      Alignment Mask
  29    NW      Not Write-through (cache policy)
  30    CD      Cache Disable
  31    PG      Paging Enable (1 = virtual memory active)

  #define CR0_PE  (1UL << 0)
  #define CR0_WP  (1UL << 16)
  #define CR0_PG  (1UL << 31)

  /* Linux kernel enables paging and write protection: */
  unsigned long cr0 = read_cr0();
  cr0 |= CR0_PG;    /* enable paging */
  cr0 |= CR0_WP;    /* write protect */
  write_cr0(cr0);
```

---

## 13. C Implementations

### 13.1 Complete Bitmask Utility Library

```c
/* bitmask.h — Production-quality bit manipulation library */
#ifndef BITMASK_H
#define BITMASK_H

#include <stdint.h>
#include <stdbool.h>
#include <limits.h>

/* ══════════════════════════════════════════════
 * Bit and Mask Construction
 * ══════════════════════════════════════════════ */

/* Single bit at position n */
#define BIT(n)          ((uint64_t)1 << (n))
#define BIT32(n)        ((uint32_t)1 << (n))
#define BIT8(n)         ((uint8_t)1 << (n))

/* Bitmask covering bits [high:low] inclusive */
#define GENMASK(h, l)   \
    (((uint64_t)(~0ULL)) >> (63 - (h)) & (~0ULL << (l)))

#define GENMASK32(h, l) \
    (((uint32_t)(~0U)) >> (31 - (h)) & ((uint32_t)(~0U) << (l)))

/* ══════════════════════════════════════════════
 * Bit Testing
 * ══════════════════════════════════════════════ */

static inline bool bit_test(uint64_t val, unsigned int n)
{ return !!(val & BIT(n)); }

static inline bool mask_all_set(uint64_t val, uint64_t mask)
{ return (val & mask) == mask; }

static inline bool mask_any_set(uint64_t val, uint64_t mask)
{ return !!(val & mask); }

static inline bool mask_none_set(uint64_t val, uint64_t mask)
{ return !(val & mask); }

static inline bool is_power_of_2(uint64_t n)
{ return n && !(n & (n - 1)); }

/* ══════════════════════════════════════════════
 * Bit Manipulation
 * ══════════════════════════════════════════════ */

static inline uint64_t bit_set(uint64_t val, unsigned int n)
{ return val | BIT(n); }

static inline uint64_t bit_clear(uint64_t val, unsigned int n)
{ return val & ~BIT(n); }

static inline uint64_t bit_toggle(uint64_t val, unsigned int n)
{ return val ^ BIT(n); }

static inline uint64_t mask_set(uint64_t val, uint64_t mask)
{ return val | mask; }

static inline uint64_t mask_clear(uint64_t val, uint64_t mask)
{ return val & ~mask; }

static inline uint64_t mask_toggle(uint64_t val, uint64_t mask)
{ return val ^ mask; }

/* Conditionally set or clear a mask */
static inline uint64_t mask_assign(uint64_t val, uint64_t mask, bool set)
{ return set ? (val | mask) : (val & ~mask); }

/* ══════════════════════════════════════════════
 * Field Extraction and Insertion
 * ══════════════════════════════════════════════ */

/* Extract field defined by mask (auto-shifts) */
static inline uint64_t field_get(uint64_t reg, uint64_t mask) {
    return (reg & mask) >> __builtin_ctzll(mask);
}

/* Insert value into field defined by mask */
static inline uint64_t field_prep(uint64_t mask, uint64_t val) {
    return (val << __builtin_ctzll(mask)) & mask;
}

/* Modify a field: read-modify-write */
static inline uint64_t field_set(uint64_t reg, uint64_t mask, uint64_t val) {
    return (reg & ~mask) | field_prep(mask, val);
}

/* ══════════════════════════════════════════════
 * Count and Find
 * ══════════════════════════════════════════════ */

/* Count set bits (population count) */
static inline int popcount32(uint32_t x) { return __builtin_popcount(x); }
static inline int popcount64(uint64_t x) { return __builtin_popcountll(x); }

/* Count leading zeros */
static inline int clz32(uint32_t x) { return x ? __builtin_clz(x) : 32; }
static inline int clz64(uint64_t x) { return x ? __builtin_clzll(x) : 64; }

/* Count trailing zeros */
static inline int ctz32(uint32_t x) { return x ? __builtin_ctz(x) : 32; }
static inline int ctz64(uint64_t x) { return x ? __builtin_ctzll(x) : 64; }

/* Position of highest set bit (floor(log2(x))) */
static inline int bit_floor_log2(uint32_t x)
{ return x ? (31 - clz32(x)) : -1; }

/* Position of lowest set bit */
static inline int find_lsb(uint64_t x)
{ return x ? ctz64(x) : -1; }

/* Clear lowest set bit */
static inline uint64_t clear_lsb(uint64_t x) { return x & (x - 1); }

/* Extract lowest set bit */
static inline uint64_t extract_lsb(uint64_t x) { return x & (-x); }

/* ══════════════════════════════════════════════
 * Alignment
 * ══════════════════════════════════════════════ */

/* align must be power of 2 */
#define ALIGN_UP(n, align)   (((n) + (align) - 1) & ~((align) - 1))
#define ALIGN_DOWN(n, align) ((n) & ~((align) - 1))
#define IS_ALIGNED(n, align) (!((n) & ((align) - 1)))

/* ══════════════════════════════════════════════
 * Rotation
 * ══════════════════════════════════════════════ */

static inline uint32_t rotl32(uint32_t x, unsigned int n) {
    n &= 31;
    return (x << n) | (x >> ((-n) & 31));
}

static inline uint32_t rotr32(uint32_t x, unsigned int n) {
    n &= 31;
    return (x >> n) | (x << ((-n) & 31));
}

static inline uint64_t rotl64(uint64_t x, unsigned int n) {
    n &= 63;
    return (x << n) | (x >> ((-n) & 63));
}

/* ══════════════════════════════════════════════
 * Byte Reversal (Endian swap)
 * ══════════════════════════════════════════════ */

static inline uint16_t bswap16(uint16_t x)
{ return __builtin_bswap16(x); }

static inline uint32_t bswap32(uint32_t x)
{ return __builtin_bswap32(x); }

static inline uint64_t bswap64(uint64_t x)
{ return __builtin_bswap64(x); }

/* Network byte order conversions */
#if __BYTE_ORDER__ == __ORDER_LITTLE_ENDIAN__
  #define hton32(x)  bswap32(x)
  #define ntoh32(x)  bswap32(x)
  #define hton16(x)  bswap16(x)
  #define ntoh16(x)  bswap16(x)
#else
  #define hton32(x)  (x)
  #define ntoh32(x)  (x)
  #define hton16(x)  (x)
  #define ntoh16(x)  (x)
#endif

/* ══════════════════════════════════════════════
 * Fast Arithmetic
 * ══════════════════════════════════════════════ */

/* Unsigned fast division by power of 2 */
#define FAST_DIV2(x, n)   ((uint64_t)(x) >> (n))

/* Unsigned fast modulo by power of 2 */
#define FAST_MOD2(x, n)   ((uint64_t)(x) & (((uint64_t)1 << (n)) - 1))

/* Signed absolute value (branchless) */
static inline int32_t abs_branchless(int32_t x) {
    int32_t mask = x >> 31;
    return (x + mask) ^ mask;
}

/* Next power of 2 >= x */
static inline uint32_t next_pow2_32(uint32_t x) {
    if (x == 0) return 1;
    x--;
    x |= x >> 1;
    x |= x >> 2;
    x |= x >> 4;
    x |= x >> 8;
    x |= x >> 16;
    return x + 1;
}

#endif /* BITMASK_H */
```

### 13.2 Practical Flag System in C

```c
/* flags_demo.c — Complete flag system example */
#include <stdio.h>
#include <stdint.h>
#include <stdbool.h>

/* ── Permission flags (file system style) ── */
typedef uint16_t FilePerms;

#define PERM_NONE       0x0000
#define PERM_OX         (1 << 0)   /* owner execute */
#define PERM_OW         (1 << 1)   /* owner write */
#define PERM_OR         (1 << 2)   /* owner read */
#define PERM_GX         (1 << 3)   /* group execute */
#define PERM_GW         (1 << 4)   /* group write */
#define PERM_GR         (1 << 5)   /* group read */
#define PERM_WX         (1 << 6)   /* world execute */
#define PERM_WW         (1 << 7)   /* world write */
#define PERM_WR         (1 << 8)   /* world read */
#define PERM_STICKY     (1 << 9)   /* sticky bit */
#define PERM_SGID       (1 << 10)  /* set group ID */
#define PERM_SUID       (1 << 11)  /* set user ID */

/* Unix chmod 755 = rwxr-xr-x */
#define PERM_755  (PERM_OR | PERM_OW | PERM_OX | \
                   PERM_GR | PERM_GX | \
                   PERM_WR | PERM_WX)

/* Unix chmod 644 = rw-r--r-- */
#define PERM_644  (PERM_OR | PERM_OW | PERM_GR | PERM_WR)

void print_perms(FilePerms p) {
    printf("%c%c%c%c%c%c%c%c%c",
        (p & PERM_SUID) ? 's' : '-',
        (p & PERM_OR) ? 'r' : '-',
        (p & PERM_OW) ? 'w' : '-',
        (p & PERM_OX) ? 'x' : '-',
        (p & PERM_GR) ? 'r' : '-',
        (p & PERM_GW) ? 'w' : '-',
        (p & PERM_GX) ? 'x' : '-',
        (p & PERM_WR) ? 'r' : '-',
        (p & PERM_WW) ? 'w' : '-'
    );
    printf(" [0x%04X = 0%o]\n", p, p);
}

/* ── Connection state flags ── */
typedef uint32_t ConnState;

#define CONN_CONNECTED      BIT32(0)
#define CONN_TLS_ACTIVE     BIT32(1)
#define CONN_AUTHENTICATED  BIT32(2)
#define CONN_IDLE           BIT32(3)
#define CONN_CLOSING        BIT32(4)
#define CONN_ERROR          BIT32(5)
#define CONN_KEEPALIVE      BIT32(6)
#define CONN_HTTP2          BIT32(7)

/* A "secure idle authenticated" connection */
#define CONN_SECURE_IDLE  (CONN_CONNECTED | CONN_TLS_ACTIVE | \
                           CONN_AUTHENTICATED | CONN_IDLE)

static inline bool conn_is_ready(ConnState s) {
    return mask_all_set(s, CONN_CONNECTED | CONN_AUTHENTICATED);
}

static inline bool conn_is_healthy(ConnState s) {
    return mask_any_set(s, CONN_CONNECTED) &&
           mask_none_set(s, CONN_ERROR | CONN_CLOSING);
}

/* ── Bitmap for tracking free slots ── */
typedef struct {
    uint64_t words[4];   /* 256 slots */
    int capacity;
} SlotBitmap;

static inline void bitmap_set_slot(SlotBitmap *bm, int slot) {
    bm->words[slot / 64] |= (uint64_t)1 << (slot % 64);
}

static inline void bitmap_clear_slot(SlotBitmap *bm, int slot) {
    bm->words[slot / 64] &= ~((uint64_t)1 << (slot % 64));
}

static inline bool bitmap_test_slot(const SlotBitmap *bm, int slot) {
    return !!(bm->words[slot / 64] & ((uint64_t)1 << (slot % 64)));
}

/* Find first free slot (bit that is 0, i.e. find first 0 bit) */
static int bitmap_find_free(const SlotBitmap *bm) {
    for (int i = 0; i < 4; i++) {
        uint64_t w = ~bm->words[i];   /* invert: 1s are now free slots */
        if (w) {
            return i * 64 + __builtin_ctzll(w);
        }
    }
    return -1;   /* all slots used */
}
```

### 13.3 Network Packet Parser in C

```c
/* pkt_parse.c — Parse IPv4/TCP headers using bit operations */
#include <stdint.h>
#include <stdbool.h>
#include <arpa/inet.h>

/* IPv4 header (big-endian, as it appears on wire) */
struct ipv4_hdr {
    uint8_t  ver_ihl;       /* version (4 bits) + IHL (4 bits) */
    uint8_t  dscp_ecn;      /* DSCP (6 bits) + ECN (2 bits) */
    uint16_t total_len;
    uint16_t ident;
    uint16_t flags_foff;    /* flags (3 bits) + fragment offset (13 bits) */
    uint8_t  ttl;
    uint8_t  protocol;
    uint16_t checksum;
    uint32_t src;
    uint32_t dst;
} __attribute__((packed));

/* TCP header */
struct tcp_hdr {
    uint16_t sport;
    uint16_t dport;
    uint32_t seq;
    uint32_t ack;
    uint8_t  data_off;      /* data offset (4 bits) + reserved (4 bits) */
    uint8_t  flags;
    uint16_t window;
    uint16_t checksum;
    uint16_t urgent;
} __attribute__((packed));

/* Bit field masks */
#define IPV4_VERSION_MASK   0xF0
#define IPV4_IHL_MASK       0x0F
#define IPV4_DSCP_MASK      0xFC
#define IPV4_ECN_MASK       0x03
#define IPV4_FLAG_DF        0x4000
#define IPV4_FLAG_MF        0x2000
#define IPV4_FOFF_MASK      0x1FFF

#define TCP_FLAG_FIN        0x01
#define TCP_FLAG_SYN        0x02
#define TCP_FLAG_RST        0x04
#define TCP_FLAG_PSH        0x08
#define TCP_FLAG_ACK        0x10
#define TCP_FLAG_URG        0x20
#define TCP_FLAG_ECE        0x40
#define TCP_FLAG_CWR        0x80
#define TCP_DOFF_MASK       0xF0
#define TCP_DOFF_SHIFT      4

typedef struct {
    uint8_t  version;       /* IP version */
    uint8_t  ihl;           /* header length in 32-bit words */
    uint8_t  dscp;          /* differentiated services */
    uint8_t  ecn;           /* explicit congestion notification */
    uint16_t total_len;
    bool     dont_frag;
    bool     more_frags;
    uint16_t frag_offset;   /* in bytes */
    uint8_t  ttl;
    uint8_t  protocol;
    uint32_t src;
    uint32_t dst;
    /* TCP fields (if protocol == 6) */
    uint16_t sport;
    uint16_t dport;
    uint8_t  tcp_flags;
    uint32_t tcp_seq;
    uint32_t tcp_ack;
    bool     is_syn;
    bool     is_ack;
    bool     is_fin;
    bool     is_rst;
} ParsedPkt;

int parse_ipv4_tcp(const uint8_t *data, int len, ParsedPkt *out) {
    if (len < (int)sizeof(struct ipv4_hdr)) return -1;

    const struct ipv4_hdr *ip = (const struct ipv4_hdr *)data;

    /* Extract version and IHL from first byte */
    out->version  = (ip->ver_ihl & IPV4_VERSION_MASK) >> 4;
    out->ihl      = (ip->ver_ihl & IPV4_IHL_MASK);
    if (out->version != 4) return -1;

    /* DSCP and ECN from second byte */
    out->dscp     = (ip->dscp_ecn & IPV4_DSCP_MASK) >> 2;
    out->ecn      = (ip->dscp_ecn & IPV4_ECN_MASK);

    out->total_len = ntohs(ip->total_len);

    /* Fragmentation flags (network byte order → host) */
    uint16_t ff   = ntohs(ip->flags_foff);
    out->dont_frag   = !!(ff & IPV4_FLAG_DF);
    out->more_frags  = !!(ff & IPV4_FLAG_MF);
    out->frag_offset = (ff & IPV4_FOFF_MASK) << 3;  /* ×8 for byte offset */

    out->ttl      = ip->ttl;
    out->protocol = ip->protocol;
    out->src      = ntohl(ip->src);
    out->dst      = ntohl(ip->dst);

    /* Parse TCP if applicable */
    if (out->protocol == 6) {
        int ip_hdr_bytes = out->ihl * 4;
        if (len < ip_hdr_bytes + (int)sizeof(struct tcp_hdr)) return -1;

        const struct tcp_hdr *tcp =
            (const struct tcp_hdr *)(data + ip_hdr_bytes);

        out->sport     = ntohs(tcp->sport);
        out->dport     = ntohs(tcp->dport);
        out->tcp_seq   = ntohl(tcp->seq);
        out->tcp_ack   = ntohl(tcp->ack);
        out->tcp_flags = tcp->flags;

        /* Decode flags using bit tests */
        out->is_syn = !!(tcp->flags & TCP_FLAG_SYN);
        out->is_ack = !!(tcp->flags & TCP_FLAG_ACK);
        out->is_fin = !!(tcp->flags & TCP_FLAG_FIN);
        out->is_rst = !!(tcp->flags & TCP_FLAG_RST);
    }

    return 0;
}
```

---

## 14. Rust Implementations

### 14.1 Type-Safe Bit Flags with the `bitflags` Pattern

```rust
// bitflags_demo.rs
// Rust's strong type system allows completely safe, zero-cost bit flags.

use std::fmt;

/// A type-safe wrapper around a u32 bitmask.
/// Uses const generics and const fns for zero-cost abstractions.
#[derive(Copy, Clone, PartialEq, Eq, Default)]
pub struct Flags<T: FlagSet>(u32, std::marker::PhantomData<T>);

pub trait FlagSet: Copy + Clone {}

impl<T: FlagSet> Flags<T> {
    pub const fn empty() -> Self {
        Flags(0, std::marker::PhantomData)
    }

    pub const fn from_bits_unchecked(bits: u32) -> Self {
        Flags(bits, std::marker::PhantomData)
    }

    pub const fn bits(self) -> u32 { self.0 }

    pub const fn contains(self, other: Self) -> bool {
        (self.0 & other.0) == other.0
    }

    pub const fn intersects(self, other: Self) -> bool {
        (self.0 & other.0) != 0
    }

    pub const fn is_empty(self) -> bool { self.0 == 0 }

    pub const fn union(self, other: Self) -> Self {
        Flags(self.0 | other.0, std::marker::PhantomData)
    }

    pub const fn intersection(self, other: Self) -> Self {
        Flags(self.0 & other.0, std::marker::PhantomData)
    }

    pub const fn difference(self, other: Self) -> Self {
        Flags(self.0 & !other.0, std::marker::PhantomData)
    }

    pub const fn symmetric_difference(self, other: Self) -> Self {
        Flags(self.0 ^ other.0, std::marker::PhantomData)
    }

    pub const fn complement(self) -> Self {
        Flags(!self.0, std::marker::PhantomData)
    }
}

// ── Concrete flag type: FilePermissions ──

#[derive(Copy, Clone)]
pub struct FilePerm;
impl FlagSet for FilePerm {}

pub type FilePermissions = Flags<FilePerm>;

impl FilePermissions {
    pub const NONE:         Self = Self::from_bits_unchecked(0);
    pub const OWNER_EXEC:   Self = Self::from_bits_unchecked(1 << 0);
    pub const OWNER_WRITE:  Self = Self::from_bits_unchecked(1 << 1);
    pub const OWNER_READ:   Self = Self::from_bits_unchecked(1 << 2);
    pub const GROUP_EXEC:   Self = Self::from_bits_unchecked(1 << 3);
    pub const GROUP_WRITE:  Self = Self::from_bits_unchecked(1 << 4);
    pub const GROUP_READ:   Self = Self::from_bits_unchecked(1 << 5);
    pub const WORLD_EXEC:   Self = Self::from_bits_unchecked(1 << 6);
    pub const WORLD_WRITE:  Self = Self::from_bits_unchecked(1 << 7);
    pub const WORLD_READ:   Self = Self::from_bits_unchecked(1 << 8);
    pub const STICKY:       Self = Self::from_bits_unchecked(1 << 9);
    pub const SETGID:       Self = Self::from_bits_unchecked(1 << 10);
    pub const SETUID:       Self = Self::from_bits_unchecked(1 << 11);

    // Compound permissions
    pub const OWNER_RWX: Self = Self::from_bits_unchecked(
        Self::OWNER_READ.0 | Self::OWNER_WRITE.0 | Self::OWNER_EXEC.0
    );
    pub const PERM_755: Self = Self::from_bits_unchecked(
        Self::OWNER_RWX.0 |
        Self::GROUP_READ.0 | Self::GROUP_EXEC.0 |
        Self::WORLD_READ.0 | Self::WORLD_EXEC.0
    );
}

impl fmt::Display for FilePermissions {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}{}{}{}{}{}{}{}{}",
            if self.contains(Self::SETUID)      { 's' } else { '-' },
            if self.contains(Self::OWNER_READ)   { 'r' } else { '-' },
            if self.contains(Self::OWNER_WRITE)  { 'w' } else { '-' },
            if self.contains(Self::OWNER_EXEC)   { 'x' } else { '-' },
            if self.contains(Self::GROUP_READ)   { 'r' } else { '-' },
            if self.contains(Self::GROUP_WRITE)  { 'w' } else { '-' },
            if self.contains(Self::GROUP_EXEC)   { 'x' } else { '-' },
            if self.contains(Self::WORLD_READ)   { 'r' } else { '-' },
            if self.contains(Self::WORLD_WRITE)  { 'w' } else { '-' },
        )
    }
}

// ── Bit manipulation primitives ──

/// Safe bit field extraction with compile-time mask validation.
/// MASK must be a contiguous run of 1-bits.
pub const fn field_get(reg: u64, mask: u64) -> u64 {
    (reg & mask) >> mask.trailing_zeros()
}

pub const fn field_prep(mask: u64, val: u64) -> u64 {
    (val << mask.trailing_zeros()) & mask
}

pub const fn field_set(reg: u64, mask: u64, val: u64) -> u64 {
    (reg & !mask) | field_prep(mask, val)
}

// ── Integer bit tricks ──

pub fn popcount(x: u64) -> u32 {
    x.count_ones()       // compiles to POPCNT instruction
}

pub fn find_lsb(x: u64) -> Option<u32> {
    if x == 0 { None } else { Some(x.trailing_zeros()) }
}

pub fn find_msb(x: u64) -> Option<u32> {
    if x == 0 { None } else { Some(63 - x.leading_zeros()) }
}

pub fn clear_lsb(x: u64) -> u64 {
    x & x.wrapping_sub(1)
}

pub fn is_power_of_2(x: u64) -> bool {
    x != 0 && (x & (x - 1)) == 0
}

pub fn next_power_of_2(x: u64) -> u64 {
    x.next_power_of_two()   // std library, uses BSR/CLZ instructions
}

pub fn align_up(n: usize, align: usize) -> usize {
    debug_assert!(is_power_of_2(align as u64));
    (n + align - 1) & !(align - 1)
}

pub fn align_down(n: usize, align: usize) -> usize {
    debug_assert!(is_power_of_2(align as u64));
    n & !(align - 1)
}

pub fn rotl32(x: u32, n: u32) -> u32 {
    x.rotate_left(n)    // std library, compiles to ROL instruction
}

pub fn rotr32(x: u32, n: u32) -> u32 {
    x.rotate_right(n)   // compiles to ROR instruction
}

// ── Branchless arithmetic ──

pub fn abs_branchless(x: i32) -> i32 {
    let mask = x >> 31;
    (x + mask) ^ mask
}

pub fn min_branchless(a: i32, b: i32) -> i32 {
    let d = a.wrapping_sub(b);
    b.wrapping_add(d & (d >> 31))
}

// ── Atomic bit operations (for multi-threaded contexts) ──

use std::sync::atomic::{AtomicU64, Ordering};

pub struct AtomicFlags(AtomicU64);

impl AtomicFlags {
    pub const fn new(val: u64) -> Self {
        AtomicFlags(AtomicU64::new(val))
    }

    pub fn set(&self, mask: u64) {
        self.0.fetch_or(mask, Ordering::SeqCst);
    }

    pub fn clear(&self, mask: u64) {
        self.0.fetch_and(!mask, Ordering::SeqCst);
    }

    pub fn toggle(&self, mask: u64) {
        self.0.fetch_xor(mask, Ordering::SeqCst);
    }

    pub fn test(&self, mask: u64) -> bool {
        (self.0.load(Ordering::SeqCst) & mask) != 0
    }

    /// Atomically test-and-set; returns whether the bit was previously set.
    pub fn test_and_set(&self, bit: u64) -> bool {
        let prev = self.0.fetch_or(bit, Ordering::SeqCst);
        (prev & bit) != 0
    }

    /// Atomically test-and-clear; returns whether the bit was previously set.
    pub fn test_and_clear(&self, bit: u64) -> bool {
        let prev = self.0.fetch_and(!bit, Ordering::SeqCst);
        (prev & bit) != 0
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_field_ops() {
        // Mask covering bits [5:2]
        let mask: u64 = 0b00111100;
        let reg:  u64 = 0b10110110;

        assert_eq!(field_get(reg, mask), 0b1101);  // bits [5:2] = 13
        let new = field_set(reg, mask, 0b0101);
        assert_eq!(new & mask >> 2, 0b0101 & 0b1111);
    }

    #[test]
    fn test_permissions() {
        let p = FilePermissions::PERM_755;
        assert!(p.contains(FilePermissions::OWNER_READ));
        assert!(p.contains(FilePermissions::OWNER_WRITE));
        assert!(p.contains(FilePermissions::OWNER_EXEC));
        assert!(!p.contains(FilePermissions::WORLD_WRITE));
        println!("{}", p);   // -rwxr-xr-x
    }
}
```

### 14.2 IPv4 Packet Parser in Rust

```rust
// net_parse.rs — Zero-copy IPv4/TCP parsing

#[repr(C, packed)]
pub struct Ipv4Header {
    pub ver_ihl:    u8,
    pub dscp_ecn:   u8,
    pub total_len:  u16,   // big-endian
    pub ident:      u16,
    pub flags_foff: u16,   // big-endian
    pub ttl:        u8,
    pub protocol:   u8,
    pub checksum:   u16,
    pub src:        u32,   // big-endian
    pub dst:        u32,   // big-endian
}

impl Ipv4Header {
    pub fn from_bytes(bytes: &[u8]) -> Option<&Self> {
        if bytes.len() < std::mem::size_of::<Self>() {
            return None;
        }
        // SAFETY: Ipv4Header is #[repr(C, packed)], alignment 1
        Some(unsafe { &*(bytes.as_ptr() as *const Self) })
    }

    pub fn version(&self)  -> u8 { (self.ver_ihl & 0xF0) >> 4 }
    pub fn ihl(&self)      -> u8 { self.ver_ihl & 0x0F }
    pub fn hdr_len(&self)  -> usize { (self.ihl() as usize) * 4 }
    pub fn dscp(&self)     -> u8 { (self.dscp_ecn & 0xFC) >> 2 }
    pub fn ecn(&self)      -> u8 { self.dscp_ecn & 0x03 }

    pub fn total_len(&self)  -> u16 { u16::from_be(self.total_len) }
    pub fn ident(&self)      -> u16 { u16::from_be(self.ident) }

    fn flags_foff_host(&self) -> u16 { u16::from_be(self.flags_foff) }
    pub fn dont_fragment(&self) -> bool { (self.flags_foff_host() & 0x4000) != 0 }
    pub fn more_fragments(&self) -> bool { (self.flags_foff_host() & 0x2000) != 0 }
    pub fn frag_offset(&self)  -> u16 { (self.flags_foff_host() & 0x1FFF) << 3 }

    pub fn src_addr(&self) -> std::net::Ipv4Addr {
        std::net::Ipv4Addr::from(u32::from_be(self.src))
    }
    pub fn dst_addr(&self) -> std::net::Ipv4Addr {
        std::net::Ipv4Addr::from(u32::from_be(self.dst))
    }
}

#[repr(C, packed)]
pub struct TcpHeader {
    pub sport:    u16,
    pub dport:    u16,
    pub seq:      u32,
    pub ack:      u32,
    pub data_off: u8,
    pub flags:    u8,
    pub window:   u16,
    pub checksum: u16,
    pub urgent:   u16,
}

impl TcpHeader {
    pub fn from_bytes(bytes: &[u8]) -> Option<&Self> {
        if bytes.len() < std::mem::size_of::<Self>() {
            return None;
        }
        Some(unsafe { &*(bytes.as_ptr() as *const Self) })
    }

    pub fn sport(&self)  -> u16 { u16::from_be(self.sport) }
    pub fn dport(&self)  -> u16 { u16::from_be(self.dport) }
    pub fn seq(&self)    -> u32 { u32::from_be(self.seq) }
    pub fn ack(&self)    -> u32 { u32::from_be(self.ack) }
    pub fn data_offset(&self) -> u8 { (self.data_off & 0xF0) >> 4 }
    pub fn hdr_len(&self)    -> usize { (self.data_offset() as usize) * 4 }

    pub fn flag_fin(&self) -> bool { (self.flags & 0x01) != 0 }
    pub fn flag_syn(&self) -> bool { (self.flags & 0x02) != 0 }
    pub fn flag_rst(&self) -> bool { (self.flags & 0x04) != 0 }
    pub fn flag_psh(&self) -> bool { (self.flags & 0x08) != 0 }
    pub fn flag_ack(&self) -> bool { (self.flags & 0x10) != 0 }
    pub fn flag_urg(&self) -> bool { (self.flags & 0x20) != 0 }
    pub fn flag_ece(&self) -> bool { (self.flags & 0x40) != 0 }
    pub fn flag_cwr(&self) -> bool { (self.flags & 0x80) != 0 }

    pub fn is_syn_ack(&self) -> bool { self.flag_syn() && self.flag_ack() }
    pub fn is_rst_ack(&self) -> bool { self.flag_rst() && self.flag_ack() }
}

// Parse a complete packet
pub fn parse_ipv4_tcp(packet: &[u8]) {
    if let Some(ip) = Ipv4Header::from_bytes(packet) {
        println!("IPv4 v={} ihl={} ttl={} proto={}",
            ip.version(), ip.ihl(), ip.ttl, ip.protocol);
        println!("  src={} dst={}", ip.src_addr(), ip.dst_addr());
        println!("  DF={} MF={} frag_off={}",
            ip.dont_fragment(), ip.more_fragments(), ip.frag_offset());

        if ip.protocol == 6 {
            let tcp_start = ip.hdr_len();
            if let Some(tcp) = TcpHeader::from_bytes(&packet[tcp_start..]) {
                println!("TCP {}→{} seq={} ack={}",
                    tcp.sport(), tcp.dport(), tcp.seq(), tcp.ack());
                println!("  flags: SYN={} ACK={} FIN={} RST={} PSH={}",
                    tcp.flag_syn(), tcp.flag_ack(),
                    tcp.flag_fin(), tcp.flag_rst(), tcp.flag_psh());
            }
        }
    }
}
```

---

## 15. Go Implementations

### 15.1 Bit Operations and Flag System in Go

```go
// bitops.go — Comprehensive bit operations in Go
package bitops

import (
    "fmt"
    "math/bits"
)

// ── Basic bit operations ──

// TestBit returns true if bit n is set in x.
func TestBit(x uint64, n uint) bool {
    return x&(1<<n) != 0
}

// SetBit returns x with bit n set to 1.
func SetBit(x uint64, n uint) uint64 {
    return x | (1 << n)
}

// ClearBit returns x with bit n cleared.
func ClearBit(x uint64, n uint) uint64 {
    return x &^ (1 << n)   // &^ is Go's "AND NOT" (bit clear) operator
}

// ToggleBit flips bit n in x.
func ToggleBit(x uint64, n uint) uint64 {
    return x ^ (1 << n)
}

// ── Mask operations ──

// MaskAllSet tests if all bits in mask are set in val.
func MaskAllSet(val, mask uint64) bool {
    return (val & mask) == mask
}

// MaskAnySet tests if any bit in mask is set in val.
func MaskAnySet(val, mask uint64) bool {
    return val&mask != 0
}

// MaskSet sets all bits in mask.
func MaskSet(val, mask uint64) uint64 { return val | mask }

// MaskClear clears all bits in mask.
func MaskClear(val, mask uint64) uint64 { return val &^ mask }

// MaskToggle flips all bits in mask.
func MaskToggle(val, mask uint64) uint64 { return val ^ mask }

// GenMask creates a mask for bits [high:low] inclusive.
func GenMask(high, low uint) uint64 {
    width := high - low + 1
    return ((1 << width) - 1) << low
}

// ── Field extraction and insertion ──

// FieldGet extracts a field defined by mask (shifts automatically).
func FieldGet(reg, mask uint64) uint64 {
    return (reg & mask) >> uint(bits.TrailingZeros64(mask))
}

// FieldPrep prepares a value to be inserted at a field defined by mask.
func FieldPrep(mask, val uint64) uint64 {
    return (val << uint(bits.TrailingZeros64(mask))) & mask
}

// FieldSet replaces the field defined by mask with val.
func FieldSet(reg, mask, val uint64) uint64 {
    return (reg &^ mask) | FieldPrep(mask, val)
}

// ── Count and find ──

// Popcount counts the number of set bits.
func Popcount(x uint64) int { return bits.OnesCount64(x) }

// FindLSB returns the position of the lowest set bit, or -1 if x==0.
func FindLSB(x uint64) int {
    if x == 0 { return -1 }
    return bits.TrailingZeros64(x)
}

// FindMSB returns the position of the highest set bit, or -1 if x==0.
func FindMSB(x uint64) int {
    if x == 0 { return -1 }
    return 63 - bits.LeadingZeros64(x)
}

// ClearLSB removes the lowest set bit.
func ClearLSB(x uint64) uint64 { return x & (x - 1) }

// IsPowerOf2 tests if x is a non-zero power of 2.
func IsPowerOf2(x uint64) bool { return x != 0 && (x&(x-1)) == 0 }

// NextPowerOf2 returns the smallest power of 2 >= x.
func NextPowerOf2(x uint64) uint64 {
    if x == 0 { return 1 }
    return 1 << bits.Len64(x-1)
}

// ── Alignment ──

// AlignUp aligns n up to the next multiple of align (must be power of 2).
func AlignUp(n, align uintptr) uintptr {
    return (n + align - 1) &^ (align - 1)
}

// AlignDown aligns n down to a multiple of align (must be power of 2).
func AlignDown(n, align uintptr) uintptr {
    return n &^ (align - 1)
}

// IsAligned returns true if n is a multiple of align (power of 2).
func IsAligned(n, align uintptr) bool {
    return n&(align-1) == 0
}

// ── Rotation ──

func Rotl32(x uint32, n int) uint32 { return bits.RotateLeft32(x, n) }
func Rotr32(x uint32, n int) uint32 { return bits.RotateLeft32(x, -n) }
func Rotl64(x uint64, n int) uint64 { return bits.RotateLeft64(x, n) }

// ── Fast arithmetic ──

// FastDiv2 performs unsigned division by 2^n.
func FastDiv2(x uint64, n uint) uint64 { return x >> n }

// FastMod2 performs unsigned modulo by 2^n.
func FastMod2(x uint64, n uint) uint64 { return x & ((1 << n) - 1) }

// AbsBranchless returns the absolute value of a signed integer without branching.
func AbsBranchless(x int32) int32 {
    mask := x >> 31
    return (x + mask) ^ mask
}

// ── Flag type ──

// Flags is a type-safe flag container.
type Flags uint64

const FlagsNone Flags = 0

// Set returns f with the given flags set.
func (f Flags) Set(flags Flags) Flags { return f | flags }

// Clear returns f with the given flags cleared.
func (f Flags) Clear(flags Flags) Flags { return f &^ flags }

// Toggle returns f with the given flags toggled.
func (f Flags) Toggle(flags Flags) Flags { return f ^ flags }

// Has tests if all given flags are set.
func (f Flags) Has(flags Flags) bool { return (f & flags) == flags }

// Any tests if any of the given flags are set.
func (f Flags) Any(flags Flags) bool { return f&flags != 0 }

// None tests if none of the given flags are set.
func (f Flags) None(flags Flags) bool { return f&flags == 0 }

// ── Network flags ──

// TCP flags
const (
    TCPFlagFIN Flags = 1 << 0
    TCPFlagSYN Flags = 1 << 1
    TCPFlagRST Flags = 1 << 2
    TCPFlagPSH Flags = 1 << 3
    TCPFlagACK Flags = 1 << 4
    TCPFlagURG Flags = 1 << 5
    TCPFlagECE Flags = 1 << 6
    TCPFlagCWR Flags = 1 << 7
)

// IPv4 fragmentation flags
const (
    IPv4FlagDF Flags = 1 << 14
    IPv4FlagMF Flags = 1 << 13
)

// ParseTCPFlags returns a Flags from a raw TCP flags byte.
func ParseTCPFlags(raw byte) Flags {
    return Flags(raw)
}

func (f Flags) TCPString() string {
    var result string
    if f.Has(TCPFlagSYN) { result += "SYN " }
    if f.Has(TCPFlagACK) { result += "ACK " }
    if f.Has(TCPFlagFIN) { result += "FIN " }
    if f.Has(TCPFlagRST) { result += "RST " }
    if f.Has(TCPFlagPSH) { result += "PSH " }
    if f.Has(TCPFlagURG) { result += "URG " }
    if result == "" { result = "(none)" }
    return result
}

// ── Bitmap ──

// Bitmap is a variable-size bitmap backed by a slice of uint64.
type Bitmap struct {
    words []uint64
    size  int
}

func NewBitmap(size int) *Bitmap {
    words := (size + 63) / 64
    return &Bitmap{words: make([]uint64, words), size: size}
}

func (b *Bitmap) Set(n int) {
    b.words[n/64] |= 1 << uint(n%64)
}

func (b *Bitmap) Clear(n int) {
    b.words[n/64] &^= 1 << uint(n%64)
}

func (b *Bitmap) Test(n int) bool {
    return b.words[n/64]&(1<<uint(n%64)) != 0
}

// FindFirst returns the index of the first set bit, or -1 if none.
func (b *Bitmap) FindFirst() int {
    for i, w := range b.words {
        if w != 0 {
            return i*64 + bits.TrailingZeros64(w)
        }
    }
    return -1
}

// FindFirstZero returns the index of the first zero bit, or -1 if none.
func (b *Bitmap) FindFirstZero() int {
    for i, w := range b.words {
        if w != ^uint64(0) {   // not all ones
            return i*64 + bits.TrailingZeros64(^w)
        }
    }
    return -1
}

// Iterate calls fn for each set bit.
func (b *Bitmap) Iterate(fn func(int)) {
    for i, w := range b.words {
        for w != 0 {
            n := bits.TrailingZeros64(w)
            fn(i*64 + n)
            w &= w - 1   // clear lowest set bit
        }
    }
}

// Popcount returns the number of set bits.
func (b *Bitmap) Popcount() int {
    total := 0
    for _, w := range b.words {
        total += bits.OnesCount64(w)
    }
    return total
}

// ── IPv4 subnet calculations ──

// SubnetMask returns a subnet mask for the given prefix length.
func SubnetMask(prefixLen int) uint32 {
    if prefixLen == 0 {
        return 0
    }
    return ^uint32(0) << uint(32-prefixLen)
}

// NetworkAddr returns the network address of the subnet.
func NetworkAddr(ip uint32, prefixLen int) uint32 {
    return ip & SubnetMask(prefixLen)
}

// BroadcastAddr returns the broadcast address of the subnet.
func BroadcastAddr(ip uint32, prefixLen int) uint32 {
    mask := SubnetMask(prefixLen)
    return (ip & mask) | ^mask
}

// SameSubnet returns true if two IPs are in the same subnet.
func SameSubnet(a, b uint32, prefixLen int) bool {
    mask := SubnetMask(prefixLen)
    return (a & mask) == (b & mask)
}

// ── Debug print ──

// PrintBits prints a value in binary, grouped by 4.
func PrintBits(name string, val uint64, width int) {
    fmt.Printf("%-12s = ", name)
    for i := width - 1; i >= 0; i-- {
        if val&(1<<uint(i)) != 0 {
            fmt.Print("1")
        } else {
            fmt.Print("0")
        }
        if i > 0 && i%4 == 0 {
            fmt.Print("_")
        }
    }
    fmt.Printf(" (0x%0*X = %d)\n", (width+3)/4, val, val)
}
```

---

## 16. Advanced Patterns: Bitfields, Packed Structs, and Portability

### 16.1 C Bitfields

```c
/* C allows declaring bitfield members in structs */
struct IpFlags {
    uint16_t frag_offset : 13;  /* 13 bits */
    uint16_t more_frags  : 1;   /* 1 bit */
    uint16_t dont_frag   : 1;   /* 1 bit */
    uint16_t reserved    : 1;   /* 1 bit */
};

/*
 * WARNING — C bitfields are full of portability traps:
 *
 * 1. Bit ordering within a word is implementation-defined.
 *    On little-endian: LSB is first member (usually).
 *    On big-endian: MSB is first member (usually).
 *    The C standard says nothing about this!
 *
 * 2. Members can't span a "storage unit" boundary (impl-defined).
 *
 * 3. You can't take the address of a bitfield member.
 *
 * 4. Bitfields of type 'int' may be signed or unsigned (impl-defined).
 *
 * BEST PRACTICE for network/hardware code:
 * Use explicit integers + bitmask constants instead of bitfields.
 * Only use bitfields for internal data structures where portability
 * between compilers matters less than within a single codebase.
 */

/* Safe alternative — explicit masks (portable): */
#define FLAGS_RESERVED    0x8000
#define FLAGS_DF          0x4000
#define FLAGS_MF          0x2000
#define FLAGS_FOFF_MASK   0x1FFF

uint16_t flags_foff = raw_value;
uint16_t foff = (flags_foff & FLAGS_FOFF_MASK) << 3;
bool df = !!(flags_foff & FLAGS_DF);
```

### 16.2 The CONTAINER_OF Macro (Linux Kernel)

```c
/*
 * One of the most beautiful bit/pointer tricks in the kernel.
 * Given a pointer to a member field, recover the outer struct pointer.
 *
 * Uses the fact that struct member offsets are compile-time constants.
 */

#define container_of(ptr, type, member) ({                     \
    void *__mptr = (void *)(ptr);                              \
    ((type *)(__mptr - offsetof(type, member)));               \
})

/*
 * Example:
 * struct Node {
 *     int data;
 *     struct list_head list;   ← only this pointer is in the linked list
 * };
 *
 * struct list_head *pos = get_from_list();
 * struct Node *node = container_of(pos, struct Node, list);
 *
 * How it works:
 * If &node->list is at address 0x1008,
 * and offsetof(Node, list) == 4 bytes from start of Node,
 * then &node == 0x1008 - 4 == 0x1004
 *
 * The entire Linux linked list implementation (include/linux/list.h)
 * is built on this one pointer arithmetic trick.
 */
```

### 16.3 Endian-Safe Field Access

```c
/*
 * In protocol code, always use explicit byte-order conversion.
 * NEVER read multi-byte fields from packed structs directly on
 * a host that might have different endianness than the protocol.
 */

/* Correct approach: */
static inline uint32_t get_ip_src(const uint8_t *pkt) {
    /* Read byte by byte, then assemble — endian-safe */
    return ((uint32_t)pkt[12] << 24) |
           ((uint32_t)pkt[13] << 16) |
           ((uint32_t)pkt[14] <<  8) |
           ((uint32_t)pkt[15]);
}

/* Or: use memcpy + ntohl to avoid alignment issues */
static inline uint32_t get_u32_be(const uint8_t *p) {
    uint32_t val;
    memcpy(&val, p, 4);
    return ntohl(val);
}
```

---

## 17. Performance Analysis and CPU-Level Mechanics

### 17.1 CPU Instruction Mapping

```
C/Rust/Go Operation → x86-64 Instruction → Latency (Haswell)
─────────────────────────────────────────────────────────────
x & y              → AND r, r/m        → 1 cycle (throughput)
x | y              → OR  r, r/m        → 1 cycle
x ^ y              → XOR r, r/m        → 1 cycle
~x                 → NOT r/m           → 1 cycle
x << n             → SHL r, CL / SHL r, imm8 → 1 cycle
x >> n (unsigned)  → SHR r, CL / SHR r, imm8 → 1 cycle
x >> n (signed)    → SAR r, CL / SAR r, imm8  → 1 cycle
popcount(x)        → POPCNT r, r/m     → 3 cycles
__builtin_ctz(x)   → BSF / TZCNT       → 3 cycles
__builtin_clz(x)   → BSR / LZCNT       → 3 cycles
rotl(x, n)         → ROL r, CL         → 1 cycle
x * y (64-bit)     → IMUL r, r/m       → 3 cycles, 1/cycle throughput
x / y (64-bit)     → IDIV r/m          → 20-90 cycles (!!)

Key insight:
  Division is 20-90x more expensive than a right shift.
  Always replace n/2^k with n>>k for unsigned, or the adjusted form for signed.
```

### 17.2 Branch Prediction and Branchless Code

```
Branch misprediction penalty: ~15-20 cycles on modern CPUs.

For operations that produce one of two values based on a condition:

Branching version:
  if (x < 0) { result = -x; } else { result = x; }
  → CMP + JLE + NEG  (2 paths, branch predictor must guess)
  → ~1 cycle if predicted, ~15 if mispredicted
  → High variance in tight loops

Branchless version (arithmetic right shift):
  int mask = x >> 31;
  result = (x + mask) ^ mask;
  → SAR + ADD + XOR  (always 3 cycles, no prediction needed)
  → Consistent, no variance
  → Better for random/unpredictable data

When to use branchless:
  - Data is random / unpredictable (prediction fails often)
  - Inside very tight inner loops (vectorization opportunity)
  - The condition and both values are cheap to compute

When branching is fine:
  - Data is predictable (prediction hits 95%+)
  - One branch is rare (always-taken or almost-never-taken)
  - The branchless form is much more complex
```

### 17.3 SIMD and Bit Operations

```
SIMD (Single Instruction, Multiple Data) extends bit operations to
operate on 128/256/512-bit wide registers simultaneously.

AVX2 (256-bit) bitwise operations:
  _mm256_and_si256(a, b)    → AND 256 bits at once
  _mm256_or_si256(a, b)     → OR  256 bits at once
  _mm256_xor_si256(a, b)    → XOR 256 bits at once
  _mm256_andnot_si256(a, b) → AND NOT (a = ~a & b)

Example: Check 32 bytes at once for any non-zero:
  __m256i chunk = _mm256_loadu_si256(ptr);
  __m256i zero  = _mm256_setzero_si256();
  __m256i cmp   = _mm256_cmpeq_epi8(chunk, zero);
  int mask      = _mm256_movemask_epi8(cmp);
  // mask is 32 bits, one per byte. mask == 0xFFFFFFFF means all zeros.
  bool has_nonzero = (mask != 0xFFFFFFFF);

  // popcount(mask) = number of zero bytes
  // find_lsb(~mask) = position of first non-zero byte

This is how modern memchr(), strcmp(), and strlen() work.
```

---

## 18. Real-World Case Studies

### 18.1 Linux Kernel Scheduler — Task State Transitions

```
The Linux scheduler uses bit operations for task state management.
A task's state is a bitmask; wakeup checks if the current state
matches the desired state using AND:

                  ┌─────────────────────────┐
                  │   Task State Machine     │
                  └─────────────────────────┘

  TASK_RUNNING (0x00)
       ↑   ↓ (context switch)
  schedule()         schedule() returns
       │
       ├──────────────────────────────────────────────────┐
       │                                                  │
  sleep (mutex_lock, wait_event, ...)            wakeup conditions
       │                                                  │
       ▼                                                  │
  TASK_UNINTERRUPTIBLE (0x02)      ←──── Signal kills     │
  TASK_INTERRUPTIBLE   (0x01)      ←──── Signal wakes ──► └── try_to_wake_up()
  TASK_KILLABLE        (0x102)                                 sets TASK_RUNNING
       │                                                       bit by bit:
       ▼                                                       p->state & state_mask
  __TASK_STOPPED (0x04)    ← SIGSTOP                           (non-zero = match)
  __TASK_TRACED  (0x08)    ← ptrace

  void try_to_wake_up(task_t *p, unsigned int state) {
      // state is the bitmask of states that should be woken
      // if p->state matches any bit in 'state', wake the task
      if (p->state & state) {
          p->state = TASK_RUNNING;
          // ... put on run queue
      }
  }
```

### 18.2 Linux Network Stack — sk_buff Processing

```
sk_buff (socket buffer) lifecycle and flag propagation:

  NIC Hardware
       │
       │ DMA into sk_buff
       ▼
  ┌──────────────────────────────────────────────┐
  │ sk_buff                                      │
  │  .data_len                                   │
  │  .ip_summed = CHECKSUM_UNNECESSARY (HW done) │  ← bit field, 2 bits
  │  .vlan_present = 1                           │  ← single bit
  │  .protocol = ETH_P_IP                        │
  │  .pkt_type = PACKET_HOST                     │  ← 3-bit field
  └──────────────────────────────────────────────┘
       │
       ▼ netif_receive_skb()
  ┌─────────────────┐
  │ Protocol dispatch│  ← based on .protocol field (2 bytes from frame)
  │ ip_rcv()         │
  └─────────────────┘
       │
       ▼
  ip_rcv() checks flags:
    if (skb->ip_summed == CHECKSUM_UNNECESSARY) → skip checksum
    frag = iph->frag_off & htons(IP_MF | IP_OFFMASK)
    if (frag) → ip_defrag()   // packet is fragmented
       │
       ▼
  tcp_v4_rcv() checks:
    sk = tcp lookup (4-tuple hash)
    tcp_flags = tcp_hdr(skb)->th_flags
    if (tcp_flags & TCP_FLAG_RST) → tcp_reset()
    if (tcp_flags & TCP_FLAG_SYN) → tcp_conn_request()
    if (tcp_flags & TCP_FLAG_ACK) → tcp_ack()
```

### 18.3 CRC Computation (Network Checksums)

```c
/*
 * CRC-32 is the checksum in Ethernet FCS (Frame Check Sequence).
 * It uses XOR-based polynomial division — pure bit manipulation.
 *
 * The idea: treat data as a polynomial over GF(2) (binary field),
 * divide by a generator polynomial, remainder is the CRC.
 *
 * Generator polynomial for CRC-32: 0xEDB88320 (reflected form)
 */

uint32_t crc32_byte_at_a_time(const uint8_t *data, size_t len) {
    uint32_t crc = 0xFFFFFFFF;   /* init with all 1s */

    for (size_t i = 0; i < len; i++) {
        crc ^= data[i];           /* XOR in current byte (low 8 bits) */
        for (int bit = 0; bit < 8; bit++) {
            /* If LSB is 1, XOR with polynomial */
            if (crc & 1) {
                crc = (crc >> 1) ^ 0xEDB88320;
            } else {
                crc >>= 1;
            }
        }
    }

    return ~crc;   /* final XOR */
}

/*
 * How it works:
 * - Each bit of the polynomial division step is one iteration
 * - (crc & 1) tests if we need to XOR the generator polynomial
 * - (crc >> 1) shifts in the next bit
 * - The generator 0xEDB88320 is the reversed form of 0x04C11DB7
 *
 * In practice, a 256-entry lookup table pre-computes 8 iterations:
 * uint32_t crc32_table[256];
 * crc = (crc >> 8) ^ crc32_table[(crc ^ *data++) & 0xFF];
 *
 * Modern CPUs have the CRC32 instruction (SSE4.2):
 * _mm_crc32_u8(), _mm_crc32_u32(), _mm_crc32_u64()
 */
```

### 18.4 Hash Maps — Power-of-2 Sizing and Bit Indexing

```c
/*
 * High-performance hash tables use power-of-2 bucket counts
 * to replace expensive modulo with a single AND.
 */

struct HashMap {
    struct Entry **buckets;
    size_t mask;         /* = count - 1, where count is power of 2 */
    size_t count;
    size_t size;
};

/* Initialize with power-of-2 bucket count */
void hashmap_init(struct HashMap *m, size_t capacity) {
    /* Round up to next power of 2 */
    capacity = next_pow2_32((uint32_t)capacity);
    m->buckets = calloc(capacity, sizeof(*m->buckets));
    m->mask = capacity - 1;   /* the magic: 0b111...111 */
    m->count = capacity;
    m->size = 0;
}

/* Lookup — the AND replaces modulo */
struct Entry *hashmap_get(struct HashMap *m, uint64_t key) {
    uint64_t hash = hash_function(key);
    size_t   idx  = hash & m->mask;   /* FAST: replaces hash % m->count */
    return m->buckets[idx];
}

/*
 * Python's dict, Java's HashMap, Go's map — all use this trick.
 * The Linux kernel's hash tables (include/linux/hashtable.h) also
 * use power-of-2 sizes with & for bucket indexing.
 */
```

---

## Appendix: Quick Reference

### Bit Operation Cheat Sheet

```
Operation                          Expression
──────────────────────────────────────────────────────────────────
Set bit N in x                     x |= (1 << N)
Clear bit N in x                   x &= ~(1 << N)
Toggle bit N in x                  x ^= (1 << N)
Test bit N in x                    (x >> N) & 1  OR  !!(x & (1<<N))
Clear lowest set bit               x &= x - 1
Extract lowest set bit             x & (-x)   [= x & (~x+1)]
Isolate bit field [H:L]            (x >> L) & ((1 << (H-L+1)) - 1)
Insert val at bits [H:L]           x = (x & ~mask) | ((val << L) & mask)
Is power of 2?                     x && !(x & (x-1))
Round up to next power of 2        next_power_of_two(x)   (see above)
Align up to 2^n                    (x + (1<<n) - 1) & ~((1<<n) - 1)
Align down to 2^n                  x & ~((1<<n) - 1)
Unsigned multiply by 2^n          x << n
Unsigned divide by 2^n            x >> n  (fills with 0)
Signed floor-divide by 2^n        x >> n  (fills with sign bit)
Unsigned modulo 2^n               x & ((1 << n) - 1)
Absolute value (branchless)        mask=x>>31; (x+mask)^mask
Count set bits                     __builtin_popcount(x)
Highest set bit position           31 - __builtin_clz(x)
Lowest set bit position            __builtin_ctz(x)
Swap without temp                  a^=b; b^=a; a^=b;
Parity (even=0, odd=1)             x ^= x>>16; x ^= x>>8; x^=x>>4; x^=x>>2; x^=x>>1; x&1
Reverse bits in uint32_t          see reverse_bits() above
Byte swap (endian)                 __builtin_bswap32(x)
```

### Flag Operation Idioms

```c
/* Initialize */
uint32_t flags = 0;

/* Set */
flags |= FLAG_A;
flags |= FLAG_A | FLAG_B | FLAG_C;

/* Clear */
flags &= ~FLAG_A;
flags &= ~(FLAG_A | FLAG_B);

/* Toggle */
flags ^= FLAG_A;

/* Test one flag */
if (flags & FLAG_A) { ... }

/* Test all flags in set */
if ((flags & (FLAG_A | FLAG_B)) == (FLAG_A | FLAG_B)) { ... }

/* Test any flag in set */
if (flags & (FLAG_A | FLAG_B | FLAG_C)) { ... }

/* Test no flag in set */
if (!(flags & (FLAG_A | FLAG_B))) { ... }

/* Conditional set/clear */
flags = (flags & ~FLAG_A) | (condition ? FLAG_A : 0);

/* Replace all flags */
flags = FLAG_A | FLAG_C;

/* Save and restore */
uint32_t saved = flags;
flags &= ~FLAG_A;
/* ... do work ... */
flags = (flags & ~FLAG_A) | (saved & FLAG_A);
```

### Common Masks by Width

```
1-bit  mask:  0x1            (     1)
2-bit  mask:  0x3            (     3)
4-bit  mask:  0xF            (    15)  "nibble"
8-bit  mask:  0xFF           (   255)  "byte"
10-bit mask:  0x3FF          (  1023)
12-bit mask:  0xFFF          (  4095)
13-bit mask:  0x1FFF         (  8191)  (IPv4 fragment offset)
16-bit mask:  0xFFFF         ( 65535)  "word"
24-bit mask:  0xFFFFFF       (16M-1)
32-bit mask:  0xFFFFFFFF     (4G-1)    "dword"
64-bit mask:  0xFFFFFFFFFFFFFFFF        "qword"

Low nibble:   value & 0x0F
High nibble:  (value >> 4) & 0x0F
Low byte:     value & 0xFF
Byte N:       (value >> (8*N)) & 0xFF
```

---

*This guide covers the complete theory and practice of bit operations in systems programming. The mental model to internalize: every piece of state in a computer is ultimately bits, and bit operations are the most direct and efficient way to read, modify, and combine that state. Master these patterns and you'll read kernel source, network protocols, and hardware datasheets with fluency.*
