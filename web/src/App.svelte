<script lang="ts">
  import { onMount } from "svelte";
  import "./lib/theme.css";
  import { api, type Conversation, type Message, type RegistryModel } from "./lib/api";
  import { detectLang, t, type Lang } from "./lib/i18n";
  import Sidebar from "./lib/Sidebar.svelte";
  import Home from "./lib/Home.svelte";
  import Chat from "./lib/Chat.svelte";
  import CommandPalette from "./lib/CommandPalette.svelte";
  import Models from "./lib/Models.svelte";
  import Pairing from "./lib/Pairing.svelte";
  import Settings from "./lib/Settings.svelte";
  import Agent from "./lib/Agent.svelte";

  type View = "home" | "chat" | "models" | "pairing" | "settings" | "agent";
  let view: View = "home";

  let lang: Lang = detectLang();
  let theme: "system" | "light" | "dark" = (safeGet("runyx.theme") as any) ?? "system";
  let reduceMotion = safeGet("runyx.reduceMotion") === "1";

  let conversations: Conversation[] = [];
  let activeId: string | null = null;
  let messages: Message[] = [];
  let registryModels: RegistryModel[] = [];
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
    conversations = (await api.listConversations().catch(() => [])) ?? [];
  }

  async function refreshMessages() {
    if (!activeId) return;
    messages = (await api.messages(activeId).catch(() => [])) ?? [];
  }

  async function refreshRegistry() {
    registryModels = await api.registry().catch(() => []);
  }

  $: installedModels = registryModels.filter((m) => m.installed);

  async function selectConversation(id: string) {
    activeId = id;
    view = "chat";
    pendingFirstText = "";
    pendingFirstAttachments = [];
    const c = conversations.find((c) => c.ID === id);
    if (c) currentModel = c.Model;
    await refreshMessages();
  }

  async function newConversation() {
    const c = await api.newConversation("", currentModel || installedModels[0]?.pull_spec || "");
    await refreshConversations();
    await selectConversation(c.ID);
  }

  // Home's composer has no conversation yet: create one, then hand the
  // typed text to the freshly-mounted Chat (via initialText), which
  // sends it with its own normal streaming logic.
  let pendingFirstText = "";
  let pendingFirstAttachments: { file_name: string; kind: string; text: string }[] = [];

  async function onHomeSend(e: CustomEvent<{ text: string; attachments: { file_name: string; kind: string; text: string }[] }>) {
    const c = await api.newConversation("", currentModel || installedModels[0]?.pull_spec || "");
    await refreshConversations();
    pendingFirstText = e.detail.text;
    pendingFirstAttachments = e.detail.attachments;
    activeId = c.ID;
    view = "chat";
    await refreshMessages();
  }

  async function togglePin(id: string, pinned: boolean) {
    await api.pin(id, pinned);
    await refreshConversations();
  }

  function onTab(v: "home" | "models" | "agent") {
    view = v;
    if (v === "home" && activeId) view = "chat";
  }

  onMount(async () => {
    applyTheme();
    await refreshConversations();
    await refreshRegistry();
    if (!currentModel) currentModel = installedModels[0]?.pull_spec ?? "";
    setInterval(refreshRegistry, 10000);

    window.addEventListener("keydown", (e) => {
      if ((e.ctrlKey || e.metaKey) && e.key === "k") {
        e.preventDefault();
        paletteOpen = true;
      }
    });

    // Without this, dropping a file anywhere outside the Composer's own
    // drop zone falls through to the browser's default behavior --
    // navigating the whole tab to the dropped file -- which would wipe
    // out the app. The Composer's own dragover/drop handlers still run
    // first (they stop propagation isn't needed since both just call
    // preventDefault), so a drop *on* the composer still attaches it.
    window.addEventListener("dragover", (e) => e.preventDefault());
    window.addEventListener("drop", (e) => e.preventDefault());
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
  <Sidebar
    {view}
    {onTab}
    onOpenSettings={() => (view = "settings")}
    {conversations}
    {activeId}
    onSelect={selectConversation}
    onNew={() => {
      activeId = null;
      view = "home";
    }}
    onTogglePin={togglePin}
    onOpenDownloads={() => (view = "models")}
  />

  <main>
    {#if view === "home"}
      <Home {installedModels} bind:model={currentModel} onBrowseMore={() => (view = "models")} on:send={onHomeSend} />
    {:else if view === "chat" && activeId}
      {#key activeId}
        <Chat
          conversationId={activeId}
          model={currentModel}
          {messages}
          onMessagesChanged={refreshMessages}
          {installedModels}
          onBrowseMore={() => (view = "models")}
          initialText={pendingFirstText}
          initialAttachments={pendingFirstAttachments}
        />
      {/key}
    {:else if view === "models"}
      <Models
        onUse={(spec) => {
          currentModel = spec;
          view = activeId ? "chat" : "home";
        }}
      />
    {:else if view === "pairing"}
      <Pairing {lang} />
    {:else if view === "settings"}
      <Settings {theme} {reduceMotion} onSetTheme={setTheme} onToggleReduceMotion={setReduceMotion} />
    {:else if view === "agent"}
      <Agent onPairDevice={() => (view = "pairing")} />
    {/if}
  </main>
</div>

<CommandPalette {lang} bind:open={paletteOpen} {commands} />

<style>
  .app {
    display: flex;
    height: 100vh;
    width: 100vw;
    overflow: hidden;
  }
  main {
    flex: 1;
    display: flex;
    min-width: 0;
  }
</style>
