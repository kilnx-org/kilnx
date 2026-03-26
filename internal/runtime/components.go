package runtime

import (
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strings"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/parser"
)

var componentPlaceholderRe = regexp.MustCompile(`\{(table|search|form|paginate|list|alert)\s+([^}]*)\}`)

// resolveComponentPlaceholders replaces {table name}, {search name}, {form name},
// {paginate name} placeholders in HTML content with rendered components.
func resolveComponentPlaceholders(content string, page parser.Page, ctx *renderContext, app *parser.App, db *database.DB, r *http.Request, pagePath string) string {
	// First apply data interpolation for {query.field} values
	result := interpolate(content, ctx)

	// Then resolve component placeholders
	result = componentPlaceholderRe.ReplaceAllStringFunc(result, func(match string) string {
		parts := componentPlaceholderRe.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		keyword := parts[1]
		args := strings.TrimSpace(parts[2])
		name := args
		// Extract name (first word) from args
		if idx := strings.Index(args, " "); idx > 0 {
			name = args[:idx]
		}

		switch keyword {
		case "table":
			node := findNodeByName(page.Body, parser.NodeTable, name)
			if node == nil {
				node = &parser.Node{Type: parser.NodeTable, Name: name}
			}
			return renderTable(*node, ctx, pagePath)

		case "search":
			node := findNodeByName(page.Body, parser.NodeSearch, name)
			if node == nil {
				node = &parser.Node{Type: parser.NodeSearch, Name: name}
			}
			return renderSearch(*node, pagePath)

		case "form":
			node := findNodeByModelName(page.Body, parser.NodeForm, name)
			if node == nil {
				node = &parser.Node{Type: parser.NodeForm, ModelName: name}
			}
			return renderForm(*node, app, db, r)

		case "paginate":
			if info, ok := ctx.paginate[name]; ok {
				return renderPagination(info, pagePath)
			}
			return ""

		case "list":
			node := findNodeByName(page.Body, parser.NodeList, name)
			if node == nil {
				node = &parser.Node{Type: parser.NodeList, Name: name}
			}
			return renderList(*node, ctx)

		case "alert":
			// {alert info "message"} or {alert error "message"}
			alertType := name
			msg := ""
			if idx := strings.Index(args, " "); idx > 0 {
				rest := strings.TrimSpace(args[idx+1:])
				msg = strings.Trim(rest, "\"")
			}
			return fmt.Sprintf("<div class=\"kilnx-alert kilnx-alert-%s\">%s</div>\n",
				html.EscapeString(alertType), html.EscapeString(msg))

		default:
			return match
		}
	})

	return result
}

// findNodeByName finds a node in the body by type and name
func findNodeByName(body []parser.Node, nodeType parser.NodeType, name string) *parser.Node {
	for i := range body {
		if body[i].Type == nodeType && body[i].Name == name {
			return &body[i]
		}
	}
	return nil
}

// findNodeByModelName finds a form node by model name
func findNodeByModelName(body []parser.Node, nodeType parser.NodeType, modelName string) *parser.Node {
	for i := range body {
		if body[i].Type == nodeType && body[i].ModelName == modelName {
			return &body[i]
		}
	}
	return nil
}

// renderList renders a list component from query results.
// Syntax:
//
//	list users
//	  title: name
//	  subtitle: email
func renderList(node parser.Node, ctx *renderContext) string {
	rows, ok := ctx.queries[node.Name]
	if !ok {
		return fmt.Sprintf("    <p style=\"color:#888\">No data for list '%s'</p>\n", node.Name)
	}

	if len(rows) == 0 {
		return "    <p style=\"color:#888\">No items found.</p>\n"
	}

	titleField := node.Props["title"]
	subtitleField := node.Props["subtitle"]

	var b strings.Builder
	b.WriteString("    <ul class=\"kilnx-list\">\n")

	for _, row := range rows {
		title := getField(row, titleField)
		subtitle := getField(row, subtitleField)

		b.WriteString("      <li class=\"kilnx-list-item\">\n")
		if title != "" {
			b.WriteString(fmt.Sprintf("        <strong>%s</strong>\n", html.EscapeString(title)))
		}
		if subtitle != "" {
			b.WriteString(fmt.Sprintf("        <span>%s</span>\n", html.EscapeString(subtitle)))
		}
		b.WriteString("      </li>\n")
	}

	b.WriteString("    </ul>\n")
	return b.String()
}

