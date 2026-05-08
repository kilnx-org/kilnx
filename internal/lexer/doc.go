// Package lexer transforms .kilnx source text into a token stream
// consumed by internal/parser.
//
// The lexer is indent-significant: leading whitespace at the start of a
// logical line determines block structure, and INDENT/DEDENT tokens are
// emitted when the indentation level changes (modeled after Python's
// tokenize module). Inline comments starting with '#' are stripped
// before tokenization.
//
// Top-level entry point is [Tokenize]. Helpers IsFieldType and
// IsFieldConstraint expose the lexer's keyword tables to the parser
// and analyzer.
package lexer
