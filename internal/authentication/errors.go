//go:build cssstub

// Package authentication provides implementations for authentication and credential management.
package authentication

import "errors"

// Common authentication errors
var (
	ErrNotImplemented = errors.New("not implemented")
	ErrInvalidToken   = errors.New("invalid token")
	ErrMissingToken   = errors.New("missing token")
)
