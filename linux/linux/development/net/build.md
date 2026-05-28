Your existing notes cover the general case well. For **net subsystem development specifically**, the answer is more nuanced — the right config strategy depends on a 3-axis decision:

```
Axis 1: Build speed vs hardware fidelity
Axis 2: QEMU iteration vs real hardware testing
Axis 3: Subsystem coverage vs full distro parity
```

Let me give you the expert answer for each scenario, then the recommended layered approach.

---

## The Core Problem With Generic Advice

`cp /boot/config-$(uname -r) .config && make olddefconfig` is the **safe beginner choice** — but for net subsystem dev it has two real problems:

```
Problem 1: Ubuntu's distro config enables ~12,000 options.
           Build time: 45-60 min on 8 cores.
           For net subsystem, you touch ~200 files.
           You're compiling 11,800 things you never touch.

Problem 2: Distro config DISABLES most net debug options.
           CONFIG_NET_DROP_MONITOR, CONFIG_DYNAMIC_DEBUG,
           CONFIG_DEBUG_NET — all off by default in Ubuntu kernels.
           You need these. You won't know they're missing until
           you're blind to a kernel bug.
```

---

## The Recommended Strategy: Layered Config

Think of it as three layers applied in sequence:

```
Layer 1: Minimal bootable QEMU base
         (fast builds, deterministic environment)
         
Layer 2: Net subsystem essentials
         (everything net/socket/protocol related)
         
Layer 3: Debug and observability
         (what kernel devs actually use to find bugs)
```

---

## Step-by-Step: The Expert Net Subsystem Config

### Phase 1 — Start from QEMU-optimized base

```bash
cd ~/path/to/linux

# Start with the x86_64 KVM/QEMU-tuned config
# This is the RIGHT starting point for subsystem dev — not your distro config
make x86_64_defconfig
make kvm_guest.config
```

`kvm_guest.config` is a **config fragment** maintained by the kernel developers at `kernel/configs/kvm_guest.config` specifically for running kernels under KVM. It enables virtio, serial console, 9P — exactly what you need.

```bash
# Verify these are now set:
grep -E "CONFIG_(VIRTIO|KVM_GUEST|9P|SERIAL_8250_CONSOLE)" .config
```

### Phase 2 — Apply the net subsystem config fragment

```bash
# The kernel ships a network-specific config fragment too
# It lives at: net/Kconfig fragments — but easier via scripts/config

# Core networking stack (probably already in defconfig, verify):
scripts/config --enable CONFIG_NET
scripts/config --enable CONFIG_INET
scripts/config --enable CONFIG_IPV6
scripts/config --enable CONFIG_PACKET
scripts/config --enable CONFIG_UNIX

# Network namespaces — CRITICAL for testing isolation
scripts/config --enable CONFIG_NET_NS
scripts/config --enable CONFIG_USER_NS    # needed for unpriv netns

# Virtual devices — your test lab inside QEMU
scripts/config --enable CONFIG_VETH       # veth pairs (backbone of container net)
scripts/config --enable CONFIG_DUMMY      # dummy interfaces
scripts/config --enable CONFIG_TUN        # TUN/TAP
scripts/config --enable CONFIG_BRIDGE     # L2 bridging
scripts/config --enable CONFIG_VLAN_8021Q # 802.1Q VLAN

# Tunneling protocols — your Lumen background
scripts/config --enable CONFIG_VXLAN
scripts/config --enable CONFIG_GENEVE
scripts/config --enable CONFIG_GRE
scripts/config --enable CONFIG_IP_GRE
scripts/config --enable CONFIG_IPIP
scripts/config --enable CONFIG_IP6_GRE

# eBPF/XDP — non-negotiable for modern net subsystem work
scripts/config --enable CONFIG_BPF_SYSCALL
scripts/config --enable CONFIG_XDP_SOCKETS
scripts/config --enable CONFIG_BPF_JIT
scripts/config --enable CONFIG_BPF_JIT_ALWAYS_ON
scripts/config --enable CONFIG_CGROUP_BPF
scripts/config --enable CONFIG_BPF_EVENTS
scripts/config --enable CONFIG_BPF_STREAM_PARSER

# TC (Traffic Control) — needed for BPF tc programs
scripts/config --enable CONFIG_NET_SCHED
scripts/config --enable CONFIG_NET_SCH_INGRESS
scripts/config --enable CONFIG_NET_CLS_BPF
scripts/config --enable CONFIG_NET_ACT_BPF

# BGP/routing infrastructure
scripts/config --enable CONFIG_IP_ADVANCED_ROUTER
scripts/config --enable CONFIG_IP_MULTIPLE_TABLES
scripts/config --enable CONFIG_IPV6_MULTIPLE_TABLES
scripts/config --enable CONFIG_IP_ROUTE_MULTIPATH

# IPsec (your mTLS/IPsec background)
scripts/config --enable CONFIG_XFRM
scripts/config --enable CONFIG_XFRM_USER
scripts/config --enable CONFIG_XFRM_STATISTICS
scripts/config --enable CONFIG_NET_KEY
scripts/config --enable CONFIG_INET_ESP
scripts/config --enable CONFIG_INET_AH
```

