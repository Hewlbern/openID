//go:build cssstub

package authorization

import (
	"solid-go/internal/authorization/permissions"
	"testing"
)

type mockReader struct {
	modes map[string]permissions.PermissionSet
	err   error
}

func (m *mockReader) Read(input PermissionReaderInput) (map[string]permissions.PermissionSet, error) {
	return m.modes, m.err
}

func TestPathBasedReader(t *testing.T) {
	tests := []struct {
		name           string
		baseURL        string
		paths          map[string]PermissionReader
		requestedModes map[string]permissions.PermissionSet
		expectedModes  map[string]permissions.PermissionSet
		expectError    bool
	}{
		{
			name:    "Single Path Match",
			baseURL: "https://example.org/",
			paths: map[string]PermissionReader{
				"/resource": &mockReader{
					modes: map[string]permissions.PermissionSet{
						"https://example.org/resource": func() permissions.PermissionSet {
							ps := permissions.NewPermissionSet()
							ps.Add(permissions.Read)
							return ps
						}(),
					},
				},
			},
			requestedModes: map[string]permissions.PermissionSet{
				"https://example.org/resource": permissions.NewPermissionSet(),
			},
			expectedModes: map[string]permissions.PermissionSet{
				"https://example.org/resource": func() permissions.PermissionSet {
					ps := permissions.NewPermissionSet()
					ps.Add(permissions.Read)
					return ps
				}(),
			},
			expectError: false,
		},
		{
			name:    "Multiple Path Matches",
			baseURL: "https://example.org/",
			paths: map[string]PermissionReader{
				"/resource1": &mockReader{
					modes: map[string]permissions.PermissionSet{
						"https://example.org/resource1": func() permissions.PermissionSet {
							ps := permissions.NewPermissionSet()
							ps.Add(permissions.Read)
							return ps
						}(),
					},
				},
				"/resource2": &mockReader{
					modes: map[string]permissions.PermissionSet{
						"https://example.org/resource2": func() permissions.PermissionSet {
							ps := permissions.NewPermissionSet()
							ps.Add(permissions.Write)
							return ps
						}(),
					},
				},
			},
			requestedModes: map[string]permissions.PermissionSet{
				"https://example.org/resource1": permissions.NewPermissionSet(),
				"https://example.org/resource2": permissions.NewPermissionSet(),
			},
			expectedModes: map[string]permissions.PermissionSet{
				"https://example.org/resource1": func() permissions.PermissionSet {
					ps := permissions.NewPermissionSet()
					ps.Add(permissions.Read)
					return ps
				}(),
				"https://example.org/resource2": func() permissions.PermissionSet {
					ps := permissions.NewPermissionSet()
					ps.Add(permissions.Write)
					return ps
				}(),
			},
			expectError: false,
		},
		{
			name:    "No Path Match",
			baseURL: "https://example.org/",
			paths: map[string]PermissionReader{
				"/resource": &mockReader{
					modes: map[string]permissions.PermissionSet{
						"https://example.org/resource": func() permissions.PermissionSet {
							ps := permissions.NewPermissionSet()
							ps.Add(permissions.Read)
							return ps
						}(),
					},
				},
			},
			requestedModes: map[string]permissions.PermissionSet{
				"https://example.org/other": permissions.NewPermissionSet(),
			},
			expectedModes: map[string]permissions.PermissionSet{},
			expectError:   false,
		},
		{
			name:    "Regex Path Match",
			baseURL: "https://example.org/",
			paths: map[string]PermissionReader{
				"/resource/.*": &mockReader{
					modes: map[string]permissions.PermissionSet{
						"https://example.org/resource/123": func() permissions.PermissionSet {
							ps := permissions.NewPermissionSet()
							ps.Add(permissions.Read)
							return ps
						}(),
					},
				},
			},
			requestedModes: map[string]permissions.PermissionSet{
				"https://example.org/resource/123": permissions.NewPermissionSet(),
			},
			expectedModes: map[string]permissions.PermissionSet{
				"https://example.org/resource/123": func() permissions.PermissionSet {
					ps := permissions.NewPermissionSet()
					ps.Add(permissions.Read)
					return ps
				}(),
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := NewPathBasedReader(tt.baseURL, tt.paths)
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
