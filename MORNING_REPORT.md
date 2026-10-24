# Morning report

Written and updated through the night. Nothing here is merged into
`main` -- every commit below is on a branch, waiting for you to open
and review the PR yourself.

**Last updated**: 2026-10-24, early in the night shift. This file is
being updated incrementally as work lands; if a section looks sparse,
that work hasn't happened yet, not that it failed silently.

## Kaggle credentials -- the key does not authenticate

You gave `KAGGLE_USERNAME=furaxdev` and a key starting `KGAT_...`.
I wrote it to `~/.kaggle/kaggle.json` (600 permissions, outside the
repo, never committed -- verified with `gitleaks` and `git status`).

**It does not work.** Tested two ways, neither printing the key value:
- The installed Kaggle CLI (2.2.5) now requires an interactive OAuth
  browser login (`kaggle auth login`) for every command -- there is no
  non-interactive key-based path in this version at all, so even a
  *valid* classic username+key pair wouldn't authenticate through the
  CLI as installed.
- Dropping to the raw REST API directly (`curl -u
  furaxdev:<key> https://www.kaggle.com/api/v1/kernels/list`,
  bypassing the CLI entirely) returned `401 Unauthenticated` --
  so the key itself doesn't validate against Kaggle's API, independent
  of the CLI's OAuth requirement.

**What this means**: all of item 5 (Runyx Core / Runyx Code fine-
tuning on Kaggle) is blocked on a working credential. I wrote
`docs/MODELS_PLAN.md` anyway (the plan doesn't need a live credential
to write), and did not attempt any training run -- there's nothing to
run it against. Nothing was pushed to Hugging Face or Kaggle.

**What you need to do**: generate a fresh Kaggle API token from
https://www.kaggle.com/settings (Account -> API -> Create New Token),
and separately decide how you want this handled given the CLI's new
OAuth requirement -- the options are (a) run `kaggle auth login`
yourself interactively once and hand me the resulting cached
credential file, or (b) I drive the Kaggle REST API directly (not the
CLI) once I have a key that actually authenticates, which sidesteps
the OAuth requirement entirely since the API itself may still accept
key auth on endpoints the CLI's new wrapper doesn't expose that way --
untested, since the current key doesn't authenticate at all to check.

## What's done tonight, by branch and commit

### `ccr-8047f90c-ur2jz8` (the pre-existing TUI/demo branch)

Everything here was actually finished *before* tonight's session
started (see the conversation's earlier turns) and is unchanged
tonight except as noted:
- Demo CI fix (streaming cursor bug), TUI T3 (command approval, pull
  progress, pairing) and T4 (Go launch wiring, `--dump-frame`), and
  the composer redesign were already committed and pushed.
- Flaky CI note: `build-test-lint` failed once tonight
  (`TestSearchRealDuckDuckGo`, `internal/websearch` -- a real-network
  test unrelated to anything in that commit, which only touched
  `internal/approval`/`agent`/`api`/`daemon` + TUI files). Re-ran the
  failed job via the GitHub API; confirmed locally too (`go test
  ./internal/websearch/...` passes repeatedly here). Recorded as a
  known flake, not fixed in code (it's a real DuckDuckGo network call
  by design, inherently flaky in CI) -- **you should decide** whether
  to make that specific test skip-on-network-error or leave it as is;
  I didn't change test behavior without being asked to.

### `feat/runyx-code` (new branch, branched from the above; nothing
merged into `main`)

- `c6527e3` -- `docs/RUNYX_CODE.md` (architecture, tool schemas,
  threat model, C0-C3 staging) + `assets/mascot/` (sprites.json,
  animations.json, source svg/gif) copied in from your zip.
- `bbd9e45` -- `internal/memory` (level-1 memory: USER.md/MEMORY.md/
  per-project MEMORY.md, § separator, 2200-char cap enforced,
  audit.jsonl, secret redaction before write) + `internal/secretscan`
  (regex-based redaction: GitHub/AWS/Slack/OpenAI-shaped tokens, PEM
  key blocks, generic credential assignments). 18 tests.
- `087eae5` -- `internal/sleepinhibit` (anti-sleep: Linux via a held
  `systemd-inhibit` subprocess -- smoke-tested against the real binary
  on this machine; macOS via `caffeinate`; Windows via
  `SetThreadExecutionState`, cross-compile-checked only, not run on
  real hardware; a `Manager` on top with refcounting, 4h max-hold,
  and battery-threshold pause logic). 17 tests. Also hardened
  `.gitignore` (secrets, model weights) and installed `gitleaks` with
  a `.gitleaks.toml` allowlist for this repo's own fake test tokens --
  run before every commit from here on.

