# Build, check and format the behavior specifications.
#
# The specs are Sphinx-Needs documents in src/. Every target other than
# help and image runs in a container built from the Dockerfile in this
# directory, so that it uses the pinned toolchain.

DOCKER ?= docker
BUILDX ?= $(DOCKER) buildx

SPEC_DIR := $(CURDIR)
REPO_DIR := $(abspath $(SPEC_DIR)/../..)
SPEC_PATH := docs/spec
IMAGE := moby-specs

DOCKER_RUN := $(DOCKER) run --rm -u "$$(id -u):$$(id -g)" -e HOME=/tmp \
	-v "$(REPO_DIR):/src" -w /src/$(SPEC_PATH) $(IMAGE)

SOURCES := $(wildcard src/*.rst src/*/*.rst)
SPHINX := sphinx-build -W --keep-going -q src

.DEFAULT_GOAL := help

.PHONY: help
help: ## show this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-10s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: image
image: ## build the toolchain image
	$(BUILDX) build --load -t "$(IMAGE)" .

.PHONY: html
html: image ## build the HTML specs into _build/html
	$(DOCKER_RUN) $(SPHINX) -b html _build/html

.PHONY: needs
needs: image ## build _build/needs/needs.json
	$(DOCKER_RUN) $(SPHINX) -b needs _build/needs

.PHONY: fmt
fmt: image ## format the specs in place
	$(DOCKER_RUN) docstrfmt -p pyproject.toml $(SOURCES)

.PHONY: validate
validate: image ## check that the specs build without warnings and are formatted
	$(DOCKER_RUN) sh -c '$(SPHINX) -b needs _build/needs && \
		docstrfmt -p pyproject.toml --check $(SOURCES)'

.PHONY: diff
diff: image ## report the items that changed since BASE (default: the merge base with BASE_BRANCH, master)
	$(DOCKER_RUN) sh -c 'git config --global --add safe.directory /src && \
		python3 tools/needs_diff.py --repo /src --project $(SPEC_PATH)/src --sphinx-build sphinx-build \
		$${BASE:-$$(git merge-base HEAD $${BASE_BRANCH:-master})} $${HEAD_REV:-HEAD}'
