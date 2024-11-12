package registry // import "github.com/docker/docker/registry"

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/registry"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestV1EndpointPing(t *testing.T) {
	testPing := func(index *registry.IndexInfo, expectedStandalone bool, assertMessage string) {
		ep, err := newV1Endpoint(index, nil)
		if err != nil {
			t.Fatal(err)
		}
		regInfo, err := ep.ping()
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, regInfo.Standalone, expectedStandalone, assertMessage)
	}

	testPing(makeIndex("/v1/"), true, "Expected standalone to be true (default)")
	testPing(makeHTTPSIndex("/v1/"), true, "Expected standalone to be true (default)")
	testPing(makePublicIndex(), false, "Expected standalone to be false for public index")
}

func TestV1Endpoint(t *testing.T) {
	// Simple wrapper to fail test if err != nil
	expandEndpoint := func(index *registry.IndexInfo) *v1Endpoint {
		endpoint, err := newV1Endpoint(index, nil)
		if err != nil {
			t.Fatal(err)
		}
		return endpoint
	}

	assertInsecureIndex := func(index *registry.IndexInfo) {
		index.Secure = true
		_, err := newV1Endpoint(index, nil)
		assert.ErrorContains(t, err, "insecure-registry", index.Name+": Expected insecure-registry  error for insecure index")
		index.Secure = false
	}

	assertSecureIndex := func(index *registry.IndexInfo) {
		index.Secure = true
		_, err := newV1Endpoint(index, nil)
		assert.ErrorContains(t, err, "certificate signed by unknown authority", index.Name+": Expected cert error for secure index")
		index.Secure = false
	}

	index := &registry.IndexInfo{}
	index.Name = makeURL("/v1/")
	endpoint := expandEndpoint(index)
	assert.Equal(t, endpoint.String(), index.Name, "Expected endpoint to be "+index.Name)
	assertInsecureIndex(index)

	index.Name = makeURL("")
	endpoint = expandEndpoint(index)
	assert.Equal(t, endpoint.String(), index.Name+"/v1/", index.Name+": Expected endpoint to be "+index.Name+"/v1/")
	assertInsecureIndex(index)

	httpURL := makeURL("")
	index.Name = strings.SplitN(httpURL, "://", 2)[1]
	endpoint = expandEndpoint(index)
	assert.Equal(t, endpoint.String(), httpURL+"/v1/", index.Name+": Expected endpoint to be "+httpURL+"/v1/")
	assertInsecureIndex(index)

	index.Name = makeHTTPSURL("/v1/")
	endpoint = expandEndpoint(index)
	assert.Equal(t, endpoint.String(), index.Name, "Expected endpoint to be "+index.Name)
	assertSecureIndex(index)

	index.Name = makeHTTPSURL("")
	endpoint = expandEndpoint(index)
	assert.Equal(t, endpoint.String(), index.Name+"/v1/", index.Name+": Expected endpoint to be "+index.Name+"/v1/")
	assertSecureIndex(index)

	httpsURL := makeHTTPSURL("")
	index.Name = strings.SplitN(httpsURL, "://", 2)[1]
	endpoint = expandEndpoint(index)
	assert.Equal(t, endpoint.String(), httpsURL+"/v1/", index.Name+": Expected endpoint to be "+httpsURL+"/v1/")
	assertSecureIndex(index)

	badEndpoints := []string{
		"http://127.0.0.1/v1/",
		"https://127.0.0.1/v1/",
		"http://127.0.0.1",
		"https://127.0.0.1",
		"127.0.0.1",
	}
	for _, address := range badEndpoints {
		index.Name = address
		_, err := newV1Endpoint(index, nil)
		assert.Check(t, err != nil, "Expected error while expanding bad endpoint: %s", address)
	}
}

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
			expectedErr: "search is not supported on v2 endpoints: https://0.0.0.0:5000/v2/",
		},
	}
	for _, tc := range tests {
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
func TestV1EndpointValidate(t *testing.T) {
	requireBasicAuthHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("WWW-Authenticate", `Basic realm="localhost"`)
		w.WriteHeader(http.StatusUnauthorized)
	})

	// Make a test server which should validate as a v1 server.
	testServer := httptest.NewServer(requireBasicAuthHandler)
	defer testServer.Close()

	testEndpoint, err := newV1Endpoint(&registry.IndexInfo{Name: testServer.URL}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if testEndpoint.URL.Scheme != "http" {
		t.Fatalf("expecting to validate endpoint as http, got url %s", testEndpoint.String())
	}
}

func TestTrustedLocation(t *testing.T) {
	for _, u := range []string{"http://example.com", "https://example.com:7777", "http://docker.io", "http://test.docker.com", "https://fakedocker.com"} {
		req, _ := http.NewRequest(http.MethodGet, u, nil)
		assert.Check(t, !trustedLocation(req))
	}

	for _, u := range []string{"https://docker.io", "https://test.docker.com:80"} {
		req, _ := http.NewRequest(http.MethodGet, u, nil)
		assert.Check(t, trustedLocation(req))
	}
}

func TestAddRequiredHeadersToRedirectedRequests(t *testing.T) {
	for _, urls := range [][]string{
		{"http://docker.io", "https://docker.com"},
		{"https://foo.docker.io:7777", "http://bar.docker.com"},
		{"https://foo.docker.io", "https://example.com"},
	} {
		reqFrom, _ := http.NewRequest(http.MethodGet, urls[0], nil)
		reqFrom.Header.Add("Content-Type", "application/json")
		reqFrom.Header.Add("Authorization", "super_secret")
		reqTo, _ := http.NewRequest(http.MethodGet, urls[1], nil)

		_ = addRequiredHeadersToRedirectedRequests(reqTo, []*http.Request{reqFrom})

		if len(reqTo.Header) != 1 {
			t.Fatalf("Expected 1 headers, got %d", len(reqTo.Header))
		}

		if reqTo.Header.Get("Content-Type") != "application/json" {
			t.Fatal("'Content-Type' should be 'application/json'")
		}

		if reqTo.Header.Get("Authorization") != "" {
			t.Fatal("'Authorization' should be empty")
		}
	}

	for _, urls := range [][]string{
		{"https://docker.io", "https://docker.com"},
		{"https://foo.docker.io:7777", "https://bar.docker.com"},
	} {
		reqFrom, _ := http.NewRequest(http.MethodGet, urls[0], nil)
		reqFrom.Header.Add("Content-Type", "application/json")
		reqFrom.Header.Add("Authorization", "super_secret")
		reqTo, _ := http.NewRequest(http.MethodGet, urls[1], nil)

		_ = addRequiredHeadersToRedirectedRequests(reqTo, []*http.Request{reqFrom})

		if len(reqTo.Header) != 2 {
			t.Fatalf("Expected 2 headers, got %d", len(reqTo.Header))
		}

		if reqTo.Header.Get("Content-Type") != "application/json" {
			t.Fatal("'Content-Type' should be 'application/json'")
		}

		if reqTo.Header.Get("Authorization") != "super_secret" {
			t.Fatal("'Authorization' should be 'super_secret'")
		}
	}
}
