//go:build cssstub

// Package metadatawriter implements a writer that adds the WAC-Allow header for access control.
package metadatawriter

type WacAllowMetadataWriter struct{}

func (w *WacAllowMetadataWriter) Handle(input MetadataWriterInput) error {
	response := input.Response.(map[string]interface{}) // TODO: Replace with actual HttpResponse type
	metadata := input.Metadata.(map[string]interface{}) // TODO: Replace with actual RepresentationMetadata type
	userModes := toSet(metadata["userMode"].([]string))
	publicModes := toSet(metadata["publicMode"].([]string))
	for m := range publicModes {
		userModes[m] = struct{}{}
	}
	headerStrings := []string{}
	if len(userModes) > 0 {
		headerStrings = append(headerStrings, createAccessParam("user", userModes))
	}
	if len(publicModes) > 0 {
		headerStrings = append(headerStrings, createAccessParam("public", publicModes))
	}
	if len(headerStrings) > 0 {
		setHeader(response, "WAC-Allow", join(headerStrings, ","))
	}
	return nil
}

func toSet(list []string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, v := range list {
		set[v] = struct{}{}
	}
	return set
}

func createAccessParam(name string, modes map[string]struct{}) string {
	modeList := []string{}
	for m := range modes {
		modeList = append(modeList, m)
	}
	sort.Strings(modeList)
	return name + "=\"" + join(modeList, " ") + "\""
}
