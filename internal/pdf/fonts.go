package pdf

// Built-in PDF font metrics for Helvetica and Helvetica-Bold.
// Widths are in units of 1/1000 of a text space unit (standard PDF glyph metrics).
// WinAnsiEncoding covers Latin-1 characters including Portuguese accents.

// FontMetrics holds glyph widths for a built-in font
type FontMetrics struct {
	Name   string
	Widths [256]int // glyph width per WinAnsiEncoding code point
}

// Helvetica metrics (approximate, standard PDF core font)
var helveticaWidths = [256]int{
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	278, 278, 355, 556, 556, 889, 667, 191, 333, 333, 389, 584, 278, 333, 278, 278, // 32-47
	556, 556, 556, 556, 556, 556, 556, 556, 556, 556, 278, 278, 584, 584, 584, 556, // 48-63
	1015, 667, 667, 722, 722, 667, 611, 778, 722, 278, 500, 667, 556, 833, 722, 778, // 64-79
	667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611, 278, 278, 278, 469, 556, // 80-95
	333, 556, 556, 500, 556, 556, 278, 556, 556, 222, 222, 500, 222, 833, 556, 556, // 96-111
	556, 556, 333, 500, 278, 556, 500, 722, 500, 500, 500, 334, 260, 334, 584, 0, // 112-127
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, // 128-143
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, // 144-159
	278, 333, 556, 556, 556, 556, 260, 556, 333, 737, 370, 556, 584, 333, 737, 333, // 160-175
	400, 584, 333, 333, 333, 556, 537, 278, 333, 333, 365, 556, 834, 834, 834, 611, // 176-191
	667, 667, 667, 667, 667, 667, 1000, 722, 667, 667, 667, 667, 278, 278, 278, 278, // 192-207 (ÀÁÂÃÄÅ...)
	722, 722, 778, 778, 778, 778, 778, 584, 778, 722, 722, 722, 722, 667, 667, 611, // 208-223 (ÐÑÒ...)
	556, 556, 556, 556, 556, 556, 889, 500, 556, 556, 556, 556, 278, 278, 278, 278, // 224-239 (àáâ...)
	556, 556, 556, 556, 556, 556, 556, 584, 611, 556, 556, 556, 556, 500, 556, 500, // 240-255 (ðñò...)
}

// Helvetica-Bold metrics (approximate)
var helveticaBoldWidths = [256]int{
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	278, 333, 474, 556, 556, 889, 722, 238, 333, 333, 389, 584, 278, 333, 278, 278,
	556, 556, 556, 556, 556, 556, 556, 556, 556, 556, 333, 333, 584, 584, 584, 611,
	975, 722, 722, 722, 722, 667, 611, 778, 722, 278, 556, 722, 611, 833, 722, 778,
	667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611, 333, 278, 333, 584, 556,
	333, 556, 611, 556, 611, 556, 333, 611, 611, 278, 278, 556, 278, 889, 611, 611,
	611, 611, 389, 556, 333, 611, 556, 778, 556, 556, 500, 389, 280, 389, 584, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	278, 333, 556, 556, 556, 556, 280, 556, 333, 737, 370, 556, 584, 333, 737, 333,
	400, 584, 333, 333, 333, 611, 556, 278, 333, 333, 365, 556, 834, 834, 834, 611,
	722, 722, 722, 722, 722, 722, 1000, 722, 667, 667, 667, 667, 278, 278, 278, 278,
	722, 722, 778, 778, 778, 778, 778, 584, 778, 722, 722, 722, 722, 667, 667, 611,
	556, 556, 556, 556, 556, 556, 889, 556, 556, 556, 556, 556, 278, 278, 278, 278,
	611, 611, 611, 611, 611, 611, 611, 584, 611, 611, 611, 611, 611, 556, 611, 556,
}

var HelveticaMetrics = FontMetrics{
	Name:   "Helvetica",
	Widths: helveticaWidths,
}

var HelveticaBoldMetrics = FontMetrics{
	Name:   "Helvetica-Bold",
	Widths: helveticaBoldWidths,
}

// TextWidth calculates the width of a string in points at the given font size
func (fm *FontMetrics) TextWidth(text string, fontSize float64) float64 {
	var total int
	for _, ch := range text {
		code := int(ch)
		if code >= 0 && code < 256 {
			total += fm.Widths[code]
		} else {
			total += 500 // default width for unmapped characters
		}
	}
	return float64(total) * fontSize / 1000.0
}

// encodeWinAnsi converts a UTF-8 string to WinAnsiEncoding bytes.
// Covers ASCII and Latin-1 supplement (Portuguese accents, etc.).
func encodeWinAnsi(s string) []byte {
	var out []byte
	for _, r := range s {
		if r < 128 {
			out = append(out, byte(r))
		} else if r >= 0xA0 && r <= 0xFF {
			// Direct Latin-1 supplement mapping
			out = append(out, byte(r))
		} else {
			// Map some common Unicode chars to WinAnsi
			switch r {
			case 0x2013: // en dash
				out = append(out, 0x96)
			case 0x2014: // em dash
				out = append(out, 0x97)
			case 0x2018: // left single quote
				out = append(out, 0x91)
			case 0x2019: // right single quote
				out = append(out, 0x92)
			case 0x201C: // left double quote
				out = append(out, 0x93)
			case 0x201D: // right double quote
				out = append(out, 0x94)
			case 0x2022: // bullet
				out = append(out, 0x95)
			case 0x20AC: // euro sign
				out = append(out, 0x80)
			default:
				out = append(out, '?')
			}
		}
	}
	return out
}

// escapePDFString escapes a WinAnsi byte slice for use in a PDF string literal
func escapePDFString(data []byte) string {
	var out []byte
	out = append(out, '(')
	for _, b := range data {
		switch b {
		case '(', ')', '\\':
			out = append(out, '\\', b)
		default:
			out = append(out, b)
		}
	}
	out = append(out, ')')
	return string(out)
}
