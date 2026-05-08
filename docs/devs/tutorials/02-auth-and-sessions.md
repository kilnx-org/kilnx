# Tutorial 2: Auth and sessions

This tutorial extends the [Todo app from Tutorial 1](01-todo-app.md) with login, registration, logout, and role-based access control. By the end every todo will belong to a user and an `admin` role will see an extra moderation page that regular users cannot reach.

References used here: [`auth`](../reference/keywords/auth.md), [`permissions`](../reference/keywords/permissions.md), [`requires`](../reference/attributes/requires.md), and the auth attributes [`table`](../reference/attributes/table.md), [`identity`](../reference/attributes/identity.md), [`password`](../reference/attributes/password.md), [`login`](../reference/attributes/login.md), [`logout`](../reference/attributes/logout.md), [`register`](../reference/attributes/register.md), [`after_login`](../reference/attributes/after_login.md), [`superuser`](../reference/attributes/superuser.md).

## 1. Add a `user` model

Open `app.kilnx` from Tutorial 1 and add a user model above the `todo` model. Notice the `password` field type, which Kilnx automatically bcrypt-hashes on write and verifies on login:

```kilnx
model user
  id: uuid auto
  email: email required unique
  password: password required
  role: option [admin, member] default "member"
  created_at: timestamp auto
```

The `option [admin, member]` field is what we will gate routes on later. See the [`model`](../reference/keywords/model.md) reference for the full list of field types.

## 2. Wire up `auth`

Below the models, add an `auth` block. Each attribute is written `name: value` (note `after login:` is two words):

```kilnx
auth
  table: user
  identity: email
  password: password
  login: /login
  logout: /logout
  register: /register
  after login: /todos
  superuser: env SUPERUSER_EMAIL
```

What this gives you for free:

- `GET /login` and `GET /register` render forms.
- `POST /login` issues a session cookie (HTTP-only, signed with `SECRET_KEY`).
- `POST /logout` ends the session.
- `POST /register` creates a `user` row with the password already bcrypt-hashed.
- After a successful login the user lands on `/todos`.
- The address in `SUPERUSER_EMAIL` (read at runtime) bypasses every `requires` check, so you can always recover access.

Run the server:

```bash
SUPERUSER_EMAIL=you@example.com kilnx run app.kilnx
```

Open `http://localhost:8080/register`, create an account, and you should be redirected to `/todos` after logging in. The session cookie is now set.

## 3. Scope todos to the current user

Right now any user sees every todo. Add a `user_id` reference to the `Todo` model so each row is owned by someone:

```kilnx
model todo
  id: uuid auto
  user_id: reference user required
  title: text required min 1 max 200
  done: bool default false
  due_at: timestamp
  created_at: timestamp auto
```

The `due_at` field is unused for now. Tutorial 3 will use it to send overdue reminders. Restart the server and the migration adds the new columns.

## 4. Gate every todo route

Add `requires auth` to every endpoint that touches todos so anonymous users get redirected to `/login`. We also filter every query by `:current_user_id`, the session-bound parameter Kilnx exposes inside route bodies:

```kilnx
page /todos title "Todos" requires auth
  query todos: SELECT id, title, done FROM todo WHERE user_id = :current_user_id ORDER BY created_at DESC
  html
    <h1>Todos</h1>
    <p>
      <a href="/todos/new">New todo</a>
      <form method="post" action="/logout" style="display:inline">
        <button type="submit">Log out</button>
      </form>
    </p>
    <ul id="todo-list">
      {#each todos}
        {> todo_row(todo=this)}
      {/each}
    </ul>

action /todos/create method POST requires auth
  validate todo
  query: INSERT INTO todo (title, user_id) VALUES (:title, :current_user_id)
  redirect /todos
```

Apply the same pattern to `update`, `toggle`, and `delete`. The `WHERE` clause should always include `user_id = :current_user_id` so users cannot mutate other people's rows by guessing IDs:

```kilnx
action /todos/:id/update method POST requires auth
  validate todo
  query: UPDATE todo SET title = :title, done = :done
         WHERE id = :id AND user_id = :current_user_id
  redirect /todos

action /todos/:id/toggle method POST requires auth
  query: UPDATE todo SET done = NOT done
         WHERE id = :id AND user_id = :current_user_id
  query todo: SELECT id, title, done FROM todo
              WHERE id = :id AND user_id = :current_user_id
  respond
    html
      {> todo_row(todo=todo)}

action /todos/:id/delete method POST requires auth
  query: DELETE FROM todo WHERE id = :id AND user_id = :current_user_id
  redirect /todos
```

The same `requires auth` line goes on `/todos/new` and `/todos/:id/edit`. See [`requires`](../reference/attributes/requires.md) for the full grammar.

Try it: log out, hit `http://localhost:8080/todos`, and you should be bounced to the login form. Log in as a different user and confirm you only see your own todos.

## 5. Roles with `permissions`

Now we add role-based gating. Add a `permissions` block:

```kilnx
permissions
  admin: all
  member: read todo where user_id = current_user, write todo where user_id = current_user
```

What each rule means:

