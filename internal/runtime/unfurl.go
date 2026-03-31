package runtime

import (
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

// unfurlCache stores fetched OG metadata to avoid re-fetching
var (
	unfurlCache   = make(map[string]*ogData)
	unfurlCacheMu sync.RWMutex
)

type ogData struct {
	Title       string
	Description string
	Image       string
	SiteName    string
	URL         string
	FetchedAt   time.Time
}

var ogTagRe = regexp.MustCompile(`<meta\s+(?:property|name)="(og:[^"]+|twitter:[^"]+)"\s+content="([^"]*)"`)
var ogTagRe2 = regexp.MustCompile(`<meta\s+content="([^"]*)"\s+(?:property|name)="(og:[^"]+|twitter:[^"]+)"`)
var titleTagRe = regexp.MustCompile(`(?i)<title[^>]*>([^<]+)</title>`)

// fetchOGData fetches Open Graph metadata from a URL
func fetchOGData(url string) *ogData {
	// Check cache first
	unfurlCacheMu.RLock()
	if cached, ok := unfurlCache[url]; ok {
		// Cache for 1 hour
		if time.Since(cached.FetchedAt) < time.Hour {
			unfurlCacheMu.RUnlock()
			return cached
		}
	}
	unfurlCacheMu.RUnlock()

	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Kilnx/1.0 (Link Preview)")
	req.Header.Set("Accept", "text/html")

	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil
	}

	// Read only first 64KB for OG tags (they're in <head>)
	limited := io.LimitReader(resp.Body, 64*1024)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil
	}

	content := string(body)
	og := &ogData{URL: url, FetchedAt: time.Now()}

	// Parse OG/Twitter meta tags
	tags := make(map[string]string)
	for _, match := range ogTagRe.FindAllStringSubmatch(content, -1) {
		tags[match[1]] = match[2]
	}
	for _, match := range ogTagRe2.FindAllStringSubmatch(content, -1) {
		tags[match[2]] = match[1]
	}

	og.Title = tags["og:title"]
	if og.Title == "" {
		og.Title = tags["twitter:title"]
	}
	og.Description = tags["og:description"]
	if og.Description == "" {
		og.Description = tags["twitter:description"]
	}
	og.Image = tags["og:image"]
	if og.Image == "" {
		og.Image = tags["twitter:image"]
	}
	og.SiteName = tags["og:site_name"]

	// Fallback to <title> tag
	if og.Title == "" {
		if m := titleTagRe.FindStringSubmatch(content); len(m) > 1 {
			og.Title = strings.TrimSpace(m[1])
		}
	}

	// Don't cache empty results
	if og.Title == "" && og.Description == "" {
		return nil
	}

	// Cache it
	unfurlCacheMu.Lock()
	// Limit cache size
	if len(unfurlCache) > 1000 {
		for k := range unfurlCache {
			delete(unfurlCache, k)
			break
		}
	}
	unfurlCache[url] = og
	unfurlCacheMu.Unlock()

	return og
}

// renderUnfurl generates an HTML card for unfurled link preview
func renderUnfurl(og *ogData) string {
	if og == nil {
		return ""
	}

	var b strings.Builder
	b.WriteString(`<div style="border-left:3px solid #3f3f46;margin:6px 0;padding:8px 12px;border-radius:0 6px 6px 0;background:rgba(255,255,255,0.03);max-width:400px">`)

	if og.SiteName != "" {
		b.WriteString(fmt.Sprintf(`<div style="font-size:11px;color:#9a9b9e;margin-bottom:2px">%s</div>`, html.EscapeString(og.SiteName)))
	}

	if og.Title != "" {
		b.WriteString(fmt.Sprintf(`<a href="%s" target="_blank" rel="noopener" style="color:#1d9bd1;font-size:14px;font-weight:600;text-decoration:none;display:block;margin-bottom:2px">%s</a>`,
			html.EscapeString(og.URL), html.EscapeString(og.Title)))
	}

	if og.Description != "" {
		desc := og.Description
		if len(desc) > 200 {
			desc = desc[:200] + "..."
		}
		b.WriteString(fmt.Sprintf(`<div style="font-size:13px;color:#9a9b9e;line-height:1.4">%s</div>`, html.EscapeString(desc)))
	}

	if og.Image != "" {
		b.WriteString(fmt.Sprintf(`<img src="%s" style="max-width:100%%;max-height:200px;border-radius:4px;margin-top:6px;display:block" loading="lazy">`,
			html.EscapeString(og.Image)))
	}

	b.WriteString(`</div>`)
	return b.String()
}

// unfurlURLs finds URLs in text and appends link preview cards
func unfurlURLs(text string) string {
	matches := urlRe.FindAllStringSubmatch(text, 3) // max 3 unfurls per message
	if len(matches) == 0 {
		return ""
	}

	var cards strings.Builder
	seen := make(map[string]bool)
	for _, m := range matches {
		url := m[2]
		if seen[url] {
			continue
		}
		seen[url] = true
		og := fetchOGData(url)
		if og != nil {
			cards.WriteString(renderUnfurl(og))
		}
	}
	return cards.String()
}
