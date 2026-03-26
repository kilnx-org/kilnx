package pdf

// PageSize defines the dimensions of a page
type PageSize struct {
	Width  float64
	Height float64
}

var (
	A4     = PageSize{Width: 595.28, Height: 841.89}
	Letter = PageSize{Width: 612, Height: 792}
)

// Margins defines page margins in points
type Margins struct {
	Top    float64
	Right  float64
	Bottom float64
	Left   float64
}

// Page represents a single PDF page with content operations
type Page struct {
	size     PageSize
	margins  Margins
	ops      []pageOp
	document *Document
}

type opKind int

const (
	opHeading opKind = iota
	opText
	opSpace
	opTable
)

type pageOp struct {
	kind  opKind
	text  string
	space float64
	table *tableData
}

type tableData struct {
	Headers []string
	Rows    [][]string
}

// AddHeading adds large bold heading text
func (p *Page) AddHeading(text string) {
	p.ops = append(p.ops, pageOp{kind: opHeading, text: text})
}

// AddText adds a regular paragraph
func (p *Page) AddText(text string) {
	p.ops = append(p.ops, pageOp{kind: opText, text: text})
}

// AddSpace adds vertical space in points
func (p *Page) AddSpace(points float64) {
	p.ops = append(p.ops, pageOp{kind: opSpace, space: points})
}

// SetMargins sets page margins
func (p *Page) SetMargins(top, right, bottom, left float64) {
	p.margins = Margins{Top: top, Right: right, Bottom: bottom, Left: left}
}

// AddTable adds a table with headers and rows
func (p *Page) AddTable(headers []string, rows [][]string) {
	p.ops = append(p.ops, pageOp{
		kind: opTable,
		table: &tableData{
			Headers: headers,
			Rows:    rows,
		},
	})
}
