# Tutorial 1: Build a Todo app

In this tutorial you will build a small Todo CRUD app from scratch. By the end you will have:

- a `Todo` model with two fields,
- three pages: list, new, and edit,
- three actions: create, toggle, and delete,
- an inline toggle that updates a single row over htmx without a full page reload.

The whole app fits in one `.kilnx` file. Useful references: [`model`](../reference/keywords/model.md), [`page`](../reference/keywords/page.md), [`action`](../reference/keywords/action.md), [`fragment`](../reference/keywords/fragment.md), [`query`](../reference/keywords/query.md).

## 1. Set up

Create an empty file:

```bash
mkdir todo-app && cd todo-app
touch app.kilnx
```

You will edit `app.kilnx` step by step and re-run `kilnx run app.kilnx` after each step. The runtime auto-migrates the SQLite database, so the `todo` table appears the first time you start the server.

## 2. Declare the `Todo` model

Open `app.kilnx` and add:

```kilnx
model todo
  id: uuid auto
  title: text required min 1 max 200
  done: bool default false
  created_at: timestamp auto
```

What is happening:

- `id: uuid auto` gives every row a generated UUID primary key. See [`auto`](../reference/attributes/auto.md).
- `title: text required min 1 max 200` enforces a non-empty title at most 200 chars. See [`required`](../reference/attributes/required.md), [`min`](../reference/attributes/min.md), [`max`](../reference/attributes/max.md).
- `done: bool default false` defaults new todos to unfinished. See [`default`](../reference/attributes/default.md).
- `created_at: timestamp auto` stamps the row at insert time.

Run the server once to apply the migration:

```bash
kilnx run app.kilnx
```

You should see a log line about the `todo` table being created. Stop the server with Ctrl+C.

## 3. List page

Add the index page that shows all todos:

```kilnx
page /todos title "Todos"
  query todos: SELECT id, title, done FROM todo ORDER BY created_at DESC
  html
    <h1>Todos</h1>
    <p><a href="/todos/new">New todo</a></p>
    <ul id="todo-list">
      {#each todos}
        {> todo_row(todo=this)}
      {/each}
    </ul>
```

The page runs an inline [`query`](../reference/keywords/query.md) named `todos` and binds the rows to a template variable. Inside the `{#each}` loop, each row is rendered through a reusable component named `todo_row`. We define that component next.

## 4. The `todo_row` component fragment

Component fragments are declared with parentheses. They render a slice of HTML and are invoked with `{> name(arg=value)}`:

```kilnx
fragment todo_row(todo)
  html
    <li id="todo-{todo.id}" class="todo">
      <input type="checkbox"
             hx-post="/todos/{todo.id}/toggle"
             hx-target="#todo-{todo.id}"
             hx-swap="outerHTML"
             {#if todo.done}checked{/if}>
      <span class="{#if todo.done}done{/if}">{todo.title}</span>
      <a href="/todos/{todo.id}/edit">edit</a>
      <form method="post" action="/todos/{todo.id}/delete" style="display:inline">
        <button type="submit">delete</button>
      </form>
    </li>
```

The checkbox uses three htmx attributes:

- `hx-post` posts to the toggle endpoint when clicked,
- `hx-target` says which DOM node receives the response,
- `hx-swap="outerHTML"` replaces the whole `<li>` with what the server returns.

We will write that toggle response in step 7.

## 5. New todo: page plus action

A page renders the form, an action processes the submission. Add both:

```kilnx
page /todos/new title "New todo"
  html
    <h1>New todo</h1>
    <form method="post" action="/todos/create">
      <label>Title <input name="title" required></label>
      <button type="submit">Create</button>
      <a href="/todos">Cancel</a>
    </form>

action /todos/create method POST
  validate todo
  query: INSERT INTO todo (title) VALUES (:title)
  redirect /todos
```

`validate todo` runs the model-level rules from step 2: it rejects empty titles and titles longer than 200 chars. See [`validate`](../reference/attributes/validate.md). On success the action inserts the row and sends the user back to the list.

Restart the server, visit `http://localhost:8080/todos/new`, submit a title, and you should land on `/todos` with one row visible.

## 6. Edit a todo

The edit flow mirrors the new flow, except the form is preloaded and the SQL is an `UPDATE`:

```kilnx
page /todos/:id/edit title "Edit todo"
  query todo: SELECT id, title, done FROM todo WHERE id = :id
  html
    <h1>Edit todo</h1>
    <form method="post" action="/todos/{todo.id}/update">
      <label>Title <input name="title" value="{todo.title}" required></label>
      <label><input type="checkbox" name="done" {#if todo.done}checked{/if}> Done</label>
      <button type="submit">Save</button>
      <a href="/todos">Cancel</a>
    </form>

action /todos/:id/update method POST
  validate todo
  query: UPDATE todo SET title = :title, done = :done WHERE id = :id
  redirect /todos
```

