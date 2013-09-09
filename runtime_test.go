package docker

import (
	"bytes"
	"fmt"
	"github.com/dotcloud/docker/utils"
	"io"
	"log"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

const (
	unitTestImageName     = "docker-test-image"
	unitTestImageID       = "83599e29c455eb719f77d799bc7c51521b9551972f5a850d7ad265bc1b5292f6" // 1.0
	unitTestNetworkBridge = "testdockbr0"
	unitTestStoreBase     = "/var/lib/docker/unit-tests"
	testDaemonAddr        = "127.0.0.1:4270"
	testDaemonProto       = "tcp"
)

var (
	globalRuntime   *Runtime
	startFds        int
	startGoroutines int
)

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

func cleanup(runtime *Runtime) error {
	for _, container := range runtime.List() {
		container.Kill()
		runtime.Destroy(container)
	}
	images, err := runtime.graph.All()
	if err != nil {
		return err
	}
	for _, image := range images {
		if image.ID != unitTestImageID {
			runtime.graph.Delete(image.ID)
		}
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
	os.Setenv("TEST", "1")

	// Hack to run sys init during unit testing
	if selfPath := utils.SelfPath(); selfPath == "/sbin/init" || selfPath == "/.dockerinit" {
		SysInit()
		return
	}

	if uid := syscall.Geteuid(); uid != 0 {
		log.Fatal("docker tests need to be run as root")
	}

	NetworkBridgeIface = unitTestNetworkBridge

	// Make it our Store root
	if runtime, err := NewRuntimeFromDirectory(unitTestStoreBase, false); err != nil {
		panic(err)
	} else {
		globalRuntime = runtime
	}

	// Create the "Server"
	srv := &Server{
		runtime:     globalRuntime,
		enableCors:  false,
		pullingPool: make(map[string]struct{}),
		pushingPool: make(map[string]struct{}),
	}
	// If the unit test is not found, try to download it.
	if img, err := globalRuntime.repositories.LookupImage(unitTestImageName); err != nil || img.ID != unitTestImageID {
		// Retrieve the Image
		if err := srv.ImagePull(unitTestImageName, "", os.Stdout, utils.NewStreamFormatter(false), nil, nil, true); err != nil {
			panic(err)
		}
	}
	// Spawn a Daemon
	go func() {
		if err := ListenAndServe(testDaemonProto, testDaemonAddr, srv, os.Getenv("DEBUG") != ""); err != nil {
			panic(err)
		}
	}()

	// Give some time to ListenAndServer to actually start
	time.Sleep(time.Second)

	startFds, startGoroutines = utils.GetTotalUsedFds(), runtime.NumGoroutine()
}

// FIXME: test that ImagePull(json=true) send correct json output

func GetTestImage(runtime *Runtime) *Image {
	imgs, err := runtime.graph.All()
	if err != nil {
		panic(err)
	}
	for i := range imgs {
		if imgs[i].ID == unitTestImageID {
			return imgs[i]
		}
	}
	panic(fmt.Errorf("Test image %v not found", unitTestImageID))
}

func TestRuntimeCreate(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)

	// Make sure we start we 0 containers
	if len(runtime.List()) != 0 {
		t.Errorf("Expected 0 containers, %v found", len(runtime.List()))
	}

	container, err := runtime.Create(&Config{
		Image: GetTestImage(runtime).ID,
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
	if runtime.List()[0].ID != container.ID {
		t.Errorf("Unexpected container %v returned by List", runtime.List()[0])
	}

	// Make sure we can get the container with Get()
	if runtime.Get(container.ID) == nil {
		t.Errorf("Unable to get newly created container")
	}

	// Make sure it is the right container
	if runtime.Get(container.ID) != container {
		t.Errorf("Get() returned the wrong container")
	}

	// Make sure Exists returns it as existing
	if !runtime.Exists(container.ID) {
		t.Errorf("Exists() returned false for a newly created container")
	}

	// Make sure crete with bad parameters returns an error
	_, err = runtime.Create(
		&Config{
			Image: GetTestImage(runtime).ID,
		},
	)
	if err == nil {
		t.Fatal("Builder.Create should throw an error when Cmd is missing")
	}

	_, err = runtime.Create(
		&Config{
			Image: GetTestImage(runtime).ID,
			Cmd:   []string{},
		},
	)
	if err == nil {
		t.Fatal("Builder.Create should throw an error when Cmd is empty")
	}
}

func TestDestroy(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)
	container, err := runtime.Create(&Config{
		Image: GetTestImage(runtime).ID,
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
	if runtime.Get(container.ID) != nil {
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
	runtime := mkRuntime(t)
	defer nuke(runtime)

	container1, _, _ := mkContainer(runtime, []string{"_", "ls", "-al"}, t)
	defer runtime.Destroy(container1)

	container2, _, _ := mkContainer(runtime, []string{"_", "ls", "-al"}, t)
	defer runtime.Destroy(container2)

	container3, _, _ := mkContainer(runtime, []string{"_", "ls", "-al"}, t)
	defer runtime.Destroy(container3)

	if runtime.Get(container1.ID) != container1 {
		t.Errorf("Get(test1) returned %v while expecting %v", runtime.Get(container1.ID), container1)
	}

	if runtime.Get(container2.ID) != container2 {
		t.Errorf("Get(test2) returned %v while expecting %v", runtime.Get(container2.ID), container2)
	}

	if runtime.Get(container3.ID) != container3 {
		t.Errorf("Get(test3) returned %v while expecting %v", runtime.Get(container3.ID), container3)
	}

}

func startEchoServerContainer(t *testing.T, proto string) (*Runtime, *Container, string) {
	var err error
	runtime := mkRuntime(t)
	port := 5554
	var container *Container
	var strPort string
	for {
		port += 1
		strPort = strconv.Itoa(port)
		var cmd string
		if proto == "tcp" {
			cmd = "socat TCP-LISTEN:" + strPort + ",reuseaddr,fork EXEC:/bin/cat"
		} else if proto == "udp" {
			cmd = "socat UDP-RECVFROM:" + strPort + ",fork EXEC:/bin/cat"
		} else {
			t.Fatal(fmt.Errorf("Unknown protocol %v", proto))
		}
		t.Log("Trying port", strPort)
		container, err = runtime.Create(&Config{
			Image:     GetTestImage(runtime).ID,
			Cmd:       []string{"sh", "-c", cmd},
			PortSpecs: []string{fmt.Sprintf("%s/%s", strPort, proto)},
		})
		if container != nil {
			break
		}
		if err != nil {
			nuke(runtime)
			t.Fatal(err)
		}
		t.Logf("Port %v already in use", strPort)
	}

	hostConfig := &HostConfig{}
	if err := container.Start(hostConfig); err != nil {
		nuke(runtime)
		t.Fatal(err)
	}

	setTimeout(t, "Waiting for the container to be started timed out", 2*time.Second, func() {
		for !container.State.Running {
			time.Sleep(10 * time.Millisecond)
		}
	})

	// Even if the state is running, lets give some time to lxc to spawn the process
	container.WaitTimeout(500 * time.Millisecond)

	strPort = container.NetworkSettings.PortMapping[strings.Title(proto)][strPort]
	return runtime, container, strPort
}

// Run a container with a TCP port allocated, and test that it can receive connections on localhost
func TestAllocateTCPPortLocalhost(t *testing.T) {
	runtime, container, port := startEchoServerContainer(t, "tcp")
	defer nuke(runtime)
	defer container.Kill()

	for i := 0; i != 10; i++ {
		conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%v", port))
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()

		input := bytes.NewBufferString("well hello there\n")
		_, err = conn.Write(input.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		buf := make([]byte, 16)
		read := 0
		conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		read, err = conn.Read(buf)
		if err != nil {
			if err, ok := err.(*net.OpError); ok {
				if err.Err == syscall.ECONNRESET {
					t.Logf("Connection reset by the proxy, socat is probably not listening yet, trying again in a sec")
					conn.Close()
					time.Sleep(time.Second)
					continue
				}
				if err.Timeout() {
					t.Log("Timeout, trying again")
					conn.Close()
					continue
				}
			}
			t.Fatal(err)
		}
		output := string(buf[:read])
		if !strings.Contains(output, "well hello there") {
			t.Fatal(fmt.Errorf("[%v] doesn't contain [well hello there]", output))
		} else {
			return
		}
	}

	t.Fatal("No reply from the container")
}

// Run a container with an UDP port allocated, and test that it can receive connections on localhost
func TestAllocateUDPPortLocalhost(t *testing.T) {
	runtime, container, port := startEchoServerContainer(t, "udp")
	defer nuke(runtime)
	defer container.Kill()

	conn, err := net.Dial("udp", fmt.Sprintf("localhost:%v", port))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	input := bytes.NewBufferString("well hello there\n")
	buf := make([]byte, 16)
	// Try for a minute, for some reason the select in socat may take ages
	// to return even though everything on the path seems fine (i.e: the
	// UDPProxy forwards the traffic correctly and you can see the packets
	// on the interface from within the container).
	for i := 0; i != 120; i++ {
		_, err := conn.Write(input.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		read, err := conn.Read(buf)
		if err == nil {
			output := string(buf[:read])
			if strings.Contains(output, "well hello there") {
				return
			}
		}
	}

	t.Fatal("No reply from the container")
}

func TestRestore(t *testing.T) {
	runtime1 := mkRuntime(t)
	defer nuke(runtime1)
	// Create a container with one instance of docker
	container1, _, _ := mkContainer(runtime1, []string{"_", "ls", "-al"}, t)
	defer runtime1.Destroy(container1)

	// Create a second container meant to be killed
	container2, _, _ := mkContainer(runtime1, []string{"-i", "_", "/bin/cat"}, t)
	defer runtime1.Destroy(container2)

	// Start the container non blocking
	hostConfig := &HostConfig{}
	if err := container2.Start(hostConfig); err != nil {
		t.Fatal(err)
	}

	if !container2.State.Running {
		t.Fatalf("Container %v should appear as running but isn't", container2.ID)
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
		t.Fatalf("Container %v should appear as running but isn't", container2.ID)
	}

	// Here are are simulating a docker restart - that is, reloading all containers
	// from scratch
	runtime2, err := NewRuntimeFromDirectory(runtime1.root, false)
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
			t.Errorf("Running container found: %v (%v)", c.ID, c.Path)
			runningCount++
		}
	}
	if runningCount != 0 {
		t.Fatalf("Expected 0 container alive, %d found", runningCount)
	}
	container3 := runtime2.Get(container1.ID)
	if container3 == nil {
		t.Fatal("Unable to Get container")
	}
	if err := container3.Run(); err != nil {
		t.Fatal(err)
	}
	container2.State.Running = false
}
