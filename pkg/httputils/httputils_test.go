package httputils

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/docker/docker/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err)
	assert.Equal(t, expected, string(actual))
}

func TestDownload400Errors(t *testing.T) {
	expectedError := "Got HTTP status code >= 400: 403 Forbidden"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 403
		http.Error(w, "something failed (forbidden)", http.StatusForbidden)
	}))
	defer ts.Close()
	// Expected status code = 403
	_, err := Download(ts.URL)
	assert.EqualError(t, err, expectedError)
}

func TestDownloadOtherErrors(t *testing.T) {
	_, err := Download("I'm not an url..")
	testutil.ErrorContains(t, err, "unsupported protocol scheme")
}
