# LibreChat + Tauri Desktop App — Ubuntu Setup Guide

Everything runs on one machine: LibreChat's backend (Docker) and the Tauri shell pointed at `localhost:3080`. Ollama runs alongside for local models; cloud providers configured too.

---

## 1. System dependencies for Tauri (Linux/WebKitGTK)

```bash
sudo apt update
sudo apt install -y \
  libwebkit2gtk-4.1-dev \
  libgtk-3-dev \
  libayatana-appindicator3-dev \
  librsvg2-dev \
  build-essential \
  curl \
  wget \
  file \
  patchelf
```

> If `libwebkit2gtk-4.1-dev` isn't found, check your Ubuntu version's package name — 22.04 uses `4.0`, 24.04+ uses `4.1`. Run `apt-cache search webkit2gtk` if unsure.

## 2. Install Rust

```bash
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
source "$HOME/.cargo/env"
rustc --version   # sanity check
```

## 3. Install Node.js

Use nvm so you're not fighting apt's stale Node packages:

```bash
curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.1/install.sh | bash
source ~/.bashrc
nvm install --lts
node --version
npm --version
```

## 4. Install Tauri CLI

```bash
npm install -g @tauri-apps/cli
tauri --version
```

---

## 5. Set up LibreChat (Docker)

```bash
git clone https://github.com/danny-avila/LibreChat.git
cd LibreChat
cp .env.example .env
```

Edit `.env` — key values to set:

```bash
HOST=0.0.0.0
PORT=3080

# Generate real secrets, don't ship the example ones:
# openssl rand -hex 32   (run 4x for the values below)
JWT_SECRET=<generated>
JWT_REFRESH_SECRET=<generated>
CREDS_KEY=<generated 32-byte hex>
CREDS_IV=<generated 16-byte hex>

# Cloud providers — fill in whichever you use
OPENAI_API_KEY=sk-...
ANTHROPIC_API_KEY=sk-ant-...
```

Bring it up:

```bash
docker compose up -d
docker compose ps        # confirm mongodb + api containers are healthy
```

Visit `http://localhost:3080` in a regular browser first to confirm it works and create your local user account (`Sign up`, or `npm run create-user` inside the container if you disable open registration).

---

## 6. Add Ollama for local models

```bash
curl -fsSL https://ollama.com/install.sh | sh
ollama pull llama3.1        # or whichever model you want; swap freely
ollama serve                # usually auto-runs as a systemd service after install — check:
systemctl status ollama
```

Point LibreChat at it — create/edit `librechat.yaml` in the LibreChat repo root (mount it via docker-compose.override.yml if not already mounted):

```yaml
endpoints:
  custom:
    - name: "Ollama"
      apiKey: "ollama"
      baseURL: "http://host.docker.internal:11434/v1"
      models:
        default: ["llama3.1"]
        fetch: true
      titleConvo: true
```

> LibreChat's containers need `host.docker.internal` to reach Ollama running on the host. On Linux this isn't automatic like Docker Desktop — add this to the `api` service in `docker-compose.override.yml`:
> ```yaml
> services:
>   api:
>     extra_hosts:
>       - "host.docker.internal:host-gateway"
> ```

Restart to pick up config: `docker compose down && docker compose up -d`.

---

## 7. Auto-start the backend on login (systemd user service)

```bash
mkdir -p ~/.config/systemd/user
cat > ~/.config/systemd/user/librechat.service << 'EOF'
[Unit]
Description=LibreChat Docker stack
After=docker.service network-online.target

[Service]
Type=oneshot
RemainAfterExit=true
WorkingDirectory=%h/LibreChat
ExecStart=/usr/bin/docker compose up -d
ExecStop=/usr/bin/docker compose down

[Install]
WantedBy=default.target
EOF

systemctl --user daemon-reload
systemctl --user enable --now librechat.service
loginctl enable-linger $USER   # so it starts even without an active login session
```

Verify: `systemctl --user status librechat.service`.

---

## 8. Scaffold the Tauri app

```bash
cd ~
npm create tauri-app@latest librechat-desktop
# When prompted: choose your preferred frontend template (or "vanilla" — we're not
# building real UI, just a window pointed at LibreChat) and npm as package manager.
cd librechat-desktop
```

Edit `src-tauri/tauri.conf.json`:

```jsonc
{
  "productName": "LibreChat",
  "identifier": "com.yourname.librechat-desktop",
  "app": {
    "windows": [
      {
        "title": "LibreChat",
        "url": "http://localhost:3080",
        "width": 1280,
        "height": 800
      }
    ],
    "security": {
      "csp": "default-src 'self' http://localhost:3080; connect-src 'self' http://localhost:3080 ws://localhost:3080; img-src 'self' http://localhost:3080 data:; style-src 'self' 'unsafe-inline' http://localhost:3080"
    }
  }
}
```

