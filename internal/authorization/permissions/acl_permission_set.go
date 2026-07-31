//go:build cssstub

// Package permissions provides types and utilities for handling authorization permissions.
package permissions

// AclMode represents WebACL-specific modes.
type AclMode string

const (
	Control AclMode = "control"
)

// AclPermissionSet extends PermissionSet to include WebACL-specific modes.
type AclPermissionSet struct {
	PermissionSet
	Control bool
}
