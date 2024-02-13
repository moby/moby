package image

import (
	"archive/tar"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/cpuguy83/tar2go"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/integration/internal/build"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/internal/testutils"
	"github.com/docker/docker/internal/testutils/specialimage"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/testutil/fakecontext"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

type imageSaveManifestEntry struct {
	Config   string
	RepoTags []string
	Layers   []string
}

func tarIndexFS(t *testing.T, rdr io.Reader) fs.FS {
	t.Helper()

	dir := t.TempDir()

	f, err := os.Create(filepath.Join(dir, "image.tar"))
	assert.NilError(t, err)

	// Do not close at the end of this function otherwise the indexer won't work
	t.Cleanup(func() { f.Close() })

	_, err = io.Copy(f, rdr)
	assert.NilError(t, err)

	return tar2go.NewIndex(f).FS()
}

func TestSaveCheckTimes(t *testing.T) {
	ctx := setupTest(t)

	t.Parallel()
	client := testEnv.APIClient()

	const repoName = "busybox:latest"
	img, _, err := client.ImageInspectWithRaw(ctx, repoName)
	assert.NilError(t, err)

	rdr, err := client.ImageSave(ctx, []string{repoName})
	assert.NilError(t, err)

	tarfs := tarIndexFS(t, rdr)

	dt, err := fs.ReadFile(tarfs, "manifest.json")
	assert.NilError(t, err)

	var ls []imageSaveManifestEntry
	assert.NilError(t, json.Unmarshal(dt, &ls))
	assert.Assert(t, cmp.Len(ls, 1))

	info, err := fs.Stat(tarfs, ls[0].Config)
	assert.NilError(t, err)

	created, err := time.Parse(time.RFC3339, img.Created)
	assert.NilError(t, err)

	if testEnv.UsingSnapshotter() {
		// containerd archive export sets the mod time to zero.
		assert.Check(t, is.Equal(info.ModTime(), time.Unix(0, 0)))
	} else {
		assert.Check(t, is.Equal(info.ModTime().Format(time.RFC3339), created.Format(time.RFC3339)))
	}
}

// Regression test for https://github.com/moby/moby/issues/47065
func TestSaveOCI(t *testing.T) {
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.44"), "OCI layout support was introduced in v25")

	ctx := setupTest(t)
	client := testEnv.APIClient()

	const busybox = "busybox:latest"
	inspectBusybox, _, err := client.ImageInspectWithRaw(ctx, busybox)
	assert.NilError(t, err)

	type testCase struct {
		image                 string
		expectedOCIRef        string
		expectedContainerdRef string
	}

	testCases := []testCase{
		// Busybox by tagged name
		testCase{image: busybox, expectedContainerdRef: "docker.io/library/busybox:latest", expectedOCIRef: "latest"},

		// Busybox by ID
		testCase{image: inspectBusybox.ID},
	}

	if testEnv.DaemonInfo.OSType != "windows" {
		multiLayerImage := specialimage.Load(ctx, t, client, specialimage.MultiLayer)
		// Multi-layer image
		testCases = append(testCases, testCase{image: multiLayerImage, expectedContainerdRef: "docker.io/library/multilayer:latest", expectedOCIRef: "latest"})

	}

	// Busybox frozen image will have empty RepoDigests when loaded into the
	// graphdriver image store so we can't use it.
	// This will work with the containerd image store though.
	if len(inspectBusybox.RepoDigests) > 0 {
		// Digested reference
		testCases = append(testCases, testCase{
			image: inspectBusybox.RepoDigests[0],
		})
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.image, func(t *testing.T) {
			// Get information about the original image.
			inspect, _, err := client.ImageInspectWithRaw(ctx, tc.image)
			assert.NilError(t, err)

			rdr, err := client.ImageSave(ctx, []string{tc.image})
			assert.NilError(t, err)
			defer rdr.Close()

			tarfs := tarIndexFS(t, rdr)

			indexData, err := fs.ReadFile(tarfs, "index.json")
			assert.NilError(t, err, "failed to read index.json")

			var index ocispec.Index
			assert.NilError(t, json.Unmarshal(indexData, &index), "failed to unmarshal index.json")

			// All test images are single-platform, so they should have only one manifest.
			assert.Assert(t, is.Len(index.Manifests, 1))

			manifestData, err := fs.ReadFile(tarfs, "blobs/sha256/"+index.Manifests[0].Digest.Encoded())
			assert.NilError(t, err)

			var manifest ocispec.Manifest
			assert.NilError(t, json.Unmarshal(manifestData, &manifest))

			t.Run("Manifest", func(t *testing.T) {
				assert.Check(t, is.Len(manifest.Layers, len(inspect.RootFS.Layers)))

				var digests []string
				// Check if layers referenced by the manifest exist in the archive
				// and match the layers from the original image.
				for _, l := range manifest.Layers {
					layerPath := "blobs/sha256/" + l.Digest.Encoded()
					stat, err := fs.Stat(tarfs, layerPath)
					assert.NilError(t, err)

					assert.Check(t, is.Equal(l.Size, stat.Size()))

					f, err := tarfs.Open(layerPath)
					assert.NilError(t, err)

					layerDigest, err := testutils.UncompressedTarDigest(f)
					f.Close()

					assert.NilError(t, err)

					digests = append(digests, layerDigest.String())
				}

				assert.Check(t, is.DeepEqual(digests, inspect.RootFS.Layers))
			})

			t.Run("Config", func(t *testing.T) {
				configData, err := fs.ReadFile(tarfs, "blobs/sha256/"+manifest.Config.Digest.Encoded())
				assert.NilError(t, err)

				var config ocispec.Image
				assert.NilError(t, json.Unmarshal(configData, &config))

				var diffIDs []string
				for _, l := range config.RootFS.DiffIDs {
					diffIDs = append(diffIDs, l.String())
				}

				assert.Check(t, is.DeepEqual(diffIDs, inspect.RootFS.Layers))
			})

			t.Run("Containerd image name", func(t *testing.T) {
				assert.Check(t, is.Equal(index.Manifests[0].Annotations["io.containerd.image.name"], tc.expectedContainerdRef))
			})

			t.Run("OCI reference tag", func(t *testing.T) {
				assert.Check(t, is.Equal(index.Manifests[0].Annotations["org.opencontainers.image.ref.name"], tc.expectedOCIRef))
			})

		})
	}
}

