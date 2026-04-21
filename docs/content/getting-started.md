# Getting started

Kilnx runs on Linux, macOS, and Windows. A single binary, no runtime.

## Install

### Homebrew (macOS, Linux)

```bash
brew tap kilnx-org/tap
brew install kilnx
```

### Direct download

Grab the latest release from [github.com/kilnx-org/kilnx/releases](https://github.com/kilnx-org/kilnx/releases), extract, and place `kilnx` on your `PATH`.

### Verify

```bash
kilnx version
```

## Hello world

Create `app.kilnx`:

```kilnx
page /
  "Hello World"
```

Run it:

```bash
kilnx run app.kilnx
```

Open `http://localhost:8080`. Two useful lines. That's a web server.

## A real app in 20 lines

```kilnx
config
  database: env DATABASE_URL default "sqlite://app.db"

model post
  title: text required min 2 max 200
  body: richtext required
  created: timestamp auto

page /
  query posts: SELECT title, body, created FROM post
               ORDER BY created DESC paginate 10
  html
    <h1>Blog</h1>
    {{each posts}}
      <article>
        <h2>{title}</h2>
        <time>{created | timeago}</time>
        <div>{body | raw}</div>
      </article>
    {{end}}

action /posts/create method POST
  validate post
  query: INSERT INTO post (title, body) VALUES (:title, :body)
  redirect /
```

On first run, Kilnx auto-creates the SQLite database, runs migrations based on the `model` definitions, and serves the page. There is no separate migration step to run.

## CLI essentials

| Command | Purpose |
|---------|---------|
| `kilnx run <file>` | Dev server with hot reload |
| `kilnx build <file> -o <binary>` | Compile to standalone binary |
| `kilnx check <file>` | Static analysis (types, security, SQL) |
| `kilnx test <file>` | Run declarative `test` blocks |
| `kilnx migrate <file>` | Apply schema migrations (also runs automatically at `run`) |
| `kilnx lsp` | Language Server Protocol endpoint for editors |
| `kilnx mcp` | Model Context Protocol server for AI tools |

See the full [CLI reference](reference/cli.html) for flags and options.

## Next steps

- [Models](guides/models.html) — field types, constraints, references
- [Pages &amp; actions](guides/pages-actions.html) — routing, mutations, redirects
- [Auth &amp; permissions](guides/auth-permissions.html) — register, login, roles
- [Grammar reference](reference/grammar.html) — the complete syntax specification
