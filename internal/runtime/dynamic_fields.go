package runtime

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/parser"
)

var fieldDefsTableRe = regexp.MustCompile(`(?i)\b_([a-zA-Z_][a-zA-Z0-9_]*)_field_defs\b`)

// mergeDBFieldDefs queries _<model>_field_defs and merges rows into base.
// Static fields always win on name collision.
func (s *Server) mergeDBFieldDefs(modelName string, base *parser.CustomFieldManifest) *parser.CustomFieldManifest {
	merged := &parser.CustomFieldManifest{ModelName: modelName}
	staticNames := make(map[string]bool, len(base.Fields))
	for _, f := range base.Fields {
		merged.Fields = append(merged.Fields, f)
		staticNames[f.Name] = true
	}

	sql := fmt.Sprintf(
		`SELECT "name","kind","label","required","options","reference_model" FROM "_%s_field_defs" ORDER BY "sort_order","id"`,
		modelName)
	rows, err := s.db.QueryRows(sql)
	if err != nil {
		// Table may not exist yet (migration not run); degrade gracefully.
		return base
	}

	for _, row := range rows {
		name := row["name"]
		if name == "" || staticNames[name] || !database.IsValidIdentifier(name) {
			continue
		}
		def := parser.CustomFieldDef{
			Name:      name,
			Kind:      parser.CustomFieldKind(row["kind"]),
			Label:     row["label"],
			Required:  row["required"] == "1" || row["required"] == "true",
			Reference: row["reference_model"],
		}
		if optJSON := row["options"]; optJSON != "" {
			var opts []string
			if json.Unmarshal([]byte(optJSON), &opts) == nil {
				def.Options = opts
			}
		}
		merged.Fields = append(merged.Fields, def)
		staticNames[name] = true
	}
	return merged
}

// invalidateDynamicManifestCache clears the manifest cache entry when a
// mutation targets a _<model>_field_defs table.
func (s *Server) invalidateDynamicManifestCache(sql string) {
	if m := fieldDefsTableRe.FindStringSubmatch(sql); len(m) > 1 {
		s.manifestCache.Delete("__dynamic__:" + m[1])
	}
}

// modelHasCustomFields reports whether a model has any custom field support
// (static manifest or dynamic fields).
func (s *Server) modelHasCustomFields(app *parser.App, modelName string) bool {
	if _, ok := app.CustomManifests[modelName]; ok {
		return true
	}
	for _, m := range app.Models {
		if m.Name == modelName {
			return m.DynamicFields
		}
	}
	return false
}
