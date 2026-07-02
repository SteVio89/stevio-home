package mailer

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/smtp"
	"strings"
)

// SMTPConfig holds SMTP connection parameters.
type SMTPConfig struct {
	Host     string // e.g. "smtp.postmarkapp.com"
	Port     int    // e.g. 587
	Username string
	Password string
	From     string // sender address, e.g. "noreply@myapp.com"

	// Logger, when set, receives a rendered-message dump on Send() if Host is
	// empty. Supports the local-dev workflow of grepping magic links out of
	// application logs instead of configuring SMTP.
	Logger *log.Logger
}

// Mailer sends email via stdlib net/smtp.
type Mailer struct {
	cfg SMTPConfig
}

// New creates a Mailer with the given SMTP configuration.
func New(cfg SMTPConfig) *Mailer {
	return &Mailer{cfg: cfg}
}

// Send sends a plain-text email to the given address.
//
// When Host is empty and Logger is set, the message is logged instead of sent
// (dev workflow). When Host is empty and Logger is nil, Send returns an error.
func (m *Mailer) Send(to, subject, body string) error {
	if m.cfg.Host == "" {
		if m.cfg.Logger != nil {
			m.cfg.Logger.Printf("mail: not configured — would send to %s\nSubject: %s\n%s", to, subject, body)
			return nil
		}
		return fmt.Errorf("mailer: SMTP host not configured")
	}
	addr := net.JoinHostPort(m.cfg.Host, fmt.Sprintf("%d", m.cfg.Port))
	auth := smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)
	msg := buildMessage(m.cfg.From, to, subject, body)

	if m.cfg.Port == 465 {
		return sendTLS(addr, m.cfg.Host, auth, m.cfg.From, to, msg)
	}
	return sendSTARTTLS(addr, m.cfg.Host, auth, m.cfg.From, to, msg)
}

// sanitizeHeader strips CR and LF characters to prevent SMTP header injection.
func sanitizeHeader(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	return s
}

func buildMessage(from, to, subject, body string) []byte {
	from = sanitizeHeader(from)
	to = sanitizeHeader(to)
	subject = sanitizeHeader(subject)

	var b strings.Builder
	b.WriteString("From: ")
	b.WriteString(from)
	b.WriteString("\r\n")
	b.WriteString("To: ")
	b.WriteString(to)
	b.WriteString("\r\n")
	b.WriteString("Subject: ")
	b.WriteString(subject)
	b.WriteString("\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	b.WriteString("\r\n")
	b.WriteString(body)
	return []byte(b.String())
}

// sendSTARTTLS connects on a plain port and upgrades to TLS via STARTTLS.
// Returns an error if STARTTLS is not supported by the server.
func sendSTARTTLS(addr, host string, auth smtp.Auth, from, to string, msg []byte) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("mailer: dial: %w", err)
	}
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("mailer: new client: %w", err)
	}
	defer func() { _ = client.Close() }()

	if ok, _ := client.Extension("STARTTLS"); !ok {
		return fmt.Errorf("mailer: server does not support STARTTLS")
	}
	if err := client.StartTLS(&tls.Config{ServerName: host}); err != nil {
		return fmt.Errorf("mailer: starttls: %w", err)
	}
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("mailer: auth: %w", err)
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("mailer: mail from: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("mailer: rcpt to: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("mailer: data: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("mailer: write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("mailer: close data: %w", err)
	}
	return client.Quit()
}

func sendTLS(addr, host string, auth smtp.Auth, from, to string, msg []byte) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: host})
	if err != nil {
		return fmt.Errorf("mailer: tls dial: %w", err)
	}
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("mailer: new client: %w", err)
	}
	defer func() { _ = client.Close() }()

	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("mailer: auth: %w", err)
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("mailer: mail from: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("mailer: rcpt to: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("mailer: data: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("mailer: write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("mailer: close data: %w", err)
	}
	return client.Quit()
}
