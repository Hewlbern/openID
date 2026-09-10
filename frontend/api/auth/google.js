const {
  googleConfigured,
  googleClientId,
  randomState,
  stateCookie,
  authorizeURL,
  setupHTML,
} = require("../_lib/google");

function cors(res) {
  res.setHeader("Access-Control-Allow-Origin", "*");
  res.setHeader("Access-Control-Allow-Methods", "GET, OPTIONS");
  res.setHeader("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept");
}

module.exports = async function handler(req, res) {
  cors(res);
  if (req.method === "OPTIONS") {
    res.status(204).end();
    return;
  }
  if (req.method !== "GET") {
    res.status(405).json({ error: "GET only" });
    return;
  }
  const q = req.query || {};
  if (q.probe === "1" || String(req.headers.accept || "").includes("application/json")) {
    res.status(200).json({
      configured: googleConfigured(),
      start: "/api/auth/google",
      callback: "/api/auth/google/callback",
      clientIdSet: !!googleClientId(),
    });
    return;
  }
  if (!googleConfigured()) {
    res.setHeader("Content-Type", "text/html; charset=utf-8");
    res.status(503).send(setupHTML());
    return;
  }
  const state = randomState();
  res.setHeader("Set-Cookie", stateCookie(state, 600));
  res.status(302).setHeader("Location", authorizeURL(req, state));
  res.end();
};
