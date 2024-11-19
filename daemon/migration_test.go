package daemon

import (
	"context"
	"errors"
	"testing"

	"github.com/containerd/platforms"
	"github.com/docker/docker/container"
	"github.com/docker/docker/image"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
)

type mockPlatformReader struct{}

func (m mockPlatformReader) ReadPlatformFromImage(ctx context.Context, id image.ID) (ocispec.Platform, error) {
	switch id {
	case "multiplatform":
		// This image has multiple platforms, but GetImage will prefer the first one
		// because the ID points to the full image index, not a specific platform.
		return platforms.DefaultSpec(), nil
	case "linux/arm64/v8":
		return ocispec.Platform{
			OS:           "linux",
			Architecture: "arm64",
			Variant:      "v8",
		}, nil
	case "linux/amd64":
		return ocispec.Platform{
			OS:           "linux",
			Architecture: "amd64",
		}, nil
	case "windows/amd64":
		return ocispec.Platform{
			OS:           "windows",
			Architecture: "amd64",
		}, nil
	default:
		return ocispec.Platform{}, errors.New("image not found")
	}
}

func (m mockPlatformReader) ReadPlatformFromConfigByImageManifest(ctx context.Context, desc ocispec.Descriptor) (ocispec.Platform, error) {
	return m.ReadPlatformFromImage(ctx, image.ID(desc.Digest))
}

//nolint:staticcheck // ignore SA1019 because we are testing deprecated field migration
func TestContainerMigrateOS(t *testing.T) {
	type Container = container.Container

	var mock mockPlatformReader

	// ImageManifest is nil for containers created with graphdrivers image store
	var graphdrivers *ocispec.Descriptor = nil

	for _, tc := range []struct {
		name     string
		ctr      Container
		expected ocispec.Platform
	}{
		{
			name: "gd pre-OS container",
			ctr: Container{
				ImageManifest: graphdrivers,
				OS:            "",
			},
			expected: platforms.DefaultSpec(),
		},
		{
			name: "gd with linux arm64 image",
			ctr: Container{
				ImageManifest: graphdrivers,
				ImageID:       "linux/arm64/v8",
				OS:            "linux",
			},
			expected: ocispec.Platform{
				OS:           "linux",
				Architecture: "arm64",
				Variant:      "v8",
			},
		},
		{
			name: "gd with windows image",
			ctr: Container{
				ImageManifest: graphdrivers,
				ImageID:       "windows/amd64",
				OS:            "windows",
			},
			expected: ocispec.Platform{
				OS:           "windows",
				Architecture: "amd64",
			},
		},
		{
			name: "gd with an image thats no longer available",
			ctr: Container{
				ImageManifest: graphdrivers,
				ImageID:       "notfound",
				OS:            "linux",
			},
			expected: platforms.Platform{
				OS: "linux",
			},
		},
		{
			name: "c8d with linux arm64 image",
			ctr: Container{
				ImageManifest: &ocispec.Descriptor{
					Digest: "linux/arm64/v8",
				},
				OS:      "linux",
				ImageID: "linux/arm64/v8",
			},
			expected: ocispec.Platform{
				OS:           "linux",
				Architecture: "arm64",
				Variant:      "v8",
			},
		},
		{
			name: "c8d with an image thats no longer available",
			ctr: Container{
				ImageManifest: &ocispec.Descriptor{
					Digest: "notfound",
				},
				OS:      "linux",
				ImageID: "notfound",
			},
			expected: platforms.Platform{
				OS: "linux",
			},
		},
		{
			name: "c8d with ImageManifest that is no longer available",
			ctr: Container{
				ImageManifest: &ocispec.Descriptor{
					Digest: "notfound",
				},
				OS:      "linux",
				ImageID: "multiplatform",
			},
			// Note: This might produce unexpected results, because if the platform-specific manifest
			// is not available, and the ImageID points to a multi-platform image, then GetImage will
			// return any available platform with host platform being the priority.
			// So it will just use whatever platform is returned by GetImage (docker image inspect).
			expected: platforms.DefaultSpec(),
		},
		{
			name: "ImageManifest has priority over ImageID migration",
			ctr: Container{
				ImageManifest: &ocispec.Descriptor{
					Digest: "linux/arm64/v8",
				},
				OS:      "linux",
				ImageID: "linux/amd64",
			},
			expected: ocispec.Platform{
				OS:           "linux",
				Architecture: "arm64",
				Variant:      "v8",
			},
		},
	} {
		ctr := tc.ctr
		t.Run(tc.name, func(t *testing.T) {
			migrateContainerOS(context.Background(), mock, &ctr)

			assert.DeepEqual(t, tc.expected, ctr.ImagePlatform)
		})
	}

}
