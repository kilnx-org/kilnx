package analyzer

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kilnx-org/kilnx/internal/parser"
)

// checkSecurity performs security gap analysis on the app.
// It detects missing auth, exposed sensitive fields, unprotected webhooks,
// and other common security misconfigurations.
func checkSecurity(app *parser.App, schema *Schema) []Diagnostic {
	var diags []Diagnostic

	diags = append(diags, checkUnauthActions(app)...)
	diags = append(diags, checkUnauthAPIs(app)...)
	diags = append(diags, checkUnauthStreams(app)...)
	diags = append(diags, checkUnauthSockets(app)...)
	diags = append(diags, checkWebhookSecrets(app)...)
	diags = append(diags, checkPasswordExposure(app, schema)...)
	diags = append(diags, checkAuthWithoutPermissions(app)...)
	diags = append(diags, checkCSRFProtection(app)...)

	return diags
}

// checkUnauthActions warns when actions (POST/PUT/DELETE) don't require auth.
func checkUnauthActions(app *parser.App) []Diagnostic {
	var diags []Diagnostic
	for _, a := range app.Actions {
		if a.Auth || a.RequiresRole != "" {
			continue
		}
		method := a.Method
		if method == "" {
			method = "POST"
		}
		diags = append(diags, Diagnostic{
			Level:   "warning",
			Message: fmt.Sprintf("%s action has no authentication requirement; any visitor can trigger it", method),
			Context: fmt.Sprintf("action %s", a.Path),
		})
	}
	return diags
}

// checkUnauthAPIs warns when API endpoints that write data don't require auth.
func checkUnauthAPIs(app *parser.App) []Diagnostic {
	var diags []Diagnostic
	for _, api := range app.APIs {
		if api.Auth || api.RequiresRole != "" {
			continue
		}
		if hasMutatingSQL(api.Body) {
			diags = append(diags, Diagnostic{
				Level:   "warning",
				Message: "API endpoint with write queries has no authentication requirement",
				Context: fmt.Sprintf("api %s", api.Path),
			})
		}
	}
	return diags
}

// checkUnauthStreams warns when streams don't require auth.
func checkUnauthStreams(app *parser.App) []Diagnostic {
	var diags []Diagnostic
	for _, s := range app.Streams {
		if s.Auth || s.RequiresRole != "" {
			continue
		}
		diags = append(diags, Diagnostic{
			Level:   "warning",
			Message: "stream has no authentication requirement; data is publicly accessible via SSE",
			Context: fmt.Sprintf("stream %s", s.Path),
		})
	}
	return diags
}

// checkUnauthSockets warns when sockets don't require auth.
func checkUnauthSockets(app *parser.App) []Diagnostic {
	var diags []Diagnostic
	for _, s := range app.Sockets {
		if s.Auth || s.RequiresRole != "" {
			continue
		}
		if hasMutatingSQL(s.OnConnect) || hasMutatingSQL(s.OnMessage) || hasMutatingSQL(s.OnDisconnect) {
			diags = append(diags, Diagnostic{
				Level:   "warning",
				Message: "socket with write queries has no authentication requirement",
				Context: fmt.Sprintf("socket %s", s.Path),
			})
		}
	}
	return diags
}

// checkWebhookSecrets warns when webhooks don't have a secret configured.
func checkWebhookSecrets(app *parser.App) []Diagnostic {
	var diags []Diagnostic
	for _, wh := range app.Webhooks {
		if wh.SecretEnv == "" {
			diags = append(diags, Diagnostic{
				Level:   "warning",
				Message: "webhook has no secret configured; requests cannot be verified as authentic",
				Context: fmt.Sprintf("webhook %s", wh.Path),
			})
		}
	}
	return diags
}

// checkPasswordExposure warns when queries in public-facing endpoints
// select password fields from the auth table.
func checkPasswordExposure(app *parser.App, schema *Schema) []Diagnostic {
	var diags []Diagnostic

	// Build set of tables that have password fields
	passwordTables := make(map[string][]string) // table -> password column names
	for _, m := range schema.Tables {
		for colName, col := range m.Columns {
			if col.FieldType == parser.FieldPassword {
				passwordTables[m.Name] = append(passwordTables[m.Name], colName)
			}
		}
	}
	for k := range passwordTables {
		sort.Strings(passwordTables[k])
	}
	if len(passwordTables) == 0 {
		return nil
	}

	// Check pages (public GET endpoints)
	for _, p := range app.Pages {
		ctx := fmt.Sprintf("page %s", p.Path)
		diags = append(diags, checkNodesForPasswordExposure(p.Body, passwordTables, schema, ctx)...)
	}

	// Check fragments
	for _, f := range app.Fragments {
		ctx := fmt.Sprintf("fragment %s", f.Path)
		diags = append(diags, checkNodesForPasswordExposure(f.Body, passwordTables, schema, ctx)...)
	}

	// Check APIs
	for _, a := range app.APIs {
		ctx := fmt.Sprintf("api %s", a.Path)
		diags = append(diags, checkNodesForPasswordExposure(a.Body, passwordTables, schema, ctx)...)
	}

	// Check actions
	for _, a := range app.Actions {
		ctx := fmt.Sprintf("action %s", a.Path)
		diags = append(diags, checkNodesForPasswordExposure(a.Body, passwordTables, schema, ctx)...)
	}

	return diags
}

