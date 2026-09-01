package openidmcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"solid-go/internal/authn"
)

// Tool is an MCP tool descriptor.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

func (s *Server) denySparkLocked(name, token string) error {
	if s.Tokens == nil || strings.TrimSpace(token) == "" {
		return nil
	}
	creds, err := s.Tokens.Parse(token)
	if err != nil {
		if errors.Is(err, authn.ErrRevoked) {
			return fmt.Errorf("spark connect token revoked")
		}
		return nil
	}
	if creds.IsSpark() && !strings.HasPrefix(name, "spark_") {
		return fmt.Errorf("spark connect token cannot call %s", name)
	}
	return nil
}

func objectSchema(props map[string]any, required ...string) map[string]any {
	s := map[string]any{
		"type":       "object",
		"properties": props,
	}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func strProp(desc string) map[string]any {
	return map[string]any{"type": "string", "description": desc}
}

// Tools is the full OpenID tool catalog grokbot can call.
func Tools() []Tool {
	token := strProp("Bearer token from login, register, or register_agent")
	path := strProp("Pod-relative path, e.g. mike/public/note.json")
	base := []Tool{
		{Name: "openid_open", Description: "Open the local OpenID system: dashboard URL, MCP endpoint, health, and how grokbot should use it.", InputSchema: objectSchema(nil)},
		{Name: "openid_status", Description: "Health and /api/status of the OpenID Solid server.", InputSchema: objectSchema(nil)},
		{Name: "openid_discover", Description: "OIDC and Solid well-known discovery documents.", InputSchema: objectSchema(nil)},
		{Name: "openid_handle_available", Description: "Check whether a public handle can be claimed.", InputSchema: objectSchema(map[string]any{"handle": strProp("Handle to check")}, "handle")},
		{Name: "openid_register", Description: "Register a human account, WebID, and pod.", InputSchema: objectSchema(map[string]any{
			"handle":   strProp("Public handle"),
			"password": strProp("Password"),
			"name":     strProp("Display name"),
			"email":    strProp("Optional email"),
			"bio":      strProp("Optional bio"),
		}, "handle", "password")},
		{Name: "openid_login", Description: "Log in with handle or email and receive a Bearer token.", InputSchema: objectSchema(map[string]any{
			"handle":   strProp("Handle"),
			"email":    strProp("Email"),
			"password": strProp("Password"),
		}, "password")},
		{Name: "openid_me", Description: "Return the account for a Bearer token.", InputSchema: objectSchema(map[string]any{"token": token}, "token")},
		{Name: "openid_register_agent", Description: "Register an AI agent WebID, pod, Ed25519 key, and Bearer token.", InputSchema: objectSchema(map[string]any{"name": strProp("Agent name")})},
		{Name: "openid_list_agents", Description: "List registered AI agents.", InputSchema: objectSchema(nil)},
		{Name: "openid_public_profile", Description: "Read a public handle page (/i/{handle}).", InputSchema: objectSchema(map[string]any{"handle": strProp("Handle")}, "handle")},
		{Name: "openid_pod_get", Description: "GET a Solid resource.", InputSchema: objectSchema(map[string]any{"path": path, "token": token, "accept": strProp("Optional Accept header")}, "path")},
		{Name: "openid_pod_put", Description: "PUT a Solid resource (create or replace).", InputSchema: objectSchema(map[string]any{
			"path":         path,
			"body":         strProp("Resource body"),
			"content_type": strProp("Content-Type, default text/plain"),
			"token":        token,
		}, "path", "body")},
		{Name: "openid_pod_delete", Description: "DELETE a Solid resource.", InputSchema: objectSchema(map[string]any{"path": path, "token": token}, "path")},
		{Name: "openid_pod_list", Description: "List an LDP container (Turtle ldp:contains).", InputSchema: objectSchema(map[string]any{"path": strProp("Container path ending with /"), "token": token}, "path")},
		{Name: "openid_client_credentials", Description: "Mint OAuth client id/secret for the logged-in account.", InputSchema: objectSchema(map[string]any{"token": token, "name": strProp("Client name")}, "token")},
		{Name: "openid_token", Description: "Exchange client credentials for an access token.", InputSchema: objectSchema(map[string]any{
			"client_id":     strProp("Client id"),
			"client_secret": strProp("Client secret"),
		}, "client_id", "client_secret")},
		{Name: "openid_audit_events", Description: "List tamper-evident audit events.", InputSchema: objectSchema(nil)},
		{Name: "openid_audit_flush", Description: "Flush the Merkle/OTS audit batch.", InputSchema: objectSchema(nil)},
		{Name: "openid_audit_verify", Description: "Verify one audit event by id.", InputSchema: objectSchema(map[string]any{"id": strProp("Event id")}, "id")},
	}
	return append(base, sparkTools()...)
}

type httpResult struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers,omitempty"`
	JSON    any               `json:"json,omitempty"`
	Text    string            `json:"text,omitempty"`
	URL     string            `json:"url,omitempty"`
}

