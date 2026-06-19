# cc-lens

Local-first "AI Coding Wrapped" for Claude Code, Codex, Gemini/Antigravity, Continue, Aider, Cursor, OpenCode, and other agent tools.

`cc-lens` runs as a tiny local web app. It scans the agent history files already on your machine, shows an aggregate, screenshot-friendly summary, and **never uploads anything**. Raw prompts stay on disk — only counts and dates are rendered.

![Dashboard](assets/dashboard.png)

## Quick Start

You do **not** need Go or any toolchain to run cc-lens — every option below downloads a prebuilt, self-contained binary. Pick the one that matches what you already have.

### 1. One-line install — *needs: `curl`/`bash` (preinstalled on macOS & Linux)*

```bash
curl -fsSL https://raw.githubusercontent.com/SemihMutlu07/cc-lens/main/install.sh | bash
```

Downloads the right binary for your OS, adds it to your `PATH`, and launches the dashboard.

### 2. Download a binary directly — *needs: nothing*

The app is a single static binary. Grab the one for your machine, make it executable, and run it:

```bash
# macOS Apple Silicon
curl -L https://github.com/SemihMutlu07/cc-lens/releases/latest/download/cc-lens-darwin-arm64 -o cc-lens
# macOS Intel:  cc-lens-darwin-amd64
# Linux x64:    cc-lens-linux-amd64
# Linux arm64:  cc-lens-linux-arm64
chmod +x cc-lens
./cc-lens
```

Windows: download `cc-lens-windows-amd64.exe` from the [latest release](https://github.com/SemihMutlu07/cc-lens/releases/latest) and run it.

> **macOS Gatekeeper:** the binary is unsigned, so the first launch may be blocked ("cannot be opened"). Allow it once with `xattr -d com.apple.quarantine ./cc-lens`, or right-click the file → **Open**.

### 3. npx — *needs: Node.js ≥ 16*

```bash
npx cclens
```

Then open:

```
http://localhost:8080
```

## Sources

cc-lens scans for many tools and honestly reports what it can see: **loaded** (prompts parsed into the summary) or **detected** (found on disk, parser pending). Tools that aren't installed are simply hidden — no noise for things you never had.

| Tool | Status | Notes |
| --- | --- | --- |
| Claude Code | Loaded | Reads `~/.claude/history.jsonl` |
| Codex CLI | Loaded | Reads `~/.codex/history.jsonl`; Codex project names are not exposed there |
| Gemini / Antigravity | Loaded | Reads `~/.gemini/antigravity-cli/history.jsonl` when present |
| Continue.dev | Loaded | Reads `~/.continue/sessions/*.json` |
| Aider | Loaded | Reads `~/.aider.input.history` |
| Cursor | Detected | Reads `state.vscdb` best-effort; prompts carry no per-prompt timestamps, so they show as a count |
| OpenCode | Loaded | Reads `opencode.db`; decodes each `message` row's JSON `data` to count user prompts (role-aware), dated by `time_created`. Falls back to detected on an unrecognized schema |
| Windsurf / Codeium | Detected | Storage located; chat format not parsed yet |
| Cline / Roo | Detected | VSCode storage located; task history not parsed yet |
| pi | Loaded | Reads `~/.pi/agent/sessions/<project>/*.jsonl`; per-project, one real session per file |
| hermes | Detected | Probed at common paths; local format not confirmed yet |

Adding a tool is one row in `sourceRegistry()` (`parser.go`). Sources we can find but not parse stay **detected** — cc-lens never invents data.

## Privacy & Safety

cc-lens is built to be a closed, local system:

- **Nothing is uploaded.** The app makes no outbound network calls — it only reads local files and serves a page to your browser.
- **Local-only by default.** The server binds to `127.0.0.1`, so nothing on your network can reach it. (Override with `CC_LENS_HOST=0.0.0.0` only if you deliberately want to expose it, e.g. inside a container.)
- **Aggregates only.** Raw prompts are never rendered — only counts, dates, project names, and **estimated** tokens (a `chars / 4` approximation, labeled as such in the UI).
- **Masked by default.** Project names render as fixed, length-independent codenames (e.g. `fuzzling`, `noodling`) — a shared screenshot leaks neither the real name nor its length. One button (Reveal) toggles between the codename and the real name.

## From source

*Needs: Go.* Only required if you want to build from source instead of using a release binary.

```bash
git clone https://github.com/SemihMutlu07/cc-lens.git
cd cc-lens
go run .
```

## Configuration

| Env var | Default | Purpose |
| --- | --- | --- |
| `PORT` | `8080` | Port to serve on |
| `CC_LENS_HOST` | `127.0.0.1` | Bind address (loopback by default) |
| `CC_LENS_HOME` | your home dir | Root to scan for history files |
| `CC_LENS_NO_BROWSER` | unset | Set to `1` to skip auto-opening the browser |

## Built with

Go + Vanilla JS
