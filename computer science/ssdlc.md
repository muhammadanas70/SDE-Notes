# SSDLC in Linux Kernel, Network Subsystem & Cloud Network Security
## Complete Engineering Reference — Deep Dive

> *Build the mental models to think about systems security with clarity and efficiency.*
> *Every concept here connects: SSDLC shapes how you build, kernel architecture shapes what is possible, the network subsystem is the battleground, cloud is the operational reality.*

---

## TABLE OF CONTENTS

### PART I — SECURE SOFTWARE DEVELOPMENT LIFECYCLE
- Ch 1: Foundations & Mental Models
- Ch 2: Threat Modeling — STRIDE, PASTA, Attack Trees
- Ch 3: Security Requirements Engineering
- Ch 4: Secure Design Principles
- Ch 5: Implementation Security
- Ch 6: Verification, Testing & Fuzzing
- Ch 7: Release, CVE Management & Incident Response

### PART II — LINUX KERNEL SECURITY ARCHITECTURE
- Ch 8: Kernel Architecture & Trust Boundaries
- Ch 9: Memory Safety — KASAN, KFENCE, KMSAN, UBSAN, KCSAN
- Ch 10: Control Flow Integrity & Stack Protection
- Ch 11: Hardware Mitigations — SMEP, SMAP, KASLR, CET, UMIP
- Ch 12: Linux Security Modules (LSM) Framework & SELinux
- Ch 13: Capabilities & Privilege Decomposition
- Ch 14: Namespaces, Seccomp-BPF & Cgroups v2
- Ch 15: Kernel Fuzzing with Syzkaller
- Ch 16: Static Analysis — Sparse, Smatch, Coccinelle
- Ch 17: Kernel Patch Review & CVE Embargo Process

### PART III — LINUX NETWORK SUBSYSTEM
- Ch 18: Network Stack Architecture — Full Packet Flow
- Ch 19: sk_buff — The Core Data Structure
- Ch 20: Socket Layer & Protocol Families
- Ch 21: TCP/IP Stack — ip_rcv to tcp_v4_rcv
- Ch 22: Netfilter — Hooks, Tables, Chains, Conntrack
- Ch 23: nftables vs iptables Architecture
- Ch 24: eBPF Subsystem — Verifier, Maps, Programs, BTF/CO-RE
- Ch 25: XDP — eXpress Data Path
- Ch 26: Traffic Control — qdisc, Filters, Actions
- Ch 27: Network Namespaces & Virtual Interfaces
- Ch 28: Tunneling — VXLAN, Geneve, GRE, IPIP
- Ch 29: WireGuard Internals & IPsec / xfrm
- Ch 30: NIC Offloading, RSS/RPS, DPDK

### PART IV — CLOUD NETWORK SECURITY
- Ch 31: Cloud Networking Fundamentals
- Ch 32: VPC Architecture & Security Design
- Ch 33: Zero Trust Network Architecture (ZTNA)
- Ch 34: Service Mesh — Istio, Envoy, xDS Protocol
- Ch 35: mTLS, SPIFFE & SPIRE
- Ch 36: Kubernetes Networking — kube-proxy, Services, DNS
- Ch 37: CNI Plugin Architecture
- Ch 38: Cilium — eBPF-Native Cloud Networking
- Ch 39: Network Policy & Microsegmentation
- Ch 40: Security Observability — Falco, Tetragon, Hubble

### PART V — CODE IMPLEMENTATIONS
- Ch 41: C — Netfilter Hook Security Module
- Ch 42: C — eBPF XDP Firewall Program
- Ch 43: C — BPF LSM Security Hooks
- Ch 44: C — sk_buff Inspection & Raw Socket
- Ch 45: Rust — Linux Kernel Module (rust-for-linux)
- Ch 46: Rust — eBPF Programs with Aya Framework
- Ch 47: Rust — Async Network Security Server (Tokio)
- Ch 48: Go — CNI Plugin Implementation
- Ch 49: Go — Network Policy Controller
- Ch 50: Go — mTLS Service & eBPF Loader

---

## PART I — SECURE SOFTWARE DEVELOPMENT LIFECYCLE (SSDLC)

### Chapter 1: SSDLC Foundations & Mental Models

#### What Is SSDLC and Why Does It Matter?

SSDLC (Secure Software Development Lifecycle) is the deliberate integration of security activities into every phase of software development. The foundational axiom: **security is an emergent property of the system design, not a feature bolted on after the fact**.

The Linux kernel, the network stack, and cloud infrastructure are all software. Each is a potential attack surface. Historically, the most catastrophic security failures — Spectre/Meltdown, Heartbleed, the SolarWinds supply chain breach, Dirty COW — all resulted from decisions made during design or implementation that were never reconsidered through a security lens.

**Mental Model: Security Debt**

Every security decision deferred creates debt. Unlike performance debt (you can add caching later), security debt compounds — a weak architectural foundation enables entire classes of exploits that cannot be patched individually. Building a firewall on top of an insecure network stack is whack-a-mole.

```
Cost to Fix a Security Defect by Discovery Phase
=================================================

  100x |                                         [Prod / Exploit]
       |                                         *
   60x |                                  [Test] *
       |                                  *
   15x |                     [Impl]       *
       |                     *
    5x |          [Design]   *
       |          *
    1x | [Req]   *
       *------------------------------------------
          Req   Design   Impl   Test   Prod
```

The 100x factor is conservative. When Heartbleed was found, the fix was two lines of code, but the operational cost — certificate revocation, replacement, re-issuance across the internet — was billions of dollars. The design mistake (no bounds check on heartbeat payload length) was trivially fixable in design phase.

#### The Root Causes of Security Bugs

Every security vulnerability traces back to one or more of:

1. **Incorrect trust assumption** — treating attacker-controlled input as safe (classic: not calling `copy_from_user()` before dereferencing a pointer passed from userspace)
2. **Incorrect state transition** — allowing the system to enter an illegal state (classic: use-after-free — freeing memory then accessing it)
3. **Resource mismanagement** — mishandling memory, file descriptors, capabilities (classic: integer overflow in allocation size → heap overflow)
4. **Incorrect privilege model** — more privilege than needed (classic: running a service as root when it only needs `CAP_NET_BIND_SERVICE`)
5. **Broken cryptography** — weak algorithms, key mismanagement, incorrect usage (classic: AES-ECB for structured data)

When you read a CVE, your first question should be: which of these root causes is this? That tells you what design decision failed and how to think about preventing similar bugs.

#### SSDLC Phase Architecture

```
+-----------------------------------------------------------------------------+
|                        SSDLC Phase Model                                    |
|                                                                             |
| [Training] -> [Requirements] -> [Design] -> [Implementation] -> [Verify]    |
|     |              |              |               |                 |       |
|   SDL               Abuse        Threat          Banned           SAST      |
|   training          cases        model           functions        DAST      |
|   Tool              Security     Attack          Input            Fuzz      |
|   training          reqs         surface         validation       PenTest   |
|   Metrics           Privacy      Crypto design   Crypto impl      Review    |
|                     review       Min privilege   Code review                |
|                                                                             |
| -> [Release] -> [Response]                                                  |
|       |              |                                                      |
|     Code signing   Incident plan                                            |
|     Repro. builds  CVE process                                              |
|     Archive        Patch release                                            |
|     SBOM           Post-mortem                                              |
|                                                                             |
| Continuous: Security training, threat intel, tool updates, metrics review   |
+-----------------------------------------------------------------------------+
```

#### Security First Principles (The Mental Foundation)

Internalize these — every system design decision maps to one or more:

1. **Defense in Depth**: Layer multiple independent controls. No single control is sufficient. The kernel uses: KASLR + SMEP + SMAP + stack canaries + CFI + KASAN. Cloud uses: WAF + Security Groups + NACLs + network policy + pod security. Failure of one layer does not compromise security.

2. **Least Privilege**: Every entity gets exactly the permissions needed — no more. Linux capabilities decompose root into ~40 specific permissions. Kubernetes RBAC grants specific API access. eBPF programs run with minimal kernel context.

3. **Fail Safely (Secure Defaults)**: On error or ambiguity, default to deny. Netfilter's `POLICY DROP` is the secure default. Seccomp default action `SCMP_ACT_KILL` kills on unknown syscall. Zero Trust policy default is deny-all.

4. **Attack Surface Reduction**: Every feature, open port, enabled syscall, and running daemon is attack surface. Remove what is not needed. WireGuard has a deliberately minimal attack surface (~4,000 lines vs ~400,000 in OpenVPN). Seccomp removes 400+ syscalls from container attack surface.