The `:id` placeholder in the path becomes a parameter that the SQL can reference as `:id`. Same for `:title` and `:done`, which come from the form body.

## 7. Inline toggle: action plus fragment

This is the part that makes the list feel snappy. The checkbox in `todo_row` posts to `/todos/:id/toggle`. The action flips the `done` flag and asks the server to render only the updated row, which htmx then swaps in place.

```kilnx
action /todos/:id/toggle method POST
  query: UPDATE todo SET done = NOT done WHERE id = :id
  query todo: SELECT id, title, done FROM todo WHERE id = :id
  respond
    html
      {> todo_row(todo=todo)}
```

What you should see when you click a checkbox at `http://localhost:8080/todos`:

1. The browser issues a `POST /todos/<id>/toggle`.
2. The action updates the row, reloads it, and renders the `todo_row` component.
3. htmx replaces the matching `<li id="todo-<id>">` with the new HTML. No full page reload.

If you prefer the explicit form, [`respond`](../reference/attributes/respond.md) also accepts a CSS selector via `respond fragment ".selector"`. The version above keeps the fragment lookup local to the action body, which is easier to reason about when you are starting out.

## 8. Delete

Same idea as toggle, except we redirect back to the list rather than swap a fragment:

```kilnx
action /todos/:id/delete method POST
  query: DELETE FROM todo WHERE id = :id
  redirect /todos
```

## 9. Try it end to end

1. `kilnx run app.kilnx`.
2. Open `http://localhost:8080/todos`. Empty list.
3. Click "New todo", create three todos.
4. Click a checkbox. Only that row repaints. The text gains a `done` class.
5. Click "edit" on any row, change the title, save. Back to the list with the new title.
6. Click "delete". Row disappears.

If something goes wrong, run `kilnx check app.kilnx` to surface parse and type errors before the server even starts.

## 10. Full source

Save this as `app.kilnx`:

```kilnx
model todo
  id: uuid auto
  title: text required min 1 max 200
  done: bool default false
  created_at: timestamp auto

page /todos title "Todos"
  query todos: SELECT id, title, done FROM todo ORDER BY created_at DESC
  html
    <h1>Todos</h1>
    <p><a href="/todos/new">New todo</a></p>
    <ul id="todo-list">
      {#each todos}
        {> todo_row(todo=this)}
      {/each}
    </ul>

fragment todo_row(todo)
  html
    <li id="todo-{todo.id}" class="todo">
      <input type="checkbox"
             hx-post="/todos/{todo.id}/toggle"
             hx-target="#todo-{todo.id}"
             hx-swap="outerHTML"
             {#if todo.done}checked{/if}>
      <span class="{#if todo.done}done{/if}">{todo.title}</span>
      <a href="/todos/{todo.id}/edit">edit</a>
      <form method="post" action="/todos/{todo.id}/delete" style="display:inline">
        <button type="submit">delete</button>
      </form>
    </li>

page /todos/new title "New todo"
  html
    <h1>New todo</h1>
    <form method="post" action="/todos/create">
      <label>Title <input name="title" required></label>
      <button type="submit">Create</button>
      <a href="/todos">Cancel</a>
    </form>

action /todos/create method POST
  validate todo
  query: INSERT INTO todo (title) VALUES (:title)
  redirect /todos

page /todos/:id/edit title "Edit todo"
  query todo: SELECT id, title, done FROM todo WHERE id = :id
  html
    <h1>Edit todo</h1>
    <form method="post" action="/todos/{todo.id}/update">
      <label>Title <input name="title" value="{todo.title}" required></label>
      <label><input type="checkbox" name="done" {#if todo.done}checked{/if}> Done</label>
      <button type="submit">Save</button>
      <a href="/todos">Cancel</a>
    </form>

action /todos/:id/update method POST
  validate todo
  query: UPDATE todo SET title = :title, done = :done WHERE id = :id
  redirect /todos

action /todos/:id/toggle method POST
  query: UPDATE todo SET done = NOT done WHERE id = :id
  query todo: SELECT id, title, done FROM todo WHERE id = :id
  respond
    html
      {> todo_row(todo=todo)}

action /todos/:id/delete method POST
  query: DELETE FROM todo WHERE id = :id
  redirect /todos
```

## Where to go next

- Add login so each user only sees their own todos: [Tutorial 2](02-auth-and-sessions.md).
- Send a reminder email when a todo is overdue: [Tutorial 3](03-background-jobs.md).
