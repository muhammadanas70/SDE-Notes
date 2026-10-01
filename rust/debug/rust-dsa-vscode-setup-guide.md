# Rust DSA Debugging in VS Code — Windows & Linux Setup Guide

Companion to the six-tool debugging guide, focused purely on getting each
tool working *inside VS Code* on both platforms. Where setup differs
between Windows and Linux, both are covered side by side; where it's
identical, it's written once.

---

## 0. Baseline setup (both platforms)

### Install Rust
**Linux:**
```bash
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
source "$HOME/.cargo/env"
```

**Windows:**
Download and run `rustup-init.exe` from https://rustup.rs. This also
prompts you to install the **MSVC Build Tools** (or you already have them
if Visual Studio is installed) — accept this, since the default Windows
Rust toolchain (`x86_64-pc-windows-msvc`) needs the MSVC linker.

Verify on either OS:
```bash
rustc --version
cargo --version
```

### Required VS Code extensions
Install these on both platforms (same extension IDs):

| Extension | ID | Purpose |
|---|---|---|
| rust-analyzer | `rust-lang.rust-analyzer` | LSP: inline errors, hover types, go-to-definition, inlay hints |
| CodeLLDB | `vadimcn.vscode-lldb` | The actual debugger backend — breakpoints, step, watch, locals |
| Even Better TOML | `tamasfe.even-better-toml` | Optional, but `Cargo.toml` editing is nicer with it |

Install via the Extensions panel (`Ctrl+Shift+X`) or command line:
```bash
code --install-extension rust-lang.rust-analyzer
code --install-extension vadimcn.vscode-lldb
```

**Uninstall/disable the old `rust` (rust-lang.rust) extension if present** —
it's deprecated in favor of `rust-analyzer` and the two can conflict.

### Project setup
Use a real cargo project, not loose `.rs` files — the debugger and
rust-analyzer both need `Cargo.toml` to work properly:
```bash
cargo new dsa-practice
cd dsa-practice
code .
```
Put each problem in its own function in `src/main.rs`, or as separate
binaries under `src/bin/` (e.g. `src/bin/two_sum.rs`) if you want to keep
problems isolated and runnable individually with `cargo run --bin two_sum`.

---

## 1. `dbg!()` — no setup required

Works identically in VS Code on both platforms — it's just `cargo run` in
the integrated terminal. Open the terminal with `` Ctrl+` ``
(`Ctrl+Shift+\`` on some Windows keyboard layouts also works), then:
```bash
cargo run
```
`dbg!` output goes to stderr, which VS Code's integrated terminal shows
inline with stdout by default, so no extra config needed.

**Tip:** rust-analyzer adds a lightbulb/code-action on hover to wrap an
expression in `dbg!()` automatically — place the cursor on an expression,
`Ctrl+.` (Windows/Linux both) to open the quick-fix menu.

---

## 2. Step debugging — the main setup (CodeLLDB)

This replaces `rust-gdb`/`rust-lldb` from the terminal-only guide — inside
VS Code, CodeLLDB drives LLDB under the hood on both OSes, so the
**config is identical on Windows and Linux** once installed.

### `.vscode/launch.json`
Create this file in your project root (VS Code will offer to generate it
automatically the first time you click the Run/Debug icon and select
"LLDB"):
```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "type": "lldb",
      "request": "launch",
      "name": "Debug DSA binary",
      "cargo": {
        "args": ["build", "--bin=dsa-practice"]
      },
      "args": [],
      "cwd": "${workspaceFolder}"
    }
  ]
}
```
If you're using `src/bin/` for separate problems, change `--bin=dsa-practice`
to the specific binary name, or duplicate the config block per problem with
a different `name` — VS Code shows a dropdown to pick between them.

### Using it
1. Click in the gutter (left of the line number) to set a breakpoint — same
   gesture on both platforms.
2. Press `F5` to start debugging.
3. Step controls (identical keybindings on Windows and Linux):
   - `F10` — step over
   - `F11` — step into
   - `Shift+F11` — step out
   - `F5` — continue
4. While paused:
   - **Variables panel** (left sidebar, auto-shown) — live locals, expand
     structs/vecs/enums inline.
   - **Watch panel** — pin a specific expression (e.g. `arr[j]`) to track
     across steps.
   - **Debug Console** (bottom panel) — type any LLDB expression directly,
     e.g. `arr` or `i * 2`.
   - **Call Stack panel** — click any frame to jump the editor to that
     point in the recursion; this is your `bt` equivalent from the
     terminal workflow.

### DSA-specific workflow, adapted to VS Code
Same method as the terminal version, just using gutter clicks instead of
`break`:
1. Set a breakpoint inside your core loop (swap line, partition step, memo
   lookup).
2. `F5`, then `F10` through 2–3 iterations, watching the Variables panel.
3. On recursive functions, set the breakpoint at the function's first line
   and use the Call Stack panel to watch frames stack up — a wrong base
   case shows up immediately as one extra (or missing) frame.

