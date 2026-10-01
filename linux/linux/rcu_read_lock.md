## RCU (Read-Copy-Update) Synchronization

**RCU** is a kernel synchronization primitive optimized for read-heavy workloads where readers vastly outnumber writers. It's the backbone of hot-path code like IP routing lookups, netfilter rules, and protocol handler dispatch.

### The Semantics

**`rcu_read_lock()` / `rcu_read_unlock()`** define a **read-side critical section**. Within this section:
- The reader is guaranteed that any RCU-protected data structure it holds a pointer to **will not be freed** until after `rcu_read_unlock()` returns.
- Readers do **not acquire locks**. They don't block. No spinlocks, no mutexes.
- Multiple readers can execute the critical section **concurrently**.

**Writers (on the RCU-protected data) do different work:**
- Use `synchronize_rcu()` to wait for all pre-existing readers to exit their critical sections
- Then safely free/modify the old data

### In This Code

```c
rcu_read_lock();
{
    const struct net_protocol *ipprot = rcu_dereference(inet_protos[protocol]);
    if (ipprot) {
        int ret = ipprot->handler(skb);
    }
}
rcu_read_unlock();
```

The reader (this function) ensures:
- No writer will free `inet_protos[protocol]` before this code is done using it
- The table can be updated by protocol registration code, but readers aren't blocked

### C Syntax

`rcu_read_lock()` and `rcu_read_unlock()` are **macros**, typically expanding to:

**On non-preemptible kernels:**
```c
#define rcu_read_lock()   do { } while (0)    // No-op; relies on non-preemption
#define rcu_read_unlock() do { } while (0)
```

**On preemptible kernels:**
```c
#define rcu_read_lock()   preempt_disable()   // Increment counter, disable preemption
#define rcu_read_unlock() preempt_enable()    // Decrement counter, re-enable preemption
```

The **bare braces `{ ... }`** are just a C block scope—they're a stylistic convention to visually delimit the critical section, making it obvious to reviewers.

### The `rcu_dereference()` Macro

This is **essential**. `rcu_dereference(inet_protos[protocol])` does:
1. Dereferences the pointer (`inet_protos[protocol]`)
2. Inserts a **compiler barrier** to prevent the compiler from optimizing away the dereference or reordering reads
3. In sparse-RCU-checked kernels: compile-time type checking to ensure you only dereference RCU pointers inside read-side critical sections

**Without `rcu_dereference()`**, the compiler might cache the pointer value across statements, defeating RCU's safety guarantees.

### Why It Matters Here

The `inet_protos[]` table is **global, mutable, hot-path data**:
- Writers (protocol registration via `inet_add_protocol()`) are rare (module load/unload)
- Readers (every incoming IP packet) are constant
- RCU is **free for readers** (no lock acquisition), making this dispatcher extremely fast

**Performance implication:** Millions of packets/sec flow through this code; `rcu_read_lock()` costs essentially nothing on a non-preemptible kernel, and only a counter increment on preemptible kernels.