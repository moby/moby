.PHONY: all all-local build build-local clean cross cross-local check check-code check-format run-tests integration-tests check-local coveralls circle-ci-cross circle-ci-build circle-ci-check circle-ci
SHELL=/bin/bash
build_image=libnetworkbuild
dockerargs = --privileged -v $(shell pwd):/go/src/github.com/docker/libnetwork -w /go/src/github.com/docker/libnetwork
container_env = -e "INSIDECONTAINER=-incontainer=true"
docker = docker run --rm -it ${dockerargs} $$EXTRA_ARGS ${container_env} ${build_image}
ciargs = -e "COVERALLS_TOKEN=$$COVERALLS_TOKEN" -e "INSIDECONTAINER=-incontainer=true"
cidocker = docker run ${dockerargs} ${ciargs} ${container_env} ${build_image}
CROSS_PLATFORMS = linux/amd64 linux/386 linux/arm windows/amd64 windows/386

${build_image}.created:
	docker build -f Dockerfile.build -t ${build_image} .
	touch ${build_image}.created

all: ${build_image}.created build check integration-tests clean

all-local: build-local check-local integration-tests-local clean

build: ${build_image}.created
	@echo "Building code... "
	@${docker} ./wrapmake.sh build-local
	@echo "Done building code"

build-local:
	@mkdir -p "bin"
	$(shell which godep) go build -o "bin/dnet" ./cmd/dnet

clean:
	@if [ -d bin ]; then \
		echo "Removing dnet binaries"; \
		rm -rf bin; \
	fi

cross: ${build_image}.created
	@mkdir -p "bin"
	@for platform in ${CROSS_PLATFORMS}; do \
	        EXTRA_ARGS="-e GOOS=$${platform%/*} -e GOARCH=$${platform##*/}" ; \
		echo "$${platform}..." ; \
	        ${docker} make cross-local ; \
	done

cross-local:
	$(shell which godep) go build -o "bin/dnet-$$GOOS-$$GOARCH" ./cmd/dnet

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

check-local:	check-format check-code run-tests

integration-tests: ./bin/dnet
	@./test/integration/dnet/run-integration-tests.sh

./bin/dnet:
	make build

coveralls:
	-@goveralls -service circleci -coverprofile=coverage.coverprofile -repotoken $$COVERALLS_TOKEN

# CircleCI's Docker fails when cleaning up using the --rm flag
# The following targets are a workaround for this
circle-ci-cross: ${build_image}.created
	@mkdir -p "bin"
	@for platform in ${CROSS_PLATFORMS}; do \
	        EXTRA_ARGS="-e GOOS=$${platform%/*} -e GOARCH=$${platform##*/}" ; \
		echo "$${platform}..." ; \
	        ${cidocker} make cross-local ; \
	done

circle-ci-check: ${build_image}.created
	@${cidocker} make check-local coveralls

circle-ci-build: ${build_image}.created
	@${cidocker} make build-local

circle-ci: circle-ci-check circle-ci-build integration-tests
