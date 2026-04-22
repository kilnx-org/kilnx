package pdf

import (
	"testing"
)

func TestNewDocument(t *testing.T) {
	doc := NewDocument()
	if doc == nil {
		t.Fatal("NewDocument returned nil")
	}
	if len(doc.pages) != 0 {
		t.Errorf("expected 0 pages, got %d", len(doc.pages))
	}
}

func TestDocumentRenderEmpty(t *testing.T) {
	doc := NewDocument()
	bytes := doc.Render()
	if len(bytes) == 0 {
		t.Error("Render() returned empty bytes for empty document")
	}
	// PDF magic number
	if len(bytes) < 4 || string(bytes[:4]) != "%PDF" {
		t.Errorf("Render() did not produce a valid PDF header: %q", string(bytes[:min(4, len(bytes))]))
	}
}

func TestDocumentRenderWithContent(t *testing.T) {
	doc := NewDocument()
	doc.SetTitle("Test Report")
	doc.SetFooter("Page {page} of {pages}")

	page := doc.AddPage()
	page.AddHeading("Sales Report")
	page.AddSpace(10)
	page.AddText("This is a test paragraph.")
	page.AddTable(
		[]string{"Product", "Qty", "Price"},
		[][]string{
			{"Widget", "10", "$100"},
			{"Gadget", "5", "$250"},
		},
	)

	bytes := doc.Render()
	if len(bytes) == 0 {
		t.Error("Render() returned empty bytes")
	}
	if len(bytes) < 4 || string(bytes[:4]) != "%PDF" {
		t.Error("Render() did not produce a valid PDF header")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
