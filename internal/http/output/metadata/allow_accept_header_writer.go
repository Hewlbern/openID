//go:build cssstub

// Package metadatawriter implements a writer that generates Allow, Accept-Patch, Accept-Post, and Accept-Put headers.
package metadatawriter

type AllowAcceptHeaderWriter struct {
	SupportedMethods []string
	AcceptTypes      map[string][]string // keys: patch, post, put
}

func NewAllowAcceptHeaderWriter(supportedMethods []string, acceptTypes map[string][]string) *AllowAcceptHeaderWriter {
	return &AllowAcceptHeaderWriter{SupportedMethods: supportedMethods, AcceptTypes: acceptTypes}
}

func (w *AllowAcceptHeaderWriter) Handle(input MetadataWriterInput) error {
	// This function generates Allow, Accept-Patch, Accept-Post, and Accept-Put headers
	// based on supported methods, accept types, and metadata.
	response := input.Response.(map[string]interface{}) // TODO: Replace with actual HttpResponse type
	metadata := input.Metadata.(map[string]interface{}) // TODO: Replace with actual RepresentationMetadata type

	// Determine resource type (document, container, unknown)
	resourceType := "unknown"
	if types, ok := metadata["types"].([]string); ok {
		for _, t := range types {
			if t == "ldp:Resource" {
				if isContainerPath(metadata["identifier"].(string)) {
					resourceType = "container"
				} else {
					resourceType = "document"
				}
				break
			}
		}
	} else if target, ok := metadata["target"].(string); ok {
		if isContainerPath(target) {
			resourceType = "container"
		} else {
			resourceType = "document"
		}
	}

	// Filter allowed methods (stub: allow all supported methods)
	allowedMethods := make(map[string]bool)
	for _, m := range w.SupportedMethods {
		allowedMethods[m] = true
	}
	// TODO: Remove methods from allowedMethods based on metadata (e.g., disallowedMethod)

	// Generate Allow header
	allowList := []string{}
	for m := range allowedMethods {
		allowList = append(allowList, m)
	}
	if len(allowList) > 0 {
		setHeader(response, "Allow", join(allowList, ", "))
	}

	// Generate Accept-Patch, Accept-Post, Accept-Put headers if method is allowed
	if allowedMethods["PATCH"] && len(w.AcceptTypes["patch"]) > 0 {
		setHeader(response, "Accept-Patch", join(w.AcceptTypes["patch"], ", "))
	}
	if allowedMethods["POST"] && len(w.AcceptTypes["post"]) > 0 {
		setHeader(response, "Accept-Post", join(w.AcceptTypes["post"], ", "))
	}
	if allowedMethods["PUT"] && len(w.AcceptTypes["put"]) > 0 {
		setHeader(response, "Accept-Put", join(w.AcceptTypes["put"], ", "))
	}

	return nil
}

// isContainerPath is a stub for checking if a path is a container
func isContainerPath(path string) bool {
	return len(path) > 0 && path[len(path)-1] == '/'
}

// setHeader is a stub for setting a header on the response
func setHeader(response map[string]interface{}, key, value string) {
	headers, ok := response["headers"].(map[string]string)
	if !ok {
		headers = make(map[string]string)
		response["headers"] = headers
	}
	headers[key] = value
}

// join is a helper to join string slices
func join(list []string, sep string) string {
	result := ""
	for i, s := range list {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}
