package registry

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/docker/docker/utils"
)

var (
	token = []string{"fake-token"}
)

const (
	imageID = "42d718c941f5c532ac049bf0b0ab53f0062f09a03afd4aa4a02c098e46032b9d"
	REPO    = "foo42/bar"
)

func spawnTestRegistrySession(t *testing.T) *Session {
	authConfig := &AuthConfig{}
	endpoint, err := NewEndpoint(makeURL("/v1/"), insecureRegistries)
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewSession(authConfig, utils.NewHTTPRequestFactory(), endpoint, true)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestPingRegistryEndpoint(t *testing.T) {
	ep, err := NewEndpoint(makeURL("/v1/"), insecureRegistries)
	if err != nil {
		t.Fatal(err)
	}
	regInfo, err := ep.Ping()
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, regInfo.Standalone, true, "Expected standalone to be true (default)")
}

func TestGetRemoteHistory(t *testing.T) {
	r := spawnTestRegistrySession(t)
	hist, err := r.GetRemoteHistory(imageID, makeURL("/v1/"), token)
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, len(hist), 2, "Expected 2 images in history")
	assertEqual(t, hist[0], imageID, "Expected "+imageID+"as first ancestry")
	assertEqual(t, hist[1], "77dbf71da1d00e3fbddc480176eac8994025630c6590d11cfc8fe1209c2a1d20",
		"Unexpected second ancestry")
}

func TestLookupRemoteImage(t *testing.T) {
	r := spawnTestRegistrySession(t)
	found := r.LookupRemoteImage(imageID, makeURL("/v1/"), token)
	assertEqual(t, found, true, "Expected remote lookup to succeed")
	found = r.LookupRemoteImage("abcdef", makeURL("/v1/"), token)
	assertEqual(t, found, false, "Expected remote lookup to fail")
}

func TestGetRemoteImageJSON(t *testing.T) {
	r := spawnTestRegistrySession(t)
	json, size, err := r.GetRemoteImageJSON(imageID, makeURL("/v1/"), token)
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, size, 154, "Expected size 154")
	if len(json) <= 0 {
		t.Fatal("Expected non-empty json")
	}

	_, _, err = r.GetRemoteImageJSON("abcdef", makeURL("/v1/"), token)
	if err == nil {
		t.Fatal("Expected image not found error")
	}
}

func TestGetRemoteImageLayer(t *testing.T) {
	r := spawnTestRegistrySession(t)
	data, err := r.GetRemoteImageLayer(imageID, makeURL("/v1/"), token, 0)
	if err != nil {
		t.Fatal(err)
	}
	if data == nil {
		t.Fatal("Expected non-nil data result")
	}

	_, err = r.GetRemoteImageLayer("abcdef", makeURL("/v1/"), token, 0)
	if err == nil {
		t.Fatal("Expected image not found error")
	}
}

func TestGetRemoteTags(t *testing.T) {
	r := spawnTestRegistrySession(t)
	tags, err := r.GetRemoteTags([]string{makeURL("/v1/")}, REPO, token)
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, len(tags), 1, "Expected one tag")
	assertEqual(t, tags["latest"], imageID, "Expected tag latest to map to "+imageID)

	_, err = r.GetRemoteTags([]string{makeURL("/v1/")}, "foo42/baz", token)
	if err == nil {
		t.Fatal("Expected error when fetching tags for bogus repo")
	}
}

func TestGetRepositoryData(t *testing.T) {
	r := spawnTestRegistrySession(t)
	parsedURL, err := url.Parse(makeURL("/v1/"))
	if err != nil {
		t.Fatal(err)
	}
	host := "http://" + parsedURL.Host + "/v1/"
	data, err := r.GetRepositoryData("foo42/bar")
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, len(data.ImgList), 2, "Expected 2 images in ImgList")
	assertEqual(t, len(data.Endpoints), 2,
		fmt.Sprintf("Expected 2 endpoints in Endpoints, found %d instead", len(data.Endpoints)))
	assertEqual(t, data.Endpoints[0], host,
		fmt.Sprintf("Expected first endpoint to be %s but found %s instead", host, data.Endpoints[0]))
	assertEqual(t, data.Endpoints[1], "http://test.example.com/v1/",
		fmt.Sprintf("Expected first endpoint to be http://test.example.com/v1/ but found %s instead", data.Endpoints[1]))

}

