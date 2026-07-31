//go:build cssstub

package authorization

import (
	"solid-go/internal/authentication"
	"solid-go/internal/authorization/permissions"
	"solid-go/internal/identity/interaction/pod"
	"testing"
)

type mockPodStore struct {
	pods   map[string]*pod.Pod
	owners map[string][]pod.Owner
}

func (m *mockPodStore) FindByBaseURL(baseURL string) (*pod.Pod, error) {
	return m.pods[baseURL], nil
}

func (m *mockPodStore) GetOwners(podID string) ([]pod.Owner, error) {
	return m.owners[podID], nil
}

type mockAuthStrategy struct {
	isAuxiliary bool
}

func (m *mockAuthStrategy) IsAuxiliaryIdentifier(identifier string) bool {
	return m.isAuxiliary
}

type mockStorageStrategy struct {
	storageID string
}

func (m *mockStorageStrategy) GetStorageIdentifier(identifier string) (string, error) {
	return m.storageID, nil
}

func TestOwnerPermissionReader(t *testing.T) {
	tests := []struct {
		name            string
		credentials     *authentication.Credentials
		requestedModes  map[string]permissions.PermissionSet
		podStore        *mockPodStore
		authStrategy    *mockAuthStrategy
		storageStrategy *mockStorageStrategy
		expectedModes   map[string]permissions.PermissionSet
		expectError     bool
	}{
		{
			name: "Owner Access",
			credentials: &authentication.Credentials{
				Agent: &authentication.Agent{WebID: "https://example.org/owner"},
			},
			requestedModes: map[string]permissions.PermissionSet{
				"https://example.org/resource.acl": permissions.NewPermissionSet(),
			},
			podStore: &mockPodStore{
				pods: map[string]*pod.Pod{
					"https://example.org/": {
						ID: "pod1",
					},
				},
				owners: map[string][]pod.Owner{
					"pod1": {
						{WebID: "https://example.org/owner"},
					},
				},
			},
			authStrategy: &mockAuthStrategy{
				isAuxiliary: true,
			},
			storageStrategy: &mockStorageStrategy{
				storageID: "https://example.org/",
			},
			expectedModes: map[string]permissions.PermissionSet{
				"https://example.org/resource.acl": func() permissions.PermissionSet {
					ps := permissions.NewPermissionSet()
					ps.Add(permissions.Read)
					ps.Add(permissions.Write)
					ps.Add(permissions.Append)
					ps.Add(permissions.Create)
					ps.Add(permissions.Delete)
					return ps
				}(),
			},
			expectError: false,
		},
		{
			name: "Non-Owner Access",
			credentials: &authentication.Credentials{
				Agent: &authentication.Agent{WebID: "https://example.org/user"},
			},
			requestedModes: map[string]permissions.PermissionSet{
				"https://example.org/resource.acl": permissions.NewPermissionSet(),
			},
			podStore: &mockPodStore{
				pods: map[string]*pod.Pod{
					"https://example.org/": {
						ID: "pod1",
					},
				},
				owners: map[string][]pod.Owner{
					"pod1": {
						{WebID: "https://example.org/owner"},
					},
				},
			},
			authStrategy: &mockAuthStrategy{
				isAuxiliary: true,
			},
			storageStrategy: &mockStorageStrategy{
				storageID: "https://example.org/",
			},
			expectedModes: map[string]permissions.PermissionSet{},
			expectError:   false,
		},
		{
			name:        "No Credentials",
			credentials: nil,
			requestedModes: map[string]permissions.PermissionSet{
				"https://example.org/resource.acl": permissions.NewPermissionSet(),
			},
			podStore: &mockPodStore{
				pods: map[string]*pod.Pod{
					"https://example.org/": {
						ID: "pod1",
					},
				},
				owners: map[string][]pod.Owner{
					"pod1": {
						{WebID: "https://example.org/owner"},
					},
				},
			},
			authStrategy: &mockAuthStrategy{
				isAuxiliary: true,
			},
			storageStrategy: &mockStorageStrategy{
				storageID: "https://example.org/",
			},
			expectedModes: map[string]permissions.PermissionSet{},
			expectError:   false,
		},
		{
			name: "Non-Auxiliary Resource",
			credentials: &authentication.Credentials{
				Agent: &authentication.Agent{WebID: "https://example.org/owner"},
			},
			requestedModes: map[string]permissions.PermissionSet{
				"https://example.org/resource": permissions.NewPermissionSet(),
			},
			podStore: &mockPodStore{
				pods: map[string]*pod.Pod{
					"https://example.org/": {
						ID: "pod1",
					},
				},
				owners: map[string][]pod.Owner{
					"pod1": {
						{WebID: "https://example.org/owner"},
					},
				},
			},
			authStrategy: &mockAuthStrategy{
				isAuxiliary: false,
			},
			storageStrategy: &mockStorageStrategy{
				storageID: "https://example.org/",
			},
			expectedModes: map[string]permissions.PermissionSet{},
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := NewOwnerPermissionReader(
				tt.podStore,
				tt.authStrategy,
				tt.storageStrategy,
			)
			input := PermissionReaderInput{
				Credentials:    tt.credentials,
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
