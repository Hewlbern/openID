//go:build cssstub

package util

const AccountType = "account"

type IndexedStorage interface {
	// Add methods as needed
}

type LoginStorage interface {
	DefineType(typeName, description string, isLogin bool) error
	// Add other IndexedStorage methods as needed
}
