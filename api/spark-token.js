/**
 * Vercel-minted 30-day Spark connect tokens (aud=spark-mcp, scope=spark).
 * Embeds the session Bearer so /api/mcp can write LDP to Railway without
 * Railway understanding spark JWTs.
 */
const { accountMe, bearer } = require("./_lib/pod");
const { parseSparkToken, AUD, SCOPE } = require("./_lib/jwt");
const { mintSparkConnect, grantsPath, loadJSON, saveJSON, mcpUrlFromReq } = require("./_lib/spark-mint");

function cors(res) {
  res.setHeader("Access-Control-Allow-Origin", "*");
  res.setHeader("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS");
  res.setHeader("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept");
}

function revokedPath(handle) {
  return "/" + handle + "/.openid/spark-revoked.json";
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
    const mcpUrl = mcpUrlFromReq(req);

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
      const minted = await mintSparkConnect(session, req);
      res.status(200).json(minted);
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
