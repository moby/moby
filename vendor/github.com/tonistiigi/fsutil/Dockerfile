# syntax=docker/dockerfile:1

ARG GO_VERSION=1.26
ARG ALPINE_VERSION=3.23
ARG XX_VERSION=1.9.0

FROM --platform=$BUILDPLATFORM tonistiigi/xx:${XX_VERSION} AS xx

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine${ALPINE_VERSION} AS base
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

FROM base AS bench-base
WORKDIR /app
RUN --mount=type=bind,source=go.mod,target=/app/go.mod \
    --mount=type=bind,source=go.sum,target=/app/go.sum \
    --mount=type=cache,target=/root/.cache \
    --mount=type=cache,target=/go/pkg/mod <<EOT
  set -ex
  apk add --no-cache rsync
  go install tool github.com/jstemmer/go-junit-report/v2
EOT

FROM bench-base AS bench
WORKDIR /src
ARG BENCH_FILE_SIZE
RUN --mount=target=. \
    --mount=target=/go/pkg/mod,type=cache \
    --mount=target=/root/.cache,type=cache <<EOT
  set -ex
  set -o pipefail
  mkdir -p /tmp/bench-results
  CGO_ENABLED=0 xx-go test -benchmem -bench=. -run=^$ . 2>&1 | tee /tmp/fsutil.log
  go-junit-report -in /tmp/fsutil.log -out /tmp/bench-results/fsutil.junit.xml
  cd bench
  CGO_ENABLED=0 xx-go test -benchmem -bench=. -run=^$ . 2>&1 | tee /tmp/bench.log
  go-junit-report -in /tmp/bench.log -out /tmp/bench-results/bench.junit.xml
EOT

FROM bench-base AS bench-noroot
WORKDIR /src
RUN mkdir -p /go/pkg && chmod 0777 /go/pkg
USER 1000:1000
ARG BENCH_FILE_SIZE
RUN --mount=target=. \
    --mount=target=/tmp/.cache,type=cache <<EOT
  set -ex
  set -o pipefail
  mkdir -p /tmp/bench-results
  CGO_ENABLED=0 GOCACHE=/tmp/gocache xx-go test -bench=. -benchmem -run=^$ . 2>&1 | tee /tmp/fsutil.log
  go-junit-report -in /tmp/fsutil.log -out /tmp/bench-results/fsutil.junit.xml
  cd bench
  CGO_ENABLED=0 GOCACHE=/tmp/gocache xx-go test -bench=. -benchmem -run=^$ . 2>&1 | tee /tmp/bench.log
  go-junit-report -in /tmp/bench.log -out /tmp/bench-results/bench.junit.xml
EOT

FROM scratch AS bench-root-results
COPY --from=bench /tmp/bench-results /bench-root

FROM scratch AS bench-noroot-results
COPY --from=bench-noroot /tmp/bench-results /bench-noroot

FROM build
