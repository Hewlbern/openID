//go:build cssstub

// Package response provides response description interfaces for HTTP output.
package response

import "fmt"

// ResponseDescription represents an HTTP response, including status, metadata, and data.
type ResponseDescription struct {
	StatusCode int
	Metadata   map[string]interface{} // TODO: Replace with concrete RepresentationMetadata type
	Data       interface{}            // TODO: Replace with concrete data/stream type
}

// Describe returns a string summary of the response (for debugging/logging).
func (r *ResponseDescription) Describe() string {
	return fmt.Sprintf("Status: %d, Metadata: %v", r.StatusCode, r.Metadata)
}

// CreatedResponseDescription corresponds to a 201 response, containing the relevant location metadata.
type CreatedResponseDescription struct {
	ResponseDescription
}

// NewCreatedResponseDescription constructs a new CreatedResponseDescription with the given location.
func NewCreatedResponseDescription(location string) *CreatedResponseDescription {
	metadata := map[string]interface{}{"location": location} // TODO: Use actual RepresentationMetadata
	return &CreatedResponseDescription{
		ResponseDescription: ResponseDescription{
			StatusCode: 201,
			Metadata:   metadata,
		},
	}
}

// OkResponseDescription corresponds to a 200 or 206 response, containing relevant metadata and potentially data.
type OkResponseDescription struct {
	ResponseDescription
}

// NewOkResponseDescription constructs a new OkResponseDescription with the given metadata and data.
func NewOkResponseDescription(metadata map[string]interface{}, data interface{}) *OkResponseDescription {
	status := 200
	if _, ok := metadata["unit"]; ok {
		status = 206
	}
	return &OkResponseDescription{
		ResponseDescription: ResponseDescription{
			StatusCode: status,
			Metadata:   metadata,
			Data:       data,
		},
	}
}

// RedirectResponseDescription corresponds to a redirect response, containing the relevant location metadata.
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

// ResetResponseDescription corresponds to a 205 response.
type ResetResponseDescription struct {
	ResponseDescription
}

// NewResetResponseDescription constructs a new ResetResponseDescription.
func NewResetResponseDescription() *ResetResponseDescription {
	return &ResetResponseDescription{
		ResponseDescription: ResponseDescription{
			StatusCode: 205,
		},
	}
}