### Phase 3 — Debug and observability (most missed, most critical)

```bash
# BTF — mandatory for bpftool, libbpf, Cilium debugging
scripts/config --enable CONFIG_DEBUG_INFO
scripts/config --enable CONFIG_DEBUG_INFO_BTF
scripts/config --enable CONFIG_DEBUG_INFO_DWARF5

# Dynamic debug — lets you enable pr_debug() at runtime with no recompile
scripts/config --enable CONFIG_DYNAMIC_DEBUG
scripts/config --enable CONFIG_DYNAMIC_DEBUG_CORE

# Net drop monitor — see WHERE in the stack packets are dropped
scripts/config --enable CONFIG_NET_DROP_MONITOR

# Socket monitoring — ss/iproute2 advanced stats
scripts/config --enable CONFIG_INET_DIAG
scripts/config --enable CONFIG_INET_UDP_DIAG
scripts/config --enable CONFIG_INET_RAW_DIAG

# Tracing infrastructure (ftrace, perf events for net functions)
scripts/config --enable CONFIG_FTRACE
scripts/config --enable CONFIG_FUNCTION_TRACER
scripts/config --enable CONFIG_DYNAMIC_FTRACE
scripts/config --enable CONFIG_KPROBES
scripts/config --enable CONFIG_UPROBES
scripts/config --enable CONFIG_PERF_EVENTS

# KASAN — catches memory bugs in kernel (priceless for net buf handling)
# WARNING: 2-3x slowdown, only enable when hunting bugs
# scripts/config --enable CONFIG_KASAN
# scripts/config --enable CONFIG_KASAN_INLINE

# Lockdep — catches deadlocks in net locking paths
# Also a slowdown, enable targeted:
# scripts/config --enable CONFIG_PROVE_LOCKING
# scripts/config --enable CONFIG_LOCKDEP

# Netfilter (even if not your focus — many tests assume it)
scripts/config --enable CONFIG_NETFILTER
scripts/config --enable CONFIG_NF_CONNTRACK
scripts/config --enable CONFIG_NF_TABLES
```

### Phase 4 — Resolve and finalize

```bash
# After all scripts/config changes, always run this:
# It resolves dependencies (enabling one option may require enabling others)
make olddefconfig

# Sanity check — verify your key options survived:
for opt in BPF_SYSCALL XDP_SOCKETS VXLAN GENEVE NET_NS DEBUG_INFO_BTF; do
    val=$(grep "CONFIG_${opt}" .config | head -1)
    printf "%-35s %s\n" "CONFIG_${opt}" "${val:-NOT FOUND}"
done
```

---

## Expected Config State After This Process

```
CONFIG_BPF_SYSCALL=y
CONFIG_XDP_SOCKETS=y
CONFIG_BPF_JIT=y
CONFIG_DEBUG_INFO_BTF=y
CONFIG_VXLAN=m
CONFIG_GENEVE=m
CONFIG_NET_NS=y
CONFIG_VETH=m
CONFIG_DYNAMIC_DEBUG=y
CONFIG_NET_DROP_MONITOR=m
CONFIG_KPROBES=y
...
```

---

## Build Time Comparison

```
Strategy                       Approx Build Time (8 cores)
─────────────────────────────────────────────────────────
Ubuntu distro config           45-60 min
defconfig + kvm + net layers   8-12 min      ← your target
defconfig alone                5-7 min
Single subsystem (net/ only)   ~30 seconds   ← for fast checks
```

The 8-12 min build is your **daily driver**. For a single-file change in `net/ipv4/tcp.c`, incremental rebuild after that is 3-10 seconds.

---

## One More Weapon: `localmodconfig`

This is an underused tool that many professional kernel devs rely on:

```bash
# First, make sure all the kernel modules you use daily are LOADED
# on your running system:
lsmod

# Then run:
make LSMOD=/tmp/my-modules.txt localmodconfig
# It reads the currently loaded modules and generates a config that
# includes EXACTLY those modules — nothing more.

# Steps:
lsmod > /tmp/my-modules.txt
make LSMOD=/tmp/my-modules.txt localmodconfig
# Result: very lean config tuned to YOUR machine's actual hardware
```

The catch: `localmodconfig` only captures what's loaded NOW. If you haven't exercised your VXLAN/GRE interfaces, those modules won't be in `lsmod` and will be excluded. Fix: load all relevant modules before running it:

```bash
sudo modprobe vxlan geneve gre ip_gre veth dummy tun
sudo modprobe xfrm_user esp4 ah4
lsmod > /tmp/my-modules.txt
make LSMOD=/tmp/my-modules.txt localmodconfig

# Then add the net debug options on top via scripts/config (Phase 3 above)
make olddefconfig
```

---

## Decision Map for Your Specific Case

