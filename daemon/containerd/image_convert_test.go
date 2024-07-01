package containerd

import (
	"context"
	"testing"

	"github.com/containerd/containerd/content/local"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/platforms"
	"github.com/containerd/log/logtest"
	"github.com/distribution/reference"
	imagetypes "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/internal/testutils/specialimage"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestImageConvert(t *testing.T) {
	ctx := namespaces.WithNamespace(context.TODO(), "testing")

	blobsDir := t.TempDir()

	twoplatform, err := specialimage.TwoPlatform(blobsDir)
	assert.NilError(t, err)

	dstRef := func(s string) reference.NamedTagged {
		ref, err := reference.ParseNormalizedNamed(s)
		assert.NilError(t, err)
		return reference.TagNameOnly(ref).(reference.NamedTagged)
	}

	cs, err := local.NewStore(blobsDir)
	assert.NilError(t, err)

	for _, tc := range []struct {
		name  string
		src   *ocispec.Index
		err   error
		opts  imagetypes.ConvertOptions
		check func(t *testing.T, src, dst images.Image)
	}{
		{
			src:  twoplatform,
			name: "twoplatform-to-noop",
			opts: imagetypes.ConvertOptions{},
			err:  errConvertNoop,
		},
		{
			src:  twoplatform,
			name: "twoplatform-to-available",
			opts: imagetypes.ConvertOptions{
				OnlyAvailablePlatforms: true,
			},
			// All are available so should be the same
			check: func(t *testing.T, src, dst images.Image) {
				assert.Check(t, is.Equal(src.Target.Digest, dst.Target.Digest))
			},
		},
		{
			src:  twoplatform,
			name: "twoplatform-to-one",
			opts: imagetypes.ConvertOptions{
				Platforms: []ocispec.Platform{platforms.MustParse("linux/amd64")},
			},
			check: func(t *testing.T, src, dst images.Image) {
				assert.Assert(t, src.Target.Digest != dst.Target.Digest)

				assert.Check(t, is.Equal(dst.Target.MediaType, ocispec.MediaTypeImageManifest))
				mfst, err := readManifest(ctx, cs, dst.Target)
				assert.NilError(t, err)

				var cfg ocispec.Image
				assert.NilError(t, readConfig(ctx, cs, mfst.Config, &cfg))

				assert.Check(t, is.Equal(cfg.Platform.OS, "linux"))
				assert.Check(t, is.Equal(cfg.Platform.Architecture, "amd64"))
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := logtest.WithT(ctx, t)
			service := fakeImageService(t, ctx, cs)

			srcImg, err := service.images.Create(ctx, imagesFromIndex(tc.src)[0])
			assert.NilError(t, err)

			dst := dstRef(tc.name)
			err = service.ImageConvert(ctx, srcImg.Name, []reference.NamedTagged{dst}, tc.opts)

			if tc.err != nil {
				assert.ErrorIs(t, err, tc.err)
				return
			} else {
				assert.NilError(t, err)
			}

			newImg, err := service.images.Get(ctx, dst.String())
			assert.NilError(t, err)
			tc.check(t, srcImg, newImg)
		})
	}

}
