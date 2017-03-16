package image

import (
	"testing"

	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/testutil/assert"
	"github.com/opencontainers/go-digest"
)

func TestRestore(t *testing.T) {
	fs, cleanup := defaultFSStoreBackend(t)
	defer cleanup()

	id1, err := fs.Set([]byte(`{"comment": "abc", "rootfs": {"type": "layers"}}`))
	assert.NilError(t, err)

	_, err = fs.Set([]byte(`invalid`))
	assert.NilError(t, err)

	id2, err := fs.Set([]byte(`{"comment": "def", "rootfs": {"type": "layers", "diff_ids": ["2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae"]}}`))
	assert.NilError(t, err)

	err = fs.SetMetadata(id2, "parent", []byte(id1))
	assert.NilError(t, err)

	is, err := NewImageStore(fs, &mockLayerGetReleaser{})
	assert.NilError(t, err)

	assert.Equal(t, len(is.Map()), 2)

	img1, err := is.Get(ID(id1))
	assert.NilError(t, err)
	assert.Equal(t, img1.computedID, ID(id1))
	assert.Equal(t, img1.computedID.String(), string(id1))

	img2, err := is.Get(ID(id2))
	assert.NilError(t, err)
	assert.Equal(t, img1.Comment, "abc")
	assert.Equal(t, img2.Comment, "def")

	p, err := is.GetParent(ID(id1))
	assert.Error(t, err, "failed to read metadata")

	p, err = is.GetParent(ID(id2))
	assert.NilError(t, err)
	assert.Equal(t, p, ID(id1))

	children := is.Children(ID(id1))
	assert.Equal(t, len(children), 1)
	assert.Equal(t, children[0], ID(id2))
	assert.Equal(t, len(is.Heads()), 1)

	sid1, err := is.Search(string(id1)[:10])
	assert.NilError(t, err)
	assert.Equal(t, sid1, ID(id1))

	sid1, err = is.Search(digest.Digest(id1).Hex()[:6])
	assert.NilError(t, err)
	assert.Equal(t, sid1, ID(id1))

	invalidPattern := digest.Digest(id1).Hex()[1:6]
	_, err = is.Search(invalidPattern)
	assert.Error(t, err, "No such image")
}

func TestAddDelete(t *testing.T) {
	is, cleanup := defaultImageStore(t)
	defer cleanup()

	id1, err := is.Create([]byte(`{"comment": "abc", "rootfs": {"type": "layers", "diff_ids": ["2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae"]}}`))
	assert.NilError(t, err)
	assert.Equal(t, id1, ID("sha256:8d25a9c45df515f9d0fe8e4a6b1c64dd3b965a84790ddbcc7954bb9bc89eb993"))

	img, err := is.Get(id1)
	assert.NilError(t, err)
	assert.Equal(t, img.Comment, "abc")

	id2, err := is.Create([]byte(`{"comment": "def", "rootfs": {"type": "layers", "diff_ids": ["2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae"]}}`))
	assert.NilError(t, err)

	err = is.SetParent(id2, id1)
	assert.NilError(t, err)

	pid1, err := is.GetParent(id2)
	assert.NilError(t, err)
	assert.Equal(t, pid1, id1)

	_, err = is.Delete(id1)
	assert.NilError(t, err)

	_, err = is.Get(id1)
	assert.Error(t, err, "failed to get digest")

	_, err = is.Get(id2)
	assert.NilError(t, err)

	_, err = is.GetParent(id2)
	assert.Error(t, err, "failed to read metadata")
}

func TestSearchAfterDelete(t *testing.T) {
	is, cleanup := defaultImageStore(t)
	defer cleanup()

	id, err := is.Create([]byte(`{"comment": "abc", "rootfs": {"type": "layers"}}`))
	assert.NilError(t, err)

	id1, err := is.Search(string(id)[:15])
	assert.NilError(t, err)
	assert.Equal(t, id1, id)

	_, err = is.Delete(id)
	assert.NilError(t, err)

	_, err = is.Search(string(id)[:15])
	assert.Error(t, err, "No such image")
}

func TestParentReset(t *testing.T) {
	is, cleanup := defaultImageStore(t)
	defer cleanup()

	id, err := is.Create([]byte(`{"comment": "abc1", "rootfs": {"type": "layers"}}`))
	assert.NilError(t, err)

	id2, err := is.Create([]byte(`{"comment": "abc2", "rootfs": {"type": "layers"}}`))
	assert.NilError(t, err)

	id3, err := is.Create([]byte(`{"comment": "abc3", "rootfs": {"type": "layers"}}`))
	assert.NilError(t, err)

	assert.NilError(t, is.SetParent(id, id2))
	assert.Equal(t, len(is.Children(id2)), 1)

	assert.NilError(t, is.SetParent(id, id3))
	assert.Equal(t, len(is.Children(id2)), 0)
	assert.Equal(t, len(is.Children(id3)), 1)
}

func defaultImageStore(t *testing.T) (Store, func()) {
	fsBackend, cleanup := defaultFSStoreBackend(t)

	store, err := NewImageStore(fsBackend, &mockLayerGetReleaser{})
	assert.NilError(t, err)

	return store, cleanup
}

type mockLayerGetReleaser struct{}

func (ls *mockLayerGetReleaser) Get(layer.ChainID) (layer.Layer, error) {
	return nil, nil
}

func (ls *mockLayerGetReleaser) Release(layer.Layer) ([]layer.Metadata, error) {
	return nil, nil
}
