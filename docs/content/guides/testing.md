# Testing

> This page is a stub. Full content coming soon.

Declarative HTTP tests in the same language — no Selenium, no Cypress.

```kilnx
test "user can register"
  visit /register
  fill name "Alice"
  fill identity "alice@test.com"
  fill password "secret123"
  submit
  expect page /login contains "Log in"

test "homepage loads"
  visit /
  expect status 200
  expect page / contains "Blog"

test "admin access"
  as admin
  visit /admin
  expect status 200
```

Run with:

```bash
kilnx test app.kilnx
```

Tests run against a real in-memory server with a fresh SQLite database. No mocks.

## Step reference

| Step | Effect |
|------|--------|
| `visit <path>` | GET the path |
| `fill <field> "<value>"` | Set a form field |
| `submit` | Submit the last form |
| `expect status <N>` | Assert response status code |
| `expect page <path> contains "<text>"` | Assert page body contains text |
| `expect redirect to <path>` | Assert Location header |
| `expect query: <SQL> returns <N>` | Run SQL, assert returned count |
| `as <role>` | Log in as a user with the given role |
