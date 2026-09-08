# Linux Kernel Programming
## A Comprehensive Guide: Internals, Modules, and Synchronization

> Audience: Systems programmers working in C on Linux kernel subsystems,  
> network security, and device-driver-level code.  
> Emphasis: Mental models, production thinking, correctness, and depth.

---

## How to Use This Syllabus

Each module follows a pattern:
- **What it is** — the concept and its role in the kernel
- **Core abstractions** — data structures and APIs to master
- **Mental model questions** — what you must be able to answer without notes
- **Production concerns** — what breaks in real deployments
- **Reference points** — where to look in the source tree

You do not progress by reading. You progress by being able to **explain** every
mental model question to a colleague and **trace** every API through kernel
source code before you call it.

---

## Prerequisites

Before kernel programming, you must have mastered:

| Area | What You Must Know |
|---|---|
| C | Pointers, pointer-to-pointer, function pointers, struct layout, unions, bit fields, volatile |
| Memory | Virtual vs physical, stack vs heap, alignment, cache lines |
| x86/ARM | Calling conventions, registers, privilege rings, interrupts |
| Linux userspace | system calls, procfs, /sys, /dev, signals, mmap |
| Concurrency | Race conditions, mutual exclusion — at least POSIX threads level |
| GCC/Clang | Inline assembly basics, compiler optimization flags |
| Make | Basic Makefiles, understanding build systems |

---

## MODULE 1: Kernel Architecture and Mental Model

### 1.1 Privilege and Address Space Split

**What it is:**  
The CPU enforces two privilege levels for Linux: ring 0 (kernel mode) and
ring 3 (user mode). The kernel occupies the upper portion of the virtual
address space; user processes occupy the lower.

**Core abstractions:**
- PAGE_OFFSET — where the kernel mapping begins (architecture-dependent)
- TASK_SIZE — upper limit of user virtual address space
- `__user` annotation — pointer into user space (requires copy_to/from_user)
- `__kernel` — pointer into kernel space

**Mental model questions:**
- Why can a user-space pointer not be directly dereferenced in kernel code?
- What happens if a kernel code path page-faults? (hint: oops vs. panic)
- What is the difference between kernel logical addresses and kernel virtual addresses?
- Why does __user exist if the compiler doesn't enforce it at runtime?

**Source reference:**  
`arch/x86/include/asm/page_types.h`  
`include/linux/compiler.h` (for __user sparse annotation)

---

### 1.2 Kernel Subsystems and Their Relationships

**What it is:**  
The Linux kernel is monolithic with loadable modules. All subsystems share
one address space. A bug in any driver can corrupt any other subsystem.

**Major subsystems:**

```
┌─────────────────────────────────────────────────────┐
│                 System Call Interface                │
├──────────┬──────────┬──────────┬──────────┬─────────┤
│ Process  │ Memory   │ VFS      │ Network  │ IPC     │
│ Mgmt     │ Mgmt     │ Layer    │ Stack    │         │
├──────────┴──────────┴──────────┴──────────┴─────────┤
│              Architecture-Dependent Code             │
│        (x86, arm64, riscv — arch/ directory)        │
├─────────────────────────────────────────────────────┤
│                    Hardware                          │
└─────────────────────────────────────────────────────┘
```

**Mental model questions:**
- Why does a kernel NULL pointer dereference crash the entire machine vs.
  just the process that caused it?
- What is the kernel's execution model — event-driven, threaded, or both?
- Name three distinct contexts in which kernel code can execute.

---

### 1.3 Kernel Execution Contexts

This is the most important mental model in all of kernel programming.
Everything you write depends on knowing which context you are in.

**The three contexts:**

| Context | Preemptible? | Can Sleep? | Has Process? | Examples |
|---|---|---|---|---|
| Process context | Yes (if CONFIG_PREEMPT) | Yes | Yes | syscalls, kernel threads |
| Softirq context | No | No | No | NET_RX_SOFTIRQ, NET_TX_SOFTIRQ |
| Hardirq context | No | No | No | device IRQ handlers |

**Critical rules derived from context:**
- You may NEVER sleep in interrupt context (no mutex_lock, no kmalloc with GFP_KERNEL)
- You may NEVER call schedule() from atomic context
- In_interrupt() is true in both hardirq and softirq context
- in_atomic() is true when preemption is disabled (spin_lock held, etc.)

**Source reference:**  
`include/linux/preempt.h` — in_interrupt(), in_atomic(), in_softirq()  
`kernel/softirq.c` — softirq execution model

---

## MODULE 2: Process Management Internals

### 2.1 task_struct — The Process Descriptor

**What it is:**  
Every process and kernel thread has a `struct task_struct`. This is the
central object of the process management subsystem.

**Key fields you must know:**
```c
struct task_struct {
    /* State */
    volatile long       state;       /* TASK_RUNNING, TASK_INTERRUPTIBLE, etc. */
    
    /* Identity */
    pid_t               pid;         /* process ID */
    pid_t               tgid;        /* thread group ID (== pid for main thread) */
    
    /* Scheduling */
    int                 prio;        /* dynamic priority */
    int                 static_prio; /* nice-based priority */
    unsigned int        policy;      /* SCHED_NORMAL, SCHED_FIFO, SCHED_RR */
    
    /* Memory */
    struct mm_struct    *mm;         /* NULL for kernel threads */
    struct mm_struct    *active_mm;  /* borrowed mm for kernel threads */
    
    /* Files */
    struct files_struct *files;      /* open file descriptors */
    
    /* Credentials */
    const struct cred   *cred;       /* effective UID/GID, capabilities */
    
    /* Relationships */
    struct task_struct  *parent;
    struct list_head    children;
    struct list_head    sibling;
    
    /* Stack */
    void                *stack;      /* kernel stack pointer */
};
```

**How to get current task:**
```c
current               /* macro — always valid in process context */
current->pid          /* always safe in process context */
current->comm         /* 16-byte task name */
```

**Mental model questions:**
- Why does a kernel thread have mm == NULL?
- Why does task_struct use list_head for children rather than a pointer?
- What is the relationship between pid and tgid for POSIX threads?
- Where is task_struct allocated? (hint: not on the heap with kmalloc)

**Source reference:**  
`include/linux/sched.h` — struct task_struct  
`kernel/fork.c` — copy_process() for how task_struct is created

---

### 2.2 Process States and Transitions

```
TASK_RUNNING ──── schedule() ────▶ TASK_RUNNING (on CPU)
     ▲                                    │
     │                              preempt / yield
     │                                    │
  wake_up()                         TASK_RUNNING (runqueue)
     │                                    │
TASK_INTERRUPTIBLE ◀──────────────── wait_event()
TASK_UNINTERRUPTIBLE ◀────────────── wait_event() I/O
```

**Mental model questions:**
- What is the difference between TASK_INTERRUPTIBLE and TASK_UNINTERRUPTIBLE?
- Why does D-state (uninterruptible sleep) not respond to kill -9?
- What does it mean for a process to be "on the runqueue" vs "running on a CPU"?

---

### 2.3 Scheduler Internals (CFS)

**What it is:**  
The Completely Fair Scheduler (CFS) uses a red-black tree ordered by virtual
runtime (vruntime) to select the next task to run.

