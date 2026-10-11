package image

import (
	"archive/tar"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/remotes/docker"
	"github.com/containerd/containerd/v2/plugins/content/local"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/platforms"
	"github.com/cpuguy83/tar2go"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/moby/go-archive/compression"
	registrytypes "github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/client"
	"github.com/moby/moby/client/pkg/versions"
	"github.com/moby/moby/v2/integration/internal/build"
	"github.com/moby/moby/v2/integration/internal/container"
	iimage "github.com/moby/moby/v2/integration/internal/image"
	"github.com/moby/moby/v2/internal/testutil"
	"github.com/moby/moby/v2/internal/testutil/daemon"
	"github.com/moby/moby/v2/internal/testutil/fakecontext"
	"github.com/moby/moby/v2/internal/testutil/registry"
	"github.com/moby/moby/v2/internal/testutil/specialimage"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
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
	t.Cleanup(func() { _ = f.Close() })

	_, err = io.Copy(f, rdr)
	assert.NilError(t, err)

	return tar2go.NewIndex(f).FS()
}

func TestSaveCheckTimes(t *testing.T) {
	ctx := setupTest(t)

	t.Parallel()
	apiClient := testEnv.APIClient()

	const repoName = "busybox:latest"
	img, err := apiClient.ImageInspect(ctx, repoName)
	assert.NilError(t, err)

	rdr, err := apiClient.ImageSave(ctx, []string{repoName})
	assert.NilError(t, err)
	defer func() { _ = rdr.Close() }()

	created, err := time.Parse(time.RFC3339, img.Created)
	assert.NilError(t, err)

	// containerd archive export sets mod times of all members to zero
	// otherwise no member should be newer than the image created date
	threshold := time.Unix(0, 0)
	if !testEnv.UsingSnapshotter() {
		threshold = created
	}

	tr := tar.NewReader(rdr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		assert.NilError(t, err)
		modtime := hdr.ModTime
		assert.Check(t, !modtime.After(threshold), "%s has modtime %s after %s", hdr.Name, modtime, threshold)
	}
}

