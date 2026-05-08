package parser

import "github.com/kilnx-org/kilnx/internal/spec"

func init() {
	spec.Register(spec.Entity{
		Name:    "translations",
		Kind:    spec.KindKeyword,
		Summary: "i18n translation strings keyed by language and key.",
		Description: "The `translations` block contains nested language sub-blocks; each " +
			"language block is a list of `key: \"value\"` entries. Templates reference " +
			"keys via `{t 'key'}`. Multiple `translations` blocks merge by language. " +
			"The active language is selected per-request based on the `detect language` " +
			"strategy in `config`.",
		Syntax:     "translations",
		Repeatable: true,
		Since:      "0.1.0",
		Examples: []spec.Example{
			{
				Title: "English and Portuguese",
				Code: `translations
  en
    welcome: "Welcome back"
    users: "Users"
  pt
    welcome: "Bem-vindo de volta"
    users: "Usuarios"`,
			},
		},
		SeeAlso: []string{"config"},
	})
}
