.PHONY: all binary build cross default docs docs-build docs-shell shell test test-unit test-integration test-integration-cli validate

# to allow `make BINDDIR=. shell` or `make BINDDIR= test`
# (default to no bind mount if DOCKER_HOST is set)
BINDDIR := $(if $(DOCKER_HOST),,bundles)
# to allow `make DOCSPORT=9000 docs`
DOCSPORT := 8000

GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
GITCOMMIT := $(shell git rev-parse --short HEAD 2>/dev/null)
DOCKER_IMAGE := docker$(if $(GIT_BRANCH),:$(GIT_BRANCH))
DOCKER_DOCS_IMAGE := docker-docs$(if $(GIT_BRANCH),:$(GIT_BRANCH))
DOCKER_MOUNT := $(if $(BINDDIR),-v "$(CURDIR)/$(BINDDIR):/go/src/github.com/docker/docker/$(BINDDIR)")

DOCKER_RUN_DOCKER := docker run --rm -it --privileged -e TESTFLAGS -e TESTDIRS -e DOCKER_GRAPHDRIVER -e DOCKER_EXECDRIVER $(DOCKER_MOUNT) "$(DOCKER_IMAGE)"
# to allow `make DOCSDIR=docs docs-shell`
DOCKER_RUN_DOCS := docker run --rm -it $(if $(DOCSDIR),-v $(CURDIR)/$(DOCSDIR):/$(DOCSDIR)) -e AWS_S3_BUCKET

default: binary

all: build
	$(DOCKER_RUN_DOCKER) hack/make.sh

binary: build
	$(DOCKER_RUN_DOCKER) hack/make.sh binary

cross: build
	$(DOCKER_RUN_DOCKER) hack/make.sh binary cross

docs: docs-build
	$(DOCKER_RUN_DOCS) -p $(if $(DOCSPORT),$(DOCSPORT):)8000 "$(DOCKER_DOCS_IMAGE)" mkdocs serve

docs-shell: docs-build
	$(DOCKER_RUN_DOCS) -p $(if $(DOCSPORT),$(DOCSPORT):)8000 "$(DOCKER_DOCS_IMAGE)" bash

docs-release: docs-build
	$(DOCKER_RUN_DOCS) -e BUILD_ROOT "$(DOCKER_DOCS_IMAGE)" ./release.sh

test: build
	$(DOCKER_RUN_DOCKER) hack/make.sh binary cross test-unit test-integration test-integration-cli

test-unit: build
	$(DOCKER_RUN_DOCKER) hack/make.sh test-unit

test-integration: build
	$(DOCKER_RUN_DOCKER) hack/make.sh test-integration

test-integration-cli: build
	$(DOCKER_RUN_DOCKER) hack/make.sh binary test-integration-cli

validate: build
	$(DOCKER_RUN_DOCKER) hack/make.sh validate-gofmt validate-dco

shell: build
	$(DOCKER_RUN_DOCKER) bash

build: bundles
	docker build -t "$(DOCKER_IMAGE)" .

docs-build:
	cp ./VERSION docs/VERSION
	echo "$(GIT_BRANCH)" > docs/GIT_BRANCH
	echo "$(AWS_S3_BUCKET)" > docs/AWS_S3_BUCKET
	echo "$(GITCOMMIT)" > docs/GITCOMMIT
	docker build -t "$(DOCKER_DOCS_IMAGE)" docs

bundles:
	mkdir bundles
