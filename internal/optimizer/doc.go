// Package optimizer rewrites the parser AST so the runtime can execute
// it efficiently. Optimize mutates *parser.App in place.
//
// Named-query reference resolution ({queryName.field}, {^field},
// {^^field}, bare {field}) is performed earlier, by parser.Parse, before
// the optimizer runs. This package consumes those interpolations to
// drive its own rewrites.
//
// Current responsibilities:
//
//   - SELECT * rewriting: when consumer interpolations reveal exactly
//     which columns a page/fragment/api uses, rewrite "SELECT *" to an
//     explicit projection (optimizePage in optimizer.go).
//   - JOIN pruning: drop joins whose tables are not referenced by any
//     interpolation or by remaining SQL.
//   - Query deduplication: merge identical queries within a single
//     page/fragment/api body (deduplicateQueries).
//   - Stream candidacy: tag scheduled tasks that can be served as SSE
//     streams without extra work (markStreamCandidates).
//
// The optimizer is purely AST-to-AST and never returns errors: callers
// invoke Optimize(app) after parsing/analysis and use the same *App.
package optimizer
