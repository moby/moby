
gofumpt       := mvdan.cc/gofumpt@v0.6.0
gosimports    := github.com/rinchsan/gosimports/cmd/gosimports@v0.3.8
golangci_lint := github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.5
asmfmt        := github.com/klauspost/asmfmt/cmd/asmfmt@v1.3.2
# sync this with netlify.toml!
hugo          := github.com/gohugoio/hugo@v0.115.2

# Make 3.81 doesn't support '**' globbing: Set explicitly instead of recursion.
all_sources   := $(wildcard *.go */*.go */*/*.go */*/*/*.go */*/*/*.go */*/*/*/*.go)
all_testdata  := $(wildcard testdata/* */testdata/* */*/testdata/* */*/testdata/*/* */*/*/testdata/*)
all_testing   := $(wildcard internal/testing/* internal/testing/*/* internal/testing/*/*/*)
all_examples  := $(wildcard examples/* examples/*/* examples/*/*/* */*/example/* */*/example/*/* */*/example/*/*/*)
all_it        := $(wildcard internal/integration_test/* internal/integration_test/*/* internal/integration_test/*/*/*)
# main_sources exclude any test or example related code
main_sources  := $(wildcard $(filter-out %_test.go $(all_testdata) $(all_testing) $(all_examples) $(all_it), $(all_sources)))
# main_packages collect the unique main source directories (sort will dedupe).
# Paths need to all start with ./, so we do that manually vs foreach which strips it.
main_packages := $(sort $(foreach f,$(dir $(main_sources)),$(if $(findstring ./,$(f)),./,./$(f))))

go_test_options ?= -timeout 300s

.PHONY: test.examples
test.examples:
	@go test $(go_test_options) ./examples/... ./imports/assemblyscript/example/... ./imports/emscripten/... ./imports/wasi_snapshot_preview1/example/...

.PHONY: build.examples.as
build.examples.as:
	@cd ./imports/assemblyscript/example/testdata && npm install && npm run build

%.wasm: %.zig
	@(cd $(@D); zig build -Doptimize=ReleaseSmall)
	@mv $(@D)/zig-out/*/$(@F) $(@D)

.PHONY: build.examples.zig
build.examples.zig: examples/allocation/zig/testdata/greet.wasm imports/wasi_snapshot_preview1/example/testdata/zig/cat.wasm imports/wasi_snapshot_preview1/testdata/zig/wasi.wasm
	@cd internal/testing/dwarftestdata/testdata/zig; zig build; mv zig-out/*/main.wasm ./ # Need DWARF custom sections.

tinygo_reactor_sources_reactor := examples/basic/testdata/add.go examples/allocation/tinygo/testdata/greet.go
.PHONY: build.examples.tinygo_reactor
build.examples.tinygo_reactor: $(tinygo_sources_reactor)
	@for f in $^; do \
	    tinygo build -o $$(echo $$f | sed -e 's/\.go/\.wasm/') -scheduler=none --no-debug --target=wasip1 -buildmode=c-shared $$f; \
	done

tinygo_sources_clis := examples/cli/testdata/cli.go imports/wasi_snapshot_preview1/example/testdata/tinygo/cat.go imports/wasi_snapshot_preview1/testdata/tinygo/wasi.go cmd/wazero/testdata/cat/cat.go
.PHONY: build.examples.tinygo_clis
build.examples.tinygo_clis: $(tinygo_sources_clis)
	@for f in $^; do \
	    tinygo build -o $$(echo $$f | sed -e 's/\.go/\.wasm/') -scheduler=none --no-debug --target=wasip1 $$f; \
	done
	@mv cmd/wazero/testdata/cat/cat.wasm cmd/wazero/testdata/cat/cat-tinygo.wasm

.PHONY: build.examples.tinygo
build.examples.tinygo: build.examples.tinygo_reactor build.examples.tinygo_clis

