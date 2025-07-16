package worker

import (
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestMergePlatforms(t *testing.T) {
	defaultPlatform := ocispec.Platform{OS: "linux", Architecture: "amd64"}
	otherPlatform := ocispec.Platform{OS: "windows", Architecture: "amd64"}
	thirdPlatform := ocispec.Platform{OS: "darwin", Architecture: "arm64"}

	tests := []struct {
		name      string
		defined   []ocispec.Platform
		supported []ocispec.Platform
		contains  []ocispec.Platform
		wantLen   int
	}{
		{
			name:      "AllUnique",
			defined:   []ocispec.Platform{defaultPlatform},
			supported: []ocispec.Platform{otherPlatform, thirdPlatform},
			contains:  []ocispec.Platform{defaultPlatform, otherPlatform, thirdPlatform},
			wantLen:   3,
		},
		{
			name:      "SomeOverlap",
			defined:   []ocispec.Platform{defaultPlatform, otherPlatform},
			supported: []ocispec.Platform{otherPlatform, thirdPlatform},
			contains:  []ocispec.Platform{defaultPlatform, otherPlatform, thirdPlatform},
			wantLen:   3,
		},
		{
			name:      "AllOverlap",
			defined:   []ocispec.Platform{defaultPlatform, otherPlatform},
			supported: []ocispec.Platform{defaultPlatform, otherPlatform},
			contains:  []ocispec.Platform{defaultPlatform, otherPlatform},
			wantLen:   2,
		},
		{
			name:      "EmptySupported",
			defined:   []ocispec.Platform{defaultPlatform},
			supported: []ocispec.Platform{},
			contains:  []ocispec.Platform{defaultPlatform},
			wantLen:   1,
		},
		{
			name:      "EmptyDefined",
			defined:   []ocispec.Platform{},
			supported: []ocispec.Platform{defaultPlatform, otherPlatform},
			contains:  []ocispec.Platform{defaultPlatform, otherPlatform},
			wantLen:   2,
		},
		{
			name:      "BothEmpty",
			defined:   []ocispec.Platform{},
			supported: []ocispec.Platform{},
			contains:  []ocispec.Platform{},
			wantLen:   0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mergePlatforms(tc.defined, tc.supported)
			assert.Equal(t, len(got), tc.wantLen)
			for _, p := range tc.contains {
				assert.Assert(t, is.Contains(got, p))
			}
		})
	}
}
