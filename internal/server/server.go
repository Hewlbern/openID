package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"solid-go/internal/agent"
	"solid-go/internal/audit"
	"solid-go/internal/authn"
	"solid-go/internal/identityapi"
	"solid-go/internal/ipfs"
	"solid-go/internal/logging"
	"solid-go/internal/notify"
	"solid-go/internal/openidmcp"
	"solid-go/internal/ots"
	"solid-go/internal/resourcestore"
	"solid-go/internal/solid"
	"solid-go/internal/storage"
	"solid-go/internal/wac"
	"solid-go/web"
)

// ServerOptions configures the Solid server.
type ServerOptions struct {
	Port            int
	HTTPS           bool
	CertFile        string
	KeyFile         string
	Storage         storage.Storage
	StoragePath     string
	Logger          logging.Logger
	BaseURL         string
	TokenSecret     string
	IPFSAPI         string
	OTSCalendars    []string
	AuditBatchEvery time.Duration
}

// Server is the SolidGo HTTP server facade.
type Server struct {
	opts   *ServerOptions
	http   *http.Server
	logger logging.Logger
	store  *resourcestore.Store
	audit  *audit.Logger
	cancel context.CancelFunc
}

// NewServer wires storage, LDP, WAC, identity, agents, audit, and notifications.
func NewServer(opts *ServerOptions) *Server {
	if opts.Logger == nil {
		opts.Logger = logging.NewBasicLogger(logging.Info)
	}
	if opts.Port == 0 {
		opts.Port = 3000
	}
	if opts.BaseURL == "" {
		scheme := "http"
		if opts.HTTPS {
			scheme = "https"
		}
		opts.BaseURL = fmt.Sprintf("%s://localhost:%d", scheme, opts.Port)
	}
	if opts.AuditBatchEvery == 0 {
		opts.AuditBatchEvery = 30 * time.Second
	}
	if v := os.Getenv("SOLID_BASE_URL"); v != "" {
		opts.BaseURL = strings.TrimRight(v, "/")
	}
	if v := os.Getenv("IPFS_API"); v != "" {
		opts.IPFSAPI = v
	}
	if v := os.Getenv("SOLID_TOKEN_SECRET"); v != "" {
		opts.TokenSecret = v
	}
	if v := os.Getenv("OTS_CALENDAR"); v != "" {
		opts.OTSCalendars = strings.Split(v, ",")
	}

	rs := resourcestore.New(opts.Storage, opts.StoragePath)
	tokens := authn.NewTokenService(opts.TokenSecret)
	wacChecker := wac.NewChecker(rs)
	hub := notify.NewHub()
	ipfsClient := ipfs.New(opts.IPFSAPI)
	otsClient := ots.New(opts.OTSCalendars...)
	auditLog := audit.New(rs, ipfsClient, otsClient, opts.AuditBatchEvery)
	agents := agent.NewRegistry(rs, tokens, opts.BaseURL)
	idp := identityapi.New(rs, tokens, opts.BaseURL)

	ldp := &solid.LDPHandler{
		Store:   rs,
		WAC:     wacChecker,
		Tokens:  tokens,
		BaseURL: opts.BaseURL,
		Logger:  opts.Logger,
		OnNotify: func(path, activity string) {
			hub.Publish(path, activity)
		},
		OnAudit: func(ctx context.Context, agentWebID, method, path string, body []byte) {
			pk, _ := agents.PrivateKey(agentWebID)
			if _, err := auditLog.Append(ctx, agentWebID, method, path, body, pk); err != nil {
				opts.Logger.Warn("audit append failed: %v", err)
			}
		},
	}

	site := web.New(idp, opts.BaseURL)

	root := http.NewServeMux()
	root.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	root.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"name":        "OpenID",
			"product":     "SolidGo",
			"version":     "1.0.0",
			"status":      "ok",
			"protocol":    "Solid Protocol",
			"baseUrl":     opts.BaseURL,
			"storagePath": opts.StoragePath,
			"accounts":    idp.AccountCount(),
			"agents":      agents.Count(),
			"endpoints": map[string]string{
				"idp":           opts.BaseURL + "/idp/",
				"oidc":          opts.BaseURL + "/.well-known/openid-configuration",
				"solid":         opts.BaseURL + "/.well-known/solid",
				"agents":        opts.BaseURL + "/agents",
				"audit":         opts.BaseURL + "/audit/events/",
				"notifications": opts.BaseURL + "/notifications/",
				"dashboard":     opts.BaseURL + "/",
				"welcome":       opts.BaseURL + "/welcome",
				"mcp":           opts.BaseURL + "/mcp",
				"records":       opts.BaseURL + "/records",
				"sparql":        opts.BaseURL + "/records/sparql",
			},
		})
	})
	mcp := openidmcp.New(opts.BaseURL)
	root.Handle("/mcp", mcp.Handler())
	root.Handle("/mcp/", mcp.Handler())
	idp.Routes(root)
	agents.Routes(root)
	auditLog.Routes(root)
	hub.Routes(root)
	site.Routes(root)
	root.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			if web.WantsHTML(r) || r.URL.Query().Get("format") == "html" {
				site.ServeDashboard(w, r)
				return
			}
			http.Redirect(w, r, "/api/status", http.StatusFound)
			return
		}
		ldp.ServeHTTP(w, r)
	})

	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", opts.Port),
		Handler:           corsMiddleware(root),
		ReadHeaderTimeout: 10 * time.Second,
	}

	return &Server{
		opts:   opts,
		http:   httpServer,
		logger: opts.Logger,
		store:  rs,
		audit:  auditLog,
	}
}

