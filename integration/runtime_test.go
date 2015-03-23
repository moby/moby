package docker

import (
	"bytes"
	"fmt"
	"io"
	std_log "log"
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

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon"
	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/image"
	"github.com/docker/docker/nat"
	"github.com/docker/docker/pkg/common"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/reexec"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/utils"
)

const (
	unitTestImageName        = "docker-test-image"
	unitTestImageID          = "83599e29c455eb719f77d799bc7c51521b9551972f5a850d7ad265bc1b5292f6" // 1.0
	unitTestImageIDShort     = "83599e29c455"
	unitTestNetworkBridge    = "testdockbr0"
	unitTestStoreBase        = "/var/lib/docker/unit-tests"
	unitTestDockerTmpdir     = "/var/lib/docker/tmp"
	testDaemonAddr           = "127.0.0.1:4270"
	testDaemonProto          = "tcp"
	testDaemonHttpsProto     = "tcp"
	testDaemonHttpsAddr      = "localhost:4271"
	testDaemonRogueHttpsAddr = "localhost:4272"
)

var (
	// FIXME: globalDaemon is deprecated by globalEngine. All tests should be converted.
	globalDaemon           *daemon.Daemon
	globalEngine           *engine.Engine
	globalHttpsEngine      *engine.Engine
	globalRogueHttpsEngine *engine.Engine
	startFds               int
	startGoroutines        int
)

// FIXME: nuke() is deprecated by Daemon.Nuke()
func nuke(daemon *daemon.Daemon) error {
	return daemon.Nuke()
}

// FIXME: cleanup and nuke are redundant.
func cleanup(eng *engine.Engine, t *testing.T) error {
	daemon := mkDaemonFromEngine(eng, t)
	for _, container := range daemon.List() {
		container.Kill()
		daemon.Rm(container)
	}
	job := eng.Job("images")
	images, err := job.Stdout.AddTable()
	if err != nil {
		t.Fatal(err)
	}
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}
	for _, image := range images.Data {
		if image.Get("Id") != unitTestImageID {
			eng.Job("image_delete", image.Get("Id")).Run()
		}
	}
	return nil
}

