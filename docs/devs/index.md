# Kilnx for Developers

Kilnx is a declarative backend language that compiles to a single Go binary. You describe your app in a `.kilnx` file: models, pages, actions, queries, auth. The compiler handles HTTP routing, SQL migrations, template rendering, sessions, CSRF, and htmx integration.

The whole language is around 27 keywords. The runtime ships with two dependencies: SQLite and bcrypt. There is no JavaScript build step, no ORM to configure, no framework to pick.

A complete app can fit in three lines:

```kilnx
page /
  html
    <h1>Hello World</h1>
```

`kilnx run app.kilnx` and you have an HTTP server on port 8080.

## What to read next

If you have never run Kilnx before, start with the quickstart. It walks from install to a running binary in five minutes.

If you want to understand the model behind the language before writing code, jump into the concepts. They explain the runtime, data layer, request lifecycle, auth, and background work.

If you already know the basics and want to look up a keyword, the reference is generated from the parser spec and is the source of truth.

## Navigation

- [Quickstart](quickstart.md): install, write `app.kilnx`, run it, build a binary.
- [Concepts](concepts/index.md): the mental model and the main building blocks.
  - [Mental model](concepts/mental-model.md)
  - [Models and data](concepts/models-and-data.md)
  - [Pages, actions, fragments](concepts/pages-actions-fragments.md)
  - [Auth and permissions](concepts/auth-and-permissions.md)
  - [Jobs and schedules](concepts/jobs-and-schedules.md)
- [Reference](reference/index.md): every keyword and attribute, with syntax and examples.

## Where Kilnx fits

Kilnx is a full-stack server framework, closer in scope to Rails or Django than to Express or FastAPI. It expects to own the database schema, the HTTP layer, and the rendered HTML. The natural front end is [htmx](https://htmx.org), already embedded in the runtime, but any client that speaks HTTP works.

Two design choices set the tone for everything else:

1. SQL is part of the language. Queries are inline, parsed and validated against your models at compile time. There is no ORM in the path.
2. The output is a binary. `kilnx build app.kilnx -o app` produces one executable that embeds the runtime, your templates, htmx, SQLite, and bcrypt. Copy it to a server and run it.

Read [PRINCIPLES.md](https://github.com/kilnx-org/kilnx/blob/main/PRINCIPLES.md) for the constitutional rules that drive these decisions.

## Editor support

A [VS Code extension](https://marketplace.visualstudio.com/items?itemName=atoolz.kilnx-vscode-toolkit) provides syntax highlighting, diagnostics, completions, hover docs, and go-to-definition through the built-in LSP server.

## Try it without installing

The [online playground](https://kilnx.org/playground.html) runs Kilnx in the browser. Useful for quick experiments before installing the CLI.
