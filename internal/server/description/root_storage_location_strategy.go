//go:build cssstub

package description

// RootStorageLocationStrategy is used when the server has one storage in the root container.
type RootStorageLocationStrategy struct {
	root ResourceIdentifier
}

// NewRootStorageLocationStrategy creates a new RootStorageLocationStrategy with the given base URL.
func NewRootStorageLocationStrategy(baseUrl string) *RootStorageLocationStrategy {
	return &RootStorageLocationStrategy{
		root: ResourceIdentifier{Path: baseUrl},
	}
}

// GetStorageIdentifier returns the root storage identifier.
func (s *RootStorageLocationStrategy) GetStorageIdentifier(_ ResourceIdentifier) (ResourceIdentifier, error) {
	return s.root, nil
}
