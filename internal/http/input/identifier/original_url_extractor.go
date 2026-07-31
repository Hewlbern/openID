//go:build cssstub

// Package identifier provides OriginalUrlExtractor for reconstructing the original URL from an HTTP request.
package identifier

import (
	"fmt"
	"net/url"
	"strings"
	"solid-go-main/internal/http/representation"
)

// IdentifierStrategy checks if an identifier is within the configured space.
type IdentifierStrategy interface {
	SupportsIdentifier(identifier *representation.ResourceIdentifier) bool
}

// OriginalUrlExtractorArgs holds configuration for OriginalUrlExtractor.
type OriginalUrlExtractorArgs struct {
	IdentifierStrategy IdentifierStrategy
	IncludeQueryString bool
	FixedBaseUrl       string
}

// OriginalUrlExtractor reconstructs the original URL of an incoming request.
type OriginalUrlExtractor struct {
	IdentifierStrategy IdentifierStrategy
	IncludeQueryString bool
	FixedHost          string
}

// NewOriginalUrlExtractor creates a new OriginalUrlExtractor.
func NewOriginalUrlExtractor(args OriginalUrlExtractorArgs) *OriginalUrlExtractor {
	fixedHost := ""
	if args.FixedBaseUrl != "" {
		u, err := url.Parse(args.FixedBaseUrl)
		if err == nil {
			fixedHost = u.Host
		}
	}
	return &OriginalUrlExtractor{
		IdentifierStrategy: args.IdentifierStrategy,
		IncludeQueryString: args.IncludeQueryString,
		FixedHost:          fixedHost,
	}
}

// Handle reconstructs the original URL and returns a ResourceIdentifier.
func (e *OriginalUrlExtractor) Handle(method string, headers map[string]string, urlPath string, connectionIsTLS bool) (*representation.ResourceIdentifier, error) {
	if urlPath == "" {
		return nil, fmt.Errorf("missing URL")
	}
	// Extract host and protocol
	host := headers["host"]
	protocol := "http"
	if connectionIsTLS {
		protocol = "https"
	}
	// TODO: parse Forwarded/X-Forwarded-* headers if present
	if e.FixedHost != "" {
		host = e.FixedHost
	}
	if host == "" {
		return nil, fmt.Errorf("missing Host header")
	}
	if strings.ContainsAny(host, "/\\*") {
		return nil, fmt.Errorf("invalid Host header: %s", host)
	}
	originalUrl := &url.URL{
		Scheme: protocol,
		Host:   host,
	}
	pathname, search := splitPathAndQuery(urlPath)
	originalUrl.Path = pathname
	if e.IncludeQueryString && search != "" {
		originalUrl.RawQuery = strings.TrimPrefix(search, "?")
	}
	identifier := &representation.ResourceIdentifier{Path: originalUrl.String()}
	if !e.IdentifierStrategy.SupportsIdentifier(identifier) {
		return nil, fmt.Errorf("the identifier %s is outside the configured identifier space", identifier.Path)
	}
	return identifier, nil
}

// splitPathAndQuery splits a URL path into path and query string.
func splitPathAndQuery(urlPath string) (string, string) {
	if idx := strings.Index(urlPath, "?"); idx != -1 {
		return urlPath[:idx], urlPath[idx:]
	}
	return urlPath, ""
}
