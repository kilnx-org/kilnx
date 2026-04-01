package runtime

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kilnx-org/kilnx/internal/parser"
)

// CSRF token store with expiry (#6 fix: bounded store with TTL cleanup)
type csrfEntry struct {
	createdAt time.Time
}

var (
	csrfTokens   = make(map[string]csrfEntry)
	csrfTokensMu sync.Mutex
	csrfMaxAge   = 30 * time.Minute
)

func init() {
	go csrfCleanupLoop()
}

func csrfCleanupLoop() {
	for {
		time.Sleep(5 * time.Minute)
		csrfTokensMu.Lock()
		now := time.Now()
		for token, entry := range csrfTokens {
			if now.Sub(entry.createdAt) > csrfMaxAge {
				delete(csrfTokens, token)
			}
		}
		csrfTokensMu.Unlock()
	}
}

func generateCSRFToken() string {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		panic("kilnx: failed to generate CSRF token: " + err.Error())
	}
	token := hex.EncodeToString(b)
	csrfTokensMu.Lock()
	csrfTokens[token] = csrfEntry{createdAt: time.Now()}
	csrfTokensMu.Unlock()
	return token
}

func validateCSRFToken(token string) bool {
	csrfTokensMu.Lock()
	defer csrfTokensMu.Unlock()
	if entry, ok := csrfTokens[token]; ok {
		delete(csrfTokens, token)
		return time.Since(entry.createdAt) < csrfMaxAge
	}
	return false
}

// validateFormData validates form data against model constraints
func validateFormData(modelName string, app *parser.App, formData map[string]string) []string {
	var errors []string

	var model *parser.Model
	for i := range app.Models {
		if app.Models[i].Name == modelName {
			model = &app.Models[i]
			break
		}
	}

	if model == nil {
		return []string{"Unknown model: " + modelName}
	}

	for _, field := range model.Fields {
		if field.Auto || field.Type == parser.FieldReference {
			continue
		}

		val := formData[field.Name]

		if field.Required && strings.TrimSpace(val) == "" {
			label := strings.ToUpper(field.Name[:1]) + field.Name[1:]
			errors = append(errors, label+" is required")
		}

		if field.Type == parser.FieldEmail && val != "" {
			if !strings.Contains(val, "@") || !strings.Contains(val, ".") {
				errors = append(errors, "Invalid email address")
			}
		}

		if field.Type == parser.FieldOption && len(field.Options) > 0 && val != "" {
			valid := false
			for _, opt := range field.Options {
				if val == opt {
					valid = true
					break
				}
			}
			if !valid {
				label := strings.ToUpper(field.Name[:1]) + field.Name[1:]
				errors = append(errors, fmt.Sprintf("%s must be one of: %s", label, strings.Join(field.Options, ", ")))
			}
		}

		if field.Min != "" && val != "" {
			var min int
			fmt.Sscanf(field.Min, "%d", &min)
			label := strings.ToUpper(field.Name[:1]) + field.Name[1:]
			if field.Type == parser.FieldInt || field.Type == parser.FieldFloat {
				if n, err := strconv.ParseFloat(val, 64); err == nil && n < float64(min) {
					errors = append(errors, fmt.Sprintf("%s must be at least %d", label, min))
				}
			} else if len(val) < min {
				errors = append(errors, fmt.Sprintf("%s must be at least %d characters", label, min))
			}
		}

		if field.Max != "" && val != "" {
			var max int
			fmt.Sscanf(field.Max, "%d", &max)
			label := strings.ToUpper(field.Name[:1]) + field.Name[1:]
			if field.Type == parser.FieldInt || field.Type == parser.FieldFloat {
				if n, err := strconv.ParseFloat(val, 64); err == nil && max > 0 && n > float64(max) {
					errors = append(errors, fmt.Sprintf("%s must be at most %d", label, max))
				}
			} else if max > 0 && len(val) > max {
				errors = append(errors, fmt.Sprintf("%s must be at most %d characters", label, max))
			}
		}
	}

	return errors
}

