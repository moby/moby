.PHONY: all all-local build build-local check check-code check-format run-tests check-local install-deps

docker = docker run --rm --privileged -v $(shell pwd):/go/src/github.com/docker/libnetwork -w /go/src/github.com/docker/libnetwork golang:1.4

all: 
	${docker} make all-local

all-local: 	install-deps check-local build-local

build: 
	${docker} make install-deps build-local

build-local:
	$(shell which godep) go build ./...

check:
	${docker} make install-deps check-local

check-code:
	test -z "$$(golint ./... | tee /dev/stderr)"
	go vet ./...

check-format:
	test -z "$$(shell goimports -l . | grep -v Godeps/_workspace/src/ | tee /dev/stderr)"

run-tests:
	$(shell which godep) go test -test.v ./...

check-local: 	check-format check-code run-tests 

install-deps:
	apt-get update && apt-get -y install iptables
	go get github.com/tools/godep
	go get github.com/golang/lint/golint
	go get golang.org/x/tools/cmd/vet
	go get golang.org/x/tools/cmd/goimports
