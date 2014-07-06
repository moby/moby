package docker

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/dotcloud/docker/api/client"
	"github.com/dotcloud/docker/daemon"
	"github.com/dotcloud/docker/engine"
	"github.com/dotcloud/docker/pkg/term"
	"github.com/dotcloud/docker/utils"
)

func closeWrap(args ...io.Closer) error {
	e := false
	ret := fmt.Errorf("Error closing elements")
	for _, c := range args {
		if err := c.Close(); err != nil {
			e = true
			ret = fmt.Errorf("%s\n%s", ret, err)
		}
	}
	if e {
		return ret
	}
	return nil
}

func setRaw(t *testing.T, c *daemon.Container) *term.State {
	pty, err := c.GetPtyMaster()
	if err != nil {
		t.Fatal(err)
	}
	state, err := term.MakeRaw(pty.Fd())
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func unsetRaw(t *testing.T, c *daemon.Container, state *term.State) {
	pty, err := c.GetPtyMaster()
	if err != nil {
		t.Fatal(err)
	}
	term.RestoreTerminal(pty.Fd(), state)
}

func waitContainerStart(t *testing.T, timeout time.Duration) *daemon.Container {
	var container *daemon.Container

	setTimeout(t, "Waiting for the container to be started timed out", timeout, func() {
		for {
			l := globalDaemon.List()
			if len(l) == 1 && l[0].State.IsRunning() {
				container = l[0]
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
	})

	if container == nil {
		t.Fatal("An error occured while waiting for the container to start")
	}

	return container
}

func setTimeout(t *testing.T, msg string, d time.Duration, f func()) {
	c := make(chan bool)

	// Make sure we are not too long
	go func() {
		time.Sleep(d)
		c <- true
	}()
	go func() {
		f()
		c <- false
	}()
	if <-c && msg != "" {
		t.Fatal(msg)
	}
}

func expectPipe(expected string, r io.Reader) error {
	o, err := bufio.NewReader(r).ReadString('\n')
	if err != nil {
		return err
	}
	if strings.Trim(o, " \r\n") != expected {
		return fmt.Errorf("Unexpected output. Expected [%s], received [%s]", expected, o)
	}
	return nil
}

func assertPipe(input, output string, r io.Reader, w io.Writer, count int) error {
	for i := 0; i < count; i++ {
		if _, err := w.Write([]byte(input)); err != nil {
			return err
		}
		if err := expectPipe(output, r); err != nil {
			return err
		}
	}
	return nil
}

// TestRunWorkdirExistsAndIsFile checks that if 'docker run -w' with existing file can be detected
func TestRunWorkdirExistsAndIsFile(t *testing.T) {

	cli := client.NewDockerCli(nil, nil, ioutil.Discard, testDaemonProto, testDaemonAddr, nil)
	defer cleanup(globalEngine, t)

	c := make(chan struct{})
	go func() {
		defer close(c)
		if err := cli.CmdRun("-w", "/bin/cat", unitTestImageID, "pwd"); err == nil {
			t.Fatal("should have failed to run when using /bin/cat as working dir.")
		}
	}()

	setTimeout(t, "CmdRun timed out", 5*time.Second, func() {
		<-c
	})
}

func TestRunExit(t *testing.T) {
	stdin, stdinPipe := io.Pipe()
	stdout, stdoutPipe := io.Pipe()

	cli := client.NewDockerCli(stdin, stdoutPipe, ioutil.Discard, testDaemonProto, testDaemonAddr, nil)
	defer cleanup(globalEngine, t)

	c1 := make(chan struct{})
	go func() {
		cli.CmdRun("-i", unitTestImageID, "/bin/cat")
		close(c1)
	}()

	setTimeout(t, "Read/Write assertion timed out", 2*time.Second, func() {
		if err := assertPipe("hello\n", "hello", stdout, stdinPipe, 150); err != nil {
			t.Fatal(err)
		}
	})

	container := globalDaemon.List()[0]

	// Closing /bin/cat stdin, expect it to exit
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}

	// as the process exited, CmdRun must finish and unblock. Wait for it
	setTimeout(t, "Waiting for CmdRun timed out", 10*time.Second, func() {
		<-c1

		go func() {
			cli.CmdWait(container.ID)
		}()

		if _, err := bufio.NewReader(stdout).ReadString('\n'); err != nil {
			t.Fatal(err)
		}
	})

	// Make sure that the client has been disconnected
	setTimeout(t, "The client should have been disconnected once the remote process exited.", 2*time.Second, func() {
		// Expecting pipe i/o error, just check that read does not block
		stdin.Read([]byte{})
	})

	// Cleanup pipes
	if err := closeWrap(stdin, stdinPipe, stdout, stdoutPipe); err != nil {
		t.Fatal(err)
	}
}

// Expected behaviour: the process dies when the client disconnects
func TestRunDisconnect(t *testing.T) {

	stdin, stdinPipe := io.Pipe()
	stdout, stdoutPipe := io.Pipe()

	cli := client.NewDockerCli(stdin, stdoutPipe, ioutil.Discard, testDaemonProto, testDaemonAddr, nil)
	defer cleanup(globalEngine, t)

	c1 := make(chan struct{})
	go func() {
		// We're simulating a disconnect so the return value doesn't matter. What matters is the
		// fact that CmdRun returns.
		cli.CmdRun("-i", unitTestImageID, "/bin/cat")
		close(c1)
	}()

	setTimeout(t, "Read/Write assertion timed out", 2*time.Second, func() {
		if err := assertPipe("hello\n", "hello", stdout, stdinPipe, 150); err != nil {
			t.Fatal(err)
		}
	})

	// Close pipes (simulate disconnect)
	if err := closeWrap(stdin, stdinPipe, stdout, stdoutPipe); err != nil {
		t.Fatal(err)
	}

	// as the pipes are close, we expect the process to die,
	// therefore CmdRun to unblock. Wait for CmdRun
	setTimeout(t, "Waiting for CmdRun timed out", 2*time.Second, func() {
		<-c1
	})

	// Client disconnect after run -i should cause stdin to be closed, which should
	// cause /bin/cat to exit.
	setTimeout(t, "Waiting for /bin/cat to exit timed out", 2*time.Second, func() {
		container := globalDaemon.List()[0]
		container.State.WaitStop(-1 * time.Second)
		if container.State.IsRunning() {
			t.Fatalf("/bin/cat is still running after closing stdin")
		}
	})
}

// Expected behaviour: the process stay alive when the client disconnects
// but the client detaches.
func TestRunDisconnectTty(t *testing.T) {

	stdin, stdinPipe := io.Pipe()
	stdout, stdoutPipe := io.Pipe()

	cli := client.NewDockerCli(stdin, stdoutPipe, ioutil.Discard, testDaemonProto, testDaemonAddr, nil)
	defer cleanup(globalEngine, t)

	c1 := make(chan struct{})
	go func() {
		defer close(c1)
		// We're simulating a disconnect so the return value doesn't matter. What matters is the
		// fact that CmdRun returns.
		if err := cli.CmdRun("-i", "-t", unitTestImageID, "/bin/cat"); err != nil {
			utils.Debugf("Error CmdRun: %s", err)
		}
	}()

	container := waitContainerStart(t, 10*time.Second)

	state := setRaw(t, container)
	defer unsetRaw(t, container, state)

	// Client disconnect after run -i should keep stdin out in TTY mode
	setTimeout(t, "Read/Write assertion timed out", 2*time.Second, func() {
		if err := assertPipe("hello\n", "hello", stdout, stdinPipe, 150); err != nil {
			t.Fatal(err)
		}
	})

	// Close pipes (simulate disconnect)
	if err := closeWrap(stdin, stdinPipe, stdout, stdoutPipe); err != nil {
		t.Fatal(err)
	}

	// wait for CmdRun to return
	setTimeout(t, "Waiting for CmdRun timed out", 5*time.Second, func() {
		<-c1
	})

	// In tty mode, we expect the process to stay alive even after client's stdin closes.

	// Give some time to monitor to do his thing
	container.State.WaitStop(500 * time.Millisecond)
	if !container.State.IsRunning() {
		t.Fatalf("/bin/cat should  still be running after closing stdin (tty mode)")
	}
}

// TestRunDetach checks attaching and detaching with the escape sequence.
func TestRunDetach(t *testing.T) {

	stdin, stdinPipe := io.Pipe()
	stdout, stdoutPipe := io.Pipe()

	cli := client.NewDockerCli(stdin, stdoutPipe, ioutil.Discard, testDaemonProto, testDaemonAddr, nil)
	defer cleanup(globalEngine, t)

	ch := make(chan struct{})
	go func() {
		defer close(ch)
		cli.CmdRun("-i", "-t", unitTestImageID, "cat")
	}()

	container := waitContainerStart(t, 10*time.Second)

	state := setRaw(t, container)
	defer unsetRaw(t, container, state)

	setTimeout(t, "First read/write assertion timed out", 2*time.Second, func() {
		if err := assertPipe("hello\n", "hello", stdout, stdinPipe, 150); err != nil {
			t.Fatal(err)
		}
	})

	setTimeout(t, "Escape sequence timeout", 5*time.Second, func() {
		stdinPipe.Write([]byte{16})
		time.Sleep(100 * time.Millisecond)
		stdinPipe.Write([]byte{17})
	})

	// wait for CmdRun to return
	setTimeout(t, "Waiting for CmdRun timed out", 15*time.Second, func() {
		<-ch
	})
	closeWrap(stdin, stdinPipe, stdout, stdoutPipe)

	time.Sleep(500 * time.Millisecond)
	if !container.State.IsRunning() {
		t.Fatal("The detached container should be still running")
	}

	setTimeout(t, "Waiting for container to die timed out", 20*time.Second, func() {
		container.Kill()
	})
}

// TestAttachDetach checks that attach in tty mode can be detached using the long container ID
func TestAttachDetach(t *testing.T) {
	stdin, stdinPipe := io.Pipe()
	stdout, stdoutPipe := io.Pipe()

	cli := client.NewDockerCli(stdin, stdoutPipe, ioutil.Discard, testDaemonProto, testDaemonAddr, nil)
	defer cleanup(globalEngine, t)

	ch := make(chan struct{})
	go func() {
		defer close(ch)
		if err := cli.CmdRun("-i", "-t", "-d", unitTestImageID, "cat"); err != nil {
			t.Fatal(err)
		}
	}()

	container := waitContainerStart(t, 10*time.Second)

	setTimeout(t, "Reading container's id timed out", 10*time.Second, func() {
		buf := make([]byte, 1024)
		n, err := stdout.Read(buf)
		if err != nil {
			t.Fatal(err)
		}

		if strings.Trim(string(buf[:n]), " \r\n") != container.ID {
			t.Fatalf("Wrong ID received. Expect %s, received %s", container.ID, buf[:n])
		}
	})
	setTimeout(t, "Starting container timed out", 10*time.Second, func() {
		<-ch
	})

	state := setRaw(t, container)
	defer unsetRaw(t, container, state)

	stdin, stdinPipe = io.Pipe()
	stdout, stdoutPipe = io.Pipe()
	cli = client.NewDockerCli(stdin, stdoutPipe, ioutil.Discard, testDaemonProto, testDaemonAddr, nil)

	ch = make(chan struct{})
	go func() {
		defer close(ch)
		if err := cli.CmdAttach(container.ID); err != nil {
			if err != io.ErrClosedPipe {
				t.Fatal(err)
			}
		}
	}()

	setTimeout(t, "First read/write assertion timed out", 2*time.Second, func() {
		if err := assertPipe("hello\n", "hello", stdout, stdinPipe, 150); err != nil {
			if err != io.ErrClosedPipe {
				t.Fatal(err)
			}
		}
	})

	setTimeout(t, "Escape sequence timeout", 5*time.Second, func() {
		stdinPipe.Write([]byte{16})
		time.Sleep(100 * time.Millisecond)
		stdinPipe.Write([]byte{17})
	})

	// wait for CmdRun to return
	setTimeout(t, "Waiting for CmdAttach timed out", 15*time.Second, func() {
		<-ch
	})

	closeWrap(stdin, stdinPipe, stdout, stdoutPipe)

	time.Sleep(500 * time.Millisecond)
	if !container.State.IsRunning() {
		t.Fatal("The detached container should be still running")
	}

	setTimeout(t, "Waiting for container to die timedout", 5*time.Second, func() {
		container.Kill()
	})
}

// TestAttachDetachTruncatedID checks that attach in tty mode can be detached
func TestAttachDetachTruncatedID(t *testing.T) {
	stdin, stdinPipe := io.Pipe()
	stdout, stdoutPipe := io.Pipe()

	cli := client.NewDockerCli(stdin, stdoutPipe, ioutil.Discard, testDaemonProto, testDaemonAddr, nil)
	defer cleanup(globalEngine, t)

	// Discard the CmdRun output
	go stdout.Read(make([]byte, 1024))
	setTimeout(t, "Starting container timed out", 2*time.Second, func() {
		if err := cli.CmdRun("-i", "-t", "-d", unitTestImageID, "cat"); err != nil {
			t.Fatal(err)
		}
	})

	container := waitContainerStart(t, 10*time.Second)

	state := setRaw(t, container)
	defer unsetRaw(t, container, state)

	stdin, stdinPipe = io.Pipe()
	stdout, stdoutPipe = io.Pipe()
	cli = client.NewDockerCli(stdin, stdoutPipe, ioutil.Discard, testDaemonProto, testDaemonAddr, nil)

	ch := make(chan struct{})
	go func() {
		defer close(ch)
		if err := cli.CmdAttach(utils.TruncateID(container.ID)); err != nil {
			if err != io.ErrClosedPipe {
				t.Fatal(err)
			}
		}
	}()

	setTimeout(t, "First read/write assertion timed out", 2*time.Second, func() {
		if err := assertPipe("hello\n", "hello", stdout, stdinPipe, 150); err != nil {
			if err != io.ErrClosedPipe {
				t.Fatal(err)
			}
		}
	})

	setTimeout(t, "Escape sequence timeout", 5*time.Second, func() {
		stdinPipe.Write([]byte{16})
		time.Sleep(100 * time.Millisecond)
		stdinPipe.Write([]byte{17})
	})

	// wait for CmdRun to return
	setTimeout(t, "Waiting for CmdAttach timed out", 15*time.Second, func() {
		<-ch
	})
	closeWrap(stdin, stdinPipe, stdout, stdoutPipe)

	time.Sleep(500 * time.Millisecond)
	if !container.State.IsRunning() {
		t.Fatal("The detached container should be still running")
	}

	setTimeout(t, "Waiting for container to die timedout", 5*time.Second, func() {
		container.Kill()
	})
}

// Expected behaviour, the process stays alive when the client disconnects
func TestAttachDisconnect(t *testing.T) {
	stdin, stdinPipe := io.Pipe()
	stdout, stdoutPipe := io.Pipe()

	cli := client.NewDockerCli(stdin, stdoutPipe, ioutil.Discard, testDaemonProto, testDaemonAddr, nil)
	defer cleanup(globalEngine, t)

	go func() {
		// Start a process in daemon mode
		if err := cli.CmdRun("-d", "-i", unitTestImageID, "/bin/cat"); err != nil {
			utils.Debugf("Error CmdRun: %s", err)
		}
	}()

	setTimeout(t, "Waiting for CmdRun timed out", 10*time.Second, func() {
		if _, err := bufio.NewReader(stdout).ReadString('\n'); err != nil {
			t.Fatal(err)
		}
	})

	setTimeout(t, "Waiting for the container to be started timed out", 10*time.Second, func() {
		for {
			l := globalDaemon.List()
			if len(l) == 1 && l[0].State.IsRunning() {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
	})

	container := globalDaemon.List()[0]

	// Attach to it
	c1 := make(chan struct{})
	go func() {
		// We're simulating a disconnect so the return value doesn't matter. What matters is the
		// fact that CmdAttach returns.
		cli.CmdAttach(container.ID)
		close(c1)
	}()

	setTimeout(t, "First read/write assertion timed out", 2*time.Second, func() {
		if err := assertPipe("hello\n", "hello", stdout, stdinPipe, 150); err != nil {
			t.Fatal(err)
		}
	})
	// Close pipes (client disconnects)
	if err := closeWrap(stdin, stdinPipe, stdout, stdoutPipe); err != nil {
		t.Fatal(err)
	}

	// Wait for attach to finish, the client disconnected, therefore, Attach finished his job
	setTimeout(t, "Waiting for CmdAttach timed out", 2*time.Second, func() {
		<-c1
	})

	// We closed stdin, expect /bin/cat to still be running
	// Wait a little bit to make sure container.monitor() did his thing
	_, err := container.State.WaitStop(500 * time.Millisecond)
	if err == nil || !container.State.IsRunning() {
		t.Fatalf("/bin/cat is not running after closing stdin")
	}

	// Try to avoid the timeout in destroy. Best effort, don't check error
	cStdin, _ := container.StdinPipe()
	cStdin.Close()
	container.State.WaitStop(-1 * time.Second)
}

// Expected behaviour: container gets deleted automatically after exit
func TestRunAutoRemove(t *testing.T) {
	t.Skip("Fixme. Skipping test for now, race condition")
	stdout, stdoutPipe := io.Pipe()
	cli := client.NewDockerCli(nil, stdoutPipe, ioutil.Discard, testDaemonProto, testDaemonAddr, nil)
	defer cleanup(globalEngine, t)

	c := make(chan struct{})
	go func() {
		defer close(c)
		if err := cli.CmdRun("--rm", unitTestImageID, "hostname"); err != nil {
			t.Fatal(err)
		}
	}()

	var temporaryContainerID string
	setTimeout(t, "Reading command output time out", 2*time.Second, func() {
		cmdOutput, err := bufio.NewReader(stdout).ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		temporaryContainerID = cmdOutput
		if err := closeWrap(stdout, stdoutPipe); err != nil {
			t.Fatal(err)
		}
	})

	setTimeout(t, "CmdRun timed out", 10*time.Second, func() {
		<-c
	})

	time.Sleep(500 * time.Millisecond)

	if len(globalDaemon.List()) > 0 {
		t.Fatalf("failed to remove container automatically: container %s still exists", temporaryContainerID)
	}
}

// Expected behaviour: error out when attempting to bind mount non-existing source paths
func TestRunErrorBindNonExistingSource(t *testing.T) {

	cli := client.NewDockerCli(nil, nil, ioutil.Discard, testDaemonProto, testDaemonAddr, nil)
	defer cleanup(globalEngine, t)

	c := make(chan struct{})
	go func() {
		defer close(c)
		// This check is made at runtime, can't be "unit tested"
		if err := cli.CmdRun("-v", "/i/dont/exist:/tmp", unitTestImageID, "echo 'should fail'"); err == nil {
			t.Fatal("should have failed to run when using /i/dont/exist as a source for the bind mount")
		}
	}()

	setTimeout(t, "CmdRun timed out", 5*time.Second, func() {
		<-c
	})
}

// #2098 - Docker cidFiles only contain short version of the containerId
//sudo docker run --cidfile /tmp/docker_test.cid ubuntu echo "test"
// TestRunCidFile tests that run --cidfile returns the longid
func TestRunCidFileCheckIDLength(t *testing.T) {
	stdout, stdoutPipe := io.Pipe()

	tmpDir, err := ioutil.TempDir("", "TestRunCidFile")
	if err != nil {
		t.Fatal(err)
	}
	tmpCidFile := path.Join(tmpDir, "cid")

	cli := client.NewDockerCli(nil, stdoutPipe, ioutil.Discard, testDaemonProto, testDaemonAddr, nil)
	defer cleanup(globalEngine, t)

	c := make(chan struct{})
	go func() {
		defer close(c)
		if err := cli.CmdRun("--cidfile", tmpCidFile, unitTestImageID, "ls"); err != nil {
			t.Fatal(err)
		}
	}()

	defer os.RemoveAll(tmpDir)
	setTimeout(t, "Reading command output time out", 2*time.Second, func() {
		cmdOutput, err := bufio.NewReader(stdout).ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if len(cmdOutput) < 1 {
			t.Fatalf("'ls' should return something , not '%s'", cmdOutput)
		}
		//read the tmpCidFile
		buffer, err := ioutil.ReadFile(tmpCidFile)
		if err != nil {
			t.Fatal(err)
		}
		id := string(buffer)

		if len(id) != len("2bf44ea18873287bd9ace8a4cb536a7cbe134bed67e805fdf2f58a57f69b320c") {
			t.Fatalf("--cidfile should be a long id, not '%s'", id)
		}
		//test that its a valid cid? (though the container is gone..)
		//remove the file and dir.
	})

	setTimeout(t, "CmdRun timed out", 5*time.Second, func() {
		<-c
	})

}

// Ensure that CIDFile gets deleted if it's empty
// Perform this test by making `docker run` fail
func TestRunCidFileCleanupIfEmpty(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "TestRunCidFile")
	if err != nil {
		t.Fatal(err)
	}
	tmpCidFile := path.Join(tmpDir, "cid")

	cli := client.NewDockerCli(nil, ioutil.Discard, ioutil.Discard, testDaemonProto, testDaemonAddr, nil)
	defer cleanup(globalEngine, t)

	c := make(chan struct{})
	go func() {
		defer close(c)
		if err := cli.CmdRun("--cidfile", tmpCidFile, unitTestImageID); err == nil {
			t.Fatal("running without a command should haveve failed")
		}
		if _, err := os.Stat(tmpCidFile); err == nil {
			t.Fatalf("empty CIDFile '%s' should've been deleted", tmpCidFile)
		}
	}()
	defer os.RemoveAll(tmpDir)

	setTimeout(t, "CmdRun timed out", 5*time.Second, func() {
		<-c
	})
}

func TestContainerOrphaning(t *testing.T) {

	// setup a temporary directory
	tmpDir, err := ioutil.TempDir("", "project")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// setup a CLI and server
	cli := client.NewDockerCli(nil, ioutil.Discard, ioutil.Discard, testDaemonProto, testDaemonAddr, nil)
	defer cleanup(globalEngine, t)
	srv := mkServerFromEngine(globalEngine, t)

	// closure to build something
	buildSomething := func(template string, image string) string {
		dockerfile := path.Join(tmpDir, "Dockerfile")
		replacer := strings.NewReplacer("{IMAGE}", unitTestImageID)
		contents := replacer.Replace(template)
		ioutil.WriteFile(dockerfile, []byte(contents), 0x777)
		if err := cli.CmdBuild("-t", image, tmpDir); err != nil {
			t.Fatal(err)
		}
		job := globalEngine.Job("image_get", image)
		info, _ := job.Stdout.AddEnv()
		if err := job.Run(); err != nil {
			t.Fatal(err)
		}
		return info.Get("Id")
	}

	// build an image
	imageName := "orphan-test"
	template1 := `
	from {IMAGE}
	cmd ["/bin/echo", "holla"]
	`
	img1 := buildSomething(template1, imageName)

	// create a container using the fist image
	if err := cli.CmdRun(imageName); err != nil {
		t.Fatal(err)
	}

	// build a new image that splits lineage
	template2 := `
	from {IMAGE}
	cmd ["/bin/echo", "holla"]
	expose 22
	`
	buildSomething(template2, imageName)

	// remove the second image by name
	resp := engine.NewTable("", 0)
	if err := srv.DeleteImage(imageName, resp, true, false, false); err == nil {
		t.Fatal("Expected error, got none")
	}

	// see if we deleted the first image (and orphaned the container)
	for _, i := range resp.Data {
		if img1 == i.Get("Deleted") {
			t.Fatal("Orphaned image with container")
		}
	}

}
