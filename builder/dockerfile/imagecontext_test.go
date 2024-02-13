package dockerfile // import "github.com/docker/docker/builder/dockerfile"

import (
	"context"
	"fmt"
	"runtime"
	"testing"

	"github.com/containerd/containerd/platforms"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/image"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
)

func getMockImageSource(getImageImage builder.Image, getImageLayer builder.ROLayer, getImageError error) *imageSources {
	return &imageSources{
		byImageID: make(map[string]*imageMount),
		mounts:    []*imageMount{},
		getImage: func(_ context.Context, name string, localOnly bool, platform *ocispec.Platform) (builder.Image, builder.ROLayer, error) {
			return getImageImage, getImageLayer, getImageError
		},
	}
}

func getMockImageMount() *imageMount {
	return &imageMount{
		image: nil,
		layer: nil,
	}
}

func TestAddScratchImageAddsToMounts(t *testing.T) {
	is := getMockImageSource(nil, nil, fmt.Errorf("getImage is not implemented"))
	im := getMockImageMount()

	// We are testing whether the imageMount is added to is.mounts
	assert.Equal(t, len(is.mounts), 0)
	is.Add(im, nil)
	assert.Equal(t, len(is.mounts), 1)
}

func TestAddFromScratchPopulatesPlatform(t *testing.T) {
	is := getMockImageSource(nil, nil, fmt.Errorf("getImage is not implemented"))

	platforms := []*ocispec.Platform{
		{
			OS:           "linux",
			Architecture: "amd64",
		},
		{
			OS:           "linux",
			Architecture: "arm64",
			Variant:      "v8",
		},
	}

	for i, platform := range platforms {
		im := getMockImageMount()
		assert.Equal(t, len(is.mounts), i)
		is.Add(im, platform)
		image, ok := im.image.(*image.Image)
		assert.Assert(t, ok)
		assert.Equal(t, image.OS, platform.OS)
		assert.Equal(t, image.Architecture, platform.Architecture)
		assert.Equal(t, image.Variant, platform.Variant)
	}
}

func TestAddFromScratchDoesNotModifyArgPlatform(t *testing.T) {
	is := getMockImageSource(nil, nil, fmt.Errorf("getImage is not implemented"))
	im := getMockImageMount()

	platform := &ocispec.Platform{
		OS:           "windows",
		Architecture: "amd64",
	}
	argPlatform := *platform

	is.Add(im, &argPlatform)
	// The way the code is written right now, this test
	// really doesn't do much except on Windows.
	assert.DeepEqual(t, *platform, argPlatform)
}

func TestAddFromScratchPopulatesPlatformIfNil(t *testing.T) {
	is := getMockImageSource(nil, nil, fmt.Errorf("getImage is not implemented"))
	im := getMockImageMount()
	is.Add(im, nil)
	image, ok := im.image.(*image.Image)
	assert.Assert(t, ok)

	expectedPlatform := platforms.DefaultSpec()
	if runtime.GOOS == "windows" {
		expectedPlatform.OS = "linux"
	}
	assert.Equal(t, expectedPlatform.OS, image.OS)
	assert.Equal(t, expectedPlatform.Architecture, image.Architecture)
	assert.Equal(t, expectedPlatform.Variant, image.Variant)
}

func TestImageSourceGetAddsToMounts(t *testing.T) {
	is := getMockImageSource(nil, nil, nil)
	ctx := context.Background()
	_, err := is.Get(ctx, "test", false, nil)
	assert.NilError(t, err)
	assert.Equal(t, len(is.mounts), 1)
}
