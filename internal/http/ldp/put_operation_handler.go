//go:build cssstub

// Package ldp provides the PutOperationHandler struct.
package ldp

type PutOperationHandler struct {
	Store            interface{} // TODO: use actual ResourceStore type
	MetadataStrategy interface{} // TODO: use actual AuxiliaryStrategy type
}
