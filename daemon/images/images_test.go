package images

import (
	"io/ioutil"
	"os"
	"runtime"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/reference"
	"gotest.tools/assert"
)

var marshalledSaveLoadTestCases = []byte(`{"Repositories":{"busybox":{"busybox:latest":"sha256:1f86fbc76ed941d216a0639c8d787b586c21a2beca6e4dcc28abc9172bc2d6dc"},"jess/hollywood":{"jess/hollywood:latest":"sha256:472de72579154d792ace622ef45572124819d668624ab847b29dcaaf71f79f5d"},"registry":{"registry@sha256:367eb40fd0330a7e464777121e39d2f5b3e8e23a1e159342e53ab05c9e4d94e6":"sha256:24126a56805beb9711be5f4590cc2eb55ab8d4a85ebd618eed72bb19fc50631c"},"registry:5000/foobar":{"registry:5000/foobar:HEAD":"sha256:470022b8af682154f57a2163d030eb369549549cba00edc69e1b99b46bb924d6","registry:5000/foobar:alternate":"sha256:ae300ebc4a4f00693702cfb0a5e0b7bc527b353828dc86ad09fb95c8a681b793","registry:5000/foobar:latest":"sha256:ed5db16db354a7facad7f953d708fd4dc9f9dd3653bb66ec621f4c35d380e64a","registry:5000/foobar:master":"sha256:6c9917af4c4e05001b346421959d7ea81b6dc9d25718466a37a6add865dfd7fc"}}}`)

type mockLayerGetReleaser struct{}

func (ls *mockLayerGetReleaser) Get(layer.ChainID) (layer.Layer, error) {
	return nil, nil
}

func (ls *mockLayerGetReleaser) Release(layer.Layer) ([]layer.Metadata, error) {
	return nil, nil
}

func defaultFSStoreBackend(t *testing.T) (image.StoreBackend, func()) {
	tmpdir, err := ioutil.TempDir("", "images-fs-store")
	assert.NilError(t, err)

	fsBackend, err := image.NewFSStoreBackend(tmpdir)
	assert.NilError(t, err)

	return fsBackend, func() { os.RemoveAll(tmpdir) }
}

func defaultImageStore(t *testing.T) (image.Store, func()) {
	fsBackend, cleanup := defaultFSStoreBackend(t)

	mlgrMap := make(map[string]image.LayerGetReleaser)
	mlgrMap[runtime.GOOS] = &mockLayerGetReleaser{}
	store, err := image.NewImageStore(fsBackend, mlgrMap)
	assert.NilError(t, err)

	return store, cleanup
}

func defaultReferenceStore(t *testing.T, loadFrom []byte) (reference.Store, *os.File) {
	jsonFile, err := ioutil.TempFile("", "tag-store-test")
	assert.NilError(t, err)

	defer os.RemoveAll(jsonFile.Name())

	// Write canned json to the temp file
	_, err = jsonFile.Write(loadFrom)
	assert.NilError(t, err)

	rs, err := reference.NewReferenceStore(jsonFile.Name())
	assert.NilError(t, err)

	return rs, jsonFile

}

func TestFilterMultipleImageReferences(t *testing.T) {
	// imageStore initialize
	store, cleanup := defaultImageStore(t)
	defer cleanup()

	_, err := store.Create([]byte(`{"comment": "test1", "rootfs": {"type": "layers"}, "id": "1"}`))
	assert.NilError(t, err)

	_, err = store.Create([]byte(`{"comment": "test2", "rootfs": {"type": "layers"}, "id": "2"}`))
	assert.NilError(t, err)

	_, err = store.Create([]byte(`{"comment": "test3", "rootfs": {"type": "layers"}, "id": "3"}`))
	assert.NilError(t, err)

	// Reference store initialize
	rs, jf := defaultReferenceStore(t, marshalledSaveLoadTestCases)
	jf.Close()

	daemon := &ImageService{
		imageStore:     store,
		referenceStore: rs,
	}

	var images []*types.ImageSummary
	expectedImageCount := 10

	for i := 0; i < 10; i++ {
		ifs := filters.NewArgs(filters.Arg("reference", "busybox"), filters.Arg("reference", "ls"))
		img, err := daemon.Images(ifs, false, false)
		assert.NilError(t, err)

		images = append(images, img...)
	}

	assert.Equal(t, expectedImageCount, len(images))
}
