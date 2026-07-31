//go:build cssstub

package util

import (
	"time"
)

type ExpiringStorage interface {
	Get(key string) (string, error)
	Set(key, value string, ttlMs int) error
	Delete(key string) error
}

type CookieStore interface {
	Generate(accountId string) (string, error)
	Get(cookie string) (string, error)
	Refresh(cookie string) (*time.Time, error)
	Delete(cookie string) error
}

type BaseCookieStore struct {
	storage ExpiringStorage
	ttl     int // milliseconds
}

func NewBaseCookieStore(storage ExpiringStorage, ttlSeconds int) *BaseCookieStore {
	return &BaseCookieStore{
		storage: storage,
		ttl:     ttlSeconds * 1000,
	}
}

func (s *BaseCookieStore) Generate(accountId string) (string, error) {
	cookie := generateUUID() // Placeholder for UUID generation
	err := s.storage.Set(cookie, accountId, s.ttl)
	return cookie, err
}

func (s *BaseCookieStore) Get(cookie string) (string, error) {
	return s.storage.Get(cookie)
}

func (s *BaseCookieStore) Refresh(cookie string) (*time.Time, error) {
	accountId, err := s.storage.Get(cookie)
	if err != nil {
		return nil, err
	}
	if accountId != "" {
		err = s.storage.Set(cookie, accountId, s.ttl)
		if err != nil {
			return nil, err
		}
		expiration := time.Now().Add(time.Duration(s.ttl) * time.Millisecond)
		return &expiration, nil
	}
	return nil, nil
}

func (s *BaseCookieStore) Delete(cookie string) error {
	return s.storage.Delete(cookie)
}

func generateUUID() string {
	// Placeholder for UUID generation
	return "uuid-placeholder"
}