All three `feat/runyx-code` commits' CI is green (`ci.yml`, full Go
suite + race detector; this branch doesn't touch `tui/` or `web/` yet
so only the Go job runs).

**Not started yet tonight, in the order the brief gives them**: the
file/command tools (`read_file`/`write_file`/`edit_file`/`run_command`
etc.), the agent loop, GitHub tools, the mascot renderers (web/TUI/
Android), the "New session" screen, Remote Control device scopes,
skills, session search, and the model fine-tuning (blocked on the
credential above). This is a multi-week mission; tonight covers the
foundational, most security-critical pieces of C0 (memory, secrets,
anti-sleep) that everything else builds on, not all of C0.

## Demo / inference readability (item 2) -- checked, found still broken, isolation job running

**You were right to ask this be checked.** I looked at the actual text
in the latest demo capture
(`docs/screenshots/light/03-conversation-response.png` on the
`demo-screenshots` branch, from commit `9a308d9`) and it's still
garbled multilingual gibberish -- the same symptom as the original
AMD-EPYC bug, not fixed by the CPU-kernel-pruning workaround I added
earlier this session. I had previously (wrongly) treated the demo as
fixed once the CI pipeline went green and the screenshot *timing*
issue (cursor never detaching) was resolved -- I never actually read
the reply text in that fix's own commit. That was a mistake; this
report is the correction, not a second claim of "fixed" without proof.

While investigating I found a real bug in the pruning workaround
itself: `Fetcher.Ensure` (`internal/backend/fetch.go`) returns early
("already cached") the moment `llama-server`'s binary path exists on
disk, *before* ever reaching the `pruneToBaselineCPUKernel` call --
so pruning only ever runs on a genuinely fresh extraction. This isn't
necessarily the whole story (a correctly-pruned, cached extraction
should still prune on its *first* population and stay pruned on later
cache hits), but it's one concrete thing that could silently defeat
the workaround under the wrong cache-key conditions, and it's exactly
the kind of flags-and-caching interaction item 2 asked me to isolate
rather than guess at.

**Cannot reproduce locally**: this sandbox is an Intel Xeon host with
no outbound access to `github.com/ggml-org` (same restriction as
`docs/BLOCKED.md` already documents for other fetchers), so the
AMD-EPYC-specific behavior can only be tested on GitHub's own runners.

**What I did**: wrote `.github/workflows/demo-diagnose.yml`
(`workflow_dispatch`-only, doesn't run on every push) -- a dedicated
isolation matrix, per your instructions, testing the axes you named:
CPU-kernel pruning on/off (two *separate* backend extractions, so the
caching bug above can't mask the comparison), the chat template
(`--jinja` on/off), and a second, unrelated model family
(Llama-3.2-1B-Instruct) as a control against the Qwen demo model.
Each of the 5 variants gets a direct `curl` to its own `llama-server`
instance (bypassing Runyx's daemon entirely, to isolate llama.cpp/
model behavior from any Runyx-side bug) with a real completion
request, and the raw response text is written to the job summary for
direct reading.

I triggered this job and am waiting on its result -- see the next
update in this section once it completes. **I have not yet written a
content assertion into `demo.yml` or a `runyx doctor`/`run --selftest`
check** (the second half of item 2): doing that before knowing the
actual root cause risks encoding the wrong fix (e.g. an assertion that
only happens to pass because a flag change papered over the real bug).
That follows once the isolation job's results are in.

## TUI T5/T6 (item 1)

The original TUI mission (earlier in this session) only ever defined
T0 through T4, and T4 is finished (see `TUI_REPORT.md`). There's no
T5/T6 scope on record to "continue" into. Decision (logged in
`docs/DECISIONS.md`): rather than invent arbitrary new milestones,
remaining TUI gaps already tracked honestly in `TUI_REPORT.md` (mouse
selection in Chat, real cross-platform builds, hands-on terminal
testing) stay tracked there, and TUI work for the Code section itself
happens as part of Runyx Code's own C0-C3 staging instead.

## What you should test yourself once a PR exists

(Expanded as more lands tonight.)

- A real Kaggle token that actually authenticates (see above) before
  any fine-tuning work can start at all.
- Everything already listed in `TUI_REPORT.md` and `docs/BLOCKED.md`
  from earlier in this session (real model inference, real GitHub
  token, pairing from a real phone, real OS sleep/wake, Windows/macOS
  binaries on real hardware).

## PR checklist

Branches, not merged, waiting for you:
- `ccr-8047f90c-ur2jz8` -- PR #1 already open (TUI + demo + composer).
- `feat/runyx-code` -- no PR opened; push and open one yourself when
  ready, per your instruction not to merge or open PRs for you.
