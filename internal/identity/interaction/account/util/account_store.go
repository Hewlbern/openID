//go:build cssstub

package util

import (
	"context"
)

const AccountSettingsRememberLogin = "rememberLogin"

type AccountSettings map[string]interface{}

type AccountStore interface {
	Create(ctx context.Context) (string, error)
	GetSetting(ctx context.Context, id, setting string) (interface{}, error)
	UpdateSetting(ctx context.Context, id, setting string, value interface{}) error
}
