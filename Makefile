DOCKER_PACKAGE := github.com/dotcloud/docker

BUILD_DIR := $(CURDIR)/build

GOPATH ?= $(BUILD_DIR)
export GOPATH

SRC_DIR := $(GOPATH)/src

DOCKER_DIR := $(SRC_DIR)/$(DOCKER_PACKAGE)
DOCKER_MAIN := $(DOCKER_DIR)/docker

DOCKER_BIN := $(CURDIR)/bin/docker

.PHONY: all clean

all: $(DOCKER_BIN)

$(DOCKER_BIN): $(DOCKER_DIR)
	@mkdir -p  $(dir $@)
	(cd $(DOCKER_MAIN); go get; go build -o $@)

$(DOCKER_DIR):
	@mkdir -p $(dir $@)
	@ln -sf $(CURDIR)/ $@

clean:
	@rm -rf $(dir $(DOCKER_BIN))
ifeq ($(GOPATH), $(BUILD_DIR))
	@rm -rf $(BUILD_DIR)
else ifneq ($(DOCKER_DIR), $(realpath $(DOCKER_DIR)))
	@rm -f $(DOCKER_DIR)
endif
