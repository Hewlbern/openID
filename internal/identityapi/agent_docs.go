package identityapi

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"solid-go/internal/authn"
)

func publicAppOrigin(r *http.Request, fallback string) string {
	if v := strings.TrimRight(os.Getenv("OPENID_PUBLIC_URL"), "/"); v != "" {
		return v
	}
	if v := strings.TrimRight(os.Getenv("OPENID_APP_URL"), "/"); v != "" {
		return v
	}
	if r != nil {
		if proto, host := r.Header.Get("X-Forwarded-Proto"), r.Header.Get("X-Forwarded-Host"); proto != "" && host != "" {
			return proto + "://" + host
		}
		if o := strings.TrimRight(r.Header.Get("Origin"), "/"); o != "" {
			return o
		}
	}
	return strings.TrimRight(fallback, "/")
}

func buildAgentPrompt(origin, token string) string {
	base := strings.TrimRight(origin, "/")
	if base == "" {
		base = "https://askclaw.xyz"
	}
	return strings.Join([]string{
		"Help me save conversation traces to my OpenID Solid pod. If it is unclear what I want, ask me before you write or share.",
		"",
		"OpenID is Solid Protocol identity for AI agents — a WebID, a personal pod, and MCP so Codex / Cursor / Claude / Grok / Gemini Spark can write threads into conversations/spark/.",
		"App URL: " + base + "/",
		"",
		"## Protocol",
		"GET " + base + "/llms.txt",
		"GET " + base + "/api/agent/prompt (current instructions, no token minted)",
		"POST " + base + "/api/agent/session (signed-in browser mint only — do not call this yourself)",
		"",
		"## Auth",
		"Send this header on user-scoped MCP and conversation APIs:",
		"Authorization: Bearer " + token,
		"Keep this token private (short-lived, aud=spark-mcp). On HTTP 401, ask me to copy a new OpenID / agent prompt.",
		"Never invent credentials, WebIDs, client secrets, or tokens.",
		"",
		"## Read",
		"- Human passport: " + base + "/app",
		"- GET " + base + "/api/spark-conversations — list {handle}/conversations/spark/",
		"- Remote MCP: " + base + "/mcp (alias " + base + "/api/mcp)",
		"- MCP tools (only these): spark_save_conversation, spark_list_conversations, spark_get_conversation, spark_share_conversation, spark_unshare_conversation",
		"",
		"## Write",
		"- Prefer MCP spark_save_conversation with title and messages: [{role, content|text, timestamp?}]. Optional source (codex|cursor|claude|grok|gemini-spark).",
		"- Or POST " + base + "/api/spark-conversations { title?, messages?, text?, source?, source_url? }",
		"Sequential single-op commits: save one conversation, wait for resourceUrl, then list or share. Do not invent ids or overwrite another handle's pod.",
		"Writes land at {handle}/conversations/spark/{id}.json (JSON-LD) plus {id}.ttl.",
		"",
		"## Agent recipe",
		"1. Confirm I want this thread stored on OpenID.",
		"2. Call spark_save_conversation with the full transcript. Set source to the tool I am in (codex, cursor, claude, grok, gemini-spark).",
		"3. Read resourceUrl / confirmation. Optionally share if I ask.",
		"4. Tell me what you saved and send " + base + "/app",
		"On 401, stop and ask me to open /app and copy a fresh Open / agent prompt.",
		"",
		"When you are done, tell me what you saved and send:",
		base + "/app",
	}, "\n")
}

func buildLlmsTxt(origin string) string {
	base := strings.TrimRight(origin, "/")
	if base == "" {
		base = "https://askclaw.xyz"
	}
	return `# OpenID — Solid identity for AI agents

> OpenID gives each person and agent a WebID, a Solid pod, and a tamper-evident audit trail. The hosted passport lets you mint short-lived MCP tokens and save Codex / Cursor / Claude / Grok / Gemini Spark traces into ` + "`{handle}/conversations/spark/`" + `.

This is not a robot workshop. Do not invent credentials, WebIDs, or tokens.

## Base URL
` + base + `

## Remote MCP

- ` + base + `/mcp
- Alias: ` + base + `/api/mcp

Tools: spark_save_conversation, spark_list_conversations, spark_get_conversation, spark_share_conversation, spark_unshare_conversation.

Auth: Authorization: Bearer from POST /api/agent/session (copied prompt) or POST /api/spark-token.

## Agent prompt APIs

- GET /api/agent/prompt — copyable instructions, no token minted. CORS *.
- POST /api/agent/session — signed-in browser only. Returns { prompt, token, expiresAt, tokenKind }. Short-lived spark-mcp JWT. Remint on each copy.
- GET /llms.txt — this document.

## Conversation APIs

- GET/POST /api/spark-conversations
- POST /api/spark-conversations/{id}/share
- GET /share/c/{token}

On 401, ask the owner to copy a new OpenID / agent prompt.

## Identity

- POST /idp/register — handle + password
- POST /idp/login — handle or email + password
- GET /api/auth/google — Continue with Google (hosted passport)
- POST /idp/google — Google ID token → session

## Agent recipe

1. Use the Bearer from the prompt the human copied.
2. spark_save_conversation with the full transcript and source.
3. Sequential single-op commits. Send ` + base + `/app when done.
`
}

func agentCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept")
}

func (s *Service) handleLLMs(w http.ResponseWriter, r *http.Request) {
	agentCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(buildLlmsTxt(publicAppOrigin(r, s.BaseURL))))
}

func (s *Service) handleAgentPrompt(w http.ResponseWriter, r *http.Request) {
	agentCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	origin := publicAppOrigin(r, s.BaseURL)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"prompt": buildAgentPrompt(origin, ""),
		"docs":   origin + "/llms.txt",
		"mint":   "POST /api/agent/session (signed-in browser only)",
		"auth":   "Authorization: Bearer on /mcp and /api/spark-conversations",
	})
}

func (s *Service) handleAgentSession(w http.ResponseWriter, r *http.Request) {
	agentCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	acc := s.requireFullAccount(r)
	if acc == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	ttl := 2 * time.Hour
	if raw := strings.TrimSpace(os.Getenv("OPENID_AGENT_TOKEN_TTL")); raw != "" {
		if n, err := time.ParseDuration(raw + "s"); err == nil && n >= time.Minute && n <= 30*24*time.Hour {
			ttl = n
		} else if d, err := time.ParseDuration(raw); err == nil && d >= time.Minute && d <= 30*24*time.Hour {
			ttl = d
		}
	}
	tok, jti, exp, err := s.Tokens.IssueSpark(acc.WebID, ttl)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.mu.Lock()
	s.spark[jti] = &sparkGrant{
		JTI: jti, WebID: acc.WebID, Issued: time.Now().UTC(), Expires: exp,
	}
	s.saveSparkGrantsLocked()
	s.mu.Unlock()
	origin := publicAppOrigin(r, s.BaseURL)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"prompt":    buildAgentPrompt(origin, tok),
		"token":     tok,
		"expiresAt": exp.Format(time.RFC3339),
		"tokenKind": "spark-mcp",
		"tokenType": "Bearer",
		"jti":       jti,
		"aud":       authn.AudienceSparkMCP,
		"scope":     authn.ScopeSpark,
		"webId":     acc.WebID,
		"mcpUrl":    origin + "/mcp",
		"ttl":       ttl.String(),
	})
}
