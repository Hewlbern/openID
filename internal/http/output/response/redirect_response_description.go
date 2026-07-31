//go:build cssstub

// Package response provides a RedirectResponseDescription for redirect responses.
package response

type RedirectResponseDescription struct {
	ResponseDescription
}

// NewRedirectResponseDescription constructs a new RedirectResponseDescription with the given status code, metadata, and location.
func NewRedirectResponseDescription(statusCode int, metadata map[string]interface{}, location string) *RedirectResponseDescription {
	metadata["location"] = location
	return &RedirectResponseDescription{
		ResponseDescription: ResponseDescription{
			StatusCode: statusCode,
			Metadata:   metadata,
		},
	}
}
