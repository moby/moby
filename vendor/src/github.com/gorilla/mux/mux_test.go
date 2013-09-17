// Copyright 2012 The Gorilla Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mux

import (
	"fmt"
	"net/http"
	"testing"
)

type routeTest struct {
	title       string            // title of the test
	route       *Route            // the route being tested
	request     *http.Request     // a request to test the route
	vars        map[string]string // the expected vars of the match
	host        string            // the expected host of the match
	path        string            // the expected path of the match
	shouldMatch bool              // whether the request is expected to match the route at all
}

func TestHost(t *testing.T) {
	// newRequestHost a new request with a method, url, and host header
	newRequestHost := func(method, url, host string) *http.Request {
		req, err := http.NewRequest(method, url, nil)
		if err != nil {
			panic(err)
		}
		req.Host = host
		return req
	}

	tests := []routeTest{
		{
			title:       "Host route match",
			route:       new(Route).Host("aaa.bbb.ccc"),
			request:     newRequest("GET", "http://aaa.bbb.ccc/111/222/333"),
			vars:        map[string]string{},
			host:        "aaa.bbb.ccc",
			path:        "",
			shouldMatch: true,
		},
		{
			title:       "Host route, wrong host in request URL",
			route:       new(Route).Host("aaa.bbb.ccc"),
			request:     newRequest("GET", "http://aaa.222.ccc/111/222/333"),
			vars:        map[string]string{},
			host:        "aaa.bbb.ccc",
			path:        "",
			shouldMatch: false,
		},
		{
			title:       "Host route with port, match",
			route:       new(Route).Host("aaa.bbb.ccc:1234"),
			request:     newRequest("GET", "http://aaa.bbb.ccc:1234/111/222/333"),
			vars:        map[string]string{},
			host:        "aaa.bbb.ccc:1234",
			path:        "",
			shouldMatch: true,
		},
		{
			title:       "Host route with port, wrong port in request URL",
			route:       new(Route).Host("aaa.bbb.ccc:1234"),
			request:     newRequest("GET", "http://aaa.bbb.ccc:9999/111/222/333"),
			vars:        map[string]string{},
			host:        "aaa.bbb.ccc:1234",
			path:        "",
			shouldMatch: false,
		},
		{
			title:       "Host route, match with host in request header",
			route:       new(Route).Host("aaa.bbb.ccc"),
			request:     newRequestHost("GET", "/111/222/333", "aaa.bbb.ccc"),
			vars:        map[string]string{},
			host:        "aaa.bbb.ccc",
			path:        "",
			shouldMatch: true,
		},
		{
			title:       "Host route, wrong host in request header",
			route:       new(Route).Host("aaa.bbb.ccc"),
			request:     newRequestHost("GET", "/111/222/333", "aaa.222.ccc"),
			vars:        map[string]string{},
			host:        "aaa.bbb.ccc",
			path:        "",
			shouldMatch: false,
		},
		// BUG {new(Route).Host("aaa.bbb.ccc:1234"), newRequestHost("GET", "/111/222/333", "aaa.bbb.ccc:1234"), map[string]string{}, "aaa.bbb.ccc:1234", "", true},
		{
			title:       "Host route with port, wrong host in request header",
			route:       new(Route).Host("aaa.bbb.ccc:1234"),
			request:     newRequestHost("GET", "/111/222/333", "aaa.bbb.ccc:9999"),
			vars:        map[string]string{},
			host:        "aaa.bbb.ccc:1234",
			path:        "",
			shouldMatch: false,
		},
		{
			title:       "Host route with pattern, match",
			route:       new(Route).Host("aaa.{v1:[a-z]{3}}.ccc"),
			request:     newRequest("GET", "http://aaa.bbb.ccc/111/222/333"),
			vars:        map[string]string{"v1": "bbb"},
			host:        "aaa.bbb.ccc",
			path:        "",
			shouldMatch: true,
		},
		{
			title:       "Host route with pattern, wrong host in request URL",
			route:       new(Route).Host("aaa.{v1:[a-z]{3}}.ccc"),
			request:     newRequest("GET", "http://aaa.222.ccc/111/222/333"),
			vars:        map[string]string{"v1": "bbb"},
			host:        "aaa.bbb.ccc",
			path:        "",
			shouldMatch: false,
		},
		{
			title:       "Host route with multiple patterns, match",
			route:       new(Route).Host("{v1:[a-z]{3}}.{v2:[a-z]{3}}.{v3:[a-z]{3}}"),
			request:     newRequest("GET", "http://aaa.bbb.ccc/111/222/333"),
			vars:        map[string]string{"v1": "aaa", "v2": "bbb", "v3": "ccc"},
			host:        "aaa.bbb.ccc",
			path:        "",
			shouldMatch: true,
		},
		{
			title:       "Host route with multiple patterns, wrong host in request URL",
			route:       new(Route).Host("{v1:[a-z]{3}}.{v2:[a-z]{3}}.{v3:[a-z]{3}}"),
			request:     newRequest("GET", "http://aaa.222.ccc/111/222/333"),
			vars:        map[string]string{"v1": "aaa", "v2": "bbb", "v3": "ccc"},
			host:        "aaa.bbb.ccc",
			path:        "",
			shouldMatch: false,
		},
	}
	for _, test := range tests {
		testRoute(t, test)
	}
}

