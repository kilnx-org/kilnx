# Build stage: compile the .kilnx app into a static binary
FROM golang:1.25-alpine AS builder

WORKDIR /kilnx

# Copy kilnx source and install
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /usr/local/bin/kilnx ./cmd/kilnx/

# Build the app
WORKDIR /app
COPY . /kilnx-src
ARG KILNX_FILE=app.kilnx
COPY ${KILNX_FILE} .
RUN kilnx build ${KILNX_FILE} -o server

# Runtime stage: minimal image with just the binary
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/server /usr/local/bin/server

WORKDIR /data
EXPOSE 8080

CMD ["server"]
