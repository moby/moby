FROM    alpine:3.4

RUN     apk add -U git go bash curl gcc musl-dev make

RUN     mkdir -p /go/src /go/bin /go/pkg
ENV     GOPATH=/go
ENV     PATH=$PATH:/go/bin
RUN     go get -u github.com/LK4D4/vndr

COPY    vendor.conf /yamlvendor/
WORKDIR /yamlvendor/
RUN     vndr
ENV     GOPATH=$GOPATH:/go/src/github.com/docker/docker/vendor:/yamlvendor

WORKDIR /go/src/github.com/docker/docker/
ENTRYPOINT ["yaml/generate.sh"]
