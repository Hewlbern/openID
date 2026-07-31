package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"
)

func main() {
	privB64 := flag.String("priv", "", "base64url private key")
	pubB64 := flag.String("pub", "", "base64url public key")
	webID := flag.String("webid", "", "webid")
	method := flag.String("method", "PUT", "http method")
	path := flag.String("path", "/", "request path")
	flag.Parse()

	raw, err := base64.RawURLEncoding.DecodeString(*privB64)
	if err != nil {
		raw, err = base64.StdEncoding.DecodeString(*privB64)
	}
	if err != nil {
		fatal(err)
	}
	var priv ed25519.PrivateKey
	switch len(raw) {
	case ed25519.PrivateKeySize:
		priv = ed25519.PrivateKey(raw)
	case ed25519.SeedSize:
		priv = ed25519.NewKeyFromSeed(raw)
	default:
		fatal(fmt.Errorf("unexpected private key length %d", len(raw)))
	}
	_ = pubB64
	ts := fmt.Sprintf("%d", time.Now().Unix())
	msg := []byte(*method + "|" + *path + "|" + ts + "|" + *webID)
	sig := ed25519.Sign(priv, msg)
	_ = json.NewEncoder(os.Stdout).Encode(map[string]string{
		"ts":  ts,
		"sig": base64.RawURLEncoding.EncodeToString(sig),
	})
}

func fatal(err error) {
	_ = json.NewEncoder(os.Stdout).Encode(map[string]string{"error": err.Error()})
	os.Exit(1)
}
