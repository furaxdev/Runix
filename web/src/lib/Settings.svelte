<script lang="ts">
  import { onMount } from "svelte";
  import { api, type PermissionsSnapshot } from "./api";
  import { t, type Lang } from "./i18n";

  export let lang: Lang;
  export let theme: "system" | "light" | "dark";
  export let reduceMotion: boolean;
  export let onSetTheme: (v: "system" | "light" | "dark") => void;
  export let onToggleReduceMotion: (v: boolean) => void;

  let perms: PermissionsSnapshot | null = null;

  async function refresh() {
    perms = await api.permissions().catch(() => null);
  }

  function onThemeChange(e: Event) {
    const v = (e.target as HTMLSelectElement).value;
    onSetTheme(v === "light" || v === "dark" ? v : "system");
  }

  function onReduceMotionChange(e: Event) {
    onToggleReduceMotion((e.target as HTMLInputElement).checked);
  }

  async function revoke(id: string) {
    await api.revokeDevice(id);
    await refresh();
  }

  onMount(refresh);
</script>

<div class="page">
  <h1>{t(lang, "settingsTitle")}</h1>

  <section>
    <h2>{t(lang, "settingsAppearance")}</h2>
    <div class="row">
      <label>{t(lang, "theme")}</label>
      <select value={theme} on:change={onThemeChange}>
        <option value="system">{t(lang, "system")}</option>
        <option value="light">{t(lang, "light")}</option>
        <option value="dark">{t(lang, "dark")}</option>
      </select>
    </div>
    <div class="row">
      <label>
        <input type="checkbox" checked={reduceMotion} on:change={onReduceMotionChange} />
        {t(lang, "reduceMotion")}
      </label>
    </div>
  </section>

  {#if perms}
    <section>
      <h2>{t(lang, "settingsShellAgent")}</h2>
      <p class="status">{perms.shell_agent.enabled ? t(lang, "settingsEnabled") : t(lang, "settingsDisabled")}</p>
    </section>

    <section>
      <h2>{t(lang, "settingsVision")} / {t(lang, "settingsVoice")} / {t(lang, "settingsWebSearch")}</h2>
      <ul class="flags">
        <li>{t(lang, "settingsVision")}: {perms.vision_enabled ? "✓" : "—"}</li>
        <li>{t(lang, "settingsVoice")}: {perms.voice_enabled ? "✓" : "—"}</li>
        <li>{t(lang, "settingsWebSearch")}: {perms.web_search_on ? "✓" : "—"}</li>
      </ul>
    </section>

    <section>
      <h2>{t(lang, "settingsDevices")}</h2>
      {#if (perms.devices ?? []).length === 0}
        <p class="muted">{t(lang, "settingsNoDevices")}</p>
      {:else}
        <ul class="devices">
          {#each perms.devices as d}
            <li>
              <span>{d.name}</span>
              <button on:click={() => revoke(d.id)}>{t(lang, "settingsRevoke")}</button>
            </li>
          {/each}
        </ul>
      {/if}
    </section>
  {/if}
</div>

<style>
  .page {
    flex: 1;
    overflow-y: auto;
    padding: 2rem 2.5rem;
    max-width: 640px;
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
  .row {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    margin-bottom: 0.75rem;
  }
  select {
    padding: 0.4rem 0.6rem;
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    background: var(--bg-elevated);
    color: var(--text);
  }
  .status,
  .muted {
    color: var(--text-muted);
  }
  .flags {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    gap: 1.5rem;
    font-size: 0.9rem;
  }
  .devices {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .devices li {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.5rem 0;
    border-bottom: 1px solid var(--border);
  }
  .devices button {
    border: 1px solid var(--border);
    background: transparent;
    color: var(--text-muted);
    border-radius: var(--radius-sm);
    padding: 0.3rem 0.6rem;
    font-size: 0.8rem;
  }
</style>
