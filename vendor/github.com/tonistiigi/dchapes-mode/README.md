Mode
========

This is a fork of [hg.sr.ht/~dchapes/mode](https://hg.sr.ht/~dchapes/mode) with minimal patches and basic CI.

[Mode](https://hg.sr.ht/~dchapes/mode)
is a [Go](http://golang.org/) package that provides
a native Go implementation of BSD's
[`setmode`](https://www.freebsd.org/cgi/man.cgi?query=setmode&sektion=3)
and `getmode` which can be used to modify the mode bits of
an [`os.FileMode`](https://golang.org/pkg/os#FileMode) value
based on a symbolic value as described by the
Unix [`chmod`](https://www.freebsd.org/cgi/man.cgi?query=chmod&sektion=1) command.

[![Go Reference](https://pkg.go.dev/badge/hg.sr.ht/~dchapes/mode.svg)](https://pkg.go.dev/hg.sr.ht/~dchapes/mode)

Online package documentation is available via
[pkg.go.dev](https://pkg.go.dev/hg.sr.ht/~dchapes/mode).

To install:

		go get hg.sr.ht/~dchapes/mode

or `go build` any Go code that imports it:

		import "hg.sr.ht/~dchapes/mode"
