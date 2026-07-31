//go:build cssstub

package authorization

import (
	"solid-go/internal/authorization/permissions"
	"testing"
)

func TestAllStaticReader(t *testing.T) {
	tests := []struct {
		name           string
		allow          bool
		requestedModes map[string]permissions.PermissionSet
		expectedModes  map[string]permissions.PermissionSet
	}{
		{
			name:  "Allow All Permissions",
			allow: true,
			requestedModes: map[string]permissions.PermissionSet{
				"https://example.org/resource": permissions.NewPermissionSet(),
			},
			expectedModes: map[string]permissions.PermissionSet{
				"https://example.org/resource": func() permissions.PermissionSet {
					ps := permissions.NewPermissionSet()
					ps.Add(permissions.Read)
					ps.Add(permissions.Write)
					ps.Add(permissions.Append)
					ps.Add(permissions.Create)
					ps.Add(permissions.Delete)
					return ps
				}(),
			},
		},
		{
			name:  "Deny All Permissions",
			allow: false,
			requestedModes: map[string]permissions.PermissionSet{
				"https://example.org/resource": permissions.NewPermissionSet(),
			},
			expectedModes: map[string]permissions.PermissionSet{
				"https://example.org/resource": permissions.NewPermissionSet(),
			},
		},
		{
			name:  "Multiple Resources",
			allow: true,
			requestedModes: map[string]permissions.PermissionSet{
				"https://example.org/resource1": permissions.NewPermissionSet(),
				"https://example.org/resource2": permissions.NewPermissionSet(),
			},
			expectedModes: map[string]permissions.PermissionSet{
				"https://example.org/resource1": func() permissions.PermissionSet {
					ps := permissions.NewPermissionSet()
					ps.Add(permissions.Read)
					ps.Add(permissions.Write)
					ps.Add(permissions.Append)
					ps.Add(permissions.Create)
					ps.Add(permissions.Delete)
					return ps
				}(),
				"https://example.org/resource2": func() permissions.PermissionSet {
					ps := permissions.NewPermissionSet()
					ps.Add(permissions.Read)
					ps.Add(permissions.Write)
					ps.Add(permissions.Append)
					ps.Add(permissions.Create)
					ps.Add(permissions.Delete)
					return ps
				}(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := NewAllStaticReader(tt.allow)
			input := PermissionReaderInput{
				RequestedModes: tt.requestedModes,
			}

			result, err := reader.Read(input)
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
