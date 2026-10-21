# Usage (M0 + M1)

## Build

```bash
go build -o runyx ./cmd/runyx
```

## Hardware detection & recommendations

```bash
./runyx doctor       # CPU/RAM/GPU/VRAM report + best backend to use
./runyx recommend    # models from the default catalog that fit this machine
```

## Models

```bash
./runyx pull <owner/repo>[:file.gguf]   # download from Hugging Face
./runyx list                            # installed models
./runyx show <model>                    # size, source, tuning
./runyx tune <model>                    # (re)compute and cache tuning params
./runyx rm <model>                      # remove an installed model
```

`pull` also works with models already on disk from Ollama: point
`OLLAMA_MODELS` (or the default `~/.ollama/models`) and a future `runyx
import --ollama` scan will register them without re-downloading — the
scanning logic is in `internal/modelstore.ScanOllamaModels` today; wiring
it to a CLI subcommand is a short follow-up.

## Chat

```bash
./runyx run <model>
```

Pulls the model if it isn't installed, loads `llama-server` with
hardware-tuned parameters, and opens an interactive chat loop. Type
`/exit` to quit.

## Daemon + APIs

```bash
./runyx serve
```

Starts the HTTP server on `127.0.0.1:11535` (configurable in
`~/.config/runyx/config.toml` on Linux, the OS-equivalent elsewhere):

- **Ollama-compatible**: `GET /api/tags`, `POST /api/show`, `DELETE
  /api/delete`, `POST /api/generate`, `POST /api/chat`.
- **OpenAI-compatible**: `GET /v1/models`, `POST /v1/chat/completions`,
  `POST /v1/completions`, `POST /v1/embeddings`.

Example with `curl`:

```bash
curl http://127.0.0.1:11535/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{"model":"<model-name-from-runyx-list>","messages":[{"role":"user","content":"Bonjour !"}]}'
```

Requests from `127.0.0.1`/`::1` are allowed without a token by default.
Reaching the API from another machine (an explicit opt-in — change
`listen_addr` in the config) requires a device token; issuing/revoking
tokens via the CLI is tracked as a short follow-up (the `daemon.TokenStore`
API already supports it).

## Known limitations in this milestone

- `runyx pull` always re-downloads rather than consulting mirrors; mirror
  fallback is a follow-up.
- The llama.cpp backend binary is fetched from GitHub releases on first
  use; if that fetch fails (no network, GitHub unreachable), `runyx run`
  / `runyx serve` report a clear error rather than silently retrying.
- `/api/pull` (server-side pull over HTTP) only confirms models already
  installed via the CLI; true server-side pulling is deferred to M2 when
  the web UI needs it.
