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

PACKAGES=$(shell go list ./... | grep -v /vendor/)
GO_TAGS=$(if $(GO_BUILDTAGS),-tags "$(strip $(GO_BUILDTAGS))",)
GO ?= go
GO_BUILD_FLAGS ?=

all: cgutil
	$(GO) build -v $(GO_TAGS)

cgutil:
	cd cmd/cgctl && $(GO) build $(GO_BUILD_FLAGS) -v $(GO_TAGS)

proto:
	protobuild --quiet ${PACKAGES}
	# Keep them Go-idiomatic and backward-compatible with the gogo/protobuf era.
	go-fix-acronym -w -a '(Cpu|Tcp|Rss|Psi)' $(shell find cgroup1/stats/ cgroup2/stats/ -name '*.pb.go')
