.PHONY: all binary build cross default docs docs-build docs-shell shell test test-integration

GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
DOCKER_IMAGE := docker:$(GIT_BRANCH)
DOCKER_DOCS_IMAGE := docker-docs:$(GIT_BRANCH)
DOCKER_RUN_DOCKER := docker run -rm -i -t -privileged -e TESTFLAGS -v $(CURDIR)/bundles:/go/src/github.com/dotcloud/docker/bundles "$(DOCKER_IMAGE)"

default: binary

all: build
	$(DOCKER_RUN_DOCKER) hack/make.sh

binary: build
	$(DOCKER_RUN_DOCKER) hack/make.sh binary

cross: build
	$(DOCKER_RUN_DOCKER) hack/make.sh binary cross

docs: docs-build
	docker run -rm -i -t -p 8000:8000 "$(DOCKER_DOCS_IMAGE)"

docs-shell: docs-build
	docker run -rm -i -t -p 8000:8000 "$(DOCKER_DOCS_IMAGE)" bash

test: build
	$(DOCKER_RUN_DOCKER) hack/make.sh test test-integration

test-integration: build
	$(DOCKER_RUN_DOCKER) hack/make.sh test-integration

shell: build
	$(DOCKER_RUN_DOCKER) bash

build: bundles
	docker build -rm -t "$(DOCKER_IMAGE)" .

docs-build:
	docker build -rm -t "$(DOCKER_DOCS_IMAGE)" docs

bundles:
	mkdir bundles
