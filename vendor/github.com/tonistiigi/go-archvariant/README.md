# go-archvariant

[![Go Reference](https://pkg.go.dev/badge/github.com/tonistiigi/go-archvariant.svg)](https://pkg.go.dev/github.com/tonistiigi/go-archvariant)
[![Build Status](https://github.com/tonistiigi/go-archvariant/workflows/ci/badge.svg)](https://github.com/tonistiigi/go-archvariant/actions)

Go package for determining the maximum compatibility version of the current system. The main use case is to use this value in container [platform definitions](https://github.com/containerd/containerd/blob/v1.5.9/platforms/platforms.go#L55).

On x86-64 platforms this package returns the maximum current microarchitecture level as defined in https://en.wikipedia.org/wiki/X86-64#Microarchitecture_levels . This value can be used to configure compiler in [LLVM since 12.0](https://github.com/llvm/llvm-project/commit/012dd42e027e2ff3d183cc9dcf27004cf9711720) and [GCC since 11.0](https://github.com/gcc-mirror/gcc/commit/324bec558e95584e8c1997575ae9d75978af59f1). [Go1.18+](https://tip.golang.org/doc/go1.18#amd64) uses `GOAMD64` environemnt to configure Go compiler with this value.

#### Scope

The goal of this repository is to only provide the variant with minimal external dependencies. If you need more specific CPU features detection you should look at [`golang.org/x/sys/cpu`](https://pkg.go.dev/golang.org/x/sys/cpu) or [`github.com/klauspost/cpuid`](https://pkg.go.dev/github.com/klauspost/cpuid/v2) instead.

#### Credits

The checks in this repository are based on the checks Go runtime does [on startup](https://github.com/golang/go/blob/go1.18beta1/src/runtime/asm_amd64.s#L95-L96).
