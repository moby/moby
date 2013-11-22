package docker

import (
	"github.com/dotcloud/docker"
	"io/ioutil"
	"os"
	"path"
	"testing"
)

func TestMount(t *testing.T) {
	graph := tempGraph(t)
	defer os.RemoveAll(graph.Root)
	archive, err := fakeTar()
	if err != nil {
		t.Fatal(err)
	}
	image, err := graph.Create(archive, nil, "Testing", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	tmp, err := ioutil.TempDir("", "docker-test-graph-mount-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	rootfs := path.Join(tmp, "rootfs")
	if err := os.MkdirAll(rootfs, 0700); err != nil {
		t.Fatal(err)
	}
	rw := path.Join(tmp, "rw")
	if err := os.MkdirAll(rw, 0700); err != nil {
		t.Fatal(err)
	}
	if err := image.Mount(rootfs, rw); err != nil {
		t.Fatal(err)
	}
	// FIXME: test for mount contents
	defer func() {
		if err := docker.Unmount(rootfs); err != nil {
			t.Error(err)
		}
	}()
}

//FIXME: duplicate
func tempGraph(t *testing.T) *docker.Graph {
	tmp, err := ioutil.TempDir("", "docker-graph-")
	if err != nil {
		t.Fatal(err)
	}
	graph, err := docker.NewGraph(tmp)
	if err != nil {
		t.Fatal(err)
	}
	return graph
}
