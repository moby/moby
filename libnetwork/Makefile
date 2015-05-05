.PHONY: all all-local build build-local check check-code check-format run-tests check-local install-deps coveralls circle-ci
SHELL=/bin/bash
dockerargs = --privileged -v $(shell pwd):/go/src/github.com/docker/libnetwork -w /go/src/github.com/docker/libnetwork golang:1.4
docker = docker run --rm ${dockerargs}
ciargs = -e "COVERALLS_TOKEN=$$COVERALLS_TOKEN"
cidocker = docker run ${ciargs} ${dockerargs}

all: 
	${docker} make all-local

all-local: 	install-deps check-local build-local

build: 
	${docker} make install-deps build-local

build-local:
	$(shell which godep) go build ./...

check:
	${docker} make install-deps check-local

check-code:
	@echo "Checking code... "
	test -z "$$(golint ./... | tee /dev/stderr)"
	go vet ./...
	@echo "Done checking code"

check-format:
	@echo "Checking format... "
	test -z "$$(goimports -l . | grep -v Godeps/_workspace/src/ | tee /dev/stderr)"
	@echo "Done checking format"

run-tests:
	@echo "Running tests... "
	@echo "mode: count" > coverage.coverprofile
	@for dir in $$(find . -maxdepth 10 -not -path './.git*' -not -path '*/_*' -type d); do \
	    if ls $$dir/*.go &> /dev/null; then \
            	$(shell which godep) go test -test.v -covermode=count -coverprofile=$$dir/profile.tmp $$dir ; \
		if [ $$? -ne 0 ]; then exit $$?; fi ;\
	        if [ -f $$dir/profile.tmp ]; then \
		        cat $$dir/profile.tmp | tail -n +2 >> coverage.coverprofile ; \
				rm $$dir/profile.tmp ; \
            fi ; \
        fi ; \
	done
	@echo "Done running tests"

check-local: 	check-format check-code run-tests 

install-deps:
	apt-get update && apt-get -y install iptables
	go get github.com/tools/godep
	go get github.com/golang/lint/golint
	go get golang.org/x/tools/cmd/vet
	go get golang.org/x/tools/cmd/goimports
	go get golang.org/x/tools/cmd/cover
	go get github.com/mattn/goveralls

coveralls:
	-@goveralls -service circleci -coverprofile=coverage.coverprofile -repotoken $$COVERALLS_TOKEN

# CircleCI's Docker fails when cleaning up using the --rm flag
# The following target is a workaround for this

circle-ci:
	@${cidocker} make install-deps check-local coveralls

