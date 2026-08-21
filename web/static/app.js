const token = localStorage.getItem("openid.token");
if (!token) location.replace("/dashboard");

const headers = {
  Authorization: "Bearer " + token,
  "Content-Type": "application/json",
};

async function me() {
  const res = await fetch(openidURL("/idp/accounts/me"), { headers });
  if (!res.ok) {
    localStorage.removeItem("openid.token");
    location.replace("/dashboard");
    return null;
  }
  return res.json();
}

function row(label, href) {
  const a = document.createElement("a");
  a.className = "row";
  a.href = href;
  a.textContent = label;
  return a;
}

async function load() {
  const acc = await me();
  if (!acc) return;
  document.getElementById("hello").textContent = acc.name || acc.handle;
  document.getElementById("who").textContent = acc.webId;
  const pub = document.getElementById("publicLink");
  pub.href = acc.publicUrl || "/i/" + acc.handle;
  pub.textContent = (acc.publicUrl || "/i/" + acc.handle) + " →";
  document.querySelector("#profileForm [name=name]").value = acc.name || "";
  document.querySelector("#profileForm [name=bio]").value = acc.bio || "";

  const list = document.getElementById("podList");
  list.innerHTML = "";
  const podRes = await fetch(openidURL("/" + acc.podPath), { headers: { Authorization: "Bearer " + token, Accept: "text/turtle" } });
  if (podRes.ok) {
    list.appendChild(row("Open pod container", openidURL("/" + acc.podPath)));
    list.appendChild(row("WebID card", openidURL("/" + acc.podPath + "profile/card")));
  }

  const agents = await fetch(openidURL("/agents"), { headers }).then((r) => r.json()).catch(() => []);
  const agentList = document.getElementById("agentList");
  agentList.innerHTML = "";
  (agents || []).forEach((a) => agentList.appendChild(row(a.name + " · " + a.webId, openidURL("/" + a.podPath + "profile/card"))));

  const events = await fetch(openidURL("/audit/events/"), { headers }).then((r) => r.json()).catch(() => []);
  const auditList = document.getElementById("auditList");
  auditList.innerHTML = "";
  (events || []).slice().reverse().slice(0, 12).forEach((e) => {
    auditList.appendChild(row((e.method || "") + " " + e.resource, openidURL("/audit/events/" + e.id + "/verify")));
  });
}

document.getElementById("profileForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  await fetch(openidURL("/idp/profile"), {
    method: "PATCH",
    headers,
    body: JSON.stringify({
      name: e.target.name.value,
      bio: e.target.bio.value,
    }),
  });
  load();
});

document.getElementById("putForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  const acc = await me();
  const path = e.target.path.value.replace(/^\/+/, "");
  const res = await fetch(openidURL("/" + acc.podPath + path), {
    method: "PUT",
    headers: {
      Authorization: "Bearer " + token,
      "Content-Type": "text/plain",
    },
    body: e.target.body.value,
  });
  alert(res.ok ? "Saved" : "Could not save (" + res.status + ")");
  load();
});

document.getElementById("agentForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  const res = await fetch(openidURL("/agents"), {
    method: "POST",
    headers,
    body: JSON.stringify({ name: e.target.name.value || "agent" }),
  });
  const data = await res.json();
  document.getElementById("agentSecret").textContent =
    "token: " + data.token + "\nprivateKey: " + (data.privateKey || "") + "\nwebId: " + data.agent.webId;
  load();
});

document.getElementById("flush").addEventListener("click", async () => {
  await fetch(openidURL("/audit/flush"), { method: "POST", headers });
  load();
});

document.getElementById("logout").addEventListener("click", async () => {
  await fetch(openidURL("/idp/logout"), { method: "POST" });
  localStorage.removeItem("openid.token");
  location.replace("/dashboard");
});

load();
