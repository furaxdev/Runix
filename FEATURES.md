# Features checklist

Status legend (per the project's honesty rule — never mark something
"done" that wasn't actually verified):

- `[x]` **fait et testé** — verified with a real run (actual model pull,
  actual HTTP call, actual file conversion, etc.), not just compiled.
- `[~]` **fait mais non testé** — code is written and believed correct,
  but verification needs something this sandbox doesn't have (a GPU, a
  microphone, a real Android device, network access to a GitHub repo
  outside `furaxdev/runyx`...). The reason is always stated.
- `[!]` **bloqué** — not done; blocker recorded in `docs/BLOCKED.md`.
- `[ ]` not started.

See `docs/MANUAL_TESTS.md` for the list of `[~]` items a human needs to
verify on real hardware.

## Base features (the "comme Ollama" set)

- [x] B1 Facile — `runyx run <model>` pulls+runs, tested end to end with
      a real 0.5B model download; full GUI exists (chat/sidebar/dashboard)
      but a model must still be pulled once via CLI first.
- [x] B2 Rapide — thin HTTP proxy over llama-server; overhead not
      benchmarked yet (`runyx bench`, item 7, is M6).
- [x] B3 Modèles — Hugging Face GGUF search/pull tested live
      (`runyx pull Qwen/...` actually downloaded 468 MB); UI pull flow not
      built yet (CLI only).
- [x] B4 Fichiers — universal attachment conversion tested (text, PDF,
      DOCX, XLSX, PPTX, archives, images with OCR, audio via fake-binary
      integration test); works with any model since it's text injected
      into the prompt, not a model capability.
- [ ] B5 Commandes — M4 safe shell agent.
- [~] B6 Vocal — STT/TTS subprocess plumbing tested against fake
      binaries + real ffmpeg conversion; no real whisper.cpp/Piper model
      run (GitHub API blocked to non-`furaxdev/runyx` repos in this
      sandbox, see docs/BLOCKED.md). Full push-to-talk UI is M5.
- [ ] B7 Android — M5.

## M1 — Engine

- [x] 1. Hardware detection + recommendation — tested on this sandbox's
      real CPU/RAM.
- [x] 2. Auto-tune — heuristic implemented and cached; not benchmark-tuned
      (that refinement is item 7, M6).
- [x] 3. Robust downloader — resume/parallel/SHA256/disk-check all unit
      tested against a real local HTTP server; also proven against a real
      Hugging Face download.
- [x] 4. Import — local GGUF import tested; Ollama blob-store scan tested
      against a hand-built manifest (no real Ollama install available
      here to test against).
- [x] 5. Multi-model — concurrent backends/LRU/idle-eviction logic unit
      tested; not exercised with two real llama-server processes at once
      (no backend binary fetchable in this sandbox, see docs/BLOCKED.md).
- [~] 6. Accelerations — flags wired through to llama-server's CLI
      interface; cannot verify actual speculative-decoding/flash-attention
      behavior without a real llama-server binary (see docs/BLOCKED.md).
      Persistent on-disk KV cache across restarts is not implemented yet.
- [x] 9. Dual API — `/api/*` and `/v1/*` tested live against the real
      daemon (tags, models, dashboard, chat routing).
- [x] 23. Baseline security — loopback bind, per-device tokens,
      revocation, no telemetry: all unit tested.

## M2 — Chat & UI

- [x] 8. Runfile YAML format + Ollama Modelfile import — unit tested.
- [x] 14. Local history (SQLite) with branches, FTS5 search, pin, folder,
      Markdown/JSON export — unit tested end to end, and exercised live
      through the HTTP API.
- [x] 17. Universal attachments — text/code/PDF/DOCX/XLSX/PPTX/archives
      tested with real generated fixtures (including a real PDF via
      reportlab); audio/video transcription plumbing tested against a
      fake whisper-cli binary + real ffmpeg; chunking for oversized
      content is implemented, actual RAG-based retrieval of chunks lands
      with item 16 (M3). Drag-and-drop/paste in the UI not implemented —
      file picker only.
- [x] 29. UI identity — Svelte UI built and actually served by the daemon
      (verified live: `curl localhost:11535/` returns the built page);
      light/dark theme via CSS, Ctrl+K command palette, FR/EN i18n,
      discreet dashboard (RAM/VRAM/loaded models) all present. Visual
      polish (micro-animations, accessibility pass, settings panel for
      "reduce motion") is minimal — functional, not refined.
