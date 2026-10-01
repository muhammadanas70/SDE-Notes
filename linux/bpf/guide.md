
# Learning eBPF

## Preface

* Who This Book Is For
* What This Book Covers
* Prerequisite Knowledge
* Example Code and Exercises
* Is eBPF Only for Linux?
* Conventions Used in This Book
* Using Code Examples
* O’Reilly Online Learning
* How to Contact Us
* Acknowledgments

---

# Chapter 1 — What Is eBPF, and Why Is It Important?

* eBPF’s Roots: The Berkeley Packet Filter
* From BPF to eBPF
* The Evolution of eBPF to Production Systems
* Naming Is Hard
* The Linux Kernel
* Adding New Functionality to the Kernel
* Kernel Modules
* Dynamic Loading of eBPF Programs
* High Performance of eBPF Programs
* eBPF in Cloud Native Environments
* Summary

### What this chapter teaches

This chapter introduces:

* History of classic BPF → modern eBPF
* Why eBPF became important in cloud-native systems
* Kernel extensibility
* Why eBPF is safer than kernel modules
* Observability + networking + security use cases


---

# Chapter 2 — eBPF’s “Hello World”

* BCC’s “Hello World”
* Running “Hello World”
* BPF Maps
* Hash Table Map
* Perf and Ring Buffer Maps
* Function Calls
* Tail Calls
* Summary
* Exercises

### What this chapter teaches

You learn:

* Your first tracing program
* How BPF maps exchange data with userspace
* Event delivery mechanisms
* Tail calls and modular eBPF programs
* Intro to BCC tooling


---

# Chapter 3 — Anatomy of an eBPF Program

Known sections from previews include:

* The eBPF Virtual Machine
* eBPF Registers
* eBPF Instructions
* eBPF “Hello World” for a Network Interface

Additional topics in this chapter discuss:

* Instruction execution model
* Packet parsing basics
* Memory access
* Program lifecycle
* Stack and registers

### What this chapter teaches

This chapter dives into:

* Internal VM architecture
* eBPF bytecode
* Register model
* Instruction formats
* Low-level packet processing


---

# Chapter 4 — The bpf() System Call

Likely major sections include:

* Loading eBPF programs
* Creating maps
* Attaching programs
* Querying BPF objects
* File descriptors and kernel interaction

### What this chapter teaches

You learn:

* How userspace communicates with eBPF
* Kernel APIs behind all eBPF tooling
* Low-level object management
* How loaders actually work internally


---

# Chapter 5 — CO-RE, BTF, and Libbpf

Likely sections:

* Compile Once — Run Everywhere (CO-RE)
* BPF Type Format (BTF)
* Libbpf fundamentals
* Portable eBPF programs
* Skeleton generation

### What this chapter teaches

Focus areas:

* Kernel-version portability
* Modern production-grade eBPF development
* Type metadata
* Libbpf architecture
* Avoiding recompilation per kernel


---

# Chapter 6 — The eBPF Verifier

* The Verification Process
* The Verifier Log
* Visualizing Control Flow
* Validating Helper Functions
* Helper Function Arguments
* Checking the License
* Checking Memory Access
* Checking Pointers Before Dereferencing Them
* Accessing Context
* Running to Completion
* Loops
* Checking the Return Code
* Invalid Instructions
* Unreachable Instructions
* Summary
* Exercises

### What this chapter teaches

This is one of the most important chapters:

* Why eBPF is safe
* Static analysis rules
* Control-flow validation
* Pointer tracking
* Loop constraints
* Memory safety
* Common verifier failures


---

# Chapter 7 — eBPF Program and Attachment Types

* Program Context Arguments
* Helper Functions and Return Codes
* Kfuncs
* Tracing
* Kprobes and Kretprobes
* Fentry/Fexit
* Tracepoints
* BTF-Enabled Tracepoints
* User Space Attachments
* LSM
* Networking
* Sockets
* Traffic Control
* XDP
* Flow Dissector
* Lightweight Tunnels
* Cgroups
* Infrared Controllers
* BPF Attachment Types
* Summary
* Exercises

### What this chapter teaches

This chapter maps the eBPF ecosystem:

* Every major attachment point
* Tracing mechanisms
* Networking hooks
* Security hooks
* XDP packet processing
* Kernel/user-space instrumentation


---

# Chapter 8 — eBPF for Networking

* Packet Drops
* XDP Program Return Codes
* XDP Packet Parsing
* Load Balancing and Forwarding
* XDP Offloading
* Traffic Control (TC)
* Packet Encryption and Decryption
* User Space SSL Libraries
* eBPF and Kubernetes Networking
* Avoiding iptables
* Coordinated Network Programs
* Network Policy Enforcement
* Encrypted Connections
* Summary
* Exercises and Further Reading

### What this chapter teaches

Core networking concepts:

* XDP fast-path networking
* Packet filtering
* Load balancing
* Kubernetes datapaths
* Replacing iptables
* Encryption visibility
* High-performance packet processing

This chapter is especially relevant if you’re working with:

* Cilium
* Hubble
* Kubernetes CNIs
* Cloud-native networking


---

# Chapter 9 — eBPF for Security

* Security Observability Requires Policy and Context
* Using System Calls for Security Events
* Seccomp
* Generating Seccomp Profiles
* Syscall-Tracking Security Tools
* BPF LSM
* Cilium Tetragon
* Attaching to Internal Kernel Functions
* Preventative Security
* Network Security
* Summary

### What this chapter teaches

Security-oriented eBPF:

* Runtime security
* Syscall monitoring
* LSM hooks
* Policy enforcement
* Threat detection
* Container runtime protection

Important projects discussed include:

* Cilium Tetragon
* Seccomp


---

# Chapter 10 — eBPF Programming

* Bpftrace
* Language Choices for eBPF
* In the Kernel
* BCC
* Python/Lua/C++
* C and Libbpf
* Go
* Gobpf
* Ebpf-go
* Libbpfgo
* Rust
* Libbpf-rs
* Redbpf
* Aya
* Rust-bcc
* Testing BPF Programs
* Multiple eBPF Programs
* Summary
* Exercises

### What this chapter teaches

Practical development ecosystems:

* BCC vs libbpf
* Go eBPF libraries
* Rust eBPF frameworks
* bpftrace scripting
* Multi-language workflows
* Testing and deployment patterns

Useful tools/frameworks mentioned:

* bpftrace
* BCC
* libbpf
* Aya


---

# Chapter 11 — The Future Evolution of eBPF

* The eBPF Foundation
* eBPF for Windows
* Linux eBPF Evolution
* eBPF Is a Platform, Not a Feature
* Conclusion

### What this chapter teaches

Future directions:

* Cross-platform eBPF
* Standardization
* Ecosystem growth
* Emerging production use cases
* eBPF as infrastructure platform

---

# Index

Comprehensive reference section for:

* Maps
* Helpers
* Attachments
* Verifier rules
* Networking concepts
* Security tooling

---
