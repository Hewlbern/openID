//go:build cssstub

package util

var AccountStorageDescription = map[string]string{
	AccountSettingsRememberLogin: "boolean?",
}

type BaseAccountStore struct {
	*GenericAccountStore
}

type AccountLoginStorage interface {
	// Add methods as needed
}

func NewBaseAccountStore(storage AccountLoginStorage) *BaseAccountStore {
	return &BaseAccountStore{
		GenericAccountStore: NewGenericAccountStore(storage, AccountStorageDescription),
	}
}
