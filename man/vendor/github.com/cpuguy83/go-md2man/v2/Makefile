GO111MODULE ?= on

export GO111MODULE

GOOS ?= $(if $(TARGETOS),$(TARGETOS),)
GOARCH ?= $(if $(TARGETARCH),$(TARGETARCH),)

ifeq ($(TARGETARCH),amd64)
GOAMD64 ?= $(TARGETVARIANT)
endif

ifeq ($(TARGETARCH),arm)
GOARM ?= $(TARGETVARIANT:v%=%)
endif

ifneq ($(GOOS),)
export GOOS
endif

ifneq ($(GOARCH),)
export GOARCH
endif

ifneq ($(GOAMD64),)
export GOAMD64
endif

ifneq ($(GOARM),)
export GOARM
endif

.PHONY:
build: bin/go-md2man

.PHONY: clean
clean:
	@rm -rf bin/*

.PHONY: test
test:
	@go test $(TEST_FLAGS) ./...

bin/go-md2man: go.mod go.sum md2man/* *.go
	@mkdir -p bin
	CGO_ENABLED=0 go build $(BUILD_FLAGS) -o $@

.PHONY: mod
mod:
	@go mod tidy