### Platform-specific notes
- **Windows:** if CodeLLDB fails to find a debugger backend, make sure
  you're on the `msvc` toolchain (`rustup show` should list
  `x86_64-pc-windows-msvc` as default) — the `gnu` toolchain has spottier
  LLDB support on Windows. Switch if needed: `rustup default stable-msvc`.
- **Linux:** works out of the box in most distros; if breakpoints show as
  hollow/unbound circles, it usually means the binary was built in
  `--release` mode (symbols stripped) — make sure `"cargo": {"args":
  ["build", ...]}` doesn't include `--release`.

---

## 3. Reading the compiler — inline in VS Code

Most of this is automatic once rust-analyzer is installed; a few things are
worth wiring up explicitly.

### Inline error diagnostics
rust-analyzer underlines errors/warnings directly in the editor and shows
the full message on hover — no setup needed. Click the error code (e.g.
`E0502`) in the **Problems panel** (`Ctrl+Shift+M`) to open
`rustc --explain` output in a side panel automatically — same behavior on
both OSes.

### `cargo expand` inside VS Code
Install the CLI tool once (needs nightly):
```bash
rustup component add rust-src --toolchain nightly
cargo install cargo-expand
```
Run it from the integrated terminal:
```bash
cargo expand --bin your_binary
```
There's no dedicated GUI panel for this — the practical VS Code workflow is
running it in a **split terminal** (`Ctrl+Shift+5` to split) so you can
compare expanded output next to your source side by side.

### `cargo clippy` — enable as the default checker
By far the most useful rust-analyzer setting for this workflow: make
clippy run instead of plain `cargo check` on every save, so lint warnings
appear inline like errors do.

Open Settings (`Ctrl+,`), search `rust-analyzer check command`, and set:
```json
"rust-analyzer.check.command": "clippy"
```
Or add directly to `.vscode/settings.json`:
```json
{
  "rust-analyzer.check.command": "clippy"
}
```
Identical setting on both platforms. Once enabled, `needless_clone` and
similar lints show up as yellow squiggles the moment you save, without
running anything manually.

---

## 4. Miri — run from the integrated terminal

Miri doesn't have VS Code UI integration; it's a terminal tool run inside
the same integrated terminal panel:
```bash
rustup component add miri --toolchain nightly
cargo +nightly miri run
```
Both platforms: identical commands. One Windows-specific caveat — Miri's
Windows support has historically been less complete than Linux/macOS for
certain OS-interaction edge cases; for pure DSA code (no file I/O, no
threads touching OS primitives) this doesn't matter, but if you hit a
Miri-specific error that looks environment-related rather than logic-related,
it's worth cross-checking on Linux (e.g. via WSL — see the note at the
bottom) before assuming it's your code.

**Workflow tip:** keep Miri in a second terminal tab (`Ctrl+Shift+5` or the
`+` icon in the terminal panel) so you can re-run it without losing your
`cargo run` / debugger terminal history.

---

## 5. Godbolt — browser-based, but wire it to your editor

Compiler Explorer is a website, not a VS Code extension, but there are two
ways to keep it in your loop without breaking flow:

### Fastest: manual copy-paste
Select the function in VS Code, copy, paste into https://godbolt.org, pick
the Rust compiler on the right pane, toggle `-O0` vs `-O3` in the compiler
flags box. No setup — this is genuinely the path of least resistance for
occasional use.

### Tighter loop: `godbolt.vim`-style extensions
There are community VS Code extensions (search "Godbolt" or "Compiler
Explorer" in the Extensions panel) that send the current selection to
Compiler Explorer and open the result in a browser tab automatically. These
are third-party and change over time, so check current ratings/last-update
date before installing rather than trusting a specific extension ID here.

Both platforms behave identically since this is entirely browser-based —
no OS-specific setup at all.

---

## 6. Windows-specific note: WSL as a Linux fallback

If you ever want the terminal-only Linux workflow (`rust-gdb`, native
Miri support, etc.) without leaving Windows, VS Code's **Remote - WSL**
extension (`ms-vscode-remote.remote-wsl`) lets you open the *same* project
inside a WSL2 Ubuntu environment with one click, reusing the identical
`launch.json`/`settings.json` config from this guide. This isn't required
for anything above — CodeLLDB, rust-analyzer, clippy, and Miri all work
natively on Windows — but it's a useful escape hatch if a tool's Windows
support ever lags behind Linux.

---

## Quick reference

| Tool | Setup needed | Where it runs |
|---|---|---|
| `dbg!()` | None | Integrated terminal (`cargo run`) |
| Step debugger | `CodeLLDB` extension + `launch.json` | Debug panel, `F5` |
| Inline compiler errors | `rust-analyzer` extension (default) | Editor + Problems panel |
| `cargo expand` | `cargo install cargo-expand` | Integrated terminal |
| `clippy` inline | One `settings.json` line | Editor (on save) |
| Miri | `rustup component add miri --toolchain nightly` | Integrated terminal |
| Godbolt | None (browser) or optional extension | Browser tab |
