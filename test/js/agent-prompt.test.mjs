import assert from "node:assert/strict";
import { test } from "node:test";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { createRequire } from "node:module";

const root = join(dirname(fileURLToPath(import.meta.url)), "../..");
const require = createRequire(import.meta.url);
const { buildAgentPrompt, buildLlmsTxt, sanitizeSource, publicOrigin, TOKEN_PLACEHOLDER } = require(join(root, "api/_lib/agent-prompt.js"));
const { resolveSessionToken } = require(join(root, "api/_lib/jwt.js"));
const {
  issueAgentSession,
  parseAgentToken,
  maskPromptToken,
  agentTTLSec,
  TOKEN_PLACEHOLDER: AGENT_PLACEHOLDER,
} = require(join(root, "api/_lib/agent-token.js"));

function withEnv(vars, fn) {
  const prev = {};
  for (const [k, v] of Object.entries(vars)) {
    prev[k] = process.env[k];
    if (v == null) delete process.env[k];
    else process.env[k] = v;
  }
  try { return fn(); }
  finally {
    for (const [k, v] of Object.entries(prev)) {
      if (v == null) delete process.env[k];
      else process.env[k] = v;
    }
  }
}

test("public agent prompt has protocol, <TOKEN> placeholder, and no minted Bearer", () => {
  const prompt = buildAgentPrompt({ origin: "https://askclaw.xyz" });
  assert.match(prompt, /Help me save conversation traces/);
  assert.match(prompt, /https:\/\/askclaw\.xyz\/llms\.txt/);
  assert.match(prompt, /GET https:\/\/askclaw\.xyz\/api\/agent\/prompt/);
  assert.match(prompt, /POST https:\/\/askclaw\.xyz\/api\/agent\/session/);
  assert.match(prompt, /do not call this yourself/);
  assert.match(prompt, /Authorization: Bearer <TOKEN>/);
  assert.equal(TOKEN_PLACEHOLDER, "<TOKEN>");
  assert.match(prompt, /spark_save_conversation/);
  assert.match(prompt, /On HTTP 401/);
  assert.doesNotMatch(prompt, /service.role|SUPABASE_SERVICE/i);
  assert.doesNotMatch(prompt, /megabot\.tech/);
});

test("session prompt embeds the minted Bearer", () => {
  const prompt = buildAgentPrompt({ origin: "https://askclaw.xyz", token: "tok_abc" });
  assert.match(prompt, /Authorization: Bearer tok_abc/);
  assert.equal(maskPromptToken(prompt), prompt.replace("tok_abc", AGENT_PLACEHOLDER));
});

test("llms.txt documents MCP, agent APIs, 30m JWT, and Google callback", () => {
  const txt = buildLlmsTxt("https://askclaw.xyz");
  assert.match(txt, /# OpenID/);
  assert.match(txt, /https:\/\/askclaw\.xyz\/mcp/);
  assert.match(txt, /GET \/api\/agent\/prompt/);
  assert.match(txt, /POST \/api\/agent\/session/);
  assert.match(txt, /GET \/api\/auth\/google/);
  assert.match(txt, /\/auth\/callback/);
  assert.match(txt, /AGENT_JWT_SECRET/);
  assert.match(txt, /spark_save_conversation/);
  assert.match(txt, /Authorization: Bearer <TOKEN>/);
  assert.doesNotMatch(txt, /search_robots|vibeHardware/);
});

test("sanitizeSource keeps known tools and defaults", () => {
  assert.equal(sanitizeSource("Codex"), "codex");
  assert.equal(sanitizeSource("cursor!!"), "cursor");
  assert.equal(sanitizeSource(""), "gemini-spark");
});

test("publicOrigin prefers OPENID_PUBLIC_URL", () => {
  withEnv({ OPENID_PUBLIC_URL: "https://askclaw.xyz/" }, () => {
    assert.equal(publicOrigin({ headers: { host: "localhost:3000" } }), "https://askclaw.xyz");
  });
});

test("without AGENT_JWT_SECRET, session mint returns the access token", () => {
  withEnv({
    AGENT_JWT_SECRET: null,
    OPENID_SPARK_SECRET: null,
    SOLID_TOKEN_SECRET: null,
    OPENID_TOKEN_SECRET: null,
  }, () => {
    const minted = issueAgentSession({
      webId: "https://pod.example/ada/profile/card#me",
      handle: "ada",
      sessionToken: "session-bearer",
    });
    assert.equal(minted.token, "session-bearer");
    assert.equal(minted.tokenKind, "access_token");
    assert.equal(minted.jti, "");
  });
});

test("AGENT_JWT_SECRET mints a ~30m user-scoped JWT that unwraps to the session", () => {
  withEnv({
    AGENT_JWT_SECRET: "test-agent-secret",
    OPENID_SPARK_SECRET: null,
    SOLID_TOKEN_SECRET: null,
    OPENID_TOKEN_SECRET: null,
    OPENID_AGENT_TOKEN_TTL: null,
    AGENT_JWT_TTL: null,
  }, () => {
    assert.equal(agentTTLSec(), 1800);
    const minted = issueAgentSession({
      webId: "https://pod.example/ada/profile/card#me",
      handle: "ada",
      sessionToken: "session-bearer",
    });
    assert.equal(minted.tokenKind, "agent_jwt");
    assert.equal(minted.expiresIn, 1800);
    const parsed = parseAgentToken(minted.token);
    assert.equal(parsed.handle, "ada");
    assert.equal(parsed.sessionToken, "session-bearer");
    assert.equal(resolveSessionToken(minted.token), "session-bearer");
    assert.equal(resolveSessionToken("plain-session"), "plain-session");
  });
});

test("yellow PROMPT dock remints on signed-in copy and opens sign-in when signed out", () => {
  const dock = readFileSync(join(root, "web/static/agent-dock.js"), "utf8");
  assert.match(dock, /PROMPT/);
  assert.match(dock, /library-dock-chip-kicker/);
  assert.match(dock, /\/api\/agent\/session/);
  assert.match(dock, /openidOpenSignIn/);
  assert.match(dock, /Sign in to copy/);
  assert.match(dock, /Authorization: Bearer <TOKEN>/);
  assert.match(dock, /Session expired/);
});

test("app.js remints agent session on copy and asks for a fresh prompt on 401", () => {
  const app = readFileSync(join(root, "web/static/app.js"), "utf8");
  assert.match(app, /\/api\/agent\/session/);
  assert.match(app, /Copy agent prompt/);
  assert.match(app, /mintAndCopyAgentPrompt/);
  assert.match(app, /copy a fresh prompt/);
});

test("landing and dashboard offer Continue with Google; callback is /auth/callback", () => {
  const index = readFileSync(join(root, "web/static/index.html"), "utf8");
  const dash = readFileSync(join(root, "web/static/dash.html"), "utf8");
  const vercel = readFileSync(join(root, "vercel.json"), "utf8");
  const google = readFileSync(join(root, "api/_lib/google.js"), "utf8");
  assert.match(index, /Continue with Google/);
  assert.match(index, /\/api\/auth\/google/);
  assert.match(index, /agent-dock\.js/);
  assert.match(dash, /Continue with Google/);
  assert.match(dash, /\/api\/auth\/google/);
  assert.match(dash, /agent-dock\.js/);
  assert.match(vercel, /\/auth\/callback/);
  assert.match(google, /\/auth\/callback/);
});
