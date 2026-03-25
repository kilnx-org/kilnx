package runtime

import (
	"fmt"
	"net/smtp"
	"os"
	"strings"
)

// EmailConfig holds SMTP settings from environment variables
type EmailConfig struct {
	Host     string // KILNX_SMTP_HOST (default: localhost)
	Port     string // KILNX_SMTP_PORT (default: 25)
	User     string // KILNX_SMTP_USER
	Password string // KILNX_SMTP_PASS
	From     string // KILNX_SMTP_FROM (default: noreply@localhost)
}

func loadEmailConfig() EmailConfig {
	cfg := EmailConfig{
		Host: getEnv("KILNX_SMTP_HOST", "localhost"),
		Port: getEnv("KILNX_SMTP_PORT", "25"),
		User: getEnv("KILNX_SMTP_USER", ""),
		Password: getEnv("KILNX_SMTP_PASS", ""),
		From: getEnv("KILNX_SMTP_FROM", "noreply@localhost"),
	}
	return cfg
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

// SendEmail sends an email using SMTP
func SendEmail(to, subject, body string) error {
	cfg := loadEmailConfig()

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		cfg.From, to, subject, body)

	addr := cfg.Host + ":" + cfg.Port

	var auth smtp.Auth
	if cfg.User != "" {
		auth = smtp.PlainAuth("", cfg.User, cfg.Password, cfg.Host)
	}

	err := smtp.SendMail(addr, auth, cfg.From, []string{to}, []byte(msg))
	if err != nil {
		return fmt.Errorf("sending email to %s: %w", to, err)
	}

	fmt.Printf("  email sent to %s: %s\n", to, subject)
	return nil
}

// resolveEmailRecipient resolves the "to" field.
// Can be a literal email, a :param from context, or a query result.
func resolveEmailRecipient(to string, params map[string]string) string {
	if strings.HasPrefix(to, ":") {
		paramName := strings.TrimPrefix(to, ":")
		if val, ok := params[paramName]; ok {
			return val
		}
	}
	return to
}
