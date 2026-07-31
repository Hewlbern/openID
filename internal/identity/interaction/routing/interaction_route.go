//go:build cssstub

package routing

// RouteParameter represents the parameters supported for the given route
type RouteParameter interface{}

// ExtendedRoute represents a route that adds a parameter to an existing route type
type ExtendedRoute interface {
	GetPath(parameters map[string]string) string
	MatchPath(path string) map[string]string
}

// InteractionRoute represents routes used to handle the pathing for API calls
// They can have dynamic values in the paths they support
type InteractionRoute interface {
	/**
	 * Returns the path that is the result of having the specified values for the dynamic parameters.
	 *
	 * Will throw an error in case the input `parameters` object is missing one of the expected keys.
	 *
	 * @param parameters - Values for the dynamic parameters.
	 */
	GetPath(parameters map[string]string) string

	/**
	 * Checks if the provided path matches the route (pattern).
	 *
	 * The result will be `nil` if there is no match.
	 *
	 * If there is a match the result object will have the corresponding values for all the parameters.
	 *
	 * @param path - The path to verify.
	 */
	MatchPath(path string) map[string]string
}
