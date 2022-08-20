# syntax=docker/dockerfile:1

# ubuntu base is only used for riscv64 builds
# we also need to keep debian to be able to build for armel
ARG DEBIAN_BASE="debian:bullseye"
ARG UBUNTU_BASE="ubuntu:22.04"

ARG DEBIAN_FRONTEND=noninteractive
ARG APT_MIRROR=deb.debian.org
ARG DOCKER_LINKMODE=static
ARG SYSTEMD=false

## build deps
ARG GO_VERSION=1.18.5
ARG TINI_VERSION=v0.19.0
ARG GOWINRES_VERSION=v0.2.3

## extra tools
ARG CONTAINERD_VERSION=v1.6.7
ARG RUNC_VERSION=v1.1.3
ARG ROOTLESSKIT_VERSION=1920341cd41e047834a21007424162a2dc946315
ARG VPNKIT_VERSION=0.5.0
ARG CONTAINERUTILITY_VERSION=aa1ba87e99b68e0113bd27ec26c60b88f9d4ccd9

## dev deps
# XX_VERSION specifies the version of xx, an helper for cross-compilation.
ARG XX_VERSION=1.1.2
# GOSWAGGER_VERSION specifies the version of the go-swagger binary to build and
# install. Go-swagger is used in CI for validating swagger.yaml in
# hack/validate/swagger-gen
ARG GOSWAGGER_VERSION=c56166c036004ba7a3a321e5951ba472b9ae298c
ARG GOLANGCI_LINT_VERSION=v1.46.2
ARG GOTESTSUM_VERSION=v1.8.1
ARG SHFMT_VERSION=v3.0.2
# GOTOML_VERSION specifies the version of the tomll binary. When updating this
# version, consider updating the github.com/pelletier/go-toml dependency in
# vendor.mod accordingly.
ARG GOTOML_VERSION=v1.8.1
# DELVE_VERSION specifies the version of the Delve debugger binary
# from the https://github.com/go-delve/delve repository.
# It can be used to run Docker with a possibility of
# attaching debugger to it.
ARG DELVE_VERSION=v1.8.1
ARG SKOPEO_VERSION=v1.9.0
ARG CRIU_VERSION=v3.16.1
ARG CRUN_VERSION=1.4.5
ARG DOCKERCLI_VERSION=v17.06.2-ce
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

# cross compilation helper
FROM --platform=$BUILDPLATFORM tonistiigi/xx:${XX_VERSION} AS xx

# dummy stage to make sure the image is built for unsupported deps
FROM --platform=$BUILDPLATFORM busybox AS build-dummy
RUN mkdir -p /out
FROM scratch AS binary-dummy
COPY --from=build-dummy /out /out

# go base image to retrieve /usr/local/go
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS golang

# base
FROM --platform=$BUILDPLATFORM ${UBUNTU_BASE} AS base-ubuntu
FROM --platform=$BUILDPLATFORM ${DEBIAN_BASE} AS base-debian
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
ENV GO111MODULE=on
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

# registry
FROM base AS registry-src
WORKDIR /usr/src/registry
RUN git init . && git remote add origin "https://github.com/distribution/distribution.git"

FROM base AS registry
WORKDIR /go/src/github.com/docker/distribution
ENV GO111MODULE=off
ENV CGO_ENABLED=0
ARG REGISTRY_VERSION
ARG REGISTRY_VERSION_SCHEMA1
ARG BUILDPLATFORM
RUN --mount=from=registry-src,src=/usr/src/registry,rw \
    --mount=type=cache,target=/root/.cache \
    --mount=type=cache,target=/go/pkg/mod <<EOT
  set -e
  git fetch --depth 1 origin "${REGISTRY_VERSION}"
  git checkout -q FETCH_HEAD
  export GOPATH="/go/src/github.com/docker/distribution/Godeps/_workspace:$GOPATH"
  go build -o /out/registry-v2 -v ./cmd/registry
  xx-verify /out/registry-v2
  case $BUILDPLATFORM in
    linux/amd64|linux/arm/v7|linux/ppc64le|linux/s390x)
      git fetch --depth 1 origin "${REGISTRY_VERSION_SCHEMA1}"
      git checkout -q FETCH_HEAD
      go build -o /out/registry-v2-schema1 -v ./cmd/registry
      xx-verify /out/registry-v2-schema1
      ;;
  esac
