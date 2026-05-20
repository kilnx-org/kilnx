package runtime

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// TestE2E_Agent_In_Job runs an llm-agent inside a job: the agent result is
// bound into job params for a following query, and the `on progress` body
// writes progress rows.
func TestE2E_Agent_In_Job(t *testing.T) {
	stub := fakeClaudeBin(t, "testdata/agent_streams/job_progress.jsonl")
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("KILNX_CLAUDE_BIN", stub)
	wsRoot := t.TempDir()

	src := `config
  workspace-root: ` + wsRoot + `

model review_job
  status: text default "queued"
  progress: text
  result: text

job run-review
  query: INSERT INTO review_job (status) VALUES ('queued')
  llm review
    model: claude-sonnet-4-6
    system: Supervisor
    agent
      cwd: ` + wsRoot + `
      max-budget-usd: 1.0
      on progress
        query: UPDATE review_job SET progress = :progress
  query: UPDATE review_job SET status = 'done', result = :review.text

page /
  html
    <h1>Home</h1>

api /debug/job
  query row: SELECT status, progress, result FROM review_job LIMIT 1

action /start method POST
  enqueue run-review
  redirect /
`
	baseURL, _, srv, cleanup := startTestServerWithHandles(t, src)
	defer cleanup()
	srv.StartJobQueue()

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Get(baseURL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	resp.Body.Close()

	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req, _ := http.NewRequest("POST", baseURL+"/start", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("POST /start: %v", err)
	}
	if resp.StatusCode >= 400 {
		t.Fatalf("POST /start status: %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Job queue polls once per second; allow a few ticks.
	deadline := time.Now().Add(8 * time.Second)
	var status, progress, result string
	for time.Now().Before(deadline) {
		status, progress, result = queryReviewJob(t, baseURL, client)
		if status == "done" {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	if status != "done" {
		t.Fatalf("job did not finish: status=%q", status)
	}
	if result != "REPORT_DONE" {
		t.Errorf("result not bound from agent: got %q, want REPORT_DONE", result)
	}
	if progress == "" {
		t.Errorf("on progress handler never wrote a row")
	}
}

// queryReviewJob reads the single review_job row over the /debug/job api
// route declared in the test source.
func queryReviewJob(t *testing.T, baseURL string, client *http.Client) (status, progress, result string) {
	t.Helper()
	resp, err := client.Get(baseURL + "/debug/job")
	if err != nil {
		t.Fatalf("GET /debug/job: %v", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	body := string(raw)
	if strings.Contains(body, `"status": "done"`) {
		status = "done"
	}
	if strings.Contains(body, `"result": "REPORT_DONE"`) {
		result = "REPORT_DONE"
	}
	if strings.Contains(body, `"progress": "`) && !strings.Contains(body, `"progress": ""`) {
		progress = "set"
	}
	return status, progress, result
}
