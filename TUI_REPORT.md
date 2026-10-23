# Runyx TUI -- status report

A from-scratch terminal UI for Runyx (TypeScript, no Ink/React/blessed),
talking to the real Go daemon over HTTP+SSE. This report is updated as
milestones land; it is not a promise of what will eventually exist --
only what is true right now.

## What works

- **Rendering engine (T0)**: double-buffered screen, cell-diff repaint,
  ANSI/SGR output, color-level and terminal-theme (light/dark)
  detection, full input parsing (keys, mouse, bracketed paste, focus),
  a dirty-flag render loop (coalesced to ~30fps while streaming).
- **Layout + widgets (T1)**: single-axis flex layout, virtualized
  scrolling, box/button/list/progressbar/spinner/text/textinput
  widgets, a mouse-hit-testing zone registry.
- **Markdown + selection + clipboard (T2)**: a markdown renderer
  producing styled, wrapped `RenderedLine[]` with source-span tracking;
  content-anchored selection (word/line/select-all, soft-wrap-aware
  copy, "copy as markdown", per-code-block copy) that survives
  scrolling and streaming; OSC 52 clipboard (tmux-DCS-wrapped) with a
  CLI-tool fallback.
- **Daemon client (T3 part 1)**: `src/api/config.ts` resolves
  `listen_addr` from the same `config.toml` the Go daemon reads;
  `src/api/daemon.ts` probes `/healthz` and spawns `runyx serve`
  (detached) if nothing answers, polling until healthy or a timeout;
  `src/api/client.ts` is a typed client for
  `/api/tags`, `/api/runyx/{conversations,dashboard,permissions,registry}`,
  and `/v1/chat/completions` (streaming), with field casing matching
  the Go structs exactly.
- **Chat screen (T3 part 1)**: a real, working conversation against
  the real daemon -- history persisted via
  `/api/runyx/conversations`, token-by-token streaming, Ctrl+X to
  abort. Deliberately mirrors `web/src/lib/Chat.svelte`'s fix for the
  "streaming cursor never detaches" demo-CI bug (see
  `docs/DECISIONS.md`): no `max_tokens` cap on a real conversation,
  and a 20s stall watchdog that shows "model is slow or stalled"
  instead of a spinner that can never resolve.
- **Dashboard / Models / Permissions screens + Ctrl+K palette (T3 part
  2)**: read-only views of `/api/runyx/dashboard` (loaded models,
  RAM/VRAM, backend), `/api/runyx/registry` (install state + hardware
  fit per model), and `/api/runyx/permissions` (shell-agent scope,
  vision/voice/web-search flags, paired devices -- device revocation
  is wired, Enter on a device sends the `DELETE`). `src/app.ts` is the
  app shell: Chat is the default screen; leaving any screen (Esc/q)
  opens the Ctrl+K-style palette to jump to another; Ctrl+C quits.

## What is tested

- 153 unit tests (`bun test`, see `tui/test/*.test.ts`): terminal
  buffer/diff, input parsing, layout/scroll math, widgets, markdown
  rendering, selection/clipboard (T0-T2), plus the client's SSE
  termination handling (resolves on `[DONE]` and on plain connection
  close, rejects on abort/non-OK) and `config.toml` parsing (T3).
- `tsc --noEmit` is clean across the whole `tui/` tree.
- The compiled binary was smoke-tested against a real `runyx serve`:
  daemon-autostart from cold (no daemon running, `runyx` found on
  `$PATH`) reaching a healthy `/healthz`, and the "no model installed"
  error path. Not exercised end-to-end with a real model loaded and a
  live terminal (no TTY available in this environment to drive the
  interactive Chat/Dashboard/Models/Permissions screens visually) --
  this is the biggest real gap in verification and should be the
  first thing a human checks before trusting this report.

## What is not built yet

- **Pairing screen + QR code** (`/api/runyx/pairing/qrcode`): not
  started.
- **Command-approval UI** for the shell agent
  (`/api/runyx/agent/execute` + its undo): not started -- the
  Permissions screen only shows the agent's *current* scope, not a
  live approval prompt for a pending command.
- **Model pull / download progress**: the Models screen is read-only;
  `/api/runyx/models/pull`'s streamed NDJSON progress is not wired to
  any UI (no pull button, no progress bar, even though
  `widgets/progressbar.ts` exists from T1).
- **Mouse + text selection inside the Chat transcript**: T2's
  selection/clipboard engine is built and unit-tested against
  synthetic `RenderedLine[]`, but the Chat screen does not yet wire
  mouse drag events to it (no click-to-select, no double/triple-click,
  no Copy button) -- it currently only scrolls with Up/Down.
- **T4 (packaging)**: no `--dump-frame` CLI flag (though
  `screenToSVG` exists from T0), no Go-side `runyx`/`runyx tui`
  sibling-executable launch wiring (documented as a decision in
  `docs/DECISIONS.md` but not implemented), no cross-platform release
  builds beyond the existing `bun build --compile` targets already in
  `package.json`, no terminal-compatibility hands-on verification (the
  matrix that exists is protocol-support reasoning, not tested against
  real terminal emulators), no `THIRD_PARTY.md` entry for the TUI's
  dependencies (it has none at runtime, but this should still be
  recorded explicitly).

## Captures

None yet -- `--dump-frame` (T4) is what would produce these, and it
isn't built. The Go daemon's own demo (`demo.yml`, screenshots on the
`demo-screenshots` branch) covers the *web* UI only, not this TUI.