**Key concepts:**
- vruntime — normalized runtime weighted by task priority
- sched_entity — the schedulable unit embedded in task_struct
- cfs_rq — per-CPU CFS runqueue
- min_vruntime — the leftmost key of the rb-tree

**Source reference:**  
`kernel/sched/fair.c`  
`include/linux/sched.h` — struct sched_entity

---

## MODULE 3: Memory Management

### 3.1 Physical Memory Layout

**Zones:**

| Zone | Range (x86_64) | Purpose |
|---|---|---|
| ZONE_DMA | 0–16 MB | Legacy ISA DMA devices |
| ZONE_DMA32 | 0–4 GB | Devices with 32-bit DMA |
| ZONE_NORMAL | All the rest | Main kernel/user memory |
| ZONE_HIGHMEM | x86-32 only | Memory above 896 MB on 32-bit |

**Page frame structure:**
```c
struct page {
    unsigned long   flags;     /* PG_locked, PG_dirty, PG_uptodate... */
    atomic_t        _refcount; /* usage count */
    struct list_head lru;      /* LRU list linkage */
    /* ... many more fields depending on page usage ... */
};
```

**Mental model questions:**
- Why was ZONE_HIGHMEM necessary on 32-bit and why does it not exist on 64-bit?
- A struct page exists for every physical page frame. What is the memory
  overhead of the page array for a machine with 32 GB RAM?
- What does page_to_pfn() and pfn_to_page() do? Why are these macros important?

---

### 3.2 Kernel Memory Allocators

You must understand which allocator to use and why.

**The allocator hierarchy:**

```
User request for memory
        │
        ├─── Need physically contiguous? ──YES──► kmalloc() / kzalloc()
        │                                          (backed by SLUB/SLAB)
        │
        ├─── Need virtually contiguous? ────────► vmalloc() / vzalloc()
        │                                          (physically discontiguous OK)
        │
        ├─── Fixed-size objects (performance)? ─► kmem_cache_alloc()
        │                                          (slab cache — best for frequent alloc/free)
        │
        ├─── Need whole pages? ─────────────────► __get_free_pages() / alloc_pages()
        │                                          (page allocator / buddy system)
        │
        └─── Per-CPU temp buffer? ──────────────► get_cpu_ptr() + per-CPU vars
```

**GFP flags — critical for correctness:**

| Flag | Meaning | When to Use |
|---|---|---|
| GFP_KERNEL | Can sleep, can reclaim | Process context, not holding spinlock |
| GFP_ATOMIC | Cannot sleep, cannot reclaim | Interrupt context, spinlock held |
| GFP_NOWAIT | No wait, no reclaim | Soft-rt paths that must not block |
| GFP_DMA | Must come from ZONE_DMA | Legacy DMA devices |
| __GFP_ZERO | Zero the memory | When you need zeroed allocation |

**The single most common kernel memory bug:**  
Using `GFP_KERNEL` inside a spinlock or interrupt context → immediate deadlock or
corruption. The kernel will warn if lockdep is enabled.

**Mental model questions:**
- Why can't vmalloc() be used for DMA buffers?
- What is the internal structure of the buddy allocator? What is a "buddy pair"?
- When is a slab cache preferable to kmalloc? (Think about frequency and fragmentation)
- What happens when kmalloc(size, GFP_ATOMIC) fails? Is it safe to return NULL?
- Why does kfree() not need a size argument but free_pages() does?

**Source reference:**  
`mm/slub.c` — SLUB allocator  
`mm/vmalloc.c` — vmalloc implementation  
`include/linux/gfp.h` — all GFP flags

---

### 3.3 Virtual Memory Areas (VMAs) and mm_struct

```c
struct mm_struct {
    struct maple_tree   mm_mt;      /* VMA tree (maple tree since 6.1) */
    unsigned long       mmap_base;
    pgd_t               *pgd;       /* page global directory */
    atomic_t            mm_count;
    atomic_t            mm_users;
    unsigned long       start_code, end_code;
    unsigned long       start_data, end_data;
    unsigned long       start_brk,  brk;
    unsigned long       start_stack;
};
```

**Mental model questions:**
- What is the difference between mm_count and mm_users?
- Why does a kernel thread have active_mm but mm == NULL?
- When does copy_mm() create a new mm_struct vs. sharing one (think: fork vs. clone)?

---

## MODULE 4: Interrupt Handling Architecture

### 4.1 Interrupt Processing Model

**The two-phase model (top half / bottom half):**

```
Hardware Interrupt fires
        │
        ▼
  ┌──────────────┐
  │   Top Half   │  ← hardirq context, minimal work, saves registers,
  │ (IRQ handler)│    acknowledges IRQ, schedules bottom half
  └──────┬───────┘
         │   raise_softirq() or tasklet_schedule()
         ▼
  ┌──────────────┐
  │ Bottom Half  │  ← runs after IRQ re-enabled, still not schedulable
  │  (softirq/   │
  │   tasklet/   │
  │  workqueue)  │
  └──────┬───────┘
         │ (workqueue only)
         ▼
  ┌──────────────┐
  │ Kernel thread│  ← process context, can sleep
  │  (kworker)   │
  └──────────────┘
```

**Bottom-half mechanisms compared:**

| Mechanism | Context | Can Sleep? | Per-CPU? | Use Case |
|---|---|---|---|---|
| softirq | softirq | No | Yes | Network RX/TX, timers — high frequency |
| tasklet | softirq | No | No | General deferred work — serialized |
| workqueue | process | Yes | Configurable | Work that needs to sleep |
| threaded IRQ | process | Yes | No | request_threaded_irq() |

**Network relevance:**  
The entire network receive path runs in NET_RX_SOFTIRQ. This is why
network driver receive handlers call `napi_schedule()` — they arm the NAPI
poll in the softirq context.

**Mental model questions:**
- Why are there exactly 10 softirq types? Can you add a new one?
- Why can tasklets never run concurrently with themselves?
- What is NAPI and why was it introduced? What problem does it solve
  compared to pure interrupt-driven receive?
- A workqueue handler wants to call mutex_lock(). Is this allowed? Why?

**Source reference:**  
`kernel/softirq.c` — softirq and tasklet implementation  
`kernel/workqueue.c` — workqueue implementation  
`net/core/dev.c` — net_rx_action() (softirq handler for network RX)

---

### 4.2 IRQ Registration and Management

```c
/* Register an interrupt handler */
int request_irq(unsigned int irq,
                irq_handler_t handler,
                unsigned long flags,
                const char *name,
                void *dev);

/* IRQ flags of significance */
IRQF_SHARED     /* multiple devices share this IRQ line */
IRQF_TRIGGER_*  /* edge/level trigger configuration */

/* Free an interrupt handler */
free_irq(unsigned int irq, void *dev);
```

**Mental model questions:**
- What does dev_id (the last argument to request_irq) do for IRQF_SHARED?
- What happens if an IRQ handler returns IRQ_NONE vs IRQ_HANDLED?
- Why should IRQ handlers be as short as possible?

---

## MODULE 5: Kernel Data Structures

### 5.1 Intrusive Linked Lists (list_head)

**What it is:**  
The kernel uses intrusive lists: the list node (list_head) is embedded inside
the data structure, not the reverse. This avoids a pointer indirection and
a separate allocation.

