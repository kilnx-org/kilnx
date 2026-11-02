# Tutorial 1: Build a Todo app

In this tutorial you will build a small Todo CRUD app from scratch. By the end you will have:

- a `todo` model with three fields,
- three pages: list, new, and edit,
- four actions: create, update, toggle, and delete,
- a reusable `todo_row` component fragment,
- an inline toggle and delete that update a single row over htmx without a full page reload.

The whole app fits in one `.kilnx` file. Useful references: [`model`](../reference/keywords/model.md), [`page`](../reference/keywords/page.md), [`action`](../reference/keywords/action.md), [`fragment`](../reference/keywords/fragment.md), [`query`](../reference/keywords/query.md).

## 1. Set up

Create an empty file:

```bash
mkdir todo-app && cd todo-app
touch app.kilnx
```

You will edit `app.kilnx` step by step. Start the dev server once with `kilnx run app.kilnx`; it watches the file and reloads on save. The runtime auto-migrates a SQLite database named after the source file (`app.db`), so the `todo` table appears the first time the server starts.

## 2. Declare the `todo` model

Open `app.kilnx` and add:

```kilnx
model todo
  title: text required min 1 max 200
  done: bool default false
  created_at: timestamp auto
```

What is happening:

- Every model gets an integer `id` primary key automatically, so you do not declare one.
- `title: text required min 1 max 200` enforces a non-empty title of at most 200 chars. See [`required`](../reference/attributes/required.md), [`min`](../reference/attributes/min.md), [`max`](../reference/attributes/max.md).
- `done: bool default false` defaults new todos to unfinished. See [`default`](../reference/attributes/default.md).
- `created_at: timestamp auto` stamps the row at insert time. See [`auto`](../reference/attributes/auto.md).

Run the server:

```bash
kilnx run app.kilnx
```

You should see a line like `Auto-migrated 1 change(s) to app.db`, followed by the list of routes. Leave the server running.

## 3. The `todo_row` component fragment

Each todo renders as one `<li>`. Because the list page and the toggle response both need that markup, put it in a component fragment. Component fragments are declared with a name and an argument list, and invoked from HTML with `{{name arg=value}}`:

```kilnx
fragment todo_row(id, label, done)
  html
    <li id="todo-{id}" class="todo">
      <input type="checkbox"
             hx-post="/todos/{id}/toggle"
             hx-vals='{"_csrf":"{csrf}"}'
             hx-target="#todo-{id}"
             hx-swap="outerHTML"
             {{if done}}checked{{end}}>
      <span class="{{if done}}done{{end}}">{label}</span>
      <a href="/todos/{id}/edit">edit</a>
      <button hx-post="/todos/{id}/delete"
              hx-vals='{"_csrf":"{csrf}"}'
              hx-target="#todo-{id}"
              hx-swap="outerHTML">delete</button>
    </li>
```

A few things to notice:

- Inside the fragment, arguments are plain placeholders: `{id}`, `{label}`, and `{{if done}}...{{end}}` for conditionals. A value of `0`, `false`, or empty is falsy.
- The argument is called `label`, not `title`, because `title` is a reserved word in Kilnx and cannot be used as an argument name.
- Every POST to an action must carry a CSRF token in a field named `_csrf`. Regular forms use a hidden input; htmx elements send it with `hx-vals`. `{csrf}` is replaced with a fresh, single-use token each time it is rendered. Because the fragment is rendered once per row, every row gets its own tokens.
- The htmx attributes say: post to the endpoint on click (`hx-post`), put the response into this row (`hx-target`), and replace the whole `<li>` (`hx-swap="outerHTML"`).

## 4. List page

Add the index page that shows all todos:

```kilnx
page /todos title "Todos"
  query todos: SELECT id, title, done FROM todo ORDER BY created_at DESC, id DESC
  html
    <h1>Todos</h1>
    <p><a href="/todos/new">New todo</a></p>
    <ul id="todo-list">
      {{each todos}}
        {{todo_row id=id label=title done=done}}
      {{else}}
        <li>Nothing to do yet.</li>
      {{end}}
    </ul>
```

