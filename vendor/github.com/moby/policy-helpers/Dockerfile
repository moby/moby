# syntax=docker/dockerfile:1.19-labs

ARG ALPINE_VERSION=3.22
ARG ROOT_SIGNING_VERSION=main
ARG GOLANG_VERSION=1.25
ARG XX_VERSION=1.8.0
ARG DOCKER_HARDENED_IMAGES_KEYRING_VERSION=main

FROM scratch AS sigstore-root-signing
ARG ROOT_SIGNING_VERSION
ADD https://www.github.com/sigstore/root-signing.git#${ROOT_SIGNING_VERSION} /

FROM scratch AS tuf-root
COPY --from=sigstore-root-signing metadata/root.json metadata/snapshot.json metadata/timestamp.json metadata/targets.json /
COPY --parents --from=sigstore-root-signing targets/trusted_root.json /

FROM alpine:${ALPINE_VERSION} AS validate-tuf-root
RUN --mount=type=bind,from=tuf-root,target=/a \
    --mount=type=bind,source=roots/tuf-root,target=/b \
    diff -ruN /a /b

FROM --platform=$BUILDPLATFORM tonistiigi/xx:${XX_VERSION} AS xx

FROM scratch AS dhi-keyring
ARG DOCKER_HARDENED_IMAGES_KEYRING_VERSION
ADD https://www.github.com/docker-hardened-images/keyring.git#${DOCKER_HARDENED_IMAGES_KEYRING_VERSION} /

FROM scratch AS dhi-pubkey
COPY --from=dhi-keyring /publickey/dhi-latest.pub /dhi.pub

FROM alpine:${ALPINE_VERSION} AS validate-dhi-pubkey
RUN --mount=type=bind,from=dhi-pubkey,target=/a \
    --mount=type=bind,source=roots/dhi,target=/b \
    diff -u /a/dhi-latest.pub /b/dhi.pub

FROM --platform=$BUILDPLATFORM golang:${GOLANG_VERSION}-alpine${ALPINE_VERSION} AS build
COPY --from=xx / /
WORKDIR /go/src/github.com/moby/policy-helpers
ARG TARGETPLATFORM
RUN --mount=target=. xx-go build -o /out/policy-helper ./cmd/policy-helper

FROM scratch AS binary
COPY --from=build /out/policy-helper /
