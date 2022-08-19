# syntax=docker/dockerfile:1

# ubuntu base is only used for riscv64 builds
# we also need to keep debian to be able to build for armel
ARG DEBIAN_BASE="debian:bullseye"
ARG UBUNTU_BASE="ubuntu:22.04"

ARG DEBIAN_FRONTEND=noninteractive
ARG APT_MIRROR=deb.debian.org
ARG DOCKER_LINKMODE=static
ARG CROSS="false"
ARG SYSTEMD="false"

## build deps
ARG GO_VERSION=1.18.5
ARG TINI_VERSION=v0.19.0

## extra tools
ARG CONTAINERD_VERSION=v1.6.7
ARG RUNC_VERSION=v1.1.3
ARG VPNKIT_VERSION=0.5.0

## dev deps
# XX_VERSION specifies the version of xx, an helper for cross-compilation.
ARG XX_VERSION=1.1.2
ARG SKOPEO_VERSION=v1.9.0
ARG CRIU_VERSION=v3.16.1

# cross compilation helper
FROM --platform=$BUILDPLATFORM tonistiigi/xx:${XX_VERSION} AS xx

# dummy stage to make sure the image is built for unsupported deps
FROM --platform=$BUILDPLATFORM busybox AS build-dummy
RUN mkdir -p /out
FROM scratch AS binary-dummy
COPY --from=build-dummy /out /out

# go base image to retrieve /usr/local/go
FROM golang:${GO_VERSION} AS golang

# base
FROM ${UBUNTU_BASE} AS base-ubuntu
FROM ${DEBIAN_BASE} AS base-debian
FROM base-debian AS base-windows
FROM base-debian AS base-linux-amd64
FROM base-debian AS base-linux-armv5
FROM base-debian AS base-linux-armv6
FROM base-debian AS base-linux-armv7
FROM base-debian AS base-linux-arm64
FROM base-debian AS base-linux-ppc64le
FROM base-ubuntu AS base-linux-riscv64
FROM base-debian AS base-linux-s390x

FROM base-linux-${TARGETARCH}${TARGETVARIANT} AS base-linux
FROM base-${TARGETOS} AS base
COPY --from=xx / /
RUN echo 'Binary::apt::APT::Keep-Downloaded-Packages "true";' > /etc/apt/apt.conf.d/keep-cache
ARG APT_MIRROR
RUN sed -ri "s/(httpredir|deb).debian.org/${APT_MIRROR:-deb.debian.org}/g" /etc/apt/sources.list \
 && sed -ri "s/(security).debian.org/${APT_MIRROR:-security.debian.org}/g" /etc/apt/sources.list
ENV GO111MODULE=off
ARG DEBIAN_FRONTEND
RUN --mount=type=cache,sharing=locked,id=moby-base-aptlib,target=/var/lib/apt \
    --mount=type=cache,sharing=locked,id=moby-base-aptcache,target=/var/cache/apt \
    apt-get update && apt-get install --no-install-recommends -y \
      bash \
      ca-certificates \
      cmake \
      curl \
      file \
      gcc \
      git \
      libc6-dev \
      lld \
      make \
      pkg-config
COPY --from=golang /usr/local/go /usr/local/go
ENV GOROOT="/usr/local/go"
ENV GOPATH="/go"
ENV PATH="$GOPATH/bin:/usr/local/go/bin:$PATH"
RUN mkdir -p "$GOPATH/src" "$GOPATH/bin" && chmod -R 777 "$GOPATH"

# criu
FROM base AS criu-src
WORKDIR /usr/src/criu
RUN git init . && git remote add origin "https://github.com/checkpoint-restore/criu.git"
ARG CRIU_VERSION
RUN git fetch --depth 1 origin "${CRIU_VERSION}" && git checkout -q FETCH_HEAD

FROM base AS criu
WORKDIR /go/src/github.com/checkpoint-restore/criu
ARG DEBIAN_FRONTEND
RUN --mount=type=cache,sharing=locked,id=moby-criu-aptlib,target=/var/lib/apt \
    --mount=type=cache,sharing=locked,id=moby-criu-aptcache,target=/var/cache/apt \
    apt-get update && apt-get install -y --no-install-recommends \
      clang \
      gcc \
      libc6-dev \
      libcap-dev \
      libnet1-dev \
      libnl-3-dev \
      libprotobuf-dev \
      libprotobuf-c-dev \
      protobuf-c-compiler \
      protobuf-compiler \
      python3-protobuf
