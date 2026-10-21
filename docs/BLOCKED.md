# Blocked items

Things not done, with why, so nothing is silently skipped.

## GitHub API access scoped to `furaxdev/runyx` only (this sandbox)

This session's network proxy allows GitHub API access (`api.github.com`)
only for `furaxdev/runyx`. `add_repo` for other repos was not attempted
repeatedly since the task doesn't call for attaching unrelated repos —
but it means these fetchers, while implemented against real public
release APIs and unit-tested for their selection logic, were never
exercised against the real network in this environment:

- `internal/backend.Fetcher` — fetches `llama-server` from
  `ggml-org/llama.cpp` releases. **Impact**: no real llama-server binary
  could be run here, so `runyx run`'s actual chat loop, multi-model
  concurrency (item 5), acceleration flags (item 6), and the vision
  bridge's auxiliary-model path (item 31) are all implemented and unit
  tested but not exercised end-to-end with real inference.
- `internal/voice.Fetcher` — fetches whisper.cpp/Piper from their GitHub
  releases. **Impact**: STT/TTS subprocess plumbing is tested against
  fake binaries (real argument passing, real stdin/stdout, real JSON
  parsing) and real ffmpeg conversion, but no real speech was ever
  transcribed or synthesized here.

**Not actually blocked for a normal user**: outside this sandbox, with
normal internet access, both fetchers hit real, public, officially
documented release APIs and should work as-is. A maintainer with normal
GitHub access should run through `docs/MANUAL_TESTS.md` to confirm.

## No real Android device or emulator

The Android module (`android/`) is a full project (Gradle, manifest,
Compose UI skeleton) intended to compile in CI, but no real device,
emulator, or even an installed Android SDK was used to run it in this
sandbox session. See `docs/MANUAL_TESTS.md`.

## No GPU in this sandbox

Every acceleration path that depends on an actual GPU (CUDA/ROCm/Vulkan
offload, the `BestBackend()` selection beyond "cpu-avx2") is implemented
and its selection *logic* is unit tested with synthetic hardware info,
but never run against real GPU hardware.

## No real microphone / speaker

Push-to-talk and hands-free voice interaction (item 25) need a real audio
input/output device to test the actual user experience (wake word
detection, interruption while speaking). Not available in this sandbox.
