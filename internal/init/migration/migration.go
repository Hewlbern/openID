//go:build cssstub

package migration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
)

// Migration represents a migration
type Migration interface {
	// Run runs the migration
	Run(ctx context.Context) error
}

// V6MigrationInitializer initializes v6 migrations
type V6MigrationInitializer struct {
	storage Storage
}

// NewV6MigrationInitializer creates a new V6MigrationInitializer
func NewV6MigrationInitializer(storage Storage) *V6MigrationInitializer {
	return &V6MigrationInitializer{
		storage: storage,
	}
}

// Initialize implements Initializer.Initialize
func (i *V6MigrationInitializer) Initialize(ctx context.Context) error {
	// TODO: Implement v6 migration
	return nil
}

// Storage represents a storage backend
type Storage interface {
	// Get gets a value from storage
	Get(ctx context.Context, key string) ([]byte, error)
	// Set sets a value in storage
	Set(ctx context.Context, key string, value []byte) error
	// Delete deletes a value from storage
	Delete(ctx context.Context, key string) error
}

// SingleContainerJSONStorage is a storage implementation that uses a single JSON file
type SingleContainerJSONStorage struct {
	path string
}

// NewSingleContainerJSONStorage creates a new SingleContainerJSONStorage
func NewSingleContainerJSONStorage(path string) *SingleContainerJSONStorage {
	return &SingleContainerJSONStorage{
		path: path,
	}
}

// Get implements Storage.Get
func (s *SingleContainerJSONStorage) Get(ctx context.Context, key string) ([]byte, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var store map[string]interface{}
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, err
	}

	if value, ok := store[key]; ok {
		return json.Marshal(value)
	}
	return nil, nil
}

// Set implements Storage.Set
func (s *SingleContainerJSONStorage) Set(ctx context.Context, key string, value []byte) error {
	var store map[string]interface{}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		store = make(map[string]interface{})
	} else {
		if err := json.Unmarshal(data, &store); err != nil {
			return err
		}
	}

	var valueInterface interface{}
	if err := json.Unmarshal(value, &valueInterface); err != nil {
		return err
	}
	store[key] = valueInterface

	data, err = json.Marshal(store)
	if err != nil {
		return err
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(s.path, data, 0644)
}

// Delete implements Storage.Delete
func (s *SingleContainerJSONStorage) Delete(ctx context.Context, key string) error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var store map[string]interface{}
	if err := json.Unmarshal(data, &store); err != nil {
		return err
	}

	delete(store, key)

	data, err = json.Marshal(store)
	if err != nil {
		return err
	}

	return os.WriteFile(s.path, data, 0644)
}
