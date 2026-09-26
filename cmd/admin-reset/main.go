// Command admin-reset asks a running pod to issue a one-time password reset
// link, or to revoke exposed client credentials. The admin secret and the
// reset token are never written to a log file by this command; the link is
// printed to stdout only.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	base := flag.String("base", env("SOLID_BASE_URL", "https://pod-production-ebe1.up.railway.app"), "Pod origin")
	handle := flag.String("handle", "", "Account handle")
	secret := flag.String("secret", os.Getenv("OPENID_ADMIN_RESET_SECRET"), "Operator secret (OPENID_ADMIN_RESET_SECRET)")
	revoke := flag.Bool("revoke-clients", false, "Revoke client credentials instead of issuing a reset link")
	all := flag.Bool("all", false, "With -revoke-clients, revoke every account's client credentials")
	flag.Parse()

	if strings.TrimSpace(*secret) == "" {
		fmt.Fprintln(os.Stderr, "OPENID_ADMIN_RESET_SECRET is required")
		os.Exit(2)
	}
	if !*all && strings.TrimSpace(*handle) == "" {
		fmt.Fprintln(os.Stderr, "usage: admin-reset -handle mike")
		fmt.Fprintln(os.Stderr, "       admin-reset -revoke-clients -all")
		os.Exit(2)
	}

	path := "/idp/password/admin-reset"
	body := map[string]any{"handle": strings.TrimSpace(*handle)}
	if *revoke {
		path = "/idp/admin/revoke-clients"
		body["all"] = *all
	}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(*base, "/")+path, bytes.NewReader(raw))
	if err != nil {
		fmt.Fprintln(os.Stderr, "request failed")
		os.Exit(1)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-OpenID-Admin-Secret", *secret)
	hc := &http.Client{Timeout: 30 * time.Second}
	resp, err := hc.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "pod unreachable")
		os.Exit(1)
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		fmt.Fprintf(os.Stderr, "pod returned %d\n", resp.StatusCode)
		os.Exit(1)
	}
	if *revoke {
		var out struct {
			Revoked int `json:"revoked"`
		}
		_ = json.Unmarshal(payload, &out)
		fmt.Printf("revoked %d client credential(s)\n", out.Revoked)
		return
	}
	var out struct {
		ResetURL string `json:"resetUrl"`
		Handle   string `json:"handle"`
		Expires  string `json:"expires"`
	}
	if err := json.Unmarshal(payload, &out); err != nil || out.ResetURL == "" {
		fmt.Fprintln(os.Stderr, "pod did not return a reset link")
		os.Exit(1)
	}
	fmt.Printf("handle: %s\nexpires: %s\n%s\n", out.Handle, out.Expires, out.ResetURL)
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
