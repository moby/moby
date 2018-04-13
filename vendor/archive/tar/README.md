This is a fork of Go 1.10 `archive/tar` package from the official
[repo](https://github.com/golang/go/tree/release-branch.go1.10/src/archive/tar),
with a partial [revert](https://github.com/kolyshkin/go-tar/commit/d651d6e45972363e9bb62b8e9d876df440b31628)
of upstream [commit 0564e304a6ea](https://github.com/golang/go/commit/0564e304a6ea394a42929060c588469dbd6f32af).
It is suggested as a replacement to the original package included with Go 1.10
in case you want to build a static Linux/glibc binary that works, and
can't afford to use `CGO_ENABLED=0`.

## Details

Using Go 1.10 [archive/tar](https://golang.org/pkg/archive/tar/) from a static binary
compiled with glibc on Linux can result in a panic upon calling
[`tar.FileInfoHeader()`](https://golang.org/pkg/archive/tar/#FileInfoHeader).
This is a major regression in Go 1.10, filed as
[Go issue #24787](https://github.com/golang/go/issues/24787).

The above issue is caused by an unfortunate combination of:
1. glibc way of dynamic loading of nss libraries even for a static build;
2. Go `os/user` package hard-coded reliance on libc to resolve user/group IDs to names (unless CGO is disabled).

While glibc can probably not be fixed and is not considered a bug per se,
the `os/user` issue is documented (see [Go issue #23265](https://github.com/golang/go/issues/23265))
and already fixed by [Go commit 62f0127d81](https://github.com/golang/go/commit/62f0127d8104d8266d9a3fb5a87e2f09ec8b6f5b).
The fix is expected to make its way to Go 1.11, and requires `osusergo` build tag
to be used for a static build.

This repository serves as a temporary workaround until the above fix is available.
