<script lang="ts">
  import { onMount } from "svelte";
  import { api, type ModelEntry } from "./api";
  import { t, type Lang } from "./i18n";

  export let lang: Lang;

  let models: ModelEntry[] = [];
  let pullName = "";
  let pullBusy = false;
  let pullMessage = "";

  async function refresh() {
    const res = await api.tags().catch(() => ({ models: [] }));
    models = res.models ?? [];
  }

  async function doPull() {
    if (!pullName.trim()) return;
    pullBusy = true;
    pullMessage = "";
    try {
      await api.pull(pullName.trim());
      pullMessage = "✓";
      await refresh();
    } catch (e) {
      pullMessage = `${e}`;
    } finally {
      pullBusy = false;
    }
  }

  function fmtSize(bytes: number): string {
    return `${(bytes / 1024 / 1024 / 1024).toFixed(2)} GB`;
  }

  onMount(refresh);
</script>

<div class="page">
  <h1>{t(lang, "navModels")}</h1>

  <section>
    <h2>{t(lang, "pullModel")}</h2>
    <form on:submit|preventDefault={doPull} class="pull-form">
      <input placeholder={t(lang, "pullModelPlaceholder")} bind:value={pullName} />
      <button disabled={pullBusy}>{t(lang, "pullButton")}</button>
    </form>
    {#if pullMessage}
      <p class="msg">{pullMessage}</p>
    {/if}
  </section>

  <section>
    <h2>{t(lang, "modelsInstalled")}</h2>
    {#if models.length === 0}
      <p class="muted">—</p>
    {:else}
      <table>
        <thead>
          <tr>
            <th>{t(lang, "model")}</th>
            <th>{t(lang, "size")}</th>
            <th>{t(lang, "modified")}</th>
          </tr>
        </thead>
        <tbody>
          {#each models as m}
            <tr>
              <td>{m.name}</td>
              <td>{fmtSize(m.size)}</td>
              <td>{new Date(m.modified_at).toLocaleString()}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </section>
</div>

<style>
  .page {
    flex: 1;
    overflow-y: auto;
    padding: 2rem 2.5rem;
    max-width: 760px;
  }
  h1 {
    margin: 0 0 1.5rem;
  }
  section {
    margin-bottom: 2rem;
  }
  h2 {
    font-size: 0.8rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-muted);
    font-weight: 500;
    margin: 0 0 0.75rem;
  }
  .pull-form {
    display: flex;
    gap: 0.5rem;
  }
  .pull-form input {
    flex: 1;
    padding: 0.6rem 0.8rem;
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    background: var(--bg-elevated);
    color: var(--text);
  }
  .pull-form button {
    padding: 0.6rem 1.1rem;
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    background: var(--accent);
    color: var(--accent-contrast);
  }
  .msg {
    margin-top: 0.5rem;
    font-size: 0.85rem;
    color: var(--text-muted);
    word-break: break-word;
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.9rem;
  }
  th {
    text-align: left;
    color: var(--text-muted);
    font-weight: 500;
    padding: 0.5rem 0.6rem;
    border-bottom: 1px solid var(--border);
  }
  td {
    padding: 0.5rem 0.6rem;
    border-bottom: 1px solid var(--border);
  }
  .muted {
    color: var(--text-muted);
  }
</style>
