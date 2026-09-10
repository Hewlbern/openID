/**
 * Yellow PROMPT chip + copy dock (megabot LibraryPromptDock / AgentPromptDock shape).
 * Signed-out Copy → open sign-in, no mint. Signed-in Copy → POST /api/agent/session.
 */
(function (g) {
  function hasSession() {
    try { return !!(localStorage.getItem("openid.token") || ""); } catch (e) { return false; }
  }

  function mask(text) {
    return String(text || "").replace(/Authorization: Bearer \S+/g, "Authorization: Bearer <TOKEN>");
  }

  function openSignIn() {
    if (typeof g.openidOpenSignIn === "function") {
      g.openidOpenSignIn();
      return;
    }
    if (location.pathname === "/app" || location.pathname === "/app/") {
      location.href = "/?next=/app";
      return;
    }
    const loginBtn = document.getElementById("loginBtn");
    if (loginBtn) loginBtn.click();
    const lead = document.getElementById("gateLead");
    if (lead) lead.textContent = "Sign in to copy an OpenID / agent prompt.";
    const handle = document.querySelector("#loginForm input[name='handle']") || document.getElementById("handle");
    if (handle) handle.focus();
  }

  function mount() {
    if (document.getElementById("openidAgentDock")) return;
    const wrap = document.createElement("div");
    wrap.id = "openidAgentDock";
    wrap.className = "library-dock";
    wrap.innerHTML = `
      <div class="library-dock-panel" id="library-agent-prompt" hidden role="dialog" aria-label="OpenID agent prompt">
        <p class="library-dock-kicker">Agent prompt</p>
        <p class="library-dock-copy">Copy a DingCAD-style brief for Codex, Cursor, Claude, or Grok. Copy mints a short-lived user token for MCP and conversation APIs. Keep it private.</p>
        <p class="library-dock-expiry" id="agentDockExpiry" hidden></p>
        <p class="library-dock-banner" id="agentDockBanner" hidden></p>
        <pre class="library-dock-preview" id="agentDockPreview">Loading…</pre>
        <div class="library-dock-actions">
          <button type="button" class="library-dock-copy-btn" id="agentDockCopy">Sign in to copy</button>
          <button type="button" class="library-dock-collapse" id="agentDockCollapse">Collapse</button>
        </div>
      </div>
      <button type="button" class="library-dock-chip" id="agentDockChip" aria-expanded="false" aria-controls="library-agent-prompt">
        <span class="library-dock-chip-kicker">PROMPT</span>
        <span id="agentDockChipLabel">openid prompt</span>
      </button>
    `;
    document.body.appendChild(wrap);

    const panel = document.getElementById("library-agent-prompt");
    const chip = document.getElementById("agentDockChip");
    const chipLabel = document.getElementById("agentDockChipLabel");
    const preview = document.getElementById("agentDockPreview");
    const copyBtn = document.getElementById("agentDockCopy");
    const collapse = document.getElementById("agentDockCollapse");
    const expiry = document.getElementById("agentDockExpiry");
    const banner = document.getElementById("agentDockBanner");
    let open = false;
    let copied = false;
    let minting = false;

    function setBanner(text, warn) {
      if (!banner) return;
      banner.hidden = !text;
      banner.textContent = text || "";
      banner.className = "library-dock-banner" + (warn ? " warn" : "");
    }

    function setOpen(on) {
      open = !!on;
      wrap.classList.toggle("is-open", open);
      if (panel) panel.hidden = !open;
      if (chip) chip.setAttribute("aria-expanded", open ? "true" : "false");
      if (chipLabel) chipLabel.textContent = open ? (copied ? "Copied" : "Hide brief") : "openid prompt";
      if (copyBtn) {
        copyBtn.disabled = minting;
        copyBtn.textContent = minting ? "Minting…" : (copied ? "Copied" : (hasSession() ? "Copy prompt" : "Sign in to copy"));
      }
    }

    async function loadPreview() {
      try {
        const res = await fetch("/api/agent/prompt", { headers: { Accept: "application/json" } });
        const doc = await res.json();
        if (preview && doc.prompt) preview.textContent = mask(doc.prompt);
      } catch (e) {
        if (preview) preview.textContent = "Could not load /api/agent/prompt.";
      }
    }

    async function mintAndCopy() {
      if (!hasSession()) {
        setBanner("Sign in to copy an OpenID prompt token.", true);
        openSignIn();
        return "";
      }
      minting = true;
      setBanner(null);
      setOpen(true);
      try {
        const fetchFn = g.openidFetch || fetch;
        const res = await fetchFn("/api/agent/session", {
          method: "POST",
          headers: { "Content-Type": "application/json", Accept: "application/json" },
          body: "{}",
        });
        const text = await res.text();
        let doc = {};
        try { doc = JSON.parse(text); } catch (e) { doc = { error: text }; }
        if (res.status === 401) {
          setBanner("Session expired. Sign in and copy a fresh prompt.", true);
          openSignIn();
          return "";
        }
        if (!res.ok || !doc.prompt) {
          setBanner(doc.error || doc.message || "Could not mint a prompt token. Sign in and try again.", true);
          return "";
        }
        if (preview) preview.textContent = mask(doc.prompt);
        try { await navigator.clipboard.writeText(doc.prompt); } catch (e) { /* ignore */ }
        copied = true;
        if (expiry && doc.expiresAt) {
          expiry.hidden = false;
          expiry.textContent = "Last token expires " + new Date(doc.expiresAt).toLocaleTimeString();
        } else if (expiry) {
          expiry.hidden = doc.tokenKind !== "access_token";
          if (doc.tokenKind === "access_token") {
            expiry.hidden = false;
            expiry.textContent = "Using your signed-in session token (no dedicated agent JWT secret).";
          }
        }
        setOpen(true);
        window.setTimeout(() => { copied = false; setOpen(open); }, 2200);
        return doc.prompt;
      } catch (e) {
        setBanner("Could not copy the OpenID prompt.", true);
        return "";
      } finally {
        minting = false;
        setOpen(open);
      }
    }

    if (chip) chip.addEventListener("click", () => setOpen(!open));
    if (copyBtn) copyBtn.addEventListener("click", () => mintAndCopy());
    if (collapse) collapse.addEventListener("click", () => setOpen(false));
    g.openidMintAgentPrompt = mintAndCopy;
    setOpen(false);
    loadPreview();
  }

  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", mount);
  else mount();
})(window);
