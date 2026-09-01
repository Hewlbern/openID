const tokenKey = "openid.token";
const handleKey = "openid.handle";
const $ = (id) => document.getElementById(id);

let token = "";
try { token = localStorage.getItem(tokenKey) || ""; } catch (e) {}
let account = null;
let traces = [];
let conversations = [];
let selected = null;
let selectedKind = "account";

function headers(extra) {
  return openidHeaders(extra);
}

function setStatus(text, ok) {
  $("status").textContent = text;
  $("status").className = "status" + (ok === false ? " bad" : ok ? " ok" : "");
}

async function me() {
  if (!token) {
    location.replace("/");
    return null;
  }
  const res = await openidFetch("/idp/accounts/me");
  if (!res.ok) {
    try { localStorage.removeItem(tokenKey); } catch (e) {}
    location.replace("/");
    return null;
  }
  return res.json();
}

function field(t, ...keys) {
  for (const k of keys) {
    if (t[k] != null && t[k] !== "") return t[k];
  }
  return "";
}

function hueFor(s) {
  let n = 18;
  String(s || "").split("").forEach((c) => { n = (n * 31 + c.charCodeAt(0)) % 360; });
  return n;
}

function escapeHtml(s) {
  return String(s).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
}

function nl2br(s) {
  return escapeHtml(s).replace(/\n/g, "<br />");
}

function allItems() {
  const spark = conversations.map((c) => ({ kind: "spark", id: c.id, name: c.title, pkg: c.source || "gemini-spark", when: c.updated || c.created, raw: c }));
  const recs = traces.map((t) => ({
    kind: "record",
    id: t.identifier || t["@id"] || "",
    name: field(t, "name", "schema:name"),
    pkg: field(t, "package") || "record",
    when: field(t, "dateCreated"),
    raw: t,
  }));
  return spark.concat(recs);
}

function visible() {
  const q = ($("q").value || "").toLowerCase().trim();
  return allItems().filter((t) => {
    if (!q) return true;
    const hay = [t.name, t.pkg, t.id].join(" ").toLowerCase();
    return hay.includes(q);
  });
}

function renderList() {
  const rows = visible();
  if (!rows.length) {
    $("list").innerHTML = `<li class="empty">${allItems().length ? "No matches." : "No conversations yet. Save a Spark thread."}</li>`;
    return;
  }
  $("list").innerHTML = rows.slice(0, 400).map((t) => {
    return `<li data-id="${escapeHtml(t.id)}" data-kind="${escapeHtml(t.kind)}" class="${selected === t.id && selectedKind === t.kind ? "on" : ""}">
      <span class="orb" style="--h:${hueFor(t.pkg)}"></span>
      <span><strong>${escapeHtml(t.name || t.id)}</strong><small>${escapeHtml(t.pkg)}</small></span>
    </li>`;
  }).join("");
  $("list").querySelectorAll("li[data-id]").forEach((li) => {
    li.addEventListener("click", () => {
      selected = li.dataset.id;
      selectedKind = li.dataset.kind;
      renderList();
      showDetail();
    });
  });
}

function mcpURL() {
  return location.origin + "/mcp";
}

function showAccount() {
  if (!account) return;
  $("detail").innerHTML = `
    <h1>${escapeHtml(account.name || account.handle)}</h1>
    <div class="bubble">Your pod. One identity for you and the agents you allow.</div>
    <div class="chips">
      <span>${escapeHtml(account.handle || "")}</span>
      <span>Solid</span>
    </div>
    <p class="mono">${escapeHtml(account.webId || "")}</p>
    <p><a href="${escapeHtml(account.publicUrl || "/i/" + account.handle)}">Public page →</a></p>
    <section class="spark-connect" id="sparkConnect">
      <h2>Spark connect</h2>
      <p>Gemini Spark → Settings → Custom Connected Apps / MCP → paste the MCP URL and this Bearer token. Then in a chat say <strong>Save this conversation to my Solid pod.</strong></p>
      <label>MCP URL</label>
      <p class="token-box mono" id="sparkMcpUrl">${escapeHtml(mcpURL())}</p>
      <div class="row">
        <button type="button" class="btn" id="sparkMintBtn">Create / copy connect token</button>
        <button type="button" class="btn ghost" id="sparkCopyUrlBtn">Copy MCP URL</button>
        <button type="button" class="btn ghost" id="sparkCopyTokBtn">Copy token</button>
        <button type="button" class="btn ghost" id="sparkRevokeBtn">Revoke</button>
      </div>
      <label>Connect token</label>
      <p class="token-box mono" id="sparkTokenBox">No token yet. Create one — it lasts 30 days, is scoped to your WebID, and only works for Spark save/list/share.</p>
      <p class="hint" id="sparkConnectHint"></p>
    </section>
    <p class="hint">The Save button is a paste fallback if Spark is not connected.</p>`;
  bindSparkConnect();
}

