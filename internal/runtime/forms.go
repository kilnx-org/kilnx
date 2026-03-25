package runtime

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/kilnx-org/kilnx/internal/database"
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

// renderForm generates an HTML form from a model definition
func renderForm(node parser.Node, app *parser.App, db *database.DB, r *http.Request) string {
	modelName := node.ModelName
	var model *parser.Model
	for i := range app.Models {
		if app.Models[i].Name == modelName {
			model = &app.Models[i]
			break
		}
	}

	if model == nil {
		return fmt.Sprintf("    <p style=\"color:red\">Model '%s' not found</p>\n", html.EscapeString(modelName))
	}

	// Pre-fill data if "form user with query: ..."
	var prefill database.Row
	if node.QuerySQL != "" && db != nil {
		params := extractPathParams(r)
		rows, err := db.QueryRowsWithParams(node.QuerySQL, params)
		if err == nil && len(rows) > 0 {
			prefill = rows[0]
		}
	}

	csrfToken := generateCSRFToken()

	// Check if model has image fields for multipart encoding
	hasImageField := false
	for _, field := range model.Fields {
		if field.Type == parser.FieldImage {
			hasImageField = true
			break
		}
	}

	var b strings.Builder
	if hasImageField {
		b.WriteString("    <form method=\"POST\" class=\"kilnx-form\" enctype=\"multipart/form-data\">\n")
	} else {
		b.WriteString("    <form method=\"POST\" class=\"kilnx-form\">\n")
	}
	b.WriteString(fmt.Sprintf("      <input type=\"hidden\" name=\"_csrf\" value=\"%s\">\n", csrfToken))

	for _, field := range model.Fields {
		// Skip auto fields (timestamp auto, etc.) and password on edit forms
		if field.Auto {
			continue
		}

		if field.Type == parser.FieldReference {
			b.WriteString(renderReferenceField(field, prefill, db))
			continue
		}

		b.WriteString(renderFormField(field, prefill))
	}

	label := "Save"
	if prefill != nil {
		label = "Update"
	}

	b.WriteString(fmt.Sprintf("      <button type=\"submit\" class=\"kilnx-btn\">%s</button>\n", label))
	b.WriteString("    </form>\n")

	return b.String()
}

func renderFormField(field parser.Field, prefill database.Row) string {
	var b strings.Builder

	name := field.Name
	label := strings.ToUpper(name[:1]) + name[1:]
	value := ""
	if prefill != nil {
		if v, ok := prefill[name]; ok {
			value = v
		}
	}

	required := ""
	if field.Required {
		required = " required"
	}

	b.WriteString("      <div class=\"kilnx-field\">\n")
	b.WriteString(fmt.Sprintf("        <label for=\"%s\">%s</label>\n", name, html.EscapeString(label)))

	switch field.Type {
	case parser.FieldText, parser.FieldPhone:
		minAttr := ""
		maxAttr := ""
		if field.Min != "" {
			minAttr = fmt.Sprintf(" minlength=\"%s\"", field.Min)
		}
		if field.Max != "" {
			maxAttr = fmt.Sprintf(" maxlength=\"%s\"", field.Max)
		}
		b.WriteString(fmt.Sprintf("        <input type=\"text\" id=\"%s\" name=\"%s\" value=\"%s\"%s%s%s>\n",
			name, name, html.EscapeString(value), required, minAttr, maxAttr))

	case parser.FieldEmail:
		b.WriteString(fmt.Sprintf("        <input type=\"email\" id=\"%s\" name=\"%s\" value=\"%s\"%s>\n",
			name, name, html.EscapeString(value), required))

	case parser.FieldPassword:
		b.WriteString(fmt.Sprintf("        <input type=\"password\" id=\"%s\" name=\"%s\"%s>\n",
			name, name, required))

	case parser.FieldInt:
		b.WriteString(fmt.Sprintf("        <input type=\"number\" id=\"%s\" name=\"%s\" value=\"%s\"%s>\n",
			name, name, html.EscapeString(value), required))

	case parser.FieldFloat:
		b.WriteString(fmt.Sprintf("        <input type=\"number\" step=\"any\" id=\"%s\" name=\"%s\" value=\"%s\"%s>\n",
			name, name, html.EscapeString(value), required))

	case parser.FieldBool:
		checked := ""
		if value == "1" || value == "true" {
			checked = " checked"
		}
		b.WriteString(fmt.Sprintf("        <input type=\"checkbox\" id=\"%s\" name=\"%s\" value=\"1\"%s>\n",
			name, name, checked))

	case parser.FieldRichtext:
		b.WriteString(fmt.Sprintf("        <textarea id=\"%s\" name=\"%s\" rows=\"6\"%s>%s</textarea>\n",
			name, name, required, html.EscapeString(value)))

	case parser.FieldImage:
		b.WriteString(fmt.Sprintf("        <input type=\"file\" id=\"%s\" name=\"%s\" accept=\"image/*\"%s>\n",
			name, name, required))

	case parser.FieldOption:
		b.WriteString(fmt.Sprintf("        <select id=\"%s\" name=\"%s\"%s>\n", name, name, required))
		for _, opt := range field.Options {
			selected := ""
			if opt == value || (value == "" && opt == field.Default) {
				selected = " selected"
			}
			b.WriteString(fmt.Sprintf("          <option value=\"%s\"%s>%s</option>\n",
				html.EscapeString(opt), selected, html.EscapeString(opt)))
		}
		b.WriteString("        </select>\n")

	default:
		b.WriteString(fmt.Sprintf("        <input type=\"text\" id=\"%s\" name=\"%s\" value=\"%s\"%s>\n",
			name, name, html.EscapeString(value), required))
	}

	b.WriteString("      </div>\n")
	return b.String()
}

