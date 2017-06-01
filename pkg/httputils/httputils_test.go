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
