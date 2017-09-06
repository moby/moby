BUILDTAGS=

PROJECT=github.com/containerd/containerd

GIT_COMMIT := $(shell git rev-parse HEAD 2> /dev/null || true)
GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2> /dev/null)

LDFLAGS := -X github.com/containerd/containerd.GitCommit=${GIT_COMMIT} ${LDFLAGS}

TEST_TIMEOUT ?= 5m
TEST_SUITE_TIMEOUT ?= 10m

RUNTIME ?= runc

# if this session isn't interactive, then we don't want to allocate a
# TTY, which would fail, but if it is interactive, we do want to attach
# so that the user can send e.g. ^C through.
INTERACTIVE := $(shell [ -t 0 ] && echo 1 || echo 0)
ifeq ($(INTERACTIVE), 1)
	DOCKER_FLAGS += -t
endif

TESTBENCH_ARTIFACTS_DIR := output/test-artifacts
TESTBENCH_BUNDLE_DIR := $(TESTBENCH_ARTIFACTS_DIR)/archives

DOCKER_IMAGE := containerd-dev$(if $(GIT_BRANCH),:$(GIT_BRANCH))
DOCKER_RUN := docker run --privileged --rm -i $(DOCKER_FLAGS) "$(DOCKER_IMAGE)"


export GOPATH:=$(CURDIR)/vendor:$(GOPATH)

.PHONY: integration-test

all: client daemon shim

static: client-static daemon-static shim-static

bin:
	mkdir -p bin/

clean:
	rm -rf bin && rm -rf output

client: bin
	cd ctr && go build -ldflags "${LDFLAGS}" -o ../bin/ctr

client-static:
	cd ctr && go build -ldflags "-w -extldflags -static ${LDFLAGS}" -tags "$(BUILDTAGS)" -o ../bin/ctr

daemon: bin
	cd containerd && go build -ldflags "${LDFLAGS}"  -tags "$(BUILDTAGS)" -o ../bin/containerd

daemon-static:
	cd containerd && go build -ldflags "-w -extldflags -static ${LDFLAGS}" -tags "$(BUILDTAGS)" -o ../bin/containerd

shim: bin
	cd containerd-shim && go build -tags "$(BUILDTAGS)" -ldflags "-w ${LDFLAGS}" -o ../bin/containerd-shim

shim-static:
	cd containerd-shim && go build -ldflags "-w -extldflags -static ${LDFLAGS}" -tags "$(BUILDTAGS)" -o ../bin/containerd-shim

$(TESTBENCH_BUNDLE_DIR)/busybox.tar:
	mkdir -p $(TESTBENCH_BUNDLE_DIR)
	curl -sSL 'https://github.com/jpetazzo/docker-busybox/raw/buildroot-2014.11/rootfs.tar' -o $(TESTBENCH_BUNDLE_DIR)/busybox.tar

bundles-rootfs: $(TESTBENCH_BUNDLE_DIR)/busybox.tar

dbuild: $(TESTBENCH_BUNDLE_DIR)/busybox.tar
	@docker build --rm --force-rm -t "$(DOCKER_IMAGE)" .

dtest: dbuild
	$(DOCKER_RUN) make test

dbench: dbuild
	$(DOCKER_RUN) make bench

install:
	cp bin/* /usr/local/bin/

protoc:
	protoc -I ./api/grpc/types ./api/grpc/types/api.proto --go_out=plugins=grpc:api/grpc/types

fmt:
	@gofmt -s -l . | grep -v vendor | grep -v .pb. | tee /dev/stderr

lint:
	@hack/validate-lint

shell: dbuild
	$(DOCKER_RUN) bash

test: validate install bundles-rootfs
	go test -bench=. -v $(shell go list ./... | grep -v /vendor | grep -v /integration-test ) -runtime=$(RUNTIME)
ifneq ($(wildcard /.dockerenv), )
	cd integration-test ; \
go test -check.v -check.timeout=$(TEST_TIMEOUT) $(TESTFLAGS) timeout=$(TEST_SUITE_TIMEOUT) github.com/containerd/containerd/integration-test
endif

integration-test:
	cd integration-test ; \
go test -check.v -check.timeout=$(TEST_TIMEOUT) $(TESTFLAGS) timeout=$(TEST_SUITE_TIMEOUT) github.com/containerd/containerd/integration-test

bench: shim validate install bundles-rootfs
	go test -bench=. -v $(shell go list ./... | grep -v /vendor | grep -v /integration-test) -runtime=$(RUNTIME)

validate: fmt lint

uninstall:
	$(foreach file,containerd containerd-shim ctr,rm /usr/local/bin/$(file);)