The page runs an inline [`query`](../reference/keywords/query.md) named `todos`. `{{each todos}}...{{end}}` repeats its body once per row, and inside the loop the row's columns are available by name, so `id=id label=title done=done` passes the current row's values to the component. The optional `{{else}}` branch renders when the query returns no rows.

Open `http://localhost:8080/todos`. You should see "Nothing to do yet." Kilnx wraps the page in a default HTML document that already loads htmx and adds a small navigation bar, so there is nothing else to include.

## 5. New todo: page plus action

A page renders the form, an action processes the submission. Add both:

```kilnx
page /todos/new title "New todo"
  html
    <h1>New todo</h1>
    <form method="POST" action="/todos/create">
      <input type="hidden" name="_csrf" value="{csrf}">
      <label>Title <input name="title" required></label>
      <button type="submit">Create</button>
      <a href="/todos">Cancel</a>
    </form>

action /todos/create
  validate todo
  query: INSERT INTO todo (title) VALUES (:title)
  redirect /todos
```

Actions default to `POST`, so no `method` is needed. `validate todo` runs the model rules from step 2: an empty title or one longer than 200 chars stops the action and returns a `422` response with the error message (for example "Title is required"). See [`validate`](../reference/attributes/validate.md). On success the action inserts the row and redirects back to the list. Form fields become SQL parameters by name, so the `title` input is `:title`.

Visit `http://localhost:8080/todos/new`, submit a title, and you should land on `/todos` with one row visible.

## 6. Edit a todo

The edit flow mirrors the new flow, except the form is preloaded and the SQL is an `UPDATE`:

```kilnx
page /todos/:id/edit title "Edit todo"
  query todo: SELECT id, title FROM todo WHERE id = :id
  html
    <h1>Edit todo</h1>
    <form method="POST" action="/todos/{todo.id}/update">
      <input type="hidden" name="_csrf" value="{csrf}">
      <label>Title <input name="title" value="{todo.title}" required></label>
      <button type="submit">Save</button>
      <a href="/todos">Cancel</a>
    </form>

action /todos/:id/update
  validate todo
  query: UPDATE todo SET title = :title WHERE id = :id
  redirect /todos
```

The `:id` segment in the path becomes a parameter that the SQL can reference as `:id`. Outside a loop, a single-row query is read with `{query.column}`, here `{todo.id}` and `{todo.title}`. The done state is not on this form because the checkbox in the list already toggles it.

## 7. Inline toggle: action plus route fragment

This is the part that makes the list feel snappy. The checkbox in `todo_row` posts to `/todos/:id/toggle`. The action flips the `done` flag; the response must be just the updated `<li>`, which htmx swaps in place.

A route fragment is a GET endpoint that returns a slice of HTML with no page wrapper. Add one that renders a single row through the same component:

```kilnx
fragment /todos/:id/row
  query todo: SELECT id, title, done FROM todo WHERE id = :id
  html
    {{todo_row id=todo.id label=todo.title done=todo.done}}

action /todos/:id/toggle
  query: UPDATE todo SET done = NOT done WHERE id = :id
  redirect /todos/:id/row
```

When an action redirects during an htmx request, Kilnx looks for a route fragment matching the redirect path and returns its HTML directly instead of a redirect. Here `redirect /todos/:id/row` therefore answers the htmx request with the freshly rendered row. A plain, non-htmx POST gets an ordinary `303` redirect to the fragment URL instead.

What you should see when you click a checkbox at `http://localhost:8080/todos`:

1. The browser issues a `POST /todos/<id>/toggle` with the row's CSRF token.
2. The action updates the row and renders `/todos/<id>/row`.
3. htmx replaces the matching `<li id="todo-<id>">` with the new HTML, which also carries new CSRF tokens. No full page reload.

## 8. Delete

Delete follows the same htmx pattern, but there is nothing left to render, so the action uses [`respond fragment delete`](../reference/attributes/respond.md):

