const tokenKey = "openid.token";
const $ = (id) => document.getElementById(id);
let token = localStorage.getItem(tokenKey) || "";
let handle = "mike";
let traces = [];
let catalog = null;
let grokDoc = {};
let sparqlIDs = null;
let cwd = [];
let selected = null;

function headers() {
  const h = { Accept: "application/ld+json, application/json" };
  if (token) h.Authorization = "Bearer " + token;
  return h;
}

function setToken(t) {
  token = t || "";
  if (token) localStorage.setItem(tokenKey, token);
  else localStorage.removeItem(tokenKey);
}

function fmt(n) {
  return new Intl.NumberFormat().format(n);
}

function when(iso) {
  if (!iso) return "";
  return String(iso).replace("T", " ").replace(/\.\d+Z?$/, "").slice(0, 16);
}

function graph(doc) {
  if (Array.isArray(doc)) return doc;
  if (doc && Array.isArray(doc["@graph"])) return doc["@graph"];
  return doc ? [doc] : [];
}

function field(t, ...keys) {
  for (const k of keys) {
    if (t[k] != null && t[k] !== "") return t[k];
  }
  return "";
}

function haystack(t) {
  return [
    field(t, "name", "schema:name"),
    field(t, "description", "oid:query", "query"),
    field(t, "workType", "genre"),
    field(t, "domain", "about"),
    field(t, "package", "oid:package"),
    field(t, "artifact"),
    field(t, "identifier"),
    field(t, "sourcePath", "oid:path"),
    [].concat(t.keywords || []).join(" "),
  ].join(" ").toLowerCase();
}

function queryTerms() {
  return ($("q").value || "").trim().toLowerCase().split(/\s+/).filter(Boolean);
}

function matchesSearch(t) {
  const terms = queryTerms();
  if (!terms.length) return true;
  const hay = haystack(t);
  return terms.every((term) => hay.includes(term));
}

function highlight(text) {
  const raw = String(text || "");
  const terms = queryTerms().filter((t) => t.length > 1);
  if (!terms.length) return escapeHtml(raw);
  const re = new RegExp("(" + terms.map((t) => t.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")).join("|") + ")", "ig");
  return escapeHtml(raw).replace(re, "<mark>$1</mark>");
}

async function api(path) {
  const res = await fetch(openidURL(path), { headers: headers() });
  if (res.status === 401 || res.status === 403) {
    throw Object.assign(new Error("auth"), { status: res.status });
  }
  if (!res.ok) throw new Error(path + " " + res.status);
  const ct = res.headers.get("content-type") || "";
  if (ct.includes("json")) return res.json();
  return res.text();
}

function enterArchive() {
  document.body.classList.add("in");
  $("gate").hidden = true;
  $("archive").hidden = false;
  $("findForm").hidden = false;
}

function leaveArchive() {
  document.body.classList.remove("in");
  $("archive").hidden = true;
  $("findForm").hidden = true;
  $("gate").hidden = false;
  cwd = [];
  selected = null;
}

function setQuery(value, push) {
  $("q").value = value || "";
  $("clearQ").hidden = !$("q").value;
  const url = new URL(location.href);
  if ($("q").value) url.searchParams.set("q", $("q").value);
  else url.searchParams.delete("q");
  if (push !== false) history.replaceState({}, "", url);
  renderDir();
}

$("loginForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  $("gateMsg").textContent = "Opening…";
  $("gateMsg").className = "auth-msg";
  const fd = new FormData(e.target);
  const res = await fetch(openidURL("/idp/login"), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ handle: fd.get("handle"), password: fd.get("password") }),
  });
  if (!res.ok) {
    $("gateMsg").textContent = "That handle and password did not match.";
    $("gateMsg").className = "auth-msg bad";
    return;
  }
  setToken((await res.json()).token);
  enterArchive();
  openArchive();
});

$("logout").addEventListener("click", () => {
  setToken("");
  sparqlIDs = null;
  leaveArchive();
});

