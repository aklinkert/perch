# Stage 1: Build Tailwind CSS
# Using node:22-alpine + npm to install tailwindcss@3 and run the CLI.
# This avoids architecture-specific binary fetching.
FROM node:22-alpine AS tailwind

WORKDIR /app

COPY web/tailwind.css web/tailwind.css
COPY tailwind.config.js tailwind.config.js
COPY internal/views/ internal/views/

RUN npm init -y && \
    npm install --save-dev tailwindcss@^3 && \
    npx tailwindcss -i web/tailwind.css -o web/static/app.css --minify

# Stage 2: Build Go binary
FROM golang:1.26-bookworm AS build

# CGO is required for SQLite (go-sqlite3); glibc base matches distroless/cc runtime.
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential libsqlite3-dev \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Overwrite the placeholder app.css with the Tailwind-built one
COPY --from=tailwind /app/web/static/app.css ./web/static/app.css

RUN CGO_ENABLED=1 go build \
    -trimpath \
    -ldflags='-s -w' \
    -o /out/perch \
    ./cmd/perch

# Pre-create /data with nonroot ownership so the named volume inherits perms.
RUN mkdir -p /out/data && chown 65532:65532 /out/data

# Stage 3: Runtime — distroless/cc for CGO (glibc-compatible)
FROM gcr.io/distroless/cc-debian12:nonroot

COPY --from=build /out/perch /perch
COPY --from=build --chown=nonroot:nonroot /out/data /data

EXPOSE 8080

VOLUME ["/data"]

ENTRYPOINT ["/perch", "run", "--config", "/etc/perch/config.yaml"]