5. **Separation of Duties**: No single entity holds unconstrained authority. Namespaces separate privilege. SELinux types isolate processes. Kubernetes RBAC separates admin, developer, and operator roles.

6. **Economy of Mechanism (Keep It Simple)**: Complexity is the enemy of security. Simple is reviewable. Simple is testable. The WireGuard protocol uses only 5 primitives: Curve25519, ChaCha20, Poly1305, BLAKE2s, SipHash.

7. **Complete Mediation**: Every access to every resource must be checked. LSM hooks implement this — every kernel object access (file, socket, process, IPC) passes through security checks. No exceptions.

8. **Open Design**: Security must not depend on secrecy of mechanism — only secrecy of keys. The Linux kernel is open source. Its security does not depend on obscurity; it depends on correctness.

9. **Psychological Acceptability**: Controls too complex to use will be bypassed. eBPF programs are safe by construction (verifier enforces it) rather than requiring manual security audits — this makes eBPF usable in production by default.

---

### Chapter 2: Threat Modeling — STRIDE, PASTA, Attack Trees

Threat modeling is the systematic process of identifying and prioritizing potential threats before they materialize as vulnerabilities. The governing question: **"What can go wrong, and how bad would it be?"**

The mental model: adopt the adversary's perspective. Attackers find the weakest link. They chain multiple small issues into exploits. They target humans as readily as software. They have time you do not expect.

#### STRIDE Threat Classification

STRIDE is an acronym defining six orthogonal threat categories. Every threat falls into one or more:

```
+------------------------------------------------------------------------+
|                         STRIDE Framework                               |
|                                                                        |
| Category           | Violates    | Example in Network/Kernel Context   |
| -------------------+-------------+------------------------------------ |
| S - Spoofing       | Auth        | IP source spoofing (raw socket),    |
|                    |             | forged ICMP redirect, ARP poisoning |
|                    |             |                                     |
| T - Tampering      | Integrity   | sk_buff payload modification,       |
|                    |             | BGP route injection, DNS poisoning  |
|                    |             |                                     |
| R - Repudiation    | Non-repud.  | Spoofed source makes log attribution|
|                    |             | impossible, kernel log manipulation |
|                    |             |                                     |
| I - Info Disclose  | Confid.     | Kernel memory leak (KMSAN bug),     |
|                    |             | Spectre/Meltdown, TCP timestamps    |
|                    |             |                                     |
| D - Denial of Svc  | Avail.      | SYN flood, eBPF map exhaustion,     |
|                    |             | conntrack table overflow, OOM       |
|                    |             |                                     |
| E - Elevation of   | Authz       | Kernel exploit (local->ring0),      |
|     Privilege      |             | container escape, namespace escape  |
+------------------------------------------------------------------------+
```

**STRIDE Mitigations:**

| Threat | Primary Mitigations |
|--------|---------------------|
| Spoofing | mTLS, certificate pinning, rp_filter (reverse path filtering), BCP38 |
| Tampering | HMAC/MACs on packets, IPsec ESP, immutable audit logs, code signing |
| Repudiation | Signed non-repudiable logs, WORM storage, kernel audit subsystem |
| Info Disclosure | Encryption (TLS/IPsec/WireGuard), access control, side-channel mitigations (Retpoline) |
| DoS | SYN cookies, tc rate limiting, cgroups, connection limits, circuit breakers |
| EoP | Least privilege, KASAN/CFI, SMEP/SMAP, seccomp, LSM |

#### STRIDE Applied to the Linux Network Stack

This is the operational mental model — mapping STRIDE threats to specific kernel components:

```
Network Stack STRIDE Threat Map
================================

  Socket Layer:
    [S] Processes can forge source IPs via CAP_NET_RAW + raw sockets
        -> Mitigation: Require CAP_NET_RAW, network namespace isolation
    [E] Buffer overflow in socket option parsing -> kernel privilege
        -> Mitigation: KASAN, careful bounds checking, fuzzing

  IP Layer:
    [S] IP source spoofing (saddr is user-controlled)
        -> Mitigation: rp_filter=1 (strict reverse path check)
    [D] IP fragmentation attack (fragment table exhaustion)
        -> Mitigation: nf_conntrack_frag6_timeout, IPFRAG_HIGH_THRESH

  TCP Layer:
    [I] TCP sequence number prediction (historical) -> session hijacking
        -> Mitigation: Cryptographic ISN generation (since Linux 2.6)
    [D] SYN flood -> SYN backlog exhaustion
        -> Mitigation: SYN cookies (tcp_syncookies=1)
    [T] TCP PAWS (protection against wrapped sequence) bypass
        -> Mitigation: Timestamps + PAWS check

  Netfilter:
    [T] Conntrack bypass via crafted packets
        -> Mitigation: ct_invalid packet counting, careful parser review
    [E] Netfilter OOB write (CVE-2022-1015) -> root
        -> Mitigation: KASAN, extensive fuzzing, careful index validation
```

#### PASTA — Process for Attack Simulation and Threat Analysis

PASTA is a risk-centric threat modeling methodology suited for complex systems. Its 7-stage structure maps better to cloud/distributed architectures than STRIDE alone:

```
PASTA 7 Stages
==============

Stage 1: Define Business Objectives
         What assets matter? What are the regulatory requirements?
         What is the business impact of a breach?
         -> "Customer payment data must never be exposed"
         -> "Availability SLA is 99.99%"

Stage 2: Define Technical Scope
         System architecture, data flows, trust boundaries, asset inventory
         -> Draw DFDs (Data Flow Diagrams) for each service
         -> Identify all external interfaces

Stage 3: Application Decomposition
         Entry points, data stores, trust levels, privilege levels
         -> "External users hit ALB on port 443"
         -> "Payment service accesses PostgreSQL on port 5432"

Stage 4: Threat Analysis
         Map MITRE ATT&CK techniques to architecture
         -> "TA0001: Initial Access via public-facing app (T1190)"
         -> "TA0004: Privilege Escalation via kernel exploit (T1068)"

Stage 5: Vulnerability Analysis
         Known CVEs, misconfigurations, design weaknesses
         -> "Payment service runs as root in container - EoP risk"
         -> "CVE-2022-0185 in kernel version in use"

Stage 6: Attack Modeling
         Build attack trees, model kill chains
         -> "Exploit CVE -> escape container -> pivot to DB -> exfil"

Stage 7: Risk & Impact Analysis
         Quantify risk, prioritize remediation
         -> CVSS 9.8 * (customer data impact = $10M) = prioritize immediately
```

#### Attack Trees — Formal Threat Decomposition

An attack tree decomposes an attacker's goal into a hierarchy of sub-goals. Root is the objective; leaves are primitive attacks with assigned cost and probability.

```
Goal: Exfiltrate Kubernetes Pod Secret Data
====================================================

                    [Exfiltrate Secret]
                           |
        +------------------+------------------+
        |                  |                  |
  [API Server         [etcd Direct       [Container
   Compromise]         Access]            Escape]
        |                  |                  |
   +----+----+         +---+---+          +---+---+
   |         |         |       |          |       |
[Steal   [Exploit   [Network [Unencrypted [Kernel [Mounted
 SvcAcct  RBAC bug]  Access]  etcd]       Exploit] Secret]
 Token]                                     |
                                       [CVE in  ]
                                       [runtime ]
```

Formal notation:
- **AND gate**: All sub-attacks must succeed (attacker must achieve all children)
- **OR gate**: Any sub-attack suffices (attacker needs only one path)

The attack tree's value: it makes the defender's problem concrete. Defending each leaf with independent controls means the attacker must defeat all paths simultaneously — exponentially harder.

#### CVSS v3.1 Scoring — Quantifying Vulnerability Severity

CVSS provides a standardized score (0.0-10.0) for each vulnerability:

