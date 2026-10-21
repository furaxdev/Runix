# Features checklist

Status legend: `[ ]` todo · `[~]` in progress · `[x]` done

## Base features (the "comme Ollama" set)

- [x] B1 Facile — `runyx run <model>` pulls+runs; GUI still pending (M2).
- [x] B2 Rapide — thin HTTP proxy over llama-server, fast daemon start.
- [x] B3 Modèles — Hugging Face GGUF search/pull (CLI; UI pending M2).
- [ ] B4 Fichiers — attach files to a conversation with **any model**
      (universal conversion, see items 17/31) — M2/M3.
- [ ] B5 Commandes — M4 safe shell agent.
- [ ] B6 Vocal — M5.
- [ ] B7 Android — M5.

## M1 — Engine

- [x] 1. Hardware detection + recommendation (`doctor`, `recommend`)
- [x] 2. Auto-tune (`tune`) — basic heuristic + per-model cache
- [x] 3. Robust downloader — resume, parallel, SHA256, disk check, progress
- [x] 4. Import — local GGUF import + Ollama blob store reuse
- [x] 5. Multi-model — concurrent backends, idle keep-alive, LRU eviction
- [x] 6. Accelerations — flags wired through to llama-server (speculative
      decoding, flash attention, continuous batching); persistent KV cache
      path reserved, not yet auto-restored across process restarts
- [x] 9. Dual API — `/api/*` (Ollama) + `/v1/*` (OpenAI)
- [x] 23. Baseline security — loopback-only default bind, per-device token,
      revocation, no telemetry

## M2 — Chat & UI

- [ ] 8. Runfile format + Ollama Modelfile import
- [ ] 14. Local history, full-text search, folders, pin, export, branches
- [ ] 17. Universal attachments — **any model** can receive **any file**:
      Runyx converts everything to model-ready text first (PDF/DOCX/XLSX/
      PPTX/code/archives, keeping structure — pages, headings, tables as
      Markdown; audio via local transcription; video via keyframes +
      transcription). Content exceeding context is chunked and
      retrieved (reuses the RAG pipeline, item 16) instead of truncated.
      Drag-and-drop, clipboard paste, and a clickable per-file chip
      showing exactly what the model received.
- [ ] 29. UI identity, themes, command palette, i18n, dashboard
- [ ] 31. Vision bridge for non-vision models — when the selected model
      can't read images (detected via GGUF metadata / no multimodal
      projector), Runyx routes the image to a small auxiliary vision
      model + local OCR, then hands the text model a structured result
      (description, read text, extracted tables/charts). Auxiliary model
      is hardware-recommended, downloaded on demand, user-swappable
      (maintain a registry of recommended small vision models — e.g.
      Qwen-VL, SmolVLM, Moondream). Requirements: transparent "image
      analyzed by \<model\>" chip with user-editable description before
      send; a `look_at_image(question)` tool so the text model can ask
      follow-ups instead of relying on one frozen description; cache by
      image hash; graceful degradation to OCR-only with a clear
      limitations warning when no vision model is installed, and a
      no-op when the chosen model already sees natively; OCR'd text is
      untreated input (§7) and can never trigger a tool on its own;
      works from the Android app (photo, screenshot) too.

## M3 — Knowledge

- [ ] 15. Projects
- [ ] 16. Local RAG with citations
- [ ] 18. User memory (visible, editable)
- [ ] 19. Long context compression + token gauge
- [ ] 20. Web search tool with citations

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
- [x] GitHub Actions CI (build + vet + unit tests on Linux; cross-compile
      check for Windows/macOS)
- [x] Unit tests for core logic (hwdetect scoring, config, modelstore,
      downloader resume logic)
- [ ] Integration test with a cached ~0.5B GGUF model in CI (requires
      network egress in CI runners; wired up but not yet verified against
      a live Hugging Face download from this sandbox)
- [x] `web/` and `android/` scaffolding present (UI itself is M2/M5 work)
