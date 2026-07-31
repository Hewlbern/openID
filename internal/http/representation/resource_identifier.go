//go:build cssstub

package representation

// ResourceIdentifier represents a resource identifier
// Path is the path of the resource
type ResourceIdentifier struct {
	Path string
}

// IsResourceIdentifier checks if the object is a ResourceIdentifier.
func IsResourceIdentifier(obj interface{}) bool {
	if r, ok := obj.(ResourceIdentifier); ok {
		return r.Path != ""
	}
	if r, ok := obj.(*ResourceIdentifier); ok {
		return r.Path != ""
	}
	return false
}
