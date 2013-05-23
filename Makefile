DOCKER_PACKAGE := github.com/dotcloud/docker
RELEASE_VERSION := $(shell git tag | grep -E "v[0-9\.]+$$" | sort -nr | head -n 1)
SRCRELEASE := docker-$(RELEASE_VERSION)
BINRELEASE := docker-$(RELEASE_VERSION).tgz

GIT_ROOT := $(shell git rev-parse --show-toplevel)
BUILD_DIR := $(CURDIR)/.gopath

GOPATH ?= $(BUILD_DIR)
export GOPATH

GO_OPTIONS ?=
ifeq ($(VERBOSE), 1)
GO_OPTIONS += -v
endif

GIT_COMMIT = $(shell git rev-parse --short HEAD)
GIT_STATUS = $(shell test -n "`git status --porcelain`" && echo "+CHANGES")

BUILD_OPTIONS = -ldflags "-X main.GIT_COMMIT $(GIT_COMMIT)$(GIT_STATUS)"

SRC_DIR := $(GOPATH)/src

DOCKER_DIR := $(SRC_DIR)/$(DOCKER_PACKAGE)
DOCKER_MAIN := $(DOCKER_DIR)/docker

DOCKER_BIN_RELATIVE := bin/docker
DOCKER_BIN := $(CURDIR)/$(DOCKER_BIN_RELATIVE)

.PHONY: all clean test hack release srcrelease $(BINRELEASE) $(SRCRELEASE) $(DOCKER_BIN) $(DOCKER_DIR)

all: $(DOCKER_BIN)

$(DOCKER_BIN): $(DOCKER_DIR)
	@mkdir -p  $(dir $@)
	@(cd $(DOCKER_MAIN); go build $(GO_OPTIONS) $(BUILD_OPTIONS) -o $@)
	@echo $(DOCKER_BIN_RELATIVE) is created.

$(DOCKER_DIR):
	@mkdir -p $(dir $@)
	@if [ -h $@ ]; then rm -f $@; fi; ln -sf $(CURDIR)/ $@
	@(cd $(DOCKER_MAIN); go get -d $(GO_OPTIONS))

whichrelease:
	echo $(RELEASE_VERSION)

release: $(BINRELEASE)
	s3cmd -P put $(BINRELEASE) s3://get.docker.io/builds/`uname -s`/`uname -m`/docker-$(RELEASE_VERSION).tgz

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

clean:
	@rm -rf $(dir $(DOCKER_BIN))
ifeq ($(GOPATH), $(BUILD_DIR))
	@rm -rf $(BUILD_DIR)
else ifneq ($(DOCKER_DIR), $(realpath $(DOCKER_DIR)))
	@rm -f $(DOCKER_DIR)
endif

test: all
	@(cd $(DOCKER_DIR); sudo -E go test $(GO_OPTIONS))

fmt:
	@gofmt -s -l -w .

hack:
	cd $(CURDIR)/hack && vagrant up

ssh-dev:
	cd $(CURDIR)/hack && vagrant ssh
