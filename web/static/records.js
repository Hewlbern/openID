const tokenKey = "openid.token";
const $ = (id) => document.getElementById(id);
let token = localStorage.getItem(tokenKey) || "";
let traces = [];
let catalog = null;
let filters = { workType: "all", domain: "all", package: "all" };
let sparqlIDs = null;
let lastSparql = "";

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
  if (!iso) return "—";
  return String(iso).replace("T", " ").replace("Z", " UTC");
}

function graph(doc) {
  if (Array.isArray(doc)) return doc;
  if (doc && Array.isArray(doc["@graph"])) return doc["@graph"];
  return doc ? [doc] : [];
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

$("loginForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  $("gateMsg").textContent = "";
  const fd = new FormData(e.target);
  const res = await fetch(openidURL("/idp/login"), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ handle: fd.get("handle"), password: fd.get("password") }),
  });
  if (!res.ok) {
    $("gateMsg").textContent = "Could not sign in. Check handle and password.";
    return;
  }
  setToken((await res.json()).token);
  openArchive();
});

$("logout").addEventListener("click", () => {
  setToken("");
  sparqlIDs = null;
  $("archive").hidden = true;
  $("sparqlForm").hidden = true;
  $("sparqlCard").hidden = true;
  $("gate").hidden = false;
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
  $("pwMsg").textContent = res.ok ? "Password saved. Use it the next time you sign in." : "Could not update password.";
  if (res.ok) e.target.reset();
});

async function openArchive() {
  try {
    catalog = await api("/mike/records/catalog.jsonld");
    traces = graph(await api("/mike/records/cursor/transcripts.jsonld"));
    const grokDoc = await api("/mike/records/grokbot/settings.jsonld").catch(() => ({}));
    const me = await api("/idp/accounts/me");
    $("gate").hidden = true;
    $("archive").hidden = false;
    $("sparqlForm").hidden = false;
    $("whoLine").textContent = (me.name || me.handle) + " · JSON-LD archive · SPARQL on /records/sparql";
    renderStats();
    renderGrok(grokDoc, me);
    renderExamples();
    renderFacets();
    renderTraces();
  } catch (err) {
    setToken("");
    $("archive").hidden = true;
    $("gate").hidden = false;
    if (err.status) $("gateMsg").textContent = "This archive is private. Sign in as the pod owner.";
  }
}

function renderStats() {
  const c = catalog.counts || {};
  $("stats").innerHTML = [
    [fmt(c.traces || traces.length), "Traces"],
    [fmt((c.byWorkType && Object.keys(c.byWorkType).length) || 0), "Work types"],
    [fmt(c.packages || 0), "Packages"],
    ["$" + fmt(c.listPriceUSD || 0), "List (not for sale)"],
  ].map(([n, l]) => `<div><strong>${n}</strong><span>${l}</span></div>`).join("");
}

function renderGrok(grok, me) {
  const settings = grok.json || grok;
  $("grokKV").innerHTML = [
    ["Type", "oid:AgentSettings"],
    ["Tools", settings.localToolPermission || "—"],
    ["MCP", Object.keys(settings.mcpCustomInstructions || {}).join(", ") || "none"],
    ["Package", grok.package || "openid-agent-identity"],
  ].map(([k, v]) => `<dt>${k}</dt><dd>${v}</dd>`).join("");
  $("podKV").innerHTML = [
    ["Handle", me.handle || "mike"],
    ["WebID", me.webId || ""],
    ["Catalog", "catalog.jsonld · catalog.ttl"],
    ["Updated", when(catalog.dateModified)],
  ].map(([k, v]) => `<dt>${k}</dt><dd>${v}</dd>`).join("");
}

function facetButtons(el, field, counts) {
  const keys = ["all", ...Object.keys(counts).sort((a, b) => counts[b] - counts[a])];
  el.innerHTML = keys.map((k) =>
    `<button type="button" data-val="${k}" class="${filters[field] === k ? "on" : ""}">${k} · ${k === "all" ? traces.length : counts[k]}</button>`
  ).join("");
  el.querySelectorAll("button").forEach((btn) => {
    btn.addEventListener("click", () => {
      filters[field] = btn.dataset.val;
      renderFacets();
      renderTraces();
    });
  });
}

function renderFacets() {
  const work = {}, domain = {}, pkg = {};
  traces.forEach((t) => {
    work[t.workType || t.genre] = (work[t.workType || t.genre] || 0) + 1;
    domain[t.domain || t.about] = (domain[t.domain || t.about] || 0) + 1;
    pkg[t.package] = (pkg[t.package] || 0) + 1;
  });
  facetButtons($("workTypes"), "workType", work);
  facetButtons($("domains"), "domain", domain);
  facetButtons($("packages"), "package", pkg);
}

