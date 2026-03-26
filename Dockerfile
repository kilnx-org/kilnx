# Dockerfile for building kilnx apps into minimal containers.
#
# Usage from the kilnx repo (with your .kilnx file):
#   docker build --build-arg KILNX_FILE=app.kilnx -t myapp .
#
# For Railway/Fly.io/Render: set KILNX_FILE build arg if not app.kilnx.
# The platform sets PORT env var automatically; the binary reads it.

FROM golang:1.25-alpine AS builder

WORKDIR /kilnx

# Download dependencies first (cache layer)
COPY go.mod go.sum ./
RUN go mod download

# Copy kilnx source and build the CLI
COPY . .
RUN CGO_ENABLED=0 go build -o /usr/local/bin/kilnx ./cmd/kilnx/

# Build the .kilnx app into a standalone binary.
# CWD must be inside the kilnx source tree so findKilnxRoot() works.
ARG KILNX_FILE=app.kilnx
COPY ${KILNX_FILE} /kilnx/${KILNX_FILE}
RUN kilnx build ${KILNX_FILE} -o /app/server

# Runtime stage: minimal image with just the compiled binary
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/server /usr/local/bin/server

WORKDIR /data
EXPOSE 8080

CMD ["server"]
