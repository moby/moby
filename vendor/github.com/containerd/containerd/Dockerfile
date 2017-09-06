FROM debian:jessie

# allow replacing httpredir mirror
ARG APT_MIRROR=httpredir.debian.org
RUN sed -i s/httpredir.debian.org/$APT_MIRROR/g /etc/apt/sources.list

RUN apt-get update && apt-get install -y \
	build-essential \
	ca-certificates \
	curl \
	make \
	jq \
	pkg-config \
	apparmor \
	libapparmor-dev \
	--no-install-recommends \
	&& rm -rf /var/lib/apt/lists/*

# add git repo
RUN apt-key adv --keyserver hkp://p80.pool.sks-keyservers.net:80 --recv-keys E1DD270288B4E6030699E45FA1715D88E1DF1F24
RUN echo deb http://ppa.launchpad.net/git-core/ppa/ubuntu trusty main > /etc/apt/sources.list.d/git.list

RUN apt-get update && apt-get install -y git
# Install Go
ENV GO_VERSION 1.8.1
RUN curl -sSL  "https://storage.googleapis.com/golang/go${GO_VERSION}.linux-amd64.tar.gz" | tar -v -C /usr/local -xz
ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go:/go/src/github.com/containerd/containerd/vendor

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

WORKDIR /go/src/github.com/containerd/containerd

# install seccomp: the version shipped in trusty is too old
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

# Install runc
ENV RUNC_COMMIT 992a5be178a62e026f4069f443c6164912adbf09
RUN set -x \
	&& export GOPATH="$(mktemp -d)" \
    && git clone git://github.com/opencontainers/runc.git "$GOPATH/src/github.com/opencontainers/runc" \
	&& cd "$GOPATH/src/github.com/opencontainers/runc" \
	&& git checkout -q "$RUNC_COMMIT" \
	&& make BUILDTAGS="seccomp apparmor selinux" && make install

COPY . /go/src/github.com/containerd/containerd

WORKDIR /go/src/github.com/containerd/containerd

RUN make all install
