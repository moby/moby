.PHONY: all binary build default docs shell test

DOCKER_RUN_DOCKER := docker run -rm -i -t -privileged -e TESTFLAGS -v $(CURDIR)/bundles:/go/src/github.com/dotcloud/docker/bundles docker

default: binary

all: build
	$(DOCKER_RUN_DOCKER) hack/make.sh

binary: build
	$(DOCKER_RUN_DOCKER) hack/make.sh binary

docs:
	docker build -t docker-docs docs && docker run -p 8000:8000 docker-docs

test: build
	$(DOCKER_RUN_DOCKER) hack/make.sh test

shell: build
	$(DOCKER_RUN_DOCKER) bash

build: bundles
	docker build -t docker .

bundles:
	mkdir bundles
