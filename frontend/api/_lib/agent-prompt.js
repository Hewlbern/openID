/**
 * Copyable OpenID agent instructions (Codex / Cursor / Claude / Grok / Spark).
 * Public GET mints no token. Signed-in POST /api/agent/session fills Bearer.
 */

function publicOrigin(req) {
  const env = (process.env.OPENID_PUBLIC_URL || process.env.OPENID_APP_URL || "").replace(/\/$/, "");
  if (env) return env;
  if (req && req.headers) {
    const proto = req.headers["x-forwarded-proto"] || "https";
    const host = req.headers["x-forwarded-host"] || req.headers.host;
    if (host) return proto + "://" + host;
    if (req.headers.origin) return String(req.headers.origin).replace(/\/$/, "");
  }
  return "https://askclaw.xyz";
}

function sanitizeSource(raw) {
  const s = String(raw || "").toLowerCase().replace(/[^a-z0-9-]/g, "").slice(0, 40);
  return s || "gemini-spark";
}

function buildAgentPrompt({ origin, token } = {}) {
  const base = String(origin || "https://askclaw.xyz").replace(/\/$/, "");
  const bearer = token ? String(token) : "";
  return [
    "Help me save conversation traces to my OpenID Solid pod. If it is unclear what I want, ask me before you write or share.",
    "",
    "OpenID is Solid Protocol identity for AI agents — a WebID, a personal pod, and MCP so Codex / Cursor / Claude / Grok / Gemini Spark can write threads into conversations/spark/.",
    "App URL: " + base + "/",
    "",
    "## Protocol",
    "GET " + base + "/llms.txt",
    "GET " + base + "/api/agent/prompt (current instructions, no token minted)",
    "POST " + base + "/api/agent/session (signed-in browser mint only — do not call this yourself)",
    "",
    "## Auth",
    "Send this header on user-scoped MCP and conversation APIs:",
    "Authorization: Bearer " + bearer,
    "Keep this token private (short-lived, aud=spark-mcp). On HTTP 401, ask me to copy a new OpenID / agent prompt.",
    "Never invent credentials, WebIDs, client secrets, or tokens.",
    "",
    "## Read",
    "- Human passport: " + base + "/app",
    "- GET " + base + "/api/spark-conversations — list {handle}/conversations/spark/",
    "- GET " + base + "/api/spark-conversations is owner-only; there is no public library listing",
    "- Remote MCP: " + base + "/mcp (alias " + base + "/api/mcp)",
    "- MCP tools (only these): spark_save_conversation, spark_list_conversations, spark_get_conversation, spark_share_conversation, spark_unshare_conversation",
    "",
    "## Write",
    "- Prefer MCP spark_save_conversation with title and messages: [{role, content|text, timestamp?}]. Optional source (codex|cursor|claude|grok|gemini-spark).",
    "- Or POST " + base + "/api/spark-conversations { title?, messages?, text?, source?, source_url? }",
    "- POST " + base + "/api/spark-conversations/{id}/share — mint a public /share/c/{token} snapshot",
    "- POST " + base + "/api/spark-conversations/{id}/unshare — revoke that snapshot",
    "Sequential single-op commits: save one conversation, wait for resourceUrl, then list or share. Do not invent ids or overwrite another handle's pod.",
    "Writes land at {handle}/conversations/spark/{id}.json (JSON-LD) plus {id}.ttl. Containers conversations/ and conversations/spark/ are created first.",
    "",
    "## Agent recipe",
    "1. Confirm I want this thread (or a named past thread) stored on OpenID.",
    "2. Call spark_save_conversation (or POST /api/spark-conversations) with the full transcript. Set source to the tool I am in (codex, cursor, claude, grok, gemini-spark).",
    "3. Read resourceUrl / confirmation. Optionally spark_share_conversation if I ask to share.",
    "4. Tell me what you saved and send " + base + "/app",
    "On 401, stop and ask me to open /app and copy a fresh Open / agent prompt. Do not retry with a guessed Bearer.",
    "",
    "When you are done, tell me what you saved and send:",
    base + "/app",
  ].join("\n");
}

