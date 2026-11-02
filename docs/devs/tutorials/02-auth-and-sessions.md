# Tutorial 2: Auth and sessions

This tutorial extends the [Todo app from Tutorial 1](01-todo-app.md) with registration, login, logout, password reset, and role-based access control. By the end every todo will belong to a user and an `admin` role will see an extra page listing everyone's todos that regular users cannot reach.

References used here: [`auth`](../reference/keywords/auth.md), [`permissions`](../reference/keywords/permissions.md), [`requires`](../reference/attributes/requires.md), and the auth attributes [`table`](../reference/attributes/table.md), [`identity`](../reference/attributes/identity.md), [`password`](../reference/attributes/password.md), [`login`](../reference/attributes/login.md), [`logout`](../reference/attributes/logout.md), [`register`](../reference/attributes/register.md), [`after_login`](../reference/attributes/after_login.md), [`superuser`](../reference/attributes/superuser.md).

## 1. Add a `user` model

Open `app.kilnx` from Tutorial 1 and add a user model above the `todo` model. Notice the `password` field type, which stores a bcrypt hash:

```kilnx
model user
  email: email required unique
  password: password required
  role: option [admin, member] default "member"
  created_at: timestamp auto
```

The `role` field is what we will gate routes on later. New accounts get `member`. See the [`model`](../reference/keywords/model.md) reference for the full list of field types.

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

What the runtime does for you:

- `POST /login` checks the password against the bcrypt hash and issues a signed, HTTP-only session cookie.
- `POST /register` creates a `user` row with the password already hashed, then redirects to `/login`.
- `POST /logout` ends the session and redirects to `/login`.
- `POST /forgot-password` and `POST /reset-password` run the password reset flow.
- After a successful login the user lands on `/todos` (or on the page they were bounced from).
- The account whose email matches `SUPERUSER_EMAIL` (read when the app starts) passes every role check, so you can always recover access.

What the runtime does not do is draw the forms. Kilnx owns the POST side of these routes; you own the HTML. `kilnx check` fails until the app declares a page at each auth path: `/login`, `/register`, `/forgot-password`, and `/reset-password` (the last two use their default paths here; change them with the [`forgot`](../reference/attributes/forgot.md) and [`reset`](../reference/attributes/reset.md) attributes).

## 3. The auth pages

Add the four pages:

```kilnx
page /login title "Log in"
  html
    <h1>Log in</h1>
    <p class="error">{params.error}</p>
    <form method="POST" action="/login?next={params.next}">
      <input type="hidden" name="_csrf" value="{csrf}">
      <label>Email <input name="identity" type="email"></label>
      <label>Password <input name="password" type="password"></label>
      <button type="submit">Log in</button>
    </form>
    <p><a href="/register">Create an account</a> . <a href="/forgot-password">Forgot your password?</a></p>

page /register title "Sign up"
  html
    <h1>Sign up</h1>
    <p class="error">{params.error}</p>
    <form method="POST" action="/register">
      <input type="hidden" name="_csrf" value="{csrf}">
      <label>Email <input name="identity" type="email"></label>
      <label>Password <input name="password" type="password"></label>
      <button type="submit">Create account</button>
    </form>

page /forgot-password title "Forgot password"
  html
    <h1>Forgot password</h1>
    {{if params.sent}}<p>If that address has an account, a reset link is on its way.</p>{{end}}
    <p class="error">{params.error}</p>
    <form method="POST" action="/forgot-password">
      <input type="hidden" name="_csrf" value="{csrf}">
      <label>Email <input name="email" type="email"></label>
      <button type="submit">Send reset link</button>
    </form>

page /reset-password title "Reset password"
  html
    <h1>Choose a new password</h1>
    <p class="error">{params.error}</p>
    <form method="POST" action="/reset-password?token={params.token}">
      <input type="hidden" name="_csrf" value="{csrf}">
      <label>New password <input name="password" type="password"></label>
      <label>Repeat it <input name="password_confirm" type="password"></label>
      <button type="submit">Reset password</button>
    </form>
```

The field names matter, because the runtime reads them:

- Login and register post `identity` and `password`, whatever your identity column is called. Registration requires a password of at least 6 characters.
- A visitor bounced from a protected page arrives at `/login?next=<path>`. The login form posts to `/login?next={params.next}` so the runtime can send them back there; with no `next` they land on the `after login` path.
- Forgot password posts `email`. Reset posts `password` and `password_confirm`, and the reset token travels in the form's `action` query string, copied from the link with `{params.token}`.
- Every form carries the `_csrf` hidden input.

When something fails, the runtime redirects back to the page with `?error=<message>`, and `{params.error}` renders it (HTML-escaped). After a reset request it redirects to `/forgot-password?sent=1`, and after a successful reset to `/login?reset=1`. Without SMTP settings the reset link is printed to the server console instead of emailed, which is handy locally.

Run the server:

```bash
SUPERUSER_EMAIL=you@example.com kilnx run app.kilnx
```

