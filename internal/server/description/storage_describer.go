//go:build cssstub

package description

// StorageDescriber describes storage containers and can handle storage description requests.
type StorageDescriber interface {
	// CanHandle checks if the describer can handle the given target.
	CanHandle(target ResourceIdentifier) error
	// Handle returns the storage description for the given target.
	Handle(target ResourceIdentifier) (interface{}, error)
}
