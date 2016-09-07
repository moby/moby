package registry

import (
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestEndpointParse(c *check.C) {
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
			c.Errorf("%q: %s", td.str, err)
		}
		if e == nil {
			c.Logf("something's fishy, endpoint for %q is nil", td.str)
			continue
		}
		if e.String() != td.expected {
			c.Errorf("expected %q, got %q", td.expected, e.String())
		}
	}
}

func (s *DockerSuite) TestEndpointParseInvalid(c *check.C) {
	testData := []string{
		"http://0.0.0.0:5000/v2/",
	}
	for _, td := range testData {
		e, err := newV1EndpointFromStr(td, nil, "", nil)
		if err == nil {
			c.Errorf("expected error parsing %q: parsed as %q", td, e)
		}
	}
}

// Ensure that a registry endpoint that responds with a 401 only is determined
// to be a valid v1 registry endpoint
func (s *DockerSuite) TestValidateEndpoint(c *check.C) {
	requireBasicAuthHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("WWW-Authenticate", `Basic realm="localhost"`)
		w.WriteHeader(http.StatusUnauthorized)
	})

	// Make a test server which should validate as a v1 server.
	testServer := httptest.NewServer(requireBasicAuthHandler)
	defer testServer.Close()

	testServerURL, err := url.Parse(testServer.URL)
	if err != nil {
		c.Fatal(err)
	}

	testEndpoint := V1Endpoint{
		URL:    testServerURL,
		client: HTTPClient(NewTransport(nil)),
	}

	if err = validateEndpoint(&testEndpoint); err != nil {
		c.Fatal(err)
	}

	if testEndpoint.URL.Scheme != "http" {
		c.Fatalf("expecting to validate endpoint as http, got url %s", testEndpoint.String())
	}
}
