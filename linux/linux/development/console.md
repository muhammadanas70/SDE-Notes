Good context from your notes. The SSH storm is a fundamental infrastructure problem — you're instrumenting the same interface your control channel runs over. The right fix is architectural, not just filtering. Here are the approaches from best to simplest:

---

## The Real Fix: Separate Your Control Channel from Your Test Traffic

### Option 1 — `virsh console` (Best — Zero SSH traffic in the VM)

From your Dell G3 host, access the KVM guest via serial console instead of SSH:

```bash
# On host: enable serial console in the guest first (one-time setup)
# Inside guest, add to GRUB:
# GRUB_CMDLINE_LINUX="... console=tty0 console=ttyS0,115200n8"
sudo update-grub

# Enable the serial service inside guest
sudo systemctl enable --now serial-getty@ttyS0.service

# Then from host, connect via virsh (no SSH, no network at all)
virsh console <vm-name>
# Exit with: Ctrl+]
```

This eliminates SSH from the picture entirely. Your test NIC carries only experiment traffic.

---

### Option 2 — Two NICs in the KVM Guest (Clean Separation)

Add a second virtual NIC to your VM — one for SSH management, one for experiments. You only attach XDP/eBPF probes to the experiment NIC.

```bash
# On host: add a second NIC to a running VM
virsh attach-interface <vm-name> network default --model virtio --persistent

# Inside guest: identify the two interfaces
ip link show
# e.g. enp1s0 = management (SSH), enp2s0 = experiment

# Attach your XDP only to the experiment interface
sudo ip link set dev enp2s0 xdp obj nl_ssh_filter.bpf.o sec xdp

# SSH stays on enp1s0 — completely untouched by your probes
```

---

### Option 3 — Network Namespaces + veth pairs (Fully Isolated Test Environment)

For testing VXLAN/GENEVE/XDP without *any* real interface involved:

```bash
# Create isolated namespaces
sudo ip netns add ns_test_a
sudo ip netns add ns_test_b

# Connect them with a veth pair
sudo ip link add veth0 type veth peer name veth1
sudo ip link set veth0 netns ns_test_a
sudo ip link set veth1 netns ns_test_b

# Assign addresses
sudo ip netns exec ns_test_a ip addr add 10.1.0.1/24 dev veth0
sudo ip netns exec ns_test_b ip addr add 10.1.0.2/24 dev veth1
sudo ip netns exec ns_test_a ip link set veth0 up
sudo ip netns exec ns_test_b ip link set veth1 up

# Attach XDP to veth inside the namespace
sudo ip netns exec ns_test_a ip link set dev veth0 xdp obj nl_ssh_filter.bpf.o sec xdp

# Generate test traffic (from a separate terminal — no SSH involvement)
sudo ip netns exec ns_test_b ping 10.1.0.1
sudo ip netns exec ns_test_b iperf3 -c 10.1.0.1
```

Your SSH session runs on the real NIC. The namespaces are completely isolated — zero overlap.

---

### Option 4 — tmux + Redirect Traces to File (Quick Workaround)

If you can't do any of the above immediately, decouple the trace reading from the SSH session:

```bash
# Inside guest: write traces to a file, not stdout over SSH
sudo cat /sys/kernel/debug/tracing/trace_pipe > /tmp/trace.log &

# Do your test work

# Review traces offline (or tail in a second SSH window)
tail -f /tmp/trace.log | grep -v "sshd\|ssh"
```

And filter SSH from `dmesg` output:

```bash
sudo dmesg -w | grep -v "\[NL.*:22\]"
```

---

## Recommended Setup for Your Workflow

Given you have a KVM guest on a Dell G3, the cleanest long-term setup is:

```
Dell G3 Host
 └── KVM Guest
      ├── enp1s0  ← management NIC (SSH from host)
      │              never touched by XDP/eBPF probes
      └── enp2s0  ← experiment NIC (VXLAN, GENEVE, XDP work)
                     all your probes attach here

For unit testing (no real traffic needed):
      └── netns ns_test_a ↔ netns ns_test_b via veth pair
```

And use `virsh console` when you want to observe things that would affect `enp1s0` too (e.g. tracing `ip_rcv` globally). That way you have zero network dependency for your control terminal.

