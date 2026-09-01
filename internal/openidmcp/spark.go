package openidmcp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

const sparkSaveDescription = `FIRST-CLASS tool for Gemini Spark. When the user asks to save, upload, store, or write this chat / thread / conversation to their Solid pod or OpenID, you MUST call this tool immediately with the full current conversation. Do not tell them to leave Spark or paste into /app — that UI is a fallback only. Pass title and the complete messages array as [{role, content|text, timestamp?}]. Include timezone-aware ISO-8601 timestamps on each message when Spark has them. Spark itself supplies the transcript; do not scrape Gemini. The server writes conversations/spark/{id}.json (JSON-LD) plus a Turtle sidecar (schema:Conversation / schema:Message, dcterms:created, dcterms:modified, schema:dateCreated, foaf/schema agent roles, owner WebID, source=gemini-spark) using audited LDP PUTs after ensuring the conversations/ and conversations/spark/ containers exist (paths end with /). Returns resourceUrl, webId, optional shareUrl, created/modified, and confirmation text to show the user.`

func sparkTools() []Tool {
	token := strProp("Spark connect token from /app (POST /idp/spark-token). Prefer this over the forever login Bearer. Or send Authorization: Bearer on HTTP /mcp.")
	id := strProp("Conversation id returned by spark_save_conversation")
	return []Tool{
		{Name: "spark_save_conversation", Description: sparkSaveDescription, InputSchema: objectSchema(map[string]any{
			"title": strProp("Conversation title"),
			"messages": map[string]any{
				"type":        "array",
				"description": "Full current thread. Each turn: role (user|assistant|model|system), content or text, optional timestamp (ISO-8601, timezone-aware)",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"role":      strProp("user, assistant, model, or system"),
						"text":      strProp("Message text"),
						"content":   strProp("Alias for text (Spark often sends content)"),
						"timestamp": strProp("Optional ISO-8601 timestamp, timezone-aware when known"),
						"time":      strProp("Alias for timestamp"),
					},
				},
			},
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
		msgs := normalizeSparkMessages(in.Messages)
		body := map[string]any{
			"title":      in.Title,
			"messages":   msgs,
			"source_url": in.SourceURL,
			"text":       in.Text,
			"source":     "gemini-spark",
		}
		res, err := s.postJSON("/conversations", token, body)
		if err != nil && isContainerPostBug(err) {
			return textResult(s.saveSparkViaLDP(token, in.Title, msgs, in.SourceURL, in.Text))
		}
		if err != nil {
			return textResult(nil, err)
		}
		return textResult(unwrapSparkSave(res), nil)
	case "spark_list_conversations":
		var in struct {
			Token string `json:"token"`
		}
		_ = decodeArgs(args, &in)
		token = firstToken(in.Token, bearer)
		if token == "" {
			return textResult(nil, fmt.Errorf("token is required"))
		}
		res, err := s.getJSON("/conversations", token)
		if err != nil && isContainerPostBug(err) {
			return textResult(s.listSparkViaLDP(token))
		}
		return textResult(res, err)
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
		res, err := s.getJSON("/conversations/"+in.ID, token)
		if err != nil && isContainerPostBug(err) {
			return textResult(s.getSparkViaLDP(token, in.ID))
		}
		return textResult(res, err)
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

func isContainerPostBug(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Can only POST to containers") ||
		strings.Contains(msg, " -> 404 ") ||
		strings.Contains(msg, " -> 405 ")
}

func unwrapSparkSave(res any) any {
	if res == nil {
		return nil
	}
	var m map[string]any
	switch v := res.(type) {
	case *httpResult:
		if v == nil {
			return nil
		}
		m, _ = v.JSON.(map[string]any)
		if m == nil {
			return v
		}
	case map[string]any:
		m = v
	default:
		return res
	}
	if _, has := m["resourceUrl"]; has {
		return m
	}
	if _, has := m["confirmation"]; has {
		return m
	}
	return saveResultFromConversationMap(m)
}

func saveResultFromConversationMap(m map[string]any) map[string]any {
	id, _ := m["id"].(string)
	title, _ := m["title"].(string)
	resource, _ := m["resource"].(string)
	owner, _ := m["owner"].(string)
	if owner == "" {
		owner, _ = m["creator"].(string)
	}
	resourceURL, _ := m["@id"].(string)
	if resourceURL == "" {
		resourceURL, _ = m["resourceUrl"].(string)
	}
	created := m["created"]
	if created == nil {
		created = m["dateCreated"]
	}
	modified := m["updated"]
	if modified == nil {
		modified = m["dateModified"]
	}
	msgs, _ := m["messages"].([]any)
	out := map[string]any{
		"ok":           true,
		"id":           id,
		"title":        title,
		"resourceUrl":  resourceURL,
		"metaTtlUrl":   m["metaTtl"],
		"webId":        owner,
		"source":       firstNonEmptyString(fmt.Sprint(m["source"]), "gemini-spark"),
		"created":      created,
		"modified":     modified,
		"messageCount": len(msgs),
		"conversation": m,
	}
	if share, ok := m["share"].(map[string]any); ok {
		if u, _ := share["url"].(string); u != "" {
			out["shareUrl"] = u
		}
	}
	out["confirmation"] = fmt.Sprintf(
		"Saved %q to your Solid pod as %s (%d messages, source=gemini-spark). WebID %s.",
		title, resourceURL, len(msgs), owner,
	)
	if resource != "" && resourceURL == "" {
		out["resourceUrl"] = resource
	}
	return out
}

func normalizeSparkMessages(in []map[string]any) []map[string]any {
	var out []map[string]any
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
		item := map[string]any{"role": role, "text": text}
		if ts := firstNonEmptyString(asString(m["timestamp"]), asString(m["time"]), asString(m["created"])); ts != "" {
			item["timestamp"] = ts
		}
		out = append(out, item)
	}
	return out
}

func asString(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		if t > 1e12 {
			return time.UnixMilli(int64(t)).UTC().Format(time.RFC3339)
		}
		if t > 0 {
			return time.Unix(int64(t), 0).UTC().Format(time.RFC3339)
		}
	}
	return ""
}

func firstNonEmptyString(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func (s *Server) accountFromToken(token string) (handle, webID, podPath string, err error) {
	res, err := s.getJSON("/idp/accounts/me", token)
	if err != nil {
		return "", "", "", err
	}
	hr, ok := res.(*httpResult)
	var m map[string]any
	if ok && hr != nil {
		m, _ = hr.JSON.(map[string]any)
	} else {
		m, _ = res.(map[string]any)
	}
	if m == nil {
		return "", "", "", fmt.Errorf("account lookup failed")
	}
	handle, _ = m["handle"].(string)
	webID, _ = m["webId"].(string)
	podPath, _ = m["podPath"].(string)
	if handle == "" {
		return "", "", "", fmt.Errorf("account missing handle")
	}
	if podPath == "" {
		podPath = handle + "/"
	}
	return handle, webID, podPath, nil
}

func (s *Server) ensureLDPContainer(token, path string) error {
	path = strings.TrimPrefix(path, "/")
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	req, err := http.NewRequest(http.MethodPut, strings.TrimRight(s.BaseURL, "/")+"/"+path, strings.NewReader("# container\n"))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "text/turtle")
	req.Header.Set("Link", `<http://www.w3.org/ns/ldp#BasicContainer>; rel="type"`)
	resp, err := s.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusNoContent, http.StatusConflict:
		return nil
	}
	// Some servers return an odd PUT status; accept if GET then finds the container.
	getReq, err := http.NewRequest(http.MethodGet, strings.TrimRight(s.BaseURL, "/")+"/"+path, nil)
	if err != nil {
		return fmt.Errorf("ensure container %s -> %d", path, resp.StatusCode)
	}
	getReq.Header.Set("Authorization", "Bearer "+token)
	getReq.Header.Set("Accept", "text/turtle, */*")
	getResp, err := s.HTTP.Do(getReq)
	if err != nil {
		return fmt.Errorf("ensure container %s -> %d", path, resp.StatusCode)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode < 400 {
		return nil
	}
	return fmt.Errorf("ensure container %s -> %d", path, resp.StatusCode)
}

func (s *Server) putLDP(token, path, ct, body string) error {
	path = strings.TrimPrefix(path, "/")
	if strings.HasSuffix(path, "/") {
		return fmt.Errorf("refusing to PUT a document to container path %q", path)
	}
	_, err := s.pod("PUT", path, token, "", ct, []byte(body))
	return err
}

func (s *Server) saveSparkViaLDP(token, title string, msgs []map[string]any, sourceURL, text string) (any, error) {
	handle, webID, podPath, err := s.accountFromToken(token)
	if err != nil {
		return nil, err
	}
	if len(msgs) == 0 && strings.TrimSpace(text) != "" {
		msgs = []map[string]any{{"role": "user", "text": strings.TrimSpace(text)}}
	}
	if len(msgs) == 0 {
		return nil, fmt.Errorf("messages or text required")
	}
	if strings.TrimSpace(title) == "" {
		if t, _ := msgs[0]["text"].(string); t != "" {
			title = t
			if len(title) > 80 {
				title = strings.TrimSpace(title[:80]) + "…"
			}
		} else {
			title = "Untitled conversation"
		}
	}
	id := uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339)
	base := strings.Trim(podPath, "/")
	if base == "" {
		base = handle
	}
	convoDir := base + "/conversations/"
	sparkDir := base + "/conversations/spark/"
	if err := s.ensureLDPContainer(token, convoDir); err != nil {
		return nil, err
	}
	if err := s.ensureLDPContainer(token, sparkDir); err != nil {
		return nil, err
	}
	resourcePath := sparkDir + id + ".json"
	ttlPath := sparkDir + id + ".ttl"
	resourceURL := strings.TrimRight(s.BaseURL, "/") + "/" + resourcePath
	ttlURL := strings.TrimRight(s.BaseURL, "/") + "/" + ttlPath
	doc := map[string]any{
		"@context": map[string]any{
			"@vocab":       "https://schema.org/",
			"schema":       "https://schema.org/",
			"dcterms":      "http://purl.org/dc/terms/",
			"foaf":         "http://xmlns.com/foaf/0.1/",
			"xsd":          "http://www.w3.org/2001/XMLSchema#",
			"created":      map[string]any{"@id": "dcterms:created", "@type": "xsd:dateTime"},
			"updated":      map[string]any{"@id": "dcterms:modified", "@type": "xsd:dateTime"},
			"dateCreated":  map[string]any{"@id": "schema:dateCreated", "@type": "xsd:dateTime"},
			"dateModified": map[string]any{"@id": "schema:dateModified", "@type": "xsd:dateTime"},
		},
		"@type":        "Conversation",
		"@id":          resourceURL,
		"id":           id,
		"title":        title,
		"name":         title,
		"source":       "gemini-spark",
		"sourceUrl":    sourceURL,
		"created":      now,
		"updated":      now,
		"dateCreated":  now,
		"dateModified": now,
		"messages":     msgs,
		"owner":        webID,
		"creator":      webID,
		"podPath":      strings.TrimSuffix(podPath, "/") + "/",
		"resource":     resourcePath,
		"metaTtl":      ttlURL,
	}
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := s.putLDP(token, resourcePath, "application/ld+json", string(raw)); err != nil {
		return nil, err
	}
	ttl := sparkTurtle(resourceURL, title, id, webID, now, msgs)
	if err := s.putLDP(token, ttlPath, "text/turtle", ttl); err != nil {
		return nil, err
	}
	return map[string]any{
		"ok":           true,
		"id":           id,
		"title":        title,
		"resourceUrl":  resourceURL,
		"metaTtlUrl":   ttlURL,
		"webId":        webID,
		"pod":          strings.TrimRight(s.BaseURL, "/") + "/" + base + "/",
		"source":       "gemini-spark",
		"created":      now,
		"modified":     now,
		"messageCount": len(msgs),
		"confirmation": fmt.Sprintf(
			"Saved %q to your Solid pod as %s (%d messages, source=gemini-spark). WebID %s. Created %s.",
			title, resourceURL, len(msgs), webID, now,
		),
		"conversation": doc,
	}, nil
}

