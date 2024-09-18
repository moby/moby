#syntax=docker/dockerfile:1.8
#check=error=true

ARG GO_VERSION=1.22
ARG XX_VERSION=1.4.0

ARG COVER_FILENAME="cover.out"
ARG BENCH_FILENAME="bench.txt"

FROM --platform=${BUILDPLATFORM} tonistiigi/xx:${XX_VERSION} AS xx

FROM --platform=${BUILDPLATFORM} golang:${GO_VERSION}-alpine AS golang
COPY --link --from=xx / /
WORKDIR /src
ARG TARGETPLATFORM

FROM golang AS build
RUN --mount=target=/root/.cache,type=cache \
    --mount=type=bind xx-go build .

FROM golang AS runbench
ARG BENCH_FILENAME
RUN --mount=target=/root/.cache,type=cache \
    --mount=type=bind \
    xx-go test -v --run skip --bench . | tee /tmp/${BENCH_FILENAME}

FROM scratch AS bench
ARG BENCH_FILENAME
COPY --from=runbench /tmp/${BENCH_FILENAME} /

FROM golang AS runtest
ARG TESTFLAGS="-v"
ARG COVER_FILENAME
RUN --mount=target=/root/.cache,type=cache \
    --mount=type=bind \
    xx-go test -coverprofile=/tmp/${COVER_FILENAME} $TESTFLAGS .

FROM scratch AS test
ARG COVER_FILENAME
COPY --from=runtest /tmp/${COVER_FILENAME} /

FROM build
