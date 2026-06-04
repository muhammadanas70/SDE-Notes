`#define` in C is a **preprocessor directive**.

It tells the compiler to **replace text** before the program is compiled.

Example:

```c
#define PI 3.14
```

Now everywhere `PI` appears, the preprocessor changes it to `3.14`.

So this:

```c
float area = PI * r * r;
```

becomes roughly:

```c
float area = 3.14 * r * r;
```

### Simple idea

Think of `#define` as a **shortcut or label**.

### Common uses

1. **Constants**

```c
#define MAX 100
```

2. **Macros**

```c
#define SQUARE(x) ((x) * (x))
```

Then:

```c
SQUARE(5)
```

becomes:

```c
(5) * (5)
```

### Important

`#define` is not a real variable.
It is just **text replacement** done before compilation.

A safer modern C style for constants is often:

```c
const int max = 100;
```

For very short memory:
`#define` = **replace this text with that text**.


### The “#...” lines in C are preprocessor directives

They run before the compiler proper. The preprocessor can include files, define macros, and conditionally include/exclude code.

The main directives you’ll see

| Directive | Purpose                                     |
| --------- | ------------------------------------------- |
| #define   | Define a macro (text replacement)           |
| #undef    | Remove a macro definition                   |
| #include  | Insert another file                         |
| #if       | Conditional compilation using an expression |
| #ifdef    | Compile if a macro is defined               |
| #ifndef   | Compile if a macro is not defined           |
| #elif     | “Else if” branch                            |
| #else     | Fallback branch                             |
| #endif    | End a conditional block                     |
| #error    | Emit a compilation error                    |
| #pragma   | Compiler-specific instructions              |

### What #ifdef / #ifndef / #endif actually do

`#ifdef` — “if defined”

Because `DEBUG` is defined, the `printf` line is kept.

`#ifndef` — “if not defined”

This block is included only when `DEBUG` is not defined.

`#endif` — end the conditional

Every `#if`, `#ifdef`, or `#ifndef` must eventually be closed with `#endif`.

### A practical pattern: header guards

Historically, `#ifndef` is most famous for preventing a header from being included multiple times.

First inclusion:

1. `MYLIB_H` is not defined → enter the block.

2. Define `MYLIB_H`.

3. Declare `hello`.

Second inclusion:

1. `MYLIB_H` is already defined → skip the whole block.

Modern compilers also support `#pragma once` as a simpler alternative, but traditional header guards are standard and portable.

### #if and #elif: conditions, not just existence

Unlike `#ifdef`, `#if` evaluates a preprocessor expression.

### #undef

### Common C preprocessor directives: #ifdef, #ifndef, #endif, etc.

These are all handled by the C preprocessor, which runs before the compiler.

Quick reference

| Directive                             | Meaning                                                |
| ------------------------------------- | ------------------------------------------------------ |
| #define NAME value                    | Define a macro (text replacement)                      |
| #undef NAME                           | Remove a macro definition                              |
| #ifdef NAME                           | Compile the following code only if NAME is defined     |
| #ifndef NAME                          | Compile the following code only if NAME is not defined |
| #if expr                              | Compile if a constant expression is true               |
| #elif expr                            | Else-if branch for #if                                 |
| #else                                 | Fallback branch                                        |
| #endif                                | Ends an #if, #ifdef, or #ifndef block                  |
| #include <file.h> / #include "file.h" | Insert contents of another file                        |
| #error "message"                      | Stop compilation with an error message                 |
| #pragma ...                           | Compiler-specific instructions                         |

### #ifdef / #ifndef / #endif

Think of them as conditional compilation.

#ifdef: “if defined”

Because `DEBUG` is defined, the `printf` line is compiled.

#ifndef: “if not defined”

This code is compiled only when `DEBUG` is not defined.

#endif: closes the block

Every `#ifdef`, `#ifndef`, or `#if` must eventually be matched by an `#endif`.

### #if / #elif / #else / #endif

Use these when you need a condition, not just “defined or not”.

### Header guards (#ifndef pattern)

