/**
 * Hosted MCP at /mcp (rewritten here). Spark talks to this origin; we write
 * the conversation into the Railway Solid pod with LDP PUTs to container
 * paths that end in /. Accepts session Bearer or Vercel-minted Spark connect
 * tokens (aud=spark-mcp). Does not scrape Gemini — Spark supplies the thread.
 */
const { POD, podFetch, ensureContainer, accountMe } = require("./_lib/pod");
const { parseSparkToken, isSparkTokenShape } = require("./_lib/jwt");
const { shareConversation, unshareConversation } = require("./_lib/share");
const { mintSparkConnect, originFromReq } = require("./_lib/spark-mint");

const SPARK_INSTRUCTIONS = `Claude / Gemini Spark: when the user says things like "save this conversation", "share these logs", or "log into my OpenID":
1. If you have no token yet (no Authorization: Bearer on this connector and no prior spark_login in this chat), ask the user for their OpenID handle (or email) and password, then call spark_login. Do not send them to /app to mint a token first.
2. Call spark_save_conversation with the full current thread (title + messages[{role,content|text,timestamp?}]) and the token from spark_login (or the existing Bearer).
3. If they asked to share, call spark_share_conversation with the saved id + token and return the /share/c/… URL.
Keep the existing Claude custom-connector Request header path: Authorization: Bearer <token> from /app still works and does not require spark_login.`;

const SPARK_LOGIN = `Log into OpenID with handle (or email) + password inside this chat. POSTs to /idp/login, then mints a 30-day Spark connect token (same as /api/spark-token). Call this FIRST when the user wants to save/share logs and you have no token yet — ask them for handle + password; do not send them to /app. Returns {ok, handle, webId, mcpUrl, token, expires, tokenType:"Bearer", hint}. Never echo the password. On later spark_* calls pass the returned token as \`token\`, or tell the user they may paste Authorization: Bearer <token> into the Claude connector Request headers.`;

const SPARK_REGISTER = `Register a new OpenID handle + password (optional name), then mint a 30-day Spark connect token. Same return shape as spark_login. Use when the user does not have an account yet. Never echo the password.`;

const SPARK_SAVE = `FIRST-CLASS tool for Claude / Gemini Spark. When the user asks to save, upload, store, share, or write this chat / thread / conversation / logs to their Solid pod or OpenID, you MUST call this with the full current conversation. If you have no token yet, call spark_login first (ask for handle + password in chat) — do not tell them to leave the chat or paste into /app. Then pass title, the complete messages array as [{role, content|text, timestamp?}], and token. Include timezone-aware ISO-8601 timestamps when known. Spark/Claude supplies the transcript; do not scrape Gemini. Writes conversations/spark/{id}.json (JSON-LD) plus a Turtle sidecar. Returns resourceUrl, webId, optional shareUrl, created/modified, and confirmation text to show the user. After a successful save, if they asked to share, call spark_share_conversation and show the /share/c/… URL.`;

const SPARK_SHARE = `Mint a stable /share/c/… URL for a saved conversation. Call this after spark_save_conversation when the user asked to share these logs. Pass id (from save) and token. Returns shareUrl — show that URL to the user. Default is unlisted (secret token). Set public=true for a public link.`;

function sparkTools() {
  const token = { type: "string", description: "Optional if Authorization: Bearer is already set on /mcp. After spark_login, pass the returned token here on later spark_* calls. Also accepts a 30-day Spark connect token minted on /app via /api/spark-token." };
  const id = { type: "string", description: "Conversation id returned by spark_save_conversation" };
  const handle = { type: "string", description: "OpenID handle (or pass email instead)" };
  const email = { type: "string", description: "Email if the account was registered with one" };
  const password = { type: "string", description: "Account password. Never stored or echoed by this tool." };
  return [
    {
      name: "spark_login",
      description: SPARK_LOGIN,
      inputSchema: {
        type: "object",
        properties: { handle, email, password },
        required: ["password"],
      },
    },
    {
      name: "spark_register",
      description: SPARK_REGISTER,
      inputSchema: {
        type: "object",
        properties: {
          handle,
          password,
          name: { type: "string", description: "Optional display name" },
          email,
        },
        required: ["handle", "password"],
      },
    },
    {
      name: "spark_save_conversation",
      description: SPARK_SAVE,
      inputSchema: {
        type: "object",
        properties: {
          title: { type: "string", description: "Conversation title" },
          messages: {
            type: "array",
            description: "Full current thread: role, content or text, optional timestamp",
            items: {
              type: "object",
              properties: {
                role: { type: "string" },
                text: { type: "string" },
                content: { type: "string" },
                timestamp: { type: "string" },
                time: { type: "string" },
              },
            },
          },
          source_url: { type: "string" },
          text: { type: "string" },
          token,
        },
      },
    },
    { name: "spark_list_conversations", description: "List Spark conversations saved in the caller's pod.", inputSchema: { type: "object", properties: { token } } },
    { name: "spark_get_conversation", description: "Read one saved Spark conversation.", inputSchema: { type: "object", properties: { id, token }, required: ["id"] } },
    { name: "spark_share_conversation", description: SPARK_SHARE, inputSchema: { type: "object", properties: { id, public: { type: "boolean" }, token }, required: ["id"] } },
    { name: "spark_unshare_conversation", description: "Revoke the share link.", inputSchema: { type: "object", properties: { id, token }, required: ["id"] } },
  ];
}

