.PHONY: all all-local build build-local clean cross cross-local gosimple vet lint misspell check check-local check-code check-format unit-tests protobuf protobuf-local check-protobuf
SHELL=/bin/bash

dockerbuildargs ?= --target dev - < Dockerfile
dockerargs ?= --privileged -v $(shell pwd):/go/src/github.com/docker/libnetwork -w /go/src/github.com/docker/libnetwork
build_image=libnetworkbuild
container_env = -e "INSIDECONTAINER=-incontainer=true"
docker = docker run --rm -it --init ${dockerargs} $$EXTRA_ARGS ${container_env} ${build_image}

CROSS_PLATFORMS = linux/amd64 linux/386 linux/arm windows/amd64
PACKAGES=$(shell go list ./... | grep -v /vendor/)
PROTOC_FLAGS=-I=. -I=/go/src -I=/go/src/github.com/gogo/protobuf -I=/go/src/github.com/gogo/protobuf/protobuf

export PATH := $(CURDIR)/bin:$(PATH)


# Several targets in this Makefile expect to run within the
# libnetworkbuild container.   In general, a target named '<target>-local'
# relies on utilities inside the build container.   Usually there is also
# a wrapper called '<target>' which starts a container and runs
# 'make <target>-local' inside it.

###########################################################################
# Top level targets
###########################################################################

all: build check clean

all-local: build-local check-local clean


###########################################################################
# Build targets
###########################################################################

# builder builds the libnetworkbuild container.  All wrapper targets
# must depend on this to ensure that the container exists.
builder:
	docker build -t ${build_image} ${dockerbuildargs}

build: builder
	@echo "🐳 $@"
	@${docker} make build-local

build-local:
	@echo "🐳 $@"
	@mkdir -p "bin"
	go build -tags experimental -o "bin/dnet" ./cmd/dnet
	go build -o "bin/docker-proxy" ./cmd/proxy
	CGO_ENABLED=0 go build -o "bin/diagnosticClient" ./cmd/diagnostic
	CGO_ENABLED=0 go build -o "bin/testMain" ./cmd/networkdb-test/testMain.go

build-images:
	@echo "🐳 $@"
	cp cmd/diagnostic/daemon.json ./bin
	docker build -f cmd/diagnostic/Dockerfile.client -t dockereng/network-diagnostic:onlyclient bin/
	docker build -f cmd/diagnostic/Dockerfile.dind -t dockereng/network-diagnostic:17.12-dind bin/
	docker build -f cmd/networkdb-test/Dockerfile -t dockereng/e2e-networkdb:master bin/
	docker build -t dockereng/network-diagnostic:support.sh support/

push-images: build-images
	@echo "🐳 $@"
	docker push dockereng/network-diagnostic:onlyclient
	docker push dockereng/network-diagnostic:17.12-dind
	docker push dockereng/e2e-networkdb:master
	docker push dockereng/network-diagnostic:support.sh

clean:
	@echo "🐳 $@"
	@if [ -d bin ]; then \
		echo "Removing binaries"; \
		rm -rf bin; \
	fi

cross: builder
	@mkdir -p "bin"
	@for platform in ${CROSS_PLATFORMS}; do \
		EXTRA_ARGS="-e GOOS=$${platform%/*} -e GOARCH=$${platform##*/}" ; \
		echo "$${platform}..." ; \
		${docker} make cross-local ; \
	done

cross-local:
	@echo "🐳 $@"
	go build -o "bin/dnet-$$GOOS-$$GOARCH" ./cmd/dnet
	go build -o "bin/docker-proxy-$$GOOS-$$GOARCH" ./cmd/proxy

# Rebuild protocol buffers.
# These may need to be rebuilt after vendoring updates, so .proto files are declared .PHONY so they are always rebuilt.
PROTO_FILES=$(shell find . -path ./vendor -prune -o -name \*.proto -print)
PB_FILES=$(PROTO_FILES:.proto=.pb.go)

# Pattern rule for protoc.   If PROTOC_CHECK is defined, it checks
# whether the generated files are up to date and fails if they are not
%.pb.go: %.proto
	@if [ ${PROTOC_CHECK} ]; then \
	protoc ${PROTOC_FLAGS} --gogo_out=/tmp $< ; \
	diff -q $@ /tmp/$@ >/dev/null || (echo "👹 $@ is out of date; please run 'make protobuf' and check in updates" && exit 1) ; \
	else \
	protoc ${PROTOC_FLAGS} --gogo_out=./ $< ; \
	fi

.PHONY: $(PROTO_FILES)
protobuf: builder
	@${docker} make protobuf-local
protobuf-local: $(PB_FILES)


###########################################################################
# Test targets
###########################################################################

check: builder
	@${docker} make check-local

check-local: check-code check-format

check-code: check-protobuf lint gosimple vet ineffassign

check-format: fmt misspell

unit-tests: builder
	${docker} make unit-tests-local

unit-tests-local:
	@echo "🐳 Running tests... "
	@echo "mode: count" > coverage.coverprofile
	@go build -o "bin/docker-proxy" ./cmd/proxy
	@for dir in $$( find . -maxdepth 10 -not -path './.git*' -not -path '*/_*' -not -path './vendor/*' -type d); do \
	if ls $$dir/*.go &> /dev/null; then \
		pushd . &> /dev/null ; \
		cd $$dir ; \
		go test ${INSIDECONTAINER} -test.parallel 5 -test.v -covermode=count -coverprofile=./profile.tmp ; \
		ret=$$? ;\
		if [ $$ret -ne 0 ]; then exit $$ret; fi ;\
		popd &> /dev/null; \
		if [ -f $$dir/profile.tmp ]; then \
			cat $$dir/profile.tmp | tail -n +2 >> coverage.coverprofile ; \
				rm $$dir/profile.tmp ; \
	    fi ; \
	fi ; \
	done
	@echo "Done running tests"

# Depends on binaries because vet will silently fail if it can not load compiled imports
vet: ## run go vet
	@echo "🐳 $@"
	@test -z "$$(go vet ${PACKAGES} 2>&1 | grep -v 'constant [0-9]* not a string in call to Errorf' | egrep -v '(timestamp_test.go|duration_test.go|exit status 1)' | tee /dev/stderr)"

misspell:
	@echo "🐳 $@"
	@test -z "$$(find . -type f | grep -v vendor/ | grep "\.go\|\.md" | xargs misspell -error | tee /dev/stderr)"

fmt: ## run go fmt
	@echo "🐳 $@"
	@test -z "$$(gofmt -s -l . | grep -v vendor/ | grep -v ".pb.go$$" | tee /dev/stderr)" || \
		(echo "👹 please format Go code with 'gofmt -s -w'" && false)

lint: ## run go lint
	@echo "🐳 $@"
	@test -z "$$(golint ./... | grep -v vendor/ | grep -v ".pb.go:" | grep -v ".mock.go" | tee /dev/stderr)"

ineffassign: ## run ineffassign
	@echo "🐳 $@"
	@test -z "$$(ineffassign . | grep -v vendor/ | grep -v ".pb.go:" | grep -v ".mock.go" | tee /dev/stderr)"

gosimple: ## run gosimple
	@echo "🐳 $@"
	@test -z "$$(gosimple . | grep -v vendor/ | grep -v ".pb.go:" | grep -v ".mock.go" | tee /dev/stderr)"

# check-protobuf rebuilds .pb.go files and fails if they have changed
check-protobuf: PROTOC_CHECK=1
check-protobuf: $(PB_FILES)
	@echo "🐳 $@"


###########################################################################
# Utility targets
###########################################################################

shell: builder
	@${docker} ${SHELL}