// checkAuthWithoutPermissions warns when auth is configured but no permissions are defined.
func checkAuthWithoutPermissions(app *parser.App) []Diagnostic {
	if app.Auth == nil {
		return nil
	}
	if len(app.Permissions) > 0 {
		return nil
	}
	// Don't warn if any route already uses RequiresRole (per-route access control)
	for _, p := range app.Pages {
		if p.RequiresRole != "" {
			return nil
		}
	}
	for _, a := range app.Actions {
		if a.RequiresRole != "" {
			return nil
		}
	}
	for _, a := range app.APIs {
		if a.RequiresRole != "" {
			return nil
		}
	}
	return []Diagnostic{{
		Level:   "warning",
		Message: "auth is configured but no permissions or role requirements are defined; all authenticated users have equal access",
		Context: "auth",
	}}
}

// hasMutatingSQL checks if any nodes contain INSERT, UPDATE, or DELETE queries.
func hasMutatingSQL(nodes []parser.Node) bool {
	for _, n := range nodes {
		if n.Type == parser.NodeQuery && n.SQL != "" {
			upper := strings.ToUpper(strings.TrimSpace(n.SQL))
			if strings.HasPrefix(upper, "INSERT") || strings.HasPrefix(upper, "UPDATE") || strings.HasPrefix(upper, "DELETE") || strings.HasPrefix(upper, "REPLACE") {
				return true
			}
		}
		if n.Type == parser.NodeOn {
			if hasMutatingSQL(n.Children) {
				return true
			}
		}
	}
	return false
}

// checkCSRFProtection warns when a page has a raw HTML form targeting a
// mutating action instead of using the Kilnx form keyword (which auto-adds
// a CSRF token). Only pages whose path matches an action's path are checked.
func checkCSRFProtection(app *parser.App) []Diagnostic {
	// Build a set of action paths that accept POST/PUT/DELETE.
	actionPaths := make(map[string]string) // path -> method
	for _, a := range app.Actions {
		method := a.Method
		if method == "" {
			method = "POST"
		}
		upper := strings.ToUpper(method)
		if upper == "POST" || upper == "PUT" || upper == "DELETE" {
			actionPaths[a.Path] = upper
		}
	}
	if len(actionPaths) == 0 {
		return nil
	}

	var diags []Diagnostic
	for _, p := range app.Pages {
		method, ok := actionPaths[p.Path]
		if !ok {
			continue
		}
		if hasRawHTMLForm(p.Body) && !hasFormNode(p.Body) {
			diags = append(diags, Diagnostic{
				Level:   "warning",
				Message: fmt.Sprintf("page has a raw HTML <form> targeting a %s action; use the 'form' keyword instead so Kilnx auto-adds a CSRF token", method),
				Context: fmt.Sprintf("page %s", p.Path),
			})
		}
	}
	return diags
}

// hasRawHTMLForm checks if any NodeHTML in the tree contains a <form tag.
func hasRawHTMLForm(nodes []parser.Node) bool {
	for _, n := range nodes {
		if n.Type == parser.NodeHTML {
			lower := strings.ToLower(n.HTMLContent)
			if strings.Contains(lower, "<form") {
				return true
			}
		}
		if hasRawHTMLForm(n.Children) {
			return true
		}
	}
	return false
}

// hasFormNode checks if any node in the tree uses the Kilnx form keyword.
func hasFormNode(nodes []parser.Node) bool {
	for _, n := range nodes {
		if n.Type == parser.NodeForm {
			return true
		}
		if hasFormNode(n.Children) {
			return true
		}
	}
	return false
}

// checkNodesForPasswordExposure checks if query nodes expose password columns.
func checkNodesForPasswordExposure(nodes []parser.Node, passwordTables map[string][]string, schema *Schema, context string) []Diagnostic {
	var diags []Diagnostic
	for _, n := range nodes {
		if n.Type != parser.NodeQuery || n.SQL == "" {
			continue
		}
		tokens := tokenizeSQL(n.SQL)
		if len(tokens) == 0 {
			continue
		}
		if tokens[0].typ != stKeyword || tokens[0].lower != "select" {
			continue
		}

		tableRefs := extractTableRefs(tokens)

		// Check if any referenced table has a password field
		for _, ref := range tableRefs {
			pwCols, hasPw := passwordTables[ref.name]
			if !hasPw {
				continue
			}

			// SELECT * from a table with password is always a warning
			cols := extractSelectColumns(tokens)
			if cols == nil {
				// nil means SELECT * was used
				diags = append(diags, Diagnostic{
					Level:   "warning",
					Message: fmt.Sprintf("SELECT * from '%s' exposes the '%s' field; select specific columns instead", ref.name, strings.Join(pwCols, "', '")),
					Context: context,
				})
				break
			}

			// Check if any password column is explicitly selected
			for _, col := range cols {
				for _, pwCol := range pwCols {
					if col.column == pwCol {
						diags = append(diags, Diagnostic{
							Level:   "warning",
							Message: fmt.Sprintf("query selects '%s' from '%s' which is a password field; this should not be exposed", pwCol, ref.name),
							Context: context,
						})
					}
				}
			}
		}
	}
	return diags
}
