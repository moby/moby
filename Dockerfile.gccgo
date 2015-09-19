# This file describes the standard way to build Docker, using docker
#
# Usage:
#
# # Assemble the full dev environment. This is slow the first time.
# docker build -t docker -f Dockerfile.gccgo .
#

FROM gcc:5.2

# Packaged dependencies
RUN apt-get update && apt-get install -y \
	apparmor \
	aufs-tools \
	btrfs-tools \
	build-essential \
	curl \
	git \
	iptables \
	net-tools \
	libapparmor-dev \
	libcap-dev \
	libsqlite3-dev \
	mercurial \
	parallel \
	python-mock \
	python-pip \
	python-websocket \
	--no-install-recommends

# Get lvm2 source for compiling statically
RUN git clone -b v2_02_103 https://git.fedorahosted.org/git/lvm2.git /usr/local/lvm2
# see https://git.fedorahosted.org/cgit/lvm2.git/refs/tags for release tags

# Compile and install lvm2
RUN cd /usr/local/lvm2 \
	&& ./configure --enable-static_link \
	&& make device-mapper \
	&& make install_device-mapper
# see https://git.fedorahosted.org/cgit/lvm2.git/tree/INSTALL

# Install lxc
ENV LXC_VERSION 1.1.2
RUN mkdir -p /usr/src/lxc \
	&& curl -sSL https://linuxcontainers.org/downloads/lxc/lxc-${LXC_VERSION}.tar.gz | tar -v -C /usr/src/lxc/ -xz --strip-components=1
RUN cd /usr/src/lxc \
	&& ./configure \
	&& make \
	&& make install \
	&& ldconfig

ENV GOPATH /go:/go/src/github.com/docker/docker/vendor

# Get the "docker-py" source so we can run their integration tests
ENV DOCKER_PY_COMMIT 139850f3f3b17357bab5ba3edfb745fb14043764
RUN git clone https://github.com/docker/docker-py.git /docker-py \
	&& cd /docker-py \
	&& git checkout -q $DOCKER_PY_COMMIT

# Add an unprivileged user to be used for tests which need it
RUN groupadd -r docker
RUN useradd --create-home --gid docker unprivilegeduser

VOLUME /var/lib/docker
WORKDIR /go/src/github.com/docker/docker
ENV DOCKER_BUILDTAGS apparmor selinux

# Wrap all commands in the "docker-in-docker" script to allow nested containers
ENTRYPOINT ["hack/dind"]

# Upload docker source
COPY . /go/src/github.com/docker/docker