The `skb_is_ssh()` guards in your `nl_debug.h` are still useful as a second line of defense, but they shouldn't be your *only* protection — if you're tracing at the XDP or `ip_rcv` level before those guards apply, the architectural separation is what saves you.

You're completely right. The infrastructure suggestions are "cleaner" on paper but they add layers you didn't set up yourself — and when something breaks during a VXLAN or XDP test, you won't know if it's your code or the extra NIC/namespace setup interfering. Manual control is the correct approach for learning kernel networking deeply.

Your existing approach is already the right philosophy. The thing to make sure is that your SSH filter guards are applied **at every layer** you're tracing, not just the ones you've done so far.

The gaps that can still cause a storm even with your current `nl_debug.h`:

**1. Global `pr_info` hooks before your guard kicks in**
If you add a trace point *above* where `ip_hdr()` or `tcp_hdr()` is valid, the guard can't be applied yet — so those will still log SSH. Example: anything in `__netif_receive_skb_core()` before `eth_type_trans()` runs.

**2. `bpf_printk` in XDP before your `nl_is_ssh_xdp()` check**
If you print something before calling the SSH check in your XDP program, it leaks.

**3. `trace_pipe` output itself**
Even with guards in your BPF/kernel code, `cat trace_pipe` over SSH generates more packets → more trace entries. The fix here is manual too — redirect trace output to a local file inside the VM, then review it rather than tailing it live:

```bash
# inside VM, background the trace capture
sudo cat /sys/kernel/debug/tracing/trace_pipe > /tmp/nl_trace.log &

# do your test

# kill it and read offline
kill %1
grep -v "ssh\|:22" /tmp/nl_trace.log
```

**4. `dmesg -w` live tailing over SSH** — same problem. Use `dmesg` after the fact, not live.

So the real discipline is: **never read traces live over SSH**. Write to file, test, disconnect, read. Your `skb_is_ssh()` guards handle the kernel side correctly — the storm usually comes from the *observation side*, not the instrumentation side.

That's it. Your manual approach is solid. Just close the live-tailing loop and you've handled it entirely in your own code.

The problem has two distinct parts. Fix both manually:

---

## Part 1 — Stop SSH from appearing in your traces

You already have `skb_is_ssh()` and `ip_rcv_is_ssh()` in `nl_debug.h`. The discipline is: **every single `pr_info` and `bpf_printk` call must be wrapped in a guard, no exceptions.**

The spots people commonly miss:

```c
// net/core/dev.c — before transport header is valid
// Use ip_rcv_is_ssh() style manual peek here, NOT skb_is_ssh()

// Any new trace point you add — check immediately:
// Does ip_hdr(skb) work here? Does tcp_hdr(skb) work here?
// If unsure, use the ip_rcv_is_ssh() approach (manual peek past IP header)
```

For your BPF side, same rule — guard before every `bpf_printk`:

```c
SEC("xdp")
int nl_xdp_trace(struct xdp_md *ctx)
{
    void *data     = (void *)(long)ctx->data;
    void *data_end = (void *)(long)ctx->data_end;

    // Check SSH FIRST, before any printk
    if (nl_is_ssh_xdp(data, data_end))
        return XDP_PASS;

    // Now safe to printk anything below this line
    bpf_printk("[NL-XDP] ...\n");
}
```

---

## Part 2 — Stop the observation loop (the harder problem)

Even with perfect guards in your code, **reading `trace_pipe` live over SSH generates new SSH packets, which your hooks see, which generate traces, which you read over SSH...** This is the loop.

The manual fix is to **never observe traces live over SSH**. Use this workflow instead:

### Step 1 — Start a background capture inside the VM

```bash
# SSH in, start the capture backgrounded, then disconnect
sudo cat /sys/kernel/debug/tracing/trace_pipe > /tmp/nl_trace.log &
echo $! > /tmp/trace_capture.pid

# For dmesg (pr_info output)
sudo dmesg --follow > /tmp/nl_dmesg.log &
echo $! >> /tmp/trace_capture.pid
```

### Step 2 — Run your test, then exit SSH completely

```bash
# Load your XDP program or trigger your test
sudo ip link set dev enp1s0 xdp obj nl_ssh_filter.bpf.o sec xdp

# Run whatever traffic generation you need
ping 10.0.0.2 -c 20

# Exit SSH — the background processes keep running
exit
```

### Step 3 — SSH back in only to read results

