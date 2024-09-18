# Singleflight

[![GoDoc](https://godoc.org/resenje.org/singleflight?status.svg)](https://godoc.org/resenje.org/singleflight)
[![Go](https://github.com/janos/singleflight/workflows/Go/badge.svg)](https://github.com/janos/singleflight/actions?query=workflow%3AGo)

Package singleflight provides a duplicate function call suppression
mechanism similar to golang.org/x/sync/singleflight but with support
for context cancelation.

## Installation

Run `go get resenje.org/singleflight` from command line.