```
You are developing Linux net subsystem:

Are you iterating on kernel logic (socket, tcp, xdp code paths)?
  YES → x86_64_defconfig + kvm_guest.config + net layers (above)
        Build in QEMU. 8-12 min first build, ~5 sec incremental.

Are you debugging a specific crash/memory bug?
  YES → Same config + enable CONFIG_KASAN + CONFIG_PROVE_LOCKING
        Accept 2-3x slowdown. Worth it.

Are you testing with real tools (iproute2, bpftool, Cilium)?
  YES → Ensure CONFIG_DEBUG_INFO_BTF=y. Without BTF, bpftool is blind.

Do you need to test on real hardware (not QEMU)?
  YES → cp /boot/config-$(uname -r) .config + olddefconfig
        Then layer your net debug options on top.
        Don't reboot until QEMU confirms it works.
```

---

The mental model to carry forward: **your `.config` is a test harness contract, not just a compilation manifest.** Every `CONFIG_DEBUG_*` and `CONFIG_NET_DROP_MONITOR` you enable is an instrumentation point — the kernel's way of exposing its internal state to you. Kernel subsystem devs who miss this work blind. The time cost of the extra options is trivial compared to the observability they unlock.

Good — now the picture is clear. Let me map your exact topology first, then give you the precise config strategy for it.

---

## Your Actual Topology

```
┌─────────────────────────────────────────────────┐
│  HOST: Dell G3 3500 — Ubuntu Desktop (Wayland)  │
│                                                  │
│  ┌─────────────────────────────────────────┐    │
│  │  KVM VM: Ubuntu Server                  │    │
│  │                                         │    │
│  │  - systemd, udev, real userspace        │    │
│  │  - iproute2, bpftool, clang/llvm        │    │
│  │  - virtio_net (your XDP test NIC)       │    │
│  │  - serial console: ttyS0               │    │
│  └─────────────────────────────────────────┘    │
│         ↑                        ↑              │
│   build kernel here       virsh console         │
│   copy bzImage + modules → install in VM        │
└─────────────────────────────────────────────────┘
```

This is a **build-on-host, run-in-VM** workflow. The critical insight this changes:

> You must NOT use your host's `/boot/config-$(uname -r)` as the base. That is the host kernel config — it knows nothing about virtio, the VM's hardware profile, or Ubuntu Server's init requirements.

---

## The Right Config Source: Pull From the VM Itself

```bash
# Get the config that currently boots your Ubuntu Server VM
# (run this on your HOST)
scp user@<vm-ip>:/boot/config-$(uname -r) ~/kernel-dev/vm-base.config

# Copy into your kernel source tree
cp ~/kernel-dev/vm-base.config /path/to/linux/.config

# Resolve any new options from your kernel version vs VM's kernel version
make olddefconfig
```

This gives you a config that:
- Already has virtio_net, virtio_blk, virtio_pci correctly set
- Already satisfies systemd/udev's kernel feature requirements
- Already has serial console enabled (Ubuntu Server sets this)
- Will boot the VM with real userspace intact

If you can't SCP (VM not running), use `virsh`:

```bash
# Alternative: copy config from VM disk via virsh
virsh start <vm-name>
virsh console <vm-name>
# inside VM:
cat /boot/config-$(uname -r) > /tmp/vm.config
# back on host via scp
```

---

## Layer XDP/eBPF Development Options On Top

After the base config is in place:

