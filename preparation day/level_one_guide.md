# Systems Programming Interview Master Guide
## C · Go · Rust — Deep Internals, Memory Models, Algorithms

> A complete, production-grade reference for systems-level interviews at companies
> like Cloudflare, CrowdStrike, Palo Alto Networks, Google, Meta, and Isovalent.
> Every concept explained from the metal up.

---

# TABLE OF CONTENTS

```
PART I   — C MEMORY MODEL & FUNDAMENTALS
  Chapter 1   Process Memory Layout
  Chapter 2   Stack Mechanics & Stack Frames
  Chapter 3   Heap & Dynamic Memory Internals
  Chapter 4   Pointers — Complete Taxonomy
  Chapter 5   Arrays vs Pointers — Full Disambiguation
  Chapter 6   Strings in C — All Variants
  Chapter 7   Functions, Call Conventions, ABI
  Chapter 8   Structs, Unions, Enums — Memory Layout
  Chapter 9   Preprocessor & Compilation Pipeline
  Chapter 10  Type System, Qualifiers, Casts
  Chapter 11  Bits, Bytes, Endianness, Alignment
  Chapter 12  Undefined Behavior — Full Taxonomy
  Chapter 13  Output Prediction Traps

PART II  — GO RUNTIME & PATTERNS
  Chapter 14  Go Memory Model
  Chapter 15  GMP Scheduler Internals
  Chapter 16  Yielding & Cooperative Scheduling
  Chapter 17  Large File I/O — Streaming Patterns
  Chapter 18  Channels as Generators
  Chapter 19  Go Iterator Pattern (1.22+)
  Chapter 20  Go Type System Internals

PART III — RUST MEMORY SAFETY
  Chapter 21  Ownership & Borrowing — Mental Model
  Chapter 22  Lifetimes — Complete Guide
  Chapter 23  Unsafe Rust — When and How
  Chapter 24  Rust Equivalents to Every C Pattern

PART IV  — DATA STRUCTURES: INTERNALS & ASCII DIAGRAMS
  Chapter 25  Arrays & Slices
  Chapter 26  Linked Lists — Singly, Doubly, Circular
  Chapter 27  Hash Tables — Open Addressing & Chaining
  Chapter 28  Binary Trees & BST
  Chapter 29  Heaps
  Chapter 30  Stacks & Queues

PART V   — INTERVIEW Q&A: ALL 100 C QUESTIONS ANSWERED
```

---

# PART I: C MEMORY MODEL & FUNDAMENTALS

---

## Chapter 1: Process Memory Layout

Before you can answer any C memory question correctly, you need the complete mental
picture of what a running process looks like in virtual memory. Without this, everything
else is guesswork.

### 1.1 Virtual Address Space — Linux x86-64

```
High address (0xFFFFFFFFFFFFFFFF)
+---------------------------------------+
|          Kernel Space                 |  <-- kernel code, data, page tables
|       (mapped but not accessible      |      not directly accessible from userspace
|        from userspace)                |
+---------------------------------------+  0xFFFF800000000000
|          (non-canonical gap)          |
+---------------------------------------+  0x00007FFFFFFFFFFF
|          Stack                        |
|          (grows downward ↓)           |  <-- function call frames live here
|                                       |
|   [stack frame n]                     |
|   [stack frame n-1]                   |
|   [stack frame n-2]                   |
|   ...                                 |
|   [main() frame]                      |
+---------------------------------------+  stack grows toward lower addresses
|              ↕ gap (ASLR randomized)  |
+---------------------------------------+
|   Memory-Mapped Region                |  <-- mmap(), shared libs, file mappings
|   (shared libraries .so, mmap files)  |
+---------------------------------------+
|              ↕                        |
+---------------------------------------+
|   Heap                                |
|   (grows upward ↑)                    |  <-- malloc/calloc/realloc live here
|                                       |      managed by allocator (ptmalloc, jemalloc)
|   [allocated chunk]                   |
|   [free chunk]                        |
|   [allocated chunk]                   |
+---------------------------------------+  brk pointer (top of heap)
|   BSS Segment                         |  <-- zero-initialized globals
|   (uninitialized static data)         |      int g; static int x; (no initializer)
|                                       |      ALL ZEROED by OS before program starts
+---------------------------------------+
|   Data Segment                        |  <-- initialized globals and statics
|   (initialized static data)           |      int g = 42; static int x = 7;
|                                       |      stored in binary, copied to memory
+---------------------------------------+
|   Text Segment (Code)                 |  <-- machine instructions (read-only)
|   (read-only, executable)             |      string literals also live here
|                                       |      mapped directly from ELF binary
+---------------------------------------+  0x0000000000400000 (typical base)
|   NULL page (unmapped)                |  <-- dereferencing NULL causes SIGSEGV
+---------------------------------------+  0x0000000000000000
Low address
```

### 1.2 What Lives Where — The Critical Table

This table is the answer to half of all C memory interview questions:

```
Variable Declaration                   | Storage Location | Lifetime
---------------------------------------|-----------------|----------------------
int x = 5;           (global)          | Data segment    | Entire program
int x;               (global)          | BSS segment     | Entire program
static int x = 5;    (local static)    | Data segment    | Entire program
static int x;        (local static)    | BSS segment     | Entire program
int x = 5;           (local, inside fn)| Stack           | Function scope
int *p = malloc(n);  (any scope)       | Heap (the data) | Until free()
"hello"              (string literal)  | Text/rodata     | Entire program (read-only)
char s[] = "hello";  (local)           | Stack           | Function scope
const char *s = "hi";(local)           | ptr on stack,   | ptr: fn scope
                                       | data in rodata  | data: program
```

### 1.3 Key Insight: Why Returning Local Pointers Fails

```c
char *createMessage() {
    char buff[500] = "how are you";  // On STACK of createMessage
    char *ptr = buff;                 // ptr holds address of stack memory
    return ptr;                       // Returns address that will be INVALID
}
```

Step by step, with memory layout:

```
BEFORE createMessage() returns:

Stack (high to low address):
+------------------------+   <- stack pointer when main called createMessage
| main() locals          |
| msg = ???              |
+------------------------+
| return address to main |   <- so createMessage knows where to jump back
+------------------------+
| saved frame pointer    |
+------------------------+
| createMessage locals:  |
|   buff[0]  = 'h'       |   <- 0x7ffc1000
|   buff[1]  = 'o'       |   <- 0x7ffc1001
|   ...                  |
|   buff[11] = '\0'      |   <- 0x7ffc100b
|   buff[12..499] = 0    |
|   ptr = 0x7ffc1000     |   <- ptr points INTO this same frame
+------------------------+   <- stack pointer (current)

AFTER createMessage() returns:

Stack pointer moves BACK UP (toward higher address).
createMessage's frame is "popped" conceptually.

+------------------------+   <- stack pointer now here
| main() locals          |
| msg = 0x7ffc1000       |   <- still points to old location!
+------------------------+

That memory at 0x7ffc1000 is no longer "owned."
The next function call or OS interrupt can overwrite it.
```

This is why the behavior is undefined: the memory location still *exists* in the
hardware sense, the values may still be there (no one wiped them), but they can be
overwritten at any moment by anything the runtime does.

