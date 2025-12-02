# wazero: the zero dependency WebAssembly runtime for Go developers

[![Go Reference](https://pkg.go.dev/badge/github.com/tetratelabs/wazero.svg)](https://pkg.go.dev/github.com/tetratelabs/wazero) [![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

WebAssembly is a way to safely run code compiled in other languages. Runtimes
execute WebAssembly Modules (Wasm), which are most often binaries with a `.wasm`
extension.

wazero is a WebAssembly Core Specification [1.0][1] and [2.0][2] compliant
runtime written in Go. It has *zero dependencies*, and doesn't rely on CGO.
This means you can run applications in other languages and still keep cross
compilation.

Import wazero and extend your Go application with code written in any language!

## Example

The best way to learn wazero is by trying one of our [examples](examples/README.md). The
most [basic example](examples/basic) extends a Go application with an addition
function defined in WebAssembly.

## Runtime

There are two runtime configurations supported in wazero: _Compiler_ is default:

By default, ex `wazero.NewRuntime(ctx)`, the Compiler is used if supported. You
can also force the interpreter like so:
```go
r := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfigInterpreter())
```

### Interpreter
Interpreter is a naive interpreter-based implementation of Wasm virtual
machine. Its implementation doesn't have any platform (GOARCH, GOOS) specific
code, therefore _interpreter_ can be used for any compilation target available
for Go (such as `riscv64`).

### Compiler
Compiler compiles WebAssembly modules into machine code ahead of time (AOT),
during `Runtime.CompileModule`. This means your WebAssembly functions execute
natively at runtime. Compiler is faster than Interpreter, often by order of
magnitude (10x) or more. This is done without host-specific dependencies.

### Conformance

Both runtimes pass WebAssembly Core [1.0][7] and [2.0][14] specification tests
on supported platforms:

|   Runtime   |                 Usage                  | amd64 | arm64 | others |
|:-----------:|:--------------------------------------:|:-----:|:-----:|:------:|
| Interpreter | `wazero.NewRuntimeConfigInterpreter()` |   ✅   |   ✅   |   ✅    |
|  Compiler   |  `wazero.NewRuntimeConfigCompiler()`   |   ✅   |   ✅   |   ❌    |

## Support Policy

The below support policy focuses on compatibility concerns of those embedding
wazero into their Go applications.

### wazero

wazero's [1.0 release][15] happened in March 2023, and is [in use][16] by many
projects and production sites.

We offer an API stability promise with semantic versioning. In other words, we
promise to not break any exported function signature without incrementing the
major version. This does not mean no innovation: New features and behaviors
happen with a minor version increment, e.g. 1.0.11 to 1.2.0. We also fix bugs
or change internal details with a patch version, e.g. 1.0.0 to 1.0.1.

You can get the latest version of wazero like this.
```bash
go get github.com/tetratelabs/wazero@latest
```

Please give us a [star][17] if you end up using wazero!

### Go

wazero has no dependencies except Go, so the only source of conflict in your
project's use of wazero is the Go version.

wazero follows the same version policy as Go's [Release Policy][10]: two
versions. wazero will ensure these versions work and bugs are valid if there's
an issue with a current Go version.

Additionally, wazero intentionally delays usage of language or standard library
features one additional version. For example, when Go 1.29 is released, wazero
can use language features or standard libraries added in 1.27. This is a
convenience for embedders who have a slower version policy than Go. However,
only supported Go versions may be used to raise support issues.

### Platform

wazero has two runtime modes: Interpreter and Compiler. The only supported operating
systems are ones we test, but that doesn't necessarily mean other operating
system versions won't work.

We currently test Linux (Ubuntu and scratch), MacOS and Windows as packaged by
[GitHub Actions][11], as well as nested VMs running on Linux for FreeBSD, NetBSD,
OpenBSD, DragonFly BSD, illumos and Solaris.

We also test cross compilation for many `GOOS` and `GOARCH` combinations.

* Interpreter
  * Linux is tested on amd64 (native) as well arm64 and riscv64 via emulation.
  * Windows, FreeBSD, NetBSD, OpenBSD, DragonFly BSD, illumos and Solaris are
    tested only on amd64.
  * macOS is tested only on arm64.
* Compiler
  * Linux is tested on amd64 (native) as well arm64 via emulation.
  * Windows, FreeBSD, NetBSD, DragonFly BSD, illumos and Solaris are
    tested only on amd64.
  * macOS is tested only on arm64.

wazero has no dependencies and doesn't require CGO. This means it can also be
embedded in an application that doesn't use an operating system. This is a main
differentiator between wazero and alternatives.

We verify zero dependencies by running tests in Docker's [scratch image][12].
This approach ensures compatibility with any parent image.

-----
wazero is a registered trademark of Tetrate.io, Inc. in the United States and/or other countries

[1]: https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/
[2]: https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/
[4]: https://github.com/WebAssembly/meetings/blob/main/process/subgroups.md
[5]: https://github.com/WebAssembly/WASI
[6]: https://pkg.go.dev/golang.org/x/sys/unix
[7]: https://github.com/WebAssembly/spec/tree/wg-1.0/test/core
[9]: https://github.com/tetratelabs/wazero/issues/506
[10]: https://go.dev/doc/devel/release
[11]: https://github.com/actions/virtual-environments
[12]: https://docs.docker.com/develop/develop-images/baseimages/#create-a-simple-parent-image-using-scratch
[13]: https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md
[14]: https://github.com/WebAssembly/spec/tree/d39195773112a22b245ffbe864bab6d1182ccb06/test/core
[15]: https://tetrate.io/blog/introducing-wazero-from-tetrate/
[16]: https://wazero.io/community/users/
[17]: https://github.com/tetratelabs/wazero/stargazers
