# Singleflight

[![GoDoc](https://godoc.org/resenje.org/singleflight?status.svg)](https://godoc.org/resenje.org/singleflight)
[![Go](https://github.com/janos/singleflight/workflows/Go/badge.svg)](https://github.com/janos/singleflight/actions?query=workflow%3AGo)

Package singleflight provides a duplicate function call suppression 
mechanism similar to [golang.org/x/sync/singleflight](https://pkg.go.dev/golang.org/x/sync/singleflight) but with:

- support for context cancelation. The context passed to the callback function is a context that preserves all values
from the passed context but is cancelled by the singleflight only when all awaiting caller's contexts are cancelled.
- support for generics.

## Installation

Run `go get resenje.org/singleflight` from command line.