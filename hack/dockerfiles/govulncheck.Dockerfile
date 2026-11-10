# syntax=docker/dockerfile:1

ARG GO_VERSION=1.27.3

# GOVULNCHECK_VERSION sets the version of govulncheck to install.
# renovate: datasource=github-tags depName=golang/vuln
ARG GOVULNCHECK_VERSION=v1.9.0
ARG FORMAT=text

FROM golang:${GO_VERSION}-alpine AS base
WORKDIR /usr/src/moby
RUN apk add --no-cache jq moreutils
ARG GOVULNCHECK_VERSION
ADD https://github.com/golang/vuln.git?ref=${GOVULNCHECK_VERSION}&keep-git-dir=1 /go/src/golang.org/x/vuln
RUN --mount=type=cache,target=/root/.cache \
    --mount=type=cache,target=/go/pkg/mod \
    cd /go/src/golang.org/x/vuln && \
    go install ./cmd/govulncheck

FROM base AS run
ARG FORMAT
RUN --mount=type=bind,target=.,rw <<EOT
  set -ex
  mkdir /out
  rc=0
  govulncheck -format ${FORMAT} ./... > /out/govulncheck.out || rc=$?
  cat /out/govulncheck.out
  # Exit status 3 means vulnerabilities were found and reported in text mode.
  [ "$rc" -eq 0 ] || [ "$rc" -eq 3 ]
EOT

FROM scratch AS output
COPY --from=run /out /
