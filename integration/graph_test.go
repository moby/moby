package docker

import (
	"github.com/dotcloud/docker"
	"github.com/dotcloud/docker/graphdriver"
	"io/ioutil"
	"os"
	"path"
	"testing"
)

func TestMount(t *testing.T) {
	graph, driver := tempGraph(t)
	defer os.RemoveAll(graph.Root)
	defer driver.Cleanup()

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

	if _, err := driver.Get(image.ID); err != nil {
		t.Fatal(err)
	}
}

//FIXME: duplicate
func tempGraph(t *testing.T) (*docker.Graph, graphdriver.Driver) {
	tmp, err := ioutil.TempDir("", "docker-graph-")
	if err != nil {
		t.Fatal(err)
	}
	driver, err := graphdriver.New(tmp)
	if err != nil {
		t.Fatal(err)
	}
	graph, err := docker.NewGraph(tmp, driver)
	if err != nil {
		t.Fatal(err)
	}
	return graph, driver
}