```c
struct list_head {
    struct list_head *next, *prev;  /* doubly-circular */
};

/* Embedded in your structure */
struct my_object {
    int              data;
    struct list_head list;   /* embedded node */
};

/* Recover the containing structure */
struct my_object *obj = list_entry(ptr, struct my_object, list);
/* or equivalently: container_of(ptr, struct my_object, list) */
```

**Key APIs:**
```c
LIST_HEAD(my_list);                          /* declare and initialize */
list_add(&obj->list, &my_list);             /* add at head */
list_add_tail(&obj->list, &my_list);        /* add at tail */
list_del(&obj->list);                       /* remove */
list_for_each_entry(obj, &my_list, list)    /* iterate */
list_for_each_entry_safe(obj, tmp, &my_list, list)  /* safe iteration (can delete) */
```

**Mental model questions:**
- Why is the kernel's list doubly-circular rather than NULL-terminated?
- Why does list_del() set prev and next to LIST_POISON? (Hint: debugging)
- Why must you use list_for_each_entry_safe() when deleting during iteration?
- How does container_of() work at the machine level? (Trace through offsetof)

**Source reference:**  
`include/linux/list.h`

---

### 5.2 Red-Black Trees (rbtree)

**What it is:**  
Self-balancing BST. Used for ordered data: VMAs in mm_struct, timer wheel
(hrtimers), CFQ I/O scheduler, and many other places.

```c
#include <linux/rbtree.h>

struct rb_root root = RB_ROOT;

struct my_node {
    struct rb_node  rb;       /* embedded node */
    unsigned long   key;
    void            *data;
};

/* Search — you must write yourself */
static struct my_node *my_search(struct rb_root *root, unsigned long key)
{
    struct rb_node *node = root->rb_node;
    while (node) {
        struct my_node *data = rb_entry(node, struct my_node, rb);
        if (key < data->key)
            node = node->rb_left;
        else if (key > data->key)
            node = node->rb_right;
        else
            return data;
    }
    return NULL;
}

/* After finding the insertion point, fixup balancing: */
rb_link_node(&new_node->rb, parent, new_link);
rb_insert_color(&new_node->rb, root);
```

**Mental model questions:**
- Why doesn't the kernel provide a generic rb_insert() function? (Design philosophy)
- What operations does rb_insert_color() perform?
- When would you prefer an rbtree over a hash table, and vice versa?

---

### 5.3 Hash Tables (Linux-style)

The kernel provides `hlist` (single-pointer head) for hash bucket chains
because hash tables need only forward traversal.

```c
DEFINE_HASHTABLE(htable, bits);  /* 2^bits buckets */

hash_add(htable, &obj->node, key);
hash_for_each_possible(htable, obj, node, key) { ... }
hash_del(&obj->node);
```

**Mental model questions:**
- Why does a hash table use hlist_head (single pointer) instead of list_head (two pointers)?
- What hash function does the kernel use? (See hash_long() in linux/hash.h)

---

### 5.4 XArray and Maple Tree

**What it is:**  
XArray (since 4.20) replaced radix trees. Maple Tree (since 6.1) replaced the
VMA rbtree in mm_struct. Both are optimized for page cache and VMA lookups.

**Mental model questions:**
- What is the access pattern that makes XArray faster than rbtree for page cache?
- Why does the VMA tree need range-based lookups? What does that require from the data structure?

---

### 5.5 Per-CPU Variables

**What it is:**  
Per-CPU variables have one copy per CPU. They eliminate cache line bouncing
for frequently updated global state and require no locking when a CPU
accesses its own copy.

```c
DEFINE_PER_CPU(unsigned long, my_counter);

/* Access (disables preemption around the access) */
unsigned long val = get_cpu_var(my_counter);
get_cpu_var(my_counter)++;
put_cpu_var(my_counter);

/* Or use this_cpu_* (requires you've already disabled preemption) */
this_cpu_inc(my_counter);

/* Aggregate across all CPUs */
unsigned long total = 0;
int cpu;
for_each_possible_cpu(cpu)
    total += per_cpu(my_counter, cpu);
```

**Mental model questions:**
- Why does get_cpu_var() disable preemption? What would go wrong without it?
- What is the difference between `__get_cpu_var` (old) and `this_cpu_ptr`?
- Network stack uses per-CPU receive queues. Why? What does this avoid?

**Source reference:**  
`include/linux/percpu.h`  
`net/core/dev.c` — softnet_data per-CPU receive structure

---

## MODULE 6: Writing Kernel Modules

### 6.1 Module Lifecycle and Skeleton

```c
#include <linux/module.h>
#include <linux/kernel.h>
#include <linux/init.h>

MODULE_LICENSE("GPL");
MODULE_AUTHOR("Your Name");
MODULE_DESCRIPTION("Module description");
MODULE_VERSION("1.0");

static int __init my_module_init(void)
{
    pr_info("my_module: loaded\n");
    /* Return 0 on success, negative errno on failure */
    return 0;
}

static void __exit my_module_exit(void)
{
    pr_info("my_module: unloaded\n");
    /* Cannot fail — must always clean up completely */
}

module_init(my_module_init);
module_exit(my_module_exit);
```

**The __init and __exit annotations:**
- `__init` — section hint: code can be freed after initialization
- `__exit` — omitted entirely if module is built into kernel (not loadable)

**Minimal Makefile:**
```makefile
obj-m := my_module.o

KDIR := /lib/modules/$(shell uname -r)/build

all:
	$(MAKE) -C $(KDIR) M=$(PWD) modules

clean:
	$(MAKE) -C $(KDIR) M=$(PWD) clean
```

**Mental model questions:**
- What does `insmod` actually do at the kernel level? (hint: init_module syscall,
  ELF loading, symbol resolution)
- Why can't module_exit() return an error?
- What does `EXPORT_SYMBOL_GPL()` mean? What is the difference from EXPORT_SYMBOL?
- What happens when you rmmod a module that still has users?

---

### 6.2 Module Parameters

```c
static int timeout = 30;
static char *device_name = "eth0";
static int debug_level = 0;

module_param(timeout, int, 0644);       /* name, type, sysfs permissions */
module_param(device_name, charp, 0);
module_param(debug_level, int, 0644);

MODULE_PARM_DESC(timeout, "Timeout in seconds (default: 30)");
MODULE_PARM_DESC(device_name, "Network device name");
```

**Usage:**
```bash
insmod my_module.ko timeout=60 debug_level=2
# Or after load via sysfs:
echo 2 > /sys/module/my_module/parameters/debug_level
```

---

### 6.3 Kernel Logging

```c
pr_emerg("...");    /* KERN_EMERG — system is unusable */
pr_alert("...");    /* KERN_ALERT — action must be taken immediately */
pr_crit("...");     /* KERN_CRIT — critical condition */
pr_err("...");      /* KERN_ERR — error condition */
pr_warn("...");     /* KERN_WARNING */
pr_notice("...");   /* KERN_NOTICE */
pr_info("...");     /* KERN_INFO */
pr_debug("...");    /* KERN_DEBUG — only when DEBUG defined or dynamic debug on */

/* Rate-limited variants (prevent log flooding) */
pr_err_ratelimited("...");

/* Device-contextual (preferred in driver code) */
dev_err(dev, "...");
dev_info(dev, "...");

/* Net device contextual (preferred in netdev code) */
netdev_err(netdev, "...");
```

