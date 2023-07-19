# Lint rules to ignore
LINTIGNORESINGLEFIGHT='internal/sync/singleflight/singleflight.go:.+error should be the last type'
LINT_IGNORE_S3MANAGER_INPUT='feature/s3/manager/upload.go:.+struct field SSEKMSKeyId should be SSEKMSKeyID'

UNIT_TEST_TAGS=
BUILD_TAGS=-tags "example,codegen,integration,ec2env,perftest"

SMITHY_GO_SRC ?= $(shell pwd)/../smithy-go

SDK_MIN_GO_VERSION ?= 1.15

EACHMODULE_FAILFAST ?= true
EACHMODULE_FAILFAST_FLAG=-fail-fast=${EACHMODULE_FAILFAST}

EACHMODULE_CONCURRENCY ?= 1
EACHMODULE_CONCURRENCY_FLAG=-c ${EACHMODULE_CONCURRENCY}

EACHMODULE_SKIP ?=
EACHMODULE_SKIP_FLAG=-skip="${EACHMODULE_SKIP}"

EACHMODULE_FLAGS=${EACHMODULE_CONCURRENCY_FLAG} ${EACHMODULE_FAILFAST_FLAG} ${EACHMODULE_SKIP_FLAG}

# SDK's Core and client packages that are compatible with Go 1.9+.
SDK_CORE_PKGS=./aws/... ./internal/...
SDK_CLIENT_PKGS=./service/...
SDK_COMPA_PKGS=${SDK_CORE_PKGS} ${SDK_CLIENT_PKGS}

# SDK additional packages that are used for development of the SDK.
SDK_EXAMPLES_PKGS=
SDK_ALL_PKGS=${SDK_COMPA_PKGS} ${SDK_EXAMPLES_PKGS}

RUN_NONE=-run NONE
RUN_INTEG=-run '^TestInteg_'

CODEGEN_RESOURCES_PATH=$(shell pwd)/codegen/smithy-aws-go-codegen/src/main/resources/software/amazon/smithy/aws/go/codegen
CODEGEN_API_MODELS_PATH=$(shell pwd)/codegen/sdk-codegen/aws-models
ENDPOINTS_JSON=${CODEGEN_RESOURCES_PATH}/endpoints.json
ENDPOINT_PREFIX_JSON=${CODEGEN_RESOURCES_PATH}/endpoint-prefix.json

LICENSE_FILE=$(shell pwd)/LICENSE.txt

SMITHY_GO_VERSION ?=
PRE_RELEASE_VERSION ?=
RELEASE_MANIFEST_FILE ?=
RELEASE_CHGLOG_DESC_FILE ?=

REPOTOOLS_VERSION ?= latest
REPOTOOLS_MODULE = github.com/awslabs/aws-go-multi-module-repository-tools
REPOTOOLS_CMD_ANNOTATE_STABLE_GEN = ${REPOTOOLS_MODULE}/cmd/annotatestablegen@${REPOTOOLS_VERSION}
REPOTOOLS_CMD_MAKE_RELATIVE = ${REPOTOOLS_MODULE}/cmd/makerelative@${REPOTOOLS_VERSION}
REPOTOOLS_CMD_CALCULATE_RELEASE = ${REPOTOOLS_MODULE}/cmd/calculaterelease@${REPOTOOLS_VERSION}
REPOTOOLS_CMD_UPDATE_REQUIRES = ${REPOTOOLS_MODULE}/cmd/updaterequires@${REPOTOOLS_VERSION}
REPOTOOLS_CMD_UPDATE_MODULE_METADATA = ${REPOTOOLS_MODULE}/cmd/updatemodulemeta@${REPOTOOLS_VERSION}
REPOTOOLS_CMD_GENERATE_CHANGELOG = ${REPOTOOLS_MODULE}/cmd/generatechangelog@${REPOTOOLS_VERSION}
REPOTOOLS_CMD_CHANGELOG = ${REPOTOOLS_MODULE}/cmd/changelog@${REPOTOOLS_VERSION}
REPOTOOLS_CMD_TAG_RELEASE = ${REPOTOOLS_MODULE}/cmd/tagrelease@${REPOTOOLS_VERSION}
REPOTOOLS_CMD_EDIT_MODULE_DEPENDENCY = ${REPOTOOLS_MODULE}/cmd/editmoduledependency@${REPOTOOLS_VERSION}

