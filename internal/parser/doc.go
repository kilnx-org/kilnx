// Package parser builds the Kilnx AST from a token stream produced by
// internal/lexer. Each top-level keyword (page, action, model, ...) has
// a parse function in this package and a co-located *_spec.go file that
// registers its language-spec entry with internal/spec for documentation
// and tooling.
//
// To regenerate the reference documentation under docs/devs/reference
// after editing any *_spec.go file, run:
//
//	go generate ./...
package parser

//go:generate go run ../../cmd/kilnx-gendocs -o ../../docs/devs/reference
