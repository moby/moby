package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/internal/test/request"
	"github.com/docker/docker/pkg/stdcopy"
	"gotest.tools/assert"
	"gotest.tools/skip"
)

// Regression test for #35370
// Makes sure that when following we don't get an EOF error when there are no logs
func TestLogsFollowTailEmpty(t *testing.T) {
	// FIXME(vdemeester) fails on a e2e run on linux...
	skip.If(t, testEnv.IsRemoteDaemon())
	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	id := container.Run(t, ctx, client, container.WithCmd("sleep", "100000"))

	logs, err := client.ContainerLogs(ctx, id, types.ContainerLogsOptions{ShowStdout: true, Tail: "2"})
	if logs != nil {
		defer logs.Close()
	}
	assert.Check(t, err)

	_, err = stdcopy.StdCopy(ioutil.Discard, ioutil.Discard, logs)
	assert.Check(t, err)
}

type daemonResources [2]int

func (r daemonResources) String() string {
	return fmt.Sprintf("goroutines: %d, file descriptors: %d", r[0], r[1])
}

func (r daemonResources) Delta(r2 daemonResources) (d daemonResources) {
	for i := 0; i < len(r); i++ {
		d[i] = r2[i] - r[i]
		if d[i] < 0 { // negative values do not make sense here
			d[i] = 0
		}
	}
	return
}

// Test for #37391
func TestLogsFollowGoroutineLeak(t *testing.T) {
	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	getDaemonResources := func() (r daemonResources) {
		info, err := client.Info(ctx)
		assert.NilError(t, err)
		// this will fail for daemon run without -D/--debug
		assert.Check(t, info.NGoroutines > 1)
		assert.Check(t, info.NFd > 1)
		r[0] = info.NGoroutines
		r[1] = info.NFd

		return
	}

	isZero := func(delta daemonResources) bool {
		for i := 0; i < len(delta); i++ {
			if delta[i] > 0 {
				return false
			}
		}

		return true
	}

	waitToFreeResources := func(exp daemonResources) error {
		tm := time.After(10 * time.Second)
		for {
			select {
			case <-tm:
				// one last chance
				r := getDaemonResources()
				t.Logf("daemon resources after: %v", r)
				d := exp.Delta(r)
				if isZero(d) {
					return nil
				}
				return fmt.Errorf("Leaked %v", d)
			default:
				d := exp.Delta(getDaemonResources())
				if isZero(d) {
					return nil
				}
				time.Sleep(200 * time.Millisecond)
			}
		}
	}

	// start a container producing lots of logs
	id := container.Run(t, ctx, client, container.WithCmd("yes", "lorem ipsum"))

	exp := getDaemonResources()
	t.Logf("daemon resources before: %v", exp)

	// consume logs
	stopCh := make(chan struct{})
	errCh := make(chan error)
	go func() {
		logs, err := client.ContainerLogs(ctx, id, types.ContainerLogsOptions{
			Follow:     true,
			ShowStdout: true,
			ShowStderr: true,
		})
		if err != nil {
			errCh <- err
			return
		}
		assert.Check(t, logs != nil)

		rd := 0
		buf := make([]byte, 1024)
		defer func() {
			logs.Close()
			t.Logf("exit after reading %d bytes", rd)
		}()

		for {
			select {
			case <-stopCh:
				errCh <- nil
				return
			default:
				n, err := logs.Read(buf)
				rd += n
				if err != nil {
					errCh <- err
					return
				}
			}
		}
	}()

	// read logs for a bit, then stop the reader
	select {
	case err := <-errCh:
		// err can't be nil here
		t.Fatalf("logs unexpectedly closed: %v", err)
	case <-time.After(1 * time.Second):
		close(stopCh)
	}
	// wait for log reader to stop
	select {
	case <-errCh:
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for log reader to stop")
	}

	err := waitToFreeResources(exp)
	if err != nil {
		t.Fatal(err)
	}
}

// test for #37630 ("docker logs -f exits whenever container stops").
// Parameter 'stoppedContainer', if set to true, means that the logger
// starts for a stopped container.
func testLogsFollow(t *testing.T, stoppedContainer bool) {
	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()
	tm := time.Second * 1

	// start a container producing some logs
	id := container.Run(t, ctx, client, container.WithCmd("sh", "-c", "while true; do date +%s; sleep 0.1; done"))
	if stoppedContainer {
		err := client.ContainerStop(ctx, id, &tm)
		assert.NilError(t, err)
	}

	// consume logs
	errCh := make(chan error)
	rd := 0 // read bytes counter
	go func() {
		logs, err := client.ContainerLogs(ctx, id, types.ContainerLogsOptions{
			Follow:     true,
			ShowStdout: true,
			ShowStderr: true,
			Tail:       "all",
		})
		if err != nil {
			errCh <- err
			return
		}
		assert.Check(t, logs != nil)

		buf := make([]byte, 1024)
		for {
			n, err := logs.Read(buf)
			rd += n
			if err != nil {
				errCh <- err
				return
			}
		}
	}()

	if stoppedContainer {
		// time for log reader to process something
		time.Sleep(tm)
	} else {
		err := client.ContainerStop(ctx, id, &tm)
		assert.NilError(t, err)
	}

	select {
	case err := <-errCh:
		t.Fatalf("logs unexpectedly closed: %v", err)
	default:
	}

	// make sure we have read some more bytes since last call
	r := 0
	checkReadMore := func(state string) {
		oldR := r
		r = rd
		t.Logf("container %s; read %d bytes so far", state, r)
		if r <= oldR {
			t.Fatalf("logs stuck? expected > %d, got %d", oldR, r)
		}
	}

	checkReadMore("stopped")

	// start the container again, read some more...
	err := client.ContainerStart(ctx, id, types.ContainerStartOptions{})
	assert.NilError(t, err)
	// wait a bit
	select {
	case err := <-errCh:
		t.Fatalf("logs unexpectedly closed: %v", err)
	case <-time.After(tm):
		checkReadMore("restarted")
	}

	// stop and remove the container
	err = client.ContainerStop(ctx, id, &tm)
	assert.NilError(t, err)
	err = client.ContainerRemove(ctx, id, types.ContainerRemoveOptions{})
	assert.NilError(t, err)

	// wait for log reader to stop
	select {
	case err := <-errCh:
		if err != io.EOF {
			t.Fatalf("logs returned: %v, expected: %v", err, io.EOF)
		}
		checkReadMore("removed")
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for log reader to stop")
	}
}

// Check that ContainerLogs(opts.Follow=true)
//  - won't stop even if the container is stopped;
//  - keep reading logs once the container is restarted;
//  - only stops when the container is removed.
func TestLogsFollowNonStop(t *testing.T) {
	testLogsFollow(t, false)
}

// Check that ContainerLogs(opts.Follow=true) works
// as expected for existing stopped container, i.e. it does
// not exit but keeps waiting for the logs to come.
func TestLogsFollowStopped(t *testing.T) {
	testLogsFollow(t, true)
}