func (s *Server) callTool(name string, args json.RawMessage, bearer string) *CallResult {
	tokenHint := bearer
	if tokenHint == "" && len(args) > 0 {
		var peek struct {
			Token string `json:"token"`
		}
		_ = json.Unmarshal(args, &peek)
		tokenHint = peek.Token
	}
	if err := s.denySparkLocked(name, tokenHint); err != nil {
		return textResult(nil, err)
	}
	if strings.HasPrefix(name, "spark_") {
		return s.callSpark(name, args, bearer)
	}
	switch name {
	case "openid_open":
		return textResult(s.toolOpen(), nil)
	case "openid_status":
		return textResult(s.toolStatus())
	case "openid_discover":
		return textResult(s.toolDiscover())
	case "openid_handle_available":
		var in struct {
			Handle string `json:"handle"`
		}
		if err := decodeArgs(args, &in); err != nil || in.Handle == "" {
			return textResult(nil, fmt.Errorf("handle is required"))
		}
		return textResult(s.getJSON("/idp/handles/"+url.PathEscape(in.Handle), ""))
	case "openid_register":
		var in struct {
			Handle   string `json:"handle"`
			Password string `json:"password"`
			Name     string `json:"name"`
			Email    string `json:"email"`
			Bio      string `json:"bio"`
		}
		if err := decodeArgs(args, &in); err != nil || in.Handle == "" || in.Password == "" {
			return textResult(nil, fmt.Errorf("handle and password are required"))
		}
		return textResult(s.postJSON("/idp/register", "", map[string]any{
			"handle": in.Handle, "password": in.Password, "name": in.Name,
			"email": in.Email, "bio": in.Bio, "createPod": true,
		}))
	case "openid_login":
		var in struct {
			Handle   string `json:"handle"`
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := decodeArgs(args, &in); err != nil || in.Password == "" {
			return textResult(nil, fmt.Errorf("password is required, plus handle or email"))
		}
		return textResult(s.postJSON("/idp/login", "", map[string]any{
			"handle": in.Handle, "email": in.Email, "password": in.Password,
		}))
	case "openid_me":
		var in struct {
			Token string `json:"token"`
		}
		if err := decodeArgs(args, &in); err != nil || in.Token == "" {
			return textResult(nil, fmt.Errorf("token is required"))
		}
		return textResult(s.getJSON("/idp/accounts/me", in.Token))
	case "openid_register_agent":
		var in struct {
			Name string `json:"name"`
		}
		_ = decodeArgs(args, &in)
		if in.Name == "" {
			in.Name = "grokbot"
		}
		return textResult(s.postJSON("/agents", "", map[string]any{"name": in.Name}))
	case "openid_list_agents":
		return textResult(s.getJSON("/agents", ""))
	case "openid_public_profile":
		var in struct {
			Handle string `json:"handle"`
		}
		if err := decodeArgs(args, &in); err != nil || in.Handle == "" {
			return textResult(nil, fmt.Errorf("handle is required"))
		}
		return textResult(s.getJSON("/i/"+url.PathEscape(in.Handle), ""))
	case "openid_pod_get":
		var in struct {
			Path   string `json:"path"`
			Token  string `json:"token"`
			Accept string `json:"accept"`
		}
		if err := decodeArgs(args, &in); err != nil || in.Path == "" {
			return textResult(nil, fmt.Errorf("path is required"))
		}
		return textResult(s.pod(http.MethodGet, in.Path, in.Token, in.Accept, "", nil))
	case "openid_pod_put":
		var in struct {
			Path        string `json:"path"`
			Body        string `json:"body"`
			ContentType string `json:"content_type"`
			Token       string `json:"token"`
		}
		if err := decodeArgs(args, &in); err != nil || in.Path == "" {
			return textResult(nil, fmt.Errorf("path and body are required"))
		}
		if in.ContentType == "" {
			in.ContentType = "text/plain"
		}
		return textResult(s.pod(http.MethodPut, in.Path, in.Token, "", in.ContentType, []byte(in.Body)))
	case "openid_pod_delete":
		var in struct {
			Path  string `json:"path"`
			Token string `json:"token"`
		}
		if err := decodeArgs(args, &in); err != nil || in.Path == "" {
			return textResult(nil, fmt.Errorf("path is required"))
		}
		return textResult(s.pod(http.MethodDelete, in.Path, in.Token, "", "", nil))
	case "openid_pod_list":
		var in struct {
			Path  string `json:"path"`
			Token string `json:"token"`
		}
		if err := decodeArgs(args, &in); err != nil || in.Path == "" {
			return textResult(nil, fmt.Errorf("path is required"))
		}
		if !strings.HasSuffix(in.Path, "/") {
			in.Path += "/"
		}
		return textResult(s.pod(http.MethodGet, in.Path, in.Token, "text/turtle", "", nil))
	case "openid_client_credentials":
		var in struct {
			Token string `json:"token"`
			Name  string `json:"name"`
		}
		if err := decodeArgs(args, &in); err != nil || in.Token == "" {
			return textResult(nil, fmt.Errorf("token is required"))
		}
		if in.Name == "" {
			in.Name = "grokbot"
		}
		return textResult(s.postJSON("/idp/client-credentials", in.Token, map[string]any{"name": in.Name}))
	case "openid_token":
		var in struct {
			ClientID     string `json:"client_id"`
			ClientSecret string `json:"client_secret"`
		}
		if err := decodeArgs(args, &in); err != nil || in.ClientID == "" || in.ClientSecret == "" {
			return textResult(nil, fmt.Errorf("client_id and client_secret are required"))
		}
		return textResult(s.oauthToken(in.ClientID, in.ClientSecret))
	case "openid_audit_events":
		return textResult(s.getJSON("/audit/events/", ""))
	case "openid_audit_flush":
		return textResult(s.postJSON("/audit/flush", "", map[string]any{}))
	case "openid_audit_verify":
		var in struct {
			ID string `json:"id"`
		}
		if err := decodeArgs(args, &in); err != nil || in.ID == "" {
			return textResult(nil, fmt.Errorf("id is required"))
		}
		return textResult(s.getJSON("/audit/events/"+url.PathEscape(in.ID)+"/verify", ""))
	default:
		return textResult(nil, fmt.Errorf("unknown tool %q", name))
	}
}

func (s *Server) toolOpen() map[string]any {
	health, _ := s.getJSON("/health", "")
	status, _ := s.getJSON("/api/status", "")
	return map[string]any{
		"open":      true,
		"dashboard": s.BaseURL + "/",
		"welcome":   s.BaseURL + "/welcome",
		"mcp":       s.BaseURL + "/mcp",
		"health":    health,
		"status":    status,
		"grokbot": map[string]any{
			"stdio": map[string]any{
				"command": "openid-mcp",
				"env":     map[string]string{"OPENID_BASE_URL": s.BaseURL},
			},
			"http": s.BaseURL + "/mcp",
			"flow": []string{
				"Call openid_open or openid_status",
				"Call openid_register_agent (or openid_login) to get a token",
				"Call openid_pod_put / openid_pod_get with that token",
				"Call spark_login (handle+password) if you have no token, then spark_save_conversation / spark_share_conversation",
				"Call openid_audit_flush then openid_audit_verify for receipts",
			},
			"spark": map[string]any{
				"mcp":    s.BaseURL + "/mcp",
				"stdio":  "go run ./cmd/mcp  (OPENID_BASE_URL=" + s.BaseURL + ")",
				"auth":     "spark_login in chat (handle + password), or Authorization: Bearer on HTTP /mcp",
				"prompt":   "Save this conversation to my Solid pod.",
				"tool":     "spark_save_conversation",
				"login":    "spark_login",
				"register": "spark_register",
			},
		},
	}
}

func (s *Server) toolStatus() (any, error) {
	health, err := s.getJSON("/health", "")
	if err != nil {
		return nil, err
	}
	status, err := s.getJSON("/api/status", "")
	if err != nil {
		return nil, err
	}
	return map[string]any{"health": health, "status": status}, nil
}

func (s *Server) toolDiscover() (any, error) {
	oidc, err := s.getJSON("/.well-known/openid-configuration", "")
	if err != nil {
		return nil, err
	}
	solid, err := s.getJSON("/.well-known/solid", "")
	if err != nil {
		return nil, err
	}
	return map[string]any{"oidc": oidc, "solid": solid}, nil
}

func (s *Server) getJSON(path, token string) (any, error) {
	res, err := s.do(http.MethodGet, path, token, "application/json", "", nil)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (s *Server) postJSON(path, token string, body any) (any, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return s.do(http.MethodPost, path, token, "application/json", "application/json", raw)
}

func (s *Server) oauthToken(id, secret string) (any, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", id)
	form.Set("client_secret", secret)
	return s.do(http.MethodPost, "/oauth/token", "", "application/json", "application/x-www-form-urlencoded", []byte(form.Encode()))
}

func (s *Server) pod(method, path, token, accept, contentType string, body []byte) (any, error) {
	path = strings.TrimPrefix(path, "/")
	if accept == "" {
		accept = "*/*"
	}
	return s.do(method, "/"+path, token, accept, contentType, body)
}

func (s *Server) do(method, path, token, accept, contentType string, body []byte) (*httpResult, error) {
	u := strings.TrimRight(s.BaseURL, "/") + path
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, u, rdr)
	if err != nil {
		return nil, err
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := s.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	out := &httpResult{
		Status:  resp.StatusCode,
		URL:     u,
		Headers: map[string]string{},
	}
	for _, k := range []string{"Content-Type", "ETag", "Location", "Wac-Allow", "Link"} {
		if v := resp.Header.Get(k); v != "" {
			out.Headers[k] = v
		}
	}
	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "json") && len(raw) > 0 {
		var v any
		if json.Unmarshal(raw, &v) == nil {
			out.JSON = v
		} else {
			out.Text = string(raw)
		}
	} else {
		out.Text = string(raw)
	}
	if resp.StatusCode >= 400 {
		return out, fmt.Errorf("openid %s %s -> %d %s", method, path, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return out, nil
}

func defaultHTTP() *http.Client {
	return &http.Client{Timeout: 15 * time.Second}
}