A very common real-world use of `#ifndef` is preventing a header from being included multiple times.

Meaning:

1. If `MYLIB_H` is not defined, define it and include the declarations.

2. If the header is included again later, `MYLIB_H` is already defined, so the contents are skipped.

### #undef

After `#undef TEMP`, the macro no longer exists.

### #include

Angle brackets usually search system include paths first; quotes usually search the current/project paths first.

### #error

Compilation stops with the given message.

### #pragma once

A widely supported alternative to traditional header guards:

It tells the compiler to include the header only once. Traditional guards are still the most portable across all compilers and tools.

One-line mental model

`#define` creates a label. `#ifdef` asks “is that label defined?”. `#ifndef` asks “is that label not defined?”. `#if` asks “does this constant expression evaluate true?”. `#endif` ends the conditional block.

Important caveat

These directives operate during preprocessing, before the compiler sees the code. They do not create variables, types, or runtime conditions; they decide which source text the compiler receives.

Good question. When people say **"before the compiler"**, they mean there is a step that happens **before actual C compilation starts**.

A simplified C build process looks like this:

```text
Source Code (.c)
      |
      v
+----------------+
| Preprocessor   |
+----------------+
      |
      v
Expanded C Code
      |
      v
+----------------+
| Compiler       |
+----------------+
      |
      v
Assembly Code
      |
      v
+----------------+
| Assembler      |
+----------------+
      |
      v
Object File (.o)
      |
      v
+----------------+
| Linker         |
+----------------+
      |
      v
Executable
```

---

## Example 1: #define

You write:

```c
#define MAX 100

int main() {
    int x = MAX;
}
```

### Preprocessor output

Before the compiler sees it:

```c
int main() {
    int x = 100;
}
```

The compiler never sees `MAX`.

It only sees:

```c
int x = 100;
```

---

## Example 2: #ifdef

You write:

```c
#define DEBUG

int main() {
#ifdef DEBUG
    printf("Debug mode\n");
#endif
}
```

### Preprocessor output

```c
int main() {
    printf("Debug mode\n");
}
```

The compiler never sees `#ifdef`, `#endif`, or `DEBUG`.

---

If `DEBUG` is not defined:

```c
int main() {
#ifdef DEBUG
    printf("Debug mode\n");
#endif
}
```

becomes:

```c
int main() {
}
```

The compiler only compiles the remaining code.

---

## Why is it called a "preprocessor"?

Because it **processes the source code first**.

It handles:

* `#define`
* `#include`
* `#ifdef`
* `#ifndef`
* `#if`
* `#pragma`
* `#error`

and then produces a new C file that is passed to the compiler.

---

## Linux Kernel Example

You'll see a lot of code like:

```c
#ifdef CONFIG_IPV6
    ipv6_init();
#endif
```

If the kernel is built with IPv6 support:

```c
ipv6_init();
```

is included.

If not, the code is completely removed before compilation.

This is one reason the Linux kernel can support many configurations without maintaining separate source files.

---

## How to actually see the preprocessor output

Using GCC:

```bash
gcc -E file.c
```

or

```bash
cpp file.c
```

Example:

```bash
gcc -E hello.c
```

This shows the source code **after preprocessing** and **before compilation**.

For learning C and kernel development, running `gcc -E` on small examples is one of the best ways to understand what `#define`, `#ifdef`, and `#include` really do.

The best way to understand preprocessor directives is to see where they're used in real projects like the Linux kernel, system libraries, embedded software, and production applications.

---

# 1. `#define` — Constants

### Simple example

```c
#define MAX_PACKET_SIZE 1500

char buffer[MAX_PACKET_SIZE];
```

After preprocessing:

```c
char buffer[1500];
```

### Real-world use

Network stacks:

```c
#define ETH_ALEN 6
#define ETH_HLEN 14
```

Linux networking code uses these constants everywhere.

---

# 2. `#define` — Function-like Macros

### Example

```c
#define MIN(a, b) ((a) < (b) ? (a) : (b))

int x = MIN(10, 20);
```

Becomes:

