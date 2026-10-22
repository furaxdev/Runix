<script lang="ts">
  import type { Conversation } from "./api";
  import { t, type Lang } from "./i18n";

  export let lang: Lang;
  export let conversations: Conversation[] = [];
  export let activeId: string | null = null;
  export let onSelect: (id: string) => void;
  export let onNew: () => void;
  export let onTogglePin: (id: string, pinned: boolean) => void;
</script>

<nav>
  <button class="new-chat" on:click={onNew}>+ {t(lang, "newChat")}</button>
  <h2>{t(lang, "conversations")}</h2>
  {#if conversations.length === 0}
    <p class="empty">{t(lang, "noConversations")}</p>
  {/if}
  <ul>
    {#each conversations as c}
      <li class:active={c.ID === activeId}>
        <button class="title" on:click={() => onSelect(c.ID)}>
          {c.Title || c.Model || "…"}
        </button>
        <button
          class="pin"
          title={t(lang, c.Pinned ? "unpin" : "pin")}
          on:click={() => onTogglePin(c.ID, !c.Pinned)}
        >
          {c.Pinned ? "★" : "☆"}
        </button>
      </li>
    {/each}
  </ul>
</nav>

<style>
  nav {
    width: 260px;
    flex-shrink: 0;
    background: var(--bg-sunken);
    border-right: 1px solid var(--border);
    padding: 1rem 0.75rem;
    overflow-y: auto;
    height: 100%;
  }
  h2 {
    font-size: 0.75rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-muted);
    margin: 1.25rem 0.5rem 0.5rem;
    font-weight: 500;
  }
  .new-chat {
    width: 100%;
    padding: 0.6rem 0.9rem;
    border-radius: var(--radius-md);
    border: 1px solid var(--border);
    background: var(--bg-elevated);
    color: var(--text);
    font-size: 0.9rem;
  }
  .empty {
    color: var(--text-muted);
    font-size: 0.85rem;
    padding: 0 0.5rem;
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
    font-size: 0.88rem;
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
  /* Below ~760px the sidebar + dashboard no longer fit alongside the
     chat pane (they're fixed-width), so collapse to a single-pane
     chat view, the same pattern a phone-sized screen needs. */
  @media (max-width: 760px) {
    nav {
      display: none;
    }
  }
</style>
