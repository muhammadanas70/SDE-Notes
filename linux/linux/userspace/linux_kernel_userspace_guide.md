# The Complete Guide to Linux Kernel–User Space Communication

> A deep, in-depth reference covering every mechanism, concept, and real-world pattern for how user space talks to the Linux kernel — with ASCII architecture diagrams and C/Rust implementations.

---

## Table of Contents

1. [Mental Model: The Two Worlds](#1-mental-model-the-two-worlds)
2. [CPU Privilege Rings & Protection Model](#2-cpu-privilege-rings--protection-model)
3. [The System Call Interface — The Master Gateway](#3-the-system-call-interface--the-master-gateway)
4. [C Library (glibc/musl) — The Abstraction Layer](#4-c-library-glibcmusl--the-abstraction-layer)
5. [The Terminal & Shell — How CLI Talks to the Kernel](#5-the-terminal--shell--how-cli-talks-to-the-kernel)
6. [Virtual Filesystems: /proc, /sys, /dev, /run](#6-virtual-filesystems-proc-sys-dev-run)
7. [Device Files & ioctl](#7-device-files--ioctl)
8. [Signals — Asynchronous Kernel–User Communication](#8-signals--asynchronous-kerneluser-communication)
9. [Pipes & FIFOs](#9-pipes--fifos)
10. [Sockets — Unix Domain & Netlink](#10-sockets--unix-domain--netlink)
11. [Memory-Mapped I/O (mmap)](#11-memory-mapped-io-mmap)
12. [eBPF — Programmable Kernel Extension](#12-ebpf--programmable-kernel-extension)
13. [FUSE — Filesystem in Userspace](#13-fuse--filesystem-in-userspace)
14. [Kernel Modules & /dev Interface](#14-kernel-modules--dev-interface)
15. [Uevents & udev — Hotplug & Device Management](#15-uevents--udev--hotplug--device-management)
16. [Audit Subsystem](#16-audit-subsystem)
17. [perf & ftrace — Performance & Tracing Interfaces](#17-perf--ftrace--performance--tracing-interfaces)
18. [io_uring — Modern Async I/O](#18-io_uring--modern-async-io)
19. [vsyscall & vDSO — Kernel-Mapped User Pages](#19-vsyscall--vdso--kernel-mapped-user-pages)
20. [Complete Architecture Overview](#20-complete-architecture-overview)
21. [Rust in Linux Kernel Context](#21-rust-in-linux-kernel-context)
22. [Summary Comparison Table](#22-summary-comparison-table)

---

## 1. Mental Model: The Two Worlds

The Linux operating system is split into two fundamental, isolated execution environments:

```
  ┌─────────────────────────────────────────────────────────────────┐
  │                        USER  SPACE                              │
  │                                                                 │
  │   bash   python   nginx   Firefox   your_program   systemd     │
  │                                                                 │
  │   Each process has its own virtual address space.               │
  │   Cannot directly touch hardware.                               │
  │   Cannot read/write other process's memory.                     │
  │   Limited instruction set (no privileged CPU instructions).     │
  └─────────────────────────┬───────────────────────────────────────┘
                             │  <<< HARDWARE BOUNDARY >>>
                             │  Crossing requires a CPU mode switch
                             │  (ring 3 → ring 0)
  ┌─────────────────────────▼───────────────────────────────────────┐
  │                       KERNEL SPACE                              │
  │                                                                 │
  │   Process Scheduler   Memory Manager   VFS   TCP/IP Stack       │
  │   Device Drivers      IPC              IRQ   Security (LSM)     │
  │                                                                 │
  │   Has full access to all physical memory.                       │
  │   Runs privileged CPU instructions (HLT, IN, OUT, CR0...).      │
  │   Manages hardware via drivers.                                 │
  └─────────────────────────┬───────────────────────────────────────┘
                             │
  ┌─────────────────────────▼───────────────────────────────────────┐
  │                        HARDWARE                                 │
  │   CPU   RAM   Disk   NIC   GPU   USB   PCI Bus   Serial Port    │
  └─────────────────────────────────────────────────────────────────┘
```

**Why this separation exists:**

- **Stability** — A buggy user program cannot crash the entire system
- **Security** — Processes cannot spy on each other or hijack hardware
- **Portability** — The kernel ABI abstracts over dozens of hardware architectures
- **Fairness** — The kernel scheduler decides who gets CPU time

---

## 2. CPU Privilege Rings & Protection Model

Modern x86-64 CPUs have four privilege levels called **rings**. Linux only uses ring 0 (kernel) and ring 3 (user):

```
  ┌──────────────────────────────────────────────┐
  │                 x86-64 CPU                   │
  │                                              │
  │  ┌─────────────────────────────────────────┐ │
  │  │            Ring 0 (CPL=0)               │ │
  │  │         KERNEL / OS CODE                │ │
  │  │  Full hardware access. Can read/write   │ │
  │  │  CR0-CR4, MSRs, I/O ports. Can execute │ │
  │  │  HLT, LGDT, LIDT, IN, OUT, WRMSR.      │ │
  │  │                                         │ │
  │  │  ┌───────────────────────────────────┐  │ │
  │  │  │         Ring 1 (unused)           │  │ │
  │  │  │  ┌─────────────────────────────┐  │  │ │
  │  │  │  │      Ring 2 (unused)        │  │  │ │
  │  │  │  │  ┌───────────────────────┐  │  │  │ │
  │  │  │  │  │  Ring 3 (CPL=3)      │  │  │  │ │
  │  │  │  │  │  USER SPACE CODE     │  │  │  │ │
  │  │  │  │  │  No hardware access  │  │  │  │ │
  │  │  │  │  │  No privileged instr │  │  │  │ │
  │  │  │  │  └───────────────────────┘  │  │  │ │
  │  │  │  └─────────────────────────────┘  │  │ │
  │  │  └───────────────────────────────────┘  │ │
  │  └─────────────────────────────────────────┘ │
  └──────────────────────────────────────────────┘
```

### How the Mode Switch Works (x86-64 SYSCALL instruction)

When user code executes `SYSCALL`:

1. CPU saves `RIP` (next instruction) into `RCX`
2. CPU saves `RFLAGS` into `R11`
3. CPU reads the target kernel entry address from `MSR_LSTAR` (set at boot by the kernel)
4. CPU sets CPL to 0 (ring 0)
5. CPU jumps to `entry_SYSCALL_64` in the kernel
6. Kernel runs the syscall handler
7. `SYSRET` instruction reverses: CPL back to 3, RIP restored from RCX

```
  USER SPACE                       KERNEL SPACE
  ──────────────────               ──────────────────────────────────

  mov rax, 1        ──────────►  entry_SYSCALL_64:
  mov rdi, 1          SYSCALL      push regs to kernel stack
  mov rsi, buf                     call do_syscall_64()
  mov rdx, len                       switch(rax):
  syscall                              case 1: sys_write()
  ◄── continues here  SYSRET                → VFS → driver
                                   pop regs
                                   SYSRET
```

### ARM64 Equivalent (used on Raspberry Pi, Apple Silicon, Android phones)

On ARM64, the equivalent mechanism is the `SVC #0` (Supervisor Call) instruction. It triggers a synchronous exception, changing execution level from EL0 (user) to EL1 (kernel).

---

## 3. The System Call Interface — The Master Gateway

Every interaction between user space and the kernel ultimately goes through **system calls** (syscalls). There are ~350 syscalls in Linux (x86-64). They cover:

```
  ┌─────────────────────────────────────────────────────────────────┐
  │                   SYSCALL CATEGORIES                            │
  │                                                                 │
  │  FILE I/O         open, read, write, close, lseek, fsync       │
  │  PROCESSES        fork, execve, wait, exit, clone, kill        │
  │  MEMORY           mmap, munmap, mprotect, brk, madvise         │
  │  NETWORK          socket, bind, connect, send, recv, accept     │
  │  IPC              pipe, socketpair, shmget, msgget, semget      │
  │  TIME             clock_gettime, nanosleep, timerfd_create      │
  │  SIGNALS          sigaction, sigprocmask, rt_sigreturn          │
  │  SECURITY         seccomp, capget, capset, prctl                │
  │  ASYNC I/O        io_uring_setup, io_uring_enter                │
  │  eBPF             bpf()                                         │
  │  DIRECTORIES      getdents64, mkdir, rmdir, rename              │
  └─────────────────────────────────────────────────────────────────┘
```

### Syscall Calling Convention (x86-64)

| Register | Purpose |
|----------|---------|
| `rax` | Syscall number (e.g., 1 = write) |
| `rdi` | 1st argument |
| `rsi` | 2nd argument |
| `rdx` | 3rd argument |
| `r10` | 4th argument |
| `r8`  | 5th argument |
| `r9`  | 6th argument |
| `rax` (return) | Return value (negative = error, maps to errno) |

### Raw Syscall in C (no libc)

```c
// raw_syscall.c
// Demonstrate direct syscall invocation WITHOUT libc
// Compile: gcc -nostdlib -static -o raw_syscall raw_syscall.c

#include <sys/syscall.h>

// Inline assembly to perform a raw syscall
// rax=syscall number, rdi=fd, rsi=buf, rdx=count
static long raw_write(int fd, const void *buf, unsigned long count) {
    long ret;
    __asm__ volatile (
        "syscall"
        : "=a"(ret)                              // output: rax = return value
        : "0"(SYS_write),                        // input: rax = syscall number
          "D"((long)fd),                         // rdi = file descriptor
          "S"(buf),                              // rsi = buffer pointer
          "d"(count)                             // rdx = byte count
        : "rcx", "r11", "memory"                 // clobbered registers
    );
    return ret;
}

static long raw_exit(int code) {
    long ret;
    __asm__ volatile (
        "syscall"
        : "=a"(ret)
        : "0"(SYS_exit_group),
          "D"((long)code)
        : "rcx", "r11"
    );
    return ret;
}

void _start(void) {  // entry point without libc (replaces main)
    const char msg[] = "Hello from raw syscall — no libc!\n";
    raw_write(1, msg, sizeof(msg) - 1);
    raw_exit(0);
}
```

### Kernel-Side Syscall Path (Simplified C)

```c
// Kernel: arch/x86/entry/common.c (simplified concept)

// The SYSCALL instruction jumps here (actual name: entry_SYSCALL_64 in asm)
// After saving registers, the kernel calls:
noinstr void do_syscall_64(struct pt_regs *regs, int nr) {
    // nr = syscall number from rax
    // regs holds all user register values

    // 1. Apply seccomp filters (if any)
    if (unlikely(current->seccomp.mode == SECCOMP_MODE_FILTER))
        nr = seccomp_run_filters(regs);

    // 2. Bounds check
    if (likely(nr < NR_syscalls)) {
        // 3. Lookup in the syscall table
        // sys_call_table is an array of function pointers
        regs->ax = sys_call_table[nr](regs);
    }

    // 4. Handle signals, reschedule if needed
    syscall_exit_to_user_mode(regs);
}

// sys_call_table is defined in arch/x86/entry/syscall_64.c:
// const sys_call_ptr_t sys_call_table[__NR_syscall_max+1] = {
//     [0] = sys_read,
//     [1] = sys_write,
//     [2] = sys_open,
//     ...
// };
```

### Tracing Syscalls with `strace`

```bash
# strace intercepts every syscall a program makes
strace ls /tmp

# Filter by syscall type
strace -e trace=file ls /tmp

# Show timing
strace -T -e trace=read,write cat /etc/hostname

# Attach to running process
strace -p $(pgrep nginx)

# Count syscalls
strace -c ls /
```

---

## 4. C Library (glibc/musl) — The Abstraction Layer

Almost all programs use a C standard library. It wraps raw syscalls with:
- Portable function names (`fopen`, `printf`, `malloc`)
- Error handling (sets `errno`)
- Buffering (stdio buffers I/O for performance)
- Thread safety
- Locale, math, string utilities

```
  ┌─────────────────────────────────────────────────────────────────┐
  │  YOUR PROGRAM                                                   │
  │                                                                 │
  │   printf("hello\n");    fopen("f","r");    malloc(1024);        │
  └────────────────┬───────────────┬──────────────┬────────────────┘
                   │               │              │
  ┌────────────────▼───────────────▼──────────────▼────────────────┐
  │  GLIBC / MUSL / UCLIBC  (C standard library)                   │
  │                                                                 │
  │  printf → write() → sys_write     (buffered, formatted)        │
  │  fopen  → open()  → sys_open      (adds FILE* buffering)       │
  │  malloc → brk()/mmap() → sys_brk  (heap management)           │
  │  pthread_create → clone() → sys_clone                          │
  └─────────────────────────────────────────────────────────────────┘
                   │
  ┌────────────────▼────────────────────────────────────────────────┐
  │  LINUX KERNEL SYSCALL INTERFACE                                 │
  └─────────────────────────────────────────────────────────────────┘
```

### How glibc wraps a syscall (write example)

```c
// glibc's implementation of write() (simplified):
// sysdeps/unix/sysv/linux/write.c

#include <unistd.h>
#include <sysdep.h>    // INLINE_SYSCALL macro

ssize_t __libc_write(int fd, const void *buf, size_t count) {
    // INLINE_SYSCALL expands to inline assembly calling SYSCALL instruction
    return INLINE_SYSCALL(write, 3, fd, buf, count);
    // If kernel returns negative value, glibc negates it and stores in errno
    // Then returns -1 to the caller
}

// weak alias so both write() and __write() resolve to __libc_write
weak_alias(__libc_write, write)
weak_alias(__libc_write, __write)
```

### Checking errno (C)

```c
// errno_demo.c
#include <stdio.h>
#include <errno.h>
#include <string.h>
#include <fcntl.h>

int main(void) {
    // Attempt to open a non-existent file
    int fd = open("/nonexistent_file_xyz", O_RDONLY);
    
    if (fd == -1) {
        // errno is a thread-local variable set by glibc
        // when a syscall returns a negative error code
        printf("errno = %d\n", errno);           // prints: errno = 2
        printf("strerror = %s\n", strerror(errno)); // prints: No such file or directory
        perror("open failed");                    // prints: open failed: No such file or directory
    }

    // Kernel errno codes:
    // EPERM   =  1  (operation not permitted)
    // ENOENT  =  2  (no such file or directory)
    // EACCES  = 13  (permission denied)
    // EEXIST  = 17  (file exists)
    // ENODEV  = 19  (no such device)
    // EINVAL  = 22  (invalid argument)
    // ENOSPC  = 28  (no space left on device)
    
    return 0;
}
```

---

## 5. The Terminal & Shell — How CLI Talks to the Kernel

The terminal is itself a sophisticated abstraction over multiple kernel subsystems.

### The Full Terminal Stack

```
  ┌─────────────────────────────────────────────────────────────────┐
  │  USER TYPES: "ls -la /tmp"                                     │
  └──────────────────────────────┬──────────────────────────────────┘
                                 │ keystrokes
  ┌──────────────────────────────▼──────────────────────────────────┐
  │  TERMINAL EMULATOR (e.g., gnome-terminal, xterm, alacritty)     │
  │  - Renders text on screen using fonts                           │
  │  - Translates key events to escape sequences                    │
  │  - Reads output bytes and renders them as characters/colors     │
  └──────────────────────────────┬──────────────────────────────────┘
                                 │ writes to /dev/pts/N (pseudoterminal)
  ┌──────────────────────────────▼──────────────────────────────────┐
  │  PTY MASTER (kernel: drivers/tty/pty.c)                         │
  │  - One side of the pseudoterminal pair                          │
  │  - Terminal emulator reads/writes here                          │
  └──────────────────────────────┬──────────────────────────────────┘
                                 │ kernel tty layer
  ┌──────────────────────────────▼──────────────────────────────────┐
  │  LINE DISCIPLINE (N_TTY — kernel: drivers/tty/n_tty.c)          │
  │  - Buffers input until Enter is pressed (canonical mode)        │
  │  - Handles Ctrl+C (SIGINT), Ctrl+Z (SIGTSTP), Ctrl+D (EOF)     │
  │  - Handles backspace, line editing in raw terminal              │
  │  - Echo: sends typed chars back to terminal emulator            │
  └──────────────────────────────┬──────────────────────────────────┘
                                 │
  ┌──────────────────────────────▼──────────────────────────────────┐
  │  PTY SLAVE (/dev/pts/0, /dev/pts/1, ...)                        │
  │  - Shell reads keystrokes from here (via read() syscall)        │
  │  - Shell writes output here (via write() syscall)               │
  └──────────────────────────────┬──────────────────────────────────┘
                                 │ file descriptor (stdin/stdout/stderr)
  ┌──────────────────────────────▼──────────────────────────────────┐
  │  SHELL (bash/zsh/fish)                                          │
  │  - Reads command from stdin                                     │
  │  - Parses: "ls" is the command, "-la" and "/tmp" are args       │
  │  - fork() creates a child process                               │
  │  - execve("/bin/ls", ["ls","-la","/tmp"], envp) replaces child  │
  │  - Parent wait()s for child to finish                           │
  └──────────────────────────────┬──────────────────────────────────┘
                                 │ fork + execve syscalls
  ┌──────────────────────────────▼──────────────────────────────────┐
  │  /bin/ls (new process)                                          │
  │  - Calls openat(), getdents64() to read directory               │
  │  - Calls stat() on each file for metadata                       │
  │  - Calls write() to output results                              │
  └─────────────────────────────────────────────────────────────────┘
```

### Shell Command Execution — Deep Dive in C

```c
// mini_shell.c — A minimal shell showing real kernel interactions
// Compile: gcc -o mini_shell mini_shell.c

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <sys/wait.h>
#include <sys/types.h>
#include <errno.h>
#include <fcntl.h>

#define MAX_INPUT  1024
#define MAX_ARGS   64

// Parse space-separated input into argv array
static int parse_args(char *line, char **argv) {
    int argc = 0;
    char *token = strtok(line, " \t\n");
    while (token && argc < MAX_ARGS - 1) {
        argv[argc++] = token;
        token = strtok(NULL, " \t\n");
    }
    argv[argc] = NULL;  // execve requires NULL-terminated array
    return argc;
}

// Handle built-in commands (must run in shell process itself)
static int handle_builtin(char **argv) {
    if (strcmp(argv[0], "cd") == 0) {
        const char *dir = argv[1] ? argv[1] : getenv("HOME");
        if (chdir(dir) != 0)           // sys_chdir — changes kernel CWD
            perror("cd");
        return 1;
    }
    if (strcmp(argv[0], "exit") == 0) {
        exit(argv[1] ? atoi(argv[1]) : 0);
    }
    return 0;  // not a builtin
}

// Execute a pipeline: cmd1 | cmd2
static void exec_pipeline(char **argv1, char **argv2) {
    int pipefd[2];

    // sys_pipe2: creates two file descriptors
    // pipefd[0] = read end, pipefd[1] = write end
    if (pipe(pipefd) == -1) {
        perror("pipe");
        return;
    }

    pid_t pid1 = fork();  // sys_clone (SIGCHLD flag)
    if (pid1 == 0) {
        // Child 1: write into pipe
        close(pipefd[0]);                  // close unused read end
        dup2(pipefd[1], STDOUT_FILENO);    // stdout → pipe write end
        close(pipefd[1]);
        execvp(argv1[0], argv1);
        perror("execvp");
        exit(1);
    }

    pid_t pid2 = fork();
    if (pid2 == 0) {
        // Child 2: read from pipe
        close(pipefd[1]);                  // close unused write end
        dup2(pipefd[0], STDIN_FILENO);     // stdin ← pipe read end
        close(pipefd[0]);
        execvp(argv2[0], argv2);
        perror("execvp");
        exit(1);
    }

    close(pipefd[0]);
    close(pipefd[1]);
    waitpid(pid1, NULL, 0);
    waitpid(pid2, NULL, 0);
}

// Execute a single command (possibly with I/O redirection)
static void execute(char **argv) {
    pid_t pid = fork();   // sys_clone: creates a copy of this process
    if (pid < 0) {
        perror("fork");
        return;
    }

    if (pid == 0) {
        // ─── CHILD PROCESS ───
        // execve replaces this process image entirely:
        // - New code/data/stack from the ELF binary
        // - Inherits open file descriptors (unless FD_CLOEXEC)
        // - Inherits PID, PPID, file descriptor table, signal handlers reset
        execvp(argv[0], argv);

        // execvp only returns on error
        if (errno == ENOENT)
            fprintf(stderr, "%s: command not found\n", argv[0]);
        else
            perror("execvp");
        exit(127);
    }

    // ─── PARENT PROCESS (shell) ───
    int status;
    // sys_wait4: suspends shell until child terminates
    waitpid(pid, &status, 0);

    if (WIFEXITED(status))
        printf("[exit code: %d]\n", WEXITSTATUS(status));
    else if (WIFSIGNALED(status))
        printf("[killed by signal: %d]\n", WTERMSIG(status));
}

int main(void) {
    char input[MAX_INPUT];
    char *argv[MAX_ARGS];

    while (1) {
        // Print prompt showing current directory
        char cwd[256];
        getcwd(cwd, sizeof(cwd));         // sys_getcwd
        printf("%s $ ", cwd);
        fflush(stdout);

        // Read a line (sys_read under the hood, via glibc buffering)
        if (!fgets(input, sizeof(input), stdin)) {
            printf("\n");
            break;  // EOF (Ctrl+D)
        }

        // Strip newline
        input[strcspn(input, "\n")] = '\0';
        if (strlen(input) == 0) continue;

        parse_args(input, argv);
        if (argv[0] == NULL) continue;

        if (!handle_builtin(argv))
            execute(argv);
    }

    return 0;
}
```

### The TTY Line Discipline — Deep Concept

The line discipline is a processing layer in the kernel between the hardware driver and the user-space process. `N_TTY` is the default:

```c
// Simplified concept of what N_TTY does (kernel: drivers/tty/n_tty.c)

// When a character arrives from PTY master (terminal emulator):
void n_tty_receive_char(struct tty_struct *tty, unsigned char c) {
    // Canonical mode (normal typing): buffer until newline
    if (tty->icanon) {
        if (c == '\003') {         // Ctrl+C
            kill_fg_pgrp(tty, SIGINT);
            return;
        }
        if (c == '\032') {         // Ctrl+Z
            kill_fg_pgrp(tty, SIGTSTP);
            return;
        }
        if (c == '\010' || c == '\177') {  // Backspace/DEL
            erase_char(tty);       // remove last char from line buffer
            return;
        }
        // Buffer the character
        put_tty_queue(c, &tty->read_buf);
        if (c == '\n' || c == tty->termios.c_cc[VEOL]) {
            wake_up_interruptible(&tty->read_wait);  // wake the shell
        }
    }
    // Raw mode (e.g., vim, readline): pass every character immediately
    else {
        put_tty_queue(c, &tty->read_buf);
        wake_up_interruptible(&tty->read_wait);
    }
}
```

---

## 6. Virtual Filesystems: /proc, /sys, /dev, /run

Linux exposes kernel data structures as files. This is the **VFS (Virtual Filesystem Switch)** design principle: "everything is a file."

```
  ┌─────────────────────────────────────────────────────────────────┐
  │               VIRTUAL FILESYSTEM (VFS) LAYER                   │
  │                                                                 │
  │  open(), read(), write(), ioctl(), mmap() work on ALL of:      │
  │                                                                 │
  │  ext4/xfs/btrfs  ──── real files on disk                       │
  │  tmpfs            ──── RAM-backed temporary files               │
  │  procfs (/proc)   ──── kernel process/system information        │
  │  sysfs  (/sys)    ──── kernel device/driver model               │
  │  devtmpfs (/dev)  ──── device files                             │
  │  cgroupfs         ──── cgroup resource control                  │
  │  debugfs          ──── kernel debug interface                   │
  │  tracefs          ──── ftrace/perf tracing                      │
  │  FUSE             ──── user-implemented filesystems             │
  │  NFS/SMB          ──── network filesystems                      │
  └─────────────────────────────────────────────────────────────────┘
```

### /proc — Process and System Information

`/proc` is a **procfs** pseudo-filesystem mounted at boot. It has no disk backing; the kernel generates content on-demand when you read from it.

```
  /proc/
  ├── [PID]/                  ← One directory per running process
  │   ├── cmdline             ← Command that started the process
  │   ├── status              ← Process state, memory, UIDs
  │   ├── maps                ← Virtual memory map (address ranges)
  │   ├── smaps               ← Detailed memory map with RSS/PSS
  │   ├── fd/                 ← Symlinks to every open file descriptor
  │   ├── fdinfo/             ← File descriptor metadata (offset, flags)
  │   ├── mem                 ← Process virtual memory (readable with ptrace)
  │   ├── environ             ← Environment variables (NUL-separated)
  │   ├── exe                 → symlink to the executable binary
  │   ├── cwd                 → symlink to current working directory
  │   ├── root                → symlink to root directory (chroot)
  │   ├── stat                ← Scheduling stats (used by ps, top)
  │   ├── io                  ← I/O bytes read/written
  │   ├── limits              ← Resource limits (ulimit)
  │   ├── net/                ← Network state of the process's network namespace
  │   └── task/               ← Per-thread subdirectories
  │       └── [TID]/
  ├── cpuinfo                 ← CPU model, features, caches
  ├── meminfo                 ← RAM usage (total, free, cached, buffers)
  ├── mounts                  ← All mounted filesystems
  ├── net/
  │   ├── dev                 ← Network interface statistics
  │   ├── tcp                 ← TCP connection table
  │   ├── udp                 ← UDP socket table
  │   └── if_inet6            ← IPv6 interface info
  ├── sys/                    ← Writable kernel tuning parameters (sysctl)
  │   ├── kernel/             ← Kernel parameters (hostname, panic, etc.)
  │   ├── net/                ← Network tuning
  │   └── vm/                 ← Virtual memory tuning
  ├── interrupts              ← IRQ counts per CPU
  ├── loadavg                 ← 1/5/15 minute load averages
  ├── uptime                  ← System uptime in seconds
  ├── version                 ← Kernel version string
  └── kallsyms                ← All kernel symbol addresses (if permitted)
```

### Reading /proc in C

```c
// proc_reader.c — Demonstrate reading kernel data from /proc
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <fcntl.h>

// Read a process's memory mappings from /proc/[pid]/maps
void print_memory_map(pid_t pid) {
    char path[64];
    snprintf(path, sizeof(path), "/proc/%d/maps", pid);

    FILE *f = fopen(path, "r");
    if (!f) { perror("fopen"); return; }

    printf("=== Memory Map for PID %d ===\n", pid);
    printf("%-30s %-5s %-5s %-10s %-10s %s\n",
           "Address Range", "Perms", "Offset", "Device", "Inode", "Pathname");

    char line[256];
    while (fgets(line, sizeof(line), f)) {
        // Format: address perms offset dev inode pathname
        // e.g.: 7f1a2b3c0000-7f1a2b3c1000 r-xp 00000000 fd:01 12345 /lib/libc.so
        printf("%s", line);
    }
    fclose(f);
}

// Read a specific /proc/[pid]/status field
char* read_proc_status_field(pid_t pid, const char *field) {
    char path[64];
    snprintf(path, sizeof(path), "/proc/%d/status", pid);

    FILE *f = fopen(path, "r");
    if (!f) return NULL;

    static char value[128];
    char line[256];
    size_t field_len = strlen(field);

    while (fgets(line, sizeof(line), f)) {
        if (strncmp(line, field, field_len) == 0 && line[field_len] == ':') {
            // Extract value after ": "
            char *v = line + field_len + 1;
            while (*v == ' ' || *v == '\t') v++;
            strncpy(value, v, sizeof(value) - 1);
            value[strcspn(value, "\n")] = '\0';
            fclose(f);
            return value;
        }
    }
    fclose(f);
    return NULL;
}

// Read system memory info from /proc/meminfo
void print_meminfo(void) {
    FILE *f = fopen("/proc/meminfo", "r");
    if (!f) { perror("fopen"); return; }

    printf("=== /proc/meminfo ===\n");
    char line[128];
    int count = 0;
    while (fgets(line, sizeof(line), f) && count < 10) {
        printf("%s", line);
        count++;
    }
    fclose(f);
}

// Write to /proc/sys to tune kernel parameter (requires root)
void set_sysctl(const char *param_path, const char *value) {
    // param_path example: "/proc/sys/net/ipv4/ip_forward"
    int fd = open(param_path, O_WRONLY);
    if (fd < 0) { perror("open sysctl"); return; }
    
    if (write(fd, value, strlen(value)) < 0)
        perror("write sysctl");
    else
        printf("Set %s = %s\n", param_path, value);
    
    close(fd);
}

int main(void) {
    pid_t my_pid = getpid();   // sys_getpid

    printf("My PID: %d\n", my_pid);

    // Read our own process name
    char *name = read_proc_status_field(my_pid, "Name");
    if (name) printf("Process name: %s\n", name);

    // Read our VM size
    char *vmsize = read_proc_status_field(my_pid, "VmSize");
    if (vmsize) printf("Virtual memory: %s\n", vmsize);

    // Read number of threads
    char *threads = read_proc_status_field(my_pid, "Threads");
    if (threads) printf("Threads: %s\n", threads);

    print_meminfo();
    print_memory_map(my_pid);

    // Example: enable IP forwarding (requires root)
    // set_sysctl("/proc/sys/net/ipv4/ip_forward", "1");

    return 0;
}
```

### /sys — The sysfs Filesystem

`sysfs` (mounted at `/sys`) exposes the kernel's **device model** as a filesystem. Every device, bus, driver, and class has a directory.

```
  /sys/
  ├── bus/                    ← All buses (pci, usb, platform, i2c, spi...)
  │   ├── pci/devices/        ← PCI devices by bus:slot.func
  │   └── usb/devices/        ← USB devices by bus/port
  ├── class/                  ← Devices grouped by class
  │   ├── net/                ← Network interfaces (eth0, wlan0...)
  │   ├── block/              ← Block devices (sda, nvme0n1...)
  │   └── input/              ← Input devices (keyboard, mouse...)
  ├── devices/                ← The full device tree (mirroring hardware)
  ├── kernel/                 ← Kernel internal parameters
  │   ├── debug/              ← Debugfs mount point sometimes here
  │   └── mm/                 ← Memory management tuning
  └── module/                 ← Loaded kernel modules and their parameters
      └── [modname]/
          └── parameters/     ← Module parameters (read/write)
```

```bash
# Real sysfs interactions:

# Get network interface speed
cat /sys/class/net/eth0/speed

# Get disk model
cat /sys/block/sda/device/model

# Get CPU cache sizes
cat /sys/devices/system/cpu/cpu0/cache/index0/size

# Enable/disable a network interface (root)
echo 0 > /sys/class/net/eth0/carrier

# Read a CPU scaling governor
cat /sys/devices/system/cpu/cpu0/cpufreq/scaling_governor

# Change to performance mode (root)
echo performance > /sys/devices/system/cpu/cpu0/cpufreq/scaling_governor

# Control LED brightness (root)
echo 100 > /sys/class/leds/keyboard-backlight/brightness

# Read PCI device vendor/device IDs
cat /sys/bus/pci/devices/0000:00:1f.2/vendor
cat /sys/bus/pci/devices/0000:00:1f.2/device
```

---

## 7. Device Files & ioctl

Device files in `/dev` are the primary interface to hardware drivers. They come in two types:

```
  CHAR DEVICES (/dev/tty*, /dev/null, /dev/urandom, /dev/input/event*)
  ┌─────────────────────────────────────────────────────────────────┐
  │  Sequential access. Data flows as a stream of bytes.            │
  │  No random seeking (usually). Examples: serial ports, keyboards  │
  └─────────────────────────────────────────────────────────────────┘

  BLOCK DEVICES (/dev/sda, /dev/nvme0n1, /dev/loop0)
  ┌─────────────────────────────────────────────────────────────────┐
  │  Random access in fixed-size blocks.                            │
  │  The kernel's block layer adds caching (page cache).            │
  │  Filesystems are built on top of block devices.                 │
  └─────────────────────────────────────────────────────────────────┘
```

### Device Number System

Every device file has a **major** and **minor** number:

```bash
ls -la /dev/sda /dev/null /dev/tty0
# brw-rw---- 1 root disk    8,  0 ...  /dev/sda    (block, major=8, minor=0)
# crw-rw-rw- 1 root root    1,  3 ...  /dev/null   (char,  major=1, minor=3)
# crw--w---- 1 root tty     4,  0 ...  /dev/tty0   (char,  major=4, minor=0)
```

- **Major number** → selects which driver handles this device
- **Minor number** → identifies which instance (e.g., sda=0, sdb=16, sdc=32)

### ioctl — Device-Specific Control

`ioctl` (I/O Control) is a catch-all syscall for sending commands to a device driver that don't fit the `read`/`write` model.

```
  int ioctl(int fd, unsigned long request, ...);

  fd      = open file descriptor to a device file
  request = command code (driver-specific, encoded with _IO/_IOR/_IOW/_IOWR macros)
  ...     = optional argument (pointer to struct or a plain integer)
```

```c
// ioctl_examples.c — Real ioctl usage examples

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <fcntl.h>
#include <sys/ioctl.h>
#include <linux/fs.h>        // BLKGETSIZE64, BLKSSZGET
#include <linux/input.h>     // input event ioctl
#include <termios.h>         // terminal ioctl (TIOCGWINSZ etc.)
#include <sys/socket.h>
#include <net/if.h>          // SIOCGIFADDR, SIOCGIFFLAGS
#include <arpa/inet.h>

// Example 1: Get terminal window size
void get_terminal_size(void) {
    struct winsize ws;

    // TIOCGWINSZ = "Terminal I/O Control Get WINdow SiZe"
    // This reads from the kernel's TTY driver
    if (ioctl(STDOUT_FILENO, TIOCGWINSZ, &ws) == 0) {
        printf("Terminal: %d rows x %d cols\n", ws.ws_row, ws.ws_col);
        printf("Pixel size: %d x %d\n", ws.ws_xpixel, ws.ws_ypixel);
    }
}

// Example 2: Get block device size
void get_disk_size(const char *dev_path) {
    int fd = open(dev_path, O_RDONLY);
    if (fd < 0) { perror("open"); return; }

    uint64_t size_bytes;
    uint32_t sector_size;

    // BLKGETSIZE64: get total size in bytes
    if (ioctl(fd, BLKGETSIZE64, &size_bytes) == 0)
        printf("%s size: %.2f GB\n", dev_path, size_bytes / 1e9);

    // BLKSSZGET: get physical sector size
    if (ioctl(fd, BLKSSZGET, &sector_size) == 0)
        printf("%s sector size: %u bytes\n", dev_path, sector_size);

    close(fd);
}

// Example 3: Get network interface IP address
void get_interface_ip(const char *ifname) {
    int sockfd = socket(AF_INET, SOCK_DGRAM, 0);
    if (sockfd < 0) { perror("socket"); return; }

    struct ifreq ifr;
    memset(&ifr, 0, sizeof(ifr));
    strncpy(ifr.ifr_name, ifname, IFNAMSIZ - 1);

    // SIOCGIFADDR: "Socket I/O Control Get InterFace ADDRess"
    // Even though it's an ioctl, we use a dummy socket as the fd
    // The kernel's networking subsystem handles this
    if (ioctl(sockfd, SIOCGIFADDR, &ifr) == 0) {
        struct sockaddr_in *addr = (struct sockaddr_in*)&ifr.ifr_addr;
        printf("%s IP: %s\n", ifname, inet_ntoa(addr->sin_addr));
    } else {
        perror("SIOCGIFADDR");
    }

    // SIOCGIFFLAGS: get interface flags (UP, RUNNING, LOOPBACK, etc.)
    if (ioctl(sockfd, SIOCGIFFLAGS, &ifr) == 0) {
        printf("%s flags: ", ifname);
        if (ifr.ifr_flags & IFF_UP)      printf("UP ");
        if (ifr.ifr_flags & IFF_RUNNING) printf("RUNNING ");
        if (ifr.ifr_flags & IFF_LOOPBACK)printf("LOOPBACK ");
        printf("\n");
    }

    close(sockfd);
}

// Example 4: Read input events from /dev/input/event*
void read_input_events(const char *device) {
    int fd = open(device, O_RDONLY);
    if (fd < 0) { perror("open input device"); return; }

    // Get device name
    char name[256];
    ioctl(fd, EVIOCGNAME(sizeof(name)), name);
    printf("Input device: %s\n", name);

    printf("Listening for events (Ctrl+C to stop)...\n");
    struct input_event ev;
    while (read(fd, &ev, sizeof(ev)) == sizeof(ev)) {
        if (ev.type == EV_KEY) {
            printf("Key event: code=%d, value=%d (%s)\n",
                   ev.code, ev.value,
                   ev.value == 1 ? "pressed" :
                   ev.value == 0 ? "released" : "repeat");
        }
    }
    close(fd);
}

int main(void) {
    get_terminal_size();
    get_interface_ip("lo");
    // get_disk_size("/dev/sda");  // requires root or disk group
    // read_input_events("/dev/input/event0");  // requires root or input group
    return 0;
}
```

### Writing a Kernel Module with Custom ioctl

```c
// mydev_driver.c — Simple character device driver with ioctl
// Compile: place in kernel module, build with Makefile

#include <linux/module.h>
#include <linux/fs.h>
#include <linux/uaccess.h>
#include <linux/cdev.h>
#include <linux/ioctl.h>

#define MY_MAGIC  'M'
#define MY_GET    _IOR(MY_MAGIC, 1, int)   // read-direction ioctl
#define MY_SET    _IOW(MY_MAGIC, 2, int)   // write-direction ioctl
#define MY_RESET  _IO(MY_MAGIC, 3)         // no data

static int my_value = 42;
static dev_t dev_num;
static struct cdev my_cdev;

static long my_ioctl(struct file *file, unsigned int cmd, unsigned long arg) {
    int tmp;
    switch (cmd) {
    case MY_GET:
        // copy_to_user: safely copies kernel data to user-space pointer
        // Must NEVER directly dereference user pointers in kernel code!
        if (copy_to_user((int __user *)arg, &my_value, sizeof(int)))
            return -EFAULT;
        return 0;

    case MY_SET:
        // copy_from_user: safely reads from user-space pointer
        if (copy_from_user(&tmp, (int __user *)arg, sizeof(int)))
            return -EFAULT;
        my_value = tmp;
        return 0;

    case MY_RESET:
        my_value = 0;
        return 0;

    default:
        return -ENOTTY;  // "Not a typewriter" — standard for unknown ioctl
    }
}

static struct file_operations my_fops = {
    .owner          = THIS_MODULE,
    .unlocked_ioctl = my_ioctl,
};

static int __init my_init(void) {
    alloc_chrdev_region(&dev_num, 0, 1, "mydev");
    cdev_init(&my_cdev, &my_fops);
    cdev_add(&my_cdev, dev_num, 1);
    pr_info("mydev loaded: major=%d\n", MAJOR(dev_num));
    return 0;
}

static void __exit my_exit(void) {
    cdev_del(&my_cdev);
    unregister_chrdev_region(dev_num, 1);
}

module_init(my_init);
module_exit(my_exit);
MODULE_LICENSE("GPL");
```

---

## 8. Signals — Asynchronous Kernel–User Communication

Signals are the kernel's mechanism for asynchronous notification. The kernel delivers a signal to a process (or thread), interrupting whatever it was doing.

```
  Signal sources:
  ┌──────────────────────────────────────────────────────────────┐
  │  Keyboard (Ctrl+C → SIGINT, Ctrl+Z → SIGTSTP, Ctrl+\ → SIGQUIT) │
  │  kill(2) syscall from another process                        │
  │  kill command (sends signal via kill() syscall)              │
  │  Hardware exceptions (SIGSEGV=segfault, SIGFPE=div-by-zero)  │
  │  Timers (SIGALRM, SIGVTALRM, SIGPROF via alarm/setitimer)    │
  │  Child process state change (SIGCHLD)                        │
  │  I/O readiness (SIGIO/SIGPOLL with O_ASYNC)                  │
  │  Resource limits exceeded (SIGXCPU, SIGXFSZ)                 │
  │  Window resize (SIGWINCH from terminal driver)               │
  └──────────────────────────────────────────────────────────────┘
```

### Signal Delivery Mechanism

```
  Process is running in user space
         │
         │  (interrupt occurs, e.g., kill() from another process,
         │   or hardware exception, or timer expiry)
         │
         ▼
  Kernel marks signal as PENDING in task_struct::pending
         │
         ▼
  At next opportunity (syscall return, interrupt return):
  kernel checks: does this task have pending, unmasked signals?
         │
         ├─── YES ───►  Kernel sets up a signal frame on user stack:
         │              - Saves user registers to signal frame
         │              - Pushes signal number and siginfo
         │              - Sets RIP to the signal handler address
         │              - Returns to user space → handler runs
         │              - Handler calls sigreturn() → kernel restores
         │                original registers, resumes where interrupted
         │
         └─── NO  ───►  Continue normally
```

```c
// signals_demo.c — Comprehensive signal handling
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <signal.h>
#include <sys/signalfd.h>
#include <errno.h>
#include <time.h>

// Global flag for graceful shutdown
static volatile sig_atomic_t running = 1;

// Signal handler (executes in interrupted context — be careful!)
// Rules for signal handlers:
//   - Only call async-signal-safe functions (write, not printf!)
//   - Don't call malloc/free
//   - Don't use non-reentrant globals
//   - Do minimal work; set a flag for the main loop to check
static void sigint_handler(int signum) {
    const char msg[] = "\nCaught SIGINT — shutting down gracefully...\n";
    write(STDOUT_FILENO, msg, sizeof(msg) - 1);  // write() is async-signal-safe
    running = 0;
}

// SIGSEGV handler — show crash info then die
static void sigsegv_handler(int signum, siginfo_t *info, void *ctx) {
    char buf[128];
    int len = snprintf(buf, sizeof(buf),
        "SIGSEGV: fault addr=%p, code=%d\n",
        info->si_addr, info->si_code);
    write(STDERR_FILENO, buf, len);
    // Re-raise with default handler to get core dump
    signal(SIGSEGV, SIG_DFL);
    raise(SIGSEGV);
}

// Using sigaction (preferred over signal())
void setup_signals(void) {
    struct sigaction sa;
    memset(&sa, 0, sizeof(sa));

    // SIGINT handler
    sa.sa_handler = sigint_handler;
    sigemptyset(&sa.sa_mask);
    // SA_RESTART: automatically restart interrupted syscalls (read, write, etc.)
    sa.sa_flags = SA_RESTART;
    sigaction(SIGINT, &sa, NULL);

    // SIGTERM handler (same as SIGINT here)
    sigaction(SIGTERM, &sa, NULL);

    // SIGSEGV handler with extended info
    sa.sa_sigaction = sigsegv_handler;
    sa.sa_flags = SA_SIGINFO | SA_RESETHAND;  // SA_RESETHAND: reset to default after firing
    sigaction(SIGSEGV, &sa, NULL);

    // Ignore SIGPIPE (happens when writing to a closed pipe/socket)
    sa.sa_handler = SIG_IGN;
    sa.sa_flags = 0;
    sigaction(SIGPIPE, &sa, NULL);
}

// Modern approach: signalfd — receive signals as file descriptor events
void demo_signalfd(void) {
    sigset_t mask;
    sigemptyset(&mask);
    sigaddset(&mask, SIGUSR1);
    sigaddset(&mask, SIGUSR2);

    // Block these signals from normal delivery so signalfd gets them
    sigprocmask(SIG_BLOCK, &mask, NULL);

    // Create a file descriptor that delivers signals as readable data
    int sfd = signalfd(-1, &mask, SFD_NONBLOCK | SFD_CLOEXEC);
    if (sfd < 0) { perror("signalfd"); return; }

    printf("Send SIGUSR1 (%d) or SIGUSR2 (%d) to PID %d\n",
           SIGUSR1, SIGUSR2, getpid());

    // Now you can poll/select/epoll on sfd — integrate with event loop!
    struct signalfd_siginfo fdsi;
    ssize_t s = read(sfd, &fdsi, sizeof(fdsi));  // blocks until signal arrives
    if (s == sizeof(fdsi)) {
        printf("Got signal %u from PID %d\n",
               fdsi.ssi_signo, fdsi.ssi_pid);
    }
    close(sfd);
}

int main(void) {
    setup_signals();

    printf("PID=%d running. Press Ctrl+C or send SIGINT to stop.\n", getpid());

    // Send ourselves a SIGUSR1 after 2 seconds
    alarm(2);  // SIGALRM in 2 seconds

    struct sigaction sa_alrm = { .sa_handler = SIG_IGN };
    sigaction(SIGALRM, &sa_alrm, NULL);

    while (running) {
        printf("Tick...\n");
        sleep(1);
    }

    printf("Exited cleanly.\n");
    return 0;
}
```

---

## 9. Pipes & FIFOs

Pipes are the original Unix IPC mechanism — a one-directional byte stream with a kernel buffer.

```
  ┌──────────┐   write(pipefd[1])   ┌──────────────────┐   read(pipefd[0])   ┌──────────┐
  │ Process A│ ──────────────────►  │  KERNEL PIPE BUF  │ ──────────────────► │ Process B│
  │  (writer)│                      │  (64KB default)   │                     │  (reader)│
  └──────────┘                      └──────────────────┘                      └──────────┘

  Properties:
  - Blocking: write blocks when buffer full; read blocks when buffer empty
  - Anonymous pipes: only between related processes (parent/child via fork)
  - Named pipes (FIFOs): any processes can open /path/to/fifo
  - Kernel manages synchronization: no mutexes needed
  - POSIX: writes ≤ PIPE_BUF (4096 bytes) are atomic
```

```c
// pipes_demo.c — Pipes, FIFOs and splice()
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <fcntl.h>
#include <sys/wait.h>
#include <sys/stat.h>

// Demo 1: Anonymous pipe between parent and child
void demo_anonymous_pipe(void) {
    int pipefd[2];
    pipe2(pipefd, O_CLOEXEC);  // sys_pipe2 with flags

    pid_t pid = fork();
    if (pid == 0) {
        // Child reads
        close(pipefd[1]);  // close write end
        char buf[128];
        ssize_t n = read(pipefd[0], buf, sizeof(buf) - 1);
        buf[n] = '\0';
        printf("Child received: %s\n", buf);
        close(pipefd[0]);
        exit(0);
    } else {
        // Parent writes
        close(pipefd[0]);  // close read end
        const char *msg = "Hello from parent via kernel pipe buffer!";
        write(pipefd[1], msg, strlen(msg));
        close(pipefd[1]);  // EOF signal to child
        wait(NULL);
    }
}

// Demo 2: Named pipe (FIFO) — between unrelated processes
void demo_fifo_writer(void) {
    const char *fifo_path = "/tmp/my_fifo";
    mkfifo(fifo_path, 0666);  // create FIFO file (like mknod with S_IFIFO)

    printf("Writer: opening FIFO (will block until reader opens)...\n");
    int fd = open(fifo_path, O_WRONLY);  // blocks until someone opens for reading
    write(fd, "data via FIFO\n", 14);
    close(fd);
    unlink(fifo_path);
}

// Demo 3: splice() — zero-copy data transfer between file descriptors
// splice moves data through the kernel pipe buffer WITHOUT copying to userspace
void demo_splice_copy(const char *src, const char *dst) {
    int src_fd = open(src, O_RDONLY);
    int dst_fd = open(dst, O_WRONLY | O_CREAT | O_TRUNC, 0644);
    int pipefd[2];
    pipe(pipefd);

    off_t offset = 0;
    ssize_t total = 0;
    ssize_t spliced;

    while (1) {
        // splice from src into pipe (kernel → pipe buffer)
        spliced = splice(src_fd, NULL, pipefd[1], NULL, 65536,
                         SPLICE_F_MOVE | SPLICE_F_MORE);
        if (spliced <= 0) break;

        // splice from pipe into dst (pipe buffer → dst, zero-copy)
        splice(pipefd[0], NULL, dst_fd, NULL, spliced, SPLICE_F_MOVE);
        total += spliced;
    }

    printf("splice: copied %zd bytes with zero userspace copies\n", total);
    close(pipefd[0]); close(pipefd[1]);
    close(src_fd); close(dst_fd);
}

int main(void) {
    demo_anonymous_pipe();
    return 0;
}
```

---

## 10. Sockets — Unix Domain & Netlink

### Unix Domain Sockets

Unix domain sockets provide IPC between processes on the same machine using the filesystem namespace. They are significantly faster than TCP/IP loopback because there's no network stack overhead.

```c
// unix_socket_server.c — Stream socket server using AF_UNIX
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <sys/socket.h>
#include <sys/un.h>

#define SOCKET_PATH "/tmp/my_unix_socket"

void run_server(void) {
    // AF_UNIX = Unix domain socket (not network)
    // SOCK_STREAM = reliable, ordered, bidirectional byte stream
    int server_fd = socket(AF_UNIX, SOCK_STREAM, 0);

    struct sockaddr_un addr;
    memset(&addr, 0, sizeof(addr));
    addr.sun_family = AF_UNIX;
    strncpy(addr.sun_path, SOCKET_PATH, sizeof(addr.sun_path) - 1);

    unlink(SOCKET_PATH);  // remove stale socket file
    bind(server_fd, (struct sockaddr*)&addr, sizeof(addr));
    listen(server_fd, 5);

    printf("Server listening on %s\n", SOCKET_PATH);

    while (1) {
        int client_fd = accept(server_fd, NULL, NULL);
        char buf[256];
        ssize_t n = read(client_fd, buf, sizeof(buf) - 1);
        buf[n] = '\0';
        printf("Received: %s\n", buf);

        const char *response = "ACK";
        write(client_fd, response, strlen(response));
        close(client_fd);
    }
}
```

### Netlink Sockets — The Kernel's Native IPC Protocol

**Netlink** is a socket family (`AF_NETLINK`) designed specifically for communication between user space and the kernel. It is used by:

- `ip`, `ss`, `tc` commands (iproute2)
- `NetworkManager`, `systemd-networkd`
- Audit daemon (`auditd`)
- udev (device management)
- SELinux/AppArmor policy loading

```
  ┌──────────────────────────────────────────────────────────────────┐
  │  USER SPACE                                                      │
  │  iproute2 (ip route add ...)                                     │
  │  ss (socket statistics)                                          │
  │  NetworkManager                                                  │
  └────────────────────────┬─────────────────────────────────────────┘
                           │ AF_NETLINK socket
                           │ sendmsg() / recvmsg()
  ┌────────────────────────▼─────────────────────────────────────────┐
  │  KERNEL NETLINK SUBSYSTEM                                        │
  │                                                                  │
  │  NETLINK_ROUTE    → routing table, ARP, interfaces               │
  │  NETLINK_AUDIT    → Linux Audit framework                        │
  │  NETLINK_KOBJECT_UEVENT → udev device events                     │
  │  NETLINK_NETFILTER → iptables/nftables configuration             │
  │  NETLINK_GENERIC  → extensible (used by nl80211, taskstats...)   │
  └─────────────────────────────────────────────────────────────────┘
```

```c
// netlink_routes.c — List kernel routing table via Netlink
// Shows how tools like 'ip route' actually work
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <sys/socket.h>
#include <linux/netlink.h>
#include <linux/rtnetlink.h>
#include <arpa/inet.h>
#include <net/if.h>

int main(void) {
    // Open a netlink socket for routing messages
    int sock = socket(AF_NETLINK, SOCK_RAW, NETLINK_ROUTE);
    if (sock < 0) { perror("socket"); return 1; }

    // Bind to kernel
    struct sockaddr_nl src_addr = {
        .nl_family = AF_NETLINK,
        .nl_pid    = getpid(),   // our PID as the "address"
        .nl_groups = 0
    };
    bind(sock, (struct sockaddr*)&src_addr, sizeof(src_addr));

    // Build request: RTM_GETROUTE = dump the routing table
    struct {
        struct nlmsghdr  nlh;
        struct rtmsg     rtm;
    } req;
    memset(&req, 0, sizeof(req));

    req.nlh.nlmsg_len   = NLMSG_LENGTH(sizeof(struct rtmsg));
    req.nlh.nlmsg_type  = RTM_GETROUTE;
    req.nlh.nlmsg_flags = NLM_F_REQUEST | NLM_F_DUMP;
    req.nlh.nlmsg_seq   = 1;
    req.nlh.nlmsg_pid   = getpid();

    req.rtm.rtm_family  = AF_INET;
    req.rtm.rtm_table   = RT_TABLE_MAIN;

    // Send request to kernel
    struct sockaddr_nl dest = { .nl_family = AF_NETLINK };
    sendto(sock, &req, req.nlh.nlmsg_len, 0,
           (struct sockaddr*)&dest, sizeof(dest));

    // Receive and parse responses
    char buf[8192];
    printf("%-20s %-20s %-10s %s\n",
           "Destination", "Gateway", "Interface", "Flags");
    printf("%-20s %-20s %-10s %s\n",
           "───────────", "───────", "─────────", "─────");

    while (1) {
        ssize_t len = recv(sock, buf, sizeof(buf), 0);
        struct nlmsghdr *nlh = (struct nlmsghdr*)buf;

        for (; NLMSG_OK(nlh, len); nlh = NLMSG_NEXT(nlh, len)) {
            if (nlh->nlmsg_type == NLMSG_DONE) goto done;
            if (nlh->nlmsg_type == NLMSG_ERROR) { fprintf(stderr, "Error\n"); goto done; }
            if (nlh->nlmsg_type != RTM_NEWROUTE) continue;

            struct rtmsg *rtm = NLMSG_DATA(nlh);
            if (rtm->rtm_family != AF_INET) continue;

            struct in_addr dst = {0}, gw = {0};
            char ifname[IF_NAMESIZE] = "?";
            int rtlen = RTM_PAYLOAD(nlh);

            for (struct rtattr *rta = RTM_RTA(rtm);
                 RTA_OK(rta, rtlen);
                 rta = RTA_NEXT(rta, rtlen)) {
                switch (rta->rta_type) {
                case RTA_DST:     memcpy(&dst, RTA_DATA(rta), 4); break;
                case RTA_GATEWAY: memcpy(&gw,  RTA_DATA(rta), 4); break;
                case RTA_OIF:
                    if_indextoname(*(int*)RTA_DATA(rta), ifname); break;
                }
            }

            char dst_str[32], gw_str[32];
            if (dst.s_addr)
                snprintf(dst_str, sizeof(dst_str), "%s/%d",
                         inet_ntoa(dst), rtm->rtm_dst_len);
            else
                strcpy(dst_str, "default");

            strcpy(gw_str, gw.s_addr ? inet_ntoa(gw) : "-");

            printf("%-20s %-20s %-10s\n", dst_str, gw_str, ifname);
        }
    }
done:
    close(sock);
    return 0;
}
```

---

## 11. Memory-Mapped I/O (mmap)

`mmap` maps a file or device memory directly into a process's virtual address space. After mapping, you access the data with pointer loads/stores — no `read`/`write` syscalls needed.

```
  ┌──────────────────────────────────────────────────────────────────┐
  │  VIRTUAL ADDRESS SPACE of your process                          │
  │                                                                  │
  │  0x0000000000000000                                             │
  │  ├── text segment (.text)    ELF code                           │
  │  ├── data segment (.data)    initialized globals                │
  │  ├── BSS segment (.bss)      zero-initialized globals           │
  │  ├── heap (brk/mmap)         malloc arena                       │
  │  │                                                              │
  │  ├── MMAP REGION ──────────────────────────────────────────┐    │
  │  │   0x7f000000: /lib/libc.so.6 (file-backed, shared)     │    │
  │  │   0x7f100000: /etc/passwd (file-backed, private copy)   │    │
  │  │   0x7f200000: anonymous mmap (private RAM for malloc)   │    │
  │  │   0x7f300000: /dev/mem or device MMIO registers         │    │
  │  └────────────────────────────────────────────────────────────┘  │
  │  ├── stack                   grows downward                     │
  │  └── 0xffffffffffffffff                                         │
  └──────────────────────────────────────────────────────────────────┘

  When you ACCESS a mapped page for the first time:
  - CPU generates a page fault (no physical page assigned yet)
  - Kernel's page fault handler runs
  - For file-backed: reads page from disk into page cache
  - Maps physical frame into page table
  - Returns to user code — transparent!
```

```c
// mmap_demo.c — mmap for file I/O, shared memory, and huge pages
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <fcntl.h>
#include <sys/mman.h>
#include <sys/stat.h>

// Example 1: Fast file reading via mmap
void mmap_read_file(const char *path) {
    int fd = open(path, O_RDONLY);
    if (fd < 0) { perror("open"); return; }

    struct stat st;
    fstat(fd, &st);         // get file size
    size_t size = st.st_size;

    // MAP_SHARED: writes would go to the file
    // MAP_PRIVATE: copy-on-write; writes don't affect file
    // PROT_READ: only read access
    char *data = mmap(NULL, size, PROT_READ, MAP_PRIVATE, fd, 0);
    if (data == MAP_FAILED) { perror("mmap"); close(fd); return; }

    // advise the kernel about access pattern (improves prefetching)
    madvise(data, size, MADV_SEQUENTIAL);

    // Access file content directly via pointer — no read() needed!
    printf("First 80 chars of %s:\n%.80s\n", path, data);

    // Count newlines without any extra copies
    size_t lines = 0;
    for (size_t i = 0; i < size; i++)
        if (data[i] == '\n') lines++;
    printf("Lines: %zu\n", lines);

    munmap(data, size);   // release mapping
    close(fd);
}

// Example 2: POSIX shared memory — fastest IPC between processes
// No kernel copies: both processes see the same physical pages
void demo_shared_memory_writer(void) {
    // Create a named shared memory object
    int shm_fd = shm_open("/my_shared_mem", O_CREAT | O_RDWR, 0666);
    ftruncate(shm_fd, 4096);  // set size

    // Map it
    void *ptr = mmap(NULL, 4096, PROT_READ | PROT_WRITE, MAP_SHARED, shm_fd, 0);
    strcpy((char*)ptr, "Hello from shared memory!");

    printf("Written to shared memory. Run reader to verify.\n");
    sleep(5);  // keep alive for reader

    munmap(ptr, 4096);
    close(shm_fd);
    shm_unlink("/my_shared_mem");  // remove the shm object
}

// Example 3: Anonymous private mapping — fast bulk allocation
void demo_anonymous_mmap(void) {
    size_t size = 100 * 1024 * 1024;  // 100MB

    // MAP_ANONYMOUS: not backed by a file (pure RAM)
    // MAP_PRIVATE: not shared with anyone
    // Linux uses lazy allocation: physical pages assigned on first access
    char *buf = mmap(NULL, size, PROT_READ | PROT_WRITE,
                     MAP_PRIVATE | MAP_ANONYMOUS, -1, 0);
    if (buf == MAP_FAILED) { perror("mmap"); return; }

    // madvise for huge pages (reduces TLB pressure for large allocs)
    madvise(buf, size, MADV_HUGEPAGE);

    // mlock: prevent the kernel from swapping these pages out
    // mlock(buf, size);  // requires CAP_IPC_LOCK or small enough RLIMIT_MEMLOCK

    printf("Mapped %zu MB at %p\n", size / 1024 / 1024, (void*)buf);
    memset(buf, 0, size);  // touch all pages (triggers page faults)
    printf("All pages faulted in.\n");

    munmap(buf, size);
}

// Example 4: mmap to talk to a hardware device
// (Conceptual — actual device depends on hardware)
void demo_device_mmap(const char *dev_path, off_t offset, size_t size) {
    int fd = open(dev_path, O_RDWR);
    if (fd < 0) { perror("open device"); return; }

    // Map device memory-mapped I/O registers into our address space
    // Writing to these pointers directly controls hardware!
    volatile uint32_t *regs = mmap(NULL, size,
                                   PROT_READ | PROT_WRITE,
                                   MAP_SHARED, fd, offset);
    if (regs == MAP_FAILED) { perror("mmap device"); close(fd); return; }

    // Read/write hardware registers via pointer dereference
    uint32_t status = regs[0];          // read register 0
    regs[1] = 0x00000001;              // write register 1 (e.g., enable bit)

    printf("Device register[0] = 0x%08x\n", status);

    munmap((void*)regs, size);
    close(fd);
}

int main(void) {
    mmap_read_file("/etc/hostname");
    demo_anonymous_mmap();
    return 0;
}
```

---

## 12. eBPF — Programmable Kernel Extension

**eBPF** (extended Berkeley Packet Filter) is one of the most powerful modern mechanisms for interacting with the kernel. It allows you to load small, sandboxed programs into the kernel that run in response to events — without modifying kernel source or loading a full kernel module.

```
  ┌─────────────────────────────────────────────────────────────────┐
  │  USER SPACE                                                     │
  │  Write eBPF program (C restricted subset, or Rust)             │
  │  Compile with clang -target bpf → BPF bytecode (.o)            │
  │  Use libbpf or bpftool to load                                 │
  └─────────────────────────────────────────────────────────────────┘
                             │
                             │ bpf() syscall (BPF_PROG_LOAD)
                             ▼
  ┌─────────────────────────────────────────────────────────────────┐
  │  KERNEL: eBPF VERIFIER                                          │
  │  - Statically proves the program is safe:                       │
  │    • No infinite loops                                          │
  │    • No out-of-bounds memory access                             │
  │    • No calling arbitrary kernel functions                      │
  │    • Stack depth bounded                                        │
  │  - JIT compiles to native machine code                          │
  └─────────────────────────────────────────────────────────────────┘
                             │ attached to a hook point
  ┌──────────────────────────▼──────────────────────────────────────┐
  │  eBPF HOOK POINTS                                               │
  │                                                                 │
  │  Tracing:     kprobe/kretprobe (any kernel function)            │
  │               uprobe/uretprobe (any user function)              │
  │               tracepoint (static kernel trace points)           │
  │               perf events                                       │
  │                                                                 │
  │  Networking:  XDP (eXpress Data Path — before skb allocation)   │
  │               tc (traffic control — ingress/egress)             │
  │               socket filter                                     │
  │               cgroup socket (control per-cgroup)                │
  │               sk_msg (socket message interception)              │
  │                                                                 │
  │  Security:    LSM hooks (seccomp alternative)                   │
  │               bpf_lsm                                           │
  └─────────────────────────────────────────────────────────────────┘
                             │
  ┌──────────────────────────▼──────────────────────────────────────┐
  │  eBPF MAPS — Shared kernel-user data structures                 │
  │                                                                 │
  │  Hash map, Array, Ring buffer, LRU, Per-CPU, Queue, Stack...    │
  │  eBPF program reads/writes maps.                                │
  │  User space reads/writes the same maps via bpf() syscall.       │
  └─────────────────────────────────────────────────────────────────┘
```

### eBPF Program (C kernel-side) + User-Space Loader

```c
// trace_open.bpf.c — eBPF program: trace every open() syscall
// Compile: clang -O2 -target bpf -c trace_open.bpf.c -o trace_open.bpf.o

#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

// Ringbuf map: efficient kernel→user data transfer
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 1 << 20);  // 1MB ring buffer
} events SEC(".maps");

struct event {
    __u32 pid;
    __u32 uid;
    char  filename[256];
};

// SEC macro attaches this function to the sys_enter_openat tracepoint
SEC("tracepoint/syscalls/sys_enter_openat")
int trace_openat(struct trace_event_raw_sys_enter *ctx) {
    struct event *e;

    // Reserve space in ring buffer
    e = bpf_ringbuf_reserve(&events, sizeof(*e), 0);
    if (!e) return 0;

    e->pid = bpf_get_current_pid_tgid() >> 32;
    e->uid = bpf_get_current_uid_gid() & 0xFFFFFFFF;

    // bpf_probe_read_user_str: safely read a string from user-space pointer
    // (ctx->args[1] = filename argument of openat)
    bpf_probe_read_user_str(e->filename, sizeof(e->filename),
                             (const char *)(long)ctx->args[1]);

    bpf_ringbuf_submit(e, 0);
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
```

```c
// trace_open_user.c — User-space loader and reader
// Link with: -lbpf -lelf -lz
#include <stdio.h>
#include <unistd.h>
#include <bpf/libbpf.h>
#include <bpf/bpf.h>

struct event { uint32_t pid, uid; char filename[256]; };

static int handle_event(void *ctx, void *data, size_t sz) {
    struct event *e = data;
    printf("PID=%-6u UID=%-5u open(\"%s\")\n", e->pid, e->uid, e->filename);
    return 0;
}

int main(void) {
    // Load and verify the BPF object
    struct bpf_object *obj = bpf_object__open("trace_open.bpf.o");
    bpf_object__load(obj);  // JIT-compiles and loads into kernel

    // Find and attach the tracepoint program
    struct bpf_program *prog = bpf_object__find_program_by_name(obj, "trace_openat");
    struct bpf_link *link = bpf_program__attach(prog);

    // Find the ring buffer map
    struct bpf_map *map = bpf_object__find_map_by_name(obj, "events");
    int map_fd = bpf_map__fd(map);

    // Poll the ring buffer for events
    struct ring_buffer *rb = ring_buffer__new(map_fd, handle_event, NULL, NULL);
    printf("Tracing open() calls... Ctrl+C to stop.\n");
    while (1) ring_buffer__poll(rb, 100);

    ring_buffer__free(rb);
    bpf_link__destroy(link);
    bpf_object__close(obj);
    return 0;
}
```

---

## 13. FUSE — Filesystem in Userspace

FUSE lets you implement a complete filesystem in user space. The kernel's FUSE driver forwards VFS operations to your daemon.

```
  User: open("/mnt/myfs/file.txt")
         │
         ▼
  VFS layer in kernel
         │
         ▼  (mount point belongs to FUSE)
  FUSE kernel module
         │ sends request via /dev/fuse
         ▼
  YOUR USER-SPACE DAEMON
  (implements read, write, readdir, getattr, etc.)
         │ sends response back via /dev/fuse
         ▼
  FUSE kernel module → VFS → returns data to calling process
```

```c
// hello_fuse.c — Minimal FUSE filesystem that has one file
// Compile: gcc -o hello_fuse hello_fuse.c $(pkg-config fuse3 --cflags --libs)
// Mount:   ./hello_fuse /mnt/test
// Unmount: fusermount3 -u /mnt/test

#define FUSE_USE_VERSION 31
#include <fuse3/fuse.h>
#include <string.h>
#include <errno.h>

static const char *hello_content = "Hello from FUSE userspace filesystem!\n";
static const char *hello_path    = "/hello.txt";

// Called when ls or stat is run
static int hello_getattr(const char *path, struct stat *stbuf,
                          struct fuse_file_info *fi) {
    memset(stbuf, 0, sizeof(*stbuf));
    if (strcmp(path, "/") == 0) {
        stbuf->st_mode  = S_IFDIR | 0755;
        stbuf->st_nlink = 2;
        return 0;
    }
    if (strcmp(path, hello_path) == 0) {
        stbuf->st_mode  = S_IFREG | 0444;
        stbuf->st_nlink = 1;
        stbuf->st_size  = strlen(hello_content);
        return 0;
    }
    return -ENOENT;
}

// Called when ls reads a directory
static int hello_readdir(const char *path, void *buf,
                          fuse_fill_dir_t filler,
                          off_t offset, struct fuse_file_info *fi,
                          enum fuse_readdir_flags flags) {
    if (strcmp(path, "/") != 0) return -ENOENT;
    filler(buf, ".",        NULL, 0, 0);
    filler(buf, "..",       NULL, 0, 0);
    filler(buf, "hello.txt", NULL, 0, 0);
    return 0;
}

// Called when reading file content
static int hello_read(const char *path, char *buf, size_t size,
                       off_t offset, struct fuse_file_info *fi) {
    if (strcmp(path, hello_path) != 0) return -ENOENT;
    size_t len = strlen(hello_content);
    if (offset >= (off_t)len) return 0;
    if (offset + size > len) size = len - offset;
    memcpy(buf, hello_content + offset, size);
    return size;
}

static const struct fuse_operations hello_ops = {
    .getattr = hello_getattr,
    .readdir = hello_readdir,
    .read    = hello_read,
};

int main(int argc, char *argv[]) {
    return fuse_main(argc, argv, &hello_ops, NULL);
}
```

---

## 14. Kernel Modules & /dev Interface

Kernel modules are compiled code loaded at runtime into the kernel. They can add filesystem support, device drivers, network protocols, and more.

```bash
# Module management commands
lsmod                         # list loaded modules (reads /proc/modules)
modinfo e1000e                # show module metadata
insmod ./mymodule.ko          # load module from file
rmmod mymodule                # unload module
modprobe e1000e               # load with dependencies resolved
modprobe -r e1000e            # remove with unused dependencies

# Module parameters
modprobe usbcore autosuspend=0  # pass parameter at load time
echo 0 > /sys/module/usbcore/parameters/autosuspend  # change at runtime

# Where modules live
ls /lib/modules/$(uname -r)/   # module files
cat /proc/modules               # runtime loaded modules
```

### Complete Kernel Module with procfs interface

```c
// procfs_module.c — Kernel module that exposes data via /proc
// Build with Makefile:
//   obj-m := procfs_module.o
//   all:
//       make -C /lib/modules/$(shell uname -r)/build M=$(PWD) modules

#include <linux/module.h>
#include <linux/proc_fs.h>
#include <linux/seq_file.h>
#include <linux/uaccess.h>
#include <linux/slab.h>
#include <linux/timer.h>
#include <linux/jiffies.h>

MODULE_LICENSE("GPL");
MODULE_AUTHOR("Your Name");
MODULE_DESCRIPTION("Demo: procfs + timer + kernel stats");

static struct proc_dir_entry *proc_entry;
static unsigned long access_count = 0;
static ktime_t load_time;

// This function generates the /proc/mymodule content
// seq_file handles large outputs that don't fit in one read()
static int show_stats(struct seq_file *m, void *v) {
    ktime_t now = ktime_get();
    s64 uptime_ms = ktime_to_ms(ktime_sub(now, load_time));

    seq_printf(m, "=== mymodule stats ===\n");
    seq_printf(m, "Module uptime:   %lld ms\n", uptime_ms);
    seq_printf(m, "Access count:    %lu\n", ++access_count);
    seq_printf(m, "Kernel version:  %s\n", utsname()->release);
    seq_printf(m, "Jiffies (HZ=%d): %lu\n", HZ, jiffies);
    seq_printf(m, "Total RAM:       %lu KB\n",
               totalram_pages() * PAGE_SIZE / 1024);
    seq_printf(m, "Free RAM:        %lu KB\n",
               global_zone_page_state(NR_FREE_PAGES) * PAGE_SIZE / 1024);

    return 0;
}

static int mymod_open(struct inode *inode, struct file *file) {
    return single_open(file, show_stats, NULL);
}

static const struct proc_ops mymod_fops = {
    .proc_open    = mymod_open,
    .proc_read    = seq_read,
    .proc_lseek   = seq_lseek,
    .proc_release = single_release,
};

static int __init mymod_init(void) {
    load_time = ktime_get();
    proc_entry = proc_create("mymodule", 0444, NULL, &mymod_fops);
    if (!proc_entry) {
        pr_err("mymodule: failed to create /proc/mymodule\n");
        return -ENOMEM;
    }
    pr_info("mymodule: loaded. Try: cat /proc/mymodule\n");
    return 0;
}

static void __exit mymod_exit(void) {
    proc_remove(proc_entry);
    pr_info("mymodule: unloaded\n");
}

module_init(mymod_init);
module_exit(mymod_exit);
```

---

## 15. Uevents & udev — Hotplug & Device Management

When hardware is connected/disconnected, the kernel sends **uevents** via Netlink to user space. `udev` (part of systemd) listens and creates device nodes, runs rules, and notifies applications.

```
  ┌────────────────────────────────────────────────────────────────┐
  │  USB device plugged in                                         │
  └───────────────┬────────────────────────────────────────────────┘
                  │ USB bus detects device
                  ▼
  ┌────────────────────────────────────────────────────────────────┐
  │  USB SUBSYSTEM (drivers/usb/core/)                             │
  │  - Enumerates device (vendor ID, product ID, class)            │
  │  - Loads matching driver from drivers/usb/...                  │
  │  - Registers device in sysfs (/sys/bus/usb/devices/)           │
  └───────────────┬────────────────────────────────────────────────┘
                  │ kobject_uevent(kobj, KOBJ_ADD)
                  ▼
  ┌────────────────────────────────────────────────────────────────┐
  │  NETLINK KOBJECT_UEVENT socket (broadcast to all listeners)    │
  │  Payload: ACTION=add SUBSYSTEM=usb DEVTYPE=usb_device          │
  │           DEVNAME=bus/usb/001/002 MAJOR=189 MINOR=1            │
  └───────────────┬────────────────────────────────────────────────┘
                  │ AF_NETLINK NETLINK_KOBJECT_UEVENT
                  ▼
  ┌────────────────────────────────────────────────────────────────┐
  │  systemd-udevd (listens on netlink socket)                     │
  │  - Reads /etc/udev/rules.d/*.rules                             │
  │  - Matches device by subsystem, vendor, product IDs            │
  │  - Runs: mknod /dev/bus/usb/001/002 c 189 1                    │
  │  - Sets permissions, ownership                                 │
  │  - Creates symlinks: /dev/disk/by-uuid/, /dev/disk/by-label/   │
  │  - Runs custom scripts if rules specify RUN+=                  │
  └────────────────────────────────────────────────────────────────┘
```

```c
// listen_uevents.c — Listen to kernel device events directly
#include <stdio.h>
#include <string.h>
#include <unistd.h>
#include <sys/socket.h>
#include <linux/netlink.h>

int main(void) {
    int sock = socket(AF_NETLINK, SOCK_RAW, NETLINK_KOBJECT_UEVENT);

    struct sockaddr_nl addr = {
        .nl_family = AF_NETLINK,
        .nl_pid    = getpid(),
        .nl_groups = 1  // group 1 = all kernel uevent broadcasts
    };
    bind(sock, (struct sockaddr*)&addr, sizeof(addr));

    printf("Listening for kernel uevents (plug/unplug devices)...\n");

    char buf[4096];
    while (1) {
        ssize_t len = recv(sock, buf, sizeof(buf) - 1, 0);
        if (len <= 0) continue;
        buf[len] = '\0';

        // Uevents are null-separated key=value strings
        printf("─── uevent ───────────────────\n");
        char *p = buf;
        while (p < buf + len) {
            printf("  %s\n", p);
            p += strlen(p) + 1;
        }
    }

    close(sock);
    return 0;
}
```

---

## 16. Audit Subsystem

The Linux Audit subsystem records system events for security compliance (PCI-DSS, HIPAA, etc.).

```bash
# Enable auditing of file access
auditctl -w /etc/passwd -p rwa -k passwd_watch

# Audit all execve() calls by a specific user
auditctl -a always,exit -F arch=b64 -S execve -F uid=1000 -k user_exec

# View audit log
ausearch -k passwd_watch
aureport --summary
```

---

## 17. perf & ftrace — Performance & Tracing Interfaces

### perf

`perf` uses kernel performance counters (via the `perf_event_open()` syscall) to measure hardware and software events.

```bash
# Count CPU events during a run
perf stat ls /

# Sample which functions use the most CPU (statistical profiling)
perf record -g ./my_program    # record with call graph
perf report                    # interactive TUI

# Trace specific events
perf trace ls /              # like strace but lower overhead

# Count branch mispredictions
perf stat -e branch-misses,branches ./my_program

# Measure cache misses
perf stat -e cache-misses,cache-references ./my_program
```

### ftrace — Function Tracer

ftrace is a kernel built-in tracer accessible through `tracefs` (mounted at `/sys/kernel/tracing` or `/sys/kernel/debug/tracing`):

```bash
# List available tracers
cat /sys/kernel/tracing/available_tracers
# output: blk function_graph function nop ...

# Enable function tracer
echo function > /sys/kernel/tracing/current_tracer

# Trace a specific kernel function
echo do_sys_open > /sys/kernel/tracing/set_ftrace_filter

# Enable tracing
echo 1 > /sys/kernel/tracing/tracing_on

# Run your program, then read the trace
cat /sys/kernel/tracing/trace

# Disable
echo 0 > /sys/kernel/tracing/tracing_on

# Function graph tracer — shows call stack with timing
echo function_graph > /sys/kernel/tracing/current_tracer
echo do_filp_open > /sys/kernel/tracing/set_graph_function
echo 1 > /sys/kernel/tracing/tracing_on
cat /sys/kernel/tracing/trace
# Output:
#  0)               |  do_filp_open() {
#  0)   0.312 us    |    path_openat();
#  0)   1.542 us    |  }
```

```c
// perf_event.c — Use perf_event_open() to count hardware events
#include <stdio.h>
#include <string.h>
#include <unistd.h>
#include <sys/syscall.h>
#include <linux/perf_event.h>

static long perf_event_open(struct perf_event_attr *hw,
                             pid_t pid, int cpu, int group_fd, unsigned long flags) {
    return syscall(SYS_perf_event_open, hw, pid, cpu, group_fd, flags);
}

int main(void) {
    struct perf_event_attr pe;
    memset(&pe, 0, sizeof(pe));

    pe.type         = PERF_TYPE_HARDWARE;
    pe.size         = sizeof(pe);
    pe.config       = PERF_COUNT_HW_INSTRUCTIONS;  // count CPU instructions
    pe.disabled     = 1;
    pe.exclude_kernel = 1;     // only count user-space instructions
    pe.exclude_hv   = 1;

    // pid=0: measure this process; cpu=-1: all CPUs
    int fd = perf_event_open(&pe, 0, -1, -1, 0);
    if (fd < 0) { perror("perf_event_open"); return 1; }

    // Enable counting
    ioctl(fd, PERF_EVENT_IOC_RESET,  0);
    ioctl(fd, PERF_EVENT_IOC_ENABLE, 0);

    // ─── Code to measure ───
    volatile long sum = 0;
    for (long i = 0; i < 1000000; i++) sum += i;
    // ───────────────────────

    // Disable and read
    ioctl(fd, PERF_EVENT_IOC_DISABLE, 0);

    uint64_t count;
    read(fd, &count, sizeof(count));
    printf("Instructions executed: %lu\n", count);
    printf("Sum = %ld (suppress optimization)\n", sum);

    close(fd);
    return 0;
}
```

---

## 18. io_uring — Modern Async I/O

`io_uring` (Linux 5.1+) is the state-of-the-art asynchronous I/O interface. It uses two shared ring buffers between user space and kernel — eliminating syscall overhead for batched operations.

```
  USER SPACE                        KERNEL SPACE
  ──────────────────────────────    ──────────────────────────────

  ┌─────────────────────────────┐   ┌─────────────────────────────┐
  │  SUBMISSION QUEUE (SQ)      │   │  SUBMISSION QUEUE (SQ)      │
  │  User writes SQE structs    │──►│  Kernel reads SQEs          │
  │  (describes I/O operations) │   │  Performs I/O async         │
  └─────────────────────────────┘   └─────────────────────────────┘
           shared mmap                        │
                                              │ completion
  ┌─────────────────────────────┐   ┌────────▼────────────────────┐
  │  COMPLETION QUEUE (CQ)      │◄──│  COMPLETION QUEUE (CQ)      │
  │  User reads CQEs            │   │  Kernel writes CQEs         │
  │  (results of I/O)           │   │  (result, error, user_data)│
  └─────────────────────────────┘   └─────────────────────────────┘
           shared mmap

  Key advantage: ZERO COPIES, ZERO SYSCALLS for steady-state I/O
  (kernel polls the SQ; user polls the CQ)
```

```c
// io_uring_demo.c — Read a file using io_uring (liburing)
// Link: -luring
#include <stdio.h>
#include <fcntl.h>
#include <unistd.h>
#include <string.h>
#include <liburing.h>

#define BUFFER_SIZE 4096

int main(void) {
    struct io_uring ring;

    // Setup: io_uring_setup() syscall creates the ring buffers
    // IORING_SETUP_SQPOLL: kernel thread polls SQ (zero-syscall mode)
    io_uring_queue_init(32, &ring, 0);

    int fd = open("/etc/hostname", O_RDONLY);
    if (fd < 0) { perror("open"); return 1; }

    char buf[BUFFER_SIZE];
    struct iovec iov = { .iov_base = buf, .iov_len = BUFFER_SIZE };

    // Get a submission queue entry (SQE)
    struct io_uring_sqe *sqe = io_uring_get_sqe(&ring);

    // Prepare a readv operation (like preadv but async)
    io_uring_prep_readv(sqe, fd, &iov, 1, 0);
    sqe->user_data = 42;  // tag for identifying this completion

    // Submit to kernel (ONE syscall for any number of queued ops)
    io_uring_submit(&ring);

    // Wait for completion
    struct io_uring_cqe *cqe;
    io_uring_wait_cqe(&ring, &cqe);

    if (cqe->res < 0) {
        fprintf(stderr, "I/O error: %s\n", strerror(-cqe->res));
    } else {
        buf[cqe->res] = '\0';
        printf("Read %d bytes: %s\n", cqe->res, buf);
    }

    // Advance the CQ ring (mark as consumed)
    io_uring_cqe_seen(&ring, cqe);

    close(fd);
    io_uring_queue_exit(&ring);
    return 0;
}
```

---

## 19. vsyscall & vDSO — Kernel-Mapped User Pages

Some syscalls are called extremely frequently (e.g., `gettimeofday`, `clock_gettime`, `getpid`). Making a full ring 3 → ring 0 switch for each is wasteful. Linux solves this with:

### vDSO (virtual Dynamic Shared Object)

The kernel maps a small shared library into every process's address space at startup. This library contains implementations of certain syscalls that can execute **in user space** by reading kernel-maintained memory.

```
  /proc/[pid]/maps output:
  ...
  7fff12abc000-7fff12abd000 r-xp  [vdso]   ← kernel-mapped page
  ...

  When you call clock_gettime():
  glibc → calls vDSO's __vdso_clock_gettime()
           (this function runs in user space ring 3)
           reads kernel's vvar page (mapped read-only into process)
           vvar contains: current time, tsc multiplier, clockseq
           Computes time using RDTSC instruction
           Returns without ever entering ring 0!
```

```c
// vdso_demo.c — Show that clock_gettime can run without a syscall
#include <stdio.h>
#include <time.h>
#include <sys/syscall.h>
#include <unistd.h>

// Count actual syscalls using perf or strace:
// strace -e trace=clock_gettime ./vdso_demo
// (You'll see: if vDSO works, clock_gettime doesn't appear in strace output!)

int main(void) {
    struct timespec ts;

    // This typically uses vDSO — no kernel entry!
    clock_gettime(CLOCK_MONOTONIC, &ts);
    printf("vDSO time: %ld.%09ld\n", ts.tv_sec, ts.tv_nsec);

    // Force a real syscall (bypass vDSO/libc)
    syscall(SYS_clock_gettime, CLOCK_MONOTONIC, &ts);
    printf("Kernel time: %ld.%09ld\n", ts.tv_sec, ts.tv_nsec);

    return 0;
}
```

---

## 20. Complete Architecture Overview

```
╔══════════════════════════════════════════════════════════════════════════════╗
║                       COMPLETE LINUX KERNEL INTERFACE MAP                  ║
╠══════════════════════════════════════════════════════════════════════════════╣
║                                                                              ║
║  ┌─────────┐  ┌─────────┐  ┌──────────┐  ┌────────┐  ┌────────────────┐   ║
║  │  Shell  │  │  Python │  │  Nginx   │  │  Java  │  │  Your C/Rust   │   ║
║  │  (bash) │  │  Script │  │  Server  │  │   App  │  │  Application   │   ║
║  └────┬────┘  └────┬────┘  └────┬─────┘  └───┬────┘  └───────┬────────┘   ║
║       │            │            │             │               │             ║
║  ╔════╧════════════╧════════════╧═════════════╧═══════════════╧═══════════╗ ║
║  ║         C STANDARD LIBRARY (glibc / musl / uclibc)                    ║ ║
║  ║  Provides: printf, fopen, malloc, pthread, regex, locale, math...      ║ ║
║  ╚═══════════════════════════════════════════════════════════════════════╗ ║ ║
║                                                                         ║ ║ ║
║  ╔══════════════════════════════════════════════════════════════════════╗║ ║ ║
║  ║                    SYSCALL INTERFACE                                ║║ ║ ║
║  ║  x86-64: SYSCALL/SYSRET  ARM64: SVC #0  RISCV: ECALL               ║║ ║ ║
║  ║  ~350 syscalls: read,write,open,close,fork,exec,mmap,socket...      ║║ ║ ║
║  ╚══════════════════════════════════════════════════════════════════════╝║ ║ ║
║            │          │          │          │           │               ║ ║ ║
║    ┌───────▼──┐  ┌────▼────┐ ┌──▼──────┐ ┌▼────────┐ ┌▼──────────┐   ║ ║ ║
║    │  PROCESS │  │ MEMORY  │ │  FILE   │ │ NETWORK │ │   IPC /   │   ║ ║ ║
║    │MANAGEMENT│  │MANAGER  │ │SYSTEMS  │ │  STACK  │ │  SIGNALS  │   ║ ║ ║
║    │          │  │         │ │  VFS    │ │TCP/UDP  │ │  PIPES    │   ║ ║ ║
║    │ scheduler│  │ kmalloc │ │         │ │IP/ICMP  │ │  SOCKETS  │   ║ ║ ║
║    │ fork     │  │ vmalloc │ │ ┌─────┐ │ │netfilter│ │  FUTEXES  │   ║ ║ ║
║    │ execve   │  │ page    │ │ │ext4 │ │ │eBPF XDP │ │  SHM      │   ║ ║ ║
║    │ wait     │  │ cache   │ │ │btrfs│ │ └─────────┘ └───────────┘   ║ ║ ║
║    │ cgroups  │  │ mmap    │ │ │xfs  │ │                             ║ ║ ║
║    │ namespac │  │ swap    │ │ │proc │ │  ┌──────────────────────┐   ║ ║ ║
║    └──────────┘  └─────────┘ │ │sys  │ │  │   DEVICE DRIVERS     │   ║ ║ ║
║                               │ │fuse │ │  │                      │   ║ ║ ║
║    ╔══════════════╗            │ │tmpfs│ │  │ Block: NVMe,SATA,USB │   ║ ║ ║
║    ║  eBPF ENGINE ║            │ └─────┘ │  │ Net: Ethernet, WiFi  │   ║ ║ ║
║    ║  Verifier    ║            └─────────┘  │ Char: TTY, Input     │   ║ ║ ║
║    ║  JIT         ║                         │ GPU, Sound, Camera   │   ║ ║ ║
║    ║  Maps        ║                         └──────────────────────┘   ║ ║ ║
║    ╚══════════════╝                                                     ║ ║ ║
║                                                                         ║ ║ ║
║  ══ USER INTERFACES ═══════════════════════════════════════════════════ ║ ║ ║
║  /proc  /sys  /dev  Netlink  ioctl  mmap  signalfd  io_uring  vDSO     ║ ║ ║
╚═════════════════════════════════════════════════════════════════════════╝ ║ ║
                                                                           ╝ ║
  ┌─────────────────────────────────────────────────────────────────────────┐ ║
  │                        HARDWARE                                         │ ║
  │  CPU (x86-64/ARM64/RISC-V)  RAM  NVMe  SATA  USB  PCIe  GPIO  UART    │ ║
  └─────────────────────────────────────────────────────────────────────────┘ ║
                                                                              ║
  ╔════════════════════════════════════════════════════════════════════════════╣
  ║  COMMUNICATION CHANNELS SUMMARY:                                         ║
  ║  ┌────────────────────────────────────────────────────────────────────┐  ║
  ║  │  Channel         Direction        Mechanism                        │  ║
  ║  │  ─────────────   ─────────────    ──────────────────────────────── │  ║
  ║  │  System calls    user→kernel      SYSCALL instruction              │  ║
  ║  │  Return values   kernel→user      rax register + errno             │  ║
  ║  │  /proc /sys      bidirectional    read()/write() on pseudo-files   │  ║
  ║  │  /dev files      bidirectional    read/write/ioctl/mmap            │  ║
  ║  │  Signals         kernel→user      async interrupt of execution     │  ║
  ║  │  Netlink         bidirectional    socket(AF_NETLINK)               │  ║
  ║  │  mmap            bidirectional    shared page table entries        │  ║
  ║  │  vDSO            kernel→user      mapped shared library            │  ║
  ║  │  eBPF            user→kernel→user bpf() syscall + maps             │  ║
  ║  │  io_uring        bidirectional    shared ring buffers              │  ║
  ║  │  FUSE            kernel→user→ker  /dev/fuse + protocol             │  ║
  ║  │  uevents         kernel→user      NETLINK_KOBJECT_UEVENT           │  ║
  ║  └────────────────────────────────────────────────────────────────────┘  ║
  ╚════════════════════════════════════════════════════════════════════════════╝
```

---

## 21. Rust in Linux Kernel Context

Rust was officially added to the Linux kernel in 6.1 (December 2022). It offers memory safety guarantees (no use-after-free, no data races at compile time) for kernel code.

### Rust Kernel Module

```rust
// rust_hello_module.rs — Rust kernel module
// Part of the Linux kernel source tree under samples/rust/

use kernel::prelude::*;

// The module! macro generates the module metadata (like MODULE_AUTHOR, MODULE_LICENSE)
module! {
    type: HelloModule,
    name: "rust_hello",
    author: "Your Name",
    description: "Hello from Rust in the Linux kernel",
    license: "GPL",
}

struct HelloModule;

impl kernel::Module for HelloModule {
    fn init(_module: &'static ThisModule) -> Result<Self> {
        // pr_info! is the Rust equivalent of pr_info() in C
        pr_info!("Rust Hello Module loaded!\n");
        pr_info!("Kernel Rust works!\n");
        Ok(HelloModule)
    }
}

impl Drop for HelloModule {
    fn drop(&mut self) {
        pr_info!("Rust Hello Module unloaded!\n");
    }
}
```

### Rust User Space: Raw Syscalls with the `syscall` Crate

```rust
// user_syscalls.rs — Raw Linux syscalls from Rust user space
// Cargo.toml: [dependencies] libc = "0.2"

use std::ffi::CString;
use std::os::unix::io::RawFd;

// Demonstrate raw syscall via inline assembly (unsafe, educational)
fn raw_getpid() -> u32 {
    let pid: u32;
    unsafe {
        std::arch::asm!(
            "syscall",
            in("rax") 39u64,        // SYS_getpid = 39 on x86-64
            out("rax") pid,
            // syscall clobbers rcx and r11
            lateout("rcx") _,
            lateout("r11") _,
        );
    }
    pid
}

// Safe wrapper using libc
fn demo_proc_reading() {
    use std::fs;
    use std::io::{self, Read};

    let pid = std::process::id();
    println!("My PID: {}", pid);

    // Read /proc/self/status
    let status_path = format!("/proc/{}/status", pid);
    let content = fs::read_to_string(&status_path).unwrap();

    for line in content.lines().take(10) {
        println!("{}", line);
    }

    // Read /proc/self/maps (memory map)
    let maps = fs::read_to_string("/proc/self/maps").unwrap();
    println!("\nMemory regions:");
    for line in maps.lines().take(8) {
        println!("  {}", line);
    }
}

// mmap via libc bindings
fn demo_mmap() {
    use libc::{mmap, munmap, PROT_READ, PROT_WRITE, MAP_PRIVATE, MAP_ANONYMOUS};
    use std::ptr;

    let size: usize = 4096;

    // Allocate anonymous memory via mmap
    let ptr = unsafe {
        mmap(
            ptr::null_mut(),
            size,
            PROT_READ | PROT_WRITE,
            MAP_PRIVATE | MAP_ANONYMOUS,
            -1,
            0,
        )
    };

    if ptr == libc::MAP_FAILED {
        eprintln!("mmap failed");
        return;
    }

    // Write to the mapped memory
    unsafe {
        let slice = std::slice::from_raw_parts_mut(ptr as *mut u8, size);
        for (i, byte) in slice.iter_mut().enumerate() {
            *byte = (i & 0xFF) as u8;
        }
        println!("mmap: wrote {} bytes at {:?}", size, ptr);

        munmap(ptr, size);
    }
    println!("mmap: unmapped successfully");
}

// Signal handling in Rust using libc
fn demo_signal_handling() {
    use libc::{signal, SIGINT, SIG_DFL};

    unsafe extern "C" fn handler(sig: libc::c_int) {
        // Only async-signal-safe operations here!
        let msg = b"Caught SIGINT in Rust handler!\n";
        libc::write(1, msg.as_ptr() as *const libc::c_void, msg.len());
    }

    unsafe {
        // Register signal handler
        signal(SIGINT, handler as libc::sighandler_t);
    }

    println!("Press Ctrl+C to trigger the Rust signal handler...");
    std::thread::sleep(std::time::Duration::from_secs(3));

    // Restore default handler
    unsafe { signal(SIGINT, SIG_DFL); }
}

// Netlink socket from Rust
fn demo_netlink() {
    use std::net::UdpSocket;

    // Using socket2 crate for AF_NETLINK would be cleaner,
    // but here is the raw libc approach:
    let sock = unsafe {
        libc::socket(
            libc::AF_NETLINK,
            libc::SOCK_RAW,
            libc::NETLINK_KOBJECT_UEVENT,
        )
    };

    if sock < 0 {
        // Need root or CAP_NET_ADMIN for uevent listening
        println!("Note: Netlink uevent requires elevated privileges");
        return;
    }

    let addr = libc::sockaddr_nl {
        nl_family: libc::AF_NETLINK as u16,
        nl_pad:    0,
        nl_pid:    std::process::id(),
        nl_groups: 1,
    };

    unsafe {
        libc::bind(
            sock,
            &addr as *const _ as *const libc::sockaddr,
            std::mem::size_of_val(&addr) as u32,
        );
    }

    println!("Netlink socket bound, listening for uevents...");
    unsafe { libc::close(sock); }
}

fn main() {
    println!("=== Rust Linux Kernel Interface Demo ===\n");

    let pid = raw_getpid();
    println!("Raw getpid() via assembly: {}", pid);

    demo_proc_reading();
    demo_mmap();
    demo_signal_handling();
    demo_netlink();
}
```

### Rust Kernel Driver (Character Device, Linux 6.1+ rust support)

```rust
// rust_chardev.rs — Character device driver in Rust
// Part of kernel source tree

use kernel::{
    file::{self, File},
    io_buffer::{IoBufferReader, IoBufferWriter},
    miscdev,
    prelude::*,
    sync::Mutex,
};

module! {
    type: RustChardev,
    name: "rust_chardev",
    author: "Example",
    description: "A character device in Rust",
    license: "GPL",
}

struct DeviceData {
    buffer: Vec<u8>,
}

struct RustChardev {
    _dev: Pin<Box<miscdev::Registration<Self>>>,
}

#[vtable]
impl file::Operations for RustChardev {
    type Data = Arc<Mutex<DeviceData>>;
    type OpenData = Arc<Mutex<DeviceData>>;

    fn open(data: &Arc<Mutex<DeviceData>>, _file: &File) -> Result<Self::Data> {
        pr_info!("rust_chardev: device opened\n");
        Ok(data.clone())
    }

    fn read(
        data: ArcBorrow<'_, Mutex<DeviceData>>,
        _file: &File,
        writer: &mut impl IoBufferWriter,
        offset: u64,
    ) -> Result<usize> {
        let locked = data.lock();
        let buffer = &locked.buffer;
        let offset = offset as usize;
        if offset >= buffer.len() {
            return Ok(0);
        }
        let to_write = core::cmp::min(buffer.len() - offset, writer.len());
        writer.write_slice(&buffer[offset..offset + to_write])?;
        Ok(to_write)
    }

    fn write(
        data: ArcBorrow<'_, Mutex<DeviceData>>,
        _file: &File,
        reader: &mut impl IoBufferReader,
        _offset: u64,
    ) -> Result<usize> {
        let mut locked = data.lock();
        let len = reader.len();
        locked.buffer.resize(len, 0);
        reader.read_slice(&mut locked.buffer)?;
        pr_info!("rust_chardev: received {} bytes\n", len);
        Ok(len)
    }
}

impl kernel::Module for RustChardev {
    fn init(_module: &'static ThisModule) -> Result<Self> {
        let data = Arc::try_new(Mutex::new(DeviceData {
            buffer: Vec::new(),
        }))?;

        let dev = miscdev::Registration::new_pinned(fmt!("rust_chardev"), data)?;
        pr_info!("rust_chardev: /dev/rust_chardev created\n");

        Ok(Self { _dev: dev })
    }
}
```

---

## 22. Summary Comparison Table

| Mechanism | Direction | Kernel Path | Overhead | Use Case |
|---|---|---|---|---|
| System Call | U→K→U | syscall table dispatch | Low (SYSCALL instr) | All fundamental operations |
| /proc read | K→U | procfs seq_file | Very Low | Process info, sysctl |
| /sys read/write | Bidirectional | sysfs kobject attrs | Very Low | Device/driver config |
| /dev ioctl | Bidirectional | char/block driver | Low | Device-specific control |
| Signals | K→U (async) | signal delivery path | Very Low | Notifications, events |
| Pipe | U→U (via K) | pipe buffer in kernel | Low | Process IPC, streaming |
| Unix socket | Bidirectional | sock/af_unix | Low–Med | Daemon IPC |
| Netlink socket | Bidirectional | netlink_rcv | Medium | Routing, udev, audit |
| mmap (file) | Bidirectional | VFS + page cache | Near-zero after fault | Fast file I/O |
| mmap (anon) | U↔K on fault | memory manager | Near-zero after fault | Shared memory, alloc |
| vDSO | K→U (preloaded) | kernel-mapped pages | Zero | clock_gettime, getpid |
| eBPF | U→K via bpf() | BPF verifier + JIT | Zero (after load) | Tracing, net, security |
| io_uring | Bidirectional | shared ring buffers | Zero (SQPOLL mode) | High-perf async I/O |
| FUSE | K→U→K | /dev/fuse protocol | High | Custom filesystems |
| perf_event | U→K→U | perf subsystem | Low | Performance counters |
| ftrace | U→K | tracefs writes | Low | Kernel function tracing |
| uevents | K→U | NETLINK_KOBJECT_UEVENT | Low | Hotplug, device mgmt |
| Audit | K→U | NETLINK_AUDIT | Low | Security compliance |

---

## Quick Reference: Key Files and Commands

```bash
# ═══ PROCESS INFORMATION ══════════════════════════════════════
cat /proc/$$/status          # current shell's process status
cat /proc/$$/maps            # virtual memory map
ls -la /proc/$$/fd/          # open file descriptors
cat /proc/$$/cmdline | tr '\0' ' '  # command line

# ═══ SYSTEM INFORMATION ═══════════════════════════════════════
cat /proc/cpuinfo            # CPU details
cat /proc/meminfo            # memory stats
cat /proc/interrupts         # IRQ counts per CPU
cat /proc/net/dev            # network interface stats
cat /proc/net/tcp            # TCP connections (hex format)
ss -tulnp                    # modern replacement for netstat

# ═══ KERNEL TUNING (sysctl) ═══════════════════════════════════
sysctl -a                    # show all parameters
sysctl net.ipv4.ip_forward   # show one parameter
sysctl -w net.ipv4.ip_forward=1  # set (root)
echo "net.ipv4.ip_forward=1" >> /etc/sysctl.conf  # persist

# ═══ KERNEL MODULES ═══════════════════════════════════════════
lsmod                        # loaded modules
modinfo bluetooth            # module info
strace -c ls /               # syscall summary count

# ═══ DEVICE TREE ══════════════════════════════════════════════
lspci -tv                    # PCI device tree
lsusb -t                     # USB device tree
ls /sys/class/               # device classes
udevadm monitor              # watch udev events live
udevadm info /dev/sda        # device properties

# ═══ TRACING ══════════════════════════════════════════════════
strace -p <pid>              # trace running process
ltrace ./binary              # trace library calls
bpftrace -e 'tracepoint:syscalls:sys_enter_open { printf("%s\n", str(args->filename)); }'
perf stat ls                 # hardware event counts
perf record -ag sleep 5      # system-wide profiling
cat /sys/kernel/tracing/available_events | grep syscalls
```

---

*End of Guide — Linux Kernel User-Space Interface: Complete Reference*