let sparkConnectToken = "";
try { sparkConnectToken = sessionStorage.getItem("openid.sparkToken") || ""; } catch (e) {}

async function copyText(value) {
  const t = String(value || "");
  if (!t) return false;
  try {
    await navigator.clipboard.writeText(t);
    return true;
  } catch (e) {
    return false;
  }
}

function setSparkHint(text, ok) {
  const el = $("sparkConnectHint");
  if (!el) return;
  el.textContent = text;
  el.className = "hint" + (ok === false ? " bad" : ok ? " ok" : "");
}

function renderSparkTokenBox(info) {
  const box = $("sparkTokenBox");
  if (!box) return;
  if (sparkConnectToken) {
    box.textContent = sparkConnectToken;
    return;
  }
  if (info && info.tokens && info.tokens.length) {
    const t = info.tokens[0];
    box.textContent = "A connect token is active until " + (t.expires || "expiry") + ". The secret is only shown when you create it — create again or copy if you still have it.";
    return;
  }
  box.textContent = "No token yet. Create one — it lasts 30 days, is scoped to your WebID, and only works for Spark save/list/share.";
}

async function loadSparkTokenStatus() {
  const res = await openidFetch("/idp/spark-token");
  if (!res.ok) return;
  const info = await res.json();
  renderSparkTokenBox(info);
  if (info.ttl) {
    const hint = $("sparkConnectHint");
    if (hint && !hint.textContent) hint.textContent = "Lifetime " + info.ttl + " (SOLID_SPARK_TOKEN_TTL).";
  }
}

function bindSparkConnect() {
  const mint = $("sparkMintBtn");
  const copyUrl = $("sparkCopyUrlBtn");
  const copyTok = $("sparkCopyTokBtn");
  const revoke = $("sparkRevokeBtn");
  if (copyUrl) copyUrl.addEventListener("click", async () => {
    setSparkHint(await copyText(mcpURL()) ? "Copied MCP URL." : mcpURL(), true);
  });
  if (copyTok) copyTok.addEventListener("click", async () => {
    if (!sparkConnectToken) {
      setSparkHint("Create a token first — the secret is only shown once.", false);
      return;
    }
    setSparkHint(await copyText(sparkConnectToken) ? "Copied connect token." : "Copy failed.", true);
  });
  if (mint) mint.addEventListener("click", async () => {
    setSparkHint("Minting…");
    const res = await openidFetch("/idp/spark-token", { method: "POST", headers: { "Content-Type": "application/json" }, body: "{}" });
    if (!res.ok) {
      setSparkHint(await res.text(), false);
      return;
    }
    const doc = await res.json();
    sparkConnectToken = doc.token || "";
    try { sessionStorage.setItem("openid.sparkToken", sparkConnectToken); } catch (e) {}
    renderSparkTokenBox();
    const copied = await copyText(sparkConnectToken);
    setSparkHint((copied ? "Created and copied. " : "Created. ") + "Expires " + (doc.expires || "") + ". Paste as Bearer in Spark.", true);
  });
  if (revoke) revoke.addEventListener("click", async () => {
    const res = await openidFetch("/idp/spark-token", { method: "DELETE" });
    if (!res.ok) {
      setSparkHint(await res.text(), false);
      return;
    }
    sparkConnectToken = "";
    try { sessionStorage.removeItem("openid.sparkToken"); } catch (e) {}
    renderSparkTokenBox({ tokens: [] });
    setSparkHint("Revoked. Spark can no longer save with the old token.", true);
  });
  renderSparkTokenBox();
  loadSparkTokenStatus();
}

