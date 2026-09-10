const { POD } = require("../../_lib/pod");
const {
  googleConfigured,
  readStateCookie,
  stateCookie,
  exchangeCode,
  verifyIdToken,
  derivedPassword,
  handleFromEmail,
  finishHTML,
  setupHTML,
} = require("../../_lib/google");

function html(res, status, body) {
  res.setHeader("Content-Type", "text/html; charset=utf-8");
  res.setHeader("Set-Cookie", stateCookie("", 0));
  res.status(status).send(body);
}

async function podJSON(path, { method = "POST", body } = {}) {
  const res = await fetch(POD + path, {
    method,
    headers: { "Content-Type": "application/json", Accept: "application/json" },
    body: body ? JSON.stringify(body) : undefined,
  });
  const text = await res.text();
  let doc = {};
  try { doc = JSON.parse(text); } catch (e) { doc = { error: text }; }
  return { status: res.status, doc, text };
}

async function loginViaPod(idToken, profile) {
  const google = await podJSON("/idp/google", {
    method: "POST",
    body: { idToken, email: profile.email, name: profile.name, sub: profile.sub },
  });
  if (google.status < 400 && google.doc && google.doc.token) return google.doc;
  if (google.status === 404 || google.status === 405) return null;
  throw new Error(google.doc.error || google.text || ("pod google " + google.status));
}

async function loginViaDerived(profile) {
  const password = derivedPassword(profile.sub);
  const email = profile.email;
  const login = await podJSON("/idp/login", {
    method: "POST",
    body: { email, password },
  });
  if (login.status < 400 && login.doc && login.doc.token) return login.doc;

  let handle = handleFromEmail(email, profile.name);
  for (let i = 0; i < 8; i++) {
    const candidate = i === 0 ? handle : handle.slice(0, 20) + "-" + (i + 1);
    const avail = await fetch(POD + "/idp/handles/" + encodeURIComponent(candidate));
    let info = {};
    try { info = await avail.json(); } catch (e) { info = {}; }
    if (info.available) {
      handle = candidate;
      break;
    }
  }
  const reg = await podJSON("/idp/register", {
    method: "POST",
    body: {
      handle,
      email,
      name: profile.name || handle,
      password,
      createPod: true,
    },
  });
  if (reg.status < 400 && reg.doc && reg.doc.token) return reg.doc;
  if (/account exists|handle taken/i.test(reg.text || "")) {
    throw new Error("This Google email already has an OpenID handle. Sign in with handle + password, then we can link Google after the pod /idp/google route is live.");
  }
  throw new Error(reg.doc.error || reg.text || ("register " + reg.status));
}

module.exports = async function handler(req, res) {
  if (req.method !== "GET") {
    res.status(405).end("GET only");
    return;
  }
  if (!googleConfigured()) {
    html(res, 503, setupHTML());
    return;
  }
  const q = req.query || {};
  if (q.error) {
    html(res, 400, finishHTML({ error: q.error_description || q.error }));
    return;
  }
  const code = String(q.code || "").trim();
  const state = String(q.state || "").trim();
  const expect = readStateCookie(req);
  if (!code) {
    html(res, 400, finishHTML({ error: "Missing Google authorization code." }));
    return;
  }
  if (!state || !expect || state !== expect) {
    html(res, 400, finishHTML({ error: "Google sign-in state mismatch. Try Continue with Google again." }));
    return;
  }
  try {
    const tokens = await exchangeCode(req, code);
    const profile = await verifyIdToken(tokens.id_token);
    let session = null;
    try {
      session = await loginViaPod(tokens.id_token, profile);
    } catch (e) {
      const msg = String(e.message || e);
      if (!/not found|404|Cannot POST|method not allowed|pod google/i.test(msg)) {
        throw e;
      }
    }
    if (!session) session = await loginViaDerived(profile);
    if (!session || !session.token) throw new Error("Google sign-in did not return a session token.");
    html(res, 200, finishHTML({ token: session.token, next: "/app" }));
  } catch (e) {
    html(res, 400, finishHTML({ error: String(e.message || e) }));
  }
};
