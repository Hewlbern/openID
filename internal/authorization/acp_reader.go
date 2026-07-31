//go:build cssstub

// Package authorization provides implementations for reading Access Control Policy permissions.
package authorization

import (
	"context"
	"encoding/json"
	"path/filepath"

	"solid-go/internal/authorization/permissions"
)

// ACP represents an Access Control Policy document
type ACP struct {
	Policy []struct {
		Allow []struct {
			Mode []string `json:"mode"`
		} `json:"allow"`
		Deny []struct {
			Mode []string `json:"mode"`
		} `json:"deny"`
	} `json:"policy"`
}

// ACPReader reads permissions from Access Control Policy files.
// It parses ACP documents to determine permissions for resources.
type ACPReader struct {
	storage Storage
}

// NewACPReader creates a new ACPReader with the given storage.
func NewACPReader(storage Storage) *ACPReader {
	return &ACPReader{
		storage: storage,
	}
}

// Read implements PermissionReader.
// It reads and parses the ACP file for the given resource.
func (r *ACPReader) Read(ctx context.Context, resource string) (permissions.PermissionSet, error) {
	// Get the ACP file path
	acpPath := r.getACPPath(resource)

	// Read the ACP file
	data, err := r.storage.Get(ctx, acpPath)
	if err != nil {
		return permissions.NewACLPermissionSet(), err
	}

	// Parse the ACP document
	var acp ACP
	if err := json.Unmarshal(data, &acp); err != nil {
		return permissions.NewACLPermissionSet(), err
	}

	// Create a new permission set
	perms := permissions.NewACLPermissionSet()

	// Process each policy
	for _, policy := range acp.Policy {
		// Process allow rules
		for _, allow := range policy.Allow {
			for _, mode := range allow.Mode {
				switch mode {
				case "Read":
					perms.Add(permissions.Read)
				case "Write":
					perms.Add(permissions.Write)
				case "Append":
					perms.Add(permissions.Append)
				case "Control":
					perms.Add(permissions.Control)
				}
			}
		}

		// Process deny rules
		for _, deny := range policy.Deny {
			for _, mode := range deny.Mode {
				switch mode {
				case "Read":
					perms.Remove(permissions.Read)
				case "Write":
					perms.Remove(permissions.Write)
				case "Append":
					perms.Remove(permissions.Append)
				case "Control":
					perms.Remove(permissions.Control)
				}
			}
		}
	}

	return perms, nil
}

// getACPPath returns the path to the ACP file for a resource.
func (r *ACPReader) getACPPath(resource string) string {
	return filepath.Join(filepath.Dir(resource), ".acp")
}

// GetAcpPath gets the ACP file path
func (r *ACPReader) GetAcpPath(path string) string {
	return filepath.Join(filepath.Dir(path), ".acp")
}

// IsAcp checks if a path is an ACP file
func (r *ACPReader) IsAcp(path string) bool {
	return filepath.Base(path) == ".acp"
}

// GetAcpResource gets the ACP resource for a path
func (r *ACPReader) GetAcpResource(path string) string {
	return filepath.Join(filepath.Dir(path), "acp")
}
