const $ = (id) => document.getElementById(id);
const tokenKey = "openid.token";
let token = localStorage.getItem(tokenKey) || "";
let account = null;
let handleTimer = 0;

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

function field(form, name) {
  return form.elements.namedItem(name);
}

function say(role, text) {
  const div = document.createElement("div");
  div.className = "bubble " + (role === "me" ? "me-msg" : "bot-msg");
  div.textContent = text;
  $("stream").appendChild(div);
}

function showAuth(which) {
  const login = which !== "register";
  $("tabLogin").classList.toggle("on", login);
  $("tabRegister").classList.toggle("on", !login);
  $("loginForm").hidden = !login;
  $("registerForm").hidden = login;
  const first = login ? field($("loginForm"), "handle") : field($("registerForm"), "handle");
  if (first) first.focus();
}

function setSignedIn(on) {
  document.body.classList.toggle("in", on);
  $("authGate").hidden = on;
  $("work").hidden = !on;
}

function openPane(id) {
  if (!account && id === "identity") {
    $("authGate").scrollIntoView({ behavior: "smooth", block: "center" });
    showAuth("login");
    return;
  }
  $("drawer").hidden = false;
  document.querySelectorAll(".pane").forEach((p) => p.classList.toggle("on", p.id === id));
  if (id === "agents") loadAgents();
  if (id === "audit") loadAudit();
  if (id === "identity") loadSession();
  if (id === "browser") browse($("resPath").value || (account ? "/" + account.podPath : "/"));
  $("drawer").scrollIntoView({ behavior: "smooth", block: "start" });
}

function closeDrawer() {
  $("drawer").hidden = true;
  document.querySelectorAll(".pane").forEach((p) => p.classList.remove("on"));
}

async function loadStatus() {
  const health = await fetch(openidURL("/health")).then((r) => r.json()).catch(() => ({ status: "down" }));
  const ok = health.status === "ok";
  $("healthPill").textContent = ok ? "healthy" : "down";
  $("healthPill").classList.toggle("bad", !ok);
  const st = await fetch(openidURL("/api/status")).then((r) => r.json()).catch(() => ({}));
  $("baseLabel").textContent = st.baseUrl || "";
  const rows = [
    ["Product", (st.product || "") + " " + (st.version || "")],
    ["Protocol", st.protocol || "Solid Protocol"],
    ["Base URL", st.baseUrl || ""],
    ["Storage", st.storagePath || "(memory/file)"],
    ["Accounts", String(st.accounts ?? "—")],
    ["Agents", String(st.agents ?? "—")],
  ];
  $("statusList").innerHTML = rows.map(([k, v]) => `<dt>${k}</dt><dd>${v}</dd>`).join("");
  $("disco").innerHTML = Object.entries(st.endpoints || {})
    .map(([k, v]) => `<li><a href="${v}">${k}</a> <span class="mono">${v}</span></li>`)
    .join("");
}

async function loadSession() {
  $("sessionActions").innerHTML = "";
  if (!token) {
    account = null;
    setSignedIn(false);
    $("who").textContent = "Sign in to your pod. One identity for you and the agents you allow.";
    $("webid").textContent = "";
    $("meName").textContent = "Sign in";
    $("meAvatar").textContent = "";
    $("headTitle").textContent = "oid";
    return;
  }
  const res = await fetch(openidURL("/idp/accounts/me"), { headers: headers() });
  if (!res.ok) {
    setToken("");
    return loadSession();
  }
  account = await res.json();
  setSignedIn(true);
  $("who").textContent = (account.name || account.handle) + " · /" + account.podPath;
  $("webid").textContent = account.webId;
  $("meName").textContent = account.name || account.handle;
  $("meAvatar").textContent = "";
  $("headTitle").textContent = account.name || account.handle;
  $("composerHint").textContent = "Notes save to /" + account.podPath + "notes/";
  $("sessionActions").innerHTML = `
    <a class="btn" href="/app">Passport</a>
    <a class="btn" href="/records">Records</a>
    <a class="btn ghost" href="${account.publicUrl || "/i/" + account.handle}">Public page</a>
    <button class="btn ghost" id="logout" type="button">Log out</button>`;
  const logout = $("logout");
  if (logout) {
    logout.addEventListener("click", async () => {
      await fetch(openidURL("/idp/logout"), { method: "POST" });
      setToken("");
      account = null;
      $("stream").innerHTML = "";
      closeDrawer();
      loadSession();
    });
  }
  if (account.podPath && $("resPath") && !$("resPath").dataset.touched) {
    $("resPath").value = "/" + account.podPath;
  }
}

