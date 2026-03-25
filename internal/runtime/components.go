package runtime

import (
	"fmt"
	"html"
	"strings"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/parser"
)

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

	// Pagination controls
	if paginateInfo, ok := ctx.paginate[node.Name]; ok {
		b.WriteString(renderPagination(paginateInfo, currentPath))
	}

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
	Page     int
	PerPage  int
	Total    int
	HasPrev  bool
	HasNext  bool
}

func renderPagination(info PaginateInfo, currentPath string) string {
	if !info.HasPrev && !info.HasNext {
		return ""
	}

	var b strings.Builder
	b.WriteString("    <div class=\"kilnx-pagination\">\n")

	if info.HasPrev {
		b.WriteString(fmt.Sprintf("      <a href=\"%s?page=%d\" hx-get=\"%s?page=%d\" hx-target=\"main\" hx-push-url=\"true\">&laquo; Previous</a>\n",
			currentPath, info.Page-1, currentPath, info.Page-1))
	} else {
		b.WriteString("      <span class=\"disabled\">&laquo; Previous</span>\n")
	}

	totalPages := (info.Total + info.PerPage - 1) / info.PerPage
	b.WriteString(fmt.Sprintf("      <span class=\"kilnx-page-info\">Page %d of %d</span>\n", info.Page, totalPages))

	if info.HasNext {
		b.WriteString(fmt.Sprintf("      <a href=\"%s?page=%d\" hx-get=\"%s?page=%d\" hx-target=\"main\" hx-push-url=\"true\">Next &raquo;</a>\n",
			currentPath, info.Page+1, currentPath, info.Page+1))
	} else {
		b.WriteString("      <span class=\"disabled\">Next &raquo;</span>\n")
	}

	b.WriteString("    </div>\n")
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
