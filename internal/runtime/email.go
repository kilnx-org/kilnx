package runtime

import (
	crand "crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/smtp"
	"os"
	"path/filepath"
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

// LoadEmailTemplate loads a template file and interpolates {key} placeholders
func LoadEmailTemplate(templateName string, params map[string]string) string {
	// Try to load from templates/ directory
	path := "templates/" + templateName + ".html"
	data, err := os.ReadFile(path)
	if err != nil {
		// Try without .html extension
		path = "templates/" + templateName
		data, err = os.ReadFile(path)
		if err != nil {
			return "" // template not found
		}
	}

	content := string(data)
	// Interpolate {key} placeholders
	for k, v := range params {
		content = strings.ReplaceAll(content, "{"+k+"}", v)
	}
	return content
}

// SendEmailWithTemplate sends an email, optionally using a template
func SendEmailWithTemplate(to, subject, body, templateName string, params map[string]string) error {
	if templateName != "" {
		tmplBody := LoadEmailTemplate(templateName, params)
		if tmplBody != "" {
			body = tmplBody
		}
	}
	return SendEmail(to, subject, body)
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

// SendEmailWithAttachment sends an email with a file attachment using MIME multipart
func SendEmailWithAttachment(to, subject, body, attachPath string) error {
	cfg := loadEmailConfig()

	randBytes := make([]byte, 16)
	crand.Read(randBytes)
	boundary := "KilnxBoundary" + hex.EncodeToString(randBytes)

	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("From: %s\r\n", cfg.From))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", to))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=\"%s\"\r\n", boundary))
	msg.WriteString("\r\n")

	// Body part
	msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	msg.WriteString("Content-Transfer-Encoding: 7bit\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)
	msg.WriteString("\r\n")

	// Attachment part
	fileData, err := os.ReadFile(attachPath)
	if err != nil {
		return fmt.Errorf("reading attachment %s: %w", attachPath, err)
	}

	fileName := filepath.Base(attachPath)
	contentType := "application/octet-stream"
	if strings.HasSuffix(fileName, ".pdf") {
		contentType = "application/pdf"
	}

	msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	msg.WriteString(fmt.Sprintf("Content-Type: %s; name=\"%s\"\r\n", contentType, fileName))
	msg.WriteString("Content-Transfer-Encoding: base64\r\n")
	msg.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n", fileName))
	msg.WriteString("\r\n")
	msg.WriteString(base64.StdEncoding.EncodeToString(fileData))
	msg.WriteString("\r\n")

	msg.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	addr := cfg.Host + ":" + cfg.Port

	var auth smtp.Auth
	if cfg.User != "" {
		auth = smtp.PlainAuth("", cfg.User, cfg.Password, cfg.Host)
	}

	err = smtp.SendMail(addr, auth, cfg.From, []string{to}, []byte(msg.String()))
	if err != nil {
		return fmt.Errorf("sending email with attachment to %s: %w", to, err)
	}

	fmt.Printf("  email sent to %s (with attachment): %s\n", to, subject)
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