func (s *Server) listSparkViaLDP(token string) (any, error) {
	handle, _, podPath, err := s.accountFromToken(token)
	if err != nil {
		return nil, err
	}
	base := strings.Trim(podPath, "/")
	if base == "" {
		base = handle
	}
	dir := base + "/conversations/spark/"
	res, err := s.pod("GET", dir, token, "text/turtle", "", nil)
	if err != nil {
		return map[string]any{"conversations": []any{}}, nil
	}
	return res, nil
}

func (s *Server) getSparkViaLDP(token, id string) (any, error) {
	handle, _, podPath, err := s.accountFromToken(token)
	if err != nil {
		return nil, err
	}
	base := strings.Trim(podPath, "/")
	if base == "" {
		base = handle
	}
	return s.pod("GET", base+"/conversations/spark/"+id+".json", token, "application/ld+json", "", nil)
}

func sparkTurtle(id, title, convID, owner, now string, msgs []map[string]any) string {
	var b strings.Builder
	b.WriteString("@prefix schema: <https://schema.org/> .\n")
	b.WriteString("@prefix dcterms: <http://purl.org/dc/terms/> .\n")
	b.WriteString("@prefix foaf: <http://xmlns.com/foaf/0.1/> .\n")
	b.WriteString("@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .\n\n")
	fmt.Fprintf(&b, "<%s> a schema:Conversation ;\n", id)
	fmt.Fprintf(&b, "  schema:name %q ;\n", title)
	fmt.Fprintf(&b, "  schema:identifier %q ;\n", convID)
	fmt.Fprintf(&b, "  dcterms:created %q^^xsd:dateTime ;\n", now)
	fmt.Fprintf(&b, "  dcterms:modified %q^^xsd:dateTime ;\n", now)
	fmt.Fprintf(&b, "  schema:dateCreated %q^^xsd:dateTime ;\n", now)
	fmt.Fprintf(&b, "  schema:dateModified %q^^xsd:dateTime ;\n", now)
	fmt.Fprintf(&b, "  dcterms:source \"gemini-spark\"")
	if owner != "" {
		fmt.Fprintf(&b, " ;\n  schema:creator <%s> ;\n  foaf:maker <%s>", owner, owner)
	}
	for i := range msgs {
		fmt.Fprintf(&b, " ;\n  schema:hasPart <%s#msg-%d>", id, i+1)
	}
	b.WriteString(" .\n")
	for i, m := range msgs {
		role, _ := m["role"].(string)
		text, _ := m["text"].(string)
		fmt.Fprintf(&b, "\n<%s#msg-%d> a schema:Message ;\n", id, i+1)
		fmt.Fprintf(&b, "  schema:text %q ;\n", text)
		fmt.Fprintf(&b, "  schema:author %q", role)
		if ts, _ := m["timestamp"].(string); ts != "" {
			fmt.Fprintf(&b, " ;\n  schema:dateCreated %q^^xsd:dateTime ;\n  dcterms:created %q^^xsd:dateTime", ts, ts)
		}
		if role == "assistant" {
			b.WriteString(" ;\n  foaf:Agent \"gemini-spark\"")
		} else if owner != "" {
			fmt.Fprintf(&b, " ;\n  foaf:maker <%s>", owner)
		}
		b.WriteString(" .\n")
	}
	return b.String()
}
