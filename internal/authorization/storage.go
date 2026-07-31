//go:build cssstub

// Package authorization provides implementations for authorization and access control.
package authorization

// Storage interface for reading and writing data
type Storage interface {
	// Get retrieves data from storage at the given path
	Get(path string) ([]byte, error)

	// Put stores data in storage at the given path
	Put(path string, data []byte) error

	// Delete removes data from storage at the given path
	Delete(path string) error

	// Exists checks if data exists at the given path
	Exists(path string) bool
}