func TestPushImageJSONRegistry(t *testing.T) {
	r := spawnTestRegistrySession(t)
	imgData := &ImgData{
		ID:       "77dbf71da1d00e3fbddc480176eac8994025630c6590d11cfc8fe1209c2a1d20",
		Checksum: "sha256:1ac330d56e05eef6d438586545ceff7550d3bdcb6b19961f12c5ba714ee1bb37",
	}

	err := r.PushImageJSONRegistry(imgData, []byte{0x42, 0xdf, 0x0}, makeURL("/v1/"), token)
	if err != nil {
		t.Fatal(err)
	}
}

func TestPushImageLayerRegistry(t *testing.T) {
	r := spawnTestRegistrySession(t)
	layer := strings.NewReader("")
	_, _, err := r.PushImageLayerRegistry(imageID, layer, makeURL("/v1/"), token, []byte{})
	if err != nil {
		t.Fatal(err)
	}
}

func TestResolveRepositoryName(t *testing.T) {
	_, _, err := ResolveRepositoryName("https://github.com/docker/docker")
	assertEqual(t, err, ErrInvalidRepositoryName, "Expected error invalid repo name")
	ep, repo, err := ResolveRepositoryName("fooo/bar")
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, ep, IndexServerAddress(), "Expected endpoint to be index server address")
	assertEqual(t, repo, "fooo/bar", "Expected resolved repo to be foo/bar")

	u := makeURL("")[7:]
	ep, repo, err = ResolveRepositoryName(u + "/private/moonbase")
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, ep, u, "Expected endpoint to be "+u)
	assertEqual(t, repo, "private/moonbase", "Expected endpoint to be private/moonbase")

	ep, repo, err = ResolveRepositoryName("ubuntu-12.04-base")
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, ep, IndexServerAddress(), "Expected endpoint to be "+IndexServerAddress())
	assertEqual(t, repo, "ubuntu-12.04-base", "Expected endpoint to be ubuntu-12.04-base")
}

func TestPushRegistryTag(t *testing.T) {
	r := spawnTestRegistrySession(t)
	err := r.PushRegistryTag("foo42/bar", imageID, "stable", makeURL("/v1/"), token)
	if err != nil {
		t.Fatal(err)
	}
}

