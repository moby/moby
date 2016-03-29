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
# Note: AppArmor used to mess with privileged mode, but this is no longer
# the case. Therefore, you don't have to disable it anymore.
#

FROM debian:jessie

# add zfs ppa
RUN apt-key adv --keyserver hkp://p80.pool.sks-keyservers.net:80 --recv-keys E871F18B51E0147C77796AC81196BA81F6B0FC61 \
	|| apt-key adv --keyserver hkp://pgp.mit.edu:80 --recv-keys E871F18B51E0147C77796AC81196BA81F6B0FC61
RUN echo deb http://ppa.launchpad.net/zfs-native/stable/ubuntu trusty main > /etc/apt/sources.list.d/zfs.list

# add llvm repo
RUN apt-key adv --keyserver hkp://p80.pool.sks-keyservers.net:80 --recv-keys 6084F3CF814B57C1CF12EFD515CF4D18AF4F7421 \
	|| apt-key adv --keyserver hkp://pgp.mit.edu:80 --recv-keys 6084F3CF814B57C1CF12EFD515CF4D18AF4F7421
RUN echo deb http://llvm.org/apt/jessie/ llvm-toolchain-jessie-3.8 main > /etc/apt/sources.list.d/llvm.list

# allow replacing httpredir mirror
ARG APT_MIRROR=httpredir.debian.org
RUN sed -i s/httpredir.debian.org/$APT_MIRROR/g /etc/apt/sources.list

# Packaged dependencies
RUN apt-get update && apt-get install -y \
	apparmor \
	apt-utils \
	aufs-tools \
	automake \
	bash-completion \
	bsdmainutils \
	btrfs-tools \
	build-essential \
	clang-3.8 \
	createrepo \
	curl \
	dpkg-sig \
	gcc-mingw-w64 \
	git \
	iptables \
	jq \
	libapparmor-dev \
	libcap-dev \
	libltdl-dev \
	libsqlite3-dev \
	libsystemd-journal-dev \
	libtool \
	mercurial \
	net-tools \
	pkg-config \
	python-dev \
	python-mock \
	python-pip \
	python-websocket \
	ubuntu-zfs \
	xfsprogs \
	libzfs-dev \
	tar \
	--no-install-recommends \
	&& pip install awscli==1.10.15 \
	&& ln -snf /usr/bin/clang-3.8 /usr/local/bin/clang \
	&& ln -snf /usr/bin/clang++-3.8 /usr/local/bin/clang++

# Get lvm2 source for compiling statically
ENV LVM2_VERSION 2.02.103
RUN mkdir -p /usr/local/lvm2 \
	&& curl -fsSL "https://mirrors.kernel.org/sourceware/lvm2/LVM2.${LVM2_VERSION}.tgz" \
		| tar -xzC /usr/local/lvm2 --strip-components=1
# see https://git.fedorahosted.org/cgit/lvm2.git/refs/tags for release tags

# Compile and install lvm2
RUN cd /usr/local/lvm2 \
	&& ./configure \
		--build="$(gcc -print-multiarch)" \
		--enable-static_link \
	&& make device-mapper \
	&& make install_device-mapper
# see https://git.fedorahosted.org/cgit/lvm2.git/tree/INSTALL

# Configure the container for OSX cross compilation
ENV OSX_SDK MacOSX10.11.sdk
ENV OSX_CROSS_COMMIT 8aa9b71a394905e6c5f4b59e2b97b87a004658a4
RUN set -x \
	&& export OSXCROSS_PATH="/osxcross" \
	&& git clone https://github.com/tpoechtrager/osxcross.git $OSXCROSS_PATH \
	&& ( cd $OSXCROSS_PATH && git checkout -q $OSX_CROSS_COMMIT) \
	&& curl -sSL https://s3.dockerproject.org/darwin/${OSX_SDK}.tar.xz -o "${OSXCROSS_PATH}/tarballs/${OSX_SDK}.tar.xz" \
	&& UNATTENDED=yes OSX_VERSION_MIN=10.6 ${OSXCROSS_PATH}/build.sh
ENV PATH /osxcross/target/bin:$PATH

