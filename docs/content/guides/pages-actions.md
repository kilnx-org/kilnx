# Pages &amp; actions

Pages are GET routes that return HTML. Actions are POST/PUT/DELETE routes that mutate data.

## Pages

```kilnx
page /posts
  query posts: SELECT title, created FROM post ORDER BY created DESC
  html
    <h1>All posts</h1>
    {{each posts}}
      <article><h2>{title}</h2></article>
    {{end}}
```

### Modifiers

```kilnx
page /dashboard requires auth layout main title "Dashboard"
```

| Modifier | Effect |
|----------|--------|
| `requires auth` | Redirects to login if no session |
| `requires <role>` | Requires a specific role (e.g. `requires admin`) |
| `layout <name>` | Wraps output in the named `layout` block |
| `title "<text>"` | Sets `{page.title}` for the layout |

### Path parameters

```kilnx
page /posts/:slug
  query post: SELECT title, body FROM post WHERE slug = :slug
  html
    <article>
      <h1>{post.title}</h1>
      <div>{post.body | raw}</div>
    </article>
```

Named parameters use `:name` both in the path and in the SQL. Kilnx binds them automatically (safely — no string concatenation).

### Inline text

Short pages skip `html`:

```kilnx
page /about
  "About us"
```

## Actions

POST routes for mutations:

```kilnx
action /posts/create method POST
  validate post
  query: INSERT INTO post (title, body) VALUES (:title, :body)
  redirect /posts
```

### The action body

| Construct | Purpose |
|-----------|---------|
| `validate <model>` | Validate form against model constraints |
| `validate` (block) | Inline validation rules |
| `query: <SQL>` | Run SQL (INSERT/UPDATE/DELETE or SELECT) |
| `query <name>: <SQL>` | Named query — result available in later steps |
| `on success` / `on error` / `on not found` | Conditional branches |
| `redirect <path>` | HTTP redirect (also supports htmx HX-Redirect) |
| `respond fragment <selector>` | Return partial HTML for htmx swaps |
| `enqueue <job>` | Dispatch an async job |
| `send email to <recipient>` | Send an email |

### Validation

Against a model&apos;s constraints:

```kilnx
action /users/create method POST
  validate user
  query: INSERT INTO user (name, email) VALUES (:name, :email)
  redirect /users
```

Inline rules:

```kilnx
action /login method POST
  validate
    email: required, is email
    password: required, min 8
  query user: SELECT id FROM user WHERE email = :email
  on not found
    redirect /login?error=invalid
  redirect /dashboard
```

### Branching

```kilnx
action /posts/create method POST
  validate post
  query: INSERT INTO post (title, body, author_id)
         VALUES (:title, :body, :current_user.id)
  on success
    redirect /posts
  on error
    alert "Could not create post"
```

### Transactions

All queries within a single action run in an implicit transaction. If any query fails, all prior writes roll back.

## Implicit queries

Inside a page body, `query <name>: <SQL>` makes the result available for template interpolation:

```kilnx
page /posts/:id
  query post: SELECT * FROM post WHERE id = :id
  query comments: SELECT body, author FROM comment
                  WHERE post_id = :id
                  ORDER BY created ASC
  html
    <h1>{post.title}</h1>
    <p>{post.body}</p>
    <h2>Comments ({comments.count})</h2>
    {{each comments}}
      <div><strong>{author}</strong>: {body}</div>
    {{end}}
```

Single-row queries give `{name.field}`. Multi-row queries are iterable via `{{each name}}...{{end}}`.

## Pagination

Add `paginate N` to a SELECT:

```kilnx
query posts: SELECT title FROM post ORDER BY created DESC paginate 20
```

Kilnx reads `?page=N` from the request, injects LIMIT/OFFSET, and exposes `{posts.pagination.next}`, `{posts.pagination.prev}`, `{posts.pagination.total}` in the template.

## Template filters

Built-in filters for formatting:

```
{name | upcase}                  ALICE
{name | truncate:20}             Alice Wonderla...
{created | timeago}              3 hours ago
{created | date:"Jan 02, 2006"}  Mar 27, 2026
{price | currency:"$"}           $1,234.56
{bio | raw}                      unescaped HTML
```

## Layouts

Wrap multiple pages in a common HTML shell:

```kilnx
layout main
  html
    <html>
    <head>
      <title>{page.title}</title>
      {kilnx.js}
    </head>
    <body>
      {nav}
      {page.content}
    </body>
    </html>

page /dashboard layout main title "Dashboard"
  html
    <h1>Welcome</h1>
```

Four placeholders:

- `{page.title}` — escaped page title
- `{page.content}` — the rendered page body
- `{nav}` — auto-generated navigation bar
- `{kilnx.js}` — **required.** htmx and SSE scripts. Without this, htmx functionality breaks.
