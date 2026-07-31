//go:build cssstub

package routing

type AbsolutePathInteractionRoute struct {
	path string
}

func NewAbsolutePathInteractionRoute(path string, ensureSlash bool) *AbsolutePathInteractionRoute {
	if ensureSlash {
		path = ensureTrailingSlash(path)
	}

	return &AbsolutePathInteractionRoute{
		path: path,
	}
}

func (r *AbsolutePathInteractionRoute) GetPath(parameters map[string]string) string {
	return r.path
}

func (r *AbsolutePathInteractionRoute) MatchPath(path string) map[string]string {
	if path == r.path {
		return make(map[string]string)
	}
	return nil
}
