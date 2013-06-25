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

GO_OPTIONS ?=
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
