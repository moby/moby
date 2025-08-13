# syntax=docker/dockerfile:1

ARG DEBIAN_VERSION=trixie

# XX_VERSION specifies the version of the xx utility to use.
# It must be a valid tag in the docker.io/tonistiigi/xx image repository.
ARG XX_VERSION=1.7.0

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
