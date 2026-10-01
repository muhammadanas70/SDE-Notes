# io_uring on Linux — A Deep, Systems-Level Guide

> **Scope:** This is a systems-programming guide to Linux `io_uring`, from the mental model and shared-memory protocol through kernel internals, operation families, memory ordering, resource registration, networking, storage, performance, debugging, security, and practical C/Rust implementations.
>
> **Version note:** `io_uring` is a living Linux UAPI. New opcodes, flags, registration commands, and restrictions are added over time. Code should probe capabilities and treat kernel-version differences as normal. This guide therefore distinguishes stable concepts from version-sensitive features.
>
> **Primary references:** Linux UAPI/header and kernel documentation, `io_uring(7)`, `io_uring_setup(2)`, `io_uring_enter(2)`, `io_uring_register(2)`, liburing, and the Rust `io-uring` crate.

---

## 1. The shortest useful mental model

Think of `io_uring` as a **shared-memory command/completion protocol plus a small set of syscalls used to create, wake, and configure it**.

Instead of:

```text
application
    |
    | read(fd, buf, n)
    v
kernel
    |
    | blocks / performs I/O
    v
application resumes
```

you can have:

```text
application                         kernel
    |                                  |
    | write SQE into shared SQ         |
    |--------------------------------->|
    |                                  | consumes SQE
    |                                  | starts/continues operation
    |                                  |
    |                         CQE <-----|
    |                                  |
    | read CQE from shared CQ          |
    |<---------------------------------|
    |
    | result = cqe->res
```

The key idea is **not "a faster read syscall."**

The key idea is:

> **Describe work in memory, publish it to the kernel, let the kernel execute it asynchronously, and receive results through another shared ring.**

This enables batching: many operations can be described with many SQEs and submitted with one `io_uring_enter()` call. With submission polling, the kernel can consume SQEs without a syscall on every batch.

---

# 2. Why io_uring exists

Linux historically offered several I/O models:

### Blocking I/O

```c
read(fd, buf, len);
```

Simple, but the calling thread may sleep.

### Nonblocking I/O

```c
read(fd, buf, len);  /* returns EAGAIN when not ready */
```

The application must repeatedly determine readiness.

### `select`, `poll`, `epoll`

These primarily answer:

> "Which descriptors are ready?"

The application still performs the actual I/O.

Typical `epoll` loop:

```text
epoll_wait()
     |
     +--> fd 10 readable
     |
     +--> fd 14 writable
     |
     v
read(fd 10)
write(fd 14)
```

### POSIX AIO

Linux implementations historically had limitations and did not provide a uniform high-performance asynchronous model for the broad set of modern I/O operations.

### io_uring

io_uring moves toward:

```text
submit operation descriptions
            |
            v
        kernel
            |
     asynchronous work
            |
            v
      completion events
```

The important distinction is:

- `epoll`: **readiness notification**
- `io_uring`: **operation submission + completion notification**

That does not mean io_uring makes every operation magically asynchronous. The kernel may execute an operation inline, asynchronously, through worker threads, through polling, or through device-specific asynchronous machinery.

---

# 3. The architecture

A realistic conceptual architecture looks like this:

```text
                         USER SPACE
+----------------------------------------------------------------+
| Application                                                    |
|                                                                |
|  producer logic                                                |
|       |                                                        |
|       | fill struct io_uring_sqe                               |
|       v                                                        |
|  +----------------------+                                      |
|  | Submission Queue     |                                      |
|  | SQE 0 | SQE 1 | ...  |                                      |
|  +----------+-----------+                                      |
|             | shared memory                                    |
|             |                                                  |
|             v                                                  |
|  +----------------------+                                      |
|  | SQ metadata           |                                      |
|  | head/tail/mask/array  |                                      |
|  +----------+-----------+                                      |
|             |                                                  |
|             | mmap()                                           |
+-------------|--------------------------------------------------+
              |
==============|==================================================
              | USER/KERNEL SHARED MEMORY
              |
+-------------|--------------------------------------------------+
|             v                 KERNEL SPACE                     |
|  +----------------------+                                      |
|  | io_ring_ctx          |                                      |
|  | ring state/resources |                                      |
|  +----------+-----------+                                      |
|             |                                                  |
|             v                                                  |
|      SQE fetch / validation                                    |
|             |                                                  |
|       +-----+------+----------------------------------+         |
|       |            |                                  |         |
|       v            v                                  v         |
|  VFS/filesystem   networking                    io-wq workers  |
|       |            |                                  |         |
|       v            v                                  v         |
|  block layer    socket stack                    blocking work  |
|       |            |                                  |         |
|       +------------+------------------+---------------+         |
|                                        |                         |
|                                        v                         |
|                              completion generation              |
|                                        |                         |
|                                        v                         |
|                              +------------------+                |
|                              | Completion Queue |                |
|                              | CQE 0 | CQE 1... |                |
|                              +--------+---------+                |
+---------------------------------------|--------------------------+
                                        |
========================================|==========================
                                        |
                         USER SPACE      |
                                        v
                              CQE consumer / event loop
```

The exact kernel path differs by operation.

For example:

```text
SQE
 |
 +--> IORING_OP_READ
 |       |
 |       +--> io_read()
 |               |
 |               +--> VFS
 |                     |
 |                     +--> filesystem
 |                           |
 |                           +--> page cache / block layer
 |
 +--> IORING_OP_RECV
 |       |
 |       +--> socket receive path
 |
 +--> IORING_OP_ACCEPT
 |       |
 |       +--> TCP listening socket
 |
 +--> IORING_OP_TIMEOUT
         |
         +--> kernel timer machinery
```

The SQ/CQ are the common transport. The operation implementation is operation-specific.

---

# 4. The three fundamental data structures

At the UAPI level, the important objects are:

1. `struct io_uring_params`
2. `struct io_uring_sqe`
3. `struct io_uring_cqe`

Conceptually:

```text
io_uring_setup()
        |
        v
struct io_uring_params
        |
        +---- tells kernel what kind of ring you want
        |
        v
shared ring memory
        |
        +---- SQ metadata
        +---- SQE array
        +---- CQ metadata
        +---- CQE array
```

---

# 5. Submission Queue Entry — SQE

An SQE is a command descriptor.

Conceptually it says:

```text
opcode = READ
fd     = 7
addr   = 0x7f...
len    = 4096
offset = 8192
user_data = 1234
```

The actual structure is deliberately packed to accommodate many operation types.

Important fields include:

- `opcode`
- `flags`
- `ioprio`
- `fd`
- `off`
- `addr`
- `len`
- `rw_flags`
- `user_data`
- `buf_index`
- `personality`
- operation-specific unions/fields

Do not treat the entire SQE as if every field has one universal meaning.

Instead:

> **The opcode defines the semantic interpretation of the other fields.**

---

# 6. Completion Queue Entry — CQE

A CQE reports the result.

Conceptually:

```c
struct io_uring_cqe {
    __u64 user_data;
    __s32 res;
    __u32 flags;
};
```

The most important fields are:

### `user_data`

Opaque application-owned identity.

Typical usage:

```c
sqe->user_data = (uintptr_t)request;
```

Then:

```c
struct request *req =
    (struct request *)(uintptr_t)cqe->user_data;
```

Or encode a small integer:

```c
sqe->user_data = request_id;
```

### `res`

The operation result.

For reads:

```text
res > 0  number of bytes
res == 0 EOF
res < 0  -errno
```

This is a critical difference from ordinary libc wrappers.

An io_uring CQE usually reports errors as:

```c
-res
```

For example:

```text
-EAGAIN
-ENOENT
-EINVAL
```

rather than setting the thread-local `errno` in the normal syscall-wrapper style.

### `flags`

Per-completion flags.

These are especially important for:

- buffer selection
- multishot operations
- more advanced completion semantics

---

# 7. The two-ring mental model

The simplest way to reason about io_uring is:

```text
                 USER                         KERNEL

             SQ producer
                  |
                  v
        +-------------------+
        | SQE SQE SQE SQE   |
        +-------------------+
                  |
                  | submit
                  v
             kernel consumes
                  |
                  |
             executes I/O
                  |
                  v
        +-------------------+
        | CQE CQE CQE CQE   |
        +-------------------+
                  ^
                  |
             user consumes
```

The Submission Queue is primarily:

```text
user -> kernel
```

The Completion Queue is primarily:

```text
kernel -> user
```

This directionality is crucial when thinking about concurrency and memory ordering.

---

# 8. Ring indexes are not pointers

The queues use monotonically increasing head/tail counters.

Conceptually:

```text
logical index = counter
array index   = counter & ring_mask
```

For a power-of-two ring:

```c
index = counter & (entries - 1);
```

Example:

```text
entries = 8
mask    = 7

counter: 0 1 2 3 4 5 6 7 8 9 ...
index:   0 1 2 3 4 5 6 7 0 1 ...
```

The counter itself continues growing.

The mask maps it into the physical ring.

This solves wraparound without storing enormous amounts of state.

---

# 9. Why monotonically increasing counters matter

Suppose:

```text
head = 100
tail = 108
```

Then:

```text
pending = tail - head = 8
```

Even though the physical indexes have wrapped, the logical counters preserve ordering.

This is the standard bounded-ring technique.

The conceptual invariant is:

```text
0 <= tail - head <= ring_capacity
```

for a correctly maintained single-producer/single-consumer style queue.

io_uring has additional synchronization and kernel-side details, but this is the correct starting mental model.

---

# 10. Memory ordering is not optional

One of the most important io_uring concepts is:

> **Shared memory does not automatically mean shared ordering.**

Modern CPUs and compilers can reorder ordinary memory accesses.

Consider:

```c
sqe->fd = fd;
sqe->addr = buffer;
sqe->len = len;

sq_tail = new_tail;
```

The kernel must not observe:

```text
sq_tail updated
```

before observing:

```text
SQE fields updated
```

Therefore publication requires release semantics.

Conceptually:

```text
write SQE fields
      |
      | store-release
      v
publish SQ tail
```

The consumer performs an acquire operation:

```text
load-acquire SQ tail
      |
      v
read SQE
```

This creates the required happens-before relationship.

The same idea applies to CQ consumption/publication.

This is why raw io_uring examples contain explicit memory barriers and why using liburing is valuable: it encapsulates much of the delicate queue manipulation.

The official examples demonstrate acquire/release operations around the ring indexes. citeturn0search3turn0search10

---

# 11. Setup: io_uring_setup

At the raw UAPI level:

```c
int ring_fd = syscall(
    __NR_io_uring_setup,
    entries,
    &params
);
```

The syscall creates an io_uring instance.

The application supplies:

```text
requested queue entries
+
setup parameters
```

The kernel returns:

```text
ring file descriptor
+
filled io_uring_params
```

The parameters describe offsets and features needed to map the ring.

Important conceptual distinction:

```text
io_uring_setup()
    = create/configure the ring

io_uring_enter()
    = submit/wait/drive the ring

io_uring_register()
    = register resources/features
```

---

# 12. Why io_uring uses mmap

The shared queues need to be visible to both user space and kernel space.

The traditional model is:

```text
userspace buffer
      |
      | syscall copies data
      v
kernel
```

The io_uring queue model is:

```text
             same physical memory
                    |
         +----------+----------+
         |                     |
     user mapping         kernel mapping
         |                     |
         +----------+----------+
                    |
                ring data
```

The exact VM implementation is more nuanced, but the key optimization is:

> **The command/completion metadata lives in shared mappings rather than requiring a syscall argument structure to be copied for every operation.**

This does not mean io_uring never copies payload data. File/network I/O may still involve normal kernel and device memory movement. "Zero copy" is operation-specific, not a universal property of io_uring.

---

# 13. io_uring_enter

`io_uring_enter()` is the primary control syscall.

Conceptually:

```c
io_uring_enter(
    ring_fd,
    to_submit,
    min_complete,
    flags,
    sig,
    sigsz
);
```

It can be used to:

- tell the kernel that SQEs are ready
- wait for completions
- combine submission and waiting
- perform synchronization/control operations depending on flags and setup

The important optimization is batching:

```text
prepare SQE #1
prepare SQE #2
prepare SQE #3
prepare SQE #4

             |
             v

       one enter syscall
```

instead of:

```text
syscall
syscall
syscall
syscall
```

The liburing manual explicitly emphasizes batching as a central advantage. citeturn0search2turn0search3

---

# 14. Submission is not execution

A common beginner mistake is:

```text
push SQE
    =
operation started immediately
```

Not necessarily.

There are multiple stages:

```text
construct SQE
      |
      v
publish SQ entry
      |
      v
kernel notices/submits it
      |
      v
kernel validates it
      |
      v
operation is dispatched
      |
      v
operation executes
      |
      v
completion is produced
```

Depending on setup and workload, some stages can happen very close together or in the same context.