```bash
cd /path/to/linux

# ── Core BPF ────────────────────────────────────────────────
scripts/config --enable  CONFIG_BPF_SYSCALL
scripts/config --enable  CONFIG_BPF_JIT
scripts/config --enable  CONFIG_BPF_JIT_ALWAYS_ON
scripts/config --enable  CONFIG_BPF_EVENTS
scripts/config --enable  CONFIG_BPF_STREAM_PARSER
scripts/config --enable  CONFIG_CGROUP_BPF

# ── XDP ─────────────────────────────────────────────────────
scripts/config --enable  CONFIG_XDP_SOCKETS
scripts/config --enable  CONFIG_XDP_SOCKETS_DIAG      # AF_XDP diagnostics

# ── virtio_net XDP — THIS IS THE CRITICAL ONE ────────────────
# virtio_net supports XDP but the config must be right
scripts/config --enable  CONFIG_VIRTIO_NET
# XDP on virtio_net requires XDP_SOCKETS — already done above
# Also requires: kernel ≥ 4.10 for basic XDP, ≥ 5.1 for AF_XDP zero-copy

# ── TC BPF (tc-bpf programs, cls_bpf) ───────────────────────
scripts/config --enable  CONFIG_NET_SCHED
scripts/config --enable  CONFIG_NET_SCH_INGRESS        # needed for tc redirect
scripts/config --enable  CONFIG_NET_CLS_BPF            # cls_bpf classifier
scripts/config --enable  CONFIG_NET_ACT_BPF            # act_bpf action
scripts/config --enable  CONFIG_NET_CLS_ACT

# ── Your specific domain: VXLAN, GRE, GENEVE, IPsec ─────────
scripts/config --enable  CONFIG_VXLAN
scripts/config --enable  CONFIG_GENEVE
scripts/config --enable  CONFIG_GRE
scripts/config --enable  CONFIG_IP_GRE
scripts/config --enable  CONFIG_IP6_GRE
scripts/config --enable  CONFIG_XFRM
scripts/config --enable  CONFIG_XFRM_USER
scripts/config --enable  CONFIG_INET_ESP
scripts/config --enable  CONFIG_INET_AH

# ── Network namespaces (your test isolation layer) ───────────
scripts/config --enable  CONFIG_NET_NS
scripts/config --enable  CONFIG_USER_NS
scripts/config --enable  CONFIG_VETH

# ── BTF — bpftool/libbpf/Cilium are BLIND without this ──────
scripts/config --enable  CONFIG_DEBUG_INFO
scripts/config --enable  CONFIG_DEBUG_INFO_BTF
scripts/config --set-val CONFIG_DEBUG_INFO_REDUCED  n    # BTF needs full debug info
scripts/config --enable  CONFIG_DEBUG_INFO_DWARF5

# ── Serial console — confirm it survives the config merge ────
scripts/config --enable  CONFIG_SERIAL_8250
scripts/config --enable  CONFIG_SERIAL_8250_CONSOLE
scripts/config --enable  CONFIG_TTY

# ── Tracing/observability ────────────────────────────────────
scripts/config --enable  CONFIG_KPROBES
scripts/config --enable  CONFIG_KPROBE_EVENTS
scripts/config --enable  CONFIG_UPROBES
scripts/config --enable  CONFIG_UPROBE_EVENTS
scripts/config --enable  CONFIG_FTRACE
scripts/config --enable  CONFIG_FUNCTION_TRACER
scripts/config --enable  CONFIG_DYNAMIC_FTRACE
scripts/config --enable  CONFIG_DYNAMIC_DEBUG
scripts/config --enable  CONFIG_NET_DROP_MONITOR
scripts/config --enable  CONFIG_TRACEPOINTS

# ── Resolve all dependencies ─────────────────────────────────
make olddefconfig
```

---

## The Transfer-Install-Boot Cycle

This is your core iteration loop. Automate it from day one:

```bash
# On HOST — save as ~/bin/kdeploy
#!/bin/bash
set -e

VM_USER="your-user"
VM_IP="192.168.122.x"          # get with: virsh domifaddr <vm-name>
KDIR="/path/to/linux"
VM_KERNEL_VERSION=$(make -s -C "$KDIR" kernelversion)

echo "── [1/4] Building kernel ──"
cd "$KDIR"
make -j$(nproc) 2>&1 | tail -10

echo "── [2/4] Copying bzImage ──"
scp arch/x86/boot/bzImage ${VM_USER}@${VM_IP}:/tmp/bzImage-dev

echo "── [3/4] Installing modules ──"
# installs to /lib/modules/<version>/ inside VM
make INSTALL_MOD_PATH=/tmp/kmod-staging modules_install
tar -czf /tmp/kmod.tar.gz -C /tmp/kmod-staging .
scp /tmp/kmod.tar.gz ${VM_USER}@${VM_IP}:/tmp/

echo "── [4/4] Installing on VM ──"
ssh ${VM_USER}@${VM_IP} << 'REMOTE'
  sudo cp /tmp/bzImage-dev /boot/vmlinuz-dev
  sudo tar -xzf /tmp/kmod.tar.gz -C /
  sudo depmod -a
  # Update GRUB — add entry for dev kernel
  sudo update-grub
  echo "Ready. Run: sudo grub-reboot 'Advanced options > dev kernel'"
  sudo reboot
REMOTE

echo "── Connecting serial console in 5s ──"
sleep 5
virsh console <vm-name>
```

```bash
chmod +x ~/bin/kdeploy
kdeploy
```

---

## Serial Console — Verify It's Configured Correctly

On the VM's GRUB config, the kernel cmdline must have:

```bash
# Inside VM: check current cmdline
cat /proc/cmdline
# Should contain: console=tty0 console=ttyS0,115200n8
# Both tty0 AND ttyS0 — tty0 for local VGA, ttyS0 for your serial access
```

If it's missing, edit `/etc/default/grub` on the VM:

```bash
# In VM:
sudo vim /etc/default/grub

# Change this line:
GRUB_CMDLINE_LINUX_DEFAULT="quiet splash"

# To this:
GRUB_CMDLINE_LINUX_DEFAULT="console=tty0 console=ttyS0,115200n8 nokaslr"
#                                                                  ↑
#                                              nokaslr: fixed kernel addresses
#                                              helps bpftool/perf symbol resolution

sudo update-grub
```

On the HOST, connect:

```bash
virsh console <vm-name>
# Disconnect: Ctrl+]
```

---

## Verify XDP Works After Boot

Once you're in the VM via serial console:

