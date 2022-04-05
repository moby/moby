package image // import "github.com/docker/docker/image"

import (
	"context"
	"fmt"
	"testing"

	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/layer"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestCreate(t *testing.T) {
	is, cleanup := defaultImageStore(t)
	defer cleanup()

	_, err := is.Create(context.Background(), []byte(`{}`))
	assert.Check(t, cmp.Error(err, "invalid image JSON, no RootFS key"))
}

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

	is, err := NewImageStore(context.Background(), fs, &mockLayerGetReleaser{})
	assert.NilError(t, err)

	imgmap, err := is.Map(context.Background())
	assert.Check(t, err)
	assert.Check(t, cmp.Len(imgmap, 2))

	img1, err := is.Get(context.Background(), ID(id1))
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(ID(id1), img1.computedID))
	assert.Check(t, cmp.Equal(string(id1), img1.computedID.String()))

	img2, err := is.Get(context.Background(), ID(id2))
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal("abc", img1.Comment))
	assert.Check(t, cmp.Equal("def", img2.Comment))

	_, err = is.GetParent(context.Background(), ID(id1))
	assert.Check(t, errdefs.IsNotFound(err), "got error %q", err)

	p, err := is.GetParent(context.Background(), ID(id2))
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(ID(id1), p))

	children, err := is.Children(context.Background(), ID(id1))
	assert.Check(t, err)
	assert.Check(t, cmp.Len(children, 1))
	assert.Check(t, cmp.Equal(ID(id2), children[0]))
	heads, err := is.Heads(context.Background())
	assert.Check(t, err)
	assert.Check(t, cmp.Len(heads, 1))

	sid1, err := is.Search(context.Background(), string(id1)[:10])
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(ID(id1), sid1))

	sid1, err = is.Search(context.Background(), id1.Hex()[:6])
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(ID(id1), sid1))

	invalidPattern := id1.Hex()[1:6]
	_, err = is.Search(context.Background(), invalidPattern)
	assert.ErrorContains(t, err, "No such image")
}

func TestAddDelete(t *testing.T) {
	is, cleanup := defaultImageStore(t)
	defer cleanup()

	id1, err := is.Create(context.Background(), []byte(`{"comment": "abc", "rootfs": {"type": "layers", "diff_ids": ["2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae"]}}`))
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(ID("sha256:8d25a9c45df515f9d0fe8e4a6b1c64dd3b965a84790ddbcc7954bb9bc89eb993"), id1))

	img, err := is.Get(context.Background(), id1)
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal("abc", img.Comment))

	id2, err := is.Create(context.Background(), []byte(`{"comment": "def", "rootfs": {"type": "layers", "diff_ids": ["2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae"]}}`))
	assert.NilError(t, err)

	err = is.SetParent(context.Background(), id2, id1)
	assert.NilError(t, err)

	pid1, err := is.GetParent(context.Background(), id2)
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(pid1, id1))

	_, err = is.Delete(context.Background(), id1)
	assert.NilError(t, err)

	_, err = is.Get(context.Background(), id1)
	assert.ErrorContains(t, err, "failed to get digest")

	_, err = is.Get(context.Background(), id2)
	assert.NilError(t, err)

	_, err = is.GetParent(context.Background(), id2)
	assert.Check(t, errdefs.IsNotFound(err), "got error %q", err)
}

func TestSearchAfterDelete(t *testing.T) {
	is, cleanup := defaultImageStore(t)
	defer cleanup()

	id, err := is.Create(context.Background(), []byte(`{"comment": "abc", "rootfs": {"type": "layers"}}`))
	assert.NilError(t, err)

	id1, err := is.Search(context.Background(), string(id)[:15])
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(id1, id))

	_, err = is.Delete(context.Background(), id)
	assert.NilError(t, err)

	_, err = is.Search(context.Background(), string(id)[:15])
	assert.ErrorContains(t, err, "No such image")
}

func TestParentReset(t *testing.T) {
	is, cleanup := defaultImageStore(t)
	defer cleanup()

	id, err := is.Create(context.Background(), []byte(`{"comment": "abc1", "rootfs": {"type": "layers"}}`))
	assert.NilError(t, err)

	id2, err := is.Create(context.Background(), []byte(`{"comment": "abc2", "rootfs": {"type": "layers"}}`))
	assert.NilError(t, err)

	id3, err := is.Create(context.Background(), []byte(`{"comment": "abc3", "rootfs": {"type": "layers"}}`))
	assert.NilError(t, err)

	children := func(id ID) []ID {
		t.Helper()
		ids, err := is.Children(context.Background(), id)
		assert.Check(t, err)
		return ids
	}

	assert.Check(t, is.SetParent(context.Background(), id, id2))
	assert.Check(t, cmp.Len(children(id2), 1))

	assert.Check(t, is.SetParent(context.Background(), id, id3))
	assert.Check(t, cmp.Len(children(id2), 0))
	assert.Check(t, cmp.Len(children(id3), 1))
}

func defaultImageStore(t *testing.T) (Store, func()) {
	fsBackend, cleanup := defaultFSStoreBackend(t)

	store, err := NewImageStore(context.Background(), fsBackend, &mockLayerGetReleaser{})
	assert.NilError(t, err)

	return store, cleanup
}

func TestGetAndSetLastUpdated(t *testing.T) {
	store, cleanup := defaultImageStore(t)
	defer cleanup()

	id, err := store.Create(context.Background(), []byte(`{"comment": "abc1", "rootfs": {"type": "layers"}}`))
	assert.NilError(t, err)

	updated, err := store.GetLastUpdated(context.Background(), id)
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(updated.IsZero(), true))

	assert.Check(t, store.SetLastUpdated(context.Background(), id))

	updated, err = store.GetLastUpdated(context.Background(), id)
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(updated.IsZero(), false))
}

func TestStoreLen(t *testing.T) {
	store, cleanup := defaultImageStore(t)
	defer cleanup()

	expected := 10
	for i := 0; i < expected; i++ {
		_, err := store.Create(context.Background(), []byte(fmt.Sprintf(`{"comment": "abc%d", "rootfs": {"type": "layers"}}`, i)))
		assert.NilError(t, err)
	}
	numImages, err := store.Len(context.Background())
	assert.Check(t, err)
	assert.Equal(t, expected, numImages)
	imgMap, err := store.Map(context.Background())
	assert.Check(t, err)
	assert.Equal(t, len(imgMap), numImages)
}

type mockLayerGetReleaser struct{}

func (ls *mockLayerGetReleaser) Get(layer.ChainID) (layer.Layer, error) {
	return nil, nil
}

func (ls *mockLayerGetReleaser) Release(layer.Layer) ([]layer.Metadata, error) {
	return nil, nil
}
