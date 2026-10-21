<script lang="ts">
  import { api, type Message } from "./api";
  import { t, type Lang } from "./i18n";

  export let lang: Lang;
  export let conversationId: string;
  export let model: string;
  export let messages: Message[] = [];
  export let onMessagesChanged: () => void;

  let input = "";
  let streaming = false;
  let liveReply = "";
  let fileInput: HTMLInputElement;
  let pendingAttachments: { file_name: string; kind: string; text: string }[] = [];

  async function send() {
    const text = input.trim();
    if (!text && pendingAttachments.length === 0) return;
    input = "";

    let fullContent = text;
    for (const a of pendingAttachments) {
      fullContent += `\n\n[Attachment: ${a.file_name}]\n${a.text}`;
    }
    pendingAttachments = [];

    await api.addMessage(conversationId, "user", fullContent);
    onMessagesChanged();

    streaming = true;
    liveReply = "";
    try {
      const history = [...messages, { role: "user", content: fullContent }].map((m: any) => ({
        role: m.Role ?? m.role,
        content: m.Content ?? m.content,
      }));
      await api.streamChat(model, history, (delta) => {
        liveReply += delta;
      });
      await api.addMessage(conversationId, "assistant", liveReply);
    } catch (e) {
      liveReply += `\n\n[${e}]`;
      await api.addMessage(conversationId, "assistant", liveReply);
    } finally {
      streaming = false;
      liveReply = "";
      onMessagesChanged();
    }
  }

  async function onFilePicked(e: Event) {
    const files = (e.target as HTMLInputElement).files;
    if (!files) return;
    for (const f of Array.from(files)) {
      try {
        const res = await api.uploadAttachment(f);
        pendingAttachments = [...pendingAttachments, res];
      } catch (err) {
        alert(`${f.name}: ${err}`);
      }
    }
    fileInput.value = "";
  }

  function onKeydown(e: KeyboardEvent) {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      send();
    }
  }
</script>

<div class="chat">
  <div class="messages">
    {#each messages as m}
      <div class="msg {m.Role}">
        <div class="bubble">{m.Content}</div>
      </div>
    {/each}
    {#if streaming}
      <div class="msg assistant">
        <div class="bubble">{liveReply}<span class="cursor">▌</span></div>
      </div>
    {/if}
  </div>

  {#if pendingAttachments.length > 0}
    <div class="chips">
      {#each pendingAttachments as a}
        <span class="chip" title={a.text.slice(0, 300)}>{a.file_name} ({a.kind})</span>
      {/each}
    </div>
  {/if}

  <div class="composer">
    <button class="attach" title={t(lang, "attachFile")} on:click={() => fileInput.click()}>📎</button>
    <input
      type="file"
      bind:this={fileInput}
      on:change={onFilePicked}
      style="display: none"
      multiple
    />
    <textarea
      bind:value={input}
      placeholder={t(lang, "placeholder")}
      on:keydown={onKeydown}
      rows="1"
    />
    <button class="send" on:click={send} disabled={streaming}>{t(lang, "send")}</button>
  </div>
</div>

<style>
  .chat {
    flex: 1;
    display: flex;
    flex-direction: column;
    height: 100%;
    min-width: 0;
  }
  .messages {
    flex: 1;
    overflow-y: auto;
    padding: 2rem 1.5rem 1rem;
    display: flex;
    flex-direction: column;
    gap: 1rem;
    max-width: 720px;
    width: 100%;
    margin: 0 auto;
  }
  .msg {
    display: flex;
  }
  .msg.user {
    justify-content: flex-end;
  }
  .bubble {
    max-width: 85%;
    padding: 0.7rem 1rem;
    border-radius: var(--radius-lg);
    white-space: pre-wrap;
    line-height: 1.5;
  }
  .msg.user .bubble {
    background: var(--accent);
    color: var(--accent-contrast);
  }
  .msg.assistant .bubble {
    background: var(--bg-elevated);
    border: 1px solid var(--border);
  }
  .cursor {
    opacity: 0.5;
  }
  .chips {
    padding: 0 1.5rem;
    display: flex;
    gap: 0.5rem;
    max-width: 720px;
    margin: 0 auto;
    width: 100%;
  }
  .chip {
    font-size: 0.78rem;
    background: var(--bg-sunken);
    border: 1px solid var(--border);
    border-radius: 999px;
    padding: 0.25rem 0.7rem;
    color: var(--text-muted);
  }
  .composer {
    display: flex;
    align-items: flex-end;
    gap: 0.5rem;
    max-width: 720px;
    width: calc(100% - 3rem);
    margin: 0 auto 1.5rem;
    background: var(--bg-elevated);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    padding: 0.6rem 0.75rem;
    box-shadow: var(--shadow);
  }
  textarea {
    flex: 1;
    border: none;
    background: transparent;
    resize: none;
    font-family: inherit;
    font-size: 0.95rem;
    color: var(--text);
    outline: none;
    max-height: 160px;
    padding: 0.4rem 0.2rem;
  }
  .attach, .send {
    border: none;
    background: transparent;
    border-radius: var(--radius-md);
    padding: 0.5rem 0.7rem;
    color: var(--text-muted);
  }
  .send {
    background: var(--accent);
    color: var(--accent-contrast);
  }
  .send:disabled {
    opacity: 0.5;
  }
</style>
