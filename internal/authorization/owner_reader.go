//go:build cssstub

// Package authorization provides implementations for owner-based permission reading.
package authorization

import (
	"solid-go/internal/authorization/permissions"
	"solid-go/internal/http/auxiliary"
	"solid-go/internal/identity/interaction/pod"
	"solid-go/internal/server/description"
)

// OwnerPermissionReader allows control access if the request is being made by an owner of the pod containing the resource.
type OwnerPermissionReader struct {
	podStore        pod.Store
	authStrategy    auxiliary.IdentifierStrategy
	storageStrategy description.StorageLocationStrategy
}

// NewOwnerPermissionReader creates a new OwnerPermissionReader.
func NewOwnerPermissionReader(
	podStore pod.Store,
	authStrategy auxiliary.IdentifierStrategy,
	storageStrategy description.StorageLocationStrategy,
) *OwnerPermissionReader {
	return &OwnerPermissionReader{
		podStore:        podStore,
		authStrategy:    authStrategy,
		storageStrategy: storageStrategy,
	}
}

// Read implements PermissionReader.
// It grants access if the request is being made by an owner of the pod containing the resource.
func (r *OwnerPermissionReader) Read(input PermissionReaderInput) (map[string]permissions.PermissionSet, error) {
	result := make(map[string]permissions.PermissionSet)

	// Filter for authorization resources
	auths := make([]string, 0)
	for resource := range input.RequestedModes {
		if r.authStrategy.IsAuxiliaryIdentifier(resource) {
			auths = append(auths, resource)
		}
	}

	if len(auths) == 0 {
		return result, nil
	}

	// Get WebID from credentials
	webID := ""
	if input.Credentials != nil {
		webID = input.Credentials.WebID
	}
	if webID == "" {
		return result, nil
	}

	// Find pods and owners
	pods, err := r.findPods(auths)
	if err != nil {
		return result, err
	}

	owners, err := r.findOwners(pods)
	if err != nil {
		return result, err
	}

	// Grant permissions to owners
	for _, auth := range auths {
		webIDs := owners[pods[auth]]
		if len(webIDs) == 0 {
			continue
		}

		// Check if the user is an owner
		for _, ownerWebID := range webIDs {
			if ownerWebID == webID {
				perms := permissions.NewPermissionSet()
				perms.Add(permissions.Read)
				perms.Add(permissions.Write)
				perms.Add(permissions.Append)
				perms.Add(permissions.Create)
				perms.Add(permissions.Delete)
				result[auth] = perms
				break
			}
		}
	}

	return result, nil
}

// findPods finds all pods that contain the given identifiers.
// Return value is a map where the keys are the identifiers and the values the associated pod.
func (r *OwnerPermissionReader) findPods(identifiers []string) (map[string]string, error) {
	pods := make(map[string]string)
	for _, identifier := range identifiers {
		pod, err := r.storageStrategy.GetStorageIdentifier(identifier)
		if err != nil {
			continue
		}
		pods[identifier] = pod
	}
	return pods, nil
}

// findOwners finds the owners of the given pods.
// Return value is a map where the keys are the pods and the values are all the WebIDs that own this pod.
func (r *OwnerPermissionReader) findOwners(pods map[string]string) (map[string][]string, error) {
	owners := make(map[string][]string)
	uniquePods := make(map[string]bool)

	// Get unique pod values
	for _, pod := range pods {
		uniquePods[pod] = true
	}

	// Find owners for each pod
	for baseURL := range uniquePods {
		pod, err := r.podStore.FindByBaseURL(baseURL)
		if err != nil {
			continue
		}

		podOwners, err := r.podStore.GetOwners(pod.ID)
		if err != nil {
			continue
		}

		// Convert owners to WebIDs
		webIDs := make([]string, len(podOwners))
		for i, owner := range podOwners {
			webIDs[i] = owner.WebID
		}
		owners[baseURL] = webIDs
	}

	return owners, nil
}

// GetPodStore returns the pod store.
func (r *OwnerPermissionReader) GetPodStore() pod.Store {
	return r.podStore
}

// GetAuthStrategy returns the auth strategy.
func (r *OwnerPermissionReader) GetAuthStrategy() auxiliary.IdentifierStrategy {
	return r.authStrategy
}

// GetStorageStrategy returns the storage strategy.
func (r *OwnerPermissionReader) GetStorageStrategy() description.StorageLocationStrategy {
	return r.storageStrategy
}
