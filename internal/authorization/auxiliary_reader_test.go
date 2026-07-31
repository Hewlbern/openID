//go:build cssstub

package authorization

import (
	"solid-go/internal/authorization/permissions"
	"testing"
)

type mockAuxiliaryReader struct {
	modes map[string]permissions.PermissionSet
	err   error
}

func (m *mockAuxiliaryReader) Read(input PermissionReaderInput) (map[string]permissions.PermissionSet, error) {
	return m.modes, m.err
}

func TestAuxiliaryReader(t *testing.T) {
	tests := []struct {
		name           string
		reader         PermissionReader
		requestedModes map[string]permissions.PermissionSet
		expectedModes  map[string]permissions.PermissionSet
		expectError    bool
	}{
		{
			name: "Auxiliary Resource Access",
			reader: &mockAuxiliaryReader{
				modes: map[string]permissions.PermissionSet{
					"/path/to/.resource.meta": func() permissions.PermissionSet {
						ps := permissions.NewPermissionSet()
						ps.Add(permissions.Read)
						return ps
					}(),
				},
			},
			requestedModes: map[string]permissions.PermissionSet{
				"/path/to/resource": permissions.NewPermissionSet(),
			},
			expectedModes: map[string]permissions.PermissionSet{
				"/path/to/resource": func() permissions.PermissionSet {
					ps := permissions.NewPermissionSet()
					ps.Add(permissions.Read)
					return ps
				}(),
			},
			expectError: false,
		},
		{
			name: "No Auxiliary Resources",
			reader: &mockAuxiliaryReader{
				modes: map[string]permissions.PermissionSet{},
				err:   nil,
			},
			requestedModes: map[string]permissions.PermissionSet{
				"/path/to/resource": permissions.NewPermissionSet(),
			},
			expectedModes: map[string]permissions.PermissionSet{},
			expectError:   false,
		},
		{
			name: "Multiple Permissions",
			reader: &mockAuxiliaryReader{
				modes: map[string]permissions.PermissionSet{
					"/path/to/.resource.meta": func() permissions.PermissionSet {
						ps := permissions.NewPermissionSet()
						ps.Add(permissions.Read)
						ps.Add(permissions.Write)
						ps.Add(permissions.Append)
						return ps
					}(),
					"/path/to/.resource.acl": func() permissions.PermissionSet {
						ps := permissions.NewPermissionSet()
						ps.Add(permissions.Read)
						return ps
					}(),
				},
			},
			requestedModes: map[string]permissions.PermissionSet{
				"/path/to/resource": permissions.NewPermissionSet(),
			},
			expectedModes: map[string]permissions.PermissionSet{
				"/path/to/resource": func() permissions.PermissionSet {
					ps := permissions.NewPermissionSet()
					ps.Add(permissions.Read)
					ps.Add(permissions.Write)
					ps.Add(permissions.Append)
					return ps
				}(),
			},
			expectError: false,
		},
		{
			name: "Error Reading Auxiliary Resource",
			reader: &mockAuxiliaryReader{
				modes: nil,
				err:   nil,
			},
			requestedModes: map[string]permissions.PermissionSet{
				"/path/to/resource": permissions.NewPermissionSet(),
			},
			expectedModes: map[string]permissions.PermissionSet{},
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := NewAuxiliaryReader(tt.reader)
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
