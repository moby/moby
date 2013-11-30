package docker

import (
	"bytes"
	"fmt"
	"github.com/dotcloud/docker"
	"github.com/dotcloud/docker/engine"
	"github.com/dotcloud/docker/sysinit"
	"github.com/dotcloud/docker/utils"
	"io"
	"log"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

const (
	unitTestImageName     = "docker-test-image"
	unitTestImageID       = "83599e29c455eb719f77d799bc7c51521b9551972f5a850d7ad265bc1b5292f6" // 1.0
	unitTestImageIDShort  = "83599e29c455"
	unitTestNetworkBridge = "testdockbr0"
	unitTestStoreBase     = "/var/lib/docker/unit-tests"
	testDaemonAddr        = "127.0.0.1:4270"
	testDaemonProto       = "tcp"
)

var (
	// FIXME: globalRuntime is deprecated by globalEngine. All tests should be converted.
	globalRuntime   *docker.Runtime
	globalEngine    *engine.Engine
	startFds        int
	startGoroutines int
)

// FIXME: nuke() is deprecated by Runtime.Nuke()
func nuke(runtime *docker.Runtime) error {
	return runtime.Nuke()
}

// FIXME: cleanup and nuke are redundant.
func cleanup(eng *engine.Engine, t *testing.T) error {
	runtime := mkRuntimeFromEngine(eng, t)
	for _, container := range runtime.List() {
		container.Kill()
		runtime.Destroy(container)
	}
	srv := mkServerFromEngine(eng, t)
	images, err := srv.Images(true, "")
	if err != nil {
		return err
	}
	for _, image := range images {
		if image.ID != unitTestImageID {
			srv.ImageDelete(image.ID, false)
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
	// Always use the same driver (vfs) for all integration tests.
	// To test other drivers, we need a dedicated driver validation suite.
	os.Setenv("DOCKER_DRIVER", "vfs")
	os.Setenv("TEST", "1")

	// Hack to run sys init during unit testing
	if selfPath := utils.SelfPath(); selfPath == "/sbin/init" || selfPath == "/.dockerinit" {
		sysinit.SysInit()
		return
	}

	if uid := syscall.Geteuid(); uid != 0 {
		log.Fatal("docker tests need to be run as root")
	}

	// Copy dockerinit into our current testing directory, if provided (so we can test a separate dockerinit binary)
	if dockerinit := os.Getenv("TEST_DOCKERINIT_PATH"); dockerinit != "" {
		src, err := os.Open(dockerinit)
		if err != nil {
			log.Fatalf("Unable to open TEST_DOCKERINIT_PATH: %s\n", err)
		}
		defer src.Close()
		dst, err := os.OpenFile(filepath.Join(filepath.Dir(utils.SelfPath()), "dockerinit"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0555)
		if err != nil {
			log.Fatalf("Unable to create dockerinit in test directory: %s\n", err)
		}
		defer dst.Close()
		if _, err := io.Copy(dst, src); err != nil {
			log.Fatalf("Unable to copy dockerinit to TEST_DOCKERINIT_PATH: %s\n", err)
		}
		dst.Close()
		src.Close()
	}

	// Setup the base runtime, which will be duplicated for each test.
	// (no tests are run directly in the base)
	setupBaseImage()

	// Create the "global runtime" with a long-running daemon for integration tests
	spawnGlobalDaemon()
	startFds, startGoroutines = utils.GetTotalUsedFds(), runtime.NumGoroutine()
}

func setupBaseImage() {
	eng, err := engine.New(unitTestStoreBase)
	if err != nil {
		log.Fatalf("Can't initialize engine at %s: %s", unitTestStoreBase, err)
	}
	job := eng.Job("initapi")
	job.Setenv("Root", unitTestStoreBase)
	job.SetenvBool("Autorestart", false)
	job.Setenv("BridgeIface", unitTestNetworkBridge)
	if err := job.Run(); err != nil {
		log.Fatalf("Unable to create a runtime for tests:", err)
	}
	srv := mkServerFromEngine(eng, log.New(os.Stderr, "", 0))

	// If the unit test is not found, try to download it.
	if img, err := srv.ImageInspect(unitTestImageName); err != nil || img.ID != unitTestImageID {
		// Retrieve the Image
		if err := srv.ImagePull(unitTestImageName, "", os.Stdout, utils.NewStreamFormatter(false), nil, nil, true); err != nil {
			log.Fatalf("Unable to pull the test image: %s", err)
		}
	}
}

func spawnGlobalDaemon() {
	if globalRuntime != nil {
		utils.Debugf("Global runtime already exists. Skipping.")
		return
	}
	t := log.New(os.Stderr, "", 0)
	eng := NewTestEngine(t)
	globalEngine = eng
	globalRuntime = mkRuntimeFromEngine(eng, t)

	// Spawn a Daemon
	go func() {
		utils.Debugf("Spawning global daemon for integration tests")
		listenURL := &url.URL{
			Scheme: testDaemonProto,
			Host:   testDaemonAddr,
		}
		job := eng.Job("serveapi", listenURL.String())
		job.SetenvBool("Logging", os.Getenv("DEBUG") != "")
		if err := job.Run(); err != nil {
			log.Fatalf("Unable to spawn the test daemon: %s", err)
		}
	}()
	// Give some time to ListenAndServer to actually start
	// FIXME: use inmem transports instead of tcp
	time.Sleep(time.Second)
}

// FIXME: test that ImagePull(json=true) send correct json output

func GetTestImage(runtime *docker.Runtime) *docker.Image {
	imgs, err := runtime.Graph().Map()
	if err != nil {
		log.Fatalf("Unable to get the test image:", err)
	}
	for _, image := range imgs {
		if image.ID == unitTestImageID {
			return image
		}
	}
	log.Fatalf("Test image %v not found in %s: %s", unitTestImageID, runtime.Graph().Root, imgs)
	return nil
}

func TestRuntimeCreate(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)

	// Make sure we start we 0 containers
	if len(runtime.List()) != 0 {
		t.Errorf("Expected 0 containers, %v found", len(runtime.List()))
	}

	container, _, err := runtime.Create(&docker.Config{
		Image: GetTestImage(runtime).ID,
		Cmd:   []string{"ls", "-al"},
	},
		"",
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

	// Test that conflict error displays correct details
	testContainer, _, _ := runtime.Create(
		&docker.Config{
			Image: GetTestImage(runtime).ID,
			Cmd:   []string{"ls", "-al"},
		},
		"conflictname",
	)
	if _, _, err := runtime.Create(&docker.Config{Image: GetTestImage(runtime).ID, Cmd: []string{"ls", "-al"}}, testContainer.Name); err == nil || !strings.Contains(err.Error(), utils.TruncateID(testContainer.ID)) {
		t.Fatalf("Name conflict error doesn't include the correct short id. Message was: %s", err.Error())
	}

	// Make sure create with bad parameters returns an error
	if _, _, err = runtime.Create(&docker.Config{Image: GetTestImage(runtime).ID}, ""); err == nil {
		t.Fatal("Builder.Create should throw an error when Cmd is missing")
	}

	if _, _, err := runtime.Create(
		&docker.Config{
			Image: GetTestImage(runtime).ID,
			Cmd:   []string{},
		},
		"",
	); err == nil {
		t.Fatal("Builder.Create should throw an error when Cmd is empty")
	}

	config := &docker.Config{
		Image:     GetTestImage(runtime).ID,
		Cmd:       []string{"/bin/ls"},
		PortSpecs: []string{"80"},
	}
	container, _, err = runtime.Create(config, "")

	_, err = runtime.Commit(container, "testrepo", "testtag", "", "", config)
	if err != nil {
		t.Error(err)
	}

	// test expose 80:8000
	container, warnings, err := runtime.Create(&docker.Config{
		Image:     GetTestImage(runtime).ID,
		Cmd:       []string{"ls", "-al"},
		PortSpecs: []string{"80:8000"},
	},
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	if warnings == nil || len(warnings) != 1 {
		t.Error("Expected a warning, got none")
	}
}

func TestDestroy(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)

	container, _, err := runtime.Create(&docker.Config{
		Image: GetTestImage(runtime).ID,
		Cmd:   []string{"ls", "-al"},
	}, "")
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

func startEchoServerContainer(t *testing.T, proto string) (*docker.Runtime, *docker.Container, string) {
	var (
		err     error
		id      string
		strPort string
		eng     = NewTestEngine(t)
		runtime = mkRuntimeFromEngine(eng, t)
		port    = 5554
		p       docker.Port
	)
	defer func() {
		if err != nil {
			runtime.Nuke()
		}
	}()

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
		ep := make(map[docker.Port]struct{}, 1)
		p = docker.Port(fmt.Sprintf("%s/%s", strPort, proto))
		ep[p] = struct{}{}

		jobCreate := eng.Job("create")
		jobCreate.Setenv("Image", unitTestImageID)
		jobCreate.SetenvList("Cmd", []string{"sh", "-c", cmd})
		jobCreate.SetenvList("PortSpecs", []string{fmt.Sprintf("%s/%s", strPort, proto)})
		jobCreate.SetenvJson("ExposedPorts", ep)
		jobCreate.Stdout.AddString(&id)
		if err := jobCreate.Run(); err != nil {
			t.Fatal(err)
		}
		// FIXME: this relies on the undocumented behavior of runtime.Create
		// which will return a nil error AND container if the exposed ports
		// are invalid. That behavior should be fixed!
		if id != "" {
			break
		}
		t.Logf("Port %v already in use, trying another one", strPort)

	}

	jobStart := eng.Job("start", id)
	portBindings := make(map[docker.Port][]docker.PortBinding)
	portBindings[p] = []docker.PortBinding{
		{},
	}
	if err := jobStart.SetenvJson("PortsBindings", portBindings); err != nil {
		t.Fatal(err)
	}
	if err := jobStart.Run(); err != nil {
		t.Fatal(err)
	}

	container := runtime.Get(id)
	if container == nil {
		t.Fatalf("Couldn't fetch test container %s", id)
	}

	setTimeout(t, "Waiting for the container to be started timed out", 2*time.Second, func() {
		for !container.State.IsRunning() {
			time.Sleep(10 * time.Millisecond)
		}
	})

	// Even if the state is running, lets give some time to lxc to spawn the process
	container.WaitTimeout(500 * time.Millisecond)

	strPort = container.NetworkSettings.Ports[p][0].HostPort
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
	eng := NewTestEngine(t)
	runtime1 := mkRuntimeFromEngine(eng, t)
	defer runtime1.Nuke()
	// Create a container with one instance of docker
	container1, _, _ := mkContainer(runtime1, []string{"_", "ls", "-al"}, t)
	defer runtime1.Destroy(container1)

	// Create a second container meant to be killed
	container2, _, _ := mkContainer(runtime1, []string{"-i", "_", "/bin/cat"}, t)
	defer runtime1.Destroy(container2)

	// Start the container non blocking
	if err := container2.Start(); err != nil {
		t.Fatal(err)
	}

	if !container2.State.IsRunning() {
		t.Fatalf("Container %v should appear as running but isn't", container2.ID)
	}

	// Simulate a crash/manual quit of dockerd: process dies, states stays 'Running'
	cStdin, _ := container2.StdinPipe()
	cStdin.Close()
	if err := container2.WaitTimeout(2 * time.Second); err != nil {
		t.Fatal(err)
	}
	container2.State.SetRunning(42)
	container2.ToDisk()

	if len(runtime1.List()) != 2 {
		t.Errorf("Expected 2 container, %v found", len(runtime1.List()))
	}
	if err := container1.Run(); err != nil {
		t.Fatal(err)
	}

	if !container2.State.IsRunning() {
		t.Fatalf("Container %v should appear as running but isn't", container2.ID)
	}

	// Here are are simulating a docker restart - that is, reloading all containers
	// from scratch
	root := eng.Root()
	eng, err := engine.New(root)
	if err != nil {
		t.Fatal(err)
	}
	job := eng.Job("initapi")
	job.Setenv("Root", eng.Root())
	job.SetenvBool("Autorestart", false)
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}

	runtime2 := mkRuntimeFromEngine(eng, t)
	if len(runtime2.List()) != 2 {
		t.Errorf("Expected 2 container, %v found", len(runtime2.List()))
	}
	runningCount := 0
	for _, c := range runtime2.List() {
		if c.State.IsRunning() {
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
	container2.State.SetStopped(0)
}

func TestReloadContainerLinks(t *testing.T) {
	// FIXME: here we don't use NewTestEngine because it calls initapi with Autorestart=false,
	// and we want to set it to true.
	root, err := newTestDirectory(unitTestStoreBase)
	if err != nil {
		t.Fatal(err)
	}
	eng, err := engine.New(root)
	if err != nil {
		t.Fatal(err)
	}
	job := eng.Job("initapi")
	job.Setenv("Root", eng.Root())
	job.SetenvBool("Autorestart", true)
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}

	runtime1 := mkRuntimeFromEngine(eng, t)
	defer nuke(runtime1)
	// Create a container with one instance of docker
	container1, _, _ := mkContainer(runtime1, []string{"-i", "_", "/bin/sh"}, t)
	defer runtime1.Destroy(container1)

	// Create a second container meant to be killed
	container2, _, _ := mkContainer(runtime1, []string{"-i", "_", "/bin/cat"}, t)
	defer runtime1.Destroy(container2)

	// Start the container non blocking
	if err := container2.Start(); err != nil {
		t.Fatal(err)
	}
	// Add a link to container 2
	// FIXME @shykes: setting hostConfig.Links seems redundant with calling RegisterLink().
	// Why do we need it @crosbymichael?
	// container1.hostConfig.Links = []string{"/" + container2.ID + ":first"}
	if err := runtime1.RegisterLink(container1, container2, "first"); err != nil {
		t.Fatal(err)
	}
	if err := container1.Start(); err != nil {
		t.Fatal(err)
	}

	if !container2.State.IsRunning() {
		t.Fatalf("Container %v should appear as running but isn't", container2.ID)
	}

	if !container1.State.IsRunning() {
		t.Fatalf("Container %s should appear as running but isn't", container1.ID)
	}

	if len(runtime1.List()) != 2 {
		t.Errorf("Expected 2 container, %v found", len(runtime1.List()))
	}

	// Here are are simulating a docker restart - that is, reloading all containers
	// from scratch
	eng, err = engine.New(root)
	if err != nil {
		t.Fatal(err)
	}
	job = eng.Job("initapi")
	job.Setenv("Root", eng.Root())
	job.SetenvBool("Autorestart", false)
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}

	runtime2 := mkRuntimeFromEngine(eng, t)
	if len(runtime2.List()) != 2 {
		t.Errorf("Expected 2 container, %v found", len(runtime2.List()))
	}
	runningCount := 0
	for _, c := range runtime2.List() {
		if c.State.IsRunning() {
			runningCount++
		}
	}
	if runningCount != 2 {
		t.Fatalf("Expected 2 container alive, %d found", runningCount)
	}

	// FIXME: we no longer test if containers were registered in the right order,
	// because there is no public
	// Make sure container 2 ( the child of container 1 ) was registered and started first
	// with the runtime
	//
	containers := runtime2.List()
	if len(containers) == 0 {
		t.Fatalf("Runtime has no containers")
	}
	first := containers[0]
	if first.ID != container2.ID {
		t.Fatalf("Container 2 %s should be registered first in the runtime", container2.ID)
	}

	// Verify that the link is still registered in the runtime
	if c := runtime2.Get(container1.Name); c == nil {
		t.Fatal("Named container is no longer registered after restart")
	}
}

func TestDefaultContainerName(t *testing.T) {
	eng := NewTestEngine(t)
	runtime := mkRuntimeFromEngine(eng, t)
	defer nuke(runtime)

	config, _, _, err := docker.ParseRun([]string{unitTestImageID, "echo test"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	container := runtime.Get(createNamedTestContainer(eng, config, t, "some_name"))
	containerID := container.ID

	if container.Name != "/some_name" {
		t.Fatalf("Expect /some_name got %s", container.Name)
	}

	if c := runtime.Get("/some_name"); c == nil {
		t.Fatalf("Couldn't retrieve test container as /some_name")
	} else if c.ID != containerID {
		t.Fatalf("Container /some_name has ID %s instead of %s", c.ID, containerID)
	}
}

func TestRandomContainerName(t *testing.T) {
	eng := NewTestEngine(t)
	runtime := mkRuntimeFromEngine(eng, t)
	defer nuke(runtime)

	config, _, _, err := docker.ParseRun([]string{GetTestImage(runtime).ID, "echo test"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	container := runtime.Get(createTestContainer(eng, config, t))
	containerID := container.ID

	if container.Name == "" {
		t.Fatalf("Expected not empty container name")
	}

	if c := runtime.Get(container.Name); c == nil {
		log.Fatalf("Could not lookup container %s by its name", container.Name)
	} else if c.ID != containerID {
		log.Fatalf("Looking up container name %s returned id %s instead of %s", container.Name, c.ID, containerID)
	}
}

func TestLinkChildContainer(t *testing.T) {
	eng := NewTestEngine(t)
	runtime := mkRuntimeFromEngine(eng, t)
	defer nuke(runtime)

	config, _, _, err := docker.ParseRun([]string{unitTestImageID, "echo test"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	container := runtime.Get(createNamedTestContainer(eng, config, t, "/webapp"))

	webapp, err := runtime.GetByName("/webapp")
	if err != nil {
		t.Fatal(err)
	}

	if webapp.ID != container.ID {
		t.Fatalf("Expect webapp id to match container id: %s != %s", webapp.ID, container.ID)
	}

	config, _, _, err = docker.ParseRun([]string{GetTestImage(runtime).ID, "echo test"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	childContainer := runtime.Get(createTestContainer(eng, config, t))

	if err := runtime.RegisterLink(webapp, childContainer, "db"); err != nil {
		t.Fatal(err)
	}

	// Get the child by it's new name
	db, err := runtime.GetByName("/webapp/db")
	if err != nil {
		t.Fatal(err)
	}
	if db.ID != childContainer.ID {
		t.Fatalf("Expect db id to match container id: %s != %s", db.ID, childContainer.ID)
	}
}

func TestGetAllChildren(t *testing.T) {
	eng := NewTestEngine(t)
	runtime := mkRuntimeFromEngine(eng, t)
	defer nuke(runtime)

	config, _, _, err := docker.ParseRun([]string{unitTestImageID, "echo test"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	container := runtime.Get(createNamedTestContainer(eng, config, t, "/webapp"))

	webapp, err := runtime.GetByName("/webapp")
	if err != nil {
		t.Fatal(err)
	}

	if webapp.ID != container.ID {
		t.Fatalf("Expect webapp id to match container id: %s != %s", webapp.ID, container.ID)
	}

	config, _, _, err = docker.ParseRun([]string{unitTestImageID, "echo test"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	childContainer := runtime.Get(createTestContainer(eng, config, t))

	if err := runtime.RegisterLink(webapp, childContainer, "db"); err != nil {
		t.Fatal(err)
	}

	children, err := runtime.Children("/webapp")
	if err != nil {
		t.Fatal(err)
	}

	if children == nil {
		t.Fatal("Children should not be nil")
	}
	if len(children) == 0 {
		t.Fatal("Children should not be empty")
	}

	for key, value := range children {
		if key != "/webapp/db" {
			t.Fatalf("Expected /webapp/db got %s", key)
		}
		if value.ID != childContainer.ID {
			t.Fatalf("Expected id %s got %s", childContainer.ID, value.ID)
		}
	}
}