// validateInlineRules validates form data against explicit validation rules
// (not model-based). Supports: "required", "is email", "is date", "min N", "max N"
func validateInlineRules(validations []parser.Validation, formData map[string]string) []string {
	var errors []string

	for _, v := range validations {
		val := formData[v.Field]
		label := strings.ToUpper(v.Field[:1]) + v.Field[1:]

		for i := 0; i < len(v.Rules); i++ {
			rule := v.Rules[i]

			switch rule {
			case "required":
				if strings.TrimSpace(val) == "" {
					errors = append(errors, label+" is required")
				}
			case "is":
				// "is" is followed by the type: "email", "date"
				if i+1 < len(v.Rules) {
					i++
					ruleType := v.Rules[i]
					switch ruleType {
					case "email":
						if val != "" && (!strings.Contains(val, "@") || !strings.Contains(val, ".")) {
							errors = append(errors, label+" must be a valid email address")
						}
					case "date":
						if val != "" {
							// Check common date formats
							valid := false
							for _, layout := range []string{"2006-01-02", "01/02/2006", "02-01-2006", "2006/01/02"} {
								if _, err := time.Parse(layout, val); err == nil {
									valid = true
									break
								}
							}
							if !valid {
								errors = append(errors, label+" must be a valid date")
							}
						}
					}
				}
			case "min":
				if i+1 < len(v.Rules) {
					i++
					var min int
					fmt.Sscanf(v.Rules[i], "%d", &min)
					if val != "" && len(val) < min {
						errors = append(errors, fmt.Sprintf("%s must be at least %d characters", label, min))
					}
				}
			case "max":
				if i+1 < len(v.Rules) {
					i++
					var max int
					fmt.Sscanf(v.Rules[i], "%d", &max)
					if val != "" && max > 0 && len(val) > max {
						errors = append(errors, fmt.Sprintf("%s must be at most %d characters", label, max))
					}
				}
			}
		}
	}

	return errors
}

// extractFormData reads form values from a POST request, including file uploads.
// config may be nil; defaults are used in that case.
func extractFormData(r *http.Request, config *parser.AppConfig) map[string]string {
	contentType := r.Header.Get("Content-Type")
	data := make(map[string]string)

	// Resolve upload settings from config
	uploadsDir := "uploads"
	var maxUploadBytes int64 = 32 << 20 // 32MB default
	if config != nil {
		if config.UploadsDir != "" {
			uploadsDir = config.UploadsDir
		}
		if config.UploadsMaxMB > 0 {
			maxUploadBytes = int64(config.UploadsMaxMB) << 20
		}
	}

	if strings.Contains(contentType, "multipart/form-data") {
		r.ParseMultipartForm(maxUploadBytes)
		if r.MultipartForm != nil {
			for key, values := range r.MultipartForm.Value {
				if len(values) > 0 {
					data[key] = values[0]
				}
			}
			// Handle file uploads
			for key, fileHeaders := range r.MultipartForm.File {
				if len(fileHeaders) > 0 {
					file, err := fileHeaders[0].Open()
					if err != nil {
						continue
					}
					defer file.Close()

					// Sanitize filename to prevent path traversal
					safeName := filepath.Base(fileHeaders[0].Filename)
					fileName := fmt.Sprintf("%d_%s", time.Now().UnixNano(), safeName)
					filePath := filepath.Join(uploadsDir, fileName)

					// Create uploads directory if needed
					os.MkdirAll(uploadsDir, 0755)

					dst, err := os.Create(filePath)
					if err != nil {
						continue
					}
					defer dst.Close()
					io.Copy(dst, file)

					// Store the web-accessible path
					data[key] = "/_uploads/" + fileName
				}
			}
		}
	} else {
		r.ParseForm()
		for key, values := range r.PostForm {
			if len(values) > 0 {
				data[key] = values[0]
			}
		}
	}

	return data
}
