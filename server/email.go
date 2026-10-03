package main

import (
	"crypto/tls"
	"database/sql"
	"fmt"
	"log"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// EmailSettings holds SMTP configuration (singleton id=1)
type EmailSettings struct {
	Host string `json:"smtp_host"`
	Port int    `json:"smtp_port"`
	User string `json:"smtp_user"`
	Pass string `json:"smtp_pass"`
	From string `json:"smtp_from"`
	TLS  string `json:"smtp_tls"` // starttls|ssl|none
}

const emailSettingsSchema = `
CREATE TABLE IF NOT EXISTS email_settings (
	id INTEGER PRIMARY KEY CHECK (id = 1),
	smtp_host TEXT,
	smtp_port INTEGER,
	smtp_user TEXT,
	smtp_pass TEXT,
	smtp_from TEXT,
	smtp_tls TEXT,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
);`

func ensureEmailSettingsTable(db *sql.DB) {
	if db == nil {
		return
	}
	if _, err := db.Exec(emailSettingsSchema); err != nil {
		log.Printf("Warning: cannot migrate email_settings: %v", err)
	}
}

func (s *Server) effectiveEmailSettings() (EmailSettings, bool) {
	if s.config.SMTPHost != "" {
		return EmailSettings{
			Host: s.config.SMTPHost,
			Port: s.config.SMTPPort,
			User: s.config.SMTPUser,
			Pass: s.config.SMTPPass,
			From: s.config.SMTPFrom,
			TLS:  s.config.SMTPTLS,
		}, true
	}
	if s.session != nil && s.session.db != nil {
		ensureEmailSettingsTable(s.session.db)
		var host, user, pass, from, tlsMode sql.NullString
		var port sql.NullInt64
		err := s.session.db.QueryRow(`SELECT smtp_host,smtp_port,smtp_user,smtp_pass,smtp_from,smtp_tls FROM email_settings WHERE id=1`).Scan(&host, &port, &user, &pass, &from, &tlsMode)
		if err == nil && host.Valid && host.String != "" {
			p := 0
			if port.Valid {
				p = int(port.Int64)
			}
			return EmailSettings{Host: host.String, Port: p, User: user.String, Pass: pass.String, From: from.String, TLS: tlsMode.String}, true
		}
	}
	return EmailSettings{}, false
}

func (s *Server) saveEmailSettings(es EmailSettings) error {
	if s.session == nil || s.session.db == nil {
		return fmt.Errorf("no database configured")
	}
	ensureEmailSettingsTable(s.session.db)
	now := time.Now().Unix()
	_, err := s.session.db.Exec(`INSERT INTO email_settings (id,smtp_host,smtp_port,smtp_user,smtp_pass,smtp_from,smtp_tls,created_at,updated_at) VALUES (1,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET smtp_host=excluded.smtp_host, smtp_port=excluded.smtp_port, smtp_user=excluded.smtp_user, smtp_pass=excluded.smtp_pass, smtp_from=excluded.smtp_from, smtp_tls=excluded.smtp_tls, updated_at=excluded.updated_at`,
		es.Host, es.Port, es.User, es.Pass, es.From, es.TLS, now, now)
	return err
}

// sendMailFunc allows stubbing in tests
var sendMailFunc = sendMailReal

func sendMailReal(es EmailSettings, to, subject, body string) error {
	if es.Host == "" {
		return fmt.Errorf("email not configured")
	}
	port := es.Port
	if port == 0 {
		if strings.EqualFold(es.TLS, "ssl") {
			port = 465
		} else {
			port = 587
		}
	}
	addr := net.JoinHostPort(es.Host, fmt.Sprintf("%d", port))
	from := es.From
	if from == "" {
		from = es.User
	}
	if from == "" {
		return fmt.Errorf("email not configured: from address required")
	}
	headers := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n", from, to, subject)
	msg := []byte(headers + body)

	tlsMode := strings.ToLower(strings.TrimSpace(es.TLS))
	if tlsMode == "" {
		if port == 465 {
			tlsMode = "ssl"
		} else {
			tlsMode = "starttls"
		}
	}
	if tlsMode == "ssl" {
		// Implicit TLS
		tlsCfg := &tls.Config{ServerName: es.Host}
		conn, err := tls.Dial("tcp", addr, tlsCfg)
		if err != nil {
			return err
		}
		defer conn.Close()
		c, err := smtp.NewClient(conn, es.Host)
		if err != nil {
			return err
		}
		defer c.Quit()
		if es.User != "" {
			auth := smtp.PlainAuth("", es.User, es.Pass, es.Host)
			if err := c.Auth(auth); err != nil {
				return err
			}
		}
		if err := c.Mail(from); err != nil {
			return err
		}
		if err := c.Rcpt(to); err != nil {
			return err
		}
		w, err := c.Data()
		if err != nil {
			return err
		}
		if _, err := w.Write(msg); err != nil {
			w.Close()
			return err
		}
		w.Close()
		return c.Quit()
	}
	// STARTTLS or none
	var auth smtp.Auth
	if es.User != "" {
		auth = smtp.PlainAuth("", es.User, es.Pass, es.Host)
	}
	// For "none", we still use smtp.SendMail which will attempt STARTTLS opportunistically; that's acceptable
	return smtp.SendMail(addr, auth, from, []string{to}, msg)
}
