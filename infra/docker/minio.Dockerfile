# Local-development object storage only (ADR-010).
#
# MinIO no longer publishes community images (Docker Hub and Quay return no tags and the
# upstream repository is archived), so the final community source release is built locally.
# The tag is verified against its commit so a moved tag cannot change what is built.
# Never use this image outside local development.

FROM golang:1.26-alpine@sha256:3082400e369fa24d5fc60bca20edab3f6d604e0c5a690ec66b295eff4dd87ade AS build
ARG MINIO_TAG=RELEASE.2025-10-15T17-29-55Z
ARG MINIO_COMMIT=9e49d5e7a648f00e26f2246f4dc28e6b07f8c84a
RUN apk add --no-cache git
WORKDIR /src
RUN git clone --quiet --depth 1 --branch "${MINIO_TAG}" https://github.com/minio/minio.git . \
 && test "$(git rev-parse HEAD)" = "${MINIO_COMMIT}"
ENV CGO_ENABLED=0 GOTOOLCHAIN=local
RUN go build -trimpath -tags kqueue -ldflags "$(go run buildscripts/gen-ldflags.go)" -o /out/minio .

FROM alpine:3.22@sha256:5291449c3df73caf6ed85e649dec1b9e818b39a5d8c871e97afc13e9cd5e8fa8
RUN apk add --no-cache ca-certificates curl \
 && adduser -D -H -u 10001 minio \
 && mkdir -p /data && chown minio:minio /data
COPY --from=build /out/minio /usr/local/bin/minio
USER minio
EXPOSE 9000 9001
ENTRYPOINT ["minio"]
CMD ["server", "/data", "--console-address", ":9001"]
