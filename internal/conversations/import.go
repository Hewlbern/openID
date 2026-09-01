package conversations

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

var (
	errNotPublicShare = fmt.Errorf("this Gemini share link is not publicly readable; paste the transcript instead. Google does not publish a bulk export API")
	titleTagRE        = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	ogTitleRE         = regexp.MustCompile(`(?is)<meta[^>]+property=["']og:title["'][^>]+content=["']([^"']+)["']`)
	ogDescRE          = regexp.MustCompile(`(?is)<meta[^>]+property=["']og:description["'][^>]+content=["']([^"']+)["']`)
	metaDescRE        = regexp.MustCompile(`(?is)<meta[^>]+name=["']description["'][^>]+content=["']([^"']+)["']`)
)

func normalizeShareURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		return "https://" + raw
	}
	return raw
}

// ImportShareURL fetches a public Gemini share page. Logged-in Google sessions
// are never used. If the page is not public, the caller should fall back to paste.
func (s *Service) ImportShareURL(ctx context.Context, rawURL string) (*Conversation, error) {
	rawURL = normalizeShareURL(rawURL)
	if !IsGeminiShareURL(rawURL) {
		return nil, fmt.Errorf("not a Gemini share URL (expected g.co/gemini/share/… or gemini.google.com/share/…)")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html,application/json;q=0.9,*/*;q=0.8")
	req.Header.Set("User-Agent", "OpenID-Solid/1.0 (public share import only; no Google session)")
	client := s.HTTP
	if client == nil {
		client = defaultHTTP()
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not fetch share URL: %w. Paste the transcript instead", err)
	}
	defer resp.Body.Close()
	finalHost := ""
	if resp.Request != nil && resp.Request.URL != nil {
		finalHost = strings.ToLower(resp.Request.URL.Host)
	}
	if strings.Contains(finalHost, "accounts.google.") || resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, errNotPublicShare
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("share URL returned HTTP %d. %s", resp.StatusCode, errNotPublicShare.Error())
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "json") {
		c, err := ParseTranscript(string(body))
		if err == nil {
			c.SourceURL = rawURL
			c.Source = SourceGeminiSpark
			return c, nil
		}
	}
	c, ok := conversationFromHTML(string(body), rawURL)
	if !ok {
		return nil, errNotPublicShare
	}
	return c, nil
}

func conversationFromHTML(html, sourceURL string) (*Conversation, bool) {
	title := htmlUnescape(firstMatch(ogTitleRE, html))
	if title == "" {
		title = htmlUnescape(firstMatch(titleTagRE, html))
	}
	title = strings.TrimSpace(strings.Split(title, " - ")[0])
	desc := htmlUnescape(firstMatch(ogDescRE, html))
	if desc == "" {
		desc = htmlUnescape(firstMatch(metaDescRE, html))
	}
	desc = strings.TrimSpace(desc)
	if title == "" || strings.EqualFold(title, "Google") || strings.EqualFold(title, "Gemini") {
		if desc == "" {
			return nil, false
		}
		title = clipTitle(desc)
	}
	text := desc
	if text == "" {
		return nil, false
	}
	return &Conversation{
		Title:     title,
		Source:    SourceGeminiSpark,
		SourceURL: sourceURL,
		Messages: []Message{
			{Role: "assistant", Text: text},
		},
	}, true
}

func firstMatch(re *regexp.Regexp, s string) string {
	m := re.FindStringSubmatch(s)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

func htmlUnescape(s string) string {
	r := strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", `"`,
		"&#39;", "'",
		"&nbsp;", " ",
	)
	return r.Replace(s)
}
