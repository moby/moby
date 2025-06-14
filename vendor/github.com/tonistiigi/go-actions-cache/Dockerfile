# syntax=docker/dockerfile:1

ARG GO_VERSION=1.24
ARG XX_VERSION=1.6.1

FROM --platform=$BUILDPLATFORM tonistiigi/xx:${XX_VERSION} AS xx

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS base
RUN apk add --no-cache git openssl
COPY --from=xx / /
WORKDIR /src

FROM base AS test
ARG TESTFLAGS
ARG GITHUB_REPOSITORY
ARG ACTIONS_CACHE_URL
ARG ACTIONS_CACHE_API_FORCE_VERSION
ARG ACTIONS_CACHE_SERVICE_V2
ARG ACTIONS_RESULTS_URL
RUN --mount=target=. \
    --mount=target=/go/pkg/mod,type=cache \
    --mount=target=/root/.cache,type=cache \
    --mount=type=secret,id=GITHUB_TOKEN,env=GITHUB_TOKEN \
    --mount=type=secret,id=ACTIONS_RUNTIME_TOKEN,env=ACTIONS_RUNTIME_TOKEN \
    CGO_ENABLED=0 xx-go test -v -coverprofile=/tmp/coverage.txt -covermode=atomic ${TESTFLAGS} ./...

FROM scratch AS test-coverage
COPY --from=test /tmp/coverage.txt /
