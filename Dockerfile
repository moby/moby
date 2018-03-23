# This file describes the standard way to build Docker, using docker
#
# Usage:
#
# # Use make to build a development environment image and run it in a container.
# # This is slow the first time.
# make BIND_DIR=. shell
#
# The following commands are executed inside the running container.

# # Make a dockerd binary.
# # hack/make.sh binary
#
# # Install dockerd to /usr/local/bin
# # make install
#
# # Run unit tests
# # hack/test/unit
#
# # Run tests e.g. integration, py
# # hack/make.sh binary test-integration test-docker-py
#
# # Publish a release:
# docker run --privileged \
#  -e AWS_S3_BUCKET=baz \
#  -e AWS_ACCESS_KEY=foo \
#  -e AWS_SECRET_KEY=bar \
#  -e GPG_PASSPHRASE=gloubiboulga \
#  docker hack/release.sh
#
# Note: AppArmor used to mess with privileged mode, but this is no longer
# the case. Therefore, you don't have to disable it anymore.
#

FROM golang:1.9.4 AS base
# FIXME(vdemeester) this is kept for other script depending on it to not fail right away
# Remove this once the other scripts uses something else to detect the version
ENV GO_VERSION 1.9.4
# allow replacing httpredir or deb mirror
ARG APT_MIRROR=deb.debian.org
RUN sed -ri "s/(httpredir|deb).debian.org/$APT_MIRROR/g" /etc/apt/sources.list

FROM base AS criu
# Install CRIU for checkpoint/restore support
ENV CRIU_VERSION 3.6
# Install dependancy packages specific to criu
RUN apt-get update && apt-get install -y \
	libnet-dev \
	libprotobuf-c0-dev \
	libprotobuf-dev \
	libnl-3-dev \
	libcap-dev \
	protobuf-compiler \
	protobuf-c-compiler \
	python-protobuf \
	&& mkdir -p /usr/src/criu \
	&& curl -sSL https://github.com/checkpoint-restore/criu/archive/v${CRIU_VERSION}.tar.gz | tar -C /usr/src/criu/ -xz --strip-components=1 \
	&& cd /usr/src/criu \
	&& make \
	&& make PREFIX=/opt/criu install-criu

FROM base AS registry
# Install two versions of the registry. The first is an older version that
# only supports schema1 manifests. The second is a newer version that supports
# both. This allows integration-cli tests to cover push/pull with both schema1
# and schema2 manifests.
ENV REGISTRY_COMMIT_SCHEMA1 ec87e9b6971d831f0eff752ddb54fb64693e51cd
ENV REGISTRY_COMMIT 47a064d4195a9b56133891bbb13620c3ac83a827
RUN set -x \
	&& export GOPATH="$(mktemp -d)" \
	&& git clone https://github.com/docker/distribution.git "$GOPATH/src/github.com/docker/distribution" \
	&& (cd "$GOPATH/src/github.com/docker/distribution" && git checkout -q "$REGISTRY_COMMIT") \
	&& GOPATH="$GOPATH/src/github.com/docker/distribution/Godeps/_workspace:$GOPATH" \
		go build -buildmode=pie -o /usr/local/bin/registry-v2 github.com/docker/distribution/cmd/registry \
	&& case $(dpkg --print-architecture) in \
		amd64|ppc64*|s390x) \
		(cd "$GOPATH/src/github.com/docker/distribution" && git checkout -q "$REGISTRY_COMMIT_SCHEMA1"); \
		GOPATH="$GOPATH/src/github.com/docker/distribution/Godeps/_workspace:$GOPATH"; \
			go build -buildmode=pie -o /usr/local/bin/registry-v2-schema1 github.com/docker/distribution/cmd/registry; \
		;; \
	   esac \
	&& rm -rf "$GOPATH"



FROM base AS docker-py
# Get the "docker-py" source so we can run their integration tests
ENV DOCKER_PY_COMMIT 8b246db271a85d6541dc458838627e89c683e42f
RUN git clone https://github.com/docker/docker-py.git /docker-py \
	&& cd /docker-py \
	&& git checkout -q $DOCKER_PY_COMMIT



