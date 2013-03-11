package image

import (
	"bytes"
	"github.com/dotcloud/docker/fake"
	"github.com/dotcloud/docker/future"
	"io/ioutil"
	"os"
	"testing"
)

func TestAddLayer(t *testing.T) {
	tmp, err := ioutil.TempDir("", "docker-test-image")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	store, err := NewLayerStore(tmp)
	if err != nil {
		t.Fatal(err)
	}
	archive, err := fake.FakeTar()
	if err != nil {
		t.Fatal(err)
	}
	layer, err := store.AddLayer(archive)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(layer); err != nil {
		t.Fatalf("Error testing for existence of layer: %s\n", err.Error())
	}
}

func TestComputeId(t *testing.T) {
	id1, err := future.ComputeId(bytes.NewBufferString("hello world\n"))
	if err != nil {
		t.Fatal(err)
	}
	id2, err := future.ComputeId(bytes.NewBufferString("foo bar\n"))
	if err != nil {
		t.Fatal(err)
	}
	if id1 == id2 {
		t.Fatalf("Identical checksums for difference content (%s == %s)", id1, id2)
	}
}
