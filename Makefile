.PHONY: all binary build cross default docs docs-build docs-shell shell test test-unit test-integration-cli test-docker-py validate

# env vars passed through directly to Docker's build scripts
# to allow things like `make DOCKER_CLIENTONLY=1 binary` easily
# `docs/sources/contributing/devenvironment.md ` and `project/PACKAGERS.md` have some limited documentation of some of these
DOCKER_ENVS := \
	-e BUILDFLAGS \
	-e DOCKER_CLIENTONLY \
	-e DOCKER_DEBUG \
	-e DOCKER_EXECDRIVER \
	-e DOCKER_EXPERIMENTAL \
	-e DOCKER_GRAPHDRIVER \
	-e DOCKER_STORAGE_OPTS \
	-e DOCKER_USERLANDPROXY \
	-e TESTDIRS \
	-e TESTFLAGS \
	-e TIMEOUT
# note: we _cannot_ add "-e DOCKER_BUILDTAGS" here because even if it's unset in the shell, that would shadow the "ENV DOCKER_BUILDTAGS" set in our Dockerfile, which is very important for our official builds

# to allow `make BIND_DIR=. shell` or `make BIND_DIR= test`
# (default to no bind mount if DOCKER_HOST is set)
# note: BINDDIR is supported for backwards-compatibility here
BIND_DIR := $(if $(BINDDIR),$(BINDDIR),$(if $(DOCKER_HOST),,bundles))
DOCKER_MOUNT := $(if $(BIND_DIR),-v "$(CURDIR)/$(BIND_DIR):/go/src/github.com/docker/docker/$(BIND_DIR)")

# to allow `make DOCSDIR=docs docs-shell` (to create a bind mount in docs)
DOCS_MOUNT := $(if $(DOCSDIR),-v $(CURDIR)/$(DOCSDIR):/$(DOCSDIR))

# to allow `make DOCSPORT=9000 docs`
DOCSPORT := 8000

GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
DOCKER_IMAGE := docker-dev$(if $(GIT_BRANCH),:$(GIT_BRANCH))
DOCKER_DOCS_IMAGE := docker-docs$(if $(GIT_BRANCH),:$(GIT_BRANCH))

DOCKER_RUN_DOCKER := docker run --rm -it --privileged $(DOCKER_ENVS) $(DOCKER_MOUNT) "$(DOCKER_IMAGE)"

DOCKER_RUN_DOCS := docker run --rm -it $(DOCS_MOUNT) -e AWS_S3_BUCKET -e NOCACHE

# for some docs workarounds (see below in "docs-build" target)
GITCOMMIT := $(shell git rev-parse --short HEAD 2>/dev/null)

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
	$(DOCKER_RUN_DOCS) -e OPTIONS -e BUILD_ROOT -e DISTRIBUTION_ID \
		-v $(CURDIR)/docs/awsconfig:/docs/awsconfig \
		"$(DOCKER_DOCS_IMAGE)" ./release.sh

docs-test: docs-build
	$(DOCKER_RUN_DOCS) "$(DOCKER_DOCS_IMAGE)" ./test.sh

test: build
	$(DOCKER_RUN_DOCKER) hack/make.sh binary cross test-unit test-integration-cli test-docker-py

test-unit: build
	$(DOCKER_RUN_DOCKER) hack/make.sh test-unit

test-integration-cli: build
	$(DOCKER_RUN_DOCKER) hack/make.sh binary test-integration-cli

test-docker-py: build
	$(DOCKER_RUN_DOCKER) hack/make.sh binary test-docker-py

validate: build
	$(DOCKER_RUN_DOCKER) hack/make.sh validate-dco validate-gofmt validate-test validate-toml validate-vet

shell: build
	$(DOCKER_RUN_DOCKER) bash

build: bundles
	docker build -t "$(DOCKER_IMAGE)" .

docs-build:
	cp ./VERSION docs/VERSION
	echo "$(GIT_BRANCH)" > docs/GIT_BRANCH
#	echo "$(AWS_S3_BUCKET)" > docs/AWS_S3_BUCKET
	echo "$(GITCOMMIT)" > docs/GITCOMMIT
	docker pull docs/base
	docker build -t "$(DOCKER_DOCS_IMAGE)" docs

bundles:
	mkdir bundles
