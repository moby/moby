FROM    ppc64le/ubuntu:xenial

RUN     apt-get update && apt-get install -y \
        curl \
        gcc \
        git \
        make \
        tar

ENV     GO_VERSION 1.7.5
RUN     curl -fsSL "https://golang.org/dl/go${GO_VERSION}.linux-ppc64le.tar.gz" \
        | tar -xzC /usr/local
ENV     PATH /usr/local/go/bin:$PATH
ENV     GOPATH=/go

RUN     mkdir -p /go/src /go/bin /go/pkg
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
