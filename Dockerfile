# This file describes the standard way to build Docker, using docker
#
# Usage:
#
# # Assemble the full dev environment. This is slow the first time.
# docker build -t docker .
#
# # Mount your source in an interactive container for quick testing:
# docker run -v `pwd`:/go/src/github.com/docker/docker --privileged -i -t docker bash
#
# # Run the test suite:
# docker run --privileged docker hack/make.sh test
#
# # Publish a release:
# docker run --privileged \
#  -e AWS_S3_BUCKET=baz \
#  -e AWS_ACCESS_KEY=foo \
#  -e AWS_SECRET_KEY=bar \
#  -e GPG_PASSPHRASE=gloubiboulga \
#  docker hack/release.sh
#
# Note: Apparmor used to mess with privileged mode, but this is no longer
# the case. Therefore, you don't have to disable it anymore.
#

FROM ubuntu:14.04
MAINTAINER Tianon Gravi <admwiggin@gmail.com> (@tianon)

# Packaged dependencies
RUN apt-get update && apt-get install -y \
	apparmor \
	aufs-tools \
	automake \
	btrfs-tools \
	build-essential \
	curl \
	dpkg-sig \
	git \
	iptables \
	libapparmor-dev \
	libcap-dev \
	libsqlite3-dev \
	mercurial \
	parallel \
	python-mock \
	python-pip \
	python-websocket \
	reprepro \
	ruby1.9.1 \
	ruby1.9.1-dev \
	s3cmd=1.1.0* \
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
ENV LXC_VERSION 1.0.7
RUN mkdir -p /usr/src/lxc \
	&& curl -sSL https://linuxcontainers.org/downloads/lxc/lxc-${LXC_VERSION}.tar.gz | tar -v -C /usr/src/lxc/ -xz --strip-components=1
RUN cd /usr/src/lxc \
	&& ./configure \
	&& make \
	&& make install \
	&& ldconfig

# Install Go
ENV GO_VERSION 1.4.2
RUN curl -sSL https://golang.org/dl/go${GO_VERSION}.src.tar.gz | tar -v -C /usr/local -xz \
	&& mkdir -p /go/bin
ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go:/go/src/github.com/docker/docker/vendor
RUN cd /usr/local/go/src && ./make.bash --no-clean 2>&1

# Compile Go for cross compilation
ENV DOCKER_CROSSPLATFORMS \
	linux/386 linux/arm \
	darwin/amd64 darwin/386 \
	freebsd/amd64 freebsd/386 freebsd/arm \
	windows/amd64 windows/386

# (set an explicit GOARM of 5 for maximum compatibility)
ENV GOARM 5
RUN cd /usr/local/go/src \
	&& set -x \
	&& for platform in $DOCKER_CROSSPLATFORMS; do \
		GOOS=${platform%/*} \
		GOARCH=${platform##*/} \
			./make.bash --no-clean 2>&1; \
	done

# We still support compiling with older Go, so need to grab older "gofmt"
ENV GOFMT_VERSION 1.3.3
RUN curl -sSL https://storage.googleapis.com/golang/go${GOFMT_VERSION}.$(go env GOOS)-$(go env GOARCH).tar.gz | tar -C /go/bin -xz --strip-components=2 go/bin/gofmt

# Grab Go's cover tool for dead-simple code coverage testing
RUN go get golang.org/x/tools/cmd/cover

# TODO replace FPM with some very minimal debhelper stuff
RUN gem install --no-rdoc --no-ri fpm --version 1.3.2

# Install registry
ENV REGISTRY_COMMIT d957768537c5af40e4f4cd96871f7b2bde9e2923
RUN set -x \
	&& git clone https://github.com/docker/distribution.git /go/src/github.com/docker/distribution \
	&& (cd /go/src/github.com/docker/distribution && git checkout -q $REGISTRY_COMMIT) \
	&& GOPATH=/go/src/github.com/docker/distribution/Godeps/_workspace:/go \
		go build -o /go/bin/registry-v2 github.com/docker/distribution/cmd/registry

# Get the "docker-py" source so we can run their integration tests
ENV DOCKER_PY_COMMIT 91985b239764fe54714fa0a93d52aa362357d251
RUN git clone https://github.com/docker/docker-py.git /docker-py \
	&& cd /docker-py \
	&& git checkout -q $DOCKER_PY_COMMIT

# Setup s3cmd config
RUN { \
		echo '[default]'; \
		echo 'access_key=$AWS_ACCESS_KEY'; \
		echo 'secret_key=$AWS_SECRET_KEY'; \
	} > ~/.s3cfg

# Set user.email so crosbymichael's in-container merge commits go smoothly
RUN git config --global user.email 'docker-dummy@example.com'

# Add an unprivileged user to be used for tests which need it
RUN groupadd -r docker
RUN useradd --create-home --gid docker unprivilegeduser

VOLUME /var/lib/docker
WORKDIR /go/src/github.com/docker/docker
ENV DOCKER_BUILDTAGS apparmor selinux btrfs_noversion

# Let us use a .bashrc file
RUN ln -sfv $PWD/.bashrc ~/.bashrc

# Get useful and necessary Hub images so we can "docker load" locally instead of pulling
COPY contrib/download-frozen-image.sh /go/src/github.com/docker/docker/contrib/
RUN ./contrib/download-frozen-image.sh /docker-frozen-images \
	busybox:latest@4986bf8c15363d1c5d15512d5266f8777bfba4974ac56e3270e7760f6f0a8125 \
	hello-world:frozen@e45a5af57b00862e5ef5782a9925979a02ba2b12dff832fd0991335f4a11e5c5
# see also "hack/make/.ensure-frozen-images" (which needs to be updated any time this list is)

# Install man page generator
COPY vendor /go/src/github.com/docker/docker/vendor
# (copy vendor/ because go-md2man needs golang.org/x/net)
RUN set -x \
	&& git clone -b v1.0.1 https://github.com/cpuguy83/go-md2man.git /go/src/github.com/cpuguy83/go-md2man \
	&& git clone -b v1.2 https://github.com/russross/blackfriday.git /go/src/github.com/russross/blackfriday \
	&& go install -v github.com/cpuguy83/go-md2man

# install toml validator
ENV TOMLV_COMMIT 9baf8a8a9f2ed20a8e54160840c492f937eeaf9a
RUN set -x \
	&& git clone https://github.com/BurntSushi/toml.git /go/src/github.com/BurntSushi/toml \
	&& (cd /go/src/github.com/BurntSushi/toml && git checkout -q $TOMLV_COMMIT) \
	&& go install -v github.com/BurntSushi/toml/cmd/tomlv

# Wrap all commands in the "docker-in-docker" script to allow nested containers
ENTRYPOINT ["hack/dind"]

# Upload docker source
COPY . /go/src/github.com/docker/docker
