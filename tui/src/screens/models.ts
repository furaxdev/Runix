// Models screen: a scrollable, read-only view of the curated registry
// (/api/runyx/registry) merged with install state and hardware fit --
// the same data the web UI's Models page renders as cards. Pulling a
// model (a streamed NDJSON download) is not wired up yet; see
// TUI_REPORT.md.
import type { Terminal } from "../term/terminal.js";
import type { Screen } from "../term/buffer.js";
import type { Rect } from "../layout/flex.js";
import { insetRect } from "../layout/flex.js";
import { drawBox } from "../widgets/box.js";
import { drawLine } from "../widgets/text.js";
import { clampScroll, scrollBy, visibleRange, type RowHeights } from "../layout/scroll.js";
import type { RunyxClient } from "../api/client.js";
import type { RegistryEntry } from "../api/types.js";

export interface ModelsScreenResult {
  action: "back";
}

function fitLabel(fit: RegistryEntry["fit"]): string {
  switch (fit) {
    case "comfortable":
      return "comfortable";
    case "ok":
      return "ok";
    case "tight":
      return "tight";
    case "too_large":
      return "too large";
    default:
      return fit;
  }
}

export async function runModelsScreen(terminal: Terminal, client: RunyxClient): Promise<ModelsScreenResult> {
  let entries: RegistryEntry[] = [];
  let error: string | null = null;
  let scroll = { topIndex: 0, topSkip: 0 };

  async function load(): Promise<void> {
    try {
      entries = await client.listRegistry();
      error = null;
    } catch (e) {
      error = (e as Error).message ?? String(e);
    }
    terminal.requestRender();
  }

  function draw(screen: Screen): void {
    const theme = terminal.theme;
    const full: Rect = { row: 0, col: 0, width: screen.width, height: screen.height };
    const inner = drawBox(screen, full, { title: "Models", borderFg: theme.border, fg: theme.accent });
    const body: Rect = { ...insetRect(inner, 0), height: Math.max(0, inner.height - 1) };
    const footer: Rect = { row: inner.row + inner.height - 1, col: inner.col, width: inner.width, height: 1 };

    if (error) {
      drawLine(screen, body, body.row, `error: ${error}`, { fg: theme.danger });
      return;
    }

    const rows: RowHeights = { count: entries.length, heightOf: () => 1 };
    scroll = clampScroll(scroll, rows, body.height);
    const visible = visibleRange(scroll, rows, body.height);
    for (const v of visible) {
      const m = entries[v.index]!;
      const sizeGB = (m.size_bytes / 1e9).toFixed(1);
      const flag = m.installed ? "[installed]" : "";
      const line = `${m.name.padEnd(28)} ${sizeGB.padStart(6)}GB  ${fitLabel(m.fit).padEnd(11)} ${flag}`;
      drawLine(screen, body, body.row + v.screenRow, line, { fg: m.installed ? theme.success : theme.text }, true);
    }
    drawLine(screen, footer, footer.row, "Esc/q: back  Up/Down: scroll  r: refresh", { fg: theme.textMuted });
  }

  terminal.setDrawFn(draw);
  await load();

  return new Promise<ModelsScreenResult>((resolve) => {
    const onKey = (e: { key: string; ctrl: boolean }) => {
      if (e.key === "Escape" || e.key === "q") {
        terminal.off("key", onKey);
        resolve({ action: "back" });
        return;
      }
      if (e.key === "r") void load();
      else if (e.key === "Up") scroll = scrollBy(scroll, { count: entries.length, heightOf: () => 1 }, 1, -1);
      else if (e.key === "Down") scroll = scrollBy(scroll, { count: entries.length, heightOf: () => 1 }, 1, 1);
      terminal.requestRender();
    };
    terminal.on("key", onKey);
    terminal.requestRender();
  });
}
