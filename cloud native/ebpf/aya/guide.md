## Getting Started with Aya (eBPF in Rust)

**Aya** is a Rust library for writing eBPF programs — kernel-space code that runs safely inside the Linux kernel for networking, tracing, security, and observability.

### The Mental Model First

Aya projects have **two separate programs** that work together:

```
┌─────────────────────────────────────────┐
│           User Space (normal Rust)       │
│  • Loads eBPF bytecode into the kernel  │
│  • Reads maps, handles events           │
│  • Your CLI / daemon lives here         │
└────────────────┬────────────────────────┘
                 │  loads & communicates via Maps
┌────────────────▼────────────────────────┐
│         Kernel Space (eBPF program)     │
│  • Attached to hooks (XDP, TC, kprobe) │
│  • Runs on every packet / syscall / etc │
│  • Writes into Maps                     │
└─────────────────────────────────────────┘
```

---

### The Entry Point: `aya-template`

The fastest way to start is the official template:

```bash
# Install cargo-generate if you don't have it
cargo install cargo-generate

# Generate a new Aya project
cargo generate https://github.com/aya-rs/aya-template
```

It asks you a program type (XDP, TC, kprobe, etc.) and scaffolds the full dual-crate project.

---

### Project Structure

```
my-project/
├── my-project/          ← User-space crate (loads eBPF, reads maps)
│   └── src/main.rs      ← YOUR ENTRY POINT for user space
│
├── my-project-ebpf/     ← Kernel-space crate (the eBPF program)
│   └── src/main.rs      ← YOUR ENTRY POINT for kernel space
│
└── my-project-common/   ← Shared types (Maps, structs) between both
    └── src/lib.rs
```

---

### The Kernel-Space Entry Point (`*-ebpf/src/main.rs`)

```rust
#![no_std]
#![no_main]

use aya_ebpf::{macros::xdp, programs::XdpContext};
use aya_ebpf::bindings::xdp_action;

#[xdp]                          // ← This is the hook type
pub fn my_program(ctx: XdpContext) -> u32 {
    match try_my_program(ctx) {
        Ok(ret) => ret,
        Err(_) => xdp_action::XDP_ABORTED,
    }
}

fn try_my_program(_ctx: XdpContext) -> Result<u32, u32> {
    Ok(xdp_action::XDP_PASS)    // Just pass all packets for now
}

#[panic_handler]
fn panic(_info: &core::panic::PanicInfo) -> ! {
    loop {}
}
```

Key things: `#![no_std]`, `#![no_main]` — you're in kernel space, no stdlib.

---

### The User-Space Entry Point (`*/src/main.rs`)

```rust
use aya::{Bpf, programs::{Xdp, XdpFlags}};

#[tokio::main]
async fn main() -> Result<(), anyhow::Error> {
    // 1. Load the compiled eBPF bytecode
    let mut bpf = Bpf::load(include_bytes_aligned!(
        "../../target/bpfel-unknown-none/release/my-project"
    ))?;

    // 2. Get the program by name
    let program: &mut Xdp = bpf.program_mut("my_program").unwrap().try_into()?;

    // 3. Load it into the kernel
    program.load()?;

    // 4. Attach it to a network interface
    program.attach("eth0", XdpFlags::default())?;

    // 5. Keep running (read maps, handle signals, etc.)
    tokio::signal::ctrl_c().await?;
    Ok(())
}
```

---

### Hook Types — Choose Your Entry Point

| Hook | Use case | Attribute |
|------|----------|-----------|
| **XDP** | Fastest packet processing (before kernel stack) | `#[xdp]` |
| **TC** | Traffic control (after XDP, bidirectional) | `#[classifier]` |
| **kprobe** | Trace any kernel function call | `#[kprobe]` |
| **uprobe** | Trace user-space function calls | `#[uprobe]` |
| **tracepoint** | Stable kernel trace hooks | `#[tracepoint]` |
| **socket filter** | Filter on a socket | `#[socket_filter]` |

---

### Learning Path (Beginner-Friendly Order)

1. **Start with XDP** — simplest mental model (packet in → action out)
2. **Add a HashMap map** — share data between kernel and user space
3. **Log with `aya_log`** — debug your eBPF program
4. **Try a kprobe** — trace a syscall like `execve`
5. **Build something real** — e.g., a packet counter per IP

---

### Essential Resources

- 📖 **Aya Book**: https://aya-rs.dev/book/ ← start here
- 🔧 **aya-template**: https://github.com/aya-rs/aya-template
- 💡 **Examples**: https://github.com/aya-rs/book/tree/main/examples
- 🔍 **aya crates**: `aya` (user space) + `aya-ebpf` (kernel space)

---

### Build & Run

```bash
# Build the eBPF part first (cross-compile to BPF target)
cargo build --package my-project-ebpf \
  --target bpfel-unknown-none -Z build-std=core

# Then build and run user space (needs root for eBPF)
cargo build --package my-project
sudo ./target/debug/my-project
```

> **Tip**: The template includes a `xtask` helper that wraps all of this into `cargo xtask build-ebpf` and `cargo xtask run`.

---

A great first exercise: use the template to generate an **XDP program** that counts packets per source IP and prints the map from user space every second. That covers maps, XDP hooks, and user↔kernel communication all at once. Want me to walk through building that?