REPOTOOLS_CALCULATE_RELEASE_VERBOSE ?= false
REPOTOOLS_CALCULATE_RELEASE_VERBOSE_FLAG=-v=${REPOTOOLS_CALCULATE_RELEASE_VERBOSE}

REPOTOOLS_CALCULATE_RELEASE_ADDITIONAL_ARGS ?=

ifneq ($(PRE_RELEASE_VERSION),)
	REPOTOOLS_CALCULATE_RELEASE_ADDITIONAL_ARGS += -preview=${PRE_RELEASE_VERSION}
endif

.PHONY: all
all: generate unit

###################
# Code Generation #
###################
.PHONY: generate smithy-generate smithy-build smithy-build-% smithy-clean smithy-go-publish-local format \
gen-config-asserts gen-repo-mod-replace gen-mod-replace-smithy gen-mod-dropreplace-smithy-% gen-aws-ptrs tidy-modules-% \
add-module-license-files sync-models sync-endpoints-model sync-endpoints.json clone-v1-models gen-internal-codegen \
sync-api-models copy-attributevalue-feature min-go-version-% update-requires smithy-annotate-stable \
update-module-metadata download-modules-%

generate: smithy-generate update-requires gen-repo-mod-replace update-module-metadata smithy-annotate-stable \
gen-config-asserts gen-internal-codegen copy-attributevalue-feature gen-mod-dropreplace-smithy-. min-go-version-. \
tidy-modules-. add-module-license-files gen-aws-ptrs format

smithy-generate:
	cd codegen && ./gradlew clean build -Plog-tests && ./gradlew clean

smithy-build:
	cd codegen && ./gradlew clean build -Plog-tests

smithy-build-%:
	@# smithy-build- command that uses the pattern to define build filter that
	@# the smithy API model service id starts with. Strips off the
	@# "smithy-build-".
	@#
	@# e.g. smithy-build-com.amazonaws.rds
	@# e.g. smithy-build-com.amazonaws.rds#AmazonRDSv19
	cd codegen && \
	SMITHY_GO_BUILD_API="$(subst smithy-build-,,$@)" ./gradlew clean build -Plog-tests

smithy-annotate-stable:
	go run ${REPOTOOLS_CMD_ANNOTATE_STABLE_GEN}

smithy-clean:
	cd codegen && ./gradlew clean

smithy-go-publish-local:
	rm -rf /tmp/smithy-go-local
	git clone https://github.com/aws/smithy-go /tmp/smithy-go-local
	make -C /tmp/smithy-go-local smithy-clean smithy-publish-local

format:
	gofmt -w -s .

gen-config-asserts:
	@echo "Generating SDK config package implementor assertions"
	cd config \
	    && go mod tidy \
	    && go generate

gen-internal-codegen:
	@echo "Generating internal/codegen"
	cd internal/codegen \
		&& go mod tidy \
		&& go generate

gen-repo-mod-replace:
	@echo "Generating go.mod replace for repo modules"
	go run ${REPOTOOLS_CMD_MAKE_RELATIVE}

gen-mod-replace-smithy-%:
	@# gen-mod-replace-smithy- command that uses the pattern to define build filter that
	@# for modules to add replace to. Strips off the "gen-mod-replace-smithy-".
	@#
	@# SMITHY_GO_SRC environment variable is the path to add replace to
	@#
	@# e.g. gen-mod-replace-smithy-service_ssooidc
	cd ./internal/repotools/cmd/eachmodule \
		&& go run . -p $(subst _,/,$(subst gen-mod-replace-smithy-,,$@)) ${EACHMODULE_FLAGS} \
			"go mod edit -replace github.com/aws/smithy-go=${SMITHY_GO_SRC}"

gen-mod-dropreplace-smithy-%:
	@# gen-mod-dropreplace-smithy- command that uses the pattern to define build filter that
	@# for modules to add replace to. Strips off the "gen-mod-dropreplace-smithy-".
	@#
	@# e.g. gen-mod-dropreplace-smithy-service_ssooidc
	cd ./internal/repotools/cmd/eachmodule \
		&& go run . -p $(subst _,/,$(subst gen-mod-dropreplace-smithy-,,$@)) ${EACHMODULE_FLAGS} \
			"go mod edit -dropreplace github.com/aws/smithy-go"

gen-aws-ptrs:
	cd aws && go generate