The conceptual distinction remains important.

---

# 15. Completion is not necessarily "the whole story"

For simple operations:

```text
one SQE
   |
   v
one CQE
```

For advanced operations:

```text
one SQE
   |
   +--> CQE
   +--> CQE
   +--> CQE
   +--> ...
```

This is the idea behind **multishot operations**.

Examples include multishot:

- accept
- receive
- poll
- other operation families as supported by the running kernel

A multishot request can remain active and produce multiple CQEs until terminated or invalidated.

Therefore:

> **Do not automatically free the request object after the first CQE if the SQE represents a multishot operation.**

---

# 16. `user_data` is your correlation mechanism

A scalable event loop needs to answer:

> "Which logical request generated this completion?"

Use `user_data`.

Example:

```c
struct request {
    int fd;
    void *buf;
    size_t len;
    int state;
};

struct request *req = ...;

sqe->user_data = (uintptr_t)req;
```

Completion:

```c
struct request *req =
    (struct request *)(uintptr_t)cqe->user_data;
```

This is essentially an application-level pointer-sized correlation token.

It is often better than maintaining a separate hash table keyed by CQ position.

---

# 17. The kernel-side mental model

The kernel has a context object representing the ring.

Conceptually:

```text
io_ring_ctx
 |
 +-- submission queue state
 |
 +-- completion queue state
 |
 +-- registered files
 |
 +-- registered buffers
 |
 +-- personalities
 |
 +-- work queues
 |
 +-- request tracking
 |
 +-- polling state
 |
 +-- task-work/deferred execution state
 |
 +-- cancellation state
 |
 +-- resource accounting
```

The implementation has evolved substantially over kernel releases, so source-level field names and helper functions should be checked against the exact kernel tree being studied.

Historically important kernel files include:

```text
fs/io_uring.c
fs/io-wq.c
fs/io-wq.h
include/linux/io_uring.h
include/uapi/linux/io_uring.h
tools/io_uring/
```

These are identified in Linux's maintainer documentation. citeturn1search0turn1search3

---

# 18. `io_kiocb`: the kernel request mental model

A useful kernel-level concept is the internal request object commonly represented by `struct io_kiocb`.

Think:

```text
SQE
 |
 | parsed/validated
 v
kernel request state
 |
 +-- operation-specific state
 +-- file/resource references
 +-- cancellation state
 +-- completion state
 +-- async context
```

The user-space SQE is a compact command.

The kernel cannot rely on that SQE remaining available as the complete lifetime state of the operation.

Therefore it creates/maintains internal request state.

This is one of the most important conceptual transitions:

> **SQE is the command descriptor; kernel request state is the lifetime object.**

---

# 19. The operation dispatch model

A useful simplified model:

```text
                SQE
                 |
                 v
        identify opcode
                 |
                 v
        operation handler
                 |
        +--------+---------+
        |                  |
        v                  v
   completes now       needs async work
        |                  |
        |                  v
        |              defer/workqueue/
        |              task work/device
        |                  |
        +--------+---------+
                 |
                 v
             CQE
```

Not every asynchronous operation uses the same mechanism.

Some can execute synchronously in the submitting context.

Some block.

Some are delegated to io-wq.

Some rely on filesystem/network/device asynchronous behavior.

Some use polling.

Some use task work to complete later in an appropriate task context.

---

# 20. io-wq: why worker threads exist

A fundamental fact:

> Linux cannot make a fundamentally blocking backend magically nonblocking merely by changing the API.

Suppose an operation eventually reaches a filesystem path that blocks.

io_uring can delegate blocking work to an internal worker mechanism, historically known as **io-wq**.

Conceptually:

```text
io_uring request
       |
       v
can execute without blocking?
       |
    +--+--+
    |     |
   yes    no
    |     |
    v     v
 execute  io-wq
 inline     |
            v
        worker thread
            |
            v
        blocking backend
```

This lets the application use a uniform completion model even when the backend cannot provide true nonblocking execution.

But there is an important performance consequence:

> **"Asynchronous API" does not imply "no kernel threads."**

If the workload causes heavy io-wq usage, you should reason about worker scheduling, CPU consumption, filesystem behavior, and concurrency limits.

---

# 21. SQPOLL

`IORING_SETUP_SQPOLL` enables a kernel thread to poll the submission queue.

Conceptually:

```text
application
    |
    | writes SQEs
    v
shared SQ
    ^
    |
kernel SQ polling thread
    |
    v
dispatch operations
```

This can reduce the need for `io_uring_enter()` calls for submission.

The trade-off:

```text
less syscall overhead
        versus
dedicated kernel polling CPU activity
```

SQPOLL is therefore a throughput/latency optimization, not a free speedup.

The liburing documentation describes submission queue polling as a way to avoid the `io_uring_enter()` call used to notify the kernel about SQEs. citeturn0search3

---

# 22. Busy polling versus SQPOLL

Do not conflate:

### SQPOLL

Kernel polls for **submitted SQEs**.

### Network busy polling

Kernel/network stack may poll hardware/network state to reduce interrupt/scheduling latency.

They solve different problems.

A useful distinction:

```text
SQPOLL:
    "How do I notice new commands quickly?"

Network polling:
    "How do I notice network packets quickly?"
```

---

# 23. CQ waiting

An application can:

```text
submit
+
wait for completion
```

with appropriate `io_uring_enter()` semantics.

Typical high-level loop:

```text
while (running) {
    prepare requests
    submit
    wait for at least N completions
    consume CQEs
    process results
}
```

The application can choose:

- polling
- blocking waits
- event-loop integration
- batched consumption
- hybrid strategies

---

# 24. Ring capacity and backpressure

The SQ has finite capacity.

If the producer gets ahead:

```text
application
    |
    v
SQ full
```

the application must:

- submit pending SQEs
- wait for capacity
- reduce concurrency
- or otherwise apply backpressure

Similarly, the CQ can overflow if completions are generated faster than the application consumes them.

The lesson is:

> **io_uring does not remove queueing theory. It makes queueing explicit.**

You still need:

```text
producer rate
consumer rate
queue capacity
latency
backpressure
```

---

# 25. Queue depth is a performance parameter, not a magic number

Too small:

```text
QD = 1
```

means little parallelism.

Too large:

```text
QD = 100000
```

can cause:

- memory overhead
- excessive outstanding I/O
- device queue pressure
- cache pressure
- scheduler pressure
- higher tail latency
- resource exhaustion

A good queue depth depends on:

- device
- filesystem
- request size
- workload
- CPU count
- latency target
- network behavior
- storage queue depth

Measure it.

---

# 26. Operation families

io_uring has a large and evolving opcode set.

The important families are:

### File I/O

- read
- write
- readv
- writev
- preadv
- pwritev
- fixed-buffer variants
- newer vector/extended variants

### Filesystem operations

- open
- openat
- close
- stat-like operations
- mkdir
- unlink
- rename
- symlink
- hard link
- filesystem synchronization
- fallocate
- fadvise

### Networking

- accept
- connect
- recv
- send
- recvmsg
- sendmsg
- recv/recvmsg multishot forms
- send zerocopy-related operations
- socket-related operations

### Polling/events

- poll add
- poll remove/update
- timeout
- timeout remove/update
- async cancel

### Advanced composition

- linked operations
- hard-linked operations
- multishot
- buffer selection
- message passing between rings
- futex operations
- uring_cmd

### Data movement

- splice
- tee-like facilities where supported
- zero-copy network operations

The exact opcode list is kernel-version dependent.

The safest production strategy is:

> **Compile against the UAPI you need, and probe runtime support for optional features.**

---

# 27. Read: the canonical example

A simple conceptual SQE:

```c
struct io_uring_sqe *sqe = io_uring_get_sqe(&ring);

io_uring_prep_read(
    sqe,
    fd,
    buffer,
    buffer_len,
    offset
);

sqe->user_data = request_id;
```

Then:

```c
io_uring_submit(&ring);
```

Later:

```c
struct io_uring_cqe *cqe;

io_uring_wait_cqe(&ring, &cqe);

if (cqe->res >= 0) {
    printf("read %d bytes\n", cqe->res);
} else {
    printf("error: %s\n", strerror(-cqe->res));
}
```

---

# 28. Why the buffer lifetime matters

This is one of the most important correctness rules.

Suppose:

```c
char buf[4096];

io_uring_prep_read(sqe, fd, buf, sizeof(buf), 0);
io_uring_submit(&ring);

return;
```

If `buf` belongs to a stack frame that disappears before the kernel completes the operation, the request is invalid.

The memory must remain valid for the operation's lifetime.

Therefore:

```text
SQE lifetime
    !=
buffer lifetime
```

The buffer must live until completion, unless the API/operation explicitly provides a different lifetime model.

The Rust `io-uring` crate documentation likewise warns that the developer must ensure pointers such as file descriptors and buffers remain valid. citeturn0search7

---

# 29. Fixed buffers

For high-throughput workloads, repeatedly pinning/mapping/validating user buffers can be expensive.

io_uring supports registered/fixed buffers.

Conceptually:

```text
startup:
    register buffer A
    register buffer B
    register buffer C

runtime:
    READ_FIXED(buffer_index=1)
    READ_FIXED(buffer_index=2)
    READ_FIXED(buffer_index=0)
```

Instead of repeatedly describing arbitrary user memory.

Benefits can include:

- reduced per-request overhead
- stable resource references
- better predictability
- useful integration with high-performance I/O

Trade-offs include:

- memory pinning/accounting
- registration cost
- resource management complexity
- limited flexibility

---

# 30. Fixed files

Similarly, file descriptors can be registered.

Ordinary:

```text
SQE
 |
 +--> fd = 57
 |
 +--> kernel looks up process fd table
```

Registered file:

```text
SQE
 |
 +--> fixed file index = 3
 |
 +--> io_uring registered file table
```

This can reduce repeated descriptor lookup overhead.

It is particularly useful when:

- a server has a stable set of hot descriptors
- the same descriptors are reused heavily
- the workload is sensitive to per-operation overhead

---

# 31. Direct descriptors

Registered-file mechanisms have evolved and include different ways to manage descriptors.

The important mental model is:

```text
normal fd:
    process fd table -> file object

registered/fixed:
    io_uring resource table -> file object
```

Do not confuse:

- file descriptor number
- registered-file index
- file object pointer
- socket object

They are different abstraction layers.

---

# 32. Resource lifetime

Registered resources complicate lifetime.

Think in terms of:

```text
application owns logical resource
          |
          v
io_uring holds a reference
          |
          v
outstanding request holds a reference
```

Closing/unregistering one layer does not necessarily mean the underlying object is immediately destroyed.

Kernel reference counting and request lifetime rules determine when destruction occurs.

This is a general systems principle:

> **An object can remain alive because an asynchronous operation still owns a reference to it.**

---

# 33. Provided buffers / buffer selection

For high-rate networking, pre-posting one buffer per request is inefficient.

Instead:

```text
                 buffer pool
        +----+----+----+----+----+
        | B0 | B1 | B2 | B3 | B4 |
        +----+----+----+----+----+
                    |
                    v
               kernel selects
                    |
                    v
               completion
```

The application supplies a pool.

The kernel chooses an appropriate buffer when data arrives.

The CQE identifies the selected buffer through completion flags/index metadata.

This is especially useful for:

- high-throughput receives
- multishot receive
- event-driven servers
- workloads where request sizes vary

---

# 34. Multishot receive

A normal receive request:

```text
recv SQE
   |
   v
one receive
   |
   v
one CQE
```

A multishot receive:

```text
recv multishot SQE
   |
   +--> CQE packet 1
   +--> CQE packet 2
   +--> CQE packet 3
   +--> ...
```

This reduces repeated submission overhead.

But it introduces lifecycle complexity:

```text
active request
     |
     +--> many completions
     |
     +--> termination
```

A server must understand exactly when the multishot request is no longer active.

---

# 35. Linked operations

io_uring can express dependencies between operations.

Conceptually:

```text
SQE A --> SQE B --> SQE C
```

For example:

```text
open
  |
  v
read
  |
  v
close
```

The value is not simply "three operations."

The value is:

> **The kernel knows that these operations form a dependency chain.**

This can avoid application-level round trips.

---

# 36. Hard links versus ordinary links

io_uring has different linking semantics.

The conceptual distinction is:

### Soft dependency

A failure can influence whether later linked work proceeds, depending on the link semantics.

### Hard link

The chain has stronger continuation/ordering behavior.

When building chains, always read the exact semantics of the link flags for the kernel version you target.

Never infer:

```text
linked = transactional
```

It is not a database transaction.

---

# 37. Async cancellation

Cancellation is not:

```text
kill whatever the kernel is doing instantly
```

It is a request to cancel matching in-flight operations where the operation supports the relevant cancellation semantics.

Think:

```text
application
   |
   | cancel request
   v
kernel request lookup
   |
   +--> already completed?
   |
   +--> cancellable?
   |
   +--> executing backend?
   |
   v
cancellation result
```

An operation may race with completion.

Therefore a correct program must tolerate:

```text
completion and cancellation racing
```

---

# 38. Timeouts

io_uring can represent timeout work as an operation.

This is powerful because the timeout itself becomes part of the same completion model.

Instead of:

```text
epoll
timerfd
read timerfd
application bookkeeping
```

you can reason about:

```text
I/O operation
+
timeout operation
+
linked/dependency semantics
```

This can make complex asynchronous state machines cleaner.

---

# 39. Poll operations

`IORING_OP_POLL_ADD` can integrate readiness monitoring into io_uring.

This is useful when a device or descriptor is better modeled as:

```text
wait until readable/writable
```

rather than:

```text
submit a direct I/O operation
```

This is also an important reminder:

> io_uring does not make epoll obsolete in every design.

Some workloads still benefit from explicit readiness APIs or a hybrid model.

---

# 40. io_uring and epoll

A useful comparison:

```text
epoll:

events
  |
  v
epoll_wait()
  |
  v
application gets readiness
  |
  +--> read()
  +--> write()
```

io_uring:

```text
SQE
 |
 v
kernel executes operation
 |
 v
CQE
```

But a hybrid is possible:

```text
io_uring
   +
epoll
   +
traditional blocking APIs
```

Use the simplest mechanism that matches the workload.

---

# 41. Networking architecture

For a TCP server:

```text
                 TCP listening socket
                         |
                         v
                  ACCEPT SQE
                         |
                         v
                  ACCEPT CQE
                         |
                         v
                    client fd
                         |
             +-----------+-----------+
             |                       |
          RECV SQE                SEND SQE
             |                       |
             v                       v
        network stack            network stack
             |                       |
             +-----------+-----------+
                         |
                         v
                       CQEs
```

With multishot accept:

```text
one accept request
       |
       +--> connection 1
       +--> connection 2
       +--> connection 3
       +--> ...
```

This is an excellent match for event-driven servers.

---

# 42. TCP server state machine

A robust design can model each connection explicitly:

```text
          +---------+
          | ACCEPT  |
          +----+----+
               |
               v
          +---------+
          | RECV    |
          +----+----+
               |
        data received?
          /          \
        yes           EOF/error
         |               |
         v               v
   parse/application   CLOSE
         |
         v
      SEND
         |
         v
      RECV again
```

In a real server, there may be multiple outstanding operations:

```text
connection
 |
 +-- read state
 +-- write queue
 +-- timeout
 +-- protocol state
 +-- cancellation state
```

`user_data` can point to the connection/request state.

---

# 43. Backpressure in a server

Never allow:

```text
incoming data
     |
     v
allocate unlimited memory
     |
     v
send queue grows forever
```

Instead:

```text
socket
 |
 v
bounded receive buffers
 |
 v
application processing
 |
 v
bounded output queue
 |
 v
kernel send
```

Backpressure should exist at multiple levels:

- connection count
- outstanding SQEs
- buffer pool
- application queue
- send queue
- storage queue

---

# 44. Zero-copy networking

io_uring supports increasingly sophisticated data movement facilities.

Important concepts include:

- send zero-copy
- registered buffers
- receive-side zero-copy mechanisms on supported hardware/kernel/network paths
- buffer selection
- `uring_cmd`
- specialized networking integrations

The important principle is:

> "Zero copy" is a property of a particular data path, not simply a property of choosing io_uring.

Linux documentation now includes io_uring zero-copy receive mechanisms that use a registered memory area and a refill ring. citeturn1search1

---

# 45. Zero-copy receive architecture

A simplified modern zero-copy receive model:

```text
                  NIC
                   |
                   v
             network driver
                   |
                   v
          zero-copy buffer area
             +----+----+
             | B0 | B1 |
             +----+----+
                   |
                   v
              io_uring CQE
                   |
                   v
             application
                   |
                   v
             process data
                   |
                   v
             recycle buffer
```

The recycle path matters.

A zero-copy system is not:

```text
receive forever
```

It is:

```text
allocate/register
      |
receive
      |
consume
      |
recycle
      |
receive again
```

Linux's current zero-copy receive documentation describes a refill ring for returning consumed buffers to the kernel. citeturn1search1

---

# 46. File I/O architecture

For storage:

```text
SQE READ
   |
   v
io_uring operation handler
   |
   v
VFS
   |
   +--> page cache
   |
   +--> filesystem
   |
   +--> iomap/direct I/O path
   |
   v
block layer
   |
   v
device driver
   |
   v
NVMe / SSD / storage
```

Whether the request is actually asynchronous depends on the path.

For buffered I/O:

```text
application
   |
   v
page cache
```

may satisfy the request quickly.

For direct I/O:

```text
application
   |
   v
filesystem
   |
   v
block layer
   |
   v
device
```

the device may provide true asynchronous completion.

---

# 47. Buffered versus direct I/O

### Buffered I/O

Advantages:

- page cache
- simpler application logic
- good locality
- filesystem-managed caching

Disadvantages:

- page-cache effects can hide device latency
- memory pressure interactions
- less deterministic data path

### Direct I/O

Advantages:

- bypasses much of page cache
- can be useful for databases and specialized storage engines
- explicit buffer management

Disadvantages:

- alignment constraints
- buffer lifetime requirements
- more application responsibility
- filesystem/device-specific behavior

io_uring does not remove these underlying storage semantics.

---

# 48. Registered buffers and direct I/O

A common high-performance storage design:

```text
startup:
    allocate aligned buffers
    register buffers

runtime:
    submit fixed-buffer reads/writes

completion:
    recycle buffer
```

This can reduce repeated setup work.

But you should benchmark:

```text
ordinary buffer + buffered I/O
vs
registered buffer + direct I/O
```

because the faster architecture depends on the workload.

---

# 49. `O_DIRECT` is not automatically faster

This is a classic systems misconception.

`O_DIRECT` can be beneficial when:

- application has its own cache
- workload is large/random
- page cache would cause unwanted duplication
- storage engine wants direct control

It can be worse when:

- data is reused
- request sizes are small
- alignment is inconvenient
- application has poor buffering strategy

The correct question is:

> **Which layer should own caching?**

---

# 50. Filesystem semantics still apply

io_uring does not change:

- POSIX-like file semantics where applicable
- filesystem locking
- durability semantics
- metadata behavior
- rename atomicity
- permission checks
- mount behavior
- filesystem-specific limitations

An asynchronous API does not make the underlying filesystem transactional.

---

# 51. Durability and `fsync`

Writing:

```text
write
```

does not necessarily mean:

```text
data is durable on nonvolatile media
```

If durability matters:

```text
write
   |
   v
fsync / fdatasync semantics
   |
   v
durability boundary
```

io_uring can submit synchronization operations through the same completion framework.

For databases:

```text
write data
write WAL
flush WAL
commit
```

still requires careful durability reasoning.

---

# 52. `uring_cmd`

`uring_cmd` allows specialized drivers/subsystems to expose commands through io_uring.

Conceptually:

```text
application
    |
    v
io_uring SQE
    |
    v
driver-specific command
    |
    v
hardware/subsystem
    |
    v
CQE
```

This is an important extensibility mechanism.

It means io_uring can act as a generalized asynchronous command transport for selected kernel subsystems, not merely files and sockets.

---

# 53. Message passing between rings

Modern io_uring includes mechanisms for communication between rings.

Conceptually:

```text
ring A
  |
  | message
  v
ring B
```

This can be useful for distributing work across worker contexts without routing everything through a conventional socket/eventfd-style mechanism.

The design question becomes:

```text
ring per thread?
ring per CPU?
ring per service?
shared ring?
```

---

# 54. Thread-per-ring versus shared ring

A common scalable design is:

```text
CPU 0 ---> ring 0
CPU 1 ---> ring 1
CPU 2 ---> ring 2
CPU 3 ---> ring 3
```

Advantages:

- reduced contention
- CPU locality
- easier ownership
- fewer locks
- predictable cache behavior

But a shared ring:

```text
thread A ---+
thread B ---+--> shared ring
thread C ---+
```

can be simpler.

The right architecture depends on workload and operation semantics.

---

# 55. Single-producer versus multi-producer thinking

Many ring-buffer designs are easiest when one logical producer owns the SQ.

If many threads write SQEs concurrently:

```text
thread A --+
thread B --+--> SQ
thread C --+
```

you need synchronization.

A scalable pattern is often:

```text
worker thread
    |
    v
its own ring
```

rather than forcing many producers into one ring.

---

# 56. Cache locality

A ring is fundamentally a cache-sensitive data structure.

Consider:

```text
producer writes SQE
        |
        v
cache line
        |
        v
kernel reads SQE
```

If producer and consumer constantly modify the same cache lines, you get cache-line bouncing.

This is why:

- ring ownership
- CPU affinity
- queue placement
- batching
- structure layout
- false sharing avoidance

matter.

---

# 57. Batching

Suppose you have 100 operations.

Bad:

```text
prepare one
submit
wait

prepare one
submit
wait

...
```

Better:

```text
prepare 100
submit batch
wait for completions
consume batch
```

The advantage comes from amortization:

```text
fixed overhead / number of operations
```

gets smaller.

But huge batches can increase latency.

Therefore the real tuning problem is:

```text
throughput <-> latency
```

---

# 58. Latency versus throughput

### Latency-sensitive workload

Prefer:

- smaller batches
- CPU locality
- polling where justified
- low queue depth
- minimal allocations
- pre-registration
- careful scheduling

### Throughput-sensitive workload

Prefer:

- larger batches
- deeper queues
- multiple outstanding requests
- registered resources
- multishot operations
- efficient buffer pools

Do not optimize both extremes with the same settings.

---

# 59. SQPOLL trade-off

A simple decision model:

```text
if syscall overhead dominates:
    consider SQPOLL

if CPU is already saturated:
    SQPOLL may hurt

if latency is extremely sensitive:
    polling may help

if workload is bursty:
    polling may waste CPU
```

Measure CPU residency and tail latency, not only average throughput.

---

# 60. Polling and power

Polling can reduce latency but consumes CPU.

On a server:

```text
busy polling
    |
    +--> lower wakeup latency
    |
    +--> higher CPU utilization
    |
    +--> potentially higher power
```

A latency improvement that costs an entire CPU core may be a poor trade unless the workload values it.

---

# 61. Feature probing

Do not assume a modern feature exists.

A robust application can:

1. create the ring
2. probe supported operations/features
3. choose a fast path
4. choose a fallback

Conceptually:

```text
                    start
                      |
                      v
                 probe kernel
                      |
          +-----------+-----------+
          |                       |
      feature yes             feature no
          |                       |
          v                       v
      fast path                fallback
```

This is particularly important for:

- multishot operations
- newer opcodes
- advanced registration
- zero-copy features
- specialized polling
- newer timeout/futex features

---

# 62. `io_uring_register`

Registration is the third major UAPI entry point.

Conceptually:

```text
io_uring_setup
      |
      v
ring
      |
      +--> io_uring_register
               |
               +--> buffers
               +--> files
               +--> personalities
               +--> eventfd
               +--> restrictions
               +--> probes
               +--> other features
```

Registration moves some setup cost from:

```text
every operation
```

to:

```text
startup/configuration
```

This is classic amortization.

---

# 63. Registration is not always faster

Registration has costs.

If you:

```text
register buffer
submit one request
unregister buffer
```

you may have made things worse.

Registration is most useful when:

```text
startup cost
    |
    v
many requests
    |
    v
amortized overhead
```

Think in terms of:

```text
registration_cost / number_of_operations
```

---

# 64. Personalities

io_uring can associate operations with credentials/personality state in supported configurations.

This matters when an application has complex credential requirements and wants operations to execute under a selected credential context.

Do not confuse:

```text
process credentials
file permissions
io_uring personality
```

They are related but distinct concepts.

---

# 65. Restrictions and sandboxing

io_uring supports mechanisms for restricting what operations a ring may issue.

This is useful in privileged or semi-trusted architectures.

Example:

```text
application
    |
    v
ring restricted to:
    READ
    WRITE
    CLOSE
```

rather than allowing every opcode.

This is conceptually similar to reducing a capability surface.

---

# 66. Security model

io_uring is a powerful kernel interface.

Therefore:

- validate untrusted data before putting it into SQEs
- bound queue depth
- bound buffer registration
- avoid untrusted pointer lifetimes
- use restrictions where appropriate
- keep kernels patched
- use seccomp and other sandboxing layers when appropriate
- do not expose privileged io_uring functionality to untrusted plugins without analysis

The security boundary remains:

```text
user
  |
  | UAPI
  v
kernel
```

A malformed SQE must never compromise kernel safety; the kernel validates user-controlled fields.

