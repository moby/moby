package v2

import (
	"net/url"
	"testing"
)

type urlBuilderTestCase struct {
	description string
	expected    string
	build       func() (string, error)
}

// TestURLBuilder tests the various url building functions, ensuring they are
// returning the expected values.
func TestURLBuilder(t *testing.T) {

	root := "http://localhost:5000/"
	urlBuilder, err := NewURLBuilderFromString(root)
	if err != nil {
		t.Fatalf("unexpected error creating urlbuilder: %v", err)
	}

	for _, testcase := range []struct {
		description string
		expected    string
		build       func() (string, error)
	}{
		{
			description: "test base url",
			expected:    "http://localhost:5000/v2/",
			build:       urlBuilder.BuildBaseURL,
		},
		{
			description: "test tags url",
			expected:    "http://localhost:5000/v2/foo/bar/tags/list",
			build: func() (string, error) {
				return urlBuilder.BuildTagsURL("foo/bar")
			},
		},
		{
			description: "test manifest url",
			expected:    "http://localhost:5000/v2/foo/bar/manifests/tag",
			build: func() (string, error) {
				return urlBuilder.BuildManifestURL("foo/bar", "tag")
			},
		},
		{
			description: "build blob url",
			expected:    "http://localhost:5000/v2/foo/bar/blobs/tarsum.v1+sha256:abcdef0123456789",
			build: func() (string, error) {
				return urlBuilder.BuildBlobURL("foo/bar", "tarsum.v1+sha256:abcdef0123456789")
			},
		},
		{
			description: "build blob upload url",
			expected:    "http://localhost:5000/v2/foo/bar/blobs/uploads/",
			build: func() (string, error) {
				return urlBuilder.BuildBlobUploadURL("foo/bar")
			},
		},
		{
			description: "build blob upload url with digest and size",
			expected:    "http://localhost:5000/v2/foo/bar/blobs/uploads/?digest=tarsum.v1%2Bsha256%3Aabcdef0123456789&size=10000",
			build: func() (string, error) {
				return urlBuilder.BuildBlobUploadURL("foo/bar", url.Values{
					"size":   []string{"10000"},
					"digest": []string{"tarsum.v1+sha256:abcdef0123456789"},
				})
			},
		},
		{
			description: "build blob upload chunk url",
			expected:    "http://localhost:5000/v2/foo/bar/blobs/uploads/uuid-part",
			build: func() (string, error) {
				return urlBuilder.BuildBlobUploadChunkURL("foo/bar", "uuid-part")
			},
		},
		{
			description: "build blob upload chunk url with digest and size",
			expected:    "http://localhost:5000/v2/foo/bar/blobs/uploads/uuid-part?digest=tarsum.v1%2Bsha256%3Aabcdef0123456789&size=10000",
			build: func() (string, error) {
				return urlBuilder.BuildBlobUploadChunkURL("foo/bar", "uuid-part", url.Values{
					"size":   []string{"10000"},
					"digest": []string{"tarsum.v1+sha256:abcdef0123456789"},
				})
			},
		},
	} {
		u, err := testcase.build()
		if err != nil {
			t.Fatalf("%s: error building url: %v", testcase.description, err)
		}

		if u != testcase.expected {
			t.Fatalf("%s: %q != %q", testcase.description, u, testcase.expected)
		}
	}

}
