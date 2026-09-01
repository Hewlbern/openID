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
const podCaps = {
  conversationsAPI: false,
  sparkToken: false,
  listMode: "",
  conversationsError: "",
  sparkTokenError: "",
  probed: false,
};

function headers(extra) {
  return openidHeaders(extra);
}

function sessionToken() {
  try { return localStorage.getItem(tokenKey) || token || ""; } catch (e) { return token || ""; }
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

function sparkDir() {
  return account && account.handle ? account.handle + "/conversations/spark/" : "conversations/spark/";
}

function mcpURL() {
  return location.origin + "/mcp";
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
    $("list").innerHTML = `<li class="empty">${allItems().length ? "No matches." : "Nothing in conversations/spark/ yet."}</li>`;
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

function goYou() {
  selected = null;
  selectedKind = "account";
  renderList();
  showAccount();
}

function railwayHonest() {
  if (podCaps.conversationsAPI && podCaps.sparkToken) return "";
  const bits = [];
  if (!podCaps.conversationsAPI) {
    bits.push(podCaps.conversationsError || "GET/POST /conversations is not on this Railway build");
  }
  if (!podCaps.sparkToken) {
    bits.push(podCaps.sparkTokenError || "POST /idp/spark-token is not on this Railway build");
  }
  return `
    <aside class="pod-banner" id="podBanner">
      <strong>Railway is still on an older Solid build.</strong>
      Login stays on this preview through the same-origin <span class="mono">/idp</span> proxy.
      ${escapeHtml(bits.join(". "))}.
      This page saves and lists through same-origin <span class="mono">/api/spark-conversations</span>,
      which creates <span class="mono">${escapeHtml(sparkDir())}</span> with LDP PUT BasicContainer on Railway.
      Share links and 30-day Spark connect tokens need a Railway redeploy of this branch.
      We never ask for a Google password.
    </aside>`;
}

function showAccount() {
  if (!account) return;
  const label = $("podPathLabel");
  if (label) label.textContent = sparkDir();
  $("detail").innerHTML = `
    ${railwayHonest()}
    <p class="walk-kicker">After login — one place</p>
    <h1>Take Spark into your Solid pod</h1>
    <p class="walk-lead">You are signed in as <strong>${escapeHtml(account.handle || "")}</strong>. Paste a Gemini thread, or let Spark push one through MCP. Saved files live at <span class="mono">${escapeHtml(sparkDir())}</span>.</p>

    <section class="walk-card" id="youCard">
      <h2><span class="step">1</span> Who you are</h2>
      <div class="you-row">
        <span class="face lg" style="--h:${hueFor(account.handle)}"></span>
        <div>
          <p class="you-name">${escapeHtml(account.name || account.handle)}</p>
          <p class="mono" id="youHandle">${escapeHtml(account.handle || "")}</p>
        </div>
      </div>
      <label>WebID</label>
      <p class="token-box mono" id="youWebId">${escapeHtml(account.webId || "")}</p>
      <div class="row">
        <button type="button" class="btn ghost" id="copyHandleBtn">Copy handle</button>
        <button type="button" class="btn ghost" id="copyWebIdBtn">Copy WebID</button>
        <a class="btn ghost" href="${escapeHtml(account.publicUrl || "/i/" + account.handle)}">Public page</a>
      </div>
      <p class="hint" id="youHint"></p>
    </section>

    <section class="walk-card" id="importCard">
      <h2><span class="step">2</span> Spark / Gemini import</h2>
      <p>Paste a transcript <em>or</em> a public Gemini share URL. Spark can also push the thread itself (step 4). No Google login on this page.</p>
      <form id="importForm">
        <div class="field"><label>Title (optional)</label><input id="importTitle" placeholder="Untitled conversation" /></div>
        <div class="field"><label>Public Gemini share URL</label><input id="importURL" placeholder="https://g.co/gemini/share/…" /></div>
        <div class="field"><label>Transcript</label><textarea id="importText" rows="8" placeholder="**User:** …&#10;**Gemini:** …"></textarea></div>
        <button class="btn" type="submit">Save to my pod</button>
        <p class="hint" id="importHint"></p>
      </form>
    </section>

    <section class="walk-card" id="savedCard">
      <h2><span class="step">3</span> Saved conversations</h2>
      <p>Listed from <span class="mono">${escapeHtml(sparkDir())}</span>${podCaps.listMode === "ldp" ? " (LDP — /conversations is not on Railway yet)" : ""}.</p>
      <ul class="dash-list" id="dashList"></ul>
    </section>

    <section class="walk-card spark-connect" id="sparkConnect">
      <h2><span class="step">4</span> Connect Gemini Spark via MCP</h2>
      <p>In Spark: Settings → Custom Connected Apps / MCP. Paste the MCP URL and a Bearer token. Then say <strong>Save this conversation to my Solid pod.</strong> Spark calls <span class="mono">spark_save_conversation</span> and writes the thread itself.</p>
      <label>MCP URL</label>
      <p class="token-box mono" id="sparkMcpUrl">${escapeHtml(mcpURL())}</p>
      <label>Session Bearer</label>
      <p class="token-box mono" id="sessionTokenBox">${sessionToken() ? escapeHtml(sessionToken()) : "Sign in again — no session token in this browser."}</p>
      <div class="row">
        <button type="button" class="btn" id="copySessionBtn">Copy session token</button>
        <button type="button" class="btn ghost" id="sparkCopyUrlBtn">Copy MCP URL</button>
        <button type="button" class="btn ghost" id="sparkMintBtn">Create 30-day Spark token</button>
        <button type="button" class="btn ghost" id="sparkCopyTokBtn">Copy Spark token</button>
        <button type="button" class="btn ghost" id="sparkRevokeBtn">Revoke Spark token</button>
      </div>
      <label>Spark connect token</label>
      <p class="token-box mono" id="sparkTokenBox">Optional scoped token. If Railway is still on the old Go build, mint will fail — use the session Bearer above.</p>
      <p class="hint" id="sparkConnectHint"></p>
    </section>`;
  bindYouCard();
  bindImportForm();
  renderDashList();
  bindSparkConnect();
}

function setHint(id, text, ok) {
  const el = $(id);
  if (!el) return;
  el.textContent = text || "";
  el.className = "hint" + (ok === false ? " bad" : ok ? " ok" : "");
}

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

function bindYouCard() {
  const handleBtn = $("copyHandleBtn");
  const webBtn = $("copyWebIdBtn");
  if (handleBtn) handleBtn.addEventListener("click", async () => {
    setHint("youHint", await copyText(account.handle) ? "Copied handle." : account.handle, true);
  });
  if (webBtn) webBtn.addEventListener("click", async () => {
    setHint("youHint", await copyText(account.webId) ? "Copied WebID." : account.webId, true);
  });
}

function bindImportForm() {
  const form = $("importForm");
  if (!form) return;
  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    await submitSave($("importHint"), {
      title: $("importTitle").value,
      source_url: $("importURL").value,
      text: $("importText").value,
      source: "gemini-spark",
    }, { clear: ["importTitle", "importURL", "importText"] });
  });
}

