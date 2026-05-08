# `internal/pdf`

> Package pdf is a minimal, dependency-free PDF writer used by the runtime to render reports and printable views from .kilnx pages.

| | |
|---|---|
| **Import path** | `github.com/kilnx-org/kilnx/internal/pdf` |
| **Source last touched** | `5da8498` (2026-05-08) |
| **Doc last touched** | `5da8498` (2026-05-08) |


## Overview

The package emits a small subset of the PDF 1.4 specification (text,
tables, basic fonts, page layout). It is intentionally not a general
PDF library: it covers what Kilnx applications need to produce
invoices, statements, and tabular reports without pulling in a heavy
dependency.

Build a document by creating a [Document], adding pages, and writing
to an io.Writer.

## Files

| File | Summary |
|------|---------|
| [`document.go`](../../../internal/pdf/document.go) | _no file-level doc_ |
| [`fonts.go`](../../../internal/pdf/fonts.go) | _no file-level doc_ |
| [`page.go`](../../../internal/pdf/page.go) | _no file-level doc_ |
| [`table.go`](../../../internal/pdf/table.go) | _no file-level doc_ |
| [`writer.go`](../../../internal/pdf/writer.go) | _no file-level doc_ |

## Types

### `Document`

```go
type Document struct {
	title		string
	footer		string
	pages		[]*Page
	pageSize	PageSize
}
```

Document is a minimal PDF builder (no external deps). Build a document by
adding pages via [Document.AddPage] and serialize it with [Document.Render].

### `FontMetrics`

```go
type FontMetrics struct {
	Name	string
	Widths	[256]int	// glyph width per WinAnsiEncoding code point
}
```

FontMetrics holds glyph widths for a built-in font

### `Margins`

```go
type Margins struct {
	Top	float64
	Right	float64
	Bottom	float64
	Left	float64
}
```

Margins defines page margins in points

### `Page`

```go
type Page struct {
	size		PageSize
	margins		Margins
	ops		[]pageOp
	document	*Document
}
```

Page represents a single PDF page with content operations

### `PageSize`

```go
type PageSize struct {
	Width	float64
	Height	float64
}
```

PageSize defines the dimensions of a page

### `opKind`

```go
type opKind int
```
### `pageOp`

```go
type pageOp struct {
	kind	opKind
	text	string
	space	float64
	table	*tableData
}
```
### `pdfWriter`

```go
type pdfWriter struct {
	buf	[]byte
	offsets	[]int	// byte offset of each object (1-indexed, offsets[0] unused)
	nextObj	int
}
```

pdfWriter serializes a Document into valid PDF 1.4 binary format

### `streamBuilder`

```go
type streamBuilder struct {
	data []byte
}
```

streamBuilder wraps a byte buffer for PDF content stream writing

### `tableData`

```go
type tableData struct {
	Headers	[]string
	Rows	[][]string
}
```
### `tableRenderer`

```go
type tableRenderer struct {
	headers		[]string
	rows		[][]string
	colWidths	[]float64
	tableWidth	float64
	rowHeight	float64
	headerBg	[3]float64	// RGB 0-1
	evenRowBg	[3]float64	// RGB 0-1
	borderGray	float64
}
```

tableRenderer handles rendering a table into PDF content stream operations

## Functions

### `NewDocument`

```go
func NewDocument() *Document
```

NewDocument creates a new PDF document with A4 page size

### `encodeWinAnsi`

```go
func encodeWinAnsi(s string) []byte
```

encodeWinAnsi converts a UTF-8 string to WinAnsiEncoding bytes.
Covers ASCII and Latin-1 supplement (Portuguese accents, etc.).

### `escapePDFString`

```go
func escapePDFString(data []byte) string
```

escapePDFString escapes a WinAnsi byte slice for use in a PDF string literal

### `estimatePageCount`

```go
func estimatePageCount(pg *Page, doc *Document, margins Margins, usableHeight float64) int
```

estimatePageCount gives a rough page count for footer rendering

### `newPDFWriter`

```go
func newPDFWriter() *pdfWriter
```
### `newTableRenderer`

```go
func newTableRenderer(headers []string, rows [][]string, availableWidth float64) *tableRenderer
```
### `renderPageContent`

```go
func renderPageContent(pg *Page, doc *Document) [][]byte
```

renderPageContent renders a page's operations into one or more content stream byte slices
(multiple if content overflows and creates extra pages)

### `replaceAll`

```go
func replaceAll(s, old, new string) string
```
### `splitWords`

```go
func splitWords(s string) []string
```
### `wrapText`

```go
func wrapText(text string, maxWidth float64, fm *FontMetrics, fontSize float64) []string
```

wrapText wraps a string into lines that fit within maxWidth at the given font size

### `(Document) AddPage`

```go
func (d *Document) AddPage() *Page
```

AddPage adds a new page and returns it for adding content

### `(Document) Render`

```go
func (d *Document) Render() []byte
```

Render produces the final PDF bytes

### `(Document) SetFooter`

```go
func (d *Document) SetFooter(footer string)
```

SetFooter sets the footer text. Use {page} and {pages} as placeholders.

