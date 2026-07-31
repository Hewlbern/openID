package storage

import (
	"context"
	"os"
	"path/filepath"
)

// Storage defines the interface for storage operations
type Storage interface {
	Get(ctx context.Context, path string) ([]byte, error)
	Put(ctx context.Context, path string, data []byte) error
	Delete(ctx context.Context, path string) error
	List(ctx context.Context, path string) ([]string, error)
	Exists(ctx context.Context, path string) (bool, error)
}

// FileStorage implements Storage using the local filesystem
type FileStorage struct {
	rootPath string
}

func NewFileStorage(rootPath string) (*FileStorage, error) {
	if err := os.MkdirAll(rootPath, 0755); err != nil {
		return nil, err
	}
	return &FileStorage{rootPath: rootPath}, nil
}

func (s *FileStorage) resolve(path string) string {
	return filepath.Join(s.rootPath, filepath.FromSlash(path))
}

func (s *FileStorage) Get(ctx context.Context, path string) ([]byte, error) {
	return os.ReadFile(s.resolve(path))
}

func (s *FileStorage) Put(ctx context.Context, path string, data []byte) error {
	fullPath := s.resolve(path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(fullPath, data, 0644)
}

func (s *FileStorage) Delete(ctx context.Context, path string) error {
	return os.Remove(s.resolve(path))
}

func (s *FileStorage) List(ctx context.Context, path string) ([]string, error) {
	fullPath := s.resolve(path)
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, err
	}
	var resources []string
	for _, entry := range entries {
		relPath := path
		if relPath != "" {
			relPath = filepath.ToSlash(filepath.Join(path, entry.Name()))
		} else {
			relPath = entry.Name()
		}
		if entry.IsDir() {
			relPath += "/"
		}
		resources = append(resources, relPath)
	}
	return resources, nil
}

func (s *FileStorage) Exists(ctx context.Context, path string) (bool, error) {
	_, err := os.Stat(s.resolve(path))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
