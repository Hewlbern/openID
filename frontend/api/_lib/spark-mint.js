/**
 * Mint + persist 30-day Spark connect tokens (same as POST /api/spark-token).
 * Used by the HTTP route and by spark_login / spark_register on /mcp.
 */
const { podFetch, accountMe, ensureContainer, requestOrigin } = require("./pod");
const { issueSparkToken, AUD, SCOPE } = require("./jwt");

const SPARK_LOGIN_HINT =
  "Pass token as `token` on later spark_* calls, OR tell the user to paste it into the Claude connector Authorization header as Bearer <token>.";

function grantsPath(handle) {
  return "/" + handle + "/.openid/spark-grants.json";
}

function originFromReq(req) {
  if (!req || !req.headers) return "https://identity-two-plum.vercel.app";
  const fromFwd = requestOrigin(req);
  if (fromFwd) return fromFwd.replace(/\/$/, "");
  const host = req.headers.host || req.headers.Host || "";
  if (host) {
    const proto = req.headers["x-forwarded-proto"] || "https";
    return proto + "://" + host;
  }
  return "https://identity-two-plum.vercel.app";
}

function mcpUrlFromReq(req) {
  return originFromReq(req) + "/mcp";
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

async function persistGrant(session, handle, webId, minted) {
  const file = await loadJSON(session, grantsPath(handle), { grants: [] });
  file.grants = file.grants || [];
  file.grants.push({
    jti: minted.jti,
    webId,
    issued: new Date().toISOString(),
    expires: minted.expires,
    revoked: false,
  });
  await saveJSON(session, handle, grantsPath(handle), file);
}

/**
 * Mint a 30-day Spark connect token from a session Bearer, persist the grant,
 * and return the spark_login response shape (never includes a password).
 */
async function mintSparkConnect(session, req) {
  const acc = await accountMe(session);
  const minted = issueSparkToken({
    webId: acc.webId,
    handle: acc.handle,
    sessionToken: session,
  });
  await persistGrant(session, acc.handle, acc.webId, minted);
  return {
    ok: true,
    handle: acc.handle,
    webId: acc.webId,
    mcpUrl: mcpUrlFromReq(req),
    token: minted.token,
    expires: minted.expires,
    tokenType: "Bearer",
    hint: SPARK_LOGIN_HINT,
    jti: minted.jti,
    aud: AUD,
    scope: SCOPE,
    expiresIn: minted.expiresIn,
    ttl: minted.ttl,
  };
}

module.exports = {
  AUD,
  SCOPE,
  SPARK_LOGIN_HINT,
  grantsPath,
  originFromReq,
  mcpUrlFromReq,
  loadJSON,
  saveJSON,
  persistGrant,
  mintSparkConnect,
};
