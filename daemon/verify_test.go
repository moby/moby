package daemon

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/idtools"
	"github.com/opencontainers/go-digest"
)

func TestChecksumPath(t *testing.T) {
	tmp, err := ioutil.TempDir("", "verify-test-")
	if err != nil {
		t.Fail()
	}
	defer os.RemoveAll(tmp)

	expected := digest.Digest("sha256:b93438477dadfeb4624a32f960fe36d5a3c2d207fac5c3b49f50fdf2f5e4e54d")

	err = ioutil.WriteFile(path.Join(tmp, "foo"), []byte("bar"), 0644)
	if err != nil {
		t.Fail()
	}

	checksumRootOwner, err := checksumPath(tmp, masker{})
	if err != nil {
		t.Fail()
	}
	if expected != checksumRootOwner {
		t.Fatalf("expected checksum=%s got %s", expected, checksumRootOwner)
	}

	// Remapped uid/gid can still compute the same checksum
	err = os.Chown(path.Join(tmp, "foo"), 5000, 5000)
	if err != nil {
		t.Fail()
	}

	idMaps := []idtools.IDMap{{0, 5000, 6000}}
	checksumAfterChown, err := checksumPath(tmp, masker{idMaps, idMaps})
	if err != nil {
		t.Fail()
	}
	if expected != checksumAfterChown {
		t.Fatalf("expected checksum=%s got %s", expected, checksumAfterChown)
	}
}

func TestChecksumFilesystem(t *testing.T) {
	tmp, err := ioutil.TempDir("", "verify-test-")
	if err != nil {
		t.Fail()
	}
	defer os.RemoveAll(tmp)

	daemon := &Daemon{
		layerStore:  NewMockLayerStore(tmp),
		configStore: config.New(),
	}

	// Empty layer
	err = os.Mkdir(path.Join(tmp, "empty-verify"), 0600)
	if err != nil {
		t.Fail()
	}

	dgst, err := daemon.checksumFilesystem("empty", "")
	if err != nil {
		t.Fail()
	}
	expected := digest.Digest("sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")
	if expected != dgst {
		t.Fatalf("expected checksum=%s got %s", expected, dgst)
	}

	// Layer with one file
	err = os.Mkdir(path.Join(tmp, "test1-verify"), 0600)
	if err != nil {
		t.Fail()
	}
	err = ioutil.WriteFile(path.Join(tmp, "test1-verify", "foz"), []byte("baz"), 0600)

	dgst, err = daemon.checksumFilesystem("test1", "")
	if err != nil {
		t.Fail()
	}
	expected2 := digest.Digest("sha256:a3c4d223772f48b471994881a8d32f1126fd0a41ab6a4a2796dd1696d5a19cc4")
	if expected2 != dgst {
		t.Fatalf("expected checksum=%s got %s", expected2, dgst)
	}
}

type mockLayerStore struct {
	root       string
	layerTable map[string]int
}

func NewMockLayerStore(root string) *mockLayerStore {
	return &mockLayerStore{
		root:       root,
		layerTable: make(map[string]int),
	}
}

func (ls *mockLayerStore) CreateRWLayer(id string, parent layer.ChainID, opts *layer.CreateRWLayerOpts) (layer.RWLayer, error) {
	layerPath := path.Join(ls.root, id)

	if _, ok := ls.layerTable[id]; ok {
		return nil, fmt.Errorf("creating layer %s already exists", id)
	}
	ls.layerTable[id] = 1
	return &mockLayer{layerPath}, nil
}

func (ls *mockLayerStore) ReleaseRWLayer(rwLayer layer.RWLayer) ([]layer.Metadata, error) {
	if _, ok := ls.layerTable[rwLayer.Name()]; !ok {
		return nil, fmt.Errorf("releasing layer %s not created", rwLayer.Name())
	}
	delete(ls.layerTable, rwLayer.Name())
	return []layer.Metadata{}, nil
}

func (ls *mockLayerStore) Register(io.Reader, layer.ChainID) (layer.Layer, error) {
	return nil, nil
}

func (ls *mockLayerStore) Get(layer.ChainID) (layer.Layer, error) {
	return nil, nil
}

func (ls *mockLayerStore) Map() map[layer.ChainID]layer.Layer {
	return map[layer.ChainID]layer.Layer{}
}

func (ls *mockLayerStore) Release(layer.Layer) ([]layer.Metadata, error) {
	return []layer.Metadata{}, nil
}

func (ls *mockLayerStore) GetRWLayer(id string) (layer.RWLayer, error) {
	return nil, nil
}

func (ls *mockLayerStore) GetMountID(id string) (string, error) {
	return "", nil
}

func (ls *mockLayerStore) Cleanup() error {
	return nil
}

func (ls *mockLayerStore) DriverStatus() [][2]string {
	return [][2]string{}
}

func (ls *mockLayerStore) DriverName() string {
	return "mock"
}

type mockLayer struct {
	path string
}

func (l *mockLayer) Name() string {
	return path.Base(l.path)
}

func (l *mockLayer) Mount(mountLabel string) (string, error) {
	return l.path, nil
}

func (l *mockLayer) Unmount() error {
	return nil
}

func (l *mockLayer) Parent() layer.Layer {
	return nil
}

func (l *mockLayer) Size() (int64, error) {
	return 0, nil
}

func (l *mockLayer) Changes() ([]archive.Change, error) {
	return []archive.Change{}, nil
}

func (l *mockLayer) Metadata() (map[string]string, error) {
	return map[string]string{}, nil
}

func (l *mockLayer) TarStream() (io.ReadCloser, error) {
	return nil, nil
}
