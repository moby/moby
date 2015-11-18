# crc32
CRC32 hash with x64 optimizations

This package is a drop-in replacement for the standard library `hash/crc32` package, that features SSE 4.2 optimizations on x64 platforms, for a 10x speedup.

[![Build Status](https://travis-ci.org/klauspost/crc32.svg?branch=master)](https://travis-ci.org/klauspost/crc32)

# usage

Install using `go get github.com/klauspost/crc32`. This library is based on Go 1.4.2 code and requires Go 1.3 or newer.

Replace `import "hash/crc32"` with `import "github.com/klauspost/crc32"` and you are good to go.

# performance

For IEEE tables (the most common), there is approximately a factor 10 speedup with SSE 4.2:
```
benchmark            old ns/op     new ns/op     delta
BenchmarkCrc32KB     99955         10258         -89.74%

benchmark            old MB/s     new MB/s     speedup
BenchmarkCrc32KB     327.83       3194.20      9.74x
```

For other tables and non-SSE 4.2 the peformance is the same as the standard library.

# license

Standard Go license. Changes are Copyright (c) 2015 Klaus Post under same conditions.
