# SIGKILL vs SIGTERM: Complete In-Depth Guide

## Table of Contents

1. [Signal Fundamentals](#signal-fundamentals)
2. [SIGTERM vs SIGKILL: Core Differences](#sigterm-vs-sigkill-core-differences)
3. [Linux Signal Architecture](#linux-signal-architecture)
4. [Process State Transitions](#process-state-transitions)
5. [Signal Delivery Mechanisms](#signal-delivery-mechanisms)
6. [Signal Masking and Handling](#signal-masking-and-handling)
7. [C Implementation Guide](#c-implementation-guide)
8. [Rust Implementation Guide](#rust-implementation-guide)
9. [Linux Kernel Deep Dive](#linux-kernel-deep-dive)
10. [Real-World Scenarios](#real-world-scenarios)
11. [Best Practices and Patterns](#best-practices-and-patterns)
12. [Debugging and Troubleshooting](#debugging-and-troubleshooting)

---

## 1. Signal Fundamentals

### What Are Signals?

Signals are software interrupts that provide a way to handle asynchronous events in Linux/Unix systems. They are one of the oldest inter-process communication (IPC) mechanisms, predating modern alternatives like pipes, message queues, and sockets.

Key characteristics:

- **Asynchronous**: Delivered at unpredictable times to interrupt process execution
- **Limited data**: Carry only a signal number (and optionally siginfo with additional context)
- **Software interrupts**: Cause CPU to transfer control to signal handler
- **Reliable delivery**: Standard signals may be lost; real-time signals are guaranteed
- **Per-process**: Each process has its own signal dispositions and masks

### Signal Numbers and Ranges

```
Standard Signals:       1-31   (POSIX.1-1990 standard)
Real-Time Signals:     34-64  (POSIX.1b real-time signals)
Reserved by kernel:   32-33, 64+ (architecture dependent)
```

### Complete Signal Listing

```c
Signal  Name          Default Action  Description
------  ----          ---------------  -----------
1       SIGHUP        Terminate        Hangup on controlling terminal
2       SIGINT        Terminate        Interrupt from keyboard (Ctrl-C)
3       SIGQUIT       Core dump        Quit from keyboard (Ctrl-\)
4       SIGILL        Core dump        Illegal instruction
5       SIGTRAP       Core dump        Trace/breakpoint trap
6       SIGABRT       Core dump        Abort signal
7       SIGBUS        Core dump        Bus error
8       SIGFPE        Core dump        Floating point exception
9       SIGKILL       Terminate        Kill signal (CANNOT BE CAUGHT/BLOCKED)
10      SIGUSR1       Terminate        User-defined signal 1
11      SIGSEGV       Core dump        Segmentation violation
12      SIGUSR2       Terminate        User-defined signal 2
13      SIGPIPE       Terminate        Broken pipe
14      SIGALRM       Terminate        Timer signal from alarm()
15      SIGTERM       Terminate        Termination signal
17      SIGCHLD       Ignore           Child process status changed
18      SIGCONT       Continue         Continue if stopped
19      SIGSTOP       Stop             Stop process (CANNOT BE CAUGHT/BLOCKED)
20      SIGTSTP       Stop             Stop typed at terminal (Ctrl-Z)
21      SIGTTIN       Stop             Background read from terminal
22      SIGTTOU       Stop             Background write to terminal
23      SIGURG        Ignore           Urgent condition on socket
24      SIGXCPU       Core dump        CPU time limit exceeded
25      SIGXFSZ       Core dump        File size limit exceeded
26      SIGVTALRM     Terminate        Virtual alarm clock
27      SIGPROF       Terminate        Profiling timer expired
28      SIGWINCH      Ignore           Window size change
29      SIGIO         Terminate        I/O now possible
30      SIGPWR        Terminate        Power failure
31      SIGSYS        Core dump        Bad system call
```

---

## 2. SIGTERM vs SIGKILL: Core Differences

### Quick Comparison Table

```
Feature                 SIGTERM               SIGKILL
================================================================================
Signal Number           15                    9
POSIX Standard         Yes (POSIX.1-1990)   Yes (POSIX.1-1990)
Default Action         Terminate             Terminate
Can be Caught/Handled  YES ✓                 NO ✗
Can be Blocked/Masked  YES ✓                 NO ✗
Graceful Shutdown      YES ✓ (designed for)  NO ✗ (forceful)
Cleanup Possible       YES ✓                 NO ✗
Child Processes        Inherited handler     Inherited, but can't catch
Use Case               Normal termination    Last resort / zombie cleanup
Handler Availability   Unlimited             N/A
Interrupt Safe         Depends on handler    N/A
```

### SIGTERM - The Polite Request

**Signal Number**: 15

SIGTERM is the standard termination signal. It's the polite way to ask a process to shut down.

**Characteristics**:
- Can be caught with a signal handler
- Can be blocked temporarily using signal masks
- Application receives notification and can perform cleanup
- Process has opportunity to:
  - Close files and connections
  - Flush buffers
  - Save state
  - Kill child processes
  - Release locks
  - Log shutdown information

**When used**:
- `kill <pid>` (default signal)
- `systemctl stop service`
- Docker container graceful shutdown (default)
- Application termination sequences
- Service manager orchestration

**Example**:
```bash
$ kill 1234              # Sends SIGTERM (default)
$ kill -15 1234          # Explicitly SIGTERM
$ kill -TERM 1234        # Using signal name
```

### SIGKILL - The Nuclear Option

**Signal Number**: 9

SIGKILL is the ultimate termination signal. It cannot be caught, blocked, or ignored.

**Characteristics**:
- Cannot be caught (no signal handler)
- Cannot be blocked by sigprocmask()
- Cannot be ignored
- Process is terminated immediately by kernel
- No cleanup code runs
- Bypasses application logic entirely
- Resources may be leaked
- File descriptors may remain open
- Locks may remain held

**When used**:
- Last resort termination
- Zombie process cleanup
- Unresponsive applications
- `kill -9 <pid>`
- Forced container termination after timeout
- Kernel cleanup during shutdown

**Example**:
```bash
$ kill -9 1234           # Force kill
$ kill -KILL 1234        # Using signal name
$ killall -9 process     # Kill all instances
```

### The Recommended Termination Pattern

```
Stage 1: Send SIGTERM
    ↓
    Wait 5-10 seconds for graceful shutdown
    ↓
    Process terminated? → SUCCESS ✓
    Process still running? → Continue to Stage 2
    ↓
Stage 2: Send SIGKILL
    ↓
    Process killed immediately
    ↓
    SUCCESS ✓ (but with potential resource leaks)
```

---

## 3. Linux Signal Architecture

### High-Level Signal Flow Architecture

```
User Space                          Kernel Space
═════════════════════════════════════════════════════════════════════════

┌─────────────────────────────────────────────────────────────────────┐
│  Process A (Sender)                                                 │
│  ┌──────────────┐                                                   │
│  │ Application  │                                                   │
│  │              │                                                   │
│  │ kill(B, 15) │──────────────────┐                               │
│  └──────────────┘                   │                               │
└─────────────────────────────────────────────────────────────────────┘
                                      │
                                      │ syscall (kill)
                                      │
                                      ▼
                                ┌───────────────────┐
                                │  System Call      │
                                │  Entry Point      │
                                │  (sys_kill)       │
                                └───────────────────┘
                                      │
                                      ▼
                                ┌───────────────────────────────────┐
                                │  Signal Delivery Kernel Code:     │
                                │                                   │
                                │  1. Validate PID                  │
                                │  2. Check permissions             │
                                │  3. Lookup target task_struct     │
                                │  4. Check signal masks            │
                                │  5. Add to pending queue          │
                                │  6. Mark task runnable            │
                                └───────────────────────────────────┘
                                      │
                                      ▼
                                ┌───────────────────────────────────┐
                                │  Kernel Signal Queue              │
                                │                                   │
                                │  pending.signal (bitmap)          │
                                │  pending.list (sigqueued)         │
                                │  sigaction[15] (handler info)     │
                                └───────────────────────────────────┘
                                      │
                                      │ (Process may be in various states)
                                      │
                        ┌─────────────┼─────────────┐
                        │             │             │
                        ▼             ▼             ▼
                   Running       Blocked/Sleep   Stopped
                   (immediate)   (on wakeup)     (needs SIGCONT)
                        │             │             │
                        └─────────────┴─────────────┘
                                      │
                                      ▼
                        ┌─────────────────────────────┐
                        │ Signal Pending Check Points:│
                        │                             │
                        │ • Return from syscall       │
                        │ • Return to userspace       │
                        │ • Schedule decision point   │
                        │ • Interrupt handler exit    │
                        └─────────────────────────────┘
                                      │
                                      ▼
                        ┌─────────────────────────────────────────┐
                        │  Determine Signal Delivery:             │
                        │                                         │
                        │  Is signal masked? → Queue it          │
                        │  Has handler?    → Setup stack frame   │
                        │  Default action? → Apply default       │
                        └─────────────────────────────────────────┘
                                      │
                                      ▼ (Setup user mode stack frame)
┌─────────────────────────────────────────────────────────────────────┐
│  Process B (Receiver)                                               │
│  ┌──────────────────────────────────────────────────────────┐       │
│  │  User Space Code                                         │       │
│  │                                                          │       │
│  │  Normal Execution ── Interrupted by Signal ──┐          │       │
│  │                                               │          │       │
│  │  Processor                                    ▼          │       │
│  │  Execution  ┌────────────────────────────────────────┐  │       │
│  │  registers  │ Jump to Signal Handler (trampoline)    │  │       │
│  │  saved      │                                        │  │       │
│  │  on stack   │ sigaction.sa_handler(sig, info, ctx)  │  │       │
│  │             │                                        │  │       │
│  │             │ Handler executes                       │  │       │
│  │             │                                        │  │       │
│  │             │ return; (or longjmp)                   │  │       │
│  │             └────────────────────────────────────────┘  │       │
│  │                                                ^         │       │
│  │  Resume ◄─ Restore saved registers ◄─────────┘         │       │
│  │  Normal                                                 │       │
│  │  Execution                                              │       │
│  └──────────────────────────────────────────────────────────┘       │
└─────────────────────────────────────────────────────────────────────┘
```

### Signal Handling Decision Tree

```
Signal Arrives at Process
         │
         ▼
    ┌─────────────────┐
    │ Is signal       │
    │ SIGKILL or      │ YES → Terminate process immediately
    │ SIGSTOP?        │       (No handler, no mask check)
    └─────────────────┘
         │ NO
         ▼
    ┌─────────────────────────┐
    │ Is signal blocked by    │
    │ process's signal mask?  │
    └─────────────────────────┘
         │ YES                │ NO
         │                    │
         ▼                    ▼
    Queue Signal      ┌────────────────────────┐
    for later         │ Has signal handler?    │
    delivery          │ (sigaction registered) │
         │            └────────────────────────┘
         │                 │ YES    │ NO
         │                 │        │
         │                 ▼        ▼
         │            Execute   Apply
         │            Handler   Default
         │               │      Action
         │               ▼        │
         │            Done       ▼
         │               │    (Terminate,
         │               │     Core dump,
         │               │     Stop, Ignore)
         │               │
         └───────────────┘
                 │
                 ▼
            Signal Delivery
            Complete
```

### Linux Kernel task_struct Signal Fields

```c
struct task_struct {
    // ...
    
    /* Signal handlers and blocked signals */
    struct signal_struct *signal;
    struct sighand_struct *sighand;
    
    sigset_t blocked;                    /* Blocked signal mask */
    sigset_t real_blocked;               /* Real-time blocked mask */
    
    unsigned long sas_ss_sp;             /* Alt signal stack addr */
    size_t sas_ss_size;                  /* Alt signal stack size */
    
    /* Signal pending */
    struct {
        struct list_head list;
        struct bitmap *signal;           /* Bitmap of pending signals */
    } pending;
    
    unsigned long signal;                /* Bitmask of pending signals */
    unsigned long blocked;               /* Bitmask of blocked signals */
    
    struct sigpending pending;           /* Pending signals queue */
    
    // ... other fields
};

struct signal_struct {
    atomic_t refcount;
    int nr_threads;                      /* Number of threads */
    wait_queue_head_t wait_chldexit;     /* For wait4() */
    
    int group_exit_code;                 /* Code for SIGTERM group exit */
    int notify_count;
    
    struct list_head posix_timers;       /* POSIX.1b interval timers */
    
    /* Real-time signals queued */
    struct sigqueue *first;
    struct sigqueue *last;
};

struct sighand_struct {
    atomic_t count;
    struct k_sigaction action[_NSIG];    /* Signal handlers array */
    spinlock_t siglock;                  /* Protects the above */
};

struct k_sigaction {
    struct sigaction sa;
    struct __user *ka_restorer;
};

struct sigpending {
    struct list_head list;
    sigset_t signal;
};
```

---

## 4. Process State Transitions

### Process Lifecycle with Signals

```
Process States:
═════════════════════════════════════════════════════════════════════

┌─────────────┐
│   CREATED   │  (fork() returns, not yet scheduled)
└──────┬──────┘
       │
       ▼
┌──────────────────┐
│   RUNNABLE       │  (Ready to run, waiting for CPU time)
│                  │  • In run queue
│                  │  • Can receive most signals
│                  │  • SIGKILL terminates immediately
│                  │  • SIGTERM queued if masked
└──┬─────────┬─────┘
   │         │
   │ (CPU    │ (I/O, sleep)
   │ slice   │
   │ ends)   │
   ▼         ▼
┌────────┐  ┌──────────────────┐
│RUNNING │  │ BLOCKED/SLEEPING │
│        │  │                  │
│ Signal │  │ • Waiting for:   │
│handled │  │   - I/O device   │
│here    │  │   - Lock         │
│if      │  │   - Timer        │
│running │  │   - Condition    │
└───┬────┘  │                  │
    │       │ • Signal arrives:│
    └───┬───┤   - If interruptible
        │   │     wake up
        │   │   - Check pending
        │   │     signals
        │   └──────────┬───────┘
        │              │
        ▼              ▼
    ┌──────────────────┐
    │ SIGNAL DELIVERY  │
    │                  │
    │ • Unblock signal │
    │ • Setup handler  │
    │ • Jump to frame  │
    └────────┬─────────┘
             │
             ▼
    ┌──────────────────┐
    │  STOPPED STATE   │  (SIGSTOP/SIGTSTP received)
    │                  │
    │  • Process       │
    │    suspended     │
    │  • Cannot        │
    │    execute code  │
    │  • Waiting for   │
    │    SIGCONT       │
    └────────┬─────────┘
             │ (SIGCONT signal)
             ▼
    ┌──────────────────┐
    │  CONTINUED       │
    │  (Resume from    │
    │   stopped state) │
    └────────┬─────────┘
             │
             ▼
    ┌──────────────────┐
    │ Back to RUNNABLE │
    └──────────────────┘

Special Case: SIGKILL
═════════════════════════════════════════════════════════════════════

ANY STATE ──── SIGKILL ──────► IMMEDIATE TERMINATION
(except zombie)                
    • No handler
    • No mask check
    • Synchronous action
    • Force cleanup by kernel
    
Special Case: SIGTERM with Default Handler
═════════════════════════════════════════════════════════════════════

RUNNING ──── SIGTERM ──► Signal handler execution ──► Resume

BLOCKED ──── SIGTERM ──► Queue signal ──► Wakeup ──► Check pending ──► 
             (if not        at interruptible             Signal deliver
              masked)       point
```

### Signal Delivery Timing Points

```
Kernel Code Flow and Signal Check Points:

User Program
    │
    └─► syscall() entry point
        │
        ▼
        ┌─────────────────────────────────┐
        │ Kernel Mode (In syscall)        │
        │                                 │
        │ Can check signals but does      │
        │ not deliver (syscall completes) │
        └────────────────────┬────────────┘
                             │
                             ▼
                    ┌────────────────────┐
                    │ Prepare to exit    │
                    │ syscall            │
                    │                    │
                    │ Return errno?      │
                    │ (interrupted?)     │
                    └─────────┬──────────┘
                              │
                              ▼
                    ┌────────────────────────────┐
                    │ SIGNAL CHECK POINT #1      │
                    │                            │
                    │ Check pending signals      │
                    │ Check signal masks         │
                    │ Queue unblocked signals    │
                    └─────────┬──────────────────┘
                              │
                              ▼
                    ┌────────────────────────────┐
                    │ Return to User Space       │
                    │                            │
                    │ Modify registers:          │
                    │ • Stack pointer            │
                    │ • Return address           │
                    │ • Return value             │
                    └─────────┬──────────────────┘
                              │
                              ▼
                    ┌────────────────────────────┐
                    │ SIGNAL CHECK POINT #2      │
                    │ (Signal Trampoline)        │
                    │                            │
                    │ Before first instruction   │
                    │ in user space:             │
                    │ • Check pending signals    │
                    │ • Setup signal frame       │
                    │ • Jump to handler or code  │
                    └─────────┬──────────────────┘
                              │
                ┌─────────────┴──────────────┐
                │                            │
                ▼                            ▼
            Signal Handler          Normal Code Execution
            Registered?                    │
                │                          │
                ▼                          ▼
            Handler Execution         ┌─────────┐
                │                     │ Running │
                │                     │ code    │
                ▼                     │ (next   │
            Resume Interrupted        │  signal │
            Code                      │  check  │
                │                     │  on     │
                └──────┬──────────────┤  exit)  │
                       │              └─────────┘
                       ▼
                  Next Instruction
                  (Or Handler Return)
```

---

## 5. Signal Delivery Mechanisms

### Sending Signals: System Call Interface

```c
/* Kill syscall family */
int kill(pid_t pid, int sig);
int killpg(pid_t pgrp, int sig);
int tgkill(pid_t tgid, pid_t tid, int sig);  /* Thread-group kill */
int rt_sigqueueinfo(pid_t pid, int sig, 
                    siginfo_t *info);  /* Queue with info */
```

### Signal Delivery from Kernel

```
Kernel Signal Delivery Process (Simplified):
═══════════════════════════════════════════════════════════════════

do_signal_pending()
    │
    ├─► Check signal queue
    │   └─ Get next pending signal
    │
    ├─► Check signal_struct and task masks
    │
    ├─► Lookup signal handler
    │   ├─ SIG_DFL (default action)
    │   ├─ SIG_IGN (ignore)
    │   └─ Custom handler (user function)
    │
    ├─► If handler registered:
    │   │
    │   ├─► Disable signal in mask
    │   │   (prevent recursion)
    │   │
    │   ├─► Build signal frame on stack
    │   │   ├─ Saved registers
    │   │   ├─ Signal context
    │   │   ├─ Signal info
    │   │   └─ Return address (trampoline)
    │   │
    │   ├─► Modify task registers
    │   │   ├─ RSP (stack pointer)
    │   │   ├─ RIP (instruction pointer)
    │   │   └─ RDI (first argument - signal number)
    │   │
    │   └─► Return from syscall to handler code
    │
    └─► If default action:
        ├─ SIGKILL/SIGTERM → exit_group()
        ├─ SIGSTOP → set_task_state(TASK_STOPPED)
        ├─ SIGCONT → set_task_state(TASK_RUNNING)
        └─ (etc.)
```

### Real-Time vs Standard Signal Delivery

```
Standard Signals (1-31):
═════════════════════════════════════════════════════════════════

Queuing Behavior:
    • Multiple instances of same signal coalesce into one
    • Only one handler invocation per signal type
    
Delivery Order:
    • Undefined (implementation-dependent)
    • Low signal numbers may have priority
    
Storage:
    • Single bit in signal bitmask
    • No info preservation beyond signal number

Example:
    Process receives SIGTERM 5 times before handling
    → Signal delivered once (5 instances lost)
    
    sigpending.signal = 0b0...0100000000000000000000000000
                                    ↑ bit 15 (SIGTERM)

Real-Time Signals (34-64):
═════════════════════════════════════════════════════════════════

Queuing Behavior:
    • Each signal instance queued separately
    • All instances delivered
    • FIFO ordering (oldest first)
    
Delivery Order:
    • Strictly defined: by signal number, then FIFO
    
Storage:
    • Sigqueue structure per instance
    • siginfo_t data preserved
    • Custom data can be attached

Example:
    Process receives SIGRTMIN+5 five times
    → Five separate handler invocations
    
    sigpending.list → [sigqueue #1]
                   → [sigqueue #2]
                   → [sigqueue #3]
                   → [sigqueue #4]
                   → [sigqueue #5]
```

---

## 6. Signal Masking and Handling

### Signal Mask Operations

```c
/* Signal mask manipulation */
int sigprocmask(int how, const sigset_t *set, sigset_t *oldset);
int pthread_sigmask(int how, const sigset_t *set, sigset_t *oldset);
int rt_sigprocmask(int how, const sigset_t *set, sigset_t *oldset, size_t sigsetsize);

/* Parameters for 'how' */
SIG_BLOCK      /* Add signals to current mask */
SIG_UNBLOCK    /* Remove signals from current mask */
SIG_SETMASK    /* Replace entire signal mask */

/* Signal set operations */
int sigemptyset(sigset_t *set);           /* Empty set */
int sigfillset(sigset_t *set);            /* All signals */
int sigaddset(sigset_t *set, int signum); /* Add signal */
int sigdelset(sigset_t *set, int signum); /* Remove signal */
int sigismember(const sigset_t *set, int signum); /* Test signal */
```

### Signal Masking Architecture

```
Task Signal Mask Enforcement:
═════════════════════════════════════════════════════════════════

Signal arrives (e.g., SIGTERM=15)
         │
         ▼
    Check if signal is in task->blocked mask
    
    blocked = { bit pattern representing masked signals }
    
    Example mask:
    SIGTERM (15) blocked:
    
    Bit:     31 30 29 ... 16 15 14 ... 2 1 0
    Signal:  (high)         TRM          (low)
    Mask:     0  0  0 ...  0  1  0 ... 0 0 0
             
             SIGTERM is blocked (bit 15 = 1)
             
         │
         ├─ YES (Masked) ─► Queue in pending → Return
         │                  (deliver when unblocked)
         │
         ▼
         NO (Not Masked)
         │
         ▼
    Proceed with signal delivery
    ├─ Check handler
    ├─ Setup frame
    └─ Jump to handler


Signal Mask Interaction with Handlers:
═════════════════════════════════════════════════════════════════

struct sigaction {
    void (*sa_handler)(int);           /* Or SIG_DFL, SIG_IGN */
    void (*sa_sigaction)(int, siginfo_t *, void *);
    sigset_t sa_mask;                  /* Mask while handler runs */
    int sa_flags;                      /* Flags */
};

When signal handler invoked:

1. Current mask saved
2. Handler's sa_mask merged with current blocked signals
3. Handler executes (additional signals blocked)
4. Handler returns
5. Original mask restored

Example:
    Handler registered for SIGUSR1 (10)
    sa_mask contains SIGTERM (15) and SIGUSR2 (12)
    
    While SIGUSR1 handler running:
    ├─ SIGUSR1 (10) blocked (by default during own handler)
    ├─ SIGTERM (15) blocked (per sa_mask)
    ├─ SIGUSR2 (12) blocked (per sa_mask)
    └─ Other signals may be delivered
    
    If SIGTERM arrives while in SIGUSR1 handler:
    → Queued in pending signals
    → Delivered after SIGUSR1 handler returns
    → Original mask restored before SIGTERM handler
```

### Atomic Signal Handling

```c
/* Safe mask modification while handling signals */
int sigprocmask(int how, const sigset_t *set, sigset_t *oldset);

/* Temporary mask change and wait for signal */
int sigsuspend(const sigset_t *mask);

/* Register signal handler */
int sigaction(int signum, const struct sigaction *act,
              struct sigaction *oldact);
int signal(int signum, sighandler_t handler);  /* Deprecated */
```

### Deferred Signal Delivery

```
Scenario: Signal arrives while blocked

Timeline:
═════════════════════════════════════════════════════════════════

T0  Process executes normal code
    └─ signal mask = { SIGTERM blocked }

T1  SIGTERM sent to process
    └─ Check mask
       └─ SIGTERM in mask → QUEUED, not delivered

T2  Process calls sigprocmask(SIG_UNBLOCK, {SIGTERM})
    └─ SIGTERM removed from blocked mask
       └─ Pending signal check
          └─ SIGTERM found in queue
             └─ DELIVERED IMMEDIATELY
                └─ Handler runs
                   └─ Returns
                      └─ Execution resumes

T3  Normal code resumes

Memory representation:
═════════════════════════════════════════════════════════════════

task->blocked (64-bit sigset_t, example)
┌─────────────────────────────────────┐
│ 0b0...01000000000000000000000000000 │
│         ↑
│         SIGTERM (bit 15)
└─────────────────────────────────────┘

task->pending.signal (pending bitmap)
┌─────────────────────────────────────┐
│ 0b0...01000000000000000000000000000 │
│         ↑
│         SIGTERM (bit 15) - queued
└─────────────────────────────────────┘

After unblock:
blocked = 0b0...00000000000000000000000000000 (SIGTERM unblocked)
pending = 0b0...01000000000000000000000000000 (still pending)

→ Delivery logic: if (pending & ~blocked) { deliver }
                  if (0b0...01 & 0b0...10) = true
                  → DELIVER
```

---

## 7. C Implementation Guide

### Basic Signal Handler Setup

```c
#include <stdio.h>
#include <stdlib.h>
#include <signal.h>
#include <unistd.h>
#include <string.h>
#include <errno.h>

/* Global flag for graceful shutdown */
volatile sig_atomic_t should_exit = 0;

/* Simple signal handler for SIGTERM */
void sigterm_handler(int sig) {
    /* IMPORTANT: Only async-signal-safe functions here */
    should_exit = 1;
}

int main(void) {
    struct sigaction sa;
    
    /* Zero out the struct */
    memset(&sa, 0, sizeof(struct sigaction));
    
    /* Set handler function */
    sa.sa_handler = sigterm_handler;
    
    /* Empty mask (no additional signals to block) */
    sigemptyset(&sa.sa_mask);
    
    /* No special flags */
    sa.sa_flags = 0;
    
    /* Register handler for SIGTERM (15) */
    if (sigaction(SIGTERM, &sa, NULL) == -1) {
        perror("sigaction");
        exit(EXIT_FAILURE);
    }
    
    /* Main loop */
    printf("Process running (PID: %d). Send SIGTERM to exit gracefully.\n", 
           getpid());
    
    while (!should_exit) {
        /* Do work */
        printf("Working...\n");
        sleep(1);
    }
    
    printf("Received SIGTERM, shutting down gracefully\n");
    
    /* Cleanup */
    return EXIT_SUCCESS;
}
```

### Advanced Handler with Signal Info

```c
#include <stdio.h>
#include <stdlib.h>
#include <signal.h>
#include <unistd.h>
#include <string.h>
#include <sys/types.h>
#include <pwd.h>

volatile sig_atomic_t shutdown_requested = 0;
volatile sig_atomic_t reload_requested = 0;

/*
 * Advanced signal handler using sigaction with siginfo_t
 * Provides more information about signal source
 */
void advanced_handler(int sig, siginfo_t *info, void *context) {
    /* sig: signal number (e.g., SIGTERM=15) */
    /* info: details about signal origin */
    /* context: processor register state (ucontext_t *) */
    
    switch (sig) {
        case SIGTERM:
            shutdown_requested = 1;
            
            /* Log shutdown request details */
            if (info) {
                char pid_str[32];
                char uid_str[32];
                
                /* Format numbers as strings (async-signal-safe) */
                int pid_len = snprintf(pid_str, sizeof(pid_str), 
                                      "%d", info->si_pid);
                int uid_len = snprintf(uid_str, sizeof(uid_str), 
                                      "%d", info->si_uid);
                
                /* Use write instead of printf */
                const char *msg = "Received SIGTERM from PID: ";
                write(STDOUT_FILENO, msg, strlen(msg));
                write(STDOUT_FILENO, pid_str, pid_len);
                write(STDOUT_FILENO, " UID: ", 6);
                write(STDOUT_FILENO, uid_str, uid_len);
                write(STDOUT_FILENO, "\n", 1);
            }
            break;
            
        case SIGUSR1:
            reload_requested = 1;
            write(STDOUT_FILENO, "Received SIGUSR1, reloading configuration\n", 42);
            break;
    }
}

int main(void) {
    struct sigaction sa;
    
    memset(&sa, 0, sizeof(struct sigaction));
    
    /* Use sa_sigaction instead of sa_handler */
    sa.sa_sigaction = advanced_handler;
    
    /* Set SA_SIGINFO flag to get siginfo_t parameter */
    sa.sa_flags = SA_SIGINFO;
    
    /* Block SIGUSR1 while handling SIGTERM (and vice versa) */
    sigemptyset(&sa.sa_mask);
    sigaddset(&sa.sa_mask, SIGUSR1);
    
    /* Register both handlers */
    if (sigaction(SIGTERM, &sa, NULL) == -1 ||
        sigaction(SIGUSR1, &sa, NULL) == -1) {
        perror("sigaction");
        exit(EXIT_FAILURE);
    }
    
    printf("Process running (PID: %d)\n", getpid());
    printf("Send SIGTERM to shutdown, SIGUSR1 to reload\n");
    
    while (!shutdown_requested) {
        if (reload_requested) {
            printf("Reloading configuration...\n");
            reload_requested = 0;
        }
        
        printf("Working...\n");
        sleep(1);
    }
    
    printf("Shutting down\n");
    return EXIT_SUCCESS;
}
```

### Complete Server Example with Multiple Signal Handlers

```c
#include <stdio.h>
#include <stdlib.h>
#include <signal.h>
#include <unistd.h>
#include <string.h>
#include <errno.h>
#include <time.h>
#include <stdarg.h>
#include <fcntl.h>
#include <sys/stat.h>

/* Configuration and state */
volatile sig_atomic_t shutdown_requested = 0;
volatile sig_atomic_t reload_requested = 0;
volatile sig_atomic_t child_exited = 0;
volatile sig_atomic_t log_level = 1;

int logfile_fd = -1;

/* Async-signal-safe logging */
void safe_log(int level, const char *format, ...) {
    if (level > log_level) return;
    
    char buffer[256];
    va_list args;
    va_start(args, format);
    vsnprintf(buffer, sizeof(buffer), format, args);
    va_end(args);
    
    if (logfile_fd != -1) {
        write(logfile_fd, buffer, strlen(buffer));
        write(logfile_fd, "\n", 1);
    } else {
        write(STDOUT_FILENO, buffer, strlen(buffer));
        write(STDOUT_FILENO, "\n", 1);
    }
}

/* Signal handlers */
void handle_sigterm(int sig, siginfo_t *info, void *context) {
    (void)context; /* Unused */
    shutdown_requested = 1;
    
    char msg[128];
    snprintf(msg, sizeof(msg), 
             "SIGTERM received from PID %d, initiating shutdown", 
             info->si_pid);
    safe_log(0, msg);
}

void handle_sigusr1(int sig, siginfo_t *info, void *context) {
    (void)sig; (void)info; (void)context;
    reload_requested = 1;
    safe_log(1, "SIGUSR1 received, configuration reload requested");
}

void handle_sigusr2(int sig, siginfo_t *info, void *context) {
    (void)sig; (void)info; (void)context;
    log_level = (log_level + 1) % 3;
    
    char msg[64];
    snprintf(msg, sizeof(msg), "Log level changed to %d", log_level);
    safe_log(0, msg);
}

void handle_sigchld(int sig, siginfo_t *info, void *context) {
    (void)sig; (void)context;
    child_exited = 1;
    
    char msg[128];
    snprintf(msg, sizeof(msg), 
             "Child process %d exited with status %d",
             info->si_pid, info->si_status);
    safe_log(1, msg);
}

int setup_signal_handlers(void) {
    struct sigaction sa;
    
    memset(&sa, 0, sizeof(struct sigaction));
    sa.sa_sigaction = handle_sigterm;
    sa.sa_flags = SA_SIGINFO;
    sigemptyset(&sa.sa_mask);
    sigaddset(&sa.sa_mask, SIGUSR1);
    sigaddset(&sa.sa_mask, SIGUSR2);
    if (sigaction(SIGTERM, &sa, NULL) == -1) {
        perror("sigaction(SIGTERM)");
        return -1;
    }
    
    sa.sa_sigaction = handle_sigusr1;
    sigemptyset(&sa.sa_mask);
    sigaddset(&sa.sa_mask, SIGTERM);
    if (sigaction(SIGUSR1, &sa, NULL) == -1) {
        perror("sigaction(SIGUSR1)");
        return -1;
    }
    
    sa.sa_sigaction = handle_sigusr2;
    sigemptyset(&sa.sa_mask);
    if (sigaction(SIGUSR2, &sa, NULL) == -1) {
        perror("sigaction(SIGUSR2)");
        return -1;
    }
    
    sa.sa_sigaction = handle_sigchld;
    sa.sa_flags = SA_SIGINFO | SA_NOCLDSTOP;
    sigemptyset(&sa.sa_mask);
    if (sigaction(SIGCHLD, &sa, NULL) == -1) {
        perror("sigaction(SIGCHLD)");
        return -1;
    }
    
    return 0;
}

int main(void) {
    /* Setup logging */
    logfile_fd = open("server.log", O_CREAT | O_APPEND | O_WRONLY, 0644);
    
    safe_log(0, "===== SERVER START =====");
    safe_log(0, "PID: %d", getpid());
    
    /* Setup signal handlers */
    if (setup_signal_handlers() == -1) {
        safe_log(0, "Failed to setup signal handlers");
        return EXIT_FAILURE;
    }
    
    safe_log(0, "Signal handlers installed");
    
    /* Main loop */
    while (!shutdown_requested) {
        if (reload_requested) {
            safe_log(1, "Reloading configuration...");
            reload_requested = 0;
            /* Reload configuration here */
        }
        
        if (child_exited) {
            safe_log(1, "Cleaning up child process...");
            child_exited = 0;
            /* Wait for children: while(waitpid(-1, NULL, WNOHANG) > 0); */
        }
        
        /* Do server work */
        safe_log(2, "Server operating normally");
        sleep(5);
    }
    
    safe_log(0, "Shutting down...");
    /* Cleanup code */
    
    if (logfile_fd != -1) {
        close(logfile_fd);
    }
    
    safe_log(0, "===== SERVER STOP =====");
    
    return EXIT_SUCCESS;
}
```

### Signal Mask Management Example

```c
#include <stdio.h>
#include <stdlib.h>
#include <signal.h>
#include <unistd.h>
#include <string.h>

void demonstrate_masking(void) {
    sigset_t set, oldset;
    
    /* Start with empty set */
    sigemptyset(&set);
    
    /* Add specific signals to block */
    sigaddset(&set, SIGTERM);
    sigaddset(&set, SIGUSR1);
    
    printf("Blocking SIGTERM and SIGUSR1\n");
    
    /* Block signals and save old mask */
    if (sigprocmask(SIG_BLOCK, &set, &oldset) == -1) {
        perror("sigprocmask(SIG_BLOCK)");
        return;
    }
    
    printf("Signals blocked. Send SIGTERM/SIGUSR1 now - they will queue\n");
    sleep(5);
    
    printf("Unblocking signals...\n");
    
    /* Restore old mask - any pending signals will be delivered */
    if (sigprocmask(SIG_SETMASK, &oldset, NULL) == -1) {
        perror("sigprocmask(SIG_SETMASK)");
        return;
    }
    
    printf("Signals unblocked. Pending signals delivered.\n");
    sleep(2);
}

/* Using sigsuspend for reliable signal handling */
void wait_for_signal_example(void) {
    sigset_t fullset, emptyset;
    
    /* Create signal sets */
    sigfillset(&fullset);      /* All signals */
    sigemptyset(&emptyset);    /* No signals */
    
    /* Block all signals except those in emptyset */
    sigprocmask(SIG_SETMASK, &fullset, NULL);
    
    printf("Waiting for signals (all but SIGKILL/SIGSTOP blocked)\n");
    
    /* 
     * Atomically:
     * 1. Restore signal mask to emptyset
     * 2. Pause until any signal
     * 3. Restore signal mask back to fullset
     * 
     * This prevents race conditions between checking
     * a flag and waiting for signals
     */
    sigsuspend(&emptyset);
    
    /* Resumes here after any signal */
    printf("Signal received, resuming\n");
}

int main(void) {
    demonstrate_masking();
    return EXIT_SUCCESS;
}
```

### SIGKILL Demonstration (What NOT to catch)

```c
#include <stdio.h>
#include <stdlib.h>
#include <signal.h>
#include <unistd.h>
#include <string.h>

void sigterm_handler(int sig) {
    printf("SIGTERM caught! Cleaning up...\n");
    exit(EXIT_SUCCESS);
}

void sigkill_handler(int sig) {
    /* This function WILL NEVER BE CALLED */
    /* SIGKILL cannot be caught */
    printf("This will never print\n");
}

int main(void) {
    struct sigaction sa;
    
    /* Try to register SIGKILL handler (will be ignored) */
    memset(&sa, 0, sizeof(struct sigaction));
    sa.sa_handler = sigkill_handler;
    
    printf("Attempting to register SIGKILL handler...\n");
    if (sigaction(SIGKILL, &sa, NULL) == -1) {
        perror("sigaction(SIGKILL)");
        printf("Error (expected): SIGKILL cannot be caught\n");
    }
    
    /* Register SIGTERM handler (will work) */
    sa.sa_handler = sigterm_handler;
    if (sigaction(SIGTERM, &sa, NULL) == -1) {
        perror("sigaction(SIGTERM)");
        exit(EXIT_FAILURE);
    }
    
    printf("Process PID: %d\n", getpid());
    printf("SIGTERM handler registered successfully\n");
    printf("SIGKILL handler registration silently ignored\n");
    printf("Try: kill -TERM %d (will be caught)\n", getpid());
    printf("Try: kill -KILL %d (cannot be caught)\n", getpid());
    
    /* Wait forever */
    while (1) {
        sleep(1);
    }
    
    return EXIT_SUCCESS;
}
```

---

## 8. Rust Implementation Guide

### Basic Rust Signal Handling with `signal-hook`

```rust
use signal_hook::consts::signal::*;
use signal_hook::iterator::Signals;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use std::thread;
use std::time::Duration;

fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Atomic flag for graceful shutdown
    let should_shutdown = Arc::new(AtomicBool::new(false));
    let should_shutdown_clone = Arc::clone(&should_shutdown);
    
    // Setup signal iterator
    let mut signals = Signals::new(&[SIGTERM, SIGINT])?;
    
    // Spawn signal handling thread
    thread::spawn(move || {
        for sig in signals.forever() {
            match sig {
                SIGTERM | SIGINT => {
                    println!("Received signal {}, initiating graceful shutdown", sig);
                    should_shutdown_clone.store(true, Ordering::Relaxed);
                }
                _ => {
                    println!("Received unexpected signal: {}", sig);
                }
            }
        }
    });
    
    println!("Server running with PID: {}", std::process::id());
    
    // Main application loop
    while !should_shutdown.load(Ordering::Relaxed) {
        println!("Working...");
        thread::sleep(Duration::from_secs(1));
    }
    
    println!("Shutting down gracefully");
    Ok(())
}
```

### Advanced Rust Signal Handler with Channel Communication

```rust
use signal_hook::consts::signal::*;
use signal_hook::iterator::Signals;
use std::sync::mpsc;
use std::thread;
use std::time::Duration;

#[derive(Debug, Clone)]
enum SignalEvent {
    Terminate,
    HangUp,
    UserDefined1,
    UserDefined2,
    ChildProcess,
    Other(i32),
}

fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Create channel for signal communication
    let (tx, rx) = mpsc::channel();
    
    // Setup signal iterator
    let mut signals = Signals::new(&[SIGTERM, SIGHUP, SIGUSR1, SIGUSR2, SIGCHLD])?;
    
    // Spawn dedicated signal handling thread
    let signal_thread = thread::spawn(move || {
        for sig in signals.forever() {
            let event = match sig {
                SIGTERM => SignalEvent::Terminate,
                SIGHUP => SignalEvent::HangUp,
                SIGUSR1 => SignalEvent::UserDefined1,
                SIGUSR2 => SignalEvent::UserDefined2,
                SIGCHLD => SignalEvent::ChildProcess,
                _ => SignalEvent::Other(sig),
            };
            
            if let Err(e) = tx.send(event) {
                eprintln!("Error sending signal event: {}", e);
                break;
            }
        }
    });
    
    println!("Server running (PID: {})", std::process::id());
    
    let mut running = true;
    
    // Main loop with signal handling
    while running {
        // Use try_recv for non-blocking signal check
        match rx.try_recv() {
            Ok(event) => {
                match event {
                    SignalEvent::Terminate => {
                        println!("Received SIGTERM, initiating shutdown");
                        running = false;
                    }
                    SignalEvent::HangUp => {
                        println!("Received SIGHUP, reloading configuration");
                    }
                    SignalEvent::UserDefined1 => {
                        println!("Received SIGUSR1, changing log level");
                    }
                    SignalEvent::UserDefined2 => {
                        println!("Received SIGUSR2, dumping state");
                    }
                    SignalEvent::ChildProcess => {
                        println!("Child process status changed");
                    }
                    SignalEvent::Other(sig) => {
                        println!("Received unexpected signal: {}", sig);
                    }
                }
            }
            Err(mpsc::TryRecvError::Empty) => {
                // No signal, do normal work
                println!("Processing work...");
                thread::sleep(Duration::from_millis(500));
            }
            Err(mpsc::TryRecvError::Disconnected) => {
                println!("Signal thread disconnected");
                running = false;
            }
        }
    }
    
    println!("Waiting for signal thread to finish...");
    let _ = signal_thread.join();
    
    println!("Shutdown complete");
    Ok(())
}
```

### Using tokio for Async Signal Handling

```rust
use tokio::signal::unix::{signal, SignalKind};
use tokio::time::{sleep, Duration};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Setup signal handlers
    let mut sigterm = signal(SignalKind::terminate())?;
    let mut sighup = signal(SignalKind::hangup())?;
    let mut sigusr1 = signal(SignalKind::user_defined1())?;
    
    println!("Async server running (PID: {})", std::process::id());
    
    let mut running = true;
    
    while running {
        tokio::select! {
            // Handle SIGTERM
            _ = sigterm.recv() => {
                println!("Received SIGTERM, initiating shutdown");
                running = false;
            }
            
            // Handle SIGHUP
            _ = sighup.recv() => {
                println!("Received SIGHUP, reloading configuration");
                // Reload configuration async
            }
            
            // Handle SIGUSR1
            _ = sigusr1.recv() => {
                println!("Received SIGUSR1, changing parameters");
            }
            
            // Regular work
            _ = sleep(Duration::from_secs(1)) => {
                println!("Doing work...");
            }
        }
    }
    
    println!("Cleaning up resources...");
    Ok(())
}
```

### Rust Signal Mask Management

```rust
use signal_hook::low_level::signal_mask;
use signal_hook::consts::signal::*;
use nix::sys::signal::{SigmaskHow, sigprocmask};
use nix::sys::signal::SigSet;

fn demonstrate_signal_masking() -> Result<(), Box<dyn std::error::Error>> {
    // Create signal set
    let mut set = SigSet::empty();
    set.add(SIGTERM);
    set.add(SIGUSR1);
    
    println!("Blocking SIGTERM and SIGUSR1");
    
    // Block signals
    sigprocmask(
        Some(SigmaskHow::SIG_BLOCK),
        Some(&set),
        None,
    )?;
    
    println!("Signals blocked for 5 seconds");
    std::thread::sleep(std::time::Duration::from_secs(5));
    
    println!("Unblocking signals");
    
    // Unblock signals
    sigprocmask(
        Some(SigmaskHow::SIG_UNBLOCK),
        Some(&set),
        None,
    )?;
    
    println!("Pending signals will now be delivered");
    Ok(())
}
```

### Complete Rust Server with State Management

```rust
use signal_hook::consts::signal::*;
use signal_hook::iterator::Signals;
use std::sync::Arc;
use std::sync::atomic::{AtomicBool, AtomicU32, Ordering};
use std::thread;
use std::time::{Duration, SystemTime};

struct ServerState {
    shutdown: Arc<AtomicBool>,
    reload_config: Arc<AtomicBool>,
    requests_processed: Arc<AtomicU32>,
    start_time: SystemTime,
}

impl ServerState {
    fn new() -> Self {
        ServerState {
            shutdown: Arc::new(AtomicBool::new(false)),
            reload_config: Arc::new(AtomicBool::new(false)),
            requests_processed: Arc::new(AtomicU32::new(0)),
            start_time: SystemTime::now(),
        }
    }
    
    fn handle_signals(&self) -> Result<(), Box<dyn std::error::Error>> {
        let mut signals = Signals::new(&[SIGTERM, SIGINT, SIGHUP])?;
        
        let shutdown = Arc::clone(&self.shutdown);
        let reload_config = Arc::clone(&self.reload_config);
        
        thread::spawn(move || {
            for sig in signals.forever() {
                match sig {
                    SIGTERM | SIGINT => {
                        println!("Received terminate signal");
                        shutdown.store(true, Ordering::Relaxed);
                    }
                    SIGHUP => {
                        println!("Received HUP signal, reloading configuration");
                        reload_config.store(true, Ordering::Relaxed);
                    }
                    _ => {}
                }
            }
        });
        
        Ok(())
    }
    
    fn is_shutdown(&self) -> bool {
        self.shutdown.load(Ordering::Relaxed)
    }
    
    fn should_reload(&self) -> bool {
        self.reload_config.swap(false, Ordering::Relaxed)
    }
    
    fn uptime(&self) -> Duration {
        self.start_time.elapsed().unwrap_or(Duration::ZERO)
    }
}

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let state = ServerState::new();
    state.handle_signals()?;
    
    println!("Server started (PID: {})", std::process::id());
    println!("Send SIGTERM to shutdown, SIGHUP to reload");
    
    while !state.is_shutdown() {
        if state.should_reload() {
            println!("Reloading configuration...");
        }
        
        // Simulate request processing
        let count = state.requests_processed.fetch_add(1, Ordering::Relaxed);
        
        println!(
            "Uptime: {:.1}s, Requests: {}",
            state.uptime().as_secs_f64(),
            count
        );
        
        thread::sleep(Duration::from_secs(1));
    }
    
    println!("Shutting down gracefully");
    Ok(())
}
```

---

## 9. Linux Kernel Deep Dive

### Kernel Signal Delivery Code Flow (Simplified)

```
linux/kernel/signal.c - Core signal delivery routines:
═════════════════════════════════════════════════════════════════

sys_kill()  [syscall entry point]
    │
    ├─► Check PID validity
    ├─► Look up target task_struct
    ├─► Check permissions (CAP_KILL, same UID/EUID, etc.)
    │
    └─► signal_send()
        │
        ├─► Check if signal is SIGKILL or SIGSTOP
        │   └─ Yes: Force delivery, skip mask checks
        │
        ├─► Check if signal blocked (sigprocmask)
        │   └─ Yes: Queue in pending
        │
        ├─► Add to signal queue
        │   ├─ Standard signal (1-31): Set bit in sigset_t
        │   └─ Real-time signal (34-64): Add sigqueue node
        │
        ├─► Mark task as runnable
        │   └─ If it's sleeping on interruptible queue
        │
        └─► Wake up task
            └─ Will check pending signals at next opportunity

do_signal()  [signal delivery in kernel/signal.c]
    │
    └─► Called from:
        ├─ System call exit path (ret_to_user)
        ├─ Interrupt handler exit
        ├─ Exception handler exit
        
        ├─► Get pending signal number
        │
        ├─► Check signal mask
        │   ├─ If masked: return to caller
        │   └─ If not masked: proceed
        │
        ├─► Lookup sigaction for signal
        │
        ├─► Determine action:
        │   ├─ SIG_DFL (default action)
        │   │  └─ SIGKILL: exit_group()
        │   │  └─ SIGSTOP: ptrace_stop()
        │   │  └─ SIGTERM: exit_group()
        │   │  └─ etc.
        │   │
        │   ├─ SIG_IGN (ignore)
        │   │  └─ return (do nothing)
        │   │
        │   └─ Custom handler
        │      └─ setup_frame()
        │         └─ Modify user stack
        │         └─ Jump to handler
        │
        └─► Return to execution point

setup_frame()  [arch/x86_64/kernel/signal.c]
    │
    └─► Setup signal frame on user stack:
        │
        ├─► Save registers
        │   ├─ RAX, RBX, RCX, RDX, RSI, RDI, RBP, RSP
        │   ├─ R8-R15
        │   ├─ RFLAGS, RIP
        │   └─ FPU state (XMM registers, etc.)
        │
        ├─► Build sigcontext
        │   └─ Processor state snapshot
        │
        ├─► Build siginfo_t (if SA_SIGINFO)
        │   ├─ Signal number
        │   ├─ Sender PID
        │   ├─ Sender UID
        │   ├─ Return address
        │   └─ Custom si_value (for real-time signals)
        │
        ├─► Modify registers:
        │   ├─ RSP (stack pointer) = new frame address
        │   ├─ RIP (instruction pointer) = handler address
        │   ├─ RDI (first arg) = signal number
        │   ├─ RSI (second arg) = siginfo_t pointer
        │   ├─ RDX (third arg) = ucontext_t pointer
        │   └─ RCX = return trampoline address (rt_sigreturn)
        │
        └─► Signal frame layout:
            
            User Stack Layout (x86-64):
            ┌──────────────────────────┐ ← Higher Address
            │  Original Stack           │
            │  (grew downward before)   │
            ├──────────────────────────┤
            │  Return Address (saved)   │ ← Original RIP
            ├──────────────────────────┤
            │  Siginfo Structure        │ → siginfo_t
            ├──────────────────────────┤
            │  Ucontext Structure       │ → ucontext_t
            │  (includes sigcontext)    │
            │  - Old registers          │
            │  - FPU state              │
            │  - Signal mask            │
            ├──────────────────────────┤
            │  Return Trampoline Code   │
            │  (rt_sigreturn syscall)   │
            ├──────────────────────────┤ ← New RSP (stack pointer)
            │  New Stack Space          │
            └──────────────────────────┘ ← Lower Address
            
            Handler execution:
            ├─ RDI = signal number
            ├─ RSI = siginfo_t *
            ├─ RDX = ucontext_t *
            └─ Call handler()
               └─ Handler uses RBP to access saved registers
               └─ Handler can modify registers
               └─ Handler calls return/longjmp
                  ├─ Normal return → rt_sigreturn() syscall
                  │  └─ Restore original registers
                  │  └─ Resume at original RIP
                  │
                  └─ longjmp() → Jump to handler context
                     └─ Restore registers from jmp_buf
```

### Kernel Data Structures for Signals

```c
/* From linux/include/linux/signal.h and linux/kernel/signal.c */

/* Signal pending queue per thread */
struct sigpending {
    struct list_head list;      /* List of pending real-time signals */
    sigset_t signal;            /* Bitmask for standard signals (1-31) */
};

/* Real-time signal queue node */
struct sigqueue {
    struct list_head list;      /* Links in task->pending.list */
    int flags;                  /* Flags (SIGQUEUE_PREALLOC, etc.) */
    siginfo_t info;             /* Signal info (includes si_value) */
    struct user_struct *user;   /* User who queued this signal */
};

/* Signal action */
struct k_sigaction {
    struct sigaction sa;        /* User-specified action */
};

struct sigaction {
    void (*sa_handler)(int);           /* Handler or SIG_DFL/SIG_IGN */
    void (*sa_sigaction)(int, siginfo_t *, void *);
    sigset_t sa_mask;                  /* Mask during handler */
    int sa_flags;                      /* Flags: SA_SIGINFO, SA_RESTART... */
    void (*sa_restorer)(void);         /* Return point (obsolete) */
};

/* Per-process signal state */
struct signal_struct {
    atomic_t refcount;                 /* Reference count */
    int nr_threads;                    /* Number of threads */
    
    /* Signal action for entire process group */
    struct k_sigaction action[_NSIG];  /* sigaction for each signal */
    
    /* Shared signal pending (group-catchable signals) */
    struct sigpending shared_pending;
    
    /* Termination signal */
    int group_exit_code;
    
    /* Kill by signal information */
    struct {
        unsigned int flags;            /* SIGNAL_GROUP_EXIT, etc. */
        int group_exit_code;
        struct siginfo *info;
    } group_exit;
};

/* Per-thread signal state */
struct sighand_struct {
    atomic_t count;                    /* Reference count */
    struct k_sigaction action[_NSIG]; /* Signal handlers for thread */
    spinlock_t siglock;                /* Protects handler table */
};

/* Task signal fields relevant to signal handling */
struct task_struct {
    /* Per-thread signal state */
    struct sigpending pending;         /* Signals to handle */
    sigset_t blocked;                  /* Blocked signals */
    
    struct sighand_struct *sighand;    /* Signal handlers table */
    
    /* Real-time signal processing */
    struct sigqueue *sigqueue_cache;   /* Pre-allocated sigqueue */
    
    /* Signal restorer (return point from handler) */
    unsigned long sas_ss_sp;           /* Alternate signal stack */
    size_t sas_ss_size;                /* Size of alt signal stack */
    unsigned sas_ss_flags;             /* State of alt signal stack */
    
    /* Tracers (debuggers) */
    struct task_struct *parent;
    struct list_head children;
};
```

### SIGKILL vs SIGTERM at Kernel Level

```c
/* Simplified kernel handling differences */

/* SIGKILL (9) handling - From linux/kernel/signal.c */
if (sig == SIGKILL) {
    /*
     * SIGKILL bypasses all checks:
     * - Cannot be blocked
     * - Cannot be caught
     * - No handler invocation
     * - Direct execution of default action
     */
    
    /* Force through sigprocmask check */
    if (sigismember(&t->blocked, SIGKILL)) {
        /* Remove from blocked set - ignored */
        sigdelset(&t->blocked, SIGKILL);
    }
    
    /* Call default action directly */
    sig_kernel_ignore(sig);  /* Actually performs: exit_group() */
    
    /* Mark task for death */
    signal_group_exit(t->signal, t->exit_code);
    
    /* Schedule for termination */
    return 0;  /* No handler to invoke */
}

/* SIGTERM (15) handling - Normal signal path */
if (!sigismember(&t->blocked, SIGTERM)) {
    /* Not blocked, proceed to delivery */
    
    /* Check for handler */
    if (t->sighand->action[SIGTERM].sa_handler != SIG_DFL) {
        /* Custom handler registered - invoke it */
        setup_signal_frame(t, SIGTERM);
        return 1;  /* Handler invoked */
    } else {
        /* Default action: terminate */
        send_signal(SIGTERM, &info, t);
        return 0;  /* Use default action */
    }
} else {
    /* Signal is blocked - queue it */
    pending_queue_add(&t->pending, SIGTERM, &info);
    return 0;  /* Queued for later */
}

/* Key difference in source code behavior:

SIGKILL:
    - Not affected by sigprocmask()
    - Not affected by sa_mask
    - Kernel code: signal_group_exit() called immediately
    - No user space handler involved
    - Process dies synchronously
    
SIGTERM:
    - Respects sigprocmask() entirely
    - Respects sa_mask during handler
    - Kernel code: normal signal delivery pipeline
    - User space handler can run
    - Process has chance to cleanup
*/
```

### Kernel Source References

```
Key kernel files involved in signal delivery:

linux/kernel/signal.c
    • sys_kill()            - kill() syscall implementation
    • sys_tkill()           - thread-specific kill
    • sys_tgkill()          - thread-group kill
    • do_signal()           - Main signal delivery routine
    • setup_frame()         - Stack frame setup for handlers
    • sigprocmask()         - Signal mask syscall
    • sigaction()           - Signal handler registration
    • signal_group_exit()   - Group termination
    
linux/kernel/exit.c
    • do_exit()             - Process exit (called by SIGKILL/SIGTERM)
    • do_group_exit()       - Process group exit
    • release_task()        - Clean up terminated process
    
arch/x86_64/kernel/signal.c
    • setup_rt_frame()      - Real-time signal frame setup
    • restore_sigcontext()  - Restore registers from signal
    • rt_sigreturn()        - Syscall to return from handler
    
include/linux/signal.h
    • struct sigaction      - Signal action structure
    • struct siginfo        - Signal information
    • struct sigpending     - Pending signals structure
    • Signal number defines - SIGKILL, SIGTERM, etc.
    
include/asm-x86/signal.h (or arch-specific)
    • Signal frame layouts
    • Register definitions
    • Stack frame structure
```

---

## 10. Real-World Scenarios

### Scenario 1: Docker Container Graceful Shutdown

```
Timeline: Docker Container Termination with SIGTERM/SIGKILL
═════════════════════════════════════════════════════════════════

T=0s    docker stop <container>
        │
        ├─► Docker daemon sends SIGTERM to PID 1 (init/app)
        │   └─ Container signal-handler.c receives SIGTERM
        │      └─ Logs: "Graceful shutdown initiated"
        │      └─ Closes database connections
        │      └─ Flushes buffers
        │      └─ Signals children to shutdown
        │      └─ Blocks until children exit

T=5s    All children exited
        │
        ├─► Application closes remaining resources
        ├─► Runs cleanup code
        └─► Calls exit(0)

T=5.1s  Container stops
        └─ SUCCESS: All work completed, no data loss

Alternative timeline (if application ignores SIGTERM):

T=0s    docker stop <container>
        │
        └─► SIGTERM sent (ignored by buggy app)

T=1s    Application still running
        └─ No response to SIGTERM

T=10s   docker stop timeout (default: 10 seconds)
        │
        └─► Docker sends SIGKILL
            └─ Process killed immediately
            └─ Cleanup code never runs
            └─ Potential data loss
            └─ Locks not released
            └─ Connections left hanging

T=10.1s Container force-stopped
        └─ FAILURE: Data may be lost, resources leaked
```

### Scenario 2: Graceful Restart Pattern

```
Multi-Process Graceful Restart Pattern:
═════════════════════════════════════════════════════════════════

Parent Process (PID 100):
    │
    ├─► Signal handler for SIGHUP
    │   └─ reload_config = true
    │
    └─► Main loop:
        ├─ if (reload_config)
        │  ├─ Stop accepting new work
        │  ├─ Send SIGTERM to worker processes
        │  ├─ Wait for workers to exit (with timeout)
        │  ├─ Reload configuration
        │  ├─ Fork new worker processes
        │  └─ Resume accepting work
        │
        └─ Process work items

Worker Processes (PIDs 101, 102, 103):
    │
    ├─► Signal handler for SIGTERM
    │   └─ graceful_shutdown = true
    │
    └─► Worker loop:
        ├─ Accept work item
        ├─ Process work item
        ├─ If graceful_shutdown set after item completes
        │  └─ Break loop and exit
        └─ Otherwise get next item

Timeline:
    T=0:    Parent processes requests
            Workers 101,102,103 active
            
    T=5:    kill -HUP 100  (SIGHUP to parent)
            │
            ├─► Parent receives SIGHUP
            │   └─ reload_config = true
            │
            ├─► Parent sends SIGTERM to 101,102,103
            │   └─ Workers receive SIGTERM
            │       └─ graceful_shutdown = true
            │       └─ Continue processing current item
            │
            ├─► Parent waits up to 30 seconds for workers
            │   └─ Workers finish current items
            │   └─ Exit gracefully
            │
            └─► Parent closes old connections
                ├─ Reloads configuration
                ├─ Forks new workers 104,105,106
                └─ Resumes accepting requests

    T=35:   New workers fully operational
            Old workers completely replaced
            No request interruption (minimal)
```

### Scenario 3: Zombie Process Cleanup

```
Zombie Process and SIGCHLD Handling:
═════════════════════════════════════════════════════════════════

Parent Process (PID 1000):
    │
    ├─► fork() creates child (PID 1001)
    │   └─ Child process starts
    │
    ├─► Child does work
    │   └─ exit(0) - calls exit but not waited by parent
    │
    ├─► Child becomes ZOMBIE
    │   └─ Task structure still exists
    │   └─ Memory freed except for process descriptor
    │   └─ Waiting for parent to call wait()
    │
    └─► Parent receives SIGCHLD (default: ignored)
        └─ Continues running (doesn't call wait())
        
        Problem: Zombie accumulates in process list
        ├─ ps aux shows: [defunct]
        ├─ Eventually process table fills up
        └─ No new processes can be created

Solution: Register SIGCHLD handler

struct sigaction sa;
memset(&sa, 0, sizeof(sa));
sa.sa_handler = handle_sigchld;
sa.sa_flags = SA_NOCLDSTOP;  /* Don't generate SIGCHLD on SIGSTOP */
sigaction(SIGCHLD, &sa, NULL);

void handle_sigchld(int sig) {
    pid_t pid;
    int status;
    
    /* Reap all terminated children */
    while ((pid = waitpid(-1, &status, WNOHANG)) > 0) {
        printf("Child %d exited with status %d\n", pid, status);
    }
}

Timeline with SIGCHLD handler:
    │
    T=0:    Parent spawns child (PID 1001)
    │
    T=1:    Child exits
    │       └─ Becomes zombie
    │       └─ Kernel sends SIGCHLD to parent (1000)
    │
    T=1.01: Parent receives SIGCHLD
    │       └─ Handler invokes
    │       └─ Calls waitpid()
    │       └─ Reaps child process
    │       └─ Zombie state resolved
    │       └─ Process table entry freed
    │
    T=1.02: Handler returns, parent resumes
            └─ Child completely cleaned up
```

### Scenario 4: Signal Ordering and Delivery

```
Multiple Signal Delivery Order:
═════════════════════════════════════════════════════════════════

Scenario: Process receives SIGTERM, SIGUSR1, SIGTERM, SIGUSR1

Time Event
════════════════════════════════════════════════════════════════

T=0     SIGTERM arrives → Queued (signal 15 bit set)
        Pending bitmask: 0b0...0100000000000000000000000000

T=1     SIGUSR1 arrives → Queued (signal 10 bit set)
        Pending bitmask: 0b0...0100000000000000010000000000

T=2     SIGTERM arrives → IGNORED (already queued, bit already set)
        Standard signals coalesce - no change
        Pending bitmask: 0b0...0100000000000000010000000000

T=3     SIGUSR1 arrives → IGNORED (already queued, bit already set)
        Standard signals coalesce
        Pending bitmask: 0b0...0100000000000000010000000000

T=4     Process checks pending signals
        └─ Found: SIGTERM and SIGUSR1
        └─ Delivery order: Signal number ascending
           (SIGUSR1=10 before SIGTERM=15)
           
        Handler execution order:
        1. SIGUSR1 handler invoked
           └─ Processes one logical signal
        2. SIGTERM handler invoked
           └─ Processes one logical signal

Result: 2 handler invocations (4 signals became 2)
        Data loss possible if signal data significant

With Real-Time Signals (34-64):
═════════════════════════════════════════════════════════════════

Time Event
════════════════════════════════════════════════════════════════

T=0     SIGRTMIN+5 (value 40) → Queued
        sigqueue: [queue node #1: si_value=100]

T=1     SIGRTMIN+5 arrives → Queued separately
        sigqueue: [queue node #1: si_value=100]
                → [queue node #2: si_value=200]

T=2     SIGRTMIN+5 arrives → Queued separately
        sigqueue: [queue node #1: si_value=100]
                → [queue node #2: si_value=200]
                → [queue node #3: si_value=300]

T=3     Process checks pending signals
        └─ Found: SIGRTMIN+5 (three instances)
        
        Handler execution order (FIFO per signal):
        1. SIGRTMIN+5 handler (si_value=100)
        2. SIGRTMIN+5 handler (si_value=200)
        3. SIGRTMIN+5 handler (si_value=300)

Result: 3 separate handler invocations
        All data preserved
        No data loss
```

---

## 11. Best Practices and Patterns

### DO's and DON'Ts

```
SIGNAL HANDLERS - BEST PRACTICES
═════════════════════════════════════════════════════════════════

DO:
✓ Use volatile sig_atomic_t for flags
✓ Call only async-signal-safe functions
✓ Keep handlers as short as possible
✓ Use sigaction() instead of signal()
✓ Register all signal handlers at startup
✓ Use signal masks to prevent reentrancy
✓ Use sigprocmask() for critical sections
✓ Log to file instead of printf() in handlers
✓ Use write() for output (async-signal-safe)
✓ Send SIGTERM first, then SIGKILL if needed
✓ Implement signal handlers in application code
✓ Test signal handling thoroughly

DON'T:
✗ Use malloc/free in handler (not async-signal-safe)
✗ Call printf() in handler (uses malloc internally)
✗ Call pthread_mutex_lock() in handler
✗ Call pthread_cond_wait() in handler
✗ Call longjmp() without precautions
✗ Use unsafe string functions (sprintf, strcpy)
✗ Assume signal order
✗ Assume signal delivery timing
✗ Block SIGKILL/SIGSTOP (impossible anyway)
✗ Use SIGKILL as first choice (use SIGTERM first)
✗ Ignore child process signals (leads to zombies)
✗ Forget to handle SIGPIPE (broken pipe)
✗ Use signal() without considering portability


ASYNC-SIGNAL-SAFE FUNCTIONS
═════════════════════════════════════════════════════════════════

Safe to call in signal handlers:

File I/O:
    • open, close, read, write
    • fsync, fdatasync
    • lseek, lseek64

String operations:
    • strlen
    • (NOT strcpy, sprintf, malloc-dependent)

Process/Signal:
    • kill, raise
    • exit, _exit, _Exit
    • sigaction, sigprocmask

Synchronization:
    • sem_post (semaphore, NOT mutex)
    • (NOT pthread_mutex_lock)

Miscellaneous:
    • getpid, getppid, getuid
    • pause, sleep, alarm
    • fork, execve (for simple exec cases)

SEE ALSO: man 7 signal-safety

UNSAFE functions (commonly misused):
    ✗ printf, fprintf, sprintf
    ✗ malloc, free, calloc, realloc
    ✗ pthread_mutex_lock
    ✗ pthread_cond_wait
    ✗ rand, random
    ✗ time, gettimeofday
    ✗ getpwuid, getpwnam (library functions)
    ✗ abort, assert
    ✗ longjmp, siglongjmp (with care)
```

### Graceful Shutdown Pattern

```c
#include <stdio.h>
#include <stdlib.h>
#include <signal.h>
#include <unistd.h>
#include <string.h>
#include <time.h>

/* Global state */
volatile sig_atomic_t shutdown_requested = 0;
volatile sig_atomic_t shutdown_timeout = 0;

/* Configuration */
#define GRACEFUL_SHUTDOWN_TIMEOUT 30

void signal_handler(int sig) {
    if (sig == SIGTERM) {
        shutdown_requested = 1;
    } else if (sig == SIGALRM) {
        shutdown_timeout = 1;
    }
}

int main(void) {
    struct sigaction sa;
    struct timespec ts_start;
    unsigned int remaining;
    
    /* Setup signal handlers */
    memset(&sa, 0, sizeof(struct sigaction));
    sa.sa_handler = signal_handler;
    sigemptyset(&sa.sa_mask);
    sa.sa_flags = 0;
    
    sigaction(SIGTERM, &sa, NULL);
    sigaction(SIGALRM, &sa, NULL);
    
    printf("Server running (PID: %d)\n", getpid());
    
    clock_gettime(CLOCK_MONOTONIC, &ts_start);
    
    /* Main loop */
    while (!shutdown_requested) {
        printf("Processing requests...\n");
        sleep(1);
    }
    
    printf("SIGTERM received, initiating graceful shutdown\n");
    
    /* Stage 1: Graceful shutdown with timeout */
    alarm(GRACEFUL_SHUTDOWN_TIMEOUT);
    
    printf("Closing connections...\n");
    sleep(2);
    
    printf("Flushing data...\n");
    sleep(2);
    
    printf("Shutdown complete\n");
    alarm(0);  /* Cancel alarm if we finished early */
    
    return EXIT_SUCCESS;
}
```

### Safe Logging in Signal Handlers

```c
#include <unistd.h>
#include <fcntl.h>
#include <string.h>
#include <stdio.h>

#define LOG_FILE "application.log"
#define MAX_LOG_MSG 256

int log_fd = -1;

/* Async-signal-safe logging */
void safe_syslog(const char *prefix, int sig, const char *msg) {
    char buffer[MAX_LOG_MSG];
    int len;
    
    /* Format message (snprintf is async-signal-safe in glibc) */
    len = snprintf(buffer, sizeof(buffer),
                   "[%d] %s: signal %d - %s\n",
                   getpid(), prefix, sig, msg);
    
    if (len < 0 || len >= (int)sizeof(buffer)) {
        len = sizeof(buffer) - 1;
    }
    
    /* Write to file (async-signal-safe) */
    if (log_fd != -1) {
        write(log_fd, buffer, len);
    }
    
    /* Also write to stderr (async-signal-safe) */
    write(STDERR_FILENO, buffer, len);
}

void sigterm_handler(int sig) {
    safe_syslog("HANDLER", sig, "SIGTERM received");
    _exit(0);  /* Use _exit, not exit() */
}

int main(void) {
    struct sigaction sa;
    
    /* Open log file at startup (not in signal handler) */
    log_fd = open(LOG_FILE, O_CREAT | O_APPEND | O_WRONLY, 0644);
    
    memset(&sa, 0, sizeof(struct sigaction));
    sa.sa_handler = sigterm_handler;
    sigaction(SIGTERM, &sa, NULL);
    
    safe_syslog("MAIN", 0, "Application started");
    
    while (1) {
        sleep(1);
    }
    
    close(log_fd);
    return 0;
}
```

### Signal Handling in Threads

```c
#include <stdio.h>
#include <stdlib.h>
#include <pthread.h>
#include <signal.h>
#include <unistd.h>
#include <string.h>

volatile sig_atomic_t shutdown = 0;

/*
 * Best practice: Dedicate one thread to signal handling
 * Block signals in all other threads
 */

void *signal_thread(void *arg) {
    sigset_t *set = (sigset_t *)arg;
    int sig;
    siginfo_t info;
    
    printf("Signal handler thread running\n");
    
    while (!shutdown) {
        /* Wait for any signal in the set */
        if (sigwait(set, &sig) == 0) {
            printf("Signal thread received signal: %d\n", sig);
            
            if (sig == SIGTERM) {
                shutdown = 1;
            }
        }
    }
    
    return NULL;
}

int main(void) {
    pthread_t sig_thread;
    sigset_t set;
    
    /* Block signals in all threads */
    sigemptyset(&set);
    sigaddset(&set, SIGTERM);
    sigaddset(&set, SIGUSR1);
    
    /* Block in main thread */
    if (pthread_sigmask(SIG_BLOCK, &set, NULL) != 0) {
        perror("pthread_sigmask");
        return EXIT_FAILURE;
    }
    
    /* Create dedicated signal handling thread */
    if (pthread_create(&sig_thread, NULL, signal_thread, &set) != 0) {
        perror("pthread_create");
        return EXIT_FAILURE;
    }
    
    printf("Main thread doing work\n");
    
    /* Main thread can do work without worrying about signals */
    while (!shutdown) {
        printf("Working...\n");
        sleep(1);
    }
    
    printf("Main thread shutting down\n");
    
    /* Wait for signal thread */
    pthread_join(sig_thread, NULL);
    
    printf("Shutdown complete\n");
    return EXIT_SUCCESS;
}
```

---

## 12. Debugging and Troubleshooting

### Checking Signal Status

```bash
# View signals sent to a process
strace -e trace=signal -p <pid>

# View pending signals in /proc
cat /proc/<pid>/status | grep SigPnd
# Output: SigPnd: 0000000000000000  (pending signals bitmask)
# Output: SigBlk: 0000000000000000  (blocked signals bitmask)
# Output: SigIgn: 0000000000000000  (ignored signals bitmask)
# Output: SigCgt: 0000000000000000  (caught signals bitmask)

# View signal handlers
cat /proc/<pid>/status | grep SigCgt
# Shows which signals have handlers registered

# View process threads and signal handling
cat /proc/<pid>/task/*/status

# Check signal status for all signals
for sig in {1..64}; do
    name=$(kill -l $sig 2>/dev/null || echo "SIG?")
    echo "Signal $sig: $name"
done
```

### Signal Debugging Tools

```bash
# gdb debugging signals
gdb ./program <pid>
(gdb) handle SIGTERM print  # Print when SIGTERM received
(gdb) handle SIGTERM nostop # Don't stop debugger on SIGTERM
(gdb) handle SIGTERM nopass # Don't send signal to process

# strace signal tracing
strace -e signal -f ./program    # Trace all signals, follow forks
strace -e /signal ./program      # Trace only signal syscalls
strace -e signal=SIGTERM ./prog  # Trace only SIGTERM

# ltrace for library calls
ltrace -e signal ./program

# Check what signals are blocking
cat /proc/<pid>/status | grep -E 'SigBlk|SigPnd'
```

### Common Issues and Solutions

```
Problem: Process doesn't respond to SIGTERM
═════════════════════════════════════════════════════════════════

Cause 1: Signal handler not registered
    └─ Solution: Check code for sigaction() call
    └─ Verify: cat /proc/<pid>/status | grep SigCgt

Cause 2: Signal handler is blocking (e.g., I/O)
    └─ Solution: Move blocking operations out of handler
    └─ Use non-blocking I/O or separate thread
    └─ Keep handler short and simple

Cause 3: Signal blocked by sigprocmask()
    └─ Solution: Check /proc/<pid>/status for SigBlk
    └─ Review code for sigprocmask calls
    └─ Verify signal mask is correctly managed

Cause 4: Handler calls exit() without cleanup
    └─ Solution: Use proper cleanup before exit
    └─ Use atexit() for cleanup functions
    └─ Close file handles explicitly

Solution: Force kill with SIGKILL
    └─ kill -9 <pid>
    └─ Last resort, may leak resources


Problem: Zombie processes accumulate
═════════════════════════════════════════════════════════════════

Cause: Parent not handling SIGCHLD
    └─ Solution: Register SIGCHLD handler
    └─ Call waitpid() in handler with WNOHANG

Cause: Parent ignoring child exit
    └─ Solution: Set SA_NOCLDWAIT flag
    └─ Or register SIGCHLD handler

Check: ps aux | grep defunct
    └─ Lists all zombie processes


Problem: Signal delivery unreliable
═════════════════════════════════════════════════════════════════

Cause: Using standard signals (1-31)
    └─ Multiple instances coalesce
    └─ Solution: Use real-time signals (34-64)
    └─ Or use pipe/socket for reliable messaging

Cause: Signal blocked during delivery
    └─ Solution: Review sa_mask in sigaction
    └─ Minimize mask to essential signals
    └─ Don't mask the signal being delivered


Problem: Handler reentrancy issues
═════════════════════════════════════════════════════════════════

Cause: Handler called multiple times simultaneously
    └─ Solution: Properly set sa_mask
    └─ Block own signal during execution
    └─ Use atomic variables

Example fix:
    struct sigaction sa;
    sigemptyset(&sa.sa_mask);
    sigaddset(&sa.sa_mask, SIGTERM);  ← Block SIGTERM in handler
    sa.sa_handler = handler;
    sigaction(SIGTERM, &sa, NULL);


Problem: Data corruption in signal handler
═════════════════════════════════════════════════════════════════

Cause: Using unsafe functions (malloc, printf)
    └─ Solution: Only use async-signal-safe functions
    └─ Use write() instead of printf()
    └─ Set flags with volatile sig_atomic_t
    └─ Use char arrays instead of malloc()

Cause: Modifying shared data structures
    └─ Solution: Make modifications atomic
    └─ Use sigprocmask() to disable signals during critical sections
    └─ Use locks designed for signal handlers
```

### Debugging with Signals

```c
#include <stdio.h>
#include <stdlib.h>
#include <signal.h>
#include <unistd.h>
#include <string.h>
#include <backtrace.h>  /* For stack traces */

#define BACKTRACE_SIZE 64

void print_backtrace(void) {
    void *addrlist[BACKTRACE_SIZE];
    int addrlen = backtrace(addrlist, BACKTRACE_SIZE);
    backtrace_symbols_fd(addrlist, addrlen, STDERR_FILENO);
}

void debug_signal_handler(int sig) {
    char msg[128];
    snprintf(msg, sizeof(msg), 
             "\n=== SIGNAL %d RECEIVED - STACK TRACE ===\n", sig);
    write(STDERR_FILENO, msg, strlen(msg));
    
    print_backtrace();
    
    write(STDERR_FILENO, "=== END TRACE ===\n", 18);
}

int main(void) {
    struct sigaction sa;
    
    memset(&sa, 0, sizeof(struct sigaction));
    sa.sa_handler = debug_signal_handler;
    sa.sa_flags = SA_RESTART;  /* Restart interrupted syscalls */
    sigemptyset(&sa.sa_mask);
    
    /* Install debug handler for crash signals */
    sigaction(SIGSEGV, &sa, NULL);  /* Segmentation fault */
    sigaction(SIGABRT, &sa, NULL);  /* Abort */
    sigaction(SIGBUS, &sa, NULL);   /* Bus error */
    
    printf("Debug handlers installed\n");
    printf("PID: %d\n", getpid());
    
    /* Test: Trigger segmentation fault for demo */
    /* int *null_ptr = NULL; */
    /* *null_ptr = 0; */
    
    return EXIT_SUCCESS;
}
```

---

## Conclusion

Understanding SIGKILL and SIGTERM is fundamental to writing robust, production-grade applications. Here's a quick recap:

**SIGTERM (15)**: The civilized termination signal
- Can be caught and handled
- Allows graceful shutdown with cleanup
- Should be the first choice for terminating processes
- Application gets notification and can save state

**SIGKILL (9)**: The nuclear option
- Cannot be caught or blocked
- Immediate, forceful termination
- No cleanup code runs
- Use only as last resort
- Can leak resources

**Best Practice Pattern**:
```
1. Send SIGTERM to process
2. Wait 5-30 seconds for graceful shutdown
3. If still running, send SIGKILL
4. Process is guaranteed dead after SIGKILL
```

By mastering signals, you'll write applications that:
- Shut down gracefully
- Clean up resources properly
- Handle errors robustly
- Integrate well with process managers
- Can be deployed reliably in production environments

This knowledge forms the foundation for understanding process management, container orchestration, and system-level programming on Unix/Linux systems.

---

## References

- POSIX.1-1990 Standard
- Linux kernel source (linux/kernel/signal.c)
- man 7 signal, man 2 kill, man 2 sigaction
- man 7 pthreads (thread signal handling)
- "Unix Network Programming" by Stevens & Fenner
- Linux kernel documentation: signals.txt
