# Deployment

> This page is a stub. Full content coming soon.

## Single binary

`kilnx build` produces a standalone binary. No runtime, no dependencies, no container required.

```bash
kilnx build app.kilnx -o myapp
scp myapp server:/opt/myapp/
ssh server "cd /opt/myapp && ./myapp"
```

The binary is ~15MB and self-contained. SQLite database is created in the working directory (or wherever `config database:` points).

## Environment variables

Production secrets belong in env vars:

```kilnx
config
  database: env DATABASE_URL required
  secret: env SECRET_KEY required
  port: env PORT default 8080
```

`env VAR required` fails fast if the variable is unset.

## Railway

Deploy via the [Railway template](https://railway.com/deploy/kilnx) — one click, zero config.

## Docker

A minimal Dockerfile:

```dockerfile
FROM debian:bookworm-slim
COPY myapp /usr/local/bin/myapp
EXPOSE 8080
CMD ["myapp"]
```

The binary is already statically linked. No base image tools needed beyond libc.

## PostgreSQL

Change the database URL:

```bash
DATABASE_URL=postgres://user:pass@host:5432/dbname ./myapp
```

Kilnx detects `postgres://` or `postgresql://` schemes and switches to the pgx driver automatically. Migrations run against the target database.

## Health checks

Every Kilnx app serves `GET /_kilnx/health` returning 200 OK with the app version. Point your load balancer at it.
