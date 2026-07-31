//go:build cssstub

// Package authorization provides implementations for static permission reading.
package authorization

import (
	"solid-go/internal/authorization/permissions"
)

// AllStaticReader sets all permissions to true or false
// independently of the identifier and requested permissions.
type AllStaticReader struct {
	permissionSet permissions.PermissionSet
}

// NewAllStaticReader creates a new AllStaticReader.
func NewAllStaticReader(allow bool) *AllStaticReader {
	perms := permissions.NewPermissionSet()
	if allow {
		perms.Add(permissions.Read)
		perms.Add(permissions.Write)
		perms.Add(permissions.Append)
		perms.Add(permissions.Create)
		perms.Add(permissions.Delete)
	}
	return &AllStaticReader{
		permissionSet: perms,
	}
}

// Read implements PermissionReader.
// It sets all permissions to true or false independently of the identifier and requested permissions.
func (r *AllStaticReader) Read(input PermissionReaderInput) (map[string]permissions.PermissionSet, error) {
	result := make(map[string]permissions.PermissionSet)
	for resource := range input.RequestedModes {
		result[resource] = r.permissionSet
	}
	return result, nil
}

// GetPermissionSet returns the permission set used by this reader.
func (r *AllStaticReader) GetPermissionSet() permissions.PermissionSet {
	return r.permissionSet
}

// GetPermissions gets the static permissions
func (r *AllStaticReader) GetPermissions() permissions.PermissionSet {
	return r.permissionSet
}

// SetPermissions sets the static permissions
func (r *AllStaticReader) SetPermissions(perms permissions.PermissionSet) {
	r.permissionSet = perms
}

// AddPermission adds a permission to the static set
func (r *AllStaticReader) AddPermission(perm permissions.AccessMode) {
	r.permissionSet.Add(perm)
}

// RemovePermission removes a permission from the static set
func (r *AllStaticReader) RemovePermission(perm permissions.AccessMode) {
	r.permissionSet.Remove(perm)
}

// HasPermission checks if a permission is in the static set
func (r *AllStaticReader) HasPermission(perm permissions.AccessMode) bool {
	return r.permissionSet.Has(perm)
}

// ClearPermissions clears all permissions from the static set
func (r *AllStaticReader) ClearPermissions() {
	r.permissionSet.Clear()
}
