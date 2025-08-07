# API Module Makefile
# This Makefile provides targets for the swagger generation and validation
# that are specific to the API module.

.PHONY: generate-swagger
generate-swagger: ## generate Go types from swagger definition
	./scripts/generate-swagger-api.sh

.PHONY: validate-swagger
validate-swagger: ## validate the swagger.yaml file
	./scripts/validate-swagger

.PHONY: validate-swagger-gen
validate-swagger-gen: ## validate that generated types are up-to-date
	./scripts/validate-swagger-gen

.PHONY: validate
validate: validate-swagger validate-swagger-gen ## run all validation checks

.PHONY: help
help: ## show this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9_-]+:.*?## / {gsub("\\\\n",sprintf("\n%22c",""), $$2);printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
