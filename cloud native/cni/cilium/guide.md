## Learning Cilium: A Roadmap for Go Developers

Great background to have! Cilium sits at the intersection of Linux networking, eBPF, and Kubernetes — your Go + Linux/networking knowledge gives you a solid head start.

---

### What is Cilium, really?

Cilium is a **CNI (Container Network Interface) plugin** for Kubernetes that uses **eBPF** (extended Berkeley Packet Filter) to enforce networking, security, and observability policies — all at the Linux kernel level, without iptables.

```
[ Your App Pods ]
       ↓
[ Cilium Agent ]  ←→  [ Hubble (observability) ]
       ↓
[ eBPF Programs in Linux Kernel ]
       ↓
[ Network / Datapath ]
```

---

### Prerequisites to fill first

Before diving into Cilium source code, make sure you're solid on:

**1. eBPF basics** (most important)
- Cilium is essentially a sophisticated eBPF program manager
- Start here: [ebpf.io/what-is-ebpf](https://ebpf.io/what-is-ebpf/)
- Then: Liz Rice's free book *"Learning eBPF"* (O'Reilly, free PDF on isovalent.com)

**2. Kubernetes networking model**
- Understand CNI, pods, services, kube-proxy
- How `ClusterIP`, `NodePort`, and `LoadBalancer` work under the hood

**3. Linux networking primitives**
- `veth` pairs, network namespaces, `tc` (traffic control), XDP
- `ip` command, `bpftool`, `iproute2`

---

### Entry Points into Cilium

