const apiHost = (() => {
  try {
    const api = openidURL("/");
    if (api && /^https?:\/\//i.test(api)) return new URL(api).host;
  } catch (e) {}
  return location.hostname === "localhost" || location.hostname === "127.0.0.1" ? "openid" : location.host;
})();
const hostPrefix = apiHost + "/i/";
document.querySelectorAll("[data-prefix]").forEach((el) => { el.textContent = hostPrefix; });
document.querySelectorAll(".host-url").forEach((el) => { el.textContent = hostPrefix + "you"; });

function drawOrb() {
  const canvas = document.getElementById("orb");
  if (!canvas) return;
  const ctx = canvas.getContext("2d");
  const w = canvas.width, h = canvas.height;
  ctx.clearRect(0, 0, w, h);
  const cx = w * 0.5, cy = h * 0.48, r = Math.min(w, h) * 0.36;
  for (let i = 0; i < 1600; i++) {
    const u = Math.random() * Math.PI * 2;
    const v = Math.acos(2 * Math.random() - 1);
    const x = Math.sin(v) * Math.cos(u);
    const y = Math.cos(v);
    const z = Math.sin(v) * Math.sin(u);
    const shade = 0.45 + z * 0.55;
    ctx.beginPath();
    ctx.fillStyle = `rgba(255,255,255,${shade})`;
    ctx.arc(cx + x * r, cy + y * r * 0.92, 1.05 + shade * 1.7, 0, Math.PI * 2);
    ctx.fill();
  }
}

const claimModal = document.getElementById("claimModal");
const loginModal = document.getElementById("loginModal");
const registerForm = document.getElementById("registerForm");
const loginForm = document.getElementById("loginForm");
const modalHandle = document.getElementById("modalHandle");
let availableHandle = "";

async function checkHandle(value, statusEl, btn) {
  const handle = value.toLowerCase().replace(/[^a-z0-9-]/g, "");
  if (handle.length < 2) {
    availableHandle = "";
    btn.disabled = true;
    statusEl.textContent = "";
    return;
  }
  const res = await fetch(openidURL("/idp/handles/" + encodeURIComponent(handle)));
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
    const name = document.querySelector(".pass-name");
    if (name) name.textContent = (input.value || "YOUR NAME").toUpperCase();
    clearTimeout(t);
    t = setTimeout(() => checkHandle(input.value, status, btn), 180);
  });
  form.addEventListener("submit", (e) => {
    e.preventDefault();
    if (!availableHandle) return;
    modalHandle.textContent = availableHandle;
    registerForm.name.value = availableHandle;
    claimModal.classList.add("open");
  });
}

bindClaim("claimForm", "handle", "claimBtn", "claimStatus");
bindClaim("claimForm2", "handle2", "claimBtn2", "claimStatus2");

document.getElementById("loginBtn").addEventListener("click", () => loginModal.classList.add("open"));
const passportCta = document.getElementById("passportCta");
if (passportCta) passportCta.addEventListener("click", () => loginModal.classList.add("open"));
[claimModal, loginModal].forEach((el) => el.addEventListener("click", (e) => {
  if (e.target === el) el.classList.remove("open");
}));

registerForm.addEventListener("submit", async (e) => {
  e.preventDefault();
  const body = {
    handle: availableHandle,
    name: registerForm.name.value,
    password: registerForm.password.value,
    email: registerForm.email.value,
    createPod: true,
  };
  const res = await fetch(openidURL("/idp/register"), {
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
  const id = loginForm.id.value.trim();
  const payload = id.includes("@")
    ? { email: id, password: loginForm.password.value }
    : { handle: id, password: loginForm.password.value };
  const res = await fetch(openidURL("/idp/login"), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  if (!res.ok) {
    alert("Could not log in");
    return;
  }
  const data = await res.json();
  localStorage.setItem("openid.token", data.token);
  location.href = "/app";
});

drawOrb();
if (localStorage.getItem("openid.token")) {
  fetch(openidURL("/idp/accounts/me"), { headers: { Authorization: "Bearer " + localStorage.getItem("openid.token") } })
    .then((r) => { if (r.ok) location.replace("/app"); });
}
