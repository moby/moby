.PHONY: all all-local build build-local clean cross cross-local check check-code check-format run-tests integration-tests check-local coveralls circle-ci-cross circle-ci-build circle-ci-check circle-ci
SHELL=/bin/bash
build_image=libnetworkbuild
dockerargs = --privileged -v $(shell pwd):/go/src/github.com/docker/libnetwork -w /go/src/github.com/docker/libnetwork
container_env = -e "INSIDECONTAINER=-incontainer=true"
docker = docker run --rm -it ${dockerargs} $$EXTRA_ARGS ${container_env} ${build_image}
ciargs = -e CIRCLECI -e "COVERALLS_TOKEN=$$COVERALLS_TOKEN" -e "INSIDECONTAINER=-incontainer=true"
cidocker = docker run ${dockerargs} ${ciargs} $$EXTRA_ARGS ${container_env} ${build_image}
CROSS_PLATFORMS = linux/amd64 linux/386 linux/arm windows/amd64
export PATH := $(CURDIR)/bin:$(PATH)
hostOS = ${shell go env GOHOSTOS}
ifeq (${hostOS}, solaris)
	gnufind=gfind
	gnutail=gtail
else
	gnufind=find
	gnutail=tail
endif

all: ${build_image}.created build check integration-tests clean

all-local: build-local check-local integration-tests-local clean

${build_image}.created:
	docker build -f Dockerfile.build -t ${build_image} .
	touch ${build_image}.created

build: ${build_image}.created
	@echo "Building code... "
	@${docker} ./wrapmake.sh build-local
	@echo "Done building code"

build-local:
	@mkdir -p "bin"
	go build -tags experimental -o "bin/dnet" ./cmd/dnet
	go build -o "bin/docker-proxy" ./cmd/proxy

clean:
	@if [ -d bin ]; then \
		echo "Removing dnet and proxy binaries"; \
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
	go build -o "bin/dnet-$$GOOS-$$GOARCH" ./cmd/dnet
	go build -o "bin/docker-proxy-$$GOOS-$$GOARCH" ./cmd/proxy

check: ${build_image}.created
	@${docker} ./wrapmake.sh check-local

check-code:
	@echo "Checking code... "
	test -z "$$(golint ./... | grep -Ev 'vendor|.pb.go:' | tee /dev/stderr)"
	test -z "$$(go vet ./... 2>&1 > /dev/null | grep -Ev 'vendor|exit' | tee /dev/stderr)"
	@echo "Done checking code"

check-format:
	@echo "Checking format... "
	test -z "$$(gofmt -s -l . | grep -v vendor/ | tee /dev/stderr)"
	@echo "Done checking format"

run-tests:
	@echo "Running tests... "
	@echo "mode: count" > coverage.coverprofile
	@for dir in $$( ${gnufind} . -maxdepth 10 -not -path './.git*' -not -path '*/_*' -not -path './vendor/*' -type d); do \
	    if [ ${hostOS} == solaris ]; then \
	        case "$$dir" in \
		    "./cmd/dnet" ) \
		    ;& \
		    "./cmd/ovrouter" ) \
		    ;& \
		    "./ns" ) \
		    ;& \
		    "./iptables" ) \
		    ;& \
		    "./ipvs" ) \
		    ;& \
		    "./drivers/bridge" ) \
		    ;& \
		    "./drivers/host" ) \
		    ;& \
		    "./drivers/ipvlan" ) \
		    ;& \
		    "./drivers/macvlan" ) \
		    ;& \
		    "./drivers/overlay" ) \
		    ;& \
		    "./drivers/remote" ) \
		    ;& \
		    "./drivers/windows" ) \
			echo "Skipping $$dir on solaris host... "; \
			continue; \
			;; \
		    * )\
			echo "Entering $$dir ... "; \
			;; \
	        esac; \
	    fi; \
	    if ls $$dir/*.go &> /dev/null; then \
		pushd . &> /dev/null ; \
		cd $$dir ; \
		go test ${INSIDECONTAINER} -test.parallel 5 -test.v -covermode=count -coverprofile=./profile.tmp ; \
		ret=$$? ;\
		if [ $$ret -ne 0 ]; then exit $$ret; fi ;\
		popd &> /dev/null; \
		if [ -f $$dir/profile.tmp ]; then \
			cat $$dir/profile.tmp | ${gnutail} -n +2 >> coverage.coverprofile ; \
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

circle-ci: circle-ci-build circle-ci-check circle-ci-cross integration-tests
