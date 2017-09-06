BUILDTAGS := selinux

check-gopath:
ifndef GOPATH
	$(error GOPATH is not set)
endif

.PHONY: test
test: check-gopath
	go test -timeout 3m -tags "${BUILDTAGS}" ${TESTFLAGS} -v ./...

.PHONY:
lint:
	golint go-selinux
