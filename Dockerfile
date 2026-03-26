# Dockerfile for building kilnx apps into minimal containers.
#
# Usage from a user's project directory (containing app.kilnx):
#   docker build --build-arg KILNX_FILE=app.kilnx -t myapp .
#
# For Railway/Fly.io/Render: just set KILNX_FILE build arg if not app.kilnx.
# The platform sets PORT env var automatically; the binary reads it.

# Build stage: install kilnx from source, then compile the .kilnx app
FROM golang:1.25-alpine AS builder

WORKDIR /kilnx-src

# Copy kilnx source and build the CLI
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-X github.com/kilnx-org/kilnx/internal/build.Version=$(go list -m -f '{{.Version}}' github.com/kilnx-org/kilnx 2>/dev/null || echo v0.0.0-local)" -o /usr/local/bin/kilnx ./cmd/kilnx/

# Build the .kilnx app into a standalone binary.
# CWD is inside the kilnx source tree so findKilnxRoot() discovers it.
ARG KILNX_FILE=app.kilnx
COPY ${KILNX_FILE} /kilnx-src/${KILNX_FILE}
RUN kilnx build ${KILNX_FILE} -o /app/server

# Runtime stage: minimal image with just the compiled binary
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/server /usr/local/bin/server

WORKDIR /data
EXPOSE 8080

CMD ["server"]
