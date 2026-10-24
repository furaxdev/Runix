<script lang="ts">
  import { onMount } from "svelte";
  import { fly } from "svelte/transition";
  import { cubicOut } from "svelte/easing";
  import { api, type Message, type RegistryModel } from "./api";
  import Composer from "./Composer.svelte";
  import { Volume2 } from "./icons";

  export let conversationId: string;
  export let model: string;
  export let messages: Message[] = [];
  export let onMessagesChanged: () => void;
  export let installedModels: RegistryModel[] = [];
  export let onBrowseMore: () => void;
  export let reduceMotion = false;

  $: messageFlyDuration = reduceMotion ? 0 : 220;
  // Set when this Chat is mounted right after Home created the
  // conversation for a just-typed first message: send it immediately
  // instead of waiting for another Composer submit.
  export let initialText = "";
  export let initialAttachments: { file_name: string; kind: string; text: string }[] = [];

  let streaming = false;
  let stalled = false;
  let liveReply = "";
  let currentAudio: HTMLAudioElement | null = null;
  let abortController: AbortController | null = null;
  let stallTimer: ReturnType<typeof setTimeout> | null = null;

  // Demo-only determinism: screenshot.mjs passes these as query params so
  // the CI demo gets a short, capped, reproducible reply instead of
  // waiting on however long the real default generation takes. Absent in
  // normal use, so real conversations are never capped by this.
  const demoParams = new URLSearchParams(location.search);
  const demoMaxTokens = demoParams.has("demoMaxTokens") ? Number(demoParams.get("demoMaxTokens")) : undefined;
  const demoTemperature = demoParams.has("demoTemp") ? Number(demoParams.get("demoTemp")) : undefined;

  // If no streamed token arrives for this long, something is stuck (a
  // stalled upstream, a dropped connection) — tell the user instead of
  // leaving the streaming cursor spinning forever. This never aborts the
  // request on its own: a real model can legitimately take a while, so
  // only the Stop button cancels it.
  const STALL_TIMEOUT_MS = 20000;

  function clearStallTimer() {
    if (stallTimer) clearTimeout(stallTimer);
    stallTimer = null;
  }

  function armStallTimer() {
    clearStallTimer();
    stallTimer = setTimeout(() => {
      stalled = true;
    }, STALL_TIMEOUT_MS);
  }

  function stopGeneration() {
    abortController?.abort();
  }

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
    stalled = false;
    liveReply = "";
    abortController = new AbortController();
    armStallTimer();
    try {
      const history = [...messages, { role: "user", content: fullContent }].map((m: any) => ({
        role: m.Role ?? m.role,
        content: m.Content ?? m.content,
      }));
      await api.streamChat(
        model,
        history,
        (delta) => {
          liveReply += delta;
          stalled = false;
          armStallTimer();
        },
        abortController.signal,
        { maxTokens: demoMaxTokens, temperature: demoTemperature }
      );
      await api.addMessage(conversationId, "assistant", liveReply);
    } catch (e) {
      if (abortController?.signal.aborted) {
        // User-initiated Stop: keep whatever was generated so far, no error noise.
        if (liveReply) await api.addMessage(conversationId, "assistant", liveReply);
      } else {
        liveReply += `\n\n[Error: ${e}]`;
        await api.addMessage(conversationId, "assistant", liveReply);
      }
    } finally {
      clearStallTimer();
      streaming = false;
      stalled = false;
      liveReply = "";
      abortController = null;
      onMessagesChanged();
    }
  }
</script>

<div class="chat">
  <div class="messages">
    {#each messages as m (m.ID)}
      <div class="msg {m.Role}" in:fly={{ y: 10, duration: messageFlyDuration, easing: cubicOut }}>
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
      <div class="msg assistant" in:fly={{ y: 10, duration: messageFlyDuration, easing: cubicOut }}>
        <div class="bubble">
          {liveReply}<span class="cursor">|</span>
          {#if stalled}
            <div class="stall-notice">
              <span>Model is slow or stalled</span>
              <button class="stop-btn" type="button" on:click={stopGeneration}>Stop</button>
            </div>
          {/if}
        </div>
      </div>
    {/if}
  </div>

  <div class="composer-dock">
    <Composer {installedModels} bind:model {onBrowseMore} disabled={streaming} on:send={onSend} {reduceMotion} />
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
    max-width: 100%;
    padding: 0;
    background: none;
    border: none;
  }
  .cursor {
    animation: runyx-pulse 1s ease-in-out infinite;
  }
  .stall-notice {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    margin-top: 0.5rem;
    font-size: 0.85rem;
    opacity: 0.8;
  }
  .stop-btn {
    border: 1px solid var(--border);
    background: transparent;
    color: inherit;
    border-radius: var(--radius-sm, 4px);
    padding: 0.15rem 0.6rem;
    cursor: pointer;
    font-size: 0.8rem;
  }
  .stop-btn:hover {
    background: var(--bg-elevated);
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
