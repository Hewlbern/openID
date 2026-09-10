/**
 * Signed-in mint: copyable agent prompt + short-lived spark-mcp Bearer.
 * Remints on each call (same manner as megabot POST /api/agent/session).
 */
const { accountMe, bearer, requestOrigin, ensureContainer, podFetch } = require("../_lib/pod");
const { issueAgentToken, parseSparkToken, AUD, SCOPE } = require("../_lib/jwt");
const { publicOrigin, buildAgentPrompt } = require("../_lib/agent-prompt");

function cors(res) {
  res.setHeader("Access-Control-Allow-Origin", "*");
  res.setHeader("Access-Control-Allow-Methods", "POST, OPTIONS");
  res.setHeader("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept");
}

function grantsPath(handle) {
  return "/" + handle + "/.openid/spark-grants.json";
}

async function loadJSON(token, path, fallback) {
  const got = await podFetch(path, { token, headers: { Accept: "application/json" } });
  if (got.status >= 400) return fallback;
  try { return JSON.parse(got.text); } catch (e) { return fallback; }
}

async function saveJSON(token, handle, path, doc) {
  await ensureContainer(token, "/" + handle + "/.openid/");
  const put = await podFetch(path, {
    method: "PUT",
    token,
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(doc, null, 2),
  });
  if (put.status >= 400) throw new Error("persist " + path + " " + put.status + " " + put.text);
}

module.exports = async function handler(req, res) {
  cors(res);
  if (req.method === "OPTIONS") {
    res.status(204).end();
    return;
  }
  if (req.method !== "POST") {
    res.status(405).json({ error: "POST only (signed-in browser mint)" });
    return;
  }
  const session = bearer(req);
  if (!session) {
    res.status(401).json({ error: "Authorization: Bearer required (full session, not a spark token)" });
    return;
  }
  try {
    parseSparkToken(session);
    res.status(403).json({ error: "spark connect token cannot mint agent sessions" });
    return;
  } catch (e) {
    /* session bearer — ok */
  }

  try {
    const acc = await accountMe(session);
    const origin = publicOrigin(req) || requestOrigin(req);
    const minted = issueAgentToken({
      webId: acc.webId,
      handle: acc.handle,
      sessionToken: session,
    });
    const file = await loadJSON(session, grantsPath(acc.handle), { grants: [] });
    file.grants = file.grants || [];
    file.grants.push({
      jti: minted.jti,
      webId: acc.webId,
      issued: new Date().toISOString(),
      expires: minted.expires,
      revoked: false,
      kind: "agent",
    });
    await saveJSON(session, acc.handle, grantsPath(acc.handle), file);
    const prompt = buildAgentPrompt({ origin, token: minted.token });
    res.status(200).json({
      prompt,
      token: minted.token,
      expiresAt: minted.expires,
      tokenKind: minted.tokenKind || "spark-mcp",
      tokenType: "Bearer",
      jti: minted.jti,
      aud: AUD,
      scope: SCOPE,
      webId: acc.webId,
      mcpUrl: String(origin || "").replace(/\/$/, "") + "/mcp",
      ttl: minted.ttl,
    });
  } catch (e) {
    const msg = String(e.message || e);
    res.status(/login required|unauthorized/i.test(msg) ? 401 : 400).json({ error: msg });
  }
};
