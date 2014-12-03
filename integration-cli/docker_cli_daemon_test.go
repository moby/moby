package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestDaemonRestartWithRunningContainersPorts(t *testing.T) {
	d := NewDaemon(t)
	if err := d.StartWithBusybox(); err != nil {
		t.Fatalf("Could not start daemon with busybox: %v", err)
	}
	defer d.Stop()

	if out, err := d.Cmd("run", "-d", "--name", "top1", "-p", "1234:80", "--restart", "always", "busybox:latest", "top"); err != nil {
		t.Fatalf("Could not run top1: err=%v\n%s", err, out)
	}
	// --restart=no by default
	if out, err := d.Cmd("run", "-d", "--name", "top2", "-p", "80", "busybox:latest", "top"); err != nil {
		t.Fatalf("Could not run top2: err=%v\n%s", err, out)
	}

	testRun := func(m map[string]bool, prefix string) {
		var format string
		for c, shouldRun := range m {
			out, err := d.Cmd("ps")
			if err != nil {
				t.Fatalf("Could not run ps: err=%v\n%q", err, out)
			}
			if shouldRun {
				format = "%scontainer %q is not running"
			} else {
				format = "%scontainer %q is running"
			}
			if shouldRun != strings.Contains(out, c) {
				t.Fatalf(format, prefix, c)
			}
		}
	}

	testRun(map[string]bool{"top1": true, "top2": true}, "")

	if err := d.Restart(); err != nil {
		t.Fatalf("Could not restart daemon: %v", err)
	}

	testRun(map[string]bool{"top1": true, "top2": false}, "After daemon restart: ")

	logDone("daemon - running containers on daemon restart")
}

func TestDaemonRestartWithVolumesRefs(t *testing.T) {
	d := NewDaemon(t)
	if err := d.StartWithBusybox(); err != nil {
		t.Fatal(err)
	}
	defer d.Stop()

	if out, err := d.Cmd("run", "-d", "--name", "volrestarttest1", "-v", "/foo", "busybox"); err != nil {
		t.Fatal(err, out)
	}
	if err := d.Restart(); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Cmd("run", "-d", "--volumes-from", "volrestarttest1", "--name", "volrestarttest2", "busybox"); err != nil {
		t.Fatal(err)
	}
	if out, err := d.Cmd("rm", "-fv", "volrestarttest2"); err != nil {
		t.Fatal(err, out)
	}
	v, err := d.Cmd("inspect", "--format", "{{ json .Volumes }}", "volrestarttest1")
	if err != nil {
		t.Fatal(err)
	}
	volumes := make(map[string]string)
	json.Unmarshal([]byte(v), &volumes)
	if _, err := os.Stat(volumes["/foo"]); err != nil {
		t.Fatalf("Expected volume to exist: %s - %s", volumes["/foo"], err)
	}

	logDone("daemon - volume refs are restored")
}

func TestDaemonStartIptablesFalse(t *testing.T) {
	d := NewDaemon(t)
	if err := d.Start("--iptables=false"); err != nil {
		t.Fatalf("we should have been able to start the daemon with passing iptables=false: %v", err)
	}
	d.Stop()

	logDone("daemon - started daemon with iptables=false")
}

// Issue #8444: If docker0 bridge is modified (intentionally or unintentionally) and
// no longer has an IP associated, we should gracefully handle that case and associate
// an IP with it rather than fail daemon start
func TestDaemonStartBridgeWithoutIPAssociation(t *testing.T) {
	d := NewDaemon(t)
	// rather than depending on brctl commands to verify docker0 is created and up
	// let's start the daemon and stop it, and then make a modification to run the
	// actual test
	if err := d.Start(); err != nil {
		t.Fatalf("Could not start daemon: %v", err)
	}
	if err := d.Stop(); err != nil {
		t.Fatalf("Could not stop daemon: %v", err)
	}

	// now we will remove the ip from docker0 and then try starting the daemon
	ipCmd := exec.Command("ip", "addr", "flush", "dev", "docker0")
	stdout, stderr, _, err := runCommandWithStdoutStderr(ipCmd)
	if err != nil {
		t.Fatalf("failed to remove docker0 IP association: %v, stdout: %q, stderr: %q", err, stdout, stderr)
	}

	if err := d.Start(); err != nil {
		warning := "**WARNING: Docker bridge network in bad state--delete docker0 bridge interface to fix"
		t.Fatalf("Could not start daemon when docker0 has no IP address: %v\n%s", err, warning)
	}

	// cleanup - stop the daemon if test passed
	if err := d.Stop(); err != nil {
		t.Fatalf("Could not stop daemon: %v", err)
	}

	logDone("daemon - successful daemon start when bridge has no IP association")
}

