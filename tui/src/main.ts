#!/usr/bin/env node
// Entry point: boots the terminal, makes sure a daemon is reachable
// (starting one if needed, see src/api/daemon.ts), picks a model to
// chat with, and runs the Chat screen until the user quits.
import { Terminal } from "./term/terminal.js";
import { ensureDaemon, DaemonUnavailableError } from "./api/daemon.js";
import { RunyxClient } from "./api/client.js";
import { runChatScreen } from "./screens/chat.js";

async function pickModel(client: RunyxClient): Promise<string | null> {
  const models = await client.listModels().catch(() => []);
  return models[0]?.name ?? null;
}

async function main(): Promise<void> {
  let baseUrl: string;
  try {
    baseUrl = await ensureDaemon();
  } catch (e) {
    if (e instanceof DaemonUnavailableError) {
      process.stderr.write(`runyx-tui: ${e.message}\n`);
      process.exitCode = 1;
      return;
    }
    throw e;
  }

  const client = new RunyxClient(baseUrl);
  const model = await pickModel(client);
  if (!model) {
    process.stderr.write("runyx-tui: no model installed yet. Run `runyx pull <model>` first.\n");
    process.exitCode = 1;
    return;
  }

  const terminal = new Terminal();
  await terminal.start();
  try {
    await runChatScreen(terminal, client, { model });
  } finally {
    terminal.stop();
  }
}

main().catch((err) => {
  process.stderr.write(`runyx-tui: fatal: ${err instanceof Error ? err.stack ?? err.message : String(err)}\n`);
  process.exitCode = 1;
});
