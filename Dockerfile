# syntax=docker/dockerfile:1

ARG GO_VERSION=1.27

# ---------------------------------------------------------------------------
# Build
# ---------------------------------------------------------------------------
# Pinned to the *build* platform and cross-compiled below via TARGETOS/TARGETARCH.
# That is what keeps a linux/arm64 build on an amd64 runner from falling back to
# QEMU emulation, which costs roughly an order of magnitude in build time.
FROM --platform=${BUILDPLATFORM} golang:${GO_VERSION} AS build

WORKDIR /src

# Dependencies first: this layer is reused by every build that does not touch
# go.mod or go.sum.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# The whole context in one layer, on purpose. Granular per-directory COPYs are
# how a build ends up compiling stale source: add a package, forget to COPY it
# (or exclude it in .dockerignore) and the layer digest never changes, so the
# build is cached and the binary silently freezes. The expensive step here is
# `go build`, and the cache mount below already makes that incremental.
COPY . .

# CGO stays off so the result is a static binary: every driver in this module is
# pure Go (pgx for Postgres, go-redis, the AWS SDK), so nothing needs libc.
ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags='-s -w' -o /usr/local/bin/api ./cmd/api && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags='-s -w' -o /usr/local/bin/migrate ./cmd/migrate

# ---------------------------------------------------------------------------
# Runtime
# ---------------------------------------------------------------------------
# distroless/static: no shell, no package manager, runs as an unprivileged user,
# and carries the CA bundle the app needs to reach Postgres over TLS, Redis over
# rediss:// and S3/R2 over HTTPS.
#
# Swap in `alpine:3.21` if you need a shell in the container (to exec in, or for
# a compose-level HEALTHCHECK); nothing else in this file has to change.
FROM gcr.io/distroless/static-debian12:nonroot AS runtime

# Passed by CI so the image records the commit it was built from.
ARG RELEASE_SHA=unknown

LABEL org.opencontainers.image.title="medusa-api" \
      org.opencontainers.image.description="Medusa API" \
      org.opencontainers.image.revision="${RELEASE_SHA}"

COPY --from=build /usr/local/bin/api /usr/local/bin/api
COPY --from=build /usr/local/bin/migrate /usr/local/bin/migrate

# HOST matters: the app defaults to localhost, which inside a container means
# "reachable from nothing". APP_ENV=production turns on Gin release mode and the
# stricter config validation, so JWT_SECRET has to be at least 32 characters.
ENV HOST=0.0.0.0 \
    PORT=8000 \
    APP_ENV=production \
    RELEASE_SHA=${RELEASE_SHA}

EXPOSE 8000

USER nonroot:nonroot

ENTRYPOINT ["/usr/local/bin/api"]

# Migrations ship in the same image, so they run against the exact code being
# deployed rather than whatever the last build produced:
#
#   docker run --rm --env-file .env --entrypoint /usr/local/bin/migrate <image>
