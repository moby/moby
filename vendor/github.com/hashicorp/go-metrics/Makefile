MAKEFLAGS += --warn-undefined-variables
SHELL := /bin/bash
.SHELLFLAGS := -o pipefail -euc
.DEFAULT_GOAL := test

.PHONY: test cov testrace check lint copywriteheaders tidy

# test runs the test suite
test:
	go test -count=1 ./...

# cov runs tests with a coverage profile
cov:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out

# testrace runs the race checker
testrace:
	go test -count=1 -race ./... $(TESTARGS)

# check runs all the linters and custom checks
check: lint tidy copywriteheaders

# lint covers go vet and go fmt
lint:
	golangci-lint run --build-tags "$(GO_TAGS)"

# make sure our copyright headers are correct
copywriteheaders:
	copywrite headers --plan

# make sure go.mod/sum are up to date
tidy:
	go mod tidy
	@if (git status --porcelain | grep -Eq "go\.(mod|sum)"); then \
		echo go.mod or go.sum needs updating; \
		git --no-pager diff go.mod; \
		git --no-pager diff go.sum; \
		exit 1; fi
