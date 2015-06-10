go-gelf - GELF library and writer for Go
========================================

GELF is graylog2's UDP logging format.  This library provides an API
that applications can use to log messages directly to a graylog2
server, along with an `io.Writer` that can be use to redirect the
standard library's log messages (or `os.Stdout`), to a graylog2 server.

Installing
----------

go-gelf is go get-able:

	go get github.com/Graylog2/go-gelf/gelf

Usage
-----

The easiest way to integrate graylog logging into your go app is by
having your `main` function (or even `init`) call `log.SetOutput()`.
By using an `io.MultiWriter`, we can log to both stdout and graylog -
giving us both centralized and local logs.  (Redundancy is nice).

	package main

	import (
		"flag"
		"github.com/Graylog2/go-gelf/gelf"
		"io"
		"log"
		"os"
	)

	func main() {
		var graylogAddr string

		flag.StringVar(&graylogAddr, "graylog", "", "graylog server addr")
		flag.Parse()

		if graylogAddr != "" {
			gelfWriter, err := gelf.NewWriter(graylogAddr)
			if err != nil {
				log.Fatalf("gelf.NewWriter: %s", err)
			}
			// log to both stderr and graylog2
			log.SetOutput(io.MultiWriter(os.Stderr, gelfWriter))
			log.Printf("logging to stderr & graylog2@'%s'", graylogAddr)
		}

		// From here on out, any calls to log.Print* functions
		// will appear on stdout, and be sent over UDP to the
		// specified Graylog2 server.

		log.Printf("Hello gray World")

		// ...
	}

The above program can be invoked as:

	go run test.go -graylog=localhost:12201

Because GELF messages are sent over UDP, graylog server availability
doesn't impact application performance or response time.  There is a
small, fixed overhead per log call, regardless of whether the target
server is reachable or not.

To Do
-----

- WriteMessage example

License
-------

go-gelf is offered under the MIT license, see LICENSE for details.
