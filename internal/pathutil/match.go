// Package pathutil provides helpers for matching declarative route templates
// (e.g. "/tasks/:id/delete") against concrete request paths (e.g. "/tasks/123/delete").
package pathutil

import "strings"

// Match reports whether template (a route pattern with optional :param segments)
// matches path (a concrete URL path). Both must have the same number of segments.
// Segments starting with ':' in template act as wildcards.
func Match(template, path string) bool {
	tParts := strings.Split(template, "/")
	pParts := strings.Split(path, "/")
	if len(tParts) != len(pParts) {
		return false
	}
	for i, tp := range tParts {
		if strings.HasPrefix(tp, ":") {
			continue
		}
		if tp != pParts[i] {
			return false
		}
	}
	return true
}