function renderDashList() {
  const el = $("dashList");
  if (!el) return;
  if (!conversations.length) {
    el.innerHTML = `<li class="empty">Nothing saved yet. Import above or connect Spark.</li>`;
    return;
  }
  el.innerHTML = conversations.map((c) => {
    const share = c.share;
    const when = (c.updated || c.created || "").toString().slice(0, 19);
    return `<li data-id="${escapeHtml(c.id)}">
      <div>
        <strong>${escapeHtml(c.title || c.id)}</strong>
        <small class="mono">${escapeHtml(c.resource || sparkDir() + c.id + ".json")}</small>
        <small>${escapeHtml(when)}${share ? " · shared" : ""}</small>
      </div>
      <div class="row">
        <button type="button" class="btn ghost dash-open">Open</button>
        <button type="button" class="btn ghost dash-share">${share ? "Copy share" : "Share"}</button>
        ${share ? `<button type="button" class="btn ghost dash-revoke">Revoke</button>` : ""}
      </div>
    </li>`;
  }).join("");
  el.querySelectorAll("li[data-id]").forEach((li) => {
    const c = conversations.find((x) => x.id === li.dataset.id);
    if (!c) return;
    const open = li.querySelector(".dash-open");
    const shareBtn = li.querySelector(".dash-share");
    const revokeBtn = li.querySelector(".dash-revoke");
    if (open) open.addEventListener("click", () => {
      selected = c.id;
      selectedKind = "spark";
      renderList();
      showSpark(c);
    });
    if (shareBtn) shareBtn.addEventListener("click", () => shareConversation(c, li));
    if (revokeBtn) revokeBtn.addEventListener("click", () => unshareConversation(c));
  });
}

let sparkConnectToken = "";
try { sparkConnectToken = sessionStorage.getItem("openid.sparkToken") || ""; } catch (e) {}

