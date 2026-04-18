package runtime

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/parser"
	"github.com/kilnx-org/kilnx/internal/pdf"
)

// StartScheduler launches goroutines for each schedule defined in the app.
// It stops any previously running schedule goroutines before starting new ones.
func (s *Server) StartScheduler() {
	s.StopScheduler()

	app := s.getApp()
	if len(app.Schedules) == 0 {
		return
	}

	stop := make(chan struct{})
	s.mu.Lock()
	s.scheduleStop = stop
	s.mu.Unlock()

	for _, sched := range app.Schedules {
		go s.runSchedule(sched, stop)
	}
	fmt.Printf("Started %d schedule(s)\n", len(app.Schedules))
}

// StopScheduler signals all running schedule goroutines to exit.
func (s *Server) StopScheduler() {
	s.mu.Lock()
	if s.scheduleStop != nil {
		close(s.scheduleStop)
		s.scheduleStop = nil
	}
	s.mu.Unlock()
}

func (s *Server) runSchedule(sched parser.Schedule, stop <-chan struct{}) {
	// If it's a cron-style schedule (e.g., "every monday at 9:00")
	if sched.Cron != "" {
		s.runCronSchedule(sched, stop)
		return
	}

	interval := time.Duration(sched.IntervalSecs) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	fmt.Printf("  schedule '%s' running every %s\n", sched.Name, interval)

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if err := s.executeNodes(sched.Body, nil); err != nil {
				fmt.Printf("  schedule '%s' error: %v\n", sched.Name, err)
			}
		}
	}
}

// runCronSchedule handles "every monday at 9:00" style expressions
func (s *Server) runCronSchedule(sched parser.Schedule, stop <-chan struct{}) {
	fmt.Printf("  schedule '%s' cron: %s\n", sched.Name, sched.Cron)

	for {
		next := nextCronOccurrence(sched.Cron)
		if next.IsZero() {
			fmt.Printf("  schedule '%s': could not parse cron expression\n", sched.Name)
			return
		}
		delay := time.Until(next)
		fmt.Printf("  schedule '%s' next run at %s (in %s)\n", sched.Name, next.Format("2006-01-02 15:04"), delay.Round(time.Second))

		timer := time.NewTimer(delay)
		select {
		case <-stop:
			timer.Stop()
			return
		case <-timer.C:
			if err := s.executeNodes(sched.Body, nil); err != nil {
				fmt.Printf("  schedule '%s' error: %v\n", sched.Name, err)
			}
		}
	}
}

// nextCronOccurrence parses "every monday at 9:00" and returns the next occurrence
func nextCronOccurrence(expr string) time.Time {
	expr = strings.ToLower(strings.TrimSpace(expr))

	dayMap := map[string]time.Weekday{
		"sunday": time.Sunday, "monday": time.Monday, "tuesday": time.Tuesday,
		"wednesday": time.Wednesday, "thursday": time.Thursday, "friday": time.Friday,
		"saturday": time.Saturday,
	}

	var targetDay time.Weekday = -1
	everyDay := false
	var hour, minute int

	parts := strings.Fields(expr)
	for i, p := range parts {
		if d, ok := dayMap[p]; ok {
			targetDay = d
		}
		if p == "day" || p == "daily" {
			everyDay = true
		}
		if p == "at" && i+1 < len(parts) {
			timeParts := strings.SplitN(parts[i+1], ":", 2)
			if len(timeParts) == 2 {
				fmt.Sscanf(timeParts[0], "%d", &hour)
				fmt.Sscanf(timeParts[1], "%d", &minute)
			} else {
				fmt.Sscanf(timeParts[0], "%d", &hour)
			}
		}
	}

	if targetDay < 0 && !everyDay {
		return time.Time{}
	}

	now := time.Now()

	if everyDay {
		// "every day at HH:MM" - run daily at the specified time
		target := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
		if now.After(target) {
			target = target.AddDate(0, 0, 1)
		}
		return target
	}

	// Find the next occurrence of the target weekday
	daysUntil := int(targetDay) - int(now.Weekday())
	if daysUntil < 0 {
		daysUntil += 7
	}
	if daysUntil == 0 {
		target := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
		if now.After(target) {
			daysUntil = 7
		}
	}

	next := time.Date(now.Year(), now.Month(), now.Day()+daysUntil, hour, minute, 0, 0, now.Location())
	return next
}

