package specialimage

import (
	"os"
	"path/filepath"
	"strings"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	danglingImageManifestDigest = "sha256:16d365089e5c10e1673ee82ab5bba38ade9b763296ad918bd24b42a1156c5456" // #nosec G101 -- ignoring: Potential hardcoded credentials (gosec)
	danglingImageConfigDigest   = "sha256:0df1207206e5288f4a989a2f13d1f5b3c4e70467702c1d5d21dfc9f002b7bd43" // #nosec G101 -- ignoring: Potential hardcoded credentials (gosec)
)

// Dangling creates an image with no layers and no tag.
// It also has an extra org.mobyproject.test.specialimage=1 label set.
// Layout: OCI.
func Dangling(dir string) (*ocispec.Index, error) {
	if err := os.WriteFile(filepath.Join(dir, "index.json"), []byte(`{"schemaVersion":2,"manifests":[{"mediaType":"application/vnd.docker.distribution.manifest.v2+json","digest":"sha256:16d365089e5c10e1673ee82ab5bba38ade9b763296ad918bd24b42a1156c5456","size":264,"annotations":{"org.opencontainers.image.created":"2023-05-19T08:00:44Z"},"platform":{"architecture":"amd64","os":"linux"}}]}`), 0o644); err != nil {
		return nil, err
	}

	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`[{"Config":"blobs/sha256/0df1207206e5288f4a989a2f13d1f5b3c4e70467702c1d5d21dfc9f002b7bd43","RepoTags":null,"Layers":null}]`), 0o644); err != nil {
		return nil, err
	}

	if err := os.Mkdir(filepath.Join(dir, "blobs"), 0o755); err != nil {
		return nil, err
	}

	blobsDir := filepath.Join(dir, "blobs", "sha256")
	if err := os.Mkdir(blobsDir, 0o755); err != nil {
		return nil, err
	}

	if err := os.WriteFile(filepath.Join(blobsDir, strings.TrimPrefix(danglingImageManifestDigest, "sha256:")), []byte(`{"schemaVersion":2,"mediaType":"application/vnd.docker.distribution.manifest.v2+json","config":{"mediaType":"application/vnd.docker.container.image.v1+json","digest":"sha256:0df1207206e5288f4a989a2f13d1f5b3c4e70467702c1d5d21dfc9f002b7bd43","size":390},"layers":[]}`), 0o644); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(blobsDir, strings.TrimPrefix(danglingImageConfigDigest, "sha256:")), []byte(`{"architecture":"amd64","config":{"Env":["PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"],"WorkingDir":"/","Labels":{"org.mobyproject.test.specialimage":"1"},"OnBuild":null},"created":null,"history":[{"created_by":"LABEL org.mobyproject.test.specialimage=1","comment":"buildkit.dockerfile.v0","empty_layer":true}],"os":"linux","rootfs":{"type":"layers","diff_ids":null}}`), 0o644); err != nil {
		return nil, err
	}

	return nil, nil
}
