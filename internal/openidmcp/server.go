package openidmcp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Server is the OpenID MCP server (stdio + HTTP).
type Server struct {
	BaseURL string
	HTTP    *http.Client
}

func New(baseURL string) *Server {
	if baseURL == "" {
		baseURL = "http://localhost:4000"
	}
	return &Server{BaseURL: strings.TrimRight(baseURL, "/"), HTTP: defaultHTTP()}
}

func (s *Server) Handle(raw []byte) *Response {
	return s.HandleWithAuth(raw, "")
}

func (s *Server) HandleWithAuth(raw []byte, bearer string) *Response {
	if len(bytesTrimSpace(raw)) == 0 {
		return fail(nil, ErrParse, "empty request", nil)
	}
	var req Request
	if err := json.Unmarshal(raw, &req); err != nil {
		return fail(nil, ErrParse, "invalid json", err.Error())
	}
	if req.JSONRPC != "" && req.JSONRPC != "2.0" {
		return fail(req.ID, ErrInvalidReq, "jsonrpc must be 2.0", nil)
	}
	if req.Method == "" {
		return fail(req.ID, ErrInvalidReq, "method required", nil)
	}
	return s.dispatch(&req, bearer)
}

func (s *Server) dispatch(req *Request, bearer string) *Response {
	switch req.Method {
	case "initialize":
		var p initializeParams
		_ = json.Unmarshal(req.Params, &p)
		ver := ProtocolVersion
		if p.ProtocolVersion != "" {
			ver = p.ProtocolVersion
		}
		return ok(req.ID, initializeResult{
			ProtocolVersion: ver,
			Capabilities:    map[string]any{"tools": map[string]any{}},
			ServerInfo:      map[string]any{"name": ServerName, "version": ServerVersion},
			Instructions:    "OpenID Solid identity. Humans: openid_login then spark_save_conversation. Gemini Spark: connect https://<origin>/mcp and pass a Bearer token from login. Agents: openid_register_agent, then pod read/write tools.",
		})
	case "notifications/initialized", "initialized", "notifications/cancelled":
		return nil
	case "ping":
		return ok(req.ID, map[string]any{})
	case "tools/list":
		return ok(req.ID, toolsListResult{Tools: Tools()})
	case "tools/call":
		var p callParams
		if err := json.Unmarshal(req.Params, &p); err != nil || p.Name == "" {
			return fail(req.ID, ErrBadParams, "tools/call requires name", nil)
		}
		return ok(req.ID, s.callTool(p.Name, p.Arguments, bearer))
	default:
		if strings.HasPrefix(req.Method, "notifications/") {
			return nil
		}
		return fail(req.ID, ErrNoMethod, fmt.Sprintf("unknown method %s", req.Method), nil)
	}
}

func bytesTrimSpace(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}