func init() {
	// Always use the same driver (vfs) for all integration tests.
	// To test other drivers, we need a dedicated driver validation suite.
	os.Setenv("DOCKER_DRIVER", "vfs")
	os.Setenv("TEST", "1")
	os.Setenv("DOCKER_TMPDIR", unitTestDockerTmpdir)

	// Hack to run sys init during unit testing
	if reexec.Init() {
		return
	}

	if uid := syscall.Geteuid(); uid != 0 {
		log.Fatalf("docker tests need to be run as root")
	}

	// Copy dockerinit into our current testing directory, if provided (so we can test a separate dockerinit binary)
	if dockerinit := os.Getenv("TEST_DOCKERINIT_PATH"); dockerinit != "" {
		src, err := os.Open(dockerinit)
		if err != nil {
			log.Fatalf("Unable to open TEST_DOCKERINIT_PATH: %s", err)
		}
		defer src.Close()
		dst, err := os.OpenFile(filepath.Join(filepath.Dir(utils.SelfPath()), "dockerinit"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0555)
		if err != nil {
			log.Fatalf("Unable to create dockerinit in test directory: %s", err)
		}
		defer dst.Close()
		if _, err := io.Copy(dst, src); err != nil {
			log.Fatalf("Unable to copy dockerinit to TEST_DOCKERINIT_PATH: %s", err)
		}
		dst.Close()
		src.Close()
	}

	// Setup the base daemon, which will be duplicated for each test.
	// (no tests are run directly in the base)
	setupBaseImage()

	// Create the "global daemon" with a long-running daemons for integration tests
	spawnGlobalDaemon()
	spawnLegitHttpsDaemon()
	spawnRogueHttpsDaemon()
	startFds, startGoroutines = utils.GetTotalUsedFds(), runtime.NumGoroutine()
}

func setupBaseImage() {
	eng := newTestEngine(std_log.New(os.Stderr, "", 0), false, unitTestStoreBase)
	job := eng.Job("image_inspect", unitTestImageName)
	img, _ := job.Stdout.AddEnv()
	// If the unit test is not found, try to download it.
	if err := job.Run(); err != nil || img.Get("Id") != unitTestImageID {
		// Retrieve the Image
		job = eng.Job("pull", unitTestImageName)
		job.Stdout.Add(ioutils.NopWriteCloser(os.Stdout))
		if err := job.Run(); err != nil {
			log.Fatalf("Unable to pull the test image: %s", err)
		}
	}
}

func spawnGlobalDaemon() {
	if globalDaemon != nil {
		log.Debugf("Global daemon already exists. Skipping.")
		return
	}
	t := std_log.New(os.Stderr, "", 0)
	eng := NewTestEngine(t)
	globalEngine = eng
	globalDaemon = mkDaemonFromEngine(eng, t)

	// Spawn a Daemon
	go func() {
		log.Debugf("Spawning global daemon for integration tests")
		listenURL := &url.URL{
			Scheme: testDaemonProto,
			Host:   testDaemonAddr,
		}
		job := eng.Job("serveapi", listenURL.String())
		job.SetenvBool("Logging", true)
		if err := job.Run(); err != nil {
			log.Fatalf("Unable to spawn the test daemon: %s", err)
		}
	}()

	// Give some time to ListenAndServer to actually start
	// FIXME: use inmem transports instead of tcp
	time.Sleep(time.Second)

	if err := eng.Job("acceptconnections").Run(); err != nil {
		log.Fatalf("Unable to accept connections for test api: %s", err)
	}
}

func spawnLegitHttpsDaemon() {
	if globalHttpsEngine != nil {
		return
	}
	globalHttpsEngine = spawnHttpsDaemon(testDaemonHttpsAddr, "fixtures/https/ca.pem",
		"fixtures/https/server-cert.pem", "fixtures/https/server-key.pem")
}

func spawnRogueHttpsDaemon() {
	if globalRogueHttpsEngine != nil {
		return
	}
	globalRogueHttpsEngine = spawnHttpsDaemon(testDaemonRogueHttpsAddr, "fixtures/https/ca.pem",
		"fixtures/https/server-rogue-cert.pem", "fixtures/https/server-rogue-key.pem")
}

func spawnHttpsDaemon(addr, cacert, cert, key string) *engine.Engine {
	t := std_log.New(os.Stderr, "", 0)
	root, err := newTestDirectory(unitTestStoreBase)
	if err != nil {
		t.Fatal(err)
	}
	// FIXME: here we don't use NewTestEngine because it configures the daemon with Autorestart=false,
	// and we want to set it to true.

	eng := newTestEngine(t, true, root)

	// Spawn a Daemon
	go func() {
		log.Debugf("Spawning https daemon for integration tests")
		listenURL := &url.URL{
			Scheme: testDaemonHttpsProto,
			Host:   addr,
		}
		job := eng.Job("serveapi", listenURL.String())
		job.SetenvBool("Logging", true)
		job.SetenvBool("Tls", true)
		job.SetenvBool("TlsVerify", true)
		job.Setenv("TlsCa", cacert)
		job.Setenv("TlsCert", cert)
		job.Setenv("TlsKey", key)
		if err := job.Run(); err != nil {
			log.Fatalf("Unable to spawn the test daemon: %s", err)
		}
	}()

	// Give some time to ListenAndServer to actually start
	time.Sleep(time.Second)

	if err := eng.Job("acceptconnections").Run(); err != nil {
		log.Fatalf("Unable to accept connections for test api: %s", err)
	}
	return eng
}

// FIXME: test that ImagePull(json=true) send correct json output

func GetTestImage(daemon *daemon.Daemon) *image.Image {
	imgs, err := daemon.Graph().Map()
	if err != nil {
		log.Fatalf("Unable to get the test image: %s", err)
	}
	for _, image := range imgs {
		if image.ID == unitTestImageID {
			return image
		}
	}
	log.Fatalf("Test image %v not found in %s: %s", unitTestImageID, daemon.Graph().Root, imgs)
	return nil
}

func TestDaemonCreate(t *testing.T) {
	daemon := mkDaemon(t)
	defer nuke(daemon)

	// Make sure we start we 0 containers
	if len(daemon.List()) != 0 {
		t.Errorf("Expected 0 containers, %v found", len(daemon.List()))
	}

	container, _, err := daemon.Create(&runconfig.Config{
		Image: GetTestImage(daemon).ID,
		Cmd:   []string{"ls", "-al"},
	},
		&runconfig.HostConfig{},
		"",
	)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := daemon.Rm(container); err != nil {
			t.Error(err)
		}
	}()

	// Make sure we can find the newly created container with List()
	if len(daemon.List()) != 1 {
		t.Errorf("Expected 1 container, %v found", len(daemon.List()))
	}

	// Make sure the container List() returns is the right one
	if daemon.List()[0].ID != container.ID {
		t.Errorf("Unexpected container %v returned by List", daemon.List()[0])
	}

	// Make sure we can get the container with Get()
	if _, err := daemon.Get(container.ID); err != nil {
		t.Errorf("Unable to get newly created container")
	}

	// Make sure it is the right container
	if c, _ := daemon.Get(container.ID); c != container {
		t.Errorf("Get() returned the wrong container")
	}

	// Make sure Exists returns it as existing
	if !daemon.Exists(container.ID) {
		t.Errorf("Exists() returned false for a newly created container")
	}

	// Test that conflict error displays correct details
	testContainer, _, _ := daemon.Create(
		&runconfig.Config{
			Image: GetTestImage(daemon).ID,
			Cmd:   []string{"ls", "-al"},
		},
		&runconfig.HostConfig{},
		"conflictname",
	)
	if _, _, err := daemon.Create(&runconfig.Config{Image: GetTestImage(daemon).ID, Cmd: []string{"ls", "-al"}}, &runconfig.HostConfig{}, testContainer.Name); err == nil || !strings.Contains(err.Error(), common.TruncateID(testContainer.ID)) {
		t.Fatalf("Name conflict error doesn't include the correct short id. Message was: %v", err)
	}

	// Make sure create with bad parameters returns an error
	if _, _, err = daemon.Create(&runconfig.Config{Image: GetTestImage(daemon).ID}, &runconfig.HostConfig{}, ""); err == nil {
		t.Fatal("Builder.Create should throw an error when Cmd is missing")
	}

	if _, _, err := daemon.Create(
		&runconfig.Config{
			Image: GetTestImage(daemon).ID,
			Cmd:   []string{},
		},
		&runconfig.HostConfig{},
		"",
	); err == nil {
		t.Fatal("Builder.Create should throw an error when Cmd is empty")
	}

	config := &runconfig.Config{
		Image:     GetTestImage(daemon).ID,
		Cmd:       []string{"/bin/ls"},
		PortSpecs: []string{"80"},
	}
	container, _, err = daemon.Create(config, &runconfig.HostConfig{}, "")

	_, err = daemon.Commit(container, "testrepo", "testtag", "", "", true, config)
	if err != nil {
		t.Error(err)
	}

	// test expose 80:8000
	container, warnings, err := daemon.Create(&runconfig.Config{
		Image:     GetTestImage(daemon).ID,
		Cmd:       []string{"ls", "-al"},
		PortSpecs: []string{"80:8000"},
	},
		&runconfig.HostConfig{},
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
	daemon := mkDaemon(t)
	defer nuke(daemon)

	container, _, err := daemon.Create(&runconfig.Config{
		Image: GetTestImage(daemon).ID,
		Cmd:   []string{"ls", "-al"},
	},
		&runconfig.HostConfig{},
		"")
	if err != nil {
		t.Fatal(err)
	}
	// Destroy
	if err := daemon.Rm(container); err != nil {
		t.Error(err)
	}

	// Make sure daemon.Exists() behaves correctly
	if daemon.Exists("test_destroy") {
		t.Errorf("Exists() returned true")
	}

	// Make sure daemon.List() doesn't list the destroyed container
	if len(daemon.List()) != 0 {
		t.Errorf("Expected 0 container, %v found", len(daemon.List()))
	}

	// Make sure daemon.Get() refuses to return the unexisting container
	if c, _ := daemon.Get(container.ID); c != nil {
		t.Errorf("Got a container that should not exist")
	}

	// Test double destroy
	if err := daemon.Rm(container); err == nil {
		// It should have failed
		t.Errorf("Double destroy did not fail")
	}
}

func TestGet(t *testing.T) {
	daemon := mkDaemon(t)
	defer nuke(daemon)

	container1, _, _ := mkContainer(daemon, []string{"_", "ls", "-al"}, t)
	defer daemon.Rm(container1)

	container2, _, _ := mkContainer(daemon, []string{"_", "ls", "-al"}, t)
	defer daemon.Rm(container2)

	container3, _, _ := mkContainer(daemon, []string{"_", "ls", "-al"}, t)
	defer daemon.Rm(container3)

	if c, _ := daemon.Get(container1.ID); c != container1 {
		t.Errorf("Get(test1) returned %v while expecting %v", c, container1)
	}

	if c, _ := daemon.Get(container2.ID); c != container2 {
		t.Errorf("Get(test2) returned %v while expecting %v", c, container2)
	}

	if c, _ := daemon.Get(container3.ID); c != container3 {
		t.Errorf("Get(test3) returned %v while expecting %v", c, container3)
	}

}

func startEchoServerContainer(t *testing.T, proto string) (*daemon.Daemon, *daemon.Container, string) {
	var (
		err          error
		id           string
		outputBuffer = bytes.NewBuffer(nil)
		strPort      string
		eng          = NewTestEngine(t)
		daemon       = mkDaemonFromEngine(eng, t)
		port         = 5554
		p            nat.Port
	)
	defer func() {
		if err != nil {
			daemon.Nuke()
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
		ep := make(map[nat.Port]struct{}, 1)
		p = nat.Port(fmt.Sprintf("%s/%s", strPort, proto))
		ep[p] = struct{}{}

		jobCreate := eng.Job("create")
		jobCreate.Setenv("Image", unitTestImageID)
		jobCreate.SetenvList("Cmd", []string{"sh", "-c", cmd})
		jobCreate.SetenvList("PortSpecs", []string{fmt.Sprintf("%s/%s", strPort, proto)})
		jobCreate.SetenvJson("ExposedPorts", ep)
		jobCreate.Stdout.Add(outputBuffer)
		if err := jobCreate.Run(); err != nil {
			t.Fatal(err)
		}
		id = engine.Tail(outputBuffer, 1)
		// FIXME: this relies on the undocumented behavior of daemon.Create
		// which will return a nil error AND container if the exposed ports
		// are invalid. That behavior should be fixed!
		if id != "" {
			break
		}
		t.Logf("Port %v already in use, trying another one", strPort)

	}

	jobStart := eng.Job("start", id)
	portBindings := make(map[nat.Port][]nat.PortBinding)
	portBindings[p] = []nat.PortBinding{
		{},
	}
	if err := jobStart.SetenvJson("PortsBindings", portBindings); err != nil {
		t.Fatal(err)
	}
	if err := jobStart.Run(); err != nil {
		t.Fatal(err)
	}

	container, err := daemon.Get(id)
	if err != nil {
		t.Fatal(err)
	}

	setTimeout(t, "Waiting for the container to be started timed out", 2*time.Second, func() {
		for !container.IsRunning() {
			time.Sleep(10 * time.Millisecond)
		}
	})

	// Even if the state is running, lets give some time to lxc to spawn the process
	container.WaitStop(500 * time.Millisecond)

	strPort = container.NetworkSettings.Ports[p][0].HostPort
	return daemon, container, strPort
}

// Run a container with a TCP port allocated, and test that it can receive connections on localhost
func TestAllocateTCPPortLocalhost(t *testing.T) {
	daemon, container, port := startEchoServerContainer(t, "tcp")
	defer nuke(daemon)
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
	daemon, container, port := startEchoServerContainer(t, "udp")
	defer nuke(daemon)
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
	daemon1 := mkDaemonFromEngine(eng, t)
	defer daemon1.Nuke()
	// Create a container with one instance of docker
	container1, _, _ := mkContainer(daemon1, []string{"_", "ls", "-al"}, t)
	defer daemon1.Rm(container1)

	// Create a second container meant to be killed
	container2, _, _ := mkContainer(daemon1, []string{"-i", "_", "/bin/cat"}, t)
	defer daemon1.Rm(container2)

	// Start the container non blocking
	if err := container2.Start(); err != nil {
		t.Fatal(err)
	}

	if !container2.IsRunning() {
		t.Fatalf("Container %v should appear as running but isn't", container2.ID)
	}

	// Simulate a crash/manual quit of dockerd: process dies, states stays 'Running'
	cStdin := container2.StdinPipe()
	cStdin.Close()
	if _, err := container2.WaitStop(2 * time.Second); err != nil {
		t.Fatal(err)
	}
	container2.SetRunning(42)
	container2.ToDisk()

	if len(daemon1.List()) != 2 {
		t.Errorf("Expected 2 container, %v found", len(daemon1.List()))
	}
	if err := container1.Run(); err != nil {
		t.Fatal(err)
	}

	if !container2.IsRunning() {
		t.Fatalf("Container %v should appear as running but isn't", container2.ID)
	}

	// Here are are simulating a docker restart - that is, reloading all containers
	// from scratch
	eng = newTestEngine(t, false, daemon1.Config().Root)
	daemon2 := mkDaemonFromEngine(eng, t)
	if len(daemon2.List()) != 2 {
		t.Errorf("Expected 2 container, %v found", len(daemon2.List()))
	}
	runningCount := 0
	for _, c := range daemon2.List() {
		if c.IsRunning() {
			t.Errorf("Running container found: %v (%v)", c.ID, c.Path)
			runningCount++
		}
	}
	if runningCount != 0 {
		t.Fatalf("Expected 0 container alive, %d found", runningCount)
	}
	container3, err := daemon2.Get(container1.ID)
	if err != nil {
		t.Fatal("Unable to Get container")
	}
	if err := container3.Run(); err != nil {
		t.Fatal(err)
	}
	container2.SetStopped(&execdriver.ExitStatus{ExitCode: 0})
}

func TestDefaultContainerName(t *testing.T) {
	eng := NewTestEngine(t)
	daemon := mkDaemonFromEngine(eng, t)
	defer nuke(daemon)

	config, _, _, err := parseRun([]string{unitTestImageID, "echo test"})
	if err != nil {
		t.Fatal(err)
	}

	container, err := daemon.Get(createNamedTestContainer(eng, config, t, "some_name"))
	if err != nil {
		t.Fatal(err)
	}
	containerID := container.ID

	if container.Name != "/some_name" {
		t.Fatalf("Expect /some_name got %s", container.Name)
	}

	c, err := daemon.Get("/some_name")
	if err != nil {
		t.Fatalf("Couldn't retrieve test container as /some_name")
	}
	if c.ID != containerID {
		t.Fatalf("Container /some_name has ID %s instead of %s", c.ID, containerID)
	}
}

func TestRandomContainerName(t *testing.T) {
	eng := NewTestEngine(t)
	daemon := mkDaemonFromEngine(eng, t)
	defer nuke(daemon)

	config, _, _, err := parseRun([]string{GetTestImage(daemon).ID, "echo test"})
	if err != nil {
		t.Fatal(err)
	}

	container, err := daemon.Get(createTestContainer(eng, config, t))
	if err != nil {
		t.Fatal(err)
	}
	containerID := container.ID

	if container.Name == "" {
		t.Fatalf("Expected not empty container name")
	}

	if c, err := daemon.Get(container.Name); err != nil {
		log.Fatalf("Could not lookup container %s by its name", container.Name)
	} else if c.ID != containerID {
		log.Fatalf("Looking up container name %s returned id %s instead of %s", container.Name, c.ID, containerID)
	}
}

func TestContainerNameValidation(t *testing.T) {
	eng := NewTestEngine(t)
	daemon := mkDaemonFromEngine(eng, t)
	defer nuke(daemon)

	for _, test := range []struct {
		Name  string
		Valid bool
	}{
		{"abc-123_AAA.1", true},
		{"\000asdf", false},
	} {
		config, _, _, err := parseRun([]string{unitTestImageID, "echo test"})
		if err != nil {
			if !test.Valid {
				continue
			}
			t.Fatal(err)
		}

		var outputBuffer = bytes.NewBuffer(nil)
		job := eng.Job("create", test.Name)
		if err := job.ImportEnv(config); err != nil {
			t.Fatal(err)
		}
		job.Stdout.Add(outputBuffer)
		if err := job.Run(); err != nil {
			if !test.Valid {
				continue
			}
			t.Fatal(err)
		}

		container, err := daemon.Get(engine.Tail(outputBuffer, 1))
		if err != nil {
			t.Fatal(err)
		}

		if container.Name != "/"+test.Name {
			t.Fatalf("Expect /%s got %s", test.Name, container.Name)
		}

		if c, err := daemon.Get("/" + test.Name); err != nil {
			t.Fatalf("Couldn't retrieve test container as /%s", test.Name)
		} else if c.ID != container.ID {
			t.Fatalf("Container /%s has ID %s instead of %s", test.Name, c.ID, container.ID)
		}
	}

}

func TestLinkChildContainer(t *testing.T) {
	eng := NewTestEngine(t)
	daemon := mkDaemonFromEngine(eng, t)
	defer nuke(daemon)

	config, _, _, err := parseRun([]string{unitTestImageID, "echo test"})
	if err != nil {
		t.Fatal(err)
	}

	container, err := daemon.Get(createNamedTestContainer(eng, config, t, "/webapp"))
	if err != nil {
		t.Fatal(err)
	}

	webapp, err := daemon.GetByName("/webapp")
	if err != nil {
		t.Fatal(err)
	}

	if webapp.ID != container.ID {
		t.Fatalf("Expect webapp id to match container id: %s != %s", webapp.ID, container.ID)
	}

	config, _, _, err = parseRun([]string{GetTestImage(daemon).ID, "echo test"})
	if err != nil {
		t.Fatal(err)
	}

	childContainer, err := daemon.Get(createTestContainer(eng, config, t))
	if err != nil {
		t.Fatal(err)
	}

	if err := daemon.RegisterLink(webapp, childContainer, "db"); err != nil {
		t.Fatal(err)
	}

	// Get the child by it's new name
	db, err := daemon.GetByName("/webapp/db")
	if err != nil {
		t.Fatal(err)
	}
	if db.ID != childContainer.ID {
		t.Fatalf("Expect db id to match container id: %s != %s", db.ID, childContainer.ID)
	}
}

func TestGetAllChildren(t *testing.T) {
	eng := NewTestEngine(t)
	daemon := mkDaemonFromEngine(eng, t)
	defer nuke(daemon)

	config, _, _, err := parseRun([]string{unitTestImageID, "echo test"})
	if err != nil {
		t.Fatal(err)
	}

	container, err := daemon.Get(createNamedTestContainer(eng, config, t, "/webapp"))
	if err != nil {
		t.Fatal(err)
	}

	webapp, err := daemon.GetByName("/webapp")
	if err != nil {
		t.Fatal(err)
	}

	if webapp.ID != container.ID {
		t.Fatalf("Expect webapp id to match container id: %s != %s", webapp.ID, container.ID)
	}

	config, _, _, err = parseRun([]string{unitTestImageID, "echo test"})
	if err != nil {
		t.Fatal(err)
	}

	childContainer, err := daemon.Get(createTestContainer(eng, config, t))
	if err != nil {
		t.Fatal(err)
	}

	if err := daemon.RegisterLink(webapp, childContainer, "db"); err != nil {
		t.Fatal(err)
	}

	children, err := daemon.Children("/webapp")
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

func TestDestroyWithInitLayer(t *testing.T) {
	daemon := mkDaemon(t)
	defer nuke(daemon)

	container, _, err := daemon.Create(&runconfig.Config{
		Image: GetTestImage(daemon).ID,
		Cmd:   []string{"ls", "-al"},
	},
		&runconfig.HostConfig{},
		"")

	if err != nil {
		t.Fatal(err)
	}
	// Destroy
	if err := daemon.Rm(container); err != nil {
		t.Fatal(err)
	}

	// Make sure daemon.Exists() behaves correctly
	if daemon.Exists("test_destroy") {
		t.Fatalf("Exists() returned true")
	}

	// Make sure daemon.List() doesn't list the destroyed container
	if len(daemon.List()) != 0 {
		t.Fatalf("Expected 0 container, %v found", len(daemon.List()))
	}

	driver := daemon.Graph().Driver()

	// Make sure that the container does not exist in the driver
	if _, err := driver.Get(container.ID, ""); err == nil {
		t.Fatal("Conttainer should not exist in the driver")
	}

	// Make sure that the init layer is removed from the driver
	if _, err := driver.Get(fmt.Sprintf("%s-init", container.ID), ""); err == nil {
		t.Fatal("Container's init layer should not exist in the driver")
	}
}
