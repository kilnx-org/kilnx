// Package build produces standalone binaries by appending an app bundle to
// a copy of the running kilnx executable, and reads that bundle back at
// startup.
//
// Used by the `kilnx build` CLI command. The kilnx CLI already contains the
// whole runtime, so a build needs neither the Go toolchain nor the kilnx
// source tree. Output runs on the same OS and architecture as the kilnx
// that produced it, with no kilnx CLI required at runtime.
package build