---

## Chapter 2: Stack Mechanics & Stack Frames

### 2.1 What a Stack Frame Contains

Every function call creates a stack frame (also called "activation record"):

```
One Stack Frame Layout (x86-64 System V ABI):

Higher address (previous frame)
+------------------------------------+
| Arguments beyond 6th              |   if function has >6 args, extras go on stack
+------------------------------------+
| Return address                    |   address to jump to when function returns
|   (pushed by CALL instruction)    |   (8 bytes on 64-bit)
+------------------------------------+
| Saved %rbp (base pointer)         |   previous frame's base, for stack walking
|   (pushed by function prologue)   |   (8 bytes)
+------------------------------------+  <- %rbp points here (frame pointer)
| Local variables                   |
|   [alignment padding if needed]   |
|   int x       (4 bytes)           |
|   char buf[N] (N bytes)           |
|   pointer p   (8 bytes)           |
|   ...                             |
+------------------------------------+  <- %rsp points here (stack pointer)
Lower address (next frame would go here)
```

The first 6 integer/pointer arguments in System V AMD64 ABI go in registers:
rdi, rsi, rdx, rcx, r8, r9 (in that order). Floating point uses xmm0-xmm7.

### 2.2 Stack Growth Direction — Why It Matters

The stack grows *downward* (toward lower addresses) on x86, ARM, MIPS, RISC-V.
This matters because:

```c
// Buffer overflow attacks exploit this direction
void vulnerable(char *input) {
    char buf[64];           // lives at lower addresses
    // return address       // lives at HIGHER address (above buf)
    strcpy(buf, input);     // if input > 64 bytes, overwrites return address!
}

Stack layout during vulnerable():

+--------------------+
| return address     |  <-- attacker wants to overwrite THIS
+--------------------+
| saved rbp          |
+--------------------+
| buf[63]            |  <- strcpy writes forward (buf[0] to buf[N])
| buf[62]            |     which means it writes TOWARD return address
| ...                |
| buf[1]             |
| buf[0]             |  <- buf starts here, &buf[0] is lowest address
+--------------------+
```

### 2.3 Stack Size Limits

```c
// Default stack size:
// Linux:   8 MB   (ulimit -s)
// macOS:   8 MB
// Windows: 1 MB

// Stack overflow = exceeding this limit
void infinite_recurse(void) {
    char big[65536];         // 64KB per frame
    infinite_recurse();      // ~128 recursions before SIGSEGV on 8MB stack
}

// Check: ulimit -s
// Change: ulimit -s unlimited (session-scoped)
// Programmatic: setrlimit(RLIMIT_STACK, &rl)
```

### 2.4 Assembly View of a Function Call

```asm
; C code: int add(int a, int b) { return a + b; }
; Caller:
;   int result = add(3, 4);

; Caller side (x86-64):
    mov    edi, 3          ; first argument in rdi
    mov    esi, 4          ; second argument in rsi
    call   add             ; push return address, jump to add
    ; result now in rax

; Callee (add function):
add:
    push   rbp             ; save caller's frame pointer
    mov    rbp, rsp        ; set our frame pointer
    ; rdi = 3, rsi = 4 (from registers)
    lea    eax, [rdi+rsi]  ; eax = 3 + 4 = 7
    pop    rbp             ; restore caller's frame pointer
    ret                    ; pop return address, jump to it
                           ; rax holds return value = 7
```

---

## Chapter 3: Heap & Dynamic Memory Internals

### 3.1 How malloc Actually Works

`malloc` is not a system call directly. It's a library function that manages a pool
of memory it gets from the OS via:
- `brk()`/`sbrk()` — extend the data segment (older, for small allocations)
- `mmap()` — map anonymous pages (for large allocations, typically > 128KB)

```
malloc(N) internals (glibc ptmalloc):

1. Check if N > MMAP_THRESHOLD (default 128KB):
   YES → use mmap() to get pages from OS directly
   NO  → check free list (bins) for a suitable free chunk

2. Free list structure (bins):
   fast bins:   [16][24][32][40][48][56][64] byte chunks (singly linked)
   small bins:  chunks < 512 bytes (doubly linked)
   large bins:  larger chunks (sorted by size)
   unsorted bin: recently freed chunks (checked first)

3. If no suitable chunk in free list:
   → Call brk() to extend the heap
   → Get a new chunk from the top of the heap
```

### 3.2 Heap Chunk Structure (ptmalloc/glibc)

```
Allocated chunk (glibc heap):

+-----------------------------+
| prev_size (8 bytes)         |  size of previous chunk IF it's free
|                             |  (otherwise used as user data by prev chunk)
+-----------------------------+
| size (8 bytes)              |  size of THIS chunk | flags
|  bit 0 = PREV_INUSE        |  bit 0: is previous chunk in use?
|  bit 1 = IS_MMAPPED        |  bit 1: was this obtained via mmap?
|  bit 2 = NON_MAIN_ARENA    |  bit 2: is this in a non-main arena?
+-----------------------------+  <- malloc() returns pointer HERE
| user data                   |
|   N bytes                   |
|   (padded to 16-byte align) |
+-----------------------------+
| (next chunk's prev_size)    |
```

```
Free chunk (glibc heap):

+-----------------------------+
| prev_size (8 bytes)         |
+-----------------------------+
| size + flags (8 bytes)      |
+-----------------------------+  <- if this were allocated, user data starts here
| fd (forward ptr, 8 bytes)  |  next free chunk in same bin
+-----------------------------+
| bk (backward ptr, 8 bytes) |  previous free chunk in same bin
+-----------------------------+
| fd_nextsize (large bins)    |  skip list pointer
+-----------------------------+
| bk_nextsize (large bins)    |
+-----------------------------+
| (unused space)              |
+-----------------------------+
```

### 3.3 malloc vs calloc vs realloc vs free

```c
// malloc(size): allocate 'size' bytes, UNINITIALIZED
void *malloc(size_t size);
// Memory contains garbage (whatever was there before).
// Danger: reading before writing gives undefined values.
int *arr = malloc(10 * sizeof(int));
// arr[0..9] contain garbage

// calloc(nmemb, size): allocate nmemb*size bytes, ZERO-INITIALIZED
void *calloc(size_t nmemb, size_t size);
// Guaranteed to be all zeros.
// Also checks for integer overflow in nmemb*size (malloc doesn't).
int *arr = calloc(10, sizeof(int));
// arr[0..9] are all 0

// realloc(ptr, newsize): resize allocation
void *realloc(void *ptr, size_t size);
// 3 cases:
// Case 1: new size > old size, space after chunk is free → expand in-place
// Case 2: new size > old size, no space → allocate new chunk, copy, free old
// Case 3: new size < old size → shrink (may or may not free the tail)
// CRITICAL: if realloc fails, it returns NULL and the ORIGINAL ptr is still valid
// WRONG:
int *arr = malloc(10 * sizeof(int));
arr = realloc(arr, 20 * sizeof(int));  // BUG: if realloc fails, arr = NULL, original lost
// CORRECT:
int *new_arr = realloc(arr, 20 * sizeof(int));
if (new_arr == NULL) { /* handle error, arr is still valid */ }
else arr = new_arr;

// free(ptr): release allocated memory
void free(void *ptr);
// Adds chunk back to free list.
// Does NOT zero the memory.
// Does NOT set ptr to NULL (you must do that yourself).
// Calling free(NULL) is defined and safe (it's a no-op).
```

