// Drives the real Runyx web UI with Playwright against a real `runyx
// serve` + real llama-server + real model, and saves the screenshots and
// a short video the demo.yml workflow publishes. No mocked data: every
// screenshot reflects what the running daemon actually returns.
import { chromium } from "playwright";
import fs from "node:fs";
import path from "node:path";

const BASE_URL = process.env.RUNYX_BASE_URL ?? "http://127.0.0.1:11535";
const OUT_DIR = process.env.RUNYX_OUT_DIR ?? "demo-output";
const SHOTS_DIR = path.join(OUT_DIR, "screenshots");
const VIDEO_DIR = path.join(OUT_DIR, "video");
const PROMPT = process.env.RUNYX_PROMPT ?? "Explain in one sentence what you are.";

// A minimal valid 1x1 PNG, used to exercise the real attachment-upload
// pipeline without shipping a binary fixture in the repo.
const TINY_PNG_B64 =
  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=";

function ensureDir(p) {
  fs.mkdirSync(p, { recursive: true });
}

async function killAnimations(page) {
  await page.addStyleTag({
    content: `*, *::before, *::after {
      animation-duration: 0s !important;
      animation-delay: 0s !important;
      transition-duration: 0s !important;
      transition-delay: 0s !important;
      caret-color: transparent !important;
    }`,
  });
}

async function setPrefs(page, theme, reduceMotion) {
  await page.evaluate(
    ({ theme, reduceMotion }) => {
      try {
        localStorage.setItem("runyx.theme", theme);
        localStorage.setItem("runyx.reduceMotion", reduceMotion ? "1" : "0");
      } catch {
        /* ignore */
      }
    },
    { theme, reduceMotion }
  );
}

async function gotoReady(page) {
  await page.goto(BASE_URL, { waitUntil: "load" });
  await killAnimations(page);
  await page.waitForSelector(".rail", { timeout: 15000 });
}

async function openView(page, index) {
  await page.locator(".rail button").nth(index).click();
  await page.waitForTimeout(250);
  await killAnimations(page);
}

async function newConversation(page) {
  // The "+ New conversation" button exists both in the (possibly hidden,
  // on a narrow/phone viewport) sidebar and in the empty-state — pick
  // whichever one is actually visible.
  const btn = page
    .locator("button:visible", { hasText: /New conversation/i })
    .first();
  await btn.click();
  await page.waitForSelector("textarea", { timeout: 10000 });
}

async function sendAndWait(page, text, timeoutMs = 120000) {
  await page.locator("textarea").fill(text);
  await page.locator("button.send").click();
  // Streaming started: a cursor marker appears, then disappears once the
  // reply is persisted back as a normal message bubble.
  await page.waitForSelector(".cursor", { timeout: 15000 }).catch(() => {});
  await page.waitForSelector(".cursor", { state: "detached", timeout: timeoutMs });
  await page.waitForTimeout(400); // let the DOM settle after the final refresh
}

async function shoot(page, file) {
  ensureDir(path.dirname(file));
  await page.screenshot({ path: file });
}

