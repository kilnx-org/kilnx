package runtime

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/kilnx-org/kilnx/internal/parser"
)

func TestFetchOGData_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head>
			<meta property="og:title" content="Test Title">
			<meta property="og:description" content="Test Description">
			<meta property="og:image" content="https://example.com/img.jpg">
			<meta property="og:site_name" content="Example">
			<title>Page Title</title>
		</head></html>`))
	}))
	defer ts.Close()

	og := fetchOGData(ts.URL)
	if og == nil {
		t.Fatal("expected og data, got nil")
	}
	if og.Title != "Test Title" {
		t.Errorf("title = %q, want Test Title", og.Title)
	}
	if og.Description != "Test Description" {
		t.Errorf("description = %q, want Test Description", og.Description)
	}
	if og.Image != "https://example.com/img.jpg" {
		t.Errorf("image = %q, want https://example.com/img.jpg", og.Image)
	}
	if og.SiteName != "Example" {
		t.Errorf("site_name = %q, want Example", og.SiteName)
	}
}

func TestFetchOGData_CacheHit(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head><meta property="og:title" content="Cached"></head></html>`))
	}))
	defer ts.Close()

	// First fetch populates cache
	og1 := fetchOGData(ts.URL)
	if og1 == nil || og1.Title != "Cached" {
		t.Fatal("first fetch failed")
	}

	// Second fetch should hit cache
	og2 := fetchOGData(ts.URL)
	if og2 == nil || og2.Title != "Cached" {
		t.Fatal("cache hit failed")
	}
}

func TestFetchOGData_Non200(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	og := fetchOGData(ts.URL)
	if og != nil {
		t.Error("expected nil for non-200 response")
	}
}

func TestFetchOGData_InvalidURL(t *testing.T) {
	og := fetchOGData("http://[invalid")
	if og != nil {
		t.Error("expected nil for invalid URL")
	}
}

func TestFetchOGData_FallbackTitle(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head><title>Fallback Title</title></head></html>`))
	}))
	defer ts.Close()

	og := fetchOGData(ts.URL)
	if og == nil {
		t.Fatal("expected og data, got nil")
	}
	if og.Title != "Fallback Title" {
		t.Errorf("title = %q, want Fallback Title", og.Title)
	}
}

func TestFetchOGData_TwitterCard(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head>
			<meta name="twitter:title" content="Twitter Title">
			<meta name="twitter:description" content="Twitter Desc">
			<meta name="twitter:image" content="https://example.com/tw.jpg">
		</head></html>`))
	}))
	defer ts.Close()

	og := fetchOGData(ts.URL)
	if og == nil {
		t.Fatal("expected og data, got nil")
	}
	if og.Title != "Twitter Title" {
		t.Errorf("title = %q, want Twitter Title", og.Title)
	}
	if og.Description != "Twitter Desc" {
		t.Errorf("description = %q, want Twitter Desc", og.Description)
	}
	if og.Image != "https://example.com/tw.jpg" {
		t.Errorf("image = %q, want https://example.com/tw.jpg", og.Image)
	}
}

func TestPrintRoutes(t *testing.T) {
	app := &parser.App{
		Pages: []parser.Page{
			{Path: "/", Method: "GET", Title: "Home"},
			{Path: "/about", Method: "GET"},
		},
		Actions: []parser.Page{
			{Path: "/contact", Method: "POST"},
		},
		Fragments: []parser.Page{
			{Path: "/header"},
		},
		APIs: []parser.Page{
			{Path: "/api/users", Method: "GET"},
		},
		Streams: []parser.Stream{
			{Path: "/events", IntervalSecs: 5},
		},
		Webhooks: []parser.Webhook{
			{Path: "/webhook"},
		},
		Jobs: []parser.Job{
			{Name: "cleanup"},
		},
		Schedules: []parser.Schedule{
			{Name: "daily", Cron: "0 0 * * *"},
		},
	}

	// Capture stdout by redirecting os.Stdout
	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	printRoutes(app)

	w.Close()
	os.Stdout = oldStdout

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if !strings.Contains(output, "Home") {
		t.Error("expected Home in output")
	}
	if !strings.Contains(output, "FRAG /header") {
		t.Error("expected FRAG /header in output")
	}
	if !strings.Contains(output, "API  GET /api/users") {
		t.Error("expected API GET /api/users in output")
	}
	if !strings.Contains(output, "SSE  /events") {
		t.Error("expected SSE /events in output")
	}
}


func TestUnfurlURLs_WithValidURL(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head>
			<meta property="og:title" content="Test Title">
			<meta property="og:description" content="Test Description">
			<meta property="og:image" content="https://example.com/img.jpg">
		</head></html>`))
	}))
	defer ts.Close()

	result := unfurlURLs("Check out " + ts.URL)
	if !strings.Contains(result, "Test Title") {
		t.Errorf("expected Test Title in unfurl, got %q", result)
	}
}

func TestUnfurlURLs_DuplicateURLs(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head>
			<meta property="og:title" content="Test Title">
		</head></html>`))
	}))
	defer ts.Close()

	result := unfurlURLs(ts.URL + " and " + ts.URL)
	// Should only unfurl once
	count := strings.Count(result, "Test Title")
	if count != 1 {
		t.Errorf("expected 1 unfurl for duplicate URLs, got %d", count)
	}
}

func TestUnfurlURLs_MaxThree(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head>
			<meta property="og:title" content="Test Title">
		</head></html>`))
	}))
	defer ts.Close()

	result := unfurlURLs(ts.URL + " " + ts.URL + "/1 " + ts.URL + "/2 " + ts.URL + "/3")
	count := strings.Count(result, "Test Title")
	if count != 3 {
		t.Errorf("expected max 3 unfurls, got %d", count)
	}
}