func TestPushImageJSONIndex(t *testing.T) {
	r := spawnTestRegistrySession(t)
	imgData := []*ImgData{
		{
			ID:       "77dbf71da1d00e3fbddc480176eac8994025630c6590d11cfc8fe1209c2a1d20",
			Checksum: "sha256:1ac330d56e05eef6d438586545ceff7550d3bdcb6b19961f12c5ba714ee1bb37",
		},
		{
			ID:       "42d718c941f5c532ac049bf0b0ab53f0062f09a03afd4aa4a02c098e46032b9d",
			Checksum: "sha256:bea7bf2e4bacd479344b737328db47b18880d09096e6674165533aa994f5e9f2",
		},
	}
	repoData, err := r.PushImageJSONIndex("foo42/bar", imgData, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if repoData == nil {
		t.Fatal("Expected RepositoryData object")
	}
	repoData, err = r.PushImageJSONIndex("foo42/bar", imgData, true, []string{r.indexEndpoint.String()})
	if err != nil {
		t.Fatal(err)
	}
	if repoData == nil {
		t.Fatal("Expected RepositoryData object")
	}
}

func TestSearchRepositories(t *testing.T) {
	r := spawnTestRegistrySession(t)
	results, err := r.SearchRepositories("fakequery")
	if err != nil {
		t.Fatal(err)
	}
	if results == nil {
		t.Fatal("Expected non-nil SearchResults object")
	}
	assertEqual(t, results.NumResults, 1, "Expected 1 search results")
	assertEqual(t, results.Query, "fakequery", "Expected 'fakequery' as query")
	assertEqual(t, results.Results[0].StarCount, 42, "Expected 'fakeimage' a ot hae 42 stars")
}

func TestValidRepositoryName(t *testing.T) {
	if err := validateRepositoryName("docker/docker"); err != nil {
		t.Fatal(err)
	}
	// Support 64-byte non-hexadecimal names (hexadecimal names are forbidden)
	if err := validateRepositoryName("thisisthesongthatneverendsitgoesonandonandonthisisthesongthatnev"); err != nil {
		t.Fatal(err)
	}
	if err := validateRepositoryName("docker/Docker"); err == nil {
		t.Log("Repository name should be invalid")
		t.Fail()
	}
	if err := validateRepositoryName("docker///docker"); err == nil {
		t.Log("Repository name should be invalid")
		t.Fail()
	}
	if err := validateRepositoryName("1a3f5e7d9c1b3a5f7e9d1c3b5a7f9e1d3c5b7a9f1e3d5d7c9b1a3f5e7d9c1b3a"); err == nil {
		t.Log("Repository name should be invalid, 64-byte hexadecimal names forbidden")
		t.Fail()
	}
}

func TestTrustedLocation(t *testing.T) {
	for _, url := range []string{"http://example.com", "https://example.com:7777", "http://docker.io", "http://test.docker.com", "https://fakedocker.com"} {
		req, _ := http.NewRequest("GET", url, nil)
		if trustedLocation(req) == true {
			t.Fatalf("'%s' shouldn't be detected as a trusted location", url)
		}
	}

	for _, url := range []string{"https://docker.io", "https://test.docker.com:80"} {
		req, _ := http.NewRequest("GET", url, nil)
		if trustedLocation(req) == false {
			t.Fatalf("'%s' should be detected as a trusted location", url)
		}
	}
}

func TestAddRequiredHeadersToRedirectedRequests(t *testing.T) {
	for _, urls := range [][]string{
		{"http://docker.io", "https://docker.com"},
		{"https://foo.docker.io:7777", "http://bar.docker.com"},
		{"https://foo.docker.io", "https://example.com"},
	} {
		reqFrom, _ := http.NewRequest("GET", urls[0], nil)
		reqFrom.Header.Add("Content-Type", "application/json")
		reqFrom.Header.Add("Authorization", "super_secret")
		reqTo, _ := http.NewRequest("GET", urls[1], nil)

		AddRequiredHeadersToRedirectedRequests(reqTo, []*http.Request{reqFrom})

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
		reqFrom, _ := http.NewRequest("GET", urls[0], nil)
		reqFrom.Header.Add("Content-Type", "application/json")
		reqFrom.Header.Add("Authorization", "super_secret")
		reqTo, _ := http.NewRequest("GET", urls[1], nil)

		AddRequiredHeadersToRedirectedRequests(reqTo, []*http.Request{reqFrom})

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

func TestIsSecure(t *testing.T) {
	tests := []struct {
		addr               string
		insecureRegistries []string
		expected           bool
	}{
		{IndexServerURL.Host, nil, true},
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
	}
	for _, tt := range tests {
		// TODO: remove this once we remove localhost insecure by default
		insecureRegistries := append(tt.insecureRegistries, "127.0.0.0/8")
		if sec, err := isSecure(tt.addr, insecureRegistries); err != nil || sec != tt.expected {
			t.Fatalf("isSecure failed for %q %v, expected %v got %v. Error: %v", tt.addr, insecureRegistries, tt.expected, sec, err)
		}
	}
}
