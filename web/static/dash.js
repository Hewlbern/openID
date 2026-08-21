const $ = (id) => document.getElementById(id);
const tokenKey = "openid.token";
let token = localStorage.getItem(tokenKey) || "";

function headers(extra) {
  const h = Object.assign({ Accept: "application/json" }, extra || {});
  if (token) h.Authorization = "Bearer " + token;
  return h;
}

function setToken(t) {
  token = t || "";
  if (token) localStorage.setItem(tokenKey, token);
  else localStorage.removeItem(tokenKey);
}

document.querySelectorAll("#tabs button").forEach((btn) => {
  btn.addEventListener("click", () => {
    document.querySelectorAll("#tabs button").forEach((b) => b.classList.toggle("on", b === btn));
    document.querySelectorAll(".pane").forEach((p) => p.classList.toggle("on", p.id === btn.dataset.tab));
    if (btn.dataset.tab === "agents") loadAgents();
    if (btn.dataset.tab === "audit") loadAudit();
    if (btn.dataset.tab === "identity") loadSession();
  });
});

async function loadStatus() {
  const health = await fetch(openidURL("/health")).then((r) => r.json()).catch(() => ({ status: "down" }));
  const ok = health.status === "ok";
  $("healthPill").textContent = ok ? "healthy" : "down";
  $("healthPill").classList.toggle("bad", !ok);
  const st = await fetch(openidURL("/api/status")).then((r) => r.json());
  $("baseLabel").textContent = st.baseUrl;
  const rows = [
    ["Product", st.product + " " + st.version],
    ["Protocol", st.protocol],
    ["Base URL", st.baseUrl],
    ["Storage", st.storagePath || "(memory/file)"],
    ["Accounts", String(st.accounts)],
    ["Agents", String(st.agents)],
  ];
  $("statusList").innerHTML = rows.map(([k, v]) => `<dt>${k}</dt><dd>${v}</dd>`).join("");
  $("disco").innerHTML = Object.entries(st.endpoints || {})
    .map(([k, v]) => `<li><a href="${v}">${k}</a> <span class="mono">${v}</span></li>`)
    .join("");
}

async function loadSession() {
  $("sessionActions").innerHTML = "";
  if (!token) {
    $("who").textContent = "Not signed in.";
    $("webid").textContent = "";
    return;
  }
  const res = await fetch(openidURL("/idp/accounts/me"), { headers: headers() });
  if (!res.ok) {
    setToken("");
    $("who").textContent = "Not signed in.";
    $("webid").textContent = "";
    return;
  }
  const acc = await res.json();
  $("who").textContent = (acc.name || acc.handle) + " · /" + acc.podPath;
  $("webid").textContent = acc.webId;
  $("sessionActions").innerHTML = `
    <a class="btn" href="/app">Passport UI</a>
    <a class="btn" href="/records">Records</a>
    <a class="btn ghost" href="${acc.publicUrl || "/i/" + acc.handle}">Public page</a>
    <button class="btn ghost" id="logout" type="button">Log out</button>`;
  $("logout").addEventListener("click", async () => {
    await fetch(openidURL("/idp/logout"), { method: "POST" });
    setToken("");
    loadSession();
  });
  if (acc.podPath && !$("resPath").dataset.touched) {
    $("resPath").value = "/" + acc.podPath;
  }
}

$("loginForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  const id = e.target.id.value.trim();
  const payload = id.includes("@")
    ? { email: id, password: e.target.password.value }
    : { handle: id, password: e.target.password.value };
  const res = await fetch(openidURL("/idp/login"), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  if (!res.ok) {
    $("who").textContent = "Login failed";
    return;
  }
  const data = await res.json();
  setToken(data.token);
  loadSession();
  loadStatus();
});

$("registerForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  const res = await fetch(openidURL("/idp/register"), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      handle: e.target.handle.value,
      name: e.target.name.value,
      password: e.target.password.value,
      createPod: true,
    }),
  });
  if (!res.ok) {
    $("who").textContent = await res.text();
    return;
  }
  const data = await res.json();
  setToken(data.token);
  loadSession();
  loadStatus();
  $("resPath").value = "/" + data.account.podPath;
});