### 3.4 Memory Errors Taxonomy

```
Memory Errors:

1. MEMORY LEAK
   malloc without free. Memory is consumed but never returned.
   The OS reclaims it when process exits, but during execution:
   - heap grows indefinitely
   - eventually: OOM → malloc returns NULL → crash
   
   int *p = malloc(100);
   // ... use p ...
   return;  // forgot free(p)

2. DOUBLE FREE
   free(ptr); free(ptr);
   - corrupts allocator's free list metadata
   - can crash immediately with "double free detected"
   - in older glibc: exploitable for heap attacks

3. USE AFTER FREE
   free(p); *p = 42;  // or: read = *p
   - memory may be reused by another malloc
   - reads return garbage, writes corrupt other data
   - can be silent or crash randomly

4. BUFFER OVERFLOW (heap)
   char *p = malloc(10);
   p[15] = 'x';  // overwrites adjacent chunk's header or data
   - corrupts allocator metadata or other allocations
   - crash happens much later, hard to debug

5. STACK OVERFLOW (not heap)
   - deep recursion
   - large local arrays: char buf[10000000] in a function

6. BUFFER UNDERFLOW
   p[-1] = 0;  // writing before the allocation
   - overwrites chunk header

7. UNINITIALIZED READ
   int *p = malloc(sizeof(int));
   int x = *p;  // reads garbage
```

### 3.5 C Implementation: Safe Allocator Wrappers

```c
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

// Always-succeeds malloc (crashes on OOM rather than silently proceeding)
void *xmalloc(size_t size) {
    void *p = malloc(size);
    if (p == NULL) {
        fprintf(stderr, "fatal: malloc(%zu) failed\n", size);
        abort();
    }
    return p;
}

// Safe realloc
void *xrealloc(void *ptr, size_t size) {
    void *p = realloc(ptr, size);
    if (p == NULL && size != 0) {
        fprintf(stderr, "fatal: realloc(%zu) failed\n", size);
        abort();
    }
    return p;
}

// Safe string duplicate
char *xstrdup(const char *s) {
    size_t len = strlen(s) + 1;
    char *dup = xmalloc(len);
    memcpy(dup, s, len);
    return dup;
}

// Safe free that nulls the pointer
#define safe_free(p)  do { free(p); (p) = NULL; } while (0)
```

### 3.6 Rust Equivalent — Ownership Prevents These Errors

```rust
// Rust's ownership system makes most of these errors compile-time impossible

fn create_message() -> String {
    let s = String::from("how are you"); // heap-allocated, owned by s
    s                                    // ownership transferred to caller
}   // s moved out, not dropped

fn main() {
    let msg = create_message();  // msg owns the String
    println!("{}", msg);
    println!("{}", msg.len());
}   // msg dropped here, memory freed automatically

// Contrast with C's dangling pointer:
// In Rust, you CANNOT return a reference to a local variable:
fn broken() -> &str {       // ERROR: missing lifetime specifier
    let s = String::from("hello");
    &s                       // COMPILE ERROR: s does not live long enough
}
```

### 3.7 Go Equivalent — Escape Analysis

```go
// Go uses escape analysis at compile time to decide stack vs heap
// Local variables that "escape" are automatically moved to heap

func createMessage() string {
    // Go strings are immutable, reference-counted-ish
    msg := "how are you"  // string literal, in read-only data
    return msg            // safe: returns a copy of string header (ptr+len)
}

// What about a local slice?
func createSlice() []int {
    s := make([]int, 10)  // compiler detects this escapes → heap allocated
    s[0] = 42
    return s              // safe: returns slice header (ptr+len+cap)
                          // underlying array on heap, won't be freed
}

// Go's garbage collector handles the rest
// No manual free needed
```

---

## Chapter 4: Pointers — Complete Taxonomy

### 4.1 What a Pointer Is

A pointer is a variable whose value is a memory address. That's it.
On 64-bit systems, every pointer is 8 bytes (regardless of what it points to).

```
int x = 42;
int *p = &x;

Memory layout:
+----------+         +----------+
|    42    |  <---   | 0x1000   |  p holds the address of x
| (x)      |         | (p)      |
| addr:    |         | addr:    |
| 0x1000   |         | 0x2000   |
+----------+         +----------+

p  = 0x1000    (the address stored in p, which is x's address)
*p = 42        (dereferencing: go to address 0x1000, read value there)
&p = 0x2000    (address of the pointer variable itself)
```

### 4.2 Pointer Taxonomy

#### Null Pointer
```c
int *p = NULL;   // NULL is ((void*)0) or just 0 in pointer context
// p == 0x0000000000000000

// The NULL page (address 0) is intentionally unmapped by the OS.
// Dereferencing NULL → SIGSEGV (segmentation fault, signal 11)
// This is a deliberate design: turns programmer errors into visible crashes.

// Checking:
if (p == NULL) { /* safe */ }
if (!p)        { /* same thing */ }
if (p != NULL) { *p = 42; } // safe to dereference
```

#### Dangling Pointer
```c
// Pointer that once pointed to valid memory, but that memory is now invalid.

// Case 1: pointing to freed heap memory
int *p = malloc(sizeof(int));
*p = 42;
free(p);
// p is now dangling — it still holds the old address
// but that memory may be reused by malloc for something else
*p = 99;  // USE AFTER FREE — undefined behavior

// Case 2: pointing to a stack variable after function returns
int *p = get_local_addr();  // returns &local_var
// p is now dangling

// Prevention:
free(p);
p = NULL;  // nullify after free — prevents accidental reuse
```

#### Wild Pointer
```c
// Uninitialized pointer — contains garbage address

int *p;         // declared but not initialized
// p contains whatever garbage was in that memory
*p = 42;        // writes to a random memory location — undefined behavior
// This is different from a null pointer (which is deliberately zero)
// and from a dangling pointer (which was once valid)

// Prevention:
int *p = NULL;  // always initialize pointers
```

#### Void Pointer
```c
// Generic pointer — can point to any type
// Cannot be directly dereferenced (no type information)
void *generic = malloc(100);  // malloc returns void*

// Must cast before dereferencing:
int *ip = (int *)generic;
*ip = 42;

// void* can hold any data pointer:
int x = 1; double d = 3.14; char c = 'A';
void *arr[3] = { &x, &d, &c };

// Common use: generic data structures, callbacks
typedef int (*compare_fn)(const void *, const void *);
void qsort(void *base, size_t n, size_t size, compare_fn cmp);
```

#### Pointer to Pointer
```c
// Used for:
// 1. Modifying a pointer from a function (pass by pointer-to-pointer)
// 2. Arrays of strings (char **argv)
// 3. Dynamic 2D arrays

int x = 42;
int *p = &x;
int **pp = &p;

**pp == 42   // true
*pp == p     // true
pp == &p     // true

// Command-line argument example:
int main(int argc, char **argv) {
    // argv[0] = program name
    // argv[1] = first argument
    // Each argv[i] is a char*, a pointer to a null-terminated string
}

// Memory layout of argv:
// argv[0]  argv[1]  argv[2]  argv[3]=NULL
//  |        |        |
//  v        v        v
// "prog"  "-v"    "file.txt"
// each is a char* pointing to a string in the OS-provided environment
```

