include Makefile.bootfiles

GO:=go
GO_FLAGS:=-ldflags "-s -w" # strip Go binaries
CGO_ENABLED:=0
GOMODVENDOR:=
KMOD:=0

CFLAGS:=-O2 -Wall
LDFLAGS:=-static -s #strip C binaries
LDLIBS:=
PREPROCESSORFLAGS:=
ifeq "$(KMOD)" "1"
LDFLAGS:= -s
LDLIBS:= -lkmod
PREPROCESSORFLAGS:=-DMODULES=1
endif

GO_FLAGS_EXTRA:=
ifeq "$(GOMODVENDOR)" "1"
GO_FLAGS_EXTRA += -mod=vendor
endif
GO_BUILD_TAGS:=
ifneq ($(strip $(GO_BUILD_TAGS)),)
GO_FLAGS_EXTRA += -tags="$(GO_BUILD_TAGS)"
endif
GO_BUILD:=CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(GO_FLAGS) $(GO_FLAGS_EXTRA)

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

bin/cmd/gcs bin/cmd/gcstools bin/cmd/hooks/wait-paths bin/cmd/tar2ext4 bin/internal/tools/snp-report:
	@mkdir -p $(dir $@)
	GOOS=linux $(GO_BUILD) -o $@ $(SRCROOT)/$(@:bin/%=%)

bin/vsockexec: vsockexec/vsockexec.o vsockexec/vsock.o
	@mkdir -p bin
	$(CC) $(LDFLAGS) -o $@ $^

bin/init: init/init.o vsockexec/vsock.o
	@mkdir -p bin
	$(CC) $(LDFLAGS) -o $@ $^ $(LDLIBS)

%.o: %.c
	@mkdir -p $(dir $@)
	$(CC) $(PREPROCESSORFLAGS) $(CFLAGS) $(CPPFLAGS) -c -o $@ $<