```c
int x = ((10) < (20) ? (10) : (20));
```

### Real-world use

Linux kernel:

```c
#define ARRAY_SIZE(arr) (sizeof(arr) / sizeof((arr)[0]))
```

Usage:

```c
int nums[100];

size_t n = ARRAY_SIZE(nums);
```

This is used everywhere in kernel code.

---

# 3. `#ifdef` — Enable Debug Code

### Example

```c
#define DEBUG

#ifdef DEBUG
printf("debug enabled\n");
#endif
```

### Real-world use

```c
#ifdef DEBUG
    dump_packet(pkt);
#endif
```

Development build:

```text
dump_packet() included
```

Production build:

```text
dump_packet() removed entirely
```

No runtime overhead.

---

# 4. `#ifndef` — Header Guards

Probably the most common use.

### mydriver.h

```c
#ifndef MYDRIVER_H
#define MYDRIVER_H

struct driver {
    int id;
};

#endif
```

### Why?

Without this:

```c
#include "mydriver.h"
#include "mydriver.h"
```

Compiler sees:

```c
struct driver {
    int id;
};

struct driver {
    int id;
};
```

Error:

```text
redefinition of struct driver
```

Header guards prevent this.

---

# 5. `#if`

Compile based on values.

### Example

```c
#define VERSION 2

#if VERSION >= 2
    init_new_api();
#endif
```

Preprocessor evaluates:

```c
2 >= 2
```

Result:

```c
init_new_api();
```

---

# 6. `#elif`

### Example

```c
#define ARCH_ARM

#ifdef ARCH_X86
    x86_init();

#elif defined(ARCH_ARM)
    arm_init();

#else
    generic_init();
#endif
```

Output:

```c
arm_init();
```

---

# 7. `#else`

### Example

```c
#ifdef ENABLE_LOGGING
    log_message();
#else
    do_nothing();
#endif
```

Useful for feature toggles.

---

# 8. `#include`

### System header

```c
#include <stdio.h>
```

Preprocessor literally inserts the file contents.

Conceptually:

```c
/* contents of stdio.h */

int printf(const char *, ...);
```

---

### Project header

```c
#include "packet.h"
```

Very common in large codebases:

```c
#include "netdev.h"
#include "tcp.h"
#include "udp.h"
```

Linux kernel has thousands of these.

---

# 9. `#undef`

Remove a macro definition.

### Example

```c
#define DEBUG

#undef DEBUG

#ifdef DEBUG
printf("debug");
#endif
```

Output:

```c
/* nothing */
```

---

### Real-world use

Sometimes libraries define conflicting macros:

```c
#undef min
#undef max
```

Windows-related code often does this.

---

# 10. `#error`

Force compilation failure.

### Example

```c
#ifndef CONFIG_IPV6
#error "IPv6 support required"
#endif
```

Build output:

```text
error: IPv6 support required
```

Useful when a feature is mandatory.

---

# 11. `#pragma`

Compiler-specific instructions.

### Example

```c
#pragma once
```

Alternative to header guards.

---

### Another example

```c
#pragma pack(push, 1)

struct packet {
    char type;
    int len;
};

#pragma pack(pop)
```

Used in:

* Network protocols
* Hardware drivers
* Binary file formats

Controls structure padding.

---

# 12. `defined()`

Used inside `#if`.

### Example

```c
#if defined(DEBUG)
    printf("debug");
#endif
```

Equivalent to:

```c
#ifdef DEBUG
```

But more flexible.

---

# Linux Kernel Example

A realistic snippet looks like:

```c
#ifdef CONFIG_IPV6

#define IPV6_VERSION 6

static void ipv6_init(void)
{
    printk("IPv6 enabled\n");
}

#else

static void ipv6_init(void)
{
}

#endif
```

When `CONFIG_IPV6=y`:

```c
#define IPV6_VERSION 6

static void ipv6_init(void)
{
    printk("IPv6 enabled\n");
}
```

When `CONFIG_IPV6=n`:

```c
static void ipv6_init(void)
{
}
```

The unwanted code never reaches the compiler.

---