---

# 67. Pointer safety

A particularly dangerous class of bugs is:

```text
SQE stores user pointer
       |
       v
application frees memory
       |
       v
kernel later dereferences pointer
```

The result can be:

- use-after-free
- corrupted I/O
- memory corruption at application level
- undefined application behavior

For C:

```text
you own lifetime correctness
```

For Rust:

```text
the type system can help,
but raw-pointer io_uring APIs still require unsafe reasoning
```

The Rust crate is explicitly a low-level interface, and its examples/documentation require the caller to ensure buffer/fd validity. citeturn0search0turn0search7

---

# 68. Rust does not make the kernel contract safe automatically

A Rust wrapper can prevent:

```text
use-after-free caused by ordinary ownership errors
```

but it cannot infer every asynchronous kernel lifetime constraint unless the API is designed to encode them.

At the low level, you will encounter:

```rust
unsafe {
    ring.submission().push(&entry)?;
}
```

The `unsafe` boundary is telling you:

> "You are now responsible for satisfying the kernel's memory/lifetime contract."

This is a healthy way to think about unsafe Rust:

```text
unsafe = responsibility boundary
```

not:

```text
unsafe = bad code
```

---

# 69. C implementation: liburing

The most practical C interface is liburing.

The project provides helpers for:

- setup
- teardown
- SQE acquisition
- operation preparation
- submission
- CQE waiting
- CQE iteration
- registration
- feature probing
- advanced operations

The official liburing repository contains examples, tests, headers, and man pages. citeturn0search8turn0search9

---

# 70. C example: asynchronous file read

```c
#define _GNU_SOURCE

#include <liburing.h>
#include <fcntl.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <errno.h>

#define QUEUE_DEPTH 8
#define BUF_SIZE    4096

struct request {
    int fd;
    char *buf;
    size_t len;
};

int main(void)
{
    struct io_uring ring;
    int ret = io_uring_queue_init(QUEUE_DEPTH, &ring, 0);
    if (ret < 0) {
        fprintf(stderr, "queue_init: %s\n", strerror(-ret));
        return 1;
    }

    int fd = open("input.txt", O_RDONLY);
    if (fd < 0) {
        perror("open");
        io_uring_queue_exit(&ring);
        return 1;
    }

    char *buf = aligned_alloc(4096, BUF_SIZE);
    if (!buf) {
        perror("aligned_alloc");
        close(fd);
        io_uring_queue_exit(&ring);
        return 1;
    }

    struct request req = {
        .fd = fd,
        .buf = buf,
        .len = BUF_SIZE,
    };

    struct io_uring_sqe *sqe = io_uring_get_sqe(&ring);
    if (!sqe) {
        fprintf(stderr, "SQ is full\n");
        free(buf);
        close(fd);
        io_uring_queue_exit(&ring);
        return 1;
    }

    io_uring_prep_read(
        sqe,
        fd,
        buf,
        BUF_SIZE,
        0
    );

    sqe->user_data = (uintptr_t)&req;

    ret = io_uring_submit(&ring);
    if (ret < 0) {
        fprintf(stderr, "submit: %s\n", strerror(-ret));
        free(buf);
        close(fd);
        io_uring_queue_exit(&ring);
        return 1;
    }

    struct io_uring_cqe *cqe;

    ret = io_uring_wait_cqe(&ring, &cqe);
    if (ret < 0) {
        fprintf(stderr, "wait_cqe: %s\n", strerror(-ret));
        free(buf);
        close(fd);
        io_uring_queue_exit(&ring);
        return 1;
    }

    struct request *completed =
        (struct request *)(uintptr_t)cqe->user_data;

    if (cqe->res < 0) {
        fprintf(stderr, "read: %s\n", strerror(-cqe->res));
    } else {
        printf("read %d bytes\n", cqe->res);
        fwrite(completed->buf, 1, cqe->res, stdout);
    }

    io_uring_cqe_seen(&ring, cqe);

    free(buf);
    close(fd);
    io_uring_queue_exit(&ring);

    return 0;
}
```

Build:

```bash
gcc -O2 -Wall -Wextra -o read_uring read_uring.c -luring
```

This follows the standard liburing programming model: acquire an SQE, prepare an operation, attach `user_data`, submit, wait for a CQE, inspect `res`, and mark the CQE consumed. Official liburing examples use the same general architecture. citeturn0search11turn0search8

---

# 71. C example: many outstanding reads

A real application should usually keep several operations in flight.

```c
#define QUEUE_DEPTH 64
#define BLOCK_SIZE  4096

struct request {
    int fd;
    void *buf;
    off_t offset;
    unsigned id;
};

for (unsigned i = 0; i < QUEUE_DEPTH; ++i) {
    struct request *req = &requests[i];

    req->buf = aligned_alloc(4096, BLOCK_SIZE);
    req->offset = (off_t)i * BLOCK_SIZE;
    req->id = i;

    struct io_uring_sqe *sqe = io_uring_get_sqe(&ring);

    io_uring_prep_read(
        sqe,
        fd,
        req->buf,
        BLOCK_SIZE,
        req->offset
    );

    sqe->user_data = (uintptr_t)req;
}

int submitted = io_uring_submit(&ring);
```

Now:

```text
request 0 ----\
request 1 -----\
request 2 ------> kernel
...
request 63 ----/
```

Then:

```c
for (unsigned i = 0; i < submitted; ++i) {
    struct io_uring_cqe *cqe;

    io_uring_wait_cqe(&ring, &cqe);

    struct request *req =
        (struct request *)(uintptr_t)cqe->user_data;

    if (cqe->res >= 0) {
        /* process req->buf */
    }

    io_uring_cqe_seen(&ring, cqe);
}
```

This is the basic pattern behind high-concurrency I/O engines.

---

# 72. C example: TCP echo server architecture

A production server needs explicit request state.

```c
enum op_type {
    OP_ACCEPT,
    OP_RECV,
    OP_SEND,
};

struct conn {
    int fd;
    enum op_type op;
    char buf[16 * 1024];
    size_t len;
    size_t sent;
};
```

Then helper functions:

```c
static void submit_recv(struct io_uring *ring, struct conn *c)
{
    struct io_uring_sqe *sqe = io_uring_get_sqe(ring);

    io_uring_prep_recv(
        sqe,
        c->fd,
        c->buf,
        sizeof(c->buf),
        0
    );

    c->op = OP_RECV;
    sqe->user_data = (uintptr_t)c;
}

static void submit_send(struct io_uring *ring, struct conn *c)
{
    struct io_uring_sqe *sqe = io_uring_get_sqe(ring);

    io_uring_prep_send(
        sqe,
        c->fd,
        c->buf + c->sent,
        c->len - c->sent,
        0
    );

    c->op = OP_SEND;
    sqe->user_data = (uintptr_t)c;
}
```

Completion:

```c
struct conn *c =
    (struct conn *)(uintptr_t)cqe->user_data;

switch (c->op) {
case OP_RECV:
    if (cqe->res > 0) {
        c->len = (size_t)cqe->res;
        c->sent = 0;
        submit_send(&ring, c);
    } else {
        close(c->fd);
        free(c);
    }
    break;

case OP_SEND:
    if (cqe->res > 0) {
        c->sent += (size_t)cqe->res;

        if (c->sent < c->len)
            submit_send(&ring, c);
        else
            submit_recv(&ring, c);
    } else {
        close(c->fd);
        free(c);
    }
    break;

default:
    break;
}
```

This is deliberately simplified. A production server must handle:

- partial sends
- connection shutdown
- errors
- concurrent operations
- cancellation
- output buffering
- protocol framing
- backpressure
- timeouts
- buffer ownership
- multishot lifetime

---

# 73. Rust implementation

The low-level Rust crate is:

```toml
[dependencies]
io-uring = "0.7"
```

The current crate documentation describes it as a low-level Linux io_uring interface and lists supported prebuilt architectures including x86_64, AArch64, RISC-V 64, LoongArch64, and PowerPC64. citeturn0search0turn0search5

---

# 74. Rust: basic read

```rust
use io_uring::{opcode, types, IoUring};
use std::fs::File;
use std::io;
use std::os::fd::AsRawFd;

fn main() -> io::Result<()> {
    let mut ring = IoUring::new(8)?;

    let file = File::open("input.txt")?;

    let mut buffer = vec![0u8; 4096];

    let entry = opcode::Read::new(
        types::Fd(file.as_raw_fd()),
        buffer.as_mut_ptr(),
        buffer.len() as u32,
    )
    .offset(0)
    .build()
    .user_data(0x42);

    unsafe {
        ring.submission()
            .push(&entry)
            .map_err(|_| io::Error::new(
                io::ErrorKind::Other,
                "submission queue full",
            ))?;
    }

    ring.submit_and_wait(1)?;

    let mut cq = ring.completion();

    if let Some(cqe) = cq.next() {
        let result = cqe.result();

        if result >= 0 {
            let n = result as usize;
            println!("read {n} bytes");
            println!("{}", String::from_utf8_lossy(&buffer[..n]));
        } else {
            let errno = -result;
            eprintln!("io error: errno={errno}");
        }
    }

    Ok(())
}
```

The official Rust crate example follows the same essential pattern: construct an `opcode::Read`, push it into the submission queue, submit/wait, and process the completion. citeturn0search7

---

# 75. Rust lifetime reasoning

The difficult part is not syntax.

It is:

```text
entry contains buffer pointer
              |
              v
kernel may use pointer later
              |
              v
buffer must remain alive
```

Rust lets you write:

```rust
let mut buffer = vec![0u8; 4096];
```

but the low-level io_uring interface uses raw pointers.

Therefore the programmer must ensure:

```text
buffer allocation
    |
    +--> SQE references buffer
    |
    +--> submit
    |
    +--> wait completion
    |
    +--> only now reuse/free/move as appropriate
```

Do not mutate or move memory in ways that invalidate the pointer while the operation is outstanding.

---

# 76. Rust: request objects

A practical design:

```rust
struct ReadRequest {
    file_fd: i32,
    buffer: Vec<u8>,
    offset: u64,
    id: u64,
}
```

You can maintain a collection of outstanding requests:

```text
HashMap<RequestId, ReadRequest>
```

and put `RequestId` into `user_data`.

This often gives a cleaner ownership model than storing raw pointers in `user_data`.

Conceptually:

```text
CQE.user_data
      |
      v
RequestId
      |
      v
request table
      |
      v
owned buffer/state
```

Trade-off:

```text
raw pointer:
    faster lookup
    harder lifetime reasoning

ID:
    easier ownership
    extra lookup
```

Benchmark only after correctness.

---

# 77. Rust and `Send`/`Sync`

If multiple threads share an io_uring object, Rust's type system forces you to confront concurrency.

That is useful.

But remember:

```text
Rust thread safety
       !=
io_uring protocol correctness
```

You still need to reason about:

- SQ ownership
- CQ ownership
- operation lifetime
- kernel completion races
- buffer ownership
- cancellation
- multishot requests

---

# 78. Tokio and io_uring

The `tokio-uring` ecosystem builds higher-level async abstractions around io_uring.

A conceptual stack:

```text
application
     |
tokio-uring / high-level runtime
     |
Rust io-uring bindings
     |
io_uring UAPI
     |
Linux kernel
```

This is useful if you want:

- futures
- task scheduling
- structured ownership
- async runtime integration

But if you are learning the kernel interface, start with raw `io-uring` concepts first.

---

# 79. io_uring is not a runtime

This distinction matters.

io_uring provides:

```text
kernel asynchronous I/O interface
```

It does not by itself provide:

- a general future executor
- application task scheduling
- actor model
- cancellation propagation across arbitrary application tasks
- memory pooling policy
- protocol state management

A runtime can be built on top of it.

---

# 80. Building an executor on io_uring

A conceptual executor:

```text
                +------------------+
                | ready task queue |
                +---------+--------+
                          |
                          v
                  submit io_uring
                          |
                          v
                    kernel I/O
                          |
                          v
                       CQE
                          |
                          v
                 wake corresponding
                       task
                          |
                          v
                  ready task queue
```

The runtime maps:

```text
CQE.user_data
       |
       v
task / waker / operation state
```

This is why `user_data` is such a powerful primitive.

---

# 81. io_uring and async/await

An async operation conceptually becomes:

```text
Future::poll()
      |
      v
submit SQE
      |
      v
Pending
      |
      v
kernel completes
      |
      v
CQE
      |
      v
wake task
      |
      v
Future::poll()
      |
      v
Ready(result)
```

This is a very useful mental model for understanding how Rust async runtimes can use io_uring.

---

# 82. Why async does not mean parallel

Consider:

```text
10 async requests
```

They may execute:

- concurrently
- sequentially
- partially overlapping
- on worker threads
- through hardware queues

