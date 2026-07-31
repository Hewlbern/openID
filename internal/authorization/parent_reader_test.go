//go:build cssstub

package authorization

import (
	"solid-go/internal/authorization/permissions"
	"testing"
)

type mockParentReader struct {
	modes map[string]permissions.PermissionSet
	err   error
}

func (m *mockParentReader) Read(input PermissionReaderInput) (map[string]permissions.PermissionSet, error) {
	return m.modes, m.err
}

func TestParentContainerReader(t *testing.T) {
	tests := []struct {
		name           string
		reader         PermissionReader
		requestedModes map[string]permissions.PermissionSet
		expectedModes  map[string]permissions.PermissionSet
		expectError    bool
	}{
		{
			name: "Parent Container Access",
			reader: &mockParentReader{
				modes: map[string]permissions.PermissionSet{
					"https://example.org/container/": func() permissions.PermissionSet {
						ps := permissions.NewPermissionSet()
						ps.Add(permissions.Read)
						return ps
					}(),
				},
			},
			requestedModes: map[string]permissions.PermissionSet{
				"https://example.org/container/resource": permissions.NewPermissionSet(),
			},
			expectedModes: map[string]permissions.PermissionSet{
				"https://example.org/container/resource": func() permissions.PermissionSet {
					ps := permissions.NewPermissionSet()
					ps.Add(permissions.Read)
					return ps
				}(),
			},
			expectError: false,
		},
		{
			name: "No Parent Container",
			reader: &mockParentReader{
				modes: map[string]permissions.PermissionSet{},
			},
			requestedModes: map[string]permissions.PermissionSet{
				"https://example.org/resource": permissions.NewPermissionSet(),
			},
			expectedModes: map[string]permissions.PermissionSet{},
			expectError:   false,
		},
		{
			name: "Multiple Resources",
			reader: &mockParentReader{
				modes: map[string]permissions.PermissionSet{
					"https://example.org/container1/": func() permissions.PermissionSet {
						ps := permissions.NewPermissionSet()
						ps.Add(permissions.Read)
						return ps
					}(),
					"https://example.org/container2/": func() permissions.PermissionSet {
						ps := permissions.NewPermissionSet()
						ps.Add(permissions.Write)
						return ps
					}(),
				},
			},
			requestedModes: map[string]permissions.PermissionSet{
				"https://example.org/container1/resource1": permissions.NewPermissionSet(),
				"https://example.org/container2/resource2": permissions.NewPermissionSet(),
			},
			expectedModes: map[string]permissions.PermissionSet{
				"https://example.org/container1/resource1": func() permissions.PermissionSet {
					ps := permissions.NewPermissionSet()
					ps.Add(permissions.Read)
					return ps
				}(),
				"https://example.org/container2/resource2": func() permissions.PermissionSet {
					ps := permissions.NewPermissionSet()
					ps.Add(permissions.Write)
					return ps
				}(),
			},
			expectError: false,
		},
		{
			name: "Nested Containers",
			reader: &mockParentReader{
				modes: map[string]permissions.PermissionSet{
					"https://example.org/container/": func() permissions.PermissionSet {
						ps := permissions.NewPermissionSet()
						ps.Add(permissions.Read)
						return ps
					}(),
					"https://example.org/container/nested/": func() permissions.PermissionSet {
						ps := permissions.NewPermissionSet()
						ps.Add(permissions.Write)
						return ps
					}(),
				},
			},
			requestedModes: map[string]permissions.PermissionSet{
				"https://example.org/container/nested/resource": permissions.NewPermissionSet(),
			},
			expectedModes: map[string]permissions.PermissionSet{
				"https://example.org/container/nested/resource": func() permissions.PermissionSet {
					ps := permissions.NewPermissionSet()
					ps.Add(permissions.Read)
					ps.Add(permissions.Write)
					return ps
				}(),
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := NewParentContainerReader(tt.reader)
			input := PermissionReaderInput{
				RequestedModes: tt.requestedModes,
			}

			result, err := reader.Read(input)
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

			// Compare results
			for resource, expectedModes := range tt.expectedModes {
				actualModes, exists := result[resource]
				if !exists {
					t.Errorf("Read() missing resource %v", resource)
					continue
				}

				// Compare each permission
				for _, mode := range []permissions.AccessMode{
					permissions.Read,
					permissions.Write,
					permissions.Append,
					permissions.Create,
					permissions.Delete,
				} {
					expected := expectedModes.Has(mode)
					actual := actualModes.Has(mode)
					if expected != actual {
						t.Errorf("Read() permission %v for resource %v = %v, want %v",
							mode, resource, actual, expected)
					}
				}
			}
		})
	}
}
