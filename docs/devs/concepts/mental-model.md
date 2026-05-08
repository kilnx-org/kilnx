# Mental model

Kilnx is a declarative language for web backends. You write `.kilnx` files, the compiler reads them, and the runtime serves HTTP. The whole thing is around 27 keywords.

## Declarative, not imperative

A Kilnx file describes the shape of an application: which models exist, which routes serve them, which queries run on each route. There is no `main` function, no request handler signature, no router to register. You declare a [`page`](../reference/keywords/page.md), and a route exists.

```kilnx
page /about
  html
    <h1>About us</h1>
```

When something needs custom logic, the language gives you primitives (`query`, `validate`, `redirect`, `respond`, `enqueue`, `send`, `fetch`) instead of dropping you into Go.

## Indent-significant syntax

Blocks are defined by indentation, like Python or YAML. Two spaces per level is conventional. The keyword opens a block, and indented children belong to it:

```kilnx
model task
  title: text required
  done: bool default false
```

Children of `model` are field declarations. Children of `page` are queries, validations, and the response. The parser rejects mixed tabs and spaces.

## How `kilnx run` and `kilnx build` differ

There are two ways to execute a `.kilnx` file.

`kilnx run app.kilnx` parses the file, builds the AST in memory, and starts an HTTP server that interprets it. Migrations apply on startup. Edits to the file are picked up on restart. This is the dev loop.

`kilnx build app.kilnx -o app` produces a standalone binary that embeds the AST, the runtime, your templates, htmx, SQLite, and bcrypt. The output is around 15 MB and has no external dependencies. This is what you ship.

There is no separate "code generation" step in either case. The AST drives the runtime. `build` just freezes the AST inside an executable. There is no Go file written to disk that mirrors your `.kilnx`.

## Pipeline

Every run, in either mode, walks the same pipeline:

```
.kilnx -> Lexer -> Parser -> Analyzer -> Optimizer -> Runtime
```

The analyzer checks types, validates that queries reference real columns, and confirms that templates only use bound variables. The optimizer rewrites `SELECT *` to the columns your templates actually use, and folds constants. The runtime then serves HTTP.

If `kilnx check app.kilnx` returns clean, the file will run.

## How it compares

If you know Rails or Django, Kilnx is in the same neighborhood: a full-stack server framework that owns the database, the routes, and the rendered HTML. The differences are scope and surface.

Rails and Django are libraries on top of a host language. You write Ruby or Python, you call into the framework, and the framework calls back into your code. Kilnx is the language. There is no host language to learn alongside it. The downside is that custom logic outside the provided primitives (LLM calls, email, fetch, validate, queries) is not available; the upside is that you can read any Kilnx app from top to bottom without leaving the file.

If you know [htmx](https://htmx.org), Kilnx is the natural server pair. Pages render full HTML, [fragments](../reference/keywords/fragment.md) render partials, and the runtime understands swap targets, triggers, and SSE. htmx is bundled in the binary, but it is not required: any client that speaks HTTP works. JSON endpoints exist via [`api`](../reference/keywords/api.md).

## What the file produces

A single `.kilnx` file can contain everything: models, auth, pages, fragments, APIs, jobs, schedules, webhooks, tests. Files can also be split, but the runtime sees the merged AST. There is no module system in the conventional sense, because there is no procedural code to scope.

The output of `kilnx run` is an HTTP server. The output of `kilnx build` is a binary. Both are driven by the same AST, validated by the same analyzer, optimized by the same passes.

## Read next

- [Models and data](models-and-data.md): how `model` becomes a table.
- [Pages, actions, fragments](pages-actions-fragments.md): the HTTP layer.
