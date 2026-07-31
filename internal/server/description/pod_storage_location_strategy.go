//go:build cssstub

package description

// IdentifierGenerator is used to extract the root pod URL from a resource identifier.
type IdentifierGenerator interface {
	ExtractPod(identifier ResourceIdentifier) (ResourceIdentifier, error)
}

// PodStorageLocationStrategy is used when the server has pods, each as a different storage.
type PodStorageLocationStrategy struct {
	generator IdentifierGenerator
}

// NewPodStorageLocationStrategy creates a new PodStorageLocationStrategy with the given generator.
func NewPodStorageLocationStrategy(generator IdentifierGenerator) *PodStorageLocationStrategy {
	return &PodStorageLocationStrategy{generator: generator}
}

// GetStorageIdentifier returns the root pod storage identifier for the given resource.
func (s *PodStorageLocationStrategy) GetStorageIdentifier(identifier ResourceIdentifier) (ResourceIdentifier, error) {
	return s.generator.ExtractPod(identifier)
}
