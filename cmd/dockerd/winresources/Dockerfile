# syntax=docker/dockerfile:1.20.0@sha256:26147acbda4f14c5add9946e2fd2ed543fc402884fd75146bd342a7f6271dc1d

# NOTE: the image digest is not pinned to a specific version,
# as the image is frequently updated with apt security updates.
#
# TODO: consider pinning for better build reproducibility.
# A reproduction build will need executing apt-get with snapshot mode.
ARG DEBIAN_VERSION=bookworm

# XX_VERSION specifies the version of the xx utility to use.
# It must be a valid tag in the docker.io/tonistiigi/xx image repository.
ARG XX_VERSION=1.7.0@sha256:010d4b66aed389848b0694f91c7aaee9df59a6f20be7f5d12e53663a37bd14e2

# xx is a helper for cross-compilation
FROM --platform=$BUILDPLATFORM tonistiigi/xx:${XX_VERSION} AS xx

FROM --platform=$BUILDPLATFORM debian:${DEBIAN_VERSION}-slim AS build
COPY --link --from=xx / /
ARG TARGETPLATFORM
RUN apt-get update && xx-apt-get --no-install-recommends install -y binutils
WORKDIR /out
RUN --mount=type=bind,target=/winresources \
    x86_64-w64-mingw32-windmc -v /winresources/event_messages.mc \
    && mv MSG00001.bin event_messages.bin

FROM scratch
COPY --from=build /out /