- `admin: all`. The `admin` role can read and write every model.
- `member: read todo where user_id = current_user, write todo where user_id = current_user`. Members can only see and modify their own todos. The `where` clause is enforced even when a route does not include the filter manually.

See [`permissions`](../reference/keywords/permissions.md) for the rule grammar.

## 6. Gate a page by role

Add an admin-only page that lists every todo across users. The route opts in via `requires admin`:

```kilnx
page /admin/todos title "All todos" requires admin
  query rows: SELECT t.id, t.title, t.done, u.email
              FROM todo t JOIN user u ON u.id = t.user_id
              ORDER BY t.created_at DESC
  html
    <h1>All todos</h1>
    <table>
      <thead><tr><th>User</th><th>Title</th><th>Done</th></tr></thead>
      <tbody>
        {#each rows}
          <tr>
            <td>{this.email}</td>
            <td>{this.title}</td>
            <td>{#if this.done}yes{#else}no{/if}</td>
          </tr>
        {/each}
      </tbody>
    </table>
```

Since `requires admin` is checked before the body runs, a logged-in `member` who tries `http://localhost:8080/admin/todos` gets a 403. The user identified by `SUPERUSER_EMAIL` bypasses the check thanks to [`superuser`](../reference/attributes/superuser.md).

To create your first admin, either:

1. Log in as `SUPERUSER_EMAIL`, or
2. Promote an existing user with one SQL statement:

```bash
sqlite3 adequa.db "UPDATE user SET role = 'admin' WHERE email = 'you@example.com';"
```

## 7. Try the gates end to end

1. Register two accounts: `alice@example.com` and `bob@example.com`.
2. Log in as Alice, create two todos.
3. Log out, log in as Bob, hit `/todos`. You should see an empty list.
4. Try `http://localhost:8080/admin/todos` as Bob. 403.
5. Promote Bob to `admin` with the SQL above and revisit the page. Both users' todos appear.

## 8. Final source

The full `app.kilnx` after Tutorials 1 and 2:

```kilnx
model user
  id: uuid auto
  email: email required unique
  password: password required
  role: option [admin, member] default "member"
  created_at: timestamp auto

model todo
  id: uuid auto
  user_id: reference user required
  title: text required min 1 max 200
  done: bool default false
  due_at: timestamp
  created_at: timestamp auto

auth
  table: user
  identity: email
  password: password
  login: /login
  logout: /logout
  register: /register
  after login: /todos
  superuser: env SUPERUSER_EMAIL

permissions
  admin: all
  member: read todo where user_id = current_user, write todo where user_id = current_user

page /todos title "Todos" requires auth
  query todos: SELECT id, title, done FROM todo
               WHERE user_id = :current_user_id
               ORDER BY created_at DESC
  html
    <h1>Todos</h1>
    <p>
      <a href="/todos/new">New todo</a>
      <form method="post" action="/logout" style="display:inline">
        <button type="submit">Log out</button>
      </form>
    </p>
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

page /todos/new title "New todo" requires auth
  html
    <h1>New todo</h1>
    <form method="post" action="/todos/create">
      <label>Title <input name="title" required></label>
      <button type="submit">Create</button>
      <a href="/todos">Cancel</a>
    </form>

action /todos/create method POST requires auth
  validate todo
  query: INSERT INTO todo (title, user_id) VALUES (:title, :current_user_id)
  redirect /todos

page /todos/:id/edit title "Edit todo" requires auth
  query todo: SELECT id, title, done FROM todo
              WHERE id = :id AND user_id = :current_user_id
  html
    <h1>Edit todo</h1>
    <form method="post" action="/todos/{todo.id}/update">
      <label>Title <input name="title" value="{todo.title}" required></label>
      <label><input type="checkbox" name="done" {#if todo.done}checked{/if}> Done</label>
      <button type="submit">Save</button>
      <a href="/todos">Cancel</a>
    </form>

action /todos/:id/update method POST requires auth
  validate todo
  query: UPDATE todo SET title = :title, done = :done
         WHERE id = :id AND user_id = :current_user_id
  redirect /todos

action /todos/:id/toggle method POST requires auth
  query: UPDATE todo SET done = NOT done
         WHERE id = :id AND user_id = :current_user_id
  query todo: SELECT id, title, done FROM todo
              WHERE id = :id AND user_id = :current_user_id
  respond
    html
      {> todo_row(todo=todo)}

action /todos/:id/delete method POST requires auth
  query: DELETE FROM todo WHERE id = :id AND user_id = :current_user_id
  redirect /todos

page /admin/todos title "All todos" requires admin
  query rows: SELECT t.id, t.title, t.done, u.email
              FROM todo t JOIN user u ON u.id = t.user_id
              ORDER BY t.created_at DESC
  html
    <h1>All todos</h1>
    <table>
      <thead><tr><th>User</th><th>Title</th><th>Done</th></tr></thead>
      <tbody>
        {#each rows}
          <tr>
            <td>{this.email}</td>
            <td>{this.title}</td>
            <td>{#if this.done}yes{#else}no{/if}</td>
          </tr>
        {/each}
      </tbody>
    </table>
```

## Where to go next

Add a background process that emails users when one of their todos is overdue: [Tutorial 3](03-background-jobs.md).