```bash
# Verify BPF subsystem loaded
bpftool version
# output should show: libbpf version x.x, features: ...

# Verify BTF is present (critical)
bpftool btf show
# should list vmlinux BTF

# Verify XDP socket support
ls /proc/net/xdp    # should exist

# Verify virtio_net supports XDP
ethtool -i eth0     # or ens3, whatever your VM's NIC is
# driver: virtio_net

# Attach a trivial XDP pass program to confirm the pipeline works
ip link set dev eth0 xdp obj xdp_pass.o sec xdp
ip link show eth0   # should show xdp/id: <N>
ip link set dev eth0 xdp off
```

---

## One Hidden Detail Most People Miss

`CONFIG_DEBUG_INFO_BTF` requires `pahole` ≥ 1.16 installed on the **build host**, not the VM:

```bash
# On HOST:
pahole --version
# If < 1.16 or missing:
sudo apt install dwarves    # Ubuntu 20.04+

# If apt version is too old (Ubuntu 18.04):
git clone https://github.com/acmel/dwarves
cd dwarves && mkdir build && cd build
cmake -D__LIB=lib .. && make && sudo make install
```

Without this, the build will silently disable BTF even though you set `CONFIG_DEBUG_INFO_BTF=y` — and `bpftool` will be unable to do CO-RE (Compile Once Run Everywhere) relocations, which kills modern libbpf programs.

---

## The Mental Model for This Setup

```
You have two distinct kernel roles:

HOST kernel     = not your concern. Ubuntu manages it.
                  Never install your dev kernel here.

VM kernel       = your laboratory. Crash it freely.
                  Config must satisfy: Ubuntu Server userspace
                  + virtio hardware + XDP/BPF development needs.

Serial console  = your lifeline when the VM kernel panics.
                  Always confirm it works BEFORE installing a new kernel.
                  A kernel that kills the serial console = blind debugging.
```

The serial console test before every new kernel install:

```bash
# Before rebooting into new kernel — confirm serial is alive:
virsh console <vm-name>
# press Enter — you should see a login prompt or systemd output
# If dead → your new kernel has a console config problem → fix before rebooting
```

This is a full runbook. I'll go phase by phase — no gaps.

---

## The Complete Picture Before You Touch Anything

```
PHASE 1: Prepare host (KVM tools, kernel build deps)
PHASE 2: Create VM + install Ubuntu Server from ISO
PHASE 3: Configure VM for kernel dev (serial, tools, SSH)
PHASE 4: Extract VM's kernel config → build kernel 7.0.6 on host
PHASE 5: Deploy built kernel into VM
PHASE 6: Boot custom kernel, verify XDP/eBPF stack
```

```
HOST (Ubuntu Desktop)                    VM (Ubuntu Server)
─────────────────────                    ──────────────────
libvirt/KVM/QEMU          manages →     virtio-net (XDP target)
kernel source tree                       systemd + real userspace
build tools (gcc, pahole)               bpftool, clang, iproute2
kdeploy script            ssh/scp →     /boot/vmlinuz-dev
                          serial  →     ttyS0 (your console)
```

---

## PHASE 1 — Prepare the Host

### 1.1 Install KVM stack

```bash
sudo apt update
sudo apt install -y \
    qemu-kvm libvirt-daemon-system libvirt-clients \
    bridge-utils virtinst virt-manager \
    cpu-checker

# Verify hardware virtualization is available
kvm-ok
# Must see: INFO: /dev/kvm exists — KVM acceleration can be used
# If not: enter BIOS → enable Intel VT-x or AMD-V

# Add yourself to both groups (logout+login after this)
sudo usermod -aG kvm,libvirt $USER
newgrp libvirt
newgrp kvm

# Verify libvirt daemon is running
sudo systemctl enable --now libvirtd
virsh version
# Should print: libvirt version x.x.x
```

### 1.2 Install kernel build dependencies on host

```bash
sudo apt install -y \
    build-essential gcc g++ make \
    libssl-dev libelf-dev \
    flex bison bc \
    pahole dwarves \
    libncurses-dev \
    git fakeroot \
    cpio xz-utils

# Verify pahole version — must be >= 1.16 for BTF support
pahole --version
# If < 1.16 or missing on older Ubuntu:
# sudo apt install -y dwarves
```

### 1.3 Get kernel 7.0.6 source on host

```bash
mkdir -p ~/kernel-dev && cd ~/kernel-dev

# Download from kernel.org
wget https://cdn.kernel.org/pub/linux/kernel/v7.x/linux-7.0.6.tar.xz
wget https://cdn.kernel.org/pub/linux/kernel/v7.x/linux-7.0.6.tar.sign

# Extract
tar -xf linux-7.0.6.tar.xz
cd linux-7.0.6

# Confirm tree structure
ls
# arch  block  certs  crypto  Documentation  drivers
# fs    include  init  ipc  kernel  lib  mm  net  ...
```

---

## PHASE 2 — Create VM and Install Ubuntu Server

### 2.1 Create the VM with the right hardware profile

The NIC model matters: `virtio` is the only one that supports XDP. Add **two NICs** — one for management (SSH), one dedicated as your XDP test interface.

