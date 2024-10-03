# syntax=docker/dockerfile:1

ARG GO_VERSION=1.22.8
ARG GOVULNCHECK_VERSION=v1.1.3
ARG FORMAT=text

FROM golang:${GO_VERSION}-alpine AS base
WORKDIR /go/src/github.com/docker/docker
RUN apk add --no-cache jq moreutils
ARG GOVULNCHECK_VERSION
RUN --mount=type=cache,target=/root/.cache \
    --mount=type=cache,target=/go/pkg/mod \
    go install golang.org/x/vuln/cmd/govulncheck@$GOVULNCHECK_VERSION

FROM base AS run
ARG FORMAT
RUN --mount=type=bind,target=.,rw <<EOT
  set -ex
  mkdir /out
  ln -s vendor.mod go.mod
  ln -s vendor.sum go.sum
  govulncheck -format ${FORMAT} ./... | tee /out/govulncheck.out
  if [ "${FORMAT}" = "sarif" ]; then
    # Make sure "results" field is defined in SARIF output otherwise GitHub Code Scanning
    # will fail when uploading report with "Invalid SARIF. Missing 'results' array in run."
    # Relates to https://github.com/golang/vuln/blob/ffdef74cc44d7eb71931d8d414c478b966812488/internal/sarif/sarif.go#L69
    jq '(.runs[] | select(.results == null) | .results) |= []' /out/govulncheck.out | tee >(sponge /out/govulncheck.out)
  fi
EOT

FROM scratch AS output
COPY --from=run /out /
