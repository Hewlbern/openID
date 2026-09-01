package conversations

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var (
	headingRE = regexp.MustCompile(`(?m)^#{1,3}\s+(.+)$`)
	turnRE    = regexp.MustCompile(`(?im)^\s*(?:\*{0,2})(user|human|you|assistant|gemini|spark|model|system)(?:\*{0,2})\s*:\s*(?:\*{0,2})\s*(.*)$`)
)

// IsGeminiShareURL reports whether s looks like a public Gemini share link.
func IsGeminiShareURL(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		// allow bare g.co links without scheme
		if strings.Contains(s, "g.co/gemini/share") {
			return true
		}
		return false
	}
	host := strings.ToLower(u.Host)
	path := strings.ToLower(u.Path)
	if host == "g.co" && strings.Contains(path, "/gemini/share") {
		return true
	}
	if (host == "gemini.google.com" || strings.HasSuffix(host, ".gemini.google.com")) && strings.Contains(path, "/share") {
		return true
	}
	return false
}

// ParseTranscript turns pasted JSON, markdown, or role-prefixed text into a conversation.
func ParseTranscript(raw string) (*Conversation, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty transcript")
	}
	if c, err := parseJSONTranscript(raw); err == nil && c != nil && len(c.Messages) > 0 {
		return c, nil
	}
	return parseMarkdownTranscript(raw)
}

func parseJSONTranscript(raw string) (*Conversation, error) {
	var asSave SaveInput
	if err := json.Unmarshal([]byte(raw), &asSave); err == nil && (len(asSave.Messages) > 0 || asSave.Text != "") {
		// accept "content" aliases that the typed Message struct ignored
		var loose struct {
			Messages []map[string]any `json:"messages"`
		}
		if json.Unmarshal([]byte(raw), &loose) == nil && len(loose.Messages) > 0 {
			asSave.Messages = normalizeMessages(messagesFromLoose(loose.Messages))
		}
		c := &Conversation{
			Title:     asSave.Title,
			Source:    firstNonEmpty(asSave.Source, SourceGeminiSpark),
			SourceURL: firstNonEmpty(asSave.SourceURL, asSave.ID),
			Messages:  normalizeMessages(asSave.Messages),
		}
		if len(c.Messages) == 0 && asSave.Text != "" {
			inner, err := ParseTranscript(asSave.Text)
			if err != nil {
				return nil, err
			}
			if c.Title == "" {
				c.Title = inner.Title
			}
			c.Messages = inner.Messages
			if c.SourceURL == "" {
				c.SourceURL = inner.SourceURL
			}
		}
		if c.Title == "" {
			c.Title = titleFromMessages(c.Messages)
		}
		if len(c.Messages) == 0 {
			return nil, fmt.Errorf("no messages")
		}
		return c, nil
	}

	var msgs []Message
	if err := json.Unmarshal([]byte(raw), &msgs); err == nil && len(msgs) > 0 {
		msgs = normalizeMessages(msgs)
		return &Conversation{
			Title:    titleFromMessages(msgs),
			Source:   SourceGeminiSpark,
			Messages: msgs,
		}, nil
	}

	var loose []map[string]any
	if err := json.Unmarshal([]byte(raw), &loose); err == nil && len(loose) > 0 {
		msgs := messagesFromLoose(loose)
		if len(msgs) > 0 {
			return &Conversation{
				Title:    titleFromMessages(msgs),
				Source:   SourceGeminiSpark,
				Messages: msgs,
			}, nil
		}
	}
	return nil, fmt.Errorf("not json transcript")
}

func parseMarkdownTranscript(raw string) (*Conversation, error) {
	title := ""
	if m := headingRE.FindStringSubmatch(raw); m != nil {
		title = strings.TrimSpace(m[1])
	}
	lines := strings.Split(raw, "\n")
	var msgs []Message
	var cur *Message
	for _, line := range lines {
		if m := turnRE.FindStringSubmatch(line); m != nil {
			if cur != nil {
				cur.Text = strings.TrimSpace(cur.Text)
				if cur.Text != "" {
					msgs = append(msgs, *cur)
				}
			}
			role := normalizeRole(m[1])
			cur = &Message{Role: role, Text: strings.TrimSpace(m[2])}
			continue
		}
		if cur != nil {
			if cur.Text != "" {
				cur.Text += "\n"
			}
			cur.Text += line
		}
	}
	if cur != nil {
		cur.Text = strings.TrimSpace(cur.Text)
		if cur.Text != "" {
			msgs = append(msgs, *cur)
		}
	}
	if len(msgs) == 0 {
		// treat the whole paste as one user message so manual save still works
		body := strings.TrimSpace(raw)
		if title != "" {
			body = strings.TrimSpace(headingRE.ReplaceAllString(body, ""))
		}
		if body == "" {
			return nil, fmt.Errorf("no messages")
		}
		msgs = []Message{{Role: "user", Text: body}}
	}
	if title == "" {
		title = titleFromMessages(msgs)
	}
	return &Conversation{
		Title:    title,
		Source:   SourceGeminiSpark,
		Messages: msgs,
	}, nil
}

func normalizeMessages(in []Message) []Message {
	var out []Message
	for _, m := range in {
		text := strings.TrimSpace(m.Text)
		if text == "" {
			continue
		}
		out = append(out, Message{Role: normalizeRole(m.Role), Text: text})
	}
	return out
}

func messagesFromLoose(in []map[string]any) []Message {
	var out []Message
	for _, m := range in {
		role, _ := m["role"].(string)
		text := firstString(m, "text", "content", "message", "body")
		if text == "" {
			continue
		}
		out = append(out, Message{Role: normalizeRole(role), Text: strings.TrimSpace(text)})
	}
	return out
}

func normalizeRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "assistant", "gemini", "spark", "model", "bot":
		return "assistant"
	case "system":
		return "system"
	default:
		return "user"
	}
}

func titleFromMessages(msgs []Message) string {
	for _, m := range msgs {
		if m.Role == "user" && strings.TrimSpace(m.Text) != "" {
			return clipTitle(m.Text)
		}
	}
	if len(msgs) > 0 {
		return clipTitle(msgs[0].Text)
	}
	return "Untitled conversation"
}

func clipTitle(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) > 80 {
		return strings.TrimSpace(s[:80]) + "…"
	}
	if s == "" {
		return "Untitled conversation"
	}
	return s
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case string:
				if strings.TrimSpace(t) != "" {
					return t
				}
			}
		}
	}
	return ""
}