$("findForm").addEventListener("submit", (e) => {
  e.preventDefault();
  const raw = $("q").value.trim();
  if (/^\s*(PREFIX|BASE|SELECT|ASK|CONSTRUCT|DESCRIBE)\b/i.test(raw)) {
    $("sparql").value = raw;
    runSPARQL(raw);
    return;
  }
  setQuery(raw);
});

$("q").addEventListener("input", () => {
  sparqlIDs = null;
  setQuery($("q").value);
});
$("clearQ").addEventListener("click", () => {
  sparqlIDs = null;
  setQuery("");
  $("q").focus();
});

$("sparqlForm").addEventListener("submit", (e) => {
  e.preventDefault();
  runSPARQL($("sparql").value);
});

$("pwForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  $("pwMsg").textContent = "";
  const password = new FormData(e.target).get("password");
  const res = await fetch(openidURL("/idp/profile"), {
    method: "PATCH",
    headers: Object.assign({ "Content-Type": "application/json" }, headers()),
    body: JSON.stringify({ password }),
  });
  $("pwMsg").textContent = res.ok ? "Password saved." : "Could not update password.";
  if (res.ok) e.target.reset();
});

document.querySelectorAll("[data-reveal]").forEach((btn) => {
  btn.addEventListener("click", () => {
    const input = btn.parentElement.querySelector("input");
    const show = input.type === "password";
    input.type = show ? "text" : "password";
    btn.textContent = show ? "Hide" : "Show";
  });
});

document.addEventListener("keydown", (e) => {
  if (e.key === "/" && document.activeElement.tagName !== "INPUT") {
    e.preventDefault();
    $("q").focus();
  }
  if (e.key === "Escape") {
    if (document.activeElement === $("q") && $("q").value) setQuery("");
    else if (cwd.length) {
      cwd = cwd.slice(0, -1);
      selected = null;
      renderDir();
    }
  }
});

async function openArchive() {
  try {
    const me = await api("/idp/accounts/me");
    handle = me.handle || "mike";
    catalog = await api("/" + handle + "/records/catalog.jsonld");
    traces = graph(await api("/" + handle + "/records/cursor/transcripts.jsonld"));
    grokDoc = await api("/" + handle + "/records/grokbot/settings.jsonld").catch(() => ({}));
    enterArchive();
    $("whoLine").textContent = (me.name || me.handle) + " · /" + handle + "/records/";
    $("rawLink").href = "/" + handle + "/records/catalog.jsonld";
    $("rdfLink").href = "/" + handle + "/records/catalog.ttl";
    renderStats();
    renderGrok(grokDoc, me);
    renderExamples();
    const start = new URLSearchParams(location.search).get("q") || "";
    setQuery(start, false);
    $("q").focus();
  } catch (err) {
    setToken("");
    leaveArchive();
    if (err.status) {
      $("gateMsg").textContent = "This archive is private. Sign in as the pod owner.";
      $("gateMsg").className = "auth-msg bad";
    }
  }
}

function packages() {
  const map = {};
  traces.forEach((t) => {
    const p = field(t, "package") || "unsorted";
    map[p] = (map[p] || 0) + 1;
  });
  return map;
}

function latestIn(pkg) {
  let latest = "";
  traces.forEach((t) => {
    if ((field(t, "package") || "unsorted") !== pkg) return;
    const d = field(t, "dateModified", "dateCreated");
    if (d > latest) latest = d;
  });
  return latest;
}

