//go:build cssstub

package configuration

type OidcProvider interface{}

type ProviderFactory interface {
	GetProvider() (OidcProvider, error)
}
