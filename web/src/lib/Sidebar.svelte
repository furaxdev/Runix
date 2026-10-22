<script lang="ts">
  import { onMount } from "svelte";
  import type { Conversation, Project } from "./api";
  import { api } from "./api";
  import Logo from "./Logo.svelte";
  import DeviceCard from "./DeviceCard.svelte";
  import {
    MessageSquare,
    Package,
    ShieldCheck,
    PanelLeftClose,
    PanelLeftOpen,
    Plus,
    FolderClosed,
    ChevronDown,
    ChevronRight,
    Search,
    Star,
    SettingsIcon,
  } from "./icons";

  export let view: "home" | "chat" | "models" | "agent" | "settings" | "pairing";
  export let onTab: (v: "home" | "models" | "agent") => void;
  export let onOpenSettings: () => void;
  export let conversations: Conversation[] = [];
  export let activeId: string | null = null;
  export let onSelect: (id: string) => void;
  export let onNew: () => void;
  export let onTogglePin: (id: string, pinned: boolean) => void;
  export let onOpenDownloads: () => void;

  let collapsed = false;
  let search = "";
  let projectsOpen = true;
  let projects: Project[] = [];

  onMount(async () => {
    // GET /api/runyx/projects returns a bare JSON `null` (nil Go slice)
    // when there are none yet, same as /api/runyx/conversations — guard
    // against that, not just a request error.
    projects = (await api.listProjects().catch(() => [])) ?? [];
  });

  async function addProject() {
    const name = window.prompt("Project name");
    if (!name) return;
    const p = await api.newProject(name).catch(() => null);
    if (p) projects = [...projects, p];
  }

  $: filtered = conversations.filter((c) =>
    (c.Title || c.Model || "").toLowerCase().includes(search.toLowerCase())
  );

  function dayBucket(iso: string): string {
    const d = new Date(iso);
    const now = new Date();
    const diffDays = Math.floor((now.setHours(0, 0, 0, 0) - new Date(d).setHours(0, 0, 0, 0)) / 86400000);
    if (diffDays <= 0) return "Today";
    if (diffDays === 1) return "Yesterday";
    if (diffDays < 7) return "This week";
    return "Earlier";
  }

  $: groups = (() => {
    const order = ["Today", "Yesterday", "This week", "Earlier"];
    const map: Record<string, Conversation[]> = {};
    for (const c of filtered) {
      const b = dayBucket(c.UpdatedAt || c.CreatedAt);
      (map[b] ??= []).push(c);
    }
    return order.filter((k) => map[k]?.length).map((k) => ({ label: k, items: map[k] }));
  })();
</script>

