package docker

import (
	"./fs"
	"io"
	"io/ioutil"
	"log"
	"os"
	"testing"
)

const testLayerPath string = "/var/lib/docker/docker-ut.tar"

func nuke(docker *Docker) error {
	return os.RemoveAll(docker.root)
}

func layerArchive(tarfile string) (io.Reader, error) {
	// FIXME: need to close f somewhere
	f, err := os.Open(tarfile)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func init() {
	// Hack to run sys init during unit testing
	if SelfPath() == "/sbin/init" {
		SysInit()
	}

	// Make sure the unit test image is there
	if _, err := os.Stat(testLayerPath); err != nil {
		if !os.IsNotExist(err) {
			panic(err)
		}
		log.Fatalf("Unit test base image not found. Please fix the problem by running \"debootstrap --arch=amd64 quantal %v\"", testLayerPath)
		return
	}
}

func newTestDocker() (*Docker, error) {
	root, err := ioutil.TempDir("", "docker-test")
	if err != nil {
		return nil, err
	}
	docker, err := NewFromDirectory(root)
	if err != nil {
		return nil, err
	}

	if layer, err := layerArchive(testLayerPath); err != nil {
		panic(err)
	} else {
		_, err = docker.Store.Create(layer, nil, "docker-ut", "unit tests")
		if err != nil {
			panic(err)
		}
	}
	return docker, nil
}

func GetTestImage(docker *Docker) *fs.Image {
	imgs, err := docker.Store.Images()
	if err != nil {
		panic(err)
	} else if len(imgs) < 1 {
		panic("GASP")
	}
	return imgs[0]
}

func TestCreate(t *testing.T) {
	docker, err := newTestDocker()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(docker)

	// Make sure we start we 0 containers
	if len(docker.List()) != 0 {
		t.Errorf("Expected 0 containers, %v found", len(docker.List()))
	}
	container, err := docker.Create(
		"test_create",
		"ls",
		[]string{"-al"},
		GetTestImage(docker),
		&Config{},
	)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := docker.Destroy(container); err != nil {
			t.Error(err)
		}
	}()

	// Make sure we can find the newly created container with List()
	if len(docker.List()) != 1 {
		t.Errorf("Expected 1 container, %v found", len(docker.List()))
	}

	// Make sure the container List() returns is the right one
	if docker.List()[0].Id != "test_create" {
		t.Errorf("Unexpected container %v returned by List", docker.List()[0])
	}

	// Make sure we can get the container with Get()
	if docker.Get("test_create") == nil {
		t.Errorf("Unable to get newly created container")
	}

	// Make sure it is the right container
	if docker.Get("test_create") != container {
		t.Errorf("Get() returned the wrong container")
	}

	// Make sure Exists returns it as existing
	if !docker.Exists("test_create") {
		t.Errorf("Exists() returned false for a newly created container")
	}
}

func TestDestroy(t *testing.T) {
	docker, err := newTestDocker()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(docker)
	container, err := docker.Create(
		"test_destroy",
		"ls",
		[]string{"-al"},
		GetTestImage(docker),
		&Config{},
	)
	if err != nil {
		t.Fatal(err)
	}
	// Destroy
	if err := docker.Destroy(container); err != nil {
		t.Error(err)
	}

	// Make sure docker.Exists() behaves correctly
	if docker.Exists("test_destroy") {
		t.Errorf("Exists() returned true")
	}

	// Make sure docker.List() doesn't list the destroyed container
	if len(docker.List()) != 0 {
		t.Errorf("Expected 0 container, %v found", len(docker.List()))
	}

	// Make sure docker.Get() refuses to return the unexisting container
	if docker.Get("test_destroy") != nil {
		t.Errorf("Unable to get newly created container")
	}

	// Make sure the container root directory does not exist anymore
	_, err = os.Stat(container.Root)
	if err == nil || !os.IsNotExist(err) {
		t.Errorf("Container root directory still exists after destroy")
	}

	// Test double destroy
	if err := docker.Destroy(container); err == nil {
		// It should have failed
		t.Errorf("Double destroy did not fail")
	}
}

func TestGet(t *testing.T) {
	docker, err := newTestDocker()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(docker)
	container1, err := docker.Create(
		"test1",
		"ls",
		[]string{"-al"},
		GetTestImage(docker),
		&Config{},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer docker.Destroy(container1)

	container2, err := docker.Create(
		"test2",
		"ls",
		[]string{"-al"},
		GetTestImage(docker),
		&Config{},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer docker.Destroy(container2)

	container3, err := docker.Create(
		"test3",
		"ls",
		[]string{"-al"},
		GetTestImage(docker),
		&Config{},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer docker.Destroy(container3)

	if docker.Get("test1") != container1 {
		t.Errorf("Get(test1) returned %v while expecting %v", docker.Get("test1"), container1)
	}

	if docker.Get("test2") != container2 {
		t.Errorf("Get(test2) returned %v while expecting %v", docker.Get("test2"), container2)
	}

	if docker.Get("test3") != container3 {
		t.Errorf("Get(test3) returned %v while expecting %v", docker.Get("test3"), container3)
	}

}

func TestRestore(t *testing.T) {
	root, err := ioutil.TempDir("", "docker-test")
	if err != nil {
		t.Fatal(err)
	}

	docker1, err := NewFromDirectory(root)
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(docker1)

	if layer, err := layerArchive(testLayerPath); err != nil {
		panic(err)
	} else {
		_, err = docker1.Store.Create(layer, nil, "docker-ut", "unit tests")
		if err != nil {
			panic(err)
		}
	}

	// Create a container with one instance of docker
	container1, err := docker1.Create(
		"restore_test",
		"ls",
		[]string{"-al"},
		GetTestImage(docker1),
		&Config{},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer docker1.Destroy(container1)
	if len(docker1.List()) != 1 {
		t.Errorf("Expected 1 container, %v found", len(docker1.List()))
	}
	if err := container1.Run(); err != nil {
		t.Fatal(err)
	}

	// Here are are simulating a docker restart - that is, reloading all containers
	// from scratch
	docker2, err := NewFromDirectory(root)
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(docker2)
	if len(docker2.List()) != 1 {
		t.Errorf("Expected 1 container, %v found", len(docker2.List()))
	}
	container2 := docker2.Get("restore_test")
	if container2 == nil {
		t.Fatal("Unable to Get container")
	}
	if err := container2.Run(); err != nil {
		t.Fatal(err)
	}
}
