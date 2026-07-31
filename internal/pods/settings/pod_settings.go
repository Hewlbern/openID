//go:build cssstub

package settings

import (
	"solid-go/internal/http/representation"
)

// PodSettings contains metadata related to pod generation
type PodSettings struct {
	// Base is the root of the pod. Determines where the pod will be created.
	Base representation.ResourceIdentifier
	// WebID is the WebID of the owner of this pod.
	WebID string
	// Template is required for dynamic pod configuration.
	// Indicates the name of the config to use for the pod.
	Template string
	// Name is the name of the owner. Used in provisioning templates.
	Name string
	// Email is the email of the owner. Used in provisioning templates.
	Email string
	// OIDCIssuer is the OIDC issuer of the owner's WebID.
	// Necessary if the WebID in the pod is registered with the IDP.
	OIDCIssuer string
}

// NewPodSettings creates a new PodSettings
func NewPodSettings(base representation.ResourceIdentifier, webID string) *PodSettings {
	return &PodSettings{
		Base:  base,
		WebID: webID,
	}
}

// WithTemplate sets the template for the pod
func (s *PodSettings) WithTemplate(template string) *PodSettings {
	s.Template = template
	return s
}

// WithName sets the name of the owner
func (s *PodSettings) WithName(name string) *PodSettings {
	s.Name = name
	return s
}

// WithEmail sets the email of the owner
func (s *PodSettings) WithEmail(email string) *PodSettings {
	s.Email = email
	return s
}

// WithOIDCIssuer sets the OIDC issuer
func (s *PodSettings) WithOIDCIssuer(issuer string) *PodSettings {
	s.OIDCIssuer = issuer
	return s
}