function childrenFromTurtle(text, base) {
  const found = [];
  const re = /ldp:contains\s+<([^>]+)>|<([^>]+)>\s+a\s+ldp:/g;
  let m;
  while ((m = re.exec(text))) {
    const href = m[1] || m[2];
    if (href && !found.includes(href)) found.push(href);
  }
  const abs = /<https?:\/\/[^>]+\/ns\/ldp#contains>\s+<([^>]+)>/g;
  while ((m = abs.exec(text))) {
    if (!found.includes(m[1])) found.push(m[1]);
  }
  return found.map((href) => {
    try {
      const u = new URL(href, base || location.origin);
      return { href, path: u.pathname + (u.pathname.endsWith("/") ? "" : "") };
    } catch {
      return { href, path: href };
    }
  });
}

async function browse(path) {
  path = path || "/";
  if (!path.startsWith("/")) path = "/" + path;
  $("resPath").value = path;
  $("browseMsg").textContent = "";
  const res = await fetch(openidURL(path), {
    headers: {
      Authorization: token ? "Bearer " + token : "",
      Accept: "text/turtle, text/plain, application/json, */*",
    },
  });
  const text = await res.text();
  $("resMeta").textContent = res.status + " " + (res.headers.get("Content-Type") || "") + "  etag=" + (res.headers.get("ETag") || "—");
  $("resBody").textContent = text;
  $("putBody").value = text;
  const kids = childrenFromTurtle(text, location.origin);
  $("children").innerHTML = "";
  if (path !== "/") {
    const up = path.replace(/\/?[^/]+\/?$/, "/") || "/";
    const b = document.createElement("button");
    b.type = "button";
    b.textContent = "..";
    b.addEventListener("click", () => browse(up));
    $("children").appendChild(b);
  }
  kids.forEach((c) => {
    const a = document.createElement("button");
    a.type = "button";
    a.textContent = c.path.replace(/\/$/, "").split("/").pop() + (c.path.endsWith("/") ? "/" : "");
    a.addEventListener("click", () => browse(c.path));
    $("children").appendChild(a);
  });
}

$("resPath").addEventListener("input", () => { $("resPath").dataset.touched = "1"; });
$("browseForm").addEventListener("submit", (e) => {
  e.preventDefault();
  browse($("resPath").value);
});
$("putForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  const path = $("resPath").value || "/";
  const res = await fetch(openidURL(path), {
    method: "PUT",
    headers: {
      Authorization: token ? "Bearer " + token : "",
      "Content-Type": $("putType").value || "text/plain",
    },
    body: $("putBody").value,
  });
  $("browseMsg").textContent = res.ok ? "Saved " + (res.headers.get("Location") || path) : "PUT failed (" + res.status + ")";
  if (res.ok) browse(path);
});

async function loadAgents() {
  const list = await fetch(openidURL("/agents"), { headers: headers() }).then((r) => r.json()).catch(() => []);
  $("agentList").innerHTML = (list || []).map((a) =>
    `<a href="${openidURL("/" + a.podPath + "profile/card")}"><span>${a.name}</span><span class="mono">${a.webId}</span></a>`
  ).join("") || "<div>No agents yet.</div>";
}

$("agentForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  const res = await fetch(openidURL("/agents"), {
    method: "POST",
    headers: headers({ "Content-Type": "application/json" }),
    body: JSON.stringify({ name: e.target.name.value || "agent" }),
  });
  const data = await res.json();
  $("agentSecret").textContent =
    "token: " + data.token + "\nwebId: " + data.agent.webId + "\nprivateKey: " + (data.privateKey || "");
  if (data.token && !token) setToken(data.token);
  loadAgents();
  loadStatus();
});

async function loadAudit() {
  const events = await fetch(openidURL("/audit/events/"), { headers: headers() }).then((r) => r.json()).catch(() => []);
  $("auditList").innerHTML = (events || []).slice().reverse().slice(0, 20).map((e) =>
    `<a href="/audit/events/${e.id}/verify"><span>${e.method || ""} ${e.resource || ""}</span><span class="mono">${e.id}</span></a>`
  ).join("") || "<div>No events yet.</div>";
}

$("flush").addEventListener("click", async () => {
  await fetch(openidURL("/audit/flush"), { method: "POST", headers: headers() });
  loadAudit();
});

loadStatus();
loadSession();
browse($("resPath").value || "/");