```bash
# Create disk image — 40GB is enough
qemu-img create -f qcow2 ~/kernel-dev/ubuntu-server-dev.qcow2 40G

# Create the VM (virt-install)
virt-install \
    --name kernel-dev \
    --ram 4096 \
    --vcpus 4 \
    --cpu host-passthrough \
    --os-variant ubuntu22.04 \
    --disk path=~/kernel-dev/ubuntu-server-dev.qcow2,bus=virtio \
    --cdrom /path/to/ubuntu-server.iso \
    --network network=default,model=virtio \
    --network network=default,model=virtio \
    --graphics none \
    --console pty,target.type=virtio \
    --serial pty \
    --extra-args "console=tty0 console=ttyS0,115200n8" \
    --boot cdrom,hd

# --cpu host-passthrough  : exposes host CPU flags to guest
#                           required for some eBPF JIT features
# Two --network lines     : ens3 = mgmt, ens4 = XDP test NIC
# --graphics none         : headless, serial only
# --serial pty            : creates /dev/pts/X on host for virsh console
```

### 2.2 Ubuntu Server install — what to choose

The installer will run in the terminal via serial. Key choices:

```
Profile setup:
  Your name:     dev
  Server name:   kernel-dev
  Username:      dev
  Password:      (set one)

Network:
  ens3 → configure with DHCP (management)
  ens4 → leave unconfigured for now (XDP test NIC)

Storage:
  Use entire disk
  No LVM (simpler for kernel dev)

SSH:
  ✓ Install OpenSSH server   ← MANDATORY

Snaps:
  Skip all (press Done)
```

After install completes, the VM reboots. Connect:

```bash
virsh console kernel-dev
# Press Enter → Ubuntu Server login prompt
# Login with dev / your password
```

---

## PHASE 3 — Configure VM for Kernel Development

Do this INSIDE the VM via serial console or SSH.

### 3.1 Get VM IP and switch to SSH (more comfortable than serial)

```bash
# Inside VM via virsh console:
ip addr show ens3
# Note the IP — something like 192.168.122.x

# From host, SSH in (easier to paste commands):
ssh dev@192.168.122.x
```

### 3.2 Configure GRUB for serial console + kernel dev flags

```bash
# Inside VM:
sudo nano /etc/default/grub
```

Set these exact values:

```bash
GRUB_DEFAULT=0
GRUB_TIMEOUT=5
GRUB_TIMEOUT_STYLE=menu           # show menu — lets you pick kernel on reboot

# Serial console: tty0 = local VGA, ttyS0 = your virsh console
GRUB_CMDLINE_LINUX_DEFAULT=""
GRUB_CMDLINE_LINUX="console=tty0 console=ttyS0,115200n8 nokaslr"
#                                                         ↑
#                                    nokaslr: fixed kernel symbols
#                                    bpftool/perf work better with this

# Enable serial in GRUB menu itself
GRUB_TERMINAL="serial console"
GRUB_SERIAL_COMMAND="serial --speed=115200 --unit=0 --word=8 --parity=no --stop=1"
```

```bash
sudo update-grub
```

### 3.3 Install XDP/eBPF test tools inside VM

```bash
sudo apt update
sudo apt install -y \
    clang llvm \
    libbpf-dev \
    linux-tools-common linux-tools-generic \
    bpftool \
    iproute2 \
    iputils-ping \
    tcpdump \
    netcat-openbsd \
    python3 \
    strace \
    perf-tools-unstable

# Verify bpftool works
bpftool version
```

### 3.4 Extract the VM's kernel config — this is your build baseline

```bash
# Inside VM:
uname -r
# e.g.: 6.8.0-57-server

# Copy it to host
# (run from HOST):
scp dev@192.168.122.x:/boot/config-$(ssh dev@192.168.122.x uname -r) \
    ~/kernel-dev/linux-7.0.6/.config

# Verify it arrived
ls -la ~/kernel-dev/linux-7.0.6/.config
# -rw-r--r-- 1 dev dev 240K  .config
```

---

## PHASE 4 — Configure and Build Kernel 7.0.6

### 4.1 Apply the VM config as base and resolve new options

```bash
cd ~/kernel-dev/linux-7.0.6

# .config is already there from scp above
# Resolve: fills new 7.x options with defaults, keeps your VM's options
make olddefconfig

# Handle Ubuntu-specific cert options that cause build failures:
scripts/config --disable SYSTEM_TRUSTED_KEYS
scripts/config --disable SYSTEM_REVOCATION_KEYS
scripts/config --disable MODULE_SIG_KEY
```

### 4.2 Apply XDP/eBPF development config layers