function dirEntries() {
  const q = $("q").value.trim();
  if (q) {
    return visibleTraces().slice(0, 300).map((t) => ({
      kind: "file",
      id: t.identifier,
      name: field(t, "name", "schema:name") || t.identifier,
      meta: ["traces", field(t, "package") || "unsorted"].join("/"),
      when: field(t, "dateModified", "dateCreated"),
      trace: t,
    }));
  }
  if (cwd.length === 0) {
    const pkgs = packages();
    const n = Object.values(pkgs).reduce((a, b) => a + b, 0);
    return [
      { kind: "folder", id: "traces", name: "traces", meta: n + " files", when: "" },
      { kind: "folder", id: "grokbot", name: "grokbot", meta: "settings", when: "" },
      { kind: "file", id: "catalog.jsonld", name: "catalog.jsonld", meta: "JSON-LD", when: catalog && catalog.dateModified },
      { kind: "file", id: "catalog.ttl", name: "catalog.ttl", meta: "Turtle", when: catalog && catalog.dateModified },
    ];
  }
  if (cwd[0] === "traces" && cwd.length === 1) {
    return Object.entries(packages())
      .sort((a, b) => b[1] - a[1])
      .map(([name, n]) => ({
        kind: "folder",
        id: name,
        name,
        meta: n + (n === 1 ? " file" : " files"),
        when: latestIn(name),
      }));
  }
  if (cwd[0] === "traces" && cwd[1]) {
    return visibleTraces()
      .filter((t) => (field(t, "package") || "unsorted") === cwd[1])
      .slice(0, 400)
      .map((t) => ({
        kind: "file",
        id: t.identifier,
        name: field(t, "name", "schema:name") || t.identifier,
        meta: field(t, "artifact") || "trace",
        when: field(t, "dateModified", "dateCreated"),
        trace: t,
      }));
  }
  if (cwd[0] === "grokbot") {
    return [{ kind: "file", id: "settings.jsonld", name: "settings.jsonld", meta: "JSON-LD", when: "" }];
  }
  return [];
}

function visibleTraces() {
  return traces.filter((t) => {
    if (!matchesSearch(t)) return false;
    if (sparqlIDs && !sparqlIDs.has(t.identifier)) return false;
    return true;
  });
}

function renderCrumbs() {
  const parts = ["/" + handle + "/records"].concat(cwd);
  $("crumbs").innerHTML = parts.map((part, i) => {
    const last = i === parts.length - 1;
    const label = i === 0 ? "/" + handle + "/records" : part;
    if (last) return `<span class="here">${escapeHtml(label)}</span>`;
    return `<button type="button" data-depth="${i}">${escapeHtml(label)}</button><span class="sep">/</span>`;
  }).join("");
  $("crumbs").querySelectorAll("button").forEach((btn) => {
    btn.addEventListener("click", () => {
      cwd = cwd.slice(0, Number(btn.dataset.depth));
      selected = null;
      renderDir();
    });
  });
}

function renderDir() {
  const entries = dirEntries();
  const q = $("q").value.trim();
  $("traceMeta").textContent = q
    ? fmt(entries.length) + " matches for “" + q + "”"
    : fmt(entries.length) + (entries.length === 1 ? " item" : " items");
  renderCrumbs();
  $("listing").innerHTML = entries.map((e) => `
    <button type="button" class="dir-row${selected && selected.id === e.id ? " on" : ""}" data-id="${escapeHtml(e.id)}" data-kind="${e.kind}">
      <span class="ico">${e.kind === "folder" ? "▸" : "·"}</span>
      <span class="name"><strong>${q && e.kind === "file" ? highlight(e.name) : escapeHtml(e.name)}</strong></span>
      <span class="meta">${escapeHtml(e.meta || "")}</span>
      <span class="when">${escapeHtml(when(e.when))}</span>
    </button>`).join("") || `<div class="empty">This folder is empty.</div>`;
  $("listing").querySelectorAll(".dir-row").forEach((btn) => {
    btn.addEventListener("click", () => openEntry(btn.dataset.kind, btn.dataset.id));
  });
  if (!selected) renderPreview(null);
}

function openEntry(kind, id) {
  if (kind === "folder") {
    cwd = cwd.concat([id]);
    selected = null;
    renderDir();
    return;
  }
  if (!queryTerms().length && cwd.length === 0 && (id === "catalog.jsonld" || id === "catalog.ttl")) {
    selected = { id, kind: "catalog", name: id };
    renderDir();
    renderPreview(selected);
    return;
  }
  if (id === "settings.jsonld") {
    selected = { id, kind: "settings", name: id };
    renderDir();
    renderPreview(selected);
    return;
  }
  const t = traces.find((x) => x.identifier === id);
  selected = t ? { id, kind: "trace", name: field(t, "name", "schema:name") || id, trace: t } : { id, kind: "file", name: id };
  renderDir();
  renderPreview(selected);
}

