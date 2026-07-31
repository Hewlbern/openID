//go:build cssstub

// Package authorization provides utility functions for Access Control Policies.
package authorization

import (
	"path/filepath"
	"strings"
)

// ACPUtil provides utility functions for working with Access Control Policies.
type ACPUtil struct{}

// NewACPUtil creates a new ACPUtil.
func NewACPUtil() *ACPUtil {
	return &ACPUtil{}
}

// IsACPPath checks if a path is an ACP file.
func (u *ACPUtil) IsACPPath(path string) bool {
	return filepath.Base(path) == ".acp"
}

// GetACPResourcePath returns the path to the ACP resource for a given path.
func (u *ACPUtil) GetACPResourcePath(path string) string {
	return filepath.Join(filepath.Dir(path), "acp")
}

// GetACPPath returns the path to the ACP file for a given resource.
func (u *ACPUtil) GetACPPath(resource string) string {
	return filepath.Join(filepath.Dir(resource), ".acp")
}

// IsACPResource checks if a path is an ACP resource.
func (u *ACPUtil) IsACPResource(path string) bool {
	return strings.HasSuffix(path, "/acp")
}

// GetACPResourceName returns the name of the ACP resource for a given path.
func (u *ACPUtil) GetACPResourceName(path string) string {
	return filepath.Base(path)
}

// GetACPResourceParent returns the parent path of an ACP resource.
func (u *ACPUtil) GetACPResourceParent(path string) string {
	return filepath.Dir(filepath.Dir(path))
}

// GetACPResourcePathFromParent returns the ACP resource path for a given parent path.
func (u *ACPUtil) GetACPResourcePathFromParent(parent string) string {
	return filepath.Join(parent, "acp")
}

// GetACPPathFromParent returns the ACP file path for a given parent path.
func (u *ACPUtil) GetACPPathFromParent(parent string) string {
	return filepath.Join(parent, ".acp")
}
