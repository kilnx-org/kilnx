# Pages, actions, fragments

Three keywords cover almost the entire HTTP layer. [`page`](../reference/keywords/page.md) renders a full HTML document. [`action`](../reference/keywords/action.md) handles state-changing requests and redirects. [`fragment`](../reference/keywords/fragment.md) returns a slice of HTML, either as an htmx target or as a reusable include. JSON endpoints use [`api`](../reference/keywords/api.md), covered briefly at the end.

## Page

A `page` is a GET endpoint that returns a full HTML document. The body holds queries and the response template.

```kilnx
page /
  html
    <h1>Hello World</h1>
```

With data:

```kilnx
page /tasks
  query tasks: SELECT id, title FROM task ORDER BY created DESC
  html
    <h1>Tasks</h1>
    <ul>
      {{each tasks}}
      <li>{title}</li>
      {{end}}
    </ul>
```

Pages can carry attributes inline:

```kilnx
page /dashboard layout app title "Dashboard" requires auth
  query @user_count: SELECT count(*) FROM users
  html
    <h1>{{ .user_count }} users</h1>
```

`layout`, `title`, and `requires` are common. See the [page reference](../reference/keywords/page.md) for the rest.

## Action

An `action` is a non-GET endpoint that mutates state. By default it does not return a full document; it ends with a [`redirect`](../reference/attributes/redirect.md) (typical browser flow) or with [`respond`](../reference/attributes/respond.md) (return a fragment to htmx).

```kilnx
action /users/create method POST requires auth
  validate user
  query: INSERT INTO user (email, name) VALUES (:email, :name)
  redirect /users
```

`validate user` runs the model's constraints against the form payload. If validation fails, the request returns to the originating page with errors bound for the template.

For htmx flows, swap the redirect for a fragment response:

```kilnx
action /tasks/:id/delete requires auth
  query: DELETE FROM task WHERE id = :id AND owner = :current_user.id
  respond fragment delete
```

This returns the rendered `delete` fragment, which htmx swaps into the DOM.

## Fragment

A `fragment` returns partial HTML without a document wrapper. There are two flavors.

Route-based fragments respond to AJAX requests:

```kilnx
fragment /users/:id/card
  query user: SELECT name, email FROM user WHERE id = :id
  html
    <div class="card">{user.name}</div>
```

Use them as `hx-get` or `hx-post` targets.

Component-based fragments are reusable templates invoked by name from inside other pages or fragments:

```kilnx
fragment badge(status, color="blue")
  html
    <span class="{color}">{status}</span>
```

You include them in another template with the fragment name and arguments.

## Putting it together

A small CRUD slice usually pairs all three:

```kilnx
model task
  title: text required
  done: bool default false
  created: timestamp auto

page /tasks
  query tasks: SELECT id, title, done FROM task ORDER BY created DESC
  html
    <h1>Tasks</h1>
    <ul id="tasks">
      {{each tasks}}
      <li>{title}
        <button hx-post="/tasks/{id}/delete"
                hx-target="closest li" hx-swap="outerHTML">x</button>
      </li>
      {{end}}
    </ul>
    <form hx-post="/tasks/new" hx-target="#tasks" hx-swap="beforeend">
      <input name="title" required>
      <button>Add</button>
    </form>

action /tasks/new method POST
  validate task
  query new: INSERT INTO task (title) VALUES (:title) RETURNING id, title
  respond fragment task-row

action /tasks/:id/delete method POST
  query: DELETE FROM task WHERE id = :id
  respond status 200

fragment task-row(task)
  html
    <li>{task.title}</li>
```

The page renders the initial list. The form submits to an action that inserts and returns a row fragment, which htmx appends. The delete button posts to an action that returns 200, and htmx removes the `<li>` based on `hx-swap`.

## JSON: `api`

When you need JSON, declare an [`api`](../reference/keywords/api.md). It looks like a `page` but returns serialized data instead of HTML.

```kilnx
api /api/tasks
  query tasks: SELECT id, title, done FROM task
  respond tasks
```

Use `api` for SPAs, mobile clients, or third-party integrations. The default response shape is JSON.

## Read next

- [Auth and permissions](auth-and-permissions.md): protect pages, actions, and fragments with `requires`.
- [Models and data](models-and-data.md): the queries inside these blocks bind to your models.