"Async" primarily describes **how the caller interacts with progress**, not a guarantee of simultaneous CPU execution.

---

# 83. Concurrency versus parallelism

### Concurrency

Multiple operations are in flight.

```text
A ------>
B   ------>
C     ------>
```

### Parallelism

Multiple operations execute simultaneously.

```text
CPU0: A ------>
CPU1: B ------>
CPU2: C ------>
```

io_uring enables high concurrency.

Actual parallelism depends on:

- CPU cores
- storage queues
- network hardware
- filesystem
- kernel scheduling
- operation dependencies

---

# 84. Operation ordering

If you submit:

```text
A
B
C
```

do not automatically assume:

```text
A completes
then B
then C
```

unless the operation semantics/dependencies require it.

Completion order may be:

```text
B
C
A
```

This is one reason `user_data` and explicit state machines are essential.

---

# 85. Ordering with linked operations

If you need:

```text
A must precede B
```

express that dependency.

Do not rely on:

```text
A submitted before B
```

as an implicit ordering guarantee.

Submission order is not the same as completion ordering.

---

# 86. Race example

Suppose:

```text
SQE 1: read connection
SQE 2: close connection
```

If you submit both without understanding dependencies, you have created a race.

Possible outcomes can depend on scheduling and operation semantics.

Correct design:

```text
read
 |
 v
completion
 |
 v
close
```

or an appropriate linked/dependency mechanism where supported.

---

# 87. Partial I/O

A completion:

```text
res = 1000
```

does not always mean:

```text
the logical operation is complete
```

For a send:

```text
requested 4096
completed 1000
```

you may need:

```text
send remaining 3096
```

For reads:

```text
requested 4096
completed 1000
```

you may have received a short read.

Applications must distinguish:

```text
kernel operation complete
```

from:

```text
application-level message complete
```

---

# 88. Network message framing

TCP is a byte stream.

If you send:

```text
HELLO
WORLD
```

the receiver may get:

```text
HELLOWORLD
```

or:

```text
HEL
LOWORLD
```

or:

```text
HELLO
WORLD
```

Therefore io_uring does not solve protocol framing.

Your application needs:

```text
length-prefix
delimiter
fixed-size records
self-describing frames
```

---

# 89. Error handling

Always inspect:

```c
cqe->res
```

Do not assume:

```c
if (cqe->res == 0)
    success;
```

The meaning is operation-specific.

Typical pattern:

```c
if (cqe->res < 0) {
    int err = -cqe->res;
    ...
} else {
    int bytes_or_result = cqe->res;
}
```

But some operations use special result conventions.

Read the opcode documentation.

---

# 90. CQE flags

CQE flags are especially important for advanced operations.

They can communicate information such as:

- buffer selection
- multishot state
- additional completion semantics

A robust completion loop should not ignore flags blindly.

---

# 91. Buffer selection mental model

A selected-buffer completion can be thought of as:

```text
CQE
 |
 +--> res = bytes
 |
 +--> flags = buffer-selected
 |
 +--> buffer ID = N
```

Then:

```text
buffer_id N
     |
     v
buffer pool[N]
     |
     v
data
```

This is more scalable than allocating a unique buffer for every network request.

---

# 92. Multishot completion termination

A multishot operation generally needs a way to determine:

```text
more completions expected?
```

The completion flags can communicate this.

Conceptually:

```text
CQE
 |
 +--> MORE = yes
 |       |
 |       v
 |    keep request alive
 |
 +--> MORE = no
         |
         v
      request ended
```

Always use the exact UAPI flag semantics for the operation you are implementing.

---

# 93. `eventfd` integration

io_uring can integrate with event notification mechanisms such as eventfd.

Conceptually:

```text
kernel completion
       |
       v
event notification
       |
       v
application event loop
```

This can help integrate io_uring with existing event-loop architectures.

---

# 94. Signals and waits

Waiting through io_uring must be designed carefully in applications that also use signals.

Important considerations include:

- signal masks
- interrupted waits
- race-free event waiting
- runtime signal handling

The raw syscall interface has explicit arguments for signal-related waiting behavior.

---

# 95. Cancellation race

Consider:

```text
Thread A:
    submit READ

Thread B:
    submit CANCEL(READ)
```

Possible timeline:

```text
READ completes
       |
       v
CQE produced
       |
       v
CANCEL arrives
```

The cancellation can now report that it could not cancel because the request already completed.

Or:

```text
CANCEL arrives first
       |
       v
READ canceled
```

Therefore:

> **Cancellation is a race protocol, not a magical rollback.**

---

# 96. Cleanup is part of asynchronous correctness

A common bug:

```c
free(request);
```

immediately after submitting it.

Correct:

```text
submit
 |
 v
outstanding
 |
 v
CQE
 |
 v
release request
```

For multishot:

```text
submit
 |
 v
active
 |
 +--> CQE
 +--> CQE
 +--> CQE
 |
 v
terminal CQE
 |
 v
release
```

For cancellation:

```text
cancel
 |
 +--> cancellation completion
 |
 +--> original operation completion may also race
```

The lifetime model must account for all of these.

---

# 97. Kernel task_work

Some io_uring operations need work to run in the context of the submitting task or another appropriate task context.

Linux has task-work mechanisms that allow deferred work to run later.

A simplified model:

```text
kernel receives request
       |
       v
cannot finish entirely here
       |
       v
attach task work
       |
       v
later task context
       |
       v
finish request
       |
       v
CQE
```

This is especially relevant when studying kernel source.

---

# 98. The kernel source reading strategy

Do not begin by reading the entire `fs/io_uring.c`.

It is too large and changes over time.

Instead:

### Step 1

Start with:

```text
include/uapi/linux/io_uring.h
```

Learn the public protocol.

### Step 2

Read:

```text
io_uring_setup(2)
io_uring_enter(2)
io_uring_register(2)
io_uring(7)
```

### Step 3

Read liburing's:

```text
liburing.h
```

and one simple example.

### Step 4

Trace one operation.

For example:

```text
IORING_OP_READ
```

### Step 5

Trace one network operation:

```text
IORING_OP_RECV
```

### Step 6

Trace one advanced operation:

```text
IORING_OP_TIMEOUT
```

or multishot accept.

### Step 7

Only then explore:

```text
io-wq
task_work
resource registration
polling
```

This creates a layered understanding.

---

# 99. Source tree map

A useful map is:

```text
linux/
 |
 +-- include/uapi/linux/io_uring.h
 |       |
 |       +-- public UAPI structures/constants
 |
 +-- include/linux/io_uring.h
 |       |
 |       +-- kernel-facing declarations
 |
 +-- fs/io_uring.c
 |       |
 |       +-- main implementation
 |
 +-- fs/io-wq.c
 |       |
 |       +-- worker infrastructure
 |
 +-- fs/io-wq.h
 |
 +-- tools/io_uring/
 |       |
 |       +-- examples/tools
 |
 +-- tools/testing/selftests/
         |
         +-- io_uring-related tests
```

The exact layout changes across releases; use the kernel tree matching the kernel you are studying. Linux's maintainer documentation identifies the principal io_uring implementation files. citeturn1search0

---

# 100. Read the UAPI header first

The UAPI header answers:

```text
What does userspace know?
```

The kernel source answers:

```text
How does Linux implement it?
```

This distinction is fundamental.

For systems learning:

```text
UAPI first
implementation second
```

is usually more efficient.

---

# 101. Operation dispatch table

A useful source-reading exercise is to find the mapping:

```text
opcode
  |
  v
operation implementation
```

Conceptually:

```text
IORING_OP_READ
       |
       v
read handler

IORING_OP_WRITE
       |
       v
write handler

IORING_OP_ACCEPT
       |
       v
accept handler

IORING_OP_TIMEOUT
       |
       v
timeout handler
```

Then inspect:

```text
prepare
issue
async
complete
cleanup
```

This is how the implementation becomes understandable.

---

# 102. The issue/complete model

Many asynchronous kernel subsystems can be understood as:

```text
prepare
   |
   v
issue
   |
   +--> completed immediately
   |
   +--> pending
           |
           v
        progress
           |
           v
        complete
```

io_uring follows this broad systems pattern.

When reading kernel code, ask:

1. Where is the request created?
2. Where is it validated?
3. Where is the operation issued?
4. Where can it return "retry/pending"?
5. What owns the request while pending?
6. What generates the CQE?
7. Who releases the request?

These seven questions are extremely powerful.

---

# 103. Request ownership

The deepest concurrency question is:

> **Who owns the request right now?**

Possible ownership states:

```text
userspace
   |
   v
SQ
   |
   v
kernel dispatch
   |
   v
filesystem/network/device
   |
   v
completion
   |
   v
CQ
   |
   v
userspace
```

Ownership can move between subsystems.

A correct mental model tracks:

```text
request owner
+
resource references
+
buffer lifetime
```

---

# 104. Reference counting

Suppose:

```text
request
  |
  +--> file reference
  +--> buffer reference
  +--> socket reference
```

Asynchronous work often requires those resources to remain alive.

Kernel reference counting ensures that the underlying object does not disappear while another component still needs it.

This is why "close" and "destroy" are not always the same event.

---

# 105. Page faults and async I/O

Even if your API is asynchronous, memory access can have hidden costs.

For example:

```text
first touch buffer
     |
     v
page fault
     |
     v
page allocation/mapping
```

High-performance applications often pre-touch or otherwise manage memory deliberately.

Registered buffers can also change memory-management behavior.

Therefore benchmark:

```text
cold memory
vs
warm memory
```

and:

```text
first request
vs
steady state
```

---

# 106. NUMA

On multi-socket systems:

```text
CPU 0 --- NUMA node 0 --- memory 0
CPU 1 --- NUMA node 1 --- memory 1
```

A ring on node 0 used heavily by CPU 1 can create remote memory traffic.

For very high-performance systems, consider:

- ring per NUMA node
- buffer pools per NUMA node
- CPU affinity
- storage queue affinity
- network RSS/RPS configuration
- memory placement

io_uring does not hide NUMA topology.

---

# 107. CPU affinity

A useful high-performance architecture:

```text
CPU 0
 |
 +-- ring 0
 +-- connections 0..N
 +-- buffer pool 0

CPU 1
 |
 +-- ring 1
 +-- connections N..M
 +-- buffer pool 1
```

This reduces cross-core synchronization.

But pinning everything can also hurt load balancing.

Measure.

---

# 108. False sharing

Suppose two CPUs repeatedly modify fields on the same cache line:

```text
cache line
+-------------------------------+
| producer_tail | consumer_head |
+-------------------------------+
```

This can cause cache-line bouncing.

Kernel and userspace ring structures are carefully designed with cache behavior in mind.

When designing your own structures around io_uring, preserve similar discipline.

---

# 109. Memory barriers: practical rule

If using liburing:

> **Use liburing's queue APIs rather than inventing your own ring manipulation.**

If using the raw interface:

> **Follow the documented acquire/release rules exactly.**

Never "optimize away" a memory barrier because:

```text
"It works on x86."
```

It may fail on weaker memory-ordering architectures.

---

# 110. Architecture portability

x86 often hides memory-ordering mistakes because its memory model is relatively strong.

ARM and RISC-V can expose incorrect assumptions more readily.

Therefore:

```text
works on x86
```

does not prove:

```text
correct on all Linux architectures
```

The Rust io_uring crate supports multiple architectures, reinforcing the importance of architecture-neutral UAPI semantics. citeturn0search0

---

# 111. Raw syscall implementation

For learning, it is valuable to implement the protocol without liburing.

The rough architecture is:

```text
io_uring_setup
      |
      v
mmap SQ ring
mmap CQ ring
mmap SQEs
      |
      v
construct SQE
      |
      v
publish SQ tail
      |
      v
io_uring_enter
      |
      v
kernel
      |
      v
CQE
      |
      v
load CQ tail
      |
      v
consume CQE
      |
      v
publish CQ head
```

This teaches the actual protocol.

But raw code is significantly easier to get wrong.

---

# 112. Why liburing exists

liburing is not a second asynchronous I/O engine.

It is primarily:

```text
userspace helper library
        |
        v
io_uring UAPI
        |
        v
Linux kernel
```

It handles:

- setup
- mmap details
- SQE preparation helpers
- queue manipulation
- registration helpers
- convenience APIs

The official project describes it as a library providing helpers for Linux kernel io_uring support. citeturn0search8

---

# 113. A useful raw-versus-library comparison

```text
Raw UAPI

application
   |
syscall + mmap + barriers
   |
io_uring kernel API
```

versus:

```text
liburing

application
   |
liburing helpers
   |
syscall + mmap + barriers
   |
io_uring kernel API
```

For production C applications, liburing is usually the practical starting point.

For kernel/UAPI education, raw implementation is invaluable.

---

# 114. C memory ownership pattern

A robust C request object often looks like:

