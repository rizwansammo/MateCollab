# syntax=docker/dockerfile:1
#
# Go services: one build stage, one minimal runtime target per binary.
#   docker build -f infra/docker/go.Dockerfile --target api .
# Targets: api, worker, mcctl. Runtime is distroless (no shell, no package manager) and runs as
# the unprivileged nonroot user. Migrations ship with every image at /app/db/migrations.

FROM golang:1.27-alpine@sha256:f92b6ef800e499660581efdabdf25d9d817a9d124eaf900924f0504e7e27e12d AS build
ENV CGO_ENABLED=0 GOTOOLCHAIN=local
WORKDIR /src/services
COPY services/go.mod services/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY services/ ./
ARG COMMIT=unknown
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath \
      -ldflags "-s -w -X github.com/rizwansammo/MateCollab/services/internal/app.Commit=${COMMIT}" \
      -o /out/ ./api ./worker ./mcctl

FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab AS runtime
WORKDIR /app
COPY db/migrations /app/db/migrations
ENV MC_MIGRATIONS_DIR=/app/db/migrations
USER nonroot:nonroot

FROM runtime AS api
COPY --from=build /out/api /usr/local/bin/api
EXPOSE 8081 8091
ENTRYPOINT ["/usr/local/bin/api"]

FROM runtime AS worker
COPY --from=build /out/worker /usr/local/bin/worker
EXPOSE 8092
ENTRYPOINT ["/usr/local/bin/worker"]

FROM runtime AS mcctl
COPY --from=build /out/mcctl /usr/local/bin/mcctl
ENTRYPOINT ["/usr/local/bin/mcctl"]
