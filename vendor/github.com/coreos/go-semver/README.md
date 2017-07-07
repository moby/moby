# go-semver - Semantic Versioning Library

[![Build Status](https://travis-ci.org/coreos/go-semver.svg?branch=master)](https://travis-ci.org/coreos/go-semver)
[![GoDoc](https://godoc.org/github.com/coreos/go-semver/semver?status.svg)](https://godoc.org/github.com/coreos/go-semver/semver)

go-semver is a [semantic versioning][semver] library for Go. It lets you parse
and compare two semantic version strings.

[semver]: http://semver.org/

## Usage

```go
vA := semver.New("1.2.3")
vB := semver.New("3.2.1")

fmt.Printf("%s < %s == %t\n", vA, vB, vA.LessThan(*vB))
```

## Example Application

```
$ go run example.go 1.2.3 3.2.1
1.2.3 < 3.2.1 == true

$ go run example.go 5.2.3 3.2.1
5.2.3 < 3.2.1 == false
```
