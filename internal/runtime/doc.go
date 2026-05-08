// Package runtime is the HTTP server and AST interpreter that backs
// `kilnx run` and the binaries produced by `kilnx build`.
//
// Responsibilities:
//
//   - HTTP routing, request parsing, and response rendering for
//     pages, actions, fragments, APIs, websockets, and streams.
//   - Built-in authentication (email/password, sessions, CSRF).
//   - Form handling and validation.
//   - Background jobs, scheduled tasks, and webhook delivery.
//   - Email sending, file uploads, and SSR template rendering.
//   - Live reload during development.
//
// The runtime executes the parser AST directly: there is no
// intermediate code generation step at run time. The optimizer
// rewrites the AST before the runtime sees it. For ahead-of-time
// builds, package internal/build embeds the runtime alongside the
// AST so the resulting binary has no compile-time dependency on the
// kilnx CLI.
package runtime