### `(Document) SetPageSize`

```go
func (d *Document) SetPageSize(size PageSize)
```

SetPageSize sets the page size for new pages

### `(Document) SetTitle`

```go
func (d *Document) SetTitle(title string)
```

SetTitle sets the document title (appears in PDF metadata)

### `(FontMetrics) TextWidth`

```go
func (fm *FontMetrics) TextWidth(text string, fontSize float64) float64
```

TextWidth calculates the width of a string in points at the given font size

### `(Page) AddHeading`

```go
func (p *Page) AddHeading(text string)
```

AddHeading adds large bold heading text

### `(Page) AddSpace`

```go
func (p *Page) AddSpace(points float64)
```

AddSpace adds vertical space in points

### `(Page) AddTable`

```go
func (p *Page) AddTable(headers []string, rows [][]string)
```

AddTable adds a table with headers and rows

### `(Page) AddText`

```go
func (p *Page) AddText(text string)
```

AddText adds a regular paragraph

### `(Page) SetMargins`

```go
func (p *Page) SetMargins(top, right, bottom, left float64)
```

SetMargins sets page margins

### `(pdfWriter) allocObj`

```go
func (w *pdfWriter) allocObj() int
```
### `(pdfWriter) endObj`

```go
func (w *pdfWriter) endObj()
```
### `(pdfWriter) render`

```go
func (w *pdfWriter) render(doc *Document) []byte
```
### `(pdfWriter) startObj`

```go
func (w *pdfWriter) startObj(id int)
```
### `(pdfWriter) write`

```go
func (w *pdfWriter) write(s string)
```
### `(pdfWriter) writeF`

```go
func (w *pdfWriter) writeF(format string, args ...interface{})
```
### `(streamBuilder) writeF`

```go
func (sb *streamBuilder) writeF(format string, args ...interface{})
```
### `(streamBuilder) writeS`

```go
func (sb *streamBuilder) writeS(s string)
```
### `(tableRenderer) calculateColumnWidths`

```go
func (tr *tableRenderer) calculateColumnWidths(availableWidth float64)
```
### `(tableRenderer) render`

```go
func (tr *tableRenderer) render(buf *streamBuilder, x, y float64, maxHeight float64) (float64, int)
```

render writes the table into a content stream buffer at the given position.
Returns the height consumed.

### `(tableRenderer) totalHeight`

```go
func (tr *tableRenderer) totalHeight() float64
```

totalHeight returns the total height needed for the table


## Notes

<!-- MANUAL-NOTES START -->
# `internal/pdf`

Minimal, dependency-free PDF 1.4 writer used by the runtime to render reports and printable views.

## Purpose

Some Kilnx apps need to emit invoices, statements, and tabular reports. Rather than depend on a heavyweight PDF library, the runtime ships a small writer that handles exactly what those use cases need: text, headings, vertical spacing, and paginated tables.

This package is intentionally **not** a general PDF library. It does not handle images, vector graphics, custom fonts, encryption, forms, annotations, hyperlinks, or anything else outside the narrow report use case. Adding any of those is a non-goal: if you need them, switch to a real PDF library.

## File map

- [`doc.go`](../../../internal/pdf/doc.go): package doc.
- [`document.go`](../../../internal/pdf/document.go): high-level `Document` and `Page` collection. Public API entry point.
- [`page.go`](../../../internal/pdf/page.go): `Page`, `PageSize` (A4, Letter), `Margins`, content-op queue.
- [`fonts.go`](../../../internal/pdf/fonts.go): glyph metrics for Helvetica and Helvetica-Bold under WinAnsiEncoding, plus the WinAnsi encoder and PDF string escaper.
- [`table.go`](../../../internal/pdf/table.go): table layout and rendering into a content stream buffer (`tableRenderer`, `streamBuilder`).
- [`writer.go`](../../../internal/pdf/writer.go): low-level `pdfWriter` that emits the actual PDF 1.4 byte stream (header, objects, xref, trailer).

## Public surface

```go
func NewDocument() *Document
(d *Document) SetTitle(title string)
(d *Document) SetFooter(footer string)              // {page} and {pages} placeholders
(d *Document) SetPageSize(size PageSize)            // A4 (default) or Letter
(d *Document) AddPage() *Page
(d *Document) Render() []byte

(p *Page) AddHeading(text string)
(p *Page) AddText(text string)
(p *Page) AddSpace(points float64)
(p *Page) AddTable(headers []string, rows [][]string)
(p *Page) SetMargins(top, right, bottom, left float64)

A4, Letter PageSize
```

Build a document, fill pages with operations, call `Render()`, write the bytes.

## Document model

A `Document` holds a list of `*Page` and document-level metadata (title, footer, page size). Each `Page` is a queue of `pageOp` values (heading, text, space, table). Nothing is laid out at append time. `Document.Render` walks the queue and produces PDF bytes in a two-pass process inside `writer.go`.

Operations:

- `opHeading`: 18pt Helvetica-Bold, advances 24 points.
- `opText`: 11pt Helvetica, word-wrapped to content width via `wrapText`, 15 points per line.
- `opSpace`: pure vertical advance, no glyphs.
- `opTable`: routed through `tableRenderer` (see below).