**Dynamic debug:**
```bash
echo "module my_module +p" > /sys/kernel/debug/dynamic_debug/control
echo "file drivers/net/my_driver.c +p" > /sys/kernel/debug/dynamic_debug/control
```

---

### 6.4 Character Device Drivers

**Registration flow:**

```
alloc_chrdev_region() → get major/minor numbers
cdev_init()           → associate file_operations with cdev
cdev_add()            → register cdev with kernel
class_create()        → create device class (/sys/class/)
device_create()       → create /dev/mydev node automatically
```

**file_operations — the dispatch table:**

```c
static const struct file_operations my_fops = {
    .owner          = THIS_MODULE,
    .open           = my_open,
    .release        = my_release,
    .read           = my_read,
    .write          = my_write,
    .unlocked_ioctl = my_ioctl,
    .mmap           = my_mmap,
    .poll           = my_poll,
    .llseek         = my_llseek,
};
```

**Data flow — read path:**
```c
static ssize_t my_read(struct file *filp, char __user *buf,
                       size_t count, loff_t *f_pos)
{
    char kernel_buf[128];
    size_t len;
    
    /* 1. Gather data into kernel_buf */
    len = snprintf(kernel_buf, sizeof(kernel_buf), "data\n");
    
    /* 2. Copy to user space — NEVER use memcpy for __user pointers */
    if (copy_to_user(buf, kernel_buf, len))
        return -EFAULT;
    
    *f_pos += len;
    return len;
}
```

**Mental model questions:**
- Why must you use copy_to_user / copy_from_user? What can go wrong with
  direct pointer access?
- What does copy_to_user() do when the user pointer is invalid?
- What is the difference between .ioctl and .unlocked_ioctl? Why was the
  big kernel lock removed?
- What does `filp->private_data` give you? How do you use it per-open-instance?

**Source reference:**  
`include/linux/fs.h` — struct file_operations, struct file, struct inode  
`fs/char_dev.c` — character device registration

---

### 6.5 Proc Filesystem Interface

```c
#include <linux/proc_fs.h>
#include <linux/seq_file.h>

/* Modern proc entry using seq_file (handles large output correctly) */
static int my_proc_show(struct seq_file *m, void *v)
{
    seq_printf(m, "counter: %lu\n", my_counter);
    seq_printf(m, "state: %s\n", my_state_str());
    return 0;
}

static int my_proc_open(struct inode *inode, struct file *file)
{
    return single_open(file, my_proc_show, NULL);
}

static const struct proc_ops my_proc_ops = {
    .proc_open    = my_proc_open,
    .proc_read    = seq_read,
    .proc_lseek   = seq_lseek,
    .proc_release = single_release,
};

/* In init */
proc_create("my_module", 0444, NULL, &my_proc_ops);

/* In exit */
remove_proc_entry("my_module", NULL);
```

**Mental model questions:**
- What is the bug in a proc handler that returns data in one read() call but
  the user program calls read() in a loop? (hint: seq_file solves this)
- Why was proc_ops introduced to replace file_operations for /proc?

---

### 6.6 Sysfs Interface

```c
/* In a kobject/device driver context */
static ssize_t my_attr_show(struct device *dev,
                            struct device_attribute *attr,
                            char *buf)
{
    return sysfs_emit(buf, "%d\n", my_value);
}

static ssize_t my_attr_store(struct device *dev,
                             struct device_attribute *attr,
                             const char *buf, size_t count)
{
    int val;
    if (kstrtoint(buf, 10, &val))
        return -EINVAL;
    my_value = val;
    return count;
}

static DEVICE_ATTR_RW(my_attr);  /* creates dev_attr_my_attr */
```

---

### 6.7 Kernel Debugging Techniques

**In order of invasiveness:**

```
Level 1 — Logging
    pr_debug(), pr_info(), dynamic_debug
    dmesg -w, journalctl -k -f

Level 2 — Runtime inspection
    /proc/modules, /proc/kallsyms
    cat /sys/module/<name>/parameters/<param>
    /sys/kernel/debug/ (debugfs)

Level 3 — Kernel Address Sanitizers
    KASAN — detects use-after-free, buffer overflow
    KFENCE — lightweight probabilistic memory safety checker
    KMSAN — detects uses of uninitialized memory

Level 4 — Tracing
    ftrace — function graph tracing, event tracing
    trace-cmd — ftrace frontend
    perf — hardware PMU + software events
    eBPF + bpftrace — live kernel introspection

Level 5 — Interactive Debuggers
    kgdb — kernel GDB stub (serial or network)
    kdb — built-in kernel debugger
    GDB + QEMU (recommended for kernel development)
```

**ftrace example:**
```bash
cd /sys/kernel/debug/tracing
echo function_graph > current_tracer
echo 1 > tracing_on
echo 0 > tracing_on
cat trace | head -100
```

**KASAN — enable in kernel config:**
```
CONFIG_KASAN=y
CONFIG_KASAN_INLINE=y
```

**Mental model questions:**
- What is the difference between a kernel Oops and a kernel Panic?
- How do you read an Oops message? What information does it contain?
- What does KASAN catch that Valgrind cannot?
- How does ftrace's function_graph tracer work without modifying source?
  (hint: CONFIG_DYNAMIC_FTRACE, mcount/fentry instrumentation)

---

## MODULE 7: Kernel Synchronization

This module is the most critical for correctness. Every production kernel bug
relating to data corruption or deadlock traces back to a violation of the
principles here.

### 7.1 Why Synchronization is Different in the Kernel

In userspace, you have threads competing. In the kernel, you have:
1. Multiple CPUs (SMP) running kernel code simultaneously
2. Preemption (one task can be preempted mid-operation)
3. Interrupt handlers that can interrupt kernel code at any point
4. Softirq handlers that run after interrupt handlers
5. NMI (Non-Maskable Interrupt) that can interrupt even disabled-interrupt code

**The synchronization decision tree:**

```
Is data accessed from interrupt context?
├── YES → must use spinlock (or disable IRQs)
│         spin_lock_irqsave() if shared with IRQ handler
│         spin_lock_bh() if shared with softirq/tasklet only
└── NO  → is the access path short (nanoseconds)?
          ├── YES → spinlock (spin_lock / spin_unlock)
          └── NO  → mutex (can sleep, long critical sections)
```

---

### 7.2 Atomic Operations

**What it is:**  
Single-instruction operations that are guaranteed not to be interrupted
mid-execution on any CPU.

```c
atomic_t ref = ATOMIC_INIT(0);

atomic_inc(&ref);           /* ref++ atomically */
atomic_dec(&ref);           /* ref-- atomically */
int val = atomic_read(&ref);
atomic_set(&ref, 5);

/* Test-and-modify (returns old value) */
int old = atomic_fetch_add(3, &ref);   /* ref += 3, returns old value */
bool changed = atomic_inc_and_test(&ref);  /* true if result is zero */
bool was_zero = atomic_dec_and_test(&ref); /* true if result is zero */

/* 64-bit */
atomic64_t big = ATOMIC64_INIT(0);
atomic64_inc(&big);

/* Bit operations */
set_bit(0, &flags);
clear_bit(0, &flags);
test_and_set_bit(0, &flags);  /* returns old bit value */
```

