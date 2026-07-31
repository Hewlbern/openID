//go:build cssstub

// Package authorization provides implementations for WebACL-based permission reading.
package authorization

import (
	"context"
	"encoding/json"
	"path/filepath"

	"solid-go/internal/authorization/permissions"
)

// Storage interface for reading ACL data
type Storage interface {
	Get(ctx context.Context, path string) ([]byte, error)
}

// WebACL represents a WebACL document
type WebACL struct {
	AccessTo      []string `json:"accessTo"`
	Default       []string `json:"default,omitempty"`
	AccessToClass []string `json:"accessToClass,omitempty"`
	Mode          []string `json:"mode"`
	Agent         []string `json:"agent,omitempty"`
	AgentClass    []string `json:"agentClass,omitempty"`
	AgentGroup    []string `json:"agentGroup,omitempty"`
}

// WebACLReader reads permissions from WebACL files.
// It parses WebACL documents to determine permissions for resources.
type WebACLReader struct {
	storage Storage
}

// NewWebACLReader creates a new WebACLReader with the given storage.
func NewWebACLReader(storage Storage) *WebACLReader {
	return &WebACLReader{
		storage: storage,
	}
}

// Read implements PermissionReader.
// It reads and parses the WebACL file for the given resource.
func (r *WebACLReader) Read(ctx context.Context, resource string) (permissions.PermissionSet, error) {
	// Get the ACL file path
	aclPath := r.getACLPath(resource)

	// Read the ACL file
	data, err := r.storage.Get(ctx, aclPath)
	if err != nil {
		return permissions.NewACLPermissionSet(), err
	}

	// Parse the ACL document
	var acl WebACL
	if err := json.Unmarshal(data, &acl); err != nil {
		return permissions.NewACLPermissionSet(), err
	}

	// Create a new permission set
	perms := permissions.NewACLPermissionSet()

	// Add permissions based on the ACL document
	for _, mode := range acl.Mode {
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

	return perms, nil
}

// getACLPath returns the path to the ACL file for a resource.
func (r *WebACLReader) getACLPath(resource string) string {
	return filepath.Join(filepath.Dir(resource), ".acl")
}

// GetPermissions gets permissions for a resource and agent
func (r *WebACLReader) GetPermissions(ctx context.Context, resource, agent string) (permissions.PermissionSet, error) {
	// Read ACL document
	acl, err := r.Read(ctx, resource)
	if err != nil {
		return nil, err
	}

	// Create permission set
	perms := permissions.NewACLPermissionSet()

	// Check if agent has access
	if !contains(acl.Agent, agent) {
		return perms, nil
	}

	// Add permissions based on modes
	for _, mode := range acl.Mode {
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

	return perms, nil
}

// Helper function to check if a string is in a slice
func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}
