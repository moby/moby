# File system notifications for Go

[![Coverage](http://gocover.io/_badge/github.com/go-fsnotify/fsnotify)](http://gocover.io/github.com/go-fsnotify/fsnotify) [![GoDoc](https://godoc.org/gopkg.in/fsnotify.v1?status.svg)](https://godoc.org/gopkg.in/fsnotify.v1)

Cross platform: Windows, Linux, BSD and OS X.

|Adapter   |OS        |Status    |
|----------|----------|----------|
|inotify   |Linux, Android\*|Supported|
|kqueue    |BSD, OS X, iOS\*|Supported|
|ReadDirectoryChangesW|Windows|Supported|
|FSEvents  |OS X          |[Planned](https://github.com/go-fsnotify/fsnotify/issues/11)|
|FEN       |Solaris 11    |[Planned](https://github.com/go-fsnotify/fsnotify/issues/12)|
|fanotify  |Linux 2.6.37+ | |
|Polling   |*All*         |[Maybe](https://github.com/go-fsnotify/fsnotify/issues/9)|
|          |Plan 9        | |

\* Android and iOS are untested.

Please see [the documentation](https://godoc.org/gopkg.in/fsnotify.v1) for usage. Consult the [Wiki](https://github.com/go-fsnotify/fsnotify/wiki) for the FAQ and further information.

## API stability

Two major versions of fsnotify exist. 

**[fsnotify.v1](https://gopkg.in/fsnotify.v1)** provides [a new API](https://godoc.org/gopkg.in/fsnotify.v1) based on [this design document](http://goo.gl/MrYxyA). You can import v1 with:

```go
import "gopkg.in/fsnotify.v1"
```

\* Refer to the package as fsnotify (without the .v1 suffix).

**[fsnotify.v0](https://gopkg.in/fsnotify.v0)** is API-compatible with [howeyc/fsnotify](https://godoc.org/github.com/howeyc/fsnotify). Bugfixes *may* be backported, but I recommend upgrading to v1.

```go
import "gopkg.in/fsnotify.v0"
```

Further API changes are [planned](https://github.com/go-fsnotify/fsnotify/milestones), but a new major revision will be tagged, so you can depend on the v1 API.

## Contributing

* Send questions to [golang-dev@googlegroups.com](mailto:golang-dev@googlegroups.com). 
* Request features and report bugs using the [GitHub Issue Tracker](https://github.com/go-fsnotify/fsnotify/issues).

A future version of Go will have [fsnotify in the standard library](https://code.google.com/p/go/issues/detail?id=4068), therefore fsnotify carries the same [LICENSE](https://github.com/go-fsnotify/fsnotify/blob/master/LICENSE) as Go. Contributors retain their copyright, so we need you to fill out a short form before we can accept your contribution: [Google Individual Contributor License Agreement](https://developers.google.com/open-source/cla/individual).

Please read [CONTRIBUTING](https://github.com/go-fsnotify/fsnotify/blob/master/CONTRIBUTING.md) before opening a pull request.

## Example

See [example_test.go](https://github.com/go-fsnotify/fsnotify/blob/master/example_test.go).