FROM base AS swagger
# Install go-swagger for validating swagger.yaml
ENV GO_SWAGGER_COMMIT c28258affb0b6251755d92489ef685af8d4ff3eb
RUN set -x \
	&& export GOPATH="$(mktemp -d)" \
	&& git clone https://github.com/go-swagger/go-swagger.git "$GOPATH/src/github.com/go-swagger/go-swagger" \
	&& (cd "$GOPATH/src/github.com/go-swagger/go-swagger" && git checkout -q "$GO_SWAGGER_COMMIT") \
	&& go build -o /usr/local/bin/swagger github.com/go-swagger/go-swagger/cmd/swagger \
	&& rm -rf "$GOPATH"


FROM base AS frozen-images
RUN apt-get update && apt-get install -y jq ca-certificates --no-install-recommends
# Get useful and necessary Hub images so we can "docker load" locally instead of pulling
COPY contrib/download-frozen-image-v2.sh /
RUN /download-frozen-image-v2.sh /docker-frozen-images \
	buildpack-deps:jessie@sha256:dd86dced7c9cd2a724e779730f0a53f93b7ef42228d4344b25ce9a42a1486251 \
	busybox:latest@sha256:bbc3a03235220b170ba48a157dd097dd1379299370e1ed99ce976df0355d24f0 \
	busybox:glibc@sha256:0b55a30394294ab23b9afd58fab94e61a923f5834fba7ddbae7f8e0c11ba85e6 \
	debian:jessie@sha256:287a20c5f73087ab406e6b364833e3fb7b3ae63ca0eb3486555dc27ed32c6e60 \
	hello-world:latest@sha256:be0cd392e45be79ffeffa6b05338b98ebb16c87b255f48e297ec7f98e123905c
# See also ensureFrozenImagesLinux() in "integration-cli/fixtures_linux_daemon_test.go" (which needs to be updated when adding images to this list)

# Just a little hack so we don't have to install these deps twice, once for runc and once for dockerd
FROM base AS runtime-dev
RUN apt-get update && apt-get install -y \
	libapparmor-dev \
	libseccomp-dev


FROM base AS tomlv
ENV INSTALL_BINARY_NAME=tomlv
COPY hack/dockerfile/install/install.sh ./install.sh
COPY hack/dockerfile/install/$INSTALL_BINARY_NAME.installer ./
RUN PREFIX=/opt/$INSTALL_BINARY_NAME ./install.sh $INSTALL_BINARY_NAME

FROM base AS vndr
ENV INSTALL_BINARY_NAME=vndr
COPY hack/dockerfile/install/install.sh ./install.sh
COPY hack/dockerfile/install/$INSTALL_BINARY_NAME.installer ./
RUN PREFIX=/opt/$INSTALL_BINARY_NAME ./install.sh $INSTALL_BINARY_NAME

FROM base AS containerd
RUN apt-get update && apt-get install -y btrfs-tools
ENV INSTALL_BINARY_NAME=containerd
COPY hack/dockerfile/install/install.sh ./install.sh
COPY hack/dockerfile/install/$INSTALL_BINARY_NAME.installer ./
RUN PREFIX=/opt/$INSTALL_BINARY_NAME ./install.sh $INSTALL_BINARY_NAME

FROM base AS proxy
ENV INSTALL_BINARY_NAME=proxy
COPY hack/dockerfile/install/install.sh ./install.sh
COPY hack/dockerfile/install/$INSTALL_BINARY_NAME.installer ./
RUN PREFIX=/opt/$INSTALL_BINARY_NAME ./install.sh $INSTALL_BINARY_NAME

FROM base AS gometalinter
ENV INSTALL_BINARY_NAME=gometalinter
COPY hack/dockerfile/install/install.sh ./install.sh
COPY hack/dockerfile/install/$INSTALL_BINARY_NAME.installer ./
RUN PREFIX=/opt/$INSTALL_BINARY_NAME ./install.sh $INSTALL_BINARY_NAME

