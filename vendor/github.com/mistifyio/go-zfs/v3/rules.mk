# Only use the recipes defined in these makefiles
MAKEFLAGS += --no-builtin-rules
.SUFFIXES:
# Delete target files if there's an error
# This avoids a failure to then skip building on next run if the output is created by shell redirection for example
# Not really necessary for now, but just good to have already if it becomes necessary later.
.DELETE_ON_ERROR:
# Treat the whole recipe as a one shell script/invocation instead of one-per-line
.ONESHELL:
# Use bash instead of plain sh
SHELL := bash
.SHELLFLAGS := -o pipefail -euc

version := $(shell git rev-parse --short HEAD)
tag := $(shell git tag --points-at HEAD)
ifneq (,$(tag))
version := $(tag)-$(version)
endif
LDFLAGS := -ldflags "-X main.version=$(version)"
export CGO_ENABLED := 0

ifeq ($(origin GOBIN), undefined)
GOBIN := ${PWD}/bin
export GOBIN
PATH := ${GOBIN}:${PATH}
export PATH
endif

toolsBins := $(addprefix bin/,$(notdir $(shell grep '^\s*_' tooling/tools.go | awk -F'"' '{print $$2}')))

# installs cli tools defined in tools.go
$(toolsBins): tooling/go.mod tooling/go.sum tooling/tools.go
$(toolsBins): CMD=$(shell awk -F'"' '/$(@F)"/ {print $$2}' tooling/tools.go)
$(toolsBins):
	cd tooling && go install $(CMD)

.PHONY: gofumpt
gofumpt: bin/gofumpt
	gofumpt -s -d .

gofumpt-fix: bin/gofumpt
	gofumpt -s -w .

.PHONY: prettier prettier-fix
prettier:
	prettier --list-different --ignore-path .gitignore .

prettier-fix:
	prettier --write --ignore-path .gitignore .
