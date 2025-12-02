# syntax=docker/dockerfile:1

FROM --platform=${BUILDPLATFORM} docker.io/tonistiigi/xx:golang AS xx
FROM --platform=${BUILDPLATFORM} golang:alpine AS base
COPY --from=xx / /
WORKDIR /src

FROM base AS tests
ARG TARGETPLATFORM
RUN --mount=target=. \
    --mount=type=cache,target=/go/pkg \
    --mount=type=cache,target=/root/.cache \
      go test -tags 'netgo osusergo static_build' -v -cover ./...
