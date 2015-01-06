FROM crosbymichael/golang

ADD . /go/src/github.com/tonistiigi/dnsdock

ENV CGO_ENABLED 0

RUN cd /go/src/github.com/tonistiigi/dnsdock && \
    go get -d ./... && \
    go install -a ./...

BUILD /go/bin

FROM scratch

ADD dnsdock .

ENTRYPOINT ["/dnsdock"]