```bash
# ── BPF core ────────────────────────────────────────────────
scripts/config --enable  CONFIG_BPF_SYSCALL
scripts/config --enable  CONFIG_BPF_JIT
scripts/config --enable  CONFIG_BPF_JIT_ALWAYS_ON
scripts/config --enable  CONFIG_BPF_EVENTS
scripts/config --enable  CONFIG_BPF_STREAM_PARSER
scripts/config --enable  CONFIG_CGROUP_BPF

# ── XDP ─────────────────────────────────────────────────────
scripts/config --enable  CONFIG_XDP_SOCKETS
scripts/config --enable  CONFIG_XDP_SOCKETS_DIAG

# ── virtio_net (your XDP target NIC inside KVM) ──────────────
scripts/config --enable  CONFIG_VIRTIO
scripts/config --enable  CONFIG_VIRTIO_NET
scripts/config --enable  CONFIG_VIRTIO_PCI

# ── TC/BPF pipeline ─────────────────────────────────────────
scripts/config --enable  CONFIG_NET_SCHED
scripts/config --enable  CONFIG_NET_SCH_INGRESS
scripts/config --enable  CONFIG_NET_CLS_BPF
scripts/config --enable  CONFIG_NET_ACT_BPF
scripts/config --enable  CONFIG_NET_CLS_ACT

# ── Tunnels (your domain) ────────────────────────────────────
scripts/config --enable  CONFIG_VXLAN
scripts/config --enable  CONFIG_GENEVE
scripts/config --enable  CONFIG_GRE
scripts/config --enable  CONFIG_IP_GRE
scripts/config --enable  CONFIG_IP6_GRE
scripts/config --enable  CONFIG_IPIP

# ── IPsec ───────────────────────────────────────────────────
scripts/config --enable  CONFIG_XFRM
scripts/config --enable  CONFIG_XFRM_USER
scripts/config --enable  CONFIG_XFRM_STATISTICS
scripts/config --enable  CONFIG_INET_ESP
scripts/config --enable  CONFIG_INET_AH

# ── Network namespaces + virtual devices ─────────────────────
scripts/config --enable  CONFIG_NET_NS
scripts/config --enable  CONFIG_USER_NS
scripts/config --enable  CONFIG_VETH
scripts/config --enable  CONFIG_DUMMY
scripts/config --enable  CONFIG_TUN
scripts/config --enable  CONFIG_BRIDGE

# ── BTF — bpftool / libbpf / CO-RE require this ─────────────
scripts/config --enable  CONFIG_DEBUG_INFO
scripts/config --set-val CONFIG_DEBUG_INFO_REDUCED  n
scripts/config --enable  CONFIG_DEBUG_INFO_BTF
scripts/config --enable  CONFIG_DEBUG_INFO_DWARF5

# ── Serial console — NEVER disable this ─────────────────────
scripts/config --enable  CONFIG_SERIAL_8250
scripts/config --enable  CONFIG_SERIAL_8250_CONSOLE
scripts/config --enable  CONFIG_TTY

# ── Observability / tracing ──────────────────────────────────
scripts/config --enable  CONFIG_KPROBES
scripts/config --enable  CONFIG_KPROBE_EVENTS
scripts/config --enable  CONFIG_UPROBES
scripts/config --enable  CONFIG_UPROBE_EVENTS
scripts/config --enable  CONFIG_FTRACE
scripts/config --enable  CONFIG_FUNCTION_TRACER
scripts/config --enable  CONFIG_DYNAMIC_FTRACE
scripts/config --enable  CONFIG_DYNAMIC_DEBUG
scripts/config --enable  CONFIG_NET_DROP_MONITOR
scripts/config --enable  CONFIG_TRACEPOINTS
scripts/config --enable  CONFIG_PERF_EVENTS

# ── Resolve all dependencies ─────────────────────────────────
make olddefconfig
```

### 4.3 Verify critical options before building

```bash
echo "=== Critical options check ==="
for opt in \
    BPF_SYSCALL BPF_JIT XDP_SOCKETS VIRTIO_NET \
    DEBUG_INFO_BTF NET_NS VXLAN SERIAL_8250_CONSOLE \
    KPROBES DYNAMIC_DEBUG NET_DROP_MONITOR; do
    val=$(grep "CONFIG_${opt}=" .config | head -1)
    printf "  %-35s %s\n" "CONFIG_${opt}" "${val:-⚠ NOT SET}"
done
```

Expected output:
```
  CONFIG_BPF_SYSCALL                CONFIG_BPF_SYSCALL=y
  CONFIG_BPF_JIT                    CONFIG_BPF_JIT=y
  CONFIG_XDP_SOCKETS                CONFIG_XDP_SOCKETS=y
  CONFIG_VIRTIO_NET                 CONFIG_VIRTIO_NET=y
  CONFIG_DEBUG_INFO_BTF             CONFIG_DEBUG_INFO_BTF=y
  CONFIG_NET_NS                     CONFIG_NET_NS=y
  CONFIG_VXLAN                      CONFIG_VXLAN=m
  CONFIG_SERIAL_8250_CONSOLE        CONFIG_SERIAL_8250_CONSOLE=y
  CONFIG_KPROBES                    CONFIG_KPROBES=y
  CONFIG_DYNAMIC_DEBUG              CONFIG_DYNAMIC_DEBUG=y
  CONFIG_NET_DROP_MONITOR           CONFIG_NET_DROP_MONITOR=m
```

### 4.4 Build

```bash
# Full build — first time takes 10-20 min depending on your machine
time make -j$(nproc)

# Watch for these at the end — means success:
#   Kernel: arch/x86/boot/bzImage is ready
#   BUILD  arch/x86/boot/bzImage
```

