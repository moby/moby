package httputils

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDownload(t *testing.T) {
	expected := "Hello, docker !"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, expected)
	}))
	defer ts.Close()
	response, err := Download(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	actual, err := ioutil.ReadAll(response.Body)
	response.Body.Close()

	if err != nil || string(actual) != expected {
		t.Fatalf("Expected the response %q, got err:%q, actual:%q", expected, err, string(actual))
	}
}

func TestDownload400Errors(t *testing.T) {
	expectedError := "Got HTTP status code >= 400: 403 Forbidden"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 403
		http.Error(w, "something failed (forbidden)", http.StatusForbidden)
	}))
	defer ts.Close()
	// Expected status code = 403
	if _, err := Download(ts.URL); err == nil || err.Error() != expectedError {
		t.Fatalf("Expected the error %q, got %q", expectedError, err)
	}
}

func TestDownloadOtherErrors(t *testing.T) {
	if _, err := Download("I'm not an url.."); err == nil || !strings.Contains(err.Error(), "unsupported protocol scheme") {
		t.Fatalf("Expected an error with 'unsupported protocol scheme', got %q", err)
	}
}

func TestNewHTTPRequestError(t *testing.T) {
	errorMessage := "Some error message"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 403
		http.Error(w, errorMessage, http.StatusForbidden)
	}))
	defer ts.Close()
	httpResponse, err := http.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	if err := NewHTTPRequestError(errorMessage, httpResponse); err.Error() != errorMessage {
		t.Fatalf("Expected err to be %q, got %q", errorMessage, err)
	}
}

func TestParseServerHeader(t *testing.T) {
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
				t.Fatalf("Failed to parse %q, and got some unexpected error: %q", header, err)
			}
			if values[0] == "error" {
				continue
			}
			t.Fatalf("Header %q failed to parse when it shouldn't have", header)
		}
		if values[0] == "error" {
			t.Fatalf("Header %q parsed ok when it should have failed(%q).", header, serverHeader)
		}

		if serverHeader.App != values[0] {
			t.Fatalf("Expected serverHeader.App for %q to equal %q, got %q", header, values[0], serverHeader.App)
		}

		if serverHeader.Ver != values[1] {
			t.Fatalf("Expected serverHeader.Ver for %q to equal %q, got %q", header, values[1], serverHeader.Ver)
		}

		if serverHeader.OS != values[2] {
			t.Fatalf("Expected serverHeader.OS for %q to equal %q, got %q", header, values[2], serverHeader.OS)
		}

	}

}
