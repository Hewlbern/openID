/**
 * Authenticated share / revoke via public LDP snapshots (no Railway /conversations).
 */
const { bearer, requestOrigin } = require("./_lib/pod");
const { shareConversation, unshareConversation } = require("./_lib/share");

function cors(res) {
  res.setHeader("Access-Control-Allow-Origin", "*");
  res.setHeader("Access-Control-Allow-Methods", "POST, OPTIONS");
  res.setHeader("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept");
}

module.exports = async function handler(req, res) {
  cors(res);
  if (req.method === "OPTIONS") {
    res.status(204).end();
    return;
  }
  if (req.method !== "POST") {
    res.status(405).json({ error: "POST only" });
    return;
  }
  const token = bearer(req);
  if (!token) {
    res.status(401).json({ error: "Authorization: Bearer required" });
    return;
  }
  const body = typeof req.body === "string" ? JSON.parse(req.body || "{}") : (req.body || {});
  const q = req.query || {};
  const url = req.url ? new URL(req.url, "http://x") : null;
  const pathMatch = url && (
    url.pathname.match(/spark-conversations\/([^/]+)\/(?:share|unshare)/)
    || url.pathname.match(/conversations\/([^/]+)\/(?:share|unshare)/)
  );
  const id = String(body.id || q.id || (pathMatch && pathMatch[1]) || "").trim();
  if (!id) {
    res.status(400).json({ error: "id required" });
    return;
  }
  const revoke = !!(body.revoke || body.action === "unshare" || q.revoke || (url && /\/unshare/.test(url.pathname)));
  try {
    if (revoke) {
      res.status(200).json(await unshareConversation(token, id));
      return;
    }
    const origin = requestOrigin(req) || ("https://" + (req.headers.host || "localhost"));
    res.status(200).json(await shareConversation(token, id, origin));
  } catch (e) {
    res.status(400).json({ error: String(e.message || e) });
  }
};
