package daemon

import (
	"runtime"
	"testing"

	"github.com/moby/moby/api/types/build"
	"github.com/moby/moby/v2/daemon/config"
	"gotest.tools/v3/assert"
)

func TestBuilderVersion(t *testing.T) {
	t.Parallel()

	defaultVersion := build.BuilderBuildKit
	if runtime.GOOS == "windows" {
		defaultVersion = build.BuilderV1
	}

	for _, tc := range []struct {
		name           string
		features       map[string]bool
		useSnapshotter bool
		expected       build.BuilderVersion
	}{
		{
			name:     "classic image store default",
			expected: defaultVersion,
		},
		{
			name:           "containerd image store default",
			useSnapshotter: true,
			expected:       defaultVersion,
		},
		{
			name:     "classic image store with buildkit enabled",
			features: map[string]bool{"buildkit": true},
			expected: build.BuilderBuildKit,
		},
		{
			name:           "containerd image store with buildkit enabled",
			features:       map[string]bool{"buildkit": true},
			useSnapshotter: true,
			expected:       build.BuilderBuildKit,
		},
		{
			name:     "classic image store with buildkit disabled",
			features: map[string]bool{"buildkit": false},
			expected: build.BuilderV1,
		},
		{
			name:           "containerd image store with buildkit disabled",
			features:       map[string]bool{"buildkit": false},
			useSnapshotter: true,
			expected:       build.BuilderV1,
		},
		{
			name:     "classic image store with mismatched feature flag",
			features: map[string]bool{"containerd-snapshotter": true},
			expected: defaultVersion,
		},
		{
			name:           "containerd image store with mismatched feature flag",
			features:       map[string]bool{"containerd-snapshotter": false},
			useSnapshotter: true,
			expected:       defaultVersion,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			daemon := &Daemon{usesSnapshotter: tc.useSnapshotter}
			daemon.configStore.Store(&configStore{
				Config: config.Config{
					CommonConfig: config.CommonConfig{Features: tc.features},
				},
			})
			assert.Equal(t, daemon.BuilderVersion(), tc.expected)
		})
	}
}