---

## PHASE 5 — Deploy Kernel Into VM

### 5.1 Create the deploy script

Save as `~/bin/kdeploy` on host:

```bash
#!/bin/bash
set -e

VM_USER="dev"
VM_IP="192.168.122.x"          # replace with your VM's actual IP
KDIR="$HOME/kernel-dev/linux-7.0.6"
KVER=$(make -s -C "$KDIR" kernelversion)

echo "━━━ Deploying kernel ${KVER} ━━━"

# 1. Copy bzImage
echo "[1/4] Copying bzImage..."
scp "$KDIR/arch/x86/boot/bzImage" \
    "${VM_USER}@${VM_IP}:/tmp/bzImage-${KVER}"

# 2. Install modules into a staging dir, tar it, copy to VM
echo "[2/4] Packaging modules..."
STAGING=$(mktemp -d)
make -C "$KDIR" INSTALL_MOD_PATH="$STAGING" modules_install 2>/dev/null
tar -czf /tmp/kmod-${KVER}.tar.gz -C "$STAGING" .
scp /tmp/kmod-${KVER}.tar.gz "${VM_USER}@${VM_IP}:/tmp/"
rm -rf "$STAGING"

# 3. Install on VM
echo "[3/4] Installing on VM..."
ssh "${VM_USER}@${VM_IP}" "bash -s" << REMOTE
  set -e
  sudo cp /tmp/bzImage-${KVER} /boot/vmlinuz-${KVER}
  
  # Extract modules
  sudo tar -xzf /tmp/kmod-${KVER}.tar.gz -C /
  sudo depmod ${KVER}
  
  # Create initramfs for the new kernel
  sudo update-initramfs -c -k ${KVER}
  
  # Add GRUB entry
  sudo update-grub
  
  # Set new kernel as next boot (once — fallback safe)
  GRUB_ENTRY=\$(grep -i "menuentry.*${KVER}" /boot/grub/grub.cfg | head -1 | \
               sed "s/menuentry '\\([^']*\\)'.*/\\1/")
  echo "GRUB entry: \$GRUB_ENTRY"
  sudo grub-reboot "\$GRUB_ENTRY"
  
  echo "━━━ Rebooting into ${KVER} ━━━"
  sudo reboot
REMOTE

# 4. Attach serial console after reboot
echo "[4/4] Connecting serial console (10s delay for boot)..."
sleep 10
virsh console kernel-dev
```

```bash
chmod +x ~/bin/kdeploy
```

### 5.2 Deploy

```bash
kdeploy
```

You'll see the boot messages stream over serial. Watch for:

```
[    0.000000] Linux version 7.0.6 ...
[    0.000000] Command line: console=tty0 console=ttyS0,115200n8 nokaslr
...
[    2.xxxxxx] bpf: BPF JIT compiler loaded
[    2.xxxxxx] NET: Registered PF_XDP protocol family
```

---

## PHASE 6 — Verify the XDP/eBPF Stack

Once logged in via serial:

### 6.1 Confirm you're running your kernel

```bash
uname -r
# 7.0.6
```

### 6.2 Verify BPF/XDP subsystem

```bash
# BTF loaded (critical)
bpftool btf show | head -5
# Should show vmlinux BTF entry

# BPF JIT enabled
cat /proc/sys/net/core/bpf_jit_enable
# 1

# XDP socket support
ls /proc/net/xdp
# should exist (empty file is fine)

# Your NICs
ip link show
# 1: lo
# 2: ens3   (management)
# 3: ens4   (XDP test NIC — this is your target)
```

### 6.3 Write and attach a minimal XDP program to confirm pipeline

On the VM, create `xdp_pass.c`:

```c
// xdp_pass.c — simplest possible XDP program
#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

SEC("xdp")
int xdp_pass_fn(struct xdp_md *ctx)
{
    return XDP_PASS;   // let all packets through
}

char _license[] SEC("license") = "GPL";
```

```bash
# Compile
clang -O2 -target bpf -c xdp_pass.c -o xdp_pass.o

# Attach to ens4 (your XDP test NIC)
sudo ip link set dev ens4 xdp obj xdp_pass.o sec xdp

# Verify it loaded
ip link show ens4
# Should show: xdp/id: 1  or  prog/xdp id 1

# Check via bpftool
sudo bpftool prog show
# Should list your xdp_pass program with id, jited: yes

# Detach
sudo ip link set dev ens4 xdp off
```

---

## Day-to-Day Iteration After This Point

```
You make a change in net/ or kernel/bpf/ on HOST
    ↓
make -j$(nproc)                  # incremental: 5-30 seconds
    ↓
kdeploy                          # copies + reboots VM
    ↓
virsh console kernel-dev         # serial: watch boot, run tests
    ↓
dmesg | tail -30                 # check your printk() output
bpftool prog show                # check BPF programs
ip link show                     # check XDP attachment
```

Save this reference for your notes — this is the complete path from zero to a working XDP/eBPF kernel dev environment.