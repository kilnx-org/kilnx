package optimizer

import (
	"testing"

	"github.com/kilnx-org/kilnx/internal/parser"
)

func TestRewriteSelectStar_TableWithColumns(t *testing.T) {
	app := &parser.App{
		Pages: []parser.Page{{
			Path: "/users",
			Body: []parser.Node{
				{
					Type: parser.NodeQuery,
					Name: "users",
					SQL:  "SELECT * FROM user",
				},
				{
					Type: parser.NodeTable,
					Name: "users",
					Columns: []parser.TableColumn{
						{Field: "name"},
						{Field: "email"},
					},
				},
			},
		}},
	}

	Optimize(app)

	got := app.Pages[0].Body[0].SQL
	want := "SELECT name, email FROM user"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRewriteSelectStar_ListWithProps(t *testing.T) {
	app := &parser.App{
		Pages: []parser.Page{{
			Path: "/users",
			Body: []parser.Node{
				{
					Type: parser.NodeQuery,
					Name: "users",
					SQL:  "SELECT * FROM user",
				},
				{
					Type: parser.NodeList,
					Name: "users",
					Props: map[string]string{
						"title":    "name",
						"subtitle": "email",
					},
				},
			},
		}},
	}

	Optimize(app)

	got := app.Pages[0].Body[0].SQL
	want := "SELECT name, email FROM user"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNoRewrite_TableWithoutColumns(t *testing.T) {
	app := &parser.App{
		Pages: []parser.Page{{
			Path: "/users",
			Body: []parser.Node{
				{
					Type: parser.NodeQuery,
					Name: "users",
					SQL:  "SELECT * FROM user",
				},
				{
					Type:    parser.NodeTable,
					Name:    "users",
					Columns: nil, // auto-detect mode
				},
			},
		}},
	}

	Optimize(app)

	got := app.Pages[0].Body[0].SQL
	want := "SELECT * FROM user"
	if got != want {
		t.Errorf("should not rewrite, got %q", got)
	}
}

func TestNoRewrite_ExplicitColumns(t *testing.T) {
	original := "SELECT name, email FROM user"
	app := &parser.App{
		Pages: []parser.Page{{
			Path: "/users",
			Body: []parser.Node{
				{
					Type: parser.NodeQuery,
					Name: "users",
					SQL:  original,
				},
				{
					Type: parser.NodeTable,
					Name: "users",
					Columns: []parser.TableColumn{
						{Field: "name"},
						{Field: "email"},
					},
				},
			},
		}},
	}

	Optimize(app)

	got := app.Pages[0].Body[0].SQL
	if got != original {
		t.Errorf("should not rewrite explicit columns, got %q", got)
	}
}

func TestRewrite_WithDistinct(t *testing.T) {
	app := &parser.App{
		Pages: []parser.Page{{
			Path: "/users",
			Body: []parser.Node{
				{
					Type: parser.NodeQuery,
					Name: "users",
					SQL:  "SELECT DISTINCT * FROM user",
				},
				{
					Type: parser.NodeTable,
					Name: "users",
					Columns: []parser.TableColumn{
						{Field: "name"},
					},
				},
			},
		}},
	}

	Optimize(app)

	got := app.Pages[0].Body[0].SQL
	want := "SELECT DISTINCT name FROM user"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRewrite_MultiLineSQL(t *testing.T) {
	app := &parser.App{
		Pages: []parser.Page{{
			Path: "/users",
			Body: []parser.Node{
				{
					Type: parser.NodeQuery,
					Name: "users",
					SQL:  "SELECT * FROM user WHERE active = 1 ORDER BY name",
				},
				{
					Type: parser.NodeList,
					Name: "users",
					Props: map[string]string{
						"title": "name",
					},
				},
			},
		}},
	}

	Optimize(app)

	got := app.Pages[0].Body[0].SQL
	want := "SELECT name FROM user WHERE active = 1 ORDER BY name"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRewrite_QueryUsedByMultipleComponents(t *testing.T) {
	app := &parser.App{
		Pages: []parser.Page{{
			Path: "/users",
			Body: []parser.Node{
				{
					Type: parser.NodeQuery,
					Name: "users",
					SQL:  "SELECT * FROM user",
				},
				{
					Type: parser.NodeList,
					Name: "users",
					Props: map[string]string{
						"title": "name",
					},
				},
				{
					Type: parser.NodeTable,
					Name: "users",
					Columns: []parser.TableColumn{
						{Field: "email"},
						{Field: "created"},
					},
				},
			},
		}},
	}

	Optimize(app)

	got := app.Pages[0].Body[0].SQL
	want := "SELECT name, email, created FROM user"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRewrite_InterpolatedFields(t *testing.T) {
	app := &parser.App{
		Pages: []parser.Page{{
			Path: "/users",
			Body: []parser.Node{
				{
					Type: parser.NodeQuery,
					Name: "stats",
					SQL:  "SELECT * FROM stats",
				},
				{
					Type:  parser.NodeText,
					Value: "Total users: {stats.total}",
				},
			},
		}},
	}

	Optimize(app)

	got := app.Pages[0].Body[0].SQL
	want := "SELECT total FROM stats"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRewrite_HTMLInterpolation(t *testing.T) {
	app := &parser.App{
		Pages: []parser.Page{{
			Path: "/profile",
			Body: []parser.Node{
				{
					Type: parser.NodeQuery,
					Name: "user",
					SQL:  "SELECT * FROM user WHERE id = 1",
				},
				{
					Type:        parser.NodeHTML,
					HTMLContent: "<h1>{user.name}</h1><p>{user.bio}</p>",
				},
			},
		}},
	}

	Optimize(app)

	got := app.Pages[0].Body[0].SQL
	want := "SELECT name, bio FROM user WHERE id = 1"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRewrite_RowActionsIncludeParams(t *testing.T) {
	app := &parser.App{
		Pages: []parser.Page{{
			Path: "/users",
			Body: []parser.Node{
				{
					Type: parser.NodeQuery,
					Name: "users",
					SQL:  "SELECT * FROM user",
				},
				{
					Type: parser.NodeTable,
					Name: "users",
					Columns: []parser.TableColumn{
						{Field: "name"},
					},
					RowActions: []parser.RowAction{
						{Label: "edit", Path: "/users/:id/edit"},
					},
				},
			},
		}},
	}

	Optimize(app)

	got := app.Pages[0].Body[0].SQL
	want := "SELECT name, id FROM user"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRewrite_Fragment(t *testing.T) {
	app := &parser.App{
		Fragments: []parser.Page{{
			Path: "/user-list",
			Body: []parser.Node{
				{
					Type: parser.NodeQuery,
					Name: "users",
					SQL:  "SELECT * FROM user",
				},
				{
					Type: parser.NodeList,
					Name: "users",
					Props: map[string]string{
						"title": "name",
					},
				},
			},
		}},
	}

	Optimize(app)

	got := app.Fragments[0].Body[0].SQL
	want := "SELECT name FROM user"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNoRewrite_NoConsumers(t *testing.T) {
	app := &parser.App{
		Pages: []parser.Page{{
			Path: "/users",
			Body: []parser.Node{
				{
					Type: parser.NodeQuery,
					Name: "users",
					SQL:  "SELECT * FROM user",
				},
			},
		}},
	}

	Optimize(app)

	got := app.Pages[0].Body[0].SQL
	want := "SELECT * FROM user"
	if got != want {
		t.Errorf("should not rewrite without consumers, got %q", got)
	}
}

func TestNoRewrite_CountInterpolation(t *testing.T) {
	app := &parser.App{
		Pages: []parser.Page{{
			Path: "/dashboard",
			Body: []parser.Node{
				{
					Type: parser.NodeQuery,
					Name: "users",
					SQL:  "SELECT * FROM user",
				},
				{
					Type:  parser.NodeText,
					Value: "Total: {users.count}",
				},
			},
		}},
	}

	Optimize(app)

	// .count is a built-in, not a real column, so no fields are collected
	got := app.Pages[0].Body[0].SQL
	want := "SELECT * FROM user"
	if got != want {
		t.Errorf("should not rewrite with only .count, got %q", got)
	}
}

func TestRewrite_CaseInsensitive(t *testing.T) {
	app := &parser.App{
		Pages: []parser.Page{{
			Path: "/users",
			Body: []parser.Node{
				{
					Type: parser.NodeQuery,
					Name: "users",
					SQL:  "select * from user",
				},
				{
					Type: parser.NodeList,
					Name: "users",
					Props: map[string]string{
						"title": "name",
					},
				},
			},
		}},
	}

	Optimize(app)

	got := app.Pages[0].Body[0].SQL
	want := "select name from user"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRewrite_DedupFields(t *testing.T) {
	app := &parser.App{
		Pages: []parser.Page{{
			Path: "/users",
			Body: []parser.Node{
				{
					Type: parser.NodeQuery,
					Name: "users",
					SQL:  "SELECT * FROM user",
				},
				{
					Type: parser.NodeList,
					Name: "users",
					Props: map[string]string{
						"title":    "name",
						"subtitle": "name",
					},
				},
			},
		}},
	}

	Optimize(app)

	got := app.Pages[0].Body[0].SQL
	want := "SELECT name FROM user"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNoRewrite_MultipleUnnamedQueries(t *testing.T) {
	app := &parser.App{
		Pages: []parser.Page{{
			Path: "/dashboard",
			Body: []parser.Node{
				{
					Type: parser.NodeQuery,
					SQL:  "SELECT * FROM user",
				},
				{
					Type: parser.NodeQuery,
					SQL:  "SELECT * FROM post",
				},
				{
					Type: parser.NodeTable,
					Columns: []parser.TableColumn{
						{Field: "name"},
					},
				},
			},
		}},
	}

	Optimize(app)

	got1 := app.Pages[0].Body[0].SQL
	got2 := app.Pages[0].Body[1].SQL
	if got1 != "SELECT * FROM user" {
		t.Errorf("should not rewrite first unnamed query, got %q", got1)
	}
	if got2 != "SELECT * FROM post" {
		t.Errorf("should not rewrite second unnamed query, got %q", got2)
	}
}

func TestNoRewrite_ActionLabelNotColumn(t *testing.T) {
	app := &parser.App{
		Pages: []parser.Page{{
			Path: "/users",
			Body: []parser.Node{
				{
					Type: parser.NodeQuery,
					Name: "users",
					SQL:  "SELECT * FROM user",
				},
				{
					Type: parser.NodeList,
					Name: "users",
					Props: map[string]string{
						"title":        "name",
						"action_label": "Edit",
						"action_path":  "/users/:id/edit",
					},
				},
			},
		}},
	}

	Optimize(app)

	got := app.Pages[0].Body[0].SQL
	want := "SELECT name, id FROM user"
	if got != want {
		t.Errorf("got %q, want %q (action_label should NOT be a column)", got, want)
	}
}

func TestRewrite_NodeOnChildren(t *testing.T) {
	app := &parser.App{
		Pages: []parser.Page{{
			Path: "/users",
			Body: []parser.Node{
				{
					Type: parser.NodeQuery,
					Name: "user",
					SQL:  "SELECT * FROM user WHERE id = 1",
				},
				{
					Type: parser.NodeOn,
					Children: []parser.Node{
						{
							Type:  parser.NodeText,
							Value: "Welcome {user.name}",
						},
						{
							Type:        parser.NodeHTML,
							HTMLContent: "<span>{user.email}</span>",
						},
					},
				},
			},
		}},
	}

	Optimize(app)

	got := app.Pages[0].Body[0].SQL
	want := "SELECT name, email FROM user WHERE id = 1"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRewrite_SearchFieldsIncluded(t *testing.T) {
	app := &parser.App{
		Pages: []parser.Page{{
			Path: "/users",
			Body: []parser.Node{
				{
					Type: parser.NodeQuery,
					Name: "users",
					SQL:  "SELECT * FROM user",
				},
				{
					Type: parser.NodeTable,
					Name: "users",
					Columns: []parser.TableColumn{
						{Field: "name"},
					},
				},
				{
					Type:         parser.NodeSearch,
					Name:         "users",
					SearchFields: []string{"email", "name"},
				},
			},
		}},
	}

	Optimize(app)

	got := app.Pages[0].Body[0].SQL
	want := "SELECT name, email FROM user"
	if got != want {
		t.Errorf("got %q, want %q (search fields must be included)", got, want)
	}
}
