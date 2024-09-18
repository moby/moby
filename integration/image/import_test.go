package image // import "github.com/docker/docker/integration/image"

import (
	"archive/tar"
	"bytes"
	"io"
	"runtime"
	"strconv"
	"strings"
	"testing"

	imagetypes "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image"
	"github.com/docker/docker/testutil"
	"github.com/docker/docker/testutil/daemon"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

// Ensure we don't regress on CVE-2017-14992.
func TestImportExtremelyLargeImageWorks(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot run daemon when remote daemon")
	skip.If(t, runtime.GOARCH == "arm64", "effective test will be time out")
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "TODO enable on windows")
	t.Parallel()

	ctx := testutil.StartSpan(baseContext, t)

	// Spin up a new daemon, so that we can run this test in parallel (it's a slow test)
	d := daemon.New(t)
	d.Start(t, "--iptables=false", "--ip6tables=false")
	defer d.Stop(t)

	client := d.NewClientT(t)

	// Construct an empty tar archive with about 8GB of junk padding at the
	// end. This should not cause any crashes (the padding should be mostly
	// ignored).
	var tarBuffer bytes.Buffer

	tw := tar.NewWriter(&tarBuffer)
	err := tw.Close()
	assert.NilError(t, err)
	imageRdr := io.MultiReader(&tarBuffer, io.LimitReader(testutil.DevZero, 8*1024*1024*1024))
	reference := strings.ToLower(t.Name()) + ":v42"

	_, err = client.ImageImport(ctx,
		imagetypes.ImportSource{Source: imageRdr, SourceName: "-"},
		reference,
		imagetypes.ImportOptions{})
	assert.NilError(t, err)
}

func TestImportWithCustomPlatform(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "TODO enable on windows")

	ctx := setupTest(t)

	client := testEnv.APIClient()

	// Construct an empty tar archive.
	var tarBuffer bytes.Buffer

	tw := tar.NewWriter(&tarBuffer)
	err := tw.Close()
	assert.NilError(t, err)
	imageRdr := io.MultiReader(&tarBuffer, io.LimitReader(testutil.DevZero, 0))

	tests := []struct {
		name     string
		platform string
		expected image.V1Image
	}{
		{
			platform: "",
			expected: image.V1Image{
				OS:           runtime.GOOS,
				Architecture: runtime.GOARCH, // this may fail on armhf due to normalization?
			},
		},
		{
			platform: runtime.GOOS,
			expected: image.V1Image{
				OS:           runtime.GOOS,
				Architecture: runtime.GOARCH, // this may fail on armhf due to normalization?
			},
		},
		{
			platform: strings.ToUpper(runtime.GOOS),
			expected: image.V1Image{
				OS:           runtime.GOOS,
				Architecture: runtime.GOARCH, // this may fail on armhf due to normalization?
			},
		},
		{
			platform: runtime.GOOS + "/sparc64",
			expected: image.V1Image{
				OS:           runtime.GOOS,
				Architecture: "sparc64",
			},
		},
	}

	for i, tc := range tests {
		tc := tc
		t.Run(tc.platform, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)
			reference := "import-with-platform:tc-" + strconv.Itoa(i)

			_, err = client.ImageImport(ctx,
				imagetypes.ImportSource{Source: imageRdr, SourceName: "-"},
				reference,
				imagetypes.ImportOptions{Platform: tc.platform})
			assert.NilError(t, err)

			inspect, _, err := client.ImageInspectWithRaw(ctx, reference)
			assert.NilError(t, err)
			assert.Equal(t, inspect.Os, tc.expected.OS)
			assert.Equal(t, inspect.Architecture, tc.expected.Architecture)
		})
	}
}

func TestImportWithCustomPlatformReject(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "TODO enable on windows")
	skip.If(t, testEnv.UsingSnapshotter(), "we support importing images/other platforms w/ containerd image store")

	ctx := setupTest(t)

	client := testEnv.APIClient()

	// Construct an empty tar archive.
	var tarBuffer bytes.Buffer

	tw := tar.NewWriter(&tarBuffer)
	err := tw.Close()
	assert.NilError(t, err)
	imageRdr := io.MultiReader(&tarBuffer, io.LimitReader(testutil.DevZero, 0))

	tests := []struct {
		name        string
		platform    string
		expected    image.V1Image
		expectedErr string
	}{
		{
			platform:    "       ",
			expectedErr: "is an invalid OS component",
		},
		{
			platform:    "/",
			expectedErr: "is an invalid OS component",
		},
		{
			platform:    "macos",
			expectedErr: "operating system is not supported",
		},
		{
			platform:    "macos/arm64",
			expectedErr: "operating system is not supported",
		},
		{
			// TODO: platforms.Normalize() only validates os or arch if a single component is passed,
			//       but ignores unknown os/arch in other cases. See:
			//       https://github.com/containerd/containerd/blob/7d4891783aac5adf6cd83f657852574a71875631/platforms/platforms.go#L183-L209
			platform:    "nintendo64",
			expectedErr: "unknown operating system or architecture",
		},
	}

	for i, tc := range tests {
		tc := tc
		t.Run(tc.platform, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)
			reference := "import-with-platform:tc-" + strconv.Itoa(i)
			_, err = client.ImageImport(ctx,
				imagetypes.ImportSource{Source: imageRdr, SourceName: "-"},
				reference,
				imagetypes.ImportOptions{Platform: tc.platform})

			assert.Check(t, is.ErrorType(err, errdefs.IsInvalidParameter))
			assert.Check(t, is.ErrorContains(err, tc.expectedErr))
		})
	}
}