RUN --mount=from=criu-src,src=/usr/src/criu,rw \
    --mount=type=cache,target=/root/.cache <<EOT
  set -e
  make
  xx-verify ./criu/criu
  mkdir /out
  mv ./criu/criu /out/
EOT

FROM base AS registry
WORKDIR /go/src/github.com/docker/distribution

# REGISTRY_VERSION specifies the version of the registry to build and install
# from the https://github.com/docker/distribution repository. This version of
# the registry is used to test both schema 1 and schema 2 manifests. Generally,
# the version specified here should match a current release.
ARG REGISTRY_VERSION=v2.3.0

# REGISTRY_VERSION_SCHEMA1 specifies the version of the registry to build and
# install from the https://github.com/docker/distribution repository. This is
# an older (pre v2.3.0) version of the registry that only supports schema1
# manifests. This version of the registry is not working on arm64, so installation
# is skipped on that architecture.
ARG REGISTRY_VERSION_SCHEMA1=v2.1.0
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=tmpfs,target=/go/src/ \
        set -x \
        && git clone https://github.com/docker/distribution.git . \
        && git checkout -q "$REGISTRY_VERSION" \
        && GOPATH="/go/src/github.com/docker/distribution/Godeps/_workspace:$GOPATH" \
           go build -buildmode=pie -o /build/registry-v2 github.com/docker/distribution/cmd/registry \
        && case $(dpkg --print-architecture) in \
               amd64|armhf|ppc64*|s390x) \
               git checkout -q "$REGISTRY_VERSION_SCHEMA1"; \
               GOPATH="/go/src/github.com/docker/distribution/Godeps/_workspace:$GOPATH"; \
                   go build -buildmode=pie -o /build/registry-v2-schema1 github.com/docker/distribution/cmd/registry; \
                ;; \
           esac

FROM base AS swagger
WORKDIR $GOPATH/src/github.com/go-swagger/go-swagger

# GO_SWAGGER_COMMIT specifies the version of the go-swagger binary to build and
# install. Go-swagger is used in CI for validating swagger.yaml in hack/validate/swagger-gen
#
# Currently uses a fork from https://github.com/kolyshkin/go-swagger/tree/golang-1.13-fix,
# TODO: move to under moby/ or fix upstream go-swagger to work for us.
ENV GO_SWAGGER_COMMIT c56166c036004ba7a3a321e5951ba472b9ae298c
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=tmpfs,target=/go/src/ \
        set -x \
        && git clone https://github.com/kolyshkin/go-swagger.git . \
        && git checkout -q "$GO_SWAGGER_COMMIT" \
        && go build -o /build/swagger github.com/go-swagger/go-swagger/cmd/swagger

# skopeo is used by frozen-images stage
FROM base AS skopeo
ARG SKOPEO_VERSION
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
      GO111MODULE=on CGO_ENABLED=0 GOBIN=/out go install -tags "exclude_graphdriver_devicemapper exclude_graphdriver_btrfs containers_image_openpgp" "github.com/containers/skopeo/cmd/skopeo@${SKOPEO_VERSION}" \
      && /out/skopeo --version

