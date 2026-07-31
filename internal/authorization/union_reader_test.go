//go:build cssstub

package authorization

import (
	"fmt"
	"solid-go/internal/authorization/permissions"
	"testing"
)

type mockUnionReader struct {
	modes map[string]permissions.PermissionSet
	err   error
}

func (m *mockUnionReader) Read(input PermissionReaderInput) (map[string]permissions.PermissionSet, error) {
	return m.modes, m.err
}

func TestUnionPermissionReader(t *testing.T) {
	tests := []struct {
		name           string
		readers        []PermissionReader
		requestedModes map[string]permissions.PermissionSet
		expectedModes  map[string]permissions.PermissionSet
		expectError    bool
	}{
		{
			name: "Combine Results",
			readers: []PermissionReader{
				&mockUnionReader{
					modes: map[string]permissions.PermissionSet{
						"https://example.org/resource": func() permissions.PermissionSet {
							ps := permissions.NewPermissionSet()
							ps.Add(permissions.Read)
							return ps
						}(),
					},
				},
				&mockUnionReader{
					modes: map[string]permissions.PermissionSet{
						"https://example.org/resource": func() permissions.PermissionSet {
							ps := permissions.NewPermissionSet()
							ps.Add(permissions.Write)
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
					ps.Add(permissions.Write)
					return ps
				}(),
			},
			expectError: false,
		},
		{
			name: "False Overrides True",
			readers: []PermissionReader{
				&mockUnionReader{
					modes: map[string]permissions.PermissionSet{
						"https://example.org/resource": func() permissions.PermissionSet {
							ps := permissions.NewPermissionSet()
							ps.Add(permissions.Read)
							return ps
						}(),
					},
				},
				&mockUnionReader{
					modes: map[string]permissions.PermissionSet{
						"https://example.org/resource": func() permissions.PermissionSet {
							ps := permissions.NewPermissionSet()
							ps.Remove(permissions.Read)
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
					ps.Remove(permissions.Read)
					return ps
				}(),
			},
			expectError: false,
		},
		{
			name: "Skip Erroring Readers",
			readers: []PermissionReader{
				&mockUnionReader{
					modes: nil,
					err:   fmt.Errorf("error"),
				},
				&mockUnionReader{
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
			name: "Multiple Resources",
			readers: []PermissionReader{
				&mockUnionReader{
					modes: map[string]permissions.PermissionSet{
						"https://example.org/resource1": func() permissions.PermissionSet {
							ps := permissions.NewPermissionSet()
							ps.Add(permissions.Read)
							return ps
						}(),
					},
				},
				&mockUnionReader{
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := NewUnionPermissionReader(tt.readers...)
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