function setSparkHint(text, ok) {
  setHint("sparkConnectHint", text, ok);
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
    box.textContent = "A connect token is active until " + (t.expires || "expiry") + ". The secret is only shown when you create it.";
    return;
  }
  if (!podCaps.sparkToken && podCaps.probed) {
    box.textContent = "Railway does not have /idp/spark-token yet. Copy the session Bearer above so Spark can push threads now.";
    return;
  }
  box.textContent = "Optional. Create a 30-day token scoped to spark_* tools, or copy the session Bearer.";
}

async function loadSparkTokenStatus() {
  const res = await openidFetch("/idp/spark-token");
  if (!res.ok) {
    podCaps.sparkToken = false;
    renderSparkTokenBox();
    return;
  }
  let info = {};
  try { info = await res.json(); } catch (e) { info = {}; }
  podCaps.sparkToken = !!(info && (Array.isArray(info.tokens) || info.mcpUrl || info.ttl || info.token));
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
  const copySession = $("copySessionBtn");
  const revoke = $("sparkRevokeBtn");
  if (copyUrl) copyUrl.addEventListener("click", async () => {
    setSparkHint(await copyText(mcpURL()) ? "Copied MCP URL." : mcpURL(), true);
  });
  if (copySession) copySession.addEventListener("click", async () => {
    const tok = sessionToken();
    if (!tok) {
      setSparkHint("No session token. Sign in again.", false);
      return;
    }
    setSparkHint(await copyText(tok) ? "Copied session Bearer. Paste it in Spark as the Authorization token." : "Copy failed — select the token box.", true);
  });
  if (copyTok) copyTok.addEventListener("click", async () => {
    if (!sparkConnectToken) {
      setSparkHint("Create a Spark token first, or copy the session Bearer.", false);
      return;
    }
    setSparkHint(await copyText(sparkConnectToken) ? "Copied Spark connect token." : "Copy failed.", true);
  });
  if (mint) mint.addEventListener("click", async () => {
    setSparkHint("Minting…");
    const res = await openidFetch("/idp/spark-token", { method: "POST", headers: { "Content-Type": "application/json" }, body: "{}" });
    const body = await res.text();
    if (!res.ok) {
      const honest = res.status === 404 || /Can only POST to containers/i.test(body) || /idp/i.test(body) && /register|login/.test(body)
        ? "Railway does not have POST /idp/spark-token yet. Copy the session Bearer above — Spark can still save through this preview’s /mcp."
        : body;
      setSparkHint(honest, false);
      return;
    }
    let doc = {};
    try { doc = JSON.parse(body); } catch (e) {
      setSparkHint("Railway returned something that is not a Spark token. Copy the session Bearer instead.", false);
      return;
    }
    if (!doc.token) {
      setSparkHint("No token in that response. Railway is probably still on the old IDP. Copy the session Bearer.", false);
      return;
    }
    sparkConnectToken = doc.token;
    try { sessionStorage.setItem("openid.sparkToken", sparkConnectToken); } catch (e) {}
    renderSparkTokenBox();
    const copied = await copyText(sparkConnectToken);
    setSparkHint((copied ? "Created and copied. " : "Created. ") + "Expires " + (doc.expires || "") + ". Paste as Bearer in Spark.", true);
  });
  if (revoke) revoke.addEventListener("click", async () => {
    const res = await openidFetch("/idp/spark-token", { method: "DELETE" });
    if (!res.ok) {
      setSparkHint(await res.text() || "Revoke is not on this Railway build yet.", false);
      return;
    }
    sparkConnectToken = "";
    try { sessionStorage.removeItem("openid.sparkToken"); } catch (e) {}
    renderSparkTokenBox({ tokens: [] });
    setSparkHint("Revoked. Spark can no longer save with the old Spark token. Your session Bearer still works until you sign out.", true);
  });
  renderSparkTokenBox();
  loadSparkTokenStatus();
}

function shareHintFor(c) {
  if (c.share && c.share.url) return c.share.url;
  if (!podCaps.conversationsAPI) {
    return "Share/revoke needs POST /conversations/{id}/share on Railway. This thread is private on your pod at " + (c.resource || sparkDir() + c.id + ".json") + ".";
  }
  return "Unlisted link until you revoke it.";
}

