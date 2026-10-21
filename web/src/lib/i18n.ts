export type Lang = "fr" | "en";

const dict: Record<Lang, Record<string, string>> = {
  fr: {
    newChat: "Nouvelle conversation",
    conversations: "Conversations",
    send: "Envoyer",
    placeholder: "Écrivez un message...",
    pin: "Épingler",
    unpin: "Désépingler",
    export: "Exporter",
    search: "Rechercher...",
    dashboard: "Tableau de bord",
    model: "Modèle",
    tokensPerSec: "tokens/s",
    loadedModels: "Modèles chargés",
    ram: "RAM",
    vram: "VRAM",
    backend: "Backend",
    commandPalette: "Palette de commandes",
    theme: "Thème",
    light: "Clair",
    dark: "Sombre",
    system: "Système",
    attachFile: "Joindre un fichier",
    noConversations: "Aucune conversation. Commencez à écrire !",
    editAndFork: "Modifier (crée une branche)",
    branches: "Branches",
    reduceMotion: "Réduire les animations",
  },
  en: {
    newChat: "New conversation",
    conversations: "Conversations",
    send: "Send",
    placeholder: "Type a message...",
    pin: "Pin",
    unpin: "Unpin",
    export: "Export",
    search: "Search...",
    dashboard: "Dashboard",
    model: "Model",
    tokensPerSec: "tokens/s",
    loadedModels: "Loaded models",
    ram: "RAM",
    vram: "VRAM",
    backend: "Backend",
    commandPalette: "Command palette",
    theme: "Theme",
    light: "Light",
    dark: "Dark",
    system: "System",
    attachFile: "Attach a file",
    noConversations: "No conversations yet. Start typing!",
    editAndFork: "Edit (forks a branch)",
    branches: "Branches",
    reduceMotion: "Reduce motion",
  },
};

export function detectLang(): Lang {
  const stored = safeGet("runyx.lang");
  if (stored === "fr" || stored === "en") return stored;
  return navigator.language.toLowerCase().startsWith("fr") ? "fr" : "en";
}

export function t(lang: Lang, key: string): string {
  return dict[lang][key] ?? key;
}

function safeGet(key: string): string | null {
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
}