```
CVSS v3.1 Base Score Metrics
==============================

Exploitability Metrics:
  Attack Vector (AV):
    N = Network  (exploitable remotely)      -> worst
    A = Adjacent (same LAN/VLAN required)
    L = Local    (needs local shell)
    P = Physical (needs physical access)     -> best
  
  Attack Complexity (AC):
    L = Low  (no special conditions)
    H = High (race condition, specific config)
  
  Privileges Required (PR):
    N = None
    L = Low  (normal user account)
    H = High (admin account)
  
  User Interaction (UI):
    N = None
    R = Required (victim must do something)
  
  Scope (S):
    U = Unchanged (exploit stays within component)
    C = Changed   (exploit affects other components)

Impact Metrics:
  Confidentiality Impact (C): N / L / H
  Integrity Impact (I):       N / L / H
  Availability Impact (A):    N / L / H

Example: Kernel network exploit (no auth, remote, full impact):
  AV:N / AC:L / PR:N / UI:N / S:C / C:H / I:H / A:H
  Base Score: 10.0 (Critical)

Example: Local kernel privilege escalation:
  AV:L / AC:L / PR:L / UI:N / S:C / C:H / I:H / A:H
  Base Score: 8.8 (High)

Example: Information disclosure via timing side channel:
  AV:N / AC:H / PR:N / UI:N / S:U / C:H / I:N / A:N
  Base Score: 5.9 (Medium)
```

---

### Chapter 3: Security Requirements Engineering

Security requirements define *what the system must do* (or must not do) to be considered secure. Writing good requirements forces clarity about security goals before implementation.

#### Types of Security Requirements

**Functional Security Requirements** — active behaviors the system must exhibit:
- "The packet filter must drop all TCP SYN packets from IPs in the blocklist before they reach the transport layer."
- "All management API calls must be authenticated with mTLS using certificates from the internal CA."
- "The audit log must record every network connection attempt with: timestamp, source IP, destination IP, port, protocol, allow/deny decision."
- "Network policies must be enforced within 500ms of policy change propagation."

**Non-Functional Security Requirements** — quality attributes:
- "Authentication must complete within 50ms at the 99th percentile under 10,000 concurrent requests."
- "The kernel module must not increase packet processing latency by more than 2 microseconds per packet."
- "Cryptographic keys must be rotated every 24 hours with zero-downtime rotation."
- "The firewall must sustain 1M pps (packets per second) without packet loss."

#### Abuse Cases — The Attacker's Use Cases

Abuse cases are the dual of use cases. For every feature, ask: how can an attacker misuse this?

```
+-------------------------------------------------------------------------+
| Use Case                | Abuse Case              | Countermeasure      |
|-------------------------+-------------------------+---------------------|
| User opens TCP conn     | SYN flood               | SYN cookies, rate   |
|                         |                         | limiting            |
|-------------------------+-------------------------+---------------------|
| Service reads config    | Config injection        | Input validation,   |
|                         |                         | schema enforcement  |
|-------------------------+-------------------------+---------------------|
| Admin deploys container | Privileged container    | Pod Security Adm.,  |
|                         | escape                  | seccomp, AppArmor   |
|-------------------------+-------------------------+---------------------|
| Process makes syscall   | Exploit banned syscall  | Seccomp-BPF filter  |
|-------------------------+-------------------------+---------------------|
| App connects to DB      | SSRF to internal APIs   | Network policy,     |
|                         |                         | egress filtering    |
|-------------------------+-------------------------+---------------------|
| Operator loads kernel   | Malicious kernel module | Module signing,     |
| module                  |                         | lockdown mode       |
+-------------------------------------------------------------------------+
```

#### Trust Boundaries — Where Validation Happens

A trust boundary is any point where data crosses between zones of different trust. Every boundary must be explicitly designed with authentication, authorization, validation, and audit:

```
Trust Boundary Stack for a Cloud-Native Application
====================================================

  [Internet - Untrusted]
        |
        | VALIDATE: TLS termination, DDoS protection, WAF rules
        v
  [Load Balancer / CDN - Partially Trusted]
        |
        | VALIDATE: JWT/OAuth token verification, rate limiting
        v
  [API Gateway / Ingress - Internal Trusted]
        |
        | VALIDATE: mTLS (service-to-service), RBAC authorization
        v
  [Service Mesh (encrypted) - Internal Trusted]
        |
        | VALIDATE: Network policy (L3/L4), L7 policy (gRPC method)
        v
  [Pod / Container - Process Trusted]
        |
        | VALIDATE: Syscall filtering (seccomp), capability checks
        v
  [Kernel Subsystem - TCB]
        |
        | VALIDATE: SMEP/SMAP, capabilities, LSM hooks, namespaces
        v
  [Hardware]
```

Each vertical arrow is a trust boundary. Each must independently validate that the data/request is authorized. Defense in depth means that compromise at one boundary does not automatically compromise all below it.

---

### Chapter 4: Secure Design Principles (Applied)

#### The CIA Triad as Design Compass

Every security control protects one or more of three fundamental properties:

```
              Confidentiality
              (data not revealed
               to unauthorized)
                     ^
                     |
                     |
   Integrity <-------+-------> Availability
   (data not        CIA       (system accessible
    modified        Triad      to authorized users)
    without auth)
```

In networking:
- IPsec **ESP** provides Confidentiality (encryption) + Integrity (HMAC)
- IPsec **AH** provides Integrity only (no encryption)
- TCP SYN cookies protect **Availability** under SYN flood
- TLS provides Confidentiality + Integrity + Authentication (a bonus property)
- WireGuard provides all three simultaneously

#### Crypto-Agility vs. Simplicity Tension

A critical design tension: cryptographic agility (supporting multiple algorithms) vs. simplicity (fewer algorithms = fewer bugs).

WireGuard chose **opinionated cryptography**: one algorithm set, no negotiation, no agility:
- Key exchange: Curve25519 (always)
- Encryption: ChaCha20-Poly1305 (always)
- Hash: BLAKE2s (always)
- MAC: SipHash24 (always)

This eliminates: downgrade attacks, algorithm negotiation exploits, implementation of weak algorithms. The tradeoff: no post-quantum readiness (being addressed in WireGuard successors).

TLS chose **agility**: negotiable cipher suites. This introduced: BEAST, POODLE, DROWN, Logjam, FREAK — all exploiting the negotiation/downgrade attack surface.

**Design principle**: Default to opinionated, minimal crypto. Add agility only when legally or interoperability required.

---

### Chapter 5: Implementation Security

#### Banned and Dangerous Functions (C Kernel Context)

In kernel C code, certain functions are categorically dangerous:

```c
// BANNED: Buffer overflow risk
// strcpy(dest, src) - no length limit
// -> USE: strscpy(dest, src, size) or strncpy + explicit null-term

// BANNED: No length limit
// strcat(dest, src)
// -> USE: strlcat(dest, src, size)

// BANNED: Unbounded format string
// sprintf(buf, fmt, ...)
// -> USE: snprintf(buf, size, fmt, ...) or scnprintf()

// CRITICAL: NEVER dereference user pointers directly
// This is the #1 source of kernel security bugs:
void bad_syscall_handler(char __user *user_buf, size_t len)
{
    char *ptr = (char *)user_buf;  // WRONG: losing __user annotation
    *ptr = 0;                       // EXPLOITABLE: direct user-space write
}

void good_syscall_handler(char __user *user_buf, size_t len)
{
    char kbuf[PAGE_SIZE];
    if (len > sizeof(kbuf))
        return -EINVAL;
    if (copy_from_user(kbuf, user_buf, len))  // CORRECT
        return -EFAULT;
    // kbuf is now safe to use in kernel context
}

// DANGEROUS: Integer overflow in size calculation
// struct = kmalloc(count * sizeof(element), GFP_KERNEL);
// If count is large, count * sizeof() overflows -> tiny allocation -> overflow
// -> USE: kmalloc_array(count, sizeof(element), GFP_KERNEL);
// kmalloc_array() does overflow-safe multiplication internally
```

#### Memory Management in Kernel C

```
Kernel Allocator Decision Tree
================================

Need kernel memory?
    |
    +-- Small objects (<= PAGE_SIZE, typically):
    |       kmalloc(size, GFP_KERNEL)      // May sleep, normal context
    |       kmalloc(size, GFP_ATOMIC)      // Cannot sleep, interrupt context
    |       kmalloc(size, GFP_NOWAIT)      // Cannot sleep, fail fast
    |       kzalloc(size, flags)           // kmalloc + memset(0) - prefer for security
    |
    +-- Fixed-size objects (many allocations of same size):
    |       struct kmem_cache *cache = kmem_cache_create(...)
    |       kmem_cache_alloc(cache, flags)
    |       kmem_cache_free(cache, obj)
    |
    +-- Large allocations (> PAGE_SIZE):
    |       vmalloc(size)                  // Virtually contiguous (may not be physical)
    |       __get_free_pages(flags, order) // Physically contiguous 2^order pages
    |
    +-- Per-CPU data:
            alloc_percpu(type)             // Separate copy per CPU, no locking needed
            __get_cpu_var(var)             // Access (deprecated, use this_cpu_ptr)
            this_cpu_ptr(&var)             // Access current CPU's copy

CRITICAL RULES:
  - Always check return value (NULL = allocation failed)
  - Match allocator with deallocator: kmalloc/kfree, vmalloc/vfree, pages/free_pages
  - Set memory to zero for security: kzalloc() not kmalloc()
    (uninitialized memory can contain sensitive data from previous use)
  - Use RCU for lock-free read-heavy data structures
```

