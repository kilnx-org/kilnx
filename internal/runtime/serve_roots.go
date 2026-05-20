package runtime

import (
	"path/filepath"
	"strings"
)

// pathInsideAnyRoot reports whether abs is equal to or inside one of roots.
// abs and each root must already be absolute and Clean-ed.
// filepath.Rel resolves ".." syntactically, so a traversal attempt like
// "/srv/reports/../etc/passwd" becomes "../etc/passwd" and is rejected.
func pathInsideAnyRoot(abs string, roots []string) bool {
	for _, root := range roots {
		rel, err := filepath.Rel(root, abs)
		if err != nil {
			continue
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		return true
	}
	return false
}
