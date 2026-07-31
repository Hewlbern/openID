//go:build cssstub

package configuration

import (
	"sync"
)

type AsymmetricSigningAlgorithm string

type JWKS struct {
	Keys []AlgJwk
}

type KeyValueStorage interface {
	Get(key string) (*JWKS, error)
	Set(key string, value *JWKS) error
}

type AlgJwk struct {
	Alg AsymmetricSigningAlgorithm
	// Add other JWK fields as needed
}

type JwkGenerator interface {
	GetPrivateKey() (*AlgJwk, error)
	GetPublicKey() (*AlgJwk, error)
}

type CachedJwkGenerator struct {
	alg     AsymmetricSigningAlgorithm
	key     string
	storage KeyValueStorage

	privateJwk *AlgJwk
	publicJwk  *AlgJwk
	mu         sync.Mutex
}

func NewCachedJwkGenerator(alg AsymmetricSigningAlgorithm, storageKey string, storage KeyValueStorage) *CachedJwkGenerator {
	return &CachedJwkGenerator{
		alg:     alg,
		key:     storageKey,
		storage: storage,
	}
}

func (g *CachedJwkGenerator) GetPrivateKey() (*AlgJwk, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.privateJwk != nil {
		return g.privateJwk, nil
	}
	jwks, err := g.storage.Get(g.key)
	if err == nil && jwks != nil && len(jwks.Keys) > 0 {
		g.privateJwk = &jwks.Keys[0]
		return g.privateJwk, nil
	}
	// Generate key pair and store (placeholder)
	privateJwk := &AlgJwk{Alg: g.alg}
	g.storage.Set(g.key, &JWKS{Keys: []AlgJwk{*privateJwk}})
	g.privateJwk = privateJwk
	return privateJwk, nil
}

func (g *CachedJwkGenerator) GetPublicKey() (*AlgJwk, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.publicJwk != nil {
		return g.publicJwk, nil
	}
	privateJwk, err := g.GetPrivateKey()
	if err != nil {
		return nil, err
	}
	// Derive public key from private key (placeholder)
	publicJwk := &AlgJwk{Alg: privateJwk.Alg}
	g.publicJwk = publicJwk
	return publicJwk, nil
}
