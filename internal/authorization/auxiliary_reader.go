//go:build cssstub

// Package authorization provides implementations for reading auxiliary resource permissions.
package authorization

import (
	"path/filepath"

	"solid-go/internal/authorization/permissions"
)

// AuxiliaryReader reads permissions from auxiliary resources.
// It handles reading permissions from resources that are auxiliary to the main resource.
type AuxiliaryReader struct {
	reader PermissionReader
}

// NewAuxiliaryReader creates a new AuxiliaryReader with the given reader.
func NewAuxiliaryReader(reader PermissionReader) *AuxiliaryReader {
	return &AuxiliaryReader{
		reader: reader,
	}
}

// Read implements PermissionReader.
// It reads permissions from auxiliary resources associated with the given resource.
func (r *AuxiliaryReader) Read(input PermissionReaderInput) (map[string]permissions.PermissionSet, error) {
	result := make(map[string]permissions.PermissionSet)

	// For each requested resource
	for resource := range input.RequestedModes {
		// Get auxiliary resource paths
		auxPaths := r.getAuxiliaryPaths(resource)

		// Create a new permission set for this resource
		perms := permissions.NewPermissionSet()

		// Read permissions from each auxiliary resource
		for _, path := range auxPaths {
			auxInput := PermissionReaderInput{
				Credentials: input.Credentials,
				RequestedModes: map[string]permissions.PermissionSet{
					path: permissions.NewPermissionSet(),
				},
			}
			auxResult, err := r.reader.Read(auxInput)
			if err != nil {
				continue // Skip this auxiliary resource if it returns an error
			}

			// Add all permissions from this auxiliary resource
			if auxPerms, exists := auxResult[path]; exists {
				for mode := range auxPerms {
					perms.Add(mode)
				}
			}
		}

		result[resource] = perms
	}

	return result, nil
}

// getAuxiliaryPaths returns the paths of auxiliary resources for the given resource.
func (r *AuxiliaryReader) getAuxiliaryPaths(resource string) []string {
	dir := filepath.Dir(resource)
	base := filepath.Base(resource)
	return []string{
		filepath.Join(dir, "."+base+".meta"),
		filepath.Join(dir, "."+base+".acl"),
	}
}

// GetAuxiliaryPath gets the auxiliary resource path
func (r *AuxiliaryReader) GetAuxiliaryPath(path string) string {
	return filepath.Join(filepath.Dir(path), ".aux")
}

// IsAuxiliary checks if a path is an auxiliary resource
func (r *AuxiliaryReader) IsAuxiliary(path string) bool {
	return filepath.Base(path) == ".aux"
}
