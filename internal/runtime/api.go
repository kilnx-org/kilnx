package runtime

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/parser"
)

// handleAPI processes an API endpoint and returns JSON.
// For mutation methods (POST/PUT/DELETE), supports validate, transactions, and redirect.
func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request, endpoint parser.Page) {
	// CORS headers: only allow configured origins
	origin := r.Header.Get("Origin")
	if origin != "" {
		app := s.getApp()
		allowed := false
		if app.Config != nil {
			for _, o := range app.Config.CORSOrigins {
				if o == "*" || o == origin {
					allowed = true
					break
				}
			}
		}
		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Add("Vary", "Origin")
		}
	}
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	pathParams := matchPathParams(endpoint.Path, r.URL.Path)

	// Merge query string params
	for key, vals := range r.URL.Query() {
		if len(vals) > 0 {
			pathParams[key] = vals[0]
		}
	}

	// For mutation methods, extract form/JSON body data
	isMutation := r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete
	if isMutation {
		formData := extractFormData(r, s.getApp().Config)
		for k, v := range formData {
			pathParams[k] = v
		}
		// Add current_user fields
		session := s.getSession(r)
		if session != nil {
			pathParams["current_user_id"] = session.UserID
			pathParams["current_user_identity"] = session.Identity
			pathParams["current_user_role"] = session.Role
		}
	}

	pageNum := 1
	if pg, ok := pathParams["page"]; ok {
		fmt.Sscanf(pg, "%d", &pageNum)
		if pageNum < 1 {
			pageNum = 1
		}
	}

	var allRows []database.Row
	var paginateInfo *PaginateInfo

	statusCode := http.StatusOK

	// Use a transaction for mutations
	var tx *database.TxHandle
	if isMutation && s.db != nil {
		var err error
		tx, err = s.db.BeginTxHandle()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "Internal error",
			})
			return
		}
		defer tx.Rollback()
	}

	app := s.getApp()

	for _, node := range endpoint.Body {
		switch node.Type {
		case parser.NodeValidate:
			modelName := node.ModelName
			if modelName == "" {
				if len(node.Validations) > 0 {
					errors := validateInlineRules(node.Validations, pathParams)
					if len(errors) > 0 {
						writeJSON(w, http.StatusUnprocessableEntity, map[string]interface{}{
							"errors": errors,
						})
						return
					}
				}
				continue
			}
			errors := validateFormData(modelName, app, pathParams)
			if len(errors) > 0 {
				writeJSON(w, http.StatusUnprocessableEntity, map[string]interface{}{
					"errors": errors,
				})
				return
			}

		case parser.NodeRespond:
			if node.StatusCode > 0 {
				statusCode = node.StatusCode
			}
			continue

		case parser.NodeQuery:
			if s.db == nil {
				continue
			}

			sql := node.SQL

			// Handle pagination
			if node.Paginate > 0 {
				countSQL := fmt.Sprintf("SELECT COUNT(*) as _count FROM (%s)", sql)
				countRows, err := s.db.QueryRowsWithParams(countSQL, pathParams)
				total := 0
				if err == nil && len(countRows) > 0 {
					fmt.Sscanf(countRows[0]["_count"], "%d", &total)
				}

				offset := (pageNum - 1) * node.Paginate
				sql = fmt.Sprintf("%s LIMIT %d OFFSET %d", sql, node.Paginate, offset)

				paginateInfo = &PaginateInfo{
					Page:    pageNum,
					PerPage: node.Paginate,
					Total:   total,
					HasPrev: pageNum > 1,
					HasNext: offset+node.Paginate < total,
				}
			}

			trimmed := strings.TrimSpace(strings.ToUpper(sql))
			if tx != nil && !strings.HasPrefix(trimmed, "SELECT") {
				// Mutation query within transaction
				err := tx.ExecWithParams(sql, pathParams)
				if err != nil {
					s.logger.LogError("api mutation query failed", err)
					writeJSON(w, http.StatusInternalServerError, map[string]string{
						"error": "Internal server error",
					})
					return
				}
			} else if tx != nil {
				rows, err := tx.QueryRowsWithParams(sql, pathParams)
				if err != nil {
					s.logger.LogError("api select query failed", err)
					writeJSON(w, http.StatusInternalServerError, map[string]string{
						"error": "Internal server error",
					})
					return
				}
				allRows = rows
				// Merge SELECT results into params for subsequent queries
				name := node.Name
				if name == "" {
					name = "_last"
				}
				for _, row := range rows {
					for k, v := range row {
						pathParams[name+"."+k] = v
					}
				}
			} else {
				rows, err := s.db.QueryRowsWithParams(sql, pathParams)
				if err != nil {
					s.logger.LogError("api query failed", err)
					writeJSON(w, http.StatusInternalServerError, map[string]string{
						"error": "Internal server error",
					})
					return
				}
				allRows = rows
			}

		case parser.NodeRedirect:
			if tx != nil {
				tx.Commit()
			}
			path := node.Value
			for k, v := range pathParams {
				path = strings.ReplaceAll(path, ":"+k, v)
			}
			writeJSON(w, statusCode, map[string]string{
				"redirect": path,
			})
			return
		}
	}

	// Commit transaction
	if tx != nil {
		tx.Commit()
	}

	// Build response
	response := map[string]interface{}{
		"data": allRows,
	}

	if paginateInfo != nil {
		response["pagination"] = map[string]interface{}{
			"page":     paginateInfo.Page,
			"per_page": paginateInfo.PerPage,
			"total":    paginateInfo.Total,
			"has_prev": paginateInfo.HasPrev,
			"has_next": paginateInfo.HasNext,
		}
	}

	writeJSON(w, statusCode, response)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(data)
}
