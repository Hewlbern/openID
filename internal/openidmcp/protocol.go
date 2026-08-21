package openidmcp

import (
	"encoding/json"
	"fmt"
)

const (
	ProtocolVersion = "2025-03-26"
	ServerName      = "openid"
	ServerVersion   = "1.0.0"
)

// Request is a JSON-RPC 2.0 request or notification.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError is a JSON-RPC 2.0 error object.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *RPCError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("rpc %d: %s", e.Code, e.Message)
}

const (
	ErrParse      = -32700
	ErrInvalidReq = -32600
	ErrNoMethod   = -32601
	ErrBadParams  = -32602
	ErrInternal   = -32603
)

func ok(id json.RawMessage, result any) *Response {
	if len(id) == 0 {
		return nil
	}
	return &Response{JSONRPC: "2.0", ID: id, Result: result}
}

func fail(id json.RawMessage, code int, msg string, data any) *Response {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	return &Response{JSONRPC: "2.0", ID: id, Error: &RPCError{Code: code, Message: msg, Data: data}}
}

type initializeParams struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities"`
	ClientInfo      map[string]any `json:"clientInfo"`
}

type initializeResult struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities"`
	ServerInfo      map[string]any `json:"serverInfo"`
	Instructions    string         `json:"instructions,omitempty"`
}

type toolsListResult struct {
	Tools []Tool `json:"tools"`
}

type callParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type CallResult struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

type Content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func textResult(v any, err error) *CallResult {
	if err != nil {
		return &CallResult{
			IsError: true,
			Content: []Content{{Type: "text", Text: err.Error()}},
		}
	}
	raw, marshalErr := json.MarshalIndent(v, "", "  ")
	if marshalErr != nil {
		return &CallResult{
			IsError: true,
			Content: []Content{{Type: "text", Text: marshalErr.Error()}},
		}
	}
	return &CallResult{Content: []Content{{Type: "text", Text: string(raw)}}}
}

func decodeArgs(raw json.RawMessage, dest any) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	return json.Unmarshal(raw, dest)
}
