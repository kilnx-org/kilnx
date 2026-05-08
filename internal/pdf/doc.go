// Package pdf is a minimal, dependency-free PDF writer used by the
// runtime to render reports and printable views from .kilnx pages.
//
// The package emits a small subset of the PDF 1.4 specification (text,
// tables, basic fonts, page layout). It is intentionally not a general
// PDF library: it covers what Kilnx applications need to produce
// invoices, statements, and tabular reports without pulling in a heavy
// dependency.
//
// Build a document by creating a [Document], adding pages, and writing
// to an io.Writer.
package pdf