# frozen-images gets useful and necessary Hub images so we can "docker load"
# locally instead of pulling. See also frozenImages in
# "testutil/environment/protect.go" (which needs to be updated when adding images to this list)
FROM base AS frozen-images
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
# OS, ARCH, VARIANT are used by skopeo cli
ENV OS=$TARGETOS
ENV ARCH=$TARGETARCH
ENV VARIANT=$TARGETVARIANT
RUN --mount=from=skopeo,source=/out/skopeo,target=/usr/bin/skopeo <<EOT
  set -e
  mkdir /out
  skopeo --insecure-policy copy docker://busybox@sha256:95cf004f559831017cdf4628aaf1bb30133677be8702a8c5f2994629f637a209 --additional-tag busybox:latest docker-archive:///out/busybox-latest.tar
  skopeo --insecure-policy copy docker://busybox@sha256:1f81263701cddf6402afe9f33fca0266d9fff379e59b1748f33d3072da71ee85 --additional-tag busybox:glibc docker-archive:///out/busybox-glibc.tar
  skopeo --insecure-policy copy docker://debian@sha256:dacf278785a4daa9de07596ec739dbc07131e189942772210709c5c0777e8437 --additional-tag debian:bullseye-slim docker-archive:///out/debian-bullseye-slim.tar
  skopeo --insecure-policy copy docker://hello-world@sha256:d58e752213a51785838f9eed2b7a498ffa1cb3aa7f946dda11af39286c3db9a9 --additional-tag hello-world:latest docker-archive:///out/hello-world-latest.tar
  skopeo --insecure-policy --override-os linux --override-arch arm --override-variant v7 copy docker://arm32v7/hello-world@sha256:50b8560ad574c779908da71f7ce370c0a2471c098d44d1c8f6b513c5a55eeeb1 --additional-tag arm32v7/hello-world:latest docker-archive:///out/arm32v7-hello-world-latest.tar
EOT

FROM base AS cross-false

FROM --platform=linux/amd64 base AS cross-true
ARG DEBIAN_FRONTEND
RUN dpkg --add-architecture arm64
RUN dpkg --add-architecture armel
RUN dpkg --add-architecture armhf
RUN dpkg --add-architecture ppc64el
RUN dpkg --add-architecture s390x
RUN --mount=type=cache,sharing=locked,id=moby-cross-true-aptlib,target=/var/lib/apt \
    --mount=type=cache,sharing=locked,id=moby-cross-true-aptcache,target=/var/cache/apt \
        apt-get update && apt-get install -y --no-install-recommends \
            crossbuild-essential-arm64 \
            crossbuild-essential-armel \
            crossbuild-essential-armhf \
            crossbuild-essential-ppc64el \
            crossbuild-essential-s390x

FROM cross-${CROSS} AS dev-base

FROM dev-base AS runtime-dev-cross-false
ARG DEBIAN_FRONTEND
RUN --mount=type=cache,sharing=locked,id=moby-cross-false-aptlib,target=/var/lib/apt \
    --mount=type=cache,sharing=locked,id=moby-cross-false-aptcache,target=/var/cache/apt \
        apt-get update && apt-get install -y --no-install-recommends \
            binutils-mingw-w64 \
            g++-mingw-w64-x86-64 \
            libapparmor-dev \
            libbtrfs-dev \
            libdevmapper-dev \
            libseccomp-dev \
            libsystemd-dev \
            libudev-dev

FROM --platform=linux/amd64 runtime-dev-cross-false AS runtime-dev-cross-true
ARG DEBIAN_FRONTEND
# These crossbuild packages rely on gcc-<arch>, but this doesn't want to install
# on non-amd64 systems, so other architectures cannot crossbuild amd64.
RUN --mount=type=cache,sharing=locked,id=moby-cross-true-aptlib,target=/var/lib/apt \
    --mount=type=cache,sharing=locked,id=moby-cross-true-aptcache,target=/var/cache/apt \
        apt-get update && apt-get install -y --no-install-recommends \
            libapparmor-dev:arm64 \
            libapparmor-dev:armel \
            libapparmor-dev:armhf \
            libapparmor-dev:ppc64el \
            libapparmor-dev:s390x \
            libseccomp-dev:arm64 \
            libseccomp-dev:armel \
            libseccomp-dev:armhf \
            libseccomp-dev:ppc64el \
            libseccomp-dev:s390x

FROM runtime-dev-cross-${CROSS} AS runtime-dev

FROM base AS delve
# DELVE_VERSION specifies the version of the Delve debugger binary
# from the https://github.com/go-delve/delve repository.
# It can be used to run Docker with a possibility of
# attaching debugger to it.
#
ARG DELVE_VERSION=v1.8.1
# Delve on Linux is currently only supported on amd64 and arm64;
# https://github.com/go-delve/delve/blob/v1.8.1/pkg/proc/native/support_sentinel.go#L1-L6
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
        case $(dpkg --print-architecture) in \
            amd64|arm64) \
                GOBIN=/build/ GO111MODULE=on go install "github.com/go-delve/delve/cmd/dlv@${DELVE_VERSION}" \
                && /build/dlv --help \
                ;; \
            *) \
                mkdir -p /build/ \
                ;; \
        esac

