package containerd

import (
	"testing"

	"gotest.tools/v3/assert"
)

type staticFeatures map[string]bool

func (f staticFeatures) Features() map[string]bool { return f }

func TestLazyPull(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		snapshotter string
		features    featuresProvider
		expected    bool
	}{
		{name: "remote snapshotter without features", snapshotter: "stargz", expected: true},
		{name: "regular snapshotter without features", snapshotter: "overlayfs", expected: false},
		{name: "unset feature uses snapshotter default", snapshotter: "stargz", features: staticFeatures{}, expected: true},
		{name: "feature disables remote snapshotter", snapshotter: "stargz", features: staticFeatures{"lazy-pull": false}, expected: false},
		{name: "feature enables regular snapshotter", snapshotter: "overlayfs", features: staticFeatures{"lazy-pull": true}, expected: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			i := &ImageService{features: tc.features}
			assert.Equal(t, i.lazyPull(tc.snapshotter), tc.expected)
		})
	}
}
