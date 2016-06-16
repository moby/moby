.PHONY: all binary build build-gccgo cross default docs docs-build docs-shell shell gccgo test test-docker-py test-integration-cli test-unit validate help

# set the graph driver as the current graphdriver if not set
DOCKER_GRAPHDRIVER := $(if $(DOCKER_GRAPHDRIVER),$(DOCKER_GRAPHDRIVER),$(shell docker info 2>&1 | grep "Storage Driver" | sed 's/.*: //'))

# get OS/Arch of docker engine
DOCKER_OSARCH := $(shell bash -c 'source hack/make/.detect-daemon-osarch && echo $${DOCKER_ENGINE_OSARCH:-$$DOCKER_CLIENT_OSARCH}')
DOCKERFILE := $(shell bash -c 'source hack/make/.detect-daemon-osarch && echo $${DOCKERFILE}')

# env vars passed through directly to Docker's build scripts
# to allow things like `make KEEPBUNDLE=1 binary` easily
# `docs/sources/contributing/devenvironment.md ` and `project/PACKAGERS.md` have some limited documentation of some of these
DOCKER_ENVS := \
	-e BUILDFLAGS \
	-e KEEPBUNDLE \
	-e DOCKER_BUILD_GOGC \
	-e DOCKER_BUILD_PKGS \
	-e DOCKER_DEBUG \
	-e DOCKER_EXPERIMENTAL \
	-e DOCKER_GITCOMMIT \
	-e DOCKER_GRAPHDRIVER=$(DOCKER_GRAPHDRIVER) \
	-e DOCKER_INCREMENTAL_BINARY \
	-e DOCKER_REMAP_ROOT \
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

# This allows the test suite to be able to run without worrying about the underlying fs used by the container running the daemon (e.g. aufs-on-aufs), so long as the host running the container is running a supported fs.
# The volume will be cleaned up when the container is removed due to `--rm`.
# Note that `BIND_DIR` will already be set to `bundles` if `DOCKER_HOST` is not set (see above BIND_DIR line), in such case this will do nothing since `DOCKER_MOUNT` will already be set.
DOCKER_MOUNT := $(if $(DOCKER_MOUNT),$(DOCKER_MOUNT),-v "/go/src/github.com/docker/docker/bundles")

GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
GIT_BRANCH_CLEAN := $(shell echo $(GIT_BRANCH) | sed -e "s/[^[:alnum:]]/-/g")
DOCKER_IMAGE := docker-dev$(if $(GIT_BRANCH_CLEAN),:$(GIT_BRANCH_CLEAN))
DOCKER_DOCS_IMAGE := docker-docs$(if $(GIT_BRANCH_CLEAN),:$(GIT_BRANCH_CLEAN))

DOCKER_FLAGS := docker run --rm -i --privileged $(DOCKER_ENVS) $(DOCKER_MOUNT)

# if this session isn't interactive, then we don't want to allocate a
# TTY, which would fail, but if it is interactive, we do want to attach
# so that the user can send e.g. ^C through.
INTERACTIVE := $(shell [ -t 0 ] && echo 1 || echo 0)
ifeq ($(INTERACTIVE), 1)
	DOCKER_FLAGS += -t
endif

DOCKER_RUN_DOCKER := $(DOCKER_FLAGS) "$(DOCKER_IMAGE)"

default: binary

all: build ## validate all checks, build linux binaries, run all tests\ncross build non-linux binaries and generate archives
	$(DOCKER_RUN_DOCKER) hack/make.sh

binary: build ## build the linux binaries
	$(DOCKER_RUN_DOCKER) hack/make.sh binary

build: bundles
	docker build ${DOCKER_BUILD_ARGS} -t "$(DOCKER_IMAGE)" -f "$(DOCKERFILE)" .

build-gccgo: bundles
	docker build ${DOCKER_BUILD_ARGS} -t "$(DOCKER_IMAGE)-gccgo" -f Dockerfile.gccgo .

bundles:
	mkdir bundles

cross: build ## cross build the binaries for darwin, freebsd and\nwindows
	$(DOCKER_RUN_DOCKER) hack/make.sh dynbinary binary cross

win: build ## cross build the binary for windows
	$(DOCKER_RUN_DOCKER) hack/make.sh win

tgz: build ## build the archives (.zip on windows and .tgz\notherwise) containing the binaries
	$(DOCKER_RUN_DOCKER) hack/make.sh dynbinary binary cross tgz

deb: build  ## build the deb packages
	$(DOCKER_RUN_DOCKER) hack/make.sh dynbinary build-deb

docs: ## build the docs
	$(MAKE) -C docs docs

gccgo: build-gccgo ## build the gcc-go linux binaries
	$(DOCKER_FLAGS) "$(DOCKER_IMAGE)-gccgo" hack/make.sh gccgo

install: ## install the linux binaries
	KEEPBUNDLE=1 hack/make.sh install-binary

rpm: build ## build the rpm packages
	$(DOCKER_RUN_DOCKER) hack/make.sh dynbinary build-rpm

shell: build ## start a shell inside the build env
	$(DOCKER_RUN_DOCKER) bash

test: build ## run the unit, integration and docker-py tests
	$(DOCKER_RUN_DOCKER) hack/make.sh dynbinary cross test-unit test-integration-cli test-docker-py

test-docker-py: build ## run the docker-py tests
	$(DOCKER_RUN_DOCKER) hack/make.sh dynbinary test-docker-py

test-integration-cli: build ## run the integration tests
	$(DOCKER_RUN_DOCKER) hack/make.sh dynbinary test-integration-cli

test-unit: build ## run the unit tests
	$(DOCKER_RUN_DOCKER) hack/make.sh test-unit

validate: build ## validate DCO, Seccomp profile generation, gofmt,\n./pkg/ isolation, golint, tests, tomls, go vet and vendor 
	$(DOCKER_RUN_DOCKER) hack/make.sh validate-dco validate-default-seccomp validate-gofmt validate-pkg validate-lint validate-test validate-toml validate-vet validate-vendor

help: ## this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {sub("\\\\n",sprintf("\n%22c"," "), $$2);printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

