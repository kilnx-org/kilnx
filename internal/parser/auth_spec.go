package parser

import "github.com/kilnx-org/kilnx/internal/spec"

func init() {
	spec.Register(spec.Entity{
		Name:    "auth",
		Kind:    spec.KindKeyword,
		Summary: "Configure email/password authentication.",
		Description: "The `auth` block enables built-in authentication: login, logout, " +
			"registration, password reset, and session management. Kilnx auto-wires CSRF " +
			"protection, password hashing (bcrypt), and HTTP-only session cookies. To " +
			"protect routes, use `requires auth` (any logged-in user) or " +
			"`requires <role>` on pages, actions, fragments, or APIs.",
		Syntax: "auth",
		Children: []string{
			"table", "identity", "password",
			"login", "logout", "register", "forgot", "reset",
			"after_login", "superuser",
		},
		Since: "0.1.0",
		Examples: []spec.Example{
			{
				Title: "Standard email/password setup",
				Code: `auth
  table: user
  identity: email
  password: password
  login: /login
  logout: /logout
  register: /register
  after login: /dashboard
  superuser: env SUPERUSER_EMAIL`,
			},
		},
		SeeAlso: []string{"permissions", "requires"},
	})

	spec.Register(spec.Entity{
		Name: "table", Kind: spec.KindAttribute,
		Summary: "Name of the model storing user accounts.", Syntax: "table: <name>",
		Args:        []spec.Arg{{Name: "name", Type: "identifier", Required: true}},
		ParentScope: []string{"auth"}, Default: "user", Since: "0.1.0",
		Examples: []spec.Example{{Title: "Use a custom user model", Code: `auth
  table: account
  identity: email`}},
	})

	spec.Register(spec.Entity{
		Name: "identity", Kind: spec.KindAttribute,
		Summary:     "Field used as the unique login identifier.",
		Description: "Usually `email` or `username`. Must exist on the auth table and be unique.",
		Syntax:      "identity: <field>",
		Args:        []spec.Arg{{Name: "field", Type: "identifier", Required: true}},
		ParentScope: []string{"auth"}, Required: true, Since: "0.1.0",
	})

	spec.Register(spec.Entity{
		Name: "password", Kind: spec.KindAttribute,
		Summary:     "Field storing the bcrypt password hash.",
		Description: "Field name on the auth table holding hashed passwords. Type must be `password` in the model.",
		Syntax:      "password: <field>",
		Args:        []spec.Arg{{Name: "field", Type: "identifier", Required: true}},
		ParentScope: []string{"auth"}, Required: true, Since: "0.1.0",
	})

	spec.Register(spec.Entity{
		Name: "login", Kind: spec.KindAttribute,
		Summary: "Path served as the login form.", Syntax: "login: <path>",
		Args:        []spec.Arg{{Name: "path", Type: "path", Required: true}},
		ParentScope: []string{"auth"}, Default: "/entrar", Since: "0.1.0",
	})

	spec.Register(spec.Entity{
		Name: "logout", Kind: spec.KindAttribute,
		Summary: "Path that terminates the session.", Syntax: "logout: <path>",
		Args:        []spec.Arg{{Name: "path", Type: "path", Required: true}},
		ParentScope: []string{"auth"}, Since: "0.1.0",
	})

	spec.Register(spec.Entity{
		Name: "register", Kind: spec.KindAttribute,
		Summary: "Path served as the registration form.", Syntax: "register: <path>",
		Args:        []spec.Arg{{Name: "path", Type: "path", Required: true}},
		ParentScope: []string{"auth"}, Since: "0.1.0",
	})

	spec.Register(spec.Entity{
		Name: "forgot", Kind: spec.KindAttribute,
		Summary:     "Path for the forgot-password flow.",
		Description: "Also accepted as `forgot_password` or `forgot-password`.",
		Syntax:      "forgot: <path>",
		Args:        []spec.Arg{{Name: "path", Type: "path", Required: true}},
		ParentScope: []string{"auth"}, Since: "0.1.0",
	})

	spec.Register(spec.Entity{
		Name: "reset", Kind: spec.KindAttribute,
		Summary:     "Path for the password-reset confirmation page.",
		Description: "Also accepted as `reset_password` or `reset-password`.",
		Syntax:      "reset: <path>",
		Args:        []spec.Arg{{Name: "path", Type: "path", Required: true}},
		ParentScope: []string{"auth"}, Since: "0.1.0",
	})

	spec.Register(spec.Entity{
		Name: "after_login", Kind: spec.KindAttribute,
		Summary:     "Default redirect target after successful login.",
		Description: "Written `after login: <path>` (with a space) in source.",
		Syntax:      "after login: <path>",
		Args:        []spec.Arg{{Name: "path", Type: "path", Required: true}},
		ParentScope: []string{"auth"}, Since: "0.1.0",
	})

	spec.Register(spec.Entity{
		Name: "superuser", Kind: spec.KindAttribute,
		Summary:     "Identity that bypasses role checks.",
		Description: "Typically pulled from an env var. The matching user is treated as having every role.",
		Syntax:      "superuser: env <VAR>",
		Args:        []spec.Arg{{Name: "value", Type: "string", Required: true}},
		ParentScope: []string{"auth"}, Since: "0.1.0",
	})
}
