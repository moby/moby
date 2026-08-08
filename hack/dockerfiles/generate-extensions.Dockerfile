# syntax=docker/dockerfile:1

# Regenerates the extension framework's generated files: the per-point .proto
# (emitted Go-first by mobyextgen), the protobuf/gRPC code, and the transport
# wiring (wire.gen.go). It is separate from generate-files.Dockerfile because
# these generators are module-aware and use the modern protobuf toolchain
# (google.golang.org/protobuf + protoc-gen-go-grpc), whereas the legacy moby
# generation runs in GOPATH mode with gogo. Drive it with `make
# generate-extensions` / `make validate-generate-extensions`.

ARG GO_VERSION=1.26.3
ARG BASE_DEBIAN_DISTRO="bookworm"
# protoc is not a Go module and the distro package lags; its version is also
# baked into the generated file headers, so pin an exact release for
# reproducible output. protoc-gen-go tracks the google.golang.org/protobuf
# version in go.mod; protoc-gen-go-grpc is a separate module.
ARG PROTOC_VERSION=21.12
ARG PROTOC_GEN_GO_VERSION=v1.36.11
ARG PROTOC_GEN_GO_GRPC_VERSION=v1.5.1

FROM golang:${GO_VERSION}-${BASE_DEBIAN_DISTRO} AS base
ENV GOTOOLCHAIN=local
RUN apt-get update && apt-get install -y --no-install-recommends unzip

# protoc cannot be `go install`ed, so fetch the pinned release for this build's
# architecture from the protobuf releases.
ARG PROTOC_VERSION TARGETOS TARGETARCH
RUN <<EOT
  set -ex
  arch=$(echo $TARGETARCH | sed -e s/amd64/x86_64/ -e s/arm64/aarch_64/)
  zip="protoc-${PROTOC_VERSION}-${TARGETOS}-${arch}.zip"
  wget -q "https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VERSION}/${zip}"
  unzip -q "$zip" -d /usr/local
EOT

# The protoc plugins are not vendored (nothing in the module imports their
# command packages), so install them standalone at pinned versions. mobyextgen
# is local to the module and run from vendor by the go:generate directives
# themselves.
ARG PROTOC_GEN_GO_VERSION PROTOC_GEN_GO_GRPC_VERSION
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod <<EOT
  set -ex
  go install google.golang.org/protobuf/cmd/protoc-gen-go@${PROTOC_GEN_GO_VERSION}
  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@${PROTOC_GEN_GO_GRPC_VERSION}
EOT
WORKDIR /src

# Run go generate over the extension packages against a throwaway copy of the
# source, then collect just the generated files (preserving their paths) in /out.
FROM base AS regen
RUN --mount=type=bind,target=/src,rw \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod <<EOT
  set -ex
  go generate ./extpoints/... ./internal/extensions/...
  mkdir /out
  find extpoints internal/extensions -type f \( -name '*.pb.go' -o -name 'wire.gen.go' \) \
    -exec cp --parents -t /out {} +
  # Go-first points have a generated .proto; sdkpb's runtime.proto is hand-written
  # (the SDK handshake protocol, not a point), so leave it out.
  find extpoints internal/extensions -type f -name '*.proto' ! -path '*/sdkpb/*' \
    -exec cp --parents -t /out {} +
EOT

# make generate-extensions: write the regenerated files back to the workspace.
FROM scratch AS update
COPY --from=regen /out /

# make validate-generate-extensions: fail if the committed files differ from a
# fresh regeneration. Diff each regenerated file against the one in the source.
FROM base AS validate
RUN --mount=type=bind,target=/src,ro \
    --mount=type=bind,from=regen,source=/out,target=/regen <<EOT
  set -e
  cd /regen
  fail=0
  for f in $(find . -type f); do
    diff -u "/src/$f" "$f" || fail=1
  done
  if [ "$fail" != 0 ]; then
    echo >&2 'ERROR: extension generated files are out of date. Run: make generate-extensions'
    exit 1
  fi
EOT