#### Function Pointer
```c
// Pointer to executable code (a function)
// Type encodes the full signature

// Syntax: returntype (*name)(paramtypes)
int (*compare)(int, int);

// Example:
int max(int a, int b) { return a > b ? a : b; }
int min(int a, int b) { return a < b ? a : b; }

int (*op)(int, int) = max;
op(3, 5);  // calls max(3, 5) → 5

op = min;
op(3, 5);  // calls min(3, 5) → 3

// typedef makes it readable:
typedef int (*binary_op)(int, int);
binary_op ops[] = { max, min };
ops[0](3, 5);  // 5
ops[1](3, 5);  // 3

// Real use: callbacks, vtables, plugin systems
void sort(int *arr, size_t n, int (*cmp)(int, int)) {
    // use cmp to compare elements
}
```

### 4.3 Pointer Arithmetic

```c
int arr[5] = {10, 20, 30, 40, 50};
int *p = arr;       // p points to arr[0]

// Arithmetic scales by sizeof(*p) = sizeof(int) = 4 bytes
p + 1               // address of arr[1] = &arr[0] + 4 bytes
p + 2               // address of arr[2] = &arr[0] + 8 bytes

*(p + 0) == 10      // arr[0]
*(p + 1) == 20      // arr[1]
*(p + 2) == 30      // arr[2]

// These are IDENTICAL:
arr[i]  ==  *(arr + i)  ==  *(i + arr)  ==  i[arr]
// Yes, i[arr] is valid C (commutative addition). It's just a curiosity.

// Memory layout:
// arr[0]  arr[1]  arr[2]  arr[3]  arr[4]
// [10]    [20]    [30]    [40]    [50]
// 0x1000  0x1004  0x1008  0x100C  0x1010
//  ^p      ^p+1    ^p+2    ^p+3    ^p+4

// Pointer subtraction (gives element count between two pointers):
int *end = arr + 5;         // one past the end
ptrdiff_t count = end - p;  // = 5 (elements, not bytes)

// Pointer comparison:
p < end   // true (p is before end)

// INVALID arithmetic:
// 1. Out of bounds: p + 6 (beyond one-past-end) — UB
// 2. Arithmetic on void*: void* arithmetic — not standard C (GCC extension)
// 3. Arithmetic on function pointers — UB
```

---

## Chapter 5: Arrays vs Pointers — Complete Disambiguation

### 5.1 The Fundamental Difference

Arrays and pointers are NOT the same thing. They are related by a single rule (array
decay), but they are fundamentally different types with different sizes and semantics.

```c
int arr[5] = {1,2,3,4,5};
int *ptr = arr;

// sizeof:
sizeof(arr)   // 20 bytes (5 * sizeof(int) = 5 * 4)
sizeof(ptr)   // 8 bytes (size of a pointer on 64-bit)

// &:
&arr          // type: int (*)[5], address of the WHOLE ARRAY
&ptr          // type: int **, address of the pointer variable

// arr is NOT a variable. It is the array itself.
// arr decays to &arr[0] in most expressions.
// But sizeof(arr) and &arr do NOT cause decay — they operate on the array itself.
```

### 5.2 Array Decay — The Rule

```c
// In most contexts, an array name decays to a pointer to its first element.
// The pointer has the type: pointer to element type.

int arr[5];
int *p = arr;    // decay: arr → &arr[0], type int*

// Decay happens when:
// 1. Passing to a function
// 2. In an expression (except sizeof, &, _Alignof)
// 3. In an initializer like int *p = arr

// Does NOT happen with:
// sizeof(arr) → gives full array size
// &arr        → gives pointer to array (type int(*)[5])

// This means:
void process(int arr[], int n) { /* arr is actually int* here */ }
void process(int *arr, int n)  { /* identical to above */ }

// These are the same function signature.
// sizeof(arr) inside process() gives 8 (pointer size), NOT 20.
```

### 5.3 Pointer to Array vs Array of Pointers

```c
int arr[5] = {1,2,3,4,5};

// Pointer to an array of 5 ints:
int (*ptr_to_arr)[5] = &arr;

(*ptr_to_arr)[0]   // = 1  (dereference to get array, then index)
ptr_to_arr[0][0]   // = 1  (same, indexing syntax)

// Array of 5 pointers to int:
int x=1, y=2, z=3, w=4, v=5;
int *arr_of_ptrs[5] = { &x, &y, &z, &w, &v };

arr_of_ptrs[0]    // = &x   (pointer to int)
*arr_of_ptrs[0]   // = 1    (dereferenced)
```

### 5.4 Multi-Dimensional Arrays in Memory

Multi-dimensional arrays in C are stored in row-major order (row by row):

```c
int matrix[3][4] = {
    {1,  2,  3,  4},
    {5,  6,  7,  8},
    {9, 10, 11, 12}
};

// Memory layout (contiguous):
// [1][2][3][4][5][6][7][8][9][10][11][12]
//  row0         row1         row2

// Indexing:
matrix[r][c]    ==  *(*(matrix + r) + c)

// matrix + r: moves r rows forward (r * 4 * sizeof(int) bytes)
// *(matrix + r): the r-th row (an array of 4 ints)
// *(matrix + r) + c: pointer to c-th element in r-th row

// Passing to a function:
void process(int matrix[][4], int rows);     // must specify all but first dim
void process(int (*matrix)[4], int rows);    // equivalent

// Dynamic 2D array (heap, truly contiguous):
int (*m)[4] = malloc(3 * sizeof(*m));  // 3 rows, 4 cols
m[1][2] = 7;
free(m);
```

---

## Chapter 6: Strings in C — All Variants

### 6.1 String Literal vs Character Array

```c
// STRING LITERAL
const char *s1 = "hello";
// Stored in read-only data section (text/rodata segment)
// Lives for the entire program
// s1 is a pointer ON THE STACK pointing to read-only memory
// s1 CAN be reassigned to point elsewhere
// *s1 = 'H';  // UNDEFINED BEHAVIOR — modifying read-only memory
//              // on most systems: SIGSEGV or just silently ignored

// CHARACTER ARRAY (stack)
char s2[] = "hello";
// Copies the string literal INTO a stack-allocated array
// s2 occupies 6 bytes on the stack (5 chars + null terminator)
// s2 is MODIFIABLE
s2[0] = 'H';  // OK, s2 is on the stack
// s2 cannot be reassigned (it's an array, not a pointer)

// Memory layout:
// s1:                          s2 (stack):
// [ptr → 0xRODATA]             ['h']['e']['l']['l']['o']['\0']
//         ↓
// rodata: ['h']['e']['l']['l']['o']['\0']  (shared, if duplicate literals)
```

### 6.2 String Functions — What They Actually Do