#### RCU — Read-Copy-Update (Critical for Kernel Network Code)

RCU is a synchronization mechanism used extensively in the network stack (routing tables, netfilter rules, conntrack). Mental model:

```
RCU Model
==========

Readers (fast, no lock):
  rcu_read_lock()
  ptr = rcu_dereference(global_ptr)  // Barrier: ensures ptr is loaded safely
  use(ptr)
  rcu_read_unlock()

Writers (atomic pointer swap):
  new_obj = kmalloc(...)
  // ... initialize new_obj ...
  old_ptr = rcu_dereference(global_ptr)
  rcu_assign_pointer(global_ptr, new_obj)  // Atomic store with barrier
  synchronize_rcu()  // Wait for all current readers to finish
  kfree(old_ptr)     // Now safe to free - no readers can see it

The guarantee:
  - Readers never see a torn pointer (atomic load)
  - Readers may see either old or new value (eventual consistency)
  - Old value is not freed until ALL readers that might have seen it are done
  - MUCH faster than mutex for read-heavy workloads (routing table lookups)
```

---

### Chapter 6: Verification, Testing & Fuzzing

#### Static Analysis Tools

Static analysis examines source code without executing it, finding bugs at review time:

```
Tool         | Language   | Specialization
-------------|------------|------------------------------------------
sparse       | C (kernel) | __user/__kernel/__iomem type annotations,
             |            | endianness (__be32 vs __le32 vs u32),
             |            | lock annotations, context annotations
             |            |
smatch       | C (kernel) | UAF patterns, null deref, integer overflow,
             |            | lock imbalance, uninitialized variable,
             |            | return value not checked
             |            |
coccinelle   | C (kernel) | Semantic patches, API migrations, bug
             |            | pattern matching across entire codebase
             |            |
clang-tidy   | C/C++      | Modern API usage, performance, portability
             |            |
clippy       | Rust       | Idiomatic Rust, common mistakes, performance
             |            |
cargo-audit  | Rust       | Known CVEs in crate dependencies (RustSec DB)
             |            |
semgrep      | Multi      | Custom security rules, taint analysis
             |            |
CodeQL       | Multi      | Database of program facts, query-based
```

#### Dynamic Analysis in the Kernel

Dynamic analysis instruments running code:

**KASAN — Kernel Address Sanitizer:**
Uses shadow memory to detect heap/stack overflows and use-after-free. Every 8 bytes of kernel memory has 1 shadow byte tracking accessibility.

```
Shadow memory encoding:
  0x00 = all 8 bytes accessible
  0x01 = only first 1 byte accessible (rest is padding/redzone)
  ...
  0x07 = only first 7 bytes accessible
  0xFx = poisoned (freed, redzone, out of bounds)

On every load/store:
  1. Compute shadow_addr = (access_addr >> 3) + KASAN_SHADOW_OFFSET
  2. Load shadow_byte = *shadow_addr
  3. If shadow_byte != 0: check if access is in valid range
  4. If invalid: report BUG_ON with full stack trace + allocation/free history
```

Overhead: ~2x memory, 10-20% CPU. Suitable for CI, not production.

**KFENCE — Kernel Electric Fence:**
Production-safe probabilistic memory safety detector. Samples ~1/100,000 allocations:

```
Normal SLUB:  [obj1][obj2][obj3][obj4][obj5]...
KFENCE:       [guard page | READONLY][sampled_obj][guard page | READONLY]

Access past sampled_obj -> page fault on guard page -> BUG() with report
```

Overhead: < 1%. Safe for production kernels.

**KMSAN — Kernel Memory Sanitizer:**
Detects uninitialized memory reads. Catches information disclosure bugs where kernel stack data leaks to userspace (defeating KASLR).

**UBSAN — Undefined Behavior Sanitizer:**
Catches signed integer overflow, shift-by-negative, misaligned access, OOB on static arrays.

**KCSAN — Kernel Concurrency Sanitizer:**
Detects data races using watchpoints. Critical for finding race-condition-based UAFs in network code.

#### Fuzzing — Automated Vulnerability Discovery

Fuzzing generates random inputs to trigger crashes. Three generations:

```
Generation 1: Dumb/random fuzzing
  -> Generate random bytes, send to target
  -> Low coverage, catches obvious parsing bugs

Generation 2: Mutation-based (AFL, libFuzzer)
  -> Start with valid input corpus
  -> Instrument binary for code coverage (edges)
  -> Mutate inputs, track which increase coverage
  -> Prioritize high-coverage mutations
  -> Feedback loop: coverage guides mutation strategy

Generation 3: Structure-aware (Syzkaller, grammar fuzzers)
  -> Know the format of input (syscall grammar, packet format)
  -> Generate structurally valid but semantically invalid inputs
  -> Can explore deep kernel paths unreachable by random bytes
```

**Syzkaller for Network Fuzzing:**

Syzkaller describes syscall interfaces in a domain-specific language:

```
# Syzkaller description for TCP socket operations
socket$inet_tcp(domain const[AF_INET], type const[SOCK_STREAM],
                proto const[IPPROTO_TCP]) fd_tcp
setsockopt$tcp_int(fd fd_tcp, level const[IPPROTO_TCP], optname flags[tcp_option_ints],
                   optval ptr[in, int32], optlen len[optval])
connect$inet(fd fd_tcp, addr ptr[in, sockaddr_in], addrlen len[addr])
sendmsg$inet(fd fd_tcp, msg ptr[in, msghdr], flags flags[send_flags])
```

Syzkaller generates random programs using this grammar, executes them in a VM with KCOV coverage tracking, and saves programs that trigger new kernel code paths. When a crash occurs, Syzkaller automatically minimizes the reproducer.

#### Penetration Testing (Network/Kernel Focus)

Structured adversarial testing phases:

```
Recon -> Scan -> Exploit -> Post-Exploit -> Report -> Remediate

Kernel/Container-specific techniques:
  - Container escape via privileged container mounts
  - Kernel exploit via UAF or heap overflow in network code
  - Namespace escape via user namespace + capability abuse
  - Side-channel via timing (Spectre gadgets in kernel)
  - Supply chain via compromised container image/dependency

Cloud-specific techniques:
  - SSRF to IMDSv1 for credential theft (AWS metadata endpoint)
  - Unrestricted egress to exfiltrate data
  - Privilege escalation via overly permissive IAM roles
  - BGP route hijacking for network-level interception
```

---

### Chapter 7: Release, CVE Management & Incident Response

#### Code Signing & Supply Chain

Every binary artifact leaving the build system must be signed:

```
Build Supply Chain Security
============================

  Source Code
      |
      | [Reproducible Build: BUILD_ID hashed]
      v
  Compiled Binary
      |
      | [Sign with HSM key: ed25519/RSA-4096]
      v
  Signed Artifact + Signature
      |
      | [Publish to artifact registry with cosign/sigstore]
      v
  Distribution / Deployment
      |
      | [Verify signature before execution]
      v
  Runtime

Linux kernel specifics:
  - Module signing: CONFIG_MODULE_SIG=y, modules signed with kernel build key
  - Secure Boot: kernel image signed by distribution key, UEFI verifies
  - Lockdown mode: prevents loading unsigned modules, disables /dev/mem
  - IMA (Integrity Measurement Architecture): measures files, enforces policy
```

**SBOM (Software Bill of Materials):** Enumerate every dependency. When a vulnerability is found in a library, the SBOM tells you exactly which deployed systems are affected.

#### CVE Process for Linux Kernel Security Vulnerabilities

