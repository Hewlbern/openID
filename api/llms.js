const { publicOrigin, buildLlmsTxt } = require("./_lib/agent-prompt");

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
    res.status(405).end("GET only");
    return;
  }
  res.setHeader("Content-Type", "text/plain; charset=utf-8");
  res.setHeader("Cache-Control", "public, max-age=60");
  res.status(200).send(buildLlmsTxt(publicOrigin(req)));
};
