//go:build cssstub

package routing

import (
	"errors"
	"regexp"
	"strings"
)

type IdInteractionRoute struct {
	base        InteractionRoute
	idName      string
	ensureSlash bool
	matchRegex  *regexp.Regexp
}

func NewIdInteractionRoute(base InteractionRoute, idName string, ensureSlash bool) *IdInteractionRoute {
	var matchRegex *regexp.Regexp
	if ensureSlash {
		matchRegex = regexp.MustCompile(`(.*/)([^/]+)/$`)
	} else {
		matchRegex = regexp.MustCompile(`(.*/)([^/]+)$`)
	}

	return &IdInteractionRoute{
		base:        base,
		idName:      idName,
		ensureSlash: ensureSlash,
		matchRegex:  matchRegex,
	}
}

func (r *IdInteractionRoute) GetPath(parameters map[string]string) string {
	id, exists := parameters[r.idName]
	if !exists || id == "" {
		panic(errors.New("missing " + r.idName + " from getPath call. This implies a misconfigured path."))
	}

	path := r.base.GetPath(parameters)
	if r.ensureSlash {
		return joinURL(path, ensureTrailingSlash(id))
	}
	return joinURL(path, id)
}

func (r *IdInteractionRoute) MatchPath(path string) map[string]string {
	match := r.matchRegex.FindStringSubmatch(path)

	if match == nil {
		return nil
	}

	id := match[2]
	head := match[1]

	baseParameters := r.base.MatchPath(head)

	if baseParameters == nil {
		return nil
	}

	// Add the ID parameter to the base parameters
	result := make(map[string]string)
	for k, v := range baseParameters {
		result[k] = v
	}
	result[r.idName] = id

	return result
}

// Helper functions
func joinURL(base, path string) string {
	if strings.HasSuffix(base, "/") {
		return base + path
	}
	return base + "/" + path
}

func ensureTrailingSlash(path string) string {
	if strings.HasSuffix(path, "/") {
		return path
	}
	return path + "/"
}