function buildLlmsTxt(origin) {
  const base = String(origin || "https://askclaw.xyz").replace(/\/$/, "");
  return `# OpenID — Solid identity for AI agents

> OpenID gives each person and agent a WebID, a Solid pod, and a tamper-evident audit trail. The hosted passport at ${base} lets you mint short-lived MCP tokens and save Codex / Cursor / Claude / Grok / Gemini Spark traces into \`{handle}/conversations/spark/\`.

This is not a robot workshop. Do not invent credentials, WebIDs, or tokens.

## Base URL
${base}

## Remote MCP (Cursor / Codex / Claude / Grok / Gemini Spark)

Canonical Streamable HTTP endpoint:

- ${base}/mcp
- Alias: ${base}/api/mcp

CORS: \`*\`. Auth: \`Authorization: Bearer <token>\` from a copied OpenID / agent prompt (\`POST /api/agent/session\`) or a 30-day Spark connect token (\`POST /api/spark-token\`).

### Tools
- \`spark_save_conversation\` — write the current thread to the caller's pod. Args: title, messages [{role, content|text, timestamp?}], optional source / source_url / text.
- \`spark_list_conversations\` — list \`{handle}/conversations/spark/\`.
- \`spark_get_conversation\` — read one saved conversation by id.
- \`spark_share_conversation\` — mint a public \`/share/c/{token}\` snapshot.
- \`spark_unshare_conversation\` — revoke that snapshot.

A Spark connect token (\`aud: spark-mcp\`, \`scope: spark\`) can only call these \`spark_*\` tools and read/write **your** \`{handle}/conversations/\` container. It cannot mint more tokens or write another pod.

## Agent prompt APIs

### GET /api/agent/prompt
Current copyable agent instructions. No token is minted. CORS \`*\`.

### POST /api/agent/session
Signed-in browser only (session Bearer or \`solid-session\` cookie). Returns \`{ prompt, token, expiresAt, tokenKind }\`. Token is user-scoped and short-lived (default 2 hours, \`OPENID_AGENT_TOKEN_TTL\` seconds). Same JWT machinery as Spark connect tokens (\`aud=spark-mcp\`) with an embedded session grant so hosted MCP can write LDP. Never a service-role key. The /app copy dock remints on each copy.

### GET /llms.txt
This document.

## Conversation APIs (owner-only, CORS *)

Send \`Authorization: Bearer <token>\` from a copied OpenID prompt, or use the signed-in cookie / session Bearer.

- \`GET /api/spark-conversations\` — list saved traces
- \`POST /api/spark-conversations\` — create from \`{ title?, messages?, text?, source?, source_url? }\`
- \`POST /api/spark-conversations/{id}/share\`
- \`POST /api/spark-conversations/{id}/unshare\`
- \`GET /share/c/{token}\` — public read-only snapshot (logged-out)

Spark tokens are unwrapped to the session Bearer before LDP writes. On 401, ask the owner to copy a new OpenID / agent prompt.

## Spark connect tokens

- \`POST /api/spark-token\` — session required → 30-day \`{ token, expires, jti, mcpUrl, webId }\`
- \`GET /api/spark-token\` — list active grants (no secret)
- \`DELETE /api/spark-token\` — revoke all, or \`?jti=\`

Prefer the short-lived agent session for Codex / Cursor. Use the 30-day Spark token for a standing Gemini Spark connection.

## Identity

- \`POST /idp/register\` — handle + password (creates WebID + pod)
- \`POST /idp/login\` — handle or email + password
- \`GET /api/auth/google\` — Continue with Google (Vercel OAuth; lands on /app)
- \`POST /idp/google\` — pod-side Google ID token exchange (Railway)
- \`GET /idp/accounts/me\` — current account

## Agent recipe

1. \`GET ${base}/llms.txt\` and \`GET ${base}/api/agent/prompt\` if you need fresh instructions.
2. Use the Bearer from the prompt the human copied (do not call \`POST /api/agent/session\` yourself).
3. \`spark_save_conversation\` (or \`POST /api/spark-conversations\`) with the full transcript. Set \`source\` to the tool you are (codex, cursor, claude, grok, gemini-spark).
4. Sequential single-op commits. Wait for \`resourceUrl\`.
5. Send the human ${base}/app (and a share URL only if they asked).
6. On 401, ask them to remint from /app.

## Human UI

- Passport: ${base}/app — signed-in save / list / share, Spark connect, **Open / agent prompt** copy dock
- Landing: ${base}/ — claim a handle or Continue with Google
- Records: ${base}/records
- Public handle: ${base}/i/{handle}
- Solid server console: ${base}/dashboard
`;
}

module.exports = {
  publicOrigin,
  sanitizeSource,
  buildAgentPrompt,
  buildLlmsTxt,
};