```
  Day  0: Researcher discovers vulnerability
          |
          v
  Day  0: Report to security@kernel.org (encrypted, embargoed)
          Include: description, reproducer, affected versions, CVSS estimate
          |
          v
  Day  1: Security team triages
          - Assign severity: Critical/High/Medium/Low
          - Contact subsystem maintainer under embargo
          - Request CVE from MITRE (linux-distros CNA)
          |
          v
  Day 1-13: Private development
             - Fix authored in private branch
             - Internal review by kernel security team
             - Testing in private VM farm
          |
          v
  Day 14: Coordinated Disclosure
          - Patch submitted to netdev/linux-kernel mailing list (public)
          - linux-distros@vs.openwall.org notified (distros can pre-patch)
          - Stable backports submitted: stable@vger.kernel.org
          - CVE published on linux-cve-announce@vger.kernel.org
          - NVD updated
          |
          v
  Day 14-30: Upstream + Distros ship kernel updates
             Users update

Embargo duration guidelines:
  - Critical (remote RCE, no auth): <=7 days
  - High (local privilege escalation): <=14 days
  - Medium: public from start preferred
  - Kernel team strongly prefers short embargoes
```

#### Live Kernel Patching (kpatch/livepatch)

For critical CVEs, rebooting every server to apply a kernel patch is expensive. Live patching applies the fix without a reboot:

```
livepatch architecture:
  1. Build patch module from before/after kernel diff
  2. ftrace hooks the old function: old_fn -> trampoline
  3. Trampoline calls new_fn (from patch module)
  4. Consistency model: all tasks transitioned to new function atomically
  5. Patch can be reverted by removing the module

Tools:
  - upstream kernel livepatch framework (CONFIG_LIVEPATCH)
  - kpatch (Red Hat)
  - kGraft (SUSE)
  - Ksplice (Oracle)
  - Cloud vendor services: AWS Kernel Live Patching, Azure Hotpatch
```

---

## PART II — LINUX KERNEL SECURITY ARCHITECTURE

### Chapter 8: Kernel Architecture & Trust Boundaries

#### The Kernel as Trusted Computing Base (TCB)

The Trusted Computing Base is the set of all hardware, firmware, and software that must be trusted for security guarantees to hold. In Linux:

- Everything running at ring 0 (kernel mode) is TCB
- A single kernel bug can compromise all processes on the machine
- The kernel enforces all security policies for all processes
- Kernel code has direct access to all physical memory and all hardware

This is why kernel security receives extraordinary scrutiny. A bug in an obscure TCP option parser (like the SACK panic bug CVE-2019-11477) can bring down any Linux system in the world.

```
Linux Kernel Trust Hierarchy
==============================

  [Hardware]
      |
      | [CPU enforces: ring levels, SMEP, SMAP, NX bit]
      | [MMU enforces: page table permissions]
      | [IOMMU enforces: DMA constraints]
      v
  [Kernel Space - Ring 0 - Full Trust]
      |
      | [Kernel enforces: all security policies]
      | [LSM enforces: mandatory access control]
      | [Capabilities enforce: privilege decomposition]
      | [Namespaces enforce: isolation]
      v
  [User Space - Ring 3 - Conditional Trust]
      |
      | [Trust depends on: UID, GID, capabilities, SELinux context]
      | [User processes trust the kernel absolutely]
      | [The kernel trusts user processes NOT AT ALL]
      v
  [Applications]
```

#### Kernel Architecture Overview

```
Full Linux Kernel Architecture (Security View)
================================================

+--------------------------------------------------------------+
|                    User Space                                |
|  Applications  Services  Containers  VMs                     |
+-------------------------------+------------------------------+
                                | System Call Interface
                                | (validates all arguments,
                                |  enforces capability checks,
                                |  runs LSM hooks before ops)
+-------------------------------+------------------------------+
|                 Kernel Space - Ring 0                        |
|                                                              |
|  [Security Subsystem]                                        |
|  LSM (SELinux/AppArmor)  Capabilities  Namespaces            |
|  Audit  Seccomp-BPF  Lockdown  Integrity (IMA/EVM)           |
|                                                              |
|  [Core Subsystems]                                           |
|  Process Mgmt   Memory Mgmt    VFS (Filesystems)             |
|  (scheduler,    (buddy alloc,  (ext4, btrfs, xfs,            |
|   signals,      SLUB/SLAB,     overlayfs, procfs,            |
|   namespaces)   page tables,   sysfs, tmpfs)                 |
|                 KASLR, ASLR)                                 |
|                                                              |
|  [Network Subsystem] <-- Our Deep Dive Territory             |
|  Socket Layer    TCP/UDP/SCTP    IP Layer (routing)          |
|  Netfilter       eBPF            XDP                         |
|  Traffic Control WireGuard       IPsec (xfrm)                |
|  Net namespaces  Bridges/VLANs   Tunnels (VXLAN/GRE)         |
|                                                              |
|  [Hardware Abstraction / Drivers]                            |
|  NIC drivers  Block drivers  PCIe  USB  IOMMU                |
|                                                              |
+--------------------------------------------------------------+
|               Hardware                                       |
|  CPUs (rings, NX, SMEP, SMAP, CET)  MMU  IOMMU  NIC  DMA     |
+--------------------------------------------------------------+
```

---

### Chapter 9: Memory Safety — KASAN, KFENCE, KMSAN, UBSAN, KCSAN

Memory bugs are the primary source of kernel exploits. Understanding each sanitizer helps you know what bugs they catch, when to use them, and what code patterns they find.

#### Memory Bug Taxonomy

```
Memory Safety Bug Taxonomy
===========================

  Spatial (access wrong location):
  |-- Buffer Overflow (stack/heap OOB write)
  |-- Out-of-Bounds Read (heap/stack OOB read) -> info disclosure
  |-- Off-By-One (boundary condition)
  |-- Underflow (writing before allocation start)

  Temporal (access at wrong time):
  |-- Use-After-Free (UAF) -> access freed memory -> type confusion
  |-- Use-After-Return (stack memory accessed after function return)
  |-- Use-After-Scope (stack memory from inner scope)
  |-- Double-Free (freeing same pointer twice)

  Initialization:
  |-- Uninitialized Memory Read -> information disclosure (KASLR defeat)
  |-- Uninitialized Pointer Dereference

  Concurrency:
  |-- Data Race (concurrent unsynchronized access)
  |-- TOCTOU (Time-of-Check-Time-of-Use)

  Arithmetic:
  |-- Integer Overflow/Underflow (especially in size calculations)
  |-- Undefined Behavior (signed overflow, negative shift)
```

#### KASAN — Kernel Address Sanitizer Deep Dive

KASAN detects spatial and temporal memory bugs using shadow memory.

**Shadow Memory Layout:**
```
For every 8 bytes of kernel virtual address space,
KASAN maintains 1 byte of shadow memory:

Kernel VA: 0xffff888000000000 ... 0xffff88fffffffffff
Shadow VA: 0xdffffc0000000000 ... (at 1/8 the offset)

Shadow byte meaning:
  0x00 = all 8 bytes valid
  0x01..0x07 = only first N bytes valid (redzone)
  0xFA = heap left redzone
  0xFB = heap right redzone (after object)
  0xFC = heap free (kfree'd memory)
  0xFD = stack left redzone
  0xFE = stack right redzone
  0xFF = global redzone

Inline check on every memory access (compiled by KASAN-aware GCC/Clang):
  // Access to addr (1/2/4/8 bytes):
  shadow_addr = (addr >> 3) + KASAN_SHADOW_OFFSET
  shadow = *shadow_addr
  if (shadow != 0) {
      // For 1-byte access: if (shadow) -> BUG
      // For N-byte access: check if access crosses into redzone
      __kasan_report(addr, size, is_write, return_addr)
  }
```

**KASAN Report Reading:**
```
==================================================================
BUG: KASAN: heap-use-after-free in netif_rx+0x52/0x180
Write of size 4 at addr ffff8880145c3210 by task softirq/1

CPU: 1 PID: 1 Comm: softirq
Call Trace:
 netif_rx+0x52/0x180
 ip_rcv+0x3a0/0x500
 ...

Allocated by task 1233:
 kmalloc+0x23/0x40
 sk_buff_alloc+0x12/0x30     <- where sk_buff was allocated

Freed by task 1233:
 kfree_skb+0x1a/0x30         <- where sk_buff was freed
 dev_kfree_skb+0x15/0x25
 
The buggy address ffff8880145c3210 belongs to the object at ffff8880145c3200
 which belongs to the cache skbuff_head_cache of size 232
==================================================================
```

This report gives you: what happened (UAF write), where it happened (netif_rx), where the memory was allocated (sk_buff_alloc), and where it was freed (kfree_skb). This is everything needed to debug the race condition.

