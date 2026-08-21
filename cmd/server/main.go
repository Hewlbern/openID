package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"solid-go/internal/logging"
	"solid-go/internal/server"
	"solid-go/internal/storage"
)

func main() {
	logger := logging.NewBasicLogger(logging.Info)

	port := flag.Int("port", envInt("SOLID_PORT", envInt("PORT", 3000)), "Port to listen on")
	https := flag.Bool("https", false, "Use HTTPS")
	certFile := flag.String("cert", "", "Path to TLS certificate file")
	keyFile := flag.String("key", "", "Path to TLS private key file")
	storagePath := flag.String("storage", envStr("SOLID_STORAGE_PATH", "./data"), "Path to storage directory")
	baseURL := flag.String("base-url", os.Getenv("SOLID_BASE_URL"), "Public base URL")
	ipfsAPI := flag.String("ipfs-api", os.Getenv("IPFS_API"), "Kubo IPFS HTTP API URL")
	flag.Parse()

	store, err := storage.NewFileStorage(*storagePath)
	if err != nil {
		logger.Error("Error creating storage: %v", err)
		os.Exit(1)
	}

	options := &server.ServerOptions{
		Port:            *port,
		HTTPS:           *https,
		CertFile:        *certFile,
		KeyFile:         *keyFile,
		Storage:         store,
		StoragePath:     *storagePath,
		Logger:          logger,
		BaseURL:         *baseURL,
		IPFSAPI:         *ipfsAPI,
		AuditBatchEvery: envDuration("AUDIT_BATCH_INTERVAL", 30*time.Second),
	}

	srv := server.NewServer(options)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		var err error
		if *https {
			if *certFile == "" || *keyFile == "" {
				logger.Error("TLS certificate and key files are required for HTTPS")
				os.Exit(1)
			}
			err = srv.ListenAndServeTLS(*certFile, *keyFile)
		} else {
			err = srv.Start()
		}
		if err != nil {
			logger.Error("Error starting server: %v", err)
			os.Exit(1)
		}
	}()

	logger.Info("Server listening on port %d", *port)
	<-ctx.Done()

	if err := srv.Shutdown(context.Background()); err != nil {
		logger.Error("Error shutting down server: %v", err)
		os.Exit(1)
	}
	logger.Info("Server stopped")
}

func envStr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil {
			return n
		}
	}
	return def
}

func envDuration(k string, def time.Duration) time.Duration {
	if v := os.Getenv(k); v != "" {
		d, err := time.ParseDuration(v)
		if err == nil {
			return d
		}
	}
	return def
}