func TestPath(t *testing.T) {
	tests := []routeTest{
		{
			title:       "Path route, match",
			route:       new(Route).Path("/111/222/333"),
			request:     newRequest("GET", "http://localhost/111/222/333"),
			vars:        map[string]string{},
			host:        "",
			path:        "/111/222/333",
			shouldMatch: true,
		},
		{
			title:       "Path route, wrong path in request in request URL",
			route:       new(Route).Path("/111/222/333"),
			request:     newRequest("GET", "http://localhost/1/2/3"),
			vars:        map[string]string{},
			host:        "",
			path:        "/111/222/333",
			shouldMatch: false,
		},
		{
			title:       "Path route with pattern, match",
			route:       new(Route).Path("/111/{v1:[0-9]{3}}/333"),
			request:     newRequest("GET", "http://localhost/111/222/333"),
			vars:        map[string]string{"v1": "222"},
			host:        "",
			path:        "/111/222/333",
			shouldMatch: true,
		},
		{
			title:       "Path route with pattern, URL in request does not match",
			route:       new(Route).Path("/111/{v1:[0-9]{3}}/333"),
			request:     newRequest("GET", "http://localhost/111/aaa/333"),
			vars:        map[string]string{"v1": "222"},
			host:        "",
			path:        "/111/222/333",
			shouldMatch: false,
		},
		{
			title:       "Path route with multiple patterns, match",
			route:       new(Route).Path("/{v1:[0-9]{3}}/{v2:[0-9]{3}}/{v3:[0-9]{3}}"),
			request:     newRequest("GET", "http://localhost/111/222/333"),
			vars:        map[string]string{"v1": "111", "v2": "222", "v3": "333"},
			host:        "",
			path:        "/111/222/333",
			shouldMatch: true,
		},
		{
			title:       "Path route with multiple patterns, URL in request does not match",
			route:       new(Route).Path("/{v1:[0-9]{3}}/{v2:[0-9]{3}}/{v3:[0-9]{3}}"),
			request:     newRequest("GET", "http://localhost/111/aaa/333"),
			vars:        map[string]string{"v1": "111", "v2": "222", "v3": "333"},
			host:        "",
			path:        "/111/222/333",
			shouldMatch: false,
		},
	}

	for _, test := range tests {
		testRoute(t, test)
	}
}

