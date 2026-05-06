package parser

import (
	"strings"
	"testing"
)

func findFirstNode(body []Node, typ NodeType) (Node, bool) {
	for _, n := range body {
		if n.Type == typ {
			return n, true
		}
	}
	return Node{}, false
}

func TestParseRespondNode_FragmentWithQuery(t *testing.T) {
	src := `action /users/update method POST
  respond fragment ".user-row" query: SELECT * FROM user WHERE id = :id`
	app := parse(t, src)
	if len(app.Actions) != 1 {
		t.Fatalf("expected 1 action")
	}
	n, ok := findFirstNode(app.Actions[0].Body, NodeRespond)
	if !ok {
		t.Fatalf("expected NodeRespond")
	}
	if n.RespondTarget != ".user-row" {
		t.Errorf("RespondTarget = %q, want .user-row", n.RespondTarget)
	}
	if !strings.Contains(n.QuerySQL, "SELECT * FROM user") {
		t.Errorf("QuerySQL = %q", n.QuerySQL)
	}
}

func TestParseRespondNode_Delete(t *testing.T) {
	src := `action /users/delete method POST
  respond fragment delete`
	app := parse(t, src)
	n, ok := findFirstNode(app.Actions[0].Body, NodeRespond)
	if !ok {
		t.Fatal("expected NodeRespond")
	}
	if n.RespondSwap != "delete" {
		t.Errorf("RespondSwap = %q, want delete", n.RespondSwap)
	}
}

func TestParseRespondNode_Status(t *testing.T) {
	src := `action /api/ping method POST
  respond status 204`
	app := parse(t, src)
	n, ok := findFirstNode(app.Actions[0].Body, NodeRespond)
	if !ok {
		t.Fatal("expected NodeRespond")
	}
	if n.StatusCode != 204 {
		t.Errorf("StatusCode = %d, want 204", n.StatusCode)
	}
}

func TestParseBroadcastNode(t *testing.T) {
	src := `action /chat/send method POST
  broadcast to :room fragment chat-message`
	app := parse(t, src)
	n, ok := findFirstNode(app.Actions[0].Body, NodeBroadcast)
	if !ok {
		t.Fatal("expected NodeBroadcast")
	}
	if n.BroadcastRoom != ":room" {
		t.Errorf("BroadcastRoom = %q, want :room", n.BroadcastRoom)
	}
	if n.BroadcastFrag != "chat-message" {
		t.Errorf("BroadcastFrag = %q, want chat-message", n.BroadcastFrag)
	}
}

func TestParseBroadcastNode_Identifier(t *testing.T) {
	src := `action /chat/send method POST
  broadcast to global`
	app := parse(t, src)
	n, ok := findFirstNode(app.Actions[0].Body, NodeBroadcast)
	if !ok {
		t.Fatal("expected NodeBroadcast")
	}
	if n.BroadcastRoom != "global" {
		t.Errorf("BroadcastRoom = %q, want global", n.BroadcastRoom)
	}
}

func TestParseFetchNode(t *testing.T) {
	src := `action /sync method POST
  fetch weather: GET https://api.weather.com/v1?city=:city
    header Authorization: env API_KEY
    body amount: :amount`
	app := parse(t, src)
	n, ok := findFirstNode(app.Actions[0].Body, NodeFetch)
	if !ok {
		t.Fatal("expected NodeFetch")
	}
	if n.Name != "weather" {
		t.Errorf("Name = %q, want weather", n.Name)
	}
	if n.FetchMethod != "GET" {
		t.Errorf("FetchMethod = %q, want GET", n.FetchMethod)
	}
	if !strings.Contains(n.FetchURL, "api.weather.com") {
		t.Errorf("FetchURL = %q", n.FetchURL)
	}
	if n.FetchHeaders["Authorization"] != "env:API_KEY" {
		t.Errorf("Authorization header = %q, want env:API_KEY", n.FetchHeaders["Authorization"])
	}
	if n.FetchBody["amount"] != ":amount" {
		t.Errorf("body amount = %q, want :amount", n.FetchBody["amount"])
	}
}