Open `http://localhost:8080/register`, create an account, log in, and you should be redirected to `/todos`. The session cookie is now set. The default page wrapper shows your email and a Logout button in its top bar, so you do not need to build a logout form yourself.

## 4. Scope todos to the current user

Right now any user sees every todo. Add an `owner` field to the `todo` model so each row is owned by someone:

```kilnx
model todo
  title: text required min 1 max 200
  done: bool default false
  owner: user required
  created_at: timestamp auto
```

A field whose type is another model's name is a reference. The column is named after the field with an `_id` suffix, so in SQL this field is `owner_id`. The server reloads and the migration adds the column.

## 5. Gate every todo route

Add `requires auth` to every endpoint that touches todos so anonymous visitors get redirected to `/login`. Then filter every query by `:current_user.id`, the logged-in user's id, which Kilnx binds in every request of an authenticated route:

```kilnx
page /todos title "Todos" requires auth
  query todos: SELECT id, title, done FROM todo
               WHERE owner_id = :current_user.id
               ORDER BY created_at DESC, id DESC
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

action /todos/create requires auth
  validate todo
  query: INSERT INTO todo (title, owner_id) VALUES (:title, :current_user.id)
  redirect /todos
```

Long SQL can continue on the following lines, as long as every continuation line uses the same indentation.

Apply the same pattern to the row fragment, the edit page, and the `update`, `toggle`, and `delete` actions. Every `WHERE` clause includes `owner_id = :current_user.id`, so a user who guesses another user's todo id simply matches no rows:

```kilnx
fragment /todos/:id/row requires auth
  query todo: SELECT id, title, done FROM todo
              WHERE id = :id AND owner_id = :current_user.id
  html
    {{todo_row id=todo.id label=todo.title done=todo.done}}

action /todos/:id/update requires auth
  validate todo
  query: UPDATE todo SET title = :title
         WHERE id = :id AND owner_id = :current_user.id
  redirect /todos

action /todos/:id/toggle requires auth
  query: UPDATE todo SET done = NOT done
         WHERE id = :id AND owner_id = :current_user.id
  redirect /todos/:id/row

action /todos/:id/delete requires auth
  query: DELETE FROM todo WHERE id = :id AND owner_id = :current_user.id
  respond fragment delete
```

The same `requires auth` goes on `/todos/new` and `/todos/:id/edit`, and the edit page's query gets the same `owner_id` filter. See [`requires`](../reference/attributes/requires.md) for the full grammar.

Try it: log out, hit `http://localhost:8080/todos`, and you should be bounced to `/login?next=%2Ftodos`. Log in as a different user and confirm you only see your own todos. Posting to another user's todo id changes nothing: the update redirects as usual and the delete returns its normal empty response, but no row matches.

## 6. Roles with `permissions`

Now add a `permissions` block that names the roles:

```kilnx
permissions
  admin: all
  member: read todo, write todo
```

What each rule means today:

- `admin: all`. The `admin` role satisfies every `requires <role>` check, whatever role the route asks for.
- `member: read todo, write todo`. Documents what members may touch.

The rule grammar also accepts row filters such as `write todo where owner = current_user`, but the current runtime does not apply those filters to your queries. Keep the explicit `owner_id = :current_user.id` conditions from step 5; they are what actually isolates users. See [`permissions`](../reference/keywords/permissions.md) for the rule grammar.

## 7. Gate a page by role

Add an admin-only page that lists every todo across users. The route opts in with `requires admin`:

```kilnx
page /admin/todos title "All todos" requires admin
  query rows: SELECT t.id, t.title, t.done, u.email
              FROM todo t JOIN user u ON u.id = t.owner_id
              ORDER BY t.created_at DESC
  html
    <h1>All todos</h1>
    <table>
      <thead><tr><th>User</th><th>Title</th><th>Done</th></tr></thead>
      <tbody>
        {{each rows}}
          <tr>
            <td>{email}</td>
            <td>{title}</td>
            <td>{{if done}}yes{{else}}no{{end}}</td>
          </tr>
        {{end}}
      </tbody>
    </table>
```

The check runs before the body: an anonymous visitor is redirected to `/login?next=%2Fadmin%2Ftodos`, and a logged-in `member` gets a `403 Forbidden` page. The account matching `SUPERUSER_EMAIL` gets through even though its role is `member`, thanks to [`superuser`](../reference/attributes/superuser.md).

To create your first admin, either:

1. Log in as `SUPERUSER_EMAIL`, or
2. Promote an existing user with one SQL statement:

```bash
sqlite3 app.db "UPDATE user SET role = 'admin' WHERE email = 'you@example.com';"
```

The role is copied into the session at login, so a promoted user must log out and log in again before the new role takes effect.

## 8. Try the gates end to end

