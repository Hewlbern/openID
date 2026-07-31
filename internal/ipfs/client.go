package ipfs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Client talks to a Kubo HTTP API.
type Client struct {
	API    string
	HTTP   *http.Client
	mu     sync.RWMutex
	Memory map[string][]byte
}

func New(api string) *Client {
	return &Client{
		API:    strings.TrimRight(api, "/"),
		HTTP:   &http.Client{Timeout: 30 * time.Second},
		Memory: map[string][]byte{},
	}
}

type addResponse struct {
	Hash string `json:"Hash"`
}

// Add pins data and returns a CID (or local fake CID when IPFS is down).
func (c *Client) Add(ctx context.Context, data []byte) (string, error) {
	if c.API == "" {
		return c.localAdd(data), nil
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", "blob")
	if err != nil {
		return "", err
	}
	if _, err := part.Write(data); err != nil {
		return "", err
	}
	_ = w.Close()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.API+"/api/v0/add?pin=true", &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return c.localAdd(data), nil
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return c.localAdd(data), nil
	}
	var ar addResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil || ar.Hash == "" {
		return c.localAdd(data), nil
	}
	return ar.Hash, nil
}

// Cat retrieves content by CID.
func (c *Client) Cat(ctx context.Context, cid string) ([]byte, error) {
	c.mu.RLock()
	if data, ok := c.Memory[cid]; ok {
		c.mu.RUnlock()
		return data, nil
	}
	c.mu.RUnlock()
	if c.API == "" {
		return nil, fmt.Errorf("cid not found locally: %s", cid)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.API+"/api/v0/cat?arg="+cid, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ipfs cat: %s", string(b))
	}
	return io.ReadAll(resp.Body)
}

func (c *Client) localAdd(data []byte) string {
	sum := sha256.Sum256(data)
	cid := fmt.Sprintf("bafy%x", sum)[:50]
	c.mu.Lock()
	c.Memory[cid] = append([]byte(nil), data...)
	c.mu.Unlock()
	return cid
}
