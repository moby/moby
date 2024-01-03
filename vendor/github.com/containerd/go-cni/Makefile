#   Copyright The containerd Authors.

#   Licensed under the Apache License, Version 2.0 (the "License");
#   you may not use this file except in compliance with the License.
#   You may obtain a copy of the License at

#       http://www.apache.org/licenses/LICENSE-2.0

#   Unless required by applicable law or agreed to in writing, software
#   distributed under the License is distributed on an "AS IS" BASIS,
#   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#   See the License for the specific language governing permissions and
#   limitations under the License.

TESTFLAGS_PARALLEL ?= 8

EXTRA_TESTFLAGS ?=

# quiet or not
ifeq ($(V),1)
	Q =
else
	Q = @
endif

.PHONY: test integration clean help

help: ## this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST) | sort

test: ## run tests, except integration tests and tests that require root
	$(Q)go test -v -race $(EXTRA_TESTFLAGS) -count=1 ./...

integration: bin/integration.test ## run integration test
	$(Q)bin/integration.test -test.v -test.count=1 -test.root $(EXTRA_TESTFLAGS) -test.parallel $(TESTFLAGS_PARALLEL)

bin/integration.test: ## build integration test binary into bin
	$(Q)cd ./integration && go test -race -c . -o ../bin/integration.test

clean: ## clean up binaries
	$(Q)rm -rf bin/
