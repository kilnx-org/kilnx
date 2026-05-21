package runtime

import (
	"path/filepath"
	"testing"
)

func TestPathInsideAnyRoot(t *testing.T) {
	abs := func(p string) string { a, _ := filepath.Abs(p); return a }
	roots := []string{abs("/srv/reports"), abs("/srv/uploads")}
	cases := []struct {
		name string
		path string
		want bool
	}{
		{"inside reports", "/srv/reports/x.pdf", true},
		{"inside uploads nested", "/srv/uploads/2025/y.pdf", true},
		{"outside both", "/etc/passwd", false},
		{"traversal escapes", "/srv/reports/../../etc/passwd", false},
		{"sibling lookalike", "/srv/reports-evil/x", false},
		{"equal to root", "/srv/reports", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := pathInsideAnyRoot(filepath.Clean(tc.path), roots)
			if got != tc.want {
				t.Errorf("got %v want %v", got, tc.want)
			}
		})
	}
}
