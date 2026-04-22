package runtime

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kilnx-org/kilnx/internal/parser"
)

// allowedUploadExts is the whitelist of permitted file extensions for uploads.
var allowedUploadExts = map[string]bool{
	"jpg": true, "jpeg": true, "png": true, "gif": true,
	"webp": true, "pdf": true, "txt": true,
	"doc": true, "docx": true, "xls": true, "xlsx": true,
	"csv": true, "zip": true,
}

func isAllowedUploadExt(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		return false
	}
	ext = ext[1:] // remove leading dot
	return allowedUploadExts[ext]
}

var customBracketRe = regexp.MustCompile(`^custom\[(\w+)\]$`)

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
		if field.Auto || field.AutoUpdate || field.Type == parser.FieldReference {
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

		if field.Type == parser.FieldURL && val != "" {
			if u, err := url.Parse(val); err != nil || u.Scheme == "" || u.Host == "" {
				errors = append(errors, "Invalid URL")
			}
		}

		if field.Type == parser.FieldDate && val != "" {
			if _, err := time.Parse("2006-01-02", val); err != nil {
				errors = append(errors, "Invalid date (expected YYYY-MM-DD)")
			}
		}

		if field.Type == parser.FieldDecimal && val != "" {
			if _, err := strconv.ParseFloat(val, 64); err != nil {
				errors = append(errors, "Invalid decimal number")
			}
		}

		if field.Type == parser.FieldBigInt && val != "" {
			if _, err := strconv.ParseInt(val, 10, 64); err != nil {
				errors = append(errors, "Invalid integer")
			}
		}

		if field.Type == parser.FieldJSON && val != "" {
			var tmp interface{}
			if err := json.Unmarshal([]byte(val), &tmp); err != nil {
				errors = append(errors, "Invalid JSON")
			}
		}

		if field.Type == parser.FieldTags && len(field.Options) > 0 && val != "" {
			allowed := make(map[string]bool, len(field.Options))
			for _, opt := range field.Options {
				allowed[opt] = true
			}
			parts := strings.Split(val, ",")
			for _, part := range parts {
				tag := strings.TrimSpace(part)
				if tag != "" && !allowed[tag] {
					label := strings.ToUpper(field.Name[:1]) + field.Name[1:]
					errors = append(errors, fmt.Sprintf("%s contains invalid tag: %s", label, tag))
				}
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

	if model.CustomFieldsFile != "" {
		if manifest, ok := app.CustomManifests[modelName]; ok {
			customVals := make(map[string]string)
			if raw := formData["custom"]; raw != "" {
				var m map[string]interface{}
				if err := json.Unmarshal([]byte(raw), &m); err != nil {
					errors = append(errors, "Invalid custom field data")
				} else {
					for k, v := range m {
						customVals[k] = fmt.Sprintf("%v", v)
					}
				}
			}
			allowedKeys := make(map[string]bool, len(manifest.Fields))
			for _, f := range manifest.Fields {
				allowedKeys[f.Name] = true
			}
			for k := range customVals {
				if !allowedKeys[k] {
					errors = append(errors, fmt.Sprintf("Unknown custom field: %s", k))
				}
			}
			for _, f := range manifest.Fields {
				// column-mode fields are promoted to top-level keys by serializeCustomBrackets
				val := customVals[f.Name]
				if f.Mode == parser.CustomFieldModeColumn {
					val = formData[f.Name]
				}
				label := f.Label
				if label == "" {
					label = f.Name
				}
				if f.Required && strings.TrimSpace(val) == "" {
					errors = append(errors, label+" is required")
				}
				if f.Kind == parser.CustomFieldKindNumber && val != "" {
					if _, err := strconv.ParseFloat(val, 64); err != nil {
						errors = append(errors, fmt.Sprintf("%s must be a number", label))
					}
				}
				if f.Kind == parser.CustomFieldKindDate && val != "" {
					valid := false
					for _, layout := range []string{"2006-01-02", "01/02/2006", "02-01-2006", "2006/01/02"} {
						if _, err := time.Parse(layout, val); err == nil {
							valid = true
							break
						}
					}
					if !valid {
						errors = append(errors, fmt.Sprintf("%s must be a valid date", label))
					}
				}
				if f.Kind == parser.CustomFieldKindEmail && val != "" {
					if !strings.Contains(val, "@") || !strings.Contains(val, ".") {
						errors = append(errors, fmt.Sprintf("%s must be a valid email address", label))
					}
				}
				if f.Kind == parser.CustomFieldKindPhone && val != "" {
					stripped := strings.Map(func(r rune) rune {
						if r >= '0' && r <= '9' {
							return r
						}
						return -1
					}, val)
					if len(stripped) < 7 {
						errors = append(errors, fmt.Sprintf("%s must be a valid phone number", label))
					}
				}
				if f.Kind == parser.CustomFieldKindBool && val != "" {
					switch strings.ToLower(val) {
					case "true", "false", "1", "0", "on", "off", "yes", "no":
					default:
						errors = append(errors, fmt.Sprintf("%s must be a boolean value", label))
					}
				}
				if f.Kind == parser.CustomFieldKindOption && len(f.Options) > 0 && val != "" {
					valid := false
					for _, opt := range f.Options {
						if val == opt {
							valid = true
							break
						}
					}
					if !valid {
						errors = append(errors, fmt.Sprintf("%s must be one of: %s", label, strings.Join(f.Options, ", ")))
					}
				}
				if f.Kind == parser.CustomFieldKindReference && val != "" {
					if _, err := strconv.Atoi(val); err != nil {
						errors = append(errors, fmt.Sprintf("%s must be a valid reference ID", label))
					}
				}
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
					if fileHeaders[0].Filename == "" {
						continue
					}
					// Validate file extension against whitelist
					if !isAllowedUploadExt(fileHeaders[0].Filename) {
						fmt.Fprintf(os.Stderr, "kilnx: rejected upload of disallowed file type: %s\n", fileHeaders[0].Filename)
						continue
					}
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

	serializeCustomBrackets(data)
	return data
}

// serializeCustomBrackets collects custom[field]=value entries from form data,
// JSON-encodes them as data["custom"], and also promotes each field as a top-level
// key so column-mode SQL (e.g. INSERT ... (:revenue)) can bind directly.
func serializeCustomBrackets(data map[string]string) {
	customMap := make(map[string]string)
	for k, v := range data {
		if m := customBracketRe.FindStringSubmatch(k); m != nil {
			customMap[m[1]] = v
		}
	}
	if len(customMap) == 0 {
		return
	}
	for k := range customMap {
		delete(data, "custom["+k+"]")
	}
	if existing, ok := data["custom"]; ok && existing != "" {
		var prev map[string]interface{}
		if json.Unmarshal([]byte(existing), &prev) == nil {
			for k, v := range customMap {
				prev[k] = v
			}
			merged := make(map[string]string, len(prev))
			for k, v := range prev {
				merged[k] = fmt.Sprintf("%v", v)
			}
			customMap = merged
		}
	}
	b, err := json.Marshal(customMap)
	if err == nil {
		data["custom"] = string(b)
	}
	// Also promote each field as a direct key so column-mode SQL binds :fieldname.
	for k, v := range customMap {
		if _, exists := data[k]; !exists {
			data[k] = v
		}
	}
}
