//go:build cssstub

package client_credentials

import (
	"log"
)

type Adapter interface {
	Find(label string) (interface{}, error)
}

type AdapterPayload struct {
	ClientId      string   `json:"client_id"`
	ClientSecret  string   `json:"client_secret"`
	GrantTypes    []string `json:"grant_types"`
	RedirectUris  []string `json:"redirect_uris"`
	ResponseTypes []string `json:"response_types"`
}

type WebIdStore interface {
	IsLinked(webId, accountId string) (bool, error)
}

type ClientCredentialsStore interface {
	FindByLabel(label string) (*ClientCredentials, error)
	Delete(id string) error
}

type ClientCredentials struct {
	Id        string
	WebId     string
	AccountId string
	Secret    string
}

type ClientCredentialsAdapter struct {
	name                   string
	source                 Adapter
	webIdStore             WebIdStore
	clientCredentialsStore ClientCredentialsStore
}

func NewClientCredentialsAdapter(name string, source Adapter, webIdStore WebIdStore, clientCredentialsStore ClientCredentialsStore) *ClientCredentialsAdapter {
	return &ClientCredentialsAdapter{
		name:                   name,
		source:                 source,
		webIdStore:             webIdStore,
		clientCredentialsStore: clientCredentialsStore,
	}
}

func (a *ClientCredentialsAdapter) Find(label string) (interface{}, error) {
	payload, _ := a.source.Find(label)

	if payload == nil && a.name == "Client" {
		credentials, _ := a.clientCredentialsStore.FindByLabel(label)
		if credentials == nil {
			return payload, nil
		}

		// Make sure the WebID wasn't unlinked in the meantime
		valid, _ := a.webIdStore.IsLinked(credentials.WebId, credentials.AccountId)
		if !valid {
			log.Printf("Client credentials token %s contains WebID that is no longer linked to the account. Removing...", label)
			a.clientCredentialsStore.Delete(credentials.Id)
			return payload, nil
		}

		log.Printf("Authenticating as %s using client credentials", credentials.WebId)

		payload = &AdapterPayload{
			ClientId:      label,
			ClientSecret:  credentials.Secret,
			GrantTypes:    []string{"client_credentials"},
			RedirectUris:  []string{},
			ResponseTypes: []string{},
		}
	}
	return payload, nil
}

type ClientCredentialsAdapterFactory struct {
	source                 AdapterFactory
	webIdStore             WebIdStore
	clientCredentialsStore ClientCredentialsStore
}

type AdapterFactory interface {
	CreateStorageAdapter(name string) Adapter
}

func NewClientCredentialsAdapterFactory(source AdapterFactory, webIdStore WebIdStore, clientCredentialsStore ClientCredentialsStore) *ClientCredentialsAdapterFactory {
	return &ClientCredentialsAdapterFactory{
		source:                 source,
		webIdStore:             webIdStore,
		clientCredentialsStore: clientCredentialsStore,
	}
}

func (f *ClientCredentialsAdapterFactory) CreateStorageAdapter(name string) Adapter {
	adapter := f.source.CreateStorageAdapter(name)
	return NewClientCredentialsAdapter(name, adapter, f.webIdStore, f.clientCredentialsStore)
}
