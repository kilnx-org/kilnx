package runtime

import (
	"os"
	"testing"
)

func TestSanitizeHostHeader(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"example.com", "example.com"},
		{"example.com:8080", "example.com:8080"},
		{"example.com\r\nevil.com", "example.comevil.com"},
		{"example.com\nevil.com", "example.comevil.com"},
		{"example.com\x00evil", "example.comevil"},
		{"example.com evil", "example.comevil"},
		{"example.com\tevil", "example.comevil"},
		{"", ""},
	}
	for _, tc := range tests {
		got := sanitizeHostHeader(tc.input)
		if got != tc.expected {
			t.Errorf("sanitizeHostHeader(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestSanitizeEmailAddress(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"user@example.com", "user@example.com"},
		{"user@example.com\r\nBcc: evil", "user@example.comBcc: evil"},
		{"user@example.com\nBcc: evil", "user@example.comBcc: evil"},
		{"user@example.com\x00", "user@example.com"},
		{"  user@example.com  ", "user@example.com"},
	}
	for _, tc := range tests {
		got := sanitizeEmailAddress(tc.input)
		if got != tc.expected {
			t.Errorf("sanitizeEmailAddress(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestIsAllowedURLScheme(t *testing.T) {
	tests := []struct {
		url      string
		expected bool
	}{
		{"https://example.com", true},
		{"http://example.com", true},
		{"mailto:test@example.com", true},
		{"tel:+123", true},
		{"javascript:alert(1)", false},
		{"data:text/html,<script>", false},
		{"/relative/path", true},
		{"#anchor", true},
		{"", true},
		{"invalid", true},
	}
	for _, tc := range tests {
		got := isAllowedURLScheme(tc.url)
		if got != tc.expected {
			t.Errorf("isAllowedURLScheme(%q) = %v, want %v", tc.url, got, tc.expected)
		}
	}
}

func TestIsPathWithinAllowedDirs(t *testing.T) {
	// These tests assume the current working directory is the repo root.
	// Absolute paths inside cwd/uploads or cwd/templates are allowed.
	cwd, _ := os.Getwd()
	tests := []struct {
		path     string
		expected bool
	}{
		{"uploads/file.txt", true},
		{"templates/mail.html", true},
		{"internal/runtime/server.go", true},
		{"/etc/passwd", false},
		{"../README.md", false},
		{"/tmp/test.txt", false},
	}
	for _, tc := range tests {
		got := isPathWithinAllowedDirs(tc.path)
		if got != tc.expected {
			t.Errorf("isPathWithinAllowedDirs(%q) = %v, want %v (cwd=%s)", tc.path, got, tc.expected, cwd)
		}
	}
}

func TestIsAllowedUploadExt(t *testing.T) {
	tests := []struct {
		filename string
		expected bool
	}{
		{"photo.jpg", true},
		{"doc.PDF", true},
		{"archive.zip", true},
		{"script.html", false},
		{"malicious.svg", false},
		{"noext", false},
		{"", false},
	}
	for _, tc := range tests {
		got := isAllowedUploadExt(tc.filename)
		if got != tc.expected {
			t.Errorf("isAllowedUploadExt(%q) = %v, want %v", tc.filename, got, tc.expected)
		}
	}
}
