//go:build cssstub

package generate

import (
	"fmt"
	"net/url"
	"path"
	"strings"

	"solid-go/internal/pods/settings"
)

// SubdomainIdentifierGenerator generates identifiers using subdomains
type SubdomainIdentifierGenerator struct {
	baseURL string
}

// NewSubdomainIdentifierGenerator creates a new SubdomainIdentifierGenerator
func NewSubdomainIdentifierGenerator(baseURL string) *SubdomainIdentifierGenerator {
	return &SubdomainIdentifierGenerator{
		baseURL: baseURL,
	}
}

// Generate implements IdentifierGenerator.Generate
func (g *SubdomainIdentifierGenerator) Generate(settings *settings.PodSettings) (string, error) {
	// Parse base URL
	base, err := url.Parse(g.baseURL)
	if err != nil {
		return "", err
	}

	// Extract subdomain from WebID
	webID, err := url.Parse(settings.WebID)
	if err != nil {
		return "", err
	}

	// Get hostname without port
	host := strings.Split(webID.Host, ":")[0]

	// Create subdomain URL
	subdomain := fmt.Sprintf("%s.%s", host, base.Host)
	return fmt.Sprintf("%s://%s", base.Scheme, subdomain), nil
}

// SuffixIdentifierGenerator generates identifiers using suffixes
type SuffixIdentifierGenerator struct {
	baseURL string
}

// NewSuffixIdentifierGenerator creates a new SuffixIdentifierGenerator
func NewSuffixIdentifierGenerator(baseURL string) *SuffixIdentifierGenerator {
	return &SuffixIdentifierGenerator{
		baseURL: baseURL,
	}
}

// Generate implements IdentifierGenerator.Generate
func (g *SuffixIdentifierGenerator) Generate(settings *settings.PodSettings) (string, error) {
	// Parse base URL
	base, err := url.Parse(g.baseURL)
	if err != nil {
		return "", err
	}

	// Extract path from WebID
	webID, err := url.Parse(settings.WebID)
	if err != nil {
		return "", err
	}

	// Create suffix URL
	suffix := path.Join(base.Path, webID.Path)
	return fmt.Sprintf("%s://%s%s", base.Scheme, base.Host, suffix), nil
}
