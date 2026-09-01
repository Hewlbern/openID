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
    <p class="hint">Save a Gemini Spark conversation with <strong>Save</strong>, or connect Spark’s custom MCP to <span class="mono">${escapeHtml(location.origin)}/mcp</span>.</p>`;
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

async function loadConversations() {
  const res = await openidFetch("/conversations");
  if (!res.ok) {
    conversations = [];
    return;
  }
  const doc = await res.json();
  conversations = Array.isArray(doc) ? doc : doc.conversations || [];
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

$("saveForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  const hint = $("saveHint");
  hint.textContent = "Saving…";
  const res = await openidFetch("/conversations", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      title: $("saveTitle").value,
      source_url: $("saveURL").value,
      text: $("saveText").value,
      source: "gemini-spark",
    }),
  });
  if (!res.ok) {
    hint.textContent = await res.text();
    hint.className = "hint bad";
    return;
  }
  const saved = await res.json();
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
