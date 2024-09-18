help: ## Print this help
	@grep --no-filename -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sed 's/:.*## /·/' | sort | column -t -W 2 -s '·' -c $(shell tput cols)

all: test ## Run tests

-include rules.mk
-include lint.mk

test: ## Run tests
	go test ./...

verify: gofumpt prettier lint ## Verify code style, is lint free, freshness ...
	git diff | (! grep .)

fix: gofumpt-fix prettier-fix ## Fix code formatting errors

tools: ${toolsBins} ## Build Go based build tools

.PHONY: all help test tools verify
