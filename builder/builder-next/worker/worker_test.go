package worker

import (
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestWorkerPlatforms(t *testing.T) {
	defaultPlatform := ocispec.Platform{OS: "linux", Architecture: "amd64"}
	otherPlatform := ocispec.Platform{OS: "windows", Architecture: "amd64"}
	thirdPlatform := ocispec.Platform{OS: "darwin", Architecture: "arm64"}

	tests := []struct {
		name        string
		platforms   []ocispec.Platform
		platformsFn func(noCache bool) []ocispec.Platform
		noCache     bool
		contains    []ocispec.Platform
		wantLen     int
	}{
		{
			name:      "SetPlatforms",
			platforms: []ocispec.Platform{defaultPlatform, otherPlatform},
			noCache:   false,
			contains:  []ocispec.Platform{defaultPlatform, otherPlatform},
			wantLen:   2,
		},
		{
			name:      "NoCacheAddsSupported",
			platforms: []ocispec.Platform{defaultPlatform},
			platformsFn: func(noCache bool) []ocispec.Platform {
				return []ocispec.Platform{defaultPlatform, otherPlatform, thirdPlatform}
			},
			noCache:  true,
			wantLen:  3,
			contains: []ocispec.Platform{defaultPlatform, otherPlatform, thirdPlatform},
		},
		{
			name:      "NoCacheFalseDoesNotAdd",
			platforms: []ocispec.Platform{defaultPlatform},
			platformsFn: func(noCache bool) []ocispec.Platform {
				return []ocispec.Platform{defaultPlatform, otherPlatform}
			},
			noCache:  false,
			wantLen:  1,
			contains: []ocispec.Platform{defaultPlatform},
		},
		{
			name:      "DuplicateSupportedNotAdded",
			platforms: []ocispec.Platform{defaultPlatform, otherPlatform},
			platformsFn: func(noCache bool) []ocispec.Platform {
				return []ocispec.Platform{defaultPlatform, otherPlatform}
			},
			noCache:  true,
			wantLen:  2,
			contains: []ocispec.Platform{defaultPlatform, otherPlatform},
		},
		{
			name:      "MultipleCallsNoDuplicates",
			platforms: []ocispec.Platform{defaultPlatform},
			platformsFn: func(noCache bool) []ocispec.Platform {
				return []ocispec.Platform{defaultPlatform, otherPlatform}
			},
			noCache:  true,
			wantLen:  2,
			contains: []ocispec.Platform{defaultPlatform, otherPlatform},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := &Worker{
				Opt:         Opt{Platforms: append([]ocispec.Platform{}, tc.platforms...)},
				platformsFn: tc.platformsFn,
			}

			got := w.Platforms(tc.noCache)

			assert.Equal(t, len(got), tc.wantLen)
			for _, p := range tc.contains {
				assert.Assert(t, is.Contains(got, p))
			}

			// For MultipleCallsNoDuplicates, call twice and compare
			if tc.name == "MultipleCallsNoDuplicates" {
				got2 := w.Platforms(tc.noCache)
				assert.DeepEqual(t, got, got2)
			}
		})
	}
}
