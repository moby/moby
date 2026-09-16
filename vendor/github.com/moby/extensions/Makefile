GO ?= go
DOCKER ?= docker
LINT_CACHE_VOLUME ?= moby-extensions-lint-cache
LINT_WORKDIR := /workspace

GO_VERSION ?= $(shell awk '/^go / { print $$2 }' go.mod)
# renovate: depName=github.com/rhysd/actionlint
ACTIONLINT_VERSION ?= v1.7.12
# renovate: depName=golang.org/x/tools/gopls
MODERNIZE_VERSION ?= v0.23.0
# renovate: depName=github.com/golangci/golangci-lint/v2
GOLANGCI_LINT_VERSION ?= v2.12.2
LINT_IMAGE ?= golang:$(GO_VERSION)

.PHONY: ci
ci: lint validate

.PHONY: lint
lint:
	@if [ -z "$(GO_VERSION)" ] || \
		[ -z "$(ACTIONLINT_VERSION)" ] || \
		[ -z "$(MODERNIZE_VERSION)" ] || \
		[ -z "$(GOLANGCI_LINT_VERSION)" ]; then \
		echo "lint versions must not be empty" >&2; \
		exit 1; \
	fi
	$(DOCKER) run --rm -i \
		--mount "type=bind,source=$(CURDIR),target=$(LINT_WORKDIR),readonly" \
		--mount "type=volume,source=$(LINT_CACHE_VOLUME),target=/root/.cache" \
		--env ACTIONLINT_VERSION=$(ACTIONLINT_VERSION) \
		--env MODERNIZE_VERSION=$(MODERNIZE_VERSION) \
		--env GOLANGCI_LINT_VERSION=$(GOLANGCI_LINT_VERSION) \
		--env GOCACHE=/root/.cache/go-build \
		--env GOMODCACHE=/root/.cache/go-mod \
		--env GOLANGCI_LINT_CACHE=/root/.cache/golangci-lint \
		--env GOFLAGS=-buildvcs=false \
		--workdir $(LINT_WORKDIR) \
		$(LINT_IMAGE) \
		make host-lint

.PHONY: host-lint
host-lint:
	$(GO) run github.com/rhysd/actionlint/cmd/actionlint@$(ACTIONLINT_VERSION)
	$(GO) run golang.org/x/tools/gopls/internal/analysis/modernize/cmd/modernize@$(MODERNIZE_VERSION) ./...
	$(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run ./...

.PHONY: validate
validate:
	@set -e; \
	files="$$(gofmt -s -l .)"; \
	files="$$(printf '%s\n' "$$files" | sed '/^vendor\//d')"; \
	if [ -n "$$files" ]; then \
		echo "::error::these files are not gofmt-ed:"; \
		echo "$$files"; \
		exit 1; \
	fi
	$(GO) vet ./...
