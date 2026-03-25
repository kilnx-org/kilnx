package pdf

// Document represents a PDF document being built
type Document struct {
	title    string
	footer   string
	pages    []*Page
	pageSize PageSize
}

// NewDocument creates a new PDF document with A4 page size
func NewDocument() *Document {
	return &Document{
		pageSize: A4,
	}
}

// SetTitle sets the document title (appears in PDF metadata)
func (d *Document) SetTitle(title string) {
	d.title = title
}

// SetFooter sets the footer text. Use {page} and {pages} as placeholders.
func (d *Document) SetFooter(footer string) {
	d.footer = footer
}

// SetPageSize sets the page size for new pages
func (d *Document) SetPageSize(size PageSize) {
	d.pageSize = size
}

// AddPage adds a new page and returns it for adding content
func (d *Document) AddPage() *Page {
	p := &Page{
		size:     d.pageSize,
		margins:  Margins{Top: 72, Right: 72, Bottom: 72, Left: 72},
		document: d,
	}
	d.pages = append(d.pages, p)
	return p
}

// Render produces the final PDF bytes
func (d *Document) Render() []byte {
	if len(d.pages) == 0 {
		d.AddPage()
	}
	w := newPDFWriter()
	return w.render(d)
}
