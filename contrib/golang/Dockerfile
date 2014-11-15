# This file exists to make creating new golang images for building/testing
# ../../Dockerfile on different Go versions easier.

# see also:
#   https://github.com/docker-library/golang/blob/455b15a41135afc8c34487c0f142c7241bbf11ce/1.3/Dockerfile
#   https://github.com/docker-library/golang/blob/455b15a41135afc8c34487c0f142c7241bbf11ce/1.3/cross/Dockerfile

FROM debian:jessie

# SCMs for "go get", gcc for cgo
RUN apt-get update && apt-get install -y \
		ca-certificates curl gcc libc6-dev make \
		bzr git mercurial \
		--no-install-recommends \
	&& rm -rf /var/lib/apt/lists/*

ENV GOLANG_VERSION 1.3.3

RUN curl -sSL https://golang.org/dl/go$GOLANG_VERSION.src.tar.gz \
		| tar -v -C /usr/src -xz

RUN cd /usr/src/go/src && ./make.bash --no-clean 2>&1

ENV PATH /usr/src/go/bin:$PATH

RUN mkdir -p /go/src
ENV GOPATH /go
ENV PATH /go/bin:$PATH
WORKDIR /go

# see https://golang.org/doc/install/source#environment
# see also http://build.golang.org/
# and canonically, see defs_OS_ARCH.h files in src/pkg/runtime
#   https://code.google.com/p/go/source/browse?name=go1.3#hg%2Fsrc%2Fpkg%2Fruntime
ENV GOLANG_CROSSPLATFORMS \
	darwin/386 darwin/amd64 \
	dragonfly/386 dragonfly/amd64 \
	freebsd/386 freebsd/amd64 freebsd/arm \
	linux/386 linux/amd64 linux/arm \
	nacl/386 nacl/amd64p32 \
	netbsd/386 netbsd/amd64 netbsd/arm \
	openbsd/386 openbsd/amd64 \
	plan9/386 plan9/amd64 \
	solaris/amd64 \
	windows/386 windows/amd64

# ls src/pkg/runtime/defs_*_*.h | sed -r 's!^.*/defs_([^_]+)_([^_]+)[.]h$!\1/\2!'

# (set an explicit GOARM of 5 for maximum ARM compatibility)
ENV GOARM 5

RUN cd /usr/src/go/src \
	&& set -ex \
	&& for platform in $GOLANG_CROSSPLATFORMS; do \
		GOOS=${platform%/*} \
		GOARCH=${platform##*/} \
		./make.bash --no-clean 2>&1; \
	done
