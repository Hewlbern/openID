/**
 * Same-origin save/list for /app when Railway lacks /conversations and the
 * Vercel catch-all rewrite 404s browser LDP PUTs to /{handle}/conversations/.
 * Talks to the Railway Solid pod directly (PUT BasicContainer, then PUT .json/.ttl).
 */
const POD = (process.env.OPENID_POD || process.env.OPENID_API || "https://pod-production-ebe1.up.railway.app").replace(/\/$/, "");

function cors(res) {
  res.setHeader("Access-Control-Allow-Origin", "*");
  res.setHeader("Access-Control-Allow-Methods", "GET, POST, OPTIONS");
  res.setHeader("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept");
}

function bearer(req) {
  const h = req.headers.authorization || req.headers.Authorization || "";
  if (String(h).toLowerCase().startsWith("bearer ")) return String(h).slice(7).trim();
  return "";
}

async function podFetch(path, { method = "GET", token, headers = {}, body } = {}) {
  const url = POD + (path.startsWith("/") ? path : "/" + path);
  const h = { ...headers };
  if (token) h.Authorization = "Bearer " + token;
  const res = await fetch(url, { method, headers: h, body });
  const text = await res.text();
  return { status: res.status, text, headers: res.headers };
}

/** LDP PUT BasicContainer. 201/200/204/409 OK if a later GET succeeds. */
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
  if (put.status === 200 || put.status === 201 || put.status === 204 || put.status === 409) {
    return;
  }
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

function normalizeMessages(inMsgs, text) {
  let messages = [];
  for (const m of inMsgs || []) {
    const t = String((m && (m.text || m.content)) || "").trim();
    if (!t) continue;
    const item = { role: m.role || "user", text: t };
    const ts = m.timestamp || m.time || m.created;
    if (ts) item.timestamp = String(ts);
    messages.push(item);
  }
  if (!messages.length && text) {
    const raw = String(text);
    try {
      const parsed = JSON.parse(raw);
      if (Array.isArray(parsed)) return normalizeMessages(parsed, "");
      if (parsed && Array.isArray(parsed.messages)) return normalizeMessages(parsed.messages, "");
    } catch (e) {
      /* paste */
    }
    const turn = /^\s*(?:\*{0,2})(user|human|you|assistant|gemini|spark|model|system)(?:\*{0,2})\s*:\s*(?:\*{0,2})\s*(.*)$/i;
    let cur = null;
    for (const line of raw.split("\n")) {
      const m = line.match(turn);
      if (m) {
        if (cur && cur.text.trim()) messages.push(cur);
        const role = /assistant|gemini|spark|model/i.test(m[1]) ? "assistant" : /system/i.test(m[1]) ? "system" : "user";
        cur = { role, text: m[2] || "" };
        continue;
      }
      if (cur) cur.text += (cur.text ? "\n" : "") + line;
    }
    if (cur && cur.text.trim()) messages.push(cur);
    if (!messages.length && raw.trim()) messages.push({ role: "user", text: raw.trim() });
  }
  return messages.map((m) => {
    const item = { role: m.role, text: String(m.text || "").trim() };
    if (m.timestamp) item.timestamp = String(m.timestamp);
    return item;
  }).filter((m) => m.text);
}

function turtle(resourceUrl, title, id, webId, now, msgs) {
  let ttl = `@prefix schema: <https://schema.org/> .
@prefix dcterms: <http://purl.org/dc/terms/> .
@prefix foaf: <http://xmlns.com/foaf/0.1/> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

<${resourceUrl}> a schema:Conversation ;
  schema:name ${JSON.stringify(title)} ;
  schema:identifier ${JSON.stringify(id)} ;
  dcterms:created ${JSON.stringify(now)}^^xsd:dateTime ;
  dcterms:modified ${JSON.stringify(now)}^^xsd:dateTime ;
  schema:dateCreated ${JSON.stringify(now)}^^xsd:dateTime ;
  schema:dateModified ${JSON.stringify(now)}^^xsd:dateTime ;
  dcterms:source "gemini-spark"`;
  if (webId) ttl += ` ;\n  schema:creator <${webId}> ;\n  foaf:maker <${webId}>`;
  (msgs || []).forEach((_, i) => { ttl += ` ;\n  schema:hasPart <${resourceUrl}#msg-${i + 1}>`; });
  ttl += " .\n";
  (msgs || []).forEach((m, i) => {
    ttl += `\n<${resourceUrl}#msg-${i + 1}> a schema:Message ;\n  schema:text ${JSON.stringify(m.text)} ;\n  schema:author ${JSON.stringify(m.role)}`;
    if (m.timestamp) ttl += ` ;\n  schema:dateCreated ${JSON.stringify(m.timestamp)}^^xsd:dateTime ;\n  dcterms:created ${JSON.stringify(m.timestamp)}^^xsd:dateTime`;
    if (m.role === "assistant") ttl += ` ;\n  foaf:Agent "gemini-spark"`;
    else if (webId) ttl += ` ;\n  foaf:maker <${webId}>`;
    ttl += " .\n";
  });
  return ttl;
}

