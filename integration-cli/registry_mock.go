package main

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"

	"github.com/go-check/check"
)

type handlerFunc func(w http.ResponseWriter, r *http.Request)

type testRegistry struct {
	server   *httptest.Server
	hostport string
	handlers map[string]handlerFunc
	mu       sync.Mutex
}

func (tr *testRegistry) registerHandler(path string, h handlerFunc) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.handlers[path] = h
}

func newTestRegistry(c *check.C) (*testRegistry, error) {
	testReg := &testRegistry{handlers: make(map[string]handlerFunc)}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		url := r.URL.String()

		var matched bool
		var err error
		for re, function := range testReg.handlers {
			matched, err = regexp.MatchString(re, url)
			if err != nil {
				c.Fatal("Error with handler regexp")
			}
			if matched {
				function(w, r)
				break
			}
		}

		if !matched {
			c.Fatalf("Unable to match %s with regexp", url)
		}
	}))

	testReg.server = ts
	testReg.hostport = strings.Replace(ts.URL, "http://", "", 1)
	return testReg, nil
}