// renderReferenceField renders a <select> dropdown for a reference field
// by querying all rows from the referenced model's table
func renderReferenceField(field parser.Field, prefill database.Row, db *database.DB) string {
	var b strings.Builder
	name := field.Name
	label := strings.ToUpper(name[:1]) + name[1:]
	value := ""
	if prefill != nil {
		if v, ok := prefill[name]; ok {
			value = v
		}
	}

	required := ""
	if field.Required {
		required = " required"
	}

	b.WriteString("      <div class=\"kilnx-field\">\n")
	b.WriteString(fmt.Sprintf("        <label for=\"%s\">%s</label>\n", name, html.EscapeString(label)))
	b.WriteString(fmt.Sprintf("        <select id=\"%s\" name=\"%s\"%s>\n", name, name, required))
	b.WriteString("          <option value=\"\">-- Select --</option>\n")

	if db != nil && field.Reference != "" {
		sql := fmt.Sprintf("SELECT id, name FROM \"%s\" ORDER BY name", field.Reference)
		rows, err := db.QueryRows(sql)
		if err == nil {
			for _, row := range rows {
				id := row["id"]
				displayName := row["name"]
				if displayName == "" {
					// Fallback: use id as display
					displayName = id
				}
				selected := ""
				if id == value {
					selected = " selected"
				}
				b.WriteString(fmt.Sprintf("          <option value=\"%s\"%s>%s</option>\n",
					html.EscapeString(id), selected, html.EscapeString(displayName)))
			}
		}
	}

	b.WriteString("        </select>\n")
	b.WriteString("      </div>\n")
	return b.String()
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

		if field.Min != "" && val != "" {
			var min int
			fmt.Sscanf(field.Min, "%d", &min)
			if len(val) < min {
				label := strings.ToUpper(field.Name[:1]) + field.Name[1:]
				errors = append(errors, fmt.Sprintf("%s must be at least %d characters", label, min))
			}
		}

		if field.Max != "" && val != "" {
			var max int
			fmt.Sscanf(field.Max, "%d", &max)
			if max > 0 && len(val) > max {
				label := strings.ToUpper(field.Name[:1]) + field.Name[1:]
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

// extractPathParams extracts :param values from URL path
func extractPathParams(r *http.Request) map[string]string {
	params := make(map[string]string)
	parts := strings.Split(r.URL.Path, "/")
	for i, part := range parts {
		if part != "" {
			params[fmt.Sprintf("p%d", i)] = part
		}
	}
	// Common pattern: /model/id -> extract "id"
	if len(parts) >= 3 {
		params["id"] = parts[len(parts)-1]
	}
	return params
}

// extractFormData reads form values from a POST request, including file uploads
func extractFormData(r *http.Request) map[string]string {
	contentType := r.Header.Get("Content-Type")
	data := make(map[string]string)

	if strings.Contains(contentType, "multipart/form-data") {
		// Parse multipart form with 32MB max
		r.ParseMultipartForm(32 << 20)
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

					// Determine uploads directory
					uploadsDir := "uploads"
					fileName := fmt.Sprintf("%d_%s", time.Now().UnixNano(), fileHeaders[0].Filename)
					filePath := uploadsDir + "/" + fileName

					// Create uploads directory if needed
					os.MkdirAll(uploadsDir, 0755)

					dst, err := os.Create(filePath)
					if err != nil {
						continue
					}
					defer dst.Close()
					io.Copy(dst, file)

					data[key] = filePath
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
