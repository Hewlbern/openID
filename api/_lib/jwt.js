const crypto = require("crypto");

const AUD = "spark-mcp";
const SCOPE = "spark";

function sparkSecret() {
  return process.env.OPENID_SPARK_SECRET
    || process.env.SOLID_TOKEN_SECRET
    || process.env.OPENID_TOKEN_SECRET
    || "openid-vercel-spark-preview";
}

function b64url(input) {
  return Buffer.from(input).toString("base64").replace(/=/g, "").replace(/\+/g, "-").replace(/\//g, "_");
}

function signJwt(payload, secret) {
  const header = b64url(JSON.stringify({ alg: "HS256", typ: "JWT" }));
  const body = b64url(JSON.stringify(payload));
  const data = header + "." + body;
  const sig = crypto.createHmac("sha256", secret).update(data).digest("base64")
    .replace(/=/g, "").replace(/\+/g, "-").replace(/\//g, "_");
  return data + "." + sig;
}

function verifyJwt(token, secret) {
  const parts = String(token || "").split(".");
  if (parts.length !== 3) throw new Error("invalid token");
  const data = parts[0] + "." + parts[1];
  const expect = crypto.createHmac("sha256", secret).update(data).digest("base64")
    .replace(/=/g, "").replace(/\+/g, "-").replace(/\//g, "_");
  const a = Buffer.from(expect);
  const b = Buffer.from(parts[2]);
  if (a.length !== b.length || !crypto.timingSafeEqual(a, b)) throw new Error("invalid token signature");
  const payload = JSON.parse(Buffer.from(parts[1].replace(/-/g, "+").replace(/_/g, "/"), "base64").toString("utf8"));
  const now = Math.floor(Date.now() / 1000);
  if (payload.exp && now >= payload.exp) throw new Error("token expired");
  return payload;
}

function issueSparkToken({ webId, handle, sessionToken, ttlSec }) {
  const secret = sparkSecret();
  const jti = crypto.randomBytes(16).toString("hex");
  const now = Math.floor(Date.now() / 1000);
  const ttl = ttlSec || 30 * 24 * 3600;
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
  };
  // jwt registered claim "jti" via standard field
  payload.jti = jti;
  const token = signJwt(payload, secret);
  return {
    token,
    jti,
    aud: AUD,
    scope: SCOPE,
    webId,
    expires: new Date(exp * 1000).toISOString(),
    expiresIn: ttl,
    ttl: "720h0m0s",
  };
}

function parseSparkToken(token) {
  const payload = verifyJwt(token, sparkSecret());
  const aud = Array.isArray(payload.aud) ? payload.aud : (payload.aud ? [payload.aud] : []);
  if (payload.scope !== SCOPE && !aud.includes(AUD)) throw new Error("not a spark token");
  return {
    webId: payload.webid || payload.webId || payload.sub,
    handle: payload.handle,
    sessionToken: payload.sess,
    jti: payload.jti || payload.jti_claim,
    aud: AUD,
    scope: SCOPE,
    exp: payload.exp,
  };
}

function isSparkTokenShape(token) {
  try {
    const parts = String(token || "").split(".");
    if (parts.length < 2) return false;
    const p = JSON.parse(Buffer.from(parts[1].replace(/-/g, "+").replace(/_/g, "/"), "base64").toString("utf8"));
    const aud = Array.isArray(p.aud) ? p.aud : (p.aud ? [p.aud] : []);
    return p.scope === SCOPE || aud.includes(AUD);
  } catch (e) {
    return false;
  }
}

module.exports = {
  AUD,
  SCOPE,
  sparkSecret,
  signJwt,
  verifyJwt,
  issueSparkToken,
  parseSparkToken,
  isSparkTokenShape,
};
