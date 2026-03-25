# Kilnx Grammar Reference

Kilnx has 29 keywords. The entire language fits on a single page.

For comparison: Python has 35 keywords and does none of these things without importing libraries. JavaScript has 64. Java has 67. Kilnx has 29 and delivers a complete web app from database to browser.

## Hello World

```kilnx
page /
  "Hello World"
```

One useful line. That's it.

---

## Keywords

### config

Global configuration. Database, port, secrets, upload limits.

```kilnx
config
  database: env DATABASE_URL default "sqlite://app.db"
  port: env PORT default 8080
  secret: env SECRET_KEY required
  uploads: ./uploads max 50mb
```

### model

Defines data types and structure. The single source of truth. From a model, the language generates: CREATE TABLE, server validation, HTML forms, client validation, and listing fragments.

```kilnx
model user
  name: text required min 2 max 100
  email: email unique
  role: option [admin, editor, viewer] default viewer
  active: bool default true
  created: timestamp auto
```

Relationships between models:

```kilnx
model post
  title: text required min 5
  body: richtext required
  status: option [draft, published, archived] default draft
  author: user required
  created: timestamp auto
  published_at: timestamp optional

model comment
  body: text required
  post: post required
  author: user required
  created: timestamp auto
```

### permissions

Access rules by role.

```kilnx
permissions
  admin: all
  editor: read post, write post where author = current_user
  viewer: read post where status = published
```

### auth

Authentication configuration. Declarative, not code.

```kilnx
auth
  table: users
  identity: email
  password: password_hash
  login: /login
  after login: /dashboard
```

### layout

Page wrapper templates.

```kilnx
layout main
  html
    <html>
    <head>
      <title>{page.title}</title>
    </head>
    <body>
      <nav>...</nav>
      {page.content}
    </body>
    </html>
```

### page

GET route that returns full HTML. The basic unit of the language.

```kilnx
page /users layout main title "Users"
  query users: select name, email from user
  list users
    title: name
    subtitle: email
```

With auth:

```kilnx
page /dashboard requires auth
  query stats: select count(*) as total from orders
  "Welcome back. You have {stats.total} orders."
```

### action

POST/PUT/DELETE route that mutates data.

```kilnx
action /users/:id/archive method POST requires auth
  query: update users set archived = true where id = :id
  respond fragment user-card with query:
    select name, email from users where id = :id
```

### fragment

Reusable piece of HTML for htmx to swap in the DOM.

```kilnx
fragment /users/:id/card
  query user: select name, email from users where id = :id
  html
    <div class="card">
      <h3>{user.name}</h3>
      <p>{user.email}</p>
    </div>
```

### stream

Server-Sent Events for realtime updates.

```kilnx
stream /notifications requires auth
  query: select message, created_at from notifications
         where user_id = :current_user.id
         and seen = false
  every 5s
```

### socket

Bidirectional WebSocket.

```kilnx
socket /chat/:room requires auth
  on connect
    query: select message, author.name, created
           from chat_message
           where room = :room
           order by created desc
           limit 50
    send history

  on message
    validate
      body: required max 500
    query: insert into chat_message (body, author, room)
           values (:body, :current_user.id, :room)
    broadcast to :room
      fragment chat-bubble with
        body: :body
        author: :current_user.name
        created: now()
```

### component

Custom declarative UI blocks.

```kilnx
component user-card
  param: name, email, avatar
  card
    image: avatar
    title: name
    subtitle: email
    action: edit /users/:id
```

### api

JSON endpoint. Same grammar as page, but returns JSON instead of HTML.

```kilnx
api /api/v1/posts requires auth
  query posts: select id, title, status, author.name, created
               from post
               where status = published
               order by created desc
               paginate 50

api /api/v1/posts method POST requires editor
  validate
    title: required min 5
    body: required
  query: insert into post (title, body, author, status)
         values (:title, :body, :current_user.id, draft)
  respond status 201
```

### webhook

Receives external events.

```kilnx
webhook /stripe/payment secret env STRIPE_SECRET
  on event payment_intent.succeeded
    query: update order set status = paid
           where stripe_id = :event.id
    send email to query: select email from user
                         where id = :event.customer_id
      template: payment-received
      subject: "Payment confirmed"
```

### schedule

Timed tasks running inside the same binary.

```kilnx
schedule cleanup every 24h
  query: delete from session where expires_at < now()

schedule report every monday at 9:00
  query stats: select count(*) as new_users from user
               where created > now() - interval 7 days
  send email to query: select email from user where role = admin
    template: weekly-report
    subject: "Weekly report: {stats.new_users} new users"
```

### job

Asynchronous background work.

```kilnx
job generate-report
  query data: select * from order
              where created > :start_date
              and created < :end_date
  generate pdf from template report with data
  send email to :requested_by
    template: report-ready
    attach: generated pdf
    subject: "Your report is ready"
```

### query / queries

SQL inline or named. Queries can be defined at the top of a file and referenced by name.