```c
// strlen(s): count chars until '\0', returns count (excludes '\0')
strlen("hello")   // = 5
strlen("")        // = 0
// Does NOT include the null terminator.
// Buffer must be AT LEAST strlen + 1 bytes.

// sizeof("hello") = 6 (includes '\0')
// strlen("hello") = 5 (excludes '\0')

// strcpy(dst, src): copies src into dst including '\0'
// DANGEROUS: no bounds check. dst must be large enough for all of src.
char dst[4];
strcpy(dst, "hello");  // BUFFER OVERFLOW — writes 6 bytes into 4-byte buffer

// strncpy(dst, src, n): copies at most n bytes
// TRAP: if src has >= n chars, '\0' is NOT appended!
char dst[5];
strncpy(dst, "hello", 5);  // copies 'h','e','l','l','o' — NO NULL TERMINATOR
// dst is not a valid C string!

// CORRECT pattern:
strncpy(dst, src, sizeof(dst) - 1);
dst[sizeof(dst) - 1] = '\0';  // force-terminate

// snprintf: safe string formatting (always null-terminates)
char buf[64];
snprintf(buf, sizeof(buf), "value: %d", 42);  // safe, always '\0'-terminated
// Returns number of chars that WOULD have been written (excluding '\0')
// If return value >= sizeof(buf), output was truncated

// strcmp(a, b): lexicographic comparison
// Returns: 0 if equal, <0 if a < b, >0 if a > b
// Common interview trap:
if (strcmp(s1, s2))  // TRUE if they are NOT equal (non-zero = different)
if (!strcmp(s1, s2)) // TRUE if they ARE equal
```

### 6.3 printf Format String Vulnerability

```c
char *user_input = get_user_input();

// DANGEROUS:
printf(user_input);   // if user inputs "%x %x %x", printf reads stack memory!

// SAFE:
printf("%s", user_input);  // user_input is data, not format string
```

### 6.4 Rust: String Types

```rust
// Rust has two main string types:

// &str: string slice — borrowed reference to UTF-8 bytes
//   - can point to string literal (static lifetime)
//   - can point into a String's data
//   - does not own its data
let s1: &str = "hello";   // &'static str, points to rodata

// String: owned, heap-allocated, growable UTF-8 string
let s2: String = String::from("hello");
let s3: String = "hello".to_owned();

// Conversion:
let slice: &str = &s2;     // &String coerces to &str (Deref)
let owned: String = slice.to_owned();

// Unlike C:
// - Strings are always valid UTF-8 (enforced at type level)
// - No null terminator (length stored explicitly)
// - No buffer overflow (bounds checked)
// - No format string vulnerabilities (format checked at compile time)
```

### 6.5 Go: String Internals

```go
// Go string is a read-only slice of bytes (UTF-8 by convention)
// Internal structure:
// type stringHeader struct {
//     Data unsafe.Pointer  // pointer to bytes
//     Len  int             // length in bytes (not runes)
// }

s := "hello"
// s.Data → read-only memory
// s.Len  → 5

// String is immutable — all "modifications" create new strings
s2 := s + " world"   // new allocation, s unchanged

// Byte vs rune:
for i, b := range []byte(s) { _ = b }   // iterate bytes
for i, r := range s           { _ = r }  // iterate runes (Unicode code points)

// C-style null-terminated string for syscall interop:
import "unsafe"
cs := C.CString("hello")   // allocates, caller must free with C.free(cs)
defer C.free(unsafe.Pointer(cs))
```

---

## Chapter 7: Functions, Call Conventions, ABI

### 7.1 Pass-by-Value in C

```c
// C is STRICTLY pass-by-value. Always.
// When you pass anything to a function, a copy is made.

void increment(int x) {
    x++;   // modifies LOCAL copy, not original
}

int main(void) {
    int a = 5;
    increment(a);
    printf("%d\n", a);  // still 5!
}

// Simulating pass-by-reference: pass a POINTER (the pointer's VALUE is copied)
void increment_real(int *p) {
    (*p)++;   // dereferences to modify original
}
int main(void) {
    int a = 5;
    increment_real(&a);  // pass address of a
    printf("%d\n", a);  // now 6
}

// Struct pass-by-value: entire struct is copied
typedef struct { int x, y; float z; } Point;
void translate(Point p, int dx) { p.x += dx; }  // copies Point, original unchanged
void translate_ref(Point *p, int dx) { p->x += dx; }  // modifies original
```

### 7.2 Return Multiple Values in C

```c
// C functions return one value.
// Workarounds:

// 1. Return a struct
typedef struct { int quotient, remainder; } DivResult;
DivResult divide(int a, int b) {
    return (DivResult){ a / b, a % b };
}
DivResult r = divide(17, 5);
r.quotient;   // 3
r.remainder;  // 2

// 2. Output parameters (pointers)
void divide2(int a, int b, int *quot, int *rem) {
    *quot = a / b;
    *rem  = a % b;
}
int q, r;
divide2(17, 5, &q, &r);

// 3. Array output
void minmax(int *arr, int n, int *min, int *max);
```

### 7.3 Recursive Stack Usage

```c
// Each recursive call adds a new frame to the stack.
// Deep recursion can overflow the stack.

int factorial(int n) {
    if (n <= 1) return 1;
    return n * factorial(n - 1);
}

// Stack frames for factorial(4):
// factorial(4)    [frame 0]
//   factorial(3)  [frame 1]
//     factorial(2)[frame 2]
//       factorial(1) [frame 3]
//       returns 1
//     returns 2*1 = 2
//   returns 3*2 = 6
// returns 4*6 = 24

// Each frame contains: n, return address, saved rbp
// On Linux x86-64: each frame ≈ 32-64 bytes minimum
// factorial(100000) → ~100000 * 64 = 6.4 MB stack → likely overflow on 8MB stack

// Tail recursion (some compilers optimize this):
int factorial_tail(int n, int acc) {
    if (n <= 1) return acc;
    return factorial_tail(n - 1, n * acc);  // tail call — last operation
}
// gcc -O2: converts to a loop, O(1) stack space
```

---

## Chapter 8: Structs, Unions, Enums — Memory Layout

### 8.1 Structure Padding and Alignment

The CPU has alignment requirements. Accessing a 4-byte int from an odd address
(say 0x1003) requires the CPU to do multiple memory bus cycles, or on some
architectures causes a bus error/trap. To avoid this, the compiler inserts padding.

**The Rule:** Each member is aligned to a multiple of its own size.
**The Struct Rule:** The struct's total size is a multiple of its largest member.

```c
struct Padded {
    char  a;    // 1 byte at offset 0
                // 3 bytes padding (next member needs 4-byte alignment)
    int   b;    // 4 bytes at offset 4
    char  c;    // 1 byte at offset 8
                // 7 bytes padding (struct size must be multiple of 8 for double)
    double d;   // 8 bytes at offset 16
};
// sizeof(Padded) = 24

// Memory layout:
// offset: 0   1   2   3   4   5   6   7   8   9  10  11  12  13  14  15  16..23
//         [a] [pad pad pad][   b (4 bytes)    ][c] [pad pad pad pad pad pad][d (8 bytes)]

struct Packed_Optimal {
    double d;   // 8 bytes at offset 0
    int    b;   // 4 bytes at offset 8
    char   a;   // 1 byte  at offset 12
    char   c;   // 1 byte  at offset 13
                // 2 bytes padding (size must be multiple of 8)
};
// sizeof(Packed_Optimal) = 16

// Technique: sort members from largest to smallest to minimize padding.
```

