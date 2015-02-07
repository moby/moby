// +build daemon

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/libtrust"
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
	if _, err := d.Cmd("run", "-d", "--volumes-from", "volrestarttest1", "--name", "volrestarttest2", "busybox", "top"); err != nil {
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

func TestDaemonAllocatesListeningPort(t *testing.T) {
	listeningPorts := [][]string{
		{"0.0.0.0", "0.0.0.0", "5678"},
		{"127.0.0.1", "127.0.0.1", "1234"},
		{"localhost", "127.0.0.1", "1235"},
	}

	cmdArgs := []string{}
	for _, hostDirective := range listeningPorts {
		cmdArgs = append(cmdArgs, "--host", fmt.Sprintf("tcp://%s:%s", hostDirective[0], hostDirective[2]))
	}

	d := NewDaemon(t)
	if err := d.StartWithBusybox(cmdArgs...); err != nil {
		t.Fatalf("Could not start daemon with busybox: %v", err)
	}
	defer d.Stop()

	for _, hostDirective := range listeningPorts {
		output, err := d.Cmd("run", "-p", fmt.Sprintf("%s:%s:80", hostDirective[1], hostDirective[2]), "busybox", "true")
		if err == nil {
			t.Fatalf("Container should not start, expected port already allocated error: %q", output)
		} else if !strings.Contains(output, "port is already allocated") {
			t.Fatalf("Expected port is already allocated error: %q", output)
		}
	}

	logDone("daemon - daemon listening port is allocated")
}

// #9629
func TestDaemonVolumesBindsRefs(t *testing.T) {
	d := NewDaemon(t)

	if err := d.StartWithBusybox(); err != nil {
		t.Fatal(err)
	}

	tmp, err := ioutil.TempDir(os.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	if err := ioutil.WriteFile(tmp+"/test", []byte("testing"), 0655); err != nil {
		t.Fatal(err)
	}

	if out, err := d.Cmd("create", "-v", tmp+":/foo", "--name=voltest", "busybox"); err != nil {
		t.Fatal(err, out)
	}

	if err := d.Restart(); err != nil {
		t.Fatal(err)
	}

	if out, err := d.Cmd("run", "--volumes-from=voltest", "--name=consumer", "busybox", "/bin/sh", "-c", "[ -f /foo/test ]"); err != nil {
		t.Fatal(err, out)
	}

	logDone("daemon - bind refs in data-containers survive daemon restart")
}

func TestDaemonKeyGeneration(t *testing.T) {
	// TODO: skip or update for Windows daemon
	os.Remove("/etc/docker/key.json")
	d := NewDaemon(t)
	if err := d.Start(); err != nil {
		t.Fatalf("Could not start daemon: %v", err)
	}
	d.Stop()

	k, err := libtrust.LoadKeyFile("/etc/docker/key.json")
	if err != nil {
		t.Fatalf("Error opening key file")
	}
	kid := k.KeyID()
	// Test Key ID is a valid fingerprint (e.g. QQXN:JY5W:TBXI:MK3X:GX6P:PD5D:F56N:NHCS:LVRZ:JA46:R24J:XEFF)
	if len(kid) != 59 {
		t.Fatalf("Bad key ID: %s", kid)
	}

	logDone("daemon - key generation")
}

func TestDaemonKeyMigration(t *testing.T) {
	// TODO: skip or update for Windows daemon
	os.Remove("/etc/docker/key.json")
	k1, err := libtrust.GenerateECP256PrivateKey()
	if err != nil {
		t.Fatalf("Error generating private key: %s", err)
	}
	if err := os.MkdirAll(filepath.Join(os.Getenv("HOME"), ".docker"), 0755); err != nil {
		t.Fatalf("Error creating .docker directory: %s", err)
	}
	if err := libtrust.SaveKey(filepath.Join(os.Getenv("HOME"), ".docker", "key.json"), k1); err != nil {
		t.Fatalf("Error saving private key: %s", err)
	}

	d := NewDaemon(t)
	if err := d.Start(); err != nil {
		t.Fatalf("Could not start daemon: %v", err)
	}
	d.Stop()

	k2, err := libtrust.LoadKeyFile("/etc/docker/key.json")
	if err != nil {
		t.Fatalf("Error opening key file")
	}
	if k1.KeyID() != k2.KeyID() {
		t.Fatalf("Key not migrated")
	}

	logDone("daemon - key migration")
}

// Simulate an older daemon (pre 1.3) coming up with volumes specified in containers
//	without corrosponding volume json
func TestDaemonUpgradeWithVolumes(t *testing.T) {
	d := NewDaemon(t)

	graphDir := filepath.Join(os.TempDir(), "docker-test")
	defer os.RemoveAll(graphDir)
	if err := d.StartWithBusybox("-g", graphDir); err != nil {
		t.Fatal(err)
	}

	tmpDir := filepath.Join(os.TempDir(), "test")
	defer os.RemoveAll(tmpDir)

	if out, err := d.Cmd("create", "-v", tmpDir+":/foo", "--name=test", "busybox"); err != nil {
		t.Fatal(err, out)
	}

	if err := d.Stop(); err != nil {
		t.Fatal(err)
	}

	// Remove this since we're expecting the daemon to re-create it too
	if err := os.RemoveAll(tmpDir); err != nil {
		t.Fatal(err)
	}

	configDir := filepath.Join(graphDir, "volumes")

	if err := os.RemoveAll(configDir); err != nil {
		t.Fatal(err)
	}

	if err := d.Start("-g", graphDir); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		t.Fatalf("expected volume path %s to exist but it does not", tmpDir)
	}

	dir, err := ioutil.ReadDir(configDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(dir) == 0 {
		t.Fatalf("expected volumes config dir to contain data for new volume")
	}

	// Now with just removing the volume config and not the volume data
	if err := d.Stop(); err != nil {
		t.Fatal(err)
	}

	if err := os.RemoveAll(configDir); err != nil {
		t.Fatal(err)
	}

	if err := d.Start("-g", graphDir); err != nil {
		t.Fatal(err)
	}

	dir, err = ioutil.ReadDir(configDir)
	if err != nil {
		t.Fatal(err)
	}

	if len(dir) == 0 {
		t.Fatalf("expected volumes config dir to contain data for new volume")
	}

	logDone("daemon - volumes from old(pre 1.3) daemon work")
}
