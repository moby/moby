package docker

import (
	"github.com/dotcloud/docker/graph"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"testing"
)

const testLayerPath string = "/var/lib/docker/docker-ut.tar"
const unitTestImageName string = "busybox"

var unitTestStoreBase string
var srv *Server

func nuke(docker *Docker) error {
	return os.RemoveAll(docker.root)
}

func CopyDirectory(source, dest string) error {
	if _, err := exec.Command("cp", "-ra", source, dest).Output(); err != nil {
		return err
	}
	return nil
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
		return
	}

	if usr, err := user.Current(); err != nil {
		panic(err)
	} else if usr.Uid != "0" {
		panic("docker tests needs to be run as root")
	}

	// Create a temp directory
	root, err := ioutil.TempDir("", "docker-test")
	if err != nil {
		panic(err)
	}
	unitTestStoreBase = root

	// Make it our Store root
	docker, err := NewFromDirectory(root)
	if err != nil {
		panic(err)
	}
	// Create the "Server"
	srv := &Server{
		containers: docker,
	}
	// Retrieve the Image
	if err := srv.CmdImport(os.Stdin, os.Stdout, unitTestImageName); err != nil {
		panic(err)
	}
}

func newTestDocker() (*Docker, error) {
	root, err := ioutil.TempDir("", "docker-test")
	if err != nil {
		return nil, err
	}
	if err := os.Remove(root); err != nil {
		return nil, err
	}
	if err := CopyDirectory(unitTestStoreBase, root); err != nil {
		panic(err)
		return nil, err
	}

	docker, err := NewFromDirectory(root)
	if err != nil {
		return nil, err
	}

	return docker, nil
}

func GetTestImage(docker *Docker) *graph.Image {
	imgs, err := docker.graph.All()
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
		"ls",
		[]string{"-al"},
		GetTestImage(docker).Id,
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
	if docker.List()[0].Id != container.Id {
		t.Errorf("Unexpected container %v returned by List", docker.List()[0])
	}

	// Make sure we can get the container with Get()
	if docker.Get(container.Id) == nil {
		t.Errorf("Unable to get newly created container")
	}

	// Make sure it is the right container
	if docker.Get(container.Id) != container {
		t.Errorf("Get() returned the wrong container")
	}

	// Make sure Exists returns it as existing
	if !docker.Exists(container.Id) {
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
		"ls",
		[]string{"-al"},
		GetTestImage(docker).Id,
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
	if docker.Get(container.Id) != nil {
		t.Errorf("Unable to get newly created container")
	}

	// Make sure the container root directory does not exist anymore
	_, err = os.Stat(container.root)
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
		"ls",
		[]string{"-al"},
		GetTestImage(docker).Id,
		&Config{},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer docker.Destroy(container1)

	container2, err := docker.Create(
		"ls",
		[]string{"-al"},
		GetTestImage(docker).Id,
		&Config{},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer docker.Destroy(container2)

	container3, err := docker.Create(
		"ls",
		[]string{"-al"},
		GetTestImage(docker).Id,
		&Config{},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer docker.Destroy(container3)

	if docker.Get(container1.Id) != container1 {
		t.Errorf("Get(test1) returned %v while expecting %v", docker.Get(container1.Id), container1)
	}

	if docker.Get(container2.Id) != container2 {
		t.Errorf("Get(test2) returned %v while expecting %v", docker.Get(container2.Id), container2)
	}

	if docker.Get(container3.Id) != container3 {
		t.Errorf("Get(test3) returned %v while expecting %v", docker.Get(container3.Id), container3)
	}

}

func TestRestore(t *testing.T) {

	root, err := ioutil.TempDir("", "docker-test")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(root); err != nil {
		t.Fatal(err)
	}
	if err := CopyDirectory(unitTestStoreBase, root); err != nil {
		t.Fatal(err)
	}

	docker1, err := NewFromDirectory(root)
	if err != nil {
		t.Fatal(err)
	}

	// Create a container with one instance of docker
	container1, err := docker1.Create(
		"ls",
		[]string{"-al"},
		GetTestImage(docker1).Id,
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
	container2 := docker2.Get(container1.Id)
	if container2 == nil {
		t.Fatal("Unable to Get container")
	}
	if err := container2.Run(); err != nil {
		t.Fatal(err)
	}
}
