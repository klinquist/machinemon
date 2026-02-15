package alerting

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"

	"github.com/machinemon/machinemon/internal/models"
)

type SMTPProvider struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`
	To       string `json:"to"`
	UseTLS   bool   `json:"use_tls"`
}

func (s *SMTPProvider) Name() string {
	return "smtp"
}

func (s *SMTPProvider) Validate() error {
	if s.Host == "" {
		return fmt.Errorf("host is required")
	}
	if s.Port == 0 {
		return fmt.Errorf("port is required")
	}
	if s.From == "" {
		return fmt.Errorf("from address is required")
	}
	if s.To == "" {
		return fmt.Errorf("to address is required")
	}
	return nil
}

func (s *SMTPProvider) Send(alert *models.Alert) error {
	subject := fmt.Sprintf("[MachineMon %s] %s", strings.ToUpper(alert.Severity), alert.AlertType)
	body := fmt.Sprintf("Subject: %s\r\nFrom: MachineMon <%s>\r\nTo: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s\r\n\r\nFired at: %s\r\n",
		subject, s.From, s.To, alert.Message, alert.FiredAt.Format("2006-01-02 15:04:05 UTC"))

	addr := fmt.Sprintf("%s:%d", s.Host, s.Port)

	var auth smtp.Auth
	if s.Username != "" {
		auth = smtp.PlainAuth("", s.Username, s.Password, s.Host)
	}

	if s.UseTLS || s.Port == 465 {
		return s.sendTLS(addr, auth, []byte(body))
	}

	return smtp.SendMail(addr, auth, s.From, []string{s.To}, []byte(body))
}

func (s *SMTPProvider) sendTLS(addr string, auth smtp.Auth, msg []byte) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: s.Host})
	if err != nil {
		return fmt.Errorf("tls dial: %w", err)
	}

	host, _, _ := net.SplitHostPort(addr)
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer c.Close()

	if auth != nil {
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := c.Mail(s.From); err != nil {
		return fmt.Errorf("smtp mail: %w", err)
	}
	if err := c.Rcpt(s.To); err != nil {
		return fmt.Errorf("smtp rcpt: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close: %w", err)
	}
	return c.Quit()
}
