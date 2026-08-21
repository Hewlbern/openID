package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"solid-go/internal/replica"
)

func main() {
	storagePath := flag.String("storage", env("SOLID_STORAGE_PATH", "./data"), "Local storage root")
	peer := flag.String("peer", env("SOLID_SYNC_PEER", ""), "Remote Solid origin")
	handle := flag.String("handle", env("SOLID_SYNC_HANDLE", "mike"), "Account handle to sync")
	password := flag.String("password", os.Getenv("SOLID_SYNC_PASSWORD"), "Account password")
	interval := flag.Duration("interval", envDuration("SOLID_SYNC_INTERVAL", 0), "Repeat every interval (0 = once)")
	rewriteLocal := flag.Bool("rewrite-local", false, "Rewrite local WebIDs to the peer origin")
	skipAdopt := flag.Bool("skip-adopt", false, "Only push pod files (no account merge)")
	flag.Parse()

	if *peer == "" || *password == "" {
		fmt.Fprintln(os.Stderr, "usage: sync -peer https://pod.example -handle mike -password …")
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := runOnce(ctx, *storagePath, *peer, *handle, *password, *rewriteLocal, *skipAdopt); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *interval <= 0 {
		return
	}
	t := time.NewTicker(*interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := runOnce(ctx, *storagePath, *peer, *handle, *password, false, *skipAdopt); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}
	}
}

func runOnce(ctx context.Context, storagePath, peer, handle, password string, rewriteLocal, skipAdopt bool) error {
	state, err := loadState(filepath.Join(storagePath, ".openid", "accounts.json"))
	if err != nil {
		return err
	}
	acc, clients, err := pickAccount(state, handle)
	if err != nil {
		return err
	}

	cli := replica.NewClient(peer)
	token, webID, err := cli.Login(ctx, handle, password)
	if err != nil {
		return fmt.Errorf("login: %w", err)
	}
	if !skipAdopt {
		adopted, err := cli.Adopt(ctx, token, replica.AdoptRequest{
			Password: password,
			Account:  acc,
			Clients:  clients,
		})
		if err != nil {
			return fmt.Errorf("adopt: %w", err)
		}
		fmt.Printf("account %s → %s\n", adopted.ID, adopted.WebID)
		if tok, _, err := cli.Login(ctx, handle, password); err == nil {
			token = tok
		}
	} else {
		fmt.Printf("push only (remote webid %s)\n", webID)
	}

	resources, err := replica.WalkPod(storagePath, handle)
	if err != nil {
		return err
	}
	from := append([]string{}, replica.DefaultLocalBases...)
	if acc.WebID != "" {
		if i := strings.Index(acc.WebID, "/"+handle+"/"); i > 0 {
			from = append(from, acc.WebID[:i])
		}
	}
	put, skipped, err := replica.PushResources(ctx, cli, token, resources, from, peer)
	if err != nil {
		return err
	}
	fmt.Printf("pushed %d resources (%d unchanged)\n", put, skipped)

	if rewriteLocal {
		if err := rewriteLocalState(filepath.Join(storagePath, ".openid", "accounts.json"), handle, peer); err != nil {
			return err
		}
		if err := rewriteLocalPod(storagePath, handle, from, peer); err != nil {
			return err
		}
		fmt.Println("rewrote local WebIDs to", strings.TrimRight(peer, "/"))
	}
	return nil
}

func loadState(path string) (*replica.StateFile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var st replica.StateFile
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

func pickAccount(st *replica.StateFile, handle string) (replica.Account, []replica.ClientCred, error) {
	handle = strings.ToLower(strings.Trim(handle, "/"))
	var acc replica.Account
	found := false
	for _, a := range st.Accounts {
		if strings.ToLower(a.Handle) == handle {
			acc = a
			found = true
			break
		}
	}
	if !found {
		return replica.Account{}, nil, fmt.Errorf("handle %s not in local accounts", handle)
	}
	var clients []replica.ClientCred
	for _, c := range st.Clients {
		if c.AccountID == acc.ID {
			clients = append(clients, c)
		}
	}
	return acc, clients, nil
}

func rewriteLocalState(path, handle, peer string) error {
	st, err := loadState(path)
	if err != nil {
		return err
	}
	peer = strings.TrimRight(peer, "/")
	handle = strings.ToLower(strings.Trim(handle, "/"))
	webID := peer + "/" + handle + "/profile/card#me"
	for i := range st.Accounts {
		if strings.ToLower(st.Accounts[i].Handle) != handle {
			continue
		}
		st.Accounts[i].WebID = webID
		st.Accounts[i].PublicURL = peer + "/i/" + st.Accounts[i].Handle
		id := st.Accounts[i].ID
		for j := range st.Clients {
			if st.Clients[j].AccountID == id {
				st.Clients[j].WebID = webID
			}
		}
	}
	raw, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0600)
}

func rewriteLocalPod(storagePath, handle string, from []string, peer string) error {
	resources, err := replica.WalkPod(storagePath, handle)
	if err != nil {
		return err
	}
	for _, r := range resources {
		if r.IsContainer || !isText(r.ContentType) {
			continue
		}
		next := replica.RewriteOrigin(r.Body, from, peer)
		if string(next) == string(r.Body) {
			continue
		}
		if err := os.WriteFile(filepath.Join(storagePath, filepath.FromSlash(r.Path)), next, 0644); err != nil {
			return err
		}
	}
	return nil
}

func isText(ct string) bool {
	ct = strings.ToLower(ct)
	return strings.HasPrefix(ct, "text/") || strings.Contains(ct, "json") || strings.Contains(ct, "xml") || ct == "text/turtle"
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
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
