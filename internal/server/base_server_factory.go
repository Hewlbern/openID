//go:build cssstub

package server

import (
	"crypto/tls"
	"io/ioutil"
	"net/http"
)

// BaseServerFactoryOptions holds options for server creation.
type BaseServerFactoryOptions struct {
	HTTPS      bool
	Key        string
	Cert       string
	Pfx        string // Not directly used in Go, placeholder
	Passphrase string // Not directly used in Go, placeholder
}

// ServerConfigurator should have a HandleSafe method for configuring the server.
type ServerConfigurator interface {
	HandleSafe(server *http.Server) error
}

// BaseServerFactory provides a base for server instantiation/configuration.
type BaseServerFactory struct {
	configurator ServerConfigurator
	options      BaseServerFactoryOptions
}

// NewBaseServerFactory creates a new BaseServerFactory.
func NewBaseServerFactory(configurator ServerConfigurator, options *BaseServerFactoryOptions) *BaseServerFactory {
	opt := BaseServerFactoryOptions{HTTPS: false}
	if options != nil {
		opt = *options
	}
	return &BaseServerFactory{
		configurator: configurator,
		options:      opt,
	}
}

// CreateServer creates and configures an HTTP(S) server.
func (f *BaseServerFactory) CreateServer() (*http.Server, error) {
	serverOptions, tlsConfig, err := f.createServerOptions()
	if err != nil {
		return nil, err
	}

	server := &http.Server{}
	*server = serverOptions
	if tlsConfig != nil {
		server.TLSConfig = tlsConfig
	}

	if err := f.configurator.HandleSafe(server); err != nil {
		return nil, err
	}

	return server, nil
}

// createServerOptions reads key/cert files and prepares server/tls config.
func (f *BaseServerFactory) createServerOptions() (http.Server, *tls.Config, error) {
	options := f.options
	var tlsConfig *tls.Config

	if options.HTTPS {
		cert, err := ioutil.ReadFile(options.Cert)
		if err != nil {
			return http.Server{}, nil, err
		}
		key, err := ioutil.ReadFile(options.Key)
		if err != nil {
			return http.Server{}, nil, err
		}
		certificate, err := tls.X509KeyPair(cert, key)
		if err != nil {
			return http.Server{}, nil, err
		}
		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{certificate},
		}
	}
	// Additional options (Pfx, Passphrase) can be handled here if needed
	return http.Server{}, tlsConfig, nil
}
