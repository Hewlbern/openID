/**
 * Public share page / JSON. Token format: {handle}.{random}
 * Snapshot lives at /{handle}/public/shares/{random}.json (world-readable).
 */
const { POD, podFetch } = require("./_lib/pod");

function cors(res) {
  res.setHeader("Access-Control-Allow-Origin", "*");
  res.setHeader("Access-Control-Allow-Methods", "GET, OPTIONS");
  res.setHeader("Access-Control-Allow-Headers", "Accept, Content-Type");
}

function escapeHtml(s) {
  return String(s || "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
}

function renderHTML(c) {
  const turns = (c.messages || []).map((m) => {
    const role = escapeHtml(m.role || "user");
    const text = escapeHtml(m.text || m.content || "").replace(/\n/g, "<br />");
    return `<div class="turn ${role}"><span class="who">${role}</span><div class="bubble">${text}</div></div>`;
  }).join("");
  return `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>${escapeHtml(c.title || "Shared")} — oid</title>
  <link rel="icon" href="/static/favicon.svg" type="image/svg+xml" />
  <link rel="stylesheet" href="/static/styles.css" />
</head>
<body>
  <header class="title">
    <a class="brand" href="/"><span class="mark" aria-hidden="true"></span><span class="app-name">oid</span></a>
    <span class="status">shared conversation</span>
  </header>
  <article class="share-page">
    <p class="lede">Read-only copy from an OpenID pod.</p>
    <h1>${escapeHtml(c.title || "Conversation")}</h1>
    <div class="chips">
      <span>${escapeHtml(c.source || "gemini-spark")}</span>
      <span>${escapeHtml(String(c.updated || c.created || "").slice(0, 25))}</span>
    </div>
    ${turns || "<p class=\"hint\">No messages.</p>"}
    <p class="legal">Saved on OpenID. Sharing can be revoked by the owner.</p>
  </article>
</body>
</html>`;
}

function parseToken(raw) {
  const token = String(raw || "").trim();
  const i = token.indexOf(".");
  if (i < 1 || i === token.length - 1) return null;
  return { handle: token.slice(0, i), random: token.slice(i + 1), token };
}

module.exports = async function handler(req, res) {
  cors(res);
  if (req.method === "OPTIONS") {
    res.status(204).end();
    return;
  }
  if (req.method !== "GET") {
    res.status(405).end("GET only");
    return;
  }
  const q = req.query || {};
  const url = req.url ? new URL(req.url, "http://x") : null;
  const pathMatch = url && url.pathname.match(/\/share\/c\/([^/]+)/);
  const raw = q.token || (url && url.searchParams.get("token")) || (pathMatch && pathMatch[1]) || "";
  const parsed = parseToken(decodeURIComponent(raw));
  if (!parsed) {
    res.status(404).json({ error: "share not found" });
    return;
  }
  const path = "/" + parsed.handle + "/public/shares/" + parsed.random + ".json";
  const got = await podFetch(path, { headers: { Accept: "application/json" } });
  if (got.status >= 400) {
    res.status(404).json({ error: "share not found" });
    return;
  }
  let doc;
  try { doc = JSON.parse(got.text); } catch (e) {
    res.status(404).json({ error: "share not found" });
    return;
  }
  const accept = String(req.headers.accept || "");
  if (accept.includes("application/json") && !accept.includes("text/html")) {
    res.status(200).json(doc);
    return;
  }
  res.setHeader("Content-Type", "text/html; charset=utf-8");
  res.status(200).end(renderHTML(doc));
};

module.exports.POD = POD;
