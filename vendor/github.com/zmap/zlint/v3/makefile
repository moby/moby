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

CMDS = zlint zlint-gtld-update
CMD_PREFIX = ./cmd/
BUILD = $(GO_ENV) go build
TEST = $(GO_ENV) GORACE=halt_on_error=1 go test -race
INT_TEST = $(GO_ENV) go test -v -tags integration -timeout 20m ./integration/... -parallelism $(PARALLELISM) $(INT_FLAGS)

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
	golangci-lint run

testdata-lint:
	./test/prepend_testcerts_openssl.sh && git diff --exit-code testdata/

.PHONY: clean zlint zlint-gtld-update test integration code-lint testdata-lint
