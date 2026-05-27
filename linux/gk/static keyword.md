# Why `static` and `__` in Kernel Code?

Two separate concepts — both important.

---

## Why `static`?

### In normal C, `static` on a function means:
> **"This function is invisible outside this file"**

```c
// file: mymodule.c
static int my_function(void) { ... }  // only visible in this file
int my_function(void) { ... }         // visible everywhere (exported)
```

### In the kernel, this matters a LOT:

The kernel is **one giant binary** — millions of functions from thousands of files all linked together. If every function was global (non-static), you'd get:

```
ERROR: function name collision!

your module:   int init(void) { ... }
some driver:   int init(void) { ... }   ← same name = CRASH
wifi driver:   int init(void) { ... }   ← linker explodes
```

So `static` = **namespace protection**.

```c
// Every module does this safely because static keeps it local:
static int __init hello_init(void) { ... }  // your init
static int __init wifi_init(void) { ... }   // wifi driver's init
static int __init usb_init(void) { ... }    // USB driver's init
// No collision — all invisible outside their own file
```

### Rule of thumb:
```c
// If a function is NOT part of the public kernel API → make it static
// Only exported functions (EXPORT_SYMBOL) should be non-static
```

---

## Why `__init` and `__exit`?

The double underscore `__` means it's a **compiler/linker directive** — not a regular function name prefix.

### What `__init` actually does:

```c
#define __init  __attribute__((__section__(".init.text")))
```

It tells the compiler:
> **"Put this function in a special memory section called `.init.text`"**

### Why a special section?

```
Kernel boots up
      ↓
Runs all __init functions  (module setup, hardware init, etc.)
      ↓
__init section is FREED from memory  ← this is the key part
      ↓
That RAM is given back to the system
```

```c
static int __init hello_init(void) {
    printk(KERN_INFO "Hello!\n");
    return 0;
    // After this runs once at boot — this code is DELETED from RAM
}
```

You've probably seen this in `dmesg`:
```
Freeing unused kernel image (initmem) memory: 2408K
```
That's the kernel throwing away all `__init` code after boot. Saves precious kernel memory.

### `__exit` is similar:

```c
#define __exit  __attribute__((__section__(".exit.text")))
```

```c
static void __exit hello_exit(void) {
    printk(KERN_INFO "Bye!\n");
    // Only called when module is removed (rmmod)
    // If module can't be unloaded, this code is never needed
    // → kernel can optimize it away entirely
}
```

---

## Visual Summary

```
Your .c file after compilation:
┌─────────────────────────────────────┐
│  .text section                      │
│    (normal functions — stay forever)│
│                                     │
│  .init.text section  (__init)       │
│    hello_init()  ← freed after boot │
│                                     │
│  .exit.text section  (__exit)       │
│    hello_exit()  ← may be discarded │
└─────────────────────────────────────┘
```

---

## Other Common `__` Prefixes You'll See

| Prefix | Meaning |
|---|---|
| `__init` | Run once at init, then free the memory |
| `__exit` | Only needed at module removal |
| `__iomem` | Pointer to I/O mapped memory (not regular RAM) |
| `__user` | Pointer to **userspace** memory (must use `copy_from_user`) |
| `__be32` | Big-endian 32-bit (common in network headers) |
| `__le16` | Little-endian 16-bit |
| `__must_check` | Caller **must** check the return value |
| `__packed` | No padding between struct fields |

---

## One-line Summary

```
static  →  "keep this function private to this file, avoid name collisions"
__init  →  "free this code from RAM after it runs once"
__exit  →  "this is only needed when the module is removed"
```

These seem like small details but they reflect real kernel design concerns — **memory efficiency** and **safety at scale** across millions of lines of linked code. 