Run it in dev mode:

```bash
npm run tauri dev
```

If it loads LibreChat's UI and you can log in and chat, the wiring works.

## 9. Build the production binary

```bash
npm run tauri build
```

Output lands in `src-tauri/target/release/bundle/` — you'll get a `.deb` and/or `.AppImage` for Ubuntu depending on your Tauri bundler config. Install the `.deb` with `sudo dpkg -i` or just run the `.AppImage` directly.

---

## Optional next steps
- **System tray + "minimize to tray"**: add `tauri-plugin-tray` — useful since the app is really just a window, tray icon gives quick show/hide.
- **Global hotkey to summon the window**: `tauri-plugin-global-shortcut`.
- **Health-check splash screen**: have the Rust side poll `http://localhost:3080/health` before showing the window, so you're not staring at a connection-refused page if Docker is still starting up after boot.

Yes, this is doable — and it's actually a pretty natural pairing, since LibreChat already exposes a web UI + REST API, and Tauri is built exactly for wrapping/embedding web content with a native shell. There are two architectures depending on how "native" you want it, and current LibreChat setup is still Node.js API + MongoDB + optional Meilisearch/RAG (Postgres+pgvector)/Redis with MongoDB 8.0.20, PostgreSQL with pgvector 0.8.0, and Meilisearch v1.35.1 as the services used in the Docker Compose stack, which matters for how hard "fully bundled" is.

## Option A — Tauri as a native shell around a running LibreChat instance (recommended)

You keep LibreChat exactly as it is (self-hosted via its own docker-compose, wherever — your homelab, a VPS, or `localhost` if you also run compose on the same machine), and Tauri just becomes the native chrome around it: system tray, native window controls, notifications, global hotkey to summon it, secure OS keychain for the session token, auto-launch on boot, etc.

Practically:

```jsonc
// tauri.conf.json
{
  "app": {
    "windows": [{ "url": "https://librechat.yourdomain.com", "title": "LibreChat" }],
    "security": {
      "csp": "default-src 'self' https://librechat.yourdomain.com; connect-src 'self' https://librechat.yourdomain.com wss://librechat.yourdomain.com"
    }
  }
}
```

Things you'll actually deal with:
- **CSP/allowlist** — Tauri's webview is stricter than a browser tab by default; you need to explicitly allow the LibreChat origin for fetch/websocket (SSE streaming for chat responses relies on this).
- **Auth persistence** — LibreChat uses JWT cookies/localStorage; Tauri's webview persists storage per-app like a normal browser profile, so login sticks across launches. No extra work needed unless you want to move the token into the OS keychain (`tauri-plugin-stronghold` or `keyring`) for extra security.
- **Deep linking / OAuth redirects** — if you use SSO providers (Google/GitHub/OIDC) for login, register a custom URL scheme via `tauri-plugin-deep-link` so the OAuth redirect can hand control back to the app instead of opening a browser tab.
- **Streaming** — chat responses stream over SSE; Tauri's webview (WebView2 on Windows, WKWebView on macOS, WebKitGTK on Linux) all handle SSE/fetch streaming fine, no special handling needed.

This gets you 90% of "desktop app" feel for maybe an hour of config, and you keep LibreChat's backend fully decoupled — upgrade it independently, run it wherever.

## Option B — Fully bundled/offline app (LibreChat backend ships inside the Tauri binary)

Harder, and worth being honest about the friction: LibreChat's backend isn't designed to be embedded. Docker Compose handles all dependencies — MongoDB, MeiliSearch, RAG API, and Vector DB are all included automatically — whereas with npm or Helm you install and configure these services separately. There's no official embedded/serverless mode.

If you still want this, the path is Tauri's **sidecar** feature (bundling external binaries alongside your Rust app) to spawn the stack on first launch:

- Bundle a Node.js runtime + the LibreChat API server as a sidecar binary (pkg/nexe or just ship node_modules + a portable Node build).
- MongoDB is the real pain point — there's no supported "embedded" production MongoDB. Your realistic choices:
  - Bundle the actual `mongod` binary as a second sidecar and spawn it pointed at a local data dir (this is what desktop apps like Mongo-backed Electron apps do — totally workable, just adds ~100MB+ and platform-specific binaries for win/mac/linux, arm64/x64).
  - Point at a free-tier MongoDB Atlas cluster instead (trades "fully offline" for "zero bundling pain").
- Skip Meilisearch/RAG API/Redis unless you need search or file-embedding features — LibreChat runs fine without them for basic chat.
- Use `tauri-plugin-shell`'s sidecar API to spawn `mongod` then the LibreChat API process in sequence on app start, wait for the health check, then point the window at `http://localhost:3080`.

