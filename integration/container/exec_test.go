package container // import "github.com/docker/docker/integration/container"

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/integration/internal/container"
	req "github.com/docker/docker/testutil/request"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

// TestExecWithCloseStdin adds case for moby#37870 issue.
func TestExecWithCloseStdin(t *testing.T) {
	skip.If(t, testEnv.RuntimeIsWindowsContainerd(), "FIXME. Hang on Windows + containerd combination")
	ctx := setupTest(t)

	apiClient := testEnv.APIClient()

	// run top with detached mode
	cID := container.Run(ctx, t, apiClient)

	const expected = "closeIO"
	execResp, err := apiClient.ContainerExecCreate(ctx, cID, containertypes.ExecOptions{
		AttachStdin:  true,
		AttachStdout: true,
		Cmd:          []string{"sh", "-c", "cat && echo " + expected},
	})
	assert.NilError(t, err)

	resp, err := apiClient.ContainerExecAttach(ctx, execResp.ID, containertypes.ExecAttachOptions{})
	assert.NilError(t, err)
	defer resp.Close()

	// close stdin to send EOF to cat
	assert.NilError(t, resp.CloseWrite())

	var (
		waitCh = make(chan struct{})
		resCh  = make(chan struct {
			content string
			err     error
		}, 1)
	)

	go func() {
		close(waitCh)
		defer close(resCh)
		r, err := io.ReadAll(resp.Reader)

		resCh <- struct {
			content string
			err     error
		}{
			content: string(r),
			err:     err,
		}
	}()

	<-waitCh
	select {
	case <-time.After(3 * time.Second):
		t.Fatal("failed to read the content in time")
	case got := <-resCh:
		assert.NilError(t, got.err)

		// NOTE: using Contains because no-tty's stream contains UX information
		// like size, stream type.
		assert.Assert(t, is.Contains(got.content, expected))
	}
}

func TestExec(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	cID := container.Run(ctx, t, apiClient, container.WithTty(true), container.WithWorkingDir("/root"))

	id, err := apiClient.ContainerExecCreate(ctx, cID, containertypes.ExecOptions{
		WorkingDir:   "/tmp",
		Env:          []string{"FOO=BAR"},
		AttachStdout: true,
		Cmd:          []string{"sh", "-c", "env"},
	})
	assert.NilError(t, err)

	inspect, err := apiClient.ContainerExecInspect(ctx, id.ID)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(inspect.ExecID, id.ID))

	resp, err := apiClient.ContainerExecAttach(ctx, id.ID, containertypes.ExecAttachOptions{})
	assert.NilError(t, err)
	defer resp.Close()
	r, err := io.ReadAll(resp.Reader)
	assert.NilError(t, err)
	out := string(r)
	assert.NilError(t, err)
	expected := "PWD=/tmp"
	if testEnv.DaemonInfo.OSType == "windows" {
		expected = "PWD=C:/tmp"
	}
	assert.Check(t, is.Contains(out, expected), "exec command not running in expected /tmp working directory")
	assert.Check(t, is.Contains(out, "FOO=BAR"), "exec command not running with expected environment variable FOO")
}