// Regression test for https://github.com/moby/moby/issues/47065
func TestSaveOCI(t *testing.T) {
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.44"), "OCI layout support was introduced in v25")

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	const busybox = "busybox:latest"
	inspectBusybox, err := apiClient.ImageInspect(ctx, busybox)
	assert.NilError(t, err)

	type testCase struct {
		image                 string
		expectedOCIRef        string
		expectedContainerdRef string
	}

	testCases := []testCase{
		// Busybox by tagged name
		{image: busybox, expectedContainerdRef: "docker.io/library/busybox:latest", expectedOCIRef: "latest"},

		// Busybox by ID
		{image: inspectBusybox.ID},
	}

	if testEnv.DaemonInfo.OSType != "windows" {
		multiLayerImage := iimage.Load(ctx, t, apiClient, specialimage.MultiLayer)
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
		t.Run(tc.image, func(t *testing.T) {
			// Get information about the original image.
			inspect, err := apiClient.ImageInspect(ctx, tc.image)
			assert.NilError(t, err)

			rdr, err := apiClient.ImageSave(ctx, []string{tc.image})
			assert.NilError(t, err)
			defer func() { _ = rdr.Close() }()

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

					layerDigest, err := testutil.UncompressedTarDigest(f)
					_ = f.Close()

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

func TestSaveAndLoadPlatform(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "The test image is a Linux image")

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	const repoName = "alpine:latest"

	type testCase struct {
		testName                string
		containerdStoreOnly     bool
		pullPlatforms           []ocispec.Platform
		savePlatforms           []ocispec.Platform
		loadPlatforms           []ocispec.Platform
		expectedSavedPlatforms  []ocispec.Platform
		expectedLoadedPlatforms []ocispec.Platform // expected platforms to be saved, if empty, all pulled platforms are expected to be saved
	}

	testCases := []testCase{
		{
			testName:            "With no platforms specified",
			containerdStoreOnly: true,
			pullPlatforms: []ocispec.Platform{
				{OS: "linux", Architecture: "amd64"},
				{OS: "linux", Architecture: "ppc64le"},
				{OS: "linux", Architecture: "riscv64"},
				{OS: "linux", Architecture: "s390x"},
				{OS: "linux", Architecture: "arm64", Variant: "v8"},
			},
			savePlatforms: nil,
			loadPlatforms: nil,
			expectedSavedPlatforms: []ocispec.Platform{
				{OS: "linux", Architecture: "amd64"},
				{OS: "linux", Architecture: "ppc64le"},
				{OS: "linux", Architecture: "riscv64"},
				{OS: "linux", Architecture: "s390x"},
				{OS: "linux", Architecture: "arm64", Variant: "v8"},
			},
			expectedLoadedPlatforms: []ocispec.Platform{
				{OS: "linux", Architecture: "amd64"},
				{OS: "linux", Architecture: "ppc64le"},
				{OS: "linux", Architecture: "riscv64"},
				{OS: "linux", Architecture: "s390x"},
				{OS: "linux", Architecture: "arm64", Variant: "v8"},
			},
		},
		{
			testName:                "With single pulled platform",
			pullPlatforms:           []ocispec.Platform{{OS: "linux", Architecture: "amd64"}},
			savePlatforms:           []ocispec.Platform{{OS: "linux", Architecture: "amd64"}},
			loadPlatforms:           []ocispec.Platform{{OS: "linux", Architecture: "amd64"}},
			expectedSavedPlatforms:  []ocispec.Platform{{OS: "linux", Architecture: "amd64"}},
			expectedLoadedPlatforms: []ocispec.Platform{{OS: "linux", Architecture: "amd64"}},
		},
		{
			testName:            "With single platform save and load",
			containerdStoreOnly: true,
			pullPlatforms: []ocispec.Platform{
				{OS: "linux", Architecture: "amd64"},
				{OS: "linux", Architecture: "riscv64"},
				{OS: "linux", Architecture: "arm64", Variant: "v8"},
			},
			savePlatforms:           []ocispec.Platform{{OS: "linux", Architecture: "amd64"}},
			loadPlatforms:           []ocispec.Platform{{OS: "linux", Architecture: "amd64"}},
			expectedSavedPlatforms:  []ocispec.Platform{{OS: "linux", Architecture: "amd64"}},
			expectedLoadedPlatforms: []ocispec.Platform{{OS: "linux", Architecture: "amd64"}},
		},
		{
			testName:            "With multiple platforms save and load",
			containerdStoreOnly: true,
			pullPlatforms: []ocispec.Platform{
				{OS: "linux", Architecture: "amd64"},
				{OS: "linux", Architecture: "riscv64"},
				{OS: "linux", Architecture: "arm64", Variant: "v8"},
			},
			savePlatforms: []ocispec.Platform{
				{OS: "linux", Architecture: "arm64", Variant: "v8"},
				{OS: "linux", Architecture: "riscv64"},
			},
			loadPlatforms: []ocispec.Platform{
				{OS: "linux", Architecture: "arm64", Variant: "v8"},
				{OS: "linux", Architecture: "riscv64"},
			},
			expectedSavedPlatforms: []ocispec.Platform{
				{OS: "linux", Architecture: "arm64", Variant: "v8"},
				{OS: "linux", Architecture: "riscv64"},
			},
			expectedLoadedPlatforms: []ocispec.Platform{
				{OS: "linux", Architecture: "arm64", Variant: "v8"},
				{OS: "linux", Architecture: "riscv64"},
			},
		},
		{
			testName:            "With mixed platform save and load",
			containerdStoreOnly: true,
			pullPlatforms: []ocispec.Platform{
				{OS: "linux", Architecture: "amd64"},
				{OS: "linux", Architecture: "riscv64"},
				{OS: "linux", Architecture: "arm64", Variant: "v8"},
			},
			savePlatforms: []ocispec.Platform{
				{OS: "linux", Architecture: "arm64", Variant: "v8"},
				{OS: "linux", Architecture: "riscv64"},
			},
			loadPlatforms: []ocispec.Platform{
				{OS: "linux", Architecture: "riscv64"},
			},
			expectedSavedPlatforms: []ocispec.Platform{
				{OS: "linux", Architecture: "arm64", Variant: "v8"},
				{OS: "linux", Architecture: "riscv64"},
			},
			expectedLoadedPlatforms: []ocispec.Platform{
				{OS: "linux", Architecture: "riscv64"},
			},
		},
	}

	for _, tc := range testCases {
		if tc.containerdStoreOnly && !testEnv.UsingSnapshotter() {
			continue
		}
		t.Run(tc.testName, func(t *testing.T) {
			// pull the image
			for _, p := range tc.pullPlatforms {
				resp, err := apiClient.ImagePull(ctx, repoName, client.ImagePullOptions{Platforms: []ocispec.Platform{p}})
				assert.NilError(t, err)
				_, err = io.ReadAll(resp)
				resp.Close()
				assert.NilError(t, err)
			}

			// export the image
			rdr, err := apiClient.ImageSave(ctx, []string{repoName}, client.ImageSaveWithPlatforms(tc.savePlatforms...))
			assert.NilError(t, err)

			// remove the pulled image
			_, err = apiClient.ImageRemove(ctx, repoName, client.ImageRemoveOptions{})
			assert.NilError(t, err)

			// load the full exported image (all platforms in it)
			resp, err := apiClient.ImageLoad(ctx, rdr)
			assert.NilError(t, err)
			_, _ = io.Copy(io.Discard, resp)
			_ = resp.Close()
			_ = rdr.Close()

			// verify the loaded image has all the expected platforms
			for _, p := range tc.expectedSavedPlatforms {
				inspectResponse, err := apiClient.ImageInspect(ctx, repoName, client.ImageInspectWithPlatform(&p))
				assert.NilError(t, err)
				assert.Check(t, is.Equal(inspectResponse.Os, p.OS))
				assert.Check(t, is.Equal(inspectResponse.Architecture, p.Architecture))
			}

			// remove the loaded image
			_, err = apiClient.ImageRemove(ctx, repoName, client.ImageRemoveOptions{})
			assert.NilError(t, err)

			// pull the image again (start fresh)
			for _, p := range tc.pullPlatforms {
				pullRes, err := apiClient.ImagePull(ctx, repoName, client.ImagePullOptions{Platforms: []ocispec.Platform{p}})
				assert.NilError(t, err)
				_, err = io.ReadAll(pullRes)
				_ = pullRes.Close()
				assert.NilError(t, err)
			}

			// export the image
			rdr, err = apiClient.ImageSave(ctx, []string{repoName}, client.ImageSaveWithPlatforms(tc.savePlatforms...))
			assert.NilError(t, err)

			// remove the pulled image
			_, err = apiClient.ImageRemove(ctx, repoName, client.ImageRemoveOptions{})
			assert.NilError(t, err)

			// load the exported image on the specified platforms only
			resp, err = apiClient.ImageLoad(ctx, rdr, client.ImageLoadWithPlatforms(tc.loadPlatforms...))
			assert.NilError(t, err)
			_, _ = io.Copy(io.Discard, resp)
			_ = resp.Close()
			_ = rdr.Close()

			// verify the image was loaded for the specified platforms
			for _, p := range tc.expectedLoadedPlatforms {
				inspectResponse, err := apiClient.ImageInspect(ctx, repoName, client.ImageInspectWithPlatform(&p))
				assert.NilError(t, err)
				assert.Check(t, is.Equal(inspectResponse.Os, p.OS))
				assert.Check(t, is.Equal(inspectResponse.Architecture, p.Architecture))
			}
		})
	}
}

func TestSaveRepoWithMultipleImages(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	makeImage := func(from string, tag string) string {
		id := container.Create(ctx, t, apiClient, func(cfg *container.TestContainerConfig) {
			cfg.Config.Image = from
			cfg.Config.Cmd = []string{"true"}
		})

		res, err := apiClient.ContainerCommit(ctx, id, client.ContainerCommitOptions{Reference: tag})
		assert.NilError(t, err)

		_, err = apiClient.ContainerRemove(ctx, id, client.ContainerRemoveOptions{Force: true})
		assert.NilError(t, err)

		return res.ID
	}

	busyboxImg, err := apiClient.ImageInspect(ctx, "busybox:latest")
	assert.NilError(t, err)

	const repoName = "foobar-save-multi-images-test"
	const tagFoo = repoName + ":foo"
	const tagBar = repoName + ":bar"

	idFoo := makeImage("busybox:latest", tagFoo)
	idBar := makeImage("busybox:latest", tagBar)
	idBusybox := busyboxImg.ID

	rdr, err := apiClient.ImageSave(ctx, []string{repoName, "busybox:latest"})
	assert.NilError(t, err)
	defer func() { _ = rdr.Close() }()

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
		assert.Assert(t, is.DeepEqual(actual, expected, cmpopts.SortSlices(func(a, b string) bool {
			return a < b
		})), "archive does not contain the right layers")
	}
}

func TestSaveDirectoryPermissions(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "Test is looking at linux specific details")

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	layerEntries := []string{"opt/", "opt/a/", "opt/a/b/", "opt/a/b/c"}
	layerEntriesAUFS := []string{"./", ".wh..wh.aufs", ".wh..wh.orph/", ".wh..wh.plnk/", "opt/", "opt/a/", "opt/a/b/", "opt/a/b/c"}

	dockerfile := `FROM busybox
RUN adduser -D user && mkdir -p /opt/a/b && chown -R user:user /opt/a
RUN touch /opt/a/b/c && chown user:user /opt/a/b/c`

	imgID := build.Do(ctx, t, apiClient, fakecontext.New(t, t.TempDir(), fakecontext.WithDockerfile(dockerfile)), client.ImageBuildOptions{})

	rdr, err := apiClient.ImageSave(ctx, []string{imgID})
	assert.NilError(t, err)
	defer func() { _ = rdr.Close() }()

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
	dec, err := compression.DecompressStream(f)
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

type saveContentEnv struct {
	ctx               context.Context
	apiClient         *client.Client
	cs                content.Store
	reg               *registry.V2
	remote            string
	manifest          ocispec.Manifest
	manifestDesc      ocispec.Descriptor
	platformManifests []ocispec.Descriptor
}

func setupSaveContentEnv(t *testing.T, imageFn specialimage.SpecialImageFunc, authCfgs ...*registrytypes.AuthConfig) *saveContentEnv {
	ctx := context.Background()

	useAuth := len(authCfgs) > 0 && authCfgs[0] != nil

	var regOps []func(*registry.Config)
	if useAuth {
		regOps = append(regOps, registry.Htpasswd)
	}
	reg := registry.NewV2(t, regOps...)
	t.Cleanup(reg.Close)
	reg.WaitReady(t)

	dir := t.TempDir()
	index, err := imageFn(dir)
	assert.NilError(t, err)
	manifestDesc := index.Manifests[0]

	store, err := local.NewStore(dir)
	assert.NilError(t, err)

	publisher, err := containerd.New("", containerd.WithServices(containerd.WithContentStore(store)))
	assert.NilError(t, err)
	t.Cleanup(func() { _ = publisher.Close() })

	remote := path.Join(registry.DefaultURL, strings.ToLower(t.Name())+":latest")

	var pushOpts []containerd.RemoteOpt
	if useAuth {
		authConfig := authCfgs[0]
		resolver := docker.NewResolver(docker.ResolverOptions{
			Host: func(ns string) (string, error) {
				return registry.DefaultURL, nil
			},
			PlainHTTP: true,
			Credentials: func(host string) (string, string, error) {
				return authConfig.Username, authConfig.Password, nil
			},
		})
		pushOpts = append(pushOpts, containerd.WithResolver(resolver))
	}

	assert.NilError(t, publisher.Push(ctx, remote, manifestDesc, pushOpts...))

	d := daemon.New(t)
	d.Start(t)
	t.Cleanup(func() { d.Cleanup(t) })
	apiClient := d.NewClientT(t)

	var pullOpts client.ImagePullOptions
	if useAuth {
		authConfig := authCfgs[0]
		authJSON, err := json.Marshal(authConfig)
		assert.NilError(t, err)
		pullOpts.RegistryAuth = base64.URLEncoding.EncodeToString(authJSON)
	}

	rdr, err := apiClient.ImagePull(ctx, remote, pullOpts)
	assert.NilError(t, err)
	_, err = io.Copy(io.Discard, rdr)
	rdr.Close()
	assert.NilError(t, err)

	info, err := apiClient.Info(ctx, client.InfoOptions{})
	assert.NilError(t, err)
	c8dClient, err := containerd.New(info.Info.Containerd.Address, containerd.WithDefaultNamespace(info.Info.Containerd.Namespaces.Containers))
	assert.NilError(t, err)
	t.Cleanup(func() { _ = c8dClient.Close() })
	cs := c8dClient.ContentStore()

	manifestJSON, err := content.ReadBlob(ctx, cs, manifestDesc)
	assert.NilError(t, err)

	var manifest ocispec.Manifest

	if manifestDesc.MediaType == ocispec.MediaTypeImageIndex {
		var innerIndex ocispec.Index
		assert.NilError(t, json.Unmarshal(manifestJSON, &innerIndex))

		env := &saveContentEnv{
			ctx:               ctx,
			apiClient:         apiClient,
			cs:                cs,
			reg:               reg,
			remote:            remote,
			manifestDesc:      manifestDesc,
			platformManifests: innerIndex.Manifests,
		}

		hostPlatform := platforms.DefaultSpec()
		pm := platforms.NewMatcher(hostPlatform)
		for _, pmd := range innerIndex.Manifests {
			if pmd.Platform != nil && pm.Match(*pmd.Platform) {
				mJSON, err := content.ReadBlob(ctx, cs, pmd)
				assert.NilError(t, err)
				assert.NilError(t, json.Unmarshal(mJSON, &manifest))
				env.manifest = manifest
				break
			}
		}

		return env
	}

	assert.NilError(t, json.Unmarshal(manifestJSON, &manifest))

	return &saveContentEnv{
		ctx:          ctx,
		apiClient:    apiClient,
		cs:           cs,
		reg:          reg,
		remote:       remote,
		manifest:     manifest,
		manifestDesc: manifestDesc,
	}
}

func assertContentPresent(t *testing.T, ctx context.Context, cs content.Store, desc ocispec.Descriptor) {
	t.Helper()
	exists, err := content.Exists(ctx, cs, desc)
	assert.NilError(t, err)
	assert.Assert(t, exists, "content blob %s not found in store", desc.Digest)
}

func assertContentAbsent(t *testing.T, ctx context.Context, cs content.Store, desc ocispec.Descriptor) {
	t.Helper()
	exists, err := content.Exists(ctx, cs, desc)
	assert.NilError(t, err)
	assert.Assert(t, !exists, "content blob %s should not be in store", desc.Digest)
}

func deleteContent(t *testing.T, ctx context.Context, cs content.Store, desc ocispec.Descriptor) {
	t.Helper()
	err := cs.Delete(ctx, desc.Digest)
	if err != nil && !cerrdefs.IsNotFound(err) {
		assert.NilError(t, err)
	}
}

func expectedDescriptors(manifest ocispec.Manifest, manifestDesc ocispec.Descriptor) []ocispec.Descriptor {
	descs := []ocispec.Descriptor{manifestDesc}
	descs = append(descs, manifest.Config)
	descs = append(descs, manifest.Layers...)
	return descs
}

func (env *saveContentEnv) expectedArchiveDescriptors() []ocispec.Descriptor {
	if len(env.platformManifests) > 0 {
		descs := []ocispec.Descriptor{env.manifestDesc}
		hostPlatform := platforms.DefaultSpec()
		pm := platforms.NewMatcher(hostPlatform)
		for _, pmd := range env.platformManifests {
			if pmd.Platform != nil && pm.Match(*pmd.Platform) {
				descs = append(descs, pmd)
				break
			}
		}
		descs = append(descs, env.manifest.Config)
		descs = append(descs, env.manifest.Layers...)
		return descs
	}
	return expectedDescriptors(env.manifest, env.manifestDesc)
}

func assertArchiveHasBlob(t *testing.T, tarfs fs.FS, desc ocispec.Descriptor) {
	t.Helper()
	blobPath := "blobs/" + desc.Digest.Algorithm().String() + "/" + desc.Digest.Encoded()
	_, err := fs.Stat(tarfs, blobPath)
	assert.NilError(t, err, "blob %s not found in archive at %s", desc.Digest, blobPath)
}

// Case A: blob present in content store, save should produce complete archive.
func TestSaveContentStoreBlobPresent(t *testing.T) {
	env := setupSaveContentEnv(t, specialimage.MultiLayer)

	config := env.manifest.Config
	assertContentPresent(t, env.ctx, env.cs, config)

	rdr, err := env.apiClient.ImageSave(env.ctx, []string{env.remote})
	assert.NilError(t, err)
	defer func() { _ = rdr.Close() }()

	tarfs := tarIndexFS(t, rdr)
	for _, desc := range expectedDescriptors(env.manifest, env.manifestDesc) {
		assertArchiveHasBlob(t, tarfs, desc)
	}
}

// Case B: missing blob but public source available, recovery should materialize.
func TestSaveContentStoreMissingButRecoverable(t *testing.T) {
	env := setupSaveContentEnv(t, specialimage.MultiLayer)

	config := env.manifest.Config
	assertContentPresent(t, env.ctx, env.cs, config)

	deleteContent(t, env.ctx, env.cs, config)
	assertContentAbsent(t, env.ctx, env.cs, config)

	rdr, err := env.apiClient.ImageSave(env.ctx, []string{env.remote})
	assert.NilError(t, err)
	defer func() { _ = rdr.Close() }()

	assertContentPresent(t, env.ctx, env.cs, config)

	tarfs := tarIndexFS(t, rdr)
	for _, desc := range expectedDescriptors(env.manifest, env.manifestDesc) {
		assertArchiveHasBlob(t, tarfs, desc)
	}
}

// Case C: missing blob, no source available (moved from registry → 404).
func TestSaveContentStoreMissingNoSource(t *testing.T) {
	env := setupSaveContentEnv(t, specialimage.MultiLayer)

	config := env.manifest.Config
	assertContentPresent(t, env.ctx, env.cs, config)

	deleteContent(t, env.ctx, env.cs, config)
	assertContentAbsent(t, env.ctx, env.cs, config)

	undo := env.reg.TempMoveBlobData(t, config.Digest)
	defer undo()

	rdr, err := env.apiClient.ImageSave(env.ctx, []string{env.remote})
	assert.Assert(t, err != nil, "save with missing, unrecoverable content should fail")
	if rdr != nil {
		assert.Check(t, rdr.Close())
	}
	assertContentAbsent(t, env.ctx, env.cs, config)
}

// Case D: missing blob, auth-protected source (anonymous recovery → 401).
func TestSaveContentStoreMissingSourceUnavailable(t *testing.T) {
	env := setupSaveContentEnv(t, specialimage.MultiLayer, &registrytypes.AuthConfig{})

	config := env.manifest.Config
	assertContentPresent(t, env.ctx, env.cs, config)

	deleteContent(t, env.ctx, env.cs, config)
	assertContentAbsent(t, env.ctx, env.cs, config)

	// Unlike Case C (which moves the blob from the registry to produce a 404),
	// Case D leaves the blob in the auth-protected registry. The save recovery
	// path calls newResolverFromAuthConfig with a nil authConfig, so the
	// anonymous fetch is rejected with HTTP 401.
	rdr, err := env.apiClient.ImageSave(env.ctx, []string{env.remote})
	assert.Assert(t, err != nil, "save with missing content and auth-protected source should fail")
	if rdr != nil {
		assert.Check(t, rdr.Close())
	}
	assertContentAbsent(t, env.ctx, env.cs, config)
}

// Case E: non-distributable (foreign) layer is never fetched from registry.
func TestSaveContentStoreNonDistributableNotFetched(t *testing.T) {
	env := setupSaveContentEnv(t, specialimage.ForeignLayer)

	layer := env.manifest.Layers[0]
	assertContentPresent(t, env.ctx, env.cs, layer)

	deleteContent(t, env.ctx, env.cs, layer)
	assertContentAbsent(t, env.ctx, env.cs, layer)

	rdr, err := env.apiClient.ImageSave(env.ctx, []string{env.remote})
	assert.NilError(t, err)
	defer func() { _ = rdr.Close() }()

	// Non-distributable blobs are never recovered from the registry.
	assertContentAbsent(t, env.ctx, env.cs, layer)
}

// Case F: repeated save with content already present should not re-fetch.
func TestSaveContentStoreRepeatedSaveNoRefetch(t *testing.T) {
	env := setupSaveContentEnv(t, specialimage.MultiLayer)

	rdr1, err := env.apiClient.ImageSave(env.ctx, []string{env.remote})
	assert.NilError(t, err)
	tarfs1 := tarIndexFS(t, rdr1)
	_ = rdr1.Close()

	for _, desc := range expectedDescriptors(env.manifest, env.manifestDesc) {
		assertContentPresent(t, env.ctx, env.cs, desc)
		assertArchiveHasBlob(t, tarfs1, desc)
	}

	rdr2, err := env.apiClient.ImageSave(env.ctx, []string{env.remote})
	assert.NilError(t, err)
	tarfs2 := tarIndexFS(t, rdr2)
	_ = rdr2.Close()

	for _, desc := range expectedDescriptors(env.manifest, env.manifestDesc) {
		assertContentPresent(t, env.ctx, env.cs, desc)
		assertArchiveHasBlob(t, tarfs2, desc)
	}
}

// Case G: multi-platform image graph — recursive Walk discovers nested platform manifests.
func TestSaveContentStoreRecursiveWalkDiscovery(t *testing.T) {
	env := setupSaveContentEnv(t, specialimage.TwoPlatform)
	assert.Assert(t, len(env.platformManifests) > 0, "TwoPlatform should produce a multi-platform image with nested platform manifests")

	var layers []ocispec.Descriptor
	for _, layer := range env.manifest.Layers {
		assertContentPresent(t, env.ctx, env.cs, layer)
		deleteContent(t, env.ctx, env.cs, layer)
		assertContentAbsent(t, env.ctx, env.cs, layer)
		layers = append(layers, layer)
	}

	rdr, err := env.apiClient.ImageSave(env.ctx, []string{env.remote})
	assert.NilError(t, err)
	defer func() { _ = rdr.Close() }()

	tarfs := tarIndexFS(t, rdr)
	for _, layer := range layers {
		assertContentPresent(t, env.ctx, env.cs, layer)
	}
	for _, desc := range env.expectedArchiveDescriptors() {
		assertArchiveHasBlob(t, tarfs, desc)
	}
}
