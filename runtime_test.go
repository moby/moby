package docker

import (
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"testing"
)

// FIXME: this is no longer needed
const testLayerPath string = "/var/lib/docker/docker-ut.tar"
const unitTestImageName string = "docker-ut"

var unitTestStoreBase string
var srv *Server

func nuke(runtime *Runtime) error {
	return os.RemoveAll(runtime.root)
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
	runtime, err := NewRuntimeFromDirectory(root)
	if err != nil {
		panic(err)
	}
	// Create the "Server"
	srv := &Server{
		runtime: runtime,
	}
	// Retrieve the Image
	if err := srv.CmdPull(os.Stdin, os.Stdout, unitTestImageName); err != nil {
		panic(err)
	}
}

func newTestRuntime() (*Runtime, error) {
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

	runtime, err := NewRuntimeFromDirectory(root)
	if err != nil {
		return nil, err
	}

	return runtime, nil
}

func GetTestImage(runtime *Runtime) *Image {
	imgs, err := runtime.graph.All()
	if err != nil {
		panic(err)
	} else if len(imgs) < 1 {
		panic("GASP")
	}
	return imgs[0]
}

func TestRuntimeCreate(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	// Make sure we start we 0 containers
	if len(runtime.List()) != 0 {
		t.Errorf("Expected 0 containers, %v found", len(runtime.List()))
	}
	container, err := runtime.Create(&Config{
		Image: GetTestImage(runtime).Id,
		Cmd:   []string{"ls", "-al"},
	},
	)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := runtime.Destroy(container); err != nil {
			t.Error(err)
		}
	}()

	// Make sure we can find the newly created container with List()
	if len(runtime.List()) != 1 {
		t.Errorf("Expected 1 container, %v found", len(runtime.List()))
	}

	// Make sure the container List() returns is the right one
	if runtime.List()[0].Id != container.Id {
		t.Errorf("Unexpected container %v returned by List", runtime.List()[0])
	}

	// Make sure we can get the container with Get()
	if runtime.Get(container.Id) == nil {
		t.Errorf("Unable to get newly created container")
	}

	// Make sure it is the right container
	if runtime.Get(container.Id) != container {
		t.Errorf("Get() returned the wrong container")
	}

	// Make sure Exists returns it as existing
	if !runtime.Exists(container.Id) {
		t.Errorf("Exists() returned false for a newly created container")
	}
}

func TestDestroy(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)
	container, err := runtime.Create(&Config{
		Image: GetTestImage(runtime).Id,
		Cmd:   []string{"ls", "-al"},
	},
	)
	if err != nil {
		t.Fatal(err)
	}
	// Destroy
	if err := runtime.Destroy(container); err != nil {
		t.Error(err)
	}

	// Make sure runtime.Exists() behaves correctly
	if runtime.Exists("test_destroy") {
		t.Errorf("Exists() returned true")
	}

	// Make sure runtime.List() doesn't list the destroyed container
	if len(runtime.List()) != 0 {
		t.Errorf("Expected 0 container, %v found", len(runtime.List()))
	}

	// Make sure runtime.Get() refuses to return the unexisting container
	if runtime.Get(container.Id) != nil {
		t.Errorf("Unable to get newly created container")
	}

	// Make sure the container root directory does not exist anymore
	_, err = os.Stat(container.root)
	if err == nil || !os.IsNotExist(err) {
		t.Errorf("Container root directory still exists after destroy")
	}

	// Test double destroy
	if err := runtime.Destroy(container); err == nil {
		// It should have failed
		t.Errorf("Double destroy did not fail")
	}
}

func TestGet(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)
	container1, err := runtime.Create(&Config{
		Image: GetTestImage(runtime).Id,
		Cmd:   []string{"ls", "-al"},
	},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container1)

	container2, err := runtime.Create(&Config{
		Image: GetTestImage(runtime).Id,
		Cmd:   []string{"ls", "-al"},
	},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container2)

	container3, err := runtime.Create(&Config{
		Image: GetTestImage(runtime).Id,
		Cmd:   []string{"ls", "-al"},
	},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container3)

	if runtime.Get(container1.Id) != container1 {
		t.Errorf("Get(test1) returned %v while expecting %v", runtime.Get(container1.Id), container1)
	}

	if runtime.Get(container2.Id) != container2 {
		t.Errorf("Get(test2) returned %v while expecting %v", runtime.Get(container2.Id), container2)
	}

	if runtime.Get(container3.Id) != container3 {
		t.Errorf("Get(test3) returned %v while expecting %v", runtime.Get(container3.Id), container3)
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

	runtime1, err := NewRuntimeFromDirectory(root)
	if err != nil {
		t.Fatal(err)
	}

	// Create a container with one instance of docker
	container1, err := runtime1.Create(&Config{
		Image: GetTestImage(runtime1).Id,
		Cmd:   []string{"ls", "-al"},
	},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime1.Destroy(container1)

	// Create a second container meant to be killed
	container1_1, err := runtime1.Create(&Config{
		Image:     GetTestImage(runtime1).Id,
		Cmd:       []string{"/bin/cat"},
		OpenStdin: true,
	},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime1.Destroy(container1_1)

	// Start the container non blocking
	if err := container1_1.Start(); err != nil {
		t.Fatal(err)
	}

	if !container1_1.State.Running {
		t.Fatalf("Container %v should appear as running but isn't", container1_1.Id)
	}

	// Simulate a crash/manual quit of dockerd: process dies, states stays 'Running'
	if err := container1_1.Stop(); err != nil {
		t.Fatalf("Could not stop container: %v", err)
	}

	container1_1.State.Running = true

	if len(runtime1.List()) != 2 {
		t.Errorf("Expected 2 container, %v found", len(runtime1.List()))
	}
	if err := container1.Run(); err != nil {
		t.Fatal(err)
	}

	if !container1_1.State.Running {
		t.Fatalf("Container %v should appear as running but isn't", container1_1.Id)
	}

	// Here are are simulating a docker restart - that is, reloading all containers
	// from scratch
	runtime2, err := NewRuntimeFromDirectory(root)
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime2)
	if len(runtime2.List()) != 2 {
		t.Errorf("Expected 2 container, %v found", len(runtime2.List()))
	}
	runningCount := 0
	for _, c := range runtime2.List() {
		if c.State.Running {
			t.Errorf("Running container found: %v (%v)", c.Id, c.Path)
			runningCount++
		}
	}
	if runningCount != 0 {
		t.Fatalf("Expected 0 container alive, %d found", runningCount)
	}
	container2 := runtime2.Get(container1.Id)
	if container2 == nil {
		t.Fatal("Unable to Get container")
	}
	if err := container2.Run(); err != nil {
		t.Fatal(err)
	}
}
