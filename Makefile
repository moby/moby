DOCKER_PACKAGE := github.com/dotcloud/docker
RELEASE_VERSION := $(shell git tag | grep -E "v[0-9\.]+$$" | sort -nr | head -n 1)
SRCRELEASE := docker-$(RELEASE_VERSION)
BINRELEASE := docker-$(RELEASE_VERSION).tgz
BUILD_SRC := build_src
BUILD_PATH := ${BUILD_SRC}/src/${DOCKER_PACKAGE}

GIT_ROOT := $(shell git rev-parse --show-toplevel)
BUILD_DIR := $(CURDIR)/.gopath

GOPATH ?= $(BUILD_DIR)
export GOPATH

GO_OPTIONS ?= -a -ldflags='-w -d'
ifeq ($(VERBOSE), 1)
GO_OPTIONS += -v
endif

GIT_COMMIT = $(shell git rev-parse --short HEAD)
GIT_STATUS = $(shell test -n "`git status --porcelain`" && echo "+CHANGES")

BUILD_OPTIONS = -a -ldflags "-X main.GITCOMMIT $(GIT_COMMIT)$(GIT_STATUS) -d -w"

SRC_DIR := $(GOPATH)/src

DOCKER_DIR := $(SRC_DIR)/$(DOCKER_PACKAGE)
DOCKER_MAIN := $(DOCKER_DIR)/docker

DOCKER_BIN_RELATIVE := bin/docker
DOCKER_BIN := $(CURDIR)/$(DOCKER_BIN_RELATIVE)

.PHONY: all clean test hack release srcrelease $(BINRELEASE) $(SRCRELEASE) $(DOCKER_BIN) $(DOCKER_DIR)

all: $(DOCKER_BIN)

$(DOCKER_BIN): $(DOCKER_DIR)
	@mkdir -p  $(dir $@)
	@(cd $(DOCKER_MAIN); CGO_ENABLED=0 go build $(GO_OPTIONS) $(BUILD_OPTIONS) -o $@)
	@echo $(DOCKER_BIN_RELATIVE) is created.

$(DOCKER_DIR):
	@mkdir -p $(dir $@)
	@if [ -h $@ ]; then rm -f $@; fi; ln -sf $(CURDIR)/ $@
	@(cd $(DOCKER_MAIN); go get -d $(GO_OPTIONS))

whichrelease:
	echo $(RELEASE_VERSION)

release: $(BINRELEASE)
	s3cmd -P put $(BINRELEASE) s3://get.docker.io/builds/`uname -s`/`uname -m`/docker-$(RELEASE_VERSION).tgz
	s3cmd -P put docker-latest.tgz s3://get.docker.io/builds/`uname -s`/`uname -m`/docker-latest.tgz
	s3cmd -P put $(SRCRELEASE)/bin/docker s3://get.docker.io/builds/`uname -s`/`uname -m`/docker
	echo $(RELEASE_VERSION) > latest ; s3cmd -P put latest s3://get.docker.io/latest ; rm latest

srcrelease: $(SRCRELEASE)
deps: $(DOCKER_DIR)

# A clean checkout of $RELEASE_VERSION, with vendored dependencies
$(SRCRELEASE):
	rm -fr $(SRCRELEASE)
	git clone $(GIT_ROOT) $(SRCRELEASE)
	cd $(SRCRELEASE); git checkout -q $(RELEASE_VERSION)

# A binary release ready to be uploaded to a mirror
$(BINRELEASE): $(SRCRELEASE)
	rm -f $(BINRELEASE)
	cd $(SRCRELEASE); make; cp -R bin docker-$(RELEASE_VERSION); tar -f ../$(BINRELEASE) -zv -c docker-$(RELEASE_VERSION)
	cd $(SRCRELEASE); cp -R bin docker-latest; tar -f ../docker-latest.tgz -zv -c docker-latest
clean:
	@rm -rf $(dir $(DOCKER_BIN))
ifeq ($(GOPATH), $(BUILD_DIR))
	@rm -rf $(BUILD_DIR)
else ifneq ($(DOCKER_DIR), $(realpath $(DOCKER_DIR)))
	@rm -f $(DOCKER_DIR)
endif

test:
	# Copy docker source and dependencies for testing
	rm -rf ${BUILD_SRC}; mkdir -p ${BUILD_PATH}
	tar --exclude=${BUILD_SRC} -cz . | tar -xz -C ${BUILD_PATH}
	GOPATH=${CURDIR}/${BUILD_SRC} go get -d
	# Do the test
	sudo -E GOPATH=${CURDIR}/${BUILD_SRC} CGO_ENABLED=0 go test ${GO_OPTIONS}

testall: all
	@(cd $(DOCKER_DIR); CGO_ENABLED=0 sudo -E go test ./... $(GO_OPTIONS))

fmt:
	@gofmt -s -l -w .

hack:
	cd $(CURDIR)/hack && vagrant up

ssh-dev:
	cd $(CURDIR)/hack && vagrant ssh
