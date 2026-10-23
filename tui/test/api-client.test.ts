// Same regression shape as web/src/lib/api.test.ts: the TUI's client
// talks to the identical /v1/chat/completions proxy, so it needs the
// same guarantee that a well-formed SSE stream's promise actually
// resolves (an unbounded/never-ending stream is the underlying bug
// behind the demo CI's "cursor never detaches" failure — see
// docs/DECISIONS.md).
import { afterEach, describe, expect, test } from "bun:test";
import { RunyxClient, ApiError } from "../src/api/client.js";

const originalFetch = globalThis.fetch;
afterEach(() => {
  globalThis.fetch = originalFetch;
});

function sseResponse(lines: string[]): Response {
  const encoder = new TextEncoder();
  const body = new ReadableStream<Uint8Array>({
    start(controller) {
      for (const line of lines) controller.enqueue(encoder.encode(line));
      controller.close();
    },
  });
  return new Response(body, { status: 200 });
}

describe("RunyxClient.streamChat", () => {
  test("resolves on [DONE], collecting every delta", async () => {
    let capturedBody: any;
    globalThis.fetch = (async (_url: string, init?: RequestInit) => {
      capturedBody = JSON.parse(init!.body as string);
      return sseResponse([
        'data: {"choices":[{"delta":{"content":"Hel"}}]}\n\n',
        'data: {"choices":[{"delta":{"content":"lo"}}]}\n\n',
        "data: [DONE]\n\n",
      ]);
    }) as unknown as typeof fetch;

    const client = new RunyxClient("http://127.0.0.1:11535");
    let collected = "";
    await client.streamChat("qwen", [{ role: "user", content: "hi" }], (d) => (collected += d));

    expect(collected).toBe("Hello");
    expect(capturedBody.model).toBe("qwen");
    expect(capturedBody.max_tokens).toBeUndefined();
  });

  test("resolves on connection close without an explicit [DONE]", async () => {
    globalThis.fetch = (async () => sseResponse(['data: {"choices":[{"delta":{"content":"ok"}}]}\n\n'])) as unknown as typeof fetch;

    const client = new RunyxClient("http://127.0.0.1:11535");
    let collected = "";
    await client.streamChat("qwen", [{ role: "user", content: "hi" }], (d) => (collected += d));
    expect(collected).toBe("ok");
  });

  test("includes max_tokens/temperature only when provided", async () => {
    let capturedBody: any;
    globalThis.fetch = (async (_url: string, init?: RequestInit) => {
      capturedBody = JSON.parse(init!.body as string);
      return sseResponse(["data: [DONE]\n\n"]);
    }) as unknown as typeof fetch;

    const client = new RunyxClient("http://127.0.0.1:11535");
    await client.streamChat("qwen", [{ role: "user", content: "hi" }], () => {}, undefined, {
      maxTokens: 64,
      temperature: 0,
    });
    expect(capturedBody.max_tokens).toBe(64);
    expect(capturedBody.temperature).toBe(0);
  });

  test("a non-OK response throws ApiError instead of hanging", async () => {
    globalThis.fetch = (async () => new Response("boom", { status: 500 })) as unknown as typeof fetch;
    const client = new RunyxClient("http://127.0.0.1:11535");
    await expect(client.streamChat("qwen", [], () => {})).rejects.toBeInstanceOf(ApiError);
  });
});

describe("RunyxClient JSON helpers", () => {
  test("listModels unwraps the {models:[...]} envelope", async () => {
    globalThis.fetch = (async () =>
      new Response(JSON.stringify({ models: [{ name: "qwen", modified_at: "", size: 1, digest: "x" }] }), {
        status: 200,
      })) as unknown as typeof fetch;
    const client = new RunyxClient("http://127.0.0.1:11535");
    const models = await client.listModels();
    expect(models).toEqual([{ name: "qwen", modified_at: "", size: 1, digest: "x" }]);
  });

  test("a 404 surfaces as ApiError with the status code", async () => {
    globalThis.fetch = (async () => new Response("not found", { status: 404 })) as unknown as typeof fetch;
    const client = new RunyxClient("http://127.0.0.1:11535");
    try {
      await client.listConversations();
      throw new Error("expected rejection");
    } catch (e) {
      expect(e).toBeInstanceOf(ApiError);
      expect((e as ApiError).status).toBe(404);
    }
  });
});
