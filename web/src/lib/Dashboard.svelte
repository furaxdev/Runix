<script lang="ts">
  import type { DashboardStats } from "./api";
  import { t, type Lang } from "./i18n";

  export let lang: Lang;
  export let stats: DashboardStats | null;
</script>

<aside>
  <h2>{t(lang, "dashboard")}</h2>
  {#if stats}
    <dl>
      <dt>{t(lang, "backend")}</dt>
      <dd>{stats.backend}</dd>
      <dt>{t(lang, "ram")}</dt>
      <dd>{(stats.total_ram_mb / 1024).toFixed(1)} GB</dd>
      {#if stats.total_vram_mb > 0}
        <dt>{t(lang, "vram")}</dt>
        <dd>{(stats.total_vram_mb / 1024).toFixed(1)} GB</dd>
      {/if}
    </dl>
    <h3>{t(lang, "loadedModels")}</h3>
    {#if (stats.loaded_models ?? []).length === 0}
      <p class="muted">—</p>
    {:else}
      <ul>
        {#each stats.loaded_models as m}
          <li>
            <span class="name">{m.name}</span>
            <span class="muted">{m.idle_seconds}s idle</span>
          </li>
        {/each}
      </ul>
    {/if}
  {/if}
</aside>

<style>
  aside {
    width: 240px;
    flex-shrink: 0;
    border-left: 1px solid var(--border);
    background: var(--bg-sunken);
    padding: 1rem;
    overflow-y: auto;
  }
  h2 {
    font-size: 0.75rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-muted);
    font-weight: 500;
    margin: 0 0 0.75rem;
  }
  h3 {
    font-size: 0.7rem;
    text-transform: uppercase;
    color: var(--text-muted);
    font-weight: 500;
    margin: 1.25rem 0 0.5rem;
  }
  dl {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: 0.25rem 0.75rem;
    font-size: 0.85rem;
    margin: 0;
  }
  dt {
    color: var(--text-muted);
  }
  dd {
    margin: 0;
    text-align: right;
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
    font-size: 0.82rem;
  }
  li {
    display: flex;
    justify-content: space-between;
    padding: 0.3rem 0;
    border-bottom: 1px solid var(--border);
  }
  .name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    max-width: 130px;
  }
  .muted {
    color: var(--text-muted);
  }
</style>
