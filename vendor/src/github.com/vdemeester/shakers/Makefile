.PHONY: all

SHAKERS_MOUNT := $(if $(BIND_DIR),-v "$(CURDIR)/$(BIND_DIR):/go/src/github.com/vdemeester/shakers/$(BIND_DIR)")

GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
SHAKERS_DEV_IMAGE := shakers-dev$(if $(GIT_BRANCH),:$(GIT_BRANCH))

DOCKER_RUN_SHAKERS := docker run $(if $(CIRCLECI),,--rm) -it $(SHAKERS_ENVS) $(SHAKERS_MOUNT) "$(SHAKERS_DEV_IMAGE)"

print-%: ; @echo $*=$($*)

default: binary

binary: build
	$(DOCKER_RUN_SHAKERS) ./script/make.sh binary

test-unit: build
	$(DOCKER_RUN_SHAKERS) ./script/make.sh test-unit

validate: build
	$(DOCKER_RUN_SHAKERS) ./script/make.sh validate-gofmt validate-golint validate-govet

validate-govet: build
	$(DOCKER_RUN_SHAKERS) ./script/make.sh validate-govet

validate-golint: build
	$(DOCKER_RUN_SHAKERS) ./script/make.sh validate-golint

validate-gofmt: build
	$(DOCKER_RUN_SHAKERS) ./script/make.sh validate-gofmt

build:
	docker build -t "$(SHAKERS_DEV_IMAGE)" .

shell: build
	$(DOCKER_RUN_SHAKERS) /bin/bash

run-dev:
	go build
	./traefik