**Mental model questions:**
- Are atomic operations a substitute for locks in general? Why or why not?
- When is atomic_t appropriate and when is it insufficient?
- What makes atomic_cmpxchg() useful? Give a lock-free use case.
- Why does atomic_read() exist as a macro? Why not just read the value directly?

---

### 7.3 Memory Barriers and Ordering

This is the most subtle topic in kernel programming. Modern CPUs and
compilers reorder reads and writes. Memory barriers tell both the compiler
and the CPU not to reorder past a certain point.

**The four barrier types:**

```c
barrier();          /* Compiler barrier ONLY — prevents compiler reordering */
mb();               /* Full memory barrier — CPU + compiler */
rmb();              /* Read memory barrier */
wmb();              /* Write memory barrier */
smp_mb();           /* SMP-aware — NOP on UP kernel */
smp_rmb();          /* SMP read barrier */
smp_wmb();          /* SMP write barrier */
```

**Acquire/release semantics (preferred in modern code):**
```c
smp_load_acquire(&ptr);   /* load + acquire barrier */
smp_store_release(&ptr, val); /* release barrier + store */
```

**The classic producer/consumer ordering problem:**
```c
/* Producer (CPU 1) */
data = compute_result();     /* (A) write data */
smp_wmb();                   /* barrier: A must be visible before B */
flag = 1;                    /* (B) publish flag */

/* Consumer (CPU 2) */
while (!flag);               /* (C) wait for flag */
smp_rmb();                   /* barrier: C must complete before D */
use(data);                   /* (D) read data */
```

Without these barriers, on weakly-ordered architectures (ARM, POWER),
the consumer might see `flag == 1` but read stale `data`.

**Mental model questions:**
- What is the difference between a compiler barrier and a CPU memory barrier?
- Why does x86 rarely need explicit memory barriers but ARM often does?
- What is TSO (Total Store Order) and how does it affect which barriers you need?
- What does `ACCESS_ONCE()` (now READ_ONCE/WRITE_ONCE) prevent? Why is it important?
- Explain a real scenario in the network stack where missing a memory barrier
  causes a bug.

**Source reference:**  
`Documentation/memory-barriers.txt` — the definitive reference (long but essential)  
`include/asm-generic/barrier.h`

---

### 7.4 Spinlocks

**What it is:**  
A busy-wait lock. The acquiring CPU spins in a tight loop until the lock is
available. Suitable only for very short critical sections.

**The fundamental rule:**  
**You cannot sleep while holding a spinlock.**

```c
spinlock_t lock;
spin_lock_init(&lock);

/* Basic — process context only, no IRQ concern */
spin_lock(&lock);
/* critical section */
spin_unlock(&lock);

/* If lock shared with IRQ handler — saves/restores IRQ state */
unsigned long flags;
spin_lock_irqsave(&lock, flags);
/* critical section */
spin_unlock_irqrestore(&lock, flags);

/* If lock shared with softirq/tasklet only */
spin_lock_bh(&lock);
/* critical section */
spin_unlock_bh(&lock);

/* Non-blocking try */
if (spin_trylock(&lock)) {
    /* got the lock */
    spin_unlock(&lock);
}
```

**What spin_lock_irqsave does internally:**
1. Saves current IRQ flags (whether interrupts were enabled)
2. Disables local CPU interrupts
3. Acquires the spinlock

**Why you need irqsave vs. just irq:**  
If you hold a spinlock and an interrupt fires on the same CPU and tries to
acquire the same spinlock → **deadlock**. The IRQ handler spins, the original
code never resumes to release it.

**Mental model questions:**
- Why does a spinlock busy-wait rather than sleep? On a uniprocessor system,
  what would happen if a spinlock slept?
- What does `preempt_disable()` have to do with spinlocks?
- You hold a spinlock in process context. An interrupt fires on the same CPU.
  Can the interrupt handler acquire a different spinlock? The same spinlock?
- What is a "lock inversion" and how does lockdep detect it?
- Raw spinlocks (raw_spinlock_t) exist for a reason. When must you use them
  instead of spinlock_t? (hint: PREEMPT_RT kernel)

**Source reference:**  
`include/linux/spinlock.h`  
`kernel/locking/spinlock.c`

---

### 7.5 Mutexes

**What it is:**  
A sleeping lock. If the lock is contended, the acquiring task suspends
(goes to TASK_UNINTERRUPTIBLE) rather than spinning. Suitable for longer
critical sections.

```c
struct mutex my_mutex;
mutex_init(&my_mutex);

/* Or statically: */
DEFINE_MUTEX(my_mutex);

mutex_lock(&my_mutex);           /* sleep until acquired */
mutex_unlock(&my_mutex);

/* Interruptible — wakes up on signal (returns -EINTR) */
if (mutex_lock_interruptible(&my_mutex))
    return -ERESTARTSYS;
mutex_unlock(&my_mutex);

/* Non-blocking */
if (mutex_trylock(&my_mutex)) {
    mutex_unlock(&my_mutex);
}
```

**Mutex constraints — the kernel enforces these:**
- Only one task may hold it at a time
- The same task that locked must unlock (not transferable)
- Cannot be used in interrupt context
- Cannot be used in atomic context

**Spinlock vs. Mutex — the decision:**

| Condition | Use |
|---|---|
| In interrupt context | Spinlock |
| Critical section < few hundred nanoseconds | Spinlock |
| Critical section > few microseconds | Mutex |
| Need to sleep inside critical section | Mutex |
| Hard real-time with PREEMPT_RT | raw_spinlock or rt_mutex |

**Source reference:**  
`kernel/locking/mutex.c`  
`include/linux/mutex.h`

---

### 7.6 Reader-Writer Locks

**What it is:**  
Allows multiple concurrent readers OR a single exclusive writer. Never both.

**Two variants:**

```c
/* rwlock_t — spinlock variant (no sleep) */
rwlock_t rw_lock;
rwlock_init(&rw_lock);

read_lock(&rw_lock);
/* read critical section */
read_unlock(&rw_lock);

write_lock(&rw_lock);
/* write critical section */
write_unlock(&rw_lock);

/* rwsem — semaphore variant (can sleep) */
struct rw_semaphore rw_sem;
init_rwsem(&rw_sem);

down_read(&rw_sem);
/* read critical section */
up_read(&rw_sem);

down_write(&rw_sem);
/* write critical section */
up_write(&rw_sem);
```

**The writer starvation problem:**  
If readers continuously hold the lock, a writer may never acquire it.
The Linux rwsem implementation is writer-fair to prevent this.

**Mental model questions:**
- In what data-access patterns does rwlock actually improve over spinlock?
- What is the cache coherency cost of multiple CPUs holding a read lock
  simultaneously? (hint: exclusive vs shared cache lines)
- Why is RCU often better than rwlock for read-heavy workloads?

---

### 7.7 RCU — Read-Copy-Update

**What it is:**  
RCU is the most important synchronization primitive in the Linux kernel
for read-heavy data structures. Readers pay near zero cost. Writers take
a copy, modify it, then atomically publish the new version.