// renderTable renders a table component from query results.
// Syntax:
//
//	table users
//	  columns: name, email as "Email", created
//	  row action: edit /users/:id/edit
func renderTable(node parser.Node, ctx *renderContext, currentPath string) string {
	rows, ok := ctx.queries[node.Name]
	if !ok {
		return fmt.Sprintf("    <p style=\"color:#888\">No data for table '%s'</p>\n", node.Name)
	}

	columns := node.Columns

	// If no columns specified, auto-detect from first row
	if len(columns) == 0 && len(rows) > 0 {
		for key := range rows[0] {
			columns = append(columns, parser.TableColumn{Field: key})
		}
	}

	if len(rows) == 0 {
		return "    <p style=\"color:#888\">No items found.</p>\n"
	}

	var b strings.Builder
	b.WriteString("    <table class=\"kilnx-table\">\n")

	// Header
	b.WriteString("      <thead><tr>\n")
	for _, col := range columns {
		label := col.Label
		if label == "" {
			label = col.Field
		}
		b.WriteString(fmt.Sprintf("        <th>%s</th>\n", html.EscapeString(label)))
	}
	if len(node.RowActions) > 0 {
		b.WriteString("        <th>Actions</th>\n")
	}
	b.WriteString("      </tr></thead>\n")

	// Body
	b.WriteString("      <tbody>\n")
	for _, row := range rows {
		b.WriteString("      <tr>\n")
		for _, col := range columns {
			val := getField(row, col.Field)
			b.WriteString(fmt.Sprintf("        <td>%s</td>\n", html.EscapeString(val)))
		}
		if len(node.RowActions) > 0 {
			b.WriteString("        <td>")
			for i, action := range node.RowActions {
				if i > 0 {
					b.WriteString(" ")
				}
				path := interpolateRowPath(action.Path, row)
				b.WriteString(fmt.Sprintf("<a href=\"%s\">%s</a>",
					html.EscapeString(path), html.EscapeString(action.Label)))
			}
			b.WriteString("</td>\n")
		}
		b.WriteString("      </tr>\n")
	}
	b.WriteString("      </tbody>\n")

	b.WriteString("    </table>\n")



	return b.String()
}

// renderAlert renders an alert message.
// Syntax: alert success "Operation completed"
func renderAlert(node parser.Node, ctx *renderContext) string {
	level := node.Props["level"]
	if level == "" {
		level = "info"
	}
	message := interpolate(node.Value, ctx)

	return fmt.Sprintf("    <div class=\"kilnx-alert kilnx-alert-%s\">%s</div>\n",
		html.EscapeString(level), html.EscapeString(message))
}

// PaginateInfo holds pagination state for a query
type PaginateInfo struct {
	Page    int
	PerPage int
	Total   int
	HasPrev bool
	HasNext bool
}

func renderPagination(info PaginateInfo, currentPath string) string {
	if !info.HasPrev && !info.HasNext {
		return ""
	}

	var b strings.Builder

	if info.HasPrev {
		b.WriteString(fmt.Sprintf("<a href=\"%s?page=%d\" hx-get=\"%s?page=%d\" hx-target=\"main\" hx-push-url=\"true\" class=\"kilnx-page-link\">&laquo; Previous</a> ",
			currentPath, info.Page-1, currentPath, info.Page-1))
	} else {
		b.WriteString("<span class=\"disabled\">&laquo; Previous</span> ")
	}

	totalPages := (info.Total + info.PerPage - 1) / info.PerPage
	b.WriteString(fmt.Sprintf("<span class=\"kilnx-page-info\">Page %d of %d</span> ", info.Page, totalPages))

	if info.HasNext {
		b.WriteString(fmt.Sprintf("<a href=\"%s?page=%d\" hx-get=\"%s?page=%d\" hx-target=\"main\" hx-push-url=\"true\" class=\"kilnx-page-link\">Next &raquo;</a>",
			currentPath, info.Page+1, currentPath, info.Page+1))
	} else {
		b.WriteString("<span class=\"disabled\">Next &raquo;</span>")
	}

	return b.String()
}

// interpolateRowPath replaces :id, :name etc. in a path with row values
func interpolateRowPath(path string, row database.Row) string {
	result := path
	for key, val := range row {
		result = strings.ReplaceAll(result, ":"+key, val)
	}
	return result
}

// renderSearch renders a search input that filters a query via htmx.
// The search input sends a GET request to the current page with ?q=term,
// and the server filters the query results using LIKE on the specified fields.
func renderSearch(node parser.Node, currentPath string) string {
	placeholder := "Search"
	if len(node.SearchFields) > 0 {
		placeholder = "Search " + strings.Join(node.SearchFields, ", ")
	}

	return fmt.Sprintf(`    <input type="search" name="q" placeholder="%s"
        hx-get="%s" hx-trigger="input changed delay:300ms, search"
        hx-target="main" hx-push-url="true"
        hx-include="this" autocomplete="off" class="kilnx-search-input">
`, html.EscapeString(placeholder), html.EscapeString(currentPath))
}

