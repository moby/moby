.PHONY: all all-local build build-local check check-code check-format run-tests check-local install-deps coveralls circle-ci
SHELL=/bin/bash
build_image=libnetwork-build
dockerargs = --privileged -v $(shell pwd):/go/src/github.com/docker/libnetwork -w /go/src/github.com/docker/libnetwork
container_env = -e "INSIDECONTAINER=-incontainer=true"
docker = docker run --rm ${dockerargs} ${container_env} ${build_image}
ciargs = -e "COVERALLS_TOKEN=$$COVERALLS_TOKEN" -e "INSIDECONTAINER=-incontainer=true"
cidocker = docker run ${ciargs} ${dockerargs} golang:1.4

all: ${build_image}.created
	${docker} make all-local

all-local: check-local build-local

${build_image}.created:
	docker run --name=libnetworkbuild -v $(shell pwd):/go/src/github.com/docker/libnetwork -w /go/src/github.com/docker/libnetwork golang:1.4 make install-deps
	docker commit libnetworkbuild ${build_image}
	docker rm libnetworkbuild
	touch ${build_image}.created

build: ${build_image}.created
	${docker} make build-local

build-local:
	$(shell which godep) go build -tags libnetwork_discovery ./...

check: ${build_image}.created
	${docker} make check-local

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
		pushd . &> /dev/null ; \
		cd $$dir ; \
		$(shell which godep) go test ${INSIDECONTAINER} -test.parallel 3 -test.v -covermode=count -coverprofile=./profile.tmp ; \
		ret=$$? ;\
		if [ $$ret -ne 0 ]; then exit $$ret; fi ;\
		popd &> /dev/null; \
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
