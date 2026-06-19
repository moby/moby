# go-toml v2

Go library for the [TOML](https://toml.io/en/) format.

This library supports [TOML v1.1.0](https://toml.io/en/v1.1.0).

[🐞 Bug Reports](https://github.com/pelletier/go-toml/issues)

[💬 Anything else](https://github.com/pelletier/go-toml/discussions)

## Documentation

Full API, examples, and implementation notes are available in the Go
documentation.

[![Go Reference](https://pkg.go.dev/badge/github.com/pelletier/go-toml/v2.svg)](https://pkg.go.dev/github.com/pelletier/go-toml/v2)

## Import

```go
import "github.com/pelletier/go-toml/v2"
```

## Features

### Stdlib behavior

As much as possible, this library is designed to behave similarly as the
standard library's `encoding/json`.

When encoding structs, fields tagged with `omitempty` are omitted if they are
empty. For `time.Time`, the zero value is considered empty, so timestamps such
as `created_at` or `updated_at` are not written unless you remove `omitempty`
from the struct tag or use a pointer type (`*time.Time`).

### Performance

While go-toml favors usability, it is written with performance in mind. Most
operations should not be shockingly slow. See [benchmarks](#benchmarks).

### Strict mode

`Decoder` can be set to "strict mode", which makes it error when some parts of
the TOML document was not present in the target structure. This is a great way
to check for typos. [See example in the documentation][strict].

[strict]: https://pkg.go.dev/github.com/pelletier/go-toml/v2#example-Decoder.DisallowUnknownFields

### Contextualized errors

When most decoding errors occur, go-toml returns [`DecodeError`][decode-err],
which contains a human readable contextualized version of the error. For
example:

```
1| [server]
2| path = 100
 |        ~~~ cannot decode TOML integer into struct field toml_test.Server.Path of type string
3| port = 50
```

[decode-err]: https://pkg.go.dev/github.com/pelletier/go-toml/v2#DecodeError

### Local date and time support

TOML supports native [local date/times][ldt]. It allows to represent a given
date, time, or date-time without relation to a timezone or offset. To support
this use-case, go-toml provides [`LocalDate`][tld], [`LocalTime`][tlt], and
[`LocalDateTime`][tldt]. Those types can be transformed to and from `time.Time`,
making them convenient yet unambiguous structures for their respective TOML
representation.

[ldt]: https://toml.io/en/v1.1.0#local-date-time
[tld]: https://pkg.go.dev/github.com/pelletier/go-toml/v2#LocalDate
[tlt]: https://pkg.go.dev/github.com/pelletier/go-toml/v2#LocalTime
[tldt]: https://pkg.go.dev/github.com/pelletier/go-toml/v2#LocalDateTime

### Commented config

Since TOML is often used for configuration files, go-toml can emit documents
annotated with [comments and commented-out values][comments-example]. For
example, it can generate the following file:

```toml
# Host IP to connect to.
host = '127.0.0.1'
# Port of the remote server.
port = 4242

# Encryption parameters (optional)
# [TLS]
# cipher = 'AEAD-AES128-GCM-SHA256'
# version = 'TLS 1.3'
```

[comments-example]: https://pkg.go.dev/github.com/pelletier/go-toml/v2#example-Marshal-Commented

## Getting started

Given the following struct, let's see how to read it and write it as TOML:

```go
type MyConfig struct {
	Version int
	Name    string
	Tags    []string
}
```

### Unmarshaling

[`Unmarshal`][unmarshal] reads a TOML document and fills a Go structure with its
content. 

Note that the struct variable names are _capitalized_, while the variables in the toml document are _lowercase_.

For example:

```go
doc := `
version = 2
name = "go-toml"
tags = ["go", "toml"]
`

var cfg MyConfig
err := toml.Unmarshal([]byte(doc), &cfg)
if err != nil {
	panic(err)
}
fmt.Println("version:", cfg.Version)
fmt.Println("name:", cfg.Name)
fmt.Println("tags:", cfg.Tags)

// Output:
// version: 2
// name: go-toml
// tags: [go toml]
```

[unmarshal]: https://pkg.go.dev/github.com/pelletier/go-toml/v2#Unmarshal


Here is an example using tables with some simple nesting:

```go
doc := `
age = 45
fruits = ["apple", "pear"]

# these are very important!
[my-variables]
first = 1
second = 0.2
third = "abc"

# this is not so important.
[my-variables.b]
bfirst = 123
`

var Document struct {
	Age int
	Fruits []string

	Myvariables struct {
		First  int
		Second float64
		Third  string

		B struct {
			Bfirst int
		}
	} `toml:"my-variables"`
}

err := toml.Unmarshal([]byte(doc), &Document)
if err != nil {
	panic(err)
}

fmt.Println("age:", Document.Age)
fmt.Println("fruits:", Document.Fruits)
fmt.Println("my-variables.first:", Document.Myvariables.First)
fmt.Println("my-variables.second:", Document.Myvariables.Second)
fmt.Println("my-variables.third:", Document.Myvariables.Third)
fmt.Println("my-variables.B.Bfirst:", Document.Myvariables.B.Bfirst)

// Output:
// age: 45
// fruits: [apple pear]
// my-variables.first: 1
// my-variables.second: 0.2
// my-variables.third: abc
// my-variables.B.Bfirst: 123
```


### Marshaling

[`Marshal`][marshal] is the opposite of Unmarshal: it represents a Go structure
as a TOML document:

```go
cfg := MyConfig{
	Version: 2,
	Name:    "go-toml",
	Tags:    []string{"go", "toml"},
}

b, err := toml.Marshal(cfg)
if err != nil {
	panic(err)
}
fmt.Println(string(b))

// Output:
// Version = 2
// Name = 'go-toml'
// Tags = ['go', 'toml']
```

[marshal]: https://pkg.go.dev/github.com/pelletier/go-toml/v2#Marshal

## Unstable API

This API does not yet follow the backward compatibility guarantees of this
library. They provide early access to features that may have rough edges or an
API subject to change.

### Parser

Parser is the unstable API that allows iterative parsing of a TOML document at
the AST level. See https://pkg.go.dev/github.com/pelletier/go-toml/v2/unstable.

## Benchmarks

Execution time speedup compared to other Go TOML libraries:

<table>
    <thead>
        <tr><th>Benchmark</th><th>go-toml v1</th><th>BurntSushi/toml</th></tr>
    </thead>
    <tbody>
        <tr><td>Marshal/HugoFrontMatter-2</td><td>2.3x</td><td>2.4x</td></tr>
        <tr><td>Marshal/ReferenceFile/map-2</td><td>2.2x</td><td>2.6x</td></tr>
        <tr><td>Marshal/ReferenceFile/struct-2</td><td>4.9x</td><td>5.0x</td></tr>
        <tr><td>Unmarshal/HugoFrontMatter-2</td><td>7.8x</td><td>5.9x</td></tr>
        <tr><td>Unmarshal/ReferenceFile/map-2</td><td>6.8x</td><td>6.4x</td></tr>
        <tr><td>Unmarshal/ReferenceFile/struct-2</td><td>6.8x</td><td>6.3x</td></tr>
     </tbody>
</table>
<details><summary>See more</summary>
<p>The table above has the results of the most common use-cases. The table below
contains the results of all benchmarks, including unrealistic ones. It is
provided for completeness.</p>

<table>
    <thead>
        <tr><th>Benchmark</th><th>go-toml v1</th><th>BurntSushi/toml</th></tr>
    </thead>
    <tbody>
        <tr><td>Marshal/SimpleDocument/map-2</td><td>2.1x</td><td>3.1x</td></tr>
        <tr><td>Marshal/SimpleDocument/struct-2</td><td>3.4x</td><td>4.8x</td></tr>
        <tr><td>Unmarshal/SimpleDocument/map-2</td><td>10.1x</td><td>7.0x</td></tr>
        <tr><td>Unmarshal/SimpleDocument/struct-2</td><td>12.4x</td><td>8.0x</td></tr>
        <tr><td>UnmarshalDataset/example-2</td><td>8.2x</td><td>6.9x</td></tr>
        <tr><td>UnmarshalDataset/code-2</td><td>7.5x</td><td>8.3x</td></tr>
        <tr><td>UnmarshalDataset/twitter-2</td><td>9.0x</td><td>7.6x</td></tr>
        <tr><td>UnmarshalDataset/citm_catalog-2</td><td>5.0x</td><td>4.5x</td></tr>
        <tr><td>UnmarshalDataset/canada-2</td><td>6.4x</td><td>4.7x</td></tr>
        <tr><td>UnmarshalDataset/config-2</td><td>10.2x</td><td>6.1x</td></tr>
        <tr><td>geomean</td><td>5.8x</td><td>5.3x</td></tr>
     </tbody>
</table>
<p>This table can be generated with <code>./ci.sh benchmark -a -html</code>.</p>
</details>

## Tools

Go-toml provides three handy command line tools:

 * `tomljson`: Reads a TOML file and outputs its JSON representation.

	```
	$ go install github.com/pelletier/go-toml/v2/cmd/tomljson@latest
	$ tomljson --help
	```

 * `jsontoml`: Reads a JSON file and outputs a TOML representation.

	```
	$ go install github.com/pelletier/go-toml/v2/cmd/jsontoml@latest
	$ jsontoml --help
	```

 * `tomll`: Lints and reformats a TOML file.

	```
	$ go install github.com/pelletier/go-toml/v2/cmd/tomll@latest
	$ tomll --help
	```

### Docker image

Those tools are also available as a [Docker image][docker]. For example, to use
`tomljson`:

```
docker run -i ghcr.io/pelletier/go-toml:v2 tomljson < example.toml
```

Multiple versions are available on [ghcr.io][docker].

[docker]: https://github.com/pelletier/go-toml/pkgs/container/go-toml

## Versioning

Expect for parts explicitly marked otherwise, go-toml follows [Semantic
Versioning](https://semver.org). The supported version of
[TOML](https://github.com/toml-lang/toml) is indicated at the beginning of this
document. The last two major versions of Go are supported (see [Go Release
Policy](https://golang.org/doc/devel/release.html#policy)).

## License

The MIT License (MIT). Read [LICENSE](LICENSE).
