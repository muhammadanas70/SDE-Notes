The error output is truncated — the real cause is hidden earlier in the build. Since there's a `build.log` file right there, that's the first place to look.

Run this to find the actual failure:

```bash
grep -n "Error\|error:\|fatal error\|undefined" build.log | tail -50
```

Or look at the lines just before the first `Error 2`:

```bash
grep -B 5 "Error 2" build.log | head -60
```

**Common causes for this kind of failure:**

1. **Missing dependencies** — e.g. `libelf-dev`, `libssl-dev`, `flex`, `bison`, `dwarves`
   ```bash
   sudo apt install build-essential libncurses-dev libssl-dev libelf-dev flex bison dwarves bc
   ```

2. **GCC/Clang version mismatch** — Linux 7.x may require a newer GCC
   ```bash
   gcc --version
   ```

3. **`.config` issue** — missing or misconfigured kernel config
   ```bash
   make olddefconfig
   ```

4. **Rust toolchain** (Linux 7.x has Rust support) — if Rust is enabled in config but toolchain isn't set up:
   ```bash
   grep "CONFIG_RUST" .config
   ```
   If it's `=y`, either install the right Rust toolchain or disable it with `scripts/config --disable CONFIG_RUST`.

5. **Out-of-disk-space** — kernel builds are large (~20GB)
   ```bash
   df -h .
   ```

Share the relevant lines from `build.log` and I can pinpoint the exact cause.

The root cause is clear — the `certs` subsystem is failing. Let's dig into exactly what's wrong there:

```bash
grep -n "certs\|error:\|Error" build.log | head -40
```

And specifically the lines just before the certs error:

```bash
grep -B 20 "Makefile.build:548: certs" build.log | head -30
```

**The `certs` error is almost always one of these two things:**

---

### 1. Missing signing key / certificate (most common)
The kernel tries to sign modules with a key that doesn't exist. Check your config:

```bash
grep -E "CONFIG_MODULE_SIG|CONFIG_SYSTEM_TRUSTED|CONFIG_SYSTEM_EXTRA" .config
```

If `CONFIG_MODULE_SIG_KEY` points to a missing file (like `certs/signing_key.pem`), fix it:

```bash
# Option A: Let the kernel auto-generate a key
./scripts/config --set-str CONFIG_MODULE_SIG_KEY "certs/signing_key.pem"

# Option B: Disable module signing entirely
./scripts/config --disable CONFIG_MODULE_SIG
./scripts/config --disable CONFIG_MODULE_SIG_ALL
```

---

### 2. Missing `openssl` or `libssl-dev`
```bash
openssl version          # should exist
dpkg -l libssl-dev       # should be installed
sudo apt install openssl libssl-dev
```

---

After fixing, clean the certs directory and rebuild:

```bash
rm -f certs/signing_key.pem certs/signing_key.x509 certs/x509.genkey
make bindeb-pkg -j$(nproc) 2>&1 | tee build.log
```

Share the output of the `grep -B 20` command above and I can give you the exact fix.

Found it. This line is the culprit:

```
CONFIG_SYSTEM_TRUSTED_KEYS="debian/canonical-certs.pem"
```

That file is part of Canonical's official Ubuntu kernel build infrastructure — it **doesn't exist** in a vanilla kernel source tree. The build fails because it can't find it.

Verify:
```bash
ls debian/canonical-certs.pem
```

It'll say "No such file or directory". Fix it by clearing that config value:

```bash
./scripts/config --set-str CONFIG_SYSTEM_TRUSTED_KEYS ""
```

Also check for a second related setting that often has the same issue:
```bash
grep "CONFIG_SYSTEM_REVOCATION_KEYS" .config
```

If it points to `debian/canonical-revoked-certs.pem`, clear that too:
```bash
./scripts/config --set-str CONFIG_SYSTEM_REVOCATION_KEYS ""
```

Then clean certs and rebuild:
```bash
rm -f certs/signing_key.pem certs/signing_key.x509 certs/x509.genkey
make bindeb-pkg -j$(nproc) 2>&1 | tee build.log
```

**Why this happens:** You're likely using a `.config` copied from an Ubuntu/Debian system (`/boot/config-$(uname -r)`). Those configs reference Canonical-internal certificate files that only exist inside Canonical's build environment, not in upstream kernel source.