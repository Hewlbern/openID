//go:build cssstub

package util

import (
	"context"
)

type ClientCredentials struct {
	Id        string
	Label     string
	WebId     string
	AccountId string
	Secret    string
}

type ClientCredentialsStore interface {
	Get(ctx context.Context, id string) (*ClientCredentials, error)
	FindByLabel(ctx context.Context, label string) (*ClientCredentials, error)
	FindByAccount(ctx context.Context, accountId string) ([]ClientCredentials, error)
	Create(ctx context.Context, label, webId, accountId string) (*ClientCredentials, error)
	Delete(ctx context.Context, id string) error
}