function renderPreview(item) {
  const el = $("preview");
  if (!item) {
    el.innerHTML = `<p class="hint">Open a folder or select a file.</p>`;
    return;
  }
  if (item.kind === "catalog") {
    const c = (catalog && catalog.counts) || {};
    el.innerHTML = `<h2>${escapeHtml(item.name)}</h2>
      <p class="mono path">/${handle}/records/${escapeHtml(item.name)}</p>
      <dl class="kv">
        <dt>Traces</dt><dd>${fmt(c.traces || traces.length)}</dd>
        <dt>Packages</dt><dd>${fmt(c.packages || Object.keys(packages()).length)}</dd>
      </dl>
      <p><a class="btn ghost" href="/${handle}/records/${escapeHtml(item.name)}">Open raw</a></p>`;
    return;
  }
  if (item.kind === "settings") {
    const settings = grokDoc.json || grokDoc;
    el.innerHTML = `<h2>settings.jsonld</h2>
      <p class="mono path">/${handle}/records/grokbot/settings.jsonld</p>
      <dl class="kv">
        <dt>Tools</dt><dd>${escapeHtml(settings.localToolPermission || "—")}</dd>
        <dt>MCP</dt><dd>${escapeHtml(Object.keys(settings.mcpCustomInstructions || {}).join(", ") || "none")}</dd>
      </dl>`;
    return;
  }
  const t = item.trace;
  if (!t) {
    el.innerHTML = `<p class="hint">Unknown file.</p>`;
    return;
  }
  el.innerHTML = `<h2>${escapeHtml(field(t, "name", "schema:name") || t.identifier)}</h2>
    <p class="mono path">/${handle}/records/traces/${escapeHtml(field(t, "package") || "unsorted")}/${escapeHtml(t.identifier)}.jsonld</p>
    <p class="lede">${escapeHtml(field(t, "description", "oid:query", "query"))}</p>
    <dl class="kv">
      <dt>Work</dt><dd>${escapeHtml(field(t, "workType", "genre") || "—")}</dd>
      <dt>Domain</dt><dd>${escapeHtml(field(t, "domain", "about") || "—")}</dd>
      <dt>Kind</dt><dd>${escapeHtml(field(t, "artifact") || "—")}</dd>
      <dt>Modified</dt><dd>${escapeHtml(when(field(t, "dateModified", "dateCreated")) || "—")}</dd>
    </dl>`;
}

function renderStats() {
  const c = (catalog && catalog.counts) || {};
  $("stats").innerHTML = [
    [fmt(c.traces || traces.length), "Traces"],
    [fmt((c.byWorkType && Object.keys(c.byWorkType).length) || 0), "Work types"],
    [fmt(c.packages || 0), "Packages"],
    ["$" + fmt(c.listPriceUSD || 0), "List"],
  ].map(([n, l]) => `<div><strong>${n}</strong><span>${l}</span></div>`).join("");
}

function renderGrok(grok, me) {
  const settings = grok.json || grok;
  $("grokKV").innerHTML = [
    ["Tools", settings.localToolPermission || "—"],
    ["MCP", Object.keys(settings.mcpCustomInstructions || {}).join(", ") || "none"],
  ].map(([k, v]) => `<dt>${k}</dt><dd>${v}</dd>`).join("");
  $("podKV").innerHTML = [
    ["Handle", me.handle || handle],
    ["WebID", me.webId || ""],
    ["Updated", when(catalog && catalog.dateModified)],
  ].map(([k, v]) => `<dt>${k}</dt><dd>${v}</dd>`).join("");
}