tidy-modules-%:
	@# tidy command that uses the pattern to define the root path that the
	@# module testing will start from. Strips off the "tidy-modules-" and
	@# replaces all "_" with "/".
	@#
	@# e.g. tidy-modules-internal_protocoltest
	cd ./internal/repotools/cmd/eachmodule \
		&& go run . -p $(subst _,/,$(subst tidy-modules-,,$@)) ${EACHMODULE_FLAGS} \
		"go mod tidy"

download-modules-%:
	@# download command that uses the pattern to define the root path that the
	@# module testing will start from. Strips off the "download-modules-" and
	@# replaces all "_" with "/".
	@#
	@# e.g. download-modules-internal_protocoltest
	cd ./internal/repotools/cmd/eachmodule \
		&& go run . -p $(subst _,/,$(subst download-modules-,,$@)) ${EACHMODULE_FLAGS} \
		"go mod download all"

add-module-license-files:
	cd internal/repotools/cmd/eachmodule && \
    	go run . -skip-root \
            "cp $(LICENSE_FILE) ."

sync-models: sync-endpoints-model sync-api-models

sync-endpoints-model: sync-endpoints.json

sync-endpoints.json:
	[[ ! -z "${ENDPOINTS_MODEL}" ]] && cp ${ENDPOINTS_MODEL} ${ENDPOINTS_JSON} || echo "ENDPOINTS_MODEL not set, must not be empty"

clone-v1-models:
	rm -rf /tmp/aws-sdk-go-model-sync
	git clone https://github.com/aws/aws-sdk-go.git --depth 1 /tmp/aws-sdk-go-model-sync

sync-api-models:
	cd internal/repotools/cmd/syncAPIModels && \
		go run . \
			-m ${API_MODELS} \
			-o ${CODEGEN_API_MODELS_PATH}

