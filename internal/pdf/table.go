package pdf

import "fmt"

// tableRenderer handles rendering a table into PDF content stream operations
type tableRenderer struct {
	headers    []string
	rows       [][]string
	colWidths  []float64
	tableWidth float64
	rowHeight  float64
	headerBg   [3]float64 // RGB 0-1
	evenRowBg  [3]float64 // RGB 0-1
	borderGray float64
}

func newTableRenderer(headers []string, rows [][]string, availableWidth float64) *tableRenderer {
	tr := &tableRenderer{
		headers:    headers,
		rows:       rows,
		rowHeight:  20,
		headerBg:   [3]float64{0.2, 0.4, 0.6},
		evenRowBg:  [3]float64{0.94, 0.94, 0.94},
		borderGray: 0.75,
		tableWidth: availableWidth,
	}
	tr.calculateColumnWidths(availableWidth)
	return tr
}

func (tr *tableRenderer) calculateColumnWidths(availableWidth float64) {
	numCols := len(tr.headers)
	if numCols == 0 {
		return
	}

	// Calculate max content width per column
	maxWidths := make([]float64, numCols)
	fm := &HelveticaMetrics
	fmBold := &HelveticaBoldMetrics
	fontSize := 9.0

	for i, h := range tr.headers {
		w := fmBold.TextWidth(h, fontSize) + 10 // padding
		if w > maxWidths[i] {
			maxWidths[i] = w
		}
	}

	for _, row := range tr.rows {
		for i := 0; i < numCols && i < len(row); i++ {
			w := fm.TextWidth(row[i], fontSize) + 10
			if w > maxWidths[i] {
				maxWidths[i] = w
			}
		}
	}

	// Distribute widths proportionally within available space
	totalNatural := 0.0
	for _, w := range maxWidths {
		totalNatural += w
	}

	tr.colWidths = make([]float64, numCols)
	if totalNatural <= availableWidth {
		// Expand proportionally
		scale := availableWidth / totalNatural
		for i, w := range maxWidths {
			tr.colWidths[i] = w * scale
		}
	} else {
		// Shrink proportionally
		scale := availableWidth / totalNatural
		for i, w := range maxWidths {
			tr.colWidths[i] = w * scale
		}
	}
}

// totalHeight returns the total height needed for the table
func (tr *tableRenderer) totalHeight() float64 {
	return tr.rowHeight * float64(1+len(tr.rows))
}

// render writes the table into a content stream buffer at the given position.
// Returns the height consumed.
func (tr *tableRenderer) render(buf *streamBuilder, x, y float64, maxHeight float64) (float64, int) {
	if len(tr.headers) == 0 {
		return 0, 0
	}

	curY := y
	rowsRendered := 0

	// Draw header row background
	buf.writeF("%.2f %.2f %.2f rg\n", tr.headerBg[0], tr.headerBg[1], tr.headerBg[2])
	buf.writeF("%.2f %.2f %.2f %.2f re f\n", x, curY-tr.rowHeight, tr.tableWidth, tr.rowHeight)

	// Header text (white, bold)
	buf.writeF("1 1 1 rg\n")
	buf.writeF("BT\n")
	buf.writeF("/F2 9 Tf\n")
	cellX := x
	for i, h := range tr.headers {
		textY := curY - tr.rowHeight + 6
		encoded := encodeWinAnsi(h)
		buf.writeF("%.2f %.2f Td %s Tj\n", cellX+4, textY, escapePDFString(encoded))
		if i < len(tr.colWidths) {
			cellX += tr.colWidths[i]
		}
	}
	buf.writeF("ET\n")

	curY -= tr.rowHeight

	// Draw border under header
	buf.writeF("%.2f G\n", tr.borderGray)
	buf.writeF("%.2f %.2f m %.2f %.2f l S\n", x, curY, x+tr.tableWidth, curY)

	// Data rows
	for rowIdx, row := range tr.rows {
		if y-curY+tr.rowHeight > maxHeight {
			break // page overflow
		}

		// Zebra striping (even rows get background)
		if rowIdx%2 == 0 {
			buf.writeF("%.2f %.2f %.2f rg\n", tr.evenRowBg[0], tr.evenRowBg[1], tr.evenRowBg[2])
			buf.writeF("%.2f %.2f %.2f %.2f re f\n", x, curY-tr.rowHeight, tr.tableWidth, tr.rowHeight)
		}

		// Row text
		buf.writeF("0 0 0 rg\n")
		buf.writeF("BT\n")
		buf.writeF("/F1 9 Tf\n")
		cellX = x
		for i := 0; i < len(tr.headers) && i < len(row); i++ {
			textY := curY - tr.rowHeight + 6
			text := row[i]
			// Truncate if too wide
			if i < len(tr.colWidths) {
				maxW := tr.colWidths[i] - 8
				fm := &HelveticaMetrics
				for len(text) > 0 && fm.TextWidth(text, 9) > maxW {
					text = text[:len(text)-1]
				}
			}
			encoded := encodeWinAnsi(text)
			buf.writeF("%.2f %.2f Td %s Tj\n", cellX+4, textY, escapePDFString(encoded))
			if i < len(tr.colWidths) {
				cellX += tr.colWidths[i]
			}
		}
		buf.writeF("ET\n")

		curY -= tr.rowHeight
		rowsRendered++

		// Border between rows
		buf.writeF("%.2f G\n", tr.borderGray)
		buf.writeF("%.2f %.2f m %.2f %.2f l S\n", x, curY, x+tr.tableWidth, curY)
	}

	return y - curY, rowsRendered
}

// streamBuilder wraps a byte buffer for PDF content stream writing
type streamBuilder struct {
	data []byte
}

func (sb *streamBuilder) writeF(format string, args ...interface{}) {
	sb.data = append(sb.data, fmt.Sprintf(format, args...)...)
}

func (sb *streamBuilder) writeS(s string) {
	sb.data = append(sb.data, s...)
}
