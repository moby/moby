.PHONY: lint test test_race build_cross_os

default: lint test build_cross_os

test:
	go test -v -cover ./...

test_race:
	CGO_ENABLED=1 go test -v -race ./...

lint:
	golangci-lint run

build_cross_os:
	./build.sh