function wrapSPARQL(raw) {
  const q = (raw || "").trim();
  if (!q) {
    return `SELECT ?id ?name ?workType ?domain ?package ?artifact ?price WHERE {
  ?id a oid:AgentTrace ;
      schema:name ?name ;
      oid:workType ?workType ;
      oid:domain ?domain ;
      oid:package ?package ;
      oid:artifact ?artifact .
  OPTIONAL { ?id schema:offers ?off . ?off schema:price ?price . }
} ORDER BY DESC(?price) LIMIT 200`;
  }
  if (/^\s*(PREFIX|BASE|SELECT|ASK|CONSTRUCT|DESCRIBE)\b/i.test(q)) return q;
  const needle = q.replace(/\\/g, "\\\\").replace(/"/g, '\\"').toLowerCase();
  return `SELECT ?id ?name ?workType ?domain ?package ?artifact ?price WHERE {
  ?id a oid:AgentTrace ;
      schema:name ?name ;
      oid:workType ?workType ;
      oid:domain ?domain ;
      oid:package ?package ;
      oid:artifact ?artifact .
  OPTIONAL { ?id schema:offers ?off . ?off schema:price ?price . }
  FILTER(
    CONTAINS(LCASE(STR(?name)), "${needle}") ||
    CONTAINS(LCASE(STR(?workType)), "${needle}") ||
    CONTAINS(LCASE(STR(?domain)), "${needle}") ||
    CONTAINS(LCASE(STR(?package)), "${needle}") ||
    CONTAINS(LCASE(STR(?artifact)), "${needle}")
  )
} ORDER BY DESC(?price) LIMIT 200`;
}

function renderExamples() {
  const examples = [
    ["All traces", ""],
    ["Implementation", "implementation"],
    ["Identity work", `SELECT ?name ?workType ?package WHERE {
  ?id a oid:AgentTrace ; schema:name ?name ; oid:workType ?workType ; oid:package ?package .
  FILTER(?workType = "identity-protocol")
}`],
    ["Packages", `SELECT DISTINCT ?package ?domain WHERE {
  ?s oid:package ?package ; oid:domain ?domain
} ORDER BY ?package`],
    ["Over $20", `SELECT ?name ?price ?package WHERE {
  ?id schema:name ?name ; oid:package ?package ; schema:offers ?off .
  ?off schema:price ?price .
  FILTER(?price >= 20)
} ORDER BY DESC(?price)`],
  ];
  $("sparqlExamples").innerHTML = examples.map(([label]) =>
    `<button type="button" data-ex="${escapeHtml(label)}">${label}</button>`
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
  lastSparql = query;
  $("sparqlCard").hidden = false;
  $("sparqlSent").textContent = query;
  $("sparqlMeta").textContent = "Running…";
  $("sparqlResults").innerHTML = "";
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
    if (sparqlIDs && sparqlIDs.size === 0) sparqlIDs = new Set();
    renderTraces();
  } catch (err) {
    $("sparqlMeta").textContent = "Query failed";
    $("sparqlResults").innerHTML = `<p class="msg">${escapeHtml(err.message || String(err))}</p>`;
  }
}

function renderSPARQLTable(vars, rows) {
  if (!vars.length) return "<p class=\"hint\">No variables.</p>";
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
  if (cell.type === "uri") return v.replace(/^https?:\/\/[^/]+/, "");
  return v;
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
        const m = v.match(/\/([a-f0-9-]{8,})\.jsonld$/i) || v.match(/\/([A-Za-z0-9_-]+)\.jsonld$/);
        if (m) ids.add(m[1]);
        const t = traces.find((x) => x["@id"] === v);
        if (t) ids.add(t.identifier);
      }
    });
  });
  return saw ? ids : null;
}

function renderTraces() {
  const rows = traces.filter((t) => {
    if (filters.workType !== "all" && (t.workType || t.genre) !== filters.workType) return false;
    if (filters.domain !== "all" && (t.domain || t.about) !== filters.domain) return false;
    if (filters.package !== "all" && t.package !== filters.package) return false;
    if (sparqlIDs && !sparqlIDs.has(t.identifier)) return false;
    return true;
  });
  $("traceMeta").textContent = fmt(rows.length) + " of " + fmt(traces.length) + " JSON-LD records";
  $("traceList").innerHTML = rows.slice(0, 200).map((t) => `
    <button type="button" class="trace" data-id="${t.identifier}">
      <span>
        <strong>${escapeHtml(t.name || t.identifier)}</strong>
        <small>${t.workType || t.genre} · ${t.domain || t.about} · ${t.package} · ${t.artifact}</small>
      </span>
      <span class="mono">$${t.offers && t.offers.price != null ? t.offers.price : "—"}</span>
    </button>`).join("") || "<div>No traces in this filter.</div>";
  $("traceList").querySelectorAll(".trace").forEach((btn) => {
    btn.addEventListener("click", () => showDetail(btn.dataset.id));
  });
}

function showDetail(id) {
  const t = traces.find((x) => x.identifier === id);
  if (!t) return;
  $("detailCard").hidden = false;
  $("detailTitle").textContent = t.name || id;
  $("detailQuery").textContent = t.description || "";
  $("detailPath").textContent = t.sourcePath || t["@id"];
  const types = [].concat(t["@type"] || []);
  $("detailKV").innerHTML = [
    ["@type", types.join(", ")],
    ["Work type", t.workType || t.genre],
    ["Domain", t.domain || t.about],
    ["Package", t.package],
    ["Artifact", t.artifact],
    ["Keywords", (t.keywords || []).join(", ")],
    ["Turns", (t.userTurns || 0) + " you · " + (t.assistantTurns || 0) + " assistant"],
    ["Offer", t.offers ? `USD ${t.offers.price} · ${String(t.offers.availability || "").split("/").pop()}` : "—"],
    ["IRI", t["@id"]],
  ].map(([k, v]) => `<dt>${k}</dt><dd>${escapeHtml(v || "—")}</dd>`).join("");
  $("detailCard").scrollIntoView({ behavior: "smooth", block: "start" });
}

function escapeHtml(s) {
  return String(s).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
}

if (token) openArchive();
else {
  $("archive").hidden = true;
  $("gate").hidden = false;
}
