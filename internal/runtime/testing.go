package runtime

import (
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/parser"
)

// RunTests executes all test blocks and returns pass/fail counts
func RunTests(app *parser.App, db *database.DB, baseURL string) (passed, failed int) {
	for _, test := range app.Tests {
		ok := runSingleTest(test, app, db, baseURL)
		if ok {
			fmt.Printf("  PASS  %s\n", test.Name)
			passed++
		} else {
			fmt.Printf("  FAIL  %s\n", test.Name)
			failed++
		}
	}
	return
}

func runSingleTest(test parser.Test, app *parser.App, db *database.DB, baseURL string) bool {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	var lastBody string
	var lastStatus int
	var lastLocation string
	var lastURL string
	formData := make(url.Values)

	for _, step := range test.Steps {
		switch step.Action {
		case "as":
			// "as user with role editor" -> register + login a test user
			if app.Auth == nil {
				fmt.Printf("    no auth config, skipping 'as' step\n")
				continue
			}
			role := "viewer"
			if strings.Contains(step.Target, "role ") {
				parts := strings.SplitAfter(step.Target, "role ")
				if len(parts) > 1 {
					role = strings.TrimSpace(parts[1])
				}
			}

			email := fmt.Sprintf("test_%s@kilnx.test", role)
			password := "testpass123"

			// Create user directly in DB
			hash, _ := HashPassword(password)
			db.ExecWithParams(
				fmt.Sprintf("INSERT OR IGNORE INTO %s (name, email, password, role) VALUES (:name, :email, :password, :role)",
					app.Auth.Table),
				map[string]string{"name": "Test " + role, "email": email, "password": hash, "role": role},
			)

			// Login via HTTP
			resp, err := client.Get(baseURL + app.Auth.LoginPath)
			if err != nil {
				fmt.Printf("    login GET error: %v\n", err)
				return false
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			csrf := extractCSRFFromHTML(string(body))
			loginData := url.Values{
				"_csrf":    {csrf},
				"identity": {email},
				"password": {password},
			}
			resp, err = client.PostForm(baseURL+app.Auth.LoginPath, loginData)
			if err != nil {
				fmt.Printf("    login POST error: %v\n", err)
				return false
			}
			resp.Body.Close()
			lastStatus = resp.StatusCode

		case "visit":
			resp, err := client.Get(baseURL + step.Target)
			if err != nil {
				fmt.Printf("    visit error: %v\n", err)
				return false
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastBody = string(body)
			lastStatus = resp.StatusCode
			lastURL = step.Target
			lastLocation = resp.Header.Get("Location")

			// Follow redirects manually
			if resp.StatusCode == 303 || resp.StatusCode == 302 {
				loc := resp.Header.Get("Location")
				if loc != "" {
					resp2, _ := client.Get(baseURL + loc)
					if resp2 != nil {
						body2, _ := io.ReadAll(resp2.Body)
						resp2.Body.Close()
						lastBody = string(body2)
					}
				}
			}

			// Extract CSRF for forms
			csrf := extractCSRFFromHTML(lastBody)
			if csrf != "" {
				formData.Set("_csrf", csrf)
			}

		case "fill":
			formData.Set(step.Target, step.Value)

		case "submit":
			// Extract form action from the last page body
			action := extractFormAction(lastBody)
			if action == "" {
				action = lastURL
			}
			if action == "" {
				action = "/"
			}

			// Ensure CSRF token is present
			if formData.Get("_csrf") == "" {
				csrf := extractCSRFFromHTML(lastBody)
				if csrf != "" {
					formData.Set("_csrf", csrf)
				}
			}

			resp, err := client.PostForm(baseURL+action, formData)
			if err != nil {
				fmt.Printf("    submit error: %v\n", err)
				return false
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastBody = string(body)
			lastStatus = resp.StatusCode
			lastLocation = resp.Header.Get("Location")

			// Follow redirect after submit
			if resp.StatusCode == 302 || resp.StatusCode == 303 {
				loc := resp.Header.Get("Location")
				if loc != "" {
					resp2, _ := client.Get(baseURL + loc)
					if resp2 != nil {
						body2, _ := io.ReadAll(resp2.Body)
						resp2.Body.Close()
						lastBody = string(body2)
						lastStatus = resp2.StatusCode
					}
				}
			}

			formData = make(url.Values) // reset form

		case "expect":
			if !evaluateExpect(step, lastBody, lastStatus, lastLocation, db) {
				return false
			}
		}
	}

	return true
}

// evaluateExpect handles all expect assertion variants
func evaluateExpect(step parser.TestStep, lastBody string, lastStatus int, lastLocation string, db *database.DB) bool {
	target := step.Target

	// expect page contains "text" or expect page /path contains "text"
	if strings.Contains(target, "contains") {
		if !strings.Contains(lastBody, step.Value) {
			fmt.Printf("    expected page to contain %q\n", step.Value)
			return false
		}
		return true
	}

	// expect redirect to /path
	if strings.HasPrefix(target, "redirect to ") || strings.HasPrefix(target, "redirect ") {
		expectedPath := strings.TrimPrefix(target, "redirect to ")
		expectedPath = strings.TrimPrefix(expectedPath, "redirect ")
		expectedPath = strings.TrimSpace(expectedPath)
		if step.Value != "" {
			expectedPath = step.Value
		}
		if lastLocation != expectedPath {
			fmt.Printf("    expected redirect to %q, got %q\n", expectedPath, lastLocation)
			return false
		}
		return true
	}

	// expect status N
	if strings.HasPrefix(target, "status ") {
		expectedStr := strings.TrimPrefix(target, "status ")
		expected, err := strconv.Atoi(strings.TrimSpace(expectedStr))
		if err != nil {
			fmt.Printf("    invalid status code: %q\n", expectedStr)
			return false
		}
		if lastStatus != expected {
			fmt.Printf("    expected status %d, got %d\n", expected, lastStatus)
			return false
		}
		return true
	}

	// expect query: SQL returns N
	if strings.Contains(target, "returns") {
		idx := strings.Index(target, "query:")
		if idx >= 0 {
			sqlPart := target[idx+6:]
			retIdx := strings.Index(sqlPart, "returns")
			if retIdx >= 0 {
				sql := strings.TrimSpace(sqlPart[:retIdx])
				rows, err := db.QueryRows(sql)
				if err != nil {
					fmt.Printf("    query error: %v\n", err)
					return false
				}
				expected := strings.TrimSpace(step.Value)
				got := "0"
				if len(rows) > 0 {
					for _, v := range rows[0] {
						got = v
						break
					}
				}
				if got != expected {
					fmt.Printf("    expected query to return %s, got %s\n", expected, got)
					return false
				}
				return true
			}
		}
	}

	return true
}

// extractFormAction finds the action attribute of the first <form> in HTML
var formActionRe = regexp.MustCompile(`<form[^>]*\saction="([^"]*)"`)

func extractFormAction(htmlContent string) string {
	matches := formActionRe.FindStringSubmatch(htmlContent)
	if len(matches) >= 2 {
		return matches[1]
	}
	// Try method="POST" forms with action in different attribute order
	altRe := regexp.MustCompile(`<form[^>]*\saction='([^']*)'`)
	matches = altRe.FindStringSubmatch(htmlContent)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

func extractCSRFFromHTML(html string) string {
	idx := strings.Index(html, `name="_csrf" value="`)
	if idx < 0 {
		return ""
	}
	start := idx + len(`name="_csrf" value="`)
	end := strings.Index(html[start:], `"`)
	if end < 0 {
		return ""
	}
	return html[start : start+end]
}
