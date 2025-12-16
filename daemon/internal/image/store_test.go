package image

import (
	"fmt"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/v2/daemon/internal/layer"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestCreate(t *testing.T) {
	imgStore := defaultImageStore(t)
	_, err := imgStore.Create([]byte(`{}`))
	assert.Check(t, is.Error(err, "invalid image JSON, no RootFS key"))
}

func TestRestore(t *testing.T) {
	fsStore := defaultFSStoreBackend(t)

	id1, err := fsStore.Set([]byte(`{"comment": "abc", "rootfs": {"type": "layers"}}`))
	assert.NilError(t, err)

	_, err = fsStore.Set([]byte(`invalid`))
	assert.NilError(t, err)

	id2, err := fsStore.Set([]byte(`{"comment": "def", "rootfs": {"type": "layers", "diff_ids": ["2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae"]}}`))
	assert.NilError(t, err)

	err = fsStore.SetMetadata(id2, "parent", []byte(id1))
	assert.NilError(t, err)

	// This produces an error log (trying to unmarshal the "invalid" value from above, but doesn't return an error;
	// ERRO[0000] invalid image                                 digest="sha256:f1234d75178d892a133a410355a5a990cf75d2f33eba25d575943d4df632f3a4" err="invalid character 'i' looking for beginning of value: invalid"
	imgStore, err := NewImageStore(fsStore, &mockLayerGetReleaser{})
	assert.NilError(t, err)

	assert.Check(t, is.Len(imgStore.Map(), 2))

	img1, err := imgStore.Get(ID(id1))
	assert.NilError(t, err)
	assert.Check(t, is.Equal(ID(id1), img1.computedID))
	assert.Check(t, is.Equal(string(id1), img1.computedID.String()))

	img2, err := imgStore.Get(ID(id2))
	assert.NilError(t, err)
	assert.Check(t, is.Equal("abc", img1.Comment))
	assert.Check(t, is.Equal("def", img2.Comment))

	_, err = imgStore.GetParent(ID(id1))
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
	assert.ErrorContains(t, err, "failed to read metadata")

	p, err := imgStore.GetParent(ID(id2))
	assert.NilError(t, err)
	assert.Check(t, is.Equal(ID(id1), p))

	children := imgStore.Children(ID(id1))
	assert.Check(t, is.Len(children, 1))
	assert.Check(t, is.Equal(ID(id2), children[0]))
	assert.Check(t, is.Len(imgStore.Heads(), 1))

	sid1, err := imgStore.Search(string(id1)[:10])
	assert.NilError(t, err)
	assert.Check(t, is.Equal(ID(id1), sid1))

	sid1, err = imgStore.Search(id1.Encoded()[:6])
	assert.NilError(t, err)
	assert.Check(t, is.Equal(ID(id1), sid1))

	invalidPattern := id1.Encoded()[1:6]
	_, err = imgStore.Search(invalidPattern)
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
	assert.Check(t, is.ErrorContains(err, invalidPattern))
}

func TestAddDelete(t *testing.T) {
	imgStore := defaultImageStore(t)

	id1, err := imgStore.Create([]byte(`{"comment": "abc", "rootfs": {"type": "layers", "diff_ids": ["2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae"]}}`))
	assert.NilError(t, err)
	assert.Check(t, is.Equal(ID("sha256:8d25a9c45df515f9d0fe8e4a6b1c64dd3b965a84790ddbcc7954bb9bc89eb993"), id1))

	img, err := imgStore.Get(id1)
	assert.NilError(t, err)
	assert.Check(t, is.Equal("abc", img.Comment))

	id2, err := imgStore.Create([]byte(`{"comment": "def", "rootfs": {"type": "layers", "diff_ids": ["2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae"]}}`))
	assert.NilError(t, err)

	err = imgStore.SetParent(id2, id1)
	assert.NilError(t, err)

	pid1, err := imgStore.GetParent(id2)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(pid1, id1))

	_, err = imgStore.Delete(id1)
	assert.NilError(t, err)

	_, err = imgStore.Get(id1)
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
	assert.ErrorContains(t, err, "failed to get digest")

	_, err = imgStore.Get(id2)
	assert.NilError(t, err)

	_, err = imgStore.GetParent(id2)
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
	assert.ErrorContains(t, err, "failed to read metadata")
}

func TestSearchAfterDelete(t *testing.T) {
	imgStore := defaultImageStore(t)

	id, err := imgStore.Create([]byte(`{"comment": "abc", "rootfs": {"type": "layers"}}`))
	assert.NilError(t, err)

	id1, err := imgStore.Search(string(id)[:15])
	assert.NilError(t, err)
	assert.Check(t, is.Equal(id1, id))

	_, err = imgStore.Delete(id)
	assert.NilError(t, err)

	_, err = imgStore.Search(string(id)[:15])
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
	assert.ErrorContains(t, err, "No such image")
}

func TestDeleteNotExisting(t *testing.T) {
	imgStore := defaultImageStore(t)
	_, err := imgStore.Delete(ID("i_dont_exists"))
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
}

func TestParentReset(t *testing.T) {
	imgStore := defaultImageStore(t)

	id, err := imgStore.Create([]byte(`{"comment": "abc1", "rootfs": {"type": "layers"}}`))
	assert.NilError(t, err)

	id2, err := imgStore.Create([]byte(`{"comment": "abc2", "rootfs": {"type": "layers"}}`))
	assert.NilError(t, err)

	id3, err := imgStore.Create([]byte(`{"comment": "abc3", "rootfs": {"type": "layers"}}`))
	assert.NilError(t, err)

	assert.Check(t, imgStore.SetParent(id, id2))
	assert.Check(t, is.Len(imgStore.Children(id2), 1))

	assert.Check(t, imgStore.SetParent(id, id3))
	assert.Check(t, is.Len(imgStore.Children(id2), 0))
	assert.Check(t, is.Len(imgStore.Children(id3), 1))
}

func defaultImageStore(t *testing.T) Store {
	t.Helper()
	fsBackend, err := NewFSStoreBackend(t.TempDir())
	assert.Check(t, err)

	imgStore, err := NewImageStore(fsBackend, &mockLayerGetReleaser{})
	assert.NilError(t, err)

	return imgStore
}

func TestGetAndSetLastUpdated(t *testing.T) {
	imgStore := defaultImageStore(t)

	id, err := imgStore.Create([]byte(`{"comment": "abc1", "rootfs": {"type": "layers"}}`))
	assert.NilError(t, err)

	updated, err := imgStore.GetLastUpdated(id)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(updated.IsZero(), true))

	assert.Check(t, imgStore.SetLastUpdated(id))

	updated, err = imgStore.GetLastUpdated(id)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(updated.IsZero(), false))
}

func TestStoreLen(t *testing.T) {
	imgStore := defaultImageStore(t)

	expected := 10
	for i := range expected {
		_, err := imgStore.Create(fmt.Appendf(nil, `{"comment": "abc%d", "rootfs": {"type": "layers"}}`, i))
		assert.NilError(t, err)
	}
	numImages := imgStore.Len()
	assert.Equal(t, expected, numImages)
	assert.Equal(t, len(imgStore.Map()), numImages)
}

type mockLayerGetReleaser struct{}

func (ls *mockLayerGetReleaser) Get(layer.ChainID) (layer.Layer, error) {
	return nil, nil
}

func (ls *mockLayerGetReleaser) Release(layer.Layer) ([]layer.Metadata, error) {
	return nil, nil
}
