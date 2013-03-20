package fs

import (
	"archive/tar"
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"testing"
)

func fakeTar() (io.Reader, error) {
	content := []byte("Hello world!\n")
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	for _, name := range []string{"/etc/postgres/postgres.conf", "/etc/passwd", "/var/log/postgres/postgres.conf"} {
		hdr := new(tar.Header)
		hdr.Size = int64(len(content))
		hdr.Name = name
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		tw.Write([]byte(content))
	}
	tw.Close()
	return buf, nil
}

func TestLayersInit(t *testing.T) {
	store := tempStore(t)
	defer os.RemoveAll(store.Root)
	// Root should exist
	if _, err := os.Stat(store.Root); err != nil {
		t.Fatal(err)
	}
	// List() should be empty
	if l := store.List(); len(l) != 0 {
		t.Fatalf("List() should return %d, not %d", 0, len(l))
	}
}

func TestAddLayer(t *testing.T) {
	store := tempStore(t)
	defer os.RemoveAll(store.Root)
	layer, err := store.AddLayer("foo", testArchive(t))
	if err != nil {
		t.Fatal(err)
	}
	// Layer path should exist
	if _, err := os.Stat(layer); err != nil {
		t.Fatal(err)
	}
	// List() should return 1 layer
	if l := store.List(); len(l) != 1 {
		t.Fatalf("List() should return %d elements, not %d", 1, len(l))
	}
	// Get("foo") should return the correct layer
	if foo := store.Get("foo"); foo != layer {
		t.Fatalf("get(\"foo\") should return '%d', not '%d'", layer, foo)
	}
}

func TestAddLayerDuplicate(t *testing.T) {
	store := tempStore(t)
	defer os.RemoveAll(store.Root)
	if _, err := store.AddLayer("foobar123", testArchive(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddLayer("foobar123", testArchive(t)); err == nil {
		t.Fatalf("Creating duplicate layer should fail")
	}
}

/*
 * HELPER FUNCTIONS
 */

func tempStore(t *testing.T) *LayerStore {
	tmp, err := ioutil.TempDir("", "docker-fs-layerstore-")
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewLayerStore(tmp)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func testArchive(t *testing.T) Archive {
	archive, err := fakeTar()
	if err != nil {
		t.Fatal(err)
	}
	return archive
}
