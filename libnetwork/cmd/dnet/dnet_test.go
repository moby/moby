package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/docker/libnetwork/netutils"
)

const dnetCommandName = "dnet"

var origStdOut = os.Stdout

func TestDnetDaemonCustom(t *testing.T) {
	if !netutils.IsRunningInContainer() {
		t.Skip("This test must run inside a container ")
	}
	customPort := 4567
	doneChan := make(chan bool)
	go func() {
		args := []string{dnetCommandName, "-d", fmt.Sprintf("-H=:%d", customPort)}
		executeDnetCommand(t, args, true)
		doneChan <- true
	}()

	select {
	case <-doneChan:
		t.Fatal("dnet Daemon is not supposed to exit")
	case <-time.After(3 * time.Second):
		args := []string{dnetCommandName, "-d=false", fmt.Sprintf("-H=:%d", customPort), "-D", "network", "ls"}
		executeDnetCommand(t, args, true)
	}
}

func TestDnetDaemonInvalidCustom(t *testing.T) {
	if !netutils.IsRunningInContainer() {
		t.Skip("This test must run inside a container ")
	}
	customPort := 4668
	doneChan := make(chan bool)
	go func() {
		args := []string{dnetCommandName, "-d=true", fmt.Sprintf("-H=:%d", customPort)}
		executeDnetCommand(t, args, true)
		doneChan <- true
	}()

	select {
	case <-doneChan:
		t.Fatal("dnet Daemon is not supposed to exit")
	case <-time.After(3 * time.Second):
		args := []string{dnetCommandName, "-d=false", "-H=:6669", "-D", "network", "ls"}
		executeDnetCommand(t, args, false)
	}
}

func TestDnetDaemonInvalidParams(t *testing.T) {
	if !netutils.IsRunningInContainer() {
		t.Skip("This test must run inside a container ")
	}
	args := []string{dnetCommandName, "-d=false", "-H=tcp:/127.0.0.1:8080"}
	executeDnetCommand(t, args, false)

	args = []string{dnetCommandName, "-d=false", "-H=unix://var/run/dnet.sock"}
	executeDnetCommand(t, args, false)

	args = []string{dnetCommandName, "-d=false", "-H=", "-l=invalid"}
	executeDnetCommand(t, args, false)

	args = []string{dnetCommandName, "-d=false", "-H=", "-l=error", "invalid"}
	executeDnetCommand(t, args, false)
}

func TestDnetDefaultsWithFlags(t *testing.T) {
	if !netutils.IsRunningInContainer() {
		t.Skip("This test must run inside a container ")
	}
	doneChan := make(chan bool)
	go func() {
		args := []string{dnetCommandName, "-d=true", "-H=", "-l=error"}
		executeDnetCommand(t, args, true)
		doneChan <- true
	}()

	select {
	case <-doneChan:
		t.Fatal("dnet Daemon is not supposed to exit")
	case <-time.After(3 * time.Second):
		args := []string{dnetCommandName, "-d=false", "network", "create", "-d=null", "test"}
		executeDnetCommand(t, args, true)

		args = []string{dnetCommandName, "-d=false", "-D", "network", "ls"}
		executeDnetCommand(t, args, true)
	}
}

func TestDnetMain(t *testing.T) {
	if !netutils.IsRunningInContainer() {
		t.Skip("This test must run inside a container ")
	}
	customPort := 4568
	doneChan := make(chan bool)
	go func() {
		args := []string{dnetCommandName, "-d=true", "-h=false", fmt.Sprintf("-H=:%d", customPort)}
		os.Args = args
		main()
		doneChan <- true
	}()
	select {
	case <-doneChan:
		t.Fatal("dnet Daemon is not supposed to exit")
	case <-time.After(2 * time.Second):
	}
}

func executeDnetCommand(t *testing.T, args []string, shouldSucced bool) {
	_, w, _ := os.Pipe()
	os.Stdout = w

	os.Args = args
	err := dnetCommand(ioutil.Discard, ioutil.Discard)
	if shouldSucced && err != nil {
		os.Stdout = origStdOut
		t.Fatalf("cli [%v] must succeed, but failed with an error :  %v", args, err)
	} else if !shouldSucced && err == nil {
		os.Stdout = origStdOut
		t.Fatalf("cli [%v] must fail, but succeeded with an error :  %v", args, err)
	}
	os.Stdout = origStdOut
}