// renderCard renders a card component from query results.
// Props: title, subtitle, image, action_label, action_path
func renderCard(node parser.Node, ctx *renderContext) string {
	rows, ok := ctx.queries[node.Name]
	if !ok {
		return fmt.Sprintf("    <p style=\"color:#888\">No data for card '%s'</p>\n", node.Name)
	}

	if len(rows) == 0 {
		return "    <p style=\"color:#888\">No items found.</p>\n"
	}

	titleField := node.Props["title"]
	subtitleField := node.Props["subtitle"]
	imageField := node.Props["image"]
	actionLabel := node.Props["action_label"]
	actionPath := node.Props["action_path"]

	var b strings.Builder
	b.WriteString("    <div class=\"kilnx-cards\">\n")

	for _, row := range rows {
		b.WriteString("      <div class=\"kilnx-card\">\n")
		if imageField != "" {
			img := getField(row, imageField)
			if img != "" {
				b.WriteString(fmt.Sprintf("        <img src=\"%s\" class=\"kilnx-card-img\" alt=\"\">\n",
					html.EscapeString(img)))
			}
		}
		b.WriteString("        <div class=\"kilnx-card-body\">\n")
		if titleField != "" {
			title := getField(row, titleField)
			b.WriteString(fmt.Sprintf("          <h3 class=\"kilnx-card-title\">%s</h3>\n",
				html.EscapeString(title)))
		}
		if subtitleField != "" {
			subtitle := getField(row, subtitleField)
			b.WriteString(fmt.Sprintf("          <p class=\"kilnx-card-subtitle\">%s</p>\n",
				html.EscapeString(subtitle)))
		}
		if actionLabel != "" && actionPath != "" {
			path := interpolateRowPath(actionPath, row)
			b.WriteString(fmt.Sprintf("          <a href=\"%s\" class=\"kilnx-card-action\">%s</a>\n",
				html.EscapeString(path), html.EscapeString(actionLabel)))
		}
		b.WriteString("        </div>\n")
		b.WriteString("      </div>\n")
	}

	b.WriteString("    </div>\n")
	return b.String()
}

// renderModal wraps content in a modal dialog, opened/closed with htmx
func renderModal(id, title, content string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("    <div id=\"%s\" class=\"kilnx-modal\" style=\"display:none\">\n", html.EscapeString(id)))
	b.WriteString("      <div class=\"kilnx-modal-overlay\" onclick=\"this.parentElement.style.display='none'\"></div>\n")
	b.WriteString("      <div class=\"kilnx-modal-content\">\n")
	b.WriteString("        <div class=\"kilnx-modal-header\">\n")
	b.WriteString(fmt.Sprintf("          <h3>%s</h3>\n", html.EscapeString(title)))
	b.WriteString("          <button onclick=\"this.closest('.kilnx-modal').style.display='none'\" class=\"kilnx-modal-close\">&times;</button>\n")
	b.WriteString("        </div>\n")
	b.WriteString("        <div class=\"kilnx-modal-body\">\n")
	b.WriteString("          " + content + "\n")
	b.WriteString("        </div>\n")
	b.WriteString("      </div>\n")
	b.WriteString("    </div>\n")
	return b.String()
}

// renderChart renders a simple CSS bar chart from query data.
// Props: type (bar), label (field for labels), value (field for values)
func renderChart(node parser.Node, ctx *renderContext) string {
	rows, ok := ctx.queries[node.Name]
	if !ok {
		return fmt.Sprintf("    <p style=\"color:#888\">No data for chart '%s'</p>\n", node.Name)
	}

	if len(rows) == 0 {
		return "    <p style=\"color:#888\">No items found.</p>\n"
	}

	labelField := node.Props["label"]
	valueField := node.Props["value"]
	chartType := node.Props["type"]
	if chartType == "" {
		chartType = "bar"
	}

	// Find max value for scaling
	maxVal := 0
	type entry struct {
		label string
		value int
	}
	var entries []entry
	for _, row := range rows {
		label := getField(row, labelField)
		valStr := getField(row, valueField)
		val := 0
		fmt.Sscanf(valStr, "%d", &val)
		if val > maxVal {
			maxVal = val
		}
		entries = append(entries, entry{label: label, value: val})
	}
	if maxVal == 0 {
		maxVal = 1
	}

	var b strings.Builder
	b.WriteString("    <div class=\"kilnx-chart\" data-type=\"" + html.EscapeString(chartType) + "\">\n")
	b.WriteString("      <table class=\"kilnx-chart-table\" style=\"width:100%;border-collapse:collapse\">\n")

	for _, e := range entries {
		pct := (e.value * 100) / maxVal
		b.WriteString("        <tr>\n")
		b.WriteString(fmt.Sprintf("          <td style=\"width:120px;padding:4px 8px;font-size:0.85rem\">%s</td>\n",
			html.EscapeString(e.label)))
		b.WriteString(fmt.Sprintf("          <td style=\"padding:4px\" data-value=\"%d\">\n", e.value))
		b.WriteString(fmt.Sprintf("            <div style=\"background:#4a7aba;height:20px;width:%d%%;border-radius:3px;min-width:2px\" title=\"%d\"></div>\n",
			pct, e.value))
		b.WriteString("          </td>\n")
		b.WriteString(fmt.Sprintf("          <td style=\"width:50px;padding:4px;font-size:0.85rem;text-align:right\">%d</td>\n", e.value))
		b.WriteString("        </tr>\n")
	}

	b.WriteString("      </table>\n")
	b.WriteString("    </div>\n")
	return b.String()
}

func getField(row database.Row, field string) string {
	if val, ok := row[field]; ok {
		return val
	}
	// Try with dot notation: author.name -> look for "author.name" or just "name"
	if parts := strings.SplitN(field, ".", 2); len(parts) == 2 {
		if val, ok := row[parts[1]]; ok {
			return val
		}
		if val, ok := row[field]; ok {
			return val
		}
	}
	return ""
}