function setMsg(id, text, kind) {
  const el = $(id);
  el.textContent = text || "";
  el.className = "auth-msg" + (kind ? " " + kind : "");
}

$("loginForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  const form = e.currentTarget;
  const id = field(form, "handle").value.trim();
  const btn = $("loginBtn");
  btn.disabled = true;
  setMsg("loginMsg", "Signing in…");
  const payload = id.includes("@")
    ? { email: id, password: field(form, "password").value }
    : { handle: id, password: field(form, "password").value };
  try {
    const res = await fetch(openidURL("/idp/login"), {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
    if (!res.ok) {
      setMsg("loginMsg", "That handle and password did not match.", "bad");
      return;
    }
    const data = await res.json();
    setToken(data.token);
    await loadSession();
    say("bot", "Welcome back. This is your pod.");
    loadStatus();
  } catch (err) {
    setMsg("loginMsg", "Could not reach the pod.", "bad");
  } finally {
    btn.disabled = false;
  }
});

$("registerForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  const form = e.currentTarget;
  const btn = $("registerBtn");
  btn.disabled = true;
  setMsg("registerMsg", "Creating your pod…");
  try {
    const res = await fetch(openidURL("/idp/register"), {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        handle: field(form, "handle").value,
        name: field(form, "name").value,
        password: field(form, "password").value,
        createPod: true,
      }),
    });
    if (!res.ok) {
      setMsg("registerMsg", await res.text() || "Could not create that handle.", "bad");
      return;
    }
    const data = await res.json();
    setToken(data.token);
    await loadSession();
    $("resPath").value = "/" + data.account.podPath;
    say("bot", "Your pod is ready.");
    loadStatus();
  } catch (err) {
    setMsg("registerMsg", "Could not reach the pod.", "bad");
  } finally {
    btn.disabled = false;
  }
});

field($("registerForm"), "handle").addEventListener("input", () => {
  const raw = field($("registerForm"), "handle").value.toLowerCase().replace(/[^a-z0-9-]/g, "");
  clearTimeout(handleTimer);
  if (raw.length < 2) {
    setMsg("registerMsg", "");
    return;
  }
  handleTimer = setTimeout(async () => {
    const res = await fetch(openidURL("/idp/handles/" + encodeURIComponent(raw)));
    const data = await res.json().catch(() => ({}));
    if (data.available) setMsg("registerMsg", raw + " is available", "ok");
    else setMsg("registerMsg", (data.handle || raw) + " is taken", "bad");
  }, 200);
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
      return { href, path: u.pathname };
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

$("closeDrawer").addEventListener("click", closeDrawer);
$("attach").addEventListener("click", () => openPane("browser"));
document.querySelectorAll("[data-open]").forEach((el) => {
  el.addEventListener("click", () => openPane(el.getAttribute("data-open")));
});
document.querySelectorAll("[data-auth]").forEach((el) => {
  el.addEventListener("click", () => showAuth(el.getAttribute("data-auth")));
});
document.querySelectorAll("[data-reveal]").forEach((btn) => {
  btn.addEventListener("click", () => {
    const input = btn.parentElement.querySelector("input");
    const show = input.type === "password";
    input.type = show ? "text" : "password";
    btn.textContent = show ? "Hide" : "Show";
  });
});
$("meBtn").addEventListener("click", () => {
  if (account) openPane("identity");
  else {
    showAuth("login");
    field($("loginForm"), "handle").focus();
  }
});

$("findRecords").addEventListener("submit", (e) => {
  e.preventDefault();
  const q = $("findQ").value.trim();
  location.href = "/records" + (q ? "?q=" + encodeURIComponent(q) : "");
});

$("composer").addEventListener("submit", async (e) => {
  e.preventDefault();
  const text = $("composerInput").value.trim();
  if (!text) return;
  $("composerInput").value = "";
  say("me", text);
  if (!token || !account) {
    showAuth("login");
    return;
  }
  const path = "/" + account.podPath + "notes/" + Date.now() + ".txt";
  const res = await fetch(openidURL(path), {
    method: "PUT",
    headers: { Authorization: "Bearer " + token, "Content-Type": "text/plain" },
    body: text,
  });
  say("bot", res.ok ? "Saved to your notes." : "Could not write (" + res.status + ")");
});

(async function boot() {
  if (window.__TAURI_INTERNALS__) document.body.classList.add("tauri");
  await loadStatus();
  await loadSession();
})();
