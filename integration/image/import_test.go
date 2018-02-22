package image // import "github.com/docker/docker/integration/image"

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"runtime"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/integration/internal/request"
	"github.com/docker/docker/internal/testutil"
)

// Ensure we don't regress on CVE-2017-14992.
func TestImportExtremelyLargeImageWorks(t *testing.T) {
	if runtime.GOARCH == "arm64" {
		t.Skip("effective test will be time out")
	}

	client := request.NewAPIClient(t)

	// Construct an empty tar archive with about 8GB of junk padding at the
	// end. This should not cause any crashes (the padding should be mostly
	// ignored).
	var tarBuffer bytes.Buffer

	tw := tar.NewWriter(&tarBuffer)
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	imageRdr := io.MultiReader(&tarBuffer, io.LimitReader(testutil.DevZero, 8*1024*1024*1024))

	_, err := client.ImageImport(context.Background(),
		types.ImageImportSource{Source: imageRdr, SourceName: "-"},
		"test1234:v42",
		types.ImageImportOptions{})
	if err != nil {
		t.Fatal(err)
	}
}
