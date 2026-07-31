//go:build cssstub

// Package authorization provides implementations for combining multiple permission readers.
package authorization

import (
	"solid-go/internal/authorization/permissions"
)

// UnionPermissionReader combines multiple PermissionReaders.
// Every permission in every credential type is handled according to the rule `false` > `true` > `undefined`.
type UnionPermissionReader struct {
	readers []PermissionReader
}

// NewUnionPermissionReader creates a new UnionPermissionReader with the given readers.
func NewUnionPermissionReader(readers ...PermissionReader) *UnionPermissionReader {
	return &UnionPermissionReader{
		readers: readers,
	}
}

// Read implements PermissionReader.
// It combines the results of multiple PermissionReaders.
func (r *UnionPermissionReader) Read(input PermissionReaderInput) (map[string]permissions.PermissionSet, error) {
	// Create a result map to store combined permissions
	result := make(map[string]permissions.PermissionSet)

	// Try each reader in sequence
	for _, reader := range r.readers {
		readerResult, err := reader.Read(input)
		if err != nil {
			continue // Skip this reader if it returns an error
		}

		// Merge the results from this reader
		r.mergePermissionMaps(readerResult, result)
	}

	return result, nil
}

// mergePermissionMaps merges all entries of the given map into the result map.
func (r *UnionPermissionReader) mergePermissionMaps(permissionMap, result map[string]permissions.PermissionSet) {
	for identifier, permissionSet := range permissionMap {
		// Get or create the result set for this identifier
		resultSet, exists := result[identifier]
		if !exists {
			resultSet = permissions.NewPermissionSet()
			result[identifier] = resultSet
		}

		// Merge the permissions
		r.mergePermissions(permissionSet, resultSet)
	}
}

// mergePermissions adds the given permissions to the result object according to the combination rules.
// The rule is: `false` > `true` > `undefined`
func (r *UnionPermissionReader) mergePermissions(permissions, result permissions.PermissionSet) {
	for mode, value := range permissions {
		// Only update if the current value is not false
		if !result.Has(mode) || !result[mode] {
			result[mode] = value
		}
	}
}

// AddReader adds a new reader to the union.
func (r *UnionPermissionReader) AddReader(reader PermissionReader) {
	r.readers = append(r.readers, reader)
}

// RemoveReader removes a reader from the union.
func (r *UnionPermissionReader) RemoveReader(reader PermissionReader) {
	for i, rd := range r.readers {
		if rd == reader {
			r.readers = append(r.readers[:i], r.readers[i+1:]...)
			break
		}
	}
}

// GetReaders returns all readers in the union.
func (r *UnionPermissionReader) GetReaders() []PermissionReader {
	return r.readers
}

// ClearReaders removes all readers from the union.
func (r *UnionPermissionReader) ClearReaders() {
	r.readers = nil
}

// HasReader checks if the union contains a specific reader.
func (r *UnionPermissionReader) HasReader(reader PermissionReader) bool {
	for _, rd := range r.readers {
		if rd == reader {
			return true
		}
	}
	return false
}

// GetReaderCount returns the number of readers in the union.
func (r *UnionPermissionReader) GetReaderCount() int {
	return len(r.readers)
}
