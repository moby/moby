package docker

import (
	"fmt"
	"github.com/dotcloud/docker/registry"
	"github.com/dotcloud/docker/utils"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"os/user"
	"sync"
	"testing"
	"time"
)

const unitTestImageName string = "docker-ut"

const unitTestStoreBase string = "/var/lib/docker/unit-tests"

func nuke(runtime *Runtime) error {
	var wg sync.WaitGroup
	for _, container := range runtime.List() {
		wg.Add(1)
		go func(c *Container) {
			c.Kill()
			wg.Done()
		}(container)
	}
	wg.Wait()
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
	if utils.SelfPath() == "/sbin/init" {
		SysInit()
		return
	}

	if usr, err := user.Current(); err != nil {
		panic(err)
	} else if usr.Uid != "0" {
		panic("docker tests needs to be run as root")
	}

	NetworkBridgeIface = "testdockbr0"

	// Make it our Store root
	runtime, err := NewRuntimeFromDirectory(unitTestStoreBase, false)
	if err != nil {
		panic(err)
	}

	// Create the "Server"
	srv := &Server{
		runtime:  runtime,
		registry: registry.NewRegistry(runtime.root),
	}
	// Retrieve the Image
	if err := srv.ImagePull(unitTestImageName, "", "", os.Stdout, false); err != nil {
		panic(err)
	}
}

// FIXME: test that ImagePull(json=true) send correct json output

func newTestRuntime() (*Runtime, error) {
	root, err := ioutil.TempDir("", "docker-test")
	if err != nil {
		return nil, err
	}
	if err := os.Remove(root); err != nil {
		return nil, err
	}
	if err := CopyDirectory(unitTestStoreBase, root); err != nil {
		return nil, err
	}

	runtime, err := NewRuntimeFromDirectory(root, false)
	if err != nil {
		return nil, err
	}
	runtime.UpdateCapabilities(true)
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

	builder := NewBuilder(runtime)

	container, err := builder.Create(&Config{
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

	// Make sure crete with bad parameters returns an error
	_, err = builder.Create(
		&Config{
			Image: GetTestImage(runtime).Id,
		},
	)
	if err == nil {
		t.Fatal("Builder.Create should throw an error when Cmd is missing")
	}

	_, err = builder.Create(
		&Config{
			Image: GetTestImage(runtime).Id,
			Cmd:   []string{},
		},
	)
	if err == nil {
		t.Fatal("Builder.Create should throw an error when Cmd is empty")
	}
}

func TestDestroy(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)
	container, err := NewBuilder(runtime).Create(&Config{
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

	builder := NewBuilder(runtime)

	container1, err := builder.Create(&Config{
		Image: GetTestImage(runtime).Id,
		Cmd:   []string{"ls", "-al"},
	},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container1)

	container2, err := builder.Create(&Config{
		Image: GetTestImage(runtime).Id,
		Cmd:   []string{"ls", "-al"},
	},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container2)

	container3, err := builder.Create(&Config{
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

// Run a container with a TCP port allocated, and test that it can receive connections on localhost
func TestAllocatePortLocalhost(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	container, err := NewBuilder(runtime).Create(&Config{
		Image:     GetTestImage(runtime).Id,
		Cmd:       []string{"sh", "-c", "echo well hello there | nc -l -p 5555"},
		PortSpecs: []string{"5555"},
	},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := container.Start(); err != nil {
		t.Fatal(err)
	}
	defer container.Kill()

	setTimeout(t, "Waiting for the container to be started timed out", 2*time.Second, func() {
		for {
			if container.State.Running {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
	})

	conn, err := net.Dial("tcp",
		fmt.Sprintf(
			"localhost:%s", container.NetworkSettings.PortMapping["5555"],
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	output, err := ioutil.ReadAll(conn)
	if err != nil {
		t.Fatal(err)
	}
	if string(output) != "well hello there\n" {
		t.Fatalf("Received wrong output from network connection: should be '%s', not '%s'",
			"well hello there\n",
			string(output),
		)
	}
	container.Wait()
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

	runtime1, err := NewRuntimeFromDirectory(root, false)
	if err != nil {
		t.Fatal(err)
	}

	builder := NewBuilder(runtime1)

	// Create a container with one instance of docker
	container1, err := builder.Create(&Config{
		Image: GetTestImage(runtime1).Id,
		Cmd:   []string{"ls", "-al"},
	},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime1.Destroy(container1)

	// Create a second container meant to be killed
	container2, err := builder.Create(&Config{
		Image:     GetTestImage(runtime1).Id,
		Cmd:       []string{"/bin/cat"},
		OpenStdin: true,
	},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime1.Destroy(container2)

	// Start the container non blocking
	if err := container2.Start(); err != nil {
		t.Fatal(err)
	}

	if !container2.State.Running {
		t.Fatalf("Container %v should appear as running but isn't", container2.Id)
	}

	// Simulate a crash/manual quit of dockerd: process dies, states stays 'Running'
	cStdin, _ := container2.StdinPipe()
	cStdin.Close()
	if err := container2.WaitTimeout(2 * time.Second); err != nil {
		t.Fatal(err)
	}
	container2.State.Running = true
	container2.ToDisk()

	if len(runtime1.List()) != 2 {
		t.Errorf("Expected 2 container, %v found", len(runtime1.List()))
	}
	if err := container1.Run(); err != nil {
		t.Fatal(err)
	}

	if !container2.State.Running {
		t.Fatalf("Container %v should appear as running but isn't", container2.Id)
	}

	// Here are are simulating a docker restart - that is, reloading all containers
	// from scratch
	runtime2, err := NewRuntimeFromDirectory(root, false)
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
	container3 := runtime2.Get(container1.Id)
	if container3 == nil {
		t.Fatal("Unable to Get container")
	}
	if err := container3.Run(); err != nil {
		t.Fatal(err)
	}
	container2.State.Running = false
}