FROM base AS tomll
# GOTOML_VERSION specifies the version of the tomll binary to build and install
# from the https://github.com/pelletier/go-toml repository. This binary is used
# in CI in the hack/validate/toml script.
#
# When updating this version, consider updating the github.com/pelletier/go-toml
# dependency in vendor.mod accordingly.
ARG GOTOML_VERSION=v1.8.1
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
        GOBIN=/build/ GO111MODULE=on go install "github.com/pelletier/go-toml/cmd/tomll@${GOTOML_VERSION}" \
     && /build/tomll --help

FROM base AS gowinres
# GOWINRES_VERSION defines go-winres tool version
ARG GOWINRES_VERSION=v0.2.3
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
        GOBIN=/build/ GO111MODULE=on go install "github.com/tc-hib/go-winres@${GOWINRES_VERSION}" \
     && /build/go-winres --help

# containerd
FROM base AS containerd-src
WORKDIR /usr/src/containerd
RUN git init . && git remote add origin "https://github.com/containerd/containerd.git"
ARG CONTAINERD_VERSION
RUN git fetch --depth 1 origin "${CONTAINERD_VERSION}" && git checkout -q FETCH_HEAD

FROM base AS containerd-build
WORKDIR /go/src/github.com/containerd/containerd
ENV GO111MODULE=off
ARG DEBIAN_FRONTEND
ARG TARGETPLATFORM
RUN --mount=type=cache,sharing=locked,id=moby-containerd-aptlib,target=/var/lib/apt \
    --mount=type=cache,sharing=locked,id=moby-containerd-aptcache,target=/var/cache/apt \
    xx-apt-get update && xx-apt-get install -y \
      binutils \
      g++ \
      gcc \
      libbtrfs-dev \
      libsecret-1-dev \
      pkg-config \
    && xx-go --wrap
ARG DOCKER_LINKMODE
RUN --mount=from=containerd-src,src=/usr/src/containerd,rw \
    --mount=type=cache,target=/root/.cache <<EOT
  set -e
  if [ "$DOCKER_LINKMODE" = "static" ]; then
    export CGO_ENABLED=1
    export BUILDTAGS="netgo osusergo static_build"
    export EXTRA_LDFLAGS='-extldflags "-fno-PIC -static"'
  fi
  export CC=$(xx-info)-gcc
  make bin/containerd
  xx-verify $([ "$DOCKER_LINKMODE" = "static" ] && echo "--static") bin/containerd
  make bin/containerd-shim-runc-v2
  xx-verify $([ "$DOCKER_LINKMODE" = "static" ] && echo "--static") bin/containerd-shim-runc-v2
  make bin/ctr
  # FIXME: ctr not statically linked: https://github.com/containerd/containerd/issues/5824
  xx-verify bin/ctr
  mv bin /out
EOT

FROM binary-dummy AS containerd-darwin
FROM binary-dummy AS containerd-freebsd
FROM containerd-build AS containerd-linux
FROM binary-dummy AS containerd-windows
FROM containerd-${TARGETOS} AS containerd

FROM base AS golangci_lint
ARG GOLANGCI_LINT_VERSION=v1.46.2
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
        GOBIN=/build/ GO111MODULE=on go install "github.com/golangci/golangci-lint/cmd/golangci-lint@${GOLANGCI_LINT_VERSION}" \
     && /build/golangci-lint --version

FROM base AS gotestsum
ARG GOTESTSUM_VERSION=v1.8.1
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
        GOBIN=/build/ GO111MODULE=on go install "gotest.tools/gotestsum@${GOTESTSUM_VERSION}" \
     && /build/gotestsum --version

FROM base AS shfmt
ARG SHFMT_VERSION=v3.0.2
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
        GOBIN=/build/ GO111MODULE=on go install "mvdan.cc/sh/v3/cmd/shfmt@${SHFMT_VERSION}" \
     && /build/shfmt --version

FROM dev-base AS dockercli
ARG DOCKERCLI_CHANNEL
ARG DOCKERCLI_VERSION
COPY /hack/dockerfile/install/install.sh /hack/dockerfile/install/dockercli.installer /
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
        PREFIX=/build /install.sh dockercli

