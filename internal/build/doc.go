// Package build compiles a .kilnx source file into a standalone Go
// binary by emitting Go source that embeds the AST and a minimal
// runtime, then invoking `go build`.
//
// Used by the `kilnx build` CLI command. Build output is a self-contained
// executable: no kilnx CLI is required at runtime, only the resulting
// binary. The build process writes a temporary Go module under a build
// directory, copies in the embedded runtime, and shells out to the host
// Go toolchain.
package build
