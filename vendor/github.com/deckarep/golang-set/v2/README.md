![example workflow](https://github.com/deckarep/golang-set/actions/workflows/ci.yml/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/deckarep/golang-set/v2)](https://goreportcard.com/report/github.com/deckarep/golang-set/v2)
[![GoDoc](https://godoc.org/github.com/deckarep/golang-set/v2?status.svg)](http://godoc.org/github.com/deckarep/golang-set/v2)

# golang-set

The missing `generic` set collection for the Go language.  Until Go has sets built-in...use this.

## Update 3/5/2023
* Packaged version: `2.2.0` release includes a refactor to minimize pointer indirection, better method documentation standards and a few constructor convenience methods to increase ergonomics when appending items `Append` or creating a new set from an exist `Map`.
* supports `new generic` syntax
* Go `1.18.0` or higher
* Workflow tested on Go `1.20`

![With Generics](new_improved.jpeg)

Coming from Python one of the things I miss is the superbly wonderful set collection.  This is my attempt to mimic the primary features of the set collection from Python.
You can of course argue that there is no need for a set in Go, otherwise the creators would have added one to the standard library.  To those I say simply ignore this repository and carry-on and to the rest that find this useful please contribute in helping me make it better by contributing with suggestions or PRs.

## Features

* *NEW* [Generics](https://go.dev/doc/tutorial/generics) based implementation (requires [Go 1.18](https://go.dev/blog/go1.18beta1) or higher)
* One common *interface* to both implementations
  * a **non threadsafe** implementation favoring *performance*
  * a **threadsafe** implementation favoring *concurrent* use
* Feature complete set implementation modeled after [Python's set implementation](https://docs.python.org/3/library/stdtypes.html#set).
* Exhaustive unit-test and benchmark suite

## Trusted by

This package is trusted by many companies and thousands of open-source packages. Here are just a few sample users of this package.

* Notable projects/companies using this package
  * Ethereum
  * Docker
  * 1Password
  * Hashicorp

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=deckarep/golang-set&type=Date)](https://star-history.com/#deckarep/golang-set&Date)


## Usage

The code below demonstrates how a Set collection can better manage data and actually minimize boilerplate and needless loops in code. This package now fully supports *generic* syntax so you are now able to instantiate a collection for any [comparable](https://flaviocopes.com/golang-comparing-values/) type object.

What is considered comparable in Go? 
* `Booleans`, `integers`, `strings`, `floats` or basically primitive types.
* `Pointers`
* `Arrays`
* `Structs` if *all of their fields* are also comparable independently

Using this library is as simple as creating either a threadsafe or non-threadsafe set and providing a `comparable` type for instantiation of the collection.

```go
// Syntax example, doesn't compile.
mySet := mapset.NewSet[T]() // where T is some concrete comparable type.

// Therefore this code creates an int set
mySet := mapset.NewSet[int]()

// Or perhaps you want a string set
mySet := mapset.NewSet[string]()

type myStruct {
  name string
  age uint8
}

// Alternatively a set of structs
mySet := mapset.NewSet[myStruct]()

// Lastly a set that can hold anything using the any or empty interface keyword: interface{}. This is effectively removes type safety.
mySet := mapset.NewSet[any]()
```

## Comprehensive Example

```go
package main

import (
  "fmt"
  mapset "github.com/deckarep/golang-set/v2"
)

func main() {
  // Create a string-based set of required classes.
  required := mapset.NewSet[string]()
  required.Add("cooking")
  required.Add("english")
  required.Add("math")
  required.Add("biology")

  // Create a string-based set of science classes.
  sciences := mapset.NewSet[string]()
  sciences.Add("biology")
  sciences.Add("chemistry")
  
  // Create a string-based set of electives.
  electives := mapset.NewSet[string]()
  electives.Add("welding")
  electives.Add("music")
  electives.Add("automotive")

  // Create a string-based set of bonus programming classes.
  bonus := mapset.NewSet[string]()
  bonus.Add("beginner go")
  bonus.Add("python for dummies")
}
```

Create a set of all unique classes.
Sets will *automatically* deduplicate the same data.

```go
  all := required
    .Union(sciences)
    .Union(electives)
    .Union(bonus)
  
  fmt.Println(all)
```

Output:
```sh
Set{cooking, english, math, chemistry, welding, biology, music, automotive, beginner go, python for dummies}
```

Is cooking considered a science class?
```go
result := sciences.Contains("cooking")
fmt.Println(result)
```

Output:
```false
false
```

Show me all classes that are not science classes, since I don't enjoy science.
```go
notScience := all.Difference(sciences)
fmt.Println(notScience)
```

```sh
Set{ music, automotive, beginner go, python for dummies, cooking, english, math, welding }
```

Which science classes are also required classes?
```go
reqScience := sciences.Intersect(required)
```

Output:
```sh
Set{biology}
```

How many bonus classes do you offer?
```go
fmt.Println(bonus.Cardinality())
```
Output:
```sh
2
```

Thanks for visiting!

-deckarep
