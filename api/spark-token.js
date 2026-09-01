/**
 * Vercel-minted 30-day Spark connect tokens (aud=spark-mcp, scope=spark).
 * Embeds the session Bearer so /api/mcp can write LDP to Railway without
 * Railway understanding spark JWTs.
 */
const { podFetch, accountMe, bearer, requestOrigin, ensureContainer } = require("./_lib/pod");
const { issueSparkToken, parseSparkToken, AUD, SCOPE } = require("./_lib/jwt");

function cors(res) {
  res.setHeader("Access-Control-Allow-Origin", "*");
  res.setHeader("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS");
  res.setHeader("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept");
}

function grantsPath(handle) {
  return "/" + handle + "/.openid/spark-grants.json";
}

function revokedPath(handle) {
  return "/" + handle + "/.openid/spark-revoked.json";
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
  const session = bearer(req);
  if (!session) {
    res.status(401).json({ error: "Authorization: Bearer required (full session, not a spark token)" });
    return;
  }
  // Reject using a spark token to mint/revoke
  try {
    parseSparkToken(session);
    res.status(403).json({ error: "spark connect token cannot mint or revoke tokens" });
    return;
  } catch (e) {
    /* session bearer — ok */
  }

  try {
    const acc = await accountMe(session);
    const origin = requestOrigin(req) || ("https://" + (req.headers.host || "localhost"));
    const mcpUrl = origin.replace(/\/$/, "") + "/mcp";

    if (req.method === "GET") {
      const file = await loadJSON(session, grantsPath(acc.handle), { grants: [] });
      const revoked = await loadJSON(session, revokedPath(acc.handle), { jtis: [] });
      const revokedSet = new Set(revoked.jtis || []);
      const now = Date.now();
      const tokens = (file.grants || []).filter((g) => {
        if (!g || g.webId !== acc.webId) return false;
        if (g.revoked || revokedSet.has(g.jti)) return false;
        if (g.expires && Date.parse(g.expires) < now) return false;
        return true;
      }).map((g) => ({
        jti: g.jti,
        expires: g.expires,
        issued: g.issued,
        aud: AUD,
        scope: SCOPE,
      }));
      res.status(200).json({ tokens, mcpUrl, webId: acc.webId, ttl: "720h0m0s" });
      return;
    }

    if (req.method === "POST") {
      const minted = issueSparkToken({
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
      });
      await saveJSON(session, acc.handle, grantsPath(acc.handle), file);
      res.status(200).json(Object.assign({}, minted, {
        tokenType: "Bearer",
        mcpUrl,
      }));
      return;
    }

    if (req.method === "DELETE") {
      const q = req.query || {};
      let jti = String(q.jti || "").trim();
      if (!jti) {
        const body = typeof req.body === "string" ? JSON.parse(req.body || "{}") : (req.body || {});
        jti = String(body.jti || "").trim();
      }
      const file = await loadJSON(session, grantsPath(acc.handle), { grants: [] });
      const revoked = await loadJSON(session, revokedPath(acc.handle), { jtis: [] });
      revoked.jtis = revoked.jtis || [];
      let n = 0;
      for (const g of file.grants || []) {
        if (!g || g.webId !== acc.webId || g.revoked) continue;
        if (jti && g.jti !== jti) continue;
        g.revoked = true;
        if (!revoked.jtis.includes(g.jti)) revoked.jtis.push(g.jti);
        n++;
      }
      await saveJSON(session, acc.handle, grantsPath(acc.handle), file);
      await saveJSON(session, acc.handle, revokedPath(acc.handle), revoked);
      res.status(200).json({ ok: true, revoked: n });
      return;
    }

    res.status(405).json({ error: "GET, POST, or DELETE" });
  } catch (e) {
    res.status(400).json({ error: String(e.message || e) });
  }
};