// JobQueue manages async background jobs with SQLite persistence and retry support
type JobQueue struct {
	server *Server
	jobs   map[string]parser.Job
}

func NewJobQueue(server *Server) *JobQueue {
	jq := &JobQueue{
		server: server,
		jobs:   make(map[string]parser.Job),
	}

	app := server.getApp()
	for _, job := range app.Jobs {
		jq.jobs[job.Name] = job
	}

	return jq
}

func (jq *JobQueue) Start() {
	jq.recoverOrphanedJobs()
	go jq.pollQueue()
	if len(jq.jobs) > 0 {
		fmt.Printf("Job queue ready (%d job type(s))\n", len(jq.jobs))
	}
}

// recoverOrphanedJobs resets jobs stuck in 'executing' state back to 'available'.
// This handles the case where the server was restarted while jobs were running.
func (jq *JobQueue) recoverOrphanedJobs() {
	db := jq.server.db
	if db == nil {
		return
	}

	rows, err := db.QueryRows(
		`SELECT id, name FROM _kilnx_jobs WHERE state = 'executing'`)
	if err != nil || len(rows) == 0 {
		return
	}

	for _, row := range rows {
		db.ExecWithParams(
			`UPDATE _kilnx_jobs SET state = 'available', scheduled_at = datetime('now') WHERE id = :id`,
			map[string]string{"id": row["id"]})
	}
	fmt.Printf("Recovered %d orphaned job(s)\n", len(rows))
}

// RefreshJobQueue updates the job definitions from the current app (for hot-reload)
func (s *Server) RefreshJobQueue() {
	app := s.getApp()
	for _, job := range app.Jobs {
		s.jobQueue.jobs[job.Name] = job
	}
}

// Enqueue persists a job to the _kilnx_jobs table
func (jq *JobQueue) Enqueue(name string, params map[string]string) error {
	job, ok := jq.jobs[name]
	if !ok {
		return fmt.Errorf("unknown job: %s", name)
	}

	db := jq.server.db
	if db == nil {
		return fmt.Errorf("no database for job persistence")
	}

	paramsJSON, _ := json.Marshal(params)
	maxAttempts := job.MaxRetries
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	err := db.ExecWithParams(
		`INSERT INTO _kilnx_jobs (name, params, state, max_attempts) VALUES (:name, :params, 'available', :max_attempts)`,
		map[string]string{
			"name":         name,
			"params":       string(paramsJSON),
			"max_attempts": fmt.Sprintf("%d", maxAttempts),
		})
	if err != nil {
		return fmt.Errorf("enqueue error: %w", err)
	}

	fmt.Printf("  enqueued job '%s'\n", name)
	return nil
}

// pollQueue continuously polls _kilnx_jobs for available work
func (jq *JobQueue) pollQueue() {
	for {
		time.Sleep(1 * time.Second)
		jq.processNextJob()
	}
}

