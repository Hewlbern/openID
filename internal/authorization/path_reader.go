//go:build cssstub

// Package authorization provides implementations for path-based permission reading.
package authorization

import (
	"regexp"
	"strings"

	"solid-go/internal/authorization/permissions"
)

// PathBasedReader redirects requests to specific PermissionReaders based on their identifier.
// The keys are regular expression strings.
// The regular expressions should all start with a slash
// and will be evaluated relative to the base URL.
//
// Will error if no match is found.
type PathBasedReader struct {
	baseURL string
	paths   map[*regexp.Regexp]PermissionReader
}

// NewPathBasedReader creates a new PathBasedReader.
func NewPathBasedReader(baseURL string, paths map[string]PermissionReader) *PathBasedReader {
	// Ensure baseURL ends with a slash
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	// Convert string patterns to compiled regexps
	regexPaths := make(map[*regexp.Regexp]PermissionReader)
	for pattern, reader := range paths {
		regexPaths[regexp.MustCompile(pattern)] = reader
	}

	return &PathBasedReader{
		baseURL: baseURL,
		paths:   regexPaths,
	}
}

// Read implements PermissionReader.
// It redirects requests to specific PermissionReaders based on their identifier.
func (r *PathBasedReader) Read(input PermissionReaderInput) (map[string]permissions.PermissionSet, error) {
	// Match readers to requested modes
	readerModes := r.matchReaders(input.RequestedModes)

	// Read permissions from each matched reader
	result := make(map[string]permissions.PermissionSet)
	for reader, modes := range readerModes {
		// Create a new input for this reader
		readerInput := PermissionReaderInput{
			Credentials:    input.Credentials,
			RequestedModes: modes,
		}

		// Read permissions from this reader
		readerResult, err := reader.Read(readerInput)
		if err != nil {
			continue // Skip this reader if it returns an error
		}

		// Merge results
		for resource, perms := range readerResult {
			result[resource] = perms
		}
	}

	return result, nil
}

// matchReaders returns for each reader the matching part of the access map.
func (r *PathBasedReader) matchReaders(accessMap map[string]permissions.PermissionSet) map[PermissionReader]map[string]permissions.PermissionSet {
	result := make(map[PermissionReader]map[string]permissions.PermissionSet)
	for resource, modes := range accessMap {
		if reader := r.findReader(resource); reader != nil {
			// Get or create the modes map for this reader
			readerModes, ok := result[reader]
			if !ok {
				readerModes = make(map[string]permissions.PermissionSet)
				result[reader] = readerModes
			}
			readerModes[resource] = modes
		}
	}
	return result
}

// findReader finds the PermissionReader corresponding to the given path.
func (r *PathBasedReader) findReader(path string) PermissionReader {
	if strings.HasPrefix(path, r.baseURL) {
		// We want to keep the leading slash
		relative := strings.TrimPrefix(path, strings.TrimSuffix(r.baseURL, "/"))
		for regex, reader := range r.paths {
			if regex.MatchString(relative) {
				return reader
			}
		}
	}
	return nil
}

// GetBaseURL returns the base URL of this reader.
func (r *PathBasedReader) GetBaseURL() string {
	return r.baseURL
}

// GetPaths returns all path patterns and their readers.
func (r *PathBasedReader) GetPaths() map[string]PermissionReader {
	result := make(map[string]PermissionReader)
	for regex, reader := range r.paths {
		result[regex.String()] = reader
	}
	return result
}

// AddPath adds a new path pattern and reader.
func (r *PathBasedReader) AddPath(pattern string, reader PermissionReader) {
	r.paths[regexp.MustCompile(pattern)] = reader
}

// RemovePath removes a path pattern and its reader.
func (r *PathBasedReader) RemovePath(pattern string) {
	for regex := range r.paths {
		if regex.String() == pattern {
			delete(r.paths, regex)
			break
		}
	}
}

// ClearPaths removes all path patterns and readers.
func (r *PathBasedReader) ClearPaths() {
	r.paths = make(map[*regexp.Regexp]PermissionReader)
}

// HasPath checks if a path pattern exists.
func (r *PathBasedReader) HasPath(pattern string) bool {
	for regex := range r.paths {
		if regex.String() == pattern {
			return true
		}
	}
	return false
}

// GetPathCount returns the number of path patterns.
func (r *PathBasedReader) GetPathCount() int {
	return len(r.paths)
}

// GetReader returns the reader for a specific path pattern.
func (r *PathBasedReader) GetReader(pattern string) PermissionReader {
	for regex, reader := range r.paths {
		if regex.String() == pattern {
			return reader
		}
	}
	return nil
}
