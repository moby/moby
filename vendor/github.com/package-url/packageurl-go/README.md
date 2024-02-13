# packageurl-go

[![build](https://github.com/package-url/packageurl-go/workflows/test/badge.svg)](https://github.com/package-url/packageurl-go/actions?query=workflow%3Atest) [![Coverage Status](https://coveralls.io/repos/github/package-url/packageurl-go/badge.svg)](https://coveralls.io/github/package-url/packageurl-go) [![PkgGoDev](https://pkg.go.dev/badge/github.com/package-url/packageurl-go)](https://pkg.go.dev/github.com/package-url/packageurl-go) [![Go Report Card](https://goreportcard.com/badge/github.com/package-url/packageurl-go)](https://goreportcard.com/report/github.com/package-url/packageurl-go)

Go implementation of the package url spec.


## Install
```
go get -u github.com/package-url/packageurl-go
```

## Versioning

The versions will follow the spec. So if the spec is released at ``1.0``. Then all versions in the ``1.x.y`` will follow the ``1.x`` spec.


## Usage

### Create from parts
```go
package main

import (
	"fmt"

	"github.com/package-url/packageurl-go"
)

func main() {
	instance := packageurl.NewPackageURL("test", "ok", "name", "version", nil, "")
	fmt.Printf("%s", instance.ToString())
}
```

### Parse from string
```go
package main

import (
	"fmt"

	"github.com/package-url/packageurl-go"
)

func main() {
	instance, err := packageurl.FromString("test:ok/name@version")
	if err != nil {
		panic(err)
	}
	fmt.Printf("%#v", instance)
}

```


## Test
Testing using the normal ``go test`` command. Using ``make test`` will pull the test fixtures shared between all package-url projects and then execute the tests.

```
$ make test
curl -L https://raw.githubusercontent.com/package-url/purl-test-suite/master/test-suite-data.json -o testdata/test-suite-data.json
  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                 Dload  Upload   Total   Spent    Left  Speed
100  7181  100  7181    0     0   1202      0  0:00:05  0:00:05 --:--:--  1611
go test -v -cover ./...
=== RUN   TestFromStringExamples
--- PASS: TestFromStringExamples (0.00s)
=== RUN   TestToStringExamples
--- PASS: TestToStringExamples (0.00s)
PASS
coverage: 94.7% of statements
ok      github.com/package-url/packageurl-go    0.002s
```