**The mental model:**  
RCU separates "removing a pointer" from "freeing the pointed-to memory."
The memory is freed only after all pre-existing RCU read-side critical
sections have completed (the "grace period").

```c
/* Reader side — extremely cheap, just marks entry/exit of RCS */
rcu_read_lock();                              /* disables preemption */
struct my_data *p = rcu_dereference(gptr);   /* safe pointer dereference */
if (p)
    use(p->value);                            /* safe: p won't be freed yet */
rcu_read_unlock();

/* Writer side */
struct my_data *old_ptr = rcu_dereference_protected(gptr, lockdep_is_held(&my_lock));
struct my_data *new_ptr = kmalloc(sizeof(*new_ptr), GFP_KERNEL);
*new_ptr = *old_ptr;         /* copy old data */
new_ptr->value = new_val;    /* modify copy */
rcu_assign_pointer(gptr, new_ptr);  /* atomically publish new pointer */

/* Wait for all existing readers to finish, then free old */
synchronize_rcu();           /* blocking — waits for grace period */
kfree(old_ptr);

/* Or non-blocking callback version: */
call_rcu(&old_ptr->rcu_head, my_free_callback);
```

**Why rcu_dereference() is not just a pointer read:**  
It includes a data dependency barrier on DEC Alpha and ensures the compiler
does not cache the pointer across the memory access.

**RCU in the networking stack:**  
The routing table (FIB — Forwarding Information Base) uses RCU. Every
packet lookup reads the routing table under rcu_read_lock(). Route updates
use the writer side. This means millions of packets per second can do
route lookups with zero lock contention.

```c
/* From net/ipv4/route.c — simplified */
rcu_read_lock();
rt = ip_route_input_noref(skb, daddr, saddr, tos, dev);
rcu_read_unlock();
```

**Mental model questions:**
- What does "grace period" mean precisely? When does it end?
- Why can an RCU reader NEVER sleep? (hint: grace period detection)
- What is SRCU (Sleepable RCU) and when do you need it?
- Why does rcu_assign_pointer include a write memory barrier?
- Draw the timeline of: writer modifies pointer, reader 1 (already in RCS)
  continues, reader 2 (starts after modification) gets new pointer, grace period
  ends, old memory freed.
- How does RCU work on uniprocessor (UP) kernels?

**Source reference:**  
`include/linux/rcupdate.h`  
`kernel/rcu/tree.c`  
`Documentation/RCU/` — the best RCU documentation exists here  
Paul McKenney's RCU papers

---

### 7.8 Seqlocks

**What it is:**  
A lock optimized for data that is written rarely and read frequently,
where readers must never block writers. Readers retry if they catch a
writer mid-update.

```c
seqlock_t my_seqlock;
seqlock_init(&my_seqlock);

/* Writer */
write_seqlock(&my_seqlock);
/* update data */
write_sequnlock(&my_seqlock);

/* Reader — retry loop */
unsigned int seq;
do {
    seq = read_seqbegin(&my_seqlock);
    /* read data snapshot */
    val1 = data.field1;
    val2 = data.field2;
} while (read_seqretry(&my_seqlock, seq));
/* val1 and val2 are consistent */
```

**Use case in the kernel:**  
`jiffies_64` on 32-bit systems uses seqlock because reading a 64-bit value
with two 32-bit loads is not atomic. The kernel's timekeeper uses seqlock
for `timespec64` updates.

**Mental model questions:**
- What happens if a reader is preempted in the middle of the read section?
- Why can't you allocate memory inside a seqlock read section?
- Compare seqlock to RCU: when is each appropriate?

---

### 7.9 Completion Variables

**What it is:**  
A one-shot synchronization point. One path signals completion; another
waits for it. Think of it as a "gate" or "rendezvous."

```c
struct completion my_completion;
init_completion(&my_completion);

/* Waiter (blocks until signaled) */
wait_for_completion(&my_completion);

/* Or with timeout */
unsigned long ret = wait_for_completion_timeout(&my_completion,
                                                msecs_to_jiffies(1000));
if (!ret)
    pr_err("timeout\n");

/* Signaler */
complete(&my_completion);         /* wake one waiter */
complete_all(&my_completion);     /* wake all waiters */
```

**Mental model questions:**
- Why not use a mutex + condition variable (like pthreads) in the kernel?
- What is the difference between complete() and complete_all()?
- What is the "use after complete_all" bug?

---

### 7.10 Lockdep — The Lock Validator

**What it is:**  
A runtime lock dependency checker built into the kernel. It tracks which
locks are acquired in which order and detects potential deadlocks before
they happen.

**What lockdep detects:**
- Lock ordering inversions (A→B in one path, B→A in another → deadlock cycle)
- Lock re-entry (acquiring a non-recursive spinlock while holding it)
- Acquiring a sleeping lock in atomic context
- IRQ-unsafe locking patterns

**Enable in kernel config:**
```
CONFIG_PROVE_LOCKING=y
CONFIG_LOCKDEP=y
CONFIG_DEBUG_LOCKDEP=y
```

**Reading a lockdep splat:**
```
======================================================
WARNING: possible circular locking dependency detected
...
-> #1 (lock_B){....}-{2:2}:
       __mutex_lock+0x...
       lock_b_function+0x...

-> #0 (lock_A){....}-{2:2}:
       __mutex_lock+0x...
       lock_a_function+0x...
```

**Mental model questions:**
- Lockdep tracks "lock classes" not individual lock instances. Why? What
  does this mean for locks inside arrays or lists?
- What is the `mutex_lock_nested()` API for and when should you use it?
- How does lockdep detect IRQ-unsafe locks? (hint: it simulates IRQ entry)

---

### 7.11 Synchronization Patterns for Kernel Network Code

These patterns appear throughout `net/` and `drivers/net/`:

**Pattern 1: RCU for routing tables and ARP cache**
```c
/* Lookup: rcu_read_lock → dereference → use → rcu_read_unlock */
/* Update: RTNL lock (route table mutex) + rcu_assign_pointer + synchronize_rcu */
```

**Pattern 2: RTNL (Routing Netlink) Lock**
```c
/* The main serializer for network configuration changes */
rtnl_lock();    /* mutex — serialize route/device changes */
/* modify routing table, netdevice, etc. */
rtnl_unlock();
```

**Pattern 3: Per-netns locking**
```c
/* Each network namespace has its own lock domain */
/* Operations scoped to a namespace use that namespace's locks */
```

**Pattern 4: Netdev xmit lock**
```c
/* struct netdev_queue has a spinlock for Tx queue serialization */
txq = netdev_get_tx_queue(dev, queue_index);
__netif_tx_lock(txq, cpu);
/* transmit */
__netif_tx_unlock(txq);
```

---

## MODULE 8: Linux Networking Subsystem Internals

*(Expanded section for your primary focus area)*

### 8.1 sk_buff — The Socket Buffer

**What it is:**  
Every packet in the Linux network stack is represented as an `sk_buff`.
This is the central data structure of the networking subsystem.
Understanding it completely is non-negotiable for networking kernel work.