EOT

# go-swagger
FROM base AS swagger-src
WORKDIR /usr/src/swagger
# Currently uses a fork from https://github.com/kolyshkin/go-swagger/tree/golang-1.13-fix
# TODO: move to under moby/ or fix upstream go-swagger to work for us.
RUN git init . && git remote add origin "https://github.com/kolyshkin/go-swagger.git"
ARG GOSWAGGER_VERSION
RUN git fetch --depth 1 origin "${GOSWAGGER_VERSION}" && git checkout -q FETCH_HEAD

FROM base AS swagger
ENV GO111MODULE=off
WORKDIR /go/src/github.com/go-swagger/go-swagger
RUN --mount=from=swagger-src,src=/usr/src/swagger,rw \
    --mount=type=cache,target=/root/.cache <<EOT
  set -e
  go build -o /out/swagger ./cmd/swagger
  xx-verify /out/swagger
EOT

# skopeo is used by frozen-images stage
FROM base AS skopeo
ARG SKOPEO_VERSION
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
      CGO_ENABLED=0 GOBIN=/out go install -tags "exclude_graphdriver_devicemapper exclude_graphdriver_btrfs containers_image_openpgp" "github.com/containers/skopeo/cmd/skopeo@${SKOPEO_VERSION}" \
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

# delve builds and installs from https://github.com/go-delve/delve. It can be
# used to run Docker with a possibility of attaching debugger to it.
FROM base AS delve
ARG DELVE_VERSION
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod  <<EOT
  set -e
  mkdir /out
  case ${TARGETPLATFORM} in
    # Delve on Linux is currently only supported on amd64 and arm64;
    # https://github.com/go-delve/delve/blob/v1.8.1/pkg/proc/native/support_sentinel.go#L1-L6
    linux/amd64 | linux/arm64)
      GOBIN=/out go install "github.com/go-delve/delve/cmd/dlv@${DELVE_VERSION}"
      xx-verify /out/dlv
      /out/dlv --help
      ;;
  esac
EOT

# tomll builds and installs from https://github.com/pelletier/go-toml. This
# binary is used in CI in the hack/validate/toml script.
FROM base AS tomll
ARG GOTOML_VERSION
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    GOBIN=/out go install "github.com/pelletier/go-toml/cmd/tomll@${GOTOML_VERSION}" \
    && /out/tomll --help

# go-winres
FROM base AS gowinres
ARG GOWINRES_VERSION
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    GOBIN=/out go install "github.com/tc-hib/go-winres@${GOWINRES_VERSION}" \
    && /out/go-winres --help

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

# golangci-lint
FROM base AS golangci-lint
ARG GOLANGCI_LINT_VERSION
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    GOBIN=/out go install "github.com/golangci/golangci-lint/cmd/golangci-lint@${GOLANGCI_LINT_VERSION}" \
    && /out/golangci-lint --version

# gotestsum
FROM base AS gotestsum
ARG GOTESTSUM_VERSION
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    GOBIN=/out go install "gotest.tools/gotestsum@${GOTESTSUM_VERSION}" \
    && /out/gotestsum --version

# shfmt
FROM base AS shfmt
ARG SHFMT_VERSION
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    GOBIN=/out go install "mvdan.cc/sh/v3/cmd/shfmt@${SHFMT_VERSION}" \
    && /out/shfmt --version

