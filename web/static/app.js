const tokenKey = "openid.token";
const handleKey = "openid.handle";
const $ = (id) => document.getElementById(id);

let token = localStorage.getItem(tokenKey) || "";
let account = null;
let traces = [];
let selected = null;

function headers(extra) {
  return Object.assign({ Accept: "application/json" }, extra || {}, token ? { Authorization: "Bearer " + token } : {});
}

function setStatus(text) {
  $("status").textContent = text;
}

async function me() {
  if (!token) {
    location.replace("/");
    return null;
  }
  const res = await fetch(openidURL("/idp/accounts/me"), { headers: headers() });
  if (!res.ok) {
    localStorage.removeItem(tokenKey);
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

function visible() {
  const q = ($("q").value || "").toLowerCase().trim();
  return traces.filter((t) => {
    if (!q) return true;
    const hay = [
      field(t, "name", "schema:name"),
      field(t, "package"),
      field(t, "description"),
      field(t, "workType"),
    ].join(" ").toLowerCase();
    return hay.includes(q);
  });
}

function renderList() {
  const rows = visible();
  if (!rows.length) {
    $("list").innerHTML = `<li class="empty">${traces.length ? "No matches." : "No conversations yet."}</li>`;
    return;
  }
  $("list").innerHTML = rows.slice(0, 400).map((t) => {
    const id = t.identifier || "";
    const name = field(t, "name", "schema:name") || id;
    const pkg = field(t, "package");
    return `<li data-id="${escapeHtml(id)}" class="${selected === id ? "on" : ""}">
      <span class="orb" style="--h:${hueFor(pkg)}"></span>
      <span><strong>${escapeHtml(name)}</strong>${pkg ? `<small>${escapeHtml(pkg)}</small>` : ""}</span>
    </li>`;
  }).join("");
  $("list").querySelectorAll("li[data-id]").forEach((li) => {
    li.addEventListener("click", () => {
      selected = li.dataset.id;
      renderList();
      showDetail(selected);
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
    <p><a href="${escapeHtml(account.publicUrl || "/i/" + account.handle)}">Public page →</a></p>`;
}

function showDetail(id) {
  const t = traces.find((x) => x.identifier === id);
  if (!t) {
    showAccount();
    return;
  }
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

async function loadRecords(handle) {
  try {
    const res = await fetch(openidURL("/" + handle + "/records/cursor/transcripts.jsonld"), {
      headers: headers({ Accept: "application/ld+json, application/json" }),
    });
    if (!res.ok) {
      traces = [];
      renderList();
      showAccount();
      return;
    }
    const doc = await res.json();
    traces = Array.isArray(doc) ? doc : doc["@graph"] || [doc];
  } catch (e) {
    traces = [];
  }
  renderList();
  if (selected) showDetail(selected);
  else showAccount();
}

$("logout").addEventListener("click", async () => {
  await fetch(openidURL("/idp/logout"), { method: "POST" });
  localStorage.removeItem(tokenKey);
  location.replace("/");
});

$("q").addEventListener("input", renderList);
$("meFace").addEventListener("click", () => {
  selected = null;
  renderList();
  showAccount();
});

(async function boot() {
  account = await me();
  if (!account) return;
  $("who").textContent = account.handle || account.name || "";
  $("meFace").style.setProperty("--h", String(hueFor(account.handle)));
  document.title = (account.handle || "oid") + " — oid";
  setStatus(account.handle || "signed in");
  await loadRecords(account.handle || "mike");
})();
