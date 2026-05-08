// Package analyzer performs static analysis on the parser AST: type
// checking of field constraints, validation of route templates,
// permission/role checks, dead-link detection, and security audits.
//
// The analyzer runs after parsing and before optimization or runtime
// execution. It is also the implementation behind `kilnx check`. Errors
// and warnings are returned as structured diagnostics so callers (CLI,
// CI) can format them.
//
// Public surface centers on Analyze, which takes a parser.App and
// returns a slice of diagnostics. Subordinate checks live in
// db_check.go (database/SQL coherence) and security.go (auth/CSRF).
package analyzer