# dockercli
FROM base AS dockercli-src
WORKDIR /usr/src/dockercli
RUN git init . && git remote add origin "https://github.com/docker/cli.git"
ARG DOCKERCLI_VERSION
RUN git fetch --depth 1 origin "${DOCKERCLI_VERSION}" && git checkout -q FETCH_HEAD

FROM base AS dockercli
ENV GO111MODULE=off
WORKDIR /go/src/github.com/docker/cli
ENV CGO_ENABLED=0
ARG DOCKERCLI_VERSION
RUN --mount=from=dockercli-src,src=/usr/src/dockercli/components/cli,rw \
    --mount=type=cache,target=/root/.cache \
    --mount=type=cache,target=/go/pkg/mod <<EOT
  set -e
  DOWNLOAD_URL="https://download.docker.com/linux/static/stable/$(xx-info march)/docker-${DOCKERCLI_VERSION#v}.tgz"
  echo "checking $DOWNLOAD_URL"
  if curl --head --silent --fail "${DOWNLOAD_URL}" 1>/dev/null 2>&1; then
    mkdir /out
    (set -x ; curl -Ls "${DOWNLOAD_URL}" | tar -xz docker/docker)
    mv docker/docker /out/docker
  else
    (set -x ; go build -o /out/docker -v ./cmd/docker)
  fi
  xx-verify /out/docker
EOT

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

# rootlesskit
FROM base AS rootlesskit-src
WORKDIR /usr/src/rootlesskit
RUN git init . && git remote add origin "https://github.com/rootless-containers/rootlesskit.git"
ARG ROOTLESSKIT_VERSION
RUN git fetch --depth 1 origin "${ROOTLESSKIT_VERSION}" && git checkout -q FETCH_HEAD

FROM base AS rootlesskit-build
WORKDIR /go/src/github.com/rootless-containers/rootlesskit
ARG DEBIAN_FRONTEND
ARG TARGETPLATFORM
RUN --mount=type=cache,sharing=locked,id=moby-rootlesskit-aptlib,target=/var/lib/apt \
    --mount=type=cache,sharing=locked,id=moby-rootlesskit-aptcache,target=/var/cache/apt \
    xx-apt-get update && xx-apt-get install -y \
      gcc \
      libc6-dev \
    && xx-go --wrap
ARG DOCKER_LINKMODE
ENV GOBIN=/out
COPY ./contrib/dockerd-rootless.sh /out/
COPY ./contrib/dockerd-rootless-setuptool.sh /out/
RUN --mount=from=rootlesskit-src,src=/usr/src/rootlesskit,rw \
    --mount=type=cache,target=/root/.cache <<EOT
  set -e
  if [ "$DOCKER_LINKMODE" = "static" ]; then
    export CGO_ENABLED=0
  else
    export ROOTLESSKIT_LDFLAGS="-linkmode=external"
  fi
  go build -o /out/rootlesskit -ldflags="$ROOTLESSKIT_LDFLAGS" -v ./cmd/rootlesskit
  xx-verify $([ "$DOCKER_LINKMODE" = "static" ] && echo "--static") /out/rootlesskit
  go build -o /out/rootlesskit-docker-proxy -ldflags="$ROOTLESSKIT_LDFLAGS" -v ./cmd/rootlesskit-docker-proxy
  xx-verify $([ "$DOCKER_LINKMODE" = "static" ] && echo "--static") /out/rootlesskit-docker-proxy
EOT

FROM binary-dummy AS rootlesskit-darwin
FROM binary-dummy AS rootlesskit-freebsd
FROM rootlesskit-build AS rootlesskit-linux
FROM binary-dummy AS rootlesskit-windows
FROM rootlesskit-${TARGETOS} AS rootlesskit

# crun
FROM base AS crun-src
WORKDIR /usr/src/crun
RUN git init . && git remote add origin "https://github.com/containers/crun.git"
ARG CRUN_VERSION
RUN git fetch --depth 1 origin "${CRUN_VERSION}" && git checkout -q FETCH_HEAD

