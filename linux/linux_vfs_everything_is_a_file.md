# Linux VFS: "Everything is a File" — A Complete In-Depth Guide

---

## Table of Contents

1. [Philosophy & Origin](#1-philosophy--origin)
2. [What Does "Everything is a File" Actually Mean?](#2-what-does-everything-is-a-file-actually-mean)
3. [The Unified Interface: File Descriptors](#3-the-unified-interface-file-descriptors)
4. [VFS Architecture — The Core Abstraction Layer](#4-vfs-architecture--the-core-abstraction-layer)
5. [Kernel Data Structures (Deep Dive)](#5-kernel-data-structures-deep-dive)
6. [System Call Layer](#6-system-call-layer)
7. [Memory Subsystem and the Page Cache](#7-memory-subsystem-and-the-page-cache)
8. [Device Files: Block and Character Devices](#8-device-files-block-and-character-devices)
9. [Special Filesystems: proc, sys, devtmpfs, tmpfs](#9-special-filesystems-proc-sys-devtmpfs-tmpfs)
10. [Network as Files: Sockets, /proc/net, io_uring](#10-network-as-files-sockets-procnet-io_uring)
11. [Pipes and FIFOs](#11-pipes-and-fifos)
12. [Signals and eventfd, timerfd, signalfd](#12-signals-and-eventfd-timerfd-signalfd)
13. [Directory Entries and the Dcache](#13-directory-entries-and-the-dcache)
14. [Namespaces and Mount Points](#14-namespaces-and-mount-points)
15. [File Locking Internals](#15-file-locking-internals)
16. [inotify, fanotify: Watching the Filesystem](#16-inotify-fanotify-watching-the-filesystem)
17. [Memory-Mapped Files (mmap)](#17-memory-mapped-files-mmap)
18. [io_uring: Async File I/O Revolution](#18-io_uring-async-file-io-revolution)
19. [Rust Perspective: Safe Abstractions Over FDs](#19-rust-perspective-safe-abstractions-over-fds)
20. [C Implementation Walkthroughs](#20-c-implementation-walkthroughs)
21. [Mental Model Summary](#21-mental-model-summary)

---

## 1. Philosophy & Origin

### Unix Heritage

The "everything is a file" doctrine comes directly from the original Unix design of Ken Thompson and Dennis Ritchie at Bell Labs (1969–1971). Their insight was radical: **collapse the interface diversity of the world into a single, uniform abstraction**.

Before Unix, every device — a disk, a terminal, a tape drive — had its own API, its own set of system calls, its own quirks. Programs were tightly coupled to hardware specifics. You couldn't write a general-purpose tool.

Unix said: **what if reading from a keyboard, reading from a disk, reading from a network, reading from another process's output — all looked identical to the program?**

This single decision shaped all of modern computing.

### The Plan 9 Extension

Bell Labs' follow-up OS, Plan 9 (1980s–1990s), pushed the idea even further: **everything is a file, including the network stack, the window manager, the process table, the CPU itself**. Linux has absorbed many of these ideas into `/proc`, `/sys`, and the socket interface.

### Why It Matters

```
WITHOUT uniform interface:          WITH uniform interface:
  program <--custom API--> disk       program
  program <--custom API--> terminal      |
  program <--custom API--> network    read(fd, buf, n) / write(fd, buf, n)
  program <--custom API--> process       |
                                    fd = anything:
                                      file, socket, pipe, device, timer,
                                      signal queue, event queue, memory...
```

The payoff: `cat`, `grep`, `wc`, `sed` — tools written once — work on files, on network streams, on kernel state, on device output. **Composability** emerges for free.

---

## 2. What Does "Everything is a File" Actually Mean?

The statement is actually a shorthand for something more precise:

> **Every I/O resource in Linux is accessed through a file descriptor (fd), which supports a common set of operations: open, read, write, close, seek, ioctl, mmap, poll/select/epoll.**

Let's enumerate what that "everything" includes:

| Resource Type              | How it's a "file"                      | Example path / API             |
|----------------------------|----------------------------------------|-------------------------------|
| Regular file               | Directly                               | `/home/user/data.txt`          |
| Directory                  | Readable, returns dirents              | `/home/user/`                  |
| Block device               | Read/write raw blocks                  | `/dev/sda`, `/dev/nvme0n1`    |
| Character device           | Stream I/O                             | `/dev/tty`, `/dev/null`        |
| Named pipe (FIFO)          | IPC via filesystem path                | `mkfifo /tmp/mypipe`           |
| Anonymous pipe             | IPC between related processes          | `pipe(fds)`                    |
| Socket                     | Network/IPC communication              | `socket()` → fd               |
| Symbolic link              | Redirects pathname resolution          | `/etc/localtime → ...`         |
| `/proc/PID/...`            | Process state as files                 | `/proc/1234/maps`              |
| `/sys/...`                 | Kernel object attributes               | `/sys/block/sda/size`          |
| `eventfd`                  | Event notification                     | `eventfd(0, 0)` → fd          |
| `timerfd`                  | Timer expiry as readable fd            | `timerfd_create(...)` → fd    |
| `signalfd`                 | Signals as readable fd                 | `signalfd(...)` → fd          |
| `memfd`                    | Anonymous memory as file               | `memfd_create(...)` → fd      |
| `userfaultfd`              | Page fault handling in userspace       | `userfaultfd(...)` → fd       |
| `pidfd`                    | Process handle (race-free)             | `pidfd_open(pid, 0)` → fd     |
| `io_uring` ring            | Async I/O submission/completion        | `io_uring_setup(...)` → fd    |
| `perf_event`               | Performance counter                    | `perf_event_open(...)` → fd   |
| `bpf` map/program          | eBPF objects                           | `bpf(BPF_MAP_CREATE,...)` → fd|
| `fanotify`/`inotify`       | Filesystem change notifications        | `inotify_init()` → fd         |
| GPU/DRM device             | Graphics rendering via fd              | `/dev/dri/card0`               |
| `/dev/shm/`                | POSIX shared memory                    | `shm_open(...)` → fd          |

---

## 3. The Unified Interface: File Descriptors

### What is a File Descriptor?

A file descriptor is an **integer** in userspace — `0`, `1`, `2`, `5`, `17`, etc. It's an index into a per-process table called the **file descriptor table**, maintained in the kernel.

```
Userspace process:
  fd=0  ──►  stdin
  fd=1  ──►  stdout
  fd=2  ──►  stderr
  fd=3  ──►  open("/etc/passwd", O_RDONLY)
  fd=4  ──►  socket(AF_INET, SOCK_STREAM, 0)
  fd=5  ──►  pipe read-end
  fd=6  ──►  timerfd_create(CLOCK_MONOTONIC, 0)
```

The integer `3` in your C program means nothing by itself. It only means something in the context of your process's fd table in the kernel.

### The Three-Level Indirection

```
USERSPACE                    KERNEL
─────────                    ──────

process A                    ┌─────────────────────────────────────────────┐
  fd_table[]                 │         struct files_struct (per-process)   │
  [0] ──────────────────────►│  fd_array[0] ─────────────────────────────► struct file
  [1] ──────────────────────►│  fd_array[1] ─────────────────────────────► struct file
  [3] ──────────────────────►│  fd_array[3] ──┐                            │
                             └────────────────│────────────────────────────┘
                                              │
                             ┌────────────────▼────────────────────────────┐
                             │           struct file (open file description)│
                             │  f_pos: current offset                       │
                             │  f_flags: O_RDONLY, O_NONBLOCK, etc.         │
                             │  f_mode: read/write permissions              │
                             │  f_op: ──────────────────────────────────►  │
                             │  f_inode: ───────────────────────────────►  │
                             │  f_path: (dentry + vfsmount)                │
                             └─────────────────────────────────────────────┘
                                              │
                             ┌────────────────▼────────────────────────────┐
                             │              struct inode                    │
                             │  i_ino: inode number                         │
                             │  i_mode: file type + permissions             │
                             │  i_size: file size                           │
                             │  i_op: inode_operations                      │
                             │  i_fop: file_operations ◄── THE KEY          │
                             │  i_mapping: address_space (page cache)       │
                             │  i_sb: superblock pointer                    │
                             └─────────────────────────────────────────────┘
```

### File Descriptor Inheritance and Sharing

Two critical properties:

1. **Fork inheritance**: Child processes inherit copies of the parent's fd table. Both parent and child have fds pointing to the same `struct file` objects (ref-counted). This is how shell pipelines work.

2. **Reference counting**: A `struct file` is freed only when its reference count drops to zero — meaning all fds pointing to it (across all processes) have been closed.

```c
// In kernel: include/linux/fdtable.h
struct files_struct {
    atomic_t        count;           // reference count
    struct fdtable  *fdt;            // pointer to fd table
    struct fdtable  fdtab;           // inline fd table for small fd counts
    spinlock_t      file_lock;
    unsigned int    next_fd;         // next available fd number
    unsigned long   close_on_exec_init[1];
    unsigned long   open_fds_init[1];
    unsigned long   full_fds_bits_init[1];
    struct file     *fd_array[NR_OPEN_DEFAULT]; // inline array (64 entries)
};
```

---

## 4. VFS Architecture — The Core Abstraction Layer

### What is VFS?

The **Virtual Filesystem Switch (VFS)** is a kernel subsystem that provides a common file model and a set of abstract interfaces. It sits between the system call layer and the actual filesystem implementations (ext4, btrfs, xfs, tmpfs, proc, etc.).

```
┌─────────────────────────────────────────────────────────────────────┐
│                        USERSPACE                                     │
│  read() write() open() close() stat() mmap() ioctl() lseek() ...    │
└──────────────────────────┬──────────────────────────────────────────┘
                           │  system call interface
┌──────────────────────────▼──────────────────────────────────────────┐
│                     SYSTEM CALL LAYER                                │
│              sys_read(), sys_write(), sys_open() ...                 │
└──────────────────────────┬──────────────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────────────┐
│                  VFS (Virtual Filesystem Switch)                      │
│                                                                       │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐               │
│  │  superblock  │  │    inode     │  │    dentry    │               │
│  │  operations  │  │  operations  │  │  operations  │               │
│  └──────────────┘  └──────────────┘  └──────────────┘               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐               │
│  │    file      │  │  address_    │  │    dcache    │               │
│  │  operations  │  │   space ops  │  │   (dentry    │               │
│  └──────────────┘  └──────────────┘  │    cache)   │               │
│                                       └──────────────┘               │
└──────┬──────────────┬──────────────┬──────────────┬─────────────────┘
       │              │              │              │
┌──────▼──────┐ ┌─────▼──────┐ ┌────▼──────┐ ┌────▼──────┐
│    ext4     │ │   btrfs    │ │  tmpfs/   │ │  procfs/  │
│  (on-disk)  │ │  (on-disk) │ │  ramfs    │ │  sysfs    │
└──────┬──────┘ └─────┬──────┘ └───────────┘ └───────────┘
       │              │
┌──────▼──────────────▼──────────────────────────────────────┐
│               Block Layer (bio, request queues)              │
└──────────────────────────┬─────────────────────────────────┘
                           │
┌──────────────────────────▼─────────────────────────────────┐
│              Storage Drivers (NVMe, SCSI, SATA...)           │
└────────────────────────────────────────────────────────────┘
```

### The Four Core VFS Objects

VFS defines four primary object types. Each has an "operations" structure — a vtable of function pointers — that concrete filesystems implement.

```
┌────────────────┐     ┌────────────────┐     ┌────────────────┐
│   superblock   │────►│     inode      │────►│    dentry      │
│                │     │                │     │                │
│ whole FS state │     │  file metadata │     │  name ↔ inode  │
│ sb_operations  │     │  inode_ops     │     │  dentry_ops    │
│                │     │  file_ops      │     │                │
└────────────────┘     └────────────────┘     └────────────────┘
                                                      │
                                               ┌──────▼─────────┐
                                               │      file      │
                                               │                │
                                               │  open instance │
                                               │  file_ops      │
                                               │  current pos   │
                                               └────────────────┘
```

---

## 5. Kernel Data Structures (Deep Dive)

### 5.1 struct super_block

Represents a mounted filesystem. There's one per mount point.

```c
// Simplified from include/linux/fs.h
struct super_block {
    struct list_head    s_list;          // list of all superblocks
    dev_t               s_dev;           // device identifier
    unsigned char       s_blocksize_bits;
    unsigned long       s_blocksize;     // block size in bytes
    loff_t              s_maxbytes;      // max file size
    struct file_system_type *s_type;     // filesystem type
    const struct super_operations *s_op; // operations vtable
    unsigned long       s_flags;         // mount flags (MS_RDONLY, etc.)
    unsigned long       s_magic;         // filesystem magic number
    struct dentry       *s_root;         // root dentry
    struct rw_semaphore s_umount;        // unmount semaphore
    int                 s_count;         // reference count
    struct list_head    s_inodes;        // all inodes on this sb
    spinlock_t          s_inode_list_lock;
    struct list_head    s_mounts;        // list of mounts of this sb
    struct block_device *s_bdev;         // backing block device (if any)
    struct backing_dev_info *s_bdi;      // backing device info
    struct hlist_node   s_instances;     // instances of same fs type
    void                *s_fs_info;      // filesystem-private data
                                         //   ext4: struct ext4_sb_info*
                                         //   proc: struct proc_fs_info*
    // ... ~100 more fields
};

struct super_operations {
    struct inode *(*alloc_inode)(struct super_block *sb);
    void          (*destroy_inode)(struct inode *);
    void          (*dirty_inode)(struct inode *, int flags);
    int           (*write_inode)(struct inode *, struct writeback_control *);
    int           (*drop_inode)(struct inode *);
    void          (*evict_inode)(struct inode *);
    void          (*put_super)(struct super_block *);
    int           (*sync_fs)(struct super_block *, int wait);
    int           (*freeze_super)(struct super_block *);
    int           (*thaw_super)(struct super_block *);
    int           (*statfs)(struct dentry *, struct kstatfs *);
    int           (*remount_fs)(struct super_block *, int *, char *);
    void          (*umount_begin)(struct super_block *);
    // ...
};
```

### 5.2 struct inode

The **inode** is the fundamental object. It represents a file's metadata and is independent of its name(s). One inode can have many names (hard links). One inode can have zero names and still exist (while open by a process).

```c
struct inode {
    umode_t             i_mode;       // file type + permissions (S_IFREG, S_IFDIR, etc.)
    unsigned short      i_opflags;
    kuid_t              i_uid;        // owner user ID
    kgid_t              i_gid;        // owner group ID
    unsigned int        i_flags;      // filesystem flags (S_IMMUTABLE, etc.)
    struct posix_acl    *i_acl;
    struct posix_acl    *i_default_acl;
    const struct inode_operations *i_op;  // inode vtable
    struct super_block  *i_sb;        // owning superblock
    struct address_space *i_mapping;  // page cache mapping

    unsigned long       i_ino;        // inode number (unique per sb)
    union {
        const unsigned int i_nlink;   // hard link count
        unsigned int    __i_nlink;
    };
    dev_t               i_rdev;       // device ID (for device files)
    loff_t              i_size;       // file size in bytes
    struct timespec64   i_atime;      // last access time
    struct timespec64   i_mtime;      // last modification time
    struct timespec64   i_ctime;      // last status change time
    spinlock_t          i_lock;
    unsigned short      i_bytes;      // # bytes in last block
    u8                  i_blkbits;
    u8                  i_write_hint;
    blkcnt_t            i_blocks;     // # 512-byte blocks allocated
    unsigned long       i_state;      // I_NEW, I_DIRTY, I_FREEING, etc.
    struct rw_semaphore i_rwsem;
    unsigned long       dirtied_when;
    unsigned long       dirtied_time_when;
    struct hlist_node   i_hash;       // hash table for inode lookup
    struct list_head    i_io_list;    // writeback list
    struct list_head    i_lru;        // LRU list
    struct list_head    i_sb_list;    // superblock's inode list
    struct list_head    i_wb_list;
    union {
        struct hlist_head i_dentry;   // list of dentries for this inode
        struct rcu_head   i_rcu;
    };
    atomic64_t          i_version;
    atomic64_t          i_sequence;
    atomic_t            i_count;      // reference count
    atomic_t            i_dio_count;
    atomic_t            i_writecount;
    union {
        const struct file_operations *i_fop; // file operations vtable
        void (*free_inode)(struct inode *);
    };
    struct file_lock_context *i_flctx;
    struct address_space    i_data;   // page cache (for non-mapped inodes)
    struct list_head    i_devices;    // devices on this inode
    union {
        struct pipe_inode_info  *i_pipe;   // for pipes
        struct cdev             *i_cdev;   // for char devices
        char                    *i_link;   // for symlinks
        unsigned                i_dir_seq; // for directories
    };
    __u32               i_generation;
    void                *i_private;   // fs-private data
};
```

**File type bits in `i_mode`:**

```c
#define S_IFMT   0170000   // bitmask for type
#define S_IFSOCK 0140000   // socket
#define S_IFLNK  0120000   // symbolic link
#define S_IFREG  0100000   // regular file
#define S_IFBLK  0060000   // block device
#define S_IFDIR  0040000   // directory
#define S_IFCHR  0020000   // character device
#define S_IFIFO  0010000   // FIFO / named pipe
```

### 5.3 struct dentry

A **dentry** (directory entry) maps a filename component to an inode. Dentries exist in memory (the dcache) as a lookup cache. They are not stored directly on disk — ext4 has its own directory format, and VFS dentries are reconstructed from it.

```c
struct dentry {
    unsigned int        d_flags;          // DCACHE_DIRECTORY_TYPE, etc.
    seqcount_spinlock_t d_seq;
    struct hlist_bl_node d_hash;          // lookup hash table node
    struct dentry       *d_parent;        // parent directory dentry
    struct qstr         d_name;           // component name (quick string)
    struct inode        *d_inode;         // inode this dentry points to
                                          // NULL if negative (not-found cache)
    unsigned char       d_iname[DNAME_INLINE_LEN]; // short name inline storage

    const struct dentry_operations *d_op; // dentry vtable
    struct super_block  *d_sb;            // superblock
    unsigned long       d_time;           // revalidation time
    void                *d_fsdata;        // fs-private data

    union {
        struct list_head d_lru;           // LRU list
        wait_queue_head_t *d_wait;
    };
    struct list_head    d_child;          // child of d_parent
    struct list_head    d_subdirs;        // children of this dentry
    union {
        struct hlist_node d_alias;        // for inode's d_alias list
        struct hlist_bl_node d_in_lookup_hash;
        struct rcu_head   d_rcu;
    } d_u;
};
```

**Dentry states:**

```
┌──────────┬───────────────────────────────────────────────────────┐
│ State    │ Meaning                                                │
├──────────┼───────────────────────────────────────────────────────┤
│ used     │ d_inode != NULL, d_count > 0: actively referenced     │
│ unused   │ d_inode != NULL, d_count == 0: cached, reclaimable    │
│ negative │ d_inode == NULL: caches "file not found" result       │
└──────────┴───────────────────────────────────────────────────────┘
```

### 5.4 struct file

The `struct file` represents an **open file description** — one instance of an open file. Multiple fds (even across processes) can point to the same `struct file` (via `dup()`, `fork()`, SCM_RIGHTS socket passing).

```c
struct file {
    union {
        struct llist_node   f_llist;
        struct rcu_head     f_rcuhead;
        unsigned int        f_iocb_flags;
    };
    spinlock_t          f_lock;
    fmode_t             f_mode;         // FMODE_READ, FMODE_WRITE, FMODE_EXEC
    atomic_long_t       f_count;        // reference count
    struct mutex        f_pos_lock;
    loff_t              f_pos;          // current file position (seek offset)
    unsigned int        f_flags;        // O_RDONLY, O_NONBLOCK, O_APPEND, etc.
    struct fown_struct  f_owner;        // SIGIO/SIGURG owner info
    const struct cred   *f_cred;        // credentials at open() time
    struct file_ra_state f_ra;          // readahead state
    u64                 f_version;
    void                *f_security;    // LSM security blob
    void                *private_data;  // fs/driver private state
    struct address_space *f_mapping;    // page cache
    errseq_t            f_wb_err;
    errseq_t            f_sb_err;
    // critical: the path (dentry + vfsmount)
    struct path         f_path;         // { vfsmount*, dentry* }
    // critical: the operations
    const struct file_operations *f_op; // file vtable (copied from inode)
} __randomize_layout;
```

### 5.5 struct file_operations — The Core Vtable

This is the **heart** of "everything is a file." Every resource that can be accessed as a file provides a `file_operations` struct with function pointers implementing the generic interface.

```c
struct file_operations {
    struct module   *owner;
    loff_t          (*llseek)(struct file *, loff_t, int);
    ssize_t         (*read)(struct file *, char __user *, size_t, loff_t *);
    ssize_t         (*write)(struct file *, const char __user *, size_t, loff_t *);
    ssize_t         (*read_iter)(struct kiocb *, struct iov_iter *);
    ssize_t         (*write_iter)(struct kiocb *, struct iov_iter *);
    int             (*iopoll)(struct kiocb *kiocb, struct io_poll_def *, __poll_t);
    int             (*iterate_shared)(struct file *, struct dir_context *);
    __poll_t        (*poll)(struct file *, struct poll_table_struct *);
    long            (*unlocked_ioctl)(struct file *, unsigned int, unsigned long);
    long            (*compat_ioctl)(struct file *, unsigned int, unsigned long);
    int             (*mmap)(struct file *, struct vm_area_struct *);
    unsigned long   mmap_supported_flags;
    int             (*open)(struct inode *, struct file *);
    int             (*flush)(struct file *, fl_owner_t id);
    int             (*release)(struct inode *, struct file *);
    int             (*fsync)(struct file *, loff_t, loff_t, int datasync);
    int             (*fasync)(int, struct file *, int);
    int             (*lock)(struct file *, int, struct file_lock *);
    ssize_t         (*sendpage)(struct file *, struct page *, int, size_t, loff_t *, int);
    unsigned long   (*get_unmapped_area)(struct file *, unsigned long, unsigned long,
                                         unsigned long, unsigned long);
    int             (*check_flags)(int);
    int             (*setfl)(struct file *, unsigned long);
    int             (*flock)(struct file *, int, struct file_lock *);
    ssize_t         (*splice_write)(struct pipe_inode_info *, struct file *,
                                    loff_t *, size_t, unsigned int);
    ssize_t         (*splice_read)(struct file *, loff_t *, struct pipe_inode_info *,
                                   size_t, unsigned int);
    void            (*splice_eof)(struct file *);
    int             (*setlease)(struct file *, long, struct file_lock **, void **);
    long            (*fallocate)(struct file *, int, loff_t, loff_t);
    void            (*show_fdinfo)(struct seq_file *, struct file *);
    ssize_t         (*copy_file_range)(struct file *, loff_t, struct file *,
                                        loff_t, size_t, unsigned int);
    loff_t          (*remap_file_range)(struct file *, loff_t, struct file *,
                                         loff_t, loff_t, unsigned int);
    int             (*fadvise)(struct file *, loff_t, loff_t, int);
    int             (*uring_cmd)(struct io_uring_cmd *, unsigned int);
    int             (*uring_cmd_iopoll)(struct io_uring_cmd *, struct io_poll_def *,
                                         __poll_t);
} __randomize_layout;
```

The `NULL` entries mean "not implemented" — VFS provides sensible defaults for many operations (e.g., `no_llseek`, `generic_read_iter`).

---

## 6. System Call Layer

### How `open()` Works — Full Kernel Path

```
userspace: fd = open("/etc/passwd", O_RDONLY)
                    │
                    │  int syscall
                    ▼
sys_openat(AT_FDCWD, "/etc/passwd", O_RDONLY, 0)
                    │
                    ▼
do_sys_openat2()
  │
  ├─► build_open_flags()          // validate & normalize flags
  │
  ├─► getname("/etc/passwd")      // copy path from userspace to kernel
  │
  └─► do_filp_open()
        │
        ├─► path_openat()          // pathname resolution loop
        │     │
        │     ├─► link_path_walk() // walk "/" then "etc" then "passwd"
        │     │     │
        │     │     ├─► lookup_fast() // check dcache first
        │     │     │
        │     │     └─► lookup_slow() // dcache miss → call inode->i_op->lookup()
        │     │           └─► ext4_lookup() // reads directory block from disk
        │     │
        │     └─► do_open()
        │           │
        │           ├─► vfs_open()
        │           │     ├─► alloc_empty_file()  // allocate struct file
        │           │     └─► do_dentry_open()
        │           │           ├─► f->f_op = inode->i_fop  // set vtable
        │           │           └─► f->f_op->open(inode, f) // call fs open
        │           │                 └─► ext4_file_open()
        │           │
        │           └─► security_file_open() // LSM (SELinux/AppArmor) check
        │
        └─► get_unused_fd_flags()  // allocate fd number
              └─► fd_install(fd, filp) // put struct file* into fd table
```

### How `read()` Works

```
userspace: n = read(fd, buf, 1024)
                │
                ▼
sys_read(fd, buf, count)
                │
                ▼
ksys_read()
  │
  ├─► fdget_pos(fd)      // get struct file* from fd table, lock f_pos
  │
  └─► vfs_read(file, buf, count, &pos)
        │
        ├─► rw_verify_area()       // check permissions, mandatory locks
        │
        └─► file->f_op->read_iter(kiocb, iov_iter)
              │
              ├─► [ext4_file_read_iter] for regular files:
              │     └─► generic_file_read_iter()
              │           └─► filemap_read()
              │                 ├─► find_get_pages_contig()  // check page cache
              │                 ├─► [page cache hit] → copy_page_to_iter()
              │                 └─► [page cache miss]
              │                       ├─► page_cache_alloc()
              │                       ├─► ext4_readahead() / ext4_readpage()
              │                       │     └─► submit_bio() → block layer
              │                       └─► wait_on_page_locked() then copy
              │
              ├─► [socket read_iter] for sockets:
              │     └─── sock_read_iter() → sock->ops->recvmsg()
              │
              └─── [pipe read_iter] for pipes:
                    └─── pipe_read() → copy from pipe buffer ring
```

### Complete Syscall Table (file-related)

```
open/openat/openat2  creat    close    read     write    pread64   pwrite64
readv    writev       preadv   pwritev  lseek    llseek   truncate  ftruncate
stat     fstat        lstat    statx    access   faccessat chmod    fchmod
chown    fchown       lchown   link     linkat   unlink   unlinkat  rename
renameat renameat2    symlink  readlink mkdir    rmdir    getdents  getdents64
dup      dup2         dup3     fcntl    ioctl    mmap     munmap    msync
mincore  madvise      select   poll     epoll_*  sendfile splice   tee
vmsplice fallocate    sync     fsync    fdatasync syncfs   flock    
inotify_* fanotify_* eventfd  eventfd2 timerfd_* signalfd* pipe    pipe2
socket   socketpair   bind     connect  listen   accept   sendmsg  recvmsg
memfd_create  pidfd_open  pidfd_send_signal  userfaultfd  io_uring_setup
```

---

## 7. Memory Subsystem and the Page Cache

### The Page Cache

The **page cache** is the central buffer between the VFS and physical storage. All file data passes through it. When you read a file, the kernel brings pages from disk into the page cache. When you write, you modify pages in the cache, and they're written back asynchronously.

```
Virtual Memory Areas (VMAs)     Page Cache              Storage
─────────────────────────       ──────────              ───────
process A: mmap(file)    ──────►  page 0   ◄───── ext4_readpage ◄── NVMe
process B: read(fd)      ──────►  page 1          submit_bio
process C: mmap(file)    ──────►  page 2   ──────► writeback ──────► NVMe
                                  page 3
                                  ...
```

The same physical page can be mapped into multiple processes' address spaces via the page cache. This is why `mmap` of shared libraries is memory-efficient.

### struct address_space

Each inode has an `address_space` (also called `i_mapping`). This represents the page cache for that file.

```c
struct address_space {
    struct inode            *host;           // owning inode
    struct xarray           i_pages;         // radix tree of cached pages
    struct rw_semaphore     invalidate_lock;
    gfp_t                   gfp_mask;
    atomic_t                i_mmap_writable; // number of writable VMAs
    struct rb_root_cached   i_mmap;          // tree of VMAs mapping this file
    struct rw_semaphore     i_mmap_rwsem;
    unsigned long           nrpages;         // number of cached pages
    unsigned long           writeback_index; // writeback start point
    const struct address_space_operations *a_ops; // operations vtable
    unsigned long           flags;
    errseq_t                wb_err;
    spinlock_t              private_lock;
    struct list_head        private_list;
    void                    *private_data;
};

struct address_space_operations {
    int (*writepage)(struct page *, struct writeback_control *);
    int (*read_folio)(struct file *, struct folio *);
    int (*writepages)(struct address_space *, struct writeback_control *);
    bool (*dirty_folio)(struct address_space *, struct folio *);
    void (*readahead)(struct readahead_control *);
    int (*write_begin)(struct file *, struct address_space *, loff_t, unsigned,
                       struct page **, void **);
    int (*write_end)(struct file *, struct address_space *, loff_t, unsigned,
                     unsigned, struct page *, void *);
    sector_t (*bmap)(struct address_space *, sector_t);
    void (*invalidate_folio)(struct folio *, size_t, size_t);
    bool (*release_folio)(struct folio *, gfp_t);
    void (*free_folio)(struct folio *);
    ssize_t (*direct_IO)(struct kiocb *, struct iov_iter *);
    int (*migrate_folio)(struct address_space *, struct folio *, struct folio *,
                          enum migrate_mode);
    int (*launder_folio)(struct folio *);
    bool (*is_partially_uptodate)(struct folio *, size_t, size_t);
    void (*is_dirty_writeback)(struct folio *, bool *, bool *);
    int (*error_remove_folio)(struct address_space *, struct folio *);
    int (*swap_activate)(struct swap_info_struct *, struct file *, sector_t *);
    void (*swap_deactivate)(struct file *);
    int (*swap_rw)(struct kiocb *, struct iov_iter *);
};
```

### Page Lifecycle

```
            COLD                                            HOT
             │                                               │
             ▼                                               ▼
        [disk/storage]  ──read_folio──►  [page cache]  ──copy──►  [userspace]
                                              │
                             ┌────────────────┴─────────────────┐
                             │           page states:            │
                             │  Uptodate: data == disk           │
                             │  Dirty: data != disk (modified)   │
                             │  Writeback: being written to disk │
                             │  Locked: being read/written       │
                             └───────────────────────────────────┘
                                              │
                          [dirty] ──writeback──► [disk/storage]
```

### Direct I/O vs Buffered I/O

```
Buffered I/O (default):
  read() → page cache → copy_to_user → userspace buf
  write() → copy_from_user → page cache → [async writeback]
  Advantage: subsequent reads are fast (cache hit)
  Disadvantage: double-copy, cache pollution for large sequential I/O

Direct I/O (O_DIRECT):
  read() → DMA straight to user buffer (bypasses page cache)
  write() → DMA straight from user buffer to device
  Advantage: zero-copy, no cache pollution
  Disadvantage: requires aligned buffers, synchronous, no readahead
  Used by: databases (PostgreSQL, MySQL manage their own buffer pool)
```

---

## 8. Device Files: Block and Character Devices

### Character Devices

Character devices provide a **byte stream** interface. They do not support `lseek` in the traditional sense. Each read/write is processed in real time.

```
/dev/tty     → terminal (line discipline layer)
/dev/null    → discard all writes, return EOF on reads
/dev/zero    → return infinite zero bytes
/dev/random  → return random bytes (blocks when entropy low)
/dev/urandom → return random bytes (non-blocking)
/dev/mem     → physical memory access (dangerous, usually restricted)
/dev/kmem    → kernel virtual memory
/dev/ptmx    → pseudoterminal master multiplexer
```

**Registration:**

```c
// In a character device driver:
static struct file_operations mydev_fops = {
    .owner   = THIS_MODULE,
    .read    = mydev_read,
    .write   = mydev_write,
    .open    = mydev_open,
    .release = mydev_release,
    .poll    = mydev_poll,
    .unlocked_ioctl = mydev_ioctl,
};

// Register:
static int __init mydev_init(void) {
    major = register_chrdev(0, "mydev", &mydev_fops);
    // Creates /dev/mydev via udev/mdev/devtmpfs
    cls = class_create(THIS_MODULE, "mydev");
    device_create(cls, NULL, MKDEV(major, 0), NULL, "mydev");
    return 0;
}
```

**`/dev/null` implementation (kernel/chr_dev.c):**

```c
static ssize_t read_null(struct file *file, char __user *buf,
                          size_t count, loff_t *ppos)
{
    return 0;  // EOF immediately
}

static ssize_t write_null(struct file *file, const char __user *buf,
                           size_t count, loff_t *ppos)
{
    return count;  // pretend to write all bytes
}

const struct file_operations null_fops = {
    .llseek  = null_lseek,
    .read    = read_null,
    .write   = write_null,
    .read_iter  = read_iter_null,
    .write_iter = write_iter_null,
    .splice_write = splice_write_null,
};
```

### Block Devices

Block devices provide **random-access, block-granular** I/O. They underlie filesystems.

```
/dev/sda        → first SCSI/SATA disk
/dev/sda1       → first partition
/dev/nvme0n1    → first NVMe namespace
/dev/loop0      → loopback (file as block device)
/dev/md0        → software RAID
/dev/dm-0       → device mapper (LVM, dm-crypt, etc.)
```

**Block device path:**

```
write(fd, buf, 4096) on a regular file
    │
    ▼
ext4_write_begin() / ext4_write_end()
    │
    ▼
block_write_full_page() / submit_bh()
    │
    ▼
bio_alloc() + bio_add_page()  ← bio = block I/O descriptor
    │
    ▼
submit_bio() → blk_mq_submit_bio()
    │
    ▼
I/O scheduler (mq-deadline, kyber, bfq, none)
    │
    ▼
NVMe/SCSI driver → hardware DMA → disk
```

**`struct bio` — the block layer's fundamental unit:**

```c
struct bio {
    struct bio          *bi_next;      // request queue link
    struct block_device *bi_bdev;      // target device
    blk_opf_t           bi_opf;        // operation flags (READ/WRITE/FLUSH)
    unsigned short      bi_flags;
    unsigned short      bi_ioprio;
    blk_status_t        bi_status;
    atomic_t            __bi_remaining;
    struct bvec_iter    bi_iter;        // current iterator state
    blk_qc_t            bi_cookie;
    bio_end_io_t        *bi_end_io;    // completion callback
    void                *bi_private;
    struct bio_vec      *bi_io_vec;    // array of (page, offset, len) tuples
    struct bio_set      *bi_pool;
    struct bio_vec      bi_inline_vecs[]; // inline storage for small bios
};
```

### major:minor Device Numbers

```c
dev_t device = MKDEV(major, minor);
// major: identifies driver
// minor: identifies specific device instance

// Example:
// /dev/sda  = 8:0
// /dev/sda1 = 8:1
// /dev/sdb  = 8:16
// /dev/null = 1:3
// /dev/zero = 1:5
// /dev/tty  = 5:0
// /dev/tty0 = 4:0
```

---

## 9. Special Filesystems: proc, sys, devtmpfs, tmpfs

### procfs (`/proc`)

`/proc` is an in-memory filesystem that exposes kernel and process state as files. It has no backing store — files are generated on the fly by kernel code when read.

```
/proc/
  ├── 1/                    ← process PID 1 (init/systemd)
  │   ├── cmdline           ← command line (null-separated)
  │   ├── environ           ← environment variables
  │   ├── maps              ← virtual memory areas
  │   ├── smaps             ← detailed memory maps with RSS, PSS, etc.
  │   ├── status            ← human-readable process status
  │   ├── stat              ← machine-readable process stat
  │   ├── statm             ← memory usage statistics
  │   ├── fd/               ← open file descriptors (symlinks)
  │   │   ├── 0 → /dev/null
  │   │   ├── 1 → /dev/pts/0
  │   │   └── 2 → /dev/pts/0
  │   ├── fdinfo/           ← detailed fd info (pos, flags, etc.)
  │   ├── mem               ← process memory (readable with ptrace)
  │   ├── pagemap           ← virtual→physical page mapping
  │   ├── wchan             ← kernel function process is waiting in
  │   ├── syscall           ← current syscall info
  │   ├── stack             ← kernel stack trace
  │   ├── task/             ← per-thread info
  │   ├── net/              ← network state (if net namespace)
  │   ├── ns/               ← namespace handles (symlinks to fds)
  │   │   ├── net → net:[4026531992]
  │   │   ├── pid → pid:[4026531836]
  │   │   └── mnt → mnt:[4026531840]
  │   └── cgroup            ← cgroup membership
  │
  ├── cpuinfo               ← CPU model, features, cache
  ├── meminfo               ← memory usage breakdown
  ├── vmstat                ← virtual memory statistics
  ├── buddyinfo             ← buddy allocator free lists
  ├── slabinfo              ← SLAB/SLUB allocator stats
  ├── interrupts            ← interrupt counts per CPU per IRQ
  ├── iomem                 ← physical memory map
  ├── ioports               ← I/O port map
  ├── kallsyms              ← kernel symbol table (with addresses)
  ├── kcore                 ← kernel memory as ELF core (gdb target)
  ├── modules               ← loaded kernel modules
  ├── mounts                ← mounted filesystems
  ├── filesystems           ← registered filesystem types
  ├── devices               ← char/block device majors
  ├── partitions            ← partition table
  ├── uptime                ← system uptime
  ├── loadavg               ← load average
  ├── version               ← kernel version string
  ├── sys/                  ← sysctl tree (writable tunables)
  │   ├── kernel/
  │   │   ├── pid_max       ← max PID value
  │   │   ├── printk        ← log levels
  │   │   └── randomize_va_space ← ASLR setting
  │   ├── vm/
  │   │   ├── swappiness    ← swappiness (0-200)
  │   │   └── dirty_ratio
  │   └── net/
  │       └── ipv4/tcp_*
  └── net/
      ├── tcp               ← TCP socket table
      ├── udp               ← UDP socket table
      ├── if_inet6          ← IPv6 interfaces
      ├── arp               ← ARP table
      └── route             ← routing table
```

**How a procfs file works internally:**

```c
// fs/proc/task_mmu.c — /proc/PID/maps implementation
static int show_map(struct seq_file *m, void *v)
{
    struct vm_area_struct *vma = v;
    struct mm_struct *mm = vma->vm_mm;
    struct file *file = vma->vm_file;
    // ... format and output one VMA line
    seq_printf(m, "%08lx-%08lx %c%c%c%c %08llx %02x:%02x %lu %s\n",
               start, end, r, w, x, p, pgoff, major, minor, ino, name);
    return 0;
}

// The seq_file abstraction:
// seq_read() calls show_map() for each VMA, buffering output.
// User's read() call gets whatever fits in the buffer.
// /proc/PID/maps generates its content FRESH every read.
```

### sysfs (`/sys`)

`/sys` exposes the **kernel object model (kobject)** as a filesystem. Every device, driver, bus, and class in the kernel is represented as a directory. Attributes are files — read them to get values, write to them to set values.

```
/sys/
  ├── block/              ← block devices
  │   └── sda/
  │       ├── size        ← capacity in 512-byte sectors
  │       ├── queue/
  │       │   ├── scheduler      ← I/O scheduler name (read/write)
  │       │   ├── rotational     ← 0=SSD, 1=HDD
  │       │   └── nr_requests    ← request queue depth
  │       └── sda1/
  │           ├── size
  │           └── start
  │
  ├── bus/
  │   ├── pci/devices/    ← PCI devices by address
  │   └── usb/devices/    ← USB devices
  │
  ├── class/
  │   ├── net/            ← network interfaces
  │   │   └── eth0/
  │   │       ├── address ← MAC address
  │   │       ├── mtu     ← MTU
  │   │       └── speed   ← link speed
  │   └── backlight/
  │       └── intel_backlight/
  │           ├── brightness        ← current brightness (writable)
  │           └── max_brightness
  │
  ├── devices/            ← the device tree (mirrors hardware topology)
  ├── firmware/           ← ACPI, EFI, DMI data
  ├── fs/                 ← per-filesystem tunables
  │   ├── ext4/
  │   └── cgroup/
  ├── kernel/             ← kernel subsystems
  │   └── mm/
  │       └── transparent_hugepage/enabled
  └── module/             ← loaded module parameters
      └── nvme_core/parameters/
```

**kobject and sysfs relationship:**

```c
// Every sysfs directory is a kobject
struct kobject {
    const char      *name;
    struct list_head entry;
    struct kobject  *parent;     // parent in hierarchy
    struct kset     *kset;
    const struct kobj_type *ktype;
    struct kernfs_node *sd;      // sysfs directory entry
    struct kref     kref;        // reference count
    unsigned int state_initialized:1;
    unsigned int state_in_sysfs:1;
    // ...
};

// A sysfs attribute (file in sysfs):
struct attribute {
    const char  *name;
    umode_t      mode;
};

struct kobj_attribute {
    struct attribute attr;
    ssize_t (*show)(struct kobject *, struct kobj_attribute *, char *);
    ssize_t (*store)(struct kobject *, struct kobj_attribute *, const char *, size_t);
};
```

### tmpfs

`tmpfs` is an in-memory filesystem backed by both RAM and swap. Unlike ramfs (which can fill RAM indefinitely), tmpfs is swap-aware.

```c
// tmpfs inodes are real VFS inodes backed by anonymous pages
// Files in tmpfs live in page cache, with swap backing
// mount -t tmpfs -o size=1G tmpfs /mnt/ramdisk
// /tmp, /run, /dev/shm are typically tmpfs
```

### devtmpfs

`devtmpfs` is mounted at `/dev` by the kernel itself during boot. The kernel automatically creates/removes device nodes as drivers register/unregister. `udev` then customizes them (permissions, symlinks, etc.).

---

## 10. Network as Files: Sockets, /proc/net, io_uring

### Sockets as File Descriptors

A socket is a file. `socket()` returns an fd. You can `read()`/`write()` on it (via the `sock_read_iter`/`sock_write_iter` file operations). You can `select()`/`poll()`/`epoll()` on it.

```c
// socket() syscall path:
sys_socket(family, type, protocol)
    │
    └─► sock_create()
          ├─► sock_alloc()          // allocate struct socket + struct inode
          │     // The socket inode has i_fop = &socket_file_ops
          │
          ├─► inet_create()         // for AF_INET
          │     ├─► sk_alloc()      // allocate struct sock
          │     └─► inet_sk(sk)->...
          │
          └─► sock_map_fd()
                ├─► get_unused_fd_flags()
                └─► sock_alloc_file() // create struct file wrapping socket
                      └─► fd_install(fd, file)
```

**Socket data structures:**

```
fd (int)
  │
  ▼
struct file
  f_op = &socket_file_ops
  private_data ──────────────────► struct socket
                                     state: SS_CONNECTED
                                     type: SOCK_STREAM
                                     ops: &inet_stream_ops  (TCP)
                                     sk ──────────────────► struct sock
                                                              sk_family: AF_INET
                                                              sk_type: SOCK_STREAM
                                                              sk_protocol: IPPROTO_TCP
                                                              sk_receive_queue: skb list
                                                              sk_write_queue: skb list
                                                              sk_state: TCP_ESTABLISHED
                                                              sk_prot: &tcp_prot
```

**`struct sk_buff` — the network packet:**

```
┌─────────────────────────────────────────────────────┐
│                    sk_buff                           │
│  head ──► ┌──────────────────────────────────────┐  │
│           │    headroom (for headers)             │  │
│  data ──► ├──────────────────────────────────────┤  │
│           │    L4 header (TCP/UDP)                │  │
│           ├──────────────────────────────────────┤  │
│           │    L3 header (IP)                     │  │
│           ├──────────────────────────────────────┤  │
│           │    L2 header (Ethernet)               │  │
│           ├──────────────────────────────────────┤  │
│           │    payload data                       │  │
│  tail ──► ├──────────────────────────────────────┤  │
│           │    tailroom                           │  │
│  end  ──► └──────────────────────────────────────┘  │
│                                                      │
│  sk: owning socket    dev: net device               │
│  len: data length     data_len: nonlinear length    │
│  protocol: ETH_P_IP   priority: packet prio         │
└─────────────────────────────────────────────────────┘
```

### /proc/net — Network State as Files

```bash
cat /proc/net/tcp       # TCP socket table with hex addresses/ports
cat /proc/net/tcp6      # IPv6 TCP
cat /proc/net/udp       # UDP sockets
cat /proc/net/unix      # Unix domain sockets
cat /proc/net/if_inet6  # IPv6 interface addresses
cat /proc/net/arp       # ARP cache
cat /proc/net/route     # IPv4 routing table
cat /proc/net/dev       # per-interface packet stats
cat /proc/net/snmp      # SNMP MIB-II stats
cat /proc/net/netstat   # extended TCP stats
cat /proc/net/sockstat  # socket allocation summary
```

### Unix Domain Sockets

Unix sockets are AF_UNIX sockets — they communicate between processes on the same machine via the filesystem namespace.

```c
// server binds to a path:
bind(fd, (struct sockaddr_un*)&addr, sizeof(addr));
// creates a socket inode at that path (S_IFSOCK in stat)

// client connects:
connect(fd, (struct sockaddr_un*)&addr, sizeof(addr));

// data flows through kernel socket buffers — zero copies across NIC
// also supports sending file descriptors (ancillary data / SCM_RIGHTS):
sendmsg(fd, &msg_with_cmsg_fd, 0);  // send an fd to another process
```

---

## 11. Pipes and FIFOs

### Anonymous Pipes

Pipes are the classic "everything is a file" IPC mechanism. Created by `pipe()`, they exist only in memory. The kernel allocates a circular buffer (pipe buffer), and two fds (read-end and write-end) point to it.

```c
// kernel/pipe.c

struct pipe_inode_info {
    struct mutex        mutex;
    wait_queue_head_t   rd_wait;    // readers waiting for data
    wait_queue_head_t   wr_wait;    // writers waiting for space
    unsigned int        head;       // producer index
    unsigned int        tail;       // consumer index
    unsigned int        max_usage;  // pipe capacity (in pages)
    unsigned int        ring_size;  // number of slots (power of 2)
    unsigned int        nr_accounted;
    unsigned int        readers;    // reader count
    unsigned int        writers;    // writer count
    unsigned int        files;      // struct file count
    unsigned int        r_counter;
    unsigned int        w_counter;
    bool                poll_usage;
    struct page         *tmp_page;
    struct fasync_struct *fasync_readers;
    struct fasync_struct *fasync_writers;
    struct pipe_buffer  *bufs;      // ring of pipe_buffer structs
    struct user_struct  *user;
};

struct pipe_buffer {
    struct page     *page;       // physical page holding data
    unsigned int    offset;      // offset within page
    unsigned int    len;         // length of data
    const struct pipe_buf_operations *ops;
    unsigned int    flags;
    unsigned long   private;
};
```

**Pipe operation:**

```
write(write_fd, "hello", 5)
      │
      ▼
pipe_write()
  ├─► acquire mutex
  ├─► check space in ring (pipe->head - pipe->tail < pipe->ring_size)
  ├─► if no space: block on wr_wait (or EAGAIN if O_NONBLOCK)
  ├─► allocate page (or reuse partially-filled last buffer)
  ├─► copy_from_user() → page
  ├─► advance pipe->head
  └─► wake_up_interruptible(&pipe->rd_wait)

read(read_fd, buf, 1024)
      │
      ▼
pipe_read()
  ├─► acquire mutex
  ├─► check data in ring (pipe->head != pipe->tail)
  ├─► if no data: block on rd_wait
  ├─── copy_to_user() ← page
  ├─── advance pipe->tail
  └─── wake_up_interruptible(&pipe->wr_wait)
```

**Splice — zero-copy pipe:**

```c
// splice() moves data between pipes and fds WITHOUT copying to userspace
// kernel/splice.c

// Example: read from file, write to socket, zero user-copy:
splice(file_fd, &file_offset, pipe_fds[1], NULL, 65536, SPLICE_F_MOVE);
splice(pipe_fds[0], NULL, socket_fd, NULL, 65536, SPLICE_F_MOVE);
```

### Named Pipes (FIFOs)

```c
mkfifo("/tmp/mypipe", 0644);    // creates S_IFIFO inode in filesystem
int fd = open("/tmp/mypipe", O_RDONLY);  // blocks until writer opens
int fd = open("/tmp/mypipe", O_WRONLY);  // blocks until reader opens
// After both ends open, behaves like anonymous pipe
```

---

## 12. Signals and eventfd, timerfd, signalfd

### The Problem with Signals

Signals are asynchronous — they interrupt the running code. Mixing signals with event loops (`epoll`) is notoriously hard. The **self-pipe trick** was the old solution. Linux 2.6.22+ added dedicated fds.

### signalfd

```c
#include <sys/signalfd.h>

sigset_t mask;
sigemptyset(&mask);
sigaddset(&mask, SIGTERM);
sigaddset(&mask, SIGINT);
sigprocmask(SIG_BLOCK, &mask, NULL);  // block signals from normal delivery

int sfd = signalfd(-1, &mask, SFD_NONBLOCK | SFD_CLOEXEC);

// Now epoll_wait() on sfd:
// When SIGTERM or SIGINT arrives, read() returns a signalfd_siginfo struct
struct signalfd_siginfo si;
read(sfd, &si, sizeof(si));
// si.ssi_signo = SIGTERM, etc.
```

### timerfd

```c
#include <sys/timerfd.h>

int tfd = timerfd_create(CLOCK_MONOTONIC, TFD_NONBLOCK | TFD_CLOEXEC);

struct itimerspec ts = {
    .it_interval = { .tv_sec = 1, .tv_nsec = 0 },  // repeat every 1s
    .it_value    = { .tv_sec = 1, .tv_nsec = 0 },  // first expiry in 1s
};
timerfd_settime(tfd, 0, &ts, NULL);

// epoll_wait() on tfd
// When timer fires, read() returns uint64_t: number of expirations
uint64_t count;
read(tfd, &count, sizeof(count));
```

### eventfd

```c
#include <sys/eventfd.h>

int efd = eventfd(0, EFD_NONBLOCK | EFD_CLOEXEC);

// From one thread/process: signal the event
uint64_t val = 1;
write(efd, &val, sizeof(val));  // adds to internal 64-bit counter

// From another thread/process: wait for event
uint64_t count;
read(efd, &count, sizeof(count));  // reads and resets counter (blocks if 0)
// count = accumulated write values since last read
```

These three fds integrate perfectly with `epoll`. Your event loop has **one** call — `epoll_wait()` — and all event sources (sockets, files, timers, signals, child processes via pidfd) are unified.

---

## 13. Directory Entries and the Dcache

### Pathname Resolution

When the kernel resolves `/etc/nginx/nginx.conf`, it walks the path component by component:

```
"/etc/nginx/nginx.conf"
    │
    ├─► start at process->fs->root (or AT_FDCWD for relative paths)
    │
    ├─► lookup "etc" in root dentry
    │     ├─► dcache_lookup(root_inode, "etc") → hit? return dentry
    │     └─► miss: call root_inode->i_op->lookup(root_inode, "etc", ...)
    │           └─── ext4_lookup() → read dir block → create dentry
    │
    ├─► lookup "nginx" in /etc dentry
    │     └─── (same process)
    │
    ├─► lookup "nginx.conf" in /etc/nginx dentry
    │     └─── (same process)
    │
    └─► result: dentry for nginx.conf → inode → file_operations
```

**RCU lookup**: The kernel uses RCU (Read-Copy-Update) for lockless dcache lookups. The fast path for a hot path like `/lib/x86_64-linux-gnu/libc.so.6` (looked up thousands of times per second) requires no locks — just RCU read-side critical section.

### The Dcache (Dentry Cache)

```c
// fs/dcache.c
struct dentry_hashtable {
    // global hash table: (parent_dentry, name_hash) → dentry
    struct hlist_bl_head *table;
    unsigned int          shift;
};

// LRU lists for reclamation
struct list_lru dentry_lru;    // per-memcg, per-NUMA

// dentry_kill() path:
// 1. Remove from hash table (unhash)
// 2. Detach from parent
// 3. Detach from inode (iput)
// 4. Free the dentry slab object
```

**Negative dentries**: When you `open("/tmp/nonexistent")` and it fails, the kernel caches a **negative dentry** (d_inode = NULL) for that name. The next lookup for the same name returns ENOENT without hitting the filesystem.

---

## 14. Namespaces and Mount Points

### VFS Namespaces

Linux supports multiple types of namespaces. The **mount namespace** (mnt namespace) is the VFS-level isolation: each process can have a different view of the filesystem hierarchy.

```
┌─────────────────────────────────────────────────────────┐
│                  Global (initial) mount namespace        │
│                                                          │
│   /  (rootfs)                                           │
│   ├── /proc (procfs)                                    │
│   ├── /sys  (sysfs)                                     │
│   ├── /dev  (devtmpfs)                                  │
│   ├── /home (ext4 on /dev/sda3)                         │
│   └── /run  (tmpfs)                                     │
└─────────────────────────────────────────────────────────┘

                       unshare(CLONE_NEWNS)

┌─────────────────────────────────────────────────────────┐
│               Container mount namespace                   │
│                                                          │
│   /  (overlayfs: upper=container layer, lower=image)    │
│   ├── /proc (new procfs — only container PIDs)          │
│   ├── /sys  (sysfs — restricted)                        │
│   ├── /dev  (tmpfs — only allowed devices)              │
│   └── /app  (bind-mounted from host)                    │
└─────────────────────────────────────────────────────────┘
```

### struct vfsmount and struct mount

```c
struct vfsmount {
    struct dentry   *mnt_root;     // root dentry of this mount
    struct super_block *mnt_sb;    // superblock
    int             mnt_flags;
    struct user_namespace *mnt_userns;
} __randomize_layout;

struct mount {
    struct hlist_node mnt_hash;
    struct mount    *mnt_parent;   // parent mount
    struct dentry   *mnt_mountpoint; // dentry where this is mounted
    struct vfsmount  mnt;
    union {
        struct rcu_head mnt_rcu;
        struct llist_node mnt_llist;
    };
    struct list_head mnt_mounts;   // list of child mounts
    struct list_head mnt_child;    // this mount's node in parent list
    struct list_head mnt_instance; // mount instances of the same sb
    const char      *mnt_devname;  // device name
    struct list_head mnt_list;     // namespace mount list
    struct mnt_namespace *mnt_ns;  // owning namespace
    // ...
};
```

### Bind Mounts

```bash
# Make /home/alice visible at /mnt/alice:
mount --bind /home/alice /mnt/alice
# This creates a new struct mount pointing to /home/alice's dentry,
# attached to /mnt/alice in the namespace.

# Used heavily in containers:
mount --bind /host/data /container/data
```

### Overlayfs (Container Layers)

```
┌────────────────────────────────────────────────────┐
│                  overlayfs mount                    │
│                                                     │
│   upper dir (writable layer, container changes)     │
│   lower dir (read-only image layers, stacked)       │
│   work dir  (for atomic renames between layers)     │
│                                                     │
│   read: upper first, then lower if not in upper    │
│   write: copy-up from lower to upper, then write   │
│   delete: create whiteout file in upper             │
└────────────────────────────────────────────────────┘
```

---

## 15. File Locking Internals

### POSIX Locks (`fcntl`)

```c
// Byte-range locks: lock only part of a file
struct flock fl = {
    .l_type   = F_WRLCK,   // F_RDLCK, F_WRLCK, F_UNLCK
    .l_whence = SEEK_SET,
    .l_start  = 0,
    .l_len    = 4096,      // lock bytes 0-4095
};
fcntl(fd, F_SETLKW, &fl);  // set lock, wait if blocked
```

**Kernel representation:**

```c
struct file_lock {
    struct file_lock    *fl_blocker; // next blocked lock
    struct list_head     fl_list;
    struct hlist_node    fl_link;
    struct list_head     fl_blocked_requests;
    struct list_head     fl_blocked_member;
    fl_owner_t           fl_owner;   // owner (files_struct*)
    unsigned int         fl_flags;   // FL_POSIX, FL_FLOCK, FL_OFDLCK
    unsigned char        fl_type;    // F_RDLCK, F_WRLCK, F_UNLCK
    unsigned int         fl_pid;
    int                  fl_link_cpu;
    wait_queue_head_t    fl_wait;    // waiting processes
    struct file         *fl_file;
    loff_t               fl_start;  // range start
    loff_t               fl_end;    // range end
    // ...
};
```

### flock Locks

`flock()` locks an entire file. It's associated with the open file description (struct file), not the process. All fds pointing to the same struct file share the lock.

### OFD Locks (Open File Description Locks, Linux 3.15+)

`fcntl(F_OFD_SETLK)` — locks are associated with the open file description, not the process. Unlike POSIX locks, they don't get released when a thread closes an fd.

---

## 16. inotify, fanotify: Watching the Filesystem

### inotify

```c
int ifd = inotify_init1(IN_NONBLOCK | IN_CLOEXEC);

// watch a file or directory:
int wd = inotify_add_watch(ifd, "/etc", IN_CREATE | IN_DELETE | IN_MODIFY);

// ifd is now a readable fd — epoll on it:
struct inotify_event ev;
read(ifd, &ev, sizeof(ev) + ev.len);
// ev.wd: which watch descriptor
// ev.mask: what happened (IN_CREATE, etc.)
// ev.name: filename (if directory watch)
```

**Kernel path:**

```
inotify_add_watch()
    │
    └─► inotify_update_watch()
          └─► fsnotify_add_inode_mark()
                └─── attaches fsnotify_mark to inode->i_fsnotify_marks

// When a file changes:
vfs_write() → fsnotify_modify() → fsnotify() → inotify_handle_event()
    └─► allocates inotify_event, places in inotify_inode_mark->events queue
    └─► wake_up_poll() on the inotify fd
```

### fanotify

fanotify is more powerful than inotify:
- Can watch entire mount points or filesystems
- Can intercept file access (permission events) — block or allow
- Used by antivirus, backup software, sandbox monitors

```c
int fan_fd = fanotify_init(FAN_CLASS_CONTENT | FAN_NONBLOCK, O_RDONLY);

fanotify_mark(fan_fd, FAN_MARK_ADD | FAN_MARK_MOUNT,
              FAN_OPEN_PERM | FAN_ACCESS_PERM | FAN_CLOSE_WRITE,
              AT_FDCWD, "/");

// Read events:
struct fanotify_event_metadata ev;
read(fan_fd, &ev, sizeof(ev));
// ev.fd: open fd to the file being accessed (you can read it!)
// ev.mask: FAN_OPEN_PERM, etc.
// ev.pid: accessing process

// For permission events, you MUST respond:
struct fanotify_response resp = { .fd = ev.fd, .response = FAN_ALLOW };
write(fan_fd, &resp, sizeof(resp));
close(ev.fd);
```

---

## 17. Memory-Mapped Files (mmap)

### What mmap Does

`mmap()` maps a file (or anonymous memory) into the process's virtual address space. After mapping, reading/writing to the memory range IS reading/writing the file — via the page cache.

```c
void *addr = mmap(NULL, length, PROT_READ | PROT_WRITE,
                  MAP_SHARED, fd, offset);
// MAP_SHARED: writes visible to other processes mapping the same file
// MAP_PRIVATE: copy-on-write — writes not visible to others
```

### How mmap Works Internally

```
mmap(NULL, 4096, PROT_READ, MAP_SHARED, fd, 0)
    │
    ▼
sys_mmap() → ksys_mmap_pgoff()
    │
    ▼
vm_mmap() → do_mmap()
    │
    ├─► mmap_region()
    │     ├─── alloc_vma()         // allocate struct vm_area_struct
    │     │     VMA: {start, end, flags, vm_file=file, pgoff}
    │     ├─── call file->f_op->mmap(file, vma)
    │     │     └─── generic_file_mmap():
    │     │           vma->vm_ops = &generic_file_vm_ops
    │     └─── vma_link() → insert into mm->mm_mt (maple tree)
    │
    └─► return virtual address (NO physical pages allocated yet!)

// When process first accesses the address:
  page fault → do_page_fault() → handle_mm_fault()
      │
      └─► vm_area_struct's vm_ops->fault() 
            └─── filemap_fault()
                  ├─── find_get_page(): check page cache
                  ├─── [hit]: map page into PTE
                  └─── [miss]: alloc page, call readpage, map PTE
```

### struct vm_area_struct

```c
struct vm_area_struct {
    unsigned long   vm_start;       // start address (inclusive)
    unsigned long   vm_end;         // end address (exclusive)
    pgprot_t        vm_page_prot;   // PTE protection bits
    unsigned long   vm_flags;       // VM_READ, VM_WRITE, VM_EXEC, VM_SHARED
    struct mm_struct *vm_mm;        // owning mm
    struct file     *vm_file;       // mapped file (NULL for anonymous)
    unsigned long   vm_pgoff;       // offset in file (in pages)
    const struct vm_operations_struct *vm_ops;
    void            *vm_private_data;
    // rb/maple tree links for efficient lookup
    struct anon_vma *anon_vma;      // for anonymous/COW pages
    struct list_head anon_vma_chain;
    // ...
};

struct vm_operations_struct {
    void (*open)(struct vm_area_struct *);
    void (*close)(struct vm_area_struct *);
    int  (*fault)(struct vm_fault *);       // page-not-present fault
    int  (*huge_fault)(struct vm_fault *, enum page_entry_size);
    void (*map_pages)(struct vm_fault *, pgoff_t, pgoff_t); // readahead
    vm_fault_t (*page_mkwrite)(struct vm_fault *);           // page about to be written
    vm_fault_t (*pfn_mkwrite)(struct vm_fault *);
    int  (*access)(struct vm_area_struct *, unsigned long, void *, int, int);
    // ...
};
```

### Copy-on-Write (CoW) via mmap

```
Parent forks → child gets copy of page table entries pointing to SAME pages
Page is marked read-only in both parent and child

Child writes to page →  hardware write-protection fault
    └─► do_wp_page()
          ├─── alloc new page
          ├─── copy content from original page
          ├─── update child's PTE to point to new page
          └─── mark both pages writable (only the respective ones)
```

---

## 18. io_uring: Async File I/O Revolution

### The Problem With Traditional I/O

- `read()`/`write()`: synchronous, one syscall per operation
- `aio_read()`/`aio_write()` (POSIX AIO): complex, limited, not truly async for all fd types
- `epoll` + non-blocking I/O: great for sockets, poor for regular files (always "ready")

### io_uring (Linux 5.1+, Jens Axboe)

io_uring provides a **shared-memory ring buffer** between userspace and kernel. Submissions and completions happen through this ring without (ideally) any syscalls.

```
USERSPACE                            KERNEL
─────────                            ──────

┌──────────────────┐                ┌──────────────────┐
│  Submission Ring │                │  Submission Ring  │
│  (SQ: SQEs)      │◄──────────────►│  (read by kernel) │
│                  │  shared memory │                   │
│  head  tail      │                │  head  tail       │
└──────────────────┘                └──────────────────┘
        │ io_uring_enter()                   │
        │ (or SQPOLL kernel thread)          ▼
        │                           process SQEs:
        │                           io_uring_submit_sqes()
        │                             ├─► IORING_OP_READ
        │                             ├─► IORING_OP_WRITE
        │                             ├─► IORING_OP_ACCEPT
        │                             ├─► IORING_OP_CONNECT
        │                             ├─► IORING_OP_OPENAT
        │                             ├─► IORING_OP_CLOSE
        │                             ├─► IORING_OP_STATX
        │                             ├─► IORING_OP_SPLICE
        │                             ├─► IORING_OP_SEND
        │                             ├─► IORING_OP_RECV
        │                             └─► ... 60+ ops

┌──────────────────┐                ┌──────────────────┐
│  Completion Ring │◄───────────────│  Completion Ring  │
│  (CQ: CQEs)      │  shared memory │  (written by      │
│                  │                │   kernel)         │
│  head  tail      │                │  head  tail       │
└──────────────────┘                └──────────────────┘
```

**Core data structures:**

```c
// Submission Queue Entry:
struct io_uring_sqe {
    __u8    opcode;         // IORING_OP_READ, IORING_OP_WRITE, ...
    __u8    flags;          // IOSQE_FIXED_FILE, IOSQE_IO_LINK, ...
    __u16   ioprio;
    __s32   fd;             // file descriptor
    union { __u64 off; __u64 addr2; };  // file offset
    union { __u64 addr; __u64 splice_off_in; };  // buffer address
    __u32   len;            // buffer length
    union { /* op-specific flags */ };
    __u64   user_data;      // returned in CQE (correlation ID)
    // ... 
};

// Completion Queue Entry:
struct io_uring_cqe {
    __u64   user_data;  // matches SQE user_data
    __s32   res;        // result (like syscall return value)
    __u32   flags;      // IORING_CQE_F_MORE, etc.
};
```

**Kernel-side io_uring setup:**

```c
// io_uring_setup() creates:
struct io_ring_ctx {
    struct {
        struct percpu_ref   refs;
        // ... submission side
    } ____cacheline_aligned_in_smp;

    struct io_rings         *rings;          // the actual ring buffers
    unsigned                sq_entries;
    unsigned                cq_entries;
    struct io_sq_data       *sq_data;        // SQPOLL thread (optional)
    struct io_wq            *io_wq;          // worker thread pool
    // fixed files (registered once, accessed by index):
    struct fixed_file_table *fixed_file_table;
    // fixed buffers:
    struct io_mapped_ubuf   **user_bufs;
    // ...
};
```

**SQPOLL mode**: Kernel creates a dedicated thread that polls the SQ ring. Userspace writes SQEs and the kernel thread picks them up **without any syscall**. Zero syscall I/O.

---

## 19. Rust Perspective: Safe Abstractions Over FDs

Rust's type system and ownership model map perfectly onto Linux's fd model.

### The Owned File Descriptor

```rust
// std::os::fd (Rust 1.63+)
use std::os::fd::{OwnedFd, BorrowedFd, RawFd, AsRawFd, IntoRawFd, FromRawFd};

// OwnedFd: owns an fd — closes it when dropped (like unique_ptr)
// BorrowedFd<'a>: borrows an fd — lifetime-scoped reference (like &T)
// RawFd = i32: just the number, no safety guarantees

// Safe wrapper:
pub struct File {
    inner: OwnedFd,  // closes fd on Drop
}

impl Drop for File {
    fn drop(&mut self) {
        // close() is called automatically
        unsafe { libc::close(self.inner.as_raw_fd()); }
    }
}
```

### Building a Safe Pipe Abstraction

```rust
use std::os::fd::{OwnedFd, FromRawFd};
use std::io::{Read, Write};
use libc;

pub struct PipeReader(OwnedFd);
pub struct PipeWriter(OwnedFd);

pub fn create_pipe() -> std::io::Result<(PipeReader, PipeWriter)> {
    let mut fds = [0i32; 2];
    let ret = unsafe { libc::pipe2(fds.as_mut_ptr(), libc::O_CLOEXEC | libc::O_NONBLOCK) };
    if ret == -1 {
        return Err(std::io::Error::last_os_error());
    }
    let reader = PipeReader(unsafe { OwnedFd::from_raw_fd(fds[0]) });
    let writer = PipeWriter(unsafe { OwnedFd::from_raw_fd(fds[1]) });
    Ok((reader, writer))
}

impl Read for PipeReader {
    fn read(&mut self, buf: &mut [u8]) -> std::io::Result<usize> {
        let n = unsafe {
            libc::read(
                self.0.as_raw_fd(),
                buf.as_mut_ptr() as *mut libc::c_void,
                buf.len(),
            )
        };
        if n == -1 {
            Err(std::io::Error::last_os_error())
        } else {
            Ok(n as usize)
        }
    }
}

impl Write for PipeWriter {
    fn write(&mut self, buf: &[u8]) -> std::io::Result<usize> {
        let n = unsafe {
            libc::write(
                self.0.as_raw_fd(),
                buf.as_ptr() as *const libc::c_void,
                buf.len(),
            )
        };
        if n == -1 {
            Err(std::io::Error::last_os_error())
        } else {
            Ok(n as usize)
        }
    }
    fn flush(&mut self) -> std::io::Result<()> { Ok(()) }
}
```

### Rust io_uring with io-uring crate

```rust
use io_uring::{IoUring, opcode, types};
use std::os::unix::io::AsRawFd;
use std::fs::File;

fn main() -> anyhow::Result<()> {
    let mut ring = IoUring::builder()
        .setup_sqpoll(1000) // SQPOLL: 1000ms idle before sleep
        .build(256)?;       // 256-entry ring

    let file = File::open("/etc/passwd")?;
    let mut buf = vec![0u8; 4096];

    // Build a read SQE:
    let read_op = opcode::Read::new(
        types::Fd(file.as_raw_fd()),
        buf.as_mut_ptr(),
        buf.len() as u32,
    )
    .offset(0)
    .build()
    .user_data(0x42); // correlation tag

    unsafe {
        ring.submission()
            .push(&read_op)
            .expect("SQ full");
    }

    ring.submit_and_wait(1)?; // wait for 1 completion

    let cqe = ring.completion().next().expect("no CQE");
    assert_eq!(cqe.user_data(), 0x42);
    let bytes_read = cqe.result() as usize;

    println!("Read {} bytes: {}", bytes_read, 
             std::str::from_utf8(&buf[..bytes_read.min(64)])?);
    Ok(())
}
```

### Rust epoll Abstraction

```rust
use libc::{epoll_create1, epoll_ctl, epoll_wait, epoll_event,
           EPOLL_CTL_ADD, EPOLLIN, EPOLL_CLOEXEC};
use std::os::fd::{OwnedFd, FromRawFd, AsRawFd};

pub struct Epoll(OwnedFd);

impl Epoll {
    pub fn new() -> std::io::Result<Self> {
        let fd = unsafe { epoll_create1(EPOLL_CLOEXEC) };
        if fd == -1 { return Err(std::io::Error::last_os_error()); }
        Ok(Epoll(unsafe { OwnedFd::from_raw_fd(fd) }))
    }

    pub fn add(&self, fd: std::os::fd::RawFd, events: u32, data: u64) -> std::io::Result<()> {
        let mut ev = epoll_event { events, u64: data };
        let ret = unsafe {
            epoll_ctl(self.0.as_raw_fd(), EPOLL_CTL_ADD, fd, &mut ev)
        };
        if ret == -1 { return Err(std::io::Error::last_os_error()); }
        Ok(())
    }

    pub fn wait(&self, events: &mut [epoll_event], timeout_ms: i32) -> std::io::Result<usize> {
        let n = unsafe {
            epoll_wait(
                self.0.as_raw_fd(),
                events.as_mut_ptr(),
                events.len() as i32,
                timeout_ms,
            )
        };
        if n == -1 { return Err(std::io::Error::last_os_error()); }
        Ok(n as usize)
    }
}
```

### Rust timerfd + signalfd unified event loop

```rust
use libc::*;
use std::os::fd::{OwnedFd, FromRawFd, AsRawFd};

fn main() -> std::io::Result<()> {
    let epoll = Epoll::new()?;

    // timerfd: fires every second
    let tfd = unsafe {
        OwnedFd::from_raw_fd({
            let fd = timerfd_create(CLOCK_MONOTONIC, TFD_NONBLOCK | TFD_CLOEXEC);
            let ts = itimerspec {
                it_interval: timespec { tv_sec: 1, tv_nsec: 0 },
                it_value:    timespec { tv_sec: 1, tv_nsec: 0 },
            };
            timerfd_settime(fd, 0, &ts, std::ptr::null_mut());
            fd
        })
    };

    // signalfd: catch SIGINT
    let mut mask: sigset_t = unsafe { std::mem::zeroed() };
    unsafe {
        sigemptyset(&mut mask);
        sigaddset(&mut mask, SIGINT);
        sigprocmask(SIG_BLOCK, &mask, std::ptr::null_mut());
    }
    let sfd = unsafe {
        OwnedFd::from_raw_fd(signalfd(-1, &mask, SFD_NONBLOCK | SFD_CLOEXEC))
    };

    epoll.add(tfd.as_raw_fd(), EPOLLIN as u32, 1)?;
    epoll.add(sfd.as_raw_fd(), EPOLLIN as u32, 2)?;

    let mut events = vec![epoll_event { events: 0, u64: 0 }; 16];
    loop {
        let n = epoll.wait(&mut events, -1)?;
        for ev in &events[..n] {
            match ev.u64 {
                1 => {
                    let mut count = 0u64;
                    unsafe { read(tfd.as_raw_fd(), &mut count as *mut _ as *mut _, 8) };
                    println!("Timer fired {} times", count);
                }
                2 => {
                    println!("SIGINT received, exiting");
                    return Ok(());
                }
                _ => {}
            }
        }
    }
}
```

---

## 20. C Implementation Walkthroughs

### 20.1 Implementing a Virtual Filesystem in Kernel Space

Here's a minimal but complete kernel module implementing a custom filesystem — every file contains the string "hello from myfs\n", generated on the fly:

```c
// myfs.c — minimal VFS filesystem module
#include <linux/module.h>
#include <linux/fs.h>
#include <linux/init.h>
#include <linux/pagemap.h>
#include <linux/seq_file.h>

#define MYFS_MAGIC 0xDEADF5F5

/* ---------- file operations for regular files ---------- */

static ssize_t myfs_read(struct file *filp, char __user *buf,
                          size_t count, loff_t *ppos)
{
    const char *hello = "hello from myfs\n";
    size_t len = strlen(hello);
    
    if (*ppos >= len)
        return 0;  // EOF
    
    count = min(count, len - (size_t)*ppos);
    if (copy_to_user(buf, hello + *ppos, count))
        return -EFAULT;
    
    *ppos += count;
    return count;
}

static const struct file_operations myfs_file_ops = {
    .read   = myfs_read,
    .llseek = generic_file_llseek,
};

/* ---------- inode operations for directories ---------- */

static struct inode *myfs_new_inode(struct super_block *sb, umode_t mode)
{
    struct inode *inode = new_inode(sb);
    if (!inode)
        return NULL;
    
    inode->i_ino   = get_next_ino();
    inode->i_mode  = mode;
    inode->i_atime = inode->i_mtime = inode->i_ctime = current_time(inode);
    inode->i_uid   = current_fsuid();
    inode->i_gid   = current_fsgid();
    
    if (S_ISDIR(mode)) {
        inode->i_op  = &simple_dir_inode_operations;
        inode->i_fop = &simple_dir_operations;
        inc_nlink(inode);  // for "."
    } else if (S_ISREG(mode)) {
        inode->i_op  = &simple_symlink_inode_operations; // minimal
        inode->i_fop = &myfs_file_ops;
        inode->i_size = 16;  // length of "hello from myfs\n"
    }
    
    return inode;
}

/* ---------- superblock setup ---------- */

static int myfs_fill_super(struct super_block *sb, void *data, int silent)
{
    struct inode *root_inode;
    struct dentry *root_dentry;
    
    sb->s_magic     = MYFS_MAGIC;
    sb->s_op        = &simple_super_operations;
    sb->s_blocksize = PAGE_SIZE;
    sb->s_blocksize_bits = PAGE_SHIFT;
    sb->s_maxbytes  = MAX_LFS_FILESIZE;
    
    /* Create root inode */
    root_inode = myfs_new_inode(sb, S_IFDIR | 0755);
    if (!root_inode)
        return -ENOMEM;
    
    /* Create root dentry */
    root_dentry = d_make_root(root_inode);
    if (!root_dentry)
        return -ENOMEM;
    
    sb->s_root = root_dentry;
    
    /* Create a file "hello" under root */
    {
        struct inode *file_inode = myfs_new_inode(sb, S_IFREG | 0444);
        struct dentry *file_dentry;
        
        if (!file_inode)
            return -ENOMEM;
        
        file_dentry = d_alloc_name(root_dentry, "hello");
        if (!file_dentry) {
            iput(file_inode);
            return -ENOMEM;
        }
        
        d_add(file_dentry, file_inode);  // attach inode to dentry
    }
    
    return 0;
}

/* ---------- filesystem type registration ---------- */

static struct dentry *myfs_mount(struct file_system_type *fs_type,
                                  int flags, const char *dev_name, void *data)
{
    // mount_nodev: doesn't need a block device
    return mount_nodev(fs_type, flags, data, myfs_fill_super);
}

static struct file_system_type myfs_type = {
    .owner    = THIS_MODULE,
    .name     = "myfs",
    .mount    = myfs_mount,
    .kill_sb  = kill_litter_super,
};

static int __init myfs_init(void)
{
    return register_filesystem(&myfs_type);
    // After this: mount -t myfs none /mnt/myfs
    // cat /mnt/myfs/hello  → prints "hello from myfs"
}

static void __exit myfs_exit(void)
{
    unregister_filesystem(&myfs_type);
}

module_init(myfs_init);
module_exit(myfs_exit);
MODULE_LICENSE("GPL");
```

### 20.2 Building an epoll-based Server Using Only Primitives

```c
// event_server.c — event loop using epoll, timerfd, signalfd, sockets
#include <sys/epoll.h>
#include <sys/timerfd.h>
#include <sys/signalfd.h>
#include <sys/socket.h>
#include <netinet/in.h>
#include <unistd.h>
#include <signal.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <fcntl.h>
#include <errno.h>

#define MAX_EVENTS 64

static int set_nonblocking(int fd) {
    int flags = fcntl(fd, F_GETFL, 0);
    return fcntl(fd, F_SETFL, flags | O_NONBLOCK);
}

static int create_listen_socket(uint16_t port) {
    int fd = socket(AF_INET6, SOCK_STREAM | SOCK_NONBLOCK | SOCK_CLOEXEC, 0);
    if (fd < 0) { perror("socket"); exit(1); }

    int one = 1;
    setsockopt(fd, SOL_SOCKET, SO_REUSEADDR, &one, sizeof(one));

    struct sockaddr_in6 addr = {
        .sin6_family = AF_INET6,
        .sin6_port   = htons(port),
        .sin6_addr   = in6addr_any,
    };
    if (bind(fd, (struct sockaddr*)&addr, sizeof(addr)) < 0) {
        perror("bind"); exit(1);
    }
    listen(fd, 128);
    return fd;
}

int main(void) {
    int epfd = epoll_create1(EPOLL_CLOEXEC);
    struct epoll_event ev, events[MAX_EVENTS];

    /* --- Listen socket --- */
    int listen_fd = create_listen_socket(8080);
    ev.events  = EPOLLIN | EPOLLET;  // edge-triggered
    ev.data.fd = listen_fd;
    epoll_ctl(epfd, EPOLL_CTL_ADD, listen_fd, &ev);

    /* --- Timer (heartbeat every 5 seconds) --- */
    int tfd = timerfd_create(CLOCK_MONOTONIC, TFD_NONBLOCK | TFD_CLOEXEC);
    struct itimerspec ts = {
        .it_interval = { .tv_sec = 5, .tv_nsec = 0 },
        .it_value    = { .tv_sec = 5, .tv_nsec = 0 },
    };
    timerfd_settime(tfd, 0, &ts, NULL);
    ev.events  = EPOLLIN;
    ev.data.fd = tfd;
    epoll_ctl(epfd, EPOLL_CTL_ADD, tfd, &ev);

    /* --- Signal handler (SIGINT, SIGTERM) --- */
    sigset_t mask;
    sigemptyset(&mask);
    sigaddset(&mask, SIGINT);
    sigaddset(&mask, SIGTERM);
    sigprocmask(SIG_BLOCK, &mask, NULL);
    int sfd = signalfd(-1, &mask, SFD_NONBLOCK | SFD_CLOEXEC);
    ev.events  = EPOLLIN;
    ev.data.fd = sfd;
    epoll_ctl(epfd, EPOLL_CTL_ADD, sfd, &ev);

    printf("Listening on :8080\n");

    for (;;) {
        int n = epoll_wait(epfd, events, MAX_EVENTS, -1);
        if (n < 0 && errno == EINTR) continue;

        for (int i = 0; i < n; i++) {
            int fd = events[i].data.fd;

            if (fd == listen_fd) {
                /* Accept new connections */
                for (;;) {
                    int cfd = accept4(listen_fd, NULL, NULL,
                                      SOCK_NONBLOCK | SOCK_CLOEXEC);
                    if (cfd < 0) break;  // EAGAIN or EWOULDBLOCK
                    printf("New connection fd=%d\n", cfd);
                    ev.events  = EPOLLIN | EPOLLET;
                    ev.data.fd = cfd;
                    epoll_ctl(epfd, EPOLL_CTL_ADD, cfd, &ev);
                }
            } else if (fd == tfd) {
                uint64_t count;
                read(tfd, &count, sizeof(count));
                printf("Heartbeat (tick %lu)\n", count);
            } else if (fd == sfd) {
                struct signalfd_siginfo si;
                read(sfd, &si, sizeof(si));
                printf("Signal %u received, shutting down\n", si.ssi_signo);
                close(epfd);
                return 0;
            } else {
                /* Data from a client */
                char buf[4096];
                ssize_t r = read(fd, buf, sizeof(buf));
                if (r <= 0) {
                    epoll_ctl(epfd, EPOLL_CTL_DEL, fd, NULL);
                    close(fd);
                    printf("Connection fd=%d closed\n", fd);
                } else {
                    /* Echo back */
                    write(fd, buf, r);
                }
            }
        }
    }
}
```

### 20.3 Reading /proc/PID/maps Programmatically

```c
// parse_maps.c — read and parse /proc/self/maps
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

struct vma_entry {
    unsigned long start, end;
    char perms[5];
    unsigned long offset;
    int dev_major, dev_minor;
    unsigned long inode;
    char pathname[512];
};

int parse_maps(pid_t pid) {
    char path[64];
    snprintf(path, sizeof(path), "/proc/%d/maps", pid);

    FILE *f = fopen(path, "r");
    if (!f) { perror("fopen"); return -1; }

    char line[1024];
    while (fgets(line, sizeof(line), f)) {
        struct vma_entry e = {0};
        sscanf(line, "%lx-%lx %4s %lx %x:%x %lu %511s",
               &e.start, &e.end, e.perms, &e.offset,
               &e.dev_major, &e.dev_minor, &e.inode, e.pathname);

        printf("[%016lx-%016lx] %s off=%lx %s\n",
               e.start, e.end, e.perms, e.offset,
               e.pathname[0] ? e.pathname : "[anonymous]");
    }

    fclose(f);
    return 0;
}

int main(void) {
    return parse_maps(getpid());
}
// Output example:
// [00400000-00401000] r--p off=0 /usr/bin/cat
// [00401000-00402000] r-xp off=1000 /usr/bin/cat
// [7fff...] rw-p off=0 [stack]
// [7f...  ] r-xp off=0 /lib/x86_64-linux-gnu/libc.so.6
```

### 20.4 Custom Character Device Driver

```c
// chardev.c — minimal character device driver
#include <linux/module.h>
#include <linux/fs.h>
#include <linux/cdev.h>
#include <linux/uaccess.h>
#include <linux/device.h>
#include <linux/slab.h>

#define DEVICE_NAME "chardev_example"
#define BUF_SIZE    1024

static int    major;
static struct class  *dev_class;
static struct cdev    chardev_cdev;
static char          *kernel_buf;

static int chardev_open(struct inode *inode, struct file *file)
{
    pr_info("chardev: opened by pid %d\n", current->pid);
    return 0;
}

static int chardev_release(struct inode *inode, struct file *file)
{
    pr_info("chardev: closed\n");
    return 0;
}

static ssize_t chardev_read(struct file *file, char __user *buf,
                              size_t count, loff_t *ppos)
{
    size_t len = strlen(kernel_buf);
    if (*ppos >= len) return 0;
    count = min(count, len - (size_t)*ppos);
    if (copy_to_user(buf, kernel_buf + *ppos, count))
        return -EFAULT;
    *ppos += count;
    return count;
}

static ssize_t chardev_write(struct file *file, const char __user *buf,
                               size_t count, loff_t *ppos)
{
    count = min(count, (size_t)(BUF_SIZE - 1));
    memset(kernel_buf, 0, BUF_SIZE);
    if (copy_from_user(kernel_buf, buf, count))
        return -EFAULT;
    kernel_buf[count] = '\0';
    pr_info("chardev: received %zu bytes: %s\n", count, kernel_buf);
    return count;
}

static long chardev_ioctl(struct file *file, unsigned int cmd, unsigned long arg)
{
    switch (cmd) {
    case 0x1001:  // example: clear buffer
        memset(kernel_buf, 0, BUF_SIZE);
        return 0;
    default:
        return -ENOTTY;  // "not a typewriter" — standard ioctl error
    }
}

static __poll_t chardev_poll(struct file *file, poll_table *wait)
{
    // This device is always readable and writable:
    return EPOLLIN | EPOLLRDNORM | EPOLLOUT | EPOLLWRNORM;
}

static const struct file_operations chardev_fops = {
    .owner          = THIS_MODULE,
    .open           = chardev_open,
    .release        = chardev_release,
    .read           = chardev_read,
    .write          = chardev_write,
    .unlocked_ioctl = chardev_ioctl,
    .poll           = chardev_poll,
};

static int __init chardev_init(void)
{
    dev_t dev;
    
    kernel_buf = kzalloc(BUF_SIZE, GFP_KERNEL);
    if (!kernel_buf) return -ENOMEM;
    strcpy(kernel_buf, "initial content\n");
    
    // Dynamically allocate major number:
    alloc_chrdev_region(&dev, 0, 1, DEVICE_NAME);
    major = MAJOR(dev);
    
    cdev_init(&chardev_cdev, &chardev_fops);
    cdev_add(&chardev_cdev, dev, 1);
    
    // Create /dev/chardev_example via udev:
    dev_class = class_create(THIS_MODULE, DEVICE_NAME);
    device_create(dev_class, NULL, dev, NULL, DEVICE_NAME);
    
    pr_info("chardev: registered with major %d\n", major);
    return 0;
}

static void __exit chardev_exit(void)
{
    dev_t dev = MKDEV(major, 0);
    device_destroy(dev_class, dev);
    class_destroy(dev_class);
    cdev_del(&chardev_cdev);
    unregister_chrdev_region(dev, 1);
    kfree(kernel_buf);
    pr_info("chardev: unregistered\n");
}

module_init(chardev_init);
module_exit(chardev_exit);
MODULE_LICENSE("GPL");
// Usage:
//   echo "hello" > /dev/chardev_example
//   cat /dev/chardev_example  → "hello"
//   ioctl(fd, 0x1001, 0)     → clears buffer
```

### 20.5 Writing a procfs File

```c
// procfile.c — expose kernel data via /proc/my_proc_file
#include <linux/module.h>
#include <linux/proc_fs.h>
#include <linux/seq_file.h>

static unsigned long event_count = 0;

static int my_proc_show(struct seq_file *m, void *v)
{
    seq_printf(m, "Kernel module: %s\n", THIS_MODULE->name);
    seq_printf(m, "Event count: %lu\n", event_count++);
    seq_printf(m, "Jiffies: %lu\n", jiffies);
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

static struct proc_dir_entry *proc_entry;

static int __init procfile_init(void)
{
    proc_entry = proc_create("my_proc_file", 0444, NULL, &my_proc_ops);
    if (!proc_entry) return -ENOMEM;
    pr_info("procfile: created /proc/my_proc_file\n");
    return 0;
}

static void __exit procfile_exit(void)
{
    proc_remove(proc_entry);
    pr_info("procfile: removed\n");
}

module_init(procfile_init);
module_exit(procfile_exit);
MODULE_LICENSE("GPL");
// cat /proc/my_proc_file
// → Kernel module: procfile
// → Event count: 42
// → Jiffies: 4297812345
```

---

## 21. Mental Model Summary

### The Full Picture — Architecture in ASCII

```
╔══════════════════════════════════════════════════════════════════════╗
║                        USERSPACE                                      ║
║                                                                        ║
║  Process A           Process B           Process C                     ║
║  fd_table:           fd_table:           fd_table:                     ║
║  [0]=stdin           [0]=stdin           [0]=stdin                     ║
║  [1]=stdout          [3]=socket          [3]=pipe-read                 ║
║  [3]=regular file    [4]=timerfd         [4]=memfd                     ║
║  [4]=epoll           [5]=signalfd        [5]=perf_event                ║
║                                                                        ║
║  All access via: read() write() open() close() ioctl() mmap() poll()   ║
╚══════════════════════╦═══════════════════════════════════════════════╝
                       ║  system call boundary (int 0x80 / syscall)
╔══════════════════════╩═══════════════════════════════════════════════╗
║                    SYSCALL LAYER                                       ║
║  sys_read → vfs_read → file->f_op->read_iter                          ║
╚══════════════════════╦═══════════════════════════════════════════════╝
                       ║
╔══════════════════════╩═══════════════════════════════════════════════╗
║                  VFS (Virtual Filesystem Switch)                       ║
║                                                                        ║
║  struct file ──► struct inode ──► file_operations (vtable)            ║
║  struct dentry (dcache) ──► pathname resolution                       ║
║  struct super_block ──► per-filesystem state                          ║
║  struct address_space ──► page cache (all file data lives here)       ║
╚══┬────────┬──────────┬────────────┬───────────┬──────────────────────╝
   │        │          │            │           │
╔══╩══╗ ╔══╩══╗ ╔═════╩═╗ ╔═══════╩═╗ ╔═══════╩═══╗
║ext4 ║ ║btrfs║ ║ tmpfs ║ ║ procfs  ║ ║  sockfs   ║
║xfs  ║ ║f2fs ║ ║ ramfs ║ ║ sysfs   ║ ║  pipefs   ║
║     ║ ║     ║ ║ devtmp║ ║ cgroupfs║ ║  eventfd  ║
╚══╦══╝ ╚══╦══╝ ╚═══════╝ ╚═════════╝ ║  timerfd  ║
   ║        ║                          ╚═══════════╝
╔══╩════════╩════════════════════════════════════╗
║             BLOCK LAYER                         ║
║  bio → request queue → I/O scheduler           ║
╚══╦════════════╦══════════════════════════════╝
   ║            ║
╔══╩══╗    ╔════╩════╗
║NVMe ║    ║  SCSI   ║
║     ║    ║  SATA   ║
║     ║    ║  USB    ║
╚═════╝    ╚═════════╝

MEMORY SUBSYSTEM (cuts across everything):
╔══════════════════════════════════════════════════════════╗
║                    Page Cache                             ║
║  xarray of folios, per address_space (per inode)         ║
║  shared by: read(), write(), mmap(), sendfile(), splice() ║
╚══════════════════════════════════════════════════════════╝
```

### Conceptual Map: Everything Really Is a Fd

```
ABSTRACT CONCEPT          KERNEL OBJECT              FD TYPE / CREATION
───────────────────────────────────────────────────────────────────────
regular file data         struct file + inode         open()
directory listing         struct file + inode (dir)   open(O_DIRECTORY)
raw block device          struct file + block_dev     open("/dev/sda")
byte-stream device        struct file + cdev          open("/dev/tty")
TCP/UDP connection        struct socket + sock        socket()
UNIX domain socket        struct socket + inode       socket(AF_UNIX)
IPC byte stream           struct pipe_inode_info      pipe()
IPC with pathname         struct pipe_inode_info      mkfifo() + open()
timer                     struct timerfd_ctx          timerfd_create()
signal queue              struct signalfd_ctx         signalfd()
event counter             struct eventfd_ctx          eventfd()
epoll interest set        struct eventpoll            epoll_create1()
process handle            struct pid                  pidfd_open()
anonymous memory          struct file + shmem_inode   memfd_create()
kernel perf counter       struct perf_event           perf_event_open()
eBPF map                  struct bpf_map              bpf(BPF_MAP_CREATE)
eBPF program              struct bpf_prog             bpf(BPF_PROG_LOAD)
async I/O ring            struct io_ring_ctx          io_uring_setup()
user fault handler        struct userfaultfd_ctx      userfaultfd()
filesystem notification   struct inotify_inode_mark   inotify_init1()
file access control       struct fsnotify_mark        fanotify_init()
kernel namespace          struct ns_common            /proc/PID/ns/net
GPU command buffer        struct drm_file             open("/dev/dri/card0")
```

### Key Invariants to Keep in Your Mental Model

1. **An fd is always just an integer**. Its meaning is entirely in the kernel's fd table.

2. **Every fd points to a `struct file`**. The `struct file` holds the `file_operations` vtable that defines the behavior of all generic operations on that fd.

3. **The vtable is the polymorphism mechanism**. `read(fd, ...)` doesn't know if it's reading a TCP socket, a regular file, a pipe, or a timerfd. It calls `f_op->read_iter()`, and each type implements it differently.

4. **The page cache is the universal buffer**. All regular file data flows through it. `read()`, `write()`, `mmap()`, `sendfile()`, `splice()` all operate on the same page cache.

5. **Inodes are independent of names**. An inode can have 0 (deleted but open), 1, or many names. Hard links are multiple dentries pointing to the same inode.

6. **Dentries are a cache of name→inode mappings**. They are reconstructed from the on-disk directory format on demand and evicted under memory pressure.

7. **Mount namespaces isolate the VFS view**. Each process has a mount namespace. What you see at `/proc` depends on which PID namespace you're in. What you see at `/` depends on your mount namespace.

8. **Everything that happens at the VFS level can be intercepted**. LSMs (SELinux, AppArmor, eBPF LSM), inotify/fanotify, strace (via ptrace), eBPF programs — they all hook into the VFS call paths.

9. **epoll, poll, select unify event waiting**. Because everything is an fd, you can wait for a timer, a signal, a socket message, a child process exit, a file change, and a new connection — all in one `epoll_wait()` call.

10. **File descriptors can be passed between processes**. Via `fork()` (inheritance), `dup()`/`dup2()` (duplication), and `SCM_RIGHTS` over Unix domain sockets (explicit transfer). This is how privilege separation architectures (like OpenSSH, Chrome's sandbox, systemd's socket activation) work.

---

*This document covers the Linux VFS and "everything is a file" design philosophy from first principles through kernel data structures, memory subsystems, system call paths, device model, special filesystems, networking, IPC primitives, async I/O, and practical C/Rust implementations. The mental model: one integer (fd), one vtable (file_operations), infinite composability.*