func TestPathPrefix(t *testing.T) {
	tests := []routeTest{
		{
			title:       "PathPrefix route, match",
			route:       new(Route).PathPrefix("/111"),
			request:     newRequest("GET", "http://localhost/111/222/333"),
			vars:        map[string]string{},
			host:        "",
			path:        "/111",
			shouldMatch: true,
		},
		{
			title:       "PathPrefix route, URL prefix in request does not match",
			route:       new(Route).PathPrefix("/111"),
			request:     newRequest("GET", "http://localhost/1/2/3"),
			vars:        map[string]string{},
			host:        "",
			path:        "/111",
			shouldMatch: false,
		},
		{
			title:       "PathPrefix route with pattern, match",
			route:       new(Route).PathPrefix("/111/{v1:[0-9]{3}}"),
			request:     newRequest("GET", "http://localhost/111/222/333"),
			vars:        map[string]string{"v1": "222"},
			host:        "",
			path:        "/111/222",
			shouldMatch: true,
		},
		{
			title:       "PathPrefix route with pattern, URL prefix in request does not match",
			route:       new(Route).PathPrefix("/111/{v1:[0-9]{3}}"),
			request:     newRequest("GET", "http://localhost/111/aaa/333"),
			vars:        map[string]string{"v1": "222"},
			host:        "",
			path:        "/111/222",
			shouldMatch: false,
		},
		{
			title:       "PathPrefix route with multiple patterns, match",
			route:       new(Route).PathPrefix("/{v1:[0-9]{3}}/{v2:[0-9]{3}}"),
			request:     newRequest("GET", "http://localhost/111/222/333"),
			vars:        map[string]string{"v1": "111", "v2": "222"},
			host:        "",
			path:        "/111/222",
			shouldMatch: true,
		},
		{
			title:       "PathPrefix route with multiple patterns, URL prefix in request does not match",
			route:       new(Route).PathPrefix("/{v1:[0-9]{3}}/{v2:[0-9]{3}}"),
			request:     newRequest("GET", "http://localhost/111/aaa/333"),
			vars:        map[string]string{"v1": "111", "v2": "222"},
			host:        "",
			path:        "/111/222",
			shouldMatch: false,
		},
	}

	for _, test := range tests {
		testRoute(t, test)
	}
}

func TestHostPath(t *testing.T) {
	tests := []routeTest{
		{
			title:       "Host and Path route, match",
			route:       new(Route).Host("aaa.bbb.ccc").Path("/111/222/333"),
			request:     newRequest("GET", "http://aaa.bbb.ccc/111/222/333"),
			vars:        map[string]string{},
			host:        "",
			path:        "",
			shouldMatch: true,
		},
		{
			title:       "Host and Path route, wrong host in request URL",
			route:       new(Route).Host("aaa.bbb.ccc").Path("/111/222/333"),
			request:     newRequest("GET", "http://aaa.222.ccc/111/222/333"),
			vars:        map[string]string{},
			host:        "",
			path:        "",
			shouldMatch: false,
		},
		{
			title:       "Host and Path route with pattern, match",
			route:       new(Route).Host("aaa.{v1:[a-z]{3}}.ccc").Path("/111/{v2:[0-9]{3}}/333"),
			request:     newRequest("GET", "http://aaa.bbb.ccc/111/222/333"),
			vars:        map[string]string{"v1": "bbb", "v2": "222"},
			host:        "aaa.bbb.ccc",
			path:        "/111/222/333",
			shouldMatch: true,
		},
		{
			title:       "Host and Path route with pattern, URL in request does not match",
			route:       new(Route).Host("aaa.{v1:[a-z]{3}}.ccc").Path("/111/{v2:[0-9]{3}}/333"),
			request:     newRequest("GET", "http://aaa.222.ccc/111/222/333"),
			vars:        map[string]string{"v1": "bbb", "v2": "222"},
			host:        "aaa.bbb.ccc",
			path:        "/111/222/333",
			shouldMatch: false,
		},
		{
			title:       "Host and Path route with multiple patterns, match",
			route:       new(Route).Host("{v1:[a-z]{3}}.{v2:[a-z]{3}}.{v3:[a-z]{3}}").Path("/{v4:[0-9]{3}}/{v5:[0-9]{3}}/{v6:[0-9]{3}}"),
			request:     newRequest("GET", "http://aaa.bbb.ccc/111/222/333"),
			vars:        map[string]string{"v1": "aaa", "v2": "bbb", "v3": "ccc", "v4": "111", "v5": "222", "v6": "333"},
			host:        "aaa.bbb.ccc",
			path:        "/111/222/333",
			shouldMatch: true,
		},
		{
			title:       "Host and Path route with multiple patterns, URL in request does not match",
			route:       new(Route).Host("{v1:[a-z]{3}}.{v2:[a-z]{3}}.{v3:[a-z]{3}}").Path("/{v4:[0-9]{3}}/{v5:[0-9]{3}}/{v6:[0-9]{3}}"),
			request:     newRequest("GET", "http://aaa.222.ccc/111/222/333"),
			vars:        map[string]string{"v1": "aaa", "v2": "bbb", "v3": "ccc", "v4": "111", "v5": "222", "v6": "333"},
			host:        "aaa.bbb.ccc",
			path:        "/111/222/333",
			shouldMatch: false,
		},
	}

	for _, test := range tests {
		testRoute(t, test)
	}
}

