package identityapi

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// mailSender delivers password-reset email. Disabled senders never error on
// Enabled checks; callers must not persist a token they cannot deliver.
type mailSender interface {
	Enabled() bool
	Send(ctx context.Context, to, subject, text string) error
}

type disabledMailer struct{}

func (disabledMailer) Enabled() bool { return false }

func (disabledMailer) Send(context.Context, string, string, string) error {
	return fmt.Errorf("email is not configured")
}

type fileMailer struct {
	dir string
}

func (m fileMailer) Enabled() bool { return m.dir != "" }

func (m fileMailer) Send(_ context.Context, to, subject, text string) error {
	if err := os.MkdirAll(m.dir, 0o700); err != nil {
		return err
	}
	raw, err := json.Marshal(map[string]string{
		"to":      to,
		"subject": subject,
		"text":    text,
	})
	if err != nil {
		return err
	}
	name := fmt.Sprintf("mail-%d.json", time.Now().UnixNano())
	return os.WriteFile(filepath.Join(m.dir, name), raw, 0o600)
}

type resendMailer struct {
	key  string
	from string
	hc   *http.Client
}

func (m resendMailer) Enabled() bool { return m.key != "" && m.from != "" }

func (m resendMailer) Send(ctx context.Context, to, subject, text string) error {
	if !m.Enabled() {
		return fmt.Errorf("resend is not configured")
	}
	body, err := json.Marshal(map[string]any{
		"from":    m.from,
		"to":      []string{to},
		"subject": subject,
		"text":    text,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+m.key)
	req.Header.Set("Content-Type", "application/json")
	hc := m.hc
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("resend status %d", resp.StatusCode)
	}
	return nil
}

type smtpMailer struct {
	host string
	port string
	user string
	pass string
	from string
}

func (m smtpMailer) Enabled() bool { return m.host != "" && m.from != "" }

func (m smtpMailer) Send(_ context.Context, to, subject, text string) error {
	if !m.Enabled() {
		return fmt.Errorf("smtp is not configured")
	}
	addr := net.JoinHostPort(m.host, m.port)
	msg := strings.Join([]string{
		"From: " + m.from,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		text,
	}, "\r\n")
	var auth smtp.Auth
	if m.user != "" {
		auth = smtp.PlainAuth("", m.user, m.pass, m.host)
	}
	if m.port == "465" {
		return sendImplicitTLS(addr, m.host, auth, m.from, []string{to}, []byte(msg))
	}
	return smtp.SendMail(addr, auth, m.from, []string{to}, []byte(msg))
}

func sendImplicitTLS(addr, host string, auth smtp.Auth, from string, to []string, msg []byte) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
	if err != nil {
		return err
	}
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		_ = conn.Close()
		return err
	}
	defer c.Close()
	if auth != nil {
		if err := c.Auth(auth); err != nil {
			return err
		}
	}
	if err := c.Mail(from); err != nil {
		return err
	}
	for _, rcpt := range to {
		if err := c.Rcpt(rcpt); err != nil {
			return err
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

func newMailerFromEnv() mailSender {
	if dir := strings.TrimSpace(os.Getenv("OPENID_EMAIL_DIR")); dir != "" {
		return fileMailer{dir: dir}
	}
	provider := strings.ToLower(strings.TrimSpace(os.Getenv("OPENID_EMAIL_PROVIDER")))
	from := strings.TrimSpace(os.Getenv("OPENID_EMAIL_FROM"))
	switch provider {
	case "off", "none", "disabled":
		return disabledMailer{}
	case "resend":
		return resendMailer{key: strings.TrimSpace(os.Getenv("RESEND_API_KEY")), from: from}
	case "smtp":
		return smtpFromEnv(from)
	}
	if key := strings.TrimSpace(os.Getenv("RESEND_API_KEY")); key != "" {
		return resendMailer{key: key, from: from}
	}
	if strings.TrimSpace(os.Getenv("OPENID_SMTP_HOST")) != "" {
		return smtpFromEnv(from)
	}
	return disabledMailer{}
}

func smtpFromEnv(from string) smtpMailer {
	port := strings.TrimSpace(os.Getenv("OPENID_SMTP_PORT"))
	if port == "" {
		port = "587"
	}
	return smtpMailer{
		host: strings.TrimSpace(os.Getenv("OPENID_SMTP_HOST")),
		port: port,
		user: os.Getenv("OPENID_SMTP_USER"),
		pass: os.Getenv("OPENID_SMTP_PASSWORD"),
		from: from,
	}
}
