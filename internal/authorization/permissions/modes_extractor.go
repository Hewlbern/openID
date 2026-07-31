//go:build cssstub

// Package permissions provides types and utilities for handling authorization permissions.
package permissions

import (
	"net/http"
)

// ModesExtractor extracts all AccessModes necessary to execute a given Operation.
type ModesExtractor interface {
	Extract(r *http.Request) (AccessMap, error)
}
