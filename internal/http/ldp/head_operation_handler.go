//go:build cssstub

// Package ldp provides the HeadOperationHandler struct.
package ldp

type HeadOperationHandler struct {
	Store      interface{} // TODO: use actual ResourceStore type
	ETagHandler interface{} // TODO: use actual ETagHandler type
}
