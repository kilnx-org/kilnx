package parser

import "github.com/kilnx-org/kilnx/internal/spec"

// Spec entries for body nodes: keywords that appear inside the indented
// body of a page, action, fragment, api, schedule, job, webhook, layout,
// or socket handler. Each node represents a runtime step.

func init() {
	bodyParents := []string{"page", "action", "fragment", "api", "schedule", "job", "webhook"}

	spec.Register(spec.Entity{
		Name: "html", Kind: spec.KindAttribute,
		Summary: "Render an inline HTML template block.",
		Description: "The `html` body emits the indented template literal. Bindings use " +
			"`{name}` or `{model.field}` syntax; control flow uses `{#if ...}`/`{/if}` and " +
			"`{#each ...}`/`{/each}`. Inside a `layout`, `html` defines the wrapper template.",
		Syntax:      "html",
		ParentScope: append(append([]string{}, bodyParents...), "layout"),
		Since:       "0.1.0",
		Examples: []spec.Example{{Title: "Inline template", Code: `page /
  html
    <h1>Welcome, {current_user.name}!</h1>`}},
	})

	spec.Register(spec.Entity{
		Name: "validate", Kind: spec.KindAttribute,
		Summary: "Validate input against a model or per-field rules.",
		Description: "Bare form `validate <model>` runs all model-level validators. Block " +
			"form lists per-field rules: `<field>: <rule>, <rule>`. Validation errors " +
			"populate template-accessible `errors` and short-circuit subsequent body nodes.",
		Syntax:      "validate [<model>]",
		ParentScope: bodyParents,
		Since:       "0.1.0",
		Examples: []spec.Example{
			{Title: "Validate against a model", Code: `action /users/create method POST
  validate user`},
			{Title: "Custom rules block", Code: `action /signup method POST
  validate
    name: required
    email: required, is email
    age: min 18, max 120`},
		},
	})

	spec.Register(spec.Entity{
		Name: "send", Kind: spec.KindAttribute,
		Summary: "Send an email.",
		Description: "Address with `to <expression>` (column, parameter, or string). Body " +
			"sub-fields: `subject:`, `template:` (named layout), `body:` (inline text), " +
			"`attach:` (file). Email transport is configured at runtime via env vars.",
		Syntax:      "send email to <target>",
		ParentScope: bodyParents,
		Since:       "0.1.0",
		Examples: []spec.Example{{Title: "Welcome email", Code: `action /signup method POST
  send email to :email
    subject: "Welcome to our app"
    template: welcome-email`}},
	})

	spec.Register(spec.Entity{
		Name: "enqueue", Kind: spec.KindAttribute,
		Summary:     "Enqueue an asynchronous `job` with named parameters.",
		Description: "Indented children are `param: value` pairs. Parameters can be literals, `:bindings`, or query results.",
		Syntax:      "enqueue <job-name>",
		ParentScope: bodyParents,
		Since:       "0.1.0",
		Examples: []spec.Example{{Title: "Enqueue a report job", Code: `action /reports method POST
  enqueue generate-report
    start_date: :start_date
    requested_by: :current_user_email`}},
	})

	spec.Register(spec.Entity{
		Name: "on", Kind: spec.KindAttribute,
		Summary: "Conditional or event handler.",
		Description: "Inside a body, branches on the result of the previous step: " +
			"`on success`, `on error`, `on not found`, `on forbidden`. Inside a `socket` " +
			"or `webhook`, dispatches on connection events (`on connect`, `on message`, " +
			"`on disconnect`) or named webhook events (`on event <name>`).",
		Syntax:      "on <condition-or-event>",
		ParentScope: append(append([]string{}, bodyParents...), "socket"),
		Repeatable:  true,
		Since:       "0.1.0",
		Examples: []spec.Example{{Title: "Branch on action outcome", Code: `action /users/create method POST
  validate user
  query: INSERT INTO user (...) VALUES (...)
  on success
    redirect /users
  on error
    respond fragment ".form" query: SELECT * FROM user WHERE id = :id`}},
	})

	spec.Register(spec.Entity{
		Name: "broadcast", Kind: spec.KindAttribute,
		Summary:     "Push a message to all clients in a websocket room.",
		Description: "Renders the named fragment server-side and sends the resulting HTML to all socket clients subscribed to the room.",
		Syntax:      "broadcast to :<room> [fragment <name>]",
		ParentScope: append(append([]string{}, bodyParents...), "socket"),
		Since:       "0.1.0",
		Examples: []spec.Example{{Title: "Broadcast a chat fragment", Code: `socket /chat/:room
  on message
    query: INSERT INTO message ...
    broadcast to :room fragment chat-message`}},
	})

	spec.Register(spec.Entity{
		Name: "fetch", Kind: spec.KindAttribute,
		Summary:     "Make an HTTP request to an external API.",
		Description: "Sub-keywords: `header <name>: <value>`, `body <key>: <value>`. The named result is bound for use by subsequent body nodes.",
		Syntax:      "fetch [<name>:] <METHOD> <URL>",
		ParentScope: bodyParents,
		Since:       "0.1.0",
		Examples: []spec.Example{{Title: "Call a weather API", Code: `page /weather
  fetch weather: GET https://api.weather.com/forecast?city=:city
    header Authorization: env API_KEY
  html
    <p>{weather.summary}</p>`}},
	})

	// `llm` is registered in llm_spec.go as a KindKeyword body block with
	// discriminated `response` / `agent` children (v0.1.3 redesign).

	spec.Register(spec.Entity{
		Name: "respond", Kind: spec.KindAttribute,
		Summary:     "Return a partial HTML response (htmx fragment) or status code.",
		Description: "Without arguments, renders the body's HTML normally. With `fragment <selector>` returns an htmx swap. With `status <code>` returns an explicit HTTP status.",
		Syntax:      "respond [fragment <selector> | status <code>]",
		ParentScope: bodyParents,
		Since:       "0.1.0",
		Examples: []spec.Example{{Title: "Replace a row via htmx", Code: `action /users/:id/toggle method POST
  query: UPDATE user SET active = NOT active WHERE id = :id
  respond fragment ".user-row" query: SELECT * FROM user WHERE id = :id`}},
	})

	spec.Register(spec.Entity{
		Name: "generate", Kind: spec.KindAttribute,
		Summary:     "Generate a PDF from a template and a data query.",
		Description: "Renders an HTML template to PDF using the internal pdf engine. The `data` argument names a previously-bound query result.",
		Syntax:      "generate pdf from template <name> data <query-name>",
		ParentScope: bodyParents,
		Since:       "0.1.0",
		Examples: []spec.Example{{Title: "Render an invoice", Code: `action /invoices/:id/pdf
  query invoice_data: SELECT * FROM invoice WHERE id = :id
  generate pdf from template invoice data invoice_data`}},
	})
}