# We use zig to build C as it is easy to install and embeds a copy of zig-cc.
# Note: Don't use "-Oz" as that breaks our wasi sock example.
c_sources := imports/wasi_snapshot_preview1/example/testdata/zig-cc/cat.c imports/wasi_snapshot_preview1/testdata/zig-cc/wasi.c internal/testing/dwarftestdata/testdata/zig-cc/main.c
.PHONY: build.examples.zig-cc
build.examples.zig-cc: $(c_sources)
	@for f in $^; do \
	    zig cc --target=wasm32-wasi -o $$(echo $$f | sed -e 's/\.c/\.wasm/') $$f; \
	done

# Here are the emcc args we use:
#
# * `-Oz` - most optimization for code size.
# * `--profiling` - adds the name section.
# * `-s STANDALONE_WASM` - ensures wasm is built for a non-js runtime.
# * `-s EXPORTED_FUNCTIONS=_malloc,_free` - export allocation functions so that
#   they can be used externally as "malloc" and "free".
# * `-s WARN_ON_UNDEFINED_SYMBOLS=0` - imports not defined in JavaScript error
#   otherwise. See https://github.com/emscripten-core/emscripten/issues/13641
# * `-s TOTAL_STACK=8KB -s TOTAL_MEMORY=64KB` - reduce memory default from 16MB
#   to one page (64KB). To do this, we have to reduce the stack size.
# * `-s ALLOW_MEMORY_GROWTH` - allows "memory.grow" instructions to succeed, but
#   requires a function import "emscripten_notify_memory_growth".
emscripten_sources := $(wildcard imports/emscripten/testdata/*.cc)
.PHONY: build.examples.emscripten
build.examples.emscripten: $(emscripten_sources)
	@for f in $^; do \
		em++ -Oz --profiling \
		-s STANDALONE_WASM \
		-s EXPORTED_FUNCTIONS=_malloc,_free \
		-s WARN_ON_UNDEFINED_SYMBOLS=0 \
		-s TOTAL_STACK=8KB -s TOTAL_MEMORY=64KB \
		-s ALLOW_MEMORY_GROWTH \
		--std=c++17 -o $$(echo $$f | sed -e 's/\.cc/\.wasm/') $$f; \
	done

%/greet.wasm : cargo_target := wasm32-unknown-unknown
%/cat.wasm : cargo_target := wasm32-wasip1
%/wasi.wasm : cargo_target := wasm32-wasip1

.PHONY: build.examples.rust
build.examples.rust: examples/allocation/rust/testdata/greet.wasm imports/wasi_snapshot_preview1/example/testdata/cargo-wasi/cat.wasm imports/wasi_snapshot_preview1/testdata/cargo-wasi/wasi.wasm internal/testing/dwarftestdata/testdata/rust/main.wasm.xz

# Normally, we build release because it is smaller. Testing dwarf requires the debug build.
internal/testing/dwarftestdata/testdata/rust/main.wasm.xz:
	cd $(@D) && cargo build --target wasm32-wasip1
	mv $(@D)/target/wasm32-wasip1/debug/main.wasm $(@D)
	cd $(@D) && xz -k -f ./main.wasm # Rust's DWARF section is huge, so compress it.

# Builds rust using cargo normally
%.wasm: %.rs
	@(cd $(@D); cargo build --target $(cargo_target) --release)
	@mv $(@D)/target/$(cargo_target)/release/$(@F) $(@D)

spectest_base_dir := internal/integration_test/spectest
spectest_v1_dir := $(spectest_base_dir)/v1
spectest_v1_testdata_dir := $(spectest_v1_dir)/testdata
spec_version_v1 := wg-1.0
spectest_v2_dir := $(spectest_base_dir)/v2
spectest_v2_testdata_dir := $(spectest_v2_dir)/testdata
# Latest draft state as of March 12, 2024.
spec_version_v2 := 1c5e5d178bd75c79b7a12881c529098beaee2a05
spectest_threads_dir := $(spectest_base_dir)/threads
spectest_threads_testdata_dir := $(spectest_threads_dir)/testdata
# From https://github.com/WebAssembly/threads/tree/upstream-rebuild which has not been merged to main yet.
# It will likely be renamed to main in the future - https://github.com/WebAssembly/threads/issues/216.
spec_version_threads := 3635ca51a17e57e106988846c5b0e0cc48ac04fc

.PHONY: build.spectest
build.spectest:
	@$(MAKE) build.spectest.v1
	@$(MAKE) build.spectest.v2

.PHONY: build.spectest.v1
build.spectest.v1: # Note: wabt by default uses >1.0 features, so wast2json flags might drift as they include more. See WebAssembly/wabt#1878
	@rm -rf $(spectest_v1_testdata_dir)
	@mkdir -p $(spectest_v1_testdata_dir)
	@cd $(spectest_v1_testdata_dir) \
		&& curl -sSL 'https://api.github.com/repos/WebAssembly/spec/contents/test/core?ref=$(spec_version_v1)' | jq -r '.[]| .download_url' | grep -E ".wast" | xargs -Iurl curl -sJL url -O
	@cd $(spectest_v1_testdata_dir) && for f in `find . -name '*.wast'`; do \
		perl -pi -e 's/\(assert_return_canonical_nan\s(\(invoke\s"f32.demote_f64"\s\((f[0-9]{2})\.const\s[a-z0-9.+:-]+\)\))\)/\(assert_return $$1 \(f32.const nan:canonical\)\)/g' $$f; \
		perl -pi -e 's/\(assert_return_arithmetic_nan\s(\(invoke\s"f32.demote_f64"\s\((f[0-9]{2})\.const\s[a-z0-9.+:-]+\)\))\)/\(assert_return $$1 \(f32.const nan:arithmetic\)\)/g' $$f; \
		perl -pi -e 's/\(assert_return_canonical_nan\s(\(invoke\s"f64\.promote_f32"\s\((f[0-9]{2})\.const\s[a-z0-9.+:-]+\)\))\)/\(assert_return $$1 \(f64.const nan:canonical\)\)/g' $$f; \
		perl -pi -e 's/\(assert_return_arithmetic_nan\s(\(invoke\s"f64\.promote_f32"\s\((f[0-9]{2})\.const\s[a-z0-9.+:-]+\)\))\)/\(assert_return $$1 \(f64.const nan:arithmetic\)\)/g' $$f; \
		perl -pi -e 's/\(assert_return_canonical_nan\s(\(invoke\s"[a-z._0-9]+"\s\((f[0-9]{2})\.const\s[a-z0-9.+:-]+\)\))\)/\(assert_return $$1 \($$2.const nan:canonical\)\)/g' $$f; \
		perl -pi -e 's/\(assert_return_arithmetic_nan\s(\(invoke\s"[a-z._0-9]+"\s\((f[0-9]{2})\.const\s[a-z0-9.+:-]+\)\))\)/\(assert_return $$1 \($$2.const nan:arithmetic\)\)/g' $$f; \
		perl -pi -e 's/\(assert_return_canonical_nan\s(\(invoke\s"[a-z._0-9]+"\s\((f[0-9]{2})\.const\s[a-z0-9.+:-]+\)\s\([a-z0-9.\s+-:]+\)\))\)/\(assert_return $$1 \($$2.const nan:canonical\)\)/g' $$f; \
		perl -pi -e 's/\(assert_return_arithmetic_nan\s(\(invoke\s"[a-z._0-9]+"\s\((f[0-9]{2})\.const\s[a-z0-9.+:-]+\)\s\([a-z0-9.\s+-:]+\)\))\)/\(assert_return $$1 \($$2.const nan:arithmetic\)\)/g' $$f; \
		perl -pi -e 's/\(assert_return_canonical_nan\s(\(invoke\s"[a-z._0-9]+"\s\((f[0-9]{2})\.const\s[a-z0-9.+:-]+\)\))\)/\(assert_return $$1 \($$2.const nan:canonical\)\)/g' $$f; \
		perl -pi -e 's/\(assert_return_arithmetic_nan\s(\(invoke\s"[a-z._0-9]+"\s\((f[0-9]{2})\.const\s[a-z0-9.+:-]+\)\))\)/\(assert_return $$1 \($$2.const nan:arithmetic\)\)/g' $$f; \
		wast2json \
			--disable-saturating-float-to-int \
			--disable-sign-extension \
			--disable-simd \
			--disable-multi-value \
			--disable-bulk-memory \
			--disable-reference-types \
			--debug-names $$f; \
	done

.PHONY: build.spectest.v2
build.spectest.v2: # Note: SIMD cases are placed in the "simd" subdirectory.
	@mkdir -p $(spectest_v2_testdata_dir)
	@cd $(spectest_v2_testdata_dir) \
		&& curl -sSL 'https://api.github.com/repos/WebAssembly/spec/contents/test/core?ref=$(spec_version_v2)' | jq -r '.[]| .download_url' | grep -E ".wast" | xargs -Iurl curl -sJL url -O
	@cd $(spectest_v2_testdata_dir) \
		&& curl -sSL 'https://api.github.com/repos/WebAssembly/spec/contents/test/core/simd?ref=$(spec_version_v2)' | jq -r '.[]| .download_url' | grep -E ".wast" | xargs -Iurl curl -sJL url -O
	@cd $(spectest_v2_testdata_dir) && for f in `find . -name '*.wast'`; do \
		wast2json --debug-names --no-check $$f || true; \
	done # Ignore the error here as some tests (e.g. comments.wast right now) are not supported by wast2json yet.

# Note: We currently cannot build the "threads" subdirectory that spawns threads due to missing support in wast2json.
# https://github.com/WebAssembly/wabt/issues/2348#issuecomment-1878003959
.PHONY: build.spectest.threads
build.spectest.threads:
	@mkdir -p $(spectest_threads_testdata_dir)
	@cd $(spectest_threads_testdata_dir) \
		&& curl -sSL 'https://api.github.com/repos/WebAssembly/threads/contents/test/core?ref=$(spec_version_threads)' | jq -r '.[]| .download_url' | grep -E "atomic.wast" | xargs -Iurl curl -sJL url -O
	@cd $(spectest_threads_testdata_dir) && for f in `find . -name '*.wast'`; do \
		wast2json --enable-threads --debug-names $$f; \
	done

.PHONY: test
test:
	@go test $(go_test_options) ./...
	@cd internal/version/testdata && go test $(go_test_options) ./...
	@cd internal/integration_test/fuzz/wazerolib && CGO_ENABLED=0 WASM_BINARY_PATH=testdata/test.wasm go test ./...

.PHONY: coverage
# replace spaces with commas
coverpkg = $(shell echo $(main_packages) | tr ' ' ',')
coverage: ## Generate test coverage
	@go test -coverprofile=coverage.txt -covermode=atomic --coverpkg=$(coverpkg) $(main_packages)
	@go tool cover -func coverage.txt

golangci_lint_path := $(shell go env GOPATH)/bin/golangci-lint

$(golangci_lint_path):
	@go install $(golangci_lint)

golangci_lint_goarch ?= $(shell go env GOARCH)

.PHONY: lint
lint: $(golangci_lint_path)
	@GOARCH=$(golangci_lint_goarch) CGO_ENABLED=0 $(golangci_lint_path) run --timeout 5m -E testableexamples

.PHONY: format
format:
	@go run $(gofumpt) -l -w .
	@go run $(gosimports) -local github.com/tetratelabs/ -w $(shell find . -name '*.go' -type f)
	@go run $(asmfmt) -w $(shell find . -name '*.s' -type f)

.PHONY: check  # Pre-flight check for pull requests
check:
# The following checks help ensure our platform-specific code used for system
# calls safely falls back on a platform unsupported by the compiler engine.
# This makes sure the intepreter can be used. Most often the package that can
# drift here is "platform" or "sysfs":
#
# Ensure we build on plan9. See #1578
	@GOARCH=amd64 GOOS=plan9 go build ./...
# Ensure we build on gojs. See #1526.
	@GOARCH=wasm GOOS=js go build ./...
# Ensure we build on wasip1. See #1526.
	@GOARCH=wasm GOOS=wasip1 go build ./...
# Ensure we build on aix. See #1723
	@GOARCH=ppc64 GOOS=aix go build ./...
# Ensure we build on windows:
	@GOARCH=amd64 GOOS=windows go build ./...
# Ensure we build on an arbitrary operating system:
	@GOARCH=amd64 GOOS=dragonfly go build ./...
# Ensure we build on solaris/illumos:
	@GOARCH=amd64 GOOS=illumos go build ./...
	@GOARCH=amd64 GOOS=solaris go build ./...
# Ensure we build on linux arm for Dapr:
#	gh release view -R dapr/dapr --json assets --jq 'first(.assets[] | select(.name = "daprd_linux_arm.tar.gz") | {url, downloadCount})'
	@GOARCH=arm GOOS=linux go build ./...
# Ensure we build on linux 386 for Trivy:
#	gh release view -R aquasecurity/trivy --json assets --jq 'first(.assets[] | select(.name| test("Linux-32bit.*tar.gz")) | {url, downloadCount})'
	@GOARCH=386 GOOS=linux go build ./...
# Ensure we build on FreeBSD amd64 for Trivy:
#	gh release view -R aquasecurity/trivy --json assets --jq 'first(.assets[] | select(.name| test("FreeBSD-64bit.*tar.gz")) | {url, downloadCount})'
	@GOARCH=amd64 GOOS=freebsd go build ./...
	@$(MAKE) lint golangci_lint_goarch=arm64
	@$(MAKE) lint golangci_lint_goarch=amd64
	@$(MAKE) format
	@go mod tidy
	@if [ ! -z "`git status -s`" ]; then \
		echo "The following differences will fail CI until committed:"; \
		git diff --exit-code; \
	fi

.PHONY: site
site: ## Serve website content
	@git submodule update --init
	@cd site && go run $(hugo) server --minify --disableFastRender --baseURL localhost:1313 --cleanDestinationDir -D

.PHONY: clean
clean: ## Ensure a clean build
	@rm -rf dist build coverage.txt
	@go clean -testcache

fuzz_default_flags := --no-trace-compares --sanitizer=none -- -rss_limit_mb=8192

fuzz_timeout_seconds ?= 10
.PHONY: fuzz
fuzz:
	@cd internal/integration_test/fuzz && cargo test
	@cd internal/integration_test/fuzz && cargo fuzz run logging_no_diff $(fuzz_default_flags) -max_total_time=$(fuzz_timeout_seconds)
	@cd internal/integration_test/fuzz && cargo fuzz run no_diff $(fuzz_default_flags) -max_total_time=$(fuzz_timeout_seconds)
	@cd internal/integration_test/fuzz && cargo fuzz run memory_no_diff $(fuzz_default_flags) -max_total_time=$(fuzz_timeout_seconds)
	@cd internal/integration_test/fuzz && cargo fuzz run validation $(fuzz_default_flags) -max_total_time=$(fuzz_timeout_seconds)

libsodium:
	cd ./internal/integration_test/libsodium/testdata && \
		curl -s "https://api.github.com/repos/jedisct1/webassembly-benchmarks/contents/2022-12/wasm?ref=7e86d68e99e60130899fbe3b3ab6e9dce9187a7c" \
		| jq -r '.[] | .download_url' | xargs -n 1 curl -LO

#### CLI release related ####

VERSION ?= dev
# Default to a dummy version 0.0.1.1, which is always lower than a real release.
# Legal version values should look like 'x.x.x.x' where x is an integer from 0 to 65534.
# https://learn.microsoft.com/en-us/windows/win32/msi/productversion?redirectedfrom=MSDN
# https://stackoverflow.com/questions/9312221/msi-version-numbers
MSI_VERSION ?= 0.0.1.1
non_windows_platforms := darwin_amd64 darwin_arm64 linux_amd64 linux_arm64
non_windows_archives  := $(non_windows_platforms:%=dist/wazero_$(VERSION)_%.tar.gz)
windows_platforms     := windows_amd64 # TODO: add arm64 windows once we start testing on it.
windows_archives      := $(windows_platforms:%=dist/wazero_$(VERSION)_%.zip) $(windows_platforms:%=dist/wazero_$(VERSION)_%.msi)
checksum_txt          := dist/wazero_$(VERSION)_checksums.txt

# define macros for multi-platform builds. these parse the filename being built
go-arch = $(if $(findstring amd64,$1),amd64,arm64)
go-os   = $(if $(findstring .exe,$1),windows,$(if $(findstring linux,$1),linux,darwin))
# msi-arch is a macro so we can detect it based on the file naming convention
msi-arch     = $(if $(findstring amd64,$1),x64,arm64)

build/wazero_%/wazero:
	$(call go-build,$@,$<)

build/wazero_%/wazero.exe:
	$(call go-build,$@,$<)

dist/wazero_$(VERSION)_%.tar.gz: build/wazero_%/wazero
	@echo tar.gz "tarring $@"
	@mkdir -p $(@D)
# On Windows, we pass the special flag `--mode='+rx' to ensure that we set the executable flag.
# This is only supported by GNU Tar, so we set it conditionally.
	@tar -C $(<D) -cpzf $@ $(if $(findstring Windows_NT,$(OS)),--mode='+rx',) $(<F)
	@echo tar.gz "ok"

define go-build
	@echo "building $1"
	@# $(go:go=) removes the trailing 'go', so we can insert cross-build variables
	@$(go:go=) CGO_ENABLED=0 GOOS=$(call go-os,$1) GOARCH=$(call go-arch,$1) go build \
		-ldflags "-s -w -X github.com/tetratelabs/wazero/internal/version.version=$(VERSION)" \
		-o $1 $2 ./cmd/wazero
	@echo build "ok"
endef

# this makes a marker file ending in .signed to avoid repeatedly calling codesign
%.signed: %
	$(call codesign,$<)
	@touch $@

# This requires osslsigncode package (apt or brew) or latest windows release from mtrojnar/osslsigncode
#
# Default is self-signed while production should be a Digicert signing key
#
# Ex.
# ```bash
# keytool -genkey -alias wazero -storetype PKCS12 -keyalg RSA -keysize 2048 -storepass wazero-bunch \
# -keystore wazero.p12 -dname "O=wazero,CN=wazero.io" -validity 3650
# ```
WINDOWS_CODESIGN_P12      ?= packaging/msi/wazero.p12
WINDOWS_CODESIGN_PASSWORD ?= wazero-bunch
define codesign
	@printf "$(ansi_format_dark)" codesign "signing $1"
	@osslsigncode sign -h sha256 -pkcs12 ${WINDOWS_CODESIGN_P12} -pass "${WINDOWS_CODESIGN_PASSWORD}" \
	-n "wazero is the zero dependency WebAssembly runtime for Go developers" -i https://wazero.io -t http://timestamp.digicert.com \
	$(if $(findstring msi,$(1)),-add-msi-dse) -in $1 -out $1-signed
	@mv $1-signed $1
	@printf "$(ansi_format_bright)" codesign "ok"
endef

# This task is only supported on Windows, where we use candle.exe (compile wxs to wixobj) and light.exe (link to msi)
dist/wazero_$(VERSION)_%.msi: build/wazero_%/wazero.exe.signed
ifeq ($(OS),Windows_NT)
	@echo msi "building $@"
	@mkdir -p $(@D)
	@candle -nologo -arch $(call msi-arch,$@) -dVersion=$(MSI_VERSION) -dBin=$(<:.signed=) -o build/wazero.wixobj packaging/msi/wazero.wxs
	@light -nologo -o $@ build/wazero.wixobj -spdb
	$(call codesign,$@)
	@echo msi "ok"
endif

dist/wazero_$(VERSION)_%.zip: build/wazero_%/wazero.exe.signed
	@echo zip "zipping $@"
	@mkdir -p $(@D)
	@zip -qj $@ $(<:.signed=)
	@echo zip "ok"

# Darwin doesn't have sha256sum. See https://github.com/actions/virtual-environments/issues/90
sha256sum := $(if $(findstring darwin,$(shell go env GOOS)),shasum -a 256,sha256sum)
$(checksum_txt):
	@cd $(@D); touch $(@F); $(sha256sum) * >> $(@F)

dist: $(non_windows_archives) $(if $(findstring Windows_NT,$(OS)),$(windows_archives),) $(checksum_txt)
