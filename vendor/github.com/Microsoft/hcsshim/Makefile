include Makefile.bootfiles

# C settings

# enable loading kernel modules in init
KMOD:=0

CFLAGS:=-O2 -Wall
LDFLAGS:=-static -s #strip C binaries
LDLIBS:=
PREPROCESSORFLAGS:=
ifeq "$(KMOD)" "1"
LDFLAGS:=-s
LDLIBS:=-lkmod
PREPROCESSORFLAGS:=-DMODULES=1
endif

# Go settings

# if Go is from the Microsoft Go fork
MSGO:=0
# explicitly use vendored modules when building
GOMODVENDOR:=
# Go tags to enable
GO_BUILD_TAGS:=
# additional Go build flags
GO_FLAGS_EXTRA:=
# use CGO
CGO_ENABLED:=0

GO:=go
GO_FLAGS:=-ldflags "-s -w" # strip Go binaries
ifeq "$(GOMODVENDOR)" "1"
GO_FLAGS+=-mod=vendor
endif
ifneq ($(strip $(GO_BUILD_TAGS)),)
GO_FLAGS+=-tags="$(GO_BUILD_TAGS)"
endif
GO_BUILD_ENV:=CGO_ENABLED=$(CGO_ENABLED)
# starting with ms-go1.25, systemcrypto (for FIPS compliance) is enabled by default, which
# requires CGo.
# disable it for non-CGo builds.
#
# https://github.com/microsoft/go/blob/microsoft/main/eng/doc/MigrationGuide.md#cgo-is-not-enabled
# https://github.com/microsoft/go/blob/microsoft/main/eng/doc/MigrationGuide.md#disabling-systemcrypto
ifeq "$(MSGO)" "1"
ifneq "$(CGO_ENABLED)" "1"
# MS_GO_NOSYSTEMCRYPTO only works for >=ms-go1.25.2
GO_BUILD_ENV+=MS_GO_NOSYSTEMCRYPTO=1 GOEXPERIMENT=nosystemcrypto
endif
endif
GO_BUILD:= $(GO_BUILD_ENV) $(GO) build $(GO_FLAGS) $(GO_FLAGS_EXTRA)

SRCROOT=$(dir $(abspath $(firstword $(MAKEFILE_LIST))))
# additional directories to search for rule prerequisites and targets
VPATH=$(SRCROOT)

# The link aliases for gcstools
GCS_TOOLS=\
	generichook \
	install-drivers

test:
	cd $(SRCROOT) && $(GO) test -v ./internal/guest/...

# This target includes utilities which may be useful for testing purposes.
out/delta-dev.tar.gz: out/delta.tar.gz bin/internal/tools/snp-report
	rm -rf rootfs-dev
	mkdir rootfs-dev
	tar -xzf out/delta.tar.gz -C rootfs-dev
	cp bin/internal/tools/snp-report rootfs-dev/bin/
	tar -zcf $@ -C rootfs-dev .
	rm -rf rootfs-dev

out/delta-snp.tar.gz: out/delta.tar.gz bin/internal/tools/snp-report boot/startup_v2056.sh boot/startup_simple.sh boot/startup.sh
	rm -rf rootfs-snp
	mkdir rootfs-snp
	tar -xzf out/delta.tar.gz -C rootfs-snp
	cp boot/startup_v2056.sh rootfs-snp/startup_v2056.sh
	cp boot/startup_simple.sh rootfs-snp/startup_simple.sh
	cp boot/startup.sh rootfs-snp/startup.sh
	cp bin/internal/tools/snp-report rootfs-snp/bin/
	chmod a+x rootfs-snp/startup_v2056.sh
	chmod a+x rootfs-snp/startup_simple.sh
	chmod a+x rootfs-snp/startup.sh
	tar -zcf $@ -C rootfs-snp .
	rm -rf rootfs-snp

out/delta.tar.gz: bin/init bin/vsockexec bin/cmd/gcs bin/cmd/gcstools bin/cmd/hooks/wait-paths Makefile
	@mkdir -p out
	rm -rf rootfs
	mkdir -p rootfs/bin/
	mkdir -p rootfs/info/
	cp bin/init rootfs/
	cp bin/vsockexec rootfs/bin/
	cp bin/cmd/gcs rootfs/bin/
	cp bin/cmd/gcstools rootfs/bin/
	cp bin/cmd/hooks/wait-paths rootfs/bin/
	for tool in $(GCS_TOOLS); do ln -s gcstools rootfs/bin/$$tool; done
	git -C $(SRCROOT) rev-parse HEAD > rootfs/info/gcs.commit && \
	git -C $(SRCROOT) rev-parse --abbrev-ref HEAD > rootfs/info/gcs.branch && \
	date --iso-8601=minute --utc > rootfs/info/tar.date
	$(if $(and $(realpath $(subst .tar,.testdata.json,$(BASE))), $(shell which jq)), \
		jq -r '.IMAGE_NAME' $(subst .tar,.testdata.json,$(BASE)) 2>/dev/null > rootfs/info/image.name && \
		jq -r '.DATETIME' $(subst .tar,.testdata.json,$(BASE)) 2>/dev/null > rootfs/info/build.date)
	tar -zcf $@ -C rootfs .
	rm -rf rootfs

# Use force target to always call `go build` per make invocation and rely on Go's build cache
# to decide if binaries should be (re)built.
# Note: don't use `.PHONY` since the targets are actual files.
#
# www.gnu.org/software/make/manual/html_node/Force-Targets.html
bin/cmd/gcs bin/cmd/gcstools bin/cmd/hooks/wait-paths bin/cmd/tar2ext4 bin/internal/tools/snp-report: FORCE
	@mkdir -p $(dir $@)
	GOOS=linux $(GO_BUILD) -o $@ $(SRCROOT)/$(@:bin/%=%)

FORCE:

bin/vsockexec: vsockexec/vsockexec.o vsockexec/vsock.o
	@mkdir -p bin
	$(CC) $(LDFLAGS) -o $@ $^

bin/init: init/init.o vsockexec/vsock.o
	@mkdir -p bin
	$(CC) $(LDFLAGS) -o $@ $^ $(LDLIBS)

%.o: %.c
	@mkdir -p $(dir $@)
	$(CC) $(PREPROCESSORFLAGS) $(CFLAGS) $(CPPFLAGS) -c -o $@ $<
