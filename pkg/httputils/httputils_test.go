package httputils

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestDownload(c *check.C) {
	expected := "Hello, docker !"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, expected)
	}))
	defer ts.Close()
	response, err := Download(ts.URL)
	if err != nil {
		c.Fatal(err)
	}

	actual, err := ioutil.ReadAll(response.Body)
	response.Body.Close()

	if err != nil || string(actual) != expected {
		c.Fatalf("Expected the response %q, got err:%v, response:%v, actual:%s", expected, err, response, string(actual))
	}
}

func (s *DockerSuite) TestDownload400Errors(c *check.C) {
	expectedError := "Got HTTP status code >= 400: 403 Forbidden"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 403
		http.Error(w, "something failed (forbidden)", http.StatusForbidden)
	}))
	defer ts.Close()
	// Expected status code = 403
	if _, err := Download(ts.URL); err == nil || err.Error() != expectedError {
		c.Fatalf("Expected the the error %q, got %v", expectedError, err)
	}
}

func (s *DockerSuite) TestDownloadOtherErrors(c *check.C) {
	if _, err := Download("I'm not an url.."); err == nil || !strings.Contains(err.Error(), "unsupported protocol scheme") {
		c.Fatalf("Expected an error with 'unsupported protocol scheme', got %v", err)
	}
}

func (s *DockerSuite) TestNewHTTPRequestError(c *check.C) {
	errorMessage := "Some error message"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 403
		http.Error(w, errorMessage, http.StatusForbidden)
	}))
	defer ts.Close()
	httpResponse, err := http.Get(ts.URL)
	if err != nil {
		c.Fatal(err)
	}
	if err := NewHTTPRequestError(errorMessage, httpResponse); err.Error() != errorMessage {
		c.Fatalf("Expected err to be %q, got %v", errorMessage, err)
	}
}

func (s *DockerSuite) TestParseServerHeader(c *check.C) {
	inputs := map[string][]string{
		"bad header":           {"error"},
		"(bad header)":         {"error"},
		"(without/spaces)":     {"error"},
		"(header/with spaces)": {"error"},
		"foo/bar (baz)":        {"foo", "bar", "baz"},
		"foo/bar":              {"error"},
		"foo":                  {"error"},
		"foo/bar (baz space)":           {"foo", "bar", "baz space"},
		"  f  f  /  b  b  (  b  s  )  ": {"f  f", "b  b", "b  s"},
		"foo/bar (baz) ignore":          {"foo", "bar", "baz"},
		"foo/bar ()":                    {"error"},
		"foo/bar()":                     {"error"},
		"foo/bar(baz)":                  {"foo", "bar", "baz"},
		"foo/bar/zzz(baz)":              {"foo/bar", "zzz", "baz"},
		"foo/bar(baz/abc)":              {"foo", "bar", "baz/abc"},
		"foo/bar(baz (abc))":            {"foo", "bar", "baz (abc)"},
	}

	for header, values := range inputs {
		serverHeader, err := ParseServerHeader(header)
		if err != nil {
			if err != errInvalidHeader {
				c.Fatalf("Failed to parse %q, and got some unexpected error: %q", header, err)
			}
			if values[0] == "error" {
				continue
			}
			c.Fatalf("Header %q failed to parse when it shouldn't have", header)
		}
		if values[0] == "error" {
			c.Fatalf("Header %q parsed ok when it should have failed(%q).", header, serverHeader)
		}

		if serverHeader.App != values[0] {
			c.Fatalf("Expected serverHeader.App for %q to equal %q, got %q", header, values[0], serverHeader.App)
		}

		if serverHeader.Ver != values[1] {
			c.Fatalf("Expected serverHeader.Ver for %q to equal %q, got %q", header, values[1], serverHeader.Ver)
		}

		if serverHeader.OS != values[2] {
			c.Fatalf("Expected serverHeader.OS for %q to equal %q, got %q", header, values[2], serverHeader.OS)
		}

	}

}
