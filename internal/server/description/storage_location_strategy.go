//go:build cssstub

package description

// ResourceIdentifier represents a resource's unique identifier (e.g., a URI or path).
type ResourceIdentifier struct {
	Path string
}

// StorageLocationStrategy is used to find the storage a specific identifier is located in.
type StorageLocationStrategy interface {
	// GetStorageIdentifier returns the identifier of the storage that contains the given resource.
	// Can return an error if the input identifier is not part of any storage.
	GetStorageIdentifier(identifier ResourceIdentifier) (ResourceIdentifier, error)
}