Pagination is handled in `renderPageContent`: when an op would dip below `bottomY` (which reserves 30 points for the footer above the bottom margin), the current content stream is finished and a new page is started. Tables that overflow are split row-by-row across page boundaries; the header row is repeated on each continuation by recreating a `tableRenderer` for the remaining rows.

## Tables

`tableRenderer` (in `table.go`) does column sizing in one pass:

1. Compute "natural" width per column from the widest header (bold font) or cell (regular font), plus 10 points of padding.
2. Sum the natural widths. Whether the total is larger or smaller than the available content width, scale every column by `availableWidth/totalNatural`. So columns are always proportionally distributed across the full content area.
3. Truncate cell text by chopping bytes off the end until it fits within `colWidth - 8` at 9pt. **Byte-level truncation, not rune-level**, so multibyte UTF-8 strings can be cut mid-codepoint. In practice the WinAnsi encoder downstream catches the result, but the truncation is technically lossy on non-Latin text.

Visual style is fixed:

- Header row: dark blue (`{0.2, 0.4, 0.6}` RGB), white bold text.
- Even data rows: light gray zebra stripe (`{0.94, 0.94, 0.94}`).
- Borders: gray (`0.75`) horizontal lines under the header and between rows.
- Row height: 20 points. Font: 9pt.

These are constants. There is no theming API.

## Fonts and encoding

Two built-in fonts: Helvetica and Helvetica-Bold. They use the standard PDF core-font widths under WinAnsiEncoding, giving Latin-1 coverage including Portuguese accents.

`encodeWinAnsi` (in `fonts.go`) maps UTF-8 input to WinAnsi bytes:

- ASCII (`< 128`): pass through.
- Latin-1 supplement (`0xA0..0xFF`): pass through directly.
- A handful of common Unicode punctuation (en/em dash, smart quotes, bullet, euro): mapped to their WinAnsi positions.
- Anything else: emitted as `?`.

`escapePDFString` wraps a byte slice in PDF string-literal parentheses and escapes `(`, `)`, `\`. No hex-string fallback; only literal strings are emitted.

`FontMetrics.TextWidth` returns string width in points at a given font size. Glyphs not in the 256-entry WinAnsi width table fall back to a default of 500 units.

## PDF object structure

`pdfWriter.render` emits the standard PDF 1.4 layout:

1. `%PDF-1.4` header plus a four-byte binary comment so file-type sniffers recognise it.
2. Object 1: Catalog, pointing to the Pages object.
3. Object 2: Pages tree containing all per-page object IDs as `Kids` and a `Count`.
4. Objects 3 and 4: Helvetica and Helvetica-Bold Type1 font objects, both with WinAnsiEncoding.
5. Two objects per logical page: the Page dictionary (MediaBox, Contents reference, Font resources `F1`/`F2`) and the content stream object.
6. `xref` table with offsets recorded as the writer goes.
7. `trailer` with `/Size`, `/Root`, optional `/Info << /Title ... >>`, then `startxref` and `%%EOF`.

The render function does a small two-pass dance: it walks pages once to count them, throws away the buffer, resets allocators, then walks again with the now-known IDs. This is safer than threading IDs through the layout code.

## Gotchas

- **Subset only**. No images, no vector graphics, no custom fonts. Anything outside text/heading/space/table is out of scope. PRs adding general PDF features will be rejected unless they keep the dependency-free, narrow-purpose contract intact.
- **WinAnsi only**. Characters outside Latin-1 plus the small Unicode allowlist render as `?`. Non-Latin scripts (CJK, Arabic, etc.) cannot be rendered. The package exists for Brazilian/European business reports, not internationalized typesetting.
- **Byte truncation in tables**. `tableRenderer.render` truncates cell text via `text[:len(text)-1]` until it fits. Multibyte runes can be sliced; output is then unpredictable through `encodeWinAnsi`.
- **Naive `replaceAll`**. `writer.go` defines a private `replaceAll` rather than using `strings.ReplaceAll`. It is correct but quadratic. Footers are short, so it does not matter, just do not lift it.
- **Byte-level offsets in `xref`**. Anything that reorders writes inside `pdfWriter` must continue to call `startObj` exactly once before each object. Forgetting that produces a `xref` with stale offsets and a corrupt file.
- **Footer page count is an estimate**. `estimatePageCount` adds up op heights divided by usable height plus one. It is only used for the footer placeholder `{pages}`, so off-by-one errors are visible to the user.
- **A `Document` is single-use**. `Render` mutates internal state inside `pdfWriter` and is not idempotent across calls. If you need two outputs, build two documents.

## When to touch this package

- New op kind (e.g. images): add to `opKind` enum, extend `Page` API, render in `renderPageContent`.
- New page size: add a `PageSize` var to `page.go`.
- Locale-specific punctuation: extend the `encodeWinAnsi` switch.

For anything else, the right answer is usually "use a real PDF library outside the kilnx tree".
<!-- MANUAL-NOTES END -->
