package optimizer

import (
	"strings"
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

// --- Deduplication tests ---

func TestDeduplicate_IdenticalQueries(t *testing.T) {
	app := &parser.App{
		Pages: []parser.Page{{
			Path: "/test",
			Body: []parser.Node{
				{Type: parser.NodeQuery, Name: "q1", SQL: "SELECT * FROM user"},
				{Type: parser.NodeQuery, Name: "q2", SQL: "SELECT * FROM user"},
				{Type: parser.NodeTable, Name: "q2", Columns: []parser.TableColumn{{Field: "name"}}},
			},
		}},
	}
	deduplicateQueries(&app.Pages[0])
	if app.Pages[0].Body[1].SQL != "" {
		t.Errorf("duplicate query should have SQL cleared, got %q", app.Pages[0].Body[1].SQL)
	}
	if app.Pages[0].Body[2].Name != "q1" {
		t.Errorf("consumer should be renamed to q1, got %q", app.Pages[0].Body[2].Name)
	}
}

func TestDeduplicate_DifferentQueries(t *testing.T) {
	app := &parser.App{
		Pages: []parser.Page{{
			Path: "/test",
			Body: []parser.Node{
				{Type: parser.NodeQuery, Name: "q1", SQL: "SELECT * FROM user"},
				{Type: parser.NodeQuery, Name: "q2", SQL: "SELECT * FROM post"},
			},
		}},
	}
	deduplicateQueries(&app.Pages[0])
	if app.Pages[0].Body[0].SQL == "" || app.Pages[0].Body[1].SQL == "" {
		t.Error("different queries should not be deduplicated")
	}
}

func TestDeduplicate_ConsumerInOnBlock(t *testing.T) {
	app := &parser.App{
		Pages: []parser.Page{{
			Path: "/test",
			Body: []parser.Node{
				{Type: parser.NodeQuery, Name: "q1", SQL: "SELECT * FROM user"},
				{Type: parser.NodeQuery, Name: "q2", SQL: "SELECT * FROM user"},
				{Type: parser.NodeOn, Children: []parser.Node{
					{Type: parser.NodeTable, Name: "q2", Columns: []parser.TableColumn{{Field: "name"}}},
				}},
			},
		}},
	}
	deduplicateQueries(&app.Pages[0])
	if app.Pages[0].Body[2].Children[0].Name != "q1" {
		t.Errorf("consumer inside NodeOn should be renamed, got %q", app.Pages[0].Body[2].Children[0].Name)
	}
}

// --- JOIN pruning tests ---

func TestPruneJoin_UnusedJoinRemoved(t *testing.T) {
	sql := "SELECT p.title FROM post p JOIN user u ON p.author_id = u.id"
	fields := newFieldSet()
	fields.add("p.title")
	got := pruneUnusedJoins(sql, fields)
	if strings.Contains(got, "JOIN") {
		t.Errorf("unused JOIN should be pruned, got %q", got)
	}
}

func TestPruneJoin_UsedJoinKept(t *testing.T) {
	sql := "SELECT p.title, u.name FROM post p JOIN user u ON p.author_id = u.id"
	fields := newFieldSet()
	fields.add("p.title")
	fields.add("u.name")
	got := pruneUnusedJoins(sql, fields)
	if !strings.Contains(got, "JOIN") {
		t.Errorf("used JOIN should be kept, got %q", got)
	}
}

func TestPruneJoin_UnqualifiedFieldSkips(t *testing.T) {
	sql := "SELECT p.title FROM post p JOIN user u ON p.author_id = u.id"
	fields := newFieldSet()
	fields.add("title") // unqualified, can't determine table
	got := pruneUnusedJoins(sql, fields)
	if got != sql {
		t.Errorf("should not prune with unqualified fields, got %q", got)
	}
}

func TestPruneJoin_WithoutAlias(t *testing.T) {
	sql := "SELECT post.title FROM post JOIN user ON post.author_id = user.id"
	fields := newFieldSet()
	fields.add("post.title")
	got := pruneUnusedJoins(sql, fields)
	if strings.Contains(strings.ToLower(got), "join user") {
		t.Errorf("JOIN without alias should still be prunable, got %q", got)
	}
}

// --- Stream materialization tests ---

func TestMarkStream_AggregateMarked(t *testing.T) {
	app := &parser.App{
		Streams: []parser.Stream{
			{Path: "/stream/stats", SQL: "SELECT count(*) FROM user", IntervalSecs: 5},
		},
	}
	markStreamCandidates(app)
	if !strings.HasPrefix(app.Streams[0].SQL, "/* kilnx:materialize-candidate */") {
		t.Errorf("aggregate stream should be marked, got %q", app.Streams[0].SQL)
	}
}

func TestMarkStream_NonAggregateNotMarked(t *testing.T) {
	app := &parser.App{
		Streams: []parser.Stream{
			{Path: "/stream/users", SQL: "SELECT * FROM user", IntervalSecs: 5},
		},
	}
	markStreamCandidates(app)
	if strings.Contains(app.Streams[0].SQL, "materialize") {
		t.Errorf("non-aggregate stream should not be marked, got %q", app.Streams[0].SQL)
	}
}

func TestMarkStream_NoInterval(t *testing.T) {
	app := &parser.App{
		Streams: []parser.Stream{
			{Path: "/stream/stats", SQL: "SELECT count(*) FROM user", IntervalSecs: 0},
		},
	}
	markStreamCandidates(app)
	if strings.Contains(app.Streams[0].SQL, "materialize") {
		t.Errorf("stream without interval should not be marked, got %q", app.Streams[0].SQL)
	}
}

func TestMarkStream_AlreadyMarked(t *testing.T) {
	app := &parser.App{
		Streams: []parser.Stream{
			{Path: "/stream/stats", SQL: "/* kilnx:materialize-candidate */ SELECT count(*) FROM user", IntervalSecs: 5},
		},
	}
	markStreamCandidates(app)
	count := strings.Count(app.Streams[0].SQL, "materialize-candidate")
	if count != 1 {
		t.Errorf("should not double-mark, got %d occurrences", count)
	}
}

func TestMarkStream_SumAvgMinMax(t *testing.T) {
	for _, fn := range []string{"SUM", "AVG", "MIN", "MAX"} {
		app := &parser.App{
			Streams: []parser.Stream{
				{Path: "/s", SQL: "SELECT " + fn + "(value) FROM deal", IntervalSecs: 10},
			},
		}
		markStreamCandidates(app)
		if !strings.Contains(app.Streams[0].SQL, "materialize") {
			t.Errorf("%s stream should be marked", fn)
		}
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
