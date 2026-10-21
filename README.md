# Runyx

[FR](#fr) · [EN](#en)

<a name="fr"></a>
## Français

**Runyx** est un runner de LLM 100 % local, pensé pour être facile à
installer, agréable à utiliser et aussi rapide que `llama.cpp` brut.
Un seul binaire : CLI, daemon et serveur HTTP compatible **Ollama**
(`/api/*`) et **OpenAI** (`/v1/*`).

### Démarrage rapide

```bash
go build -o runyx ./cmd/runyx

./runyx doctor                 # détection matérielle
./runyx recommend               # modèles qui rentrent sur cette machine
./runyx pull Qwen/Qwen2.5-0.5B-Instruct-GGUF   # télécharge un modèle
./runyx run Qwen/Qwen2.5-0.5B-Instruct-GGUF    # chat en ligne de commande
./runyx serve                  # démarre le daemon + API en arrière-plan
```

Une fois `runyx serve` lancé, les API suivantes sont disponibles sur
`http://127.0.0.1:11535` :

- Compatible Ollama : `GET /api/tags`, `POST /api/chat`, `POST /api/generate`, …
- Compatible OpenAI : `GET /v1/models`, `POST /v1/chat/completions`, …

Voir `docs/ARCHITECTURE.md`, `docs/ROADMAP.md` et `FEATURES.md` pour le
reste de la feuille de route.

### État actuel (M0 + M1)

Le moteur (M1) est fonctionnel de bout en bout : détection matérielle,
recommandation, téléchargement/import de modèles, gestion multi-modèles
avec éviction par inactivité, et les deux API. Le reste (chat & UI,
connaissances, agent, voix/Android, finition) est planifié dans
`FEATURES.md`.

---

<a name="en"></a>
## English

**Runyx** is a 100% local LLM runner, built to be effortless to install,
pleasant to use, and as fast as raw `llama.cpp`. One binary: CLI, daemon,
and an HTTP server compatible with both the **Ollama** (`/api/*`) and
**OpenAI** (`/v1/*`) APIs.

### Quick start

```bash
go build -o runyx ./cmd/runyx

./runyx doctor                 # hardware detection
./runyx recommend               # models that actually fit this machine
./runyx pull Qwen/Qwen2.5-0.5B-Instruct-GGUF   # download a model
./runyx run Qwen/Qwen2.5-0.5B-Instruct-GGUF    # chat from the terminal
./runyx serve                  # start the daemon + HTTP API in the background
```

Once `runyx serve` is running, these are available on
`http://127.0.0.1:11535`:

- Ollama-compatible: `GET /api/tags`, `POST /api/chat`, `POST /api/generate`, …
- OpenAI-compatible: `GET /v1/models`, `POST /v1/chat/completions`, …

See `docs/ARCHITECTURE.md`, `docs/ROADMAP.md` and `FEATURES.md` for the
rest of the roadmap.

### Current status (M0 + M1)

The engine (M1) works end to end: hardware detection, recommendations,
model download/import, multi-model management with idle eviction, and
both APIs. Everything else (chat & UI, knowledge, agent, voice/Android,
polish) is tracked in `FEATURES.md`.

### License

MIT — see [LICENSE](LICENSE). Third-party dependency licenses are listed
in [THIRD_PARTY.md](THIRD_PARTY.md).
