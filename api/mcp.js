/**
 * Hosted MCP at /mcp (rewritten here). Spark talks to this origin; we write
 * the conversation into the Railway Solid pod with LDP PUTs to container
 * paths that end in /. Does not scrape Gemini — Spark supplies the thread.
 */
const POD = (process.env.OPENID_POD || process.env.OPENID_API || "https://pod-production-ebe1.up.railway.app").replace(/\/$/, "");

const SPARK_SAVE = `FIRST-CLASS tool for Gemini Spark. When the user asks to save, upload, store, or write this chat / thread / conversation to their Solid pod or OpenID, you MUST call this tool immediately with the full current conversation. Do not tell them to leave Spark or paste into /app — that UI is a fallback only. Pass title and the complete messages array as [{role, content|text, timestamp?}]. Include timezone-aware ISO-8601 timestamps on each message when Spark has them. Spark itself supplies the transcript; do not scrape Gemini. Writes conversations/spark/{id}.json (JSON-LD) plus a Turtle sidecar. Returns resourceUrl, webId, optional shareUrl, created/modified, and confirmation text to show the user.`;

function sparkTools() {
  const token = { type: "string", description: "Bearer token from /idp/login (or send Authorization: Bearer on /mcp)" };
  const id = { type: "string", description: "Conversation id returned by spark_save_conversation" };
  return [
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
    { name: "spark_share_conversation", description: "Mint a /share/c/{id} URL (proxied to the pod when available).", inputSchema: { type: "object", properties: { id, public: { type: "boolean" }, token }, required: ["id"] } },
    { name: "spark_unshare_conversation", description: "Revoke the share link.", inputSchema: { type: "object", properties: { id, token }, required: ["id"] } },
  ];
}

function cors(res) {
  res.setHeader("Access-Control-Allow-Origin", "*");
  res.setHeader("Access-Control-Allow-Methods", "GET, POST, OPTIONS");
  res.setHeader("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept, Mcp-Session-Id");
  res.setHeader("Access-Control-Expose-Headers", "Mcp-Session-Id");
}

function jwtPayload(token) {
  try {
    const parts = String(token || "").split(".");
    if (parts.length < 2) return null;
    const json = Buffer.from(parts[1].replace(/-/g, "+").replace(/_/g, "/"), "base64").toString("utf8");
    return JSON.parse(json);
  } catch (e) {
    return null;
  }
}

function isSparkToken(token) {
  const p = jwtPayload(token);
  if (!p) return false;
  const aud = Array.isArray(p.aud) ? p.aud : (p.aud ? [p.aud] : []);
  return p.scope === "spark" || aud.includes("spark-mcp");
}

function bearer(req, args) {
  const fromArgs = args && typeof args.token === "string" ? args.token.trim() : "";
  if (fromArgs) return fromArgs;
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
  return { status: res.status, text, headers: res.headers, url };
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

async function ensureContainer(token, path) {
  if (!path.endsWith("/")) path += "/";
  const res = await podFetch(path, {
    method: "PUT",
    token,
    headers: {
      "Content-Type": "text/turtle",
      Link: '<http://www.w3.org/ns/ldp#BasicContainer>; rel="type"',
    },
    body: "# container\n",
  });
  if (res.status === 200 || res.status === 201 || res.status === 204 || res.status === 409) return;
  const got = await podFetch(path, { token, headers: { Accept: "text/turtle, */*" } });
  if (got.status < 400) return;
  throw new Error("ensure container " + path + " -> " + res.status + " " + res.text);
}

async function accountMe(token) {
  const res = await podFetch("/idp/accounts/me", { token, headers: { Accept: "application/json" } });
  if (res.status >= 400) throw new Error("login required: " + res.text);
  const acc = JSON.parse(res.text);
  if (!acc.handle) throw new Error("account missing handle");
  return acc;
}

async function sparkSave(args, token) {
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
  const api = await podFetch("/conversations", { token, headers: { Accept: "application/json" } });
  if (api.status < 400) {
    try { return JSON.parse(api.text); } catch (e) { return { text: api.text }; }
  }
  const acc = await accountMe(token);
  const listing = await podFetch("/" + acc.handle + "/conversations/spark/", {
    token,
    headers: { Accept: "text/turtle" },
  });
  if (listing.status >= 400) return { conversations: [] };
  return { container: listing.text };
}

async function sparkGet(token, id) {
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
    case "spark_save_conversation":
      return sparkSave(args || {}, token);
    case "spark_list_conversations":
      return sparkList(token);
    case "spark_get_conversation":
      if (!args || !args.id) throw new Error("id is required");
      return sparkGet(token, args.id);
    case "spark_share_conversation":
    case "spark_unshare_conversation": {
      const path = "/conversations/" + encodeURIComponent(args.id) + (name === "spark_share_conversation" ? "/share" : "/unshare");
      const res = await podFetch(path, {
        method: "POST",
        token,
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ public: !!(args && args.public) }),
      });
      if (res.status >= 400) throw new Error(res.text);
      try { return JSON.parse(res.text); } catch (e) { return { text: res.text }; }
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
      spark: { tool: "spark_save_conversation", prompt: "Save this conversation to my Solid pod." },
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
        instructions: "Gemini Spark: when the user asks to save/upload this conversation to their Solid pod or OpenID, call spark_save_conversation with the full current thread. Auth: Bearer from /idp/login.",
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
