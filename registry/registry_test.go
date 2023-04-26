package registry // import "github.com/docker/docker/registry"

import (
	"net/http"
	"net/http/httputil"
	"os"
	"strings"
	"testing"

	"github.com/docker/distribution/reference"
	"github.com/docker/distribution/registry/client/transport"
	"github.com/docker/docker/api/types/registry"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func spawnTestRegistrySession(t *testing.T) *session {
	authConfig := &registry.AuthConfig{}
	endpoint, err := newV1Endpoint(makeIndex("/v1/"), nil)
	if err != nil {
		t.Fatal(err)
	}
	userAgent := "docker test client"
	var tr http.RoundTripper = debugTransport{NewTransport(nil), t.Log}
	tr = transport.NewTransport(newAuthTransport(tr, authConfig, false), Headers(userAgent, nil)...)
	client := httpClient(tr)

	if err := authorizeClient(client, authConfig, endpoint); err != nil {
		t.Fatal(err)
	}
	r := newSession(client, endpoint)

	// In a normal scenario for the v1 registry, the client should send a `X-Docker-Token: true`
	// header while authenticating, in order to retrieve a token that can be later used to
	// perform authenticated actions.
	//
	// The mock v1 registry does not support that, (TODO(tiborvass): support it), instead,
	// it will consider authenticated any request with the header `X-Docker-Token: fake-token`.
	//
	// Because we know that the client's transport is an `*authTransport` we simply cast it,
	// in order to set the internal cached token to the fake token, and thus send that fake token
	// upon every subsequent requests.
	r.client.Transport.(*authTransport).token = []string{"fake-token"}
	return r
}

func TestPingRegistryEndpoint(t *testing.T) {
	skip.If(t, os.Getuid() != 0, "skipping test that requires root")
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

func TestEndpoint(t *testing.T) {
	skip.If(t, os.Getuid() != 0, "skipping test that requires root")
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

func TestParseRepositoryInfo(t *testing.T) {
	type staticRepositoryInfo struct {
		Index         *registry.IndexInfo
		RemoteName    string
		CanonicalName string
		LocalName     string
		Official      bool
	}

	expectedRepoInfos := map[string]staticRepositoryInfo{
		"fooo/bar": {
			Index: &registry.IndexInfo{
				Name:     IndexName,
				Official: true,
			},
			RemoteName:    "fooo/bar",
			LocalName:     "fooo/bar",
			CanonicalName: "docker.io/fooo/bar",
			Official:      false,
		},
		"library/ubuntu": {
			Index: &registry.IndexInfo{
				Name:     IndexName,
				Official: true,
			},
			RemoteName:    "library/ubuntu",
			LocalName:     "ubuntu",
			CanonicalName: "docker.io/library/ubuntu",
			Official:      true,
		},
		"nonlibrary/ubuntu": {
			Index: &registry.IndexInfo{
				Name:     IndexName,
				Official: true,
			},
			RemoteName:    "nonlibrary/ubuntu",
			LocalName:     "nonlibrary/ubuntu",
			CanonicalName: "docker.io/nonlibrary/ubuntu",
			Official:      false,
		},
		"ubuntu": {
			Index: &registry.IndexInfo{
				Name:     IndexName,
				Official: true,
			},
			RemoteName:    "library/ubuntu",
			LocalName:     "ubuntu",
			CanonicalName: "docker.io/library/ubuntu",
			Official:      true,
		},
		"other/library": {
			Index: &registry.IndexInfo{
				Name:     IndexName,
				Official: true,
			},
			RemoteName:    "other/library",
			LocalName:     "other/library",
			CanonicalName: "docker.io/other/library",
			Official:      false,
		},
		"127.0.0.1:8000/private/moonbase": {
			Index: &registry.IndexInfo{
				Name:     "127.0.0.1:8000",
				Official: false,
			},
			RemoteName:    "private/moonbase",
			LocalName:     "127.0.0.1:8000/private/moonbase",
			CanonicalName: "127.0.0.1:8000/private/moonbase",
			Official:      false,
		},
		"127.0.0.1:8000/privatebase": {
			Index: &registry.IndexInfo{
				Name:     "127.0.0.1:8000",
				Official: false,
			},
			RemoteName:    "privatebase",
			LocalName:     "127.0.0.1:8000/privatebase",
			CanonicalName: "127.0.0.1:8000/privatebase",
			Official:      false,
		},
		"localhost:8000/private/moonbase": {
			Index: &registry.IndexInfo{
				Name:     "localhost:8000",
				Official: false,
			},
			RemoteName:    "private/moonbase",
			LocalName:     "localhost:8000/private/moonbase",
			CanonicalName: "localhost:8000/private/moonbase",
			Official:      false,
		},
		"localhost:8000/privatebase": {
			Index: &registry.IndexInfo{
				Name:     "localhost:8000",
				Official: false,
			},
			RemoteName:    "privatebase",
			LocalName:     "localhost:8000/privatebase",
			CanonicalName: "localhost:8000/privatebase",
			Official:      false,
		},
		"example.com/private/moonbase": {
			Index: &registry.IndexInfo{
				Name:     "example.com",
				Official: false,
			},
			RemoteName:    "private/moonbase",
			LocalName:     "example.com/private/moonbase",
			CanonicalName: "example.com/private/moonbase",
			Official:      false,
		},
		"example.com/privatebase": {
			Index: &registry.IndexInfo{
				Name:     "example.com",
				Official: false,
			},
			RemoteName:    "privatebase",
			LocalName:     "example.com/privatebase",
			CanonicalName: "example.com/privatebase",
			Official:      false,
		},
		"example.com:8000/private/moonbase": {
			Index: &registry.IndexInfo{
				Name:     "example.com:8000",
				Official: false,
			},
			RemoteName:    "private/moonbase",
			LocalName:     "example.com:8000/private/moonbase",
			CanonicalName: "example.com:8000/private/moonbase",
			Official:      false,
		},
		"example.com:8000/privatebase": {
			Index: &registry.IndexInfo{
				Name:     "example.com:8000",
				Official: false,
			},
			RemoteName:    "privatebase",
			LocalName:     "example.com:8000/privatebase",
			CanonicalName: "example.com:8000/privatebase",
			Official:      false,
		},
		"localhost/private/moonbase": {
			Index: &registry.IndexInfo{
				Name:     "localhost",
				Official: false,
			},
			RemoteName:    "private/moonbase",
			LocalName:     "localhost/private/moonbase",
			CanonicalName: "localhost/private/moonbase",
			Official:      false,
		},
		"localhost/privatebase": {
			Index: &registry.IndexInfo{
				Name:     "localhost",
				Official: false,
			},
			RemoteName:    "privatebase",
			LocalName:     "localhost/privatebase",
			CanonicalName: "localhost/privatebase",
			Official:      false,
		},
		IndexName + "/public/moonbase": {
			Index: &registry.IndexInfo{
				Name:     IndexName,
				Official: true,
			},
			RemoteName:    "public/moonbase",
			LocalName:     "public/moonbase",
			CanonicalName: "docker.io/public/moonbase",
			Official:      false,
		},
		"index." + IndexName + "/public/moonbase": {
			Index: &registry.IndexInfo{
				Name:     IndexName,
				Official: true,
			},
			RemoteName:    "public/moonbase",
			LocalName:     "public/moonbase",
			CanonicalName: "docker.io/public/moonbase",
			Official:      false,
		},
		"ubuntu-12.04-base": {
			Index: &registry.IndexInfo{
				Name:     IndexName,
				Official: true,
			},
			RemoteName:    "library/ubuntu-12.04-base",
			LocalName:     "ubuntu-12.04-base",
			CanonicalName: "docker.io/library/ubuntu-12.04-base",
			Official:      true,
		},
		IndexName + "/ubuntu-12.04-base": {
			Index: &registry.IndexInfo{
				Name:     IndexName,
				Official: true,
			},
			RemoteName:    "library/ubuntu-12.04-base",
			LocalName:     "ubuntu-12.04-base",
			CanonicalName: "docker.io/library/ubuntu-12.04-base",
			Official:      true,
		},
		"index." + IndexName + "/ubuntu-12.04-base": {
			Index: &registry.IndexInfo{
				Name:     IndexName,
				Official: true,
			},
			RemoteName:    "library/ubuntu-12.04-base",
			LocalName:     "ubuntu-12.04-base",
			CanonicalName: "docker.io/library/ubuntu-12.04-base",
			Official:      true,
		},
	}

	for reposName, expectedRepoInfo := range expectedRepoInfos {
		named, err := reference.ParseNormalizedNamed(reposName)
		if err != nil {
			t.Error(err)
		}

		repoInfo, err := ParseRepositoryInfo(named)
		if err != nil {
			t.Error(err)
		} else {
			assert.Check(t, is.Equal(repoInfo.Index.Name, expectedRepoInfo.Index.Name), reposName)
			assert.Check(t, is.Equal(reference.Path(repoInfo.Name), expectedRepoInfo.RemoteName), reposName)
			assert.Check(t, is.Equal(reference.FamiliarName(repoInfo.Name), expectedRepoInfo.LocalName), reposName)
			assert.Check(t, is.Equal(repoInfo.Name.Name(), expectedRepoInfo.CanonicalName), reposName)
			assert.Check(t, is.Equal(repoInfo.Index.Official, expectedRepoInfo.Index.Official), reposName)
			assert.Check(t, is.Equal(repoInfo.Official, expectedRepoInfo.Official), reposName)
		}
	}
}

func TestNewIndexInfo(t *testing.T) {
	testIndexInfo := func(config *serviceConfig, expectedIndexInfos map[string]*registry.IndexInfo) {
		for indexName, expectedIndexInfo := range expectedIndexInfos {
			index, err := newIndexInfo(config, indexName)
			if err != nil {
				t.Fatal(err)
			} else {
				assert.Check(t, is.Equal(index.Name, expectedIndexInfo.Name), indexName+" name")
				assert.Check(t, is.Equal(index.Official, expectedIndexInfo.Official), indexName+" is official")
				assert.Check(t, is.Equal(index.Secure, expectedIndexInfo.Secure), indexName+" is secure")
				assert.Check(t, is.Equal(len(index.Mirrors), len(expectedIndexInfo.Mirrors)), indexName+" mirrors")
			}
		}
	}

	config := emptyServiceConfig
	var noMirrors []string
	expectedIndexInfos := map[string]*registry.IndexInfo{
		IndexName: {
			Name:     IndexName,
			Official: true,
			Secure:   true,
			Mirrors:  noMirrors,
		},
		"index." + IndexName: {
			Name:     IndexName,
			Official: true,
			Secure:   true,
			Mirrors:  noMirrors,
		},
		"example.com": {
			Name:     "example.com",
			Official: false,
			Secure:   true,
			Mirrors:  noMirrors,
		},
		"127.0.0.1:5000": {
			Name:     "127.0.0.1:5000",
			Official: false,
			Secure:   false,
			Mirrors:  noMirrors,
		},
	}
	testIndexInfo(config, expectedIndexInfos)

	publicMirrors := []string{"http://mirror1.local", "http://mirror2.local"}
	var err error
	config, err = makeServiceConfig(publicMirrors, []string{"example.com"})
	if err != nil {
		t.Fatal(err)
	}

	expectedIndexInfos = map[string]*registry.IndexInfo{
		IndexName: {
			Name:     IndexName,
			Official: true,
			Secure:   true,
			Mirrors:  publicMirrors,
		},
		"index." + IndexName: {
			Name:     IndexName,
			Official: true,
			Secure:   true,
			Mirrors:  publicMirrors,
		},
		"example.com": {
			Name:     "example.com",
			Official: false,
			Secure:   false,
			Mirrors:  noMirrors,
		},
		"example.com:5000": {
			Name:     "example.com:5000",
			Official: false,
			Secure:   true,
			Mirrors:  noMirrors,
		},
		"127.0.0.1": {
			Name:     "127.0.0.1",
			Official: false,
			Secure:   false,
			Mirrors:  noMirrors,
		},
		"127.0.0.1:5000": {
			Name:     "127.0.0.1:5000",
			Official: false,
			Secure:   false,
			Mirrors:  noMirrors,
		},
		"other.com": {
			Name:     "other.com",
			Official: false,
			Secure:   true,
			Mirrors:  noMirrors,
		},
	}
	testIndexInfo(config, expectedIndexInfos)

	config, err = makeServiceConfig(nil, []string{"42.42.0.0/16"})
	if err != nil {
		t.Fatal(err)
	}
	expectedIndexInfos = map[string]*registry.IndexInfo{
		"example.com": {
			Name:     "example.com",
			Official: false,
			Secure:   false,
			Mirrors:  noMirrors,
		},
		"example.com:5000": {
			Name:     "example.com:5000",
			Official: false,
			Secure:   false,
			Mirrors:  noMirrors,
		},
		"127.0.0.1": {
			Name:     "127.0.0.1",
			Official: false,
			Secure:   false,
			Mirrors:  noMirrors,
		},
		"127.0.0.1:5000": {
			Name:     "127.0.0.1:5000",
			Official: false,
			Secure:   false,
			Mirrors:  noMirrors,
		},
		"other.com": {
			Name:     "other.com",
			Official: false,
			Secure:   true,
			Mirrors:  noMirrors,
		},
	}
	testIndexInfo(config, expectedIndexInfos)
}

func TestMirrorEndpointLookup(t *testing.T) {
	skip.If(t, os.Getuid() != 0, "skipping test that requires root")
	containsMirror := func(endpoints []APIEndpoint) bool {
		for _, pe := range endpoints {
			if pe.URL.Host == "my.mirror" {
				return true
			}
		}
		return false
	}
	cfg, err := makeServiceConfig([]string{"https://my.mirror"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := Service{config: cfg}

	imageName, err := reference.WithName(IndexName + "/test/image")
	if err != nil {
		t.Error(err)
	}
	pushAPIEndpoints, err := s.LookupPushEndpoints(reference.Domain(imageName))
	if err != nil {
		t.Fatal(err)
	}
	if containsMirror(pushAPIEndpoints) {
		t.Fatal("Push endpoint should not contain mirror")
	}

	pullAPIEndpoints, err := s.LookupPullEndpoints(reference.Domain(imageName))
	if err != nil {
		t.Fatal(err)
	}
	if !containsMirror(pullAPIEndpoints) {
		t.Fatal("Pull endpoint should contain mirror")
	}
}

func TestSearchRepositories(t *testing.T) {
	r := spawnTestRegistrySession(t)
	results, err := r.searchRepositories("fakequery", 25)
	if err != nil {
		t.Fatal(err)
	}
	if results == nil {
		t.Fatal("Expected non-nil SearchResults object")
	}
	assert.Equal(t, results.NumResults, 1, "Expected 1 search results")
	assert.Equal(t, results.Query, "fakequery", "Expected 'fakequery' as query")
	assert.Equal(t, results.Results[0].StarCount, 42, "Expected 'fakeimage' to have 42 stars")
}

func TestTrustedLocation(t *testing.T) {
	for _, url := range []string{"http://example.com", "https://example.com:7777", "http://docker.io", "http://test.docker.com", "https://fakedocker.com"} {
		req, _ := http.NewRequest(http.MethodGet, url, nil)
		assert.Check(t, !trustedLocation(req))
	}

	for _, url := range []string{"https://docker.io", "https://test.docker.com:80"} {
		req, _ := http.NewRequest(http.MethodGet, url, nil)
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

func TestAllowNondistributableArtifacts(t *testing.T) {
	tests := []struct {
		addr       string
		registries []string
		expected   bool
	}{
		{IndexName, nil, false},
		{"example.com", []string{}, false},
		{"example.com", []string{"example.com"}, true},
		{"localhost", []string{"localhost:5000"}, false},
		{"localhost:5000", []string{"localhost:5000"}, true},
		{"localhost", []string{"example.com"}, false},
		{"127.0.0.1:5000", []string{"127.0.0.1:5000"}, true},
		{"localhost", nil, false},
		{"localhost:5000", nil, false},
		{"127.0.0.1", nil, false},
		{"localhost", []string{"example.com"}, false},
		{"127.0.0.1", []string{"example.com"}, false},
		{"example.com", nil, false},
		{"example.com", []string{"example.com"}, true},
		{"127.0.0.1", []string{"example.com"}, false},
		{"127.0.0.1:5000", []string{"example.com"}, false},
		{"example.com:5000", []string{"42.42.0.0/16"}, true},
		{"example.com", []string{"42.42.0.0/16"}, true},
		{"example.com:5000", []string{"42.42.42.42/8"}, true},
		{"127.0.0.1:5000", []string{"127.0.0.0/8"}, true},
		{"42.42.42.42:5000", []string{"42.1.1.1/8"}, true},
		{"invalid.example.com", []string{"42.42.0.0/16"}, false},
		{"invalid.example.com", []string{"invalid.example.com"}, true},
		{"invalid.example.com:5000", []string{"invalid.example.com"}, false},
		{"invalid.example.com:5000", []string{"invalid.example.com:5000"}, true},
	}
	for _, tt := range tests {
		config, err := newServiceConfig(ServiceOptions{
			AllowNondistributableArtifacts: tt.registries,
		})
		if err != nil {
			t.Error(err)
		}
		if v := config.allowNondistributableArtifacts(tt.addr); v != tt.expected {
			t.Errorf("allowNondistributableArtifacts failed for %q %v, expected %v got %v", tt.addr, tt.registries, tt.expected, v)
		}
	}
}

func TestIsSecureIndex(t *testing.T) {
	tests := []struct {
		addr               string
		insecureRegistries []string
		expected           bool
	}{
		{IndexName, nil, true},
		{"example.com", []string{}, true},
		{"example.com", []string{"example.com"}, false},
		{"localhost", []string{"localhost:5000"}, false},
		{"localhost:5000", []string{"localhost:5000"}, false},
		{"localhost", []string{"example.com"}, false},
		{"127.0.0.1:5000", []string{"127.0.0.1:5000"}, false},
		{"localhost", nil, false},
		{"localhost:5000", nil, false},
		{"127.0.0.1", nil, false},
		{"localhost", []string{"example.com"}, false},
		{"127.0.0.1", []string{"example.com"}, false},
		{"example.com", nil, true},
		{"example.com", []string{"example.com"}, false},
		{"127.0.0.1", []string{"example.com"}, false},
		{"127.0.0.1:5000", []string{"example.com"}, false},
		{"example.com:5000", []string{"42.42.0.0/16"}, false},
		{"example.com", []string{"42.42.0.0/16"}, false},
		{"example.com:5000", []string{"42.42.42.42/8"}, false},
		{"127.0.0.1:5000", []string{"127.0.0.0/8"}, false},
		{"42.42.42.42:5000", []string{"42.1.1.1/8"}, false},
		{"invalid.example.com", []string{"42.42.0.0/16"}, true},
		{"invalid.example.com", []string{"invalid.example.com"}, false},
		{"invalid.example.com:5000", []string{"invalid.example.com"}, true},
		{"invalid.example.com:5000", []string{"invalid.example.com:5000"}, false},
	}
	for _, tt := range tests {
		config, err := makeServiceConfig(nil, tt.insecureRegistries)
		if err != nil {
			t.Error(err)
		}
		if sec := config.isSecureIndex(tt.addr); sec != tt.expected {
			t.Errorf("isSecureIndex failed for %q %v, expected %v got %v", tt.addr, tt.insecureRegistries, tt.expected, sec)
		}
	}
}

type debugTransport struct {
	http.RoundTripper
	log func(...interface{})
}

func (tr debugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	dump, err := httputil.DumpRequestOut(req, false)
	if err != nil {
		tr.log("could not dump request")
	}
	tr.log(string(dump))
	resp, err := tr.RoundTripper.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	dump, err = httputil.DumpResponse(resp, false)
	if err != nil {
		tr.log("could not dump response")
	}
	tr.log(string(dump))
	return resp, err
}
