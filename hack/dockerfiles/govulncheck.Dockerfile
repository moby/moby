# syntax=docker/dockerfile:1.20.0@sha256:26147acbda4f14c5add9946e2fd2ed543fc402884fd75146bd342a7f6271dc1d

# NOTE: the image digest is not pinned to a specific version,
# as the image is frequently updated with apt security updates.
#
# TODO: consider pinning for better build reproducibility.
# A reproduction build will need executing apt-get with snapshot mode.
ARG GO_VERSION=1.25.4
ARG GOVULNCHECK_VERSION=v1.1.4
ARG GOVULNCHECK_COMMIT=d1f380186385b4f64e00313f31743df8e4b89a77
ARG FORMAT=text

FROM golang:${GO_VERSION}-alpine AS base
WORKDIR /go/src/github.com/moby/moby
RUN apk add --no-cache jq moreutils
ARG GOVULNCHECK_VERSION
ARG GOVULNCHECK_COMMIT
# Checkout and discard the source; we only need it to verify the commit hash.
ADD https://github.com/golang/vuln.git?tag=${GOVULNCHECK_VERSION}&checksum=${GOVULNCHECK_COMMIT} /tmp/discarded
RUN --mount=type=cache,target=/root/.cache \
    --mount=type=cache,target=/go/pkg/mod \
    go install golang.org/x/vuln/cmd/govulncheck@$GOVULNCHECK_COMMIT

FROM base AS run
ARG FORMAT
RUN --mount=type=bind,target=.,rw <<EOT
  set -ex
  mkdir /out
  govulncheck -format ${FORMAT} ./... | tee /out/govulncheck.out
EOT

FROM scratch AS output
COPY --from=run /out /