# runc
FROM base AS runc-src
WORKDIR /usr/src/runc
RUN git init . && git remote add origin "https://github.com/opencontainers/runc.git"
ARG RUNC_VERSION
RUN git fetch --depth 1 origin "${RUNC_VERSION}" && git checkout -q FETCH_HEAD

FROM base AS runc-build
WORKDIR /go/src/github.com/opencontainers/runc
ARG DEBIAN_FRONTEND
ARG TARGETPLATFORM
RUN --mount=type=cache,sharing=locked,id=moby-runc-aptlib,target=/var/lib/apt \
    --mount=type=cache,sharing=locked,id=moby-runc-aptcache,target=/var/cache/apt \
    xx-apt-get update && xx-apt-get install -y \
      binutils \
      g++ \
      gcc \
      dpkg-dev \
      libseccomp-dev \
      pkg-config \
    && xx-go --wrap
ENV CGO_ENABLED=1
ARG DOCKER_LINKMODE
# FIXME: should be built using clang but needs https://github.com/opencontainers/runc/pull/3465
RUN --mount=from=runc-src,src=/usr/src/runc,rw \
    --mount=type=cache,target=/root/.cache <<EOT
  set -e
  make BUILDTAGS="seccomp" "$([ "$DOCKER_LINKMODE" = "static" ] && echo "static" || echo "runc")"
  xx-verify $([ "$DOCKER_LINKMODE" = "static" ] && echo "--static") runc
  mkdir /out
  mv runc /out/
EOT

FROM binary-dummy AS runc-darwin
FROM binary-dummy AS runc-freebsd
FROM runc-build AS runc-linux
FROM binary-dummy AS runc-windows
FROM runc-${TARGETOS} AS runc

# tini (docker-init)
FROM base AS tini-src
WORKDIR /usr/src/tini
RUN git init . && git remote add origin "https://github.com/krallin/tini.git"
ARG TINI_VERSION
RUN git fetch --depth 1 origin "${TINI_VERSION}" && git checkout -q FETCH_HEAD

FROM base AS tini-build
ENV GO111MODULE=off
WORKDIR /go/src/github.com/krallin/tini
ARG DEBIAN_FRONTEND
ARG TARGETPLATFORM
RUN --mount=type=cache,sharing=locked,id=moby-tini-aptlib,target=/var/lib/apt \
    --mount=type=cache,sharing=locked,id=moby-tini-aptcache,target=/var/cache/apt \
    xx-apt-get update && xx-apt-get install -y \
      gcc \
      libc6-dev
ARG DOCKER_LINKMODE
RUN --mount=from=tini-src,src=/usr/src/tini,rw \
    --mount=type=cache,target=/root/.cache <<EOT
  set -e
  export TINI_TARGET=$([ "$DOCKER_LINKMODE" = "static" ] && echo "tini-static" || echo "tini")
  CC=$(xx-info)-gcc cmake .
  make "$TINI_TARGET"
  xx-verify $([ "$DOCKER_LINKMODE" = "static" ] && echo "--static") "$TINI_TARGET"
  mkdir /out
  mv "$TINI_TARGET" /out/docker-init
EOT

FROM binary-dummy AS tini-darwin
FROM binary-dummy AS tini-freebsd
FROM tini-build AS tini-linux
FROM binary-dummy AS tini-windows
FROM tini-${TARGETOS} AS tini

FROM dev-base AS rootlesskit
ARG ROOTLESSKIT_VERSION
ARG PREFIX=/build
COPY /hack/dockerfile/install/install.sh /hack/dockerfile/install/rootlesskit.installer /
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
        /install.sh rootlesskit \
     && "${PREFIX}"/rootlesskit --version \
     && "${PREFIX}"/rootlesskit-docker-proxy --help
COPY ./contrib/dockerd-rootless.sh /build
COPY ./contrib/dockerd-rootless-setuptool.sh /build

