package openidmcp

import (
	"encoding/json"
	"fmt"
	"strings"
)

func sparkTools() []Tool {
	token := strProp("Bearer token from openid_login or openid_register (or send Authorization: Bearer on HTTP /mcp)")
	id := strProp("Conversation id returned by spark_save_conversation")
	return []Tool{
		{Name: "spark_save_conversation", Description: "Save a Gemini Spark conversation into the signed-in human's Solid pod (audited LDP write).", InputSchema: objectSchema(map[string]any{
			"title":      strProp("Conversation title"),
			"messages":   map[string]any{"type": "array", "description": "Turns with role + text", "items": map[string]any{"type": "object", "properties": map[string]any{"role": strProp("user, assistant, model, or system"), "text": strProp("Message text"), "content": strProp("Alias for text")}}},
			"source_url": strProp("Optional original Gemini share URL (g.co/gemini/share/…)"),
			"text":       strProp("Optional pasted markdown/JSON transcript if messages is empty"),
			"token":      token,
		})},
		{Name: "spark_list_conversations", Description: "List Spark conversations saved in the caller's pod.", InputSchema: objectSchema(map[string]any{"token": token})},
		{Name: "spark_get_conversation", Description: "Read one saved Spark conversation.", InputSchema: objectSchema(map[string]any{"id": id, "token": token}, "id")},
		{Name: "spark_share_conversation", Description: "Mint a stable /share/c/{id} URL. Default is unlisted (secret token). Set public=true for a public link and WAC public read.", InputSchema: objectSchema(map[string]any{"id": id, "public": map[string]any{"type": "boolean", "description": "If true, anyone with the conversation id can read it"}, "token": token}, "id")},
		{Name: "spark_unshare_conversation", Description: "Revoke the share link and restore owner-only WAC.", InputSchema: objectSchema(map[string]any{"id": id, "token": token}, "id")},
	}
}

func (s *Server) callSpark(name string, args json.RawMessage, bearer string) *CallResult {
	token := bearer
	switch name {
	case "spark_save_conversation":
		var in struct {
			Title     string           `json:"title"`
			Messages  []map[string]any `json:"messages"`
			SourceURL string           `json:"source_url"`
			Text      string           `json:"text"`
			Token     string           `json:"token"`
		}
		if err := decodeArgs(args, &in); err != nil {
			return textResult(nil, err)
		}
		token = firstToken(in.Token, bearer)
		if token == "" {
			return textResult(nil, fmt.Errorf("token is required (openid_login or Authorization: Bearer)"))
		}
		body := map[string]any{
			"title":      in.Title,
			"messages":   normalizeSparkMessages(in.Messages),
			"source_url": in.SourceURL,
			"text":       in.Text,
			"source":     "gemini-spark",
		}
		return textResult(s.postJSON("/conversations", token, body))
	case "spark_list_conversations":
		var in struct {
			Token string `json:"token"`
		}
		_ = decodeArgs(args, &in)
		token = firstToken(in.Token, bearer)
		if token == "" {
			return textResult(nil, fmt.Errorf("token is required"))
		}
		return textResult(s.getJSON("/conversations", token))
	case "spark_get_conversation":
		var in struct {
			ID    string `json:"id"`
			Token string `json:"token"`
		}
		if err := decodeArgs(args, &in); err != nil || in.ID == "" {
			return textResult(nil, fmt.Errorf("id is required"))
		}
		token = firstToken(in.Token, bearer)
		if token == "" {
			return textResult(nil, fmt.Errorf("token is required"))
		}
		return textResult(s.getJSON("/conversations/"+in.ID, token))
	case "spark_share_conversation":
		var in struct {
			ID     string `json:"id"`
			Public bool   `json:"public"`
			Token  string `json:"token"`
		}
		if err := decodeArgs(args, &in); err != nil || in.ID == "" {
			return textResult(nil, fmt.Errorf("id is required"))
		}
		token = firstToken(in.Token, bearer)
		if token == "" {
			return textResult(nil, fmt.Errorf("token is required"))
		}
		return textResult(s.postJSON("/conversations/"+in.ID+"/share", token, map[string]any{"public": in.Public}))
	case "spark_unshare_conversation":
		var in struct {
			ID    string `json:"id"`
			Token string `json:"token"`
		}
		if err := decodeArgs(args, &in); err != nil || in.ID == "" {
			return textResult(nil, fmt.Errorf("id is required"))
		}
		token = firstToken(in.Token, bearer)
		if token == "" {
			return textResult(nil, fmt.Errorf("token is required"))
		}
		return textResult(s.postJSON("/conversations/"+in.ID+"/unshare", token, map[string]any{}))
	default:
		return textResult(nil, fmt.Errorf("unknown tool %q", name))
	}
}

func firstToken(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func normalizeSparkMessages(in []map[string]any) []map[string]string {
	var out []map[string]string
	for _, m := range in {
		role, _ := m["role"].(string)
		text, _ := m["text"].(string)
		if text == "" {
			text, _ = m["content"].(string)
		}
		if strings.TrimSpace(text) == "" {
			continue
		}
		if role == "" {
			role = "user"
		}
		out = append(out, map[string]string{"role": role, "text": text})
	}
	return out
}