func TestSaveRepoWithMultipleImages(t *testing.T) {
	ctx := setupTest(t)
	client := testEnv.APIClient()

	makeImage := func(from string, tag string) string {
		id := container.Create(ctx, t, client, func(cfg *container.TestContainerConfig) {
			cfg.Config.Image = from
			cfg.Config.Cmd = []string{"true"}
		})

		res, err := client.ContainerCommit(ctx, id, containertypes.CommitOptions{Reference: tag})
		assert.NilError(t, err)

		err = client.ContainerRemove(ctx, id, containertypes.RemoveOptions{Force: true})
		assert.NilError(t, err)

		return res.ID
	}

	busyboxImg, _, err := client.ImageInspectWithRaw(ctx, "busybox:latest")
	assert.NilError(t, err)

	const repoName = "foobar-save-multi-images-test"
	const tagFoo = repoName + ":foo"
	const tagBar = repoName + ":bar"

	idFoo := makeImage("busybox:latest", tagFoo)
	idBar := makeImage("busybox:latest", tagBar)
	idBusybox := busyboxImg.ID

	rdr, err := client.ImageSave(ctx, []string{repoName, "busybox:latest"})
	assert.NilError(t, err)
	defer rdr.Close()

	tarfs := tarIndexFS(t, rdr)

	dt, err := fs.ReadFile(tarfs, "manifest.json")
	assert.NilError(t, err)

	var mfstLs []imageSaveManifestEntry
	assert.NilError(t, json.Unmarshal(dt, &mfstLs))

	actual := make([]string, 0, len(mfstLs))
	for _, m := range mfstLs {
		actual = append(actual, strings.TrimPrefix(m.Config, "blobs/sha256/"))
		// make sure the blob actually exists
		_, err = fs.Stat(tarfs, m.Config)
		assert.Check(t, err)
	}

	expected := []string{idBusybox, idFoo, idBar}
	// prefixes are not in tar
	for i := range expected {
		expected[i] = digest.Digest(expected[i]).Encoded()
	}

	// With snapshotters, ID of the image is the ID of the manifest/index
	// With graphdrivers, ID of the image is the ID of the image config
	if testEnv.UsingSnapshotter() {
		// ID of image won't match the Config ID from manifest.json
		// Just check if manifests exist in blobs
		for _, blob := range expected {
			_, err = fs.Stat(tarfs, "blobs/sha256/"+blob)
			assert.Check(t, err)
		}
	} else {
		sort.Strings(actual)
		sort.Strings(expected)
		assert.Assert(t, cmp.DeepEqual(actual, expected), "archive does not contains the right layers: got %v, expected %v", actual, expected)
	}
}

func TestSaveDirectoryPermissions(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "Test is looking at linux specific details")

	ctx := setupTest(t)
	client := testEnv.APIClient()

	layerEntries := []string{"opt/", "opt/a/", "opt/a/b/", "opt/a/b/c"}
	layerEntriesAUFS := []string{"./", ".wh..wh.aufs", ".wh..wh.orph/", ".wh..wh.plnk/", "opt/", "opt/a/", "opt/a/b/", "opt/a/b/c"}

	dockerfile := `FROM busybox
RUN adduser -D user && mkdir -p /opt/a/b && chown -R user:user /opt/a
RUN touch /opt/a/b/c && chown user:user /opt/a/b/c`

	imgID := build.Do(ctx, t, client, fakecontext.New(t, t.TempDir(), fakecontext.WithDockerfile(dockerfile)))

	rdr, err := client.ImageSave(ctx, []string{imgID})
	assert.NilError(t, err)
	defer rdr.Close()

	tarfs := tarIndexFS(t, rdr)

	dt, err := fs.ReadFile(tarfs, "manifest.json")
	assert.NilError(t, err)

	var mfstLs []imageSaveManifestEntry
	assert.NilError(t, json.Unmarshal(dt, &mfstLs))

	var found bool

	for _, p := range mfstLs[0].Layers {
		var entriesSansDev []string

		f, err := tarfs.Open(p)
		assert.NilError(t, err)

		entries, err := listTar(f)
		f.Close()
		assert.NilError(t, err)

		for _, e := range entries {
			if !strings.Contains(e, "dev/") {
				entriesSansDev = append(entriesSansDev, e)
			}
		}
		assert.NilError(t, err, "encountered error while listing tar entries: %s", err)

		if reflect.DeepEqual(entriesSansDev, layerEntries) || reflect.DeepEqual(entriesSansDev, layerEntriesAUFS) {
			found = true
			break
		}
	}

	assert.Assert(t, found, "failed to find the layer with the right content listing")
}

func listTar(f io.Reader) ([]string, error) {
	// If using the containerd snapshotter, the tar file may be compressed
	dec, err := archive.DecompressStream(f)
	if err != nil {
		return nil, err
	}
	defer dec.Close()

	tr := tar.NewReader(dec)
	var entries []string

	for {
		th, err := tr.Next()
		if err == io.EOF {
			// end of tar archive
			return entries, nil
		}
		if err != nil {
			return entries, err
		}
		entries = append(entries, th.Name)
	}
}