function showSpark(c) {
  const share = c.share;
  const turns = (c.messages || []).map((m) =>
    `<div class="turn ${escapeHtml(m.role)}"><span class="who">${escapeHtml(m.role)}</span><div class="bubble">${nl2br(m.text || "")}</div></div>`
  ).join("");
  $("detail").innerHTML = `
    <h1>${escapeHtml(c.title || c.id)}</h1>
    <div class="chips">
      <span>${escapeHtml(c.source || "gemini-spark")}</span>
      <span>${escapeHtml((c.updated || c.created || "").toString().slice(0, 19))}</span>
      ${share ? `<span>shared</span>` : ""}
    </div>
    ${c.sourceUrl ? `<p class="mono">${escapeHtml(c.sourceUrl)}</p>` : ""}
    ${turns || `<p class="hint">No messages.</p>`}
    <div class="actions">
      <button type="button" class="btn" id="shareBtn">${share ? "Copy share link" : "Share"}</button>
      ${share ? `<button type="button" class="btn ghost" id="revokeBtn">Revoke</button>` : ""}
    </div>
    <p class="hint" id="shareHint">${share ? escapeHtml(share.url) : "Unlisted link until you revoke it."}</p>
    <p class="mono">${escapeHtml(c.resource || "")}</p>`;
  const shareBtn = $("shareBtn");
  if (shareBtn) shareBtn.addEventListener("click", () => shareConversation(c));
  const revokeBtn = $("revokeBtn");
  if (revokeBtn) revokeBtn.addEventListener("click", () => unshareConversation(c));
}

function showRecord(t) {
  const id = t.identifier || t["@id"] || "";
  const desc = field(t, "description", "query");
  $("detail").innerHTML = `
    <h1>${escapeHtml(field(t, "name", "schema:name") || id)}</h1>
    ${desc ? `<div class="bubble">${escapeHtml(desc)}</div>` : ""}
    <div class="chips">
      <span>${escapeHtml(field(t, "package") || "pod")}</span>
      <span>${escapeHtml(field(t, "workType") || "work")}</span>
      <span>${escapeHtml(field(t, "artifact") || "trace")}</span>
    </div>
    <p class="mono">${escapeHtml(t["@id"] || "")}</p>`;
}

function showDetail() {
  if (selectedKind === "spark") {
    const c = conversations.find((x) => x.id === selected);
    if (c) {
      showSpark(c);
      return;
    }
  }
  if (selectedKind === "record") {
    const t = traces.find((x) => (x.identifier || x["@id"]) === selected);
    if (t) {
      showRecord(t);
      return;
    }
  }
  showAccount();
}

async function listViaLDP() {
  if (!account || !account.handle) return [];
  const res = await openidFetch("/" + account.handle + "/conversations/spark/", {
    headers: { Accept: "text/turtle" },
  });
  if (!res.ok) return [];
  const ttl = await res.text();
  const ids = [...ttl.matchAll(/conversations\/spark\/([A-Za-z0-9-]+)\.json/g)].map((m) => m[1]);
  const out = [];
  for (const id of [...new Set(ids)]) {
    const g = await openidFetch("/" + account.handle + "/conversations/spark/" + id + ".json");
    if (g.ok) out.push(await g.json());
  }
  return out;
}

async function loadConversations() {
  const res = await openidFetch("/conversations");
  if (res.ok) {
    const doc = await res.json();
    conversations = Array.isArray(doc) ? doc : doc.conversations || [];
    return;
  }
  conversations = await listViaLDP();
}