```c
struct request {
    uint64_t id;

    int fd;

    void *buffer;
    size_t buffer_len;

    off_t offset;

    enum {
        REQ_READ,
        REQ_WRITE,
        REQ_RECV,
        REQ_SEND
    } type;

    bool active;
};
```

The lifetime:

```text
malloc request
      |
prepare SQE
      |
submit
      |
active = true
      |
wait
      |
CQE
      |
active = false
      |
free
```

This explicit state machine is much safer than scattering local variables across an event loop.

---

# 115. Rust ownership pattern

A Rust design can use:

```rust
struct Request {
    id: u64,
    buffer: Vec<u8>,
    offset: u64,
}
```

Then:

```text
request table owns Request
          |
          v
SQE user_data = id
          |
          v
kernel
          |
          v
CQE user_data = id
          |
          v
request table lookup
          |
          v
Request still owned
```

This is often easier to reason about than passing raw pointers.

---

# 116. Generational IDs

For large systems, a simple integer ID can suffer from reuse confusion.

A useful technique:

```text
generation + slot
```

For example:

```text
request_id = (generation << 32) | slot
```

Then:

```text
old completion
    |
    v
generation mismatch
    |
    v
ignore/reject stale event
```

This is a general async-system technique, not specific to io_uring.

---

# 117. Event loop skeleton

A high-performance event loop often resembles:

```text
initialize
    |
    +-- ring
    +-- buffers
    +-- files/sockets
    +-- feature probes
    |
    v
submit initial work
    |
    v
+-------------------------+
| event loop              |
|                         |
| consume CQEs            |
| update application      |
| recycle resources       |
| prepare new SQEs        |
| submit batch             |
| wait/poll                |
+------------+------------+
             |
             v
          shutdown
```

This is the core architecture behind many asynchronous engines.

---

# 118. Completion-driven design

Traditional code often looks like:

```text
do operation
wait
process
do next operation
```

Completion-driven code:

```text
submit
return to event loop

later:
    completion
    process
    schedule next operation
```

The application becomes a state machine.

That is the fundamental mental shift.

---

# 119. State machines are the real skill

The deepest practical skill in io_uring programming is not memorizing opcodes.

It is learning to model:

```text
request state
resource state
buffer state
connection state
failure state
cancellation state
```

For example:

```text
CONNECTION_OPEN
      |
      v
READ_PENDING
      |
      v
READ_COMPLETE
      |
      +--> protocol error --> CLOSE_PENDING
      |
      v
WRITE_PENDING
      |
      v
WRITE_COMPLETE
      |
      v
READ_PENDING
```

This scales much better than nested callbacks.

---

# 120. Common anti-pattern: one SQE at a time

This:

```text
prepare
submit
wait
prepare
submit
wait
```

may erase much of the batching advantage.

It can still be appropriate for:

- simple code
- low-throughput tools
- teaching
- operations with strict dependencies

But for throughput, keep multiple operations outstanding.

---

# 121. Common anti-pattern: allocating per I/O

This:

```text
malloc request
malloc buffer
submit
complete
free
```

can create allocator and cache overhead.

High-performance systems often use:

```text
request pool
buffer pool
ring
```

and recycle objects.

---

# 122. Common anti-pattern: giant queue depth

More outstanding operations does not automatically mean more performance.

You can reach:

```text
QD increases
   |
   +--> throughput increases
   |
   +--> saturation
   |
   +--> contention
   |
   +--> tail latency rises
```

The optimum is usually somewhere in the middle.

---

# 123. Common anti-pattern: assuming all I/O is truly async

A request can still end up in:

```text
io-wq worker
```

or otherwise execute through blocking infrastructure.

Measure kernel scheduling and worker behavior.

---

# 124. Common anti-pattern: ignoring partial results

For network operations:

```text
send requested = 1 MB
send completed = 128 KB
```

The remaining 896 KB must be handled.

Never equate:

```text
CQE received
```

with:

```text
application protocol finished
```

---

# 125. Common anti-pattern: ignoring CQ overflow

If the application cannot keep up with completions, the system needs a strategy.

Possible tools include:

- larger CQ
- faster CQ processing
- fewer outstanding requests
- batching
- multishot where appropriate
- backpressure

A completion queue is a finite resource.

---

# 126. Common anti-pattern: forgetting multishot lifetime

This bug is particularly dangerous:

```text
multishot accept
    |
first CQE
    |
free connection/request object
```

The multishot operation may still be active.

Treat multishot requests as long-lived state machines.

---

# 127. Common anti-pattern: assuming submission order equals completion order

If:

```text
A submitted first
B submitted second
```

you cannot generally assume:

```text
A completes first
```

Use explicit dependencies if ordering matters.

---

# 128. Common anti-pattern: treating `-errno` as `errno`

This:

```c
if (cqe->res < 0)
    perror("io");
```

can be misleading because `perror()` uses the process's `errno`.

Instead:

```c
if (cqe->res < 0)
    fprintf(stderr, "%s\n", strerror(-cqe->res));
```

The CQE result is the error value.

---

# 129. Debugging

Useful tools:

```bash
strace
perf
bpftrace
ftrace
gdb
gdb + liburing symbols
```

For syscall behavior:

```bash
strace -f -e trace=io_uring_setup,io_uring_enter,io_uring_register ./app
```

This helps answer:

```text
How often am I entering the kernel?
```

---

# 130. `perf`

Useful questions:

```text
Where is CPU time going?
```

Try:

```bash
perf stat ./app
perf record ./app
perf report
```

Look for:

- syscall overhead
- scheduler time
- filesystem functions
- network stack
- lock contention
- cache misses
- CPU migrations

---

# 131. BPF tracing

For deeper analysis:

```text
userspace
    |
io_uring syscall
    |
kernel tracepoints/functions
    |
BPF
    |
latency histogram
```

You can investigate:

- request latency
- syscall rates
- worker behavior
- scheduling
- filesystem paths
- network events

This is particularly useful when benchmarks contradict intuition.

---

# 132. Benchmark correctly

A benchmark should report:

```text
throughput
average latency
p50
p95
p99
p99.9
CPU utilization
context switches
syscalls
memory
queue depth
```

Do not report only:

```text
MB/s
```

A system can have excellent average throughput and terrible tail latency.

---

# 133. Warmup

Measure:

```text
cold
warm
steady-state
```

separately.

Cold execution includes:

- page faults
- filesystem cache misses
- dynamic linker activity
- CPU frequency transitions
- cache warming
- connection establishment

Steady-state tells a different story.

---

# 134. Benchmark matrix

For storage, test:

```text
queue depth:
1, 2, 4, 8, 16, 32, 64

request size:
4K, 16K, 64K, 1M

buffer:
registered / unregistered

I/O:
buffered / direct

submission:
normal / SQPOLL
```

For networking:

```text
connections
message size
batch size
multishot
buffer selection
CPU affinity
TLS/no TLS
```

---

# 135. Latency measurement

Use a monotonic clock.

Record:

```text
t_submit
t_complete
```

Then:

```text
latency = t_complete - t_submit
```

For true end-to-end latency, also measure:

```text
application event
    |
submit
    |
kernel
    |
device
    |
CQE
    |
application processing
```

Avoid measuring only one small slice and calling it "I/O latency."

---

# 136. CPU utilization interpretation

Suppose:

```text
io_uring: 10% CPU
epoll:    20% CPU
```

That does not automatically mean io_uring is better.

Maybe:

```text
io_uring = 100k req/s
epoll    = 200k req/s
```

Always normalize:

```text
CPU cost / request
```

or:

```text
requests / CPU-second
```

---

# 137. When io_uring may not help

io_uring is not automatically superior for:

- tiny low-rate programs
- simple blocking utilities
- workloads dominated by CPU
- workloads with one operation at a time
- systems where the backend itself is the bottleneck
- code where complexity is more expensive than syscall overhead

Sometimes:

```c
read(fd, buf, n);
```

is the best design.

---

# 138. When io_uring shines

Typical strong candidates:

- high-concurrency servers
- storage engines
- proxies
- databases
- high-throughput file servers
- network services
- asynchronous filesystem tooling
- high-rate message processing
- workloads with many independent I/O operations

The strongest advantage appears when:

```text
many operations
+
high concurrency
+
batching
+
low per-operation overhead
```

---

# 139. A useful performance equation

Think approximately:

```text
cost/request
  =
  syscall overhead
+ submission overhead
+ queue manipulation
+ resource lookup
+ scheduling
+ actual I/O
+ completion overhead
+ application processing
```

io_uring primarily attacks:

```text
syscall overhead
resource lookup
submission/completion coordination
```

depending on configuration.

It cannot make:

```text
SSD latency
network RTT
CPU processing
```

disappear.

---

# 140. Backpressure equation

For a stable system:

```text
average arrival rate < average service rate
```

If:

```text
arrival rate > service rate
```

then outstanding work grows until:

```text
memory exhausted
queue full
latency explodes
or requests are rejected
```

io_uring gives you a high-performance queue.

It does not repeal Little's Law.

A useful relationship is:

```text
L = λ W
```

where:

- `L` = average outstanding work
- `λ` = throughput
- `W` = average time in system

This is extremely useful for selecting queue depth.

---

# 141. Example: selecting queue depth

Suppose:

```text
target = 1,000,000 requests/s
average latency = 100 us
```

Then:

```text
L = λW
  = 1,000,000 * 0.0001
  = 100
```

So roughly 100 requests need to be in flight to sustain that throughput at that latency.

This does not mean:

```text
queue depth = exactly 100
```

because the real system has variance, multiple resources, and pipeline stages.

But it gives a useful first-order model.

---

# 142. Storage queue depth versus io_uring queue depth

Do not confuse:

```text
io_uring outstanding requests
```

with:

```text
NVMe hardware queue depth
```

There may be many layers:

```text
application
   |
io_uring
   |
filesystem
   |
block layer
   |
device driver
   |
NVMe submission queue
   |
device
```

A deep io_uring queue may still result in a shallower hardware queue depending on the path.

---

# 143. Networking queue depth

Similarly:

```text
io_uring requests
       |
       v
socket receive/send queues
       |
       v
TCP/IP stack
       |
       v
NIC rings
       |
       v
wire
```

Optimizing only the io_uring queue may miss the real bottleneck.

---

# 144. Zero-copy mental model

Always ask:

```text
How many memory ownership transitions occur?
```

For a network receive:

```text
NIC DMA
   |
   v
kernel buffer
   |
   v
user buffer
```

There may be copies.

A zero-copy design tries to change ownership/reference relationships:

```text
NIC DMA buffer
   |
   v
application-visible buffer
```

But this requires cooperation across:

- hardware
- driver
- network stack
- memory management
- io_uring
- application

Therefore zero-copy is an end-to-end design property.

---

# 145. File descriptors versus file descriptions

Linux has a distinction between:

```text
file descriptor
```

and:

```text
open file description
```

Several descriptors can reference the same underlying open file description.

io_uring registered resources can add another layer.

When debugging resource lifetime, draw:

```text
fd number
   |
   v
fd table entry
   |
   v
struct file
   |
   v
inode/socket/etc.
```

Then add:

```text
io_uring registered reference
```

where applicable.

---

# 146. Close semantics

An asynchronous close can race with operations.

Bad mental model:

```text
close(fd)
=> everything involving fd instantly disappears
```

Better:

```text
fd table reference
      |
      v
underlying file object
      |
      +--> outstanding request reference
```

The kernel manages object lifetimes through references.

Application-level correctness still requires you to avoid issuing logically invalid work.

---

# 147. Ordering close after I/O

If you need:

```text
WRITE
then CLOSE
```

do not rely on incidental scheduling.

Use:

```text
WRITE
 |
 v
completion
 |
 v
CLOSE
```

or an appropriate linked chain.

---

# 148. File operation composition

A useful io_uring pattern:

```text
OPEN
  |
  v
READ
  |
  v
WRITE
  |
  v
FSYNC
  |
  v
CLOSE
```

This can represent an entire workflow.

But remember:

```text
dependency chain
!=
transaction
```

If `WRITE` fails, application semantics still determine what cleanup/recovery means.

---

# 149. Timeout + I/O pattern

A common asynchronous state machine:

```text
              +----------+
              | READ     |
              +----+-----+
                   |
          +--------+--------+
          |                 |
       success            timeout
          |                 |
          v                 v
       process            cancel
                            |
                            v
                           close
```

The key challenge is cancellation race handling.

A robust design must handle:

```text
read completion arrives first
timeout arrives first
cancel completion arrives first
multiple completions
```

---

# 150. Protocol-level timeout

Do not confuse:

```text
kernel operation timeout
```

with:

```text
application protocol timeout
```

Example:

```text
HTTP request timeout = 5 seconds
```

may mean:

```text
total request lifecycle <= 5s
```

not:

```text
each individual recv <= 5s
```

