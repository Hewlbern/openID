//go:build cssstub

// Package access implements access control for agent groups.
package access

import (
	"regexp"
	"solid-go/internal/http/representation"
	"solid-go/internal/util/fetch"
	"solid-go/internal/util/n3"
	"solid-go/internal/util/stream"
	"solid-go/internal/util/vocabularies"
)

// AgentGroupAccessChecker checks if the given WebID belongs to a group that has access
type AgentGroupAccessChecker struct{}

// NewAgentGroupAccessChecker creates a new AgentGroupAccessChecker
func NewAgentGroupAccessChecker() *AgentGroupAccessChecker {
	return &AgentGroupAccessChecker{}
}

// Handle implements the AccessChecker interface
func (c *AgentGroupAccessChecker) Handle(args AccessCheckerArgs) (bool, error) {
	if args.Credentials.Agent != nil && args.Credentials.Agent.WebID != "" {
		groups := args.ACL.GetObjects(args.Rule, vocabularies.ACL.AgentGroup, nil)
		for _, group := range groups {
			if isMember, err := c.isMemberOfGroup(args.Credentials.Agent.WebID, group); err == nil && isMember {
				return true, nil
			}
		}
	}
	return false, nil
}

// isMemberOfGroup checks if the given agent is member of a given vCard group
func (c *AgentGroupAccessChecker) isMemberOfGroup(webID string, group n3.Term) (bool, error) {
	groupDocument := representation.ResourceIdentifier{
		Path: regexp.MustCompile(`^[^#]*`).FindString(group.Value()),
	}

	// Fetch the required vCard group file
	quads, err := c.fetchQuads(groupDocument.Path)
	if err != nil {
		return false, err
	}
	return quads.CountQuads(group, vocabularies.VCARD.HasMember, webID, nil) != 0, nil
}

// fetchQuads fetches quads from the given URL
func (c *AgentGroupAccessChecker) fetchQuads(url string) (n3.Store, error) {
	representation, err := fetch.FetchDataset(url)
	if err != nil {
		return nil, err
	}
	return stream.ReadableToQuads(representation.Data), nil
}
