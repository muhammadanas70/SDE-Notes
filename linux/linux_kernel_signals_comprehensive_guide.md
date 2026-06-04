# Linux Kernel Signals: A Comprehensive In-Depth Guide

## Table of Contents
1. [Fundamentals & History](#fundamentals--history)
2. [Signal Types & Numbering](#signal-types--numbering)
3. [Kernel Data Structures](#kernel-data-structures)
4. [Signal Delivery Architecture](#signal-delivery-architecture)
5. [Standard Signals (1-31)](#standard-signals-1-31)
6. [Real-Time Signals (32-64)](#real-time-signals-32-64)
7. [Signal Handling Mechanisms](#signal-handling-mechanisms)
8. [Synchronous Signal Delivery](#synchronous-signal-delivery)
9. [Asynchronous Signal Delivery](#asynchronous-signal-delivery)
10. [Signal Masks & Blocking](#signal-masks--blocking)
11. [Kernel Implementation Details](#kernel-implementation-details)
12. [Race Conditions & Atomicity](#race-conditions--atomicity)
13. [Signal Debugging & Tracing](#signal-debugging--tracing)
14. [Performance Considerations](#performance-considerations)
15. [Practical Examples](#practical-examples)

---

## Fundamentals & History

### What Are Signals?

Signals are a form of **inter-process communication (IPC)** and **control mechanism** in Unix/Linux systems. They provide a way to:

1. **Handle exceptional conditions** - Divide by zero, segmentation faults
2. **Manage process lifecycle** - Terminate, suspend, resume processes
3. **Implement timers** - Alarm signals, profiling
4. **Handle I/O events** - Asynchronous I/O notifications
5. **Implement job control** - Background/foreground process management

### Historical Context

Signals were introduced in **V7 Unix (1979)** as a simple asynchronous notification mechanism. The original design was minimal:

- Only 16 signals (since expanded to 64)
- No queuing (multiple identical signals result in single delivery)
- No additional context passed with signal
- Signal handler could only set a flag (volatile sig_atomic_t)

### Signal Delivery Model: Signal → Process → Handler

```
Signal Generation
       ↓
┌──────────────────┐
│  Kernel checks:  │
│ 1. Is signal OK? │
│ 2. Is it masked? │
│ 3. Handler set?  │
└──────────────────┘
       ↓
   NOT MASKED?
   ↙      ↘
Yes        No
 ↓          ↓
 │      Block/Queue
 ↓
Deliver to User Space
 ↓
┌─────────────────────────────┐
│ Interrupt current execution │
│ Save context                │
│ Jump to handler             │
└─────────────────────────────┘
 ↓
Signal Handler Executes
 ↓
Return to Original Context
```

### Key Terminology

- **Signal Number**: Identifier (1-64)
- **Signal Action**: What happens when signal is delivered (handler, default, ignore)
- **Signal Mask**: Set of blocked signals (per-thread)
- **Pending Signal**: Signal generated but not yet delivered
- **Blocked Signal**: Signal whose delivery is postponed
- **Caught Signal**: Process installed handler for this signal
- **Generated Signal**: Signal created/sent to process
- **Delivered Signal**: Signal handler executed

---

## Signal Types & Numbering

### Standard Signal Layout

```
SIGNAL SPACE (64 total)
├─ Standard Signals: 1-31 (non-queuing)
├─ Real-Time Signals: 32-64 (queuing, ordered)
└─ Signal 0: Used for kill(pid, 0) - "is process alive?"

MEMORY LAYOUT (64-bit system):
┌─────────────────────────────────────────┐
│ Signal Mask (sigset_t)                  │
├─────────────────────────────────────────┤
│ 64-bit bitmask                          │
│ Bit 0 = Signal 1 (SIGHUP)               │
│ Bit 1 = Signal 2 (SIGINT)               │
│ ...                                     │
│ Bit 63 = Signal 64 (SIGSYS)             │
└─────────────────────────────────────────┘

KERNEL sigset_t (include/linux/signal.h):
typedef struct {
    unsigned long sig[_NSIG_WORDS];
} sigset_t;

Where _NSIG_WORDS = (64 + BITS_PER_LONG - 1) / BITS_PER_LONG
On 64-bit: _NSIG_WORDS = 1
On 32-bit: _NSIG_WORDS = 2
```

### Standard Signals (1-31): Non-Queueing

| Signal | Number | Default Action | Purpose |
|--------|--------|-----------------|---------|
| SIGHUP | 1 | Terminate | Hangup detected on controlling terminal |
| SIGINT | 2 | Terminate | Interrupt from keyboard (Ctrl+C) |
| SIGQUIT | 3 | Core dump | Quit from keyboard (Ctrl+\) |
| SIGILL | 4 | Core dump | Illegal instruction |
| SIGTRAP | 5 | Core dump | Trace/breakpoint trap |
| SIGABRT | 6 | Core dump | Abort signal from abort() |
| SIGBUS | 7 | Core dump | Bus error (bad memory access) |
| SIGFPE | 8 | Core dump | Floating point exception |
| SIGKILL | 9 | Terminate | Kill signal (cannot be caught) |
| SIGUSR1 | 10 | Terminate | User-defined signal 1 |
| SIGSEGV | 11 | Core dump | Segmentation fault |
| SIGUSR2 | 12 | Terminate | User-defined signal 2 |
| SIGPIPE | 13 | Terminate | Broken pipe |
| SIGALRM | 14 | Terminate | Alarm clock |
| SIGTERM | 15 | Terminate | Termination signal |
| SIGCHLD | 17 | Ignore | Child process terminated/stopped |
| SIGCONT | 18 | Continue | Continue if stopped |
| SIGSTOP | 19 | Stop | Stop process (cannot be caught) |
| SIGTSTP | 20 | Stop | Stop typed at terminal |
| SIGTTIN | 21 | Stop | Background read from tty |
| SIGTTOU | 22 | Stop | Background write to tty |
| SIGURG | 23 | Ignore | Urgent condition on socket |
| SIGXCPU | 24 | Core dump | CPU time limit exceeded |
| SIGXFSZ | 25 | Core dump | File size limit exceeded |
| SIGVTALRM | 26 | Terminate | Virtual alarm clock |
| SIGPROF | 27 | Terminate | Profiling timer expired |
| SIGWINCH | 28 | Ignore | Window size changed |
| SIGIO | 29 | Ignore | I/O now possible |
| SIGPWR | 30 | Terminate | Power failure |
| SIGSYS | 31 | Core dump | Bad system call |

### Real-Time Signals (32-64): Queueing

**Key Differences from Standard Signals:**

```
PROPERTY COMPARISON:

Standard Signals (1-31):          Real-Time Signals (32-64):
├─ No queuing                     ├─ Full queuing
├─ Lost if pending                ├─ Preserved in queue
├─ No argument                    ├─ 32-bit int/ptr argument
├─ Unordered delivery             ├─ FIFO delivery
├─ Limited to 1 pending           ├─ 32 queued per signal
├─ Backward compatible            ├─ POSIX.1b compliant
└─ Used for simple events         └─ Used for real-time systems

REAL-TIME SIGNAL QUEUEING BUFFER:

struct sigqueue {
    struct list_head list;
    int flags;
    siginfo_t info;
    struct user_struct *user;
};

QUEUE ARCHITECTURE:
┌─────────────────────────────────────┐
│ signal_struct (per-task-group)      │
├─────────────────────────────────────┤
│ sigqueue_free_list              │ Queue of free sigqueue structs
│ sigqueue_cache                  │
└─────────────────────────────────────┘
         ↓
    For each real-time signal
         ↓
    ┌─────────────────────┐
    │ list_head pending   │ FIFO queue of siginfo structs
    ├─────────────────────┤
    │ sigqueue *q1 → ... │ Ordered delivery
    │ sigqueue *q2 → ... │ (later entries delivered first in FIFO)
    │ sigqueue *qN → ... │
    └─────────────────────┘
```

---

## Kernel Data Structures

### Core Signal Structures (include/linux/signal_types.h)

```c
// Main signal action structure (user-space exposed)
struct sigaction {
    __sighandler_t sa_handler;      // Handler function pointer or SIG_DFL/SIG_IGN
    sigset_t sa_mask;               // Signals blocked during handler execution
    int sa_flags;                   // Flags (SA_SIGINFO, SA_RESTART, etc.)
    __sigrestore_t sa_restorer;     // Unused on modern systems (from V7 days)
};

// User receives this via sigaction() syscall registration
// Kernel stores in task_struct (struct sighand_struct)
```

### sighand_struct: Thread-Shared Signal State

```c
// From: include/linux/sched/signal.h
struct sighand_struct {
    spinlock_t siglock;                          // Protects action array
    struct k_sigaction action[_NSIG];            // 64 signal action entries

    /*
     ARCHITECTURE (simplified):
    
     task_struct
         ↓
     struct sighand_struct (shared among NPTL threads via CLONE_SIGHAND)
         ├─ spinlock_t siglock
         │  Purpose: Protect concurrent access to action[] array
         │  Granularity: Per-thread-group (all threads in group)
         │
         ├─ action[0] = SIGHUP (signal 1)
         ├─ action[1] = SIGINT (signal 2)
         ├─ action[2] = SIGQUIT (signal 3)
         │  ...
         ├─ action[30] = SIGSYS (signal 31)
         ├─ action[31] = SIGRTMIN (signal 32)
         │  ...
         └─ action[63] = SIGRTMAX (signal 64)
    */
};

// Kernel internal signal action
struct k_sigaction {
    struct sigaction sa;            // User-visible part
    __sigrestore_t ka_restorer;     // Restorer function (kernel sets)
    int ka_restorer_oops;           // Oops handling flag
};
```

### signal_struct: Process Signal State (Per-Task-Group)

```c
// From: include/linux/signal_types.h
// Shared among all threads in process (via CLONE_SIGHAND)
struct signal_struct {
    atomic_t sigcnt;                    // Ref count for shared signal state
    atomic_t live;                      // Number of live threads
    
    // Pending signals for entire task group
    struct sigpending shared_pending;   // Queued signals for any thread
    
    // Thread group exit status
    int group_exit_code;                // Exit code for group
    int notify_count;                   // Threads waiting for group death
    
    // Job control
    struct tty_struct *tty;             // Controlling terminal (or NULL)
    struct list_head posix_timers;      // List of POSIX timers
    
    // More fields...
};

// sigpending: Queue structure for pending signals
struct sigpending {
    struct list_head list;              // List of sigqueue structs
    sigset_t signal;                    // Bitmask of pending signal numbers
    
    /*
     PURPOSE: Two ways to track pending signals:
     1. signal bitmask - "which signal types are pending?"
     2. list - "what are the actual queued siginfo structs?"
    
     For standard signals: Only bitmask used (overwrite old pending)
     For real-time signals: Both list and bitmask used (full queue)
    
     LOOKUP FLOW:
     if (sigismember(&pending->signal, signum)) {
         // Signal is pending
         // If real-time, find queued siginfo via list iteration
         // If standard, only one possible pending instance
     }
    */
};
```

### siginfo_t: Signal Information (include/uapi/asm-generic/siginfo.h)

```c
// Full structure passed to handler with SA_SIGINFO flag
typedef struct {
    int si_signo;           // Signal number
    int si_errno;           // Error number
    int si_code;            // Signal code (SI_USER, SI_KERNEL, etc.)
    
    // Following union depends on si_code
    union {
        int _pad[SI_PAD_SIZE];
        
        // For SI_KILL (kill() syscall)
        struct {
            __kernel_pid_t _pid;
            __kernel_uid32_t _uid;
        } _kill;
        
        // For SI_TIMER
        struct {
            __kernel_timer_t _tid;
            int _overrun;
        } _timer;
        
        // For SI_ASYNCIO
        struct {
            __kernel_pid_t _pid;
            __kernel_uid32_t _uid;
            sigval_t _sigval;
        } _rt;
        
        // For SIGCHLD
        struct {
            __kernel_pid_t _pid;
            __kernel_uid32_t _uid;
            int _status;
            __kernel_clock_t _utime;
            __kernel_clock_t _stime;
        } _sigchld;
        
        // ... more unions for other signal types
    } _sifields;
} siginfo_t;

// When SA_SIGINFO is used, handler signature:
void handler(int sig, siginfo_t *info, void *context);

// context param points to ucontext_t (machine state snapshot)
```

### task_struct Signal Fields

```c
// From: include/linux/sched/signal.h in task_struct
struct task_struct {
    // ... many fields ...
    
    // Signal state - per-thread
    struct sigpending pending;              // Pending signals for THIS thread
    unsigned long blocked;                  // Blocked signals (thread-local)
    unsigned long real_blocked;             // Real blocked (for suspend_process)
    unsigned long saved_sigmask;            // Saved by rt_sigprocmask
    
    // Signal handler info
    struct sighand_struct *sighand;         // Shared with threads
    sigset_t blocked;                       // Blocked signal set (thread-local)
    sigset_t real_blocked;                  
    
    // Exit signal
    int exit_signal;                        // Signal sent to parent on exit
    
    // ... more fields ...
};

/*
 CRITICAL ARCHITECTURAL INSIGHT:
 
 Signals are shared at TWO levels:
 
 ┌─────────────────────────────────────────────────────┐
 │ Process (Task Group)                                │
 ├─────────────────────────────────────────────────────┤
 │ struct signal_struct (shared)                       │
 │ ├─ shared_pending (for any thread)                 │
 │ └─ sighand_struct (all signal actions)             │
 │                                                     │
 │ ┌──────────────────┐  ┌──────────────────┐         │
 │ │ Thread 1 (TID 1) │  │ Thread 2 (TID 2) │         │
 │ ├──────────────────┤  ├──────────────────┤         │
 │ │ pending (t1)     │  │ pending (t2)     │         │
 │ │ blocked (t1)     │  │ blocked (t2)     │         │
 │ └──────────────────┘  └──────────────────┘         │
 │                                                     │
 │ Signal flow:                                        │
 │ 1. kill(pid, sig) → shared_pending (any thread)   │
 │ 2. pthread_kill(tid, sig) → specific thread       │
 │ 3. Kernel chooses thread to deliver to            │
 │                                                     │
 └─────────────────────────────────────────────────────┘
*/
```

---

## Signal Delivery Architecture

### Complete Signal Delivery Flow (Kernel Perspective)

```
SYSCALL BOUNDARY:

┌────────────────────────────────────────────────────────────┐
│ USER SPACE                                                 │
│                                                            │
│  Process runs user code                                   │
│  (arbitrary instruction stream)                           │
└────────────────────────────────────────────────────────────┘
           ↑ (return from signal handler)
           │
   ┌───────┴─────────────────────────────────┐
   │                                         │
   │ Signal handler executes                 │
   │ (trampoline jumps here)                 │
   │                                         │
   └───────────────────────────────────────┬─┘
                                           │ (return)
┌──────────────────────────────────────────┴──────────────────┐
│ KERNEL MODE (after exception/interrupt)                     │
│                                                             │
│ 1. Exception handler (trap/interrupt)                       │
│    ├─ Save CPU state (RIP, RSP, flags, etc.)              │
│    ├─ Handle h/w interrupt or syscall                      │
│    └─ Return to userspace preparation                      │
│                                                             │
│ 2. Signal detection (handle_signal_*() functions)          │
│    ├─ Check if pending signal                              │
│    ├─ Check if blocked                                     │
│    ├─ Check if has handler                                 │
│    ├─ Build siginfo_t                                      │
│    └─ Setup_rt_frame() or setup_frame()                   │
│                                                             │
│ 3. Frame setup (arch-specific, e.g., arch/x86/signal.c)   │
│    ├─ Allocate space on user stack                        │
│    ├─ Store: siginfo_t, ucontext_t, return addr           │
│    ├─ Install signal trampoline address as return         │
│    └─ Modify CPU state to jump to handler on return       │
│                                                             │
│ 4. Return to user mode                                     │
│    └─ Jump to signal handler (or user code if no signal)  │
│                                                             │
└────────────────────────────────────────────────────────────┘


DETAILED KERNEL PATH (simplified):

do_signal(struct pt_regs *regs)
    ↓
recalc_sigpending()  // Recompute pending bitmask
    ↓
while (true) {
    sig = next_signal(&signal->shared_pending, &current->blocked)
    if (sig == 0) break
    
    action = &sighand->action[sig]
    
    if (action.sa_handler == SIG_DFL)
        handle_default_signal()
    else if (action.sa_handler == SIG_IGN)
        continue  // Ignore
    else
        // User-installed handler
        setup_rt_frame(sig, &info, &oldset, regs)
        // Modifies regs->rip to point to handler
        // Modifies regs->rsp for signal frame
}
    ↓
return  // Returns to userspace, executes handler


SIGNAL FRAME LAYOUT (x86-64 on stack):

User Stack Top (RSP points here on handler entry):
┌─────────────────────────────────────────┐
│ return address (to __restore_rt)        │ (8 bytes)
├─────────────────────────────────────────┤
│ struct ucontext_t                       │ (>300 bytes)
│ ├─ uc_flags                             │
│ ├─ uc_link                              │
│ ├─ uc_stack                             │
│ ├─ uc_mcontext (CPU registers)          │
│ │  ├─ r8-r15                            │
│ │  ├─ rdi, rsi, rdp, rbx                │
│ │  ├─ rdx, rax, rcx, rsp, rbp           │
│ │  ├─ r9-r11                            │
│ │  ├─ r12-r15, orig_rax                 │
│ │  ├─ rip (saved PC)                    │
│ │  ├─ eflags (saved FLAGS)              │
│ │  ├─ cs, ss (segments)                 │
│ │  └─ ... FPU state ...                 │
│ └─ uc_sigmask (blocked during handler) │
├─────────────────────────────────────────┤
│ struct siginfo_t                        │ (128 bytes)
├─────────────────────────────────────────┤
│ align padding                           │
└─────────────────────────────────────────┘

Handler invoked as:
  signal_handler(signum, siginfo_t*, ucontext_t*)
  
Handler return:
  sigreturn() syscall restores:
    - All registers from uc_mcontext
    - Signal mask from uc_sigmask
    - Continues original execution
```

### Signal Routing Decision Algorithm

```
// From: kernel/signal.c recalc_sigpending() and related

ROUTING ALGORITHM (for multithreaded process):

Signal sent to process (kill(pid, sig)):
    ↓
┌───────────────────────────────────────────────────┐
│ Question 1: Is any thread running handler for sig? │
│            (check all threads' sighand_struct)     │
└───────────────────────────────────────────────────┘
    ↙ NO  ↘ YES
    │      └─→ Queue in shared_pending
    │         (don't deliver yet)
    │
┌───────────────────────────────────────┐
│ Q2: Is sig blocked in ALL threads?    │
└───────────────────────────────────────┘
    ↙ NO  ↘ YES
    │      └─→ Queue in shared_pending
    │         (blocked everywhere)
    │
┌──────────────────────────────────────────────────────┐
│ Q3: Find first thread NOT blocking sig               │
│     (iterate task_struct list)                       │
└──────────────────────────────────────────────────────┘
    │
    └─→ Send to that thread's pending queue
        (wake thread if in interruptible sleep)


Signal sent to specific thread (pthread_kill(tid, sig)):
    ↓
┌───────────────────────────────────────────────────┐
│ Question 1: Is thread running handler for sig?    │
└───────────────────────────────────────────────────┘
    ↙ NO  ↘ YES
    │      └─→ Queue in thread->pending
    │         (will deliver after current returns)
    │
┌──────────────────────────────────┐
│ Q2: Is sig blocked in thread?    │
└──────────────────────────────────┘
    ↙ NO  ↘ YES
    │      └─→ Queue in thread->pending
    │         (blocked, wait for unblock)
    │
    └─→ Deliver to thread immediately
        (set TIF_SIGPENDING, wake if sleeping)
```

---

## Standard Signals (1-31)

### Non-Queueing Behavior: The "Last One Wins" Principle

```c
// Simplified representation of pending signal delivery

struct sigpending {
    struct list_head list;      // For real-time signals
    sigset_t signal;            // For standard signals: just a bitmask
};

/*
 For standard signals (1-31): Only the PRESENCE is tracked, not instances
 
 Time:    send(SIGTERM)  send(SIGTERM)  send(SIGTERM)
          ↓              ↓              ↓
 Kernel:  signal |= (1 << SIGTERM)
          signal = 0x4000...
          
          signal |= (1 << SIGTERM)  // No change!
          signal = 0x4000...        // Still same value
          
          signal |= (1 << SIGTERM)  // No change!
          signal = 0x4000...        // Still same value
 
 Result:  Signal delivered ONCE, not three times
          This is why standard signals are NOT suitable for counters
 
 Contrast with real-time signals (32-64):
 
 Time:    send(SIGRTMIN)  send(SIGRTMIN)  send(SIGRTMIN)
          ↓               ↓               ↓
 Kernel:  list.push(sigqueue{SIGRTMIN, info})
          list.push(sigqueue{SIGRTMIN, info})
          list.push(sigqueue{SIGRTMIN, info})
 
 Result:  Handler called THREE TIMES
          Each with separate siginfo_t (argument)
*/
```

### Signal Handler Registration (Standard Approach)

```c
// POSIX API: sigaction() - most flexible
int sigaction(int signum, const struct sigaction *act,
              struct sigaction *oldact);

struct sigaction {
    void (*sa_handler)(int);                    // Or SIG_DFL, SIG_IGN
    void (*sa_sigaction)(int, siginfo_t*, void*);
    sigset_t sa_mask;                           // Blocked during handler
    int sa_flags;                               // SA_SIGINFO, SA_RESTART, etc.
    void (*sa_restorer)(void);                  // Obsolete
};
```

### Important Signal Actions

#### SIGKILL & SIGSTOP (Uncatchable)

```c
// These two signals are SPECIAL in the kernel:
// - Cannot be caught
// - Cannot be blocked
// - Cannot be ignored

// Kernel code (kernel/signal.c):
static int sig_kernel_only(int sig)
{
    return sig < SIGRTMIN && sigismember(&sig_kernel_only_mask, sig);
}

// sig_kernel_only_mask includes:
//  - SIGKILL (cannot be caught/blocked)
//  - SIGSTOP (cannot be caught/blocked)
//
// This is enforced at:
//  1. sigaction() - reject sa_handler = SIG_DFL or custom
//  2. sigprocmask() - reject blocking SIGKILL/SIGSTOP
//  3. signal delivery - always deliver regardless of mask

// Real-world impact:
// You CANNOT gracefully shutdown on SIGKILL
// You MUST handle SIGTERM instead
//
// ┌─────────────────────────────────────┐
// │ Application Shutdown Sequence       │
// ├─────────────────────────────────────┤
// │ 1. Receive SIGTERM                  │
// │    (application can handle)         │
// │ 2. Cleanup: close files, flush DB   │
// │ 3. Exit gracefully                  │
// │ 4. If doesn't exit, OS sends SIGKILL│
// │    (forceful, no handler runs)      │
// └─────────────────────────────────────┘
```

#### SIGCHLD (Child Process Notifications)

```c
// SIGCHLD sent to parent when child:
// 1. Exits (status available in wait/waitpid)
// 2. Stops (on SIGSTOP/SIGTSTP)
// 3. Continues (after SIGCONT)

struct sigaction sa;
sa.sa_handler = child_handler;
sigemptyset(&sa.sa_mask);
sa.sa_flags = SA_NOCLDSTOP | SA_NOCLDWAIT;  // Flags control behavior
sigaction(SIGCHLD, &sa, NULL);

void child_handler(int sig) {
    pid_t pid;
    int status;
    
    // Reap all available children (non-blocking)
    while ((pid = waitpid(-1, &status, WNOHANG)) > 0) {
        // Process status
        if (WIFEXITED(status)) {
            printf("Child %d exited: %d\n", pid, WEXITSTATUS(status));
        } else if (WIFSIGNALED(status)) {
            printf("Child %d killed by: %d\n", pid, WTERMSIG(status));
        }
    }
}

// Flags:
// SA_NOCLDSTOP  - Don't send SIGCHLD when child stops/continues
// SA_NOCLDWAIT  - Don't create zombie processes (auto-reap)
```

#### SIGPIPE (Broken Pipe)

```c
// SIGPIPE sent when:
// 1. Write to pipe with no readers
// 2. Write to socket with closed connection
// 3. Default action: TERMINATE process

// Common pattern in network servers:
struct sigaction sa;
sa.sa_handler = SIG_IGN;           // IGNORE broken pipes
sigemptyset(&sa.sa_mask);
sa.sa_flags = 0;
sigaction(SIGPIPE, &sa, NULL);

// Alternative: Check write() return value for -1 (EPIPE)
if (write(fd, buf, len) == -1 && errno == EPIPE) {
    // Handle gracefully instead of terminating
    close_connection();
}

// Never ignore SIGPIPE blindly:
// - Silent failures are worse than crashes
// - Use try/catch style: write() → check error
```

---

## Real-Time Signals (32-64)

### Queueing Mechanism: The Queue Implementation

```c
// From: kernel/signal.c

// Allocate a sigqueue struct to queue a signal with argument
struct sigqueue *sigqueue_alloc(void)
{
    struct sigqueue *q = kmem_cache_alloc(sigqueue_cachep, GFP_ATOMIC);
    if (q) {
        INIT_LIST_HEAD(&q->list);
        q->flags = 0;
        q->user = get_current_user();
    }
    return q;
}

// Queue a signal with siginfo (argument)
int __send_signal(int sig, struct siginfo *info, struct task_struct *t,
                  int group, int from_ancestor_ns)
{
    struct sigpending *pending;
    struct sigqueue *q;

    // Determine target: shared_pending or thread-specific pending
    if (group)
        pending = &t->signal->shared_pending;
    else
        pending = &t->pending;

    // For standard signals: just set bit (no queueing)
    if (!is_si_special(info) && sig < SIGRTMIN) {
        if (sigismember(&pending->signal, sig))
            return 0;  // Already pending, don't queue again
        sigaddset(&pending->signal, sig);
        return 1;
    }

    // For real-time signals: allocate and queue
    q = sigqueue_alloc();
    if (!q) return -EAGAIN;

    copy_siginfo(&q->info, info);
    q->info.si_signo = sig;
    q->info.si_errno = 0;
    q->info.si_code = SI_QUEUE;

    // Add to queue (FIFO order)
    list_add_tail(&q->list, &pending->list);
    sigaddset(&pending->signal, sig);

    return 1;
}

// QUEUE OPERATION ILLUSTRATION:

Initial state (queue empty):
  pending->signal bitmask = 0
  pending->list = EMPTY

Operation: queue SIGRTMIN, arg=100
  ↓
  sigqueue *q1 = kmem_cache_alloc()
  q1->info.si_value.sival_int = 100
  list_add_tail(&q1->list, &pending->list)
  sigaddset(&pending->signal, SIGRTMIN)
  ↓
  pending->signal bitmask = (1 << SIGRTMIN)
  pending->list → [q1]

Operation: queue SIGRTMIN, arg=200
  ↓
  sigqueue *q2 = kmem_cache_alloc()
  q2->info.si_value.sival_int = 200
  list_add_tail(&q2->list, &pending->list)
  sigaddset(&pending->signal, SIGRTMIN)  // No change (already set)
  ↓
  pending->signal bitmask = (1 << SIGRTMIN)
  pending->list → [q1] → [q2]

Operation: next_signal() chooses SIGRTMIN
  ↓
  Extract first from list: q1
  info = q1->info (contains arg=100)
  Deliver signal with this info
  ↓
  pending->list → [q2]  // q1 removed
  Still has SIGRTMIN in pending (more queued)

Operation: deliver SIGRTMIN again
  ↓
  Extract: q2
  info = q2->info (arg=200)
  Deliver
  ↓
  pending->list → EMPTY
  sigdelset(&pending->signal, SIGRTMIN)  // No more queued
```

### RT Signal Argument Passing

```c
// Signal can be sent with 32-bit argument via sigqueue/rt_sigqueueinfo

union sigval {
    int sival_int;      // Integer argument
    void *sival_ptr;    // Pointer argument
};

// Syscall to queue signal with argument
int rt_sigqueueinfo(pid_t pid, int sig, siginfo_t *info);

// Receiver side: argument in handler
void handler(int sig, siginfo_t *info, void *context) {
    int arg = info->si_value.sival_int;
    printf("Received signal %d with argument %d\n", sig, arg);
}

// Use case: Priority queues
// Multiple senders → task → Priority queue of work items
//
// Thread 1: rt_sigqueueinfo(worker_pid, SIGRTMIN, {priority=10})
// Thread 2: rt_sigqueueinfo(worker_pid, SIGRTMIN, {priority=20})
// Thread 3: rt_sigqueueinfo(worker_pid, SIGRTMIN, {priority=15})
//
// Worker receives THREE deliveries in order:
// - SIGRTMIN with priority=10
// - SIGRTMIN with priority=20
// - SIGRTMIN with priority=15
```

---

## Signal Handling Mechanisms

### Synchronous vs Asynchronous

```
SYNCHRONOUS SIGNALS:
├─ Generated by process itself
├─ Caused by current instruction
├─ Examples: SIGSEGV, SIGFPE, SIGABRT
├─ Delivery: Immediate (before next instruction)
└─ Use: Error handling, assertions

    User Code:
    ↓
    int x = 1 / 0;  // FPU instruction generates exception
    ↓
    CPU → Interrupt handler
    ↓
    SIGFPE queued to process
    ↓
    Signal delivered BEFORE next user instruction
    ↓
    Handler executes


ASYNCHRONOUS SIGNALS:
├─ Generated by external events
├─ Not caused by current instruction
├─ Examples: SIGTERM (kill), SIGALRM (timer), SIGCHLD (child exit)
├─ Delivery: At next suitable point
└─ Use: IPC, timers, job control

    External Event:
    ├─ kill(pid, SIGTERM) from another process
    ├─ Timer expires (SIGALRM)
    └─ Child process exits (SIGCHLD)
    
    ↓
    Kernel notes signal pending (in signal bitmask)
    ↓
    Next time process returns from:
    ├─ syscall, or
    ├─ interrupt handler, or
    ├─ exception handler
    ↓
    Kernel checks TIF_SIGPENDING flag
    ↓
    Signal delivered


REAL-TIME IMPLICATIONS:

Synchronous:
    │
    ├─ CRITICAL: Must be delivered immediately
    ├─ Stack already in known state
    ├─ No task switching needed
    ├─ Kernel must interrupt current execution
    └─ Example: Page fault handler → SIGSEGV

Asynchronous:
    │
    ├─ Can be delayed (respects signal mask)
    ├─ Delivered at "safe" points
    ├─ May wake sleeping task
    ├─ Preemption point
    └─ Example: SIGTERM from kill(2)
```

### TIF_SIGPENDING Flag (Thread Info Flag)

```c
// From: include/linux/thread_info.h

// Per-thread flag bitmap in thread_info
// Quick way to check if signals need processing

struct thread_info {
    unsigned long flags;  // Bit 0-63 for various flags
    // ...
};

#define TIF_SIGPENDING  0   // Bit 0: Signals pending
#define TIF_NEED_RESCHED 1  // Bit 1: Schedule needed
// ... more flags ...

// CHECK:
if (test_thread_flag(TIF_SIGPENDING)) {
    do_signal(regs);  // Process pending signals
}

// SET (when signal delivered):
set_thread_flag(TIF_SIGPENDING);

// CLEAR (when all processed):
clear_thread_flag(TIF_SIGPENDING);

// ARCHITECTURE: ARM64 (arch/arm64/kernel/signal.c)
// 
// After syscall returns:
//
// ret_to_user:
//     ldr     x0, [sp]           // Load return value
//     cmp     x0, #0
//     bne     ret_fast_syscall
//
//     // Check thread flags
//     ldr     x1, [tsk, #TI_FLAGS]
//     tbnz    x1, #TIF_SIGPENDING, handle_signals
//     tbnz    x1, #TIF_NEED_RESCHED, handle_resched
//     // ... other checks ...
//
// handle_signals:
//     mov     x0, sp             // pt_regs
//     bl      do_signal

// This is WHY signals are checked at specific points:
// Not continuous CPU monitoring (too expensive)
// But at: syscall exit, exception exit, interrupt exit
```

### Signal Mask (sigset_t)

```c
// Block/unblock signals for a thread

int sigprocmask(int how, const sigset_t *set, sigset_t *oldset);

// how values:
// SIG_BLOCK   - Add *set to current mask
// SIG_UNBLOCK - Remove *set from current mask
// SIG_SETMASK - Replace mask with *set

// Example: Block SIGTERM and SIGINT
sigset_t mask;
sigemptyset(&mask);
sigaddset(&mask, SIGTERM);
sigaddset(&mask, SIGINT);
sigprocmask(SIG_BLOCK, &mask, NULL);

// KERNEL IMPLEMENTATION:

// task_struct.blocked = set of blocked signals
// When signal delivered:
//
// if (sigismember(&current->blocked, sig)) {
//     // Signal is blocked
//     // Add to pending instead of delivering
//     sigaddset(&current->pending.signal, sig);
//     return;  // Don't call handler now
// }
// // Signal is unblocked, deliver immediately

// Unblock and wait for signal:
// sigset_t mask;
// sigemptyset(&mask);
// sigaddset(&mask, SIGUSR1);
// sigsuspend(&mask);  // Block all except SIGUSR1, sleep
//                      // Atomically: set mask, sleep, restore mask
//                      // Wakes on SIGUSR1 delivery

// ATOMIC SWAP (CRITICAL!):
// 
// Without atomicity:
// Thread: sigprocmask(SIG_SETMASK, &new_mask);
// ... (preempted here)
// Signal arrives between syscall and actual mask update
// Kernel: Can deliver signal incorrectly
//
// With atomicity (sigprocmask done in one syscall):
// Kernel: Atomically:
//   saved_mask = current->blocked
//   current->blocked = new_mask
//   if (signal pending and not blocked) deliver_now()
//   return saved_mask
```

### Suspended Signals vs Delivered Signals

```c
// Scenario: SIGTERM received, but process has it blocked
// What happens?

Process state:
    blocked = {SIGTERM}
    pending.signal = 0
    
Signal SIGTERM sent (kill -SIGTERM <pid>):
    ↓
    Kernel checks: blocked? YES
    ↓
    Add to pending: pending.signal |= (1 << SIGTERM)
    ↓
    DO NOT call handler yet
    
Process continues running (doesn't know signal came)

Time passes, process unblocks SIGTERM:
    sigprocmask(SIG_UNBLOCK, &unmask_sigterm, NULL);
    ↓
    Kernel: Recalculate pending signals
    ↓
    Kernel finds pending SIGTERM that's no longer blocked
    ↓
    IMMEDIATELY deliver (don't wait for next syscall)
    
// This is why signal handlers must be atomic (from user perspective):
// Handler can be called at "unexpected" times
// Even after process explicitly unblocked it


// CRITICAL SECTION PATTERN:

sigset_t oldmask, newmask;
sigemptyset(&newmask);
sigaddset(&newmask, SIGUSR1);

// Enter critical section
sigprocmask(SIG_BLOCK, &newmask, &oldmask);  // Save old mask

// ... critical work: modify global data, no SIGUSR1 handler ...
// (SIGUSR1 is blocked here, if it arrives it goes to pending)

// Exit critical section
sigprocmask(SIG_SETMASK, &oldmask, NULL);    // Restore mask
// If SIGUSR1 was pending, delivered NOW

// Better pattern: rt_sigprocmask (realtime signals)
int rt_sigprocmask(int how, const sigset_t *set, 
                   sigset_t *oset, size_t sigsetsize);
// Atomic, handles 64 signals instead of 32
```

---

## Synchronous Signal Delivery

### Exception Handling to Signal Delivery

```
ARCHITECTURE: x86-64 Page Fault Example

Memory access: touch unmapped page
    ↓
CPU: #PF exception (vector 14)
    ↓
┌─────────────────────────────────────────────┐
│ Exception Handler (arch/x86/kernel/trap.c)  │
└─────────────────────────────────────────────┘
    ↓
page_fault_handler(struct pt_regs *regs):
    ↓
    ┌────────────────────────────────┐
    │ 1. Decode fault address (CR2)  │
    │ 2. Check VMA (valid memory?)   │
    └────────────────────────────────┘
    ↓
    If valid VMA → allocate page (page fault servicing)
    If invalid → goto step 3
    ↓
    ┌────────────────────────────────┐
    │ 3. Send SIGSEGV to process     │
    │    (current task_struct)       │
    └────────────────────────────────┘
    ↓
    force_sig_fault(SIGSEGV, si_code, addr, regs)
    ↓
    ┌──────────────────────────────────┐
    │ 4. Queue signal & set TIF_SIGPENDING │
    │    (even if blocked)             │
    └──────────────────────────────────┘
    ↓
    Return from exception handler → regs modified
    ↓
    ┌──────────────────────────────────┐
    │ 5. Before returning to user mode │
    │    Check TIF_SIGPENDING          │
    │    Call do_signal(regs)          │
    └──────────────────────────────────┘
    ↓
    Signal delivered (handler called)


CRITICAL: Synchronous signals are delivered IMMEDIATELY

EXCEPTION HANDLER RETURNS:
    Check TIF_SIGPENDING? YES
        ↓ do_signal(regs)
        (handler executes)
    
    NOT:
        Return to user code
        Eventually deliver

This prevents:
    - Infinite loop on bad instruction (would keep faulting)
    - Resource exhaustion (would keep allocating stacks)
    - Non-determinism (race to next syscall)


KERNEL CODE: force_sig_fault() - Forced Signal Delivery

void force_sig_fault(int sig, int code, void __user *addr,
                     struct task_struct *t)
{
    siginfo_t info;
    unsigned long flags;
    
    spin_lock_irqsave(&t->sighand->siglock, flags);
    
    // IGNORE signal mask for synchronous signals
    // SIGKILL and synchronous are not maskable
    action = &t->sighand->action[sig-1];
    
    if (action->sa_handler == SIG_DFL) {
        // Default action (usually core dump + terminate)
        handle_default_signal(sig, t);
        spin_unlock_irqrestore(&t->sighand->siglock, flags);
        return;
    }
    
    // Queue signal (ignore if already pending)
    init_siginfo(&info, SI_KERNEL, sig, addr, code);
    __send_signal(sig, &info, t, SIGINFO);
    
    // FORCE delivery (bypass mask and pending check)
    if (!(t->blocked & (1UL << (sig-1)))) {
        set_thread_flag(TIF_SIGPENDING);
    }
    
    spin_unlock_irqrestore(&t->sighand->siglock, flags);
}
```

### Floating Point Exception (SIGFPE)

```c
// Synchronous signal: arithmetic error

// TRIGGER: FPU exception
    double x = 1.0 / 0.0;      // Division by zero
    int y = 1 / 0;             // Integer division by zero
    
// Kernel: FPU exception handler
    ↓ (arch/x86/kernel/fpu/signal.c)
    ↓
    force_sig_fault(SIGFPE, FPE_INTDIV, addr, regs);
    
// Handler:
void fpe_handler(int sig, siginfo_t *info, void *context) {
    // info->si_code indicates:
    //   FPE_INTDIV - integer division by zero
    //   FPE_INTOVF - integer overflow
    //   FPE_FLTDIV - floating division by zero
    //   FPE_FLTOVF - floating overflow
    //   FPE_FLTUND - floating underflow
    //   FPE_FLTRES - floating rounding error
    //   FPE_FLTINV - floating invalid operation
    //   FPE_FLTSUB - subscript out of range
    
    ucontext_t *uctx = (ucontext_t *) context;
    
    // Inspect FPU state
    struct fpstate *fps = &uctx->uc_mcontext.fpregs;
    // Can examine FPU registers, exception code
}
```

### Illegal Instruction (SIGILL)

```c
// Synchronous signal: CPU fetched illegal opcode

// TRIGGER: CPU instruction decode error
    asm volatile(".byte 0x0f, 0x0f, 0x0f");  // Invalid opcode

// Kernel: Illegal instruction handler
    ↓ (arch/x86/kernel/traps.c)
    ↓
    force_sig_fault(SIGILL, ILL_ILLOPC, addr, regs);
    
// Handler:
void ill_handler(int sig, siginfo_t *info, void *context) {
    // info->si_code indicates:
    //   ILL_ILLOPC - illegal opcode
    //   ILL_ILLOPN - illegal operand
    //   ILL_ILLADDRM - illegal addressing mode
    //   ILL_ILLTRP - illegal trap
    //   ILL_PRVOPC - privileged opcode
    //   ILL_PRVREG - privileged register
    //   ILL_COPROC - coprocessor error
    //   ILL_BADSTK - internal stack error
    
    ucontext_t *uctx = (ucontext_t *) context;
    
    // Can inspect RIP to find bad instruction
    unsigned long rip = uctx->uc_mcontext.rip;
    unsigned char *bad_instr = (unsigned char *) rip;
}
```

### Segmentation Fault (SIGSEGV)

```c
// Synchronous signal: Invalid memory access

// TRIGGER: Access unmapped memory
    int *p = (int *) 0xdeadbeef;
    *p = 42;  // Load from unmapped address
    
// Kernel: Page fault handler
    ↓ (arch/x86/mm/fault.c)
    ↓
    // Check if address is mapped
    // Check if access violates permissions
    // Check if stack guard page violated
    ↓
    If not recoverable:
        force_sig_fault(SIGSEGV, segv_code, addr, regs);
    
    segv_code values:
    SEGV_MAPERR - address not mapped to object
    SEGV_ACCERR - invalid permissions for mapped object
    SEGV_BNDERR - failed address bound checks
    SEGV_PKUERR - failed PKey protection checks
    
// Handler:
void segv_handler(int sig, siginfo_t *info, void *context) {
    void *fault_addr = info->si_addr;  // Address that faulted
    int code = info->si_code;          // SEGV_MAPERR, SEGV_ACCERR, etc.
    
    ucontext_t *uctx = (ucontext_t *) context;
    
    // For malloc corruption detection:
    if (fault_addr < current_heap_start || 
        fault_addr > current_heap_end) {
        fprintf(stderr, "Invalid address: %p\n", fault_addr);
        print_stack_trace(uctx);
        exit(1);
    }
    
    // Could attempt recovery (setjmp/longjmp to safe point)
    longjmp(recovery_point, 1);
}

// Alternative: Mmap-based handler
// Map guard page, fault handler installs new page
// (used in some GC implementations for barrier detection)
```

---

## Asynchronous Signal Delivery

### Interrupt-Driven Signal Delivery

```
TIMER INTERRUPT EXAMPLE (Linux uses hrtimer/jiffies):

Hardware Timer:
    Generates interrupt at HZ (usually 100-1000 Hz)
    ↓
    interrupt_handler() called
    ↓
    Increment jiffies counter
    ↓
    Decrement process timers:
        - SIGALRM (if alarm set)
        - SIGPROF (if profiling)
        - SIGVTALRM (if virtual timer)
    ↓
    Timer expired?
        → Queue SIGALRM
        → Set TIF_SIGPENDING
    ↓
    Return from interrupt
    ↓
    Kernel checks TIF_SIGPENDING
    ↓
    Signal delivered at next return to user space


SCHEDULER INTERPLAY:

Process runs user code
    ↓ (context switch)
    ↓ (no signal yet)
    
Another process runs
    ↓
    Time passes (timer ticks)
    ↓
    Timer interrupt handler
    ↓
    SIGALRM expired
    → Queue signal in original process
    → Set TIF_SIGPENDING in original task_struct
    
    Scheduler:
    Other process continues
    ↓
    Eventually returns to original process
    ↓
    Kernel: Check TIF_SIGPENDING
    ↓
    Deliver SIGALRM handler
    
Result:
    Signal delayed until process gets CPU time again
    Can be delayed by:
    - Other threads with higher priority
    - Scheduler latency
    - But not indefinitely (scheduler ensures fairness)
```

### Kill System Call: Process-to-Process Signals

```c
// int kill(pid_t pid, int sig);
// Send signal to process

// Kernel implementation (simplified):
SYSCALL_DEFINE2(kill, pid_t, pid, int, sig)
{
    struct siginfo info;
    struct pid *grp;
    int ret = 0;

    if (!valid_signal(sig))
        return -EINVAL;

    clear_siginfo(&info);
    info.si_signo = sig;
    info.si_errno = 0;
    info.si_code = SI_USER;        // Sent via kill()
    info.si_pid = task_tgid_vnr(current);
    info.si_uid = from_kuid_munged(current_user_ns(), current_uid());

    rcu_read_lock();
    
    if (pid > 0) {
        // Send to specific PID
        p = find_task_by_vpid(pid);
        if (p)
            ret = send_signal(sig, &info, p, PIDTYPE_PID);
    }
    else if (pid == -1) {
        // Send to all processes (except current, init)
        for_each_process(p) {
            if (p->pid > 1 && has_ns_capability(current, p->user_ns, CAP_KILL))
                ret = send_signal(sig, &info, p, PIDTYPE_PID);
        }
    }
    else if (pid < -1) {
        // Send to process group (-pid)
        grp = find_vpid(-pid);
        do_each_pid_task(grp, PIDTYPE_PGID, p) {
            ret = send_signal(sig, &info, p, PIDTYPE_PGID);
        } while_each_pid_task(grp, PIDTYPE_PGID, p);
    }
    else if (pid == 0) {
        // Send to process group of sender
        ret = send_signal(sig, &info, current, PIDTYPE_PGID);
    }
    
    rcu_read_unlock();
    return ret;
}

// SECURITY: CAP_KILL capability required to send signals
// to processes not owned by same user
// Exception: SIGCONT can be sent to background jobs
```

### Pthread_kill: Thread-Targeted Signals

```c
// int pthread_kill(pthread_t thread, int sig);
// Send signal to specific thread in multithreaded process

// Kernel-level:
// CLONE_SIGHAND threads share sighand_struct (signal handlers)
// But each thread has own task_struct with separate pending queue

SYSCALL_DEFINE2(tgkill, pid_t, tgid, pid_t, pid, int, sig)
{
    // tgkill(process_id, thread_id, sig)
    // More secure than tkill (specifies both)
    
    struct task_struct *p;
    const struct cred *cred, *tcred;
    int error;

    error = -EINVAL;
    if (!valid_signal(sig))
        goto out;

    error = -ESRCH;
    rcu_read_lock();
    
    // Find process by PID
    p = find_task_by_vpid(tgid);
    if (!p)
        goto out_unlock;

    // Verify thread is in that process (tgid_for_pid)
    if (!same_thread_group(p, tgid))
        goto out_unlock;

    // Find specific thread
    p = find_task_by_vpid(pid);
    if (!p || p->tgid != tgid)
        goto out_unlock;

    // Permission check
    error = check_kill_permission(sig, &tcred, p);
    if (error)
        goto out_unlock;

    // Queue signal to SPECIFIC thread (not process group)
    error = __send_signal(sig, NULL, p, PIDTYPE_PID, SI_TKILL);

    rcu_read_unlock();
    return error;
}

// Result: Signal goes to thread's pending queue, not shared_pending
// Other threads in process won't handle it
```

### Kill from User Space

```c
// C code: send SIGTERM to process

#include <signal.h>
#include <unistd.h>
#include <errno.h>

pid_t target = 1234;
int ret = kill(target, SIGTERM);

if (ret == -1) {
    switch (errno) {
        case ESRCH:
            printf("Process %d doesn't exist\n", target);
            break;
        case EPERM:
            printf("No permission to send signal\n");
            break;
        case EINVAL:
            printf("Invalid signal number\n");
            break;
    }
}

// Rust equivalent:
use nix::unistd::Pid;
use nix::sys::signal::{kill, Signal};

let pid = Pid::from_raw(1234);
kill(pid, Signal::SIGTERM)?;
```

---

## Signal Masks & Blocking

### Mask Operations: Bitmask Algebra

```c
// sigset_t operations

// Initialize empty set
sigset_t set;
sigemptyset(&set);          // set = 0x0000...

// Add signal to set
sigaddset(&set, SIGUSR1);   // set |= (1 << (SIGUSR1 - 1))
sigaddset(&set, SIGUSR2);   // set |= (1 << (SIGUSR2 - 1))
// set = 0x0000...0CC0...  (bits 10, 11 set)

// Remove signal from set
sigdelset(&set, SIGUSR1);   // set &= ~(1 << (SIGUSR1 - 1))
// set = 0x0000...0800...  (only bit 11 set)

// Test membership
if (sigismember(&set, SIGUSR2))  // if (set & (1 << (SIGUSR2 - 1)))
    // SIGUSR2 is in set

// Fill with all signals
sigfillset(&set);           // set = 0xFFFF...FFFF


KERNEL EQUIVALENT: include/linux/signal.h

#define _NSIG           64
#define _NSIG_WORDS     (_NSIG / (8 * sizeof(unsigned long)))

// On 64-bit: _NSIG_WORDS = 1
// On 32-bit: _NSIG_WORDS = 2

typedef struct {
    unsigned long sig[_NSIG_WORDS];
} sigset_t;

// Kernel macros:
#define sigaddset(set, sig) \
    ((set)->sig[_NSIG_WORD(sig)] |= _NSIG_BIT(sig))

#define sigdelset(set, sig) \
    ((set)->sig[_NSIG_WORD(sig)] &= ~_NSIG_BIT(sig))

#define sigismember(set, sig) \
    (((set)->sig[_NSIG_WORD(sig)] & _NSIG_BIT(sig)) != 0)

Where:
#define _NSIG_WORD(sig)  (((sig) - 1) >> (BITS_PER_LONG - 1))
#define _NSIG_BIT(sig)   (1UL << (((sig) - 1) & (BITS_PER_LONG - 1)))

On 64-bit system:
  Bit shifts >> 6  → index in array (all signals fit in one 64-bit word)
  Bit shifts & 63  → bit position within word
```

### Critical Section Protection

```c
// PATTERN 1: Block signals during critical section

sigset_t oldmask, newmask;
sigemptyset(&newmask);
sigaddset(&newmask, SIGUSR1);

// ENTER critical section
sigprocmask(SIG_BLOCK, &newmask, &oldmask);

// Modify shared data (protected from SIGUSR1 handler)
global_counter++;
global_list.append(new_item);

// EXIT critical section
sigprocmask(SIG_SETMASK, &oldmask, NULL);
// Signals delivered here if SIGUSR1 was pending


// PATTERN 2: Atomic sleep + signal wait

volatile int flag = 0;

void handler(int sig) {
    flag = 1;
}

void wait_for_signal(void) {
    sigset_t mask;
    sigemptyset(&mask);
    sigaddset(&mask, SIGUSR1);
    
    // Atomically: block SIGUSR1, sleep, restore
    // Without sigsuspend, race condition:
    //   1. Check flag (still 0)
    //   2. Thread scheduled away
    //   3. Signal arrives, sets flag=1, tries to wake
    //   4. But thread is about to sleep
    //   5. Signal lost!
    
    sigprocmask(SIG_BLOCK, &mask, NULL);
    
    while (!flag) {
        sigsuspend(&mask);
        // Atomically:
        // 1. Set signal mask to &mask (unblock SIGUSR1)
        // 2. Enter interruptible sleep
        // 3. Signal delivered, wakes us
        // 4. Restore signal mask
    }
}

// Key: sigsuspend is SINGLE SYSCALL, atomic from user perspective
// No window for signal to be lost
```

### Realtime Signal Delivery with Masks

```c
// Real-time signals respect masks too

void handler(int sig, siginfo_t *info, void *context) {
    printf("Got SIGRTMIN with arg=%d\n", info->si_value.sival_int);
}

main() {
    struct sigaction sa;
    sa.sa_sigaction = handler;
    sigemptyset(&sa.sa_mask);
    sa.sa_flags = SA_SIGINFO;
    sigaction(SIGRTMIN, &sa, NULL);
    
    // Block SIGRTMIN
    sigset_t set;
    sigemptyset(&set);
    sigaddset(&set, SIGRTMIN);
    sigprocmask(SIG_BLOCK, &set, NULL);
    
    // Send 3 signals
    union sigval val;
    val.sival_int = 100;
    rt_sigqueueinfo(getpid(), SIGRTMIN, &info);  // Queued but blocked
    
    val.sival_int = 200;
    rt_sigqueueinfo(getpid(), SIGRTMIN, &info);  // Queued but blocked
    
    val.sival_int = 300;
    rt_sigqueueinfo(getpid(), SIGRTMIN, &info);  // Queued but blocked
    
    sleep(1);  // Let signals queue
    
    // Now unblock - all 3 delivered in order
    sigprocmask(SIG_UNBLOCK, &set, NULL);
    
    // Output:
    // Got SIGRTMIN with arg=100
    // Got SIGRTMIN with arg=200
    // Got SIGRTMIN with arg=300
    
    // Each handler invocation is separate
}
```

---

## Kernel Implementation Details

### Signal Detection and Routing (kernel/signal.c)

```c
// Called from:
// 1. Exception handlers
// 2. Syscall exit
// 3. Interrupt return
// Always with preemption disabled

int do_signal(struct pt_regs *regs)
{
    struct k_sigaction ka;
    siginfo_t info;
    int signr;

    if (unlikely(current->blocked == 0)) {
        // Fast path: no signals blocked, might skip processing
        // Still must check pending
    }

    current->restart_block.fn = do_no_restart_syscall;

    // SIGNAL LOOP: Process one signal at a time
    while (true) {
        // 1. Recalculate which signals are actually pending
        //    (some may have been pending but now have handler that ignores)
        recalc_sigpending();

        // 2. Get next signal to deliver
        //    Checks: shared_pending, thread pending, signal masks
        signr = next_signal(&current->pending, &current->blocked);

        // signr = 0 means no pending signals
        if (signr == 0)
            break;

        // 3. Get signal action from sighand_struct
        ka = current->sighand->action[signr-1];

        // 4. Handle default actions
        if (ka.sa.sa_handler == SIG_DFL) {
            if (sig_kernel_stop(signr)) {
                // SIGSTOP, SIGTSTP, SIGTTIN, SIGTTOU
                // Stop the process, don't run handler
                do_signal_stop(signr);
                continue;  // Process next signal
            }
            
            if (sig_fatal(signr)) {
                // SIGKILL, SIGQUIT, SIGABRT, etc.
                // Terminate process
                do_group_exit(signr);  // Dies here
            }
            
            // Other defaults: ignore
            continue;
        }

        // 5. User installed handler
        //    Setup user-space frame
        if (!setup_rt_frame(signr, &ka, &info, &current->blocked, regs))
            return signr;  // Couldn't set up frame (likely ENOMEM)

        // 6. Signal successfully delivered
        //    (frame set up, handler will run when we return)
        //    Continue to deliver more signals if any
    }

    // 7. No more signals - return to user space
    return 0;
}


// NEXT_SIGNAL: Find which signal to deliver

int next_signal(struct sigpending *pending, sigset_t *blocked)
{
    unsigned long i, *s, *m;
    int sig = 0;

    // Combine shared_pending and thread pending
    s = pending->signal.sig;      // Pending bitmask
    m = blocked->sig;              // Blocked bitmask

    // Search for first pending and unblocked signal
    // Signals checked in priority order:
    // 1. Standard signals (1-31) - higher priority
    // 2. Real-time signals (32-64) - lower priority
    
    for (i = 0; i < _NSIG_WORDS; i++) {
        unsigned long idx = *s & ~*m;  // pending AND NOT blocked
        
        if (idx) {
            sig = ffz(~idx) + i * BITS_PER_LONG + 1;
            break;
        }
        s++;
        m++;
    }

    return sig;
}

// ffz(~idx) = find first zero in ~idx
//           = find first ONE in idx
//           = first set bit position

// Priority is due to iteration order (1-31 checked before 32-64)


// RECALC_SIGPENDING: Recompute which signals are actually pending

void recalc_sigpending(void)
{
    sigset_t set;

    // Get union of:
    // 1. shared_pending.signal (for whole process)
    // 2. current->pending.signal (for this thread)
    sigorsets(&set, &current->pending.signal,
              &current->signal->shared_pending.signal);

    // But subtract signals with SIG_IGN action
    // These are no longer "really pending"
    for (int i = 1; i <= _NSIG; i++) {
        if (sigismember(&set, i) &&
            current->sighand->action[i-1].sa.sa_handler == SIG_IGN)
            sigdelset(&set, i);
    }

    current->pending.signal = set;

    // Update TIF_SIGPENDING for fast path in architecture code
    if (sigisemptyset(&current->pending.signal))
        clear_thread_flag(TIF_SIGPENDING);
    else
        set_thread_flag(TIF_SIGPENDING);
}
```

### Frame Setup: setup_rt_frame()

```c
// Architecture-specific: arch/x86/kernel/signal.c
// Prepares stack for signal handler execution

int setup_rt_frame(int sig, struct k_sigaction *ka, 
                   siginfo_t *info, sigset_t *set,
                   struct pt_regs *regs)
{
    // Frame layout on stack:
    //   [return address → __restore_rt]
    //   [ucontext_t with machine state]
    //   [siginfo_t]
    //   (RSP grows down)

    struct rt_sigframe __user *frame;
    void __user *restorer;
    unsigned long uc_flags;

    // Allocate frame on user stack
    // Decrease RSP by frame size
    frame = (struct rt_sigframe __user *)
            round_down(regs->sp - sizeof(struct rt_sigframe), 16) - 8;

    // Access control: check user can write stack
    if (__copy_to_user(&frame->sig, &sig, sizeof(sig)))
        return -EFAULT;

    // Copy siginfo_t to user space
    if (copy_siginfo(&frame->info, info))
        return -EFAULT;

    // Save full CPU context
    if (__copy_to_user(&frame->uc.uc_mcontext, &regs, sizeof(*regs)))
        return -EFAULT;

    // Save signal mask (to restore after handler)
    if (__copy_to_user(&frame->uc.uc_sigmask, set, sizeof(*set)))
        return -EFAULT;

    // Set return address: jump to __restore_rt when handler returns
    // (usually in vDSO, sometimes in libc)
    restorer = (void __user *)current->mm->context.vdso +
               (current->mm->context.vdso_offset >> PAGE_SHIFT) * PAGE_SIZE +
               (current->mm->context.vdso_signal_offset);

    // Modify saved return address
    if (__put_user(restorer, &frame->pretcode))
        return -EFAULT;

    // Set up CPU registers for handler call:
    // RDI (arg0) = signal number
    // RSI (arg1) = &frame->info (siginfo_t*)
    // RDX (arg2) = &frame->uc (ucontext_t*)
    regs->di = sig;
    regs->si = (unsigned long)&frame->info;
    regs->dx = (unsigned long)&frame->uc;

    // Jump to handler on return
    regs->rip = (unsigned long)ka->sa.sa_sigaction;

    // New stack pointer
    regs->rsp = (unsigned long)frame;

    // Clear registers to avoid information leaks
    regs->r8 = 0;
    regs->r9 = 0;
    regs->r10 = 0;
    regs->r11 = 0;

    return 0;
}

// HANDLER CALLING CONVENTION:
//   rdi = signal number
//   rsi = siginfo_t*
//   rdx = ucontext_t*
//   
// This matches:
//   void handler(int sig, siginfo_t *info, void *context)

// HANDLER RETURN:
//   handler calls return
//   return address (set to __restore_rt)
//   __restore_rt executes rt_sigreturn syscall
//   Kernel restores CPU state from frame->uc.uc_mcontext
//   Continues user code from saved RIP


// SIGNAL RETURN (arch/x86/kernel/signal.c)

SYSCALL_DEFINE0(rt_sigreturn)
{
    struct pt_regs *regs = current_pt_regs();
    struct rt_sigframe __user *frame;
    sigset_t set;

    frame = (struct rt_sigframe __user *)(regs->sp - sizeof(long));

    // Restore CPU state from ucontext
    if (__copy_from_user(&regs->ax, &frame->uc.uc_mcontext,
                         sizeof(struct sigcontext)))
        goto badframe;

    // Restore signal mask
    if (__copy_from_user(&set, &frame->uc.uc_sigmask, sizeof(set)))
        goto badframe;

    sigprocmask(SIG_SETMASK, &set, NULL);

    // CPU registers now restored, handler context forgotten
    // Execution continues from saved RIP with saved RSP
    return regs->ax;

badframe:
    // Memory corruption or bad address in frame
    force_sig(SIGSEGV, current);
    return 0;
}

// CRITICAL: Registers are FULLY RESTORED
// Any modifications in handler are LOST
// Unless handler modifies frame->uc.uc_mcontext before return
//
// Example: longjmp() implementation modifies uc_mcontext
// to jump to safe location
```

### Atomicity and Spinlocks

```c
// sighand_struct is protected by spinlock

struct sighand_struct {
    spinlock_t siglock;
    struct k_sigaction action[_NSIG];
};

// CRITICAL SECTION 1: Modify signal action

int sigaction(int sig, const struct sigaction *act, 
              struct sigaction *oldact)
{
    struct k_sigaction new_ka, old_ka;
    int ret;

    // ... copy from user space ...

    spin_lock_irq(&current->sighand->siglock);
    {
        old_ka = current->sighand->action[sig-1];
        
        // ATOMIC: No signal can be delivered while modifying action
        // (IRQs disabled, spinlock held)
        current->sighand->action[sig-1] = new_ka;
    }
    spin_unlock_irq(&current->sighand->siglock);

    // ... copy to user space ...
}

// Why spinlock + irq disabled?
// Without irq disabled:
//   1. Thread A: spin_lock(&siglock)
//   2. Interrupt arrives
//   3. do_signal() tries to read action[] →  spin_lock(again)
//   4. DEADLOCK! (spinlock not re-entrant)
//
// Solution: disable IRQs while holding spinlock
// Prevents interrupt handlers from trying to acquire it

// CRITICAL SECTION 2: Send signal

int __send_signal(int sig, struct siginfo *info, task_struct *t,
                  int group, int from_ancestor_ns)
{
    struct sigpending *pending;
    
    // Which pending queue?
    if (group)
        pending = &t->signal->shared_pending;
    else
        pending = &t->pending;

    // ATOMIC modification: protect with siglock
    spin_lock_irq(&t->sighand->siglock);
    {
        // Add to pending queue or bitmask
        // Set TIF_SIGPENDING in thread
        // Determine if signal was already pending (no lost signals)
        
        // For real-time signals:
        sigqueue *q = sigqueue_alloc();
        list_add_tail(&q->list, &pending->list);
        sigaddset(&pending->signal, sig);
        
        // For standard signals:
        // sigaddset(&pending->signal, sig);  // Idempotent
    }
    spin_unlock_irq(&t->sighand->siglock);

    // Outside lock: wake thread if sleeping
    signal_wake_up(t, 0);
}

// RACE CONDITION EXAMPLE (without lock):
// Thread A:        Kernel:
// 1. send(SIGUSR1)
// 2. [preempted]   2. [reading action[SIGUSR1]]
// 3. [back]        3. [installing default handler]
// 4. action now shows IGNORE
// 5. [preempted]
//                  4. [check action[SIGUSR1]]
//                  5. [It's IGNORE, don't deliver!]
//
// With lock: step 3 & 4 are atomic, can't interleave
```

---

## Race Conditions & Atomicity

### Signal Loss in Multithreaded Code

```c
// PROBLEM: Race between signal and continuation

volatile int flag = 0;

void handler(int sig) {
    flag = 1;
}

void wait_for_signal(void) {
    // WRONG PATTERN (race condition):
    
    while (!flag) {
        // Check 1: flag is 0
        
        // RACE: Signal can arrive here!
        // Thread scheduled away
        // Signal delivered, sets flag = 1
        // But we're already about to sleep
        // Signal goes to pending, then delivered
        // But we won't wake up!
        
        sleep(1);
        // Sleep on condition: "wake me if flag == 1"
        // But flag already set to 1 before we slept!
        // Lost notification!
    }
}

// CORRECT PATTERN: Atomic sleep + signal wait

void wait_for_signal(void) {
    sigset_t mask, oldmask;
    
    sigemptyset(&mask);
    sigaddset(&mask, SIGUSR1);
    
    // Atomically: block signal, check flag, sleep
    sigprocmask(SIG_BLOCK, &mask, &oldmask);
    
    while (!flag) {
        // Now signal is blocked
        // Even if delivered, goes to pending
        // Won't interrupt sleep
        
        sigsuspend(&oldmask);
        // Single syscall does:
        // 1. Set signal mask to oldmask (UNBLOCK SIGUSR1)
        // 2. Enter interruptible sleep
        // 3. Return when signal delivered
        // 4. Kernel restores mask to current (block again)
        
        // Race-free because:
        // - Between blocking and sleep, same CPU
        // - Signal is either queued or wakes us
        // - No gap where signal lost
    }
    
    sigprocmask(SIG_SETMASK, &oldmask, NULL);  // Restore mask
}
```

### Real-Time Signal Overflow

```c
// Real-time signals QUEUE, but queue has limits

// Kernel limit:
// Each signal can queue max 32 instances
// (Per implementation, usually)

void overflow_test(void) {
    union sigval val;
    
    for (int i = 0; i < 100; i++) {
        val.sival_int = i;
        int ret = rt_sigqueueinfo(getpid(), SIGRTMIN, &info);
        
        if (ret == -1 && errno == EAGAIN) {
            // Queue full!
            // Additional signals will be LOST (like standard signals)
            printf("Signal queue overflow at %d\n", i);
            break;
        }
    }
}

// Kernel queue allocation (kernel/signal.c):

struct sigqueue *sigqueue_alloc(void) {
    struct sigqueue *q = kmem_cache_alloc(sigqueue_cachep, GFP_ATOMIC);
    
    if (!q)
        return NULL;  // Allocation failed
    
    // Also check per-user limit (prevent DoS)
    if (!atomic_inc_not_zero(&current_user()->sigpending)) {
        kmem_cache_free(sigqueue_cachep, q);
        return NULL;  // User's queue limit exceeded
    }
    
    return q;
}

// Result:
// - Queue full → rt_sigqueueinfo() returns EAGAIN
// - Application must handle overflow
// - Any pending signals delivered
// - Newly queued ones dropped
```

### Signal Race in Multithreaded Libraries

```c
// PATTERN: Library needs to use signals internally
// But user application also uses signals
// RACE: Competing handlers

// Example: tcmalloc uses SIGPROF for profiling
// User installs own SIGPROF handler
// CLASH!

// Solution 1: Use realtime signals
// tcmalloc uses SIGPROF (27)
// Application uses SIGRTMIN (32)
// No conflict

// Solution 2: Handler chaining
struct sigaction original_handler;

void wrapped_handler(int sig, siginfo_t *info, void *context) {
    // Do library work
    // ...
    
    // Chain to original handler
    if (original_handler.sa_handler != SIG_IGN &&
        original_handler.sa_handler != SIG_DFL) {
        
        if (original_handler.sa_flags & SA_SIGINFO) {
            original_handler.sa_sigaction(sig, info, context);
        } else {
            original_handler.sa_handler(sig);
        }
    }
}

main() {
    // Application installs signal
    struct sigaction sa;
    sa.sa_handler = app_handler;
    sigaction(SIGALRM, &sa, &original_handler);  // Save original
    
    // Library later installs own (overwrites app's)
    // But app's handler is in original_handler
    // Library calls wrapped_handler
    // wrapped_handler → calls original app handler
}

// LESSON: Multithreaded + multilibrary = signal chaos
// Prefer:
// - Event loops (libevent, libev)
// - Message queues (eventfd, timerfd)
// - Signals only in main thread
```

---

## Signal Debugging & Tracing

### Strace: Trace Signal Delivery

```bash
$ strace -f -e trace=signal ./myapp

# Output shows:
# - sigaction() calls (handler registration)
# - sigprocmask() calls (blocking/unblocking)
# - Signal delivery (--- SIGTERM ---)
# - Handler return (continued)

# Example trace:
sigaction(SIGUSR1, {sa_handler=0x7f123456, sa_mask=[], sa_flags=SA_RESTART}, 
          {sa_handler=SIG_DFL, sa_mask=[], sa_flags=0}) = 0

kill(1234, SIGUSR1) = 0

--- SIGUSR1 {si_signo=SIGUSR1, si_code=SI_USER, si_pid=5678, si_uid=1000} ---
```

### GDB Signal Debugging

```bash
# Setup
(gdb) handle SIGUSR1 nostop noprint  # Don't stop on SIGUSR1
(gdb) catch signal SIGUSR1            # Breakpoint on signal delivery

# During debug:
(gdb) info signals                    # Show signal dispositions
(gdb) signal SIGUSR1                  # Send signal to debugged process
(gdb) call handler(SIGUSR1, 0, 0)    # Call handler manually
(gdb) info threads                    # Which thread running handler?

# Signal frame inspection:
Program received signal SIGTERM, Terminated.
handler (sig=15) at signal_handler.c:42
42          printf("Got signal %d\n", sig);

(gdb) backtrace
#0 handler (sig=15) at signal_handler.c:42
#1 <signal handler called>
#2 main () at program.c:100
    # Shows signal interrupted main at line 100
```

### Ftrace/Tracefs: Kernel-Level Signal Tracing

```bash
# Mount tracefs
$ mount -t tracefs none /sys/kernel/debug/tracing

# Enable signal events
$ echo 1 > /sys/kernel/debug/tracing/events/signal/signal_generate/enable
$ echo 1 > /sys/kernel/debug/tracing/events/signal/signal_deliver/enable

# Run program
$ ./myapp

# View trace
$ cat /sys/kernel/debug/tracing/trace

# Output:
#           TASK-PID    CPU#    TIMESTAMP   FUNCTION
#           |    |        |        |           |
            myapp-5678  [001] 1234.567890: signal_generate: sig=15 task=myapp pid=5678 uid=1000
            myapp-5678  [001] 1234.567891: signal_deliver: sig=15 task=myapp pid=5678
```

### Profiling Signals with Perf

```bash
# Monitor signal delivery performance
$ perf record -e 'signal:signal_generate,signal:signal_deliver' ./myapp
$ perf report

# Trace signal overhead per syscall
$ perf trace -e signal_* ./myapp
```

### Custom Signal Tracing (User-Space)

```c
#include <signal.h>
#include <stdio.h>
#include <unistd.h>

// Global to trace signal invocations
unsigned int signal_count = 0;

void traced_handler(int sig, siginfo_t *info, void *context) {
    signal_count++;
    
    // Log signal info
    fprintf(stderr, "Signal %d delivered (count: %u)\n", sig, signal_count);
    fprintf(stderr, "  From PID: %d\n", info->si_pid);
    fprintf(stderr, "  Code: %d\n", info->si_code);
    
    // For real-time signals
    if (sig >= SIGRTMIN && sig <= SIGRTMAX) {
        fprintf(stderr, "  Arg: %d\n", info->si_value.sival_int);
    }
}

int main() {
    struct sigaction sa;
    sa.sa_sigaction = traced_handler;
    sigemptyset(&sa.sa_mask);
    sa.sa_flags = SA_SIGINFO;
    
    sigaction(SIGUSR1, &sa, NULL);
    sigaction(SIGRTMIN, &sa, NULL);
    
    // Generate signals
    pthread_kill(pthread_self(), SIGUSR1);
    
    // Send real-time signal with argument
    union sigval val;
    val.sival_int = 42;
    rt_sigqueueinfo(getpid(), SIGRTMIN, &info);
    
    sleep(1);
    
    printf("Total signals: %u\n", signal_count);
}
```

---

## Performance Considerations

### Signal Handling Overhead

```
LATENCY BREAKDOWN (x86-64, typical):

1. Signal Generation:
   └─ kill() syscall: ~500 ns
      send_signal()
      Set TIF_SIGPENDING
      (synchronous path: immediate)

2. Signal Detection:
   ├─ Exception handler return: check TIF_SIGPENDING (~100 ns)
   ├─ Syscall exit: check TIF_SIGPENDING (~100 ns)
   └─ Interrupt return: check TIF_SIGPENDING (~100 ns)

3. Signal Delivery:
   ├─ do_signal() execution: ~1-5 µs
   │  recalc_sigpending()
   │  next_signal() - find pending signal
   │  check signal mask
   │  lookup handler
   ├─ setup_rt_frame() - stack setup: ~1-2 µs
   │  Copy siginfo_t to user space
   │  Copy ucontext_t to user space
   │  Modify CPU registers
   └─ Total: ~2-7 µs

4. Handler Execution:
   └─ User code: application-dependent (typically 1-100 µs)

5. Return from Handler:
   ├─ rt_sigreturn() syscall: ~500 ns
   ├─ Restore CPU registers: ~100 ns
   └─ Continue user code

TOTAL: ~3-8 µs + handler + syscall overhead
       (If blocking disk I/O in handler: ~ms to sec)


OVERHEAD SOURCES:

Memory copies:
  ├─ siginfo_t (128 bytes)
  ├─ ucontext_t (300+ bytes)
  └─ Copies to/from user space: cache misses
  
CPU cache effects:
  ├─ Signal handler in different cache page
  ├─ User stack modified (new stack area)
  └─ Potential L3 cache miss: +10-50 µs

Syscall overhead:
  ├─ Context switch to kernel mode
  ├─ Return to user mode
  └─ TLB flush (sometimes): +1-5 µs

Preemption:
  ├─ Signal delivery → might wake thread
  ├─ Scheduler might choose different thread
  └─ Context switch: +1-10 µs
```

### Real-Time Signals vs Standard Signals

```c
// PERFORMANCE COMPARISON:

// Standard Signal (1-31): O(1) per delivery
//   ├─ Set one bit in bitmask
//   ├─ Check if blocked
//   └─ Deliver (no allocation)

// Real-Time Signal (32-64): O(1) per delivery
//   ├─ Allocate sigqueue struct (kmem_cache)
//   ├─ Add to list
//   ├─ On delivery, remove from list
//   └─ Free sigqueue struct

// Queue Allocation:
//   ├─ kmem_cache_alloc(): ~100 ns (cached)
//   ├─ kmem_cache_free(): ~100 ns
//   └─ Total overhead: ~200 ns per queued signal

// Memory usage:
//   ├─ Standard signal: 1 bit per signal (8 bytes total for 64 signals)
//   ├─ Real-time queue: ~64 bytes per queued instance
//   └─ If queue 1000 signals: ~64 KB


// OPTIMIZATION: Batch signals

// WRONG (slow):
for (int i = 0; i < 1000000; i++) {
    union sigval val;
    val.sival_int = i;
    rt_sigqueueinfo(worker_pid, SIGRTMIN, &info);
}
// Each syscall: ~1-2 µs = 1-2 seconds total

// RIGHT (fast):
// Send work items to queue (lock-free if possible)
// Send ONE signal with count

void worker_handler(int sig, siginfo_t *info, void *context) {
    int count = info->si_value.sival_int;
    for (int i = 0; i < count; i++) {
        process_item(queue[i]);
    }
}

main() {
    add_items_to_queue(items, 1000000);
    
    union sigval val;
    val.sival_int = 1000000;
    rt_sigqueueinfo(worker_pid, SIGRTMIN, &info);
    // One syscall + one handler = fast
}


// MEASUREMENT: perf shows signal overhead

$ perf stat -e cycles,instructions,cache-misses ./signal_test
  Performance counter stats:
    12,345,678 cycles            # ~5 µs per signal
    8,234,567 instructions
    234 cache-misses
```

### Minimizing Signal Handler Latency

```c
// Rule 1: Keep handlers SHORT

void good_handler(int sig) {
    // Safe: just set flag
    volatile sig_atomic_t *flag = &global_flag;
    *flag = 1;
    
    // Returns immediately
}

void bad_handler(int sig) {
    // NOT SAFE: locks, allocations, I/O
    pthread_mutex_lock(&mutex);  // DEADLOCK RISK
    printf("Got signal\n");       // I/O, slow
    malloc(1024);                 // Unsafe
    usleep(1000000);              // Slow!
}

// Rule 2: Use sig_atomic_t

// Atomic on most platforms (single CPU instruction)
volatile sig_atomic_t flag = 0;

void handler(int sig) {
    flag = 1;  // Atomic write
    // No protection needed
}

// Contrast:
volatile int flag = 0;  // NOT atomic
void handler(int sig) {
    flag++;  // May be compiled to:
             //   load flag
             //   increment
             //   store flag
             // Can be interrupted between ops!
}

// Rule 3: Async-signal-safe functions only

// Safe in signal handler:
write(2, "Error\n", 6);              // write()
_exit(1);                             // _exit()
signal(sig, handler);                 // signal()

// NOT safe:
printf("Error\n");                   // Uses malloc/locks
pthread_mutex_lock(&mutex);          // Might deadlock
malloc(1024);                        // Uses locks
fopen("/tmp/file", "w");             // malloc, locks

// Full list: man 7 signal-safety


// Rule 4: Use signalfd for async handling

#include <sys/signalfd.h>

int main() {
    sigset_t mask;
    sigemptyset(&mask);
    sigaddset(&mask, SIGUSR1);
    
    // Block signal (so it doesn't call handler)
    sigprocmask(SIG_BLOCK, &mask, NULL);
    
    // Create signalfd to read signal events
    int sfd = signalfd(-1, &mask, SFD_CLOEXEC);
    
    // Add to epoll/select/poll for event-driven handling
    epoll_add(epoll_fd, sfd);
    
    while (true) {
        struct signalfd_siginfo fdsi;
        
        // Wait for signal event (like I/O event)
        epoll_wait(epoll_fd, events, MAX_EVENTS, -1);
        
        // Read signal info
        read(sfd, &fdsi, sizeof(fdsi));
        
        // Handle signal (now in main event loop, not handler)
        // Can safely allocate, lock, I/O, etc.
        handle_signal(fdsi.ssi_signo);
    }
}

// Advantage:
// ├─ No signal handler complexity
// ├─ No async-signal-safety restrictions
// ├─ Better integration with event loops
// └─ More testable code
```

---

## Practical Examples

### Example 1: Graceful Shutdown (SIGTERM)

```c
#include <signal.h>
#include <unistd.h>
#include <stdio.h>
#include <stdlib.h>

volatile sig_atomic_t shutdown_requested = 0;

void sigterm_handler(int sig) {
    shutdown_requested = 1;
    // NO printf, malloc, locks here!
}

int main() {
    // Install handler
    struct sigaction sa;
    sigemptyset(&sa.sa_mask);
    sa.sa_handler = sigterm_handler;
    sa.sa_flags = 0;
    sigaction(SIGTERM, &sa, NULL);
    sigaction(SIGINT, &sa, NULL);

    // Ignore SIGPIPE (client disconnect)
    signal(SIGPIPE, SIG_IGN);

    // Main loop
    while (!shutdown_requested) {
        // Do work
        process_requests();
        
        // Check shutdown flag
        if (shutdown_requested) {
            break;
        }
        
        sleep(1);
    }

    // Cleanup
    cleanup_resources();
    printf("Shutdown complete\n");
    return 0;
}

// Usage:
// $ ./daemon &
// $ kill -SIGTERM <pid>          # Graceful shutdown
// $ kill -SIGKILL <pid>          # Force kill if hung
```

### Example 2: Child Process Monitoring (SIGCHLD)

```c
#include <signal.h>
#include <sys/wait.h>
#include <unistd.h>
#include <stdio.h>
#include <stdlib.h>

void sigchld_handler(int sig) {
    // Reap all available children (non-blocking)
    pid_t pid;
    int status;

    while ((pid = waitpid(-1, &status, WNOHANG)) > 0) {
        if (WIFEXITED(status)) {
            fprintf(stderr, "Child %d exited with code %d\n",
                    pid, WEXITSTATUS(status));
        } else if (WIFSIGNALED(status)) {
            fprintf(stderr, "Child %d killed by signal %d\n",
                    pid, WTERMSIG(status));
        }
    }
}

int main() {
    struct sigaction sa;
    sigemptyset(&sa.sa_mask);
    sa.sa_handler = sigchld_handler;
    sa.sa_flags = SA_NOCLDSTOP | SA_NOCLDWAIT;
    sigaction(SIGCHLD, &sa, NULL);

    // Fork workers
    for (int i = 0; i < 4; i++) {
        pid_t pid = fork();
        if (pid == 0) {
            // Child process
            worker_main(i);
            exit(0);
        }
    }

    // Parent: monitor children
    while (1) {
        pause();  // Wait for signal
    }
}
```

### Example 3: Real-Time Signal Queue

```c
#include <signal.h>
#include <unistd.h>
#include <stdio.h>
#include <stdlib.h>

void handler(int sig, siginfo_t *info, void *context) {
    // Safe: only read from info
    printf("Task %d (priority %d)\n",
           info->si_value.sival_int >> 16,
           info->si_value.sival_int & 0xFFFF);
}

int main() {
    // Install handler for real-time signal
    struct sigaction sa;
    sigemptyset(&sa.sa_mask);
    sa.sa_sigaction = handler;
    sa.sa_flags = SA_SIGINFO;
    sigaction(SIGRTMIN, &sa, NULL);

    // Queue work with arguments
    struct {
        int priority;
        int task_id;
    } tasks[] = {{10, 1}, {20, 2}, {15, 3}};

    siginfo_t info;
    for (int i = 0; i < 3; i++) {
        info.si_value.sival_int =
            (tasks[i].task_id << 16) | (tasks[i].priority & 0xFFFF);
        
        rt_sigqueueinfo(getpid(), SIGRTMIN, &info);
    }

    sleep(1);  // Let signals deliver
}

// Output:
// Task 1 (priority 10)
// Task 2 (priority 20)
// Task 3 (priority 15)
```

### Example 4: Timer Signal (SIGALRM)

```c
#include <signal.h>
#include <unistd.h>
#include <stdio.h>

void alarm_handler(int sig) {
    printf("Alarm!\n");
    alarm(5);  // Reset timer (safe to call from handler)
}

int main() {
    signal(SIGALRM, alarm_handler);
    
    // Set timer: 5 seconds
    alarm(5);
    
    // Work
    while (1) {
        printf("Working...\n");
        sleep(2);
    }
}

// Better: setitimer for higher precision

void timer_handler(int sig) {
    printf("Timer!\n");
}

int main() {
    struct sigaction sa;
    sigemptyset(&sa.sa_mask);
    sa.sa_handler = timer_handler;
    sigaction(SIGALRM, &sa, NULL);
    
    struct itimerval timer;
    timer.it_value.tv_sec = 1;     // 1 second
    timer.it_value.tv_usec = 0;
    timer.it_interval.tv_sec = 1;  // Repeat every 1 second
    timer.it_interval.tv_usec = 0;
    
    setitimer(ITIMER_REAL, &timer, NULL);
    
    while (1) {
        sleep(1);
    }
}
```

### Example 5: Profiling with SIGPROF

```c
#include <signal.h>
#include <unistd.h>
#include <stdio.h>
#include <sys/time.h>
#include <string.h>

unsigned int profile_count = 0;

void profile_handler(int sig, siginfo_t *info, void *context) {
    profile_count++;
    
    // In real profiler: would examine ucontext to get call stack
    ucontext_t *ctx = (ucontext_t *) context;
    void *pc = (void *) ctx->uc_mcontext.rip;
    
    // Log program counter (simplified)
    // In real code: use dl_iterate_phdr or similar to get symbol name
}

int main() {
    struct sigaction sa;
    sigemptyset(&sa.sa_mask);
    sa.sa_sigaction = profile_handler;
    sa.sa_flags = SA_SIGINFO;
    sigaction(SIGPROF, &sa, NULL);

    struct itimerval timer;
    // 10ms interval
    timer.it_value.tv_sec = 0;
    timer.it_value.tv_usec = 10000;
    timer.it_interval.tv_sec = 0;
    timer.it_interval.tv_usec = 10000;

    setitimer(ITIMER_PROF, &timer, NULL);

    // Do work
    for (int i = 0; i < 1000000000; i++) {
        volatile int x = i * 2;  // Busy loop
    }

    timer.it_value.tv_sec = 0;
    timer.it_value.tv_usec = 0;
    setitimer(ITIMER_PROF, &timer, NULL);

    printf("Samples collected: %u\n", profile_count);
}
```

### Example 6: Rust Signal Handler

```rust
// Using nix crate for signal handling

use nix::sys::signal::{signal, SigHandler, SIGTERM, SIGINT};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;

static SHUTDOWN: AtomicBool = AtomicBool::new(false);

fn setup_signal_handlers() -> nix::Result<()> {
    extern "C" fn handler(_: libc::c_int) {
        SHUTDOWN.store(true, Ordering::Relaxed);
    }

    unsafe {
        signal(SIGTERM, SigHandler::Handler(handler))?;
        signal(SIGINT, SigHandler::Handler(handler))?;
    }

    Ok(())
}

fn main() -> nix::Result<()> {
    setup_signal_handlers()?;

    while !SHUTDOWN.load(Ordering::Relaxed) {
        // Do work
        println!("Working...");
        std::thread::sleep(std::time::Duration::from_secs(1));
    }

    println!("Shutting down...");
    Ok(())
}


// Using signalfd for event-driven approach

use nix::sys::signalfd::{signalfd, SigSet};
use nix::sys::epoll::{epoll_create1, epoll_ctl, epoll_wait, EpollEvent, EpollOp, EpollFlags};
use nix::sys::signal::Signal;

fn main() {
    // Block signals (so they don't call handler)
    let mut mask = SigSet::empty();
    mask.add(Signal::SIGTERM);
    mask.add(Signal::SIGINT);
    
    nix::sys::signal::pthread_sigmask(
        nix::sys::signal::SigmaskHow::SIG_BLOCK,
        Some(&mask),
        None,
    ).unwrap();

    // Create signalfd
    let sfd = signalfd(-1, &mask, nix::sys::signalfd::SfdFlags::SFD_CLOEXEC).unwrap();

    // Create epoll
    let epfd = epoll_create1(nix::sys::epoll::EpollCreateFlags::empty()).unwrap();
    let event = EpollEvent::new(EpollFlags::EPOLLIN, sfd as u64);
    epoll_ctl(epfd, EpollOp::EpollCtlAdd, sfd, &mut event.clone()).unwrap();

    // Event loop
    let mut shutdown = false;
    while !shutdown {
        let mut events = vec![EpollEvent::empty(); 1];
        let n = epoll_wait(epfd, &mut events, -1).unwrap();

        for event in &events[..n] {
            if event.data() == sfd as u64 {
                // Read signal info
                let mut buf = [0u8; 8];
                let _ = nix::unistd::read(sfd, &mut buf);
                
                println!("Signal received");
                shutdown = true;
            }
        }
    }

    println!("Shutdown complete");
}
```

---

## Summary & Mental Model

### Key Concepts to Remember

```
1. SIGNAL DELIVERY FLOW:
   Generation → Pending → Delivery → Handler → Return
   
   Blocked signals go to pending, delivered when unblocked
   Synchronous signals delivered immediately
   Asynchronous signals delivered at safe points (syscall exit)

2. STANDARD vs REAL-TIME:
   Standard (1-31):   One bit, no queueing, lost if already pending
   Real-Time (32-64): Queue structure, FIFO delivery, arguments

3. KERNEL STRUCTURES:
   sighand_struct    → Signal handlers (shared by threads)
   signal_struct     → Process-wide signal state
   task_struct       → Per-thread pending signals & mask
   sigpending        → Queue of pending signals

4. CRITICAL SECTIONS:
   Block signals + atomically sleep = sigsuspend()
   Modify handlers = sigaction() under spinlock
   Send signals = __send_signal() under spinlock

5. ASYNCHRONOUS SAFETY:
   Only sig_atomic_t assignments safe
   No malloc, printf, locks, etc.
   Use signalfd for better patterns

6. REAL-WORLD PATTERNS:
   Graceful shutdown → SIGTERM handler sets flag → main loop exits
   Child monitoring → SIGCHLD handler reaps zombies
   Timers → SIGALRM for simple, setitimer for complex
   Profiling → SIGPROF with context inspection
   Threading → signalfd + event loop, not handlers
```

### Architecture in ASCII

```
COMPLETE SIGNAL ARCHITECTURE:

┌─────────────────────────────────────────────────────────────┐
│ USER SPACE                                                  │
│                                                             │
│ ┌────────────────────────────────────────────────────────┐ │
│ │ Application Code                                       │ │
│ │ - Normal execution                                    │ │
│ │ - Syscalls (read, write, etc.)                        │ │
│ │ - Library code                                        │ │
│ └───────────────────┬────────────────────────────────────┘ │
│                     │                                       │
│                     ↓ (trap/interrupt/exception)           │
└─────────────────────┼───────────────────────────────────────┘
                      │
         ┌────────────┴────────────┐
         │                         │
    Syscall         Exception   Interrupt
    (write, etc.)   (SIGSEGV)   (timer, etc.)
         │              │           │
         ↓              ↓           ↓
┌─────────────────────────────────────────────────────────────┐
│ KERNEL MODE                                                 │
│                                                             │
│ ┌────────────────────────────────────────────────────────┐ │
│ │ 1. Exception/Syscall/Interrupt Handler                │ │
│ │    - Save registers (pt_regs)                         │ │
│ │    - Do kernel work                                   │ │
│ │    - Return from exception                            │ │
│ └────────────────────┬─────────────────────────────────┘ │
│                      │                                    │
│ ┌────────────────────↓─────────────────────────────────┐ │
│ │ 2. Check TIF_SIGPENDING                              │ │
│ │    if (test_thread_flag(TIF_SIGPENDING)) {          │ │
│ │        do_signal(regs)                               │ │
│ │    }                                                  │ │
│ └────────────────────┬─────────────────────────────────┘ │
│                      │                                    │
│ ┌────────────────────↓─────────────────────────────────┐ │
│ │ 3. do_signal()                                        │ │
│ │    for each pending signal:                          │ │
│ │      - Check if blocked                              │ │
│ │      - Get handler from sighand_struct               │ │
│ │      - Call setup_rt_frame()                         │ │
│ │      - Modify pt_regs                                │ │
│ └────────────────────┬─────────────────────────────────┘ │
│                      │                                    │
│ ┌────────────────────↓─────────────────────────────────┐ │
│ │ 4. Return to user mode                               │ │
│ │    (pt_regs modified to jump to handler)             │ │
│ └────────────────────┬─────────────────────────────────┘ │
│                      │                                    │
└──────────────────────┼────────────────────────────────────┘
                       │
┌──────────────────────↓────────────────────────────────────┐
│ USER SPACE (Handler Execution)                           │
│                                                          │
│ ┌────────────────────────────────────────────────────┐ │
│ │ Signal Handler Runs                                │ │
│ │ handler(sig, &info, &context)                     │ │
│ │ Can inspect:                                       │ │
│ │ - info->si_signo (signal number)                  │ │
│ │ - info->si_code (reason)                          │ │
│ │ - info->si_pid (sender PID)                       │ │
│ │ - info->si_value (argument for RT signals)        │ │
│ │ - context→uc_mcontext (saved registers)           │ │
│ │                                                   │ │
│ │ Can do:                                            │ │
│ │ - Read from memory                                │ │
│ │ - sig_atomic_t writes                             │ │
│ │ - Call async-signal-safe functions               │ │
│ │                                                   │ │
│ │ Cannot do:                                         │ │
│ │ - malloc, free                                    │ │
│ │ - printf (uses malloc)                            │ │
│ │ - pthread_mutex_lock (deadlock)                   │ │
│ │ - Complex logic (might be interrupted)            │ │
│ └───────────────┬──────────────────────────────────┘ │
│                 │                                     │
│                 ↓ (return from handler)               │
│                                                      │
│ ┌────────────────────────────────────────────────┐ │
│ │ Signal Return Sequence:                        │ │
│ │ 1. Handler return address jumps to __restore_rt│ │
│ │ 2. __restore_rt calls rt_sigreturn syscall    │ │
│ │ 3. Kernel restores registers from frame       │ │
│ │ 4. Kernel restores signal mask                │ │
│ │ 5. Return to original user code               │ │
│ └────────────────────────────────────────────────┘ │
│                 │                                   │
│                 ↓ (continue as if not interrupted)  │
│                                                     │
│ ┌────────────────────────────────────────────────┐ │
│ │ Application Continues                          │ │
│ └────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────┘


DATA STRUCTURE RELATIONSHIPS:

process_group/session (multiple processes)
         │
         ↓ (NPTL: clone)
┌────────────────────────────────────┐
│ Process (PID)                      │
├────────────────────────────────────┤
│ struct signal_struct               │
│ ├─ shared_pending (for any thread) │
│ ├─ posix_timers                   │
│ └─ tty (controlling terminal)      │
│                                    │
│ struct sighand_struct (shared)     │
│ ├─ spinlock siglock                │
│ └─ action[64] (signal handlers)    │
│                                    │
│ ┌────────────────────────────────┐ │
│ │ Thread 1 (TID)                 │ │
│ ├────────────────────────────────┤ │
│ │ struct task_struct             │ │
│ │ ├─ pending (thread-specific)   │ │
│ │ ├─ blocked (signal mask)       │ │
│ │ ├─ sighand → (shared above)    │ │
│ │ └─ signal → (shared above)     │ │
│ └────────────────────────────────┘ │
│                                    │
│ ┌────────────────────────────────┐ │
│ │ Thread 2 (TID)                 │ │
│ ├────────────────────────────────┤ │
│ │ struct task_struct             │ │
│ │ ├─ pending (thread-specific)   │ │
│ │ ├─ blocked (signal mask)       │ │
│ │ ├─ sighand → (shared above)    │ │
│ │ └─ signal → (shared above)     │ │
│ └────────────────────────────────┘ │
└────────────────────────────────────┘
```

This guide covers the complete signal system from first principles through kernel internals. You now have the mental model to understand signal delivery, design robust signal-based systems, and debug signal-related issues efficiently.

