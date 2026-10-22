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

## 2026-10-21 — Pure-Go cosine similarity instead of sqlite-vec for RAG (item 16)

The brief suggests `sqlite-vec`, but it's a loadable SQLite extension
(compiled C), which the pure-Go `modernc.org/sqlite` driver can't load
without cgo — the same reasoning that picked that driver in the first
place (see the entry above) applies here too. At the scale a local,
single-user RAG index actually reaches (a few thousand chunks per
project, not millions), a linear scan computing cosine similarity in Go
over all stored embedding vectors is well under a millisecond and avoids
the dependency entirely. Revisit only if a project's index grows large
enough (tens of thousands+ chunks) that linear scan becomes a visible
delay — an in-memory approximate index (e.g. HNSW) would be the next
step, not pulling in cgo.

## 2026-10-21 — Web search uses `lite.duckduckgo.com/lite/`, not
`html.duckduckgo.com/html/`

The brief mentions either SearXNG or DuckDuckGo. `html.duckduckgo.com`
was tried first and is unreachable from this sandbox's egress proxy
(connection resets / TLS handshake timeouts on every attempt);
`lite.duckduckgo.com/lite/` worked and needed only a GET with a `q`
query param (vs. the heavier `html/` endpoint's POST form), confirmed
against the real network. No API key, no tracking parameters added.
SearXNG wasn't wired up — it needs a running instance (public ones are
rate-limited and change availability over time; self-hosting is out of
scope for M3) — but `internal/websearch.Client` is small enough to swap
or extend with a second backend later if a user wants to point it at
their own SearXNG.

## 2026-10-21 — Shell agent's "always" approval scope is in-memory only
(not yet persisted)

`agent.Agent.alwaysApprovals` lives only for the process's lifetime.
Persisting it (to the SQLite history DB, keyed by reason + maybe
work-dir) is the obvious next step, but doing it safely needs a bit more
design than this session had room for: what happens when the *reason*
text is similar-but-not-identical across requests, whether approvals
should be scoped per-project, and how a user reviews/revokes persisted
"always" grants (the permissions center, item 24, is the natural home
for that UI once it exists). Tracked as a near-term follow-up, not
forgotten.

## 2026-10-21 — Android QR pairing decodes a captured photo, not a live
camera scan

A live scanning UI needs CameraX (a `PreviewView` + an `ImageAnalysis`
use case) wired up correctly, which is meaningfully more surface area
and more places to get wrong than a one-shot photo. For a first pairing
flow, the standard `ACTION_IMAGE_CAPTURE`-style photo picker
(`TakePicturePreview` in Compose) plus zxing's pure-Java decoder gets
the same outcome — the user photographs the PC's on-screen QR code —
with far less code and no extra heavyweight dependency. Revisit with
CameraX once the pairing flow needs to feel instant rather than
"take a photo, wait a second."

## 2026-10-21 — WireGuard/Tailscale is "bring your own VPN", not bundled

Item 28 mentions WireGuard/Tailscale for remote access. Both are
substantial pieces of software (WireGuard needs a kernel module or
userspace implementation and key management; Tailscale is a whole
separate daemon and account system) that don't belong vendored into
Runyx. The realistic integration is: Runyx binds normally on the LAN or
over whatever VPN interface the user already has running, and pairing
works the same either way since it's just an IP + port + token. No
code change was needed for this to already work; what's explicitly NOT
done is any Runyx-side WireGuard/Tailscale setup automation.

## 2026-10-21 — No real Ed25519 release key embedded in `internal/update`

`update.PublicKeyHex` is deliberately left empty rather than filled
with a placeholder/test key. A "signed updates" feature that silently
verifies against a fake key would look safe while verifying nothing —
worse than clearly refusing to apply updates until a maintainer
generates a real key pair, keeps the private half secret, and embeds
the public half at release-build time (e.g. via `-ldflags
"-X github.com/furaxdev/runyx/internal/update.PublicKeyHex=<hex>"`).
`runyx update` today checks for and reports a newer release, and tells
the user exactly why it won't apply it automatically.

## 2026-10-21 — `runyx` talks to `runyxd` over plain HTTP on 127.0.0.1, no
Unix socket

