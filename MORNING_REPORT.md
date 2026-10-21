# Rapport du matin — session « mode nuit »

Bonjour Furax 👋 (promis, pas d'emoji ailleurs que dans ce salut).

Tous les jalons **M1 à M6** sont terminés, poussés sur
`ccr-8047f90c-ur2jz8`, et tagués localement (`m1-done` → `m6-done` ;
voir la note sur les tags ci-dessous). Ce n'est pas un prototype : les
161 tests Go passent, 26 paquets testés, le build passe sur
Linux/Windows/macOS (amd64+arm64) et **l'application Android compile
réellement en un vrai APK** — j'ai installé le SDK Android dans ce
sandbox spécifiquement pour vérifier ça plutôt que de l'affirmer sans
preuve.

## Ce qui marche (vérifié, pas supposé)

- **Moteur (M1)** : `runyx doctor`/`recommend`/`tune`, téléchargement
  réel depuis Hugging Face (testé en direct : 468 Mo téléchargés),
  import de modèles Ollama, API Ollama (`/api/*`) et OpenAI (`/v1/*`)
  toutes deux testées sur le vrai daemon.
- **UI (M2)** : interface Svelte buildée et **embarquée dans le
  binaire** (servie en direct, vérifié avec `curl`), thèmes clair/
  sombre, palette Ctrl+K, i18n FR/EN, dashboard, historique SQLite avec
  branches, pièces jointes universelles (PDF/DOCX/XLSX/PPTX/archives
  testés avec de vrais fichiers générés), pont vision (OCR réel testé
  sur une image générée).
- **Connaissances (M3)** : RAG local (indexation + recherche
  cosinus), mémoire utilisateur, jauge de contexte, **recherche web
  réelle** (requête DuckDuckGo en direct, pas un faux serveur).
- **Agent (M4)** : MCP client+serveur (testé avec un vrai
  sous-processus), agent shell sûr (vraies commandes shell, vrai
  dépôt git, vrai undo), interpréteur Python (vrai Python 3 exécuté,
  vrai timeout qui se déclenche), centre de permissions.
- **Voix & Android (M5)** : endpoints voix (dégradation propre testée
  en direct), appairage QR (vrai PNG généré par le serveur), mDNS,
  Wake-on-LAN (vrai paquet UDP vérifié octet par octet), **app Android
  complète** (appairage par photo QR, chat avec streaming, micro
  push-to-talk, partage) qui compile en un vrai APK de 9,4 Mo et dont
  les 4 tests JUnit passent réellement.
- **Finition (M6)** : bench (mesure réelle de timing SSE), arène
  multi-modèles, système de plugins (les plugins SONT des serveurs MCP
  — testé avec un vrai sous-processus `runyx mcp-serve`), automatisation
  (vrai cron qui se déclenche, vraie surveillance de dossier, vrai
  webhook HMAC vérifié), mises à jour signées Ed25519 (vraies paires de
  clés générées et testées), mode portable.

## Comment lancer

```bash
go build -o runyx ./cmd/runyx
./runyx doctor
./runyx pull Qwen/Qwen2.5-0.5B-Instruct-GGUF
./runyx run Qwen/Qwen2.5-0.5B-Instruct-GGUF   # ou ./runyx serve pour l'API + l'UI web
```

Pour l'app Android :
```bash
cd android && ./gradlew assembleDebug
```

Détails complets dans `docs/USAGE.md` (mis à jour à chaque jalon).

## Ce qui n'est PAS testé (et pourquoi — voir `docs/MANUAL_TESTS.md`)

Le blocage principal, répété à chaque jalon : **l'accès à l'API GitHub
dans ce sandbox est restreint à `furaxdev/runyx`**. Ça veut dire que je
n'ai jamais pu télécharger un vrai binaire `llama-server`
(`ggml-org/llama.cpp`) ni whisper.cpp/Piper. Conséquence concrète :
- Jamais de vraie génération de texte par un vrai modèle (le pull
  Hugging Face marche, le lancement du moteur d'inférence n'a jamais
  pu être vérifié de bout en bout ici).
- Jamais de vraie transcription vocale ou synthèse vocale.
- Le pont vision n'a jamais utilisé un vrai modèle auxiliaire (OCR
  seul, testé).

Autres choses non testées, honnêtement listées :
- Pas de GPU dans ce sandbox → rien côté accélération matérielle réelle.
- Pas de deuxième appareil → jamais de vrai appairage téléphone↔PC sur
  un vrai réseau local.
- Pas de Docker/brew/makepkg/nfpm → les manifestes de packaging
  (`scripts/package/`) sont écrits mais jamais construits.
- Pas de clé de signature réelle → `runyx update` vérifie qu'il n'y a
  pas de nouvelle version applicable automatiquement (comportement
  volontaire et sûr, pas un bug).

**10 premiers tests à faire à la main** (liste complète et détaillée
dans `docs/MANUAL_TESTS.md`) :
1. `runyx run <modèle>` sur une machine avec accès réseau normal — confirmer une vraie réponse du modèle.
2. Appairer un vrai téléphone Android à un vrai `runyx serve` sur le même réseau.
3. Installer `whisper-cli`/`piper` et tester une vraie transcription/synthèse.
4. `runyx pull` un modèle vision (`internal/vision.Recommended`) et attacher une vraie photo.
5. Ouvrir l'UI web dans un navigateur, basculer thème clair/sombre, tester Ctrl+K.
6. `cd android && ./gradlew installDebug` sur un appareil/émulateur réel.
7. Tester le micro push-to-talk réel (web et Android) de bout en bout.
8. Construire l'image Docker (`scripts/package/Dockerfile`) sur une machine avec Docker.
9. Tester le partage (Share Sheet) depuis une autre app Android vers Runyx.
10. Sur une machine avec GPU, confirmer que `runyx doctor` détecte bien le GPU et que l'offload fonctionne.

## Décisions prises sans vous demander (notées dans `docs/DECISIONS.md`)

- SQLite pur Go (pas de cgo) pour rester facilement cross-compilable.
- Recherche vectorielle RAG en Go pur (pas de sqlite-vec) — suffisant à cette échelle.
- Recherche web via `lite.duckduckgo.com` (pas `html.duckduckgo.com`, bloqué ici).
- Les plugins sont des serveurs MCP (réutilise le code déjà testé plutôt qu'un 2e protocole).
- QR d'appairage décodé depuis une photo (pas de scan caméra live CameraX) pour ce premier passage.
- WireGuard/Tailscale : « apportez votre VPN », pas intégré.
- Aucune clé de signature Ed25515 réelle embarquée (sécurité honnête plutôt que fausse sécurité).

## Problème d'environnement à signaler

**Impossible de pousser des tags git** depuis ce sandbox (`git push
origin <tag>` échoue systématiquement en HTTP 403, alors que
`git push origin <branche>` fonctionne). Les tags `m1-done` à
`m6-done` existent dans ce clone local mais **n'apparaissent pas**
sur `github.com/furaxdev/Runyx`. Pour les récupérer : depuis un clone
avec un accès complet, `git fetch` cette branche puis recréer les tags
aux commits correspondants (hash dans l'historique git), ou me
redonner un accès push aux tags.

## Tag final

`v0.1.0-rc1` — créé localement (même limitation que ci-dessus pour le
push). Le dernier commit sur `ccr-8047f90c-ur2jz8` représente l'état
final de cette session.