func TestHeaders(t *testing.T) {
	// newRequestHeaders creates a new request with a method, url, and headers
	newRequestHeaders := func(method, url string, headers map[string]string) *http.Request {
		req, err := http.NewRequest(method, url, nil)
		if err != nil {
			panic(err)
		}
		for k, v := range headers {
			req.Header.Add(k, v)
		}
		return req
	}

	tests := []routeTest{
		{
			title:       "Headers route, match",
			route:       new(Route).Headers("foo", "bar", "baz", "ding"),
			request:     newRequestHeaders("GET", "http://localhost", map[string]string{"foo": "bar", "baz": "ding"}),
			vars:        map[string]string{},
			host:        "",
			path:        "",
			shouldMatch: true,
		},
		{
			title:       "Headers route, bad header values",
			route:       new(Route).Headers("foo", "bar", "baz", "ding"),
			request:     newRequestHeaders("GET", "http://localhost", map[string]string{"foo": "bar", "baz": "dong"}),
			vars:        map[string]string{},
			host:        "",
			path:        "",
			shouldMatch: false,
		},
	}

	for _, test := range tests {
		testRoute(t, test)
	}

}

func TestMethods(t *testing.T) {
	tests := []routeTest{
		{
			title:       "Methods route, match GET",
			route:       new(Route).Methods("GET", "POST"),
			request:     newRequest("GET", "http://localhost"),
			vars:        map[string]string{},
			host:        "",
			path:        "",
			shouldMatch: true,
		},
		{
			title:       "Methods route, match POST",
			route:       new(Route).Methods("GET", "POST"),
			request:     newRequest("POST", "http://localhost"),
			vars:        map[string]string{},
			host:        "",
			path:        "",
			shouldMatch: true,
		},
		{
			title:       "Methods route, bad method",
			route:       new(Route).Methods("GET", "POST"),
			request:     newRequest("PUT", "http://localhost"),
			vars:        map[string]string{},
			host:        "",
			path:        "",
			shouldMatch: false,
		},
	}

	for _, test := range tests {
		testRoute(t, test)
	}
}

func TestQueries(t *testing.T) {
	tests := []routeTest{
		{
			title:       "Queries route, match",
			route:       new(Route).Queries("foo", "bar", "baz", "ding"),
			request:     newRequest("GET", "http://localhost?foo=bar&baz=ding"),
			vars:        map[string]string{},
			host:        "",
			path:        "",
			shouldMatch: true,
		},
		{
			title:       "Queries route, bad query",
			route:       new(Route).Queries("foo", "bar", "baz", "ding"),
			request:     newRequest("GET", "http://localhost?foo=bar&baz=dong"),
			vars:        map[string]string{},
			host:        "",
			path:        "",
			shouldMatch: false,
		},
	}

	for _, test := range tests {
		testRoute(t, test)
	}
}

func TestSchemes(t *testing.T) {
	tests := []routeTest{
		// Schemes
		{
			title:       "Schemes route, match https",
			route:       new(Route).Schemes("https", "ftp"),
			request:     newRequest("GET", "https://localhost"),
			vars:        map[string]string{},
			host:        "",
			path:        "",
			shouldMatch: true,
		},
		{
			title:       "Schemes route, match ftp",
			route:       new(Route).Schemes("https", "ftp"),
			request:     newRequest("GET", "ftp://localhost"),
			vars:        map[string]string{},
			host:        "",
			path:        "",
			shouldMatch: true,
		},
		{
			title:       "Schemes route, bad scheme",
			route:       new(Route).Schemes("https", "ftp"),
			request:     newRequest("GET", "http://localhost"),
			vars:        map[string]string{},
			host:        "",
			path:        "",
			shouldMatch: false,
		},
	}
	for _, test := range tests {
		testRoute(t, test)
	}
}

