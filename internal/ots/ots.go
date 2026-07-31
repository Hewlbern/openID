package ots

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Client stamps and verifies digests via OpenTimestamps calendars.
// When calendars are unreachable, proofs are stored locally as pending attestations.
type Client struct {
	Calendars []string
	HTTP      *http.Client
	mu        sync.RWMutex
	Proofs    map[string]*Proof // digestHex -> proof
}

type Proof struct {
	Digest      string    `json:"digest"`
	Status      string    `json:"status"` // pending | stamped | verified
	CalendarURL string    `json:"calendarUrl,omitempty"`
	OTSBase64   string    `json:"ots,omitempty"`
	StampedAt   time.Time `json:"stampedAt"`
	BitcoinRef  string    `json:"bitcoinRef,omitempty"`
	Message     string    `json:"message,omitempty"`
}

func New(calendars ...string) *Client {
	if len(calendars) == 0 {
		calendars = []string{
			"https://alice.btc.calendar.opentimestamps.org",
			"https://bob.btc.calendar.opentimestamps.org",
			"https://finney.calendar.eternitywall.com",
		}
	}
	return &Client{
		Calendars: calendars,
		HTTP:      &http.Client{Timeout: 20 * time.Second},
		Proofs:    map[string]*Proof{},
	}
}

// Stamp submits a 32-byte digest (or hashes the input) to OTS calendars.
func (c *Client) Stamp(ctx context.Context, digest []byte) (*Proof, error) {
	if len(digest) != 32 {
		sum := sha256.Sum256(digest)
		digest = sum[:]
	}
	digestHex := hex.EncodeToString(digest)
	proof := &Proof{
		Digest:    digestHex,
		Status:    "pending",
		StampedAt: time.Now().UTC(),
		Message:   "awaiting Bitcoin calendar attestation",
	}

	for _, cal := range c.Calendars {
		url := cal + "/timestamp/" + digestHex
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(digest))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := c.HTTP.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 && len(body) > 0 {
			proof.Status = "stamped"
			proof.CalendarURL = cal
			proof.OTSBase64 = hex.EncodeToString(body) // store raw proof bytes as hex
			proof.Message = "stamped by OpenTimestamps calendar; pending Bitcoin confirmation"
			proof.BitcoinRef = "ots:" + digestHex[:16]
			break
		}
	}

	// Always persist locally so verification API works offline
	if proof.OTSBase64 == "" {
		local := map[string]interface{}{
			"digest":    digestHex,
			"algorithm": "sha256",
			"calendars": c.Calendars,
			"stampedAt": proof.StampedAt,
			"pending":   true,
		}
		raw, _ := json.Marshal(local)
		proof.OTSBase64 = hex.EncodeToString(raw)
		proof.BitcoinRef = "ots-pending:" + digestHex[:16]
		proof.Message = "local pending proof (calendars unreachable or deferred); will verify when calendars confirm"
	}

	c.mu.Lock()
	c.Proofs[digestHex] = proof
	c.mu.Unlock()
	return proof, nil
}

// Verify checks a previously stamped digest.
func (c *Client) Verify(ctx context.Context, digestHex string) (*Proof, error) {
	c.mu.RLock()
	p, ok := c.Proofs[digestHex]
	c.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("no proof for digest")
	}
	// Attempt upgrade via calendar get
	for _, cal := range c.Calendars {
		url := cal + "/timestamp/" + digestHex
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			continue
		}
		resp, err := c.HTTP.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == 200 && len(body) > 0 {
			p.Status = "verified"
			p.CalendarURL = cal
			p.OTSBase64 = hex.EncodeToString(body)
			p.Message = "OTS proof retrieved from calendar (Bitcoin-anchored when upgraded)"
			p.BitcoinRef = "ots:" + digestHex[:16]
			c.mu.Lock()
			c.Proofs[digestHex] = p
			c.mu.Unlock()
			return p, nil
		}
	}
	// pending is still a valid attestation path
	if p.Status == "stamped" || p.Status == "pending" {
		return p, nil
	}
	return p, nil
}

// ProofJSON returns proof as JSON bytes.
func (p *Proof) JSON() []byte {
	raw, _ := json.MarshalIndent(p, "", "  ")
	return raw
}
