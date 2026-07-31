//go:build cssstub

// Package permissions provides implementations for extracting required permissions from HTTP requests.
package permissions

// Storage interface for checking resource existence
type Storage interface {
	Exists(path string) bool
}
