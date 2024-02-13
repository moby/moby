#syntax=docker/dockerfile:1
ARG GO_VERSION=1.20

FROM --platform=$BUILDPLATFORM tonistiigi/xx:1.1.0 AS xx

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
    CGO_ENABLED=0 xx-go test -test.v ${TESTFLAGS} ./...

FROM base AS test-noroot
RUN mkdir /go/pkg && chmod 0777 /go/pkg
USER 1000:1000
RUN --mount=target=. \
    --mount=target=/tmp/.cache,type=cache \
    CGO_ENABLED=0 GOCACHE=/tmp/gocache xx-go test -test.v ./...

FROM build
