.PHONY: all binary build cross default docs shell test

DOCKER_RUN_DOCKER := docker run -rm -i -t -privileged -e TESTFLAGS -v $(CURDIR)/bundles:/go/src/github.com/dotcloud/docker/bundles docker_dev_img

default: binary

all: build
	$(DOCKER_RUN_DOCKER) hack/make.sh

binary: build
	$(DOCKER_RUN_DOCKER) hack/make.sh binary

cross: build
	$(DOCKER_RUN_DOCKER) hack/make.sh binary cross

docs:
	docker build -t docker-docs docs && docker run -p 8000:8000 docker-docs

test: build
	$(DOCKER_RUN_DOCKER) hack/make.sh test test-integration

shell: build
	$(DOCKER_RUN_DOCKER) bash

build: bundles
	docker images | awk '{print $1}' | grep docker_dev_img || docker build -t docker_dev_img .

bundles:
	mkdir -p bundles
