function $(id) { return document.getElementById(id); }

function readToken() {
  const query = new URLSearchParams(location.search);
  const fromQuery = query.get("token") || "";
  if (fromQuery) {
    const next = location.pathname + "#token=" + encodeURIComponent(fromQuery);
    history.replaceState(null, "", next);
  }
  const hash = location.hash.startsWith("#") ? location.hash.slice(1) : location.hash;
  return new URLSearchParams(hash).get("token") || fromQuery || "";
}

const token = readToken();
const msg = $("resetMsg");

if (!token) {
  msg.className = "auth-msg bad";
  msg.textContent = "This reset link is missing its token. Ask for a new one.";
  $("resetBtn").disabled = true;
}

document.querySelectorAll("[data-reveal]").forEach((btn) => {
  btn.addEventListener("click", () => {
    const input = btn.parentElement.querySelector("input");
    const show = input.type === "password";
    input.type = show ? "text" : "password";
    btn.textContent = show ? "Hide" : "Show";
  });
});

$("resetForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  const fd = new FormData(e.target);
  const password = String(fd.get("password") || "");
  const confirm = String(fd.get("confirm") || "");
  msg.className = "auth-msg";
  if (password !== confirm) {
    msg.className = "auth-msg bad";
    msg.textContent = "Those passwords do not match.";
    return;
  }
  msg.textContent = "Saving…";
  $("resetBtn").disabled = true;
  try {
    const res = await openidFetch("/idp/password/reset", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ token, password }),
    });
    if (!res.ok) {
      msg.className = "auth-msg bad";
      msg.textContent = res.status === 429 ? "Too many attempts. Try again later." : "This reset link is invalid or expired.";
      $("resetBtn").disabled = false;
      return;
    }
    history.replaceState(null, "", location.pathname);
    e.target.hidden = true;
    msg.className = "auth-msg ok";
    msg.textContent = "Password updated. Sign in with the new one.";
    $("done").hidden = false;
  } catch (err) {
    msg.className = "auth-msg bad";
    msg.textContent = "Could not reach the pod.";
    $("resetBtn").disabled = false;
  }
});
