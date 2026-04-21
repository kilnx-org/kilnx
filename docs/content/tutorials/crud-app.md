# Build a CRUD app

> This tutorial is a stub. Full walkthrough coming soon.

Goal: a blog app with user registration, login, post creation, pagination, and inline editing — in under 80 lines of Kilnx.

```kilnx
config
  database: env DATABASE_URL default "sqlite://blog.db"
  secret: env SECRET_KEY required

model user
  name: text required min 2
  email: email required unique
  password: password required
  role: option [admin, author, reader] default reader
  created: timestamp auto

model post
  title: text required min 5 max 200
  body: richtext required
  author: user required
  published: bool default false
  created: timestamp auto
  updated: timestamp auto_update

auth
  table: user
  identity: email
  password: password
  login: /login
  after login: /

permissions
  admin: all
  author: read post, write post where author = current_user
  reader: read post where published = true

layout main
  html
    <!doctype html>
    <html>
    <head>
      <title>{page.title} · Blog</title>
      {kilnx.js}
    </head>
    <body>
      <header>{nav}</header>
      <main>{page.content}</main>
    </body>
    </html>

page / layout main title "Blog"
  query posts: SELECT title, body, author.name as author, created
               FROM post
               WHERE published = true
               ORDER BY created DESC
               paginate 10
  html
    {{each posts}}
      <article>
        <h2>{title}</h2>
        <p><small>by {author} &middot; {created | timeago}</small></p>
        <div>{body | raw}</div>
      </article>
    {{end}}

page /compose requires author layout main title "New post"
  html
    <form method="post" action="/posts/create">
      <input name="title" placeholder="Title" required>
      <textarea name="body" placeholder="Body" required></textarea>
      <button>Publish</button>
    </form>

action /posts/create method POST requires author
  validate post
  query: INSERT INTO post (title, body, author_id, published)
         VALUES (:title, :body, :current_user.id, true)
  redirect /
```

Run it:

```bash
kilnx run app.kilnx
```

Register at `/register`, then compose at `/compose` and see the post on `/`.

A proper walkthrough covering form validation errors, draft vs published states, edit/delete actions, and htmx-powered inline editing is coming. For now, the code above is a working blog.
