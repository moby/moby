package v2

import (
	"net/http"
	"net/url"
	"testing"
)

type urlBuilderTestCase struct {
	description  string
	expectedPath string
	build        func() (string, error)
}

func makeURLBuilderTestCases(urlBuilder *URLBuilder) []urlBuilderTestCase {
	return []urlBuilderTestCase{
		{
			description:  "test base url",
			expectedPath: "/v2/",
			build:        urlBuilder.BuildBaseURL,
		},
		{
			description:  "test tags url",
			expectedPath: "/v2/foo/bar/tags/list",
			build: func() (string, error) {
				return urlBuilder.BuildTagsURL("foo/bar")
			},
		},
		{
			description:  "test manifest url",
			expectedPath: "/v2/foo/bar/manifests/tag",
			build: func() (string, error) {
				return urlBuilder.BuildManifestURL("foo/bar", "tag")
			},
		},
		{
			description:  "build blob url",
			expectedPath: "/v2/foo/bar/blobs/tarsum.v1+sha256:abcdef0123456789",
			build: func() (string, error) {
				return urlBuilder.BuildBlobURL("foo/bar", "tarsum.v1+sha256:abcdef0123456789")
			},
		},
		{
			description:  "build blob upload url",
			expectedPath: "/v2/foo/bar/blobs/uploads/",
			build: func() (string, error) {
				return urlBuilder.BuildBlobUploadURL("foo/bar")
			},
		},
		{
			description:  "build blob upload url with digest and size",
			expectedPath: "/v2/foo/bar/blobs/uploads/?digest=tarsum.v1%2Bsha256%3Aabcdef0123456789&size=10000",
			build: func() (string, error) {
				return urlBuilder.BuildBlobUploadURL("foo/bar", url.Values{
					"size":   []string{"10000"},
					"digest": []string{"tarsum.v1+sha256:abcdef0123456789"},
				})
			},
		},
		{
			description:  "build blob upload chunk url",
			expectedPath: "/v2/foo/bar/blobs/uploads/uuid-part",
			build: func() (string, error) {
				return urlBuilder.BuildBlobUploadChunkURL("foo/bar", "uuid-part")
			},
		},
		{
			description:  "build blob upload chunk url with digest and size",
			expectedPath: "/v2/foo/bar/blobs/uploads/uuid-part?digest=tarsum.v1%2Bsha256%3Aabcdef0123456789&size=10000",
			build: func() (string, error) {
				return urlBuilder.BuildBlobUploadChunkURL("foo/bar", "uuid-part", url.Values{
					"size":   []string{"10000"},
					"digest": []string{"tarsum.v1+sha256:abcdef0123456789"},
				})
			},
		},
	}
}

// TestURLBuilder tests the various url building functions, ensuring they are
// returning the expected values.
func TestURLBuilder(t *testing.T) {
	roots := []string{
		"http://example.com",
		"https://example.com",
		"http://localhost:5000",
		"https://localhost:5443",
	}

	for _, root := range roots {
		urlBuilder, err := NewURLBuilderFromString(root)
		if err != nil {
			t.Fatalf("unexpected error creating urlbuilder: %v", err)
		}

		for _, testCase := range makeURLBuilderTestCases(urlBuilder) {
			url, err := testCase.build()
			if err != nil {
				t.Fatalf("%s: error building url: %v", testCase.description, err)
			}

			expectedURL := root + testCase.expectedPath

			if url != expectedURL {
				t.Fatalf("%s: %q != %q", testCase.description, url, expectedURL)
			}
		}
	}
}

func TestURLBuilderWithPrefix(t *testing.T) {
	roots := []string{
		"http://example.com/prefix/",
		"https://example.com/prefix/",
		"http://localhost:5000/prefix/",
		"https://localhost:5443/prefix/",
	}

	for _, root := range roots {
		urlBuilder, err := NewURLBuilderFromString(root)
		if err != nil {
			t.Fatalf("unexpected error creating urlbuilder: %v", err)
		}

		for _, testCase := range makeURLBuilderTestCases(urlBuilder) {
			url, err := testCase.build()
			if err != nil {
				t.Fatalf("%s: error building url: %v", testCase.description, err)
			}

			expectedURL := root[0:len(root)-1] + testCase.expectedPath

			if url != expectedURL {
				t.Fatalf("%s: %q != %q", testCase.description, url, expectedURL)
			}
		}
	}
}

type builderFromRequestTestCase struct {
	request *http.Request
	base    string
}

func TestBuilderFromRequest(t *testing.T) {
	u, err := url.Parse("http://example.com")
	if err != nil {
		t.Fatal(err)
	}

	forwardedProtoHeader := make(http.Header, 1)
	forwardedProtoHeader.Set("X-Forwarded-Proto", "https")

	forwardedHostHeader1 := make(http.Header, 1)
	forwardedHostHeader1.Set("X-Forwarded-Host", "first.example.com")

	forwardedHostHeader2 := make(http.Header, 1)
	forwardedHostHeader2.Set("X-Forwarded-Host", "first.example.com, proxy1.example.com")

	testRequests := []struct {
		request *http.Request
		base    string
	}{
		{
			request: &http.Request{URL: u, Host: u.Host},
			base:    "http://example.com",
		},
		{
			request: &http.Request{URL: u, Host: u.Host, Header: forwardedProtoHeader},
			base:    "https://example.com",
		},
		{
			request: &http.Request{URL: u, Host: u.Host, Header: forwardedHostHeader1},
			base:    "http://first.example.com",
		},
		{
			request: &http.Request{URL: u, Host: u.Host, Header: forwardedHostHeader2},
			base:    "http://first.example.com",
		},
	}

	for _, tr := range testRequests {
		builder := NewURLBuilderFromRequest(tr.request)

		for _, testCase := range makeURLBuilderTestCases(builder) {
			url, err := testCase.build()
			if err != nil {
				t.Fatalf("%s: error building url: %v", testCase.description, err)
			}

			expectedURL := tr.base + testCase.expectedPath

			if url != expectedURL {
				t.Fatalf("%s: %q != %q", testCase.description, url, expectedURL)
			}
		}
	}
}

func TestBuilderFromRequestWithPrefix(t *testing.T) {
	u, err := url.Parse("http://example.com/prefix/v2/")
	if err != nil {
		t.Fatal(err)
	}

	forwardedProtoHeader := make(http.Header, 1)
	forwardedProtoHeader.Set("X-Forwarded-Proto", "https")

	testRequests := []struct {
		request *http.Request
		base    string
	}{
		{
			request: &http.Request{URL: u, Host: u.Host},
			base:    "http://example.com/prefix/",
		},
		{
			request: &http.Request{URL: u, Host: u.Host, Header: forwardedProtoHeader},
			base:    "https://example.com/prefix/",
		},
	}

	for _, tr := range testRequests {
		builder := NewURLBuilderFromRequest(tr.request)

		for _, testCase := range makeURLBuilderTestCases(builder) {
			url, err := testCase.build()
			if err != nil {
				t.Fatalf("%s: error building url: %v", testCase.description, err)
			}

			expectedURL := tr.base[0:len(tr.base)-1] + testCase.expectedPath

			if url != expectedURL {
				t.Fatalf("%s: %q != %q", testCase.description, url, expectedURL)
			}
		}
	}
}
