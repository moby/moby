.PHONY: all shell dbuild man \
	    localtest localunittest localintegration \
	    test unittest integration

SOURCES := $(shell find . 2>&1 | grep -E '.*\.(c|h|go)$$')
PREFIX := $(DESTDIR)/usr/local
BINDIR := $(PREFIX)/sbin
GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
GIT_BRANCH_CLEAN := $(shell echo $(GIT_BRANCH) | sed -e "s/[^[:alnum:]]/-/g")
RUNC_IMAGE := runc_dev$(if $(GIT_BRANCH_CLEAN),:$(GIT_BRANCH_CLEAN))
PROJECT := github.com/opencontainers/runc
BUILDTAGS := seccomp
COMMIT_NO := $(shell git rev-parse HEAD 2> /dev/null || true)
COMMIT := $(if $(shell git status --porcelain --untracked-files=no),"${COMMIT_NO}-dirty","${COMMIT_NO}")

MAN_DIR := $(CURDIR)/man/man8
MAN_PAGES = $(shell ls $(MAN_DIR)/*.8)
MAN_PAGES_BASE = $(notdir $(MAN_PAGES))
MAN_INSTALL_PATH := ${PREFIX}/share/man/man8/

RELEASE_DIR := $(CURDIR)/release

VERSION := ${shell cat ./VERSION}

SHELL := $(shell command -v bash 2>/dev/null)

.DEFAULT: runc

runc: $(SOURCES)
	go build -i $(EXTRA_FLAGS) -ldflags "-X main.gitCommit=${COMMIT} -X main.version=${VERSION} $(EXTRA_LDFLAGS)" -tags "$(BUILDTAGS)" -o runc .

all: runc recvtty

recvtty: contrib/cmd/recvtty/recvtty

contrib/cmd/recvtty/recvtty: $(SOURCES)
	go build -i $(EXTRA_FLAGS) -ldflags "-X main.gitCommit=${COMMIT} -X main.version=${VERSION} $(EXTRA_LDFLAGS)" -tags "$(BUILDTAGS)" -o contrib/cmd/recvtty/recvtty ./contrib/cmd/recvtty

static: $(SOURCES)
	CGO_ENABLED=1 go build -i $(EXTRA_FLAGS) -tags "$(BUILDTAGS) cgo static_build" -ldflags "-w -extldflags -static -X main.gitCommit=${COMMIT} -X main.version=${VERSION} $(EXTRA_LDFLAGS)" -o runc .
	CGO_ENABLED=1 go build -i $(EXTRA_FLAGS) -tags "$(BUILDTAGS) cgo static_build" -ldflags "-w -extldflags -static -X main.gitCommit=${COMMIT} -X main.version=${VERSION} $(EXTRA_LDFLAGS)" -o contrib/cmd/recvtty/recvtty ./contrib/cmd/recvtty

release:
	@flag_list=(seccomp selinux apparmor static); \
	unset expression; \
	for flag in "$${flag_list[@]}"; do \
		expression+="' '{'',$${flag}}"; \
	done; \
	eval profile_list=("$$expression"); \
	for profile in "$${profile_list[@]}"; do \
		output=${RELEASE_DIR}/runc; \
		for flag in $$profile; do \
			output+=."$$flag"; \
		done; \
		tags="$$profile"; \
		ldflags="-X main.gitCommit=${COMMIT} -X main.version=${VERSION}"; \
		CGO_ENABLED=; \
		[[ "$$profile" =~ static ]] && { \
			tags="$${tags/static/static_build}"; \
			tags+=" cgo"; \
			ldflags+=" -w -extldflags -static"; \
			CGO_ENABLED=1; \
		}; \
		echo "Building target: $$output"; \
		go build -i $(EXTRA_FLAGS) -ldflags "$$ldflags $(EXTRA_LDFLAGS)" -tags "$$tags" -o "$$output" .; \
	done

dbuild: runcimage
	docker run --rm -v $(CURDIR):/go/src/$(PROJECT) --privileged $(RUNC_IMAGE) make clean all

lint:
	go vet $(allpackages)
	go fmt $(allpackages)

man:
	man/md2man-all.sh

runcimage:
	docker build -t $(RUNC_IMAGE) .

test:
	make unittest integration rootlessintegration

localtest:
	make localunittest localintegration localrootlessintegration

unittest: runcimage
	docker run -e TESTFLAGS -t --privileged --rm -v $(CURDIR):/go/src/$(PROJECT) $(RUNC_IMAGE) make localunittest

localunittest: all
	go test -timeout 3m -tags "$(BUILDTAGS)" ${TESTFLAGS} -v $(allpackages)

integration: runcimage
	docker run -e TESTFLAGS -t --privileged --rm -v $(CURDIR):/go/src/$(PROJECT) $(RUNC_IMAGE) make localintegration

localintegration: all
	bats -t tests/integration${TESTFLAGS}

rootlessintegration: runcimage
	docker run -e TESTFLAGS -t --privileged --rm -v $(CURDIR):/go/src/$(PROJECT) --cap-drop=ALL -u rootless $(RUNC_IMAGE) make localintegration

# FIXME: This should not be separate from rootlessintegration's method of running.
localrootlessintegration: all
	sudo -u rootless -H PATH="${PATH}" bats -t tests/integration${TESTFLAGS}

shell: all
	docker run -e TESTFLAGS -ti --privileged --rm -v $(CURDIR):/go/src/$(PROJECT) $(RUNC_IMAGE) bash

install:
	install -D -m0755 runc $(BINDIR)/runc

install-bash:
	install -D -m0644 contrib/completions/bash/runc $(PREFIX)/share/bash-completion/completions/runc

install-man:
	install -d -m 755 $(MAN_INSTALL_PATH)
	install -m 644 $(MAN_PAGES) $(MAN_INSTALL_PATH)

uninstall:
	rm -f $(BINDIR)/runc

uninstall-bash:
	rm -f $(PREFIX)/share/bash-completion/completions/runc

uninstall-man:
	rm -f $(addprefix $(MAN_INSTALL_PATH),$(MAN_PAGES_BASE))

clean:
	rm -f runc
	rm -f contrib/cmd/recvtty/recvtty
	rm -rf $(RELEASE_DIR)
	rm -rf $(MAN_DIR)

validate:
	script/validate-gofmt
	go vet $(allpackages)

ci: validate localtest

# memoize allpackages, so that it's executed only once and only if used
_allpackages = $(shell go list ./... | grep -v vendor)
allpackages = $(if $(__allpackages),,$(eval __allpackages := $$(_allpackages)))$(__allpackages)
