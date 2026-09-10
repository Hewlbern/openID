import assert from "node:assert/strict";
import { test } from "node:test";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { createRequire } from "node:module";

const root = join(dirname(fileURLToPath(import.meta.url)), "../..");
const require = createRequire(import.meta.url);
const { buildAgentPrompt, buildLlmsTxt, sanitizeSource, publicOrigin } = require(join(root, "api/_lib/agent-prompt.js"));
const { issueAgentToken, parseSparkToken, resolveSessionToken, agentTTLSec } = require(join(root, "api/_lib/jwt.js"));

test("public agent prompt has protocol, auth, MCP, and no invented token", () => {
  const prompt = buildAgentPrompt({ origin: "https://askclaw.xyz" });
  assert.match(prompt, /Help me save conversation traces/);
  assert.match(prompt, /https:\/\/askclaw\.xyz\/llms\.txt/);
  assert.match(prompt, /GET https:\/\/askclaw\.xyz\/api\/agent\/prompt/);
  assert.match(prompt, /POST https:\/\/askclaw\.xyz\/api\/agent\/session/);
  assert.match(prompt, /do not call this yourself/);
  assert.match(prompt, /Authorization: Bearer $/m);
  assert.match(prompt, /spark_save_conversation/);
  assert.match(prompt, /On HTTP 401/);
  assert.doesNotMatch(prompt, /service.role|SUPABASE_SERVICE/i);
  assert.doesNotMatch(prompt, /megabot\.tech/);
});

test("session prompt embeds the minted Bearer", () => {
  const prompt = buildAgentPrompt({ origin: "https://askclaw.xyz", token: "tok_abc" });
  assert.match(prompt, /Authorization: Bearer tok_abc/);
});

test("llms.txt documents MCP, agent APIs, and Google", () => {
  const txt = buildLlmsTxt("https://askclaw.xyz");
  assert.match(txt, /# OpenID/);
  assert.match(txt, /https:\/\/askclaw\.xyz\/mcp/);
  assert.match(txt, /GET \/api\/agent\/prompt/);
  assert.match(txt, /POST \/api\/agent\/session/);
  assert.match(txt, /GET \/api\/auth\/google/);
  assert.match(txt, /spark_save_conversation/);
  assert.doesNotMatch(txt, /search_robots|vibeHardware/);
});

test("sanitizeSource keeps known tools and defaults", () => {
  assert.equal(sanitizeSource("Codex"), "codex");
  assert.equal(sanitizeSource("cursor!!"), "cursor");
  assert.equal(sanitizeSource(""), "gemini-spark");
});

test("publicOrigin prefers OPENID_PUBLIC_URL", () => {
  const prev = process.env.OPENID_PUBLIC_URL;
  process.env.OPENID_PUBLIC_URL = "https://askclaw.xyz/";
  try {
    assert.equal(publicOrigin({ headers: { host: "localhost:3000" } }), "https://askclaw.xyz");
  } finally {
    if (prev == null) delete process.env.OPENID_PUBLIC_URL;
    else process.env.OPENID_PUBLIC_URL = prev;
  }
});

test("agent JWT is a short-lived spark-mcp token that unwraps to the session", () => {
  const minted = issueAgentToken({
    webId: "https://pod.example/ada/profile/card#me",
    handle: "ada",
    sessionToken: "session-bearer",
  });
  assert.equal(minted.tokenKind, "spark-mcp");
  assert.ok(minted.expiresIn <= 2 * 3600);
  assert.ok(minted.expiresIn >= 60);
  const parsed = parseSparkToken(minted.token);
  assert.equal(parsed.handle, "ada");
  assert.equal(parsed.sessionToken, "session-bearer");
  assert.equal(resolveSessionToken(minted.token), "session-bearer");
  assert.equal(resolveSessionToken("plain-session"), "plain-session");
});

test("default agent TTL is two hours", () => {
  const prev = process.env.OPENID_AGENT_TOKEN_TTL;
  delete process.env.OPENID_AGENT_TOKEN_TTL;
  try {
    assert.equal(agentTTLSec(), 7200);
  } finally {
    if (prev != null) process.env.OPENID_AGENT_TOKEN_TTL = prev;
  }
});

test("app.js remints agent session on copy", () => {
  const app = readFileSync(join(root, "web/static/app.js"), "utf8");
  assert.match(app, /\/api\/agent\/session/);
  assert.match(app, /Copy agent prompt/);
  assert.match(app, /mintAndCopyAgentPrompt/);
});

test("landing and dashboard offer Continue with Google", () => {
  const index = readFileSync(join(root, "web/static/index.html"), "utf8");
  const dash = readFileSync(join(root, "web/static/dash.html"), "utf8");
  assert.match(index, /Continue with Google/);
  assert.match(index, /\/api\/auth\/google/);
  assert.match(dash, /Continue with Google/);
  assert.match(dash, /\/api\/auth\/google/);
});
