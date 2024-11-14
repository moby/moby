# syntax=docker/dockerfile:1

ARG GO_VERSION=1.23
ARG XX_VERSION=1.6.1

FROM --platform=$BUILDPLATFORM tonistiigi/xx:${XX_VERSION} AS xx

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS base
RUN apk add --no-cache git
COPY --from=xx / /
WORKDIR /src

FROM base AS build
ARG TARGETPLATFORM
RUN --mount=target=. --mount=target=/go/pkg/mod,type=cache \
    --mount=target=/root/.cache,type=cache \
    xx-go build ./...

FROM base AS test
ARG TESTFLAGS
RUN --mount=target=. --mount=target=/go/pkg/mod,type=cache \
    --mount=target=/root/.cache,type=cache \
    CGO_ENABLED=0 xx-go test -v -coverprofile=/tmp/coverage.txt -covermode=atomic ${TESTFLAGS} ./...

FROM base AS test-noroot
RUN mkdir /go/pkg && chmod 0777 /go/pkg
USER 1000:1000
RUN --mount=target=. \
    --mount=target=/tmp/.cache,type=cache \
    CGO_ENABLED=0 GOCACHE=/tmp/gocache xx-go test -v -coverprofile=/tmp/coverage.txt -covermode=atomic ./...

FROM scratch AS test-coverage
COPY --from=test /tmp/coverage.txt /coverage-root.txt

FROM scratch AS test-noroot-coverage
COPY --from=test-noroot /tmp/coverage.txt /coverage-noroot.txt

FROM build
