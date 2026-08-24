package main

import (
	"log"
	"os"

	"solid-go/internal/openidmcp"
)

func main() {
	base := os.Getenv("OPENID_BASE_URL")
	if base == "" {
		base = os.Getenv("SOLID_BASE_URL")
	}
	if base == "" {
		base = "http://localhost:4000"
	}
	srv := openidmcp.New(base)
	if err := srv.ServeStdio(os.Stdin, os.Stdout); err != nil {
		log.Fatal(err)
	}
}
