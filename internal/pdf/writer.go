package pdf

import "fmt"

// pdfWriter serializes a Document into valid PDF 1.4 binary format
type pdfWriter struct {
	buf     []byte
	offsets []int // byte offset of each object (1-indexed, offsets[0] unused)
	nextObj int
}

func newPDFWriter() *pdfWriter {
	return &pdfWriter{nextObj: 1}
}

func (w *pdfWriter) allocObj() int {
	id := w.nextObj
	w.nextObj++
	return id
}

func (w *pdfWriter) write(s string) {
	w.buf = append(w.buf, s...)
}

func (w *pdfWriter) writeF(format string, args ...interface{}) {
	w.buf = append(w.buf, fmt.Sprintf(format, args...)...)
}

func (w *pdfWriter) startObj(id int) {
	for len(w.offsets) <= id {
		w.offsets = append(w.offsets, 0)
	}
	w.offsets[id] = len(w.buf)
	w.writeF("%d 0 obj\n", id)
}

func (w *pdfWriter) endObj() {
	w.write("endobj\n")
}

func (w *pdfWriter) render(doc *Document) []byte {
	// Pre-allocate object IDs
	catalogID := w.allocObj()  // 1
	pagesID := w.allocObj()    // 2
	fontHelvID := w.allocObj() // 3
	fontBoldID := w.allocObj() // 4

	// Build content for each logical page
	type renderedPage struct {
		contentStreams [][]byte
	}

	var allPageObjIDs []int
	allContentPairs := [][2]int{} // [pageObjID, contentObjID]

	// Render pages from document ops
	for _, pg := range doc.pages {
		contentPages := renderPageContent(pg, doc)
		for _, content := range contentPages {
			pageObjID := w.allocObj()
			contentObjID := w.allocObj()
			allPageObjIDs = append(allPageObjIDs, pageObjID)
			allContentPairs = append(allContentPairs, [2]int{pageObjID, contentObjID})
			_ = content // store for later
		}
	}

	// Re-render to get content bytes (we need IDs first for page tree)
	// Reset and redo with known IDs
	w.buf = nil
	w.offsets = nil
	w.nextObj = 1

	catalogID = w.allocObj()
	pagesID = w.allocObj()
	fontHelvID = w.allocObj()
	fontBoldID = w.allocObj()

	// Collect all rendered page content
	type pageEntry struct {
		pageID    int
		contentID int
		content   []byte
		size      PageSize
	}
	var pages []pageEntry

	for _, pg := range doc.pages {
		contentPages := renderPageContent(pg, doc)
		for _, content := range contentPages {
			pID := w.allocObj()
			cID := w.allocObj()
			pages = append(pages, pageEntry{
				pageID:    pID,
				contentID: cID,
				content:   content,
				size:      pg.size,
			})
		}
	}

	totalPages := len(pages)

	// Write PDF header
	w.write("%PDF-1.4\n")
	w.write("%\xE2\xE3\xCF\xD3\n") // binary comment for PDF identification

	// Catalog
	w.startObj(catalogID)
	w.writeF("<< /Type /Catalog /Pages %d 0 R >>\n", pagesID)
	w.endObj()

	// Pages tree
	w.startObj(pagesID)
	w.write("<< /Type /Pages /Kids [")
	for i, pg := range pages {
		if i > 0 {
			w.write(" ")
		}
		w.writeF("%d 0 R", pg.pageID)
	}
	w.writeF("] /Count %d >>\n", totalPages)
	w.endObj()

	// Font: Helvetica
	w.startObj(fontHelvID)
	w.write("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>\n")
	w.endObj()

	// Font: Helvetica-Bold
	w.startObj(fontBoldID)
	w.write("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>\n")
	w.endObj()

	// Page objects and content streams
	for _, pg := range pages {
		w.startObj(pg.pageID)
		w.writeF("<< /Type /Page /Parent %d 0 R /MediaBox [0 0 %.2f %.2f] /Contents %d 0 R /Resources << /Font << /F1 %d 0 R /F2 %d 0 R >> >> >>\n",
			pagesID, pg.size.Width, pg.size.Height, pg.contentID, fontHelvID, fontBoldID)
		w.endObj()

		w.startObj(pg.contentID)
		w.writeF("<< /Length %d >>\n", len(pg.content))
		w.write("stream\n")
		w.buf = append(w.buf, pg.content...)
		w.write("\nendstream\n")
		w.endObj()
	}

	// Cross-reference table
	xrefOffset := len(w.buf)
	w.write("xref\n")
	w.writeF("0 %d\n", w.nextObj)
	w.write("0000000000 65535 f \n")
	for i := 1; i < w.nextObj; i++ {
		offset := 0
		if i < len(w.offsets) {
			offset = w.offsets[i]
		}
		w.writeF("%010d 00000 n \n", offset)
	}

	// Trailer
	w.write("trailer\n")
	w.writeF("<< /Size %d /Root %d 0 R ", w.nextObj, catalogID)
	if doc.title != "" {
		w.writeF("/Info << /Title %s >> ", escapePDFString(encodeWinAnsi(doc.title)))
	}
	w.write(">>\n")
	w.write("startxref\n")
	w.writeF("%d\n", xrefOffset)
	w.write("%%EOF\n")

	return w.buf
}

