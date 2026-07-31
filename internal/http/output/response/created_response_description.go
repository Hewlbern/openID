//go:build cssstub

// Package response provides a CreatedResponseDescription for 201 responses.
package response

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