**KASAN Variants:**
```
CONFIG_KASAN_GENERIC=y      # Software instrumentation, works on all arches
                             # 2x memory overhead, ~15% CPU overhead
                             # Best for debugging

CONFIG_KASAN_SW_TAGS=y      # Software tag-based (AArch64)
                             # Uses top byte of pointer for tag
                             # Less overhead than generic
                             # Good for testing

CONFIG_KASAN_HW_TAGS=y      # Hardware tag-based (ARM MTE - AArch64 only)
                             # Hardware checks the tag, nearly zero overhead
                             # Suitable for production!
                             # MTE = Memory Tagging Extension
```

The ARM MTE variant of KASAN is significant: it enables memory safety checking in production workloads with < 1% overhead. Google uses this in Android. Cloud providers use ARM instances.

#### KFENCE — Production Memory Safety

KFENCE (Kernel Electric Fence) is designed specifically for always-on production use:

```
KFENCE Architecture:
=====================

  Sampled allocation (1 in N, configurable):
  
  Normal allocation:  [prev_obj][obj][next_obj] <- compactly packed in SLUB
  
  KFENCE allocation:
  
  +-----------+-----------+-----------+
  | guard page| object    | guard page|
  | (no-map,  | (normal   | (no-map,  |
  |  FAULT on |  access)  |  FAULT on |
  |  access)  |           |  access)  |
  +-----------+-----------+-----------+

  If code writes 1 byte past 'object': page fault on guard page
  -> KFENCE intercepts: reports OOB write with stack trace

  If code reads freed object: KFENCE marks page as poisoned
  -> Any access reports UAF

KFENCE pool: pre-allocated at boot, used for sampled objects
  CONFIG_KFENCE_NUM_OBJECTS=255   (default pool size)
  CONFIG_KFENCE_SAMPLE_INTERVAL=100  (milliseconds between samples)
  
  At 100ms sample rate, roughly 10 objects/second are KFENCE-protected
  -> Very low overhead (~< 1%)
  -> Probabilistic: not every bug detected, but catches bugs over time
```

**Setting KFENCE sample rate at runtime:**
```bash
# More aggressive (find bugs faster in testing):
echo 10 > /sys/module/kfence/parameters/sample_interval

# Disable (zero overhead, but no protection):
echo 0 > /sys/module/kfence/parameters/sample_interval
```

#### KMSAN — Detecting Information Leaks

KMSAN tracks which memory bytes are "initialized" using shadow metadata. Every variable gets a "shadow" tracking whether it has been assigned a value. Reading an uninitialized shadow raises a BUG.

Critical for network security because kernel stack data can leak through:
- Padding in structs copied to userspace (copy_to_user without zeroing)
- Network protocol headers (unused fields not zeroed)
- Socket option return values with stack garbage

Example bug KMSAN catches:
```c
// Vulnerable: struct has implicit padding that is never initialized
struct resp {
    u32 status;    // 4 bytes
    // 4 bytes implicit padding here (alignment)
    u64 value;     // 8 bytes
};  // total 16 bytes, but 4 bytes of padding = uninitialized

struct resp r;
r.status = 0;
r.value = 42;
// r has 4 bytes of uninitialized padding!
if (copy_to_user(user_buf, &r, sizeof(r)))  // KMSAN: reading uninit bytes
    return -EFAULT;
// Fix: use __attribute__((packed)) or memset(&r, 0, sizeof(r)) first
```

#### KCSAN — Data Race Detection

KCSAN uses a watchpoint mechanism to detect concurrent unsynchronized memory accesses:

```
KCSAN Operation:
  1. Randomly select a memory access to "watch"
  2. Record the address and CPU number
  3. Sleep for a configurable period (20ns - 40ns)
  4. If another CPU accesses the same address during sleep:
     -> If at least one access is a WRITE: DATA RACE BUG reported

Why data races matter for security:
  Classic UAF via data race:
    CPU 0: ptr = kmalloc(...)    // allocate
    CPU 0: if (ptr != NULL)      // check
      CPU 1: kfree(ptr)          // free (concurrent!)
    CPU 0:   *ptr = value        // USE-AFTER-FREE!
  
  KCSAN catches this: two CPUs accessing same address, one write
```

---

### Chapter 10: Control Flow Integrity & Stack Protection

#### Stack Buffer Overflow — The Classic Attack

```
Function Stack Layout (x86-64, grows downward):
================================================

  High addresses (top of stack)
  +------------------+
  | Return address   |  <- attacker target: overwrite to redirect execution
  +------------------+
  | Saved RBP        |  <- saved frame pointer
  +------------------+
  | Stack canary     |  <- (with STACKPROTECTOR) random value
  +------------------+
  | Local variables  |
  +------------------+
  | char buf[256]    |  <- vulnerable buffer (written low to high)
  +------------------+
  Low addresses (bottom of stack)

  Attack: write 256+ bytes into buf[] -> overwrite canary -> overwrite RBP -> overwrite RIP
  On function return: CPU jumps to attacker's chosen address (e.g., shellcode or ROP gadget)
```

#### Stack Canaries — First Line of Defense

```c
// What the compiler generates with -fstack-protector-strong:

// Original C:
void vulnerable(char *user_input) {
    char buf[64];
    strcpy(buf, user_input);
}

// Compiled with stack protection (pseudocode):
void vulnerable(char *user_input) {
    uintptr_t canary = __stack_chk_guard;  // gs:[28] on x86-64
    char buf[64];
    strcpy(buf, user_input);
    if (canary != __stack_chk_guard)
        __stack_chk_fail();  // calls kernel panic / abort
    return;
}
```

Kernel config:
```
CONFIG_STACKPROTECTOR=y           # Protect functions with char arrays
CONFIG_STACKPROTECTOR_STRONG=y    # Also protect functions with arrays of any type
                                   # and functions that reference local stack frames
```

The canary value is randomized at boot (`__stack_chk_guard` in gs segment). An overflow overwrites the canary; the comparison at function return detects it and prevents the exploit.

#### KASLR — Kernel Address Space Layout Randomization

Without KASLR, the kernel is always at a predictable virtual address. Attackers can hard-code ROP gadget addresses.

```
Without KASLR:
  Kernel text: always at 0xffffffff81000000
  __sys_execve: always at 0xffffffff811f2a00 (static)
  Attacker's ROP chain: use this known address

With KASLR (CONFIG_RANDOMIZE_BASE=y):
  Kernel text: 0xffffffff81000000 + random_offset
  random_offset: up to 26 bits entropy (AES-based PRNG at boot)
  
  __sys_execve: 0xffffffff811f2a00 + random_offset (attacker can't know)
  
  Attacker needs: info leak to determine offset first
  Then: use the offset to compute all gadget addresses
  
KASLR is defeated by:
  - /proc/kallsyms (readable by root, restricted with kptr_restrict=2)
  - kernel log messages with addresses (%pK prints hidden for non-root)
  - KMSAN-caught info leaks (uninitialized data in syscall returns)
  - Timing side channels (historically Meltdown revealed kernel addresses)
  - Speculative execution attacks (Spectre v3 = Meltdown)
```

#### SMEP — Supervisor Mode Execution Prevention

```
Intel/AMD hardware feature, enabled in CR4 register (bit 20)

Without SMEP (classic user->kernel code execution):
  1. Attacker maps shellcode at user VA 0x400000
  2. Overflows kernel stack, sets return address to 0x400000
  3. CPU returns to ring 0 but executes user-space address
  4. Shellcode runs with full kernel privileges -> root

With SMEP (CR4.SMEP = 1):
  CPU enforces: ring 0 cannot execute pages marked U/S=1 (user pages)
  Attempt to execute 0x400000 from ring 0 -> #PF (page fault) -> kernel panic
  
  Attacker must now:
  - Use Return-Oriented Programming (ROP) with kernel gadgets only
  - Chain together small code snippets already in kernel text
  - Much harder: gadgets must be found, chained correctly

CONFIG_X86_SMEP=y (automatic on supported CPUs, Ivy Bridge+)
```

#### SMAP — Supervisor Mode Access Prevention