The application state machine must define the semantic boundary.

---

# 151. File-server design

A high-performance file server might use:

```text
TCP
 |
 v
accept multishot
 |
 v
recv multishot
 |
 v
parse request
 |
 v
openat / stat
 |
 v
read / splice / send
 |
 v
response
```

Potential optimizations:

- registered buffers
- fixed files
- multishot receive
- batching
- zero-copy paths
- CPU affinity
- cache-aware buffer pools

But each optimization adds complexity.

---

# 152. Proxy design

A proxy has two sides:

```text
client
  |
  v
socket A
  |
  v
proxy
  |
  v
socket B
```

The state machine is:

```text
A readable
   |
   v
read A
   |
   v
queue to B
   |
   v
write B

B readable
   |
   v
read B
   |
   v
queue to A
   |
   v
write A
```

Backpressure is crucial.

If B is slow:

```text
A -> proxy buffer -> B
```

the proxy must stop accepting unlimited data from A.

---

# 153. Database architecture

A database can use io_uring for:

- WAL writes
- data-file reads
- fsync
- metadata operations
- background storage tasks

But databases often have their own:

- buffer pool
- scheduler
- WAL manager
- cache
- transaction system

So the correct design may be:

```text
database scheduler
        |
        v
io_uring
        |
        v
kernel
```

rather than exposing raw io_uring operations to every transaction.

---

# 154. Storage engine state machine

Example:

```text
LOGICAL WRITE
     |
     v
append WAL
     |
     v
WAL WRITE CQE
     |
     v
WAL FLUSH
     |
     v
FLUSH CQE
     |
     v
transaction durable
```

The key point:

```text
completion order
```

must be mapped to:

```text
database durability semantics
```

---

# 155. io_uring and block layer

For storage operations, understand:

```text
io_uring
   |
   v
VFS
   |
   v
filesystem
   |
   v
iomap/direct I/O
   |
   v
bio
   |
   v
blk-mq
   |
   v
driver
   |
   v
hardware
```

The exact path varies.

This is why learning io_uring is also a gateway into:

- VFS
- filesystems
- block layer
- NVMe
- page cache
- direct I/O

---

# 156. io_uring and networking

For networking:

```text
io_uring
   |
   v
socket layer
   |
   v
TCP/UDP
   |
   v
IP
   |
   v
qdisc / driver
   |
   v
NIC
```

Receive:

```text
NIC
 |
 v
driver
 |
 v
network stack
 |
 v
socket receive queue
 |
 v
io_uring recv
 |
 v
CQE
```

Send reverses the direction.

---

# 157. Interrupts versus polling

Traditional network I/O:

```text
packet arrives
    |
    v
interrupt
    |
    v
kernel
```

Polling:

```text
kernel repeatedly checks
    |
    v
packet
```

Polling can reduce interrupt overhead at high rates but costs CPU.

io_uring can participate in broader polling architectures, but the exact hardware/driver path matters.

---

# 158. NAPI

Linux networking commonly uses NAPI to balance interrupt-driven and polling behavior.

This is important because:

```text
io_uring recv
```

does not replace:

```text
NAPI
```

The packet must still travel through the network subsystem.

For serious networking performance work, study both.

---

# 159. Error taxonomy

A useful model:

### Submission error

The SQE could not be submitted.

### Operation validation error

The kernel rejects the SQE.

### Immediate operation error

The backend rejects it immediately.

### Async operation error

The operation starts but later fails.

### Partial success

Some work succeeds, but not all.

### Cancellation

The operation is canceled.

### Resource exhaustion

Examples:

- buffers
- files
- memory
- queue entries
- worker capacity

Applications should classify errors rather than treat everything as "I/O failed."

---

# 160. Retry semantics

Some errors are transient.

Examples may include:

```text
EAGAIN
EINTR
resource temporarily unavailable
```

But retrying blindly can create:

```text
busy loop
```

A good retry policy is:

```text
classify
 |
 +--> transient --> bounded retry/backoff
 |
 +--> permanent --> fail request
 |
 +--> cancellation --> cleanup
```

The exact meaning is operation-specific.

---

# 161. Resource exhaustion

A robust server must have limits:

```text
MAX_CONNECTIONS
MAX_OUTSTANDING
MAX_BUFFER_MEMORY
MAX_REQUEST_SIZE
MAX_OUTPUT_QUEUE
```

Then:

```text
if limit reached:
    apply backpressure
```

Without limits:

```text
load spike
   |
   v
more requests
   |
   v
more buffers
   |
   v
memory pressure
   |
   v
latency collapse
```

---

# 162. Graceful shutdown

A robust io_uring service needs a shutdown state:

```text
RUNNING
   |
   v
STOP_ACCEPTING
   |
   v
DRAIN
   |
   v
CANCEL_REMAINING
   |
   v
WAIT_FOR_COMPLETIONS
   |
   v
UNREGISTER_RESOURCES
   |
   v
DESTROY_RING
```

Do not simply:

```text
io_uring_queue_exit()
```

while arbitrary application state still assumes operations are alive.

---

# 163. Graceful connection shutdown

For TCP:

```text
STOP_ACCEPT
    |
    v
existing connections
    |
    +--> finish reads/writes
    |
    +--> protocol shutdown
    |
    v
close
```

Cancellation can be used, but application semantics should decide whether work is abandoned or drained.

---

# 164. Testing strategy

Test:

```text
normal success
short read
short write
EOF
EAGAIN
invalid fd
closed fd
cancellation
timeout
queue full
CQ pressure
multishot termination
buffer exhaustion
connection reset
shutdown
```

Then stress:

```text
1 connection
10
100
1,000
10,000+
```

depending on system limits.

---

# 165. Fault injection

High-quality asynchronous code must be tested under failures.

Inject:

```text
short writes
random latency
random cancellation
random disconnect
memory pressure
file errors
disk-full
connection reset
```

The state machine should remain correct.

---

# 166. Deterministic testing

Because asynchronous completion order varies, tests should not assume:

```text
request 1 completes before request 2
```

Instead assert:

```text
all expected operations eventually complete
and final state is correct
```

If ordering is required, encode ordering explicitly.

---

# 167. Race detection

C applications can use:

```text
ASan
UBSan
TSan where applicable
```

and stress testing.

Rust adds stronger compile-time guarantees but does not eliminate:

- kernel races
- protocol races
- incorrect unsafe code
- logical state-machine bugs

---

# 168. Security testing

Test:

- malicious file paths
- invalid offsets
- huge lengths
- invalid descriptors
- resource exhaustion
- cancellation races
- concurrent close
- malformed network packets
- untrusted `user_data` mappings if applicable
- sandbox restrictions

---

# 169. Observability

A production service should expose:

```text
submitted operations
completed operations
in-flight requests
queue depth
CQ backlog
timeouts
cancellations
errors by errno
buffer pool occupancy
CPU utilization
worker usage
latency percentiles
```

These metrics tell you whether the system is:

```text
CPU bound
I/O bound
queue bound
memory bound
scheduler bound
```

---

# 170. A complete conceptual request lifecycle

This is the single most important diagram to memorize:

```text
                  USER SPACE
                      |
                      | create request state
                      v
              +---------------+
              | Request object|
              +-------+-------+
                      |
                      | prepare SQE
                      v
              +---------------+
              |      SQE      |
              +-------+-------+
                      |
                      | publish SQ tail
                      v
              +---------------+
              | Submission Q  |
              +-------+-------+
                      |
                enter / SQPOLL
                      |
======================|====================== KERNEL
                      v
              +---------------+
              | fetch + parse |
              +-------+-------+
                      |
                      v
              +---------------+
              | validate SQE  |
              +-------+-------+
                      |
              +-------+-------+
              |               |
              v               v
         execute now      defer/async
                              |
                     +--------+--------+
                     |                 |
                     v                 v
                   io-wq          device/filesystem/
                                  network/task work
                     |                 |
                     +--------+--------+
                              |
                              v
                       operation result
                              |
                              v
                       generate CQE
                              |
==============================|====================
                              |
                         USER SPACE
                              v
                       +-------------+
                       |     CQE     |
                       +------+------+ 
                              |
                              | user_data
                              v
                       Request object
                              |
                              v
                        state update
                              |
                              v
                       recycle/free
```

If this diagram is clear, most of io_uring becomes easier.

---

# 171. What happens on an x86-64 system

A simplified syscall path is:

```text
userspace
   |
   | syscall instruction
   v
x86-64 syscall entry
   |
   v
kernel syscall dispatch
   |
   v
io_uring_enter
   |
   v
io_uring submission processing
```

The exact low-level entry code depends on kernel configuration and mitigations.

The important architecture lesson is:

```text
userspace ABI
    |
    v
architecture syscall mechanism
    |
    v
generic kernel syscall implementation
```

The io_uring UAPI itself is designed to be architecture-independent.

---

# 172. ARM64 mental model

On AArch64:

```text
userspace
   |
   | svc
   v
kernel exception entry
   |
   v
syscall dispatch
   |
   v
io_uring_enter
```

Again:

```text
architecture-specific entry
        |
        v
generic io_uring implementation
```

The shared ring protocol must remain correct under the architecture's memory-ordering model.

---

# 173. Why weak memory models matter

Imagine:

```text
CPU A: application
CPU B: kernel
```

Application:

```text
write SQE
write tail
```

Without correct ordering, CPU B could observe:

```text
tail updated
SQE still stale
```

The protocol therefore uses release/acquire semantics.

This is the same fundamental problem encountered in:

- lock-free queues
- producer/consumer buffers
- networking descriptor rings
- device DMA rings

Learning io_uring is therefore excellent training for concurrent systems programming.

---

# 174. io_uring resembles hardware descriptor rings

A useful deeper analogy:

```text
software SQE ring
```

resembles:

```text
NIC descriptor ring
NVMe submission queue
GPU command queue
```

All involve:

```text
producer
descriptor
publication
consumer
completion
recycling
```

Once you understand io_uring, many device-driver architectures become easier to understand.

---

# 175. Descriptor lifecycle analogy

For a NIC:

```text
driver prepares descriptor
      |
      v
publish descriptor
      |
      v
device consumes
      |
      v
device completion
      |
      v
driver recycles
```

For io_uring:

```text
application prepares SQE
      |
      v
publish SQ
      |
      v
kernel consumes
      |
      v
kernel completion
      |
      v
application recycles
```

This is a powerful systems mental model.

---

# 176. Why shared queues are efficient

Suppose you need 64 operations.

Traditional model:

```text
64 operations
   |
   +--> 64 syscall transitions
```

io_uring:

```text
64 SQEs
   |
   +--> one submission transition
```

The fixed cost is amortized.

At high request rates, this matters.

But the actual performance benefit depends on how much time is spent in the backend.

---

# 177. Syscall amortization equation

Simplified:

```text
traditional cost
≈ N * syscall_cost + N * io_cost

io_uring
≈ 1 * syscall_cost + N * queue_cost + N * io_cost
```

As `N` increases:

```text
syscall_cost / N
```

decreases.

This is why batching is fundamental.

---

# 178. Why SQPOLL can reduce it further

With SQPOLL:

```text
normal:
user -> syscall -> kernel notices SQ

SQPOLL:
user -> shared SQ
            ^
            |
        polling kernel thread
```

The syscall can sometimes be removed from the submission fast path.

But the cost becomes:

```text
dedicated CPU polling
```

So:

```text
syscall overhead
```

is exchanged for:

```text
CPU polling overhead
```

---

# 179. io_uring versus threads

A common misconception:

```text
io_uring eliminates threads
```

Not necessarily.

A design can be:

```text
1000 requests
+
1 event-loop thread
+
kernel async operations
```

instead of:

```text
1000 requests
+
1000 application threads
```

This can reduce application scheduling overhead.

But kernel worker threads may still be involved.

---

# 180. io_uring versus thread-per-request

Thread-per-request:

```text
request
  |
  v
thread
  |
  v
blocking I/O
```

io_uring:

```text
request
  |
  v
SQE
  |
  v
kernel
  |
  v
CQE
```

The latter can avoid creating/scheduling an application thread for every operation.

---

# 181. io_uring versus event-driven nonblocking I/O

With epoll:

```text
readiness
   |
   v
application
   |
   v
actual I/O
```

With io_uring:

```text
operation
   |
   v
kernel
   |
   v
completion
```

This reduces application-level "ready -> perform operation" transitions.

But io_uring can also express readiness operations.

---

# 182. The "one ring per worker" architecture

A common scalable pattern:

```text
             load balancer
                  |
       +----------+----------+
       |          |          |
       v          v          v
    worker 0   worker 1   worker 2
       |          |          |
     ring 0     ring 1     ring 2
       |          |          |
     CPU 0      CPU 1      CPU 2
```

This is especially attractive when connections can be assigned to workers.

---