1. Register two accounts: `alice@example.com` and `bob@example.com`.
2. Log in as Alice, create two todos.
3. Log out, log in as Bob, hit `/todos`. You should see an empty list.
4. Try `http://localhost:8080/admin/todos` as Bob. 403.
5. Promote Bob to `admin` with the SQL above, log out, log back in as Bob, and revisit the page. Alice's todos appear, with her email next to each one.

## 9. Final source

The full `app.kilnx` after Tutorials 1 and 2:

```kilnx
model user
  email: email required unique
  password: password required
  role: option [admin, member] default "member"
  created_at: timestamp auto

model todo
  title: text required min 1 max 200
  done: bool default false
  owner: user required
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
  member: read todo, write todo

page /login title "Log in"
  html
    <h1>Log in</h1>
    <p class="error">{params.error}</p>
    <form method="POST" action="/login?next={params.next}">
      <input type="hidden" name="_csrf" value="{csrf}">
      <label>Email <input name="identity" type="email"></label>
      <label>Password <input name="password" type="password"></label>
      <button type="submit">Log in</button>
    </form>
    <p><a href="/register">Create an account</a> . <a href="/forgot-password">Forgot your password?</a></p>

page /register title "Sign up"
  html
    <h1>Sign up</h1>
    <p class="error">{params.error}</p>
    <form method="POST" action="/register">
      <input type="hidden" name="_csrf" value="{csrf}">
      <label>Email <input name="identity" type="email"></label>
      <label>Password <input name="password" type="password"></label>
      <button type="submit">Create account</button>
    </form>

page /forgot-password title "Forgot password"
  html
    <h1>Forgot password</h1>
    {{if params.sent}}<p>If that address has an account, a reset link is on its way.</p>{{end}}
    <p class="error">{params.error}</p>
    <form method="POST" action="/forgot-password">
      <input type="hidden" name="_csrf" value="{csrf}">
      <label>Email <input name="email" type="email"></label>
      <button type="submit">Send reset link</button>
    </form>

page /reset-password title "Reset password"
  html
    <h1>Choose a new password</h1>
    <p class="error">{params.error}</p>
    <form method="POST" action="/reset-password?token={params.token}">
      <input type="hidden" name="_csrf" value="{csrf}">
      <label>New password <input name="password" type="password"></label>
      <label>Repeat it <input name="password_confirm" type="password"></label>
      <button type="submit">Reset password</button>
    </form>

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

page /todos title "Todos" requires auth
  query todos: SELECT id, title, done FROM todo
               WHERE owner_id = :current_user.id
               ORDER BY created_at DESC, id DESC
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

fragment /todos/:id/row requires auth
  query todo: SELECT id, title, done FROM todo
              WHERE id = :id AND owner_id = :current_user.id
  html
    {{todo_row id=todo.id label=todo.title done=todo.done}}

page /todos/new title "New todo" requires auth
  html
    <h1>New todo</h1>
    <form method="POST" action="/todos/create">
      <input type="hidden" name="_csrf" value="{csrf}">
      <label>Title <input name="title" required></label>
      <button type="submit">Create</button>
      <a href="/todos">Cancel</a>
    </form>

action /todos/create requires auth
  validate todo
  query: INSERT INTO todo (title, owner_id) VALUES (:title, :current_user.id)
  redirect /todos

page /todos/:id/edit title "Edit todo" requires auth
  query todo: SELECT id, title FROM todo
              WHERE id = :id AND owner_id = :current_user.id
  html
    <h1>Edit todo</h1>
    <form method="POST" action="/todos/{todo.id}/update">
      <input type="hidden" name="_csrf" value="{csrf}">
      <label>Title <input name="title" value="{todo.title}" required></label>
      <button type="submit">Save</button>
      <a href="/todos">Cancel</a>
    </form>

action /todos/:id/update requires auth
  validate todo
  query: UPDATE todo SET title = :title
         WHERE id = :id AND owner_id = :current_user.id
  redirect /todos

action /todos/:id/toggle requires auth
  query: UPDATE todo SET done = NOT done
         WHERE id = :id AND owner_id = :current_user.id
  redirect /todos/:id/row

action /todos/:id/delete requires auth
  query: DELETE FROM todo WHERE id = :id AND owner_id = :current_user.id
  respond fragment delete

page /admin/todos title "All todos" requires admin
  query rows: SELECT t.id, t.title, t.done, u.email
              FROM todo t JOIN user u ON u.id = t.owner_id
              ORDER BY t.created_at DESC
  html
    <h1>All todos</h1>
    <table>
      <thead><tr><th>User</th><th>Title</th><th>Done</th></tr></thead>
      <tbody>
        {{each rows}}
          <tr>
            <td>{email}</td>
            <td>{title}</td>
            <td>{{if done}}yes{{else}}no{{end}}</td>
          </tr>
        {{end}}
      </tbody>
    </table>
```

`kilnx check app.kilnx` prints `No issues found.` The warnings from Tutorial 1 are gone because every action now requires a login.

## Where to go next

Flag overdue todos on a schedule and email reminders from a background job: [Tutorial 3](03-background-jobs.md).
