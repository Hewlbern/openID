package conversations

import (
	"html"
	"strings"
	"time"
)

func renderShareHTML(c *Conversation) string {
	if c == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>`)
	b.WriteString(html.EscapeString(c.Title))
	b.WriteString(` — oid</title>
  <link rel="icon" href="/static/favicon.svg" type="image/svg+xml" />
  <link rel="stylesheet" href="/static/styles.css" />
</head>
<body>
  <header class="title">
    <a class="brand" href="/">
      <span class="mark" aria-hidden="true"></span>
      <span class="app-name">oid</span>
    </a>
    <span class="status">shared conversation</span>
  </header>
  <article class="share-page">
    <p class="lede">Read-only copy from an OpenID pod.</p>
    <h1>`)
	b.WriteString(html.EscapeString(c.Title))
	b.WriteString(`</h1>
    <div class="chips">
      <span>`)
	b.WriteString(html.EscapeString(firstNonEmpty(c.Source, SourceGeminiSpark)))
	b.WriteString(`</span>
      <span>`)
	b.WriteString(html.EscapeString(c.Updated.UTC().Format(time.RFC3339)))
	b.WriteString(`</span>
    </div>`)
	if c.SourceURL != "" {
		b.WriteString(`<p class="mono">`)
		b.WriteString(html.EscapeString(c.SourceURL))
		b.WriteString(`</p>`)
	}
	for _, m := range c.Messages {
		role := html.EscapeString(m.Role)
		b.WriteString(`<div class="turn `)
		b.WriteString(role)
		b.WriteString(`"><span class="who">`)
		b.WriteString(role)
		b.WriteString(`</span><div class="bubble">`)
		b.WriteString(nl2br(html.EscapeString(m.Text)))
		b.WriteString(`</div></div>`)
	}
	b.WriteString(`
    <p class="legal">Saved on OpenID. Sharing can be revoked by the owner.</p>
  </article>
</body>
</html>`)
	return b.String()
}

func nl2br(s string) string {
	return strings.ReplaceAll(s, "\n", "<br />")
}