# install seccomp: the version shipped in trusty is too old
ENV SECCOMP_VERSION 2.3.0
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
ENV GO_VERSION 1.5.3
RUN curl -fsSL "https://storage.googleapis.com/golang/go${GO_VERSION}.linux-amd64.tar.gz" \
	| tar -xzC /usr/local
ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go:/go/src/github.com/docker/docker/vendor

# Compile Go for cross compilation
ENV DOCKER_CROSSPLATFORMS \
	linux/386 linux/arm \
	darwin/amd64 \
	freebsd/amd64 freebsd/386 freebsd/arm \
	windows/amd64 windows/386

# This has been commented out and kept as reference because we don't support compiling with older Go anymore.
# ENV GOFMT_VERSION 1.3.3
# RUN curl -sSL https://storage.googleapis.com/golang/go${GOFMT_VERSION}.$(go env GOOS)-$(go env GOARCH).tar.gz | tar -C /go/bin -xz --strip-components=2 go/bin/gofmt

ENV GO_TOOLS_COMMIT 823804e1ae08dbb14eb807afc7db9993bc9e3cc3
# Grab Go's cover tool for dead-simple code coverage testing
# Grab Go's vet tool for examining go code to find suspicious constructs
# and help prevent errors that the compiler might not catch
RUN git clone https://github.com/golang/tools.git /go/src/golang.org/x/tools \
	&& (cd /go/src/golang.org/x/tools && git checkout -q $GO_TOOLS_COMMIT) \
	&& go install -v golang.org/x/tools/cmd/cover \
	&& go install -v golang.org/x/tools/cmd/vet
# Grab Go's lint tool
ENV GO_LINT_COMMIT 32a87160691b3c96046c0c678fe57c5bef761456
RUN git clone https://github.com/golang/lint.git /go/src/github.com/golang/lint \
	&& (cd /go/src/github.com/golang/lint && git checkout -q $GO_LINT_COMMIT) \
	&& go install -v github.com/golang/lint/golint

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
		go build -o /usr/local/bin/registry-v2 github.com/docker/distribution/cmd/registry \
	&& (cd "$GOPATH/src/github.com/docker/distribution" && git checkout -q "$REGISTRY_COMMIT_SCHEMA1") \
	&& GOPATH="$GOPATH/src/github.com/docker/distribution/Godeps/_workspace:$GOPATH" \
		go build -o /usr/local/bin/registry-v2-schema1 github.com/docker/distribution/cmd/registry \
	&& rm -rf "$GOPATH"

# Install notary server
ENV NOTARY_VERSION docker-v1.11-3
RUN set -x \
	&& export GO15VENDOREXPERIMENT=1 \
	&& export GOPATH="$(mktemp -d)" \
	&& git clone https://github.com/docker/notary.git "$GOPATH/src/github.com/docker/notary" \
	&& (cd "$GOPATH/src/github.com/docker/notary" && git checkout -q "$NOTARY_VERSION") \
	&& GOPATH="$GOPATH/src/github.com/docker/notary/vendor:$GOPATH" \
		go build -o /usr/local/bin/notary-server github.com/docker/notary/cmd/notary-server \
	&& GOPATH="$GOPATH/src/github.com/docker/notary/vendor:$GOPATH" \
		go build -o /usr/local/bin/notary github.com/docker/notary/cmd/notary \
	&& rm -rf "$GOPATH"

# Get the "docker-py" source so we can run their integration tests
ENV DOCKER_PY_COMMIT e2878cbcc3a7eef99917adc1be252800b0e41ece
RUN git clone https://github.com/docker/docker-py.git /docker-py \
	&& cd /docker-py \
	&& git checkout -q $DOCKER_PY_COMMIT \
	&& pip install -r test-requirements.txt

# Set user.email so crosbymichael's in-container merge commits go smoothly
RUN git config --global user.email 'docker-dummy@example.com'

# Add an unprivileged user to be used for tests which need it
RUN groupadd -r docker
RUN useradd --create-home --gid docker unprivilegeduser

VOLUME /var/lib/docker
WORKDIR /go/src/github.com/docker/docker
ENV DOCKER_BUILDTAGS apparmor seccomp selinux

# Let us use a .bashrc file
RUN ln -sfv $PWD/.bashrc ~/.bashrc

# Register Docker's bash completion.
RUN ln -sv $PWD/contrib/completion/bash/docker /etc/bash_completion.d/docker

