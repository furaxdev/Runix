# Runyx v0.1.0

**Runyx** is a 100% local LLM runner: one binary that gives you a CLI, a
daemon, an HTTP server compatible with both the Ollama (`/api/*`) and
OpenAI (`/v1/*`) APIs, an embedded web UI, and a companion Android app.
Nothing leaves your machine without your consent — no account, no
telemetry.

This is the first tagged release.

## Highlights

- **Engine** — hardware detection and model recommendation
  (`runyx doctor` / `recommend` / `tune`), a resumable parallel
  downloader with SHA-256 verification, Hugging Face GGUF search/pull,
  Ollama blob-store import, and a `llama.cpp` backend fetcher with a
  multi-model, VRAM-budgeted process manager (idle eviction, LRU).
- **Daemon & APIs** — loopback-only HTTP server with device-token auth,
  exposing both the Ollama-compatible and OpenAI-compatible APIs.
- **Chat & web UI** — full chat experience (branching/edit, search,
  export, command palette, keyboard shortcuts, drag & drop attachments,
  smooth animations respecting reduced-motion settings).
- **Attachments** — PDF, Word, Excel, PowerPoint, text, code, and
  archive files are converted to text and injected into the prompt, so
  even a small model can work with your documents; images go through
  OCR for models without vision.
- **Knowledge base** — projects with their own instructions, local
  document search with citations, and memory you can clear.
- **Shell agent** — asks for permission before running anything, stays
  inside the chosen folder, and keeps an audit log.
- **Voice & Android** — transcribe/synthesize HTTP endpoints, and a real
  Kotlin/Compose Android app with QR-code pairing, streaming chat, and
  push-to-talk voice.
- **Security hardening ("Secure max" pass)** —
  - Local daemon now rejects DNS-rebinding and cross-origin browser
    `fetch()` attacks against the loopback API (validates `Host` and
    `Origin`, not just "connection came from 127.0.0.1").
  - Constant-time comparison for API tokens.
  - Shell-agent deny-list can no longer be bypassed with path variants
    (`./sudo`, `sudo.exe`, `../sudo`, …).
  - Secrets are redacted from command output and the audit log before
    they're ever written to disk.
  - The cached `llama-server` binary is now pinned by SHA-256 on first
    use (trust-on-first-use) and re-verified on every start, so a
    tampered cache entry gets detected and re-downloaded instead of
    silently executed.
  - Office-document converters (docx/xlsx/pptx) now bound decompressed
    member size, closing a zip-bomb / decompression-bomb path for
    crafted attachments.
  - Chat history database and the shell-agent audit log are now
    created with `0600` permissions (previously world-readable).

See `docs/DECISIONS.md` and `FEATURES.md` for the full, honestly-tracked
status of every feature (including what's `[~]` untested-but-implemented
or `[!]` blocked, and why).

## Installing

Download the archive for your platform below, extract it, and run the
binary directly — no installer needed.

```bash
# Linux / macOS
tar -xzf runyx-<platform>.tar.gz
./runyx doctor
./runyx serve
```

```powershell
# Windows (PowerShell)
Expand-Archive runyx-windows-<arch>.zip -DestinationPath .
.\runyx.exe doctor
.\runyx.exe serve
```

Verify your download against `SHA256SUMS.txt`:

```bash
sha256sum -c SHA256SUMS.txt --ignore-missing
```

## Assets in this release

| File | Platform |
| --- | --- |
| `runyx-linux-amd64.tar.gz` | Linux x86_64 |
| `runyx-linux-arm64.tar.gz` | Linux ARM64 |
| `runyx-darwin-amd64.tar.gz` | macOS Intel |
| `runyx-darwin-arm64.tar.gz` | macOS Apple Silicon |
| `runyx-windows-amd64.zip` | Windows x86_64 |
| `runyx-windows-arm64.zip` | Windows ARM64 |
| `SHA256SUMS.txt` | checksums for all of the above |

All binaries are statically linked (`CGO_ENABLED=0`), built from
commit `4335a88`.

## License

MIT — see `LICENSE`. Third-party dependency licenses are listed in
`THIRD_PARTY.md`.
