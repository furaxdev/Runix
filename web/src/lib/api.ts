// Thin client for Runyx's own API (/api/runyx/*) and the Ollama/OpenAI
// surfaces, used by the web UI.

export interface Conversation {
  ID: string;
  Title: string;
  Model: string;
  Folder: string;
  Pinned: boolean;
  ProjectID: string;
  CreatedAt: string;
  UpdatedAt: string;
}

export interface Message {
  ID: string;
  ConversationID: string;
  ParentID: string;
  BranchID: string;
  Role: string;
  Content: string;
  CreatedAt: string;
}

export interface DashboardStats {
  loaded_models: { name: string; port: number; idle_seconds: number }[];
  total_ram_mb: number;
  total_vram_mb: number;
  backend: string;
}

export interface ModelEntry {
  name: string;
  modified_at: string;
  size: number;
  digest: string;
}

export interface DeviceInfo {
  id: string;
  name: string;
  created_at: string;
}

export interface PermissionsSnapshot {
  shell_agent: {
    enabled: boolean;
    work_dir: string;
    allow_prefixes: string[];
    deny_prefixes: string[];
  };
  vision_enabled: boolean;
  voice_enabled: boolean;
  web_search_on: boolean;
  devices: DeviceInfo[];
}

async function json<T>(res: Response): Promise<T> {
  if (!res.ok) {
    const body = await res.text().catch(() => "");
    throw new Error(`HTTP ${res.status}: ${body}`);
  }
  return res.json() as Promise<T>;
}

export const api = {
  listConversations: () => fetch("/api/runyx/conversations").then((r) => json<Conversation[]>(r)),
  newConversation: (title: string, model: string) =>
    fetch("/api/runyx/conversations", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ Title: title, Model: model }),
    }).then((r) => json<Conversation>(r)),
  messages: (id: string, branch = "main") =>
    fetch(`/api/runyx/conversations/${id}/messages?branch=${encodeURIComponent(branch)}`).then((r) =>
      json<Message[]>(r)
    ),
  addMessage: (id: string, role: string, content: string, branch = "main", parentId = "") =>
    fetch(`/api/runyx/conversations/${id}/messages`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ Role: role, Content: content, Branch: branch, ParentID: parentId }),
    }).then((r) => json<Message>(r)),
  pin: (id: string, pinned: boolean) =>
    fetch(`/api/runyx/conversations/${id}/pin`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ Pinned: pinned }),
    }),
  search: (q: string) => fetch(`/api/runyx/search?q=${encodeURIComponent(q)}`).then((r) => json<Message[]>(r)),
  dashboard: () => fetch("/api/runyx/dashboard").then((r) => json<DashboardStats>(r)),
  models: () => fetch("/v1/models").then((r) => json<{ data: { id: string }[] }>(r)),
  tags: () => fetch("/api/tags").then((r) => json<{ models: ModelEntry[] }>(r)),
  pull: (name: string) =>
    fetch("/api/pull", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name }),
    }).then((r) => json<{ status: string }>(r)),
  permissions: () => fetch("/api/runyx/permissions").then((r) => json<PermissionsSnapshot>(r)),
  revokeDevice: (id: string) => fetch(`/api/runyx/permissions/devices/${id}`, { method: "DELETE" }),
  pairingQRCode: async (deviceName: string): Promise<{ ok: true; url: string } | { ok: false; error: string }> => {
    const res = await fetch("/api/runyx/pairing/qrcode", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ DeviceName: deviceName }),
    });
    if (!res.ok) {
      const body = await res.json().catch(() => ({ error: `HTTP ${res.status}` }));
      return { ok: false, error: body.error ?? `HTTP ${res.status}` };
    }
    const blob = await res.blob();
    return { ok: true, url: URL.createObjectURL(blob) };
  },
  transcribeAudio: (blob: Blob): Promise<{ text: string; warnings?: string[] }> => {
    const form = new FormData();
    form.append("file", blob, "recording.webm");
    return fetch("/api/runyx/voice/transcribe", { method: "POST", body: form }).then((r) => json(r));
  },
  synthesizeSpeech: async (text: string): Promise<Blob> => {
    const res = await fetch("/api/runyx/voice/synthesize", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ Text: text }),
    });
    if (!res.ok) throw new Error(`synthesize failed: HTTP ${res.status}`);
    return res.blob();
  },
  uploadAttachment: (file: File) => {
    const form = new FormData();
    form.append("file", file);
    return fetch("/api/runyx/attachments", { method: "POST", body: form }).then((r) =>
      json<{ file_name: string; kind: string; text: string; warnings?: string[] }>(r)
    );
  },

  // Streams assistant text via the OpenAI-compatible endpoint, calling
  // onDelta for each token chunk.
  async streamChat(
    model: string,
    messages: { role: string; content: string }[],
    onDelta: (text: string) => void,
    signal?: AbortSignal
  ): Promise<void> {
    const res = await fetch("/v1/chat/completions", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ model, messages, stream: true }),
      signal,
    });
    if (!res.ok || !res.body) {
      throw new Error(`chat request failed: HTTP ${res.status}`);
    }
    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buf = "";
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buf += decoder.decode(value, { stream: true });
      const lines = buf.split("\n");
      buf = lines.pop() ?? "";
      for (const line of lines) {
        if (!line.startsWith("data: ")) continue;
        const data = line.slice(6);
        if (data === "[DONE]") return;
        try {
          const chunk = JSON.parse(data);
          const delta = chunk.choices?.[0]?.delta?.content;
          if (delta) onDelta(delta);
        } catch {
          // ignore malformed keep-alive lines
        }
      }
    }
  },
};