# Get useful and necessary Hub images so we can "docker load" locally instead of pulling
COPY contrib/download-frozen-image-v2.sh /go/src/github.com/docker/docker/contrib/
RUN ./contrib/download-frozen-image-v2.sh /docker-frozen-images \
	buildpack-deps:jessie@sha256:25785f89240fbcdd8a74bdaf30dd5599a9523882c6dfc567f2e9ef7cf6f79db6 \
	busybox:latest@sha256:e4f93f6ed15a0cdd342f5aae387886fba0ab98af0a102da6276eaf24d6e6ade0 \
	debian:jessie@sha256:f968f10b4b523737e253a97eac59b0d1420b5c19b69928d35801a6373ffe330e \
	hello-world:latest@sha256:8be990ef2aeb16dbcb9271ddfe2610fa6658d13f6dfb8bc72074cc1ca36966a7
# see also "hack/make/.ensure-frozen-images" (which needs to be updated any time this list is)

# Download man page generator
RUN set -x \
	&& export GOPATH="$(mktemp -d)" \
	&& git clone --depth 1 -b v1.0.4 https://github.com/cpuguy83/go-md2man.git "$GOPATH/src/github.com/cpuguy83/go-md2man" \
	&& git clone --depth 1 -b v1.4 https://github.com/russross/blackfriday.git "$GOPATH/src/github.com/russross/blackfriday" \
	&& go get -v -d github.com/cpuguy83/go-md2man \
	&& go build -v -o /usr/local/bin/go-md2man github.com/cpuguy83/go-md2man \
	&& rm -rf "$GOPATH"

# Download toml validator
ENV TOMLV_COMMIT 9baf8a8a9f2ed20a8e54160840c492f937eeaf9a
RUN set -x \
	&& export GOPATH="$(mktemp -d)" \
	&& git clone https://github.com/BurntSushi/toml.git "$GOPATH/src/github.com/BurntSushi/toml" \
	&& (cd "$GOPATH/src/github.com/BurntSushi/toml" && git checkout -q "$TOMLV_COMMIT") \
	&& go build -v -o /usr/local/bin/tomlv github.com/BurntSushi/toml/cmd/tomlv \
	&& rm -rf "$GOPATH"

# Build/install the tool for embedding resources in Windows binaries
ENV RSRC_COMMIT ba14da1f827188454a4591717fff29999010887f
RUN set -x \
	&& export GOPATH="$(mktemp -d)" \
	&& git clone https://github.com/akavel/rsrc.git "$GOPATH/src/github.com/akavel/rsrc" \
	&& (cd "$GOPATH/src/github.com/akavel/rsrc" && git checkout -q "$RSRC_COMMIT") \
	&& go build -v -o /usr/local/bin/rsrc github.com/akavel/rsrc \
	&& rm -rf "$GOPATH"

# Install runc
ENV RUNC_COMMIT 40f4e7873d88a4f4d12c15d9536bb1e34aa2b7fa
RUN set -x \
	&& export GOPATH="$(mktemp -d)" \
	&& git clone https://github.com/opencontainers/runc.git "$GOPATH/src/github.com/opencontainers/runc" \
	&& cd "$GOPATH/src/github.com/opencontainers/runc" \
	&& git checkout -q "$RUNC_COMMIT" \
	&& make static BUILDTAGS="seccomp apparmor selinux" \
	&& cp runc /usr/local/bin/docker-runc

# Install containerd
ENV CONTAINERD_COMMIT 471bb075214cf0ad85f74f003ca00c7651638c79
RUN set -x \
	&& export GOPATH="$(mktemp -d)" \
	&& git clone https://github.com/docker/containerd.git "$GOPATH/src/github.com/docker/containerd" \
	&& cd "$GOPATH/src/github.com/docker/containerd" \
	&& git checkout -q "$CONTAINERD_COMMIT" \
	&& make static \
	&& cp bin/containerd /usr/local/bin/docker-containerd \
	&& cp bin/containerd-shim /usr/local/bin/docker-containerd-shim \
	&& cp bin/ctr /usr/local/bin/docker-containerd-ctr

# Wrap all commands in the "docker-in-docker" script to allow nested containers
ENTRYPOINT ["hack/dind"]

# Upload docker source
COPY . /go/src/github.com/docker/docker
