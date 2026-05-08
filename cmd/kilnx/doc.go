// Command kilnx is the Kilnx CLI: it lexes, parses, analyzes, and
// either runs or compiles a .kilnx application.
//
// Usage:
//
//	kilnx run     <file.kilnx>           start dev server
//	kilnx check   <file.kilnx>           static analysis only
//	kilnx build   <file.kilnx> -o <bin>  compile to standalone binary
//	kilnx migrate <file.kilnx> [flags]   apply pending DB migrations
//	kilnx test    <file.kilnx>           run inline `test` blocks
//	kilnx version                        print version
//
// The CLI is thin: each subcommand wires the relevant internal/*
// packages together. See package internal/runtime for the dev server
// and internal/build for ahead-of-time compilation.
package main