<nav class:collapsed>
  <div class="top">
    <button class="collapse-btn" on:click={() => (collapsed = !collapsed)} aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}>
      {#if collapsed}
        <PanelLeftOpen size={17} aria-hidden="true" />
      {:else}
        <Logo size={20} />
        <span class="wordmark">Runyx</span>
        <PanelLeftClose size={16} aria-hidden="true" />
      {/if}
    </button>
  </div>

  <div class="tabs">
    <button class="tab" class:active={view === "home" || view === "chat"} title="Chat" aria-label="Chat" on:click={() => onTab("home")}>
      <MessageSquare size={17} aria-hidden="true" />
      {#if !collapsed}<span>Chat</span>{/if}
    </button>
    <button class="tab" class:active={view === "models"} title="Models" aria-label="Models" on:click={() => onTab("models")}>
      <Package size={17} aria-hidden="true" />
      {#if !collapsed}<span>Models</span>{/if}
    </button>
    <button class="tab" class:active={view === "agent"} title="Agent" aria-label="Agent" on:click={() => onTab("agent")}>
      <ShieldCheck size={17} aria-hidden="true" />
      {#if !collapsed}<span>Agent</span>{/if}
    </button>
  </div>

  {#if !collapsed}
    <div class="quick-links">
      <button class="quick-link" on:click={onNew}>
        <Plus size={15} aria-hidden="true" />
        <span>New chat</span>
      </button>
      <button class="quick-link" on:click={onOpenSettings}>
        <SettingsIcon size={15} aria-hidden="true" />
        <span>Settings</span>
      </button>
    </div>

    <div class="section">
      <button class="section-header" on:click={() => (projectsOpen = !projectsOpen)}>
        {#if projectsOpen}<ChevronDown size={13} aria-hidden="true" />{:else}<ChevronRight size={13} aria-hidden="true" />{/if}
        <FolderClosed size={14} aria-hidden="true" />
        <span>Projects</span>
        <button class="add-inline" on:click|stopPropagation={addProject} aria-label="New project">
          <Plus size={13} aria-hidden="true" />
        </button>
      </button>
      {#if projectsOpen}
        <ul class="project-list">
          {#each projects as p}
            <li>{p.Name}</li>
          {/each}
          {#if projects.length === 0}
            <li class="empty">No projects yet.</li>
          {/if}
        </ul>
      {/if}
    </div>

    <div class="section recents">
      <div class="search-box">
        <Search size={13} aria-hidden="true" />
        <input placeholder="Search conversations" bind:value={search} />
      </div>
      <h2>Recents</h2>
      {#if filtered.length === 0}
        <p class="empty">No conversations yet.</p>
      {/if}
      {#each groups as g}
        <h3>{g.label}</h3>
        <ul>
          {#each g.items as c}
            <li class:active={c.ID === activeId}>
              <button class="title" on:click={() => onSelect(c.ID)}>
                {c.Title || c.Model || "…"}
              </button>
              <button class="pin" title={c.Pinned ? "Unpin" : "Pin"} on:click={() => onTogglePin(c.ID, !c.Pinned)}>
                <Star size={13} fill={c.Pinned ? "currentColor" : "none"} aria-hidden="true" />
              </button>
            </li>
          {/each}
        </ul>
      {/each}
    </div>
  {/if}

  <DeviceCard {collapsed} {onOpenDownloads} />
</nav>

<style>
  nav {
    width: 280px;
    flex-shrink: 0;
    background: var(--bg-sunken);
    border-right: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    height: 100%;
    overflow: hidden;
    transition: width 0.15s ease;
  }
  nav.collapsed {
    width: 60px;
  }
  .top {
    padding: 0.75rem 0.6rem 0.25rem;
  }
  .collapse-btn {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    width: 100%;
    border: none;
    background: transparent;
    padding: 0.4rem;
    border-radius: var(--radius-md);
    color: var(--text);
  }
  .wordmark {
    flex: 1;
    text-align: left;
    font-weight: 600;
  }
  .tabs {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
    padding: 0.3rem 0.5rem;
  }
  .tab {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    border: none;
    background: transparent;
    border-radius: var(--radius-md);
    padding: 0.5rem 0.6rem;
    color: var(--text-muted);
    font-size: 0.88rem;
  }
  .tab.active {
    background: var(--bg-elevated);
    color: var(--text);
  }
  .quick-links {
    padding: 0.4rem 0.5rem;
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
  }
  .quick-link {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    border: none;
    background: transparent;
    border-radius: var(--radius-md);
    padding: 0.45rem 0.6rem;
    color: var(--text-muted);
    font-size: 0.85rem;
  }
  .quick-link:hover {
    color: var(--text);
  }
  .section {
    padding: 0.3rem 0.5rem;
  }
  .section-header {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    width: 100%;
    border: none;
    background: transparent;
    padding: 0.4rem 0.3rem;
    color: var(--text-muted);
    font-size: 0.8rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }
  .add-inline {
    margin-left: auto;
    border: none;
    background: transparent;
    color: var(--text-muted);
    padding: 0.1rem;
  }
  .project-list {
    list-style: none;
    margin: 0;
    padding: 0 0.3rem;
    font-size: 0.84rem;
    color: var(--text);
  }
  .project-list li {
    padding: 0.3rem 0;
  }
  .recents {
    flex: 1;
    overflow-y: auto;
  }
  .search-box {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    background: var(--bg-elevated);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    padding: 0.4rem 0.6rem;
    margin: 0.2rem 0.3rem 0.5rem;
    color: var(--text-muted);
  }
  .search-box input {
    border: none;
    background: transparent;
    outline: none;
    color: var(--text);
    font-size: 0.82rem;
    width: 100%;
  }
  h2 {
    font-size: 0.72rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-muted);
    margin: 0.4rem 0.3rem 0.1rem;
    font-weight: 500;
  }
  h3 {
    font-size: 0.68rem;
    text-transform: uppercase;
    color: var(--text-muted);
    margin: 0.6rem 0.3rem 0.1rem;
    font-weight: 500;
  }
  .empty {
    color: var(--text-muted);
    font-size: 0.82rem;
    padding: 0 0.3rem;
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  li {
    display: flex;
    align-items: center;
    border-radius: var(--radius-sm);
  }
  li.active {
    background: var(--bg-elevated);
  }
  li .title {
    flex: 1;
    text-align: left;
    background: none;
    border: none;
    padding: 0.5rem 0.6rem;
    color: var(--text);
    font-size: 0.86rem;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  li .pin {
    background: none;
    border: none;
    color: var(--text-muted);
    padding: 0.4rem;
  }
  @media (max-width: 760px) {
    nav {
      display: none;
    }
  }
</style>
