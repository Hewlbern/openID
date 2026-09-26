package solid

import (
	"path"
	"strings"
)

// CleanResourcePath normalizes an HTTP path to a slash-separated storage key.
// ".." segments are resolved so a traversal cannot escape an internal prefix.
func CleanResourcePath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.ReplaceAll(p, "\\", "/")
	trailing := strings.HasSuffix(p, "/")
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	p = path.Clean(p)
	p = strings.TrimPrefix(p, "/")
	if p == "." {
		return ""
	}
	if trailing && p != "" && !strings.HasSuffix(p, "/") {
		p += "/"
	}
	return p
}

// IsInternalServerPath reports paths that hold server state and must never be
// served over HTTP/LDP. This is the pod-root .openid tree (accounts, reset
// tokens, spark grants, shares), not a user's {handle}/.openid/ folder, plus
// storage bookkeeping files.
func IsInternalServerPath(p string) bool {
	p = CleanResourcePath(p)
	if p == "" {
		return false
	}
	seg := p
	if i := strings.IndexByte(p, '/'); i >= 0 {
		seg = p[:i]
	}
	if strings.EqualFold(seg, ".openid") || seg == ".root" || seg == ".meta.json" || seg == ".container" {
		return true
	}
	base := seg
	if i := strings.LastIndex(p, "/"); i >= 0 {
		base = p[i+1:]
	}
	if base == ".meta.json" || base == ".container" || base == ".root" || strings.HasSuffix(base, ".meta.json") {
		return true
	}
	return false
}
