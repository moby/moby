circbuf
=======

> *About this fork:* This is a fork from https://github.com/armon/circbuf including the following performance optimizations:
>
> * Add methods to efficiently read and write a single byte at a time.
> * Avoid allocating memory on `Bytes()` (at the expense of having a second, pre-allocated buffer)
> * Add a faster code path for buffers whose size is a power-of-two.

This repository provides the `circbuf` package. This provides a `Buffer` object
which is a circular (or ring) buffer. It has a fixed size, but can be written
to infinitely. Only the last `size` bytes are ever retained. The buffer implements
the `io.Writer` interface.

Documentation
=============

Full documentation can be found on [Godoc](http://godoc.org/github.com/armon/circbuf)

Usage
=====

The `circbuf` package is very easy to use:

```go
buf, _ := NewBuffer(6)
buf.Write([]byte("hello world"))

if string(buf.Bytes()) != " world" {
    panic("should only have last 6 bytes!")
}

```

