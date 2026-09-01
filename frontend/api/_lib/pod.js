const POD = (process.env.OPENID_POD || process.env.OPENID_API || "https://pod-production-ebe1.up.railway.app").replace(/\/$/, "");

async function podFetch(path, { method = "GET", token, headers = {}, body } = {}) {
  const url = POD + (path.startsWith("/") ? path : "/" + path);
  const h = { ...headers };
  if (token) h.Authorization = "Bearer " + token;
  const res = await fetch(url, { method, headers: h, body });
  const text = await res.text();
  return { status: res.status, text, headers: res.headers, url };
}

async function ensureContainer(token, path) {
  if (!path.endsWith("/")) path += "/";
  const put = await podFetch(path, {
    method: "PUT",
    token,
    headers: {
      "Content-Type": "text/turtle",
      Link: '<http://www.w3.org/ns/ldp#BasicContainer>; rel="type"',
    },
    body: "# container\n",
  });
  if (put.status === 200 || put.status === 201 || put.status === 204 || put.status === 409) return;
  const got = await podFetch(path, { token, headers: { Accept: "text/turtle, */*" } });
  if (got.status < 400) return;
  throw new Error("ensure container " + path + " -> " + put.status + " " + put.text);
}

async function accountMe(token) {
  const res = await podFetch("/idp/accounts/me", { token, headers: { Accept: "application/json" } });
  if (res.status >= 400) throw new Error("login required: " + res.text);
  const acc = JSON.parse(res.text);
  if (!acc.handle) throw new Error("account missing handle");
  return acc;
}

function bearer(req) {
  const h = req.headers.authorization || req.headers.Authorization || "";
  if (String(h).toLowerCase().startsWith("bearer ")) return String(h).slice(7).trim();
  return "";
}

function requestOrigin(req) {
  if (req.headers["x-forwarded-proto"] && req.headers["x-forwarded-host"]) {
    return req.headers["x-forwarded-proto"] + "://" + req.headers["x-forwarded-host"];
  }
  return req.headers.origin || "";
}

function publicReadACL(resourceURL, ownerWebID) {
  return [
    "@prefix acl: <http://www.w3.org/ns/auth/acl#>.",
    "@prefix foaf: <http://xmlns.com/foaf/0.1/>.",
    "<#owner> a acl:Authorization;",
    `  acl:agent <${ownerWebID}>;`,
    `  acl:accessTo <${resourceURL}>;`,
    "  acl:mode acl:Read, acl:Write, acl:Append, acl:Control.",
    "<#public> a acl:Authorization;",
    "  acl:agentClass foaf:Agent;",
    `  acl:accessTo <${resourceURL}>;`,
    "  acl:mode acl:Read.",
  ].join("\n");
}

module.exports = {
  POD,
  podFetch,
  ensureContainer,
  accountMe,
  bearer,
  requestOrigin,
  publicReadACL,
};
