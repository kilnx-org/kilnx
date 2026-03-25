package runtime

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/parser"
)

// handleAPI processes an API endpoint and returns JSON
func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request, endpoint parser.Page) {
	pathParams := matchPathParams(endpoint.Path, r.URL.Path)

	// Merge query string params
	for key, vals := range r.URL.Query() {
		if len(vals) > 0 {
			pathParams[key] = vals[0]
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

	for _, node := range endpoint.Body {
		switch node.Type {
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

				totalPages := (total + node.Paginate - 1) / node.Paginate
				paginateInfo = &PaginateInfo{
					Page:    pageNum,
					PerPage: node.Paginate,
					Total:   total,
					HasPrev: pageNum > 1,
					HasNext: offset+node.Paginate < total,
				}
				_ = totalPages
			}

			rows, err := s.db.QueryRowsWithParams(sql, pathParams)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{
					"error": err.Error(),
				})
				return
			}
			allRows = rows
		}
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
