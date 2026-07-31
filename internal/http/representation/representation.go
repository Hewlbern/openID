//go:build cssstub

// Package representation provides types and interfaces translated from the TypeScript representation folder.
package representation

import "io"

// Representation is a resource representation with metadata, data, binary flag, and isEmpty.
type Representation interface {
	GetMetadata() *RepresentationMetadata
	GetData() io.Reader
	IsBinary() bool
	IsEmpty() bool
}
