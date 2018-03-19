# Package for equality of Go values

[![GoDoc](https://godoc.org/github.com/google/go-cmp/cmp?status.svg)][godoc]
[![Build Status](https://travis-ci.org/google/go-cmp.svg?branch=master)][travis]

This package is intended to be a more powerful and safer alternative to
`reflect.DeepEqual` for comparing whether two values are semantically equal.

The primary features of `cmp` are:

* When the default behavior of equality does not suit the needs of the test,
  custom equality functions can override the equality operation.
  For example, an equality function may report floats as equal so long as they
  are within some tolerance of each other.

* Types that have an `Equal` method may use that method to determine equality.
  This allows package authors to determine the equality operation for the types
  that they define.

* If no custom equality functions are used and no `Equal` method is defined,
  equality is determined by recursively comparing the primitive kinds on both
  values, much like `reflect.DeepEqual`. Unlike `reflect.DeepEqual`, unexported
  fields are not compared by default; they result in panics unless suppressed
  by using an `Ignore` option (see `cmpopts.IgnoreUnexported`) or explicitly
  compared using the `AllowUnexported` option.

See the [GoDoc documentation][godoc] for more information.

This is not an official Google product.

[godoc]: https://godoc.org/github.com/google/go-cmp/cmp
[travis]: https://travis-ci.org/google/go-cmp

## Install

```
go get -u github.com/google/go-cmp/cmp
```

## License

BSD - See [LICENSE][license] file

[license]: https://github.com/google/go-cmp/blob/master/LICENSE
