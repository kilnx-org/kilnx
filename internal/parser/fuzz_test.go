package parser

import (
	"testing"

	"github.com/kilnx-org/kilnx/internal/lexer"
)

func FuzzParse(f *testing.F) {
	seeds := []string{
		`model user
  name: text required
  email: email unique`,
		`page / requires auth
  query users: SELECT * FROM user
  html
    {{each users}}<p>{name}</p>{{end}}`,
		`action /create method POST
  validate
    name: required
  query: INSERT INTO user (name) VALUES (:name)
  on success
    redirect /`,
		`config
  secret: env APP_SECRET
  port: 8080`,
		`auth
  table: user
  identity: email
  password: password`,
		`webhook /hook secret env HOOK_SECRET
  on event push
    query: INSERT INTO log (event) VALUES (:event.type)`,
		`schedule cleanup every 24h
  query: DELETE FROM session WHERE expires_at < datetime('now')`,
		`job send-email
  query data: SELECT email FROM user WHERE id = :user_id
  send email to :data.email`,
		`limit /api/*
  requests: 100 per minute per user`,
		`test "homepage loads"
  visit /
  expect status 200`,
		"",
		"model\n  name:",
		"page /\n  invalid block\n    foo",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		// Parse must never panic, regardless of input.
		stripped := lexer.StripComments(src)
		tokens := lexer.Tokenize(stripped)
		_, _ = Parse(tokens, stripped)
	})
}
