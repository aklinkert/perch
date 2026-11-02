# Stage 1: Build Tailwind CSS
# Using node:22-alpine + npm to install tailwindcss@3 and run the CLI.
# This avoids architecture-specific binary fetching. The CSS output is
# platform-independent, so this stage always runs natively on multi-arch builds.
FROM --platform=$BUILDPLATFORM node:22-alpine AS tailwind

WORKDIR /app

COPY web/tailwind.css web/tailwind.css
COPY tailwind.config.js tailwind.config.js
COPY internal/views/ internal/views/

RUN npm init -y && \
    npm install --save-dev tailwindcss@^3 && \
    npx tailwindcss -i web/tailwind.css -o web/static/app.css --minify

# Stage 2: Build Go binary
# Runs on the build platform and cross-compiles for the target, so multi-arch
# builds never execute Go under emulation.
FROM --platform=$BUILDPLATFORM golang:1.27-bookworm AS build

ARG BUILDARCH TARGETOS TARGETARCH

# CGO is required for SQLite (go-sqlite3); glibc base matches distroless/cc runtime.
# go-sqlite3 bundles the SQLite amalgamation, so only a C toolchain for the
# target architecture is needed.
RUN case "$TARGETARCH" in \
      amd64) triplet=x86_64-linux-gnu ;; \
      arm64) triplet=aarch64-linux-gnu ;; \
      *) echo "unsupported TARGETARCH: $TARGETARCH" >&2; exit 1 ;; \
    esac \
 && if [ "$TARGETARCH" = "$BUILDARCH" ]; then \
      pkgs="gcc libc6-dev"; \
    else \
      pkgs="gcc-$(echo "$triplet" | tr _ -) libc6-dev-$TARGETARCH-cross"; \
    fi \
 && apt-get update && apt-get install -y --no-install-recommends $pkgs \
 && rm -rf /var/lib/apt/lists/* \
 && echo "$triplet-gcc" > /tmp/cc

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Overwrite the placeholder app.css with the Tailwind-built one
COPY --from=tailwind /app/web/static/app.css ./web/static/app.css

RUN CGO_ENABLED=1 GOOS=$TARGETOS GOARCH=$TARGETARCH CC="$(cat /tmp/cc)" go build \
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
