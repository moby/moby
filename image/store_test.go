package image

import (
	"io/ioutil"
	"os"

	"github.com/docker/distribution/digest"
	"github.com/docker/docker/layer"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestRestore(c *check.C) {
	tmpdir, err := ioutil.TempDir("", "images-fs-store")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	fs, err := NewFSStoreBackend(tmpdir)
	if err != nil {
		c.Fatal(err)
	}

	id1, err := fs.Set([]byte(`{"comment": "abc", "rootfs": {"type": "layers"}}`))
	if err != nil {
		c.Fatal(err)
	}
	_, err = fs.Set([]byte(`invalid`))
	if err != nil {
		c.Fatal(err)
	}
	id2, err := fs.Set([]byte(`{"comment": "def", "rootfs": {"type": "layers", "diff_ids": ["2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae"]}}`))
	if err != nil {
		c.Fatal(err)
	}
	err = fs.SetMetadata(id2, "parent", []byte(id1))
	if err != nil {
		c.Fatal(err)
	}

	is, err := NewImageStore(fs, &mockLayerGetReleaser{})
	if err != nil {
		c.Fatal(err)
	}

	imgs := is.Map()
	if actual, expected := len(imgs), 2; actual != expected {
		c.Fatalf("invalid images length, expected 2, got %q", len(imgs))
	}

	img1, err := is.Get(ID(id1))
	if err != nil {
		c.Fatal(err)
	}

	if actual, expected := img1.computedID, ID(id1); actual != expected {
		c.Fatalf("invalid image ID: expected %q, got %q", expected, actual)
	}

	if actual, expected := img1.computedID.String(), string(id1); actual != expected {
		c.Fatalf("invalid image ID string: expected %q, got %q", expected, actual)
	}

	img2, err := is.Get(ID(id2))
	if err != nil {
		c.Fatal(err)
	}

	if actual, expected := img1.Comment, "abc"; actual != expected {
		c.Fatalf("invalid comment for image1: expected %q, got %q", expected, actual)
	}

	if actual, expected := img2.Comment, "def"; actual != expected {
		c.Fatalf("invalid comment for image2: expected %q, got %q", expected, actual)
	}

	p, err := is.GetParent(ID(id1))
	if err == nil {
		c.Fatal("expected error for getting parent")
	}

	p, err = is.GetParent(ID(id2))
	if err != nil {
		c.Fatal(err)
	}
	if actual, expected := p, ID(id1); actual != expected {
		c.Fatalf("invalid parent: expected %q, got %q", expected, actual)
	}

	children := is.Children(ID(id1))
	if len(children) != 1 {
		c.Fatalf("invalid children length: %q", len(children))
	}
	if actual, expected := children[0], ID(id2); actual != expected {
		c.Fatalf("invalid child for id1: expected %q, got %q", expected, actual)
	}

	heads := is.Heads()
	if actual, expected := len(heads), 1; actual != expected {
		c.Fatalf("invalid images length: expected %q, got %q", expected, actual)
	}

	sid1, err := is.Search(string(id1)[:10])
	if err != nil {
		c.Fatal(err)
	}
	if actual, expected := sid1, ID(id1); actual != expected {
		c.Fatalf("searched ID mismatch: expected %q, got %q", expected, actual)
	}

	sid1, err = is.Search(digest.Digest(id1).Hex()[:6])
	if err != nil {
		c.Fatal(err)
	}
	if actual, expected := sid1, ID(id1); actual != expected {
		c.Fatalf("searched ID mismatch: expected %q, got %q", expected, actual)
	}

	invalidPattern := digest.Digest(id1).Hex()[1:6]
	_, err = is.Search(invalidPattern)
	if err == nil {
		c.Fatalf("expected search for %q to fail", invalidPattern)
	}

}

func (s *DockerSuite) TestAddDelete(c *check.C) {
	tmpdir, err := ioutil.TempDir("", "images-fs-store")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	fs, err := NewFSStoreBackend(tmpdir)
	if err != nil {
		c.Fatal(err)
	}

	is, err := NewImageStore(fs, &mockLayerGetReleaser{})
	if err != nil {
		c.Fatal(err)
	}

	id1, err := is.Create([]byte(`{"comment": "abc", "rootfs": {"type": "layers", "diff_ids": ["2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae"]}}`))
	if err != nil {
		c.Fatal(err)
	}

	if actual, expected := id1, ID("sha256:8d25a9c45df515f9d0fe8e4a6b1c64dd3b965a84790ddbcc7954bb9bc89eb993"); actual != expected {
		c.Fatalf("create ID mismatch: expected %q, got %q", expected, actual)
	}

	img, err := is.Get(id1)
	if err != nil {
		c.Fatal(err)
	}

	if actual, expected := img.Comment, "abc"; actual != expected {
		c.Fatalf("invalid comment in image: expected %q, got %q", expected, actual)
	}

	id2, err := is.Create([]byte(`{"comment": "def", "rootfs": {"type": "layers", "diff_ids": ["2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae"]}}`))
	if err != nil {
		c.Fatal(err)
	}

	err = is.SetParent(id2, id1)
	if err != nil {
		c.Fatal(err)
	}

	pid1, err := is.GetParent(id2)
	if err != nil {
		c.Fatal(err)
	}
	if actual, expected := pid1, id1; actual != expected {
		c.Fatalf("invalid parent for image: expected %q, got %q", expected, actual)
	}

	_, err = is.Delete(id1)
	if err != nil {
		c.Fatal(err)
	}
	_, err = is.Get(id1)
	if err == nil {
		c.Fatalf("expected get for deleted image %q to fail", id1)
	}
	_, err = is.Get(id2)
	if err != nil {
		c.Fatal(err)
	}
	pid1, err = is.GetParent(id2)
	if err == nil {
		c.Fatalf("expected parent check for image %q to fail, got %q", id2, pid1)
	}

}

func (s *DockerSuite) TestSearchAfterDelete(c *check.C) {
	tmpdir, err := ioutil.TempDir("", "images-fs-store")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	fs, err := NewFSStoreBackend(tmpdir)
	if err != nil {
		c.Fatal(err)
	}

	is, err := NewImageStore(fs, &mockLayerGetReleaser{})
	if err != nil {
		c.Fatal(err)
	}

	id, err := is.Create([]byte(`{"comment": "abc", "rootfs": {"type": "layers"}}`))
	if err != nil {
		c.Fatal(err)
	}

	id1, err := is.Search(string(id)[:15])
	if err != nil {
		c.Fatal(err)
	}

	if actual, expected := id1, id; expected != actual {
		c.Fatalf("wrong id returned from search: expected %q, got %q", expected, actual)
	}

	if _, err := is.Delete(id); err != nil {
		c.Fatal(err)
	}

	if _, err := is.Search(string(id)[:15]); err == nil {
		c.Fatal("expected search after deletion to fail")
	}
}

func (s *DockerSuite) TestParentReset(c *check.C) {
	tmpdir, err := ioutil.TempDir("", "images-fs-store")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	fs, err := NewFSStoreBackend(tmpdir)
	if err != nil {
		c.Fatal(err)
	}

	is, err := NewImageStore(fs, &mockLayerGetReleaser{})
	if err != nil {
		c.Fatal(err)
	}

	id, err := is.Create([]byte(`{"comment": "abc1", "rootfs": {"type": "layers"}}`))
	if err != nil {
		c.Fatal(err)
	}

	id2, err := is.Create([]byte(`{"comment": "abc2", "rootfs": {"type": "layers"}}`))
	if err != nil {
		c.Fatal(err)
	}

	id3, err := is.Create([]byte(`{"comment": "abc3", "rootfs": {"type": "layers"}}`))
	if err != nil {
		c.Fatal(err)
	}

	if err := is.SetParent(id, id2); err != nil {
		c.Fatal(err)
	}

	ids := is.Children(id2)
	if actual, expected := len(ids), 1; expected != actual {
		c.Fatalf("wrong number of children: %d, got %d", expected, actual)
	}

	if err := is.SetParent(id, id3); err != nil {
		c.Fatal(err)
	}

	ids = is.Children(id2)
	if actual, expected := len(ids), 0; expected != actual {
		c.Fatalf("wrong number of children after parent reset: %d, got %d", expected, actual)
	}

	ids = is.Children(id3)
	if actual, expected := len(ids), 1; expected != actual {
		c.Fatalf("wrong number of children after parent reset: %d, got %d", expected, actual)
	}

}

type mockLayerGetReleaser struct{}

func (ls *mockLayerGetReleaser) Get(layer.ChainID) (layer.Layer, error) {
	return nil, nil
}

func (ls *mockLayerGetReleaser) Release(layer.Layer) ([]layer.Metadata, error) {
	return nil, nil
}
