//go:build cssstub

package middleware

import (
	"fmt"
	"net/http"
	"strings"
)

// TargetExtractor extracts a target identifier from a request.
type TargetExtractor interface {
	HandleSafe(r *http.Request) (string, error) // For demo, just return a string identifier
}

// AuxiliaryIdentifierStrategy checks if an identifier is auxiliary.
type AuxiliaryIdentifierStrategy interface {
	IsAuxiliaryIdentifier(identifier string) bool
}

// AcpHeaderHandler handles all required ACP headers.
type AcpHeaderHandler struct {
	targetExtractor TargetExtractor
	strategy        AuxiliaryIdentifierStrategy
	modes           []string
	attributes      []string
}

// NewAcpHeaderHandler creates a new AcpHeaderHandler.
func NewAcpHeaderHandler(targetExtractor TargetExtractor, strategy AuxiliaryIdentifierStrategy, modes, attributes []string) *AcpHeaderHandler {
	return &AcpHeaderHandler{
		targetExtractor: targetExtractor,
		strategy:        strategy,
		modes:           modes,
		attributes:      attributes,
	}
}

// Handle sets the Link header if the identifier is auxiliary.
func (h *AcpHeaderHandler) Handle(w http.ResponseWriter, r *http.Request) {
	identifier, err := h.targetExtractor.HandleSafe(r)
	if err != nil {
		return
	}
	if !h.strategy.IsAuxiliaryIdentifier(identifier) {
		return
	}
	linkValues := []string{
		fmt.Sprintf("<%s>; rel=\"type\"", "https://www.w3.org/ns/solid/acp#AccessControlResource"),
	}
	for _, mode := range h.modes {
		linkValues = append(linkValues, fmt.Sprintf("<%s>; rel=\"%s\"", mode, "grant"))
	}
	for _, attr := range h.attributes {
		linkValues = append(linkValues, fmt.Sprintf("<%s>; rel=\"%s\"", attr, "attribute"))
	}
	w.Header().Add("Link", strings.Join(linkValues, ", "))
}