```bash
# Reconnect
ssh user@vm-ip

# Stop the capture
kill $(cat /tmp/trace_capture.pid)

# Read offline — no live loop, no storm
cat /tmp/nl_dmesg.log
cat /tmp/nl_trace.log
```

---

### Alternative — ftrace snapshot (cleaner for point-in-time captures)

The kernel has a built-in snapshot buffer you can trigger manually:

```bash
# Clear old data
echo 0 > /sys/kernel/debug/tracing/snapshot

# Run your test (traces go into the ring buffer)
# ...

# Take a snapshot — freezes the buffer at this moment
echo 1 > /sys/kernel/debug/tracing/snapshot

# Read the frozen snapshot — NOT live trace_pipe, no feedback loop
cat /sys/kernel/debug/tracing/snapshot
```

---

### Put it all together as a test script

Write this once inside the VM, then you just call it:

```bash
#!/bin/bash
# /home/user/nl_test.sh

# Clear buffers
echo > /sys/kernel/debug/tracing/trace
dmesg --clear

# Start background capture
cat /sys/kernel/debug/tracing/trace_pipe > /tmp/nl_trace.log &
TRACE_PID=$!

# Run test (replace with your actual test)
ping 10.0.0.2 -c 10

# Stop capture
kill $TRACE_PID

echo "--- dmesg (pr_info) ---"
dmesg > /tmp/nl_dmesg.log

echo "Done. Read /tmp/nl_trace.log and /tmp/nl_dmesg.log"
```

Then from your Dell G3:

```bash
# Launch and disconnect — no live tail
ssh user@vm-ip "sudo bash /home/user/nl_test.sh"

# SSH back when ready to read
ssh user@vm-ip "cat /tmp/nl_dmesg.log"
```

---

## Summary

| Problem | Manual Fix |
|---|---|
| SSH packets appear in traces | `skb_is_ssh()` / `ip_rcv_is_ssh()` guard on **every** trace point |
| BPF printk SSH storm | `nl_is_ssh_xdp()` check at the **top** of every XDP SEC function |
| Live `trace_pipe` feedback loop | Background capture to file, read **after** test, never tail live |
| Live `dmesg -w` feedback loop | Same — `dmesg > file` after test, not `dmesg -w` during |

The guards handle what gets logged. The file-based observation workflow breaks the feedback loop. Both are fully manual and entirely in your control.

Good question. The key is understanding that `virsh console` and network traffic are **completely separate paths** inside the hypervisor. They never touch each other.

Here's what actually happens:

```
Dell G3 (Host)
│
├── virsh console ──► virtual serial port (ttyS0)
│                         │
│                         ▼
│                    QEMU/KVM hypervisor
│                         │
│                         ▼
│                    VM's /dev/ttyS0  ← your terminal I/O lives here
│                    (NOT a NIC, NOT in the network stack)
│
└── test traffic ──► virtual NIC (enp1s0) inside VM
                          │
                          ▼
                     XDP hook runs here  ← your eBPF/XDP sees this
                          │
                          ▼
                     kernel network stack
```

The serial port and the NIC are two completely different virtual devices in QEMU. **XDP and eBPF hooks only attach to NICs** — they have no visibility into the serial device at all.

So when you:

```bash
# On host — your control terminal (goes through serial, invisible to XDP)
virsh console myvm

# On host — generate test traffic (goes through NIC, XDP sees this)
ping <vm-ip>
curl <vm-ip>
iperf3 -c <vm-ip>
```

Your XDP/eBPF program will **only** see the ping/curl/iperf traffic. The console keystrokes and output go through `ttyS0` and are never in `enp1s0`'s packet path at all.

---

**The important one for your eBPF firewall plan:**

If your firewall has a bug and drops all packets, or you accidentally do `XDP_DROP` on everything — your `virsh console` session stays alive because it was never going through the network in the first place. With SSH you'd be locked out instantly.

```
Bug in eBPF firewall → drops all packets on enp1s0
│
├── SSH connection?  → dead, locked out
└── virsh console?  → completely unaffected, still works
```

That's the real reason it's useful for your work — not just the SSH storm problem, but **safety when testing a firewall or XDP_DROP logic**. You can break the network completely and still have a working terminal to fix it.

Do this carefully in order. **Keep SSH alive the whole time** — only rely on console after you've confirmed it works.

---

