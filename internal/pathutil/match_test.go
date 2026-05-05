package pathutil

import "testing"

func TestMatch(t *testing.T) {
	cases := []struct {
		template string
		path     string
		want     bool
	}{
		{"/tasks/:id/delete", "/tasks/123/delete", true},
		{"/tasks/:id", "/tasks/abc", true},
		{"/tasks/:id", "/tasks/abc/extra", false},
		{"/tasks", "/tasks", true},
		{"/tasks", "/users", false},
		{"/", "/", true},
		{"/a/:b/:c", "/a/x/y", true},
		{"/a/:b/c", "/a/x/y", false},
	}
	for _, tc := range cases {
		got := Match(tc.template, tc.path)
		if got != tc.want {
			t.Errorf("Match(%q,%q) = %v, want %v", tc.template, tc.path, got, tc.want)
		}
	}
}
