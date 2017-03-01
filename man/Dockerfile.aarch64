FROM    aarch64/ubuntu:xenial

RUN     apt-get update && apt-get install -y git golang-go curl

ENV     GO_VERSION 1.7.5
ENV     GOARCH arm64
ENV     PATH /go/bin:/usr/src/go/bin:$PATH

RUN     mkdir /usr/src/go && \
        curl -fsSL https://golang.org/dl/go${GO_VERSION}.src.tar.gz | tar -v -C /usr/src/go -xz --strip-components=1 && \
        cd /usr/src/go/src && \
        GOOS=linux GOARCH=arm64 GOROOT_BOOTSTRAP="$(go env GOROOT)" ./make.bash

RUN     mkdir -p /go/src /go/bin /go/pkg
ENV     GOPATH=/go
RUN     export GLIDE=v0.11.1; \
        export TARGET=/go/src/github.com/Masterminds; \
        mkdir -p ${TARGET} && \
        git clone https://github.com/Masterminds/glide.git ${TARGET}/glide && \
        cd ${TARGET}/glide && \
        git checkout $GLIDE && \
        make build && \
        cp ./glide /usr/bin/glide && \
        cd / && rm -rf /go/src/* /go/bin/* /go/pkg/*

COPY    glide.yaml /manvendor/
COPY    glide.lock /manvendor/
WORKDIR /manvendor/
RUN     glide install && mv vendor src
ENV     GOPATH=$GOPATH:/manvendor
RUN     go build -o /usr/bin/go-md2man github.com/cpuguy83/go-md2man

WORKDIR /go/src/github.com/docker/docker/
ENTRYPOINT ["man/generate.sh"]
