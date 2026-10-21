# Third-party dependencies

Runyx itself is MIT-licensed. This lists direct Go module dependencies
and their licenses, checked for compatibility with MIT distribution.

| Module | License | Used for |
|---|---|---|
| [github.com/BurntSushi/toml](https://github.com/BurntSushi/toml) | MIT | Config file (`config.toml`) parsing |
| [golang.org/x/sync](https://pkg.go.dev/golang.org/x/sync) | BSD-3-Clause | `errgroup` for parallel chunked downloads |

No code from Ollama was copied or consulted — the `/api/*` compatibility
layer in `internal/api` is implemented purely from Ollama's public API
documentation (https://github.com/ollama/ollama/blob/main/docs/api.md).

llama.cpp (`llama-server`) is run as an external subprocess, downloaded
from its official GitHub releases (https://github.com/ggml-org/llama.cpp,
MIT license); Runyx does not statically or dynamically link against it.

This file will grow as later milestones add dependencies (Svelte/Vite for
`web/`, whisper.cpp/Piper bindings for voice, etc.) — update it in the
same commit that adds a new dependency.
