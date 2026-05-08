// Package optimizer rewrites the parser AST so that the runtime can
// execute it efficiently.
//
// Current responsibilities:
//
//   - Resolve named-query references ({queryName.field}, {^field},
//     {^^field}) into their target queries so each page/action carries
//     the SQL it actually needs.
//   - Inline simple template interpolations where safe.
//
// The optimizer is purely AST-to-AST: it takes a parser.App and returns
// a parser.App, leaving the runtime free to assume references have
// already been bound.
package optimizer