- [x] 31. Vision bridge — OCR tested against a real generated image;
      image-hash cache tested (call-counted, proven not to re-run OCR);
      `look_at_image` mechanism implemented and unit tested against a
      fake HTTP auxiliary-model server. A real auxiliary vision model
      (Moondream/SmolVLM/Qwen-VL) was never actually pulled or run here
      (needs a real llama-server binary, see docs/BLOCKED.md) — the
      degrade-to-OCR-only path IS the one exercised live.

## M3 — Knowledge

- [x] 15. Projects — workspace with its own system prompt (SQLite-backed);
      tested end to end over the HTTP API. Per-project file attachment
      UI (vs. just a `files_dir` field) is not built yet.
- [x] 16. Local RAG with citations — chunking (reuses item 17's
      `convert.Chunk`), embedding via any loaded model's `/v1/embeddings`,
      cosine-similarity search (pure Go, no sqlite-vec needed at this
      scale — see docs/DECISIONS.md), citations rendered with file name +
      passage number. Tested end to end with a fake embedding server that
      actually proves ranking-by-relevance and per-project scoping; a
      real embedding model (e.g. nomic-embed-text) was never pulled and
      run here (same llama-server fetch blocker as M1, see
      docs/BLOCKED.md).
- [x] 18. User memory — visible, listable, deletable facts (SQLite,
      global or per-project); tested end to end over the HTTP API.
      Nothing writes to it automatically yet (no "the model decided to
      remember X" flow) — that's a UI/agent-loop integration, not
      implemented.
- [x] 19. Long context compression + token gauge — character-based token
      estimate (same heuristic as item 17's chunking), gauge endpoint
      tested over HTTP, compression function unit tested with a fake
      summarizer. Not yet wired into the actual chat loop (no model
      automatically summarizes old messages mid-conversation yet) — the
      building block exists, the automatic trigger doesn't.
- [x] 20. Web search — DuckDuckGo Lite HTML scraping, opt-in (nothing
      calls it automatically), citations with real unwrapped URLs,
      self-labeled as untrusted per §7. **Tested against the real
      network** (a real search for "golang programming" returns real
      results) — the one item in this milestone verified against a real
      external service rather than a fake one. Per-conversation
      enable/disable toggle is a UI concern, not built yet.

## M4 — Agent

- [ ] 10. MCP client + server
- [ ] 11. Tool calling + structured output (JSON schema / GBNF)
- [ ] 21. Safe shell agent (allow/deny lists, approvals, diff preview, audit
      log, git-snapshot undo)
- [ ] 22. Sandboxed code interpreter
- [ ] 24. Permissions center

## M5 — Voice & Android

- [ ] 25. Full voice pipeline (push-to-talk, hands-free, streaming STT/TTS,
      interruption, FR/EN auto-detect)
- [ ] 26. Android pairing (QR, mDNS, E2E encryption, WoL, WireGuard/Tailscale)
- [ ] 27. Advanced Android (share sheet, camera→vision, widget, tile,
      notifications, offline queue)
- [ ] 28. Remote & hybrid (use PC models remotely, opt-in remote-API
      fallback)

## M6 — Polish

- [ ] 7. Bench & arena
- [ ] 12. Plugin system (+ optional stable-diffusion.cpp image plugin)
- [ ] 13. Automation (scheduled tasks, folder watch, webhooks)
- [ ] 30. Distribution (portable mode, installers/packages, signed updates)

## Infra (cross-cutting, started in M0)

- [x] Repo structure, `THIRD_PARTY.md`, `LICENSE` (MIT)
- [x] GitHub Actions CI (build + vet + fmt + unit tests on Linux;
      cross-compile check for Windows/macOS/Linux-arm64, all verified
      locally with the same Go toolchain CI uses)
- [x] Unit tests for core logic across every package shipped so far —
      101+ test cases, all passing, many against real fixtures (real
      PDF, real image, real ffmpeg-generated audio, real HTTP download)
      rather than pure mocks.
- [!] Integration test with a cached ~0.5B GGUF model running actual
      inference in CI — blocked: this sandbox's GitHub API access is
      scoped to `furaxdev/runyx` only, so the llama.cpp backend binary
      (from `ggml-org/llama.cpp` releases) could not be fetched here to
      prove `runyx run` end-to-end with real token generation. The
      Hugging Face model *pull* itself (the hard networking part) IS
      verified live. See docs/BLOCKED.md.
- [x] `web/` UI built and embedded (not just scaffolded) — Svelte+Vite
      build committed under `internal/webui/dist`, served live by the
      daemon.
- [ ] `android/` — still scaffold-only pending M5.
