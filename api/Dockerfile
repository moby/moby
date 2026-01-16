# syntax=docker/dockerfile:1

ARG GO_VERSION=1.25.6

FROM golang:${GO_VERSION}-alpine AS base
RUN apk add --no-cache bash make yamllint
CMD ["/bin/bash"]

# go-swagger
FROM base AS swagger
WORKDIR /go/src/github.com/go-swagger/go-swagger
# GO_SWAGGER_VERSION specifies the version of the go-swagger binary to install.
# Go-swagger is used in CI for generating types from swagger.yaml in
# api/scripts/generate-swagger-api.sh
ARG GO_SWAGGER_VERSION=v0.33.1
ARG TARGETPLATFORM
RUN --mount=type=cache,target=/root/.cache/go-build,id=swagger-build-$TARGETPLATFORM \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 go install "github.com/go-swagger/go-swagger/cmd/swagger@${GO_SWAGGER_VERSION}" && \
    /go/bin/swagger version

# dev is a dev-environment to work with the api module.
FROM base AS dev
COPY --from=swagger /go/bin/swagger /usr/local/bin/swagger
WORKDIR /go/src/github.com/moby/moby/api