## Step 1 — Verify the serial device exists in your VM

On your **Dell G3 host**:

```bash
# Check your VM name first
virsh list --all

# Check if serial port is already defined
virsh dumpxml <vm-name> | grep -A5 "serial"
```

You should see something like:

```xml
<serial type='pty'>
  <target type='isa-serial' port='0'>
    <model name='isa-serial'/>
  </target>
</serial>
```

If you see this, the serial device is already there. If not, tell me — we add it first before touching the guest.

---

## Step 2 — Inside the guest, check current GRUB config

SSH into your VM as usual:

```bash
# See what's currently in GRUB
cat /etc/default/grub | grep CMDLINE
```

You'll see something like:

```
GRUB_CMDLINE_LINUX_DEFAULT="quiet splash"
GRUB_CMDLINE_LINUX=""
```

**Back it up before touching it:**

```bash
sudo cp /etc/default/grub /etc/default/grub.backup
```

---

## Step 3 — Add serial console to GRUB

```bash
sudo nano /etc/default/grub
```

Change `GRUB_CMDLINE_LINUX` line to:

```
GRUB_CMDLINE_LINUX="console=tty0 console=ttyS0,115200n8"
```

`tty0` stays first — this means the normal display still works. `ttyS0` is added as a second console. **Both work simultaneously**, nothing breaks.

Also add this line if it doesn't exist:

```
GRUB_TERMINAL="console serial"
GRUB_SERIAL_COMMAND="serial --speed=115200 --unit=0 --word=8 --parity=no --stop=1"
```

Then apply:

```bash
sudo update-grub
```

---

## Step 4 — Enable the serial getty service

```bash
sudo systemctl enable serial-getty@ttyS0.service
sudo systemctl start serial-getty@ttyS0.service

# Verify it's running
sudo systemctl status serial-getty@ttyS0.service
```

---

## Step 5 — Reboot and test (SSH still alive as fallback)

```bash
# Reboot the guest
sudo reboot
```

**While it's rebooting, from your host — try the console immediately:**

```bash
virsh console <vm-name>
```

You should see the boot messages scrolling. When it finishes booting, you should get a login prompt. Log in normally.

```
# Exit virsh console with:
Ctrl + ]
```

---

## Step 6 — Verify nothing broke

```bash
# After reboot, SSH still works?
ssh user@vm-ip   # should work fine

# Console still works?
virsh console <vm-name>   # should also work fine
```

Both should work independently. If both work, you're done.

---

## If something goes wrong

**VM boots but no console login prompt:**
```bash
# Check if getty is running inside the VM (via SSH)
systemctl status serial-getty@ttyS0.service
journalctl -u serial-getty@ttyS0.service
```

**VM doesn't boot after GRUB change:**
```bash
# From host, open the VM's display directly
virt-manager   # GUI, open the VM console graphically
# OR
virsh start <vm-name> --console
```

Then at GRUB menu press `e`, find the linux line, remove the console params temporarily to boot, then fix `/etc/default/grub` properly. SSH still works so you have two recovery paths.

**Restore original GRUB if needed:**
```bash
sudo cp /etc/default/grub.backup /etc/default/grub
sudo update-grub
sudo reboot
```

---

## After this is working — your normal workflow

```bash
# Daily driver: connect via console (zero network traffic)
virsh console <vm-name>

# Generate test traffic FROM HOST to VM
ping <vm-ip>
iperf3 -c <vm-ip>

# Your XDP/eBPF firewall sees test traffic only
# Console is untouched no matter what you break
```

Tell me what `virsh dumpxml` shows for the serial section before proceeding to Step 2.

The serial device is already there and properly configured:

```xml
<serial type='pty'>        ← serial port exists
<console type='pty'>       ← console mapped to same port
  <target type='serial' port='0'/>
```

Nothing to add to the VM definition. Go straight to configuring the guest. Start your VM first:

```bash
virsh start netlab
```

Wait for it to boot, then SSH in as usual and do Steps 2-4:

```bash
# Backup GRUB
sudo cp /etc/default/grub /etc/default/grub.backup

# Check current state
cat /etc/default/grub | grep CMDLINE
```

Share what that output looks like — then we edit it correctly based on what's already there, no guessing.

After that:

```bash
# Enable serial getty
sudo systemctl enable serial-getty@ttyS0.service
sudo systemctl start serial-getty@ttyS0.service
```