FROM base AS crun
WORKDIR /go/src/github.com/containers/crun
ARG DEBIAN_FRONTEND
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
      python3
RUN --mount=from=crun-src,src=/usr/src/crun,rw \
    --mount=type=cache,target=/root/.cache <<EOT
  set -e
  ./autogen.sh
  ./configure --bindir=/out
  make -j install
EOT

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

# containerutility
FROM base AS containerutility-src
WORKDIR /usr/src/containerutility
RUN git init . && git remote add origin "https://github.com/docker-archive/windows-container-utility.git"

FROM base AS containerutility-build
WORKDIR /usr/src/containerutility
ARG TARGETPLATFORM
RUN --mount=type=cache,sharing=locked,id=moby-containerutility-aptlib,target=/var/lib/apt \
    --mount=type=cache,sharing=locked,id=moby-containerutility-aptcache,target=/var/cache/apt \
    xx-apt-get update && xx-apt-get install -y \
      binutils \
      dpkg-dev \
      g++ \
      gcc \
      pkg-config
ARG CONTAINERUTILITY_VERSION
RUN --mount=from=containerutility-src,src=/usr/src/containerutility,rw \
    --mount=type=cache,target=/root/.cache <<EOT
  set -e
  git fetch --depth 1 origin "${CONTAINERUTILITY_VERSION}"
  git checkout -q FETCH_HEAD
  CC="$(xx-info)-gcc" CXX="$(xx-info)-g++" make
  mkdir /out
  mv containerutility.exe /out/
EOT

FROM binary-dummy AS containerutility-darwin
FROM binary-dummy AS containerutility-freebsd
FROM binary-dummy AS containerutility-linux
FROM containerutility-build AS containerutility-windows-amd64
FROM binary-dummy AS containerutility-windows-arm64
FROM containerutility-windows-${TARGETARCH} AS containerutility-windows
FROM containerutility-${TARGETOS} AS containerutility

FROM base AS dev-systemd-false
COPY --link --from=frozen-images    /out/ /docker-frozen-images
COPY --link --from=tini             /out/ /usr/local/bin/
COPY --link --from=runc             /out/ /usr/local/bin/
COPY --link --from=containerd       /out/ /usr/local/bin/
COPY --link --from=rootlesskit      /out/ /usr/local/bin/
COPY --link --from=containerutility /out/ /usr/local/bin/
COPY --link --from=vpnkit           /     /usr/local/bin/
COPY --link --from=swagger          /out/ /usr/local/bin/
COPY --link --from=tomll            /out/ /usr/local/bin/
COPY --link --from=delve            /out/ /usr/local/bin/
COPY --link --from=gotestsum        /out/ /usr/local/bin/
COPY --link --from=shfmt            /out/ /usr/local/bin/
COPY --link --from=golangci-lint    /out/ /usr/local/bin/
COPY --link --from=criu             /out/ /usr/local/bin/
COPY --link --from=crun             /out/ /usr/local/bin/
COPY --link --from=registry         /out/ /usr/local/bin/
COPY --link --from=dockercli        /out/ /usr/local/cli/
COPY hack/dockerfile/etc/docker/ /etc/docker/
ENV PATH=/usr/local/cli:$PATH
ARG DOCKER_BUILDTAGS
ENV DOCKER_BUILDTAGS="${DOCKER_BUILDTAGS}"
ENV GO111MODULE=off
WORKDIR /go/src/github.com/docker/docker
VOLUME /var/lib/docker
VOLUME /home/unprivilegeduser/.local/share/docker
# Wrap all commands in the "docker-in-docker" script to allow nested containers
ENTRYPOINT ["hack/dind"]

