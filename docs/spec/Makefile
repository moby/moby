# Build, check and format the behavior specifications.
#
# The specs are Sphinx-Needs documents in src/. The Markdown files in this
# directory and its subdirectories are generated from them, at the paths of
# their sources (src/network/x.rst gives network/x.md). Every target other than
# help and image runs in a container built from the Dockerfile in this
# directory, so that it uses the pinned toolchain.

DOCKER ?= docker
BUILDX ?= $(DOCKER) buildx

SPEC_DIR := $(CURDIR)
REPO_DIR := $(abspath $(SPEC_DIR)/../..)
SPEC_PATH := docs/spec
IMAGE := moby-specs

BASE_BRANCH ?= master
HEAD_REV ?= HEAD
DIFF_FLAGS ?=

DOCKER_RUN := $(DOCKER) run --rm -u "$$(id -u):$$(id -g)" -e HOME=/tmp \
	-v "$(REPO_DIR):/src" -w /src/$(SPEC_PATH) $(IMAGE)

SOURCES := $(shell find src -name "*.rst" | sort)
SPHINX := sphinx-build -W --keep-going -q src

# Build the Markdown into _build/markdown.
MD_BUILD := $(SPHINX) -b markdown _build/markdown
# The generated Markdown files, in _build/markdown and in this directory.
MD_BUILT := cd _build/markdown && find . -name "*.md" | sort
# Only a file that starts with the comment that "make md" writes is generated: "make md" leaves
# other Markdown files, such as a draft, alone.
MD_HERE := find . -path ./_build -prune -o -path ./src -prune -o -path "./.*" -prune -o -name "*.md" -print | \
	xargs -r awk "FNR == 1 && /^<!-- Generated from src\// { print FILENAME } { nextfile }" | sort

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

.PHONY: test
test: image ## test the tooling: that each check reports a broken item, the Markdown rendering, and make diff's comparison
	$(DOCKER_RUN) env PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s tests

.PHONY: needs
needs: image ## build _build/needs/needs.json
	$(DOCKER_RUN) $(SPHINX) -b needs _build/needs

.PHONY: fmt
fmt: image ## format the specs in place
	$(DOCKER_RUN) docstrfmt -p pyproject.toml $(SOURCES)

.PHONY: md
md: image ## generate the Markdown specs in this directory and its subdirectories
	$(DOCKER_RUN) sh -c 'rm -rf _build/markdown && $(MD_BUILD) && \
		for f in $$($(MD_HERE)); do [ -e "_build/markdown/$$f" ] || rm "$$f"; done && \
		for f in $$($(MD_BUILT)); do mkdir -p "$$(dirname "$$f")" && cp "_build/markdown/$$f" "$$f" || exit 1; done'

.PHONY: validate
validate: image ## check that the specs build without warnings, are formatted, and match the Markdown
	$(DOCKER_RUN) bash -c '$(SPHINX) -b needs _build/needs && \
		docstrfmt -p pyproject.toml --check $(SOURCES) && \
		rm -rf _build/markdown && $(MD_BUILD) && \
		diff -u <($(MD_HERE)) <($(MD_BUILT)) && \
		for f in $$($(MD_BUILT)); do diff -u "$$f" "_build/markdown/$$f" || exit 1; done' || \
		{ echo "The specs are not built and formatted: run make fmt md and commit the result." >&2; exit 1; }

# make diff compares commits, so a change is reported once it is committed. The
# Git directory that a linked worktree refers to is outside REPO_DIR, so the
# repository's common Git directory is mounted at its own path in the container.
# The revisions reach the container in environment variables, which its shell
# does not parse.
.PHONY: diff
diff: image ## report the items that changed between the commits BASE (default: the merge base of HEAD_REV with BASE_BRANCH, master) and HEAD_REV (default: HEAD)
	@base="$(BASE)"; [ -n "$$base" ] || base="$$(git -C "$(REPO_DIR)" merge-base "$(HEAD_REV)" "$(BASE_BRANCH)")" || exit 1; \
	common="$$(git -C "$(REPO_DIR)" rev-parse --path-format=absolute --git-common-dir)" || exit 1; \
	$(DOCKER) run --rm -u "$$(id -u):$$(id -g)" -e HOME=/tmp -e BASE="$$base" -e HEAD_REV="$(HEAD_REV)" \
		-v "$(REPO_DIR):/src" -v "$$common:$$common" -w /src/$(SPEC_PATH) $(IMAGE) \
		sh -c 'git config --global --add safe.directory "*" && \
		python3 tools/needs_diff.py --repo /src --project $(SPEC_PATH)/src --sphinx-build sphinx-build \
		$(DIFF_FLAGS) -- "$$BASE" "$$HEAD_REV"'
