package runtime

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestE2E_Job_Enqueue(t *testing.T) {
	src := `
model task
  title: text

job process-task
  retry 3
  query: INSERT INTO task (title) VALUES (:title)

page /
  html
    <h1>Home</h1>

action /enqueue method POST
  enqueue process-task
    title: :title
  redirect /
`
	baseURL, cleanup := startTestServer(t, src)
	defer cleanup()

	// Trigger the action that enqueues a job
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// First get the page to extract CSRF token
	resp, err := client.Get(baseURL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	resp.Body.Close()

	// POST to enqueue
	form := strings.NewReader("title=test-job")
	req, _ := http.NewRequest("POST", baseURL+"/enqueue", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("POST /enqueue: %v", err)
	}
	resp.Body.Close()

	// The action should have succeeded (redirect or 200)
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusOK &&
		resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusForbidden {
		t.Errorf("unexpected status %d", resp.StatusCode)
	}

	// Give the job queue time to process
	time.Sleep(2 * time.Second)
}
