//go:build cssstub

// Package metadata provides a parser for cookies and stores their values as metadata.
package metadata

import (
	"fmt"
	"strings"
)

type CookieParser struct {
	CookieMap map[string]string // cookie name -> predicate URI
}

func NewCookieParser(cookieMap map[string]string) *CookieParser {
	return &CookieParser{CookieMap: cookieMap}
}

func (p *CookieParser) Handle(input map[string]interface{}) error {
	req, ok := input["request"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid request type")
	}
	metadata, ok := input["metadata"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid metadata type")
	}
	headers, ok := req["headers"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid headers type")
	}
	cookieHeader, _ := headers["cookie"].(string)
	cookies := parseCookies(cookieHeader)
	for name, uri := range p.CookieMap {
		if value, ok := cookies[name]; ok {
			addMetadata(metadata, uri, value)
		}
	}
	return nil
}

// parseCookies parses a cookie header string into a map.
func parseCookies(header string) map[string]string {
	cookies := make(map[string]string)
	for _, part := range strings.Split(header, ";") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 {
			cookies[kv[0]] = kv[1]
		}
	}
	return cookies
}