async function loadRecords(handle) {
  try {
    const res = await openidFetch("/" + handle + "/records/cursor/transcripts.jsonld", {
      headers: { Accept: "application/ld+json, application/json" },
    });
    if (!res.ok) {
      traces = [];
      return;
    }
    const doc = await res.json();
    traces = Array.isArray(doc) ? doc : doc["@graph"] || [doc];
  } catch (e) {
    traces = [];
  }
}

async function refresh() {
  await loadConversations();
  await loadRecords(account && account.handle ? account.handle : "mike");
  renderList();
  showDetail();
}

async function shareConversation(c) {
  const hint = $("shareHint");
  if (c.share && c.share.url) {
    try {
      await navigator.clipboard.writeText(c.share.url);
      if (hint) hint.textContent = "Copied " + c.share.url;
    } catch (e) {
      if (hint) hint.textContent = c.share.url;
    }
    return;
  }
  const res = await openidFetch("/conversations/" + encodeURIComponent(c.id) + "/share", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ public: false }),
  });
  if (!res.ok) {
    if (hint) hint.textContent = await res.text();
    return;
  }
  const updated = await res.json();
  const idx = conversations.findIndex((x) => x.id === c.id);
  if (idx >= 0) conversations[idx] = updated;
  selected = updated.id;
  selectedKind = "spark";
  renderList();
  showSpark(updated);
  if (updated.share && updated.share.url) {
    try { await navigator.clipboard.writeText(updated.share.url); } catch (e) {}
  }
}

async function unshareConversation(c) {
  const res = await openidFetch("/conversations/" + encodeURIComponent(c.id) + "/unshare", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: "{}",
  });
  if (!res.ok) {
    setStatus(await res.text(), false);
    return;
  }
  await refresh();
}

$("logout").addEventListener("click", async () => {
  await openidFetch("/idp/logout", { method: "POST" });
  try { localStorage.removeItem(tokenKey); } catch (e) {}
  location.replace("/");
});

$("q").addEventListener("input", renderList);
$("meFace").addEventListener("click", () => {
  selected = null;
  selectedKind = "account";
  renderList();
  showAccount();
});

const saveModal = $("saveModal");
$("saveBtn").addEventListener("click", () => {
  $("saveHint").textContent = "";
  saveModal.classList.add("open");
  $("saveText").focus();
});
saveModal.addEventListener("click", (e) => {
  if (e.target === saveModal) saveModal.classList.remove("open");
});

async function ensureLDPContainer(path) {
  if (!path.endsWith("/")) path += "/";
  const res = await openidFetch(path, {
    method: "PUT",
    headers: {
      "Content-Type": "text/turtle",
      Link: '<http://www.w3.org/ns/ldp#BasicContainer>; rel="type"',
    },
    body: "# container\n",
  });
  if (!res.ok && res.status !== 200 && res.status !== 201 && res.status !== 204) {
    throw new Error("container " + path + " " + res.status + " " + (await res.text()));
  }
}

function parsePasteMessages(text) {
  const lines = String(text || "").split("\n");
  const msgs = [];
  let cur = null;
  const turn = /^\s*(?:\*{0,2})(user|human|you|assistant|gemini|spark|model|system)(?:\*{0,2})\s*:\s*(?:\*{0,2})\s*(.*)$/i;
  for (const line of lines) {
    const m = line.match(turn);
    if (m) {
      if (cur && cur.text.trim()) msgs.push(cur);
      const role = /assistant|gemini|spark|model/i.test(m[1]) ? "assistant" : /system/i.test(m[1]) ? "system" : "user";
      cur = { role, text: m[2] || "" };
      continue;
    }
    if (cur) cur.text += (cur.text ? "\n" : "") + line;
  }
  if (cur && cur.text.trim()) msgs.push(cur);
  if (!msgs.length && String(text || "").trim()) {
    msgs.push({ role: "user", text: String(text).trim() });
  }
  return msgs.map((m) => ({ role: m.role, text: m.text.trim() }));
}

