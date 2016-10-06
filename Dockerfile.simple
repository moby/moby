# docker build -t docker:simple -f Dockerfile.simple .
# docker run --rm docker:simple hack/make.sh dynbinary
# docker run --rm --privileged docker:simple hack/dind hack/make.sh test-unit
# docker run --rm --privileged -v /var/lib/docker docker:simple hack/dind hack/make.sh dynbinary test-integration-cli

# This represents the bare minimum required to build and test Docker.

FROM debian:jessie

# Compile and runtime deps
# https://github.com/docker/docker/blob/master/project/PACKAGERS.md#build-dependencies
# https://github.com/docker/docker/blob/master/project/PACKAGERS.md#runtime-dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
		btrfs-tools \
		build-essential \
		curl \
		gcc \
		git \
		libapparmor-dev \
		libdevmapper-dev \
		libsqlite3-dev \
		\
		ca-certificates \
		e2fsprogs \
		iptables \
		procps \
		xfsprogs \
		xz-utils \
		\
		aufs-tools \
	&& rm -rf /var/lib/apt/lists/*

# Install seccomp: the version shipped in trusty is too old
ENV SECCOMP_VERSION 2.3.1
RUN set -x \
	&& export SECCOMP_PATH="$(mktemp -d)" \
	&& curl -fsSL "https://github.com/seccomp/libseccomp/releases/download/v${SECCOMP_VERSION}/libseccomp-${SECCOMP_VERSION}.tar.gz" \
		| tar -xzC "$SECCOMP_PATH" --strip-components=1 \
	&& ( \
		cd "$SECCOMP_PATH" \
		&& ./configure --prefix=/usr/local \
		&& make \
		&& make install \
		&& ldconfig \
	) \
	&& rm -rf "$SECCOMP_PATH"

# Install Go
# IMPORTANT: If the version of Go is updated, the Windows to Linux CI machines
#            will need updating, to avoid errors. Ping #docker-maintainers on IRC
#            with a heads-up.
ENV GO_VERSION 1.7.1
RUN curl -fsSL "https://storage.googleapis.com/golang/go${GO_VERSION}.linux-amd64.tar.gz" \
	| tar -xzC /usr/local
ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go:/go/src/github.com/docker/docker/vendor
ENV CGO_LDFLAGS -L/lib

# Install runc, containerd and grimes
# Please edit hack/dockerfile/install-binaries.sh to update them.
COPY hack/dockerfile/install-binaries.sh /tmp/install-binaries.sh
RUN /tmp/install-binaries.sh runc containerd grimes

ENV AUTO_GOPATH 1
WORKDIR /usr/src/docker
COPY . /usr/src/docker