function cors(res) {
  res.setHeader("Access-Control-Allow-Origin", "*");
  res.setHeader("Access-Control-Allow-Methods", "GET, POST, OPTIONS");
  res.setHeader("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept, Mcp-Session-Id");
  res.setHeader("Access-Control-Expose-Headers", "Mcp-Session-Id");
}

function isSparkToken(token) {
  return isSparkTokenShape(token);
}

function bearer(req, args) {
  const fromArgs = args && typeof args.token === "string" ? args.token.trim() : "";
  if (fromArgs) return fromArgs;
  const h = req.headers.authorization || req.headers.Authorization || "";
  if (String(h).toLowerCase().startsWith("bearer ")) return String(h).slice(7).trim();
  return "";
}

/** Resolve Spark connect token → session Bearer for Railway LDP. */
async function resolveLdpToken(token) {
  if (!token) throw new Error("login required");
  if (!isSparkTokenShape(token)) return token;
  let spark;
  try {
    spark = parseSparkToken(token);
  } catch (e) {
    throw new Error("spark connect token invalid: " + e.message);
  }
  if (!spark.sessionToken) throw new Error("spark connect token missing session grant");
  const revoked = await podFetch("/" + spark.handle + "/.openid/spark-revoked.json", {
    token: spark.sessionToken,
    headers: { Accept: "application/json" },
  });
  if (revoked.status < 400) {
    try {
      const doc = JSON.parse(revoked.text);
      if ((doc.jtis || []).includes(spark.jti)) throw new Error("spark connect token revoked");
    } catch (e) {
      if (/revoked/.test(e.message)) throw e;
    }
  }
  return spark.sessionToken;
}

function normalizeMessages(inMsgs) {
  const out = [];
  for (const m of inMsgs || []) {
    const text = String((m && (m.text || m.content)) || "").trim();
    if (!text) continue;
    const item = { role: (m.role || "user"), text };
    const ts = m.timestamp || m.time || m.created;
    if (ts) item.timestamp = String(ts);
    out.push(item);
  }
  return out;
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
  msgs.forEach((_, i) => { ttl += ` ;\n  schema:hasPart <${resourceUrl}#msg-${i + 1}>`; });
  ttl += " .\n";
  msgs.forEach((m, i) => {
    ttl += `\n<${resourceUrl}#msg-${i + 1}> a schema:Message ;\n  schema:text ${JSON.stringify(m.text)} ;\n  schema:author ${JSON.stringify(m.role)}`;
    if (m.timestamp) ttl += ` ;\n  schema:dateCreated ${JSON.stringify(m.timestamp)}^^xsd:dateTime ;\n  dcterms:created ${JSON.stringify(m.timestamp)}^^xsd:dateTime`;
    if (m.role === "assistant") ttl += ` ;\n  foaf:Agent "gemini-spark"`;
    else if (webId) ttl += ` ;\n  foaf:maker <${webId}>`;
    ttl += " .\n";
  });
  return ttl;
}