```
Padded struct layout:
+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+
| a  | P  | P  | P  |         b (int)       | c  | P  | P  | P  | P  | P  | P  |
+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+
|<-1->|<---3 pad--->|<-------4 bytes------->|<-1->|<-----------6 pad---------->|
                    ^offset 4               ^offset 8

+------------------+------------------+
|          d (double, 8 bytes)         |
+------------------+------------------+
^offset 16
Total: 24 bytes
```

### 8.2 Forced Packing

```c
// __attribute__((packed)): remove all padding (GCC/Clang)
struct __attribute__((packed)) NetworkHeader {
    uint8_t  type;
    uint16_t length;
    uint32_t sequence;
};
// sizeof = 7 (1+2+4), no padding
// Useful for binary protocols, file formats, network packets
// Cost: misaligned access is slower, may be illegal on some RISC architectures

// Pragmatic packing:
#pragma pack(push, 1)
struct PackedStruct {
    char a;
    int  b;
};
#pragma pack(pop)
```

### 8.3 Union — Overlapping Memory

A union allocates enough memory for its *largest* member. All members share the
same starting address. Writing one member and reading another is type punning.

```c
union Value {
    int   i;
    float f;
    char  bytes[4];
};

// Memory layout:
// All members start at offset 0
// +--------+--------+--------+--------+
// | byte 0 | byte 1 | byte 2 | byte 3 |
// +--------+--------+--------+--------+
// ^--- i (4 bytes int)    starts here
// ^--- f (4 bytes float)  starts here
// ^--- bytes[0..3]        starts here
// sizeof(union Value) = 4

union Value v;
v.i = 0x3f800000;   // IEEE 754 representation of float 1.0
v.f;                 // = 1.0f  (reading the same bytes as a float)
v.bytes[3];          // = 0x3f  (high byte of the int)

// Real use cases:
// 1. Type punning (inspect float bits):
union FloatBits { float f; uint32_t u; };
union FloatBits fb = { .f = 3.14f };
printf("%08x\n", fb.u);  // prints hex representation of 3.14f's bits

// 2. Tagged union (discriminated union):
typedef enum { INT, FLOAT, STRING } Tag;
typedef struct {
    Tag type;
    union {
        int    i;
        float  f;
        char  *s;
    } data;
} Variant;
```

### 8.4 Flexible Array Member

```c
// The last member can be an unsized array — useful for variable-length structs
struct Message {
    uint32_t len;
    char     data[];   // flexible array member
};

// Allocate:
struct Message *m = malloc(sizeof(struct Message) + len);
m->len = len;
memcpy(m->data, payload, len);

// sizeof(struct Message) = 4  (only counts the len field)
// m->data is right after len in memory (no extra pointer needed)
```

### 8.5 Enum Internals

```c
// enum values are integers, typically int (implementation-defined)
enum Color { RED, GREEN, BLUE };
// RED = 0, GREEN = 1, BLUE = 2  (default: starts at 0, increments by 1)

enum Status { OK = 200, NOT_FOUND = 404, SERVER_ERROR = 500 };
// Can assign arbitrary integer values

// sizeof(enum Color) typically = sizeof(int) = 4
// But the C standard says "implementation-defined"

// Enums are NOT type-safe in C:
enum Color c = 42;   // no compile error! (42 is not RED, GREEN, or BLUE)

// In C++: scoped enums (enum class) fix this
// In Rust: enums are fully type-safe and can carry data
```

---

## Chapter 9: Preprocessor & Compilation Pipeline

### 9.1 The Four Stages of Compilation

```
Source file: main.c
                |
                v
   +--------------------------+
   | Stage 1: PREPROCESSING   |
   | Tool: cpp (C preprocessor)|
   |                          |
   | - Handles #include        |  → paste header contents literally
   | - Handles #define         |  → textual substitution
   | - Handles #ifdef/#ifndef  |  → conditional compilation
   | - Removes comments        |
   | - Processes #pragma       |
   +--------------------------+
                |
                v  (output: preprocessed C, .i file)
   +--------------------------+
   | Stage 2: COMPILATION      |
   | Tool: cc1 (compiler proper)|
   |                          |
   | - Lexing: tokens          |
   | - Parsing: AST            |
   | - Semantic analysis       |
   | - Optimization            |
   | - Code generation         |
   +--------------------------+
                |
                v  (output: assembly, .s file)
   +--------------------------+
   | Stage 3: ASSEMBLY         |
   | Tool: as (GNU assembler)  |
   |                          |
   | - Convert asm → machine   |
   |   code (object file)      |
   +--------------------------+
                |
                v  (output: object file, .o, ELF format)
   +--------------------------+
   | Stage 4: LINKING          |
   | Tool: ld (linker)         |
   |                          |
   | - Combine .o files        |
   | - Resolve external symbols|
   | - Link standard libraries |
   | - Produce executable      |
   +--------------------------+
                |
                v  (output: executable, ELF binary)
```

### 9.2 #include: System vs Local Headers

```c
#include <stdio.h>    // angle brackets: search system include paths
                      // e.g. /usr/include/, /usr/local/include/
                      // compiler's built-in include directories

#include "myheader.h" // quotes: search current directory FIRST,
                      // then fall through to system paths

// Header guards prevent double-inclusion:
#ifndef MYHEADER_H
#define MYHEADER_H

// declarations here...

#endif // MYHEADER_H

// Modern alternative:
#pragma once  // non-standard but widely supported, cleaner
```

### 9.3 Macros — Full Deep Dive

```c
// Object-like macro (constant replacement):
#define PI 3.14159265358979
#define MAX_SIZE 1024

// After preprocessing:
// PI becomes 3.14159265358979 (textual substitution)
// It has no type! Prefer: const double PI = 3.14159...;

// Function-like macro (with parameters):
#define SQUARE(x) x * x

// TRAP: operator precedence
int r = SQUARE(2 + 3);
// After substitution: 2 + 3 * 2 + 3 = 2 + 6 + 3 = 11  WRONG!
// Expected: (2+3)^2 = 25

// FIX: always wrap in parentheses:
#define SQUARE(x) ((x) * (x))
// After substitution: ((2 + 3) * (2 + 3)) = 25 ✓

// TRAP 2: side effects evaluated multiple times
#define MAX(a, b) ((a) > (b) ? (a) : (b))
int x = 5;
int r = MAX(x++, 3);
// Expands to: ((x++) > (3) ? (x++) : (3))
// x++ evaluated TWICE if x > 3! x becomes 7, not 6.

// FIX: use inline functions (evaluated once):
static inline int max(int a, int b) { return a > b ? a : b; }

// Stringification:
#define STRINGIFY(x) #x
STRINGIFY(hello)  → "hello"
STRINGIFY(42)     → "42"

// Token pasting:
#define CONCAT(a, b) a##b
CONCAT(var, 1)    → var1 (creates identifier var1)
```

### 9.4 extern vs static