func TestMatcherFunc(t *testing.T) {
	m := func(r *http.Request, m *RouteMatch) bool {
		if r.URL.Host == "aaa.bbb.ccc" {
			return true
		}
		return false
	}

	tests := []routeTest{
		{
			title:       "MatchFunc route, match",
			route:       new(Route).MatcherFunc(m),
			request:     newRequest("GET", "http://aaa.bbb.ccc"),
			vars:        map[string]string{},
			host:        "",
			path:        "",
			shouldMatch: true,
		},
		{
			title:       "MatchFunc route, non-match",
			route:       new(Route).MatcherFunc(m),
			request:     newRequest("GET", "http://aaa.222.ccc"),
			vars:        map[string]string{},
			host:        "",
			path:        "",
			shouldMatch: false,
		},
	}

	for _, test := range tests {
		testRoute(t, test)
	}
}

func TestSubRouter(t *testing.T) {
	subrouter1 := new(Route).Host("{v1:[a-z]+}.google.com").Subrouter()
	subrouter2 := new(Route).PathPrefix("/foo/{v1}").Subrouter()

	tests := []routeTest{
		{
			route:       subrouter1.Path("/{v2:[a-z]+}"),
			request:     newRequest("GET", "http://aaa.google.com/bbb"),
			vars:        map[string]string{"v1": "aaa", "v2": "bbb"},
			host:        "aaa.google.com",
			path:        "/bbb",
			shouldMatch: true,
		},
		{
			route:       subrouter1.Path("/{v2:[a-z]+}"),
			request:     newRequest("GET", "http://111.google.com/111"),
			vars:        map[string]string{"v1": "aaa", "v2": "bbb"},
			host:        "aaa.google.com",
			path:        "/bbb",
			shouldMatch: false,
		},
		{
			route:       subrouter2.Path("/baz/{v2}"),
			request:     newRequest("GET", "http://localhost/foo/bar/baz/ding"),
			vars:        map[string]string{"v1": "bar", "v2": "ding"},
			host:        "",
			path:        "/foo/bar/baz/ding",
			shouldMatch: true,
		},
		{
			route:       subrouter2.Path("/baz/{v2}"),
			request:     newRequest("GET", "http://localhost/foo/bar"),
			vars:        map[string]string{"v1": "bar", "v2": "ding"},
			host:        "",
			path:        "/foo/bar/baz/ding",
			shouldMatch: false,
		},
	}

	for _, test := range tests {
		testRoute(t, test)
	}
}

func TestNamedRoutes(t *testing.T) {
	r1 := NewRouter()
	r1.NewRoute().Name("a")
	r1.NewRoute().Name("b")
	r1.NewRoute().Name("c")

	r2 := r1.NewRoute().Subrouter()
	r2.NewRoute().Name("d")
	r2.NewRoute().Name("e")
	r2.NewRoute().Name("f")

	r3 := r2.NewRoute().Subrouter()
	r3.NewRoute().Name("g")
	r3.NewRoute().Name("h")
	r3.NewRoute().Name("i")

	if r1.namedRoutes == nil || len(r1.namedRoutes) != 9 {
		t.Errorf("Expected 9 named routes, got %v", r1.namedRoutes)
	} else if r1.Get("i") == nil {
		t.Errorf("Subroute name not registered")
	}
}

func TestStrictSlash(t *testing.T) {
	var r *Router
	var req *http.Request
	var route *Route
	var match *RouteMatch
	var matched bool

	// StrictSlash should be ignored for path prefix.
	// So we register a route ending in slash but it doesn't attempt to add
	// the slash for a path not ending in slash.
	r = NewRouter()
	r.StrictSlash(true)
	route = r.NewRoute().PathPrefix("/static/")
	req, _ = http.NewRequest("GET", "http://localhost/static/logo.png", nil)
	match = new(RouteMatch)
	matched = r.Match(req, match)
	if !matched {
		t.Errorf("Should match request %q -- %v", req.URL.Path, getRouteTemplate(route))
	}
	if match.Handler != nil {
		t.Errorf("Should not redirect")
	}
}

