SHELL := /bin/bash
# Number of linting Go routines to use in integration tests
PARALLELISM := 5
# Additional integration test flags. Example usage:
#   make integration PARALLELISM=99 INT_FLAGS="-fingerprintSummary -forceDownload"
#   make integration INT_FLAGS="-overwriteExpected -config custom.config.json"
#   make integration INT_FLAGS="-fingerprintSummary -lintSummary -fingerprintFilter='^[ea]' -lintFilter='^w_ext_cert_policy_explicit_text_not_utf8' -config small.config.json"
#   make integration INT_FLAGS="-lintSummary -fingerprintSummary -lintFilter='^e_' -config small.config.json"
#   make integration INT_FLAGS="-lintSummary -fingerprintSummary -excludeSources='Mozilla,ETSI_ESI' -config small.config.json"
#   make integration INT_FLAGS="-includeSources='Mozilla,ETSI_ESI' -config small.config.json"
INT_FLAGS :=

GIT_VERSION := "$(shell git describe --abbrev=8)"

CMDS = zlint zlint-gtld-update
CMD_PREFIX = ./cmd/
BUILD = $(GO_ENV) go build --ldflags="-X 'main.version=$(GIT_VERSION)'"
TEST = $(GO_ENV) GORACE=halt_on_error=1 go test -race
INT_TEST = $(GO_ENV) go test -v -tags integration -timeout 20m ./integration/. -parallelism $(PARALLELISM) $(INT_FLAGS)

all: $(CMDS)

zlint:
	$(BUILD) $(CMD_PREFIX)$(@)

zlint-gtld-update:
	$(BUILD) $(CMD_PREFIX)$(@)

clean:
	rm -f $(CMDS)

test:
	$(TEST) ./...

integration:
	$(INT_TEST)

code-lint:
	# Skip these two directories as they contain Go files that are tests for custom
	# code linting framework and there is no expectation of those files conforming to anything.
	golangci-lint run --skip-dirs lints/lints/testdata/,lints/testdata

custom-code-lint:
	(cd integration/lints/ && go run main.go ../../lints)
	(cd integration/lints/ && go run main.go ../../cmd/genTestCerts)

testdata-lint:
	./test/prepend_testcerts_openssl.sh && git diff --exit-code testdata/

.PHONY: clean zlint zlint-gtld-update test integration code-lint testdata-lint custom-code-lint
