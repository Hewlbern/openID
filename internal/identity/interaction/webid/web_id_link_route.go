//go:build cssstub

package webid

// import "../routing"

type WebIdLinkKey string

const WebIdLinkKeyValue WebIdLinkKey = "webIdLink"

type AccountIdRoute interface {
	GetPath(params map[string]string) string
	MatchPath(path string) map[string]string
}

type ExtendedRoute interface {
	GetPath(params map[string]string) string
	MatchPath(path string) map[string]string
}

type WebIdLinkRoute interface {
	GetPath(params map[string]string) string
	MatchPath(path string) map[string]string
}

type BaseWebIdLinkRoute struct {
	// *routing.IdInteractionRoute
	base AccountIdRoute
}

func NewBaseWebIdLinkRoute(base AccountIdRoute) *BaseWebIdLinkRoute {
	return &BaseWebIdLinkRoute{
		// IdInteractionRoute: routing.NewIdInteractionRoute(base, "webIdLink", true),
		base: base,
	}
}

func (r *BaseWebIdLinkRoute) GetPath(params map[string]string) string {
	// Placeholder implementation
	return "placeholder-webid-link-path"
}

func (r *BaseWebIdLinkRoute) MatchPath(path string) map[string]string {
	// Placeholder implementation
	return nil
}