async function runThemePass(browser, theme) {
  const dir = path.join(SHOTS_DIR, theme);
  ensureDir(dir);

  const context = await browser.newContext({
    viewport: { width: 1440, height: 900 },
    reducedMotion: "reduce",
    colorScheme: theme,
  });
  const page = await context.newPage();

  await gotoReady(page);
  await setPrefs(page, theme, true);
  await page.reload({ waitUntil: "load" });
  await killAnimations(page);
  await page.waitForTimeout(300);

  // 1. Empty home / chat.
  await shoot(page, path.join(dir, "01-home-empty.png"));

  // 2. Ctrl+K command palette.
  await page.keyboard.down("Control");
  await page.keyboard.press("k");
  await page.keyboard.up("Control");
  await page.waitForSelector(".palette", { timeout: 5000 });
  await shoot(page, path.join(dir, "02-command-palette.png"));
  await page.keyboard.press("Escape");
  await page.waitForTimeout(150);

  // 3. A real conversation with a completed real model response.
  await newConversation(page);
  await sendAndWait(page, PROMPT);
  await shoot(page, path.join(dir, "03-conversation-response.png"));

  // 4. An attachment added to a message (before it's sent).
  const pngPath = path.join(OUT_DIR, "tmp-attachment.png");
  ensureDir(OUT_DIR);
  fs.writeFileSync(pngPath, Buffer.from(TINY_PNG_B64, "base64"));
  await page.locator("button.attach").click();
  await page.setInputFiles('input[type="file"]', pngPath);
  await page.waitForSelector(".chip", { timeout: 10000 });
  await shoot(page, path.join(dir, "04-attachment.png"));
  await page.locator("button.send").click();
  await page.waitForSelector(".cursor", { state: "detached", timeout: 120000 }).catch(() => {});
  await page.waitForTimeout(300);

  // 5. Models page (installed models list).
  await openView(page, 1);
  await shoot(page, path.join(dir, "05-models.png"));

  // 6. Mobile pairing page, with its QR code.
  await openView(page, 2);
  await page.locator(".page button").click();
  await page.waitForSelector("img.qr", { timeout: 10000 }).catch(() => {});
  await page.waitForTimeout(300);
  await shoot(page, path.join(dir, "06-pairing-qrcode.png"));

  // 7. Settings & permissions center.
  await openView(page, 3);
  await page.waitForTimeout(200);
  await shoot(page, path.join(dir, "07-settings-permissions.png"));

  // 8. Dashboard (RAM/VRAM, loaded models) — dedicated crop of the panel.
  await openView(page, 0);
  await page.waitForTimeout(200);
  const aside = page.locator("aside");
  if (await aside.count()) {
    await aside.first().screenshot({ path: path.join(dir, "08-dashboard.png") });
  }

  await context.close();
}

async function runPhonePass(browser) {
  const dir = SHOTS_DIR;
  const context = await browser.newContext({
    viewport: { width: 390, height: 844 },
    reducedMotion: "reduce",
  });
  const page = await context.newPage();
  await gotoReady(page);
  await setPrefs(page, "light", true);
  await page.reload({ waitUntil: "load" });
  await killAnimations(page);
  await page.waitForTimeout(300);

  await shoot(page, path.join(dir, "phone-01-home.png"));

  await newConversation(page);
  await sendAndWait(page, PROMPT);
  await shoot(page, path.join(dir, "phone-02-chat.png"));

  await context.close();
}

async function recordVideo(browser) {
  ensureDir(VIDEO_DIR);
  const context = await browser.newContext({
    viewport: { width: 1440, height: 900 },
    reducedMotion: "reduce",
    recordVideo: { dir: VIDEO_DIR, size: { width: 1440, height: 900 } },
  });
  const page = await context.newPage();
  await gotoReady(page);
  await setPrefs(page, "light", true);
  await page.reload({ waitUntil: "load" });
  await killAnimations(page);
  await newConversation(page);
  await page.waitForTimeout(500);
  await sendAndWait(page, PROMPT);
  await page.waitForTimeout(800);

  const video = page.video();
  await context.close();
  if (video) {
    const savedPath = await video.path();
    const finalPath = path.join(VIDEO_DIR, "conversation.webm");
    fs.renameSync(savedPath, finalPath);
    console.log(`video saved to ${finalPath}`);
  }
}

async function main() {
  ensureDir(OUT_DIR);
  const launchOpts = {};
  if (process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH) {
    launchOpts.executablePath = process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH;
  }
  const browser = await chromium.launch(launchOpts);
  const errors = [];
  browser.on("disconnected", () => {});

  try {
    for (const theme of ["light", "dark"]) {
      console.log(`== theme: ${theme} ==`);
      await runThemePass(browser, theme);
    }
    console.log("== phone viewport ==");
    await runPhonePass(browser);
    console.log("== conversation video ==");
    await recordVideo(browser);
  } catch (err) {
    errors.push(err);
    console.error(err);
  } finally {
    await browser.close();
  }

  if (errors.length > 0) {
    process.exit(1);
  }
}

main();
