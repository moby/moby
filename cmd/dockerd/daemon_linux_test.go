package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/pkg/reexec"
	"golang.org/x/sys/unix"
	"gotest.tools/v3/assert"
)

const (
	testListenerNoAddrCmdPhase1 = "test-listener-no-addr1"
	testListenerNoAddrCmdPhase2 = "test-listener-no-addr2"
)

type listenerTestResponse struct {
	Err string
}

func initListenerTestPhase1() {
	os.Setenv("LISTEN_PID", strconv.Itoa(os.Getpid()))
	os.Setenv("LISTEN_FDS", "1")

	// NOTE: We cannot use O_CLOEXEC here because we need the fd to stay open for the child process.
	_, err := unix.Socket(unix.AF_UNIX, unix.SOCK_STREAM, 0)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	cmd := reexec.Command(testListenerNoAddrCmdPhase2)
	if err := unix.Exec(cmd.Path, cmd.Args, os.Environ()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func initListenerTestPhase2() {
	cfg := &config.Config{
		CommonConfig: config.CommonConfig{
			Hosts: []string{"fd://"},
		},
	}
	_, _, err := loadListeners(cfg, nil)
	var resp listenerTestResponse
	if err != nil {
		resp.Err = err.Error()
	}

	if err := json.NewEncoder(os.Stdout).Encode(resp); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// Test to make sure that the listen specs without an address are handled
// It requires a 2-phase setup due to how socket activation works (which we are using to test).
// It requires LISTEN_FDS and LISTEN_PID to be set in the environment.
//
// LISTEN_PID is used by socket activation to determine if the process is the one that should be activated.
// LISTEN_FDS is used by socket activation to determine how many file descriptors are passed to the process.
//
// We can sort of fake this without using extra processes, but it ends up not
// being a true test because that's not how socket activation is expected to
// work and we'll end up with nil listeners since the test framework has other
// file descriptors open.
//
// This is not currently testing `tcp://` or `unix://` listen specs without an address because those can conflict with the machine running the test.
// This could be worked around by using linux namespaces, however that would require root privileges which unit tests don't typically have.
func TestLoadListenerNoAddr(t *testing.T) {
	cmd := reexec.Command(testListenerNoAddrCmdPhase1)
	stdout := bytes.NewBuffer(nil)
	cmd.Stdout = stdout
	stderr := bytes.NewBuffer(nil)
	cmd.Stderr = stderr

	assert.NilError(t, cmd.Run(), stderr.String())

	var resp listenerTestResponse
	assert.NilError(t, json.NewDecoder(stdout).Decode(&resp))
	assert.Equal(t, resp.Err, "")
}