async function saveViaLDP(payload) {
  const handle = account && account.handle;
  if (!handle) throw new Error("signed-in handle required");
  const id = (crypto.randomUUID && crypto.randomUUID()) || String(Date.now());
  const now = new Date().toISOString();
  let messages = payload.messages || [];
  if (!messages.length && payload.text) {
    try {
      const parsed = JSON.parse(payload.text);
      if (Array.isArray(parsed)) messages = parsed;
      else if (parsed && Array.isArray(parsed.messages)) messages = parsed.messages;
    } catch (e) {
      messages = parsePasteMessages(payload.text);
    }
  }
  messages = messages.map((m) => ({
    role: m.role || "user",
    text: m.text || m.content || "",
    timestamp: m.timestamp || m.time || undefined,
  })).filter((m) => m.text);
  if (!messages.length) throw new Error("messages or transcript required");
  const title = payload.title || (messages[0] && messages[0].text.slice(0, 80)) || "Untitled conversation";
  await ensureLDPContainer("/" + handle + "/conversations/");
  await ensureLDPContainer("/" + handle + "/conversations/spark/");
  const resource = "/" + handle + "/conversations/spark/" + id + ".json";
  const ttlPath = "/" + handle + "/conversations/spark/" + id + ".ttl";
  const resourceUrl = location.origin + resource;
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
    sourceUrl: payload.source_url || "",
    created: now,
    updated: now,
    dateCreated: now,
    dateModified: now,
    messages,
    owner: account.webId,
    creator: account.webId,
    podPath: handle + "/",
    resource: handle + "/conversations/spark/" + id + ".json",
    metaTtl: location.origin + ttlPath,
  };
  const jsonRes = await openidFetch(resource, {
    method: "PUT",
    headers: { "Content-Type": "application/ld+json" },
    body: JSON.stringify(doc, null, 2),
  });
  if (!jsonRes.ok) throw new Error(await jsonRes.text());
  const ttl = [
    "@prefix schema: <https://schema.org/> .",
    "@prefix dcterms: <http://purl.org/dc/terms/> .",
    "@prefix foaf: <http://xmlns.com/foaf/0.1/> .",
    "@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .",
    "",
    "<" + resourceUrl + "> a schema:Conversation ;",
    "  schema:name " + JSON.stringify(title) + " ;",
    "  dcterms:created " + JSON.stringify(now) + "^^xsd:dateTime ;",
    "  dcterms:modified " + JSON.stringify(now) + "^^xsd:dateTime ;",
    "  dcterms:source \"gemini-spark\" ;",
    "  schema:creator <" + (account.webId || "") + "> .",
  ].join("\n");
  await openidFetch(ttlPath, {
    method: "PUT",
    headers: { "Content-Type": "text/turtle" },
    body: ttl,
  });
  return doc;
}

$("saveForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  const hint = $("saveHint");
  hint.textContent = "Saving…";
  hint.className = "hint";
  const payload = {
    title: $("saveTitle").value,
    source_url: $("saveURL").value,
    text: $("saveText").value,
    source: "gemini-spark",
  };
  let saved = null;
  const res = await openidFetch("/conversations", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  if (res.ok) {
    saved = await res.json();
    if (saved.conversation) saved = Object.assign({}, saved.conversation, saved);
  } else {
    const errText = await res.text();
    if (res.status === 404 || res.status === 405 || /Can only POST to containers/i.test(errText)) {
      try {
        saved = await saveViaLDP(payload);
      } catch (ldpErr) {
        hint.textContent = String(ldpErr.message || ldpErr);
        hint.className = "hint bad";
        return;
      }
    } else {
      hint.textContent = errText;
      hint.className = "hint bad";
      return;
    }
  }
  saveModal.classList.remove("open");
  $("saveText").value = "";
  $("saveURL").value = "";
  $("saveTitle").value = "";
  conversations.unshift(saved);
  selected = saved.id;
  selectedKind = "spark";
  await refresh();
});

(async function boot() {
  account = await me();
  if (!account) return;
  $("who").textContent = account.handle || account.name || "";
  $("meFace").style.setProperty("--h", String(hueFor(account.handle)));
  document.title = (account.handle || "oid") + " — oid";
  setStatus(account.handle || "signed in");
  selectedKind = "account";
  await refresh();
})();
