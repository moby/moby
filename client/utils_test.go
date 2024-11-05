package client

import (
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
)

func TestEncodePlatforms(t *testing.T) {
	tests := []struct {
		doc       string
		platforms []ocispec.Platform
		expected  []string
	}{
		{
			doc: "single platform",
			platforms: []ocispec.Platform{
				{Architecture: "arm64", OS: "windows", Variant: "v8", OSVersion: "99.99.99"},
			},
			expected: []string{
				`{"architecture":"arm64","os":"windows","os.version":"99.99.99","variant":"v8"}`,
			},
		},
		{
			doc: "multiple platforms",
			platforms: []ocispec.Platform{
				{Architecture: "arm64", OS: "linux", Variant: "v8"},
				{Architecture: "arm64", OS: "windows", Variant: "v8", OSVersion: "99.99.99"},
			},
			expected: []string{
				`{"architecture":"arm64","os":"linux","variant":"v8"}`,
				`{"architecture":"arm64","os":"windows","os.version":"99.99.99","variant":"v8"}`,
			},
		},
		{
			doc: "multiple platforms with duplicates",
			platforms: []ocispec.Platform{
				{Architecture: "arm64", OS: "linux", Variant: "v8"},
				{Architecture: "arm64", OS: "windows", Variant: "v8", OSVersion: "99.99.99"},
				{Architecture: "arm64", OS: "linux", Variant: "v8"},
			},
			expected: []string{
				`{"architecture":"arm64","os":"linux","variant":"v8"}`,
				`{"architecture":"arm64","os":"windows","os.version":"99.99.99","variant":"v8"}`,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			out, err := encodePlatforms(tc.platforms...)
			assert.NilError(t, err)
			assert.DeepEqual(t, out, tc.expected)
		})
	}
}