Simpler cross-platform story (Windows has no Unix sockets without extra
code), and the loopback-only bind plus a per-install token already satisfies
the "local only by default" security requirement. Revisit if we need to
support multiple OS users on one machine safely.

## 2026-10-22 — Android cleartext restricted to LAN/.local in code, not XML

Item 23 (TLS with certificate pinning) is still not implemented — Runyx
only ever talks plain `http://` between the Android app and a paired PC.
Blanket `android:usesCleartextTraffic="true"` was replaced with an
`android:networkSecurityConfig` pointing at
`android/app/src/main/res/xml/network_security_config.xml`, but Android's
`<domain>` entries only support exact hostnames/IPs and DNS-suffix
matching (`includeSubdomains`) — there is no CIDR/subnet syntax, so "allow
cleartext to 10.0.0.0/8 / 172.16.0.0/12 / 192.168.0.0/16" can't be
expressed declaratively for arbitrary LAN IPs the way it can for a named
domain. The actual restriction is enforced in code instead:
`Pairing.kt`'s `isPrivateOrLocalHost()` refuses to pair with (and
therefore the app never connects cleartext to) any host outside those
private IPv4 ranges, loopback, or a `.local` name — covered by unit
tests in `PairingTest.kt`. **TLS with certificate pinning (item 23)
remains tracked future work**, now also listed in `docs/ROADMAP.md`.

## 2026-10-22 — UI and docs in English only

The web UI, CLI (help/messages/errors/prompts/progress), Android app,
API error strings, logs, README and `docs/` are English-only. The
previous bilingual FR/EN `web/src/lib/i18n.ts` dict and in-app language
toggle were removed, but the **mechanism** is kept exactly as it was (a
`Lang` union type + one string table per locale) so adding a locale
later is a new union member and a new dict entry, not a rewrite. This
does **not** touch voice *recognition* (item 25): the speech-to-text
model still understands whatever language the user actually speaks —
only the interface text itself is English. `RUNYX_PROMPT.md` (the
original French spec) is kept as the historical record of what was
asked, amended in place at §1 and items 25/29 to note this change
rather than rewritten.

## 2026-10-22 — UI redesign: sidebar + home + model registry + icon set

Reworked the web UI's layout and navigation (collapsible sidebar with a
Chat/Models/Agent tab selector, recents grouped by day with search, a
device status card, a centered-composer home screen with a greeting and
shortcuts) and replaced the old "type a Hugging Face repo name" pull flow
with a real, curated model catalog.

- **Model registry** (`models/registry.json`, embedded via
  `internal/registry`): 27 hand-picked models spanning ~0.5B-72B,
  including vision and code-specialized models, each verified against the
  real Hugging Face API (repo exists, file exists, real byte size) —
  `cmd/verify-registry` does this and runs in CI
  (`.github/workflows/ci.yml`'s `verify-registry` job) so a renamed or
  removed upstream file fails CI instead of silently going stale. No
  model was invented; every entry's description reflects that model's
  actual public model card.
- **Download pause/resume**: `POST /api/runyx/models/pull` streams NDJSON
  progress over the request body. "Pause" is the client aborting the
  fetch — the request context cancels, `internal/downloader` stops
  mid-chunk, and the partial `.part` + resume-state sidecar survive on
  disk exactly as they do for CLI `runyx pull`. "Resume" is calling the
  same endpoint again; `Download()` already knows how to pick the saved
  chunk state back up. No new resume mechanism was built — this reuses
  the downloader's existing resumable-download support end to end.
- **Icon set**: lucide-svelte (ISC) on the web UI, Jetpack Compose's
  `material-icons-extended` (Apache-2.0) on Android — one stroke
  style/weight each, no emoji anywhere in the product (see the
  `scripts/check-no-emoji.sh` CI gate). Both are listed in
  `THIRD_PARTY.md`.
- **Logo**: an original simple mark (three offset bars suggesting
  motion), not derived from any other product's icon or logo.
- **"Agent" page**: split out of the old combined Settings page — shell
  agent status, vision/voice/web-search flags, and paired-device
  management (with a "Pair a device" entry point into the existing
  pairing flow) now live there; Settings is appearance-only (theme,
  reduce motion).
