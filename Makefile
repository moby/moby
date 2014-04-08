.PHONY: all binary build cross default docs docs-build docs-shell shell test test-integration test-integration-cli

# to allow `make BINDDIR=. shell` or `make BINDDIR= test`
BINDDIR := bundles
# to allow `make DOCSPORT=9000 docs`
DOCSPORT := 8000

GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
DOCKER_IMAGE := docker$(if $(GIT_BRANCH),:$(GIT_BRANCH))
DOCKER_DOCS_IMAGE := docker-docs$(if $(GIT_BRANCH),:$(GIT_BRANCH))
DOCKER_MOUNT := $(if $(BINDDIR),-v "$(CURDIR)/$(BINDDIR):/go/src/github.com/dotcloud/docker/$(BINDDIR)")

DOCKER_RUN_DOCKER := docker run --rm -it --privileged -e TESTFLAGS -e DOCKER_GRAPHDRIVER -e DOCKER_EXECDRIVER $(DOCKER_MOUNT) "$(DOCKER_IMAGE)"
DOCKER_RUN_DOCS := docker run --rm -it -p $(if $(DOCSPORT),$(DOCSPORT):)8000 "$(DOCKER_DOCS_IMAGE)"

default: binary

all: build
	$(DOCKER_RUN_DOCKER) hack/make.sh

binary: build
	$(DOCKER_RUN_DOCKER) hack/make.sh binary

cross: build
	$(DOCKER_RUN_DOCKER) hack/make.sh binary cross

docs: docs-build
	$(DOCKER_RUN_DOCS)

docs-shell: docs-build
	$(DOCKER_RUN_DOCS) bash

test: build
	$(DOCKER_RUN_DOCKER) hack/make.sh binary test test-integration test-integration-cli

test-integration: build
	$(DOCKER_RUN_DOCKER) hack/make.sh test-integration

test-integration-cli: build
	$(DOCKER_RUN_DOCKER) hack/make.sh binary test-integration-cli

shell: build
	$(DOCKER_RUN_DOCKER) bash

build: bundles
	docker build -t "$(DOCKER_IMAGE)" .

docs-build:
	docker build -t "$(DOCKER_DOCS_IMAGE)" docs

bundles:
	mkdir bundles