func (jq *JobQueue) processNextJob() {
	db := jq.server.db
	if db == nil {
		return
	}

	// Fetch next available job
	rows, err := db.QueryRows(
		`SELECT id, name, params, attempts, max_attempts FROM _kilnx_jobs
		 WHERE state = 'available' AND scheduled_at <= datetime('now')
		 ORDER BY id LIMIT 1`)
	if err != nil || len(rows) == 0 {
		return
	}

	row := rows[0]
	jobID := row["id"]
	jobName := row["name"]

	job, ok := jq.jobs[jobName]
	if !ok {
		db.ExecWithParams(
			`UPDATE _kilnx_jobs SET state = 'discarded', last_error = 'unknown job type' WHERE id = :id`,
			map[string]string{"id": jobID})
		return
	}

	// Mark as executing
	db.ExecWithParams(
		`UPDATE _kilnx_jobs SET state = 'executing', started_at = datetime('now') WHERE id = :id`,
		map[string]string{"id": jobID})

	// Deserialize params
	var params map[string]string
	if err := json.Unmarshal([]byte(row["params"]), &params); err != nil {
		params = make(map[string]string)
	}

	fmt.Printf("  running job '%s' (id=%s)\n", jobName, jobID)

	// Execute with panic recovery
	execErr := jq.safeExecuteNodes(job.Body, params)

	if execErr != nil {
		attempts := 1
		fmt.Sscanf(row["attempts"], "%d", &attempts)
		attempts++
		maxAttempts := 1
		fmt.Sscanf(row["max_attempts"], "%d", &maxAttempts)

		if attempts < maxAttempts {
			// Retry with exponential backoff
			backoffSecs := int(math.Pow(2, float64(attempts)))
			db.ExecWithParams(
				`UPDATE _kilnx_jobs SET state = 'available', attempts = :attempts,
				 last_error = :error, scheduled_at = datetime('now', '+' || :backoff || ' seconds')
				 WHERE id = :id`,
				map[string]string{
					"id":       jobID,
					"attempts": fmt.Sprintf("%d", attempts),
					"error":    execErr.Error(),
					"backoff":  fmt.Sprintf("%d", backoffSecs),
				})
			fmt.Printf("  job '%s' failed, retry %d/%d in %ds\n", jobName, attempts, maxAttempts, backoffSecs)
		} else {
			db.ExecWithParams(
				`UPDATE _kilnx_jobs SET state = 'discarded', attempts = :attempts,
				 last_error = :error, completed_at = datetime('now') WHERE id = :id`,
				map[string]string{
					"id":       jobID,
					"attempts": fmt.Sprintf("%d", attempts),
					"error":    execErr.Error(),
				})
			fmt.Printf("  job '%s' discarded after %d attempts\n", jobName, attempts)
		}
	} else {
		db.ExecWithParams(
			`UPDATE _kilnx_jobs SET state = 'completed', completed_at = datetime('now') WHERE id = :id`,
			map[string]string{"id": jobID})
		fmt.Printf("  job '%s' completed\n", jobName)
	}
}

// safeExecuteNodes runs nodes with panic recovery, returning any error
func (jq *JobQueue) safeExecuteNodes(nodes []parser.Node, params map[string]string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return jq.server.executeNodes(nodes, params)
}