```kilnx
queries
  active-users: select u.name, u.email, count(o.id) as orders
                from users u
                left join orders o on o.user_id = u.id
                where u.active = true
                group by u.id

page /users
  query users: active-users
  list users
    title: name
    subtitle: "{orders} orders"
```

### validate

Declarative validation rules.

```kilnx
action /users/new method POST
  validate
    name: required
    email: required, is email
  query: insert into users (name, email) values (:name, :email)
  redirect /users
```

### search

Full-text search with htmx integration.

```kilnx
page /posts
  search posts in title, body
  query posts: select title, author.name from post
               where status = published
               order by published_at desc
               paginate 20
  table posts
    columns: title, author.name
```

### paginate

Automatic pagination. The language generates pagination controls with htmx.

```kilnx
page /posts
  query posts: select title, author.name from post
               where status = published
               order by published_at desc
               paginate 20
  table posts
    columns: title, author.name
```

### form

Auto-generated from a model.

```kilnx
page /users/new
  form user

page /users/:id/edit
  form user with query: select * from user where id = :id
```

### send email

Declarative email sending.

```kilnx
action /users/invite method POST requires admin
  validate
    email: required, is email
  query: insert into user (email, role, active)
         values (:email, viewer, false)
  send email to :email
    template: invite
    subject: "You've been invited"
```

### redirect

Redirects to another route.

```kilnx
action /users/create method POST
  validate user
  query: insert into user (name, email) values (:name, :email)
  redirect /users
```

### on

Result handling for success, error, not found, forbidden.

```kilnx
action /users/:id/delete method POST requires auth
  query: delete from users where id = :id
  on success: redirect /users
  on error: alert "Could not delete user"
  on forbidden: redirect /login
```

### limit

Rate limiting. Declarative.

```kilnx
limit /api/*
  requests: 100 per minute per user
  on exceeded: status 429 message "Too many requests"

limit /login
  requests: 5 per minute per ip
  on exceeded: status 429 message "Too many attempts"
    delay 30s
```

### log

Observability built in.

```kilnx
log
  level: env LOG_LEVEL default info
  queries: slow > 100ms
  requests: all
  errors: all with stacktrace
```

### test

Declarative tests in the same language.

```kilnx
test "user can create post"
  as user with role editor
  visit /posts/new
  fill title with "Test Post"
  fill body with "Content here"
  submit
  expect page /posts contains "Test Post"
  expect query: select count(*) from post
                where title = 'Test Post'
         returns 1
```

### translations

Internationalization.

```kilnx
translations
  en
    welcome: "Welcome back"
    users: "Users"
  pt
    welcome: "Bem vindo de volta"
    users: "Usuários"

config
  default language: en
  detect language: header accept-language

page /dashboard requires auth
  "{t.welcome}, {current_user.name}"
```

### enqueue

Dispatches an async job.

```kilnx
action /reports/generate method POST requires admin
  validate
    start_date: required, is date
    end_date: required, is date
  enqueue generate-report with
    start_date: :start_date
    end_date: :end_date
    requested_by: :current_user.email
  respond fragment ".reports" with
    alert success "Report is being generated"
```

### broadcast

Sends data to all connected WebSocket clients in a room.

```kilnx
socket /chat/:room requires auth
  on message
    query: insert into chat_message (body, author, room)
           values (:body, :current_user.id, :room)
    broadcast to :room
      fragment chat-bubble with
        body: :body
        author: :current_user.name
```

---

## Built-in Components

The language ships with built-in UI components that render semantic HTML:

- **list** - renders items with title, subtitle, actions
- **table** - renders tabular data with columns, row actions
- **card** - renders a card with image, title, subtitle, actions
- **form** - auto-generated from a model
- **alert** - success/error/warning messages
- **nav** - navigation component
- **modal** - modal dialog
- **chart** - basic charts

Custom components are created with the `component` keyword.

---

## Complete App Example

```kilnx
config
  database: env DATABASE_URL default "sqlite://app.db"
  port: 8080
  secret: env SECRET_KEY required

model user
  name: text required
  email: email unique
  password: password required
  role: option [admin, user] default user
  created: timestamp auto

model task
  title: text required
  done: bool default false
  owner: user required
  created: timestamp auto

auth
  table: user
  identity: email
  password: password
  login: /login
  after login: /tasks

layout main
  html
    <html>
    <head><title>Tasks</title></head>
    <body>
      <nav>
        <a href="/tasks">Tasks</a>
        <a href="/logout">Logout</a>
      </nav>
      {page.content}
    </body>
    </html>

page /tasks layout main requires auth
  search tasks in title
  query tasks: select id, title, done from task
               where owner = :current_user.id
               order by created desc
               paginate 20
  table tasks
    columns: title, done
    row action: delete /tasks/:id/delete

page /tasks/new layout main requires auth
  form task

action /tasks/create method POST requires auth
  validate task
  query: insert into task (title, owner)
         values (:title, :current_user.id)
  redirect /tasks

action /tasks/:id/delete method POST requires auth
  query: delete from task where id = :id and owner = :current_user.id
  respond fragment ".task-list" with query:
    select id, title, done from task
    where owner = :current_user.id
    order by created desc
```
