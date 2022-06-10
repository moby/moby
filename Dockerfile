# syntax=docker/dockerfile:1.3

ARG CROSS="false"
ARG SYSTEMD="false"
ARG GO_VERSION=1.18.3
ARG DEBIAN_FRONTEND=noninteractive
ARG VPNKIT_VERSION=0.5.0

ARG BASE_DEBIAN_DISTRO="bullseye"
ARG GOLANG_IMAGE="golang:${GO_VERSION}-${BASE_DEBIAN_DISTRO}"

FROM ${GOLANG_IMAGE} AS base
RUN echo 'Binary::apt::APT::Keep-Downloaded-Packages "true";' > /etc/apt/apt.conf.d/keep-cache
ARG APT_MIRROR
RUN sed -ri "s/(httpredir|deb).debian.org/${APT_MIRROR:-deb.debian.org}/g" /etc/apt/sources.list \
 && sed -ri "s/(security).debian.org/${APT_MIRROR:-security.debian.org}/g" /etc/apt/sources.list
ENV GO111MODULE=off

FROM base AS criu
ARG DEBIAN_FRONTEND
ADD --chmod=0644 https://download.opensuse.org/repositories/devel:/tools:/criu/Debian_11/Release.key /etc/apt/trusted.gpg.d/criu.gpg.asc
RUN --mount=type=cache,sharing=locked,id=moby-criu-aptlib,target=/var/lib/apt \
    --mount=type=cache,sharing=locked,id=moby-criu-aptcache,target=/var/cache/apt \
        echo 'deb https://download.opensuse.org/repositories/devel:/tools:/criu/Debian_11/ /' > /etc/apt/sources.list.d/criu.list \
        && apt-get update \
        && apt-get install -y --no-install-recommends criu \
        && install -D /usr/sbin/criu /build/criu

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

FROM debian:${BASE_DEBIAN_DISTRO} AS frozen-images
ARG DEBIAN_FRONTEND
RUN --mount=type=cache,sharing=locked,id=moby-frozen-images-aptlib,target=/var/lib/apt \
    --mount=type=cache,sharing=locked,id=moby-frozen-images-aptcache,target=/var/cache/apt \
       apt-get update && apt-get install -y --no-install-recommends \
           ca-certificates \
           curl \
           jq
# Get useful and necessary Hub images so we can "docker load" locally instead of pulling
COPY contrib/download-frozen-image-v2.sh /
ARG TARGETARCH
RUN /download-frozen-image-v2.sh /build \
        busybox:latest@sha256:95cf004f559831017cdf4628aaf1bb30133677be8702a8c5f2994629f637a209 \
        busybox:glibc@sha256:1f81263701cddf6402afe9f33fca0266d9fff379e59b1748f33d3072da71ee85 \
        debian:bullseye-slim@sha256:dacf278785a4daa9de07596ec739dbc07131e189942772210709c5c0777e8437 \
        hello-world:latest@sha256:d58e752213a51785838f9eed2b7a498ffa1cb3aa7f946dda11af39286c3db9a9 \
        arm32v7/hello-world:latest@sha256:50b8560ad574c779908da71f7ce370c0a2471c098d44d1c8f6b513c5a55eeeb1
# See also frozenImages in "testutil/environment/protect.go" (which needs to be updated when adding images to this list)

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
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
        GOBIN=/build/ GO111MODULE=on go install "github.com/go-delve/delve/cmd/dlv@${DELVE_VERSION}" \
     && /build/dlv --help

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

FROM dev-base AS containerd
ARG DEBIAN_FRONTEND
RUN --mount=type=cache,sharing=locked,id=moby-containerd-aptlib,target=/var/lib/apt \
    --mount=type=cache,sharing=locked,id=moby-containerd-aptcache,target=/var/cache/apt \
        apt-get update && apt-get install -y --no-install-recommends \
            libbtrfs-dev
ARG CONTAINERD_VERSION
COPY /hack/dockerfile/install/install.sh /hack/dockerfile/install/containerd.installer /
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
        PREFIX=/build /install.sh containerd

FROM base AS golangci_lint
ARG GOLANGCI_LINT_VERSION=v1.44.0
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

FROM runtime-dev AS runc
ARG RUNC_VERSION
ARG RUNC_BUILDTAGS
COPY /hack/dockerfile/install/install.sh /hack/dockerfile/install/runc.installer /
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
        PREFIX=/build /install.sh runc

FROM dev-base AS tini
ARG DEBIAN_FRONTEND
ARG TINI_VERSION
RUN --mount=type=cache,sharing=locked,id=moby-tini-aptlib,target=/var/lib/apt \
    --mount=type=cache,sharing=locked,id=moby-tini-aptcache,target=/var/cache/apt \
        apt-get update && apt-get install -y --no-install-recommends \
            cmake \
            vim-common
COPY /hack/dockerfile/install/install.sh /hack/dockerfile/install/tini.installer /
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
        PREFIX=/build /install.sh tini

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

FROM --platform=amd64 djs55/vpnkit:${VPNKIT_VERSION} AS vpnkit-amd64

FROM --platform=arm64 djs55/vpnkit:${VPNKIT_VERSION} AS vpnkit-arm64

FROM scratch AS vpnkit
COPY --from=vpnkit-amd64 /vpnkit /build/vpnkit.x86_64
COPY --from=vpnkit-arm64 /vpnkit /build/vpnkit.aarch64

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
COPY --from=frozen-images /build/ /docker-frozen-images
COPY --from=swagger       /build/ /usr/local/bin/
COPY --from=delve         /build/ /usr/local/bin/
COPY --from=tomll         /build/ /usr/local/bin/
COPY --from=gowinres      /build/ /usr/local/bin/
COPY --from=tini          /build/ /usr/local/bin/
COPY --from=registry      /build/ /usr/local/bin/
COPY --from=criu          /build/ /usr/local/bin/
COPY --from=gotestsum     /build/ /usr/local/bin/
COPY --from=golangci_lint /build/ /usr/local/bin/
COPY --from=shfmt         /build/ /usr/local/bin/
COPY --from=runc          /build/ /usr/local/bin/
COPY --from=containerd    /build/ /usr/local/bin/
COPY --from=rootlesskit   /build/ /usr/local/bin/
COPY --from=vpnkit        /build/ /usr/local/bin/
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
RUN mkdir -p hack \
  && curl -o hack/dind-systemd https://raw.githubusercontent.com/AkihiroSuda/containerized-systemd/b70bac0daeea120456764248164c21684ade7d0d/docker-entrypoint.sh \
  && chmod +x hack/dind-systemd
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
COPY --from=tini          /build/ /usr/local/bin/
COPY --from=runc          /build/ /usr/local/bin/
COPY --from=containerd    /build/ /usr/local/bin/
COPY --from=rootlesskit   /build/ /usr/local/bin/
COPY --from=vpnkit        /build/ /usr/local/bin/
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
