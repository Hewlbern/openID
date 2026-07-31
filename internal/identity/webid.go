//go:build cssstub

package identity

// WebID represents a WebID profile
type WebID struct {
	URI      string
	Name     string
	Email    string
	Picture  string
	Accounts []Account
}

// Account represents a linked account
type Account struct {
	Provider string
	URI      string
}

// WebIDStore manages WebID profiles
type WebIDStore struct {
	profiles map[string]*WebID
}

// NewWebIDStore creates a new WebIDStore
func NewWebIDStore() *WebIDStore {
	return &WebIDStore{
		profiles: make(map[string]*WebID),
	}
}

// Get retrieves a WebID profile
func (s *WebIDStore) Get(uri string) (*WebID, error) {
	if profile, ok := s.profiles[uri]; ok {
		return profile, nil
	}
	return nil, nil
}

// Put stores a WebID profile
func (s *WebIDStore) Put(profile *WebID) error {
	s.profiles[profile.URI] = profile
	return nil
}

// Delete removes a WebID profile
func (s *WebIDStore) Delete(uri string) error {
	delete(s.profiles, uri)
	return nil
}
