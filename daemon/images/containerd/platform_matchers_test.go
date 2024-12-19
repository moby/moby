package containerd

import (
	"runtime"
	"testing"

	"github.com/containerd/platforms"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

var (
	pLinuxAmd64 = ocispec.Platform{
		OS:           "linux",
		Architecture: "amd64",
	}

	pLinuxArmv5 = ocispec.Platform{
		OS:           "linux",
		Architecture: "arm",
		Variant:      "v5",
	}

	pLinuxArmv6 = ocispec.Platform{
		OS:           "linux",
		Architecture: "arm",
		Variant:      "v6",
	}

	pLinuxArm64 = ocispec.Platform{
		OS:           "linux",
		Architecture: "arm64",
		Variant:      "v8",
	}

	pWindowsAmd64 = ocispec.Platform{
		OS:           "windows",
		Architecture: "amd64",
		OSVersion:    "10.0.14393",
	}
)

type requestedAndFirst struct {
	// Whether platforms.Only or OnlyStrict should be used
	// Nil means both should be the same
	strict    *bool
	requested *ocispec.Platform
	first     *ocispec.Platform
}

type indexTestCase struct {
	name  string
	index []ocispec.Platform
	tc    []requestedAndFirst
}

func TestMatcherOnLinuxArm64v8(t *testing.T) {
	daemonPlatform := platforms.Only(ocispec.Platform{
		OS:           "linux",
		Architecture: "arm64",
		Variant:      "v8",
	})

	yes := true
	no := false

	for _, indexTc := range []indexTestCase{
		{
			name:  "linux_amd64_armv5_armv6_arm64-windows_amd64",
			index: []ocispec.Platform{pLinuxAmd64, pLinuxArmv5, pLinuxArmv6, pLinuxArm64, pWindowsAmd64},
			tc: []requestedAndFirst{
				{requested: nil, first: &pLinuxArm64},
				{requested: &ocispec.Platform{OS: "linux", Architecture: "amd64"}, first: &pLinuxAmd64},
				{requested: &ocispec.Platform{OS: "windows", Architecture: "amd64"}, first: &pWindowsAmd64},

				// Select highest possible arm variant
				{strict: &yes, requested: &ocispec.Platform{OS: "linux", Architecture: "arm"}, first: nil},
				{strict: &no, requested: &ocispec.Platform{OS: "linux", Architecture: "arm"}, first: &pLinuxArmv6},

				{requested: &ocispec.Platform{OS: "linux", Architecture: "arm", Variant: "v5"}, first: &pLinuxArmv5},

				// Variant not present
				{strict: &yes, requested: &ocispec.Platform{OS: "linux", Architecture: "arm", Variant: "v8"}, first: nil},
				{strict: &no, requested: &ocispec.Platform{OS: "linux", Architecture: "arm", Variant: "v8"}, first: &pLinuxArmv6},

				{requested: &ocispec.Platform{OS: "linux", Architecture: "s390x"}, first: nil},
			},
		},
	} {
		testOnlyAndOnlyStrict(t, daemonPlatform, indexTc)
	}
}

func TestMatcherOnWindowsAmd64(t *testing.T) {
	skip.If(t, runtime.GOOS != "windows", "TODO: containerd matcher only matches OSVersion when on Windows")
	daemonPlatform := platforms.Only(ocispec.Platform{
		OS:           "windows",
		Architecture: "amd64",
		OSVersion:    "10.0.18362",
	})

	for _, indexTc := range []indexTestCase{
		{
			name: "various windows",
			index: []ocispec.Platform{
				{OS: "windows", Architecture: "amd64", OSVersion: "10.0.14393"},
				{OS: "windows", Architecture: "amd64", OSVersion: "10.0.17763"},
				{OS: "windows", Architecture: "amd64", OSVersion: "10.0.18362"},
				{OS: "windows", Architecture: "amd64", OSVersion: "10.0.19041"},
			},
			tc: []requestedAndFirst{
				{requested: nil, first: &ocispec.Platform{OS: "windows", Architecture: "amd64", OSVersion: "10.0.18362"}},
				{requested: &ocispec.Platform{OS: "windows", Architecture: "amd64"}, first: &ocispec.Platform{OS: "windows", Architecture: "amd64", OSVersion: "10.0.14393"}},
			},
		},
	} {
		testOnlyAndOnlyStrict(t, daemonPlatform, indexTc)
	}
}

func testOnlyAndOnlyStrict(t *testing.T, daemonPlatform platforms.MatchComparer, indexTc indexTestCase) {
	imgSvc := ImageService{}
	imgSvc.defaultPlatformOverride = daemonPlatform

	t.Run(indexTc.name, func(t *testing.T) {
		indexTc := indexTc
		idx := indexTc.index
		for _, tc := range indexTc.tc {
			for _, strict := range []bool{false, true} {
				s := "non-strict"
				if strict {
					s = "strict"
				}
				if tc.strict != nil && *tc.strict != strict {
					continue
				}

				req := "default"
				if tc.requested != nil {
					req = platforms.FormatAll(*tc.requested)
				}
				wanted := "none"
				if tc.first != nil {
					wanted = platforms.FormatAll(*tc.first)
				}
				t.Run(s+"/"+req+"=>"+wanted, func(t *testing.T) {
					pm := imgSvc.matchRequestedOrDefault(platforms.Only, tc.requested)
					if strict {
						pm = imgSvc.matchRequestedOrDefault(platforms.OnlyStrict, tc.requested)
					}

					var first *ocispec.Platform
					for _, p := range idx {
						if !pm.Match(p) {
							continue
						}
						if first == nil || pm.Less(p, *first) {
							first = &p
						}
					}

					assert.Check(t, is.DeepEqual(first, tc.first))
				})
			}
		}
	})
}