func (s *Server) bootstrap(ctx context.Context) {
	_ = s.store.EnsureContainer(ctx, "")
	rootACL := wac.DefaultPublicACL(s.opts.BaseURL+"/", "")
	_, _ = s.store.Put(ctx, ".acl", "text/turtle", []byte(rootACL), "", "")
	_ = s.store.EnsureContainer(ctx, "audit/")
	s.audit.Start(ctx)
}

// Start listens for HTTP connections.
func (s *Server) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.bootstrap(ctx)
	s.logger.Info("SolidGo listening on %s (base %s)", s.http.Addr, s.opts.BaseURL)
	err := s.http.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// ListenAndServeTLS listens with TLS.
func (s *Server) ListenAndServeTLS(certFile, keyFile string) error {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.bootstrap(ctx)
	err := s.http.ListenAndServeTLS(certFile, keyFile)
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.cancel != nil {
		s.cancel()
	}
	return s.http.Shutdown(ctx)
}

// Handler returns the root HTTP handler (useful for tests).
func (s *Server) Handler() http.Handler {
	return s.http.Handler
}

// Bootstrap initializes root storage and starts the audit worker.
func (s *Server) Bootstrap(ctx context.Context) {
	s.bootstrap(ctx)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, PUT, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, DPoP, Content-Type, Link, Slug, If-Match, If-None-Match, X-Agent-WebID, X-Agent-Signature, X-Agent-Timestamp, X-Agent-Public-Key")
		w.Header().Set("Access-Control-Expose-Headers", "Location, ETag, Link, WAC-Allow, Accept-Patch")
		if r.Method == http.MethodOptions && (strings.HasPrefix(r.URL.Path, "/idp") ||
			strings.HasPrefix(r.URL.Path, "/agents") ||
			strings.HasPrefix(r.URL.Path, "/audit") ||
			strings.HasPrefix(r.URL.Path, "/oauth") ||
			strings.HasPrefix(r.URL.Path, "/notifications") ||
			strings.HasPrefix(r.URL.Path, "/.well-known") ||
			strings.HasPrefix(r.URL.Path, "/api") ||
			strings.HasPrefix(r.URL.Path, "/app") ||
			strings.HasPrefix(r.URL.Path, "/welcome") ||
			strings.HasPrefix(r.URL.Path, "/dashboard") ||
			strings.HasPrefix(r.URL.Path, "/i/") ||
			strings.HasPrefix(r.URL.Path, "/mcp") ||
			strings.HasPrefix(r.URL.Path, "/records") ||
			r.URL.Path == "/health" || r.URL.Path == "/api/status") {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