```c
// extern: the symbol is defined ELSEWHERE (another translation unit)
// Used in headers to declare without defining
extern int global_counter;   // declaration only, no storage allocated
extern void process(void);   // function declaration

// static at file scope: INTERNAL linkage
// The symbol is only visible within THIS translation unit
static int internal_state = 0;    // not accessible from other .c files
static void helper(void) { ... }  // not exported, no symbol in object file

// static at function scope: persistent storage across calls
void count_calls(void) {
    static int count = 0;  // initialized ONCE, persists between calls
    count++;
    printf("Called %d times\n", count);
}
// count lives in the BSS/data segment, not the stack

// Difference from global without extern:
int global_var = 5;    // defined here, external linkage (accessible from other TUs)
extern int other_var;  // declared here, defined in another .c file
```

---

## Chapter 10: Type System, Qualifiers, Casts

### 10.1 const — What It Actually Means

```c
// const means "this identifier cannot be used to modify the value"
// It does NOT mean the value is truly immutable in memory.

const int x = 5;
// x = 10;  // compile error

// Pointer to const int: you can't modify the int through this pointer
const int *p = &x;
// *p = 10;  // compile error
p = &y;     // OK: p itself can be reassigned

// Const pointer to int: the pointer itself can't be changed
int *const cp = &x;
*cp = 10;   // OK: can modify through the pointer
// cp = &y; // compile error: can't reassign cp

// Const pointer to const int:
const int *const ccp = &x;
// *ccp = 10; // error
// ccp = &y;  // error

// Rule: "read right to left"
// const int *p    → p is a [pointer to] [const int]
// int *const p    → p is a [const pointer] [to int]
// const int *const p → p is a [const pointer] [to const int]

// Casting away const (UB if original was actually const):
int *writable = (int *)p;  // removes const, technically UB if x was truly const
```

### 10.2 volatile — Hardware Register Access

```c
// volatile tells the compiler: this variable can change outside normal program flow.
// The compiler must read/write it on EVERY access — no caching in registers.

// Without volatile:
int status = 0;
while (status == 0) { }  // compiler may optimize to: if (status == 0) while(1) {}

// With volatile:
volatile int status = 0;
while (status == 0) { }  // compiler re-reads status from memory every iteration

// Use cases:
// 1. Memory-mapped I/O (hardware registers)
volatile uint32_t *const gpio_output = (volatile uint32_t *)0x40020014;
*gpio_output = 0x01;    // MUST write to hardware, not optimized away

// 2. Variables modified by signal handlers
volatile sig_atomic_t signal_received = 0;

// 3. Variables shared between ISR and main code (bare-metal, single-core)
// For multi-core/threads: volatile is NOT sufficient — use atomics or mutexes

// volatile + const: read-only hardware register
const volatile uint32_t *const timer = (const volatile uint32_t *)0x40000000;
```

### 10.3 Signed vs Unsigned Integer Behavior

```c
// Signed overflow: UNDEFINED BEHAVIOR in C
int x = INT_MAX;   // 2147483647
int y = x + 1;     // UNDEFINED BEHAVIOR
// gcc -O2 may assume this never happens and optimize accordingly

// Unsigned overflow: WELL DEFINED — wraps modulo 2^N
unsigned int u = UINT_MAX;  // 4294967295
unsigned int v = u + 1;     // = 0 (wraps around, well-defined)

// Integer promotion:
// In expressions, types smaller than int are promoted to int
char c = 250;           // on systems where char is signed: this is -6 (overflow)
                        // on systems where char is unsigned: this is 250
unsigned int u = 1;
int i = -2;
if (u + i > 0)          // u+i: i is converted to unsigned!
                        // -2 as unsigned int = 4294967294
                        // 1 + 4294967294 = 4294967295 (wraps in uint)
                        // 4294967295 > 0 is TRUE, even though logically -1

// This is a CLASSIC BUG:
size_t n = get_count();
if (n - 1 >= 0)  // ALWAYS TRUE if n is size_t (unsigned), even if n = 0!
                 // 0 - 1 = SIZE_MAX (wraps), which is > 0

// Safe comparison:
if (n > 0 && n - 1 >= 0)  // still wrong — second condition always true
if (n > 1)                 // correct way to check if n >= 2
```

### 10.4 Strict Aliasing

```c
// The strict aliasing rule: the compiler may assume that pointers of different
// types do NOT alias each other (point to the same memory).
// This allows optimizations that re-order reads/writes.

// VIOLATION (UB):
int x = 42;
float *fp = (float *)&x;  // alias int as float
float val = *fp;           // UNDEFINED BEHAVIOR under strict aliasing

// The compiler may cache x's value in a register and never re-read from memory
// even though fp writes to the same location.

// LEGAL EXCEPTIONS:
// 1. char* can alias anything (special rule for char, signed char, unsigned char)
char *cp = (char *)&x;   // legal — char* can alias any type
// This is why memcpy uses char* internally

// 2. Type punning via union (C99+):
union { int i; float f; } pun;
pun.i = 0x3f800000;
float val = pun.f;  // legal in C (reading union member other than last written)

// 3. Disable strict aliasing: -fno-strict-aliasing (GCC flag)
// Needed when working with raw memory, network packet parsing, etc.
```

---

## Chapter 11: Bits, Bytes, Endianness, Alignment

### 11.1 Bitwise Operations

```c
// These operate on the binary representation, bit by bit.

unsigned int a = 0b10110101;  // 181
unsigned int b = 0b11001010;  // 202

// AND: 1 only where both are 1
a & b   // 10000000 = 128
// Use: masking (extract specific bits)
// Example: extract lower nibble
uint8_t nibble = x & 0x0F;   // isolate lower 4 bits

// OR: 1 where either is 1
a | b   // 11111111 = 255
// Use: setting specific bits
x |= (1 << n);  // set bit n

// XOR: 1 where they differ
a ^ b   // 01111111 = 127
// Use: toggle bits, check if two values differ, simple encrypt
x ^= (1 << n);   // toggle bit n
x ^ x == 0;      // always (x XOR itself = 0) — classic swap trick

// NOT (bitwise complement): flip all bits
~a      // 01001010 = 74  (for uint8_t)
// NOT + 1 = two's complement negation: ~x + 1 == -x for signed integers

// Left shift: x << n  = multiply by 2^n (for non-negative values)
1 << 3   // = 8
a << 2   // = 0b1011010100 (shifted 2 left)
// Overflow: shifting into or past sign bit of signed int → UB

// Right shift: x >> n  = divide by 2^n
// For UNSIGNED: logical shift (fills with 0)
// For SIGNED: arithmetic shift (fills with sign bit, implementation-defined)
// Safe: only right-shift unsigned values, or known non-negative signed

// Common bit tricks:
x & (x-1)       // clears lowest set bit of x (if x != 0)
x & (-x)        // isolates lowest set bit
!( x & (x-1))   // true if x is a power of 2
__builtin_popcount(x)  // count of set bits (GCC)
__builtin_ctz(x)       // count trailing zeros
__builtin_clz(x)       // count leading zeros
```

### 11.2 Endianness