copy-attributevalue-feature:
	cd ./feature/dynamodbstreams/attributevalue && \
	find . -name "*.go" | grep -v "doc.go" | xargs -I % rm % && \
	find ../../dynamodb/attributevalue -name "*.go" | grep -v "doc.go" | xargs -I % cp % . && \
	ls *.go | grep -v "convert.go" | grep -v "doc.go" | \
		xargs -I % sed -i.bk -E 's:github.com/aws/aws-sdk-go-v2/(service|feature)/dynamodb:github.com/aws/aws-sdk-go-v2/\1/dynamodbstreams:g' % &&  \
	ls *.go | grep -v "convert.go" | grep -v "doc.go" | \
		xargs -I % sed -i.bk 's:DynamoDB:DynamoDBStreams:g' % &&  \
	ls *.go | grep -v "doc.go" | \
		xargs -I % sed -i.bk 's:dynamodb\.:dynamodbstreams.:g' % &&  \
	sed -i.bk 's:streams\.:ddbtypes.:g' "convert.go" && \
	sed -i.bk 's:ddb\.:streams.:g' "convert.go" &&  \
	sed -i.bk 's:ddbtypes\.:ddb.:g' "convert.go" &&\
	sed -i.bk 's:Streams::g' "convert.go" && \
	rm -rf ./*.bk && \
	go mod tidy && \
	gofmt -w -s . && \
	go test .

min-go-version-%:
	cd ./internal/repotools/cmd/eachmodule \
		&& go run . -p $(subst _,/,$(subst min-go-version-,,$@)) ${EACHMODULE_FLAGS} \
		"go mod edit -go=${SDK_MIN_GO_VERSION}"

update-requires:
	go run ${REPOTOOLS_CMD_UPDATE_REQUIRES}

update-module-metadata:
	go run ${REPOTOOLS_CMD_UPDATE_MODULE_METADATA}

################
# Unit Testing #
################
.PHONY: unit unit-race unit-test unit-race-test unit-race-modules-% unit-modules-% build build-modules-% \
go-build-modules-% test test-race-modules-% test-modules-% cachedep cachedep-modules-% api-diff-modules-%

unit: lint unit-modules-.
unit-race: lint unit-race-modules-.

unit-test: test-modules-.
unit-race-test: test-race-modules-.

unit-race-modules-%:
	@# unit command that uses the pattern to define the root path that the
	@# module testing will start from. Strips off the "unit-race-modules-" and
	@# replaces all "_" with "/".
	@#
	@# e.g. unit-race-modules-internal_protocoltest
	cd ./internal/repotools/cmd/eachmodule \
		&& go run . -p $(subst _,/,$(subst unit-race-modules-,,$@)) ${EACHMODULE_FLAGS} \
		"go vet ${BUILD_TAGS} --all ./..." \
		"go test ${BUILD_TAGS} ${RUN_NONE} ./..." \
		"go test -timeout=1m ${UNIT_TEST_TAGS} -race -cpu=4 ./..."

unit-modules-%:
	@# unit command that uses the pattern to define the root path that the
	@# module testing will start from. Strips off the "unit-modules-" and
	@# replaces all "_" with "/".
	@#
	@# e.g. unit-modules-internal_protocoltest
	cd ./internal/repotools/cmd/eachmodule \
		&& go run . -p $(subst _,/,$(subst unit-modules-,,$@)) ${EACHMODULE_FLAGS} \
		"go vet ${BUILD_TAGS} --all ./..." \
		"go test ${BUILD_TAGS} ${RUN_NONE} ./..." \
		"go test -timeout=1m ${UNIT_TEST_TAGS} ./..."

build: build-modules-.

build-modules-%:
	@# build command that uses the pattern to define the root path that the
	@# module testing will start from. Strips off the "build-modules-" and
	@# replaces all "_" with "/".
	@#
	@# e.g. build-modules-internal_protocoltest
	cd ./internal/repotools/cmd/eachmodule \
		&& go run . -p $(subst _,/,$(subst build-modules-,,$@)) ${EACHMODULE_FLAGS} \
		"go test ${BUILD_TAGS} ${RUN_NONE} ./..."

go-build-modules-%:
	@# build command that uses the pattern to define the root path that the
	@# module testing will start from. Strips off the "build-modules-" and
	@# replaces all "_" with "/".
	@#
	@# Validates that all modules in the repo have buildable Go files.
	@#
	@# e.g. go-build-modules-internal_protocoltest
	cd ./internal/repotools/cmd/eachmodule \
		&& go run . -p $(subst _,/,$(subst go-build-modules-,,$@)) ${EACHMODULE_FLAGS} \
		"go build ${BUILD_TAGS} ./..."

test: test-modules-.

test-race-modules-%:
	@# Test command that uses the pattern to define the root path that the
	@# module testing will start from. Strips off the "test-race-modules-" and
	@# replaces all "_" with "/".
	@#
	@# e.g. test-race-modules-internal_protocoltest
	cd ./internal/repotools/cmd/eachmodule \
		&& go run . -p $(subst _,/,$(subst test-race-modules-,,$@)) ${EACHMODULE_FLAGS} \
		"go test -timeout=1m ${UNIT_TEST_TAGS} -race -cpu=4 ./..."

test-modules-%:
	@# Test command that uses the pattern to define the root path that the
	@# module testing will start from. Strips off the "test-modules-" and
	@# replaces all "_" with "/".
	@#
	@# e.g. test-modules-internal_protocoltest
	cd ./internal/repotools/cmd/eachmodule \
		&& go run . -p $(subst _,/,$(subst test-modules-,,$@)) ${EACHMODULE_FLAGS} \
		"go test -timeout=1m ${UNIT_TEST_TAGS} ./..."

cachedep: cachedep-modules-.

cachedep-modules-%:
	@# build command that uses the pattern to define the root path that the
	@# module caching will start from. Strips off the "cachedep-modules-" and
	@# replaces all "_" with "/".
	@#
	@# e.g. cachedep-modules-internal_protocoltest
	cd ./internal/repotools/cmd/eachmodule \
		&& go run . -p $(subst _,/,$(subst cachedep-modules-,,$@)) ${EACHMODULE_FLAGS} \
		"go mod download"

api-diff-modules-%:
	@# Command that uses the pattern to define the root path that the
	@# module testing will start from. Strips off the "api-diff-modules-" and
	@# replaces all "_" with "/".
	@#
	@# Requires golang.org/x/exp/cmd/gorelease to be available in the GOPATH.
	@#
	@# e.g. api-diff-modules-internal_protocoltest
	cd ./internal/repotools/cmd/eachmodule \
		&& go run . -p $(subst _,/,$(subst api-diff-modules-,,$@)) \
			-fail-fast=true \
			-c 1 \
			-skip="internal/repotools" \
			"$$(go env GOPATH)/bin/gorelease"

##############
# CI Testing #
##############
.PHONY: ci-test ci-test-no-generate ci-test-generate-validate

ci-test: generate unit-race ci-test-generate-validate
ci-test-no-generate: unit-race

ci-test-generate-validate:
	@echo "CI test validate no generated code changes"
	git update-index --assume-unchanged go.mod go.sum
	git add . -A
	gitstatus=`git diff --cached --ignore-space-change`; \
	echo "$$gitstatus"; \
	if [ "$$gitstatus" != "" ] && [ "$$gitstatus" != "skipping validation" ]; then echo "$$gitstatus"; exit 1; fi
	git update-index --no-assume-unchanged go.mod go.sum

ci-lint: ci-lint-.

ci-lint-%:
	@# Run golangci-lint command that uses the pattern to define the root path that the
	@# module check will start from. Strips off the "ci-lint-" and
	@# replaces all "_" with "/".
	@#
	@# e.g. ci-lint-internal_protocoltest
	cd ./internal/repotools/cmd/eachmodule \
		&& go run . -p $(subst _,/,$(subst ci-lint-,,$@)) \
			-fail-fast=false \
			-c 1 \
			-skip="internal/repotools" \
			"golangci-lint run"

ci-lint-install:
	@# Installs golangci-lint at GoPATH.
	@# This should be used to run golangci-lint locally.
	@#
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

#######################
# Integration Testing #
#######################
.PHONY: integration integ-modules-% cleanup-integ-buckets

integration: integ-modules-service

integ-modules-%:
	@# integration command that uses the pattern to define the root path that
	@# the module testing will start from. Strips off the "integ-modules-" and
	@# replaces all "_" with "/".
	@#
	@# e.g. test-modules-service_dynamodb
	cd ./internal/repotools/cmd/eachmodule \
		&& go run . -p $(subst _,/,$(subst integ-modules-,,$@)) ${EACHMODULE_FLAGS} \
		"go test -timeout=10m -tags "integration" -v ${RUN_INTEG} -count 1 ./..."

cleanup-integ-buckets:
	@echo "Cleaning up SDK integration resources"
	go run -tags "integration" ./internal/awstesting/cmd/bucket_cleanup/main.go "aws-sdk-go-integration"

##############
# Benchmarks #
##############
.PHONY: bench bench-modules-%

bench: bench-modules-.

bench-modules-%:
	@# benchmark command that uses the pattern to define the root path that
	@# the module testing will start from. Strips off the "bench-modules-" and
	@# replaces all "_" with "/".
	@#
	@# e.g. bench-modules-service_dynamodb
	cd ./internal/repotools/cmd/eachmodule \
		&& go run . -p $(subst _,/,$(subst bench-modules-,,$@)) ${EACHMODULE_FLAGS} \
		"go test -timeout=10m -bench . --benchmem ${BUILD_TAGS} ${RUN_NONE} ./..."

#####################
#  Release Process  #
#####################
.PHONY: preview-release pre-release-validation release

ls-changes:
	go run ${REPOTOOLS_CMD_CHANGELOG} ls

preview-release:
	go run ${REPOTOOLS_CMD_CALCULATE_RELEASE} ${REPOTOOLS_CALCULATE_RELEASE_VERBOSE_FLAG} ${REPOTOOLS_CALCULATE_RELEASE_ADDITIONAL_ARGS}

pre-release-validation:
	@if [[ -z "${RELEASE_MANIFEST_FILE}" ]]; then \
		echo "RELEASE_MANIFEST_FILE is required to specify the file to write the release manifest" && false; \
	fi
	@if [[ -z "${RELEASE_CHGLOG_DESC_FILE}" ]]; then \
		echo "RELEASE_CHGLOG_DESC_FILE is required to specify the file to write the release notes" && false; \
	fi

release: pre-release-validation
	go run ${REPOTOOLS_CMD_CALCULATE_RELEASE} -o ${RELEASE_MANIFEST_FILE} ${REPOTOOLS_CALCULATE_RELEASE_VERBOSE_FLAG} ${REPOTOOLS_CALCULATE_RELEASE_ADDITIONAL_ARGS}
	go run ${REPOTOOLS_CMD_UPDATE_REQUIRES} -release ${RELEASE_MANIFEST_FILE}
	go run ${REPOTOOLS_CMD_UPDATE_MODULE_METADATA} -release ${RELEASE_MANIFEST_FILE}
	go run ${REPOTOOLS_CMD_GENERATE_CHANGELOG} -release ${RELEASE_MANIFEST_FILE} -o ${RELEASE_CHGLOG_DESC_FILE}
	go run ${REPOTOOLS_CMD_CHANGELOG} rm -all
	go run ${REPOTOOLS_CMD_TAG_RELEASE} -release ${RELEASE_MANIFEST_FILE}

##############
# Repo Tools #
##############
.PHONY: install-repotools

install-repotools:
	go install ${REPOTOOLS_MODULE}/cmd/changelog@${REPOTOOLS_VERSION}

set-smithy-go-version:
	@if [[ -z "${SMITHY_GO_VERSION}" ]]; then \
		echo "SMITHY_GO_VERSION is required to update SDK's smithy-go module dependency version" && false; \
	fi
	go run ${REPOTOOLS_CMD_EDIT_MODULE_DEPENDENCY} -s "github.com/aws/smithy-go" -v "${SMITHY_GO_VERSION}"

##################
# Linting/Verify #
##################
.PHONY: verify lint vet vet-modules-% sdkv1check

verify: lint vet sdkv1check

lint:
	@echo "go lint SDK and vendor packages"
	@lint=`golint ./...`; \
	dolint=`echo "$$lint" | grep -E -v \
	-e ${LINT_IGNORE_S3MANAGER_INPUT} \
	-e ${LINTIGNORESINGLEFIGHT}`; \
	echo "$$dolint"; \
	if [ "$$dolint" != "" ]; then exit 1; fi

vet: vet-modules-.

vet-modules-%:
	cd ./internal/repotools/cmd/eachmodule \
		&& go run . -p $(subst _,/,$(subst vet-modules-,,$@)) ${EACHMODULE_FLAGS} \
		"go vet ${BUILD_TAGS} --all ./..."

sdkv1check:
	@echo "Checking for usage of AWS SDK for Go v1"
	@sdkv1usage=`go list -test -f '''{{ if not .Standard }}{{ range $$_, $$name := .Imports }} * {{ $$.ImportPath }} -> {{ $$name }}{{ print "\n" }}{{ end }}{{ range $$_, $$name := .TestImports }} *: {{ $$.ImportPath }} -> {{ $$name }}{{ print "\n" }}{{ end }}{{ end}}''' ./... | sort -u | grep '''/aws-sdk-go/'''`; \
	echo "$$sdkv1usage"; \
	if [ "$$sdkv1usage" != "" ]; then exit 1; fi

list-deps: list-deps-.

list-deps-%:
	@# command that uses the pattern to define the root path that the
	@# module testing will start from. Strips off the "list-deps-" and
	@# replaces all "_" with "/".
	@#
	@# Trim output to only include stdout for list of dependencies only.
	@#    make list-deps 2>&-
	@#
	@# e.g. list-deps-internal_protocoltest
	@cd ./internal/repotools/cmd/eachmodule \
		&& go run . -p $(subst _,/,$(subst list-deps-,,$@)) ${EACHMODULE_FLAGS} \
		"go list -m all | grep -v 'github.com/aws/aws-sdk-go-v2'" | sort -u

###################
# Sandbox Testing #
###################
.PHONY: sandbox-tests sandbox-build-% sandbox-run-% sandbox-test-% update-aws-golang-tip

sandbox-tests: sandbox-test-go1.15 sandbox-test-go1.16 sandbox-test-go1.17 sandbox-test-go1.18 sandbox-test-go1.19 sandbox-test-go1.20 sandbox-test-gotip

sandbox-build-%:
	@# sandbox-build-go1.17
	@# sandbox-build-gotip
	@if [ $@ == sandbox-build-gotip ]; then\
		docker build \
			-f ./internal/awstesting/sandbox/Dockerfile.test.gotip \
			-t "aws-sdk-go-$(subst sandbox-build-,,$@)" . ;\
	else\
		docker build \
			--build-arg GO_VERSION=$(subst sandbox-build-go,,$@) \
			-f ./internal/awstesting/sandbox/Dockerfile.test.goversion \
			-t "aws-sdk-go-$(subst sandbox-build-,,$@)" . ;\
	fi

sandbox-run-%: sandbox-build-%
	@# sandbox-run-go1.17
	@# sandbox-run-gotip
	docker run -i -t "aws-sdk-go-$(subst sandbox-run-,,$@)" bash
sandbox-test-%: sandbox-build-%
	@# sandbox-test-go1.17
	@# sandbox-test-gotip
	docker run -t "aws-sdk-go-$(subst sandbox-test-,,$@)"

update-aws-golang-tip:
	docker build --no-cache=true -f ./internal/awstesting/sandbox/Dockerfile.golang-tip -t "aws-golang:tip" .
