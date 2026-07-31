//go:build cssstub

// Package permissions provides types and utilities for handling authorization permissions.
package permissions

// AccessMode represents different modes that require permission
type AccessMode string

const (
	// Read permission allows reading a resource
	Read AccessMode = "read"
	// Write permission allows writing to a resource
	Write AccessMode = "write"
	// Append permission allows appending to a resource
	Append AccessMode = "append"
	// Create permission allows creating a resource
	Create AccessMode = "create"
	// Delete permission allows deleting a resource
	Delete AccessMode = "delete"
)

// AccessMap maps identifiers to a set of AccessModes.
type AccessMap map[string]map[AccessMode]struct{}

// PermissionSet represents a set of permissions
// It maps access modes to boolean values indicating whether the permission is granted
type PermissionSet map[AccessMode]bool

// PermissionMap maps identifiers to their PermissionSet.
type PermissionMap map[string]PermissionSet

// NewPermissionSet creates a new empty permission set
func NewPermissionSet() PermissionSet {
	return make(PermissionSet)
}

// Has checks if the permission set has the given permission
func (s PermissionSet) Has(mode AccessMode) bool {
	return s[mode]
}

// Add adds a permission to the set
func (s PermissionSet) Add(mode AccessMode) {
	s[mode] = true
}

// Remove removes a permission from the set
func (s PermissionSet) Remove(mode AccessMode) {
	delete(s, mode)
}

// Clear removes all permissions from the set
func (s PermissionSet) Clear() {
	for k := range s {
		delete(s, k)
	}
}

// GetPermissions returns all permissions in the set
func (s PermissionSet) GetPermissions() []AccessMode {
	var modes []AccessMode
	for mode, granted := range s {
		if granted {
			modes = append(modes, mode)
		}
	}
	return modes
}

// Intersect returns a new permission set containing only the permissions that exist in both sets
func (s PermissionSet) Intersect(other PermissionSet) PermissionSet {
	result := NewPermissionSet()
	for mode := range s {
		if other.Has(mode) {
			result.Add(mode)
		}
	}
	return result
}

// Union returns a new permission set containing all permissions from both sets
func (s PermissionSet) Union(other PermissionSet) PermissionSet {
	result := NewPermissionSet()
	for mode := range s {
		result.Add(mode)
	}
	for mode := range other {
		result.Add(mode)
	}
	return result
}

// Difference returns a new permission set containing permissions that exist in this set but not in the other
func (s PermissionSet) Difference(other PermissionSet) PermissionSet {
	result := NewPermissionSet()
	for mode := range s {
		if !other.Has(mode) {
			result.Add(mode)
		}
	}
	return result
}
