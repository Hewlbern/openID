//go:build cssstub

// Package ldp provides the GetOperationHandler struct.
package ldp

type GetOperationHandler struct {
	Store      interface{} // TODO: use actual ResourceStore type
	ETagHandler interface{} // TODO: use actual ETagHandler type
}
