# Set an output prefix, which is the local directory if not specified
PREFIX?=$(shell pwd)

vet:
	@echo "+ $@"
	@go vet ./...

fmt:
	@echo "+ $@"
	@test -z "$$(gofmt -s -l . | grep -v Godeps/_workspace/src/ | tee /dev/stderr)" || \
		echo "+ please format Go code with 'gofmt -s'"

lint:
	@echo "+ $@"
	@test -z "$$(golint ./... | grep -v Godeps/_workspace/src/ | tee /dev/stderr)"

build:
	@echo "+ $@"
	@go build -v ${GO_LDFLAGS} ./...

test:
	@echo "+ $@"
	@go test -test.short ./...

test-full:
	@echo "+ $@"
	@go test ./...

binaries: ${PREFIX}/bin/registry ${PREFIX}/bin/registry-api-descriptor-template ${PREFIX}/bin/dist
	@echo "+ $@"

clean:
	@echo "+ $@"
	@rm -rf "${PREFIX}/bin/registry" "${PREFIX}/bin/registry-api-descriptor-template"
