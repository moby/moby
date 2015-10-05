.PHONY: all all-local build build-local check check-code check-format run-tests check-local integration-tests install-deps coveralls circle-ci start-services clean
SHELL=/bin/bash
build_image=libnetwork-build
dockerargs = --privileged -v $(shell pwd):/go/src/github.com/docker/libnetwork -w /go/src/github.com/docker/libnetwork
container_env = -e "INSIDECONTAINER=-incontainer=true"
docker = docker run --rm -it ${dockerargs} ${container_env} ${build_image}
ciargs = -e "COVERALLS_TOKEN=$$COVERALLS_TOKEN" -e "INSIDECONTAINER=-incontainer=true"
cidocker = docker run ${ciargs} ${dockerargs} golang:1.4

all: ${build_image}.created build check integration-tests clean

integration-tests: ./cmd/dnet/dnet
	@./test/integration/dnet/run-integration-tests.sh

./cmd/dnet/dnet:
	make build

clean:
	@if [ -e ./cmd/dnet/dnet ]; then \
		echo "Removing dnet binary"; \
		rm -rf ./cmd/dnet/dnet; \
	fi

all-local: check-local build-local

${build_image}.created:
	docker run --name=libnetworkbuild -v $(shell pwd):/go/src/github.com/docker/libnetwork -w /go/src/github.com/docker/libnetwork golang:1.4 make install-deps
	docker commit libnetworkbuild ${build_image}
	docker rm libnetworkbuild
	touch ${build_image}.created

build: ${build_image}.created
	@echo "Building code... "
	@${docker} ./wrapmake.sh build-local
	@echo "Done building code"

build-local:
	@$(shell which godep) go build  ./...
	@$(shell which godep) go build -o ./cmd/dnet/dnet ./cmd/dnet

check: ${build_image}.created
	@${docker} ./wrapmake.sh check-local

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

check-local:	check-format check-code start-services run-tests

install-deps:
	apt-get update && apt-get -y install iptables zookeeperd
	git clone https://github.com/golang/tools /go/src/golang.org/x/tools
	go install golang.org/x/tools/cmd/vet
	go install golang.org/x/tools/cmd/goimports
	go install golang.org/x/tools/cmd/cover
	go get github.com/tools/godep
	go get github.com/golang/lint/golint
	go get github.com/mattn/goveralls

coveralls:
	-@goveralls -service circleci -coverprofile=coverage.coverprofile -repotoken $$COVERALLS_TOKEN

# CircleCI's Docker fails when cleaning up using the --rm flag
# The following target is a workaround for this

circle-ci:
	@${cidocker} make install-deps build-local check-local coveralls
	make integration-tests

start-services:
	service zookeeper start
