# Roadmap

See `FEATURES.md` for the full checklist. Milestones:

- **M0 — Foundations**: repo, CI, structure, docs, `runyx version`, daemon
  bootstrap, config. **Status: done.**
- **M1 — Engine**: llama.cpp backend manager, `pull`/`run`/`list`/`rm`/`show`,
  Ollama + OpenAI API, `doctor`/`recommend`/`tune`, downloader, import,
  multi-model, acceleration flags, baseline security. **Status: in
  progress (this session).**
- **M2 — Chat & UI**: web UI, history + branches, universal attachments
  (any file, any model — item 17), Runfile, design system, dashboard,
  vision bridge for non-vision models (item 31).
- **M3 — Knowledge**: projects, local RAG, user memory, long-context
  compression, web search tool.
- **M4 — Agent**: tool calling, MCP client + server, safe shell agent,
  code interpreter, permissions center.
- **M5 — Voice & Android**: voice pipeline, Android pairing, Android app,
  remote/hybrid access.
- **M6 — Polish**: plugins, automation, bench & arena, packaging and
  signed updates.

A milestone is done when: it builds on Linux/Windows/macOS, tests pass,
`FEATURES.md` is current, and a short usage guide exists under `docs/`.
This session's cloud sandbox is Linux-only, so Windows/macOS builds are
verified via `GOOS`/`GOARCH` cross-compilation in CI rather than native
runs; native testing on those platforms is a follow-up for maintainers
with access to that hardware.
