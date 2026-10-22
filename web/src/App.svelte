<script lang="ts">
  import { onMount } from "svelte";
  import "./lib/theme.css";
  import { api, type Conversation, type Message, type DashboardStats } from "./lib/api";
  import { detectLang, t, type Lang } from "./lib/i18n";
  import Sidebar from "./lib/Sidebar.svelte";
  import Dashboard from "./lib/Dashboard.svelte";
  import Chat from "./lib/Chat.svelte";
  import CommandPalette from "./lib/CommandPalette.svelte";
  import Models from "./lib/Models.svelte";
  import Pairing from "./lib/Pairing.svelte";
  import Settings from "./lib/Settings.svelte";

  type View = "chat" | "models" | "pairing" | "settings";
  let view: View = "chat";

  let lang: Lang = detectLang();
  let theme: "system" | "light" | "dark" = (safeGet("runyx.theme") as any) ?? "system";
  let reduceMotion = safeGet("runyx.reduceMotion") === "1";

  let conversations: Conversation[] = [];
  let activeId: string | null = null;
  let messages: Message[] = [];
  let stats: DashboardStats | null = null;
  let models: string[] = [];
  let currentModel = "";
  let paletteOpen = false;

  function safeGet(k: string): string | null {
    try {
      return localStorage.getItem(k);
    } catch {
      return null;
    }
  }
  function safeSet(k: string, v: string) {
    try {
      localStorage.setItem(k, v);
    } catch {
      /* ignore (private window, blocked storage, etc.) */
    }
  }

  function applyTheme() {
    document.documentElement.setAttribute("data-theme", theme === "system" ? "" : theme);
    document.body.classList.toggle("reduce-motion", reduceMotion);
  }

  async function refreshConversations() {
    // The API returns a bare JSON `null` (not `[]`) when there are no
    // conversations yet, since it's backed by a nil Go slice — guard
    // against that here rather than only on a request *error*.
    conversations = (await api.listConversations().catch(() => [])) ?? [];
  }

  async function refreshMessages() {
    if (!activeId) return;
    messages = (await api.messages(activeId).catch(() => [])) ?? [];
  }

  async function refreshDashboard() {
    stats = await api.dashboard().catch(() => null);
  }

  async function selectConversation(id: string) {
    activeId = id;
    const c = conversations.find((c) => c.ID === id);
    if (c) currentModel = c.Model;
    await refreshMessages();
  }

  async function newConversation() {
    const c = await api.newConversation("", currentModel || models[0] || "");
    await refreshConversations();
    await selectConversation(c.ID);
  }

  async function togglePin(id: string, pinned: boolean) {
    await api.pin(id, pinned);
    await refreshConversations();
  }

  onMount(async () => {
    applyTheme();
    await refreshConversations();
    await refreshDashboard();
    try {
      const m = await api.models();
      models = m.data.map((d) => d.id);
      if (!currentModel) currentModel = models[0] ?? "";
    } catch {
      /* daemon not reachable yet; dashboard/model list stay empty */
    }
    setInterval(refreshDashboard, 5000);

    window.addEventListener("keydown", (e) => {
      if ((e.ctrlKey || e.metaKey) && e.key === "k") {
        e.preventDefault();
        paletteOpen = true;
      }
    });
  });

  $: commands = [
    { label: t(lang, "newChat"), run: newConversation },
    { label: `${t(lang, "theme")}: ${t(lang, "light")}`, run: () => setTheme("light") },
    { label: `${t(lang, "theme")}: ${t(lang, "dark")}`, run: () => setTheme("dark") },
    { label: `${t(lang, "theme")}: ${t(lang, "system")}`, run: () => setTheme("system") },
  ];

  function setTheme(v: "system" | "light" | "dark") {
    theme = v;
    safeSet("runyx.theme", v);
    applyTheme();
  }

  function setReduceMotion(v: boolean) {
    reduceMotion = v;
    safeSet("runyx.reduceMotion", v ? "1" : "0");
    applyTheme();
  }
</script>

<div class="app">
  <nav class="rail">
    <button class:active={view === "chat"} on:click={() => (view = "chat")} title={t(lang, "navChat")}>💬</button>
    <button class:active={view === "models"} on:click={() => (view = "models")} title={t(lang, "navModels")}>📦</button>
    <button class:active={view === "pairing"} on:click={() => (view = "pairing")} title={t(lang, "navPairing")}>📱</button>
    <button class:active={view === "settings"} on:click={() => (view = "settings")} title={t(lang, "navSettings")}>⚙️</button>
  </nav>

  {#if view === "chat"}
    <Sidebar
      {lang}
      {conversations}
      {activeId}
      onSelect={selectConversation}
      onNew={newConversation}
      onTogglePin={togglePin}
    />

    {#if activeId}
      <Chat {lang} conversationId={activeId} model={currentModel} {messages} onMessagesChanged={refreshMessages} />
    {:else}
      <div class="empty-state">
        <h1>Runyx</h1>
        <p>{t(lang, "noConversations")}</p>
        <button on:click={newConversation}>+ {t(lang, "newChat")}</button>
      </div>
    {/if}

    <Dashboard {lang} {stats} />
  {:else if view === "models"}
    <Models {lang} />
  {:else if view === "pairing"}
    <Pairing {lang} />
  {:else if view === "settings"}
    <Settings {lang} {theme} {reduceMotion} onSetTheme={setTheme} onToggleReduceMotion={setReduceMotion} />
  {/if}
</div>

<CommandPalette {lang} bind:open={paletteOpen} {commands} />

<style>
  .app {
    display: flex;
    height: 100vh;
    width: 100vw;
    overflow: hidden;
  }
  .empty-state {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 1rem;
    color: var(--text-muted);
    text-align: center;
  }
  .empty-state h1 {
    font-size: 2.5rem;
    color: var(--text);
    margin: 0;
  }
  .empty-state button {
    border: 1px solid var(--border);
    background: var(--bg-elevated);
    border-radius: var(--radius-md);
    padding: 0.6rem 1.2rem;
    color: var(--text);
  }
  .rail {
    width: 52px;
    flex-shrink: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.5rem;
    padding: 1rem 0;
    background: var(--bg-sunken);
    border-right: 1px solid var(--border);
  }
  .rail button {
    width: 36px;
    height: 36px;
    border: none;
    background: transparent;
    border-radius: var(--radius-md);
    font-size: 1.1rem;
    color: var(--text-muted);
  }
  .rail button.active {
    background: var(--bg-elevated);
    color: var(--text);
  }
</style>