# The 90% You Need for System Programming

If you're working in C, Linux kernel, drivers, networking, embedded systems, you'll repeatedly see:

```c
#define
#ifdef
#ifndef
#if
#elif
#else
#endif
#include
```

Especially these patterns:

```c
/* Header guard */
#ifndef HEADER_H
#define HEADER_H
...
#endif
```

```c
/* Feature configuration */
#ifdef CONFIG_XDP
...
#endif
```

```c
/ * Utility macro */
#define ARRAY_SIZE(x)
```

```c
/* Architecture-specific code */
#if defined(CONFIG_X86)
...
#elif defined(CONFIG_ARM64)
...
#endif
```

These four patterns alone account for a huge portion of preprocessor usage in large C codebases such as Linux kernel, device drivers, networking stacks, and embedded firmware.

If your goal is **Linux kernel, networking, drivers, embedded systems, and system programming**, then the most valuable thing is not memorizing directives, but understanding **why they exist in large codebases**.

---

# 1. Architecture-Specific Code

A kernel supports multiple CPU architectures:

* x86
* ARM64
* RISC-V
* PowerPC

But the instructions for each architecture are different.

### Source

```c
#if defined(CONFIG_X86)

static inline void cpu_relax(void)
{
    asm volatile("pause");
}

#elif defined(CONFIG_ARM64)

static inline void cpu_relax(void)
{
    asm volatile("yield");
}

#endif
```

### What happens?

For x86 builds:

```c
static inline void cpu_relax(void)
{
    asm volatile("pause");
}
```

For ARM64 builds:

```c
static inline void cpu_relax(void)
{
    asm volatile("yield");
}
```

### Why?

The compiler never sees the wrong architecture's code.

---

# 2. Feature Selection in Linux Kernel

Linux can be built with:

* IPv4
* IPv6
* Bluetooth
* USB
* XDP
* eBPF

or without them.

### Source

```c
#ifdef CONFIG_IPV6

int ipv6_send_packet(...)
{
    ...
}

#endif
```

### Build with IPv6

```c
int ipv6_send_packet(...)
{
    ...
}
```

### Build without IPv6

```c
/* removed entirely */
```

### Why?

Smaller kernel image.

Less memory.

No dead code.

---

# 3. Debug Builds vs Production Builds

A classic pattern.

### Source

```c
#ifdef DEBUG

#define DBG(fmt, ...) \
    printf("[DEBUG] " fmt, ##__VA_ARGS__)

#else

#define DBG(fmt, ...)

#endif
```

Usage:

```c
DBG("packet len=%d\n", len);
```

---

### Development build

```c
printf("[DEBUG] packet len=%d\n", len);
```

---

### Production build

```c
;
```

Nothing.

### Why?

Zero runtime cost.

Not:

```c
if (debug)
```

which still executes.

---

# 4. Logging in Network Drivers

Imagine a NIC driver.

### Source

```c
#ifdef DRIVER_DEBUG

#define drv_dbg(fmt, ...) \
    printk(KERN_DEBUG fmt, ##__VA_ARGS__)

#else

#define drv_dbg(fmt, ...)

#endif
```

Usage:

```c
drv_dbg("RX packet received\n");
```

Production build:

```c
;
```

Removed completely.

---

# 5. Header Guards

Every kernel header uses this idea.

### netdev.h

```c
#ifndef _NETDEV_H
#define _NETDEV_H

struct net_device {
    char name[16];
};

#endif
```

---

### Why?

Suppose:

```c
#include "netdev.h"
#include "tcp.h"
```

and:

```c
tcp.h
    |
    +--> includes netdev.h
```

Without guards:

```c
struct net_device { ... };
struct net_device { ... };
```

Compiler error.

Header guards prevent duplicate definitions.

---

# 6. Compile-Time Configuration

Embedded systems use this constantly.

### Source

```c
#define UART_COUNT 2

#if UART_COUNT == 1

static struct uart uart0;

#elif UART_COUNT == 2

static struct uart uart0;
static struct uart uart1;

#endif
```

---

### Result

