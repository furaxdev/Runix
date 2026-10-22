<script lang="ts">
  import { createEventDispatcher } from "svelte";
  import { api, type RegistryModel } from "./api";
  import { Paperclip, Mic, Square, SendHorizontal } from "./icons";
  import ModelSelect from "./ModelSelect.svelte";

  export let installedModels: RegistryModel[] = [];
  export let model = "";
  export let onBrowseMore: () => void;
  export let disabled = false;
  export let placeholder = "Type a message...";
  export let autofocus = false;

  const dispatch = createEventDispatcher<{
    send: { text: string; attachments: { file_name: string; kind: string; text: string }[] };
  }>();

  let input = "";
  let fileInput: HTMLInputElement;
  let pendingAttachments: { file_name: string; kind: string; text: string }[] = [];
  let recording = false;
  let mediaRecorder: MediaRecorder | null = null;
  let recordedChunks: Blob[] = [];

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

  async function toggleRecording() {
    if (recording) {
      mediaRecorder?.stop();
      return;
    }
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
      recordedChunks = [];
      mediaRecorder = new MediaRecorder(stream);
      mediaRecorder.ondataavailable = (e) => recordedChunks.push(e.data);
      mediaRecorder.onstop = async () => {
        recording = false;
        stream.getTracks().forEach((t) => t.stop());
        const blob = new Blob(recordedChunks, { type: "audio/webm" });
        try {
          const result = await api.transcribeAudio(blob);
          input = (input ? input + " " : "") + result.text;
        } catch (err) {
          alert(`Transcription: ${err}`);
        }
      };
      mediaRecorder.start();
      recording = true;
    } catch (err) {
      alert(`Microphone: ${err}`);
    }
  }

  function send() {
    const text = input.trim();
    if (!text && pendingAttachments.length === 0) return;
    dispatch("send", { text, attachments: pendingAttachments });
    input = "";
    pendingAttachments = [];
  }

  function onKeydown(e: KeyboardEvent) {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      send();
    }
  }
</script>

<div class="composer">
  {#if pendingAttachments.length > 0}
    <div class="chips">
      {#each pendingAttachments as a}
        <span class="chip" title={a.text.slice(0, 300)}>{a.file_name} ({a.kind})</span>
      {/each}
    </div>
  {/if}

  <div class="row">
    <button type="button" class="icon-btn" title="Attach a file" aria-label="Attach a file" on:click={() => fileInput.click()}>
      <Paperclip size={17} aria-hidden="true" />
    </button>
    <button
      type="button"
      class="icon-btn"
      class:active={recording}
      title={recording ? "Recording... click to stop" : "Talk (push-to-talk)"}
      aria-label={recording ? "Stop recording" : "Start voice input"}
      on:click={toggleRecording}
    >
      {#if recording}
        <Square size={16} aria-hidden="true" />
      {:else}
        <Mic size={17} aria-hidden="true" />
      {/if}
    </button>
    <input type="file" bind:this={fileInput} on:change={onFilePicked} style="display: none" multiple />

    <textarea
      bind:value={input}
      {placeholder}
      on:keydown={onKeydown}
      rows="1"
      {autofocus}
    />

    <ModelSelect installed={installedModels} selected={model} {onBrowseMore} on:select={(e) => (model = e.detail)} />

    <button type="button" class="send" on:click={send} disabled={disabled} aria-label="Send message">
      <SendHorizontal size={17} aria-hidden="true" />
    </button>
  </div>
</div>

<style>
  .composer {
    max-width: 720px;
    width: 100%;
    margin: 0 auto;
    background: var(--bg-elevated);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    padding: 0.6rem 0.75rem;
    box-shadow: var(--shadow);
  }
  .chips {
    display: flex;
    gap: 0.5rem;
    padding: 0 0.2rem 0.5rem;
    flex-wrap: wrap;
  }
  .chip {
    font-size: 0.78rem;
    background: var(--bg-sunken);
    border: 1px solid var(--border);
    border-radius: 999px;
    padding: 0.25rem 0.7rem;
    color: var(--text-muted);
  }
  .row {
    display: flex;
    align-items: flex-end;
    gap: 0.5rem;
  }
  textarea {
    flex: 1;
    min-width: 0;
    border: none;
    background: transparent;
    resize: none;
    font-family: inherit;
    font-size: 0.95rem;
    color: var(--text);
    outline: none;
    max-height: 160px;
    padding: 0.4rem 0.2rem;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  textarea:focus {
    white-space: pre-wrap;
    overflow: auto;
  }
  .icon-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    border: none;
    background: transparent;
    border-radius: var(--radius-md);
    padding: 0.5rem;
    color: var(--text-muted);
  }
  .icon-btn.active {
    color: var(--accent-contrast);
    background: var(--accent);
  }
  .send {
    display: flex;
    align-items: center;
    justify-content: center;
    border: none;
    border-radius: var(--radius-md);
    padding: 0.55rem;
    background: var(--accent);
    color: var(--accent-contrast);
  }
  .send:disabled {
    opacity: 0.5;
  }
</style>
