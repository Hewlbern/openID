//go:build cssstub

package routing

import (
	"errors"
	"strings"
)

type RelativePathInteractionRoute struct {
	base         InteractionRoute
	relativePath string
}

func NewRelativePathInteractionRoute(base InteractionRoute, relativePath string, ensureSlash bool) *RelativePathInteractionRoute {
	trimmedPath := trimLeadingSlashes(relativePath)
	if ensureSlash {
		trimmedPath = ensureTrailingSlash(trimmedPath)
	}

	return &RelativePathInteractionRoute{
		base:         base,
		relativePath: trimmedPath,
	}
}

func (r *RelativePathInteractionRoute) GetPath(parameters map[string]string) string {
	path := r.base.GetPath(parameters)
	if !strings.HasSuffix(path, "/") {
		panic(errors.New("expected " + path + " to end on a slash so it could be extended. This indicates a configuration error."))
	}
	return joinURL(path, r.relativePath)
}

func (r *RelativePathInteractionRoute) MatchPath(path string) map[string]string {
	if !strings.HasSuffix(path, r.relativePath) {
		return nil
	}

	head := path[:len(path)-len(r.relativePath)]

	return r.base.MatchPath(head)
}

// Helper function
func trimLeadingSlashes(path string) string {
	return strings.TrimLeft(path, "/")
}
