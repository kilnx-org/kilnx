package analyzer

import (
	"fmt"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/parser"
)

// AnalyzeWithDB runs the standard Analyze pass plus a DB-connected pass that
// validates {q.custom.fieldName} references against the actual _field_defs tables.
// db may be nil, in which case this is equivalent to Analyze.
func AnalyzeWithDB(app *parser.App, db *database.DB) []Diagnostic {
	diags := Analyze(app)
	if db == nil {
		return diags
	}
	return append(diags, checkDynamicFieldRefsWithDB(app, db)...)
}

// checkDynamicFieldRefsWithDB queries each _<model>_field_defs table and
// validates hardcoded {q.custom.X} references against the actual field names.
func checkDynamicFieldRefsWithDB(app *parser.App, db *database.DB) []Diagnostic {
	// Build map of modelName -> known field names from DB + static manifests.
	knownFields := make(map[string]map[string]bool)
	for _, m := range app.Models {
		if !m.DynamicFields {
			continue
		}
		table := "_" + m.Name + "_field_defs"
		rows, err := db.QueryRows(fmt.Sprintf(`SELECT "name" FROM "%s"`, table))
		if err != nil {
			// Table may not exist; skip this model.
			continue
		}
		names := make(map[string]bool)
		for _, row := range rows {
			if n := row["name"]; n != "" {
				names[n] = true
			}
		}
		// Static manifest fields shadow DB rows; include them as known.
		if manifest, ok := app.CustomManifests[m.Name]; ok {
			for _, f := range manifest.Fields {
				names[f.Name] = true
			}
		}
		knownFields[m.Name] = names
	}
	if len(knownFields) == 0 {
		return nil
	}

	qMap := queryModelMap(app.Pages, app.Fragments, app.APIs)
	var diags []Diagnostic

	scanHTML := func(htmlContent, context string) {
		for _, match := range customFieldRefRe.FindAllStringSubmatch(htmlContent, -1) {
			queryName, fieldName := match[1], match[2]
			modelName, ok := qMap[queryName]
			if !ok {
				continue
			}
			fields, ok := knownFields[modelName]
			if !ok {
				continue
			}
			if !fields[fieldName] {
				diags = append(diags, Diagnostic{
					Level:   "error",
					Message: fmt.Sprintf("template reference '{%s.custom.%s}': field '%s' not found in _field_defs for model '%s'", queryName, fieldName, fieldName, modelName),
					Context: context,
				})
			}
		}
	}

	scanNodes := func(nodes []parser.Node, context string) {
		for _, n := range nodes {
			if n.Type == parser.NodeHTML {
				scanHTML(n.HTMLContent, context)
			}
		}
	}

	for _, p := range app.Pages {
		scanNodes(p.Body, fmt.Sprintf("page %s", p.Path))
	}
	for _, f := range app.Fragments {
		scanNodes(f.Body, fmt.Sprintf("fragment %s", f.Path))
	}
	for _, a := range app.APIs {
		scanNodes(a.Body, fmt.Sprintf("api %s", a.Path))
	}

	return diags
}
