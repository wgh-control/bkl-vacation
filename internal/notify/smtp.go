package notify

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// SMTP delivers mail through an existing server. It is not an in-cluster relay.
type SMTP struct {
	Host     string
	Port     int
	Mode     string // starttls, tls, off
	User     string
	Password string
	From     string
	Timeout  time.Duration
}

func (s SMTP) Send(ctx context.Context, msg Message) error {
	if strings.TrimSpace(s.Host) == "" || strings.TrimSpace(s.From) == "" {
		return ErrNotConfigured
	}
	if msg.From == "" {
		msg.From = s.From
	}
	if strings.TrimSpace(msg.To) == "" {
		return fmt.Errorf("missing recipient")
	}
	addr := fmt.Sprintf("%s:%d", s.Host, s.Port)
	timeout := s.Timeout
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	dialer := &net.Dialer{Timeout: timeout}
	var conn net.Conn
	var err error
	mode := strings.ToLower(s.Mode)
	tlsCfg := &tls.Config{ServerName: s.Host, MinVersion: tls.VersionTLS12}
	if mode == "tls" {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, tlsCfg)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	client, err := smtp.NewClient(conn, s.Host)
	if err != nil {
		return err
	}
	defer client.Close()
	if mode == "starttls" {
		if err := client.StartTLS(tlsCfg); err != nil {
			return err
		}
	}
	if s.User != "" {
		if err := client.Auth(smtp.PlainAuth("", s.User, s.Password, s.Host)); err != nil {
			return err
		}
	}
	if err := client.Mail(msg.From); err != nil {
		return err
	}
	if err := client.Rcpt(msg.To); err != nil {
		return err
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write([]byte(formatMessage(msg))); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func formatMessage(msg Message) string {
	subject := mimeHeader(msg.Subject)
	body := strings.ReplaceAll(msg.Body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\n", "\r\n")
	return strings.Join([]string{
		"From: " + msg.From,
		"To: " + msg.To,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: 8bit",
		"",
		body,
		"",
	}, "\r\n")
}

func mimeHeader(s string) string {
	for _, r := range s {
		if r > 127 {
			return fmt.Sprintf("=?UTF-8?B?%s?=", b64(s))
		}
	}
	return s
}

func b64(s string) string {
	const enc = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	src := []byte(s)
	var out strings.Builder
	for i := 0; i < len(src); i += 3 {
		var b0, b1, b2 byte
		b0 = src[i]
		left := len(src) - i
		if left > 1 {
			b1 = src[i+1]
		}
		if left > 2 {
			b2 = src[i+2]
		}
		out.WriteByte(enc[b0>>2])
		out.WriteByte(enc[((b0&0x3)<<4)|(b1>>4)])
		if left == 1 {
			out.WriteString("==")
			break
		}
		out.WriteByte(enc[((b1&0x0f)<<2)|(b2>>6)])
		if left == 2 {
			out.WriteByte('=')
			break
		}
		out.WriteByte(enc[b2&0x3f])
	}
	return out.String()
}