FROM base AS crun
ARG CRUN_VERSION=1.4.5
RUN --mount=type=cache,sharing=locked,id=moby-crun-aptlib,target=/var/lib/apt \
    --mount=type=cache,sharing=locked,id=moby-crun-aptcache,target=/var/cache/apt \
        apt-get update && apt-get install -y --no-install-recommends \
            autoconf \
            automake \
            build-essential \
            libcap-dev \
            libprotobuf-c-dev \
            libseccomp-dev \
            libsystemd-dev \
            libtool \
            libudev-dev \
            libyajl-dev \
            python3 \
            ;
RUN --mount=type=tmpfs,target=/tmp/crun-build \
    git clone https://github.com/containers/crun.git /tmp/crun-build && \
    cd /tmp/crun-build && \
    git checkout -q "${CRUN_VERSION}" && \
    ./autogen.sh && \
    ./configure --bindir=/build && \
    make -j install

# vpnkit
# TODO: build from source instead
FROM scratch AS vpnkit-windows
FROM scratch AS vpnkit-linux-386
FROM djs55/vpnkit:${VPNKIT_VERSION} AS vpnkit-linux-amd64
FROM scratch AS vpnkit-linux-arm
FROM djs55/vpnkit:${VPNKIT_VERSION} AS vpnkit-linux-arm64
FROM scratch AS vpnkit-linux-ppc64le
FROM scratch AS vpnkit-linux-riscv64
FROM scratch AS vpnkit-linux-s390x
FROM vpnkit-linux-${TARGETARCH} AS vpnkit-linux
FROM vpnkit-${TARGETOS} AS vpnkit

# TODO: Some of this is only really needed for testing, it would be nice to split this up
FROM runtime-dev AS dev-systemd-false
ARG DEBIAN_FRONTEND
RUN groupadd -r docker
RUN useradd --create-home --gid docker unprivilegeduser \
 && mkdir -p /home/unprivilegeduser/.local/share/docker \
 && chown -R unprivilegeduser /home/unprivilegeduser
# Let us use a .bashrc file
RUN ln -sfv /go/src/github.com/docker/docker/.bashrc ~/.bashrc
# Activate bash completion and include Docker's completion if mounted with DOCKER_BASH_COMPLETION_PATH
RUN echo "source /usr/share/bash-completion/bash_completion" >> /etc/bash.bashrc
RUN ln -s /usr/local/completion/bash/docker /etc/bash_completion.d/docker
RUN ldconfig
# This should only install packages that are specifically needed for the dev environment and nothing else
# Do you really need to add another package here? Can it be done in a different build stage?
RUN --mount=type=cache,sharing=locked,id=moby-dev-aptlib,target=/var/lib/apt \
    --mount=type=cache,sharing=locked,id=moby-dev-aptcache,target=/var/cache/apt \
        apt-get update && apt-get install -y --no-install-recommends \
            apparmor \
            bash-completion \
            bzip2 \
            inetutils-ping \
            iproute2 \
            iptables \
            jq \
            libcap2-bin \
            libnet1 \
            libnl-3-200 \
            libprotobuf-c1 \
            libyajl2 \
            net-tools \
            patch \
            pigz \
            python3-pip \
            python3-setuptools \
            python3-wheel \
            sudo \
            systemd-journal-remote \
            thin-provisioning-tools \
            uidmap \
            vim \
            vim-common \
            xfsprogs \
            xz-utils \
            zip \
            zstd


# Switch to use iptables instead of nftables (to match the CI hosts)
# TODO use some kind of runtime auto-detection instead if/when nftables is supported (https://github.com/moby/moby/issues/26824)
RUN update-alternatives --set iptables  /usr/sbin/iptables-legacy  || true \
 && update-alternatives --set ip6tables /usr/sbin/ip6tables-legacy || true \
 && update-alternatives --set arptables /usr/sbin/arptables-legacy || true

RUN pip3 install yamllint==1.26.1

