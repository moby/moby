# The development version of clang is distributed as the 'clang' binary,
# while stable/released versions have a version number attached.
# Pin the default clang to a stable version.
CLANG ?= clang-22
STRIP ?= llvm-strip-22
OBJCOPY ?= llvm-objcopy-22
CFLAGS := -O2 -g -Wall -Werror -mcpu=v2 $(CFLAGS)

CI_KERNEL_URL ?= https://github.com/cilium/ci-kernels/raw/master/

# Obtain an absolute path to the directory of the Makefile.
# Assume the Makefile is in the root of the repository.
REPODIR := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

# Prefer podman if installed, otherwise use docker.
# Note: Setting the var at runtime will always override.
CONTAINER_ENGINE ?= $(if $(shell command -v podman),podman,docker)

# Configure container runtime arguments based on the container engine.
CONTAINER_RUN_ARGS := \
	--env MAKEFLAGS \
	--env BPF2GO_CC="$(CLANG)" \
	--env BPF2GO_CFLAGS="$(CFLAGS)" \
	--env HOME=/tmp \
	-v "${REPODIR}":/ebpf -w /ebpf \
	-v "$(shell go env GOCACHE)":/tmp/.cache/go-build \
	-v "$(shell go env GOPATH)":/go \
	-v "$(shell go env GOMODCACHE)":/go/pkg/mod

ifeq ($(CONTAINER_ENGINE), podman)
CONTAINER_RUN_ARGS += --log-driver=none --security-opt label=disable
else
CONTAINER_RUN_ARGS += --user "$(shell stat -c '%u:%g' ${REPODIR})"
endif

IMAGE := $(shell cat ${REPODIR}/testdata/docker/IMAGE)
VERSION := $(shell cat ${REPODIR}/testdata/docker/VERSION)

TARGETS_EL := \
	testdata/linked1 \
	testdata/linked2 \
	testdata/linked

TARGETS := \
	testdata/loader-clang-14 \
	testdata/loader-clang-17 \
	testdata/loader-$(CLANG) \
	testdata/loader_nobtf \
	testdata/manyprogs \
	testdata/btf_map_init \
	testdata/invalid_map \
	testdata/raw_tracepoint \
	testdata/invalid_map_static \
	testdata/invalid_btf_map_init \
	testdata/strings \
	testdata/freplace \
	testdata/fentry_fexit \
	testdata/iproute2_map_compat \
	testdata/map_spin_lock \
	testdata/subprog_reloc \
	testdata/fwd_decl \
	testdata/kconfig \
	testdata/ksym \
	testdata/kfunc \
	testdata/invalid-kfunc \
	testdata/kfunc-kmod \
	testdata/constants \
	testdata/errors \
	testdata/variables \
	testdata/arena \
	testdata/struct_ops \
	btf/testdata/relocs \
	btf/testdata/relocs_read \
	btf/testdata/relocs_read_tgt \
	btf/testdata/relocs_enum \
	btf/testdata/tags \
	cmd/bpf2go/testdata/minimal

HEADERS := $(wildcard testdata/*.h)

.PHONY: all clean godirs container-all container-shell generate format

.DEFAULT_GOAL = container-all

# Make sure Go-related dirs exist before starting containerized builds,
# otherwise podman will fail to start.
godirs:
	mkdir -p $(shell go env GOCACHE) $(shell go env GOPATH) $(shell go env GOMODCACHE)

# Build all ELF binaries using a containerized LLVM toolchain.
container-all: godirs
	+${CONTAINER_ENGINE} run --rm -ti ${CONTAINER_RUN_ARGS} \
		"${IMAGE}:${VERSION}" \
		$(MAKE) all

# (debug) Drop the user into a shell inside the container as root.
# Set BPF2GO_ envs to make 'make generate' just work.
container-shell: godirs
	${CONTAINER_ENGINE} run --rm -ti ${CONTAINER_RUN_ARGS} \
		"${IMAGE}:${VERSION}"

clean:
	find "$(CURDIR)" -name "*.elf" -delete
	find "$(CURDIR)" -name "*.o" -delete

format:
	find . -type f -name "*.c" | xargs clang-format -i

all: format testdata update-external-deps
	ln -srf testdata/loader-$(CLANG)-el.elf testdata/loader-el.elf
	ln -srf testdata/loader-$(CLANG)-eb.elf testdata/loader-eb.elf
	$(MAKE) generate

generate:
	go generate -run "gentypes" ./...
	go generate -run "stringer" ./...
	go generate -skip "(gentypes|stringer)" ./...

testdata: $(addsuffix -el.elf,$(TARGETS)) $(addsuffix -eb.elf,$(TARGETS)) $(addsuffix -el.elf,$(TARGETS_EL))

testdata/loader-%-el.elf: testdata/loader.c $(HEADERS)
	$* $(CFLAGS) -target bpfel -c $< -o $@
	$(STRIP) -g $@

testdata/loader-%-eb.elf: testdata/loader.c $(HEADERS)
	$* $(CFLAGS) -target bpfeb -c $< -o $@
	$(STRIP) -g $@

testdata/loader_nobtf-el.elf: testdata/loader.c $(HEADERS)
	$(CLANG) $(CFLAGS) -g0 -D__NOBTF__ -target bpfel -c $< -o $@

testdata/loader_nobtf-eb.elf: testdata/loader.c $(HEADERS)
	$(CLANG) $(CFLAGS) -g0 -D__NOBTF__ -target bpfeb -c $< -o $@

%-el.elf: %.c $(HEADERS)
	$(CLANG) $(CFLAGS) -target bpfel -c $< -o $@
	$(STRIP) -g $@

%-eb.elf: %.c $(HEADERS)
	$(CLANG) $(CFLAGS) -target bpfeb -c $< -o $@
	$(STRIP) -g $@

testdata/linked-el.elf: testdata/linked1-el.elf testdata/linked2-el.elf
	bpftool gen object $@ $^

.PHONY: update-external-deps
update-external-deps:
	./scripts/update-kernel-deps.sh
	./scripts/update-efw-deps.sh
