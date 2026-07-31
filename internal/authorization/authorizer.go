//go:build cssstub

package authorization

import (
	"context"
	"errors"
	"net/http"
)

// Permission represents an access permission
type Permission struct {
	Read    bool
	Write   bool
	Append  bool
	Control bool
}

// AccessChecker checks if an agent has access to a resource
type AccessChecker interface {
	CheckAccess(ctx context.Context, agent string, resource string, permission Permission) error
}

// AgentAccessChecker implements basic agent-based access control
type AgentAccessChecker struct {
	// In a real implementation, this would store agent permissions
	permissions map[string]map[string]Permission
}

// NewAgentAccessChecker creates a new agent access checker
func NewAgentAccessChecker() *AgentAccessChecker {
	return &AgentAccessChecker{
		permissions: make(map[string]map[string]Permission),
	}
}

// CheckAccess implements AccessChecker
func (c *AgentAccessChecker) CheckAccess(ctx context.Context, agent string, resource string, permission Permission) error {
	// In a real implementation, this would check the actual permissions
	// For now, we'll just return an error if no permissions are found
	if _, exists := c.permissions[agent]; !exists {
		return errors.New("unauthorized")
	}
	return nil
}

// Authorizer handles authorization decisions
type Authorizer interface {
	Authorize(ctx context.Context, r *http.Request) error
}

// Errors
var (
	ErrUnauthorized = &AuthorizationError{msg: "unauthorized"}
	ErrForbidden    = &AuthorizationError{msg: "forbidden"}
)

// AuthorizationError represents an authorization error
type AuthorizationError struct {
	msg string
}

func (e *AuthorizationError) Error() string {
	return e.msg
}