func TestExecResize(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	cID := container.Run(ctx, t, apiClient, container.WithTty(true))
	defer container.Remove(ctx, t, apiClient, cID, containertypes.RemoveOptions{Force: true})

	cmd := []string{"top"}
	if runtime.GOOS == "windows" {
		cmd = []string{"sleep", "240"}
	}
	resp, err := apiClient.ContainerExecCreate(ctx, cID, containertypes.ExecOptions{
		Tty:    true, // Windows requires a TTY for the resize to work, otherwise fails with "is not a tty: failed precondition", see https://github.com/moby/moby/pull/48665#issuecomment-2412530345
		Detach: true,
		Cmd:    cmd,
	})
	assert.NilError(t, err)
	execID := resp.ID
	assert.NilError(t, err)
	err = apiClient.ContainerExecStart(ctx, execID, containertypes.ExecStartOptions{Detach: true})
	assert.NilError(t, err)

	t.Run("success", func(t *testing.T) {
		err := apiClient.ContainerExecResize(ctx, execID, containertypes.ResizeOptions{
			Height: 40,
			Width:  40,
		})
		assert.NilError(t, err)
		// TODO(thaJeztah): also check if the resize happened
		//
		// Note: container inspect shows the initial size that was
		// set when creating the container. Actual resize happens in
		// containerd, and currently does not update the container's
		// config after running (but does send a "resize" event).
	})

	t.Run("invalid size", func(t *testing.T) {
		const valueNotSet = "unset"

		sizes := []struct {
			doc, height, width, expErr string
		}{
			{
				doc:    "unset height",
				height: valueNotSet,
				width:  "100",
				expErr: `invalid resize height "": invalid syntax`,
			},
			{
				doc:    "unset width",
				height: "100",
				width:  valueNotSet,
				expErr: `invalid resize width "": invalid syntax`,
			},
			{
				doc:    "empty height",
				width:  "100",
				expErr: `invalid resize height "": invalid syntax`,
			},
			{
				doc:    "empty width",
				height: "100",
				expErr: `invalid resize width "": invalid syntax`,
			},
			{
				doc:    "non-numeric height",
				height: "not-a-number",
				width:  "100",
				expErr: `invalid resize height "not-a-number": invalid syntax`,
			},
			{
				doc:    "non-numeric width",
				height: "100",
				width:  "not-a-number",
				expErr: `invalid resize width "not-a-number": invalid syntax`,
			},
			{
				doc:    "negative height",
				height: "-100",
				width:  "100",
				expErr: `invalid resize height "-100": value out of range`,
			},
			{
				doc:    "negative width",
				height: "100",
				width:  "-100",
				expErr: `invalid resize width "-100": value out of range`,
			},
			{
				doc:    "out of range height",
				height: "4294967296", // math.MaxUint32+1
				width:  "100",
				expErr: `invalid resize height "4294967296": value out of range`,
			},
			{
				doc:    "out of range width",
				height: "100",
				width:  "4294967296", // math.MaxUint32+1
				expErr: `invalid resize width "4294967296": value out of range`,
			},
		}
		for _, tc := range sizes {
			tc := tc
			t.Run(tc.doc, func(t *testing.T) {
				// Manually creating a request here, as the APIClient would invalidate
				// these values before they're sent.
				vals := url.Values{}
				if tc.height != valueNotSet {
					vals.Add("h", tc.height)
				}
				if tc.width != valueNotSet {
					vals.Add("w", tc.width)
				}
				res, _, err := req.Post(ctx, "/exec/"+execID+"/resize?"+vals.Encode())
				assert.NilError(t, err)
				assert.Check(t, is.Equal(http.StatusBadRequest, res.StatusCode))

				var errorResponse types.ErrorResponse
				err = json.NewDecoder(res.Body).Decode(&errorResponse)
				assert.NilError(t, err)
				assert.Check(t, is.ErrorContains(errorResponse, tc.expErr))
			})
		}
	})

	t.Run("unknown execID", func(t *testing.T) {
		err = apiClient.ContainerExecResize(ctx, "no-such-exec-id", containertypes.ResizeOptions{
			Height: 40,
			Width:  40,
		})
		assert.Check(t, is.ErrorType(err, errdefs.IsNotFound))
		assert.Check(t, is.ErrorContains(err, "No such exec instance: no-such-exec-id"))
	})

	t.Run("invalid state", func(t *testing.T) {
		// FIXME(thaJeztah): Windows + builtin returns a NotFound instead of a Conflict error
		//
		// When using the builtin runtime, stopping the container causes
		// the exec-resize to return a "NotFound" error, whereas with containerd
		// as runtime, it returns the expected "Conflict" error. This could be
		// either a limitation of the "builtin" runtime, or there's a bug to
		// be fixed.
		//
		// See https://github.com/moby/moby/pull/48665#issuecomment-2412579701
		//
		//  === RUN   TestExecResize/invalid_state
		//      exec_test.go:234: assertion failed: error is Error response from daemon: No such exec instance: cc728a332d3f594249fb7ee9adb3bb12a59a5d1776f8f6dedc56355364361711 (errdefs.errNotFound), not errdefs.IsConflict
		//      exec_test.go:235: assertion failed: expected error to contain "is not running", got "Error response from daemon: No such exec instance: cc728a332d3f594249fb7ee9adb3bb12a59a5d1776f8f6dedc56355364361711"
		//          Error response from daemon: No such exec instance: cc728a332d3f594249fb7ee9adb3bb12a59a5d1776f8f6dedc56355364361711
		skip.If(t, testEnv.DaemonInfo.OSType == "windows" && !testEnv.RuntimeIsWindowsContainerd(), "FIXME. Windows + builtin returns a NotFound instead of a Conflict error")

		err := apiClient.ContainerKill(ctx, cID, "SIGKILL")
		assert.NilError(t, err)

		err = apiClient.ContainerExecResize(ctx, execID, containertypes.ResizeOptions{
			Height: 40,
			Width:  40,
		})
		assert.Check(t, is.ErrorType(err, errdefs.IsConflict))
		assert.Check(t, is.ErrorContains(err, "is not running"))
	})
}

func TestExecUser(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "FIXME. Probably needs to wait for container to be in running state.")
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	cID := container.Run(ctx, t, apiClient, container.WithTty(true), container.WithUser("1:1"))

	result, err := container.Exec(ctx, apiClient, cID, []string{"id"})
	assert.NilError(t, err)

	assert.Check(t, is.Contains(result.Stdout(), "uid=1(daemon) gid=1(daemon)"), "exec command not running as uid/gid 1")
}

// Test that additional groups set with `--group-add` are kept on exec when the container
// also has a user set.
// (regression test for https://github.com/moby/moby/issues/46712)
func TestExecWithGroupAdd(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "FIXME. Probably needs to wait for container to be in running state.")

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	cID := container.Run(ctx, t, apiClient, container.WithTty(true), container.WithUser("root:root"), container.WithAdditionalGroups("staff", "wheel", "audio", "777"), container.WithCmd("sleep", "5"))

	result, err := container.Exec(ctx, apiClient, cID, []string{"id"})
	assert.NilError(t, err)

	const expected = "uid=0(root) gid=0(root) groups=0(root),10(wheel),29(audio),50(staff),777"
	assert.Check(t, is.Equal(strings.TrimSpace(result.Stdout()), expected), "exec command not keeping additional groups w/ user")
}
