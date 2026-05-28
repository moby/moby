# hashstructure [![GoDoc](https://godoc.org/github.com/mitchellh/hashstructure?status.svg)](https://godoc.org/github.com/mitchellh/hashstructure)

hashstructure is a Go library for creating a unique hash value
for arbitrary values in Go.

This can be used to key values in a hash (for use in a map, set, etc.)
that are complex. The most common use case is comparing two values without
sending data across the network, caching values locally (de-dup), and so on.

## Features

  * Hash any arbitrary Go value, including complex types.

  * Tag a struct field to ignore it and not affect the hash value.

  * Tag a slice type struct field to treat it as a set where ordering
    doesn't affect the hash code but the field itself is still taken into
    account to create the hash value.

  * Optionally, specify a custom hash function to optimize for speed, collision
    avoidance for your data set, etc.

  * Optionally, hash the output of `.String()` on structs that implement fmt.Stringer,
    allowing effective hashing of time.Time

  * Optionally, override the hashing process by implementing `Hashable`.

## Installation

Standard `go get`:

```
$ go get github.com/mitchellh/hashstructure/v2
```

**Note on v2:** It is highly recommended you use the "v2" release since this
fixes some significant hash collisions issues from v1. In practice, we used
v1 for many years in real projects at HashiCorp and never had issues, but it
is highly dependent on the shape of the data you're hashing and how you use
those hashes.

When using v2+, you can still generate weaker v1 hashes by using the
`FormatV1` format when calling `Hash`.

## Usage & Example

For usage and examples see the [Godoc](http://godoc.org/github.com/mitchellh/hashstructure).

A quick code example is shown below:

```go
type ComplexStruct struct {
    Name     string
    Age      uint
    Metadata map[string]interface{}
}

v := ComplexStruct{
    Name: "mitchellh",
    Age:  64,
    Metadata: map[string]interface{}{
        "car":      true,
        "location": "California",
        "siblings": []string{"Bob", "John"},
    },
}

hash, err := hashstructure.Hash(v, hashstructure.FormatV2, nil)
if err != nil {
    panic(err)
}

fmt.Printf("%d", hash)
// Output:
// 2307517237273902113
```