FROM base AS dockercli
ENV INSTALL_BINARY_NAME=dockercli
COPY hack/dockerfile/install/install.sh ./install.sh
COPY hack/dockerfile/install/$INSTALL_BINARY_NAME.installer ./
RUN PREFIX=/opt/$INSTALL_BINARY_NAME ./install.sh $INSTALL_BINARY_NAME

FROM runtime-dev AS runc
ENV INSTALL_BINARY_NAME=runc
COPY hack/dockerfile/install/install.sh ./install.sh
COPY hack/dockerfile/install/$INSTALL_BINARY_NAME.installer ./
RUN PREFIX=/opt/$INSTALL_BINARY_NAME ./install.sh $INSTALL_BINARY_NAME

FROM base AS tini
RUN apt-get update && apt-get install -y cmake vim-common
COPY hack/dockerfile/install/install.sh ./install.sh
ENV INSTALL_BINARY_NAME=tini
COPY hack/dockerfile/install/$INSTALL_BINARY_NAME.installer ./
RUN PREFIX=/opt/$INSTALL_BINARY_NAME ./install.sh $INSTALL_BINARY_NAME



# TODO: Some of this is only really needed for testing, it would be nice to split this up
FROM runtime-dev AS dev
RUN groupadd -r docker
RUN useradd --create-home --gid docker unprivilegeduser
# Activate bash completion and include Docker's completion if mounted with DOCKER_BASH_COMPLETION_PATH
RUN echo "source /usr/share/bash-completion/bash_completion" >> /etc/bash.bashrc
RUN ln -s /usr/local/completion/bash/docker /etc/bash_completion.d/docker
RUN ldconfig
# This should only install packages that are specifically needed for the dev environment and nothing else
# Do you really need to add another package here? Can it be done in a different build stage?
RUN apt-get update && apt-get install -y \
	apparmor \
	aufs-tools \
	bash-completion \
	btrfs-tools \
	iptables \
	jq \
	libdevmapper-dev \
	libudev-dev \
	libsystemd-dev \
	binutils-mingw-w64 \
	g++-mingw-w64-x86-64 \
	net-tools \
	pigz \
	python-backports.ssl-match-hostname \
	python-dev \
	python-mock \
	python-pip \
	python-requests \
	python-setuptools \
	python-websocket \
	python-wheel \
	thin-provisioning-tools \
	vim \
	vim-common \
	xfsprogs \
	zip \
	bzip2 \
	xz-utils \
	--no-install-recommends
COPY --from=swagger /usr/local/bin/swagger* /usr/local/bin/
COPY --from=frozen-images /docker-frozen-images /docker-frozen-images
COPY --from=gometalinter /opt/gometalinter/ /usr/local/bin/
COPY --from=tomlv /opt/tomlv/ /usr/local/bin/
COPY --from=vndr /opt/vndr/ /usr/local/bin/
COPY --from=tini /opt/tini/ /usr/local/bin/
COPY --from=runc /opt/runc/ /usr/local/bin/
COPY --from=containerd /opt/containerd/ /usr/local/bin/
COPY --from=proxy /opt/proxy/ /usr/local/bin/
COPY --from=dockercli /opt/dockercli /usr/local/cli
COPY --from=registry /usr/local/bin/registry* /usr/local/bin/
COPY --from=criu /opt/criu/ /usr/local/
COPY --from=docker-py /docker-py /docker-py
# TODO: This is for the docker-py tests, which shouldn't really be needed for
# this image, but currently CI is expecting to run this image. This should be
# split out into a separate image, including all the `python-*` deps installed
# above.
RUN cd /docker-py \
	&& pip install docker-pycreds==0.2.1 \
	&& pip install -r test-requirements.txt

ENV PATH=/usr/local/cli:$PATH
ENV DOCKER_BUILDTAGS apparmor seccomp selinux
# Options for hack/validate/gometalinter
ENV GOMETALINTER_OPTS="--deadline=2m"
WORKDIR /go/src/github.com/docker/docker
VOLUME /var/lib/docker
# Wrap all commands in the "docker-in-docker" script to allow nested containers
ENTRYPOINT ["hack/dind"]
# Upload docker source
COPY . /go/src/github.com/docker/docker
