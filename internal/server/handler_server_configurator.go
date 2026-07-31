//go:build cssstub

// Go equivalent of HandlerServerConfigurator.ts
package server

import (
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
)

type HttpHandler interface {
	HandleSafe(w http.ResponseWriter, r *http.Request) error
}

type HandlerServerConfigurator struct {
	handler        HttpHandler
	showStackTrace bool
}

func NewHandlerServerConfigurator(handler HttpHandler, showStackTrace bool) *HandlerServerConfigurator {
	return &HandlerServerConfigurator{
		handler:        handler,
		showStackTrace: showStackTrace,
	}
}

// Handle attaches the handler to the http.Server's handler field
func (c *HandlerServerConfigurator) HandleSafe(server *http.Server) error {
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received %s request for %s", r.Method, r.URL.Path)
		err := c.handler.HandleSafe(w, r)
		if err != nil {
			errMsg := c.createErrorMessage(err)
			log.Printf("Request error: %s", errMsg)
			if w.Header().Get("Content-Type") == "" {
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			}
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, errMsg)
			return
		}
		// If no handler wrote a response, return 404
		if w.Header().Get("Content-Type") == "" {
			w.WriteHeader(http.StatusNotFound)
		}
	})
	return nil
}

func (c *HandlerServerConfigurator) createErrorMessage(err error) string {
	if c.showStackTrace {
		return fmt.Sprintf("%s\n%s", err.Error(), debug.Stack())
	}
	return err.Error() + "\n"
}