async function sparkSave(args, token) {
  token = await resolveLdpToken(token);
  const acc = await accountMe(token);
  let messages = normalizeMessages(args.messages);
  if (!messages.length && args.text) messages = [{ role: "user", text: String(args.text) }];
  if (!messages.length) throw new Error("messages or text required");
  const title = String(args.title || messages[0].text).slice(0, 80);
  const id = (globalThis.crypto && crypto.randomUUID && crypto.randomUUID()) || String(Date.now());
  const now = new Date().toISOString();
  const handle = acc.handle;
  await ensureContainer(token, "/" + handle + "/conversations/");
  await ensureContainer(token, "/" + handle + "/conversations/spark/");
  const resourcePath = "/" + handle + "/conversations/spark/" + id + ".json";
  const ttlPath = "/" + handle + "/conversations/spark/" + id + ".ttl";
  const resourceUrl = POD + resourcePath;
  const doc = {
    "@context": {
      "@vocab": "https://schema.org/",
      schema: "https://schema.org/",
      dcterms: "http://purl.org/dc/terms/",
      foaf: "http://xmlns.com/foaf/0.1/",
      xsd: "http://www.w3.org/2001/XMLSchema#",
      created: { "@id": "dcterms:created", "@type": "xsd:dateTime" },
      updated: { "@id": "dcterms:modified", "@type": "xsd:dateTime" },
      dateCreated: { "@id": "schema:dateCreated", "@type": "xsd:dateTime" },
      dateModified: { "@id": "schema:dateModified", "@type": "xsd:dateTime" },
    },
    "@type": "Conversation",
    "@id": resourceUrl,
    id,
    title,
    name: title,
    source: "gemini-spark",
    sourceUrl: args.source_url || "",
    created: now,
    updated: now,
    dateCreated: now,
    dateModified: now,
    messages,
    owner: acc.webId,
    creator: acc.webId,
    podPath: handle + "/",
    resource: handle + "/conversations/spark/" + id + ".json",
    metaTtl: POD + ttlPath,
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
  return {
    ok: true,
    id,
    title,
    resourceUrl,
    metaTtlUrl: POD + ttlPath,
    webId: acc.webId,
    pod: POD + "/" + handle + "/",
    source: "gemini-spark",
    created: now,
    modified: now,
    messageCount: messages.length,
    confirmation: `Saved “${title}” to your Solid pod as ${resourceUrl} (${messages.length} messages, source=gemini-spark). WebID ${acc.webId}. Created ${now}.`,
    conversation: doc,
  };
}

async function sparkList(token) {
  token = await resolveLdpToken(token);
  const api = await podFetch("/conversations", { token, headers: { Accept: "application/json" } });
  if (api.status < 400) {
    try { return JSON.parse(api.text); } catch (e) { return { text: api.text }; }
  }
  const acc = await accountMe(token);
  await ensureContainer(token, "/" + acc.handle + "/conversations/");
  await ensureContainer(token, "/" + acc.handle + "/conversations/spark/");
  const listing = await podFetch("/" + acc.handle + "/conversations/spark/", {
    token,
    headers: { Accept: "text/turtle" },
  });
  if (listing.status >= 400) return { conversations: [] };
  const ids = [...listing.text.matchAll(/conversations\/spark\/([A-Za-z0-9-]+)\.json/g)].map((m) => m[1]);
  const out = [];
  for (const id of [...new Set(ids)]) {
    const g = await podFetch("/" + acc.handle + "/conversations/spark/" + id + ".json", {
      token,
      headers: { Accept: "application/ld+json, application/json" },
    });
    if (g.status < 400) {
      try { out.push(JSON.parse(g.text)); } catch (e) { /* skip */ }
    }
  }
  return { conversations: out };
}

async function sparkGet(token, id) {
  token = await resolveLdpToken(token);
  const api = await podFetch("/conversations/" + encodeURIComponent(id), { token, headers: { Accept: "application/json" } });
  if (api.status < 400) {
    try { return JSON.parse(api.text); } catch (e) { return { text: api.text }; }
  }
  const acc = await accountMe(token);
  const got = await podFetch("/" + acc.handle + "/conversations/spark/" + id + ".json", {
    token,
    headers: { Accept: "application/ld+json, application/json" },
  });
  if (got.status >= 400) throw new Error(got.text || "not found");
  return JSON.parse(got.text);
}

/** POST /idp/login or /idp/register. Never log the body (it contains a password). */
async function idpAuth(path, body) {
  const res = await podFetch(path, {
    method: "POST",
    headers: { "Content-Type": "application/json", Accept: "application/json" },
    body: JSON.stringify(body),
  });
  if (res.status >= 400) {
    if (res.status === 401) throw new Error("invalid credentials");
    if (res.status === 409) throw new Error("handle already taken");
    throw new Error("idp " + path + " failed (" + res.status + ")");
  }
  try {
    return JSON.parse(res.text);
  } catch (e) {
    throw new Error("idp " + path + " returned non-JSON");
  }
}

async function sparkLogin(args, req) {
  const handleOrEmail = String((args && (args.handle || args.email)) || "").trim();
  const password = args && args.password != null ? String(args.password) : "";
  if (!handleOrEmail || !password) throw new Error("handle (or email) and password are required");
  const payload = { password };
  if (args.handle) payload.handle = String(args.handle).trim();
  if (args.email) payload.email = String(args.email).trim();
  if (!payload.handle && !payload.email) {
    if (handleOrEmail.includes("@")) payload.email = handleOrEmail;
    else payload.handle = handleOrEmail;
  }
  const data = await idpAuth("/idp/login", payload);
  const session = data && data.token;
  if (!session) throw new Error("login did not return a session token");
  return mintSparkConnect(session, req);
}

async function sparkRegister(args, req) {
  const handle = String((args && args.handle) || "").trim();
  const password = args && args.password != null ? String(args.password) : "";
  if (!handle || !password) throw new Error("handle and password are required");
  const payload = { handle, password, createPod: true };
  if (args && args.name) payload.name = String(args.name);
  if (args && args.email) payload.email = String(args.email);
  const data = await idpAuth("/idp/register", payload);
  const session = data && data.token;
  if (!session) throw new Error("register did not return a session token");
  return mintSparkConnect(session, req);
}

async function proxyPodMCP(body, auth) {
  const res = await fetch(POD + "/mcp", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Accept: "application/json",
      ...(auth ? { Authorization: auth } : {}),
    },
    body: JSON.stringify(body),
  });
  const text = await res.text();
  try { return JSON.parse(text); } catch (e) { return { jsonrpc: "2.0", error: { code: -32603, message: text } }; }
}

