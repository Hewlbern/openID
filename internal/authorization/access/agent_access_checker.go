//go:build cssstub

// Package access implements access control for individual agents.
package access

import (
	"solid-go/internal/util/vocabularies"
)

// AgentAccessChecker checks if the given WebID has been given access
type AgentAccessChecker struct{}

// NewAgentAccessChecker creates a new AgentAccessChecker
func NewAgentAccessChecker() *AgentAccessChecker {
	return &AgentAccessChecker{}
}

// Handle implements the AccessChecker interface
func (c *AgentAccessChecker) Handle(args AccessCheckerArgs) (bool, error) {
	if args.Credentials.Agent != nil && args.Credentials.Agent.WebID != "" {
		return args.ACL.CountQuads(args.Rule, vocabularies.ACL.Agent, args.Credentials.Agent.WebID, nil) != 0, nil
	}
	return false, nil
}
