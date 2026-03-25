package runtime

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html"
	"net/http"
	"strings"
	"sync"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/parser"
)

// CSRF token store (in-memory, good enough for single binary)
var (
	csrfTokens   = make(map[string]bool)
	csrfTokensMu sync.Mutex
)

func generateCSRFToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	token := hex.EncodeToString(b)
	csrfTokensMu.Lock()
	csrfTokens[token] = true
	csrfTokensMu.Unlock()
	return token
}

func validateCSRFToken(token string) bool {
	csrfTokensMu.Lock()
	defer csrfTokensMu.Unlock()
	if csrfTokens[token] {
		delete(csrfTokens, token) // single use
		return true
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

	var b strings.Builder
	b.WriteString("    <form method=\"POST\" class=\"kilnx-form\">\n")
	b.WriteString(fmt.Sprintf("      <input type=\"hidden\" name=\"_csrf\" value=\"%s\">\n", csrfToken))

	for _, field := range model.Fields {
		// Skip auto fields (timestamp auto, etc.) and password on edit forms
		if field.Auto {
			continue
		}
		// Skip reference fields for now (will need dropdowns later)
		if field.Type == parser.FieldReference {
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

// extractFormData reads form values from a POST request
func extractFormData(r *http.Request) map[string]string {
	r.ParseForm()
	data := make(map[string]string)
	for key, values := range r.PostForm {
		if len(values) > 0 {
			data[key] = values[0]
		}
	}
	return data
}