```
SMAP (CR4.SMAP = 1):
  Ring 0 cannot READ or WRITE user-space pages (U/S=1) without explicit permission

  Purpose: Prevent TOCTOU attacks and kernel pointer derefs into user space

  Controlled by RFLAGS.AC bit:
    - STAC instruction: sets AC = 1 (allows temporary user access)
    - CLAC instruction: clears AC = 0 (restores protection)

  The ONLY way kernel should touch user memory:
    copy_from_user(dst, src, len):   stac; memcpy; clac  (with fault handling)
    copy_to_user(dst, src, len):     stac; memcpy; clac  (with fault handling)

  Without SMAP:
    1. User maps mmap at address X with content A
    2. Kernel syscall validates content A (it looks good)
    3. User remaps address X to content B (malicious) via mmap
    4. Kernel uses "content A" address X, but gets content B -> TOCTOU exploit

  With SMAP:
    All user-space access goes through copy_from/to_user with explicit fault handling
    Cannot be bypassed by race condition because each access is protected
```

#### CET — Control-flow Enforcement Technology

Intel CET provides hardware enforcement of CFI. Two components:

```
CET Shadow Stack (SS):
  Maintains a SECOND stack for return addresses only
  On CALL: pushed to both regular stack AND shadow stack
  On RET:  compared against shadow stack
  If mismatch: #CP (Control Protection fault)
  
  Prevents: return-address overwrite exploits
  Even if attacker overwrites regular stack's return address,
  shadow stack comparison will catch it

CET Indirect Branch Tracking (IBT):
  ENDBR64 instruction marks valid indirect branch targets
  Any indirect JMP/CALL must land on ENDBR64
  Landing anywhere else: #CP fault
  
  Prevents: ROP and JOP (Jump-Oriented Programming)
  Gadgets not preceded by ENDBR64 cannot be used

Kernel support: CONFIG_X86_SHADOW_STACK=y, CONFIG_X86_IBT=y (Intel Tiger Lake+)
```

---

### Chapter 11: Hardware Mitigations Summary

```
Security Mitigations by Layer
==============================

Hardware Level (CPU):
  SMEP (CR4.SMEP):     Ring 0 cannot execute U/S=1 pages
  SMAP (CR4.SMAP):     Ring 0 cannot access U/S=1 pages (without AC=1)
  NX/XD bit:           Data pages not executable (stack/heap)
  CET (SHSTK + IBT):   Shadow stack + indirect branch tracking
  MTE (ARM):           Hardware memory tagging for KASAN-HW
  
Kernel Level (Software):
  KASLR:               Randomize kernel load address
  Stack canaries:      Detect stack buffer overflows
  RANDSTRUCT:          Randomize struct field layout (compiler plugin)
  LATENT_ENTROPY:      Better entropy at boot
  CFI (KCFI):          Clang-based control flow integrity for indirect calls
  FORTIFY_SOURCE:      Buffer overflow detection in string/memory functions
  PIE (Position Indep): Kernel compiled as PIE for KASLR
  Retpoline:           Spectre v2 mitigation (indirect branch via trampoline)
  IBRS/IBPB:           Spectre v2 hardware mitigation (branch predictor isolation)
  KPTI:                Kernel Page Table Isolation (Meltdown mitigation)

Application Level:
  ASLR (mmap_rnd_bits): Randomize user process layout
  Stack canaries:       User-space stack protection (-fstack-protector)
  PIE executables:      ASLR-compatible executable format
  RELRO:                Read-only after relocation (GOT protection)
  Seccomp:              Restrict syscall surface
```

---

### Chapter 12: Linux Security Modules (LSM) Framework

#### LSM Architecture — The Policy Hook System

LSM places security hooks at every significant kernel operation. Multiple LSM modules can be stacked (stacking introduced in Linux 4.2):

```
LSM Hook Execution Flow
========================

  Syscall: open("/etc/shadow", O_RDONLY)
                 |
                 v
  DAC Check (POSIX permissions):
    - Does process UID match file owner? -> S_IRWXU
    - Does process GID match file group? -> S_IRWXG
    - Other perms? -> S_IRWXO
    If DAC denies: return -EACCES immediately (LSM never called)
                 |
                 v (if DAC allows)
  LSM Hook: security_file_open(file, current_cred())
                 |
                 v
  For each registered LSM (in order):
  +---------------------------+
  | 1. SELinux (if enabled)   |
  |    checks: httpd_t can    |
  |    read shadow_t?         |
  |    Policy says: NO        |
  |    returns -EACCES        |
  +---------------------------+
        |
        v (first failure stops chain)
  Return -EACCES to userspace (EACCES or EPERM depending on context)
  
  If SELinux says yes, check AppArmor, check BPF LSM programs...
  ALL must approve for access to proceed.
```

**Key LSM Hooks (Network-Related):**
```c
// From include/linux/security.h (Linux 6.x)

// Socket creation: before creating any socket
int security_socket_create(int family, int type, int protocol, int kern);

// Bind: before binding to an address/port
int security_socket_bind(struct socket *sock, struct sockaddr *address, int addrlen);

// Connect: before establishing connection
int security_socket_connect(struct socket *sock, struct sockaddr *address, int addrlen);

// Listen: before accepting incoming connections
int security_socket_listen(struct socket *sock, int backlog);

// Accept: before accepting a specific connection
int security_socket_accept(struct socket *sock, struct socket *newsock);

// Sendmsg/Recvmsg: before send/receive operations
int security_socket_sendmsg(struct socket *sock, struct msghdr *msg, int size);
int security_socket_recvmsg(struct socket *sock, struct msghdr *msg, int size, int flags);

// Setsockopt/Getsockopt
int security_socket_setsockopt(struct socket *sock, int level, int optname);

// Connection request from network (TCP SYN received)
int security_inet_conn_request(const struct sock *sk, struct sk_buff *skb,
                               struct request_sock *req);

// sk_alloc: when a new socket's sk struct is allocated
int security_sk_alloc(struct sock *sk, int family, gfp_t priority);

// sk_clone: when socket is cloned (accept())
void security_sk_clone(const struct sock *sk, struct sock *newsk);
```

**Hook Implementation Detail (security/security.c):**
```c
// The actual hook dispatch macro (simplified for clarity)
// Calls each registered module's hook, stops on first failure

int security_socket_connect(struct socket *sock,
                             struct sockaddr *address, int addrlen)
{
    return call_int_hook(socket_connect, 0, sock, address, addrlen);
    // Expands to: call each module's socket_connect hook
    // If any returns non-zero: return that error immediately
    // If all return 0: return 0 (allow)
}
```

#### SELinux — Type Enforcement Deep Dive

SELinux implements Mandatory Access Control based on security contexts (labels). Every process, file, socket, and port has a security context.

```
Security Context Format: user:role:type[:sensitivity[:categories]]

System processes:
  system_u:system_r:httpd_t:s0           (Apache web server)
  system_u:system_r:sshd_t:s0            (SSH daemon)
  system_u:system_r:named_t:s0           (DNS server)

Files:
  system_u:object_r:httpd_sys_content_t:s0   (Apache web files)
  system_u:object_r:etc_t:s0                 (/etc config files)
  system_u:object_r:shadow_t:s0              (/etc/shadow)

Network ports:
  system_u:object_r:http_port_t:s0       (port 80, 443)
  system_u:object_r:ssh_port_t:s0        (port 22)
  system_u:object_r:dns_port_t:s0        (port 53)
```

**Type Enforcement Policy (allow rules):**
```
# Apache can read its web content
allow httpd_t httpd_sys_content_t:file { read getattr open };

# Apache can bind to HTTP port (80/443)
allow httpd_t http_port_t:tcp_socket name_bind;

# Apache CANNOT bind to SSH port (no allow rule)
# -> implicit deny

# Apache can connect to MySQL
allow httpd_t mysqld_port_t:tcp_socket name_connect;

# Apache CANNOT read /etc/shadow (shadow_t)
# -> no allow rule = deny, even if DAC says ok
# This is the key difference: SELinux is MANDATORY, cannot be overridden by root

# Allow named (DNS server) to send UDP on DNS port
allow named_t dns_port_t:udp_socket name_bind;
allow named_t dns_port_t:udp_socket { send_msg recv_msg };
```

**SELinux in Container/Cloud Context:**

Each container process gets MCS (Multi-Category Security) labels:
```
Container 1: system_u:system_r:container_t:s0:c1,c2
Container 2: system_u:system_r:container_t:s0:c3,c4

Policy:
  allow container_t container_t:file { read write };
  # But because of MCS: c1,c2 != c3,c4
  # MLS/MCS enforcer: denies access between different categories
  # Even if container_t can access container_t files in general,
  # c1,c2 cannot access c3,c4 category files -> container isolation
```

This is why even if a container escapes, SELinux MCS prevents it from reading another container's files.

#### BPF LSM — Dynamic Security Policy

BPF LSM (Linux 5.7+) allows attaching eBPF programs to LSM hooks:

