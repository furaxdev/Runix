<script lang="ts">
  import { onMount } from "svelte";
  import { api, type Message, type RegistryModel } from "./api";
  import Composer from "./Composer.svelte";
  import { Volume2 } from "./icons";

  export let conversationId: string;
  export let model: string;
  export let messages: Message[] = [];
  export let onMessagesChanged: () => void;
  export let installedModels: RegistryModel[] = [];
  export let onBrowseMore: () => void;
  // Set when this Chat is mounted right after Home created the
  // conversation for a just-typed first message: send it immediately
  // instead of waiting for another Composer submit.
  export let initialText = "";
  export let initialAttachments: { file_name: string; kind: string; text: string }[] = [];

  let streaming = false;
  let liveReply = "";
  let currentAudio: HTMLAudioElement | null = null;

  onMount(() => {
    if (initialText || initialAttachments.length > 0) {
      sendMessage(initialText, initialAttachments);
    }
  });

  function stopPlayback() {
    if (currentAudio) {
      currentAudio.pause();
      currentAudio = null;
    }
  }

  async function speak(text: string) {
    stopPlayback();
    try {
      const blob = await api.synthesizeSpeech(text);
      const audio = new Audio(URL.createObjectURL(blob));
      currentAudio = audio;
      audio.play();
    } catch (err) {
      alert(`Speech synthesis: ${err}`);
    }
  }

  function onSend(e: CustomEvent<{ text: string; attachments: { file_name: string; kind: string; text: string }[] }>) {
    sendMessage(e.detail.text, e.detail.attachments);
  }

  async function sendMessage(text: string, attachments: { file_name: string; kind: string; text: string }[]) {
    if (!text && attachments.length === 0) return;

    let fullContent = text;
    for (const a of attachments) {
      fullContent += `\n\n[Attachment: ${a.file_name}]\n${a.text}`;
    }

    stopPlayback();
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
      liveReply += `\n\n[Error: ${e}]`;
      await api.addMessage(conversationId, "assistant", liveReply);
    } finally {
      streaming = false;
      liveReply = "";
      onMessagesChanged();
    }
  }
</script>

<div class="chat">
  <div class="messages">
    {#each messages as m}
      <div class="msg {m.Role}">
        <div class="bubble">
          {m.Content}
          {#if m.Role === "assistant"}
            <button class="listen" title="Listen to reply" aria-label="Listen to reply" on:click={() => speak(m.Content)}>
              <Volume2 size={14} aria-hidden="true" />
            </button>
          {/if}
        </div>
      </div>
    {/each}
    {#if streaming}
      <div class="msg assistant">
        <div class="bubble">{liveReply}<span class="cursor">|</span></div>
      </div>
    {/if}
  </div>

  <div class="composer-dock">
    <Composer {installedModels} bind:model {onBrowseMore} disabled={streaming} on:send={onSend} />
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
  .listen {
    border: none;
    background: transparent;
    color: inherit;
    opacity: 0.6;
    margin-left: 0.4rem;
    cursor: pointer;
    vertical-align: middle;
  }
  .listen:hover {
    opacity: 1;
  }
  .composer-dock {
    padding: 0 1.5rem 1.5rem;
  }
</style>