async function callTool(name, args, token, req) {
  switch (name) {
    case "spark_login":
      return sparkLogin(args || {}, req);
    case "spark_register":
      return sparkRegister(args || {}, req);
    case "spark_save_conversation":
      return sparkSave(args || {}, token);
    case "spark_list_conversations":
      return sparkList(token);
    case "spark_get_conversation":
      if (!args || !args.id) throw new Error("id is required");
      return sparkGet(token, args.id);
    case "spark_share_conversation": {
      if (!args || !args.id) throw new Error("id is required");
      const ldp = await resolveLdpToken(token);
      const origin = originFromReq(req);
      const doc = await shareConversation(ldp, args.id, origin);
      const shareUrl = (doc && doc.share && doc.share.url) || "";
      return { ok: true, id: args.id, shareUrl, share: doc && doc.share, conversation: doc };
    }
    case "spark_unshare_conversation": {
      if (!args || !args.id) throw new Error("id is required");
      const ldp = await resolveLdpToken(token);
      return unshareConversation(ldp, args.id);
    }
    default:
      return null;
  }
}

module.exports = async function handler(req, res) {
  cors(res);
  res.setHeader("Mcp-Session-Id", "openid-hosted");
  if (req.method === "OPTIONS") {
    res.status(204).end();
    return;
  }
  if (req.method === "GET") {
    res.setHeader("Content-Type", "application/json");
    res.status(200).json({
      open: true,
      mcp: "/mcp",
      pod: POD,
      spark: {
        tool: "spark_save_conversation",
        login: "spark_login",
        register: "spark_register",
        prompt: "Save this conversation to my Solid pod.",
        auth: "spark_login in chat (handle + password), or Authorization: Bearer from /app",
      },
    });
    return;
  }
  if (req.method !== "POST") {
    res.status(405).end("method not allowed");
    return;
  }
  const body = typeof req.body === "string" ? JSON.parse(req.body || "{}") : (req.body || {});
  const id = body.id;
  const method = body.method;
  if (method === "notifications/initialized" || method === "initialized" || method === "notifications/cancelled") {
    res.status(202).end();
    return;
  }
  if (method === "initialize") {
    res.status(200).json({
      jsonrpc: "2.0",
      id,
      result: {
        protocolVersion: (body.params && body.params.protocolVersion) || "2025-03-26",
        capabilities: { tools: {} },
        serverInfo: { name: "openid", version: "1.0.0" },
        instructions: SPARK_INSTRUCTIONS,
      },
    });
    return;
  }
  if (method === "ping") {
    res.status(200).json({ jsonrpc: "2.0", id, result: {} });
    return;
  }
  if (method === "tools/list") {
    res.status(200).json({ jsonrpc: "2.0", id, result: { tools: sparkTools() } });
    return;
  }
  if (method === "tools/call") {
    const name = body.params && body.params.name;
    const args = (body.params && body.params.arguments) || {};
    const token = bearer(req, args);
    if (token && isSparkToken(token) && name && !String(name).startsWith("spark_")) {
      res.status(200).json({
        jsonrpc: "2.0",
        id,
        result: { isError: true, content: [{ type: "text", text: "spark connect token cannot call " + name }] },
      });
      return;
    }
    try {
      const local = await callTool(name, args, token, req);
      if (local !== null) {
        res.status(200).json({
          jsonrpc: "2.0",
          id,
          result: { content: [{ type: "text", text: JSON.stringify(local, null, 2) }] },
        });
        return;
      }
    } catch (e) {
      res.status(200).json({
        jsonrpc: "2.0",
        id,
        result: { isError: true, content: [{ type: "text", text: String(e.message || e) }] },
      });
      return;
    }
    const proxied = await proxyPodMCP(body, req.headers.authorization || req.headers.Authorization);
    res.status(200).json(proxied);
    return;
  }
  res.status(200).json({ jsonrpc: "2.0", id: id || null, error: { code: -32601, message: "unknown method " + method } });
};

module.exports.sparkTools = sparkTools;
module.exports.SPARK_INSTRUCTIONS = SPARK_INSTRUCTIONS;