```c
// bpf_lsm_network_control.c
// Attach to socket_connect hook to enforce egress policy

#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <linux/in.h>

// Map: blocked destination IPs
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 10000);
    __type(key, __u32);   // IPv4 address
    __type(value, __u8);  // 1 = blocked
} blocked_dsts SEC(".maps");

// Map: allowed destination ports
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 1000);
    __type(key, __u16);   // port number
    __type(value, __u8);  // 1 = allowed
} allowed_ports SEC(".maps");

// LSM hook: called before any socket connect
SEC("lsm/socket_connect")
int BPF_PROG(check_socket_connect, struct socket *sock,
              struct sockaddr *address, int addrlen)
{
    struct sockaddr_in *addr4;
    __u32 dst_ip;
    __u16 dst_port;
    __u8 *blocked;
    __u8 *allowed;

    // Only enforce on IPv4 TCP/UDP
    if (address->sa_family != AF_INET)
        return 0;  // allow all non-IPv4

    addr4 = (struct sockaddr_in *)address;
    dst_ip = bpf_ntohl(addr4->sin_addr.s_addr);
    dst_port = bpf_ntohs(addr4->sin_port);

    // Check if destination IP is blocked
    blocked = bpf_map_lookup_elem(&blocked_dsts, &dst_ip);
    if (blocked && *blocked == 1) {
        bpf_printk("BPF LSM: blocked connect to %u\n", dst_ip);
        return -EPERM;
    }

    // Check if destination port is allowed (allowlist model)
    allowed = bpf_map_lookup_elem(&allowed_ports, &dst_port);
    if (!allowed) {
        bpf_printk("BPF LSM: port %u not in allowlist\n", dst_port);
        return -EPERM;
    }

    return 0;  // allow
}

char _license[] SEC("license") = "GPL";
```

This BPF LSM program can be:
- Updated dynamically without kernel recompile
- Scoped to specific cgroups (container-specific policy)
- Combined with maps updated by userspace policy daemon

---

### Chapter 13: Capabilities & Privilege Decomposition

#### The Problem with Binary Root

Traditional UNIX: UID=0 gets everything, everyone else gets almost nothing. This binary model forces the "run as root" anti-pattern — services need ONE elevated permission (bind port 80) but get ALL elevated permissions (read /etc/shadow, load kernel modules, etc.).

#### Linux Capabilities — Decomposing Root

Linux capabilities split root's power into ~40 distinct units:

```
Network-Critical Capabilities:
================================

CAP_NET_ADMIN (0x20):
  - Interface configuration (ip link add, ip addr add)
  - Routing table modification (ip route add)
  - Packet filtering (iptables, nftables, ebpf tc programs)
  - Traffic shaping (tc qdisc, tc class)
  - Network namespace management
  - ARP/NDP table modification
  - WireGuard interface management
  Risk: VERY HIGH - can modify all network configuration

CAP_NET_BIND_SERVICE (0x400):
  - Bind to port < 1024 (privileged ports)
  Risk: LOW - only affects which ports can be bound

CAP_NET_RAW (0x2000):
  - Create raw sockets (AF_PACKET, SOCK_RAW)
  - Use IP_TRANSPARENT socket option (transparent proxying)
  - Promiscuous mode (packet capture - tcpdump)
  Risk: HIGH - can capture all network traffic, forge packets

CAP_NET_BROADCAST (0x4000):
  - Broadcast/multicast sockets
  Risk: LOW

CAP_BPF (0x40000000, since Linux 5.8):
  - Load eBPF programs (all types)
  - Create/access eBPF maps
  - Without this: need CAP_SYS_ADMIN for eBPF
  Risk: HIGH - eBPF programs run in kernel

CAP_PERFMON (since Linux 5.8):
  - Read kernel perf events
  - Required for some eBPF operations
  Risk: MEDIUM - can observe performance data

CAP_SYS_ADMIN (catch-all, very dangerous):
  - Network namespace creation (clone(CLONE_NEWNET))
  - Many ioctl operations
  Risk: CRITICAL - essentially root for many purposes

CAP_SETUID / CAP_SETGID:
  - Change UID/GID to arbitrary values
  Risk: HIGH - can become any user
```

**Checking capabilities in kernel code:**
```c
// From net/core/sock.c (example pattern)
static int sock_setopt(struct socket *sock, int level, int optname, ...)
{
    // ...
    case SO_BINDTODEVICE:
        if (!ns_capable(sock_net(sk)->user_ns, CAP_NET_RAW))
            return -EPERM;
        // ...
    case SO_MARK:
        if (!ns_capable(sock_net(sk)->user_ns, CAP_NET_ADMIN))
            return -EPERM;
        // ...
}

// ns_capable() checks capability relative to the network namespace's user namespace
// This is IMPORTANT: capabilities are namespace-scoped
// A process with CAP_NET_ADMIN in a network namespace
// can only administrate THAT network namespace, not the host
```

**Container capability hardening:**
```
Default Docker capabilities (subset of root):
  Allowed: CAP_CHOWN, CAP_DAC_OVERRIDE, CAP_FOWNER, CAP_FSETID,
           CAP_KILL, CAP_SETGID, CAP_SETUID, CAP_SETPCAP,
           CAP_NET_BIND_SERVICE, CAP_SYS_CHROOT, CAP_MKNOD,
           CAP_AUDIT_WRITE, CAP_SETFCAP

  Dropped by default:
    CAP_NET_ADMIN, CAP_NET_RAW, CAP_SYS_ADMIN, CAP_BPF,
    CAP_SYS_PTRACE, CAP_SYS_BOOT, CAP_SYS_MODULE, ...

Security hardening: drop ALL, add back only what's needed:
  docker run --cap-drop=ALL --cap-add=NET_BIND_SERVICE ...
```

---

### Chapter 14: Namespaces, Seccomp-BPF & Cgroups v2

#### Linux Namespaces — Isolation Building Blocks

Each namespace type partitions a specific global resource so each namespace sees its own view:

```
Namespace Types and What They Isolate:
========================================

PID Namespace:
  - Process IDs
  - init (PID 1) inside namespace != host init
  - Processes inside can't see processes outside
  - kill(2) across namespace boundary: no-op (different PID space)
  - Used by: containers (each has its own PID 1)

NET Namespace:
  - Network interfaces (each namespace gets own eth0, lo, etc.)
  - IP routing tables
  - Netfilter/iptables rules
  - Conntrack state
  - Socket table (netstat shows only this namespace's sockets)
  - Port space (container can bind :80 independently of host :80)
  - Used by: containers, network function virtualization, VMs (virtio)

MNT Namespace:
  - Filesystem mount points
  - Container sees its own root filesystem
  - Bind mounts, overlayfs invisible to other namespaces (by default)

UTS Namespace:
  - Hostname and NIS domain name
  - Container can have its own hostname

IPC Namespace:
  - System V IPC (semaphores, message queues, shared memory)
  - POSIX message queues
  - Prevents cross-container shared memory attacks

USER Namespace:
  - Map UIDs/GIDs to different values
  - UID 0 inside container = UID 100000 outside (rootless containers)
  - Capability scope: capabilities inside are only valid inside
  - POWERFUL: allows unprivileged creation of other namespace types

CGROUP Namespace:
  - Makes cgroup hierarchy appear rooted at current cgroup
  - Container can't see host cgroup names

TIME Namespace (Linux 5.6):
  - CLOCK_MONOTONIC, CLOCK_BOOTTIME offsets per namespace
  - Useful for migration (preserve relative time)
```

**Network Namespace Internal Structure:**

```
struct net (the network namespace object):
  |
  +-- struct list_head    dev_base_head     // list of net devices
  +-- struct hlist_head   *dev_name_head    // hash by name
  +-- struct hlist_head   *dev_index_head   // hash by ifindex
  +-- struct netns_ipv4   ipv4              // IPv4 state
  |     |-- struct ctl_table_header *sysctl // /proc/sys/net/ipv4/*
  |     |-- struct sock   *fifo_open_sock   // FIFO of open sockets
  |     +-- ...
  +-- struct netns_ipv6   ipv6              // IPv6 state
  +-- struct netns_nf     nf                // Netfilter state
  |     +-- struct nf_conntrack *ct             // Connection tracking state
  +-- struct netns_frags  frags             // Fragment reassembly state
  +-- struct netns_mcast  mcast             // Multicast group state
  +-- struct netns_sctp   sctp              // SCTP state
```