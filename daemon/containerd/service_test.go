package containerd

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestIsRemoteSnapshotter(t *testing.T) {
	testCases := []struct {
		name        string
		snapshotter string
		expected    bool
	}{
		{"overlayfs", "overlayfs", false},
		{"native", "native", false},
		{"erofs", "erofs", false},
		{"windows", "windows", false},
		{"stargz", "stargz", true},
		{"nydus", "nydus", true},
		{"overlaybd", "overlaybd", true},
		{"custom-remote", "custom-remote", true},
		{"empty name", "", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isRemoteSnapshotter(tc.snapshotter)
			assert.Assert(t, result == tc.expected,
				"isRemoteSnapshotter(%q) = %v, want %v", tc.snapshotter, result, tc.expected)
		})
	}
}

func TestUnpackFetchAllContentSelection(t *testing.T) {
	testCases := []struct {
		name        string
		snapshotter string
		expected    bool
	}{
		{"local overlayfs eager-fetch preserved (#49784)", "overlayfs", true},
		{"local native eager-fetch preserved (#49784)", "native", true},
		{"local erofs eager-fetch preserved (#49784)", "erofs", true},
		{"local windows eager-fetch preserved (#49784)", "windows", true},
		{"remote stargz lazy-fetched (#53864)", "stargz", false},
		{"remote nydus lazy-fetched (#53864)", "nydus", false},
		{"remote overlaybd lazy-fetched (#53864)", "overlaybd", false},
		{"custom remote lazy-fetched (#53864)", "custom-remote", false},
		{"empty name falls back to eager (#49784)", "", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := unpackFetchAllContentForSnapshotter(tc.snapshotter)
			assert.Assert(t, result == tc.expected,
				"unpackFetchAllContentForSnapshotter(%q) = %v, want %v",
				tc.snapshotter, result, tc.expected)
		})
	}
}
