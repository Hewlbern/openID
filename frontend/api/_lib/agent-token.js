/**
 * Short-lived user-scoped agent JWT (megabot agent-token.ts shape).
 * Prefer AGENT_JWT_SECRET (~30m). Else OPENID_SPARK_SECRET / SOLID_TOKEN_SECRET.
 * If none of those are set, return the signed-in session access token (never a service role).
 */
const crypto = require("crypto");
const { AUD, SCOPE, signJwt, verifyJwt, parseSparkToken, isSparkTokenShape, formatTTL, sparkSecret } = require("./jwt");

const TOKEN_PLACEHOLDER = "<TOKEN>";

function agentSecret() {
  return process.env.AGENT_JWT_SECRET
    || process.env.OPENID_SPARK_SECRET
    || process.env.SOLID_TOKEN_SECRET
    || process.env.OPENID_TOKEN_SECRET
    || "";
}

function hasDedicatedAgentSecret() {
  return !!agentSecret();
}

function agentTTLSec() {
  const raw = process.env.OPENID_AGENT_TOKEN_TTL || process.env.AGENT_JWT_TTL;
  if (raw) {
    const n = parseInt(raw, 10);
    if (Number.isFinite(n) && n >= 60 && n <= 30 * 24 * 3600) return n;
  }
  return 30 * 60;
}

function issueDedicatedAgentJwt({ webId, handle, sessionToken, ttlSec }) {
  const secret = agentSecret() || sparkSecret();
  const jti = crypto.randomBytes(16).toString("hex");
  const now = Math.floor(Date.now() / 1000);
  const ttl = ttlSec || agentTTLSec();
  const exp = now + ttl;
  const payload = {
    webid: webId,
    webId,
    handle,
    scope: SCOPE,
    aud: AUD,
    sub: webId,
    jti,
    jti_claim: jti,
    iat: now,
    exp,
    sess: sessionToken,
    kind: "agent",
  };
  const token = signJwt(payload, secret);
  return {
    token,
    jti,
    aud: AUD,
    scope: SCOPE,
    webId,
    expires: new Date(exp * 1000).toISOString(),
    expiresAt: new Date(exp * 1000).toISOString(),
    expiresIn: ttl,
    ttl: formatTTL(ttl),
    tokenKind: process.env.AGENT_JWT_SECRET ? "agent_jwt" : "spark-mcp",
  };
}

function issueAgentSession({ webId, handle, sessionToken }) {
  if (!sessionToken) throw new Error("session required");
  if (!hasDedicatedAgentSecret()) {
    return {
      token: sessionToken,
      tokenKind: "access_token",
      expires: null,
      expiresAt: null,
      expiresIn: null,
      ttl: null,
      webId,
      jti: "",
      aud: AUD,
      scope: SCOPE,
    };
  }
  return issueDedicatedAgentJwt({ webId, handle, sessionToken });
}

function parseAgentToken(token) {
  const secrets = [...new Set([agentSecret(), sparkSecret()].filter(Boolean))];
  let last = new Error("invalid token");
  for (const secret of secrets) {
    try {
      const payload = verifyJwt(token, secret);
      const aud = Array.isArray(payload.aud) ? payload.aud : (payload.aud ? [payload.aud] : []);
      if (payload.scope !== SCOPE && !aud.includes(AUD) && payload.kind !== "agent") {
        throw new Error("not an agent token");
      }
      return {
        webId: payload.webid || payload.webId || payload.sub,
        handle: payload.handle,
        sessionToken: payload.sess,
        jti: payload.jti || payload.jti_claim,
        aud: AUD,
        scope: SCOPE,
        exp: payload.exp,
        tokenKind: payload.kind === "agent" ? "agent_jwt" : "spark-mcp",
      };
    } catch (e) {
      last = e;
    }
  }
  if (isSparkTokenShape(token)) return parseSparkToken(token);
  throw last;
}

function maskPromptToken(text) {
  return String(text || "").replace(/Authorization: Bearer \S+/g, "Authorization: Bearer " + TOKEN_PLACEHOLDER);
}

module.exports = {
  TOKEN_PLACEHOLDER,
  agentSecret,
  hasDedicatedAgentSecret,
  agentTTLSec,
  issueAgentSession,
  issueDedicatedAgentJwt,
  parseAgentToken,
  maskPromptToken,
};