```c
struct sk_buff {
    /* Packet data pointers */
    unsigned char   *head;   /* start of buffer allocation */
    unsigned char   *data;   /* start of actual packet data */
    unsigned char   *tail;   /* end of actual packet data */
    unsigned char   *end;    /* end of buffer allocation */
    
    /*  head ≤ data ≤ tail ≤ end  — always */
    
    /* Lengths */
    unsigned int    len;      /* length of actual data (data to tail) */
    unsigned int    data_len; /* length of paged data (for frags) */
    
    /* Device */
    struct net_device *dev;
    
    /* Routing */
    struct dst_entry *_skb_refdst;
    
    /* Timestamps */
    ktime_t         tstamp;
    
    /* Protocol */
    __be16          protocol;
    
    /* Transport/Network header offsets */
    __u16           transport_header;
    __u16           network_header;
    __u16           mac_header;
    
    /* Control block — protocol-private scratch space */
    char            cb[48];
    
    /* Marks for filtering/routing */
    __u32           mark;
    __u32           priority;
    
    /* Fragmentation */
    struct sk_buff  *frag_list;
    skb_frag_t      frags[MAX_SKB_FRAGS];
};
```

**Header pointer navigation:**
```c
skb_mac_header(skb)       /* pointer to ethernet header */
skb_network_header(skb)   /* pointer to IP header */
skb_transport_header(skb) /* pointer to TCP/UDP header */

/* Push/pull (move data pointer, adjusting for encapsulation) */
skb_push(skb, len)  /* extend data backward (add header) */
skb_pull(skb, len)  /* shrink data forward (consume header) */
skb_put(skb, len)   /* extend tail (add to end) */
```

**Mental model questions:**
- Draw the memory layout of an sk_buff for an incoming TCP packet with
  Ethernet header. Show where head, data, tail, end point.
- When skb_pull() is called to consume the Ethernet header, what changes?
  What stays the same?
- What is the difference between skb->len and skb->data_len?
  What does a nonzero data_len indicate?
- Why does the kernel avoid copying packet data when possible? What mechanism
  allows the network stack to share packet data? (hint: skb_clone, skb_copy)
- What is skb_linearize() for and when must you call it?
- What does `skb->cb` (the control block) give you? Who uses it?
  (hint: TCP uses it for sequence numbers during processing)

**Source reference:**  
`include/linux/skbuff.h` — struct sk_buff (2000+ lines; read it all)  
`net/core/skbuff.c` — sk_buff operations

---

### 8.2 net_device — The Network Device Abstraction

```c
struct net_device {
    char            name[IFNAMSIZ];  /* "eth0", "lo", etc. */
    unsigned long   state;           /* device state flags */
    
    /* Statistics */
    struct net_device_stats stats;
    
    /* Operations — the dispatch table */
    const struct net_device_ops *netdev_ops;
    const struct ethtool_ops    *ethtool_ops;
    
    /* Hardware address */
    unsigned char   dev_addr[MAX_ADDR_LEN];
    
    /* MTU and header */
    unsigned int    mtu;
    unsigned short  type;       /* ARPHRD_ETHER, etc. */
    unsigned short  hard_header_len;
    
    /* TX queue(s) */
    struct netdev_queue *_tx;
    unsigned int    num_tx_queues;
    
    /* Namespace */
    possible_net_t  nd_net;    /* network namespace */
};
```

**net_device_ops — the driver dispatch table:**
```c
struct net_device_ops {
    int         (*ndo_open)(struct net_device *dev);
    int         (*ndo_stop)(struct net_device *dev);
    netdev_tx_t (*ndo_start_xmit)(struct sk_buff *skb,
                                   struct net_device *dev);
    int         (*ndo_set_mac_address)(struct net_device *dev, void *addr);
    int         (*ndo_change_mtu)(struct net_device *dev, int new_mtu);
    struct net_device_stats *(*ndo_get_stats)(struct net_device *dev);
    /* ... many more ... */
};
```

**Mental model questions:**
- What is the lifetime of a net_device? When is it allocated, registered,
  unregistered, freed?
- Why does ndo_start_xmit return netdev_tx_t rather than void?
  What does NETDEV_TX_BUSY mean?
- What is the relationship between net_device and the network namespace?

---

### 8.3 Packet Receive Path (RX)

```
NIC DMA → ring buffer → hardirq → NAPI poll → netif_receive_skb()
                                                       │
                            ┌──────────────────────────┘
                            ▼
                    netif_receive_skb()
                            │
                   Packet Type handlers
                   (ptype_base hash table)
                            │
                    ┌───────┴────────┐
                    │    ARP         │  → arp_rcv()
                    │    IPv4        │  → ip_rcv()
                    │    IPv6        │  → ipv6_rcv()
                    │    802.1Q VLAN │  → vlan_do_receive()
                    └────────────────┘
                            │
                         ip_rcv()
                            │
                    Netfilter PRE_ROUTING
                            │
                    Routing decision
                      (ip_route_input)
                            │
                   ┌────────┴────────┐
                   │ For local       │  → ip_local_deliver()
                   │ For forward     │  → ip_forward()
                   └─────────────────┘
```

**Source reference:**  
`net/core/dev.c` — netif_receive_skb(), net_rx_action()  
`net/ipv4/ip_input.c` — ip_rcv()  
`net/ipv4/route.c` — ip_route_input()

---

### 8.4 Netfilter Architecture

**Hooks:**
```
PREROUTING → FORWARD → POSTROUTING
                          (for forwarded packets)

PREROUTING → INPUT
                (for locally destined packets)

OUTPUT → POSTROUTING
                (for locally generated packets)
```

**Registering a hook:**
```c
static unsigned int my_hook(void *priv, struct sk_buff *skb,
                             const struct nf_hook_state *state)
{
    /* Inspect or modify skb */
    if (should_drop(skb))
        return NF_DROP;
    return NF_ACCEPT;
}

static struct nf_hook_ops my_ops = {
    .hook     = my_hook,
    .pf       = NFPROTO_IPV4,
    .hooknum  = NF_INET_PRE_ROUTING,
    .priority = NF_IP_PRI_FIRST,
};

nf_register_net_hook(net, &my_ops);
```

---

### 8.5 XDP — eXpress Data Path

**What it is:**  
XDP hooks into the driver's receive path before sk_buff allocation,
giving the fastest possible packet processing path. Used for DDoS
mitigation, load balancing, and packet filtering at line rate.

**Hook points (earliest to latest):**

```
NIC hardware
    │
    ▼
DMA ring buffer (driver)
    │
    ├── XDP_TX / XDP_DROP / XDP_PASS / XDP_REDIRECT  ← XDP hook here
    │
    ▼
sk_buff allocation
    │
    ▼
Netfilter hooks
```

**BPF program skeleton:**
```c
SEC("xdp")
int my_xdp_prog(struct xdp_md *ctx)
{
    void *data     = (void *)(long)ctx->data;
    void *data_end = (void *)(long)ctx->data_end;
    
    struct ethhdr *eth = data;
    if ((void *)(eth + 1) > data_end)
        return XDP_DROP;
    
    if (eth->h_proto == htons(ETH_P_IP)) {
        struct iphdr *iph = (void *)(eth + 1);
        if ((void *)(iph + 1) > data_end)
            return XDP_DROP;
        /* inspect iph->saddr, iph->daddr */
    }
    
    return XDP_PASS;
}
```

