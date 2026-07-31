//go:build cssstub

// Package authorization provides implementations for reading permissions from parent containers.
package authorization

import (
	"path/filepath"

	"solid-go/internal/authorization/permissions"
)

// ParentContainerReader reads permissions from parent containers.
// It determines create and delete permissions for resources that need it
// by making sure the parent container has the required permissions.
//
// Create requires append permissions on the parent container.
// Delete requires write permissions on both the parent container and the resource itself.
type ParentContainerReader struct {
	reader PermissionReader
}

// NewParentContainerReader creates a new ParentContainerReader with the given reader.
func NewParentContainerReader(reader PermissionReader) *ParentContainerReader {
	return &ParentContainerReader{
		reader: reader,
	}
}

// Read implements PermissionReader.
// It reads permissions from the parent container of the given resource.
func (r *ParentContainerReader) Read(input PermissionReaderInput) (map[string]permissions.PermissionSet, error) {
	// Find the entries for which we require parent container permissions
	containerMap := r.findParents(input.RequestedModes)

	// Merge the necessary parent container modes with the already requested modes
	combinedModes := make(map[string]permissions.PermissionSet)
	for resource, modes := range input.RequestedModes {
		combinedModes[resource] = modes
	}
	for resource, entry := range containerMap {
		container, modes := entry.container, entry.modes
		if existing, ok := combinedModes[container]; ok {
			// Merge with existing modes
			for mode := range modes {
				existing.Add(mode)
			}
		} else {
			combinedModes[container] = modes
		}
	}

	// Read permissions for all resources
	input.RequestedModes = combinedModes
	result, err := r.reader.Read(input)
	if err != nil {
		return nil, err
	}

	// Update create/delete permissions based on parent container permissions
	for resource, entry := range containerMap {
		container := entry.container
		resourceSet := result[resource]
		containerSet := result[container]
		result[resource] = r.addContainerPermissions(resourceSet, containerSet)
	}

	return result, nil
}

// containerEntry represents a parent container and its required modes
type containerEntry struct {
	container string
	modes     permissions.PermissionSet
}

// findParents finds the identifiers for which we need parent permissions.
// Returns a map of resource identifiers to their parent container and required modes.
func (r *ParentContainerReader) findParents(requestedModes map[string]permissions.PermissionSet) map[string]containerEntry {
	containerMap := make(map[string]containerEntry)
	for resource, modes := range requestedModes {
		if modes.Has(permissions.Create) || modes.Has(permissions.Delete) {
			container := filepath.Dir(resource)
			containerMap[resource] = containerEntry{
				container: container,
				modes:     r.getParentModes(modes),
			}
		}
	}
	return containerMap
}

// getParentModes determines which permissions are required on the parent container.
func (r *ParentContainerReader) getParentModes(modes permissions.PermissionSet) permissions.PermissionSet {
	containerModes := permissions.NewPermissionSet()
	if modes.Has(permissions.Create) {
		containerModes.Add(permissions.Append)
	}
	if modes.Has(permissions.Delete) {
		containerModes.Add(permissions.Write)
	}
	return containerModes
}

// addContainerPermissions merges the container permission set into the resource permission set
// based on the parent container rules for create/delete permissions.
func (r *ParentContainerReader) addContainerPermissions(resourceSet, containerSet permissions.PermissionSet) permissions.PermissionSet {
	if resourceSet == nil {
		resourceSet = permissions.NewPermissionSet()
	}
	if containerSet == nil {
		containerSet = permissions.NewPermissionSet()
	}

	return r.interpretContainerPermission(resourceSet, containerSet)
}

// interpretContainerPermission determines the create and delete permissions for the given resource permissions
// based on those of its parent container.
func (r *ParentContainerReader) interpretContainerPermission(resourcePermission, containerPermission permissions.PermissionSet) permissions.PermissionSet {
	mergedPermission := permissions.NewPermissionSet()

	// Copy existing permissions
	for mode := range resourcePermission {
		mergedPermission[mode] = resourcePermission[mode]
	}

	// When an operation requests to create a resource as a member of a container resource,
	// the server MUST match an Authorization allowing the acl:Append or acl:Write access privilege
	// on the container for new members.
	if containerPermission.Has(permissions.Append) && !resourcePermission.Has(permissions.Create) {
		mergedPermission.Add(permissions.Create)
	}

	// When an operation requests to delete a resource,
	// the server MUST match Authorizations allowing the acl:Write access privilege
	// on the resource and the containing container.
	if resourcePermission.Has(permissions.Write) && containerPermission.Has(permissions.Write) && !resourcePermission.Has(permissions.Delete) {
		mergedPermission.Add(permissions.Delete)
	}

	return mergedPermission
}

// GetReader returns the underlying permission reader.
func (r *ParentContainerReader) GetReader() PermissionReader {
	return r.reader
}

// SetReader sets the underlying permission reader.
func (r *ParentContainerReader) SetReader(reader PermissionReader) {
	r.reader = reader
}