// renderPageContent renders a page's operations into one or more content stream byte slices
// (multiple if content overflows and creates extra pages)
func renderPageContent(pg *Page, doc *Document) [][]byte {
	margins := pg.margins
	if margins.Top == 0 && margins.Bottom == 0 {
		margins = Margins{Top: 72, Right: 72, Bottom: 72, Left: 72}
	}

	pageW := pg.size.Width
	pageH := pg.size.Height
	contentW := pageW - margins.Left - margins.Right
	startY := pageH - margins.Top
	bottomY := margins.Bottom + 30 // reserve space for footer

	var allStreams [][]byte
	pageNum := 0

	finishPage := func(sb *streamBuilder) {
		pageNum++
		// Render footer
		if doc.footer != "" {
			totalPages := estimatePageCount(pg, doc, margins, bottomY-30)
			footer := doc.footer
			footer = replaceAll(footer, "{page}", fmt.Sprintf("%d", pageNum))
			footer = replaceAll(footer, "{pages}", fmt.Sprintf("%d", totalPages))
			encoded := encodeWinAnsi(footer)
			fw := HelveticaMetrics.TextWidth(footer, 8)
			fx := margins.Left + (contentW-fw)/2
			sb.writeF("BT\n/F1 8 Tf\n0.5 0.5 0.5 rg\n%.2f %.2f Td %s Tj\nET\n",
				fx, margins.Bottom, escapePDFString(encoded))
		}
		allStreams = append(allStreams, sb.data)
	}

	sb := &streamBuilder{}
	curY := startY

	for _, op := range pg.ops {
		switch op.kind {
		case opHeading:
			needed := 24.0
			if curY-needed < bottomY {
				finishPage(sb)
				sb = &streamBuilder{}
				curY = startY
			}
			encoded := encodeWinAnsi(op.text)
			sb.writeF("BT\n/F2 18 Tf\n0 0 0 rg\n%.2f %.2f Td %s Tj\nET\n",
				margins.Left, curY, escapePDFString(encoded))
			curY -= 24

		case opText:
			lines := wrapText(op.text, contentW, &HelveticaMetrics, 11)
			for _, line := range lines {
				needed := 15.0
				if curY-needed < bottomY {
					finishPage(sb)
					sb = &streamBuilder{}
					curY = startY
				}
				encoded := encodeWinAnsi(line)
				sb.writeF("BT\n/F1 11 Tf\n0 0 0 rg\n%.2f %.2f Td %s Tj\nET\n",
					margins.Left, curY, escapePDFString(encoded))
				curY -= 15
			}

		case opSpace:
			curY -= op.space

		case opTable:
			if op.table != nil {
				tr := newTableRenderer(op.table.Headers, op.table.Rows, contentW)
				maxH := curY - bottomY
				if maxH < tr.rowHeight*2 {
					finishPage(sb)
					sb = &streamBuilder{}
					curY = startY
					maxH = curY - bottomY
				}
				consumed, rendered := tr.render(sb, margins.Left, curY, maxH)
				curY -= consumed

				// If not all rows rendered, continue on new pages
				remaining := op.table.Rows[rendered:]
				for len(remaining) > 0 {
					finishPage(sb)
					sb = &streamBuilder{}
					curY = startY
					maxH = curY - bottomY
					tr2 := newTableRenderer(op.table.Headers, remaining, contentW)
					consumed2, rendered2 := tr2.render(sb, margins.Left, curY, maxH)
					curY -= consumed2
					remaining = remaining[rendered2:]
					if rendered2 == 0 {
						break // safety: avoid infinite loop
					}
				}
			}
		}
	}

	finishPage(sb)
	return allStreams
}

// estimatePageCount gives a rough page count for footer rendering
func estimatePageCount(pg *Page, doc *Document, margins Margins, usableHeight float64) int {
	totalHeight := 0.0
	for _, op := range pg.ops {
		switch op.kind {
		case opHeading:
			totalHeight += 24
		case opText:
			contentW := pg.size.Width - margins.Left - margins.Right
			lines := wrapText(op.text, contentW, &HelveticaMetrics, 11)
			totalHeight += float64(len(lines)) * 15
		case opSpace:
			totalHeight += op.space
		case opTable:
			if op.table != nil {
				totalHeight += 20 * float64(1+len(op.table.Rows))
			}
		}
	}
	pages := int(totalHeight/usableHeight) + 1
	if pages < 1 {
		pages = 1
	}
	return pages
}

// wrapText wraps a string into lines that fit within maxWidth at the given font size
func wrapText(text string, maxWidth float64, fm *FontMetrics, fontSize float64) []string {
	if text == "" {
		return []string{""}
	}

	words := splitWords(text)
	var lines []string
	current := ""

	for _, word := range words {
		test := current
		if test != "" {
			test += " "
		}
		test += word

		if fm.TextWidth(test, fontSize) > maxWidth && current != "" {
			lines = append(lines, current)
			current = word
		} else {
			current = test
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	if len(lines) == 0 {
		lines = []string{""}
	}
	return lines
}

func splitWords(s string) []string {
	var words []string
	current := ""
	for _, r := range s {
		if r == ' ' || r == '\t' {
			if current != "" {
				words = append(words, current)
				current = ""
			}
		} else {
			current += string(r)
		}
	}
	if current != "" {
		words = append(words, current)
	}
	return words
}

func replaceAll(s, old, new string) string {
	result := ""
	for i := 0; i < len(s); {
		if i+len(old) <= len(s) && s[i:i+len(old)] == old {
			result += new
			i += len(old)
		} else {
			result += string(s[i])
			i++
		}
	}
	return result
}
