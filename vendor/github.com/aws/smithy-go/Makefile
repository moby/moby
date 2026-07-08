PRE_RELEASE_VERSION ?=

RELEASE_MANIFEST_FILE ?=
RELEASE_CHGLOG_DESC_FILE ?=

REPOTOOLS_VERSION ?= latest
REPOTOOLS_MODULE = github.com/awslabs/aws-go-multi-module-repository-tools
REPOTOOLS_CMD_CALCULATE_RELEASE = ${REPOTOOLS_MODULE}/cmd/calculaterelease@${REPOTOOLS_VERSION}
REPOTOOLS_CMD_CALCULATE_RELEASE_ADDITIONAL_ARGS ?=
REPOTOOLS_CMD_UPDATE_REQUIRES = ${REPOTOOLS_MODULE}/cmd/updaterequires@${REPOTOOLS_VERSION}
REPOTOOLS_CMD_UPDATE_MODULE_METADATA = ${REPOTOOLS_MODULE}/cmd/updatemodulemeta@${REPOTOOLS_VERSION}
REPOTOOLS_CMD_GENERATE_CHANGELOG = ${REPOTOOLS_MODULE}/cmd/generatechangelog@${REPOTOOLS_VERSION}
REPOTOOLS_CMD_CHANGELOG = ${REPOTOOLS_MODULE}/cmd/changelog@${REPOTOOLS_VERSION}
REPOTOOLS_CMD_TAG_RELEASE = ${REPOTOOLS_MODULE}/cmd/tagrelease@${REPOTOOLS_VERSION}
REPOTOOLS_CMD_MODULE_VERSION = ${REPOTOOLS_MODULE}/cmd/moduleversion@${REPOTOOLS_VERSION}

UNIT_TEST_TAGS=
BUILD_TAGS=

ifneq ($(PRE_RELEASE_VERSION),)
	REPOTOOLS_CMD_CALCULATE_RELEASE_ADDITIONAL_ARGS += -preview=${PRE_RELEASE_VERSION}
endif

smithy-publish-local:
	cd codegen && ./gradlew publishToMavenLocal

smithy-build:
	cd codegen && ./gradlew build

smithy-clean:
	cd codegen && ./gradlew clean

##################
# Linting/Verify #
##################
.PHONY: verify vet cover

verify: vet

vet:
	go vet ${BUILD_TAGS} --all ./...

cover:
	go test ${BUILD_TAGS} -coverprofile c.out ./...
	@cover=`go tool cover -func c.out | grep '^total:' | awk '{ print $$3+0 }'`; \
		echo "total (statements): $$cover%";

################
# Unit Testing #
################
.PHONY: unit unit-race unit-test unit-race-test

unit: verify
	go vet ${BUILD_TAGS} --all ./... && \
	go test ${BUILD_TAGS} ${RUN_NONE} ./... && \
	go test -timeout=1m ${UNIT_TEST_TAGS} ./...

unit-race: verify
	go vet ${BUILD_TAGS} --all ./... && \
	go test ${BUILD_TAGS} ${RUN_NONE} ./... && \
	go test -timeout=1m ${UNIT_TEST_TAGS} -race -cpu=4 ./...

unit-test: verify
	go test -timeout=1m ${UNIT_TEST_TAGS} ./...

unit-race-test: verify
	go test -timeout=1m ${UNIT_TEST_TAGS} -race -cpu=4 ./...

#####################
#  Release Process  #
#####################
.PHONY: preview-release pre-release-validation release

preview-release:
	go run ${REPOTOOLS_CMD_CALCULATE_RELEASE} ${REPOTOOLS_CMD_CALCULATE_RELEASE_ADDITIONAL_ARGS}

pre-release-validation:
	@if [[ -z "${RELEASE_MANIFEST_FILE}" ]]; then \
		echo "RELEASE_MANIFEST_FILE is required to specify the file to write the release manifest" && false; \
	fi
	@if [[ -z "${RELEASE_CHGLOG_DESC_FILE}" ]]; then \
		echo "RELEASE_CHGLOG_DESC_FILE is required to specify the file to write the release notes" && false; \
	fi

release: pre-release-validation
	go run ${REPOTOOLS_CMD_CALCULATE_RELEASE} -o ${RELEASE_MANIFEST_FILE} ${REPOTOOLS_CMD_CALCULATE_RELEASE_ADDITIONAL_ARGS}
	go run ${REPOTOOLS_CMD_UPDATE_REQUIRES} -release ${RELEASE_MANIFEST_FILE}
	go run ${REPOTOOLS_CMD_UPDATE_MODULE_METADATA} -release ${RELEASE_MANIFEST_FILE}
	go run ${REPOTOOLS_CMD_GENERATE_CHANGELOG} -release ${RELEASE_MANIFEST_FILE} -o ${RELEASE_CHGLOG_DESC_FILE}
	go run ${REPOTOOLS_CMD_CHANGELOG} rm -all
	go run ${REPOTOOLS_CMD_TAG_RELEASE} -release ${RELEASE_MANIFEST_FILE}

module-version:
	@go run ${REPOTOOLS_CMD_MODULE_VERSION} .

##############
# Repo Tools #
##############
.PHONY: install-changelog

external-changelog:
	mkdir -p .changelog
	cp changelog-template.json .changelog/00000000-0000-0000-0000-000000000000.json
	@echo "Generate a new UUID and update the file at .changelog/00000000-0000-0000-0000-000000000000.json"
	@echo "Make sure to rename the file with your new id, like .changelog/12345678-1234-1234-1234-123456789012.json"
	@echo "See CONTRIBUTING.md 'Changelog Documents' and an example at https://github.com/aws/smithy-go/pull/543/files"

install-changelog:
	go install ${REPOTOOLS_MODULE}/cmd/changelog@${REPOTOOLS_VERSION}
