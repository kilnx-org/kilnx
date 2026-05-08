# Quickstart

Five minutes from zero to a running Kilnx app and a standalone binary.

## 1. Install the CLI

On macOS or Linux, install through Homebrew:

```bash
brew install kilnx-org/tap/kilnx
```

Or use the install script:

```bash
curl -fsSL https://raw.githubusercontent.com/kilnx-org/kilnx/main/install.sh | sh
```

The script downloads the latest release for your OS and architecture and drops the `kilnx` binary into `/usr/local/bin`. Verify the install:

```bash
kilnx version
```

If you prefer to build from source, clone the repo and run `go build -o kilnx ./cmd/kilnx/`.

## 2. Hello World

Create a file called `app.kilnx`:

```kilnx
page /
  html
    <h1>Hello World</h1>
```

Run it:

```bash
kilnx run app.kilnx
```

Open [http://localhost:8080](http://localhost:8080). You should see "Hello World".

The default port is `8080`. To change it, set `PORT=3000` in your environment, or declare `port: 3000` inside a [`config`](reference/keywords/config.md) block.

## 3. Add a model and a list

Stop the server with `Ctrl+C` and edit `app.kilnx`:

```kilnx
model task
  title: text required
  done: bool default false
  created: timestamp auto

page /
  query tasks: SELECT id, title, done FROM task ORDER BY created DESC
  html
    <h1>Tasks</h1>
    <ul>
      {{each tasks}}
      <li>{title} {{if done}}(done){{end}}</li>
      {{end}}
    </ul>
```

Run `kilnx run app.kilnx` again. The first start auto-migrates the schema, creating a `task` table in a SQLite database file. The default database URL is `sqlite://app.db`. Override it by setting `DATABASE_URL` in the environment.

The page is empty because there are no rows. Add one with the SQLite CLI or with a quick form:

```kilnx
action /tasks/new method POST
  query: INSERT INTO task (title) VALUES (:title)
  redirect /

page /new
  html
    <form method="post" action="/tasks/new">
      <input name="title" required>
      <button>Add</button>
    </form>
```

Visit [http://localhost:8080/new](http://localhost:8080/new), submit the form, and the redirect lands you back on the list.

## 4. Static analysis

Kilnx has a compile step that runs even before the server starts. To check your file without running it:

```bash
kilnx check app.kilnx
```

The analyzer validates field types, query column references, and route shapes. Errors point at line and column.

## 5. Build a standalone binary

When you are ready to ship:

```bash
kilnx build app.kilnx -o myapp
```

The output is a single executable, around 15 MB, that embeds the runtime, your templates, htmx, SQLite, and bcrypt. Copy it to any Linux or macOS host and run it:

```bash
scp myapp server:~/
ssh server './myapp'
```

No runtime install on the server, no container required.

## Where to go next

- [Mental model](concepts/mental-model.md): how the runtime executes your file.
- [Models and data](concepts/models-and-data.md): types, constraints, migrations.
- [Pages, actions, fragments](concepts/pages-actions-fragments.md): the HTTP layer.
- [Auth and permissions](concepts/auth-and-permissions.md): login, sessions, roles.
- [Reference](reference/index.md): every keyword and attribute.