func TestDaemonIptablesClean(t *testing.T) {
	d := NewDaemon(t)
	if err := d.StartWithBusybox(); err != nil {
		t.Fatalf("Could not start daemon with busybox: %v", err)
	}
	defer d.Stop()

	if out, err := d.Cmd("run", "-d", "--name", "top", "-p", "80", "busybox:latest", "top"); err != nil {
		t.Fatalf("Could not run top: %s, %v", out, err)
	}

	// get output from iptables with container running
	ipTablesSearchString := "tcp dpt:80"
	ipTablesCmd := exec.Command("iptables", "-nvL")
	out, _, err := runCommandWithOutput(ipTablesCmd)
	if err != nil {
		t.Fatalf("Could not run iptables -nvL: %s, %v", out, err)
	}

	if !strings.Contains(out, ipTablesSearchString) {
		t.Fatalf("iptables output should have contained %q, but was %q", ipTablesSearchString, out)
	}

	if err := d.Stop(); err != nil {
		t.Fatalf("Could not stop daemon: %v", err)
	}

	// get output from iptables after restart
	ipTablesCmd = exec.Command("iptables", "-nvL")
	out, _, err = runCommandWithOutput(ipTablesCmd)
	if err != nil {
		t.Fatalf("Could not run iptables -nvL: %s, %v", out, err)
	}

	if strings.Contains(out, ipTablesSearchString) {
		t.Fatalf("iptables output should not have contained %q, but was %q", ipTablesSearchString, out)
	}

	deleteAllContainers()

	logDone("daemon - run,iptables - iptables rules cleaned after daemon restart")
}

func TestDaemonIptablesCreate(t *testing.T) {
	d := NewDaemon(t)
	if err := d.StartWithBusybox(); err != nil {
		t.Fatalf("Could not start daemon with busybox: %v", err)
	}
	defer d.Stop()

	if out, err := d.Cmd("run", "-d", "--name", "top", "--restart=always", "-p", "80", "busybox:latest", "top"); err != nil {
		t.Fatalf("Could not run top: %s, %v", out, err)
	}

	// get output from iptables with container running
	ipTablesSearchString := "tcp dpt:80"
	ipTablesCmd := exec.Command("iptables", "-nvL")
	out, _, err := runCommandWithOutput(ipTablesCmd)
	if err != nil {
		t.Fatalf("Could not run iptables -nvL: %s, %v", out, err)
	}

	if !strings.Contains(out, ipTablesSearchString) {
		t.Fatalf("iptables output should have contained %q, but was %q", ipTablesSearchString, out)
	}

	if err := d.Restart(); err != nil {
		t.Fatalf("Could not restart daemon: %v", err)
	}

	// make sure the container is not running
	runningOut, err := d.Cmd("inspect", "--format='{{.State.Running}}'", "top")
	if err != nil {
		t.Fatalf("Could not inspect on container: %s, %v", out, err)
	}
	if strings.TrimSpace(runningOut) != "true" {
		t.Fatalf("Container should have been restarted after daemon restart. Status running should have been true but was: %q", strings.TrimSpace(runningOut))
	}

	// get output from iptables after restart
	ipTablesCmd = exec.Command("iptables", "-nvL")
	out, _, err = runCommandWithOutput(ipTablesCmd)
	if err != nil {
		t.Fatalf("Could not run iptables -nvL: %s, %v", out, err)
	}

	if !strings.Contains(out, ipTablesSearchString) {
		t.Fatalf("iptables output after restart should have contained %q, but was %q", ipTablesSearchString, out)
	}

	deleteAllContainers()

	logDone("daemon - run,iptables - iptables rules for always restarted container created after daemon restart")
}

func TestDaemonLoggingLevel(t *testing.T) {
	d := NewDaemon(t)

	if err := d.Start("--log-level=bogus"); err == nil {
		t.Fatal("Daemon should not have been able to start")
	}

	d = NewDaemon(t)
	if err := d.Start("--log-level=debug"); err != nil {
		t.Fatal(err)
	}
	d.Stop()
	content, _ := ioutil.ReadFile(d.logFile.Name())
	if !strings.Contains(string(content), `level="debug"`) {
		t.Fatalf(`Missing level="debug" in log file:\n%s`, string(content))
	}

	d = NewDaemon(t)
	if err := d.Start("--log-level=fatal"); err != nil {
		t.Fatal(err)
	}
	d.Stop()
	content, _ = ioutil.ReadFile(d.logFile.Name())
	if strings.Contains(string(content), `level="debug"`) {
		t.Fatalf(`Should not have level="debug" in log file:\n%s`, string(content))
	}

	d = NewDaemon(t)
	if err := d.Start("-D"); err != nil {
		t.Fatal(err)
	}
	d.Stop()
	content, _ = ioutil.ReadFile(d.logFile.Name())
	if !strings.Contains(string(content), `level="debug"`) {
		t.Fatalf(`Missing level="debug" in log file using -D:\n%s`, string(content))
	}

	d = NewDaemon(t)
	if err := d.Start("--debug"); err != nil {
		t.Fatal(err)
	}
	d.Stop()
	content, _ = ioutil.ReadFile(d.logFile.Name())
	if !strings.Contains(string(content), `level="debug"`) {
		t.Fatalf(`Missing level="debug" in log file using --debug:\n%s`, string(content))
	}

	d = NewDaemon(t)
	if err := d.Start("--debug", "--log-level=fatal"); err != nil {
		t.Fatal(err)
	}
	d.Stop()
	content, _ = ioutil.ReadFile(d.logFile.Name())
	if !strings.Contains(string(content), `level="debug"`) {
		t.Fatalf(`Missing level="debug" in log file when using both --debug and --log-level=fatal:\n%s`, string(content))
	}

	logDone("daemon - Logging Level")
}