For UART_COUNT=2:

```c
static struct uart uart0;
static struct uart uart1;
```

No runtime branching.

---

# 7. Platform-Specific Drivers

Same driver source supports multiple boards.

### Source

```c
#ifdef BOARD_RPI

#define LED_GPIO 17

#elif defined(BOARD_BEAGLEBONE)

#define LED_GPIO 53

#endif
```

Usage:

```c
gpio_set_value(LED_GPIO, 1);
```

---

### Raspberry Pi build

```c
gpio_set_value(17, 1);
```

### BeagleBone build

```c
gpio_set_value(53, 1);
```

Same source.

Different hardware.

---

# 8. Compile-Time Assertions

Kernel-style safety checks.

### Source

```c
#if PAGE_SIZE != 4096
#error "Unsupported page size"
#endif
```

If someone changes:

```c
#define PAGE_SIZE 8192
```

Build stops immediately.

---

### Why?

Fail early.

Avoid runtime bugs.

---

# 9. Hardware Register Definitions

Common in embedded firmware.

### Source

```c
#define GPIO_BASE 0x40020000

#define GPIO_MODER \
    (*(volatile unsigned int *)(GPIO_BASE + 0x00))

#define GPIO_ODR \
    (*(volatile unsigned int *)(GPIO_BASE + 0x14))
```

Usage:

```c
GPIO_ODR |= (1 << 5);
```

After preprocessing:

```c
(*(volatile unsigned int *)(0x40020000 + 0x14))
    |= (1 << 5);
```

---

### Why?

Readable hardware access.

---

# 10. Linux Kernel's ARRAY_SIZE()

One of the most famous macros.

### Definition

```c
#define ARRAY_SIZE(arr) \
    (sizeof(arr) / sizeof((arr)[0]))
```

Usage

```c
int ports[8];

int n = ARRAY_SIZE(ports);
```

Becomes:

```c
int n = sizeof(ports) / sizeof(ports[0]);
```

Result:

```c
8
```

---

### Why?

Avoid hardcoding:

```c
for (i = 0; i < 8; i++)
```

which breaks if array size changes.

---

# 11. Likely / Unlikely Macros

Used heavily in the Linux kernel.

### Definition

```c
#define likely(x) \
    __builtin_expect(!!(x), 1)

#define unlikely(x) \
    __builtin_expect(!!(x), 0)
```

Usage:

```c
if (likely(pkt != NULL))
{
    process_packet(pkt);
}
```

### Why?

Provides branch prediction hints to the compiler.

Important in:

* Network stacks
* Schedulers
* Memory allocators

---

# 12. Optional Functionality

Instead of:

```c
if (feature_enabled)
{
    feature_run();
}
```

Kernel often does:

```c
#ifdef CONFIG_FEATURE

feature_run();

#endif
```

### Difference

Runtime check:

```text
CPU executes condition every time.
```

Compile-time check:

```text
Code removed entirely.
```

---

# A Real Linux Kernel Pattern

You will constantly see code like:

```c
#ifdef CONFIG_SMP

void wakeup_other_cpus(void)
{
    ...
}

#else

static inline void wakeup_other_cpus(void)
{
}

#endif
```

### SMP Enabled

```c
wakeup_other_cpus();
```

actually wakes CPUs.

### SMP Disabled

```c
wakeup_other_cpus();
```

becomes:

```c
static inline void wakeup_other_cpus(void)
{
}
```

The call remains valid, but does nothing.

No `#ifdef` scattered everywhere.

This pattern is extremely common in the Linux kernel because it keeps call sites clean while allowing features to be compiled in or out.

---

### A practical rule

When reading kernel, driver, networking, or embedded code:

* `#define` → constants, utility macros, hardware registers
* `#ifdef CONFIG_*` → feature selection
* `#if defined(ARCH_*)` → architecture-specific code
* `#ifndef HEADER_H` → header guards
* `#error` → build-time validation
* `#include` → share declarations across modules

Once you recognize these patterns, a large percentage of the Linux kernel's preprocessing logic becomes much easier to follow.
