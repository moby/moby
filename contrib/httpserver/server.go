package main

import (
	"log"
	"net/http"
)

func main() {
	fs := http.FileServer(http.Dir("/static"))
	http.Handle("/", fs)
	log.Panic(http.ListenAndServe(":80", nil)) // #nosec G114 -- Ignoring for test-code: G114: Use of net/http serve function that has no support for setting timeouts (gosec)
}