```kilnx
action /todos/:id/delete
  query: DELETE FROM todo WHERE id = :id
  respond fragment delete
```

`respond fragment delete` returns an empty `200` response with an `HX-Reswap: delete` header, and htmx removes the targeted `<li>` from the page.

## 9. Try it end to end

1. `kilnx run app.kilnx`.
2. Open `http://localhost:8080/todos`. Empty list.
3. Click "New todo", create three todos.
4. Click a checkbox. Only that row repaints, and the text gains the `done` class. Click it again to undo.
5. Click "edit" on any row, change the title, save. Back to the list with the new title.
6. Click "delete". The row disappears without a page reload.

If something goes wrong, run `kilnx check app.kilnx` to surface parse and type errors before the server even starts.

## 10. Final source

Save this as `app.kilnx`:

```kilnx
model todo
  title: text required min 1 max 200
  done: bool default false
  created_at: timestamp auto

fragment todo_row(id, label, done)
  html
    <li id="todo-{id}" class="todo">
      <input type="checkbox"
             hx-post="/todos/{id}/toggle"
             hx-vals='{"_csrf":"{csrf}"}'
             hx-target="#todo-{id}"
             hx-swap="outerHTML"
             {{if done}}checked{{end}}>
      <span class="{{if done}}done{{end}}">{label}</span>
      <a href="/todos/{id}/edit">edit</a>
      <button hx-post="/todos/{id}/delete"
              hx-vals='{"_csrf":"{csrf}"}'
              hx-target="#todo-{id}"
              hx-swap="outerHTML">delete</button>
    </li>

page /todos title "Todos"
  query todos: SELECT id, title, done FROM todo ORDER BY created_at DESC, id DESC
  html
    <h1>Todos</h1>
    <p><a href="/todos/new">New todo</a></p>
    <ul id="todo-list">
      {{each todos}}
        {{todo_row id=id label=title done=done}}
      {{else}}
        <li>Nothing to do yet.</li>
      {{end}}
    </ul>

fragment /todos/:id/row
  query todo: SELECT id, title, done FROM todo WHERE id = :id
  html
    {{todo_row id=todo.id label=todo.title done=todo.done}}

page /todos/new title "New todo"
  html
    <h1>New todo</h1>
    <form method="POST" action="/todos/create">
      <input type="hidden" name="_csrf" value="{csrf}">
      <label>Title <input name="title" required></label>
      <button type="submit">Create</button>
      <a href="/todos">Cancel</a>
    </form>

action /todos/create
  validate todo
  query: INSERT INTO todo (title) VALUES (:title)
  redirect /todos

page /todos/:id/edit title "Edit todo"
  query todo: SELECT id, title FROM todo WHERE id = :id
  html
    <h1>Edit todo</h1>
    <form method="POST" action="/todos/{todo.id}/update">
      <input type="hidden" name="_csrf" value="{csrf}">
      <label>Title <input name="title" value="{todo.title}" required></label>
      <button type="submit">Save</button>
      <a href="/todos">Cancel</a>
    </form>

action /todos/:id/update
  validate todo
  query: UPDATE todo SET title = :title WHERE id = :id
  redirect /todos

action /todos/:id/toggle
  query: UPDATE todo SET done = NOT done WHERE id = :id
  redirect /todos/:id/row

action /todos/:id/delete
  query: DELETE FROM todo WHERE id = :id
  respond fragment delete
```

`kilnx check app.kilnx` reports no errors, only one warning per action:

```text
kilnx warning: [action /todos/create] POST action has no authentication requirement; any visitor can trigger it
```

That is expected: the app has no login yet, so anyone who can reach the server can change the list. Tutorial 2 fixes exactly that.

## Where to go next

- Add login so each user only sees their own todos: [Tutorial 2](02-auth-and-sessions.md).
- Flag overdue todos on a schedule and email reminders from a background job: [Tutorial 3](03-background-jobs.md).