COPY --from=dockercli     /build/ /usr/local/cli
COPY --from=frozen-images /out/   /docker-frozen-images
COPY --from=swagger       /build/ /usr/local/bin/
COPY --from=delve         /build/ /usr/local/bin/
COPY --from=tomll         /build/ /usr/local/bin/
COPY --from=gowinres      /build/ /usr/local/bin/
COPY --from=tini          /out/   /usr/local/bin/
COPY --from=registry      /build/ /usr/local/bin/
COPY --from=criu          /out/   /usr/local/bin/
COPY --from=gotestsum     /build/ /usr/local/bin/
COPY --from=golangci_lint /build/ /usr/local/bin/
COPY --from=shfmt         /build/ /usr/local/bin/
COPY --from=runc          /out/   /usr/local/bin/
COPY --from=containerd    /out/   /usr/local/bin/
COPY --from=rootlesskit   /build/ /usr/local/bin/
COPY --from=vpnkit        /       /usr/local/bin/
COPY --from=crun          /build/ /usr/local/bin/
COPY hack/dockerfile/etc/docker/  /etc/docker/
ENV PATH=/usr/local/cli:$PATH
ARG DOCKER_BUILDTAGS
ENV DOCKER_BUILDTAGS="${DOCKER_BUILDTAGS}"
WORKDIR /go/src/github.com/docker/docker
VOLUME /var/lib/docker
VOLUME /home/unprivilegeduser/.local/share/docker
# Wrap all commands in the "docker-in-docker" script to allow nested containers
ENTRYPOINT ["hack/dind"]

FROM dev-systemd-false AS dev-systemd-true
RUN --mount=type=cache,sharing=locked,id=moby-dev-aptlib,target=/var/lib/apt \
    --mount=type=cache,sharing=locked,id=moby-dev-aptcache,target=/var/cache/apt \
        apt-get update && apt-get install -y --no-install-recommends \
            dbus \
            dbus-user-session \
            systemd \
            systemd-sysv
ENTRYPOINT ["hack/dind-systemd"]

FROM dev-systemd-${SYSTEMD} AS dev

FROM runtime-dev AS binary-base
ARG DOCKER_GITCOMMIT=HEAD
ENV DOCKER_GITCOMMIT=${DOCKER_GITCOMMIT}
ARG VERSION
ENV VERSION=${VERSION}
ARG PLATFORM
ENV PLATFORM=${PLATFORM}
ARG PRODUCT
ENV PRODUCT=${PRODUCT}
ARG DEFAULT_PRODUCT_LICENSE
ENV DEFAULT_PRODUCT_LICENSE=${DEFAULT_PRODUCT_LICENSE}
ARG PACKAGER_NAME
ENV PACKAGER_NAME=${PACKAGER_NAME}
ARG DOCKER_BUILDTAGS
ENV DOCKER_BUILDTAGS="${DOCKER_BUILDTAGS}"
ENV PREFIX=/build
# TODO: This is here because hack/make.sh binary copies these extras binaries
# from $PATH into the bundles dir.
# It would be nice to handle this in a different way.
COPY --from=tini          /out/   /usr/local/bin/
COPY --from=runc          /out/   /usr/local/bin/
COPY --from=containerd    /out/   /usr/local/bin/
COPY --from=rootlesskit   /build/ /usr/local/bin/
COPY --from=vpnkit        /       /usr/local/bin/
COPY --from=gowinres      /build/ /usr/local/bin/
WORKDIR /go/src/github.com/docker/docker

FROM binary-base AS build-binary
RUN --mount=type=cache,target=/root/.cache \
    --mount=type=bind,target=.,ro \
    --mount=type=tmpfs,target=cli/winresources/dockerd \
    --mount=type=tmpfs,target=cli/winresources/docker-proxy \
        hack/make.sh binary

FROM binary-base AS build-dynbinary
RUN --mount=type=cache,target=/root/.cache \
    --mount=type=bind,target=.,ro \
    --mount=type=tmpfs,target=cli/winresources/dockerd \
    --mount=type=tmpfs,target=cli/winresources/docker-proxy \
        hack/make.sh dynbinary

FROM binary-base AS build-cross
ARG DOCKER_CROSSPLATFORMS
RUN --mount=type=cache,target=/root/.cache \
    --mount=type=bind,target=.,ro \
    --mount=type=tmpfs,target=cli/winresources/dockerd \
    --mount=type=tmpfs,target=cli/winresources/docker-proxy \
        hack/make.sh cross

FROM scratch AS binary
COPY --from=build-binary /build/bundles/ /

FROM scratch AS dynbinary
COPY --from=build-dynbinary /build/bundles/ /

FROM scratch AS cross
COPY --from=build-cross /build/bundles/ /

FROM dev AS final
COPY . /go/src/github.com/docker/docker