```
How a multi-byte value is stored in memory:

Value: 0x12345678 (4-byte integer)

LITTLE-ENDIAN (x86, x86-64, ARM, RISC-V by default):
  Address: N     N+1   N+2   N+3
           [78]  [56]  [34]  [12]
  Least significant byte at lowest address.
  "Little end first."
  
BIG-ENDIAN (MIPS historically, SPARC, network byte order):
  Address: N     N+1   N+2   N+3
           [12]  [34]  [56]  [78]
  Most significant byte at lowest address.
  "Big end first."
  Network protocols use big-endian (called "network byte order").
```

```c
// Detecting endianness at runtime:
int is_little_endian(void) {
    uint32_t x = 1;
    return *(uint8_t *)&x == 1;
    // If little-endian: byte at lowest address is 0x01 (least significant)
    // If big-endian:    byte at lowest address is 0x00
}

// Or using union:
int is_little_endian_union(void) {
    union { uint32_t i; uint8_t c[4]; } u = { .i = 1 };
    return u.c[0] == 1;
}

// Network byte order conversion (always big-endian):
#include <arpa/inet.h>
uint32_t host = 0x12345678;
uint32_t net  = htonl(host);   // host-to-network-long
uint32_t back = ntohl(net);    // network-to-host-long
// htons/ntohs: 16-bit versions

// Manual byte swap:
uint32_t bswap32(uint32_t x) {
    return ((x & 0xFF000000) >> 24) |
           ((x & 0x00FF0000) >>  8) |
           ((x & 0x0000FF00) <<  8) |
           ((x & 0x000000FF) << 24);
}
// Or: __builtin_bswap32(x) (GCC)
```

### 11.3 Alignment

```c
// Alignment: requirement that an N-byte type starts at an address divisible by N

// Type alignment requirements (typical, x86-64):
// char:     1 byte (no alignment requirement)
// short:    2 bytes (must be at even address)
// int:      4 bytes (address divisible by 4)
// long:     8 bytes (address divisible by 8)
// double:   8 bytes (address divisible by 8)
// pointer:  8 bytes (address divisible by 8)

// Check alignment: _Alignof (C11)
_Alignof(int)     // = 4
_Alignof(double)  // = 8

// Specify alignment: _Alignas (C11)
_Alignas(64) char cache_line_buffer[64];   // aligned to 64-byte cache line

// Misalignment effects:
// x86: handles misaligned access (hardware fixes it, slower)
// ARM: typically allows misaligned, may be slow or disabled
// SPARC/MIPS: trap/bus error on misaligned access

// Practical: why it matters for SIMD/vectorization
// SSE instructions require 16-byte alignment
// AVX requires 32-byte alignment
void *p = aligned_alloc(64, 4096);  // 64-byte aligned allocation
```

---

## Chapter 12: Undefined Behavior — Full Taxonomy

UB is where the C standard says "anything can happen." The compiler is free to
assume UB never occurs, and will optimize accordingly — often making the bug
*worse*, not better.

```c
// CATEGORY 1: Signed integer overflow
int x = INT_MAX;
int y = x + 1;          // UB — compiler may assume this never happens
                         // -O2 may REMOVE overflow checks that come after

// CATEGORY 2: Array out of bounds
int arr[5];
arr[5] = 42;             // UB — writes past end of array
arr[-1] = 42;            // UB — writes before start

// CATEGORY 3: Null pointer dereference
int *p = NULL;
*p = 42;                 // UB — always crashes in practice (SIGSEGV)

// CATEGORY 4: Use after free
int *p = malloc(4);
free(p);
*p = 42;                 // UB — reads/writes freed memory

// CATEGORY 5: Double free
free(p);
free(p);                 // UB — corrupts allocator metadata

// CATEGORY 6: Uninitialized variables
int x;
int y = x + 1;           // UB — x has indeterminate value

// CATEGORY 7: Dangling pointer dereference
int *p = get_stack_address();   // returns &local
*p = 42;                         // UB — accessing dead stack frame

// CATEGORY 8: Modifying string literals
char *s = "hello";
s[0] = 'H';              // UB — string literals are read-only

// CATEGORY 9: Modifying same variable twice between sequence points
int i = 0;
i = i++ + 1;             // UB — i modified twice (by ++ and by =) without sequence point
// Also: f(i++, i++);    // UB — order of evaluation of arguments is unspecified

// CATEGORY 10: Integer division by zero
int x = 5 / 0;           // UB for integers (defined behavior for floating point: inf/nan)

// CATEGORY 11: Calling through wrong function pointer type
void (*fp)(int) = (void (*)(int))some_other_fn;
fp(42);                   // UB if types don't match

// CATEGORY 12: Reading inactive union member (C strict aliasing)
// (see Chapter 10)

// CATEGORY 13: Shifting by negative or >= bit width
int x = 1;
int y = x << 32;          // UB — shifting 32-bit int by 32
int z = x << -1;          // UB — shifting by negative

// WHY "IT WORKS":
// Compilers don't insert code to "make it fail" on UB.
// Memory isn't erased. The old values may still be there.
// The program may appear to work until an optimization changes the code.
// AddressSanitizer (ASan) and UBSanitizer (UBSan) can detect these at runtime.
```

---

## Chapter 13: Output Prediction Traps

### 13.1 printf Format Mismatches

```c
// Format specifier mismatch → UB (and practically: garbage values)
printf("%d\n", 3.14);   // 3.14 is double, %d expects int → garbage
printf("%f\n", 42);     // 42 is int, %f expects double → garbage

// %c prints a character from its ASCII value:
printf("%c\n", 65);   // prints: A   (ASCII 65 = 'A')
printf("%d\n", 'A');  // prints: 65  (char promoted to int, printed as decimal)

// sizeof returns size_t (unsigned):
printf("%d\n", sizeof(int));   // UB — should use %zu (size_t)
printf("%zu\n", sizeof(int));  // correct
```

### 13.2 Sequence Points and Evaluation Order

```c
// Sequence points separate evaluations with defined ordering.
// Between sequence points, order of evaluation is unspecified.

// Function call arguments: order is UNSPECIFIED (not UB, just unspecified)
int i = 0;
printf("%d %d\n", i++, i++);
// Could print: 0 1, 1 0, or 0 0 depending on compiler
// (But having side effects on same variable is UB, not just unspecified)

// The i++ expression: evaluates to current value, THEN increments i
// ++i expression: increments i FIRST, then evaluates to new value
int i = 5;
printf("%d\n", i++);  // prints 5, then i becomes 6
printf("%d\n", ++i);  // i becomes 7, then prints 7
printf("%d\n", i);    // prints 7

// Classic trap:
int x = 5;
int y = x++ + ++x;
// Order of ++ relative to evaluation of + is UB
// Don't write code like this

// SAFE: One modification per expression, or use temporaries
int a = x++;  // safe: a = 5, x = 6
int b = ++x;  // safe: x = 7, b = 7
int y = a + b; // safe: y = 12
```

### 13.3 Uninitialized Local Variables

```c
void test(void) {
    int x;
    printf("%d\n", x);  // UB — x contains whatever was on the stack
}

// What you actually see:
// - Small programs: often 0 (fresh stack pages are zero-initialized by OS)
// - Larger programs: whatever the previous stack frame left behind
// - With optimizations: the compiler may use a register value from somewhere
// - With sanitizers: GUARANTEED to catch this
```

---
