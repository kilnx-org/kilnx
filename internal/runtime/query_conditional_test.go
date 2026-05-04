package runtime

import "testing"

func TestExpandQueryConditionalsTruthy(t *testing.T) {
	sql := "SELECT * FROM orders WHERE 1=1 {{if params.status}}AND status = :status{{end}}"
	params := map[string]string{"status": "shipped"}
	got := expandQueryConditionals(sql, params)
	want := "SELECT * FROM orders WHERE 1=1 AND status = :status"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExpandQueryConditionalsMissing(t *testing.T) {
	sql := "SELECT * FROM orders WHERE 1=1 {{if params.status}}AND status = :status{{end}}"
	params := map[string]string{}
	got := expandQueryConditionals(sql, params)
	want := "SELECT * FROM orders WHERE 1=1 "
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExpandQueryConditionalsEmpty(t *testing.T) {
	sql := "SELECT * FROM orders WHERE 1=1 {{if params.status}}AND status = :status{{end}}"
	params := map[string]string{"status": ""}
	got := expandQueryConditionals(sql, params)
	want := "SELECT * FROM orders WHERE 1=1 "
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExpandQueryConditionalsNested(t *testing.T) {
	sql := "SELECT * FROM orders WHERE 1=1 {{if params.status}}AND status = :status{{if params.priority}} AND priority = :priority{{end}}{{end}}"
	params := map[string]string{"status": "shipped", "priority": "high"}
	got := expandQueryConditionals(sql, params)
	want := "SELECT * FROM orders WHERE 1=1 AND status = :status AND priority = :priority"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExpandQueryConditionalsEquality(t *testing.T) {
	sql := `SELECT * FROM orders WHERE 1=1 {{if params.role == "admin"}}AND is_admin = 1{{end}}`
	params := map[string]string{"role": "admin"}
	got := expandQueryConditionals(sql, params)
	want := "SELECT * FROM orders WHERE 1=1 AND is_admin = 1"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExpandQueryConditionalsInequality(t *testing.T) {
	sql := `SELECT * FROM orders WHERE 1=1 {{if params.role != "admin"}}AND is_admin = 0{{end}}`
	params := map[string]string{"role": "user"}
	got := expandQueryConditionals(sql, params)
	want := "SELECT * FROM orders WHERE 1=1 AND is_admin = 0"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExpandQueryConditionalsNoConditionals(t *testing.T) {
	sql := "SELECT * FROM orders"
	params := map[string]string{"status": "shipped"}
	got := expandQueryConditionals(sql, params)
	if got != sql {
		t.Errorf("got %q, want %q", got, sql)
	}
}