FROM dev-systemd-false AS dev-systemd-true
ARG DEBIAN_FRONTEND
RUN --mount=type=cache,sharing=locked,id=moby-systemd-aptlib,target=/var/lib/apt \
    --mount=type=cache,sharing=locked,id=moby-systemd-aptcache,target=/var/cache/apt \
    apt-get update && apt-get install -y --no-install-recommends \
      curl \
      dbus \
      dbus-user-session \
      systemd \
      systemd-sysv
ENTRYPOINT ["hack/dind-systemd"]

FROM dev-systemd-${SYSTEMD} AS dev-base
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
# set dev environment as safe git directory
RUN git config --global --add safe.directory $GOPATH/src/github.com/docker/docker
RUN --mount=type=cache,sharing=locked,id=moby-build-aptlib,target=/var/lib/apt \
  --mount=type=cache,sharing=locked,id=moby-build-aptcache,target=/var/cache/apt \
  apt-get update && apt-get install --no-install-recommends -y \
    binutils \
    gcc \
    g++ \
    pkg-config \
    dpkg-dev \
    libapparmor-dev \
    libbtrfs-dev \
    libdevmapper-dev \
    libseccomp-dev \
    libsecret-1-dev \
    libsystemd-dev \
    libudev-dev

FROM base AS build-base
WORKDIR /go/src/github.com/docker/docker
ENV GO111MODULE=off
ARG DEBIAN_FRONTEND
ARG TARGETPLATFORM
RUN --mount=type=cache,sharing=locked,id=moby-build-aptlib,target=/var/lib/apt \
    --mount=type=cache,sharing=locked,id=moby-build-aptcache,target=/var/cache/apt \
    xx-apt-get update && xx-apt-get install --no-install-recommends -y \
      binutils \
      dpkg-dev \
      g++ \
      gcc \
      libapparmor-dev \
      libbtrfs-dev \
      libdevmapper-dev \
      libseccomp-dev \
      libsecret-1-dev \
      libsystemd-dev \
      libudev-dev \
      pkg-config \
    && xx-go --wrap

FROM build-base AS build
COPY --from=gowinres /out/ /usr/local/bin
ARG CGO_ENABLED
ARG DOCKER_DEBUG
ARG DOCKER_STRIP
ARG DOCKER_LINKMODE
ARG DOCKER_BUILDTAGS
ARG DOCKER_LDFLAGS
ARG DOCKER_BUILDMODE
ARG DOCKER_BUILDTAGS
ARG VERSION
ARG PLATFORM
ARG PRODUCT
ARG DEFAULT_PRODUCT_LICENSE
ARG PACKAGER_NAME
# PREFIX overrides DEST dir in make.sh script otherwise it fails because of
# read only mount in current work dir
ARG PREFIX=/tmp
# OUTPUT is used in hack/make/.binary to override DEST from make.sh script
ARG OUTPUT=/out
RUN --mount=type=bind,target=. \
    --mount=type=tmpfs,target=cli/winresources/dockerd \
    --mount=type=tmpfs,target=cli/winresources/docker-proxy \
    --mount=type=cache,target=/root/.cache <<EOT
  set -e
  ./hack/make.sh $([ "$DOCKER_LINKMODE" = "static" ] && echo "binary" || echo "dynbinary")
  xx-verify $([ "$DOCKER_LINKMODE" = "static" ] && echo "--static") /out/dockerd$([ "$(go env GOOS)" = "windows" ] && echo ".exe")
  xx-verify $([ "$DOCKER_LINKMODE" = "static" ] && echo "--static") /out/docker-proxy$([ "$(go env GOOS)" = "windows" ] && echo ".exe")
EOT

# usage:
# > docker buildx bake binary
# > DOCKER_LINKMODE=dynamic docker buildx bake binary
# or
# > make binary
# > make dynbinary
FROM scratch AS binary
COPY --link --from=tini  /out/ /
COPY --link --from=build /out  /

# usage:
# > make shell
FROM dev-base AS dev
COPY . .

FROM dev