Then edit GRUB, update, reboot. Once VM is back up, test from host **before** closing your SSH session:

```bash
virsh console netlab
```

If you get a login prompt — you're done. `Ctrl+]` to exit console, SSH still works as fallback.

Clean slate — both are empty, Ubuntu Server default. Simple edit needed.

Run this exactly:

```bash
sudo nano /etc/default/grub
```

Find these two lines and change only `GRUB_CMDLINE_LINUX`:

```bash
# Before
GRUB_CMDLINE_LINUX_DEFAULT=""
GRUB_CMDLINE_LINUX=""

# After
GRUB_CMDLINE_LINUX_DEFAULT=""
GRUB_CMDLINE_LINUX="console=tty0 console=ttyS0,115200n8"
```

Also add these two lines anywhere below (end of file is fine):

```bash
GRUB_TERMINAL="console serial"
GRUB_SERIAL_COMMAND="serial --speed=115200 --unit=0 --word=8 --parity=no --stop=1"
```

Save and exit (`Ctrl+X` → `Y` → `Enter`), then:

```bash
# Apply grub changes
sudo update-grub

# Enable serial getty
sudo systemctl enable serial-getty@ttyS0.service
sudo systemctl start serial-getty@ttyS0.service

# Verify getty is running before reboot
sudo systemctl status serial-getty@ttyS0.service
```

Share the `systemctl status` output, then we reboot and test console from host.

Serial getty is running perfectly. Now reboot:

```bash
sudo reboot
```

**Immediately after running that**, switch to your host terminal and run:

```bash
virsh console netlab
```

You should see the boot messages scroll by, then a login prompt like:

```
netuser login: _
```

Log in with your normal credentials. If you see it — setup is complete.

`Ctrl+]` to exit the console when done, then confirm SSH still works:

```bash
ssh netuser@<vm-ip>
```

Tell me what you see in the console after reboot.

Working perfectly. You can see it's running your custom kernel `7.0.6-netlab` and the VM IP is `192.168.122.23`.

Now verify SSH still works — open a **second terminal** on your host while keeping the console open:

```bash
ssh netuser@192.168.122.23
```

If both work simultaneously you're fully set up. From now on your workflow is:

```
virsh console netlab     ← daily driver, use this for all dev work
ssh netuser@192.168.122.23  ← only for file transfers (scp) if needed
```

**Quick test to confirm the storm is gone:**

In your `virsh console` session, load any XDP program or run `dmesg -w` and then from your host generate some traffic:

```bash
# From host
ping 192.168.122.23
```

You'll see only the ping traffic in your traces — no SSH feedback loop because your console goes through `ttyS0`, completely outside the network stack.

Both working simultaneously. Setup is complete.

**What you have now:**

```
Dell G3 Host
│
├── virsh console netlab  ─→  ttyS0  ─→  never touches enp1s0
│                                         XDP/eBPF blind to this
│
└── ssh 192.168.122.23    ─→  enp1s0 ─→  XDP/eBPF sees this
                                          (use only for scp/file transfer)
```

**Your dev workflow going forward:**

```bash
# Terminal 1 — control (virsh console)
virsh console netlab
# do all your kernel dev, dmesg, loading XDP programs here

# Terminal 2 — traffic generation (from host)
ping 192.168.122.23
iperf3 -c 192.168.122.23
# whatever test traffic you want your XDP/eBPF to see
```

No more SSH feedback storm. Break the network completely testing your eBPF firewall — console stays alive regardless.

```bash
Ctrl + ]
```

That's it. Just press and hold `Ctrl` then press `]`. You'll be dropped back to your host terminal immediately. Nothing happens to the VM — it keeps running normally.

```
netuser@netuser:~$        ← you're in console
# press Ctrl + ]
iamdreamer@iamdreamer-G3-3500:~$   ← back on host, VM still running
```

Reconnect anytime with `virsh console netlab`.

```bash
# Clean shutdown (like pressing power button — graceful)
virsh shutdown netlab

# Verify it stopped
virsh list --all
```

If the VM is taking too long to shut down and you need to force it:

```bash
# Force stop (like pulling the power — use only if shutdown hangs)
virsh destroy netlab
```

`destroy` sounds scary but it just force-stops it, doesn't delete anything. Your VM and all its data stays intact.