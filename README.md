# Runyx

**Runyx** is a 100% local LLM runner, built to be effortless to install,
pleasant to use, and as fast as raw `llama.cpp`. One binary: CLI, daemon,
and an HTTP server compatible with both the **Ollama** (`/api/*`) and
**OpenAI** (`/v1/*`) APIs, plus an embedded web UI and a companion
Android app.

The product is English-only (UI, CLI, docs) by design — see
`docs/DECISIONS.md`. Voice input still understands whatever language the
user speaks; only the interface itself is English.

### Quick start

```bash
go build -o runyx ./cmd/runyx

./runyx doctor                 # hardware detection
./runyx recommend               # models that actually fit this machine
./runyx pull Qwen/Qwen2.5-0.5B-Instruct-GGUF   # download a model
./runyx run Qwen/Qwen2.5-0.5B-Instruct-GGUF    # chat from the terminal
./runyx serve                  # start the daemon + HTTP API + web UI
```

Once `runyx serve` is running, these are available on
`http://127.0.0.1:11535`:

- Ollama-compatible: `GET /api/tags`, `POST /api/chat`, `POST /api/generate`, …
- OpenAI-compatible: `GET /v1/models`, `POST /v1/chat/completions`, …

See `docs/ARCHITECTURE.md`, `docs/ROADMAP.md` and `FEATURES.md` for the
rest of the roadmap and current status.

### License

MIT — see [LICENSE](LICENSE). Third-party dependency licenses are listed
in [THIRD_PARTY.md](THIRD_PARTY.md).