// ----------------------------------------------------------------------------
// Helpers
// ----------------------------------------------------------------------------

func getRouteTemplate(route *Route) string {
	host, path := "none", "none"
	if route.regexp != nil {
		if route.regexp.host != nil {
			host = route.regexp.host.template
		}
		if route.regexp.path != nil {
			path = route.regexp.path.template
		}
	}
	return fmt.Sprintf("Host: %v, Path: %v", host, path)
}

func testRoute(t *testing.T, test routeTest) {
	request := test.request
	route := test.route
	vars := test.vars
	shouldMatch := test.shouldMatch
	host := test.host
	path := test.path
	url := test.host + test.path

	var match RouteMatch
	ok := route.Match(request, &match)
	if ok != shouldMatch {
		msg := "Should match"
		if !shouldMatch {
			msg = "Should not match"
		}
		t.Errorf("(%v) %v:\nRoute: %#v\nRequest: %#v\nVars: %v\n", test.title, msg, route, request, vars)
		return
	}
	if shouldMatch {
		if test.vars != nil && !stringMapEqual(test.vars, match.Vars) {
			t.Errorf("(%v) Vars not equal: expected %v, got %v", test.title, vars, match.Vars)
			return
		}
		if host != "" {
			u, _ := test.route.URLHost(mapToPairs(match.Vars)...)
			if host != u.Host {
				t.Errorf("(%v) URLHost not equal: expected %v, got %v -- %v", test.title, host, u.Host, getRouteTemplate(route))
				return
			}
		}
		if path != "" {
			u, _ := route.URLPath(mapToPairs(match.Vars)...)
			if path != u.Path {
				t.Errorf("(%v) URLPath not equal: expected %v, got %v -- %v", test.title, path, u.Path, getRouteTemplate(route))
				return
			}
		}
		if url != "" {
			u, _ := route.URL(mapToPairs(match.Vars)...)
			if url != u.Host+u.Path {
				t.Errorf("(%v) URL not equal: expected %v, got %v -- %v", test.title, url, u.Host+u.Path, getRouteTemplate(route))
				return
			}
		}
	}
}

// https://plus.google.com/101022900381697718949/posts/eWy6DjFJ6uW
func TestSubrouterHeader(t *testing.T) {
	expected := "func1 response"
	func1 := func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, expected)
	}
	func2 := func(http.ResponseWriter, *http.Request) {}

	r := NewRouter()
	s := r.Headers("SomeSpecialHeader", "").Subrouter()
	s.HandleFunc("/", func1).Name("func1")
	r.HandleFunc("/", func2).Name("func2")

	req, _ := http.NewRequest("GET", "http://localhost/", nil)
	req.Header.Add("SomeSpecialHeader", "foo")
	match := new(RouteMatch)
	matched := r.Match(req, match)
	if !matched {
		t.Errorf("Should match request")
	}
	if match.Route.GetName() != "func1" {
		t.Errorf("Expecting func1 handler, got %s", match.Route.GetName())
	}
	resp := NewRecorder()
	match.Handler.ServeHTTP(resp, req)
	if resp.Body.String() != expected {
		t.Errorf("Expecting %q", expected)
	}
}

// mapToPairs converts a string map to a slice of string pairs
func mapToPairs(m map[string]string) []string {
	var i int
	p := make([]string, len(m)*2)
	for k, v := range m {
		p[i] = k
		p[i+1] = v
		i += 2
	}
	return p
}

// stringMapEqual checks the equality of two string maps
func stringMapEqual(m1, m2 map[string]string) bool {
	nil1 := m1 == nil
	nil2 := m2 == nil
	if nil1 != nil2 || len(m1) != len(m2) {
		return false
	}
	for k, v := range m1 {
		if v != m2[k] {
			return false
		}
	}
	return true
}

// newRequest is a helper function to create a new request with a method and url
func newRequest(method, url string) *http.Request {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		panic(err)
	}
	return req
}
