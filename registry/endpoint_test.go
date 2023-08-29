package registry // import "github.com/docker/docker/registry"

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestV1EndpointParse(t *testing.T) {
	tests := []struct {
		address     string
		expected    string
		expectedErr string
	}{
		{
			address:  IndexServer,
			expected: IndexServer,
		},
		{
			address:  "https://0.0.0.0:5000/v1/",
			expected: "https://0.0.0.0:5000/v1/",
		},
		{
			address:  "https://0.0.0.0:5000",
			expected: "https://0.0.0.0:5000/v1/",
		},
		{
			address:  "0.0.0.0:5000",
			expected: "https://0.0.0.0:5000/v1/",
		},
		{
			address:  "https://0.0.0.0:5000/nonversion/",
			expected: "https://0.0.0.0:5000/nonversion/v1/",
		},
		{
			address:  "https://0.0.0.0:5000/v0/",
			expected: "https://0.0.0.0:5000/v0/v1/",
		},
		{
			address:     "https://0.0.0.0:5000/v2/",
			expectedErr: "unsupported V1 version path v2",
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.address, func(t *testing.T) {
			ep, err := newV1EndpointFromStr(tc.address, nil, nil)
			if tc.expectedErr != "" {
				assert.Check(t, is.Error(err, tc.expectedErr))
				assert.Check(t, is.Nil(ep))
			} else {
				assert.NilError(t, err)
				assert.Check(t, is.Equal(ep.String(), tc.expected))
			}
		})
	}
}

// Ensure that a registry endpoint that responds with a 401 only is determined
// to be a valid v1 registry endpoint
func TestValidateEndpoint(t *testing.T) {
	requireBasicAuthHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("WWW-Authenticate", `Basic realm="localhost"`)
		w.WriteHeader(http.StatusUnauthorized)
	})

	// Make a test server which should validate as a v1 server.
	testServer := httptest.NewServer(requireBasicAuthHandler)
	defer testServer.Close()

	testServerURL, err := url.Parse(testServer.URL)
	if err != nil {
		t.Fatal(err)
	}

	testEndpoint := v1Endpoint{
		URL:    testServerURL,
		client: httpClient(newTransport(nil)),
	}

	if err = validateEndpoint(&testEndpoint); err != nil {
		t.Fatal(err)
	}

	if testEndpoint.URL.Scheme != "http" {
		t.Fatalf("expecting to validate endpoint as http, got url %s", testEndpoint.String())
	}
}
