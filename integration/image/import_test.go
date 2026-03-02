package image

import (
	"archive/tar"
	"bytes"
	"io"
	"net/http/httptest"
	"runtime"
	"strconv"
	"strings"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/platforms"
	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/internal/testutil"
	"github.com/moby/moby/v2/internal/testutil/daemon"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
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

	apiClient := d.NewClientT(t)

	// Construct an empty tar archive with about 8GB of junk padding at the
	// end. This should not cause any crashes (the padding should be mostly
	// ignored).
	var tarBuffer bytes.Buffer

	tw := tar.NewWriter(&tarBuffer)
	err := tw.Close()
	assert.NilError(t, err)
	imageRdr := io.MultiReader(&tarBuffer, io.LimitReader(testutil.DevZero, 8*1024*1024*1024))
	reference := strings.ToLower(t.Name()) + ":v42"

	_, err = apiClient.ImageImport(ctx,
		client.ImageImportSource{Source: imageRdr, SourceName: "-"},
		reference,
		client.ImageImportOptions{})
	assert.NilError(t, err)
}

func TestImportWithCustomPlatform(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "TODO enable on windows")

	ctx := setupTest(t)

	apiClient := testEnv.APIClient()

	// Construct an empty tar archive.
	var tarBuffer bytes.Buffer

	tw := tar.NewWriter(&tarBuffer)
	err := tw.Close()
	assert.NilError(t, err)
	imageRdr := io.MultiReader(&tarBuffer, io.LimitReader(testutil.DevZero, 0))

	tests := []struct {
		name     string
		platform ocispec.Platform
		expected ocispec.Platform
	}{
		{
			expected: ocispec.Platform{
				OS:           runtime.GOOS,
				Architecture: runtime.GOARCH, // this may fail on armhf due to normalization?
			},
		},
		{
			platform: ocispec.Platform{
				OS: runtime.GOOS,
			},
			expected: ocispec.Platform{
				OS:           runtime.GOOS,
				Architecture: runtime.GOARCH, // this may fail on armhf due to normalization?
			},
		},
		{
			platform: ocispec.Platform{
				OS:           runtime.GOOS,
				Architecture: "sparc64",
			},
			expected: ocispec.Platform{
				OS:           runtime.GOOS,
				Architecture: "sparc64",
			},
		},
	}

	for i, tc := range tests {
		t.Run(platforms.Format(tc.platform), func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)
			reference := "import-with-platform:tc-" + strconv.Itoa(i)

			_, err = apiClient.ImageImport(ctx,
				client.ImageImportSource{Source: imageRdr, SourceName: "-"},
				reference,
				client.ImageImportOptions{Platform: tc.platform})
			assert.NilError(t, err)

			inspect, err := apiClient.ImageInspect(ctx, reference)
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

	apiClient := testEnv.APIClient()

	// Construct an empty tar archive.
	var tarBuffer bytes.Buffer

	tw := tar.NewWriter(&tarBuffer)
	err := tw.Close()
	assert.NilError(t, err)
	imageRdr := io.MultiReader(&tarBuffer, io.LimitReader(testutil.DevZero, 0))

	tests := []struct {
		name        string
		platform    ocispec.Platform
		expectedErr string
	}{
		{
			name: "whitespace-only platform",
			platform: ocispec.Platform{
				OS: "       ",
			},
			expectedErr: "is an invalid OS component",
		},
		{
			name: "valid, but unsupported os",
			platform: ocispec.Platform{
				OS: "macos",
			},
			expectedErr: "operating system is not supported",
		},
		{
			name: "valid, but unsupported os/arch",
			platform: ocispec.Platform{
				OS:           "macos",
				Architecture: "arm64",
			},
			expectedErr: "operating system is not supported",
		},
		{
			name: "valid, but unsupported os",
			// TODO: platforms.Normalize() only validates os or arch if a single component is passed,
			//       but ignores unknown os/arch in other cases. See:
			//       https://github.com/containerd/containerd/blob/7d4891783aac5adf6cd83f657852574a71875631/platforms/platforms.go#L183-L209
			platform: ocispec.Platform{
				OS: "nintendo64",
			},
			expectedErr: "unknown operating system or architecture",
		},
	}

	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)
			reference := "import-with-platform:tc-" + strconv.Itoa(i)
			_, err = apiClient.ImageImport(ctx,
				client.ImageImportSource{Source: imageRdr, SourceName: "-"},
				reference,
				client.ImageImportOptions{Platform: tc.platform})

			assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
			assert.Check(t, is.ErrorContains(err, tc.expectedErr))
		})
	}
}

func TestImageImportBadSrc(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	skip.If(t, testEnv.IsRootless, "rootless daemon cannot access the test's HTTP server in the host's netns")

	server := httptest.NewServer(nil)
	defer server.Close()

	trimmedHTTP := strings.TrimPrefix(server.URL, "http://")

	tests := []struct {
		name      string
		fromSrc   string
		expectErr func(error) bool
	}{
		{
			name:      "missing file via full URL",
			fromSrc:   server.URL + "/nofile.tar",
			expectErr: cerrdefs.IsNotFound,
		},
		{
			name:      "missing file via trimmed URL",
			fromSrc:   trimmedHTTP + "/nofile.tar",
			expectErr: cerrdefs.IsNotFound,
		},
		{
			name:      "encoded path via trimmed URL",
			fromSrc:   trimmedHTTP + "/%2Fdata%2Ffile.tar",
			expectErr: cerrdefs.IsNotFound,
		},
		{
			name:      "encoded absolute path",
			fromSrc:   "%2Fdata%2Ffile.tar",
			expectErr: cerrdefs.IsInvalidArgument,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := apiClient.ImageImport(ctx,
				client.ImageImportSource{
					SourceName: tc.fromSrc,
				},
				"import-bad-src:test",
				client.ImageImportOptions{},
			)

			assert.Check(t, tc.expectErr(err))
		})
	}
}