function showSpark(c) {
  const share = c.share;
  const turns = (c.messages || []).map((m) =>
    `<div class="turn ${escapeHtml(m.role)}"><span class="who">${escapeHtml(m.role)}</span><div class="bubble">${nl2br(m.text || "")}</div></div>`
  ).join("");
  $("detail").innerHTML = `
    <p class="walk-kicker"><button type="button" class="ghost" id="backYou">← You</button></p>
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
    <p class="hint" id="shareHint">${escapeHtml(shareHintFor(c))}</p>
    <p class="mono">${escapeHtml(c.resource || "")}</p>`;
  const back = $("backYou");
  if (back) back.addEventListener("click", goYou);
  const shareBtn = $("shareBtn");
  if (shareBtn) shareBtn.addEventListener("click", () => shareConversation(c));
  const revokeBtn = $("revokeBtn");
  if (revokeBtn) revokeBtn.addEventListener("click", () => unshareConversation(c));
}

function showRecord(t) {
  const id = t.identifier || t["@id"] || "";
  const desc = field(t, "description", "query");
  $("detail").innerHTML = `
    <p class="walk-kicker"><button type="button" class="ghost" id="backYou">← You</button></p>
    <h1>${escapeHtml(field(t, "name", "schema:name") || id)}</h1>
    ${desc ? `<div class="bubble">${escapeHtml(desc)}</div>` : ""}
    <div class="chips">
      <span>${escapeHtml(field(t, "package") || "pod")}</span>
      <span>${escapeHtml(field(t, "workType") || "work")}</span>
      <span>${escapeHtml(field(t, "artifact") || "trace")}</span>
    </div>
    <p class="mono">${escapeHtml(t["@id"] || "")}</p>`;
  const back = $("backYou");
  if (back) back.addEventListener("click", goYou);
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

async function listViaHostedAPI() {
  const res = await fetch("/api/spark-conversations", {
    method: "GET",
    headers: openidHeaders(),
    credentials: "include",
  });
  if (!res.ok) return null;
  const doc = await res.json();
  return Array.isArray(doc) ? doc : doc.conversations || [];
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
    podCaps.conversationsAPI = true;
    podCaps.listMode = "api";
    return;
  }
  const errText = await res.text();
  podCaps.conversationsAPI = false;
  podCaps.conversationsError = "GET /conversations → " + res.status + (/Can only POST to containers/i.test(errText)
    ? " (Can only POST to containers)"
    : errText ? ": " + errText.slice(0, 140) : "");
  // Vercel catch-all often 404s browser LDP to /{handle}/conversations/ — use the
  // same-origin API that PUTs BasicContainers straight to Railway.
  const hosted = await listViaHostedAPI();
  if (hosted) {
    conversations = hosted;
    podCaps.listMode = "hosted-ldp";
    return;
  }
  conversations = await listViaLDP();
  podCaps.listMode = "ldp";
}

async function probePod() {
  const sparkTok = await openidFetch("/idp/spark-token");
  if (sparkTok.ok) {
    try {
      const j = await sparkTok.json();
      podCaps.sparkToken = !!(j && (Array.isArray(j.tokens) || j.mcpUrl || j.ttl || j.token));
      if (!podCaps.sparkToken) podCaps.sparkTokenError = "GET /idp/spark-token returned the old IDP catalog, not a Spark grant";
    } catch (e) {
      podCaps.sparkToken = false;
      podCaps.sparkTokenError = "GET /idp/spark-token was not JSON";
    }
  } else {
    podCaps.sparkToken = false;
    podCaps.sparkTokenError = "GET /idp/spark-token → " + sparkTok.status;
  }
  podCaps.probed = true;
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

function honestShareFailure(status, body, c) {
  if (status === 404 || status === 405 || /Can only POST to containers/i.test(body)) {
    return "Share/revoke is not on this Railway build yet (POST /conversations hits “Can only POST to containers”). Redeploy this branch. The conversation stays private at " + (c.resource || sparkDir() + c.id + ".json") + ".";
  }
  return body || "Share failed.";
}

async function shareConversation(c, fromRow) {
  const hint = $("shareHint");
  if (c.share && c.share.url) {
    const ok = await copyText(c.share.url);
    if (hint) hint.textContent = (ok ? "Copied " : "") + c.share.url;
    if (fromRow) setStatus(ok ? "Copied share link" : c.share.url, true);
    return;
  }
  const res = await openidFetch("/conversations/" + encodeURIComponent(c.id) + "/share", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ public: false }),
  });
  if (!res.ok) {
    const msg = honestShareFailure(res.status, await res.text(), c);
    if (hint) hint.textContent = msg;
    else setStatus(msg, false);
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
    await copyText(updated.share.url);
  }
}