// executeNodes runs a list of nodes (for schedules and jobs).
// Returns the first error encountered during query execution.
func (s *Server) executeNodes(nodes []parser.Node, params map[string]string) error {
	if params == nil {
		params = make(map[string]string)
	}

	ctx := &renderContext{
		queries:  make(map[string][]database.Row),
		paginate: make(map[string]PaginateInfo),
	}

	for _, node := range nodes {
		switch node.Type {
		case parser.NodeQuery:
			if s.db == nil {
				continue
			}

			// Schedules and jobs run without a logged-in user. If the SQL
			// touches a tenant-scoped table the rewriter will refuse it;
			// that is intentional. Developers wanting to operate across
			// tenants in batch jobs must use the (forthcoming) unscoped
			// escape hatch guarded by role checks.
			sql, tErr := RewriteTenantSQL(node.SQL, s.tenants, params)
			if tErr != nil {
				s.logger.LogError("tenant guard rejected scheduled query", tErr)
				return fmt.Errorf("tenant guard: %w", tErr)
			}

			// Check if it's a SELECT or a mutation
			trimmed := strings.TrimSpace(strings.ToUpper(sql))
			if strings.HasPrefix(trimmed, "SELECT") {
				rows, err := s.db.QueryRowsWithParams(sql, params)
				if err != nil {
					s.logger.LogError("schedule/job query error", err)
					return fmt.Errorf("query error: %w", err)
				}
				name := node.Name
				if name == "" {
					name = "_last"
				}
				ctx.queries[name] = rows
			} else {
				err := s.db.ExecWithParams(sql, params)
				if err != nil {
					s.logger.LogError("schedule/job exec error", err)
					return fmt.Errorf("exec error: %w", err)
				}
			}

		case parser.NodeFetch:
			fetchName := node.Name
			if fetchName == "" {
				fetchName = "_fetch"
			}
			rows, err := executeFetch(node, params)
			if err != nil {
				fmt.Printf("  fetch error: %v\n", err)
			} else {
				ctx.queries[fetchName] = rows
				// Make fetch results available as params for subsequent nodes
				if len(rows) > 0 {
					for k, v := range rows[0] {
						params["fetch."+k] = v
					}
				}
			}

		case parser.NodeSendEmail:
			recipient := resolveEmailRecipient(node.EmailTo, params)
			// Resolve recipient from SQL query if specified
			if toQuery, ok := node.Props["to_query"]; ok && toQuery != "" && s.db != nil {
				scopedQuery, tErr := RewriteTenantSQL(toQuery, s.tenants, params)
				if tErr != nil {
					s.logger.LogError("tenant guard rejected recipient query", tErr)
				} else {
					rows, err := s.db.QueryRowsWithParams(scopedQuery, params)
					if err == nil && len(rows) > 0 {
						for _, v := range rows[0] {
							recipient = v
							break
						}
					}
				}
			}
			subject := node.EmailSubject
			body := node.Props["body"]
			if body == "" {
				body = subject
			}

			// Interpolate from query results
			subject = interpolate(subject, ctx)
			body = interpolate(body, ctx)
			recipient = interpolate(recipient, ctx)

			// Check for attachment
			attach := node.EmailAttach
			if attach != "" {
				if strings.HasPrefix(attach, "_") {
					if val, ok := params[attach]; ok {
						attach = val
					}
				}
				if err := SendEmailWithAttachment(recipient, subject, body, attach); err != nil {
					fmt.Printf("  email error: %v\n", err)
					return fmt.Errorf("email error: %w", err)
				}
			} else {
				if err := SendEmail(recipient, subject, body); err != nil {
					fmt.Printf("  email error: %v\n", err)
					return fmt.Errorf("email error: %w", err)
				}
			}

		case parser.NodeValidate:
			if len(node.Validations) > 0 {
				errors := validateInlineRules(node.Validations, params)
				if len(errors) > 0 {
					fmt.Printf("  validation failed: %v\n", errors)
					return fmt.Errorf("validation failed: %v", errors)
				}
			}

		case parser.NodeGeneratePDF:
			// Generate PDF from query data
			doc := pdf.NewDocument()
			doc.SetTitle(node.TemplateName)
			doc.SetFooter("Page {page} of {pages}")
			page := doc.AddPage()
			page.AddHeading(node.TemplateName)
			page.AddSpace(10)

			// Get data from query results
			dataName := node.DataQueryName
			if rows, ok := ctx.queries[dataName]; ok && len(rows) > 0 {
				// Build table headers from first row keys
				var headers []string
				for key := range rows[0] {
					headers = append(headers, key)
				}
				// Build table rows
				var tableRows [][]string
				for _, row := range rows {
					var tr []string
					for _, h := range headers {
						tr = append(tr, row[h])
					}
					tableRows = append(tableRows, tr)
				}
				page.AddTable(headers, tableRows)
			}

			pdfBytes := doc.Render()
			tmpFile, err := os.CreateTemp("", "kilnx-*.pdf")
			if err != nil {
				fmt.Printf("  pdf generation error: %v\n", err)
				return fmt.Errorf("pdf generation error: %w", err)
			}
			if _, err := tmpFile.Write(pdfBytes); err != nil {
				fmt.Printf("  pdf write error: %v\n", err)
				tmpFile.Close()
				os.Remove(tmpFile.Name())
				return fmt.Errorf("pdf write error: %w", err)
			}
			tmpFile.Close()
			params["_generated_pdf"] = tmpFile.Name()
			fmt.Printf("  generated pdf: %s\n", tmpFile.Name())

		case parser.NodeEnqueue:
			if s.jobQueue != nil {
				resolvedParams := make(map[string]string)
				for k, v := range node.JobParams {
					if strings.HasPrefix(v, ":") {
						paramName := strings.TrimPrefix(v, ":")
						if val, ok := params[paramName]; ok {
							resolvedParams[k] = val
							continue
						}
					}
					resolvedParams[k] = v
				}
				if err := s.jobQueue.Enqueue(node.JobName, resolvedParams); err != nil {
					fmt.Printf("  enqueue error: %v\n", err)
					return fmt.Errorf("enqueue error: %w", err)
				}
			}
		}
	}

	return nil
}
