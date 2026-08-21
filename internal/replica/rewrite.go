package replica

import (
	"bytes"
	"strings"
)

// DefaultLocalBases are origins we rewrite onto the peer WebID host.
var DefaultLocalBases = []string{
	"http://localhost:4000",
	"http://127.0.0.1:4000",
	"http://localhost:3000",
	"http://127.0.0.1:3000",
	"http://localhost:8080",
}

func trimSlash(s string) string {
	return strings.TrimRight(s, "/")
}

// RewriteOrigin replaces each from-base with to-base in text bodies.
func RewriteOrigin(body []byte, fromBases []string, toBase string) []byte {
	toBase = trimSlash(toBase)
	if toBase == "" || len(body) == 0 {
		return body
	}
	out := body
	for _, from := range fromBases {
		from = trimSlash(from)
		if from == "" || from == toBase {
			continue
		}
		out = bytes.ReplaceAll(out, []byte(from), []byte(toBase))
	}
	return out
}

func isTextContent(ct string) bool {
	ct = strings.ToLower(strings.TrimSpace(ct))
	if i := strings.Index(ct, ";"); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	switch ct {
	case "text/turtle", "text/n3", "text/plain", "text/html", "text/css", "text/markdown",
		"application/json", "application/ld+json", "application/sparql-update":
		return true
	}
	return strings.HasPrefix(ct, "text/") || strings.Contains(ct, "json") || strings.Contains(ct, "xml")
}
