//go:build cssstub

// Package representation provides the BasicRepresentation struct.
package representation

import (
	"io"
	"strings"
)

// BasicRepresentation is a representation of a resource, including data, metadata, and binary flag.
type BasicRepresentation struct {
	data    io.Reader
	metadata *RepresentationMetadata
	binary   bool
}

// NewBasicRepresentation creates a new BasicRepresentation with all fields specified.
func NewBasicRepresentation(data io.Reader, metadata *RepresentationMetadata, binary bool) *BasicRepresentation {
	return &BasicRepresentation{
		data:    data,
		metadata: metadata,
		binary:  binary,
	}
}

// NewEmptyBasicRepresentation creates an empty BasicRepresentation.
func NewEmptyBasicRepresentation() *BasicRepresentation {
	return &BasicRepresentation{}
}

// NewBasicRepresentationWithContentType creates a BasicRepresentation with contentType and optional binary flag.
func NewBasicRepresentationWithContentType(data io.Reader, metadata *RepresentationMetadata, contentType string, binary ...bool) *BasicRepresentation {
	b := false
	if len(binary) > 0 {
		b = binary[0]
	} else if contentType != "" {
		b = isBinaryContentType(contentType)
	}
	if metadata != nil && contentType != "" {
		metadata.Add("contentType", contentType)
	}
	return &BasicRepresentation{
		data:    data,
		metadata: metadata,
		binary:  b,
	}
}

// NewBasicRepresentationWithIdentifier creates a BasicRepresentation with identifier and optional metadata and binary flag.
func NewBasicRepresentationWithIdentifier(data io.Reader, identifier string, metadata map[string]interface{}, binary ...bool) *BasicRepresentation {
	b := false
	if len(binary) > 0 {
		b = binary[0]
	}
	meta := NewRepresentationMetadata(identifier)
	for k, v := range metadata {
		meta.Add(k, v)
	}
	return &BasicRepresentation{
		data:    data,
		metadata: meta,
		binary:  b,
	}
}

// GetMetadata returns the metadata of the representation.
func (r *BasicRepresentation) GetMetadata() *RepresentationMetadata {
	return r.metadata
}

// GetData returns the data stream of the representation.
func (r *BasicRepresentation) GetData() io.Reader {
	return r.data
}

// IsBinary returns true if the representation is binary.
func (r *BasicRepresentation) IsBinary() bool {
	return r.binary
}

// IsEmpty returns true if the representation has no data.
func (r *BasicRepresentation) IsEmpty() bool {
	return r.data == nil
}

// Setters for flexibility (not in TypeScript, but useful in Go)
func (r *BasicRepresentation) SetData(data io.Reader) {
	r.data = data
}
func (r *BasicRepresentation) SetMetadata(metadata *RepresentationMetadata) {
	r.metadata = metadata
}
func (r *BasicRepresentation) SetBinary(binary bool) {
	r.binary = binary
}

// isBinaryContentType infers if a content type is binary.
func isBinaryContentType(contentType string) bool {
	ct := strings.ToLower(contentType)
	return strings.HasPrefix(ct, "application/octet-stream") ||
		strings.HasPrefix(ct, "image/") ||
		strings.HasPrefix(ct, "audio/") ||
		strings.HasPrefix(ct, "video/")
}
