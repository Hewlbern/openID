const apiHost = (() => {
  try {
    const api = openidURL("/");
    if (api && /^https?:\/\//i.test(api)) return new URL(api).host;
  } catch (e) {}
  return location.hostname === "localhost" || location.hostname === "127.0.0.1" ? "openid" : location.host;
})();
const hostPrefix = apiHost + "/i/";
document.querySelectorAll("[data-prefix]").forEach((el) => { el.textContent = hostPrefix; });

const claimModal = document.getElementById("claimModal");
const registerForm = document.getElementById("registerForm");
const loginForm = document.getElementById("loginForm");
const claimForm = document.getElementById("claimForm");
const modalHandle = document.getElementById("modalHandle");
const loginBtn = document.getElementById("loginBtn");
const signupBtn = document.getElementById("signupBtn");
let availableHandle = "";

function field(form, name) {
  return form && form.elements ? form.elements.namedItem(name) : null;
}

function showLogin(on) {
  if (claimForm) claimForm.hidden = on;
  if (loginForm) loginForm.hidden = !on;
  if (loginBtn) loginBtn.hidden = on;
  if (signupBtn) signupBtn.hidden = !on;
  const lead = document.getElementById("gateLead");
  if (lead) lead.textContent = on ? "Sign in. Then take Spark into your Solid pod." : "Claim a handle. Then take Spark into your Solid pod.";
  const first = on ? field(loginForm, "handle") : document.getElementById("handle");
  if (first) first.focus();
}

async function checkHandle(value, statusEl, btn) {
  const handle = value.toLowerCase().replace(/[^a-z0-9-]/g, "");
  if (handle.length < 2) {
    availableHandle = "";
    btn.disabled = true;
    statusEl.textContent = "";
    return;
  }
  const res = await openidFetch("/idp/handles/" + encodeURIComponent(handle));
  const data = await res.json();
  if (data.available) {
    availableHandle = data.handle;
    btn.disabled = false;
    statusEl.className = "status ok";
    statusEl.textContent = data.publicUrl + " is available";
  } else {
    availableHandle = "";
    btn.disabled = true;
    statusEl.className = "status bad";
    statusEl.textContent = data.handle ? data.handle + " is taken" : "Choose a longer handle";
  }
}

function bindClaim(formId, inputId, btnId, statusId) {
  const form = document.getElementById(formId);
  const input = document.getElementById(inputId);
  const btn = document.getElementById(btnId);
  const status = document.getElementById(statusId);
  if (!form) return;
  let t;
  input.addEventListener("input", () => {
    clearTimeout(t);
    t = setTimeout(() => checkHandle(input.value, status, btn), 180);
  });
  form.addEventListener("submit", (e) => {
    e.preventDefault();
    if (!availableHandle) return;
    modalHandle.textContent = availableHandle;
    field(registerForm, "name").value = availableHandle;
    claimModal.classList.add("open");
  });
}

bindClaim("claimForm", "handle", "claimBtn", "claimStatus");

if (loginBtn) loginBtn.addEventListener("click", () => showLogin(true));
if (signupBtn) signupBtn.addEventListener("click", () => showLogin(false));
if (claimModal) {
  claimModal.addEventListener("click", (e) => {
    if (e.target === claimModal) claimModal.classList.remove("open");
  });
}

registerForm.addEventListener("submit", async (e) => {
  e.preventDefault();
  const body = {
    handle: availableHandle,
    name: field(registerForm, "name").value,
    password: field(registerForm, "password").value,
    email: field(registerForm, "email").value,
    createPod: true,
  };
  const res = await openidFetch("/idp/register", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    document.getElementById("claimStatus").className = "status bad";
    document.getElementById("claimStatus").textContent = await res.text();
    return;
  }
  const data = await res.json();
  localStorage.setItem("openid.token", data.token);
  location.href = "/app";
});

loginForm.addEventListener("submit", async (e) => {
  e.preventDefault();
  const handle = (field(loginForm, "handle").value || "").trim();
  const password = field(loginForm, "password").value;
  const payload = handle.includes("@")
    ? { email: handle, password }
    : { handle, password };
  const res = await openidFetch("/idp/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  if (!res.ok) {
    const status = document.getElementById("claimStatus");
    status.className = "status bad";
    status.textContent = "Could not sign in.";
    return;
  }
  const data = await res.json();
  localStorage.setItem("openid.token", data.token);
  if (handle && !handle.includes("@")) localStorage.setItem("openid.handle", handle);
  location.href = "/app";
});

const apiStatus = document.getElementById("apiStatus");
if (apiStatus) {
  openidFetch("/health").then((r) => {
    apiStatus.textContent = r.ok ? (apiHost.includes("railway") ? "Railway" : "Pod") : "No pod";
  }).catch(() => { apiStatus.textContent = "No pod"; });
}

if (localStorage.getItem("openid.token")) {
  openidFetch("/idp/accounts/me")
    .then((r) => { if (r.ok) location.replace("/app"); });
}
