### go-csvvalue

![GitHub Release](https://img.shields.io/github/v/release/tonistiigi/go-csvvalue)
[![Go Reference](https://pkg.go.dev/badge/github.com/tonistiigi/go-csvvalue.svg)](https://pkg.go.dev/github.com/tonistiigi/go-csvvalue)
![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/tonistiigi/go-csvvalue/ci.yml)
![Codecov](https://img.shields.io/codecov/c/github/tonistiigi/go-csvvalue)
![GitHub License](https://img.shields.io/github/license/tonistiigi/go-csvvalue)


`go-csvvalue` provides an efficient parser for a single-line CSV value.

It is more efficient than the standard library `encoding/csv` package for parsing many small values. The main problem with stdlib implementation is that it calls `bufio.NewReader` internally, allocating 4KB of memory on each invocation. For multi-line CSV parsing, the standard library is still recommended. If you wish to optimize memory usage for `encoding/csv`, call `csv.NewReader` with an instance of `*bufio.Reader` that already has a 4KB buffer allocated and then reuse that buffer for all reads.

For further memory optimization, an existing string slice can be optionally passed to be reused for returning the parsed fields.

For backwards compatibility with stdlib record parser, the input may contain a trailing newline character.

### Benchmark

```
goos: linux
goarch: amd64
pkg: github.com/tonistiigi/go-csvvalue
cpu: AMD EPYC 7763 64-Core Processor                
BenchmarkFields/stdlib/withcache-4         	 1109917	      1103 ns/op	    4520 B/op	      14 allocs/op
BenchmarkFields/stdlib/nocache-4           	 1082838	      1125 ns/op	    4520 B/op	      14 allocs/op
BenchmarkFields/csvvalue/withcache-4       	28554976	        42.12 ns/op	       0 B/op	       0 allocs/op
BenchmarkFields/csvvalue/nocache-4         	13666134	        83.77 ns/op	      48 B/op	       1 allocs/op
```
```
goos: darwin
goarch: arm64
pkg: github.com/tonistiigi/go-csvvalue
BenchmarkFields/stdlib/nocache-10                1679923               784.9 ns/op          4520 B/op         14 allocs/op
BenchmarkFields/stdlib/withcache-10              1641891               826.9 ns/op          4520 B/op         14 allocs/op
BenchmarkFields/csvvalue/withcache-10           34399642                33.93 ns/op            0 B/op          0 allocs/op
BenchmarkFields/csvvalue/nocache-10             17441373                67.21 ns/op           48 B/op          1 allocs/op
PASS
```

### Credits

This package is mostly based on `encoding/csv` implementation and also uses that package for compatibility testing.

