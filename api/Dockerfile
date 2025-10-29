ARG GO_VERSION=1.25

FROM golang:${GO_VERSION}-alpine AS base

RUN apk add --no-cache bash make

# Swagger config stage
FROM base AS swagger
WORKDIR /go/src/github.com/go-swagger/go-swagger

ARG GO_SWAGGER_VERSION=v0.33.1
ARG TARGETPLATFORM

RUN --mount=type=cache,target=/root/.cache/go-build,id=swagger-build-$TARGETPLATFORM \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=tmpfs,target=/go/src/ <<EOT
  set -e
  GOBIN=/build CGO_ENABLED=0 go install "github.com/go-swagger/go-swagger/cmd/swagger@${GO_SWAGGER_VERSION}"
EOT

# Execution stage
FROM base AS dev

COPY --from=swagger /build/swagger /usr/local/bin/swagger

RUN swagger version

ENV GOPATH=/go

WORKDIR /go/src/github.com/docker/docker/api

CMD ["/bin/bash"]