const crypto = require("crypto");

function googleClientId() {
  return process.env.GOOGLE_CLIENT_ID
    || process.env.OPENID_GOOGLE_CLIENT_ID
    || process.env.NEXT_PUBLIC_GOOGLE_CLIENT_ID
    || "";
}

function googleClientSecret() {
  return process.env.GOOGLE_CLIENT_SECRET || process.env.OPENID_GOOGLE_CLIENT_SECRET || "";
}

function googleConfigured() {
  return !!(googleClientId() && googleClientSecret());
}

function requestOrigin(req) {
  if (req.headers["x-forwarded-proto"] && req.headers["x-forwarded-host"]) {
    return req.headers["x-forwarded-proto"] + "://" + req.headers["x-forwarded-host"];
  }
  if (req.headers.origin) return String(req.headers.origin).replace(/\/$/, "");
  const proto = req.headers["x-forwarded-proto"] || "https";
  const host = req.headers.host || "askclaw.xyz";
  return proto + "://" + host;
}

function callbackURL(req) {
  const explicit = (process.env.GOOGLE_REDIRECT_URI || "").replace(/\/$/, "");
  if (explicit) return explicit;
  return requestOrigin(req) + "/auth/callback";
}

function randomState() {
  return crypto.randomBytes(24).toString("hex");
}

function stateCookie(value, maxAge) {
  const parts = [
    "openid_google_state=" + value,
    "Path=/",
    "HttpOnly",
    "SameSite=Lax",
    "Max-Age=" + (maxAge == null ? 600 : maxAge),
  ];
  return parts.join("; ");
}

function readStateCookie(req) {
  const raw = req.headers.cookie || "";
  const m = raw.match(/(?:^|;\s*)openid_google_state=([^;]+)/);
  return m ? decodeURIComponent(m[1]) : "";
}

function authorizeURL(req, state) {
  const u = new URL("https://accounts.google.com/o/oauth2/v2/auth");
  u.searchParams.set("client_id", googleClientId());
  u.searchParams.set("redirect_uri", callbackURL(req));
  u.searchParams.set("response_type", "code");
  u.searchParams.set("scope", "openid email profile");
  u.searchParams.set("state", state);
  u.searchParams.set("access_type", "online");
  u.searchParams.set("prompt", "select_account");
  return u.toString();
}

async function exchangeCode(req, code) {
  const body = new URLSearchParams({
    code: String(code || ""),
    client_id: googleClientId(),
    client_secret: googleClientSecret(),
    redirect_uri: callbackURL(req),
    grant_type: "authorization_code",
  });
  const res = await fetch("https://oauth2.googleapis.com/token", {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: body.toString(),
  });
  const text = await res.text();
  let doc = {};
  try { doc = JSON.parse(text); } catch (e) { doc = { error: text }; }
  if (!res.ok || !doc.id_token) {
    throw new Error(doc.error_description || doc.error || ("google token exchange " + res.status));
  }
  return doc;
}

async function verifyIdToken(idToken) {
  const res = await fetch("https://oauth2.googleapis.com/tokeninfo?id_token=" + encodeURIComponent(idToken));
  const text = await res.text();
  let doc = {};
  try { doc = JSON.parse(text); } catch (e) { throw new Error("google tokeninfo: " + text); }
  if (!res.ok) throw new Error(doc.error_description || doc.error || "invalid google id token");
  const aud = doc.aud || doc.azp;
  const expect = googleClientId();
  if (expect && aud !== expect && doc.azp !== expect) {
    throw new Error("google id token audience mismatch");
  }
  const iss = String(doc.iss || "");
  if (iss !== "https://accounts.google.com" && iss !== "accounts.google.com") {
    throw new Error("google id token issuer mismatch");
  }
  if (doc.email_verified === "false" || doc.email_verified === false) {
    throw new Error("google email is not verified");
  }
  if (!doc.sub) throw new Error("google id token missing sub");
  return {
    sub: String(doc.sub),
    email: String(doc.email || "").toLowerCase(),
    name: String(doc.name || doc.given_name || ""),
    picture: String(doc.picture || ""),
    emailVerified: true,
  };
}

function derivedPassword(sub) {
  const secret = googleClientSecret() || process.env.OPENID_GOOGLE_LINK_SECRET || "openid-google-link";
  return crypto.createHmac("sha256", secret).update("openid-google:" + sub).digest("hex");
}

function handleFromEmail(email, name) {
  const local = String(email || "").split("@")[0] || String(name || "");
  let slug = local.toLowerCase().replace(/[^a-z0-9-]/g, "");
  if (slug.length < 2) slug = "g" + crypto.createHash("sha256").update(email || name || "user").digest("hex").slice(0, 10);
  return slug.slice(0, 24);
}

function finishHTML({ token, error, next }) {
  const dest = next || "/app";
  if (error) {
    const q = encodeURIComponent(String(error).slice(0, 180));
    return `<!doctype html><meta charset="utf-8"><title>Google sign-in</title>
<p>Could not finish Google sign-in.</p>
<p>${escapeHtml(error)}</p>
<p><a href="/?google_error=${q}">Back</a></p>`;
  }
  const tok = JSON.stringify(String(token || ""));
  const go = JSON.stringify(dest);
  return `<!doctype html><meta charset="utf-8"><title>Signing in…</title>
<script>
try { localStorage.setItem("openid.token", ${tok}); } catch (e) {}
location.replace(${go});
</script>
<p>Signed in. <a href="${escapeHtml(dest)}">Continue</a></p>`;
}

function escapeHtml(s) {
  return String(s).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
}

function setupHTML() {
  return `<!doctype html><meta charset="utf-8"><title>Google sign-in</title>
<h1>Google sign-in is not configured</h1>
<p>Create a <strong>new</strong> Web OAuth client in Google Cloud project <code>dreammachina</code> (do not edit megabot’s Supabase client).</p>
<ol>
<li>APIs &amp; Services → Credentials → Create credentials → OAuth client ID → Web application.</li>
<li>Authorized JavaScript origins: <code>https://askclaw.xyz</code>, <code>https://identity-two-plum.vercel.app</code>, <code>http://localhost:3000</code>.</li>
<li>Authorized redirect URIs: <code>https://askclaw.xyz/auth/callback</code>, <code>https://identity-two-plum.vercel.app/auth/callback</code>, <code>http://localhost:3000/auth/callback</code>.</li>
<li>Vercel project <code>identity</code> (hewlberns-projects): set <code>GOOGLE_CLIENT_ID</code> and <code>GOOGLE_CLIENT_SECRET</code>.</li>
<li>Railway pod: set <code>GOOGLE_CLIENT_ID</code> (same client) so <code>POST /idp/google</code> can verify ID tokens.</li>
</ol>
<p><a href="/">Back</a></p>`;
}

module.exports = {
  googleClientId,
  googleClientSecret,
  googleConfigured,
  requestOrigin,
  callbackURL,
  randomState,
  stateCookie,
  readStateCookie,
  authorizeURL,
  exchangeCode,
  verifyIdToken,
  derivedPassword,
  handleFromEmail,
  finishHTML,
  setupHTML,
};
