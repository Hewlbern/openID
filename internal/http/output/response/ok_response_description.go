//go:build cssstub

// Package response provides an OkResponseDescription for 200/206 responses.
package response

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