func TestParseFetchNode_NoName(t *testing.T) {
	src := `action /post method POST
  fetch POST https://api.example.com/v1/notify`
	app := parse(t, src)
	n, ok := findFirstNode(app.Actions[0].Body, NodeFetch)
	if !ok {
		t.Fatal("expected NodeFetch")
	}
	if n.FetchMethod != "POST" {
		t.Errorf("FetchMethod = %q, want POST", n.FetchMethod)
	}
}

func TestParseLLMNode(t *testing.T) {
	src := `action /chat/answer method POST
  llm reply: claude-sonnet-4-6
    history: SELECT role, content FROM message WHERE chat_id = :id ORDER BY created ASC
    system: You are a helpful assistant.`
	app := parse(t, src)
	n, ok := findFirstNode(app.Actions[0].Body, NodeLLM)
	if !ok {
		t.Fatal("expected NodeLLM")
	}
	if n.Name != "reply" {
		t.Errorf("Name = %q, want reply", n.Name)
	}
	if n.LLMModel != "claude-sonnet-4-6" {
		t.Errorf("LLMModel = %q, want claude-sonnet-4-6", n.LLMModel)
	}
	if !strings.Contains(n.LLMHistorySQL, "SELECT role, content") {
		t.Errorf("LLMHistorySQL = %q", n.LLMHistorySQL)
	}
	if !strings.Contains(n.LLMSystem, "helpful assistant") {
		t.Errorf("LLMSystem = %q", n.LLMSystem)
	}
}

func TestParseGeneratePDFNode(t *testing.T) {
	src := `action /report method POST
  generate pdf from template invoice data items`
	app := parse(t, src)
	n, ok := findFirstNode(app.Actions[0].Body, NodeGeneratePDF)
	if !ok {
		t.Fatal("expected NodeGeneratePDF")
	}
	if n.TemplateName != "invoice" {
		t.Errorf("TemplateName = %q, want invoice", n.TemplateName)
	}
	if n.DataQueryName != "items" {
		t.Errorf("DataQueryName = %q, want items", n.DataQueryName)
	}
}

func TestParsePermissions(t *testing.T) {
	src := `permissions
  admin: all
  editor: read post, write post where author = current_user
  viewer: read post where status = published`
	app := parse(t, src)
	if len(app.Permissions) != 3 {
		t.Fatalf("expected 3 permissions, got %d", len(app.Permissions))
	}
	if app.Permissions[0].Role != "admin" {
		t.Errorf("first role = %q, want admin", app.Permissions[0].Role)
	}
	if len(app.Permissions[1].Rules) != 2 {
		t.Errorf("editor expected 2 rules, got %d: %v", len(app.Permissions[1].Rules), app.Permissions[1].Rules)
	}
}

func TestParseLogConfig(t *testing.T) {
	src := `log
  level: debug
  slow-query: 250ms
  requests: all
  errors: all stacktrace`
	app := parse(t, src)
	if app.LogConfig == nil {
		t.Fatal("LogConfig nil")
	}
	if app.LogConfig.Level != "debug" {
		t.Errorf("Level = %q, want debug", app.LogConfig.Level)
	}
	if app.LogConfig.SlowQueryMs != 250 {
		t.Errorf("SlowQueryMs = %d, want 250", app.LogConfig.SlowQueryMs)
	}
	if !app.LogConfig.LogRequests {
		t.Error("LogRequests should be true")
	}
	if !app.LogConfig.Stacktrace {
		t.Error("Stacktrace should be true")
	}
}

func TestParseLogConfig_DefaultsWhenEmpty(t *testing.T) {
	src := `log`
	app := parse(t, src)
	if app.LogConfig == nil {
		t.Fatal("LogConfig nil")
	}
	if app.LogConfig.Level != "info" {
		t.Errorf("default Level = %q, want info", app.LogConfig.Level)
	}
}
