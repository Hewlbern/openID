const { publicOrigin, buildAgentPrompt } = require("../_lib/agent-prompt");

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
  const origin = publicOrigin(req);
  res.setHeader("Cache-Control", "public, max-age=30");
  res.status(200).json({
    prompt: buildAgentPrompt({ origin }),
    docs: origin + "/llms.txt",
    mint: "POST /api/agent/session (signed-in browser only)",
    auth: "Authorization: Bearer <TOKEN>",
  });
};
