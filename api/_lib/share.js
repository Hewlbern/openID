const crypto = require("crypto");
const { POD, podFetch, ensureContainer, accountMe, publicReadACL } = require("./pod");

async function loadConvo(token, handle, id) {
  const path = "/" + handle + "/conversations/spark/" + id + ".json";
  const got = await podFetch(path, { token, headers: { Accept: "application/ld+json, application/json" } });
  if (got.status >= 400) throw new Error("conversation not found");
  return { path, doc: JSON.parse(got.text) };
}

async function shareConversation(token, id, origin) {
  const acc = await accountMe(token);
  const handle = acc.handle;
  const { path, doc } = await loadConvo(token, handle, id);
  let random;
  let shareToken;
  if (doc.share && doc.share.token && String(doc.share.token).startsWith(handle + ".")) {
    shareToken = doc.share.token;
    random = shareToken.slice(handle.length + 1);
  } else {
    random = crypto.randomBytes(16).toString("hex");
    shareToken = handle + "." + random;
  }
  const publicPath = "/" + handle + "/public/shares/" + random + ".json";
  const shareUrl = String(origin || "").replace(/\/$/, "") + "/share/c/" + shareToken;

  await ensureContainer(token, "/" + handle + "/public/");
  await ensureContainer(token, "/" + handle + "/public/shares/");

  const snapshot = Object.assign({}, doc, {
    share: { token: shareToken, url: shareUrl, public: false },
  });
  const put = await podFetch(publicPath, {
    method: "PUT",
    token,
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(snapshot, null, 2),
  });
  if (put.status >= 400) throw new Error("public share put " + put.status + " " + put.text);

  // Do not write a custom .acl here: on the current Railway Solid build a bad ACL
  // locks the file (403). New resources under public/ are world-readable by default
  // (verified with unauthenticated GET). Revoke deletes the snapshot.

  await podFetch("/" + handle + "/conversations/spark/" + id + ".share.json", {
    method: "PUT",
    token,
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      token: shareToken,
      convoId: id,
      resource: handle + "/conversations/spark/" + id + ".json",
      publicPath: handle + "/public/shares/" + random + ".json",
      created: new Date().toISOString(),
    }, null, 2),
  });

  doc.share = { token: shareToken, url: shareUrl, public: false };
  const save = await podFetch(path, {
    method: "PUT",
    token,
    headers: { "Content-Type": "application/ld+json" },
    body: JSON.stringify(doc, null, 2),
  });
  if (save.status >= 400) throw new Error("update conversation " + save.status + " " + save.text);
  return doc;
}

async function unshareConversation(token, id) {
  const acc = await accountMe(token);
  const handle = acc.handle;
  const { path, doc } = await loadConvo(token, handle, id);
  const shareToken = doc.share && doc.share.token;
  if (shareToken && String(shareToken).startsWith(handle + ".")) {
    const random = shareToken.slice(handle.length + 1);
    const publicPath = "/" + handle + "/public/shares/" + random + ".json";
    await podFetch(publicPath, { method: "DELETE", token });
    await podFetch(publicPath + ".acl", { method: "DELETE", token }); // best-effort cleanup
  }
  await podFetch("/" + handle + "/conversations/spark/" + id + ".share.json", { method: "DELETE", token });
  delete doc.share;
  await podFetch(path, {
    method: "PUT",
    token,
    headers: { "Content-Type": "application/ld+json" },
    body: JSON.stringify(doc, null, 2),
  });
  return { ok: true, id };
}

module.exports = { shareConversation, unshareConversation };
