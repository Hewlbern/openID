const peers = ["http://127.0.0.1:4000", "http://localhost:4000", "https://pod-production-ebe1.up.railway.app"];
const tokenKey = "openid.token";
const handleKey = "openid.handle";
const passKey = "openid.syncPassword";
const $ = (id) => document.getElementById(id);

let origin = "";
let token = localStorage.getItem(tokenKey) || "";
let traces = [];
let selected = null;

function api(path, opts) {
  const headers = Object.assign({ Accept: "application/json" }, (opts && opts.headers) || {});
  if (token) headers.Authorization = "Bearer " + token;
  return fetch(origin + path, Object.assign({}, opts, { headers }));
}

function setStatus(text) {
  $("status").textContent = text;
}

async function findPod() {
  for (const p of peers) {
    try {
      const r = await fetch(p + "/health", { signal: AbortSignal.timeout(1500) });
      if (r.ok) {
        origin = p;
        setStatus(p.includes("railway") ? "Railway" : "Local");
        return true;
      }
    } catch (e) {}
  }
  setStatus("No pod");
  $("gateLead").textContent = "Start the pod, then sign in.";
  return false;
}

function showGate() {
  $("gate").hidden = false;
  $("shell").hidden = true;
}

function showShell() {
  $("gate").hidden = true;
  $("shell").hidden = false;
}

async function loadMe() {
  if (!token) return false;
  const res = await api("/idp/accounts/me");
  if (!res.ok) {
    token = "";
    localStorage.removeItem(tokenKey);
    return false;
  }
  const me = await res.json();
  $("who").textContent = me.handle || me.name || "";
  $("meFace").textContent = "";
  document.title = (me.handle || "oid") + " — oid";
  await loadRecords(me.handle || "mike");
  showShell();
  return true;
}

async function loadRecords(handle) {
  try {
    const res = await api("/" + handle + "/records/cursor/transcripts.jsonld", {
      headers: { Accept: "application/ld+json, application/json" },
    });
    if (!res.ok) {
      traces = [];
      renderList();
      return;
    }
    const doc = await res.json();
    traces = Array.isArray(doc) ? doc : doc["@graph"] || [doc];
  } catch (e) {
    traces = [];
  }
  renderList();
}

function field(t, ...keys) {
  for (const k of keys) {
    if (t[k] != null && t[k] !== "") return t[k];
  }
  return "";
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

function hueFor(s) {
  let n = 18;
  String(s || "").split("").forEach((c) => { n = (n * 31 + c.charCodeAt(0)) % 360; });
  return n;
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

function showDetail(id) {
  const t = traces.find((x) => x.identifier === id);
  if (!t) {
    $("detail").innerHTML = `<div class="empty-state"><span class="mark hero" aria-hidden="true"></span><p>Pick a conversation.</p></div>`;
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

function escapeHtml(s) {
  return String(s).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
}

$("login").addEventListener("submit", async (e) => {
  e.preventDefault();
  $("gateErr").textContent = "";
  $("loginBtn").disabled = true;
  try {
    if (!origin && !(await findPod())) {
      $("gateErr").textContent = "No pod is running.";
      return;
    }
    const fd = new FormData(e.target);
    const res = await api("/idp/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ handle: fd.get("handle"), password: fd.get("password") }),
    });
    if (!res.ok) {
      $("gateErr").textContent = "Could not sign in.";
      return;
    }
    const data = await res.json();
    token = data.token;
    localStorage.setItem(tokenKey, token);
    localStorage.setItem(handleKey, fd.get("handle"));
    localStorage.setItem(passKey, fd.get("password"));
    if (!(await loadMe())) $("gateErr").textContent = "Signed in, but the session failed.";
  } catch (err) {
    $("gateErr").textContent = "Could not reach the pod.";
  } finally {
    $("loginBtn").disabled = false;
  }
});

$("logout").addEventListener("click", () => {
  token = "";
  traces = [];
  selected = null;
  localStorage.removeItem(tokenKey);
  showGate();
  const handle = localStorage.getItem(handleKey);
  if (handle) $("login").handle.value = handle;
  setStatus(origin.includes("railway") ? "Railway" : origin ? "Local" : "No pod");
});

async function silentLogin() {
  const handle = localStorage.getItem(handleKey);
  const password = localStorage.getItem(passKey);
  if (!handle || !password || !origin) return false;
  const res = await api("/idp/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ handle, password }),
  });
  if (!res.ok) return false;
  const data = await res.json();
  token = data.token;
  localStorage.setItem(tokenKey, token);
  return loadMe();
}

$("q").addEventListener("input", renderList);

(async function boot() {
  const savedHandle = localStorage.getItem(handleKey);
  if (savedHandle) $("login").handle.value = savedHandle;
  const savedPass = localStorage.getItem(passKey);
  if (savedPass) $("login").password.value = savedPass;
  await findPod();
  if (token && origin && (await loadMe())) return;
  if (await silentLogin()) return;
  showGate();
})();
