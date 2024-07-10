# flock

[![Go Reference](https://pkg.go.dev/badge/github.com/gofrs/flock.svg)](https://pkg.go.dev/github.com/gofrs/flock)
[![License](https://img.shields.io/badge/license-BSD_3--Clause-brightgreen.svg?style=flat)](https://github.com/gofrs/flock/blob/master/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/gofrs/flock)](https://goreportcard.com/report/github.com/gofrs/flock)

`flock` implements a thread-safe file lock.

It also includes a non-blocking `TryLock()` function to allow locking without blocking execution.

## Installation

```bash
go get -u github.com/gofrs/flock
```

## Usage

```go
import "github.com/gofrs/flock"

fileLock := flock.New("/var/lock/go-lock.lock")

locked, err := fileLock.TryLock()

if err != nil {
	// handle locking error
}

if locked {
	// do work
	fileLock.Unlock()
}
```

For more detailed usage information take a look at the package API docs on
[GoDoc](https://pkg.go.dev/github.com/gofrs/flock).

## License

`flock` is released under the BSD 3-Clause License. See the [`LICENSE`](./LICENSE) file for more details.

## Project History

This project was originally `github.com/theckman/go-flock`, it was transferred to Gofrs by the original author [Tim Heckman ](https://github.com/theckman).
