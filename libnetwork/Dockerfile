# syntax=docker/dockerfile:1.2

ARG GO_VERSION=1.13.15
ARG BASE_DEBIAN_DISTRO="buster"
ARG GOLANG_IMAGE="golang:${GO_VERSION}-${BASE_DEBIAN_DISTRO}"

FROM ${GOLANG_IMAGE} AS dev
RUN apt-get update && apt-get -y install iptables \
		protobuf-compiler

RUN go get -d github.com/gogo/protobuf/protoc-gen-gogo && \
		cd /go/src/github.com/gogo/protobuf/protoc-gen-gogo && \
		git reset --hard 30cf7ac33676b5786e78c746683f0d4cd64fa75b && \
		go install

RUN go get golang.org/x/lint/golint \
		golang.org/x/tools/cmd/cover \
		github.com/mattn/goveralls \
		github.com/gordonklaus/ineffassign \
		github.com/client9/misspell/cmd/misspell

WORKDIR /go/src/github.com/docker/libnetwork

FROM dev

COPY . .
