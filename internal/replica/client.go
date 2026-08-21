package replica

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Account struct {
	ID           string    `json:"id"`
	Handle       string    `json:"handle"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	Bio          string    `json:"bio"`
	PasswordHash string    `json:"passwordHash"`
	WebID        string    `json:"webId"`
	PodPath      string    `json:"podPath"`
	PublicURL    string    `json:"publicUrl"`
	Created      time.Time `json:"created"`
}

type ClientCred struct {
	ID        string `json:"id"`
	Secret    string `json:"secret,omitempty"`
	WebID     string `json:"webId"`
	AccountID string `json:"accountId"`
	Name      string `json:"name"`
}

type StateFile struct {
	Accounts []Account    `json:"accounts"`
	Clients  []ClientCred `json:"clients"`
}

type Client struct {
	Peer    string
	HTTP    *http.Client
	Timeout time.Duration
}

func NewClient(peer string) *Client {
	return &Client{
		Peer:    strings.TrimRight(peer, "/"),
		HTTP:    &http.Client{Timeout: 60 * time.Second},
		Timeout: 60 * time.Second,
	}
}

func (c *Client) Login(ctx context.Context, handle, password string) (token, webID string, err error) {
	body, _ := json.Marshal(map[string]string{"handle": handle, "password": password})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Peer+"/idp/login", bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("login %s: %s", resp.Status, bytes.TrimSpace(raw))
	}
	var out struct {
		Token string `json:"token"`
		WebID string `json:"webId"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", "", err
	}
	if out.Token == "" {
		return "", "", fmt.Errorf("login: empty token")
	}
	return out.Token, out.WebID, nil
}

type AdoptRequest struct {
	Password string       `json:"password"`
	Account  Account      `json:"account"`
	Clients  []ClientCred `json:"clients"`
}

func (c *Client) Adopt(ctx context.Context, token string, req AdoptRequest) (*Account, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Peer+"/idp/replica/adopt", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("adopt %s: %s", resp.Status, bytes.TrimSpace(raw))
	}
	var acc Account
	if err := json.Unmarshal(raw, &acc); err != nil {
		return nil, err
	}
	return &acc, nil
}

func (c *Client) Put(ctx context.Context, token, path, contentType string, body []byte, container bool) error {
	url := c.Peer + "/" + strings.TrimPrefix(path, "/")
	var rdr io.Reader
	if !container {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, rdr)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if container {
		req.Header.Set("Link", `<http://www.w3.org/ns/ldp#BasicContainer>; rel="type"`)
		req.Header.Set("Content-Type", "text/turtle")
	} else {
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("PUT %s %s: %s", path, resp.Status, bytes.TrimSpace(raw))
	}
	return nil
}

func (c *Client) Get(ctx context.Context, token, path string) (body []byte, ct string, status int, err error) {
	url := c.Peer + "/" + strings.TrimPrefix(path, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", 0, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, "", 0, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, "", resp.StatusCode, err
	}
	return raw, resp.Header.Get("Content-Type"), resp.StatusCode, nil
}

// PushResources writes local pod files to the peer, rewriting local origins.
func PushResources(ctx context.Context, c *Client, token string, resources []Resource, fromBases []string, toBase string) (put, skipped int, err error) {
	// containers first so parents exist
	var containers, docs []Resource
	for _, r := range resources {
		if r.IsContainer {
			containers = append(containers, r)
		} else {
			docs = append(docs, r)
		}
	}
	for _, r := range containers {
		if err := c.Put(ctx, token, r.Path, r.ContentType, nil, true); err != nil {
			return put, skipped, err
		}
		put++
	}
	for _, r := range docs {
		body := r.Body
		if isTextContent(r.ContentType) {
			body = RewriteOrigin(body, fromBases, toBase)
		}
		remote, _, status, gerr := c.Get(ctx, token, r.Path)
		if gerr == nil && status == http.StatusOK && bytes.Equal(remote, body) {
			skipped++
			continue
		}
		if err := c.Put(ctx, token, r.Path, r.ContentType, body, false); err != nil {
			return put, skipped, err
		}
		put++
	}
	return put, skipped, nil
}
