//go:build cssstub

// Package metadatawriter implements a writer that adds a storage description link header.
package metadatawriter

type StorageDescriptionAdvertiser struct {
	StorageStrategy interface{}
	RelativePath    string
}

func NewStorageDescriptionAdvertiser(storageStrategy interface{}, relativePath string) *StorageDescriptionAdvertiser {
	return &StorageDescriptionAdvertiser{StorageStrategy: storageStrategy, RelativePath: relativePath}
}

func (w *StorageDescriptionAdvertiser) Handle(input MetadataWriterInput) error {
	response := input.Response.(map[string]interface{}) // TODO: Replace with actual HttpResponse type
	metadata := input.Metadata.(map[string]interface{}) // TODO: Replace with actual RepresentationMetadata type
	if types, ok := metadata["types"].([]string); !ok || !contains(types, "ldp:Resource") {
		return nil
	}
	identifier := map[string]string{"path": metadata["identifier"].(string)}
	storageStrategy, ok := w.StorageStrategy.(interface{ GetStorageIdentifier(identifier map[string]string) (map[string]string, error) })
	if !ok {
		return nil
	}
	storageRoot, err := storageStrategy.GetStorageIdentifier(identifier)
	if err != nil {
		return nil
	}
	storageDescription := joinUrl(storageRoot["path"], w.RelativePath)
	setHeader(response, "Link", "<"+storageDescription+">; rel=\"storageDescription\"")
	return nil
}

func contains(list []string, val string) bool {
	for _, v := range list {
		if v == val {
			return true
		}
	}
	return false
}

func joinUrl(base, rel string) string {
	if len(base) > 0 && base[len(base)-1] != '/' && len(rel) > 0 && rel[0] != '/' {
		return base + "/" + rel
	}
	return base + rel
}
