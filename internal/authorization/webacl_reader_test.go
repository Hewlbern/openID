//go:build cssstub

package authorization

import (
	"context"
	"encoding/json"
	"errors"
	"solid-go/internal/authorization/permissions"
	"testing"
)

type mockStorage struct {
	data map[string][]byte
	err  error
}

func (m *mockStorage) Get(path string) ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	if data, exists := m.data[path]; exists {
		return data, nil
	}
	return nil, errors.New("not found")
}

func (m *mockStorage) Delete(path string) error {
	return nil
}

func (m *mockStorage) Exists(path string) bool {
	_, exists := m.data[path]
	return exists
}

func (m *mockStorage) Put(path string, data []byte) error {
	if m.err != nil {
		return m.err
	}
	m.data[path] = data
	return nil
}

func TestWebACLReader(t *testing.T) {
	tests := []struct {
		name          string
		storage       *mockStorage
		resource      string
		expectedModes permissions.PermissionSet
		expectError   bool
	}{
		{
			name: "Valid ACL File",
			storage: &mockStorage{
				data: map[string][]byte{
					"/path/to/.acl": func() []byte {
						acl := WebACL{
							Mode:  []string{"Read", "Write"},
							Agent: []string{"https://example.org/agent"},
						}
						data, _ := json.Marshal(acl)
						return data
					}(),
				},
			},
			resource: "/path/to/resource",
			expectedModes: func() permissions.PermissionSet {
				ps := permissions.NewPermissionSet()
				ps.Add(permissions.Read)
				ps.Add(permissions.Write)
				return ps
			}(),
			expectError: false,
		},
		{
			name: "Missing ACL File",
			storage: &mockStorage{
				data: map[string][]byte{},
			},
			resource:      "/path/to/resource",
			expectedModes: permissions.NewPermissionSet(),
			expectError:   true,
		},
		{
			name: "Invalid ACL File Content",
			storage: &mockStorage{
				data: map[string][]byte{
					"/path/to/.acl": []byte("invalid json"),
				},
			},
			resource:      "/path/to/resource",
			expectedModes: permissions.NewPermissionSet(),
			expectError:   true,
		},
		{
			name: "All Access Modes",
			storage: &mockStorage{
				data: map[string][]byte{
					"/path/to/.acl": func() []byte {
						acl := WebACL{
							Mode:  []string{"Read", "Write", "Append"},
							Agent: []string{"https://example.org/agent"},
						}
						data, _ := json.Marshal(acl)
						return data
					}(),
				},
			},
			resource: "/path/to/resource",
			expectedModes: func() permissions.PermissionSet {
				ps := permissions.NewPermissionSet()
				ps.Add(permissions.Read)
				ps.Add(permissions.Write)
				ps.Add(permissions.Append)
				return ps
			}(),
			expectError: false,
		},
		{
			name: "Storage Error",
			storage: &mockStorage{
				err: errors.New("storage error"),
			},
			resource:      "/path/to/resource",
			expectedModes: permissions.NewPermissionSet(),
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := NewWebACLReader(tt.storage)
			ctx := context.Background()

			result, err := reader.Read(ctx, tt.resource)
			if tt.expectError {
				if err == nil {
					t.Error("Read() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("Read() error = %v", err)
				return
			}

			// Compare each permission
			for _, mode := range []permissions.AccessMode{
				permissions.Read,
				permissions.Write,
				permissions.Append,
			} {
				expected := tt.expectedModes.Has(mode)
				actual := result.Has(mode)
				if expected != actual {
					t.Errorf("Read() permission %v = %v, want %v",
						mode, actual, expected)
				}
			}
		})
	}
}

func TestWebACLReaderGetPermissions(t *testing.T) {
	tests := []struct {
		name          string
		storage       *mockStorage
		resource      string
		agent         string
		expectedModes permissions.PermissionSet
		expectError   bool
	}{
		{
			name: "Agent Has Access",
			storage: &mockStorage{
				data: map[string][]byte{
					"/path/to/.acl": func() []byte {
						acl := WebACL{
							Mode:  []string{"Read", "Write"},
							Agent: []string{"https://example.org/agent"},
						}
						data, _ := json.Marshal(acl)
						return data
					}(),
				},
			},
			resource: "/path/to/resource",
			agent:    "https://example.org/agent",
			expectedModes: func() permissions.PermissionSet {
				ps := permissions.NewPermissionSet()
				ps.Add(permissions.Read)
				ps.Add(permissions.Write)
				return ps
			}(),
			expectError: false,
		},
		{
			name: "Agent No Access",
			storage: &mockStorage{
				data: map[string][]byte{
					"/path/to/.acl": func() []byte {
						acl := WebACL{
							Mode:  []string{"Read", "Write"},
							Agent: []string{"https://example.org/other-agent"},
						}
						data, _ := json.Marshal(acl)
						return data
					}(),
				},
			},
			resource:      "/path/to/resource",
			agent:         "https://example.org/agent",
			expectedModes: permissions.NewPermissionSet(),
			expectError:   false,
		},
		{
			name: "Missing ACL File",
			storage: &mockStorage{
				data: map[string][]byte{},
			},
			resource:      "/path/to/resource",
			agent:         "https://example.org/agent",
			expectedModes: permissions.NewPermissionSet(),
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := NewWebACLReader(tt.storage)
			ctx := context.Background()

			result, err := reader.GetPermissions(ctx, tt.resource, tt.agent)
			if tt.expectError {
				if err == nil {
					t.Error("GetPermissions() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("GetPermissions() error = %v", err)
				return
			}

			// Compare each permission
			for _, mode := range []permissions.AccessMode{
				permissions.Read,
				permissions.Write,
				permissions.Append,
			} {
				expected := tt.expectedModes.Has(mode)
				actual := result.Has(mode)
				if expected != actual {
					t.Errorf("GetPermissions() permission %v = %v, want %v",
						mode, actual, expected)
				}
			}
		})
	}
}