async function save(token, body, origin) {
  const acc = await accountMe(token);
  const messages = normalizeMessages(body.messages, body.text);
  if (!messages.length) throw new Error("Paste a transcript, or a public Gemini share URL.");
  const title = String(body.title || messages[0].text).slice(0, 80) || "Untitled conversation";
  const id = (globalThis.crypto && crypto.randomUUID && crypto.randomUUID()) || String(Date.now());
  const now = new Date().toISOString();
  const handle = acc.handle;

  await ensureContainer(token, "/" + handle + "/conversations/");
  await ensureContainer(token, "/" + handle + "/conversations/spark/");

  const resourcePath = "/" + handle + "/conversations/spark/" + id + ".json";
  const ttlPath = "/" + handle + "/conversations/spark/" + id + ".ttl";
  const base = (origin || POD).replace(/\/$/, "");
  const resourceUrl = base + resourcePath;
  const doc = {
    "@context": {
      "@vocab": "https://schema.org/",
      schema: "https://schema.org/",
      dcterms: "http://purl.org/dc/terms/",
      foaf: "http://xmlns.com/foaf/0.1/",
      xsd: "http://www.w3.org/2001/XMLSchema#",
    },
    "@type": "Conversation",
    "@id": resourceUrl,
    id,
    title,
    name: title,
    source: "gemini-spark",
    sourceUrl: body.source_url || "",
    created: now,
    updated: now,
    dateCreated: now,
    dateModified: now,
    messages,
    owner: acc.webId,
    creator: acc.webId,
    podPath: handle + "/",
    resource: handle + "/conversations/spark/" + id + ".json",
    metaTtl: base + ttlPath,
  };

  const putJson = await podFetch(resourcePath, {
    method: "PUT",
    token,
    headers: { "Content-Type": "application/ld+json" },
    body: JSON.stringify(doc, null, 2),
  });
  if (putJson.status >= 400) throw new Error("PUT json " + putJson.status + " " + putJson.text);

  const putTtl = await podFetch(ttlPath, {
    method: "PUT",
    token,
    headers: { "Content-Type": "text/turtle" },
    body: turtle(resourceUrl, title, id, acc.webId, now, messages),
  });
  if (putTtl.status >= 400) throw new Error("PUT ttl " + putTtl.status + " " + putTtl.text);

  return Object.assign({}, doc, {
    ok: true,
    resourceUrl,
    webId: acc.webId,
    confirmation: `Saved “${title}” to your Solid pod as ${resourceUrl}.`,
  });
}

async function list(token) {
  const acc = await accountMe(token);
  const dir = "/" + acc.handle + "/conversations/spark/";
  // Parent may be missing for brand-new pods — ensure before listing.
  await ensureContainer(token, "/" + acc.handle + "/conversations/");
  await ensureContainer(token, dir);
  const listing = await podFetch(dir, { token, headers: { Accept: "text/turtle" } });
  if (listing.status >= 400) return { conversations: [] };
  const ids = [...listing.text.matchAll(/conversations\/spark\/([A-Za-z0-9-]+)\.json/g)].map((m) => m[1]);
  const out = [];
  for (const id of [...new Set(ids)]) {
    const g = await podFetch(dir + id + ".json", { token, headers: { Accept: "application/ld+json, application/json" } });
    if (g.status < 400) {
      try { out.push(JSON.parse(g.text)); } catch (e) { /* skip */ }
    }
  }
  out.sort((a, b) => String(b.updated || b.created || "").localeCompare(String(a.updated || a.created || "")));
  return { conversations: out, path: acc.handle + "/conversations/spark/" };
}

module.exports = async function handler(req, res) {
  cors(res);
  if (req.method === "OPTIONS") {
    res.status(204).end();
    return;
  }
  const token = bearer(req);
  if (!token) {
    res.status(401).json({ error: "Authorization: Bearer required" });
    return;
  }
  try {
    if (req.method === "GET") {
      const doc = await list(token);
      res.status(200).json(doc);
      return;
    }
    if (req.method === "POST") {
      const body = typeof req.body === "string" ? JSON.parse(req.body || "{}") : (req.body || {});
      const origin = (req.headers["x-forwarded-proto"] && req.headers["x-forwarded-host"])
        ? req.headers["x-forwarded-proto"] + "://" + req.headers["x-forwarded-host"]
        : (req.headers.origin || POD);
      const saved = await save(token, body, origin);
      res.status(201).json(saved);
      return;
    }
    res.status(405).json({ error: "GET or POST" });
  } catch (e) {
    res.status(400).json({ error: String(e.message || e) });
  }
};

module.exports.normalizeMessages = normalizeMessages;
module.exports.turtle = turtle;