#### 🔵 Conceptual entry (start here)
1. **Cilium docs** → [docs.cilium.io](https://docs.cilium.io) — read the "Concepts" section top to bottom
2. **Cilium's Architecture page** — understand the Agent, Operator, Hubble, and CNI plugin roles

#### 🟢 Hands-on entry (do this in parallel)
```bash
# Spin up a local cluster with Kind + Cilium
curl -LO https://kind.sigs.k8s.io/dl/latest/kind-linux-amd64
cilium install   # via cilium CLI
cilium status
cilium connectivity test
```
Use **minikube** or **kind** — don't waste time on a full cluster yet.

#### 🔴 Code entry (once you're hands-on)
The Cilium repo is at `github.com/cilium/cilium`. Key directories:

| Directory | What it is |
|---|---|
| `daemon/` | The main **cilium-agent** — start reading here |
| `pkg/ebpf/` | eBPF map management in Go |
| `bpf/` | The actual eBPF C programs (kernel side) |
| `pkg/policy/` | Network policy engine |
| `pkg/endpoint/` | Per-pod endpoint lifecycle |
| `cilium-health/` | Cluster connectivity checker |
| `hubble/` | Observability layer |

**The `daemon/cmd/daemon.go` file is the true entry point** — it bootstraps the agent.

---

### Suggested Learning Path

```
Week 1-2:  eBPF fundamentals + Cilium concepts docs
Week 3:    Local cluster setup, play with NetworkPolicy, Hubble UI
Week 4:    Read cilium-agent startup code (daemon/cmd/)
Week 5+:   Pick one subsystem (policy? datapath? hubble?) and go deep
```

---

### Best Resources

| Resource | Type |
|---|---|
| [docs.cilium.io/concepts](https://docs.cilium.io/en/stable/concepts/) | Official docs |
| Liz Rice – *Learning eBPF* | Free book (isovalent.com) |
| Cilium Labs on [isovalent.com/labs](https://isovalent.com/labs/) | Interactive browser labs (free!) |
| [github.com/cilium/cilium](https://github.com/cilium/cilium) | Source code |
| Cilium Slack (`cilium.slack.com`) | Community, very active |
| `#development` channel in Cilium Slack | For code questions |

---

### As a Go dev, you'll feel at home in

- `pkg/` — pure Go packages, well-structured
- The use of `github.com/cilium/ebpf` (the Go eBPF library Cilium uses)
- Their use of interfaces for policy resolution — good Go design patterns

The **`cilium/ebpf`** Go library is actually a great side project to study — it's the library that loads and manages eBPF programs from Go, and it's clean, well-documented Go code.

---

**TL;DR starting point:** Do the [Isovalent free labs](https://isovalent.com/labs/) first — they're interactive, browser-based, and get you from zero to writing NetworkPolicies and reading Hubble flows in under 2 hours. Then read the daemon source code with that hands-on context in mind.

Your mental model is **mostly correct** — good intuition! But there are a few important nuances to refine.Your mental model is **mostly correct** — good intuition! But there are a few important nuances to refine.

Here's a corrected and more precise picture:### What you got right ✅

- **Cilium runs in userspace** — yes, it's a Go process running as a DaemonSet pod on each node.
- **eBPF runs in kernel space** — correct.
- **eBPF is sandboxed** — yes, but the "sandbox" works differently than you think (see below).
- **Communication via syscall** — yes, specifically the `bpf()` syscall.

---

### What to refine 🔧

**1. The "sandbox" is a load-time verifier, not a runtime container**

This is the key correction. eBPF is NOT like a VM or WASM sandbox running at runtime. Instead:

- When Cilium loads an eBPF program, the kernel runs a **static verifier** that proves the program is safe — no infinite loops, no out-of-bounds memory, no crashing the kernel.
- Once the verifier approves it, the program is **JIT-compiled to native machine code** and runs directly in the kernel — **zero overhead sandbox**, because there's no sandbox anymore.
- Think of it like a **strict code review before merging to production**, not a containerised runtime.

**2. The "hook" mechanism**

Your mental model of hooks is correct, but hooks are **attachment points in the kernel**, not just an API. Examples:

| Hook type | When it fires |
|---|---|
| `XDP` | Earliest possible — packet arrives at NIC driver |
| `tc` (traffic control) | Packet in the kernel network stack |
| `kprobe` | Any kernel function call |
| `tracepoint` | Predefined kernel trace events |

Cilium attaches programs to `XDP` and `tc` hooks to intercept every packet going in/out of a pod.

**3. Communication is primarily via eBPF Maps, not just syscalls**

The `bpf()` syscall is used to **load programs** and **create/access maps**. But at runtime, the main communication channel is **eBPF Maps**:

- Maps are a **shared key-value store** in kernel memory, accessible from both kernel (eBPF program) and userspace (Cilium agent) simultaneously.
- Cilium writes policy rules into maps → eBPF program reads them to make decisions on every packet.
- eBPF program writes flow events into maps → Hubble reads them for observability.

So the flow is:
```
Cilium agent  --[bpf() syscall]-→  loads eBPF program into kernel
Cilium agent  --[bpf() syscall]-→  writes policy into eBPF Map
                                   eBPF program reads map on each packet
                                   eBPF program writes events into map
Hubble        --[bpf() syscall]-→  reads events from map
```

---

### Your corrected mental model

> Cilium runs in userspace and uses the `bpf()` syscall to load eBPF programs into the kernel. Before running, programs pass through a **static verifier** (load-time safety check), then get JIT-compiled to native code. The programs attach to kernel hooks (XDP, tc) and execute on every packet. Cilium and eBPF programs share state through **eBPF Maps** — kernel memory readable/writable from both sides via the `bpf()` syscall.

That's a solid mental model to build on!

Great question — this is one of the most interesting parts of Cilium's internals. There are **two phases**: compile time and runtime.The full workflow has two completely separate phases. Let's walk through them.Now let's walk through each step with the actual code involved.

---

### Phase 1 — Compile time

Cilium's eBPF programs are written in **C** (not Go), living in `bpf/*.c`. They're compiled to eBPF bytecode using clang with the BPF target:

```bash
clang -O2 -target bpf -c bpf/bpf_lxc.c -o bpf_lxc.o
```

This produces an **ELF `.o` file** — same format as a normal compiled object, but with eBPF bytecode inside instead of x86 instructions, and special ELF sections like `maps` and `xdp` that the loader knows how to parse.

Cilium then uses a tool called `bpf2go` to **embed these ELF files into the Go binary** at build time:

```go
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang LxcPolicy bpf/bpf_lxc.c
```

This generates Go code with the bytecode inlined, plus typed Go structs for every map and program. No external files needed at runtime.

---

### Phase 2 — Runtime (inside cilium-agent)

When a pod is scheduled, the agent's endpoint manager triggers **datapath regeneration**. Here's what happens in code — using the `cilium/ebpf` Go library:

**Step 1 — Create maps** (`BPF_MAP_CREATE` syscall)

```go
// pkg/maps/policymap/policymap.go
m, err := ebpf.NewMap(&ebpf.MapSpec{
    Type:       ebpf.Hash,
    KeySize:    uint32(unsafe.Sizeof(PolicyKey{})),
    ValueSize:  uint32(unsafe.Sizeof(PolicyEntry{})),
    MaxEntries: maxEntries,
})
```

Maps are created first because the program will reference them by **file descriptor**.

**Step 2 — Load the program** (`BPF_PROG_LOAD` syscall)

```go
// cilium/ebpf library handles this
spec, _ := loadLxcPolicy()   // generated by bpf2go, reads embedded ELF
spec.RewriteConstants(...)   // patch per-endpoint config into the bytecode
objs := LxcPolicyObjects{}
ebpf.LoadAndAssign(&objs, &ebpf.CollectionOptions{
    Maps: ebpf.MapOptions{...},
})
```

`BPF_PROG_LOAD` sends the bytecode + map fds into the kernel. The kernel then:
- Runs the **verifier** — static analysis proving no infinite loops, no invalid memory access, all paths terminate. This is the "sandbox" moment. If it fails, the syscall returns an error with a log of what went wrong.
- **JIT-compiles** the verified bytecode to native machine code.
- Returns a **program fd** (file descriptor) back to userspace.

**Step 3 — Attach to a hook** (via `netlink`, not `bpf()`)

```go
// pkg/datapath/loader/netlink.go
link, _ := netlink.LinkByName("eth0")

// For tc (traffic control):
qdisc := &netlink.GenericQdisc{
    QdiscAttrs: netlink.QdiscAttrs{LinkIndex: link.Attrs().Index},
    QdiscType:  "clsact",
}
netlink.QdiscAdd(qdisc)

filter := &netlink.BpfFilter{
    FilterAttrs: netlink.FilterAttrs{
        LinkIndex: link.Attrs().Index,
        Parent:    netlink.HANDLE_MIN_INGRESS,
    },
    Fd:           objs.HandleIngress.FD(),
    DirectAction: true,
}
netlink.FilterAdd(filter)
```

Notice the attachment uses **netlink** (a separate Linux IPC mechanism for network config), not the `bpf()` syscall itself. XDP attachment is similar but uses `netlink.LinkSetXdpFdWithFlags`.

---

### Key files to read in the Cilium repo

| File | What it does |
|---|---|
| `bpf/bpf_lxc.c` | Main per-endpoint eBPF program (start reading here) |
| `pkg/datapath/loader/loader.go` | Orchestrates the whole load workflow |
| `pkg/datapath/loader/compile.go` | Runs clang, manages compilation |
| `pkg/datapath/loader/netlink.go` | Attaches programs to tc/XDP hooks |
| `pkg/maps/*/` | Map definitions for policy, CT, NAT etc. |
| `vendor/github.com/cilium/ebpf/` | The Go library doing the actual syscalls |

The `pkg/datapath/loader/loader.go` → `ReinitializeXDP()` and `reloadDatapath()` functions are the best entry points to trace through the full runtime flow.

Great question — it's not arbitrary. Each language has a reason it's a poor fit here, and Go has specific properties that make it nearly ideal.

---

### Why not the others?

**C / C++**
The eBPF programs *are* written in C — but that's kernel-side code where you have no choice. For the userspace agent, C/C++ gives you no memory safety, manual memory management, no garbage collection, and — critically — terrible concurrency primitives. Managing hundreds of endpoints simultaneously with raw pthreads is a nightmare. Cilium manages complex concurrent state (policy updates, endpoint lifecycle, map updates) where Go's goroutines shine. C++ could technically work but you'd be building what Go already gives you for free.

**Python**
The GIL kills real concurrency. It's dynamically typed, meaning eBPF map structs (which are binary-packed C structs) are painful to model safely. No static binary — you'd need Python installed on every node. And it's simply too slow for a networking daemon that reacts to pod creation events in milliseconds.

**Java / JVM languages**
JVM startup time is seconds — a CNI plugin needs to be fast. JVM memory footprint is large (a DaemonSet runs on *every* node). JVM GC pauses, even modern ones, are problematic for a low-latency networking agent. And Java has no real path to `bpf()` syscalls or netlink without heavy JNI.

**Bash / Perl**
Scripting languages. Fine for one-shot scripts, terrible for a long-running daemon managing complex state, error handling, and concurrent goroutines. Early Cilium prototypes actually *did* use bash scripts for some BPF loading — they were replaced precisely because it didn't scale.

**Rust**
This is the most interesting "why not". Rust is actually a *technically valid* alternative — the [Aya](https://aya-rs.dev/) framework proves it. But Cilium was started in **2015–2016** when Rust 1.0 had just launched and its ecosystem was nowhere near mature. More importantly:

---

### Why Go specifically

The real answer has four layers:

**1. The Kubernetes ecosystem is Go**

Cilium is a Kubernetes CNI plugin. It watches pods, services, and NetworkPolicy objects using `client-go`. It implements controller patterns using `controller-runtime`. It integrates with the Kubernetes API server. All of this ecosystem is Go — using any other language means translating across a boundary that didn't need to exist.

```go
// Cilium watches K8s events natively
informer := cache.NewSharedIndexInformer(
    &cache.ListWatch{...},
    &slim_corev1.Pod{},
    ...
)
```

**2. Single static binary**

Go compiles to a **single statically linked binary** with zero runtime dependencies. This is critical for a CNI plugin — it runs on bare nodes, in minimal container images, and needs to work on any Linux distribution. No "please install Python 3.9" on a node.

```bash
$ file cilium-agent
cilium-agent: ELF 64-bit LSB executable, statically linked
```

**3. Goroutines match the problem shape**

Cilium does many things concurrently: watching K8s API, managing endpoint regeneration, updating eBPF maps, handling health checks, serving Hubble gRPC streams. Each of these is naturally a goroutine. Go's scheduler handles thousands of goroutines with low overhead — this is much harder to model correctly in threads.

```go
// Each endpoint regenerates concurrently
for _, ep := range endpoints {
    go ep.regenerate(ctx)
}
```

**4. `unsafe` + `syscall` — systems access when you need it**

Go is a memory-safe language, but it has an escape hatch. When Cilium needs to pack a C struct exactly as the kernel expects it, or call `bpf()` directly, it uses `unsafe` and `golang.org/x/sys/unix`:

```go
// Direct bpf() syscall — Go lets you go low-level when needed
fd, _, errno := unix.Syscall(unix.SYS_BPF,
    uintptr(unix.BPF_PROG_LOAD),
    uintptr(unsafe.Pointer(&attr)),
    unsafe.Sizeof(attr),
)
```

This is the key tradeoff — **safe by default, unsafe when necessary**. Python and Java can't do this cleanly. C does it unsafely everywhere. Go threads the needle.

---

### The summary

```
Language    Memory safe?   Real concurrency?   Static binary?   K8s ecosystem?   Low-level syscalls?
──────────────────────────────────────────────────────────────────────────────────────────────────────
C/C++           ✗               hard                ✓               ✗                   ✓
Python          ✓               ✗ (GIL)             ✗               partial             awkward
Java            ✓               ✓                   ✗               ✗                   JNI pain
Rust            ✓               ✓                   ✓               ✗ (2016)            ✓
Go              ✓               ✓                   ✓               ✓                   ✓
```

Go is the only language that checks all five boxes — and in 2016 when Cilium was designed, Rust didn't yet have the ecosystem maturity to compete. Today Rust *could* be used (and Aya is a real project), but Go's K8s integration advantage remains significant.