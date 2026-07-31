//go:build cssstub

// Package response provides a ResetResponseDescription for 205 responses.
package response

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
