<script lang="ts">
  // Placeholder shell for the Runyx chat UI. Real design-system work
  // (themes, typography, command palette) lands in M2 — see
  // docs/ROADMAP.md item 29. This file exists so the build pipeline and
  // go:embed wiring can be exercised end to end before the real UI is
  // built.
  let status = "not connected";

  async function checkDaemon() {
    try {
      const res = await fetch("/healthz");
      status = res.ok ? "connected" : `error: HTTP ${res.status}`;
    } catch (e) {
      status = `error: ${e}`;
    }
  }
</script>

<main>
  <h1>Runyx</h1>
  <p>UI web — à construire au jalon M2 / web UI — to be built in M2.</p>
  <button on:click={checkDaemon}>Vérifier le daemon / Check daemon</button>
  <p>Statut / Status: {status}</p>
</main>

<style>
  main {
    font-family: system-ui, sans-serif;
    max-width: 40rem;
    margin: 4rem auto;
    padding: 0 1rem;
  }
</style>
