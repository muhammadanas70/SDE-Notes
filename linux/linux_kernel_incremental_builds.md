# Linux Kernel Incremental Builds: A Complete In-Depth Guide

---

## Table of Contents

1. [Why a Full Rebuild Takes 60 Minutes — and What Actually Happens](#1-why-a-full-rebuild-takes-60-minutes--and-what-actually-happens)
2. [The Kbuild System: How Linux's Build Infrastructure Works](#2-the-kbuild-system-how-linuxs-build-infrastructure-works)
3. [Make's Dependency Tracking: The Engine of Incremental Builds](#3-makes-dependency-tracking-the-engine-of-incremental-builds)
4. [The `.cmd` and `.d` Files: Make's Memory](#4-the-cmd-and-d-files-makes-memory)
5. [How to Do a True Incremental Build](#5-how-to-do-a-true-incremental-build)
6. [What bindeb-pkg Does vs. a Plain `make`](#6-what-bindeb-pkg-does-vs-a-plain-make)
7. [Rebuilding Only One Subsystem or Directory](#7-rebuilding-only-one-subsystem-or-directory)
8. [Header File Changes and Why They Are Special](#8-header-file-changes-and-why-they-are-special)
9. [Understanding What Will Be Rebuilt Before You Build](#9-understanding-what-will-be-rebuilt-before-you-build)
10. [ccache: Caching Compilation Results Across Builds](#10-ccache-caching-compilation-results-across-builds)
11. [The Module Build System: Built-in vs. Module vs. Not-built](#11-the-module-build-system-built-in-vs-module-vs-not-built)
12. [Installing Only What Changed Without a Full Package Rebuild](#12-installing-only-what-changed-without-a-full-package-rebuild)
13. [Testing Your Changes Without Rebooting: Live Patching and Module Reloading](#13-testing-your-changes-without-rebooting-live-patching-and-module-reloading)
14. [Practical Workflows for Different Change Types](#14-practical-workflows-for-different-change-types)
15. [Common Pitfalls That Force a Full Rebuild](#15-common-pitfalls-that-force-a-full-rebuild)
16. [Advanced: Out-of-Tree Builds with O=](#16-advanced-out-of-tree-builds-with-o)
17. [Advanced: Speeding Up With distcc (Distributed Compilation)](#17-advanced-speeding-up-with-distcc-distributed-compilation)
18. [Understanding Kconfig and When `.config` Changes Break Everything](#18-understanding-kconfig-and-when-config-changes-break-everything)
19. [Compiler Flags, LTO, and Why They Matter for Incremental Builds](#19-compiler-flags-lto-and-why-they-matter-for-incremental-builds)
20. [Mental Model: Thinking Like the Build System](#20-mental-model-thinking-like-the-build-system)
21. [Quick Reference Cheat Sheet](#21-quick-reference-cheat-sheet)

---

## 1. Why a Full Rebuild Takes 60 Minutes — and What Actually Happens

When you ran `make bindeb-pkg -j$(nproc)`, a sequence of thousands of steps executed in parallel. Understanding each one is necessary before you can confidently skip any of them.

### Phase 1: Configuration Verification

Before compiling a single file, `make` reads your `.config` file and regenerates all auto-generated headers. The most important one is `include/generated/autoconf.h`. This header translates every `CONFIG_*` variable from your `.config` into a `#define`. Every `.c` file in the kernel includes this file transitively through `include/linux/kconfig.h`. If `.config` is newer than `autoconf.h`, all source files are considered potentially stale, triggering a full rebuild.

### Phase 2: Dependency Resolution (Makefile Parsing)

`make` then reads every `Makefile` and `Kbuild` file in the source tree. The kernel has roughly 2,500 of these files. Together they form one giant dependency graph. `make` walks this graph to determine which targets are out of date. Even the graph-building itself takes a few seconds.

### Phase 3: Compilation

Each `.c` file is compiled to a `.o` (object) file by GCC or Clang. This is the dominant cost. The kernel has approximately 25,000–35,000 `.c` files depending on your configuration. Even at 5 files per second per core, a 16-core machine compiles around 80 files per second. With 30,000 files, that is roughly 6 minutes of pure compilation — but kernel compilation is not that fast per file due to the number of headers included and compiler passes, making 60 minutes realistic.

### Phase 4: Linking

Object files are linked into per-directory `built-in.a` (static archives), which are then linked up the directory hierarchy into the final `vmlinux` ELF binary. This final link pass processes hundreds of megabytes of object code and produces a `vmlinux` often 50–200 MB in size.

### Phase 5: Compression and Packaging

`vmlinux` is compressed (with gzip, lzo, lzma, xz, or zstd depending on your `CONFIG_KERNEL_*` setting) into `bzImage` (on x86). Then `bindeb-pkg` calls scripts that create Debian `.deb` packages for the kernel image, headers, and libc-dev. This packaging stage adds several minutes.

---

## 2. The Kbuild System: How Linux's Build Infrastructure Works

Kbuild is a custom layer on top of GNU Make. It adds concepts GNU Make does not have natively: per-directory object lists, module compilation, and two-pass builds.

### The `obj-y`, `obj-m`, and `obj-` Variables

Inside every directory, a `Makefile` or `Kbuild` file declares which objects belong to the build:

```makefile
# net/ipv4/Makefile (simplified)
obj-y += tcp.o udp.o ip.o route.o
obj-$(CONFIG_INET_TCP_OFFLOAD) += tcp_offload.o
obj-$(CONFIG_TCP_CONG_CUBIC)   += tcp_cubic.o
```

- `obj-y` means: compile this file and link it statically into the kernel image.
- `obj-m` means: compile this file as a loadable kernel module (`.ko`).
- `obj-` (empty, because `CONFIG_SOMETHING` was not set) means: do not compile this file at all.

Kbuild resolves these at configuration time by substituting the `CONFIG_` values from `.config` into the `obj-$(CONFIG_...)` patterns. This is how the build system knows which files to compile.

### The Two-Pass Build

Kbuild performs its work in two conceptual passes:

**Pass 1 (descend):** Starting from the top-level `Makefile`, it descends into subdirectories recursively, collecting all `obj-y` and `obj-m` targets.

**Pass 2 (compile and link):** It compiles each `.c` file into a `.o`, archives per-directory `.o` files into `built-in.a`, and finally links everything together.

### The `vmlinux.lds` Linker Script

The final link of the kernel is controlled by a linker script (`arch/x86/kernel/vmlinux.lds` on x86, generated from `vmlinux.lds.S`). This script controls the exact memory layout: which sections go where, where the kernel code begins, where read-only data ends. Touching files that affect this script can force a full relink.

### Kbuild Makefiles vs. Top-Level Makefile

The top-level `Makefile` in the kernel root controls the overall build targets (`all`, `vmlinux`, `modules`, `bindeb-pkg`, etc.) and sets global variables (compiler, architecture, flags). Every subdirectory has its own `Makefile` or `Kbuild` file that only knows about its own directory. These are not stand-alone makefiles — they are fragments included by the Kbuild infrastructure.

---

## 3. Make's Dependency Tracking: The Engine of Incremental Builds

GNU Make's core algorithm is simple but powerful: a target is rebuilt if and only if any of its prerequisites are newer than the target itself. This is determined entirely by filesystem modification timestamps (mtime).

### The Rule

```
target: prerequisite1 prerequisite2 ...
    recipe
```

Make checks: is `max(mtime(prerequisites)) > mtime(target)`? If yes, run the recipe. If no, skip it.

### The Chain

For a kernel source file `net/ipv4/tcp.c`:

```
net/ipv4/tcp.o depends on: net/ipv4/tcp.c, all included .h files
net/ipv4/built-in.a depends on: net/ipv4/tcp.o, net/ipv4/udp.o, ...
net/built-in.a depends on: net/ipv4/built-in.a, net/ipv6/built-in.a, ...
vmlinux depends on: net/built-in.a, kernel/built-in.a, drivers/built-in.a, ...
bzImage depends on: vmlinux
```

If you change only `net/ipv4/tcp.c`:
- `net/ipv4/tcp.o` is newer than `net/ipv4/tcp.c`? No → recompile.
- `net/ipv4/built-in.a` is newer than `net/ipv4/tcp.o`? No → re-archive.
- `net/built-in.a` is newer than `net/ipv4/built-in.a`? No → re-archive.
- `vmlinux` is newer than `net/built-in.a`? No → relink.
- `bzImage` is newer than `vmlinux`? No → recompress.

So changing one `.c` file causes: 1 compile, ~3 archive operations, 1 full relink, and 1 recompress. The relink of `vmlinux` on a large kernel takes 20–60 seconds even when only one `.o` changed, because the linker must process all input archives to resolve symbols.

### Why the Incremental Build Is Still Faster

Even though `vmlinux` must be relinked on almost any source change, the dominant cost (compiling 30,000 files) is skipped. An incremental build after changing one or a few files typically takes 30 seconds to 3 minutes, compared to 60 minutes for a full build.

---

## 4. The `.cmd` and `.d` Files: Make's Memory

Make alone only tracks explicit dependencies. The kernel build system extends this with two additional file types per object: `.cmd` files and `.d` files.

### The `.d` Files (Dependency Files)

When GCC compiles `net/ipv4/tcp.c` into `net/ipv4/tcp.o`, it simultaneously produces `net/ipv4/.tcp.o.d` (a hidden dotfile). This file lists every header that `tcp.c` included, transitively:

```makefile
net/ipv4/tcp.o: \
  net/ipv4/tcp.c \
  include/linux/kernel.h \
  include/linux/types.h \
  include/net/tcp.h \
  include/net/sock.h \
  ...
```

Kbuild automatically includes all `.d` files it finds, which teaches Make about these implicit dependencies. This is how Make knows that if you modify `include/net/tcp.h`, every `.o` file that included it must be recompiled.

The GCC flag that produces `.d` files is `-MD` or `-MMD`. Look in the kernel's Makefile and you will see flags like `-Wp,-MD,$(depfile)` in the compilation rules.

### The `.cmd` Files

Each compiled object also has a corresponding `.cmd` file (e.g., `net/ipv4/.tcp.o.cmd`). This file records the exact compiler command line that was used to produce the `.o`. For example:

```
cmd_net/ipv4/tcp.o := gcc -Wp,-MD,... -Wall -O2 -fno-strict-aliasing ... -c -o net/ipv4/tcp.o net/ipv4/tcp.c
```

If the command line changes (for example, you add a `#define`, change a `CONFIG_` option, or modify compiler flags), the `.cmd` file will be different from what is recorded, and Kbuild will recompile that file even if the source itself did not change. This is the "command change detection" mechanism.

Kbuild implements this in `scripts/Kbuild.include` with the `if_changed` function. Every compilation rule is wrapped with `if_changed`, which:
1. Computes the command for the current build.
2. Compares it with the stored command in `.cmd`.
3. If they differ, runs the compilation and updates `.cmd`.
4. If they are the same and timestamps are newer-than, runs the compilation.

### Where These Files Live

These hidden files live alongside the object files:

```
net/ipv4/
├── tcp.c          ← your source
├── tcp.o          ← compiled object
├── .tcp.o.d       ← dependency list (which headers tcp.c uses)
└── .tcp.o.cmd     ← the exact gcc command used to compile tcp.o
```

You can inspect them to understand exactly why a file was rebuilt:

```bash
cat net/ipv4/.tcp.o.d    # see what headers tcp.o depends on
cat net/ipv4/.tcp.o.cmd  # see the exact compiler command
```

---

## 5. How to Do a True Incremental Build

This is the core answer to your question. After making changes, simply run `make` again with the same architecture and job count, but without the packaging target:

### Step 1: Recompile Only Changed Files and Relink

```bash
make -j$(nproc) 2>&1 | tee build_incremental.log
```

That is it. Make will automatically detect which files changed (based on timestamps and `.d`/`.cmd` records) and rebuild only those files plus anything that depends on them.

### Step 2: Rebuild Modules Only (If You Only Changed Module Code)

If your changes are only in files compiled as modules (`obj-m`), and you do not need to rebuild the monolithic kernel image:

```bash
make modules -j$(nproc) 2>&1 | tee build_incremental.log
```

### Step 3: If You Want the `.deb` Packages Again

After the incremental `make`, if you need updated `.deb` packages:

```bash
make bindeb-pkg -j$(nproc) 2>&1 | tee build_pkg.log
```

The package build will detect that `vmlinux`/`bzImage` are already up to date and skip the compilation entirely, only rebuilding the package metadata and `.deb` archives.

### Why This Just Works

When you run `make` the second time without cleaning, Make reads all the `.d` files it wrote during the first build. These tell it the complete dependency graph. It then checks timestamps. Only files whose source (or dependencies) are newer than their `.o` will be recompiled. Everything else is skipped.

### The Key Rule: Never Run `make clean` Unless You Have a Reason

`make clean` deletes all `.o` files and forces a full recompile. Only do this when:
- You changed `.config` in a way that affects many files.
- You suspect corrupted object files.
- You changed compiler toolchain or flags globally.
- You want a verified clean build for release.

---

## 6. What `bindeb-pkg` Does vs. a Plain `make`

Understanding this difference is important for knowing when you need it.

### Plain `make` (or `make all`)

Builds the following targets:
- `vmlinux` — the uncompressed ELF kernel binary.
- `arch/x86/boot/bzImage` — the compressed, bootable kernel image (on x86).
- All modules (`*.ko` files).

Output files are in the source tree. Nothing is installed anywhere. This is the fastest way to check if your code compiles and links correctly.

### `make modules_install`

Copies compiled `.ko` module files to `/lib/modules/$(uname -r)/`. This is needed to actually use new modules on your running system. Requires root.

### `make install`

Copies `bzImage` and `System.map` to `/boot/` and updates the bootloader. Requires root.

### `make bindeb-pkg`

Does everything `make all` does, then additionally:
1. Creates a staging directory tree.
2. Runs `make modules_install INSTALL_MOD_PATH=...` into the staging tree.
3. Runs `make install INSTALL_PATH=...` into the staging tree.
4. Copies headers and generates the `linux-headers-*` package content.
5. Creates three `.deb` files:
   - `linux-image-VERSION_amd64.deb` — the kernel image and modules.
   - `linux-headers-VERSION_amd64.deb` — kernel headers for out-of-tree module builds.
   - `linux-libc-dev_VERSION_amd64.deb` — user-space headers (UAPI).
6. The entire packaging process adds ~5–15 minutes beyond a plain `make`.

### When to Use Which

| Goal | Command |
|------|---------|
| Check if changes compile | `make -j$(nproc)` |
| Quick test with module reload | `make modules -j$(nproc)` then `insmod`/`modprobe` |
| Full kernel install on same machine | `make -j$(nproc)` then `sudo make modules_install install` |
| Distributable package | `make bindeb-pkg -j$(nproc)` |
| Only changed net subsystem | `make M=net -j$(nproc)` (see Section 7) |

---

## 7. Rebuilding Only One Subsystem or Directory

Kbuild supports partial builds scoped to a specific directory. This is the most powerful technique for fast iteration.

### Rebuilding a Specific Directory

```bash
# Rebuild only the net/ipv4 subsystem
make net/ipv4/ -j$(nproc)

# Rebuild only the networking subsystem
make net/ -j$(nproc)

# Rebuild a specific object file
make net/ipv4/tcp.o

# Rebuild a specific subsystem's built-in archive
make net/ipv4/built-in.a
```

The trailing slash tells Make to treat the argument as a directory target, which Kbuild interprets as "descend into this directory and build everything in it."

### Important Caveat: No Re-linking

When you use directory-scoped builds, Make rebuilds the `.o` files and updates the per-directory `built-in.a`, but it does **not** re-link `vmlinux`. This is intentional — it lets you quickly check compilation without waiting for the full link step.

If you need the final `vmlinux` and `bzImage` after a directory build, run a second pass:

```bash
make net/ -j$(nproc)  # compile only
make -j$(nproc)        # link vmlinux and produce bzImage
```

Or just run `make -j$(nproc)` from the start — Make is smart enough to only recompile what changed in `net/`, then relink.

### Rebuilding Only Changed Files Across the Whole Tree (Recommended Approach)

For most development workflows, the simplest and most reliable approach is:

```bash
make -j$(nproc)
```

Make will:
1. Check every object in the build graph.
2. Recompile only objects whose source or headers changed.
3. Re-archive only the `built-in.a` files that contain changed objects.
4. Relink `vmlinux` (this always happens if any object changed, but takes only ~30–60 seconds).
5. Recompress into `bzImage`.

The total time is dominated by the relink step, not the compilation step, once you have made only small changes.

---

## 8. Header File Changes and Why They Are Special

Changing a `.h` file has a fundamentally different impact from changing a `.c` file, because many `.c` files may include the same `.h` file.

### How Header Dependencies Are Tracked

As explained in Section 4, GCC generates `.d` files listing every header a `.c` file includes. When you change a `.h` file, Make scans all `.d` files to find every `.o` that depends on it, and recompiles all of them.

### The Cascade Problem

Consider `include/net/tcp.h`. This header is included by many files:

```bash
grep -rl "include/net/tcp.h" net/ --include="*.c" | wc -l
# might return 40+ files
```

Changing this one header triggers recompilation of 40+ `.c` files. If that header is also included indirectly via other headers, the cascade can be even larger.

### Wide Headers vs. Narrow Headers

The wider (more included) a header, the more expensive changes to it are:

- `include/linux/types.h` — included by nearly everything. Changing it rebuilds the entire kernel (tens of thousands of files).
- `include/linux/skbuff.h` — included by most networking code. Changing it rebuilds all networking.
- `net/ipv4/tcp_internal.h` (a hypothetical private header) — included only by a few files in `net/ipv4/`. Changing it affects only those few files.

### Best Practices for Header Changes

**Keep new declarations in the narrowest header possible.** If you are adding a helper function used only by `net/ipv4/tcp.c` and `net/ipv4/tcp_output.c`, do not add it to `include/net/tcp.h`. Create a local `net/ipv4/tcp_internal.h` or add it at the top of the `.c` file as a `static` function.

**Use forward declarations instead of full includes.** If a `.h` file only needs to know that a struct exists (not its full definition), use a forward declaration:
```c
struct sk_buff;  /* forward declaration */
void my_function(struct sk_buff *skb);
```
This avoids pulling in `skbuff.h` and all its transitive dependencies.

**Minimize what `include/` headers expose.** The `include/linux/` and `include/net/` directories are the "public API" of subsystems. Put implementation details in `subsystem/internal.h` files or directly in `.c` files.

### Estimating the Impact of a Header Change

Before modifying a header, estimate how many files will be affected:

```bash
# Count how many .c files include this header directly
grep -rl '"net/tcp.h"\|<net/tcp.h>' --include="*.c" . | wc -l

# For a header in include/, also check indirect inclusion
# (headers that include your header)
grep -rl 'include.*tcp\.h' --include="*.h" include/ net/
```

You can also use a dry-run approach (Section 9) to see what would be rebuilt.

---

## 9. Understanding What Will Be Rebuilt Before You Build

Before committing to a build, you can ask Make to show you what it would do without actually doing it.

### Dry Run with `-n`

```bash
make -n -j$(nproc) 2>&1 | head -100
```

The `-n` flag (also `--dry-run`) prints every command Make would execute but does not run any of them. This shows you exactly which files would be compiled. The output will look like:

```
  CC      net/ipv4/tcp.o
  CC      net/ipv4/tcp_input.o
  AR      net/ipv4/built-in.a
  AR      net/built-in.a
  LD      vmlinux
  OBJCOPY arch/x86/boot/compressed/vmlinux.bin
  ...
```

### Count Files to Be Rebuilt

```bash
make -n -j$(nproc) 2>&1 | grep '^\s*CC ' | wc -l
```

This tells you how many C files would be recompiled.

### Verbose Dry Run

```bash
make -n V=1 2>&1 | grep "gcc\|clang" | wc -l
```

With `V=1`, Make prints the full compiler invocation instead of the abbreviated `CC net/ipv4/tcp.o` form. This lets you see exact flags.

### Using `make --question` (`-q`)

```bash
make -q 2>&1
echo $?  # 0 = nothing to rebuild, 1 = something needs rebuilding
```

The `-q` flag returns exit code 0 if everything is up to date, or 1 if any target would be rebuilt. It does not print what would be rebuilt, just answers "is there work to do?"

---

## 10. ccache: Caching Compilation Results Across Builds

`ccache` is a compiler cache. It wraps GCC or Clang and stores the output of each compilation keyed by a hash of the source file, headers, and compiler flags. If you compile the same file with the same inputs again (after `make clean`, or when switching branches and back), `ccache` returns the cached `.o` instantly.

### How ccache Works

When ccache is invoked instead of GCC, it:
1. Computes a hash of: the preprocessed source, all headers, the compiler version, and all flags.
2. Checks its cache directory for a result matching this hash.
3. If found (cache hit): copies the cached `.o` to the output path immediately (microseconds).
4. If not found (cache miss): invokes the real GCC, stores the result in cache, returns the `.o`.

### Installing ccache

```bash
sudo apt install ccache
```

### Enabling ccache for Kernel Builds

**Method 1: PATH prepend (recommended)**

```bash
export PATH="/usr/lib/ccache:$PATH"
make -j$(nproc)
```

`/usr/lib/ccache/` contains symlinks named `gcc`, `cc`, `g++`, etc. that point to the ccache binary. When Make invokes `gcc`, it actually invokes ccache, which invokes the real gcc.

**Method 2: CC= override**

```bash
make CC="ccache gcc" -j$(nproc)
```

### Configuring ccache for Kernel Builds

The default cache size (5 GB) may be too small for kernel builds:

```bash
ccache --set-config=max_size=20G  # allow up to 20 GB cache
ccache --set-config=cache_dir=/fast/ssd/ccache  # use a fast disk
```

### When ccache Saves Time

ccache is most valuable when:
- You switch between git branches and back (same code, same flags → cache hit).
- You run `make clean` and rebuild (all cache hits after the first build).
- You frequently revert changes and re-test.

For a normal incremental build where you changed a few files, ccache does not help much (those files will miss the cache because the source changed). But ccache still helps with the unchanged files if, for some reason, Make incorrectly thinks they need rebuilding.

### Checking ccache Statistics

```bash
ccache --show-stats
```

Look for high hit rate (>80%) to confirm ccache is effective.

---

## 11. The Module Build System: Built-in vs. Module vs. Not-built

When a kernel feature is configured as a module (`=m` in `.config`), it produces a `.ko` file that can be loaded and unloaded at runtime. This has important implications for your development workflow.

### Module Compilation

Modules are compiled from `.c` files just like built-in code, but they are not linked into `vmlinux`. Instead, each module's `.o` files are linked together into a standalone `module_name.ko` ELF file.

For example, if `CONFIG_TCP_CONG_BBR=m`:
```
net/ipv4/tcp_bbr.ko   ← standalone module binary
```

### Module Versioning: `Module.symvers`

During a full build, the kernel produces a file called `Module.symvers` in the root of the source tree. This file lists every exported symbol (every `EXPORT_SYMBOL` and `EXPORT_SYMBOL_GPL`) in the kernel along with a CRC checksum.

When a module is loaded, the kernel verifies that the CRC of each symbol the module uses matches the CRC in the running kernel. This prevents loading a module built against a different kernel version. If you change a function signature that is exported via `EXPORT_SYMBOL`, the CRC changes, and any module built before your change will refuse to load.

### Implication: After Changing Exported Symbols

If you change a function that is exported (look for `EXPORT_SYMBOL(my_func)` after the function definition), you must:
1. Recompile the kernel (to update `Module.symvers`).
2. Recompile all modules that use that symbol.

A full `make modules` will handle both of these automatically.

### Loading and Unloading Modules for Testing

If your code changes are in a module (not built-in), you can test without rebooting:

```bash
# After make modules or make net/:
sudo rmmod tcp_bbr         # unload current version
sudo insmod net/ipv4/tcp_bbr.ko  # load new version from source tree
```

Or use `modprobe` for modules with dependencies, but you must first install:

```bash
sudo make modules_install  # installs to /lib/modules/$(uname -r)/
sudo modprobe -r tcp_bbr   # unload
sudo modprobe tcp_bbr      # load new version
```

### `depmod`: Rebuilding the Module Dependency Database

After `make modules_install`, run:

```bash
sudo depmod -a
```

This rebuilds `/lib/modules/$(uname -r)/modules.dep`, which `modprobe` uses to find module dependencies. Without this, `modprobe` may fail to find your new module or load the wrong dependencies.

---

## 12. Installing Only What Changed Without a Full Package Rebuild

If you are developing on the machine you are also testing on, you do not need `.deb` packages at all. Direct installation is much faster.

### Install Only New Modules

```bash
make -j$(nproc)           # incremental recompile
sudo make modules_install  # install only changed modules to /lib/modules/
sudo depmod -a             # update module dependency database
```

`make modules_install` is smart: it uses `install -D` and copies all `.ko` files, but only touches files that are newer than the installed versions. In practice it copies all modules regardless, but the copy operation is fast (seconds, not minutes).

### Install the Kernel Image and Initrd

```bash
sudo make install
# This copies bzImage to /boot/vmlinuz-VERSION
# and System.map to /boot/System.map-VERSION
# and calls update-initramfs and update-grub automatically
```

If `make install` does not call your bootloader updater automatically (depends on your `/sbin/installkernel` script), do it manually:

```bash
sudo update-initramfs -c -k $(make kernelrelease)
sudo update-grub
```

### Get the Kernel Version String

```bash
make kernelrelease
# prints something like: 6.9.0-rc3+
```

### The Full Non-Package Install Workflow

```bash
# Step 1: Incremental compile
make -j$(nproc)

# Step 2: Install modules
sudo make modules_install INSTALL_MOD_STRIP=1
# INSTALL_MOD_STRIP=1 strips debug symbols, making modules smaller and faster to install

# Step 3: Install kernel
sudo make install

# Step 4: If grub/initrd not updated automatically:
KVER=$(make kernelrelease)
sudo update-initramfs -c -k $KVER
sudo update-grub

# Step 5: Reboot
sudo reboot
```

This workflow takes 2–5 minutes total for an incremental build with small changes, compared to 60+ minutes for a full `bindeb-pkg`.

---

## 13. Testing Your Changes Without Rebooting: Live Patching and Module Reloading

For changes in modular code, you can test without rebooting. For built-in code, you normally cannot — but there are approaches.

### Module Reload (Best Option for Modular Code)

If your changed code is in a module (`.ko`), reload it:

```bash
# Compile only the changed module
make net/ipv4/tcp_bbr.ko

# Remove old module
sudo rmmod tcp_bbr

# Load new module directly from source tree (no install needed)
sudo insmod net/ipv4/tcp_bbr.ko

# Verify it loaded
lsmod | grep tcp_bbr
dmesg | tail -20
```

### Testing Built-in Code Changes

Built-in code (linked statically into `vmlinux`) cannot be reloaded. Your options are:

**Option 1: Reboot into new kernel.** The standard approach. Takes 1–3 minutes for a modern machine.

**Option 2: Use a virtual machine for development.** Run your development kernel inside QEMU/KVM. You can boot a new kernel in the VM in seconds without affecting your host:

```bash
# Boot your compiled kernel in QEMU
qemu-system-x86_64 \
  -kernel arch/x86/boot/bzImage \
  -initrd /boot/initrd.img-$(uname -r) \
  -append "root=/dev/sda console=ttyS0" \
  -drive file=test_disk.img,format=raw \
  -m 2G -nographic
```

**Option 3: UML (User Mode Linux).** Compile the kernel as a user-space binary and run it as a process. No VM overhead, no reboot needed. Very useful for kernel development. Set `ARCH=um` in your build:

```bash
make ARCH=um defconfig
make ARCH=um -j$(nproc)
./linux  # runs the kernel as a process
```

**Option 4: Live Patch (`kpatch`, `livepatch`).** For production systems, live patching applies a patch to a running kernel without reboot. For development, this is complex to set up. The kernel has a built-in live patching infrastructure in `kernel/livepatch/`.

### QEMU with `virtfs` for Fast Kernel Iteration

A powerful development setup:

```bash
# Host: share the source tree to the guest
qemu-system-x86_64 \
  -kernel arch/x86/boot/bzImage \
  ... \
  -virtfs local,path=/lib/modules,mount_tag=modules,security_model=mapped

# Guest: mount the host's modules
mount -t 9p -o trans=virtio modules /lib/modules
```

This lets you run `make modules_install` on the host and immediately have the modules available in the guest without copying files.

---

## 14. Practical Workflows for Different Change Types

### Workflow A: Changed One or a Few `.c` Files in `net/`

```bash
# Verify what will be rebuilt
make -n -j$(nproc) 2>&1 | grep '^\s*CC'

# Build
make -j$(nproc) 2>&1 | tee build_incremental.log

# Expected time: 1–3 minutes (1 compile + relink vmlinux)
```

### Workflow B: Changed a `.h` File in `net/`

```bash
# First, check impact
grep -rl "your_header.h" --include="*.c" . | wc -l

# Build (Make handles it automatically)
make -j$(nproc) 2>&1 | tee build_incremental.log

# Expected time: depends on how many .c files include the changed header
# Could be 2 minutes (narrow header) to 30+ minutes (wide header)
```

### Workflow C: Changed Only Module Code (`obj-m`)

```bash
# Build only modules
make modules -j$(nproc)

# Reload the specific module (no install needed)
sudo rmmod mymodule
sudo insmod path/to/mymodule.ko

# Expected time: < 1 minute for a single module
```

### Workflow D: Changed `.c` and `.h` Together in `net/ipv4/`

```bash
make net/ipv4/ -j$(nproc)   # compile net/ipv4 only (fast)
make -j$(nproc)              # relink vmlinux (30-60 seconds)
```

Or just:

```bash
make -j$(nproc)  # let Make figure out what needs rebuilding
```

### Workflow E: Changed Kernel Configuration (`.config`)

```bash
make oldconfig        # resolve any new/changed CONFIG options
make -j$(nproc)       # this may rebuild many files; potentially a full rebuild
```

If you only added/changed one `CONFIG_` option, only files that include `autoconf.h` in a way that depends on that option will be rebuilt. In practice, many files check `#ifdef CONFIG_*` so a config change can still be broad.

### Workflow F: Added a New `.c` File

1. Create your new file (e.g., `net/ipv4/my_feature.c`).
2. Add it to the appropriate `Makefile`: `obj-y += my_feature.o` (or `obj-$(CONFIG_MY_FEATURE) += my_feature.o`).
3. If using a new `CONFIG_*`, add it to `net/ipv4/Kconfig`.
4. Run `make menuconfig` or `make oldconfig` to set the new option.
5. Run `make -j$(nproc)`.

### Workflow G: Changed Assembler (`.S`) or Linker (`.lds`) Files

Assembly files (`.S`) compile to `.o` via the assembler path (not GCC). They follow the same Make rules. Changing `arch/x86/kernel/entry.S` rebuilds that object and forces a relink.

Linker script changes (`vmlinux.lds.S`) force a full relink of `vmlinux`. The relink itself is fast (30–60 seconds); the compile step is not affected.

### Workflow H: Changed a Header in `include/linux/` (Wide Header)

Proceed cautiously. These changes can trigger rebuilding thousands of files:

```bash
# Check impact first
make -n -j$(nproc) 2>&1 | grep '^\s*CC' | wc -l

# If rebuilding thousands of files, consider whether you can narrow the change
# If you must rebuild thousands of files:
make -j$(nproc) 2>&1 | tee build_wide.log
# Expected time: 10–45 minutes depending on header width
```

---

## 15. Common Pitfalls That Force a Full Rebuild

### Pitfall 1: Accidentally Touching Files

```bash
touch include/linux/compiler.h   # DO NOT do this
# Result: every .c file in the kernel recompiles
```

If you accidentally `touch` a widely-included header, do not panic. You can restore the original timestamp if you have not changed the file:

```bash
git checkout include/linux/compiler.h  # restore file and timestamp if unchanged
```

Or use `touch -r` to copy a reference timestamp:

```bash
touch -r include/linux/types.h include/linux/compiler.h
# Sets compiler.h's mtime to match types.h's mtime
```

### Pitfall 2: Modifying `.config` Directly with a Text Editor

Text editors often update the file's mtime even if you do not save any changes (some create backup files, etc.). If `.config` is newer than `include/generated/autoconf.h`, the build system regenerates `autoconf.h` and marks all source files potentially stale.

Always use `make menuconfig` or `make nconfig` to change configuration. These tools update `.config` only if the configuration actually changed, and they update `autoconf.h` atomically.

### Pitfall 3: Switching Git Branches Without Cleaning

If you switch to a branch with different code and then switch back:

```bash
git checkout feature-branch   # changes source files
make -j$(nproc)               # rebuilds changed files
git checkout main             # changes source files back
make -j$(nproc)               # rebuilds them again
```

This is fine and works correctly. However, if the branches have different `.config` files or different sets of generated files, you may need `make oldconfig` after switching. ccache (Section 10) makes this much faster because the second round of compilation hits the cache.

### Pitfall 4: Interrupted Build

If you interrupt a build (`Ctrl+C`) mid-way, some `.o` files may be partially written (truncated), which will corrupt the next build. The safest recovery:

```bash
make -j$(nproc)  # try again; partially written files may fail to link
```

If the build fails with link errors or corrupted object errors:

```bash
# Remove only the corrupted object (check the error message for which file)
rm net/ipv4/tcp.o net/ipv4/.tcp.o.d net/ipv4/.tcp.o.cmd
make -j$(nproc)  # recompile just that file
```

### Pitfall 5: Version String Changes

The kernel version string (e.g., `6.9.0-rc3+`) is embedded in every module and in `vmlinux`. It comes from the contents of the `.version` file in the build root. If this string changes between builds (for example, because you committed a new git commit, changing the `git describe` output), all modules are relinked (but not recompiled). This is usually fast but can be surprising.

The version string is also stored in `include/generated/utsrelease.h`. Changes to this file trigger recompilation of any `.c` file that includes `<linux/utsname.h>`.

Disable the version string change by explicitly setting:

```bash
make KERNELRELEASE=6.9.0-mydev -j$(nproc)
```

This pins the version string regardless of git state.

---

## 16. Advanced: Out-of-Tree Builds with `O=`

By default, Kbuild places all generated files (`.o`, `.cmd`, `.d`, `built-in.a`, `vmlinux`, etc.) directly in the source tree alongside the source files. This is called an "in-tree build."

Out-of-tree builds separate the source tree from the build output:

```bash
mkdir /fast/ssd/kernel-build
make O=/fast/ssd/kernel-build defconfig
make O=/fast/ssd/kernel-build -j$(nproc)
```

### Advantages of Out-of-Tree Builds

**Multiple configurations simultaneously.** You can have one source tree but separate build directories for different configs:

```bash
make O=../build-default defconfig && make O=../build-default -j$(nproc)
make O=../build-debug  defconfig && make O=../build-debug  -j$(nproc) CONFIG_DEBUG_KERNEL=y
```

**Clean source tree.** `git status` shows only your actual source changes, not thousands of generated object files.

**Faster builds on a RAM disk or SSD.** If your source tree is on a slow disk (HDD or NFS), putting the build output on a fast SSD dramatically speeds up builds because reading/writing thousands of small `.o` and `.d` files is I/O bound.

```bash
# Build output on tmpfs (RAM disk) — extremely fast
mkdir -p /dev/shm/kernel-build
make O=/dev/shm/kernel-build defconfig
make O=/dev/shm/kernel-build -j$(nproc)
# Warning: /dev/shm is limited by RAM; a full build may need 10–20 GB
```

**Switching between configurations without cleaning.** Keep separate `O=` directories, each with their own `.config` and object files. Switch by just changing which `O=` you use.

### Caveats

Every `make` invocation must include `O=`. Consider wrapping it in an alias or script:

```bash
alias kmake='make O=/fast/ssd/kernel-build'
kmake -j$(nproc)
kmake menuconfig
```

---

## 17. Advanced: Speeding Up With `distcc` (Distributed Compilation)

If you have multiple machines available, `distcc` distributes compilation across them.

### How distcc Works

`distcc` replaces the local GCC. It preprocesses the source file locally (because the preprocessor needs the local headers), then sends the preprocessed `.c` file to a remote machine for compilation. The remote machine runs GCC and returns the `.o` file.

### Setup

On each remote worker machine:

```bash
sudo apt install distcc
distccd --allow 192.168.1.0/24 --log-stderr --no-detach
```

On the build machine:

```bash
sudo apt install distcc
export DISTCC_HOSTS="localhost/4 192.168.1.2/8 192.168.1.3/8"
# format: host/maxjobs
make CC="distcc gcc" -j24 2>&1 | tee build.log
# -j should be 3–4x total available remote jobs
```

### Combined with ccache

```bash
export CC="ccache distcc gcc"
make -j24
```

The order matters: ccache checks first (local cache hit → skip distcc), then distcc distributes misses.

### When distcc Helps

distcc is effective when your builds are CPU-bound (many files to compile). For incremental builds of a few changed files, the overhead of preprocessing + network transfer + remote compilation often exceeds doing it locally. Use distcc for large rebuilds after header changes.

---

## 18. Understanding Kconfig and When `.config` Changes Break Everything

Kconfig is the configuration system — the language of `Kconfig` files and the machinery behind `make menuconfig`.

### The `.config` to `autoconf.h` Pipeline

1. You set options in `.config` (e.g., `CONFIG_TCP_CONG_BBR=m`).
2. `make` detects `.config` is newer than `include/generated/autoconf.h`.
3. `scripts/kconfig/conf` reads `.config` and regenerates `autoconf.h`.
4. Every `.c` file transitively includes `autoconf.h` via `<linux/kconfig.h>`.
5. All `.c` files are now potentially stale → full recompile.

### Minimizing Damage from `.config` Changes

When you change only one `CONFIG_` option, only files that use `#ifdef CONFIG_THAT_OPTION` (or equivalents) are actually affected. The build system handles this correctly via `.cmd` file comparisons: if the preprocessor output of a file is the same as before (because the `CONFIG_` option you changed is not referenced in that file), the resulting `.o` is identical, and `ccache` returns it immediately.

Without ccache, even "unchanged" files are recompiled (Make sees them as stale due to `autoconf.h` timestamp). With ccache, these recompilations are instant cache hits.

**Lesson:** Always use ccache when iterating on configuration.

### Kconfig Dependency Tracking: `include/config/auto.conf.cmd`

The Kbuild system also tracks which `Kconfig` files contributed to `.config`, stored in `include/config/auto.conf.cmd`. If you add a new `Kconfig` entry in `net/ipv4/Kconfig`, this file is updated, and any affected configuration is regenerated.

### `make oldconfig` vs. `make syncconfig`

After changing `Kconfig` files or getting a new `.config` from another source:

- `make oldconfig`: interactively asks about new options; preserves existing choices.
- `make syncconfig`: silently sets new options to defaults; used in scripts.
- `make menuconfig`: full interactive menu; lets you change any option.

Always run one of these before building if you modified `Kconfig` files or received a new `.config`.

---

## 19. Compiler Flags, LTO, and Why They Matter for Incremental Builds

### Per-File Compiler Flags

Kbuild allows per-file compiler flags using `CFLAGS_file.o`:

```makefile
# In net/ipv4/Makefile
CFLAGS_tcp.o := -O3 -funroll-loops
```

If you add or change such flags, the `.cmd` file for `tcp.o` changes, triggering a recompile of only `tcp.o`. This is a targeted change and does not affect other files.

### Global Compiler Flags

Changes to global flags in the top-level `Makefile` or `Makefile.build` affect every compilation command, causing all `.cmd` files to change, which triggers a full recompile.

### LTO (Link-Time Optimization) and ThinLTO

If you configured `CONFIG_LTO_CLANG_THIN=y`, the build uses Clang's ThinLTO. This changes the build process significantly:

- Object files are not native code; they are LLVM bitcode.
- The final link step performs optimization and code generation for the entire kernel simultaneously.
- The link step takes much longer (5–20 minutes instead of 30 seconds).
- Incremental rebuilds are more expensive because LTO requires re-optimizing across changed translation units.

Unless you specifically need LTO, disable it during development:

```
CONFIG_LTO_NONE=y
```

This restores fast incremental linking.

### `CONFIG_DEBUG_INFO` and Build Speed

Enabling debug info (`CONFIG_DEBUG_INFO=y`, used for kernel debugging with GDB or KGDB) adds DWARF debug symbols to every object file. This significantly increases:
- Compilation time (GCC generates large debug sections).
- Object file sizes (debug info can be larger than the code).
- Link time (linker processes more data).

For pure functionality testing, disable debug info:

```
CONFIG_DEBUG_INFO=n
```

Or use the split debug info option:

```
CONFIG_DEBUG_INFO_SPLIT=y
```

This puts debug info in separate `.dwo` files, keeping `.o` files small and linking fast, while still allowing debugging.

---

## 20. Mental Model: Thinking Like the Build System

The most important skill for efficient kernel development is understanding what the build system will do before you act. Here is how to think about it.

### The Dependency Graph is Your Map

Every file in the build has a position in a directed acyclic graph. An arrow from A to B means "B depends on A; if A changes, B must be rebuilt."

```
tcp.c ──────────────────────────────────────→ tcp.o ──→ net/ipv4/built-in.a ──→ net/built-in.a ──→ vmlinux ──→ bzImage
include/net/tcp.h ──→ tcp.o (and 40 others) ─↗
include/linux/skbuff.h ──→ many .o files ───↗
autoconf.h ──→ ALL .o files ────────────────↗
```

When you change a node, every node "downstream" (reachable by following arrows) must be rebuilt.

### The Two Questions to Ask

Before any build, ask yourself:

**1. Which source files changed?**
- Changed `.c` files: only those `.o` files are affected.
- Changed `.h` files: all `.o` files that include that header are affected.
- Changed `.config`: `autoconf.h` changes, potentially all `.o` files are affected.

**2. How far downstream do the changes propagate?**
- Changes in `net/ipv4/*.o` → `net/ipv4/built-in.a` → `net/built-in.a` → `vmlinux` → `bzImage`
- The relink of `vmlinux` is nearly always required when any object changes.
- The cost of the relink is fixed (~30–60 seconds); compilation cost scales with number of changed files.

### The ccache Mental Model

With ccache, think of compilation as having two paths:

```
source + headers + flags ──hash──→ cache lookup
                                        │
                              ┌─────────┴──────────┐
                         cache hit               cache miss
                              │                      │
                    return .o instantly        invoke GCC → store in cache
```

The cache key is deterministic: same inputs always produce the same key, regardless of which machine, which branch, or how many times you rebuilt.

### The Timestamp Mental Model

Without ccache, Make uses only timestamps. The rule is:

> A target is stale if any of its dependencies have a newer mtime.

A file's mtime is updated when:
- You save it in an editor (even without content changes, if the editor writes a new file).
- `git checkout` restores it (git sets mtime to the time of checkout, not the commit time).
- You run `touch` on it.
- A tool generates/overwrites it.

A file's mtime is NOT updated when:
- You read it.
- The disk is remounted.
- You copy it to/from a backup if you use `cp -p` (preserves timestamps).

### When to Trust the Incremental Build

Trust the incremental build when:
- You changed only source files tracked by git.
- You have not modified timestamps manually.
- You have not changed the toolchain (GCC version, Clang version).
- You have not changed global build flags.

Do a full rebuild when:
- You changed the toolchain.
- You made a significant `.config` change.
- The build produced unexpected results that might indicate stale objects.
- You need a verified clean build for testing or distribution.

### The Principle of Locality

The kernel's directory structure is designed so that related code is co-located. This works in your favor:

- Changes to `net/ipv4/` rarely affect `drivers/` or `arch/`.
- A `.h` file in `net/` that is only included by files in `net/` affects only the networking subsystem.
- You can reason about blast radius by looking at which directories `#include` your changed header.

Keep your changes as local as possible — prefer adding static helper functions in a `.c` file over adding exported functions to a `.h` file — and your incremental builds will always be fast.

---

## 21. Quick Reference Cheat Sheet

### Essential Commands

```bash
# Full incremental build (most common; works for all change types)
make -j$(nproc) 2>&1 | tee build.log

# Build only modules
make modules -j$(nproc)

# Build only a specific directory
make net/ipv4/ -j$(nproc)

# Build a specific object file
make net/ipv4/tcp.o

# Dry run: see what WOULD be rebuilt without building
make -n -j$(nproc) 2>&1 | grep '^\s*CC'

# Count files to be rebuilt
make -n -j$(nproc) 2>&1 | grep '^\s*CC' | wc -l

# Get the kernel version string
make kernelrelease

# Install modules directly (no .deb)
sudo make modules_install INSTALL_MOD_STRIP=1

# Install kernel directly (no .deb)
sudo make install
```

### Understand Dependency Files

```bash
# See what headers a .c file depends on
cat net/ipv4/.tcp.o.d

# See the exact compiler command used
cat net/ipv4/.tcp.o.cmd

# Check which .c files include a given header
grep -rl "your_header.h" --include="*.c" . | sort
```

### ccache Setup

```bash
sudo apt install ccache
export PATH="/usr/lib/ccache:$PATH"
ccache --set-config=max_size=20G
make -j$(nproc)
ccache --show-stats  # verify hits
```

### Timestamp Recovery (if you accidentally touched a file)

```bash
# Restore file and timestamp via git
git checkout path/to/file

# Copy timestamp from another file
touch -r reference_file target_file

# Set a specific timestamp
touch -t 202401010000.00 path/to/file
```

### Decision Tree for Build Command

```
Did you change .config?
  Yes → make oldconfig && make -j$(nproc)
  No ↓
Did you change a .h file?
  Yes, narrow header (few dependents) → make -j$(nproc)
  Yes, wide header (many dependents)  → make -j$(nproc)  [may take long]
  No ↓
Did you change only .c files in one subsystem?
  Yes, in obj-m module → make modules -j$(nproc)  then rmmod/insmod
  Yes, in obj-y code  → make -j$(nproc)
Did you change .S (assembly) or .lds (linker script)?
  Yes → make -j$(nproc)  [fast; just relink]
```

### Build Time Estimates (After Initial Full Build)

| Change Type | Expected Build Time |
|-------------|---------------------|
| 1–5 `.c` files changed | 30 seconds – 2 minutes |
| 10–20 `.c` files changed | 2–5 minutes |
| Narrow `.h` file (5–20 dependents) | 1–3 minutes |
| Medium `.h` file (50–200 dependents) | 5–15 minutes |
| Wide `.h` file (500+ dependents) | 20–45 minutes |
| `.config` change (1 option, with ccache) | 2–5 minutes |
| `.config` change (1 option, no ccache) | 20–50 minutes |
| Assembly or linker script only | 30–90 seconds (just relink) |

---

*Written for Linux kernel development. All commands assume an x86_64 host with the kernel source tree as the current directory. Adapt `ARCH=` and `CROSS_COMPILE=` as needed for cross-compilation.*