async function unshareConversation(c) {
  const res = await openidFetch("/conversations/" + encodeURIComponent(c.id) + "/unshare", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: "{}",
  });
  if (!res.ok) {
    const msg = honestShareFailure(res.status, await res.text(), c);
    const hint = $("shareHint");
    if (hint) hint.textContent = msg;
    else setStatus(msg, false);
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
$("meFace").addEventListener("click", goYou);
$("youBtn").addEventListener("click", goYou);

const saveModal = $("saveModal");
$("saveBtn").addEventListener("click", () => {
  if (selectedKind === "account" && $("importText")) {
    $("importCard").scrollIntoView({ behavior: "smooth", block: "start" });
    $("importText").focus();
    return;
  }
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
  if (res.status === 200 || res.status === 201 || res.status === 204 || res.status === 409) return;
  if (res.ok) return;
  const got = await openidFetch(path, { headers: { Accept: "text/turtle, */*" } });
  if (got.ok) return;
  throw new Error("container " + path + " " + res.status + " " + (await res.text()));
}

async function saveViaHostedAPI(payload) {
  const res = await fetch("/api/spark-conversations", {
    method: "POST",
    headers: openidHeaders({ "Content-Type": "application/json" }),
    credentials: "include",
    body: JSON.stringify(payload),
  });
  const text = await res.text();
  let doc = {};
  try { doc = JSON.parse(text); } catch (e) { doc = { error: text }; }
  if (!res.ok) throw new Error(doc.error || text || ("save " + res.status));
  return doc.conversation ? Object.assign({}, doc.conversation, doc) : doc;
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

function looksLikeGeminiURL(s) {
  const t = String(s || "").trim();
  return /g\.co\/gemini\/share/i.test(t) || /gemini\.google\.com\/.*share/i.test(t);
}

async function importPublicGemini(url) {
  const res = await fetch("/api/import-gemini", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ url }),
  });
  let doc = {};
  try { doc = await res.json(); } catch (e) { doc = {}; }
  if (!res.ok) {
    throw new Error(doc.error || "This share link is not publicly readable. Paste the transcript instead.");
  }
  return doc;
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
  if (!messages.length) throw new Error("Paste a transcript, or a public Gemini share URL.");
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

async function submitSave(hint, payload, opts) {
  if (!hint) return;
  hint.textContent = "Saving…";
  hint.className = "hint";
  payload = Object.assign({ source: "gemini-spark" }, payload || {});
  try {
    if (looksLikeGeminiURL(payload.source_url) && !(payload.text || "").trim() && !(payload.messages && payload.messages.length)) {
      hint.textContent = "Fetching public Gemini share (no Google login)…";
      const imported = await importPublicGemini(payload.source_url);
      payload.title = payload.title || imported.title;
      payload.messages = imported.messages;
      payload.source_url = imported.source_url || payload.source_url;
    }
  } catch (e) {
    hint.textContent = String(e.message || e);
    hint.className = "hint bad";
    return;
  }
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
        // Prefer same-origin API → Railway LDP (creates conversations/ + spark/ first).
        // Browser PUTs through the Vercel catch-all often 404 for new pods.
        saved = await saveViaHostedAPI(payload);
      } catch (apiErr) {
        try {
          saved = await saveViaLDP(payload);
        } catch (ldpErr) {
          hint.textContent = String(apiErr.message || apiErr) + " / " + String(ldpErr.message || ldpErr);
          hint.className = "hint bad";
          return;
        }
      }
    } else {
      hint.textContent = errText;
      hint.className = "hint bad";
      return;
    }
  }
  saveModal.classList.remove("open");
  (opts && opts.clear ? opts.clear : ["saveTitle", "saveURL", "saveText"]).forEach((id) => {
    const el = $(id);
    if (el) el.value = "";
  });
  conversations.unshift(saved);
  selected = saved.id;
  selectedKind = "spark";
  await refresh();
}

$("saveForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  await submitSave($("saveHint"), {
    title: $("saveTitle").value,
    source_url: $("saveURL").value,
    text: $("saveText").value,
    source: "gemini-spark",
  });
});

(async function boot() {
  account = await me();
  if (!account) return;
  $("who").textContent = account.handle || account.name || "";
  $("meFace").style.setProperty("--h", String(hueFor(account.handle)));
  document.title = (account.handle || "oid") + " — oid";
  setStatus(account.handle || "signed in");
  selectedKind = "account";
  const pathLabel = $("podPathLabel");
  if (pathLabel) pathLabel.textContent = sparkDir();
  await probePod();
  await refresh();
})();
