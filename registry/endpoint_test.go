package registry // import "github.com/docker/docker/registry"

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/registry"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

func TestV1EndpointPing(t *testing.T) {
	skip.If(t, os.Getuid() != 0, "skipping test that requires root")
	testPing := func(index *registry.IndexInfo, expectedStandalone bool, assertMessage string) {
		ep, err := newV1Endpoint(index, "", nil)
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
	skip.If(t, os.Getuid() != 0, "skipping test that requires root")
	// Simple wrapper to fail test if err != nil
	expandEndpoint := func(index *registry.IndexInfo) *v1Endpoint {
		endpoint, err := newV1Endpoint(index, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		return endpoint
	}

	assertInsecureIndex := func(index *registry.IndexInfo) {
		index.Secure = true
		_, err := newV1Endpoint(index, "", nil)
		assert.ErrorContains(t, err, "insecure-registry", index.Name+": Expected insecure-registry  error for insecure index")
		index.Secure = false
	}

	assertSecureIndex := func(index *registry.IndexInfo) {
		index.Secure = true
		_, err := newV1Endpoint(index, "", nil)
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
		_, err := newV1Endpoint(index, "", nil)
		assert.Check(t, err != nil, "Expected error while expanding bad endpoint: %s", address)
	}
}

func TestV1EndpointParse(t *testing.T) {
	testData := []struct {
		str      string
		expected string
	}{
		{IndexServer, IndexServer},
		{"http://0.0.0.0:5000/v1/", "http://0.0.0.0:5000/v1/"},
		{"http://0.0.0.0:5000", "http://0.0.0.0:5000/v1/"},
		{"0.0.0.0:5000", "https://0.0.0.0:5000/v1/"},
		{"http://0.0.0.0:5000/nonversion/", "http://0.0.0.0:5000/nonversion/v1/"},
		{"http://0.0.0.0:5000/v0/", "http://0.0.0.0:5000/v0/v1/"},
	}
	for _, td := range testData {
		e, err := newV1EndpointFromStr(td.str, nil, "", nil)
		if err != nil {
			t.Errorf("%q: %s", td.str, err)
		}
		if e == nil {
			t.Logf("something's fishy, endpoint for %q is nil", td.str)
			continue
		}
		if e.String() != td.expected {
			t.Errorf("expected %q, got %q", td.expected, e.String())
		}
	}
}

func TestV1EndpointParseInvalid(t *testing.T) {
	testData := []string{
		"http://0.0.0.0:5000/v2/",
	}
	for _, td := range testData {
		e, err := newV1EndpointFromStr(td, nil, "", nil)
		if err == nil {
			t.Errorf("expected error parsing %q: parsed as %q", td, e)
		}
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
