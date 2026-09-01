/**
 * Fetch a public Gemini share page. No Google cookies, passwords, or session.
 * If the link is behind a login wall, say so and ask the user to paste the transcript.
 */
function cors(res) {
  res.setHeader("Access-Control-Allow-Origin", "*");
  res.setHeader("Access-Control-Allow-Methods", "GET, POST, OPTIONS");
  res.setHeader("Access-Control-Allow-Headers", "Content-Type, Authorization");
}

function isGeminiShareURL(raw) {
  const s = String(raw || "").trim();
  if (!s) return false;
  if (s.includes("g.co/gemini/share")) return true;
  try {
    const u = new URL(s.includes("://") ? s : "https://" + s);
    const host = u.hostname.toLowerCase();
    const path = u.pathname.toLowerCase();
    if (host === "g.co" && path.includes("/gemini/share")) return true;
    if ((host === "gemini.google.com" || host.endsWith(".gemini.google.com")) && path.includes("/share")) return true;
  } catch (e) {
    return false;
  }
  return false;
}

function firstMatch(re, html) {
  const m = String(html || "").match(re);
  return m && m[1] ? m[1].trim() : "";
}

function unescapeHtml(s) {
  return String(s || "")
    .replace(/&amp;/g, "&")
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .replace(/&quot;/g, '"')
    .replace(/&#39;/g, "'")
    .replace(/&nbsp;/g, " ");
}

const NOT_PUBLIC = "This Gemini share link is not publicly readable. Paste the transcript instead. We do not sign into Google.";

module.exports = async function handler(req, res) {
  cors(res);
  if (req.method === "OPTIONS") {
    res.status(204).end();
    return;
  }
  if (req.method !== "POST") {
    res.status(405).json({ error: "POST a public Gemini share URL" });
    return;
  }
  const body = typeof req.body === "string" ? JSON.parse(req.body || "{}") : (req.body || {});
  const raw = String(body.url || body.source_url || "").trim();
  if (!isGeminiShareURL(raw)) {
    res.status(400).json({ error: "Expected a public Gemini share URL (g.co/gemini/share/… or gemini.google.com/share/…)." });
    return;
  }
  const url = raw.includes("://") ? raw : "https://" + raw;
  let resp;
  try {
    resp = await fetch(url, {
      redirect: "follow",
      headers: {
        Accept: "text/html,application/json;q=0.9,*/*;q=0.8",
        "User-Agent": "OpenID-Solid/1.0 (public share import only; no Google session)",
      },
    });
  } catch (e) {
    res.status(422).json({ error: "Could not fetch that share URL. Paste the transcript instead." });
    return;
  }
  const finalHost = resp.url ? String(new URL(resp.url).hostname).toLowerCase() : "";
  if (finalHost.includes("accounts.google.") || resp.status === 401 || resp.status === 403) {
    res.status(422).json({ error: NOT_PUBLIC });
    return;
  }
  if (resp.status < 200 || resp.status >= 300) {
    res.status(422).json({ error: "Share URL returned HTTP " + resp.status + ". " + NOT_PUBLIC });
    return;
  }
  const text = await resp.text();
  const ct = resp.headers.get("content-type") || "";
  if (ct.includes("json")) {
    try {
      const doc = JSON.parse(text);
      const messages = Array.isArray(doc.messages) ? doc.messages : Array.isArray(doc) ? doc : [];
      if (messages.length) {
        res.status(200).json({
          title: doc.title || doc.name || "Gemini share",
          source_url: url,
          messages,
        });
        return;
      }
    } catch (e) {
      /* fall through to HTML */
    }
  }
  let title = unescapeHtml(firstMatch(/property=["']og:title["'][^>]+content=["']([^"']+)["']/i, text)
    || firstMatch(/content=["']([^"']+)["'][^>]+property=["']og:title["']/i, text)
    || firstMatch(/<title[^>]*>([\s\S]*?)<\/title>/i, text));
  title = title.split(" - ")[0].trim();
  let desc = unescapeHtml(firstMatch(/property=["']og:description["'][^>]+content=["']([^"']+)["']/i, text)
    || firstMatch(/content=["']([^"']+)["'][^>]+property=["']og:description["']/i, text)
    || firstMatch(/name=["']description["'][^>]+content=["']([^"']+)["']/i, text));
  if (!title || /^google$/i.test(title) || /^gemini$/i.test(title)) {
    if (!desc) {
      res.status(422).json({ error: NOT_PUBLIC });
      return;
    }
    title = desc.slice(0, 80);
  }
  if (!desc) {
    res.status(422).json({ error: NOT_PUBLIC });
    return;
  }
  res.status(200).json({
    title,
    source_url: url,
    messages: [{ role: "assistant", text: desc }],
  });
};
