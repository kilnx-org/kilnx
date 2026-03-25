package runtime

import (
	"fmt"
	"strings"
	"time"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/parser"
)

// StartScheduler launches goroutines for each schedule defined in the app
func (s *Server) StartScheduler() {
	app := s.getApp()
	for _, sched := range app.Schedules {
		go s.runSchedule(sched)
	}
	if len(app.Schedules) > 0 {
		fmt.Printf("Started %d schedule(s)\n", len(app.Schedules))
	}
}

func (s *Server) runSchedule(sched parser.Schedule) {
	// If it's a cron-style schedule (e.g., "every monday at 9:00")
	if sched.Cron != "" {
		s.runCronSchedule(sched)
		return
	}

	interval := time.Duration(sched.IntervalSecs) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	fmt.Printf("  schedule '%s' running every %s\n", sched.Name, interval)

	for range ticker.C {
		s.executeNodes(sched.Body, nil)
	}
}

// runCronSchedule handles "every monday at 9:00" style expressions
func (s *Server) runCronSchedule(sched parser.Schedule) {
	fmt.Printf("  schedule '%s' cron: %s\n", sched.Name, sched.Cron)

	for {
		next := nextCronOccurrence(sched.Cron)
		if next.IsZero() {
			fmt.Printf("  schedule '%s': could not parse cron expression\n", sched.Name)
			return
		}
		delay := time.Until(next)
		fmt.Printf("  schedule '%s' next run at %s (in %s)\n", sched.Name, next.Format("2006-01-02 15:04"), delay.Round(time.Second))
		time.Sleep(delay)
		s.executeNodes(sched.Body, nil)
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
	var hour, minute int

	parts := strings.Fields(expr)
	for i, p := range parts {
		if d, ok := dayMap[p]; ok {
			targetDay = d
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

	if targetDay < 0 {
		return time.Time{}
	}

	now := time.Now()
	// Find the next occurrence of the target day
	daysUntil := int(targetDay) - int(now.Weekday())
	if daysUntil < 0 {
		daysUntil += 7
	}
	if daysUntil == 0 {
		// Same day: check if the time has passed
		target := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
		if now.After(target) {
			daysUntil = 7
		}
	}

	next := time.Date(now.Year(), now.Month(), now.Day()+daysUntil, hour, minute, 0, 0, now.Location())
	return next
}

// JobQueue manages async background jobs
type JobQueue struct {
	server *Server
	jobs   map[string]parser.Job
	queue  chan jobRequest
}

type jobRequest struct {
	Name   string
	Params map[string]string
}

func NewJobQueue(server *Server) *JobQueue {
	jq := &JobQueue{
		server: server,
		jobs:   make(map[string]parser.Job),
		queue:  make(chan jobRequest, 100),
	}

	app := server.getApp()
	for _, job := range app.Jobs {
		jq.jobs[job.Name] = job
	}

	return jq
}

func (jq *JobQueue) Start() {
	go jq.processQueue()
	if len(jq.jobs) > 0 {
		fmt.Printf("Job queue ready (%d job type(s))\n", len(jq.jobs))
	}
}

func (jq *JobQueue) Enqueue(name string, params map[string]string) error {
	if _, ok := jq.jobs[name]; !ok {
		return fmt.Errorf("unknown job: %s", name)
	}
	jq.queue <- jobRequest{Name: name, Params: params}
	fmt.Printf("  enqueued job '%s'\n", name)
	return nil
}

func (jq *JobQueue) processQueue() {
	for req := range jq.queue {
		job, ok := jq.jobs[req.Name]
		if !ok {
			fmt.Printf("  job '%s' not found, skipping\n", req.Name)
			continue
		}

		fmt.Printf("  running job '%s'\n", req.Name)
		jq.server.executeNodes(job.Body, req.Params)
		fmt.Printf("  job '%s' completed\n", req.Name)
	}
}

// executeNodes runs a list of nodes (for schedules and jobs)
func (s *Server) executeNodes(nodes []parser.Node, params map[string]string) {
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

			sql := node.SQL

			// Check if it's a SELECT or a mutation
			trimmed := strings.TrimSpace(strings.ToUpper(sql))
			if strings.HasPrefix(trimmed, "SELECT") {
				rows, err := s.db.QueryRowsWithParams(sql, params)
				if err != nil {
					fmt.Printf("  schedule/job query error: %v\n", err)
					continue
				}
				name := node.Name
				if name == "" {
					name = "_last"
				}
				ctx.queries[name] = rows
			} else {
				err := s.db.ExecWithParams(sql, params)
				if err != nil {
					fmt.Printf("  schedule/job exec error: %v\n", err)
				}
			}

		case parser.NodeSendEmail:
			recipient := resolveEmailRecipient(node.EmailTo, params)
			subject := node.EmailSubject
			body := node.Props["body"]
			if body == "" {
				body = subject
			}

			// Interpolate from query results
			subject = interpolate(subject, ctx)
			body = interpolate(body, ctx)
			recipient = interpolate(recipient, ctx)

			if err := SendEmail(recipient, subject, body); err != nil {
				fmt.Printf("  email error: %v\n", err)
			}
		}
	}
}