# 183. Work stealing versus ownership

If one worker is overloaded:

```text
worker 0: 1000 requests
worker 1: 10
```

you can:

- rebalance connections
- use a shared queue
- transfer work
- use ring-to-ring messaging
- use a runtime scheduler

Again, io_uring supplies the I/O mechanism; application architecture decides scheduling.

---

# 184. Ring sharding

Sharding by:

```text
CPU
NUMA node
tenant
connection group
storage queue
```

can improve locality.

The design objective is:

```text
minimize cross-core shared mutable state
```

---

# 185. Resource pooling

For high performance:

```text
request pool
buffer pool
connection pool
file table
```

can reduce allocation overhead.

A pool also makes ownership explicit:

```text
FREE
 |
 v
IN_FLIGHT
 |
 v
COMPLETE
 |
 v
FREE
```

This is a good generic state machine.

---

# 186. Memory budgeting

Suppose:

```text
buffer size = 64 KiB
buffers = 100,000
```

Then:

```text
64 KiB * 100,000 ≈ 6.4 GiB
```

Registration can make memory costs more visible.

Always budget:

```text
ring memory
+
SQEs
+
CQEs
+
buffers
+
request objects
+
socket buffers
+
filesystem cache
```

---

# 187. Resource limits

Linux has multiple relevant limits, including:

- file descriptor limits
- memory limits
- locked/pinned memory constraints
- cgroup limits
- process limits
- network buffer limits

A production io_uring design should understand the limits imposed by its deployment environment.

---

# 188. Containers and io_uring

Containers share the host kernel.

Therefore:

```text
container application
       |
       v
host kernel io_uring
```

Capabilities, seccomp, cgroups, and kernel version matter.

A container image cannot independently choose a newer io_uring kernel feature if the host kernel does not support it.

---

# 189. Kernel version compatibility

Think:

```text
application binary
       |
       v
io_uring UAPI
       |
       v
host kernel
```

The host kernel is the ultimate authority.

A newer userspace library can run on an older kernel only to the extent that it gracefully handles unavailable features.

Therefore runtime probing/fallback matters.

---

# 190. Portability

io_uring is Linux-specific.

It is not:

```text
POSIX async I/O
```

If portability matters, isolate it behind an abstraction:

```text
application
    |
async I/O interface
    |
 +--+-----------+-----------+
 |              |           |
Linux         BSD         other
io_uring      kqueue      ...
```

---

# 191. API abstraction design

A useful abstraction is:

```text
submit_read()
submit_write()
submit_recv()
submit_send()
cancel()
timeout()
```

rather than leaking:

```text
struct io_uring_sqe
```

through the whole application.

This allows:

- testing
- alternate backends
- platform portability
- simpler application code

But avoid abstractions so high-level that important performance semantics disappear.

---

# 192. Good abstraction boundary

Keep these visible to the I/O layer:

```text
buffer ownership
operation lifetime
completion
backpressure
queue depth
cancellation
```

Keep these hidden from most application code:

```text
SQ index manipulation
memory barriers
mmap offsets
syscall details
```

This is exactly where liburing is useful.

---

# 193. Learning plan

A strong progression is:

```text
1. Blocking read/write
        |
2. epoll
        |
3. producer/consumer ring buffers
        |
4. memory ordering
        |
5. io_uring SQ/CQ
        |
6. simple read
        |
7. multiple outstanding reads
        |
8. TCP echo server
        |
9. multishot
       |
10. fixed buffers/files
       |
11. SQPOLL
       |
12. cancellation/timeouts
       |
13. zero-copy
       |
14. kernel source tracing
       |
15. performance engineering
```

This builds the concepts in the right order.

---

# 194. Exercises that build real understanding

## Exercise 1

Implement:

```text
one file read
```

using liburing.

## Exercise 2

Implement:

```text
64 simultaneous reads
```

and measure throughput.

## Exercise 3

Implement:

```text
TCP echo server
```

with one connection at a time.

## Exercise 4

Scale to:

```text
1000 concurrent connections
```

## Exercise 5

Add:

```text
multishot accept
```

## Exercise 6

Add:

```text
buffer selection
```

## Exercise 7

Add:

```text
timeouts + cancellation
```

## Exercise 8

Add:

```text
registered buffers
```

## Exercise 9

Create:

```text
one ring per worker
```

## Exercise 10

Compare:

```text
epoll
vs
io_uring
```

under the same workload.

---

# 195. Kernel-reading exercise

Take one read operation.

Trace:

```text
userspace SQE
    |
    v
io_uring_enter
    |
    v
SQE extraction
    |
    v
opcode dispatch
    |
    v
read preparation
    |
    v
VFS
    |
    v
filesystem
    |
    v
completion
    |
    v
CQE
```

Write down:

```text
request object
resource references
buffer references
locks
possible blocking points
completion path
```

Do the same for `recv`.

Then compare.

That exercise teaches much more than memorizing documentation.

---

# 196. Source archaeology checklist

For any opcode, ask:

```text
[ ] Where is opcode defined?
[ ] Where is it dispatched?
[ ] What SQE fields does it consume?
[ ] What kernel request state is allocated?
[ ] What resources are referenced?
[ ] Can it complete synchronously?
[ ] Can it block?
[ ] Can it go to io-wq?
[ ] Can it be canceled?
[ ] Can it be multishot?
[ ] How is CQE generated?
[ ] What does cqe->res mean?
[ ] What do cqe->flags mean?
[ ] When is request memory freed?
```

This checklist is useful far beyond io_uring.

---

# 197. Systems thinking: identify queues

When analyzing any high-performance I/O system, draw every queue:

```text
application queue
      |
      v
io_uring SQ
      |
      v
kernel request queue
      |
      v
filesystem/socket queue
      |
      v
block/NIC queue
      |
      v
hardware queue
```

Then ask:

```text
Which queue is full?
Which queue is empty?
Which queue adds latency?
Which queue provides backpressure?
```

This is one of the most powerful ways to reason about performance.

---

# 198. Systems thinking: identify ownership

For every buffer:

```text
Who owns it now?
```

For every request:

```text
Who can complete it?
```

For every resource:

```text
Who holds a reference?
```

For every queue:

```text
Who can modify the head?
Who can modify the tail?
```

These questions prevent many asynchronous bugs.

---

# 199. Systems thinking: identify synchronization

For every shared variable:

```text
who writes it?
who reads it?
when?
under what ordering?
```

For a ring:

```text
producer writes entries
producer publishes tail
consumer reads tail
consumer reads entries
consumer publishes head
producer reads head
```

Then determine:

```text
where acquire/release is required
```

This is the mental model behind lock-free structures.

---

# 200. Systems thinking: identify latency boundaries

For every operation:

```text
submit
 |
 +--> userspace overhead
 |
 +--> syscall
 |
 +--> kernel dispatch
 |
 +--> scheduling
 |
 +--> backend
 |
 +--> device/network
 |
 +--> completion
 |
 +--> userspace wakeup
 |
 +--> application processing
```

Measure each boundary if performance matters.

---

# 201. Systems thinking: distinguish mechanisms from policies

io_uring is a **mechanism**:

```text
submit
complete
register
wait
```

Your application supplies policies:

```text
how many requests?
which CPU?
which buffer?
when to retry?
when to cancel?
when to backpressure?
when to close?
```

This separation is fundamental to good systems architecture.

---

# 202. A compact comparison table

| Concept | io_uring interpretation |
|---|---|
| SQE | command/request descriptor |
| SQ | user-to-kernel submission ring |
| CQE | completion/result descriptor |
| CQ | kernel-to-user completion ring |
| `user_data` | application correlation token |
| `res` | operation result / negative errno |
| `io_uring_setup` | create ring |
| `io_uring_enter` | submit/wait/drive |
| `io_uring_register` | register resources/features |
| SQPOLL | kernel polls submission queue |
| fixed buffer | pre-registered buffer resource |
| fixed file | pre-registered file resource |
| multishot | one request can produce multiple CQEs |
| link | dependency between requests |
| io-wq | worker mechanism for blocking work |
| CQ flags | extra completion metadata |
| buffer selection | kernel chooses from registered pool |
| `uring_cmd` | subsystem/driver command interface |
| `user_data` | request identity |
| CQE | completion notification |

---

# 203. The five rules worth memorizing

## Rule 1

**An SQE is a command, not a promise of immediate execution.**

## Rule 2

**A CQE is a completion, not necessarily an application-level completion.**

## Rule 3

**Buffers and resources must remain valid until the kernel is finished with them.**

## Rule 4

**Completion order is not submission order unless you establish the dependency.**

## Rule 5

**Performance comes from reducing coordination overhead while keeping enough work in flight to saturate the backend.**

---

# 204. The deepest mental model

The most productive way to think about io_uring is:

```text
io_uring = asynchronous state-machine transport
```

The application owns logical state:

```text
Request
Connection
Buffer
Timeout
File
Protocol
```

The kernel owns execution state:

```text
I/O request
socket
filesystem
device
worker
timer
```

The rings connect them:

```text
application state
       |
       | SQE
       v
kernel execution state
       |
       | CQE
       v
application state transition
```

So the application is essentially a distributed state machine across a user/kernel boundary.

---

# 205. Final architecture to keep in your head

```text
                           APPLICATION
                               |
                +--------------+--------------+
                |                             |
         request state                   buffer pools
                |                             |
                +--------------+--------------+
                               |
                               v
                     +-------------------+
                     | Submission Queue  |
                     |       SQEs        |
                     +---------+---------+
                               |
                       release/publish
                               |
===============================|================================
                               |
                               v
                        io_uring kernel
                               |
                +--------------+--------------+
                |              |              |
                v              v              v
              VFS           sockets        timers/etc.
                |              |              |
                v              v              v
          filesystem       TCP/UDP        kernel timer
                |              |              |
                +--------------+--------------+
                               |
                               v
                       async backend / io-wq
                               |
                               v
                          completion
                               |
                               v
                     +-------------------+
                     | Completion Queue  |
                     |       CQEs        |
                     +---------+---------+
                               |
                         acquire/consume
                               |
===============================|================================
                               |
                               v
                         APPLICATION
                               |
                               v
                     state transition/recycle
```

If you can mentally simulate this diagram for:

```text
read
write
accept
recv
send
timeout
cancel
multishot receive
fixed buffer
fixed file
```

you have the core of io_uring.

---

# 206. Reference material

Use primary documentation when exact semantics matter.

- Linux `io_uring(7)` describes the programming model, shared rings, batching, and examples. citeturn0search2turn0search3
- Linux kernel documentation is the authoritative source for kernel internals and current subsystem documentation. citeturn1search8turn1search10
- The kernel maintainer documentation identifies the major io_uring implementation files. citeturn1search0
- liburing is the standard helper library and its repository contains examples and tests. citeturn0search8turn0search9
- The official liburing example tree includes file copy, networking, polling, zero-copy, registration, and other examples. citeturn0search9
- The Rust `io-uring` crate provides a low-level Rust interface and documents its supported architectures and unsafe lifetime requirements. citeturn0search0turn0search7
- Linux's zero-copy receive documentation is useful for understanding modern buffer registration/refill-ring designs. citeturn1search1

Useful commands:

```bash
man 7 io_uring
man 2 io_uring_setup
man 2 io_uring_enter
man 2 io_uring_register
```

Source trees:

```text
include/uapi/linux/io_uring.h
include/linux/io_uring.h
fs/io_uring.c
fs/io-wq.c
fs/io-wq.h
tools/io_uring/
tools/testing/selftests/
```

---

# 207. Recommended mastery order

Do not try to memorize the entire opcode list.

Master these layers in order:

```text
Layer 1:
    blocking I/O

Layer 2:
    readiness / epoll

Layer 3:
    ring buffers

Layer 4:
    acquire/release memory ordering

Layer 5:
    io_uring SQ/CQ

Layer 6:
    one-shot read/write

Layer 7:
    batching + concurrency

Layer 8:
    networking

Layer 9:
    fixed resources

Layer 10:
    multishot + buffer selection

Layer 11:
    links + cancellation + timeout

Layer 12:
    SQPOLL + polling

Layer 13:
    kernel source + io-wq

Layer 14:
    zero-copy + uring_cmd

Layer 15:
    NUMA + CPU locality + performance analysis
```

At the end of that progression, io_uring stops looking like a collection of obscure Linux APIs and starts looking like what it really is:

> **A carefully engineered shared-memory asynchronous command/completion system sitting at the boundary between application state machines and Linux's I/O subsystems.**

That mental model transfers directly to:

- NVMe queues
- NIC descriptor rings
- DMA engines
- GPU command queues
- lock-free queues
- asynchronous runtimes
- database storage engines
- high-performance network servers
- kernel driver architecture

And that is the deeper reason io_uring is worth learning: the concepts behind it are much broader than the API itself.
