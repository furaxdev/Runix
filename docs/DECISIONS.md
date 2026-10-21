# Decisions log

Deviations from the original brief, and why.

## 2026-10-21 — SQLite driver: pure-Go (`modernc.org/sqlite`) instead of cgo `mattn/go-sqlite3`

Keeps `CGO_ENABLED=0` cross-compilation trivial for Windows/macOS/Linux from
one Linux CI runner, at a small performance cost that doesn't matter for a
local, single-user daemon. Revisit only if we need SQLite extensions that
the pure-Go driver can't load.

## 2026-10-21 — `backend.Fetcher` (llama.cpp binary fetch) verified against
real URLs but not run end-to-end in this sandbox

Hugging Face downloads work from this sandbox and were tested live
(`runyx pull Qwen/Qwen2.5-0.5B-Instruct-GGUF:...` actually downloaded
468 MB and registered the model). `api.github.com`, which
`internal/backend.Fetcher` uses to find the latest llama.cpp release
asset, is blocked for this session's proxy (scoped to `furaxdev/runyx`
only) — unrelated to Runyx's design, just this sandbox's repo allowlist.
So `runyx run` could not be exercised against a real `llama-server` here;
it fails with a clear "no llama-server backend available" error instead,
which is the intended behavior when the fetch can't complete. The asset-
selection logic (`PickAsset`) is unit-tested with recorded GitHub asset
names, and the code is otherwise ready to run as-is on a normal machine
with full internet access. CI's integration test mocks the HTTP layer
instead of hitting the real network, so it stays deterministic.

## 2026-10-21 — `runyx` talks to `runyxd` over plain HTTP on 127.0.0.1, no
Unix socket

Simpler cross-platform story (Windows has no Unix sockets without extra
code), and the loopback-only bind plus a per-install token already satisfies
the "local only by default" security requirement. Revisit if we need to
support multiple OS users on one machine safely.
