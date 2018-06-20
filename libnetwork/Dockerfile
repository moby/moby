FROM golang:1.10.2 as dev
RUN apt-get update && apt-get -y install iptables \
		protobuf-compiler

RUN go get github.com/golang/lint/golint \
		golang.org/x/tools/cmd/cover \
		github.com/mattn/goveralls \
		github.com/gordonklaus/ineffassign \
		github.com/client9/misspell/cmd/misspell \
		honnef.co/go/tools/cmd/gosimple \
		github.com/gogo/protobuf/protoc-gen-gogo

WORKDIR /go/src/github.com/docker/libnetwork

FROM dev

COPY . .
