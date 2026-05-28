ARG GO_VERSION=1.21

FROM --platform=${BUILDPLATFORM} golang:${GO_VERSION} AS build
COPY . /go/src/github.com/cpuguy83/go-md2man
WORKDIR /go/src/github.com/cpuguy83/go-md2man
ARG TARGETOS TARGETARCH TARGETVARIANT
RUN \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    make build

FROM scratch
COPY --from=build /go/src/github.com/cpuguy83/go-md2man/bin/go-md2man /go-md2man
ENTRYPOINT ["/go-md2man"]