function wrapSPARQL(raw) {
  const q = (raw || "").trim();
  if (!q) {
    return `SELECT ?id ?name ?package WHERE {
  ?id a oid:AgentTrace ; schema:name ?name ; oid:package ?package .
} LIMIT 200`;
  }
  if (/^\s*(PREFIX|BASE|SELECT|ASK|CONSTRUCT|DESCRIBE)\b/i.test(q)) return q;
  const needle = q.replace(/\\/g, "\\\\").replace(/"/g, '\\"').toLowerCase();
  return `SELECT ?id ?name ?package WHERE {
  ?id a oid:AgentTrace ; schema:name ?name ; oid:package ?package .
  FILTER(CONTAINS(LCASE(STR(?name)), "${needle}") || CONTAINS(LCASE(STR(?package)), "${needle}"))
} LIMIT 200`;
}

function renderExamples() {
  const examples = [
    ["All traces", ""],
    ["Implementation", "implementation"],
  ];
  $("sparqlExamples").innerHTML = examples.map(([label]) =>
    `<button type="button">${escapeHtml(label)}</button>`
  ).join("");
  $("sparqlExamples").querySelectorAll("button").forEach((btn, i) => {
    btn.addEventListener("click", () => {
      $("sparql").value = examples[i][1];
      runSPARQL(examples[i][1]);
    });
  });
}

async function runSPARQL(raw) {
  const query = wrapSPARQL(raw);
  $("sparqlSent").hidden = false;
  $("sparqlSent").textContent = query;
  $("sparqlMeta").textContent = "Running…";
  try {
    const res = await fetch(openidURL("/records/sparql"), {
      method: "POST",
      headers: Object.assign({ "Content-Type": "application/sparql-query", Accept: "application/sparql-results+json" }, headers()),
      body: query,
    });
    const text = await res.text();
    if (!res.ok) throw new Error(text || res.status);
    const data = JSON.parse(text);
    const vars = (data.head && data.head.vars) || [];
    const rows = (data.results && data.results.bindings) || [];
    $("sparqlMeta").textContent = fmt(rows.length) + " binding" + (rows.length === 1 ? "" : "s");
    $("sparqlResults").innerHTML = renderSPARQLTable(vars, rows);
    sparqlIDs = idsFromBindings(rows);
    renderDir();
  } catch (err) {
    $("sparqlMeta").textContent = "Query failed";
    $("sparqlResults").innerHTML = `<p class="msg">${escapeHtml(err.message || String(err))}</p>`;
  }
}

function renderSPARQLTable(vars, rows) {
  if (!rows.length) return "<p class=\"hint\">No solutions.</p>";
  const head = vars.map((v) => `<th>?${escapeHtml(v)}</th>`).join("");
  const body = rows.slice(0, 200).map((row) =>
    `<tr>${vars.map((v) => `<td>${escapeHtml(cellValue(row[v]))}</td>`).join("")}</tr>`
  ).join("");
  return `<table><thead><tr>${head}</tr></thead><tbody>${body}</tbody></table>`;
}

function cellValue(cell) {
  if (!cell) return "";
  const v = cell.value || "";
  return cell.type === "uri" ? v.replace(/^https?:\/\/[^/]+/, "") : v;
}

function idsFromBindings(rows) {
  const ids = new Set();
  let saw = false;
  rows.forEach((row) => {
    Object.values(row).forEach((cell) => {
      if (!cell || cell.type !== "uri") return;
      const v = cell.value || "";
      if (v.includes("/traces/") || v.includes("AgentTrace")) {
        saw = true;
        const m = v.match(/\/([A-Za-z0-9_-]+)\.jsonld$/);
        if (m) ids.add(m[1]);
        const t = traces.find((x) => x["@id"] === v);
        if (t) ids.add(t.identifier);
      }
    });
  });
  return saw ? ids : null;
}

function escapeHtml(s) {
  return String(s).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
}

if (token) openArchive();
else {
  leaveArchive();
  const pending = new URLSearchParams(location.search).get("q");
  if (pending) $("q").value = pending;
}
