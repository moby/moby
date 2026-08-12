# syntax=docker/dockerfile:1

# Regenerates the extension framework's generated files: each contract's .proto,
# its protobuf message code, and its transport wiring (wire.gen.go). Drive it
# with `make generate-extensions` / `make validate-generate-extensions`.
#
# There is nothing to install. mobyextgen is a plain Go program in this module
# that emits all three from the Go contract, driving protoc-gen-go's generator as
# a vendored library, so generation needs only the Go toolchain - no protoc, no
# protoc plugins, nothing fetched over the network. `go generate ./extpoints/...
# ./internal/extensions/...` does the same thing directly; this image exists to
# pin the Go version, so CI's drift check is hermetic.
#
# It is separate from generate-files.Dockerfile because that one runs the legacy
# moby generation in GOPATH mode with gogo.

ARG GO_VERSION=1.26.3
ARG BASE_DEBIAN_DISTRO="bookworm"

FROM golang:${GO_VERSION}-${BASE_DEBIAN_DISTRO} AS base
ENV GOTOOLCHAIN=local
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
  find extpoints internal/extensions -type f \
    \( -name '*.pb.go' -o -name 'wire.gen.go' -o -name '*.proto' \) \
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
