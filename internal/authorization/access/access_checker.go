//go:build cssstub

// Package access provides the AccessChecker interface for resource access control.
package access

import (
	"solid-go/internal/authentication"
	"solid-go/internal/util/handlers"
	"solid-go/internal/util/n3"
)

// AccessCheckerArgs represents the arguments for an access check
type AccessCheckerArgs struct {
	// ACL contains the relevant triples of the authorization
	ACL n3.Store

	// Rule is the authorization rule to be processed
	Rule n3.Term

	// Credentials of the entity that wants to use the resource
	Credentials *authentication.Credentials
}

// AccessChecker is an interface for performing authorization checks against ACL resources
type AccessChecker interface {
	handlers.AsyncHandler[AccessCheckerArgs, bool]
}
