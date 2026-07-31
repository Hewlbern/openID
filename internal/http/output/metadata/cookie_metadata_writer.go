//go:build cssstub

// Package metadatawriter implements a writer that generates Set-Cookie headers from metadata.
package metadatawriter

type CookieMetadataWriter struct {
	CookieMap map[string]struct {
		Name         string
		ExpirationUri string
	}
}

func NewCookieMetadataWriter(cookieMap map[string]struct{ Name, ExpirationUri string }) *CookieMetadataWriter {
	return &CookieMetadataWriter{CookieMap: cookieMap}
}

func (w *CookieMetadataWriter) Handle(input MetadataWriterInput) error {
	response := input.Response.(map[string]interface{}) // TODO: Replace with actual HttpResponse type
	metadata := input.Metadata.(map[string]interface{}) // TODO: Replace with actual RepresentationMetadata type
	for uri, cookie := range w.CookieMap {
		if value, ok := metadata[uri].(string); ok && value != "" {
			// Expiration is optional
			expires := ""
			if exp, ok := metadata[cookie.ExpirationUri].(string); ok && exp != "" {
				expires = "; Expires=" + exp
			}
			cookieHeader := cookie.Name + "=" + value + "; Path=/; SameSite=Lax" + expires
			setHeader(response, "Set-Cookie", cookieHeader)
		}
	}
	return nil
}