---

## MODULE 9: Advanced Topics

### 9.1 Kernel Memory Models and LKMM

**What it is:**  
The Linux Kernel Memory Model (LKMM) formally defines what orderings the
kernel provides across CPUs. Essential for lock-free algorithms and RCU.

**Source reference:**  
`tools/memory-model/` — the formal LKMM model  
`Documentation/memory-barriers.txt`

---

### 9.2 PREEMPT_RT (Real-Time Linux)

**What it is:**  
Makes most of the kernel fully preemptible by converting spinlocks to
sleeping locks (rt_mutex under the hood). Requires raw_spinlock_t for
the few cases that truly cannot sleep.

**Impact on your code:**
- `spin_lock()` becomes `rt_spin_lock()` (can sleep)
- `spinlock_t` must not be used in true atomic context
- `raw_spinlock_t` is used for code that runs on the true non-preemptible path

---

### 9.3 NUMA-Aware Programming

**What it is:**  
On NUMA (Non-Uniform Memory Access) systems, memory access latency depends
on whether the memory is local to the accessing CPU's node.

**Key APIs:**
```c
numa_node_id()                  /* current NUMA node */
kmalloc_node(size, flags, node) /* allocate on specific node */
alloc_pages_node(node, gfp, order)
```

**Mental model questions:**
- Why does the network stack use per-CPU structures instead of per-NUMA-node?
- When does NUMA-aware allocation matter for network driver performance?

---

### 9.4 cgroups v2 and Network Namespaces

**Relevance to network security work:**
- Network namespaces isolate the entire network stack per namespace
- Each namespace has its own: interfaces, routing tables, netfilter rules,
  sockets, /proc/net/ view
- Used by containers (Docker, Kubernetes) for network isolation

**Source reference:**  
`net/core/net_namespace.c`  
`include/net/net_namespace.h`

---

## MODULE 10: Testing and Validation

### 10.1 KUnit — Kernel Unit Testing

```c
#include <kunit/test.h>

static void my_test_case(struct kunit *test)
{
    int result = my_function(5);
    KUNIT_EXPECT_EQ(test, result, 10);
    KUNIT_EXPECT_NOT_NULL(test, my_alloc_func());
}

static struct kunit_case my_test_cases[] = {
    KUNIT_CASE(my_test_case),
    {}
};

static struct kunit_suite my_test_suite = {
    .name  = "my_module_tests",
    .test_cases = my_test_cases,
};

kunit_test_suite(my_test_suite);
```

**Run tests:**
```bash
./tools/testing/kunit/kunit.py run --kunitconfig=.kunit
```

---

### 10.2 kselftest — Kernel Self Tests

Location: `tools/testing/selftests/`  
Network-specific: `tools/testing/selftests/net/`

**Test categories relevant to network work:**
- `net/` — socket tests, routing tests, netfilter tests
- `bpf/` — BPF program tests
- `tc-testing/` — traffic control tests
- `drivers/net/` — network driver tests

---

### 10.3 Test Strategy for Kernel Code

**Unit level:** KUnit for individual functions and data structures  
**Integration level:** kselftest with network namespaces for network stack  
**System level:** VM-based tests (your KVM guest setup is correct)  
**Fault injection:** `CONFIG_FAULT_INJECTION` — simulate allocation failures  
**Fuzzing:** syzkaller — the kernel's primary fuzzer for syscall interfaces  

**The critical principle:**  
Never test network stack changes on the host machine's production
networking. Always use a dedicated VM with a serial console for control.

---

## Recommended Learning Path

### Phase 1 — Foundation (Weeks 1–4)
1. Read `Documentation/process/coding-style.rst` — the style you will write
2. Build the kernel from source with all debug options enabled
3. Write a character device driver with proc interface
4. Write a kernel module that creates a linked list and rbtree
5. Read `include/linux/skbuff.h` in full

### Phase 2 — Synchronization (Weeks 5–8)
1. Study `Documentation/memory-barriers.txt` — read it twice
2. Write a data structure protected by spinlock, then mutex, compare
3. Implement an RCU-protected linked list from scratch
4. Enable lockdep, introduce a lock order bug, study the splat
5. Read `kernel/rcu/tree.c` introduction comments

### Phase 3 — Networking Internals (Weeks 9–16)
1. Trace an incoming TCP packet from NIC interrupt to socket receive buffer
2. Write a Netfilter hook that logs packet metadata
3. Write a simple XDP program and benchmark vs Netfilter
4. Study `net/ipv4/route.c` — routing table and RCU usage
5. Study `net/core/dev.c` — NAPI receive path
6. Read `net/ipv4/tcp_input.c` and `tcp_output.c` for synchronization patterns

### Phase 4 — Advanced (Weeks 17+)
1. Contribute a documentation fix to the kernel (understand the submission process)
2. Implement a simple network protocol module (PF_PACKET or custom)
3. Write and benchmark an XDP-based packet filter
4. Study a complete network driver (virtio_net is readable)

---

## Essential Reference Materials

### Source Trees to Bookmark
```
include/linux/          — core kernel headers
kernel/                 — core kernel (sched, locking, rcu, workqueue)
mm/                     — memory management
net/core/               — network core
net/ipv4/               — IPv4 stack
drivers/net/ethernet/   — Ethernet drivers
Documentation/          — must-read documentation
```

### Books (in order of priority)
1. **Linux Kernel Development** — Robert Love (3rd ed.) — architecture overview
2. **Understanding the Linux Kernel** — Bovet & Cesati — deep internals
3. **Linux Device Drivers** — Corbet, Rubini, Kroah-Hartman (LDD3, free online)
4. **Linux Kernel Networking** — Rami Rosen — network stack
5. **Is Parallel Programming Hard?** — Paul McKenney (free online) — RCU and memory models

### Online Resources
- `https://kernelnewbies.org` — changelog explanations, tutorials
- `https://lwn.net` — the authoritative kernel development news source
- `https://lore.kernel.org` — all kernel mailing list archives
- `https://elixir.bootlin.com` — cross-referenced kernel source
- `https://docs.kernel.org` — official documentation

### Mailing Lists to Follow
- `linux-kernel@vger.kernel.org` — main development list
- `netdev@vger.kernel.org` — network subsystem development
- `linux-mm@kvack.org` — memory management

---

## Mental Checklist Before Writing Any Kernel Code

Before writing a single line of kernel code, answer these questions:

```
[ ] In which execution context will this code run?
[ ] Can this code path sleep? Should it?
[ ] What is the locking strategy? Which lock protects which data?
[ ] What GFP flags will I use for memory allocation? Why?
[ ] Are there any user-space pointers that must go through copy_to/from_user?
[ ] What is the cleanup/error path? Can I guarantee it is always reached?
[ ] Will this code be called from interrupt context? softirq?
[ ] Are there any reference counting concerns?
[ ] On what CPU architectures must this work? Any ordering concerns?
[ ] What happens on SMP with this code? Draw the concurrent execution scenario.
[ ] What happens when kmalloc returns NULL?
[ ] Have I reviewed the relevant kernel documentation in Documentation/?
```

---

*Last updated for kernel series: 6.x*  
*This document should be treated as a living reference — update it as you go deeper.*
