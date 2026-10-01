# The Complete Linux Kernel Processes Guide: Building a Deep Mental Model

## Table of Contents

1. [Fundamental Concepts](#fundamental-concepts)
2. [Process Representation in Kernel](#process-representation-in-kernel)
3. [Process States and Transitions](#process-states-and-transitions)
4. [Process Creation](#process-creation)
5. [Process Scheduling](#process-scheduling)
6. [Context Switching](#context-switching)
7. [Process Termination](#process-termination)
8. [Process Synchronization](#process-synchronization)
9. [Signals and IPC](#signals-and-ipc)
10. [Memory Management](#memory-management)
11. [Real Kernel Implementations](#real-kernel-implementations)
12. [Advanced Topics](#advanced-topics)

---

## 1. Fundamental Concepts

### 1.1 What is a Process?

A **process** is the fundamental abstraction for running a program in a protected environment. It's an instance of a program in execution with its own:

- **Address space**: Virtual memory isolated from other processes
- **Execution context**: Register values, instruction pointer, stack
- **Resources**: File descriptors, network connections, IPC handles
- **Scheduling state**: Priority, scheduling class, CPU affinity
- **Identity**: PID (Process ID), UID (User ID), GID (Group ID)

### 1.2 Process vs Thread vs Program

```
┌─────────────────────────────────────────────────────────────────┐
│                        PROGRAM (Code on disk)                   │
└─────────────────────────────────────────────────────────────────┘
                              ↓
        ┌─────────────────────────────────────────┐
        │          PROCESS (Program in Execution)  │
        │  ┌──────────────────────────────────┐   │
        │  │     Virtual Address Space        │   │
        │  │  ┌────────────┐                  │   │
        │  │  │   Stack    │                  │   │
        │  │  ├────────────┤                  │   │
        │  │  │   Heap     │                  │   │
        │  │  ├────────────┤                  │   │
        │  │  │   Data     │                  │   │
        │  │  ├────────────┤                  │   │
        │  │  │   Text     │                  │   │
        │  │  └────────────┘                  │   │
        │  └──────────────────────────────────┘   │
        │  ┌──────────────────────────────────┐   │
        │  │  Resources (Files, Pipes, etc)   │   │
        │  └──────────────────────────────────┘   │
        │  ┌──────────────────────────────────┐   │
        │  │  THREADS (Lightweight processes) │   │
        │  │  - Share address space          │   │
        │  │  - Independent execution        │   │
        │  │  - Stack per thread             │   │
        │  └──────────────────────────────────┘   │
        └─────────────────────────────────────────┘
```

**Key Differences:**

| Aspect | Program | Process | Thread |
|--------|---------|---------|--------|
| **Location** | Disk | Memory | Memory |
| **Address Space** | N/A | Isolated | Shared |
| **Creation Overhead** | N/A | Heavy | Light |
| **Context Switch Cost** | N/A | High | Lower |
| **Communication** | N/A | IPC (pipes, sockets) | Shared memory |
| **Scheduling** | N/A | Independently | By scheduler |

### 1.3 Process Hierarchy

```
        init (PID 1)
           │
    ┌──────┼──────┐
    │      │      │
  bash   systemd  sshd
    │      │      │
  apps  services clients
```

Every process (except init) has a **parent process**. When a parent terminates, children become **orphans** and are adopted by init or a subreaper.

---

## 2. Process Representation in Kernel

### 2.1 The Task Structure (task_struct)

The Linux kernel represents each process with a **task_struct** - a comprehensive data structure containing all process information. This is the most important data structure in Linux.

#### Conceptual Layout:

```
┌──────────────────────────────────────────────────────────────────┐
│                        task_struct                               │
├──────────────────────────────────────────────────────────────────┤
│ IDENTIFICATION                                                   │
│  • pid_t pid - Process ID                                       │
│  • pid_t tgid - Thread Group ID                                 │
│  • pid_t real_parent_pid - Parent's PID                         │
│  • struct task_struct *parent - Parent pointer                  │
│  • struct list_head children - List of child processes          │
│  • uid_t uid, euid, suid, fsuid - User IDs                      │
│  • gid_t gid, egid, sgid, fsgid - Group IDs                     │
├──────────────────────────────────────────────────────────────────┤
│ EXECUTION STATE                                                  │
│  • volatile long state - Process state (RUNNING, INTERRUPTIBLE) │
│  • unsigned int flags - Process flags                           │
│  • struct thread_info *thread_info - CPU state                  │
│  • struct pt_regs *thread_regs - Register state                 │
│  • unsigned long personality - Execution domain                 │
├──────────────────────────────────────────────────────────────────┤
│ SCHEDULING INFORMATION                                          │
│  • int prio - Priority (0-139, 0-99 realtime, 100-139 normal)   │
│  • int static_prio - Static priority (never changes)            │
│  • int normal_prio - Normal priority (affected by RT boost)     │
│  • unsigned int rt_priority - Real-time priority                │
│  • struct sched_entity se - Scheduling entity (CFS)             │
│  • unsigned int policy - Scheduling policy (SCHED_NORMAL, etc)  │
│  • const struct sched_class *sched_class - Scheduler class      │
│  • unsigned long cpus_allowed - CPU affinity mask               │
├──────────────────────────────────────────────────────────────────┤
│ MEMORY MANAGEMENT                                               │
│  • struct mm_struct *mm - Memory management structure           │
│  • struct mm_struct *active_mm - Active memory context          │
│  • unsigned long total_vm - Total virtual memory                │
│  • unsigned long locked_vm - Locked pages                       │
│  • unsigned long data_vm - Data segment size                    │
│  • unsigned long text_vm - Text segment size                    │
│  • struct vm_area_struct *vmas - Virtual memory areas           │
├──────────────────────────────────────────────────────────────────┤
│ SIGNAL HANDLING                                                 │
│  • struct signal_struct *signal - Signal table                  │
│  • sigset_t blocked - Blocked signals                           │
│  • sigset_t pending - Pending signals                           │
│  • struct sighand_struct *sighand - Signal handlers             │
├──────────────────────────────────────────────────────────────────┤
│ FILE AND FILESYSTEM                                             │
│  • struct files_struct *files - Open file descriptors           │
│  • struct fs_struct *fs - Filesystem info (cwd, root)           │
│  • int link_count - Count of hard links                         │
│  • struct dentry *pwd - Current working directory               │
│  • struct dentry *root - Root directory                         │
├──────────────────────────────────────────────────────────────────┤
│ TIMING                                                          │
│  • u64 utime - User CPU time (jiffies)                          │
│  • u64 stime - System CPU time (jiffies)                        │
│  • u64 vruntime - Virtual runtime (CFS scheduler)               │
│  • unsigned long long start_time - Process start time           │
│  • cputime_t gtime - Guest time                                 │
├──────────────────────────────────────────────────────────────────┤
│ COMMUNICATION                                                   │
│  • struct rlimit rlim[] - Resource limits                       │
│  • int exit_code - Exit status code                             │
│  • int exit_signal - Signal sent to parent on exit              │
│  • bool pdeath_signal - Parent death signal                     │
├──────────────────────────────────────────────────────────────────┤
│ KERNEL STACK AND CONTEXT                                        │
│  • void *stack - Kernel stack pointer                           │
│  • struct thread_struct thread - Architecture-specific state    │
│  • unsigned int thread_flags - Thread-specific flags            │
│  • unsigned long *pgd - Page directory                          │
├──────────────────────────────────────────────────────────────────┤
│ MISCELLANEOUS                                                   │
│  • struct list_head tasks - Global task list                    │
│  • struct list_head sibling - Sibling task list                 │
│  • pid_t session - Session ID                                   │
│  • pid_t pgrp - Process group ID                                │
│  • char *comm[TASK_COMM_LEN] - Command name                     │
│  • struct linux_binfmt *binfmt - Binary format                  │
│  • void *security - SELinux security context                    │
│  • audit_context - Audit context                                │
└──────────────────────────────────────────────────────────────────┘
```

### 2.2 Key Embedded Structures

#### 2.2.1 sched_entity (Scheduling Entity)

Used by CFS (Completely Fair Scheduler) to track process scheduling:

```c
struct sched_entity {
    struct load_weight      load;           /* For load balancing */
    struct rb_node          run_node;       /* Red-black tree node in runqueue */
    struct list_head        group_node;     /* List for group scheduling */
    unsigned int            on_rq;          /* On run queue? */
    
    u64                     exec_start;     /* Start of current run */
    u64                     sum_exec_runtime;  /* Total execution time */
    u64                     vruntime;       /* Virtual runtime - KEY! */
    u64                     prev_sum_exec_runtime;
    
    u64                     nr_migrations;  /* Number of migrations */
    
    struct sched_statistics stats;
    
    #ifdef CONFIG_FAIR_GROUP_SCHED
    struct sched_entity    *parent;
    struct cfs_rq          *cfs_rq;
    struct cfs_rq          *my_q;
    unsigned long           depth;
    #endif
};
```

**Virtual Runtime (vruntime)**: The core concept of CFS
- Tracks "ideal" CPU time if all processes shared CPU equally
- Weighted by process priority
- Always increasing, even when process sleeps (to prevent starvation)
- Formula: `vruntime = actual_runtime * (NICE_0_LOAD / task_load_weight)`

#### 2.2.2 thread_info (CPU/Architecture State)

Contains CPU-specific execution state:

```c
struct thread_info {
    unsigned long           flags;          /* Thread flags */
    unsigned long           status;         /* TS_* bits */
    __u32                   cpu;            /* Current CPU */
    int                     preempt_count;  /* Preemption counter */
    unsigned long           addr_limit;     /* User space limit */
    void                    *sysenter_return;
    unsigned long           syscall_work;   /* Syscall tracing */
    struct task_struct      *task;          /* Back pointer to task_struct */
};
```

#### 2.2.3 mm_struct (Memory Management)

Represents the virtual address space:

```c
struct mm_struct {
    struct {
        struct vm_area_struct *mmap;       /* List of VMAs */
        struct rb_root          mm_rb;     /* Red-black tree of VMAs */
        u64                     vmacache_seqnum;  /* Cache version */
    } __randomize_layout;
    
    unsigned long           total_vm;       /* Total pages */
    unsigned long           locked_vm;      /* Locked pages */
    unsigned long           pinned_vm;      /* Pinned pages */
    unsigned long           data_vm;        /* DATA+STACK */
    unsigned long           exec_vm;        /* VM_EXEC */
    unsigned long           stack_vm;       /* Stack region */
    unsigned long           start_code, end_code, start_data, end_data;
    unsigned long           start_brk, brk, start_stack;
    unsigned long           arg_start, arg_end, env_start, env_end;
    
    unsigned long           saved_auxv[AT_VECTOR_SIZE];
    struct m_area_struct    *mmap_cache;
    
    struct {
        unsigned long   pgfault, pgmajfault, pswpin, pswpout;
    } fault_stat;
    
    unsigned long           highest_vm_end;
    pgd_t                   *pgd;           /* Page directory */
    
    atomic_t                mm_users;       /* User count */
    atomic_t                mm_count;       /* Reference count */
    int                     map_count;      /* Number of VMAs */
    
    spinlock_t              page_table_lock;
    struct rw_semaphore     mmap_lock;
    struct list_head        mmlist;
};
```

### 2.3 Process Control Block (PCB) - High Level View

```
PROCESS CONTROL BLOCK (task_struct)
│
├─ Identity Info
│  ├─ pid: 1234
│  ├─ ppid: 1000
│  ├─ uid: 1000
│  └─ gid: 1000
│
├─ Memory Info (mm_struct)
│  ├─ Page Directory (pgd)
│  ├─ VMA List
│  ├─ Heap bounds
│  └─ Stack bounds
│
├─ Execution Context
│  ├─ Registers (in thread_info)
│  ├─ Program Counter
│  ├─ Stack Pointer
│  └─ Status Flags
│
├─ Scheduling Info
│  ├─ Priority: 120
│  ├─ Policy: SCHED_NORMAL
│  ├─ CPU Affinity: 0x0F
│  ├─ vruntime: 1234567890
│  └─ on_rq: true
│
├─ Resources
│  ├─ Files (open FDs)
│  ├─ Signals
│  ├─ Timers
│  └─ Resource Limits
│
└─ Accounting
   ├─ CPU Time Used
   ├─ Page Faults
   ├─ Context Switches
   └─ System Calls Made
```

---

## 3. Process States and Transitions

### 3.1 Process States

```
                         ┌──────────────────────────┐
                         │    NEW/CREATED           │
                         │  (allocated but not      │
                         │   yet scheduled)         │
                         └────────────┬─────────────┘
                                      │
                                      │ wake_up()
                                      ↓
         ┌─────────────────────────────────────────────────┐
         │                                                 │
         ↓                                                 ↓
    ┌─────────────┐                               ┌──────────────┐
    │   RUNNING   │◄──────────────────────────────│  RUNNABLE    │
    │             │    schedule_out()             │ (Ready Queue)│
    └──────┬──────┘                               └──────▲───────┘
           │                                             │
           │                                      wake_up()
           │ (I/O, sleep, yield)          (interrupt handled)
           │                                             │
           ↓                                      ┌──────┴──────────┐
    ┌──────────────────┐                         │                 │
    │  INTERRUPTIBLE   │                    ┌────┴────┐       ┌────┴────┐
    │  SLEEP (S)       │                    │ WAITING │       │ BLOCK   │
    │  - Can wake      │                    │ FOR I/O │       │ I/O (D) │
    │  - Wakeable      │────────────────────│         │       │         │
    └──────┬───────────┘  interruptible     └─────────┘       └────┬────┘
           │             syscall                                    │
           │                              ┌──────────────────────────┘
           │                              │
           │                              │ I/O Complete
           │                              │ interrupt handler
           │                              ↓
           │                    ┌──────────────────┐
           │                    │ Wakeup callback  │
           └───────────────────►│ add to runqueue  │
                wake_up()        └──────────────────┘
                                         │
                                         ↓
                              ┌────────────────────┐
                              │  RUNNABLE AGAIN    │
                              │ (in run queue)     │
                              └────────────────────┘


    UNINTERRUPTIBLE_SLEEP (D) - Cannot be woken by signals
                │
                ├─ Mutex waiting
                ├─ Disk I/O
                ├─ Page faults
                └─ Critical sections


    ┌──────────────────────────┐
    │      exit() called       │
    └────────────┬─────────────┘
                 │
                 ↓
    ┌──────────────────────────┐
    │    ZOMBIE (Z)            │
    │    - Exited              │
    │    - Resources freed     │
    │    - Waiting for parent  │
    │      to call wait()/     │
    │      waitpid()           │
    └────────────┬─────────────┘
                 │
        parent calls waitpid()
                 │
                 ↓
    ┌──────────────────────────┐
    │    TERMINATED/REAPED     │
    │    (completely removed)  │
    └──────────────────────────┘
```

### 3.2 Linux Kernel State Values

```c
/* From include/linux/sched.h */

#define TASK_RUNNING            0x0000  /* Actively running */
#define TASK_INTERRUPTIBLE      0x0001  /* Sleeping, can wake */
#define TASK_UNINTERRUPTIBLE    0x0002  /* Sleeping, cannot wake by signal */
#define __TASK_STOPPED          0x0004  /* Stopped by signal */
#define __TASK_TRACED           0x0008  /* Traced by debugger */
#define EXIT_DEAD               0x0010  /* Zombie, dead */
#define EXIT_ZOMBIE             0x0020  /* Zombie, waiting for reap */
#define TASK_DEAD               0x0040  /* Dead, removed from queue */
#define TASK_WAKEKILL           0x0100  /* Wake on signal (if interruptible) */
#define TASK_WAKING             0x0200  /* Being woken up */
#define TASK_PARKED             0x0400  /* Parked for execution */
#define TASK_NOLOAD             0x0800  /* Doesn't count to load average */
#define TASK_NEW                0x1000  /* Newly created, not yet scheduled */
#define TASK_STATE_MAX          0x2000  /* Highest valid state */
```

### 3.3 State Transition Examples

**Example 1: Process starts and runs**
```
1. Kernel loads executable
   task_struct created
   state = TASK_NEW

2. Task added to runqueue
   state = TASK_RUNNING (when scheduled)

3. Scheduler gives CPU
   starts executing
   CPU registers loaded
```

**Example 2: Process reads from disk**
```
1. read() syscall
   Inode locked
   state = TASK_UNINTERRUPTIBLE (disk I/O)
   
2. Disk device reads file
   DMA transfers data
   Interrupt generated

3. Interrupt handler
   Wakeup callback
   state = TASK_RUNNING (added to runqueue)

4. Scheduler picks process
   Continues execution
   data available
```

**Example 3: Process sleeps waiting for event**
```
1. wait_for_condition()
   prepare_to_wait(&queue, &wait, TASK_INTERRUPTIBLE)
   state = TASK_INTERRUPTIBLE
   schedule() - give up CPU

2. Another process signals condition
   wake_up(&queue)

3. Scheduler moves to runqueue
   state = TASK_RUNNING (when scheduled)

4. Finish_wait() - cleanup
```

---

## 4. Process Creation

### 4.1 Understanding fork()

The `fork()` system call creates a **new process** by **duplicating** the calling process.

#### Fork Mechanism:

```
┌──────────────────────────────────────────┐
│   Parent Process (PID 1000)              │
│  ┌────────────────────────────────────┐  │
│  │   Address Space                    │  │
│  │  ┌──────────────┐                  │  │
│  │  │ Instructions │                  │  │
│  │  ├──────────────┤                  │  │
│  │  │ Global Data  │                  │  │
│  │  ├──────────────┤                  │  │
│  │  │ Heap         │                  │  │
│  │  ├──────────────┤                  │  │
│  │  │ Stack        │ ◄── fork() called│  │
│  │  └──────────────┘                  │  │
│  └────────────────────────────────────┘  │
│                                           │
│  fork() returns 1001 (child PID)          │
└──────────────────────────────────────────┘
                    │
        ┌───────────┴───────────┐
        ↓                       ↓
┌──────────────┐         ┌──────────────┐
│  Parent cont.│         │ Child (NEW)  │
│  PID 1000    │         │ PID 1001     │
│  ppid: 500   │         │ ppid: 1000   │
│              │         │              │
│ fork()       │         │ fork()       │
│ returns 1001 │         │ returns 0    │
│              │         │              │
│ Stack        │         │ Stack (copy) │
│ state 0x1000 │         │ state 0x1000 │
└──────────────┘         └──────────────┘
                                
        (Independent execution)
        Both continue from fork() point
```

### 4.2 Copy-on-Write (CoW)

Modern fork() uses **Copy-on-Write** optimization:

```
Before fork():
┌────────────────────────┐
│ Parent's Address Space │
│  Page A: value=100     │
│  Page B: value=200     │
│  Page C: value=300     │
└────────────────────────┘

Immediately after fork():
┌────────────────────────┐  ┌────────────────────────┐
│ Parent's VMA           │  │ Child's VMA            │
│ (mmap'd regions)       │  │ (mmap'd regions)       │
│  Page A (shared)    ───┼──┼── Page A (shared)      │
│  Page B (shared)    ───┼──┼── Page B (shared)      │
│  Page C (shared)    ───┼──┼── Page C (shared)      │
│ (both R/O until write) │  │ (both R/O until write) │
└────────────────────────┘  └────────────────────────┘
         │                             │
         │ Parent writes to Page A     │ Child writes to Page B
         │                             │
         ↓                             ↓
┌────────────────────────┐  ┌────────────────────────┐
│ Parent's Address Space │  │ Child's Address Space  │
│ Page A: value=999      │  │ Page A: (shared)       │
│ (new copy)             │  │ Page B: value=500      │
│ Page B: (shared)       │  │ (new copy)             │
│ Page C: (shared)       │  │ Page C: (shared)       │
└────────────────────────┘  └────────────────────────┘
```

**Advantages of CoW:**
- Avoid copying entire address space (huge overhead)
- Share read-only memory
- Copy only when write occurs
- Reduces memory usage dramatically

### 4.3 Fork Implementation (Simplified C Code)

```c
/* Simplified fork() implementation */

long sys_fork(struct pt_regs *regs)
{
    /* 1. Create new task_struct */
    struct task_struct *p = dup_task_struct(current);
    if (!p)
        return -ENOMEM;
    
    /* 2. Copy process resources */
    if (copy_process(SIGCHLD, 0, regs, 0, NULL, NULL) < 0)
        return -ENOMEM;
    
    /* 3. Copy file descriptors */
    p->files = dup_fd(current->files);
    
    /* 4. Copy signal handlers */
    p->sighand = current->sighand;
    
    /* 5. Copy memory structures (with CoW) */
    /* VMA lists are copied, but pages marked read-only */
    p->mm = dup_mm(current);
    
    /* 6. Copy scheduling info but clear execution state */
    p->prio = current->prio;
    p->static_prio = current->static_prio;
    p->vruntime = 0;  /* Start fresh in scheduler */
    
    /* 7. Set up parent-child relationship */
    p->parent = current;
    list_add_tail(&p->sibling, &current->children);
    
    /* 8. Set PID */
    p->pid = allocate_pid();
    
    /* 9. Wake up new process */
    wake_up_new_task(p);
    
    /* 10. Return child's PID to parent, 0 to child */
    return p->pid;
}
```

### 4.4 Exec Family (execve, execvp, etc.)

`exec()` **replaces** the current process image with a new program.

#### Exec vs Fork:

```
fork():
┌──────────────────────┐
│ New Process Created  │
│ Separate Memory      │
│ Separate PID         │
│ Separate Resources   │
└──────────────────────┘

exec():
┌──────────────────────┐
│ SAME Process         │
│ SAME PID             │
│ NEW Memory Image     │
│ NEW Code/Data        │
│ Reused Resources*    │
└──────────────────────┘
*Files, sockets, pipes inherited unless marked FD_CLOEXEC
```

#### Typical Usage Pattern:

```c
pid_t child = fork();
if (child == 0) {
    /* Child process */
    execvp("ls", (char*[]){"ls", "-la", NULL});
    perror("execvp failed");  /* Only reached on error */
    exit(1);
}
else {
    /* Parent process */
    int status;
    waitpid(child, &status, 0);
    printf("Child exited with status: %d\n", WEXITSTATUS(status));
}
```

### 4.5 Exec Implementation (Simplified)

```c
long sys_execve(const char *filename, 
                const char **argv, 
                const char **envp,
                struct pt_regs *regs)
{
    /* 1. Find the binary */
    struct file *file = open_exec(filename);
    if (!file)
        return -ENOENT;
    
    /* 2. Load the binary format handler */
    struct linux_binfmt *fmt = select_binfmt(file);
    if (!fmt)
        return -ENOEXEC;
    
    /* 3. Flush old memory - unmap all VMAs */
    flush_old_exec(bprm);
    
    /* 4. Create new address space */
    struct mm_struct *new_mm = mm_alloc();
    
    /* 5. Load program segments */
    fmt->load_binary(bprm);  /* Calls elf_load_binary or similar */
    
    /* 6. Load dynamic linker if needed (for ET_DYN) */
    if (elf_interpreter) {
        load_elf_interp(elf_interpreter);
    }
    
    /* 7. Copy new argv/envp to new stack */
    copy_strings_kernel(argc, argv, bprm);
    copy_strings_kernel(envc, envp, bprm);
    
    /* 8. Set up new program entry point */
    current->mm = new_mm;
    current->thread.rip = elf_entry;  /* Set instruction pointer */
    current->thread.rsp = new_stack;  /* Set stack pointer */
    
    /* 9. Reset signal handlers */
    flush_signal_handlers(current);
    
    /* 10. Return to user space with new code */
    return 0;  /* Kernel exit does switch to new code */
}
```

### 4.6 Process Creation Flow Diagram

```
User calls fork()
│
├─ Enters syscall boundary
│  (CPU mode switches to kernel)
│
├─ Kernel allocates new task_struct
│
├─ Kernel copies parent's structures:
│  ├─ Copy registers
│  ├─ Copy mm_struct (pages marked CoW)
│  ├─ Copy file descriptors
│  ├─ Copy signal handlers
│  ├─ Copy thread_info
│  └─ Initialize scheduling info
│
├─ Allocate new PID
│
├─ Insert into:
│  ├─ Global task list (for_each_process)
│  ├─ PID hash table
│  ├─ Parent's children list
│  └─ Run queue
│
├─ Return to user space:
│  ├─ Parent: return child_pid
│  └─ Child: return 0
│
└─ Both continue execution
   (only registers differ)
```

---

## 5. Process Scheduling

### 5.1 The Scheduler's Role

The **scheduler** is the most critical kernel component. It decides:
- Which process runs on which CPU
- When to switch between processes
- How long each process gets to run

```
┌──────────────────────────────────────────────────────────────┐
│                      CPU TIME SLICING                        │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  Time ──────────────────────────────────────────────────►   │
│   │                                                          │
│   ├─ Process A (25ms) ─┬─ Process B (20ms) ─┬─ Process C    │
│   │                   │                     │ (15ms)        │
│   │                   │                     │               │
│  ●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●  │
│   └─ Context Switch ─┘├─ Context Switch ───┘               │
│                       (TLB flush)         (Cache invalidate) │
│                                                              │
│  Each process gets time quantum (timeslice)                 │
│  Based on nice level and scheduler class                    │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

### 5.2 Scheduler Classes

The Linux kernel uses a **modular scheduler architecture** with multiple **scheduler classes**:

```
┌──────────────────────────────────────────────┐
│      Kernel Scheduler (schedule())           │
│                                              │
│  Chooses which scheduler class to use        │
│  (highest priority class first)              │
└──────────┬───────────────────────────────────┘
           │
           ├─► Stop Class (stop_sched_class)
           │   └─ Stop machine, kernel migration
           │      Priority: HIGHEST
           │
           ├─► Deadline Class (dl_sched_class)
           │   └─ Hard real-time (SCHED_DEADLINE)
           │      Period, deadline, runtime
           │      Priority: VERY HIGH
           │
           ├─► Real-time Class (rt_sched_class)
           │   └─ SCHED_FIFO: Run to completion
           │   └─ SCHED_RR: Round-robin
           │      Priority: HIGH (0-99)
           │
           ├─► CFS Class (fair_sched_class) ◄─ MOST COMMON
           │   └─ SCHED_NORMAL: Normal interactive
           │   └─ SCHED_BATCH: Batch jobs
           │   └─ SCHED_IDLE: Idle tasks
           │      Priority: NORMAL (100-139)
           │
           └─► Idle Class (idle_sched_class)
               └─ CPU idle tasks
                  Priority: LOWEST
```

### 5.3 CFS - Completely Fair Scheduler

CFS is the main scheduler for normal processes. **Core principle**: Give each process a fair share of CPU time based on nice level.

#### CFS Algorithm:

```
Virtual Runtime (vruntime) = elapsed_time × (weight_0 / weight_task)

Where weight is based on nice level:
nice -20 → highest weight
nice  0  → standard weight (1024)
nice +19 → lowest weight

Example (1-second window):
Process A (nice -5): weight = 1500
Process B (nice  0): weight = 1024
Process C (nice +5): weight = 682

Total weight = 1500 + 1024 + 682 = 3206

Process A ideal time: 1024 × (1500/3206) = 479ms
Process B ideal time: 1024 × (1024/3206) = 328ms
Process C ideal time: 1024 × (682/3206) = 218ms

Actual task assignments in timeslices:
A gets: 479 ms / slice = ~19-20 slices of 25ms each
B gets: 328 ms / slice = ~13 slices
C gets: 218 ms / slice = ~9 slices
```

#### CFS Data Structure:

```c
struct cfs_rq {
    struct load_weight load;           /* Total load */
    u32 nr_running;                    /* Number of runnable tasks */
    u32 h_nr_running;                  /* Hierarchical nr_running */
    
    u64 exec_clock;                    /* Total execution time */
    u64 min_vruntime;                  /* Minimum vruntime in queue */
    
    struct rb_root tasks_timeline;     /* Red-black tree of tasks */
    struct rb_node *rb_leftmost;       /* Leftmost (smallest vruntime) */
    
    struct sched_entity *curr;         /* Currently running task */
    struct sched_entity *next;         /* Next to run (optimization) */
    struct sched_entity *last;         /* Last run task */
    
    unsigned int idle_h_nr_running;    /* For idle task tracking */
};
```

#### CFS RB-Tree (Red-Black Tree):

```
CFS maintains all runnable tasks in a red-black tree ordered by vruntime:

                    ┌──────────────┐
                    │ Task vrt=100 │
                    └──────┬───────┘
                           │
            ┌──────────────┴──────────────┐
            │                             │
        ┌───────────┐                 ┌───────────┐
        │ Task vrt  │                 │ Task vrt  │
        │   = 50    │                 │   = 200   │
        └───┬───────┘                 └───────────┘
            │
    ┌───────┴────────┐
    │                │
┌──────────┐    ┌──────────┐
│vrt=20    │    │vrt=70    │
└──────────┘    └──────────┘

Property: 
├─ Left subtree = smaller vruntime
├─ Right subtree = larger vruntime
└─ Balanced (height = O(log n))

Next task to run = leftmost node (minimum vruntime)
```

#### CFS Pick Next Task:

```
pick_next_task_fair(rq):
    │
    ├─ Is current task still valid?
    │  ├─ Yes, check if should reschedule
    │  │  ├─ If not due for reschedule, return current
    │  │  └─ Otherwise, fall through
    │  └─ No, continue to next step
    │
    ├─ Get leftmost entity from rb_tree
    │  se = rb_leftmost(cfs_rq->tasks_timeline)
    │  # This has minimum vruntime = fairest choice
    │
    ├─ If se == NULL (no runnable task)
    │  return NULL (idle)
    │
    └─ Return se as next task to run
```

### 5.4 Scheduling Latency and Timeslice

```c
/* CFS scheduling parameters */
#define NICE_0_LOAD             1024
#define DEFAULT_TIMESLICE       (100 * HZ / 1000)  /* 100ms */

/* Latency-related */
unsigned int sched_latency_ns = 6000000;        /* 6ms */
unsigned int sched_min_granularity_ns = 750000; /* 0.75ms */

/* How latency is calculated:
   
   Effective timeslice = min(DEFAULT_TIMESLICE, 
                             sched_latency_ns / nr_runnable)
   
   Example: 4 tasks, sched_latency = 6ms
   Timeslice = 6ms / 4 = 1.5ms per task
   
   This maintains low latency while load increases
   
   If only 1 task:
   Timeslice = 6ms / 1 = 6ms
   Single task gets full CPU
*/
```

### 5.5 Real-Time Scheduler (SCHED_FIFO, SCHED_RR)

```
SCHED_FIFO (First In, First Out):
┌──────────────────────────────────────────┐
│ Real-time Task 1 (priority=90)           │
│ Runs to COMPLETION or until it blocks    │
│                                          │
│ ┌────────────────────────────────────┐   │
│ │ Task 1 Execution (entire timeslice)│   │
│ └────────────────────────────────────┘   │
│                                          │
│ Only preempted by higher priority tasks  │
└──────────────────────────────────────────┘

SCHED_RR (Round-Robin):
┌──────────────────────────────────────────┐
│ Real-time Task 1 (priority=90)           │
│ Round-robin scheduling within same       │
│ priority level                           │
│                                          │
│ ┌────────────────┐   ┌───────────────┐   │
│ │ Task 1 (25ms)  │→  │ Task 1 again  │   │
│ │ yields         │   │ (round robin) │   │
│ └────────────────┘   └───────────────┘   │
│                                          │
│ Within same priority: timeslice = 100ms  │
└──────────────────────────────────────────┘
```

### 5.6 Scheduling Code Flow

```
schedule() - Main scheduling function
│
├─ Preemption disabled (atomic section)
│
├─ current = rq->curr  /* Get current running task */
│
├─ For each scheduler class (in priority order):
│  ├─ rt_sched_class->pick_next_task()
│  │  └─ If found RT task, use it
│  │
│  ├─ fair_sched_class->pick_next_task()
│  │  ├─ find_leftmost in CFS tree
│  │  └─ Return CFS task
│  │
│  └─ idle_sched_class->pick_next_task()
│     └─ Return idle task
│
├─ next = picked_next_task
│
├─ If (next == current):
│  │  /* No change needed */
│  │  return  /* Let current continue */
│  └─
│
├─ Else:
│  ├─ Update accounting
│  │  ├─ current->se.sum_exec_runtime += delta
│  │  ├─ Update load averages
│  │  └─ Update statistics
│  │
│  ├─ Update CFS tree (remove current, reinsert with new vruntime)
│  │
│  ├─ context_switch(rq, current, next)
│  │  ├─ Save current task state
│  │  ├─ Restore next task state
│  │  ├─ TLB flush (if needed)
│  │  └─ Switch to next's address space
│  │
│  └─ rq->curr = next
│
└─ Enable preemption
   Return to next task execution

Note: This happens transparently - task doesn't know it was switched
```

---

## 6. Context Switching

### 6.1 What is Context Switching?

Context switching is the mechanism of **saving the state of one process and restoring another**. This is fundamental to multitasking.

```
Time: 0ms
┌─────────────────┐     Registers:
│ Process A       │     RIP: 0x400500
│ Running         │     RSP: 0x7ffefff0
│ Doing work      │     RAX: 0x1234
└─────────────────┘     etc.

┌──────────────────────────────┐
│ Scheduler decides to switch  │
│ (time quantum expired, or    │
│  higher priority awoken)     │
└──────────────────────────────┘
            │
            ↓
Time: 0.01ms
┌────────────────────────────────────┐
│ Context Switch (in progress)       │
│ Save Process A state to memory:    │
│  - All registers to task_struct    │
│  - Stack pointer saved             │
│  - Instruction pointer saved       │
│  - CR3 saved (page table pointer)  │
│  - Flags saved                     │
│  - FPU state saved                 │
└────────────────────────────────────┘
            │
            ↓
Time: 0.02ms
┌────────────────────────────────────┐
│ Restore Process B state:           │
│  - Load all registers from memory  │
│  - Set new stack pointer (RSP)     │
│  - Set new instruction pointer     │
│  - Load CR3 (flush TLB)            │
│  - Set new flags                   │
│  - Restore FPU state               │
└────────────────────────────────────┘
            │
            ↓
Time: 0.025ms
┌─────────────────┐
│ Process B       │     Registers:
│ Running         │     RIP: 0x401200
│ (resumed from   │     RSP: 0x7ffffef0
│  where it was   │     RAX: 0x5678
│  suspended)     │     etc.
└─────────────────┘
```

### 6.2 State Saved During Context Switch

```
Saved in task_struct.thread_info / thread_struct:

1. CPU Registers (architecture-specific):
   ┌─────────────────────────────────────┐
   │ General Purpose Registers (x86-64): │
   │  RAX, RBX, RCX, RDX, RSI, RDI       │
   │  R8-R15, RBP                        │
   └─────────────────────────────────────┘

2. Pointers:
   ┌─────────────────────────────────────┐
   │ RIP (Instruction Pointer)           │
   │ RSP (Stack Pointer)                 │
   │ RBP (Base Pointer)                  │
   └─────────────────────────────────────┘

3. Control Registers:
   ┌─────────────────────────────────────┐
   │ CR3: Page Directory Base (PDBR)     │
   │ EFLAGS: Status flags                │
   │ CR4: Control bits                   │
   └─────────────────────────────────────┘

4. FPU State (if used):
   ┌─────────────────────────────────────┐
   │ XMM0-XMM15: SIMD registers          │
   │ YMM0-YMM15: AVX registers           │
   │ ZMM0-ZMM31: AVX-512 registers       │
   │ MXCSR: FPU control word             │
   └─────────────────────────────────────┘

5. Segment Registers (x86-64, mostly kernel):
   ┌─────────────────────────────────────┐
   │ CS, DS, ES, SS, FS, GS              │
   │ (mostly kernel segments)            │
   └─────────────────────────────────────┘

6. Debug Registers (if debugger active):
   ┌─────────────────────────────────────┐
   │ DR0-DR7: Breakpoint registers       │
   └─────────────────────────────────────┘
```

### 6.3 Context Switch Implementation (Simplified x86-64)

```c
/* arch/x86/kernel/process.c - Simplified */

/* Save context before switch */
static inline void prepare_switch_to(struct task_struct *prev)
{
    /* Save floating point state if used */
    if (is_fpu_user(prev)) {
        save_fpu_state(&prev->thread.fpu);
    }
    
    /* Clear task debug registers to prevent leaks */
    if (prev->thread.debugreg7) {
        update_debugctlmsr(0);
    }
}

/* Actual context switch - must be precise */
void context_switch(struct rq *rq, struct task_struct *prev,
                    struct task_struct *next)
{
    struct mm_struct *mm, *oldmm;
    
    /* Save prev's FP/vector registers */
    prepare_switch_to(prev);
    
    /* Prepare next task */
    mm = next->mm;
    oldmm = prev->active_mm;
    
    /* Switch address space */
    if (!mm) {
        /* Kernel thread - reuse previous mm */
        next->active_mm = oldmm;
        atomic_inc(&oldmm->mm_count);
    } else {
        /* User process - switch to new mm */
        switch_mm(oldmm, mm, next);
    }
    
    /* Switch stacks and registers - assembly code */
    switch_to(prev, next, prev);
    /* After this line, we're running in next's context */
    barrier();  /* Prevent compiler reordering */
    
    /* Restore next's FP registers if needed */
    if (is_fpu_user(next)) {
        restore_fpu_state(&next->thread.fpu);
    }
}

/* Assembly macro - arch/x86/include/asm/switch_to.h */
#define switch_to(prev, next, last) \
    asm volatile("pushq %%rbp\n"                  /* Save prev's RBP */ \
                 "movq %%rsp, %P[threadrsp](%[prev])\n" /* Save prev's RSP */ \
                 "movq %P[threadrsp](%[next]), %%rsp\n" /* Load next's RSP */ \
                 "movq %%rsi, %P[threadrsi](%[prev])\n" /* Save prev's RSI */ \
                 "movq %P[threadrsi](%[next]), %%rsi\n" /* Load next's RSI */ \
                 /* ... more register saves/restores ... */ \
                 "jmp __switch_to\n"              /* Jump to switch code */ \
                 : [prev] "a" (prev), [next] "d" (next) \
                 : memory);
```

### 6.4 TLB Invalidation

**TLB (Translation Lookaside Buffer)**: CPU cache for virtual-to-physical address mappings.

```
Process A Page Tables:
Virtual Address 0x1000 → Physical Address 0x5000

TLB Entry: [VAddr 0x1000] → [PAddr 0x5000]

After context switch to Process B:
Virtual Address 0x1000 → Physical Address 0x8000 (different!)

PROBLEM: TLB still has old mapping!
         Reading address 0x1000 gives wrong memory

SOLUTION: Flush TLB when switching address spaces
         (Or use ASID tags on TLB entries)

┌──────────────────────────────────┐
│ Write new page table base to CR3 │
│ (Physical address of new PGD)    │
│                                  │
│ Hardware automatically flushes   │
│ all TLB entries when CR3 changes │
└──────────────────────────────────┘

Cost: TLB miss on every first access
      Must refill TLB from page tables
      Expensive operation!
```

### 6.5 Cost Analysis

```
Context Switch Cost Breakdown:

1. Save registers to memory:     ~10-20 CPU cycles
2. Restore registers:            ~10-20 CPU cycles
3. Flush TLB (implicit):         ~50-100 cycles (partial)
4. TLB refill misses:            ~1000-5000 cycles
5. Cache invalidation effects:   ~10000+ cycles
6. Memory barrier (if SMP):      ~100-1000 cycles

Total Direct Cost:               ~100-200 cycles

Indirect Costs (much larger):
- Cold cache for new task
- TLB misses until working set refilled
- Instruction cache misses
- Data cache conflicts
- Memory bandwidth contention

REAL COST: 1-10 microseconds per context switch
           (highly workload dependent)

Why context switches are expensive:
- CPUs are deeply pipelined
- Caches are filled with old task data
- Branch prediction is reset
- Modern CPUs can have 10+ instruction latency

Optimization: Per-CPU scheduling queues reduce lock contention
              Running queues reduce migration overhead
```

---

## 7. Process Termination

### 7.1 Process Exit Flow

```
User Process                          Kernel
│
├─ exit(status) syscall
│  └─ sends SIGTERM to all threads
│
└─ Returns from main()
   │
   └─► do_exit() kernel function
       │
       ├─ Disable preemption
       │
       ├─ BUG_ON() checks
       │  (ensure not in certain states)
       │
       ├─ Notify tracers (ptrace)
       │
       ├─ Flush exit code
       │  task_struct.exit_code = status
       │  task_struct.exit_signal = SIGCHLD
       │
       ├─ Signal parent process
       │  send_signal(parent, SIGCHLD, exit_code)
       │
       ├─ Reparent threads/children
       │  ├─ If process group leader:
       │  │  move children to init
       │  └─ If session leader:
       │     move process group to init
       │
       ├─ Exit CFS scheduler
       │  └─ Remove from runqueue
       │     mark state = EXIT_ZOMBIE
       │
       ├─ Free resources
       │  ├─ Close file descriptors
       │  │  for each fd:
       │  │    filp_close(fd)
       │  │
       │  ├─ Release locks
       │  │
       │  ├─ Unmap VMA regions
       │  │  for each vma:
       │  │    unmap_region()
       │  │
       │  ├─ Release semaphores/mutexes
       │  │
       │  ├─ Free memory pages
       │  │  free mm_struct
       │  │
       │  └─ Free signal handlers
       │     flush_signal_handlers()
       │
       ├─ Delete from process table
       │
       ├─ Set state = TASK_DEAD
       │
       └─ Schedule other tasks
          (context switch away)

Parent Process
│
├─ Receives SIGCHLD signal
│
└─ Calls waitpid(child_pid, &status, 0)
   │
   └─► sys_wait4() in kernel
       │
       ├─ Verify child exists
       │
       ├─ Read exit code
       │  status = child->exit_code
       │
       ├─ Release child's task_struct
       │  ├─ Remove from process lists
       │  ├─ Free memory
       │  └─ Return PID
       │
       └─► Parent gets status
           Child completely gone
```

### 7.2 Zombie Processes

```
Parent Exits Before Child:
┌───────────────────┐
│ Parent (PID 1000) │
│ exit()            │
└───────────────────┘
        │
        ├─ Calls exit_group()
        │
        ├─ Reparents children to init (PID 1)
        │  task->parent = init_task
        │
        └─ Exit complete


Child (PID 1001):
┌───────────────────┐
│ Still running     │
│ (still holds      │
│  resources)       │
│ exit()            │
└───────────────────┘
        │
        ├─ Becomes ZOMBIE
        │  state = EXIT_ZOMBIE
        │  
        ├─ Sends SIGCHLD to NEW parent (init)
        │
        └─ Waits for init to waitpid()


If parent does NOT call waitpid():
┌──────────────────────────────┐
│ Child becomes ORPHAN         │
│ Parent ignores SIGCHLD       │
│ task_struct not freed        │
│ Resource leak!               │
└──────────────────────────────┘
   → Zombie stays in process table
   → Wastes system resources
   → Can fill up pid table (DOS)
```

### 7.3 Proper Exit and Wait Pattern

```c
/* Parent process - proper handling */

pid_t child = fork();
if (child == -1) {
    perror("fork");
    exit(1);
}
else if (child == 0) {
    /* Child process */
    do_work();
    exit(EXIT_SUCCESS);  /* MUST exit! */
}
else {
    /* Parent process */
    int status;
    
    /* Reap child properly */
    pid_t result = waitpid(child, &status, 0);
    if (result == -1) {
        perror("waitpid");
        exit(1);
    }
    
    /* Check exit status */
    if (WIFEXITED(status)) {
        int exit_code = WEXITSTATUS(status);
        printf("Child exited with code: %d\n", exit_code);
    }
    else if (WIFSIGNALED(status)) {
        int sig = WTERMSIG(status);
        printf("Child killed by signal: %d\n", sig);
    }
    
    printf("Child fully cleaned up\n");
}
```

### 7.4 Process Termination Code (Simplified)

```c
/* kernel/exit.c - Simplified */

void do_exit(long code)
{
    struct task_struct *tsk = current;
    
    /* 1. Disable preemption - atomic operation */
    preempt_disable();
    
    /* 2. Sanity checks */
    BUG_ON(tsk->pid == 0);
    BUG_ON(!list_empty(&tsk->ptrace_entry));
    
    /* 3. Store exit code */
    tsk->exit_code = code;
    
    /* 4. Exit CFS scheduler */
    sched_exit(tsk);
    
    /* 5. Close all open files */
    struct files_struct *files = tsk->files;
    if (files) {
        for (int i = 0; i < files->max_fds; i++) {
            struct file *f = files->fdt[i];
            if (f) {
                filp_close(f, files);
            }
        }
    }
    
    /* 6. Release all memory regions */
    struct mm_struct *mm = tsk->mm;
    if (mm) {
        exit_mmap(mm);  /* Unmap all VMAs */
        mmput(mm);      /* Decrement reference count */
    }
    
    /* 7. Reparent any child processes */
    list_for_each_entry(child, &tsk->children, sibling) {
        child->parent = init_task;  /* Adopt to init */
    }
    
    /* 8. Notify parent */
    struct task_struct *parent = tsk->parent;
    send_signal(parent, SIGCHLD, code);
    
    /* 9. Set to zombie state */
    tsk->exit_state = EXIT_ZOMBIE;
    tsk->flags |= PF_EXITING;
    
    /* 10. Remove from runqueue if still there */
    dequeue_task(rq, tsk, 0);
    
    /* 11. Schedule next task - never returns */
    schedule();
}
```

---

## 8. Process Synchronization

### 8.1 Race Conditions

Multiple processes accessing shared resources simultaneously can cause **race conditions**.

```
Shared Resource: counter = 100
Process A and B both increment

Without synchronization:

Time │ Counter │ Process A    │ Process B
─────┼─────────┼──────────────┼────────────
0    │   100   │ READ(100)    │
1    │   100   │ ADD(1)       │ READ(100)
2    │   100   │ WRITE(101)   │ ADD(1)
3    │   101   │              │ WRITE(101)  ← WRONG! Should be 102

Result: counter = 101 (lost update)

With Synchronization (Mutex):

Time │ Counter │ Process A          │ Process B
─────┼─────────┼────────────────────┼──────────────────
0    │   100   │ LOCK(mutex)        │
1    │   100   │ READ(100)          │ LOCK(waits...)
2    │   100   │ ADD(1)             │
3    │   100   │ WRITE(101)         │
4    │   101   │ UNLOCK(mutex)      │
5    │   101   │                    │ LOCK(acquired)
6    │   101   │                    │ READ(101)
7    │   101   │                    │ ADD(1)
8    │   102   │                    │ WRITE(102)
9    │   102   │                    │ UNLOCK(mutex)

Result: counter = 102 (correct!)
```

### 8.2 Synchronization Primitives

#### 8.2.1 Spinlock (Busy-Wait Lock)

```c
struct spinlock {
    volatile unsigned int lock;  /* 0 = unlocked, 1 = locked */
};

void spin_lock(spinlock_t *lock)
{
    /* Atomically test and set */
    while (atomic_test_and_set(&lock->lock)) {
        /* Spin (busy-wait) until lock is free */
        cpu_relax();  /* Hint to CPU (pause instruction on x86) */
    }
}

void spin_unlock(spinlock_t *lock)
{
    atomic_set(&lock->lock, 0);
    barrier();  /* Prevent reordering */
}
```

**Usage:** 
- Kernel code only (not user space)
- Hold for very short time
- Disables preemption
- Good for protecting small critical sections

```
Spinlock Behavior:

Timeline:
CPU 0                       CPU 1
│                           │
├─ LOCK acquired            │
├─ Critical section         │
│  (accessing shared data)  │
│                      ┌────┴─ LOCK requested
│                      │
│                      ├─ SPIN (busy loop)
│                      ├─ CPU 1 loop
│                      ├─ loop loop
├─ Done, UNLOCK        │
│                      │
│                      ├─ LOCK acquired
│                      ├─ Critical section
│                      │
│                      └─ UNLOCK
```

#### 8.2.2 Mutex (Semaphore with count=1)

```c
struct mutex {
    atomic_t                count;      /* Locked(0) or Unlocked(1) */
    spinlock_t              wait_lock;
    struct list_head        wait_list;  /* Waiters */
    struct task_struct      *owner;     /* Current owner (for PI) */
};

void mutex_lock(struct mutex *lock)
{
    might_sleep();  /* Assert we can sleep */
    
    /* Fast path: try atomic decrement */
    if (atomic_dec_and_test(&lock->count))
        return;  /* Got it! */
    
    /* Slow path: sleep */
    __mutex_lock_slowpath(lock);
}

void __mutex_lock_slowpath(struct mutex *lock)
{
    /* Acquire wait_lock spinlock */
    spin_lock(&lock->wait_lock);
    
    /* Add ourselves to wait queue */
    add_waiter(&lock->wait_list, current);
    
    /* Release spinlock and sleep */
    set_task_state(current, TASK_INTERRUPTIBLE);
    spin_unlock(&lock->wait_lock);
    
    /* Sleep - will wake when lock available */
    schedule();
}

void mutex_unlock(struct mutex *lock)
{
    atomic_inc(&lock->count);
    
    /* Wake first waiter if any */
    if (!list_empty(&lock->wait_list)) {
        struct task_struct *waiter = list_first_entry(&lock->wait_list);
        wake_up_process(waiter);
    }
}
```

**Key Differences vs Spinlock:**
- Sleeps instead of spinning
- Uses wait queue instead of CPU cycles
- Can be held longer
- Slower to acquire/release (more overhead)
- Cannot be used in interrupt context

#### 8.2.3 Semaphore

```c
struct semaphore {
    atomic_t count;              /* Permits available */
    spinlock_t wait_lock;
    struct list_head wait_list;
};

void down(struct semaphore *sem)
{
    /* Try to acquire permit */
    if (atomic_dec_and_test(&sem->count))
        return;  /* Got one! */
    
    /* No permits available, sleep */
    spin_lock(&sem->wait_lock);
    add_waiter(sem, current);
    set_task_state(current, TASK_INTERRUPTIBLE);
    spin_unlock(&sem->wait_lock);
    schedule();
}

void up(struct semaphore *sem)
{
    atomic_inc(&sem->count);
    
    /* Wake one waiter */
    if (!list_empty(&sem->wait_list)) {
        struct task_struct *waiter = list_first_entry(&sem->wait_list);
        wake_up_process(waiter);
    }
}

/* Semaphore with initial value of 3 */
struct semaphore sem = SEMAPHORE_INIT(3);

/* Up to 3 processes can hold this simultaneously */
```

#### 8.2.4 RW-Lock (Reader-Writer Lock)

```c
struct rw_lock {
    int readers;           /* Count of readers */
    spinlock_t write_lock;
    struct list_head writer_waiters;
};

void read_lock(struct rw_lock *lock)
{
    spin_lock(&lock->write_lock);
    lock->readers++;
    spin_unlock(&lock->write_lock);
}

void read_unlock(struct rw_lock *lock)
{
    spin_lock(&lock->write_lock);
    lock->readers--;
    if (lock->readers == 0 && !list_empty(&lock->writer_waiters)) {
        wake_up_writer();
    }
    spin_unlock(&lock->write_lock);
}

void write_lock(struct rw_lock *lock)
{
    spin_lock(&lock->write_lock);
    
    /* Wait until no readers */
    while (lock->readers > 0) {
        add_to_writer_waiters(lock, current);
        spin_unlock(&lock->write_lock);
        schedule();
        spin_lock(&lock->write_lock);
    }
    /* Now have exclusive access */
}

void write_unlock(struct rw_lock *lock)
{
    /* Wake all readers and one writer */
    wake_up_all_readers();
    if (!list_empty(&lock->writer_waiters)) {
        wake_up_one_writer();
    }
    spin_unlock(&lock->write_lock);
}
```

**Use Case:** Database read-heavy workload
```
Multiple readers can proceed:
Reader 1 ─┐
Reader 2 ─┼─ Concurrent reads (fast)
Reader 3 ─┘

But exclusive writer blocks all:
Writer (blocks readers and other writers)
```

#### 8.2.5 Read-Copy-Update (RCU)

```
RCU: Most efficient synchronization for read-heavy workloads

Principle: Readers never block, writers update by copy

Original Data: ptr → [v1, v2, v3]

Reader 1 (old version):          Writer:
  read_lock()                     write_lock()
  a = ptr->v1                     new_data = copy(data)
  b = ptr->v2                     new_data.v1 = 999
  (no lock held!)                 
                                  ptr = &new_data
                                  (atomic swap)
  
  use a, b, c                     
  read_unlock()                   

  Sees old data: v1, v2, v3       After write:
                                  Old data still exists until
                                  all readers done

  Grace period expires:
  Free old data
```

### 8.3 Deadlock

```
Deadlock Conditions (all must be true):

1. Mutual Exclusion    ─ Resources can't be shared
2. Hold and Wait       ─ Processes hold resources while waiting
3. No Preemption       ─ Resources can't be taken away
4. Circular Wait       ─ Circular chain of waiting

    Process A               Process B
       │                       │
       ├─ LOCK(res1)          │
       │                  ├─ LOCK(res2)
       │                  │
       ├─ Wait LOCK(res2)      │
           (blocks)      ├─ Wait LOCK(res1)
                            (blocks)
       
       A waits for res2 (held by B)
       B waits for res1 (held by A)
       DEADLOCK!

Prevention strategies:
- Acquire locks in consistent order
- Use timeout on locks
- Use lock-free data structures
- Use RCU instead of locks
```

### 8.4 Synchronization Implementation (Rust)

```rust
// Mutex in Rust (safe by design)
use std::sync::Mutex;
use std::sync::Arc;
use std::thread;

fn main() {
    let counter = Arc::new(Mutex::new(0));
    
    let mut handles = vec![];
    
    for _ in 0..10 {
        let counter = Arc::clone(&counter);
        let handle = thread::spawn(move || {
            /* Compiler ensures proper locking */
            /* No deadlock possible (RAII cleanup) */
            let mut num = counter.lock().unwrap();
            *num += 1;
            /* Lock automatically released when 'num' dropped */
        });
        handles.push(handle);
    }
    
    for handle in handles {
        handle.join().unwrap();
    }
    
    println!("Result: {}", *counter.lock().unwrap());  /* Prints: 10 */
}
```

```rust
// RwLock for read-heavy workloads
use std::sync::RwLock;

fn main() {
    let data = RwLock::new(vec![1, 2, 3]);
    
    /* Multiple readers */
    let r1 = data.read().unwrap();
    let r2 = data.read().unwrap();  /* Multiple readers OK */
    println!("{:?}", r1);
    
    /* Exclusive writer */
    {
        let mut w = data.write().unwrap();  /* Blocks until readers done */
        w.push(4);  /* Exclusive access */
    }
}
```

---

## 9. Signals and IPC (Inter-Process Communication)

### 9.1 Signals: Software Interrupts

Signals are **asynchronous notifications** sent to processes. They interrupt normal execution.

```
Traditional CPU Interrupt:
External Event → CPU → Interrupt Handler → Resume

Signal (Software Interrupt):
Process/Kernel → Signal Delivery → Signal Handler → Resume


Signal Delivery Timeline:

Process Running
    │
    ├─ Event occurs (Ctrl+C, segfault, timer, another process)
    │
    ├─ Kernel checks if signal should be delivered
    │  ├─ Is signal blocked? (sigprocmask)
    │  ├─ Is process sleeping?
    │  └─ Which handler? (sigaction)
    │
    ├─ Interrupt user code at next opportunity
    │  (return from syscall, page fault, etc.)
    │
    ├─ Save execution context
    │  (register state → stack)
    │
    ├─ Jump to signal handler
    │  (RIP = handler address)
    │
    ├─ Signal Handler Runs
    │  ├─ Can do cleanup
    │  ├─ Can terminate
    │  └─ Can return with sigreturn
    │
    ├─ If handler returns:
    │  ├─ Restore saved context
    │  ├─ Resume original code
    │
    └─ Continue normally
```

### 9.2 Common Signals

```
SIGNAL              NUMBER   DEFAULT ACTION   SOURCE
─────────────────────────────────────────────────────────────────
SIGHUP              1        Terminate        Terminal hangup
SIGINT              2        Terminate        Ctrl+C
SIGQUIT             3        Core dump        Ctrl+\
SIGILL              4        Core dump        Illegal instruction
SIGTRAP             5        Core dump        Breakpoint (debugger)
SIGABRT             6        Core dump        abort() function
SIGKILL             9        Terminate        Kill signal (can't catch)
SIGSEGV             11       Core dump        Segmentation fault
SIGTERM             15       Terminate        Termination signal
SIGSTOP             19       Stop            Pause process
SIGCONT             18       Continue        Resume process
SIGCHLD             17       Ignore          Child process exited
SIGUSR1             10       Terminate       User defined signal 1
SIGUSR2             12       Terminate       User defined signal 2
SIGALRM             14       Terminate       Alarm clock
SIGPIPE             13       Terminate       Broken pipe
SIGTSTP             20       Stop            Ctrl+Z
```

### 9.3 Signal Handling

```c
#include <signal.h>
#include <stdio.h>
#include <unistd.h>

volatile int interrupted = 0;

/* Signal handler */
void sigint_handler(int sig) {
    /* IMPORTANT: Only async-signal-safe functions! */
    interrupted = 1;
    write(STDOUT_FILENO, "Interrupted!\n", 13);
}

int main() {
    /* Register signal handler */
    struct sigaction sa;
    sigemptyset(&sa.sa_mask);
    sa.sa_flags = 0;
    sa.sa_handler = sigint_handler;
    sigaction(SIGINT, &sa, NULL);
    
    /* Block other signals while handler runs */
    sigset_t set;
    sigemptyset(&set);
    sigaddset(&set, SIGUSR1);
    sigprocmask(SIG_BLOCK, &set, NULL);
    
    /* Main work loop */
    while (!interrupted) {
        printf("Working...\n");
        sleep(1);
    }
    
    printf("Exiting cleanly\n");
    return 0;
}
```

### 9.4 IPC Mechanisms

#### 9.4.1 Pipes

```
Named Pipe (FIFO):
┌─────────────────────────────────────┐
│ Unidirectional Communication        │
│                                     │
│ Writer → [Buffer] → Reader         │
│                                     │
│ mkfifo("myfifo", 0666)              │
│ open("myfifo", O_WRONLY)            │
│ open("myfifo", O_RDONLY)            │
│                                     │
│ Kernel Buffered (4096 bytes default)│
└─────────────────────────────────────┘

Unnamed Pipe (used with fork):
┌─────────────────────────────────────┐
│ Parent creates pipe before fork      │
│                                     │
│ int fd[2];                          │
│ pipe(fd);  // fd[0] = read end      │
│            // fd[1] = write end     │
│                                     │
│ Parent writes: write(fd[1], ...)   │
│ Child reads:   read(fd[0], ...)    │
│                                     │
│ Parent → [kernel buffer] → Child   │
└─────────────────────────────────────┘
```

#### 9.4.2 Shared Memory

```c
/* System V Shared Memory */
#include <sys/ipc.h>
#include <sys/shm.h>

int main() {
    /* Create shared memory segment */
    key_t key = ftok("/tmp/shmfile", 1);
    int shmid = shmget(key, 4096, IPC_CREAT | 0666);
    
    /* Attach to process address space */
    void *shmaddr = shmat(shmid, NULL, 0);
    
    /* Use like normal memory */
    int *ptr = (int *)shmaddr;
    *ptr = 42;
    
    /* Multiple processes access same memory */
    /* Process A writes, Process B reads instantly */
    
    /* Detach when done */
    shmdt(shmaddr);
    shmctl(shmid, IPC_RMID, NULL);
    
    return 0;
}
```

#### 9.4.3 Message Queues

```c
/* System V Message Queue */
#include <sys/msg.h>
#include <string.h>

struct message {
    long mtype;
    char mtext[256];
};

int main() {
    key_t key = ftok("/tmp/msgfile", 1);
    int msgqid = msgget(key, IPC_CREAT | 0666);
    
    /* Send message */
    struct message msg;
    msg.mtype = 1;
    strcpy(msg.mtext, "Hello from process A");
    msgsnd(msgqid, &msg, sizeof(msg), 0);
    
    /* Receive message (blocks if queue empty) */
    msgrcv(msgqid, &msg, sizeof(msg), 1, 0);
    printf("Received: %s\n", msg.mtext);
    
    msgctl(msgqid, IPC_RMID, NULL);
    return 0;
}
```

#### 9.4.4 Sockets

```c
/* TCP/IP Sockets - most common modern IPC */
#include <sys/socket.h>
#include <netinet/in.h>

int server_socket = socket(AF_INET, SOCK_STREAM, 0);
struct sockaddr_in addr = {
    .sin_family = AF_INET,
    .sin_port = htons(8000),
    .sin_addr.s_addr = INADDR_ANY
};

bind(server_socket, (struct sockaddr*)&addr, sizeof(addr));
listen(server_socket, 5);

int client_socket = accept(server_socket, NULL, NULL);

/* Send/receive data */
send(client_socket, buffer, size, 0);
recv(client_socket, buffer, size, 0);
```

---

## 10. Memory Management

### 10.1 Virtual Address Space

```
Process Virtual Address Space (64-bit):
0xFFFFFFFFFFFFFFFF ┌─────────────────────────┐
                   │  Kernel Space (fixed)   │
                   │                         │
                   │ - kernel text/data      │
                   │ - kernel heap           │
                   │ - per-CPU data          │
0xFFFF800000000000 ├─────────────────────────┤
                   │                         │
                   │ (unmapped hole)         │
                   │                         │
0x00007FFFFFFFFFFF ├─────────────────────────┤
                   │  User Stack             │
                   │  (grows downward)       │
                   │                         │
                   │ (unmapped)              │
                   │                         │
                   ├─────────────────────────┤
                   │  Shared Libraries       │
                   │  (mmap'd regions)       │
                   │                         │
                   ├─────────────────────────┤
                   │  Heap                   │
                   │  (grows upward)         │
                   │  ↑                      │
                   │  (via brk/mmap)        │
                   ├─────────────────────────┤
                   │  BSS (uninitialized)    │
                   │  Globals (0-init)       │
                   ├─────────────────────────┤
                   │  Data (initialized)     │
                   │  Globals, statics       │
                   ├─────────────────────────┤
                   │  Text (code)            │
                   │  Read-only              │
0x0000000000400000 ├─────────────────────────┤
0x0000000000000000 │  Reserved (unmapped)    │
                   └─────────────────────────┘
```

### 10.2 Virtual Memory to Physical Memory

```
Page Table Translation:

Virtual Address: 0x12345678
                    │
                    ├─ 4KB page offset: 0x678
                    └─ 4KB page number: 0x12345
                       │
                       └─► Page Directory Entry (PDE)
                           ├─ PGD: 0x12 (points to PDP)
                           ├─ PUD: 0x34 (points to PTE table)
                           ├─ PMD: 0x56 (points to PT table)
                           └─ PTE: 0x12345 (points to physical page)
                              │
                              └─ Physical Page: 0x7AB00 + 0x678
                                             = 0x7AB678

Page Table Hierarchy (4-level, x86-64):
┌────────────────────────┐
│ Virtual Address        │
│ 63-48: Sign extend     │
│ 47-39: PGD (9 bits)    │
│ 38-30: PUD (9 bits)    │
│ 29-21: PMD (9 bits)    │
│ 20-12: PTE (9 bits)    │
│ 11-0:  Offset (12 bits)│
└────────────────────────┘
        │
        ├─► PGD Table (512 entries)
            │
            ├─► PUD Table (512 entries)
                │
                ├─► PMD Table (512 entries)
                    │
                    ├─► Page Table (512 entries)
                        │
                        └─► Physical Page (4096 bytes)

Benefits:
- Sparse address spaces use minimal memory
- Only present tables are allocated
- Deep hierarchy allows fine control
```

### 10.3 Virtual Memory Area (VMA)

```c
struct vm_area_struct {
    struct mm_struct    *vm_mm;        /* Back pointer to mm */
    unsigned long       vm_start;      /* Start address */
    unsigned long       vm_end;        /* End address + 1 */
    struct vm_area_struct *vm_next;    /* Next VMA */
    
    pgprot_t           vm_page_prot;   /* Page permissions */
    unsigned long       vm_flags;      /* VM_READ, VM_WRITE, VM_EXEC */
    
    struct list_head    anon_vma_chain;  /* For reverse mapping */
    struct file        *vm_file;       /* File backing (if mmap'd) */
    unsigned long       vm_pgoff;      /* Offset in file */
    
    void               *vm_private_data; /* Architecture specific */
};

/* Process memory layout as linked list of VMAs:

mm_struct.mmap → VMA 0x400000-0x401000 (text)
                   ↓
                 VMA 0x600000-0x602000 (data)
                   ↓
                 VMA 0x602000-0x610000 (heap)
                   ↓
                 VMA 0x7ffde000-0x800000 (stack)
                   ↓
                 VMA 0x7f1234000-0x7f1235000 (libc)
                   ↓
                 NULL
*/
```

### 10.4 Memory Faults and Page Faults

```
Page Fault Exception:

Scenario: Process accesses unmapped page

1. CPU tries to translate virtual address
   ├─ Walks page tables
   ├─ Looks in TLB first
   └─ Page not present or not accessible
   
2. CPU generates page fault exception
   ├─ Saves execution context
   ├─ Jumps to fault handler (kernel)

3. Kernel page fault handler (do_page_fault):
   
   page_fault_exception()
   ├─ Determine fault address and type
   │  ├─ Read fault: page not present
   │  ├─ Write fault: page not writable (CoW)
   │  ├─ Execute fault: page not executable
   │
   ├─ Find corresponding VMA
   │  vmarea = find_vma(mm, fault_addr)
   │
   ├─ Check if fault is valid
   │  ├─ Does VMA cover this address?
   │  ├─ Are requested permissions allowed?
   │  └─ Is this a swap page?
   │
   ├─ Determine action based on type:
   │
   │  A) Legitimate page needed
   │  │  ├─ Find or allocate page
   │  │  ├─ Fill with data if from file
   │  │  ├─ Insert PTE (mark present)
   │  │  └─ Return to user code
   │  │
   │  B) Copy-on-Write (CoW)
   │  │  ├─ Write to shared page
   │  │  ├─ Make private copy
   │  │  ├─ Decrement original page refcount
   │  │  └─ Return with writable copy
   │  │
   │  C) Stack growth
   │  │  ├─ Fault is within ULIMIT_STACK
   │  │  ├─ Allocate page for stack
   │  │  ├─ Extend VMA downward
   │  │  └─ Return
   │  │
   │  D) Invalid access
   │  │  ├─ Send SIGSEGV signal
   │  │  └─ Kill process
   │
   └─ Return from fault handler

4. CPU restores context
   ├─ Reloads registers
   ├─ Retries instruction
   └─ Succeeds now
```

### 10.5 Page Replacement (Swapping)

```
When Physical Memory Exhausted:

Scenario: Need new page but all RAM used

1. Memory pressure detected
   ├─ Check available pages
   ├─ Free cache pages if possible
   └─ If still not enough → trigger page eviction

2. Choose page to evict (LRU: Least Recently Used)
   
   Used Pages:
   ┌─ Page A (used 1000ms ago)   ◄─ EVICT THIS
   ├─ Page B (used 50ms ago)
   ├─ Page C (used 10ms ago)
   └─ Page D (just accessed)
   
3. Write page to swap space (disk)
   ├─ PTE mark as NOT PRESENT
   ├─ Swap offset stored in PTE
   └─ Physical page freed

4. When faulting process accesses evicted page:
   ├─ Page fault (not present)
   ├─ Kernel reads from swap
   ├─ Loads back into RAM
   ├─ Marks PTE as PRESENT
   └─ Process continues

Trade-off:
- RAM: Fast (nanoseconds)
- Disk Swap: Slow (milliseconds)
- Ratio: 1,000,000x slower!

So avoid swap!
```

---

## 11. Real Kernel Implementations

### 11.1 Actual Linux Kernel fork() Code

From `kernel/fork.c` in Linux 6.0+:

```c
/* Simplified but real fork() implementation */
long sys_fork(struct pt_regs *regs)
{
    return kernel_clone(&args);
}

pid_t kernel_clone(struct kernel_clone_args *args)
{
    u64 clone_flags = args->flags;
    struct task_struct *p;
    struct completion vfork;
    
    /* Increment process count */
    if (atomic_read(&current->real_cred->user->processes) >=
        task_rlimit(current, RLIMIT_NPROC)) {
        return -EAGAIN;
    }
    
    /* 1. Allocate new task_struct */
    p = copy_process(NULL, clone_flags, regs, 0, args->child_tid,
                     args->parent_tid, args->exit_signal, args->stack,
                     args->stack_size);
    if (IS_ERR(p))
        return PTR_ERR(p);
    
    /* 2. Wake up new process */
    wake_up_new_task(p);
    
    /* Handle vfork case */
    if (clone_flags & CLONE_VFORK) {
        init_completion(&vfork);
        p->vfork_done = &vfork;
        wait_for_completion(&vfork);
    }
    
    return p->pid;
}

static __latent_entropy struct task_struct *copy_process(
    struct pid *pid,
    int trace,
    int node,
    struct kernel_clone_args *args)
{
    int retval;
    struct task_struct *p;
    struct multiprocess_context mp_ctx = {};
    
    /* Check clone flags validity */
    if ((clone_flags & CLONE_NEWPID) && (clone_flags & CLONE_THREAD))
        return ERR_PTR(-EINVAL);
    
    /* 1. Duplicate task_struct */
    p = dup_task_struct(current, node);
    if (!p)
        return ERR_PTR(-ENOMEM);
    
    ftrace_graph_init_task(p);
    rt_mutex_init_task(p);
    
    /* 2. Copy files and signals */
    retval = copy_files(clone_flags, p);
    if (retval)
        goto bad_fork_cleanup_semundo;
    
    retval = copy_fs(clone_flags, p);
    if (retval)
        goto bad_fork_cleanup_files;
    
    retval = copy_sighand(clone_flags, p);
    if (retval)
        goto bad_fork_cleanup_fs;
    
    retval = copy_signal(clone_flags, p);
    if (retval)
        goto bad_fork_cleanup_sighand;
    
    /* 3. Copy memory management */
    retval = copy_mm(clone_flags, p);
    if (retval)
        goto bad_fork_cleanup_signal;
    
    /* 4. Copy namespace */
    retval = copy_namespaces(clone_flags, p);
    if (retval)
        goto bad_fork_cleanup_mm;
    
    /* 5. Copy cgroups */
    retval = copy_cgroups(clone_flags, p);
    if (retval)
        goto bad_fork_cleanup_namespaces;
    
    /* 6. Initialize scheduling */
    retval = sched_fork(clone_flags, p);
    if (retval)
        goto bad_fork_cleanup_cgroups;
    
    /* 7. Copy parent-child relationship */
    p->parent = current;
    p->real_parent = current;
    p->parent_exec_id = current->self_exec_id;
    
    /* 8. Initialize PID and add to hash tables */
    p->pid = pid_nr(pid);
    
    /* 9. Add to global task list */
    list_add_tail_rcu(&p->tasks, &init_task.tasks);
    
    /* 10. Copy exit signal */
    p->exit_signal = args->exit_signal;
    
    /* 11. Copy ptrace info */
    if (clone_flags & CLONE_PTRACE)
        p->ptrace = current->ptrace;
    
    return p;
    
    /* Error handling */
bad_fork_cleanup_cgroups:
    cgroup_cancel_fork(p);
bad_fork_cleanup_namespaces:
    exit_task_namespaces(p);
bad_fork_cleanup_mm:
    if (p->mm)
        mmput(p->mm);
bad_fork_cleanup_signal:
    exit_signal(p);
bad_fork_cleanup_sighand:
    __cleanup_sighand(p->sighand_struct);
bad_fork_cleanup_fs:
    exit_fs(p);
bad_fork_cleanup_files:
    exit_files(p);
bad_fork_cleanup_semundo:
    exit_sem(p);
    put_task_struct(p);
    
    return ERR_PTR(retval);
}
```

### 11.2 CFS Scheduler Implementation

From `kernel/sched/fair.c`:

```c
/* CFS core scheduling function */
static struct task_struct *pick_next_task_fair(struct rq *rq,
                                                struct task_struct *prev,
                                                struct rq_flags *rf)
{
    struct cfs_rq *cfs_rq = &rq->cfs;
    struct sched_entity *se;
    struct task_struct *p;
    
    /* Try to re-select current if it's still valid */
    if (prev->sched_class == &fair_sched_class) {
        if (cfs_rq->last && wakeup_preempt_entity(cfs_rq->last, prev) < 1)
            return prev;  /* Let current continue */
    }
    
put_prev_set_next:
    if (prev)
        put_prev_task(rq, prev);
    
    do {
        se = pick_next_entity(cfs_rq, NULL);  /* Key function */
        set_next_entity(cfs_rq, se);
        cfs_rq = group_cfs_rq(se);
    } while (cfs_rq);
    
    p = task_of(se);
    
    return p;
}

/* Pick next entity from CFS tree */
static struct sched_entity *pick_next_entity(struct cfs_rq *cfs_rq,
                                              struct sched_entity *curr)
{
    struct sched_entity *left = __pick_first_entity(cfs_rq);
    struct sched_entity *se;
    
    /* 1. Check if we can skip the tree */
    if (curr) {
        if (left && __entity_less(&left->vruntime, &curr->vruntime))
            se = left;
        else
            se = curr;
    } else {
        se = left;  /* left-most has smallest vruntime */
    }
    
    return se;
}

/* Get leftmost (smallest vruntime) */
static struct sched_entity *__pick_first_entity(struct cfs_rq *cfs_rq)
{
    return rb_entry(cfs_rq->rb_leftmost, struct sched_entity, run_node);
}

/* Update task vruntime after it ran */
static void update_curr(struct cfs_rq *cfs_rq)
{
    struct sched_entity *curr = cfs_rq->curr;
    u64 now = rq_clock_task(rq_of(cfs_rq));
    u64 delta_exec;
    
    if (unlikely(!curr))
        return;
    
    /* Calculate actual execution time */
    delta_exec = now - curr->exec_start;
    curr->exec_start = now;
    
    curr->sum_exec_runtime += delta_exec;
    
    /* Update vruntime = actual_time * (weight_0 / weight_task) */
    delta_exec = calc_delta_fair(delta_exec, curr);
    curr->vruntime += delta_exec;
    
    /* Update minimum vruntime */
    if (curr->vruntime > cfs_rq->min_vruntime)
        cfs_rq->min_vruntime = curr->vruntime;
}

/* Schedule out - remove from tree */
static void put_prev_task_fair(struct rq *rq, struct task_struct *prev)
{
    struct sched_entity *se = &prev->se;
    
    /* Update vruntime before removing from tree */
    update_curr(cfs_rq_of(se));
    
    /* Remove from RB-tree and reinsert with new vruntime */
    __dequeue_entity(cfs_rq_of(se), se);
    __enqueue_entity(cfs_rq_of(se), se);
}
```

### 11.3 Context Switch Assembly (x86-64)

From `arch/x86/entry/entry_64.S` and `arch/x86/kernel/process.c`:

```asm
/* __switch_to - Core context switching */
ENTRY(__switch_to)
    push %rbp
    movq %rsp, %rbp
    
    /* Save registers of previous task */
    pushq %rbx
    pushq %r12
    pushq %r13
    pushq %r14
    pushq %r15
    pushq %rax
    pushq %rcx
    pushq %rdx
    
    /* Save previous task's RSP to thread_struct */
    movq %rsp, THREAD_RSP(%rdi)  /* rdi = prev */
    
    /* Load next task's RSP from thread_struct */
    movq THREAD_RSP(%rsi), %rsp   /* rsi = next */
    
    /* Load next task's registers */
    popq %rdx
    popq %rcx
    popq %rax
    popq %r15
    popq %r14
    popq %r13
    popq %r12
    popq %rbx
    
    /* Switch FS base for kernel-space */
    swapgs  /* Swap GS with kernel GS */
    
    pop %rbp
    ret
ENDPROC(__switch_to)
```

### 11.4 Rust Implementation Example - Mini Scheduler

```rust
// A simplified scheduler in Rust
use std::collections::VecDeque;
use std::cell::RefCell;

#[derive(Clone, Debug)]
struct Task {
    pid: u32,
    vruntime: u64,
    exec_time: u64,
    state: TaskState,
}

#[derive(Clone, Debug, PartialEq)]
enum TaskState {
    Running,
    Runnable,
    Waiting,
    Zombie,
}

struct Scheduler {
    runqueue: RefCell<Vec<Task>>,  /* Sorted by vruntime */
    current: RefCell<Option<Task>>,
    pids: RefCell<u32>,
}

impl Scheduler {
    fn new() -> Self {
        Scheduler {
            runqueue: RefCell::new(Vec::new()),
            current: RefCell::new(None),
            pids: RefCell::new(1),
        }
    }
    
    /* Pick next task (CFS algorithm) */
    fn pick_next_task(&self) -> Option<Task> {
        let mut rq = self.runqueue.borrow_mut();
        
        if rq.is_empty() {
            return None;
        }
        
        /* Find task with minimum vruntime */
        let next_idx = rq.iter()
            .enumerate()
            .min_by_key(|(_, task)| task.vruntime)
            .map(|(idx, _)| idx)?;
        
        Some(rq.remove(next_idx))
    }
    
    /* Schedule a new task */
    fn schedule(&self) {
        /* Save current task state */
        if let Some(mut current) = self.current.borrow_mut().take() {
            if current.state == TaskState::Runnable {
                /* Update vruntime and reinsert */
                current.vruntime += current.exec_time;
                self.runqueue.borrow_mut().push(current);
            }
        }
        
        /* Pick next task */
        if let Some(next) = self.pick_next_task() {
            *self.current.borrow_mut() = Some(next);
        }
    }
    
    /* Create new task */
    fn fork(&self) -> u32 {
        let pid = {
            let mut p = self.pids.borrow_mut();
            let id = *p;
            *p += 1;
            id
        };
        
        let task = Task {
            pid,
            vruntime: 0,
            exec_time: 0,
            state: TaskState::Runnable,
        };
        
        self.runqueue.borrow_mut().push(task);
        pid
    }
}

fn main() {
    let scheduler = Scheduler::new();
    
    /* Create tasks */
    for _ in 0..3 {
        scheduler.fork();
    }
    
    /* Simulate scheduling */
    for _ in 0..10 {
        scheduler.schedule();
        if let Some(current) = scheduler.current.borrow().as_ref() {
            println!("Running task PID {}, vruntime {}", 
                     current.pid, current.vruntime);
        }
    }
}
```

---

## 12. Advanced Topics

### 12.1 Process Groups and Sessions

```
Session and Process Group Hierarchy:

Session: Collection of process groups
(Created by setsid() or login shell)

                    ┌─────────────────────┐
                    │     SESSION         │
                    │    (session_id)     │
                    └──────────┬──────────┘
                               │
            ┌──────────────────┼──────────────────┐
            │                  │                  │
    ┌───────────────┐  ┌───────────────┐  ┌────────────────┐
    │ PROCESS GROUP │  │ PROCESS GROUP │  │ PROCESS GROUP  │
    │  (pgrp = 100) │  │  (pgrp = 101) │  │  (pgrp = 102)  │
    └────────┬──────┘  └────────┬──────┘  └────────┬────────┘
             │                  │                  │
        ┌────┴────┐         ┌───────┐         ┌────────┐
        │          │         │       │         │        │
    [PID 100]  [PID 101] [PID 102] [PID 103] [PID 104][PID 105]

Session Leader: PID 100 (created session)
Foreground Process Group: 100 (has terminal)
Background Process Groups: 101, 102 (no terminal access)
```

### 12.2 Namespace Isolation

```
Linux Namespaces: Per-process isolation

PID Namespace:
├─ PID 1 (init in namespace)
├─ PID 2 (bash in namespace)
├─ PID 3 (process in namespace)
└─ PIDs independent of host system

Network Namespace:
├─ Isolated network interfaces
├─ Separate routing tables
├─ Separate firewall rules
└─ Processes see different network

Mount Namespace:
├─ Isolated filesystem mount points
├─ /proc, /sys specific to namespace
├─ Filesystem hierarchy independent
└─ Docker uses this

IPC Namespace:
├─ Isolated System V IPC
├─ Separate message queues
├─ Separate shared memory
└─ Separate semaphores

User Namespace:
├─ UID 0 in namespace → UID 1000 on host
├─ Allows unprivileged containers
├─ UID mapping
└─ GID mapping
```

### 12.3 Cgroups (Control Groups)

```
Resource Limiting and Accounting:

cgroup
├─ CPU Limiting
│  ├─ cpu.shares: proportional CPU time
│  ├─ cpu.quota: max microseconds per period
│  └─ Example: limit to 50% CPU
│
├─ Memory Limiting
│  ├─ memory.limit_in_bytes: max memory
│  ├─ memory.soft_limit_in_bytes: soft limit
│  └─ Kills task if exceeds hard limit
│
├─ I/O Limiting
│  ├─ Disk bandwidth throttling
│  └─ IOPS limiting
│
└─ Process Accounting
   ├─ cpuacct.usage: total CPU time
   ├─ memory.stat: memory breakdown
   └─ cpuacct.stat: user/system time split
```

### 12.4 RT Scheduling (Real-Time)

```c
/* Real-time scheduling - hard guarantees */

/* Set real-time priority */
struct sched_param param;
param.sched_priority = 80;  /* 0-99 */

pthread_attr_init(&attr);
pthread_attr_setschedpolicy(&attr, SCHED_FIFO);
pthread_attr_setschedparam(&attr, &param);
pthread_create(&thread, &attr, worker_func, NULL);

/* RT tasks never yield unless:
   ├─ Blocked on I/O
   ├─ Explicitly yielded (sched_yield)
   ├─ Preempted by higher priority RT task
   └─ Time quantum expires (for SCHED_RR) */
```

---

## Conclusion

This comprehensive guide covers Linux kernel processes from fundamental concepts to advanced implementations. Key takeaways:

1. **Process**: Fundamental abstraction for protected execution
2. **task_struct**: Comprehensive kernel data structure containing all process info
3. **Scheduling**: CFS scheduler ensures fair CPU time distribution
4. **Context Switching**: Core mechanism enabling multitasking (high cost!)
5. **Creation**: fork() + exec() pattern for launching programs
6. **Termination**: Proper cleanup and parent notification
7. **Synchronization**: Prevents race conditions (locks, semaphores, RCU)
8. **Memory**: Virtual address spaces with page tables and CoW
9. **IPC**: Multiple mechanisms for inter-process communication
10. **Advanced**: Namespaces, cgroups, real-time scheduling for isolation

The Linux kernel is a masterpiece of software engineering. Understanding processes deeply teaches you about:
- Concurrent programming
- Resource management
- Performance optimization
- Security isolation
- Operating system design

Continue exploring actual kernel source code. Read the Linux kernel documentation. Build small kernel modules. This hands-on experience will cement your understanding and make you a better systems programmer.

**Happy kernel hacking!**