This is genuinely more engineering than Option A for not a huge UX win, unless "single offline installer, no server to manage" is a hard requirement for you.

## My take

Given you already lean toward self-hosting infra (matches your Cilium/Aya background), I'd go **Option A**: run LibreChat's normal docker-compose stack on a box you control, and build the Tauri shell as a thin, well-configured wrapper. You get native app ergonomics without re-engineering LibreChat's deployment model, and you can revisit bundling later if you specifically want a "double-click installer, zero backend" distributable for other people.

Want me to sketch the actual Tauri project scaffold (Rust side: tray + deep-link + keychain token storage) for Option A?

Option A (thin native shell around a normally-deployed LibreChat) is the more reliable choice, and it's not close. Here's the reasoning across the dimensions that actually matter for reliability:

**Failure surface**
- Option A: Tauri app has one job — render a webview and manage OS-level chrome. If something breaks, it's your backend (which you'd operate the same way regardless of Tauri) or a rendering/CSP issue you can debug in minutes.
- Option B: you're now responsible for orchestrating process startup order (mongod → API → health check), handling crashed sidecars, port conflicts, stale lockfiles from unclean shutdowns, and platform-specific binary quirks (arm64 vs x64 `mongod`, macOS notarization/gatekeeper flagging a bundled database binary, Windows Defender false-positiving on a spawned Node binary). Every one of these is a support ticket you'll own.

**Upgrade path**
- Option A: LibreChat's maintainers ship fixes and security patches via their normal Docker images; you `docker compose pull` and restart. Your Tauri shell barely changes.
- Option B: you're now maintaining a fork of their deployment model. Every LibreChat release could change Mongo version requirements, add a new required service, or change env vars — and note that Mac computers with Apple Silicon don't support AVX instructions required by the default MongoDB image, so it crashes on startup unless you override to an older image — the kind of platform-specific landmine you'd now have to detect and handle per-OS inside your own installer, instead of it being a documented one-line override in someone's docker-compose.

**Data integrity**
- Option A: MongoDB runs as a proper long-lived service with normal ops practices (backups, monitoring) on infra you already manage.
- Option B: a locally-spawned `mongod` per user machine, started/stopped alongside a GUI app, is a much easier way to get an unclean shutdown mid-write, especially if the user force-quits the app or the OS suspends it. You'd need to be careful about journaling/fsync settings you don't otherwise have to think about.

**Where Option B actually wins**: if the requirement is "must work fully offline, no server anywhere, single installer for non-technical users." That's a real use case, but it's a product requirement, not a reliability one — and even then I'd point at Atlas free-tier instead of bundling `mongod` to cut the riskiest part of that list.

So: for reliability, self-host LibreChat's stack the way it's meant to run, and treat Tauri purely as a shell. Given your background you probably already have (or can spin up) a box for this, so there's no real cost to Option A's decoupling.

That actually simplifies things — "server" and "client" being the same machine means you get most of Option B's offline/self-contained feel with almost none of its bundling risk, because Docker is just... installed normally on Ubuntu, not smuggled inside your app.

**Recommended setup:**

1. **Run LibreChat's normal docker-compose stack on the laptop itself**, exactly as documented — `git clone`, `.env`, `docker compose up -d`. No sidecar hacks, no bundled `mongod` binary, no cross-platform binary matrix to worry about since you're only targeting one OS/arch.

2. **Point the Tauri window at `http://localhost:3080`** (LibreChat's default port). Same tauri.conf.json approach as before, CSP scoped to localhost instead of a remote domain.

3. **Make the backend start automatically** so the user experience is "launch app, it just works," not "remember to `docker compose up` first." Two clean ways:
   - A systemd user service that runs `docker compose up -d` on login (most reliable — survives crashes, restarts on failure, totally decoupled from the Tauri app's lifecycle).
   - Or have the Tauri app itself shell out `docker compose up -d` on launch via `tauri-plugin-shell`, and poll `localhost:3080/health` before showing the window. Slightly more "self-contained app" feel, but now the app's startup is coupled to Docker's state, which is more moving parts than the systemd route.

   I'd go systemd — it's the boring, reliable choice, and it means the backend is up before you even click the app icon.

4. **One Ubuntu-specific gotcha**: Tauri on Linux uses WebKitGTK, not Chromium, so make sure `libwebkit2gtk-4.1-dev` (or whatever the current package name is on your Ubuntu version) is installed as a build dependency — worth checking Tauri's Linux prerequisites since package names have shifted across WebKitGTK versions.

5. **Gaming laptop bonus**: if that GPU is sitting idle, this is a good setup to add Ollama alongside LibreChat for local model inference instead of (or alongside) API-based providers — LibreChat has native Ollama support, so it's just another entry in your `.env`/`librechat.yaml` model config, no extra plumbing on the Tauri side.

