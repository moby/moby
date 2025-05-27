go-md2man
=========

Converts markdown into roff (man pages).

Uses [blackfriday](https://github.com/russross/blackfriday) to process markdown into man pages.

### Usage

```bash
go install github.com/cpuguy83/go-md2man/v2@latest

go-md2man -in /path/to/markdownfile.md -out /manfile/output/path
```

For go 1.24 and above, you can run it with `go tool`:

```bash
go get -tool github.com/cpuguy83/go-md2man/v2@latest
# it will be appended to `tool` directive in go.mod file

go tool go-md2man -in /path/to/markdownfile.md -out /manfile/output/path
```
