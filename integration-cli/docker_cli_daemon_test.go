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
	"time"

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
	defer deleteAllContainers()

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

	logDone("daemon - run,iptables - iptables rules cleaned after daemon restart")
}

func TestDaemonIptablesCreate(t *testing.T) {
	defer deleteAllContainers()

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
	if !strings.Contains(string(content), `level=debug`) {
		t.Fatalf(`Missing level="debug" in log file:\n%s`, string(content))
	}

	d = NewDaemon(t)
	if err := d.Start("--log-level=fatal"); err != nil {
		t.Fatal(err)
	}
	d.Stop()
	content, _ = ioutil.ReadFile(d.logFile.Name())
	if strings.Contains(string(content), `level=debug`) {
		t.Fatalf(`Should not have level="debug" in log file:\n%s`, string(content))
	}

	d = NewDaemon(t)
	if err := d.Start("-D"); err != nil {
		t.Fatal(err)
	}
	d.Stop()
	content, _ = ioutil.ReadFile(d.logFile.Name())
	if !strings.Contains(string(content), `level=debug`) {
		t.Fatalf(`Missing level="debug" in log file using -D:\n%s`, string(content))
	}

	d = NewDaemon(t)
	if err := d.Start("--debug"); err != nil {
		t.Fatal(err)
	}
	d.Stop()
	content, _ = ioutil.ReadFile(d.logFile.Name())
	if !strings.Contains(string(content), `level=debug`) {
		t.Fatalf(`Missing level="debug" in log file using --debug:\n%s`, string(content))
	}

	d = NewDaemon(t)
	if err := d.Start("--debug", "--log-level=fatal"); err != nil {
		t.Fatal(err)
	}
	d.Stop()
	content, _ = ioutil.ReadFile(d.logFile.Name())
	if !strings.Contains(string(content), `level=debug`) {
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
//	without corresponding volume json
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

// GH#11320 - verify that the daemon exits on failure properly
// Note that this explicitly tests the conflict of {-b,--bridge} and {--bip} options as the means
// to get a daemon init failure; no other tests for -b/--bip conflict are therefore required
func TestDaemonExitOnFailure(t *testing.T) {
	d := NewDaemon(t)
	defer d.Stop()

	//attempt to start daemon with incorrect flags (we know -b and --bip conflict)
	if err := d.Start("--bridge", "nosuchbridge", "--bip", "1.1.1.1"); err != nil {
		//verify we got the right error
		if !strings.Contains(err.Error(), "Daemon exited and never started") {
			t.Fatalf("Expected daemon not to start, got %v", err)
		}
		// look in the log and make sure we got the message that daemon is shutting down
		runCmd := exec.Command("grep", "Shutting down daemon due to", d.LogfileName())
		if out, _, err := runCommandWithOutput(runCmd); err != nil {
			t.Fatalf("Expected 'shutting down daemon due to error' message; but doesn't exist in log: %q, err: %v", out, err)
		}
	} else {
		//if we didn't get an error and the daemon is running, this is a failure
		d.Stop()
		t.Fatal("Conflicting options should cause the daemon to error out with a failure")
	}

	logDone("daemon - verify no start on daemon init errors")
}

func TestDaemonUlimitDefaults(t *testing.T) {
	testRequires(t, NativeExecDriver)
	d := NewDaemon(t)

	if err := d.StartWithBusybox("--default-ulimit", "nofile=42:42", "--default-ulimit", "nproc=1024:1024"); err != nil {
		t.Fatal(err)
	}

	out, err := d.Cmd("run", "--ulimit", "nproc=2048", "--name=test", "busybox", "/bin/sh", "-c", "echo $(ulimit -n); echo $(ulimit -p)")
	if err != nil {
		t.Fatal(out, err)
	}

	outArr := strings.Split(out, "\n")
	if len(outArr) < 2 {
		t.Fatal("got unexpected output: %s", out)
	}
	nofile := strings.TrimSpace(outArr[0])
	nproc := strings.TrimSpace(outArr[1])

	if nofile != "42" {
		t.Fatalf("expected `ulimit -n` to be `42`, got: %s", nofile)
	}
	if nproc != "2048" {
		t.Fatalf("exepcted `ulimit -p` to be 2048, got: %s", nproc)
	}

	// Now restart daemon with a new default
	if err := d.Restart("--default-ulimit", "nofile=43"); err != nil {
		t.Fatal(err)
	}

	out, err = d.Cmd("start", "-a", "test")
	if err != nil {
		t.Fatal(err)
	}

	outArr = strings.Split(out, "\n")
	if len(outArr) < 2 {
		t.Fatal("got unexpected output: %s", out)
	}
	nofile = strings.TrimSpace(outArr[0])
	nproc = strings.TrimSpace(outArr[1])

	if nofile != "43" {
		t.Fatalf("expected `ulimit -n` to be `43`, got: %s", nofile)
	}
	if nproc != "2048" {
		t.Fatalf("exepcted `ulimit -p` to be 2048, got: %s", nproc)
	}

	logDone("daemon - default ulimits are applied")
}

// #11315
func TestDaemonRestartRenameContainer(t *testing.T) {
	d := NewDaemon(t)
	if err := d.StartWithBusybox(); err != nil {
		t.Fatal(err)
	}

	if out, err := d.Cmd("run", "--name=test", "busybox"); err != nil {
		t.Fatal(err, out)
	}

	if out, err := d.Cmd("rename", "test", "test2"); err != nil {
		t.Fatal(err, out)
	}

	if err := d.Restart(); err != nil {
		t.Fatal(err)
	}

	if out, err := d.Cmd("start", "test2"); err != nil {
		t.Fatal(err, out)
	}

	logDone("daemon - rename persists through daemon restart")
}

func TestDaemonLoggingDriverDefault(t *testing.T) {
	d := NewDaemon(t)

	if err := d.StartWithBusybox(); err != nil {
		t.Fatal(err)
	}
	defer d.Stop()

	out, err := d.Cmd("run", "-d", "busybox", "echo", "testline")
	if err != nil {
		t.Fatal(out, err)
	}
	id := strings.TrimSpace(out)

	if out, err := d.Cmd("wait", id); err != nil {
		t.Fatal(out, err)
	}
	logPath := filepath.Join(d.folder, "graph", "containers", id, id+"-json.log")

	if _, err := os.Stat(logPath); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(logPath)
	if err != nil {
		t.Fatal(err)
	}
	var res struct {
		Log    string    `json:log`
		Stream string    `json:stream`
		Time   time.Time `json:time`
	}
	if err := json.NewDecoder(f).Decode(&res); err != nil {
		t.Fatal(err)
	}
	if res.Log != "testline\n" {
		t.Fatalf("Unexpected log line: %q, expected: %q", res.Log, "testline\n")
	}
	if res.Stream != "stdout" {
		t.Fatalf("Unexpected stream: %q, expected: %q", res.Stream, "stdout")
	}
	if !time.Now().After(res.Time) {
		t.Fatalf("Log time %v in future", res.Time)
	}
	logDone("daemon - default 'json-file' logging driver")
}

func TestDaemonLoggingDriverDefaultOverride(t *testing.T) {
	d := NewDaemon(t)

	if err := d.StartWithBusybox(); err != nil {
		t.Fatal(err)
	}
	defer d.Stop()

	out, err := d.Cmd("run", "-d", "--log-driver=none", "busybox", "echo", "testline")
	if err != nil {
		t.Fatal(out, err)
	}
	id := strings.TrimSpace(out)

	if out, err := d.Cmd("wait", id); err != nil {
		t.Fatal(out, err)
	}
	logPath := filepath.Join(d.folder, "graph", "containers", id, id+"-json.log")

	if _, err := os.Stat(logPath); err == nil || !os.IsNotExist(err) {
		t.Fatalf("%s shouldn't exits, error on Stat: %s", logPath, err)
	}
	logDone("daemon - default logging driver override in run")
}

func TestDaemonLoggingDriverNone(t *testing.T) {
	d := NewDaemon(t)

	if err := d.StartWithBusybox("--log-driver=none"); err != nil {
		t.Fatal(err)
	}
	defer d.Stop()

	out, err := d.Cmd("run", "-d", "busybox", "echo", "testline")
	if err != nil {
		t.Fatal(out, err)
	}
	id := strings.TrimSpace(out)
	if out, err := d.Cmd("wait", id); err != nil {
		t.Fatal(out, err)
	}

	logPath := filepath.Join(d.folder, "graph", "containers", id, id+"-json.log")

	if _, err := os.Stat(logPath); err == nil || !os.IsNotExist(err) {
		t.Fatalf("%s shouldn't exits, error on Stat: %s", logPath, err)
	}
	logDone("daemon - 'none' logging driver")
}

func TestDaemonLoggingDriverNoneOverride(t *testing.T) {
	d := NewDaemon(t)

	if err := d.StartWithBusybox("--log-driver=none"); err != nil {
		t.Fatal(err)
	}
	defer d.Stop()

	out, err := d.Cmd("run", "-d", "--log-driver=json-file", "busybox", "echo", "testline")
	if err != nil {
		t.Fatal(out, err)
	}
	id := strings.TrimSpace(out)

	if out, err := d.Cmd("wait", id); err != nil {
		t.Fatal(out, err)
	}
	logPath := filepath.Join(d.folder, "graph", "containers", id, id+"-json.log")

	if _, err := os.Stat(logPath); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(logPath)
	if err != nil {
		t.Fatal(err)
	}
	var res struct {
		Log    string    `json:log`
		Stream string    `json:stream`
		Time   time.Time `json:time`
	}
	if err := json.NewDecoder(f).Decode(&res); err != nil {
		t.Fatal(err)
	}
	if res.Log != "testline\n" {
		t.Fatalf("Unexpected log line: %q, expected: %q", res.Log, "testline\n")
	}
	if res.Stream != "stdout" {
		t.Fatalf("Unexpected stream: %q, expected: %q", res.Stream, "stdout")
	}
	if !time.Now().After(res.Time) {
		t.Fatalf("Log time %v in future", res.Time)
	}
	logDone("daemon - 'none' logging driver override in run")
}

func TestDaemonLoggingDriverNoneLogsError(t *testing.T) {
	d := NewDaemon(t)

	if err := d.StartWithBusybox("--log-driver=none"); err != nil {
		t.Fatal(err)
	}
	defer d.Stop()

	out, err := d.Cmd("run", "-d", "busybox", "echo", "testline")
	if err != nil {
		t.Fatal(out, err)
	}
	id := strings.TrimSpace(out)
	out, err = d.Cmd("logs", id)
	if err == nil {
		t.Fatalf("Logs should fail with \"none\" driver")
	}
	if !strings.Contains(out, `\"logs\" command is supported only for \"json-file\" logging driver`) {
		t.Fatalf("There should be error about non-json-file driver, got %s", out)
	}
	logDone("daemon - logs not available for non-json-file drivers")
}

func TestDaemonDots(t *testing.T) {
	defer deleteAllContainers()
	d := NewDaemon(t)
	if err := d.StartWithBusybox(); err != nil {
		t.Fatal(err)
	}

	// Now create 4 containers
	if _, err := d.Cmd("create", "busybox"); err != nil {
		t.Fatalf("Error creating container: %q", err)
	}
	if _, err := d.Cmd("create", "busybox"); err != nil {
		t.Fatalf("Error creating container: %q", err)
	}
	if _, err := d.Cmd("create", "busybox"); err != nil {
		t.Fatalf("Error creating container: %q", err)
	}
	if _, err := d.Cmd("create", "busybox"); err != nil {
		t.Fatalf("Error creating container: %q", err)
	}

	d.Stop()

	d.Start("--log-level=debug")
	d.Stop()
	content, _ := ioutil.ReadFile(d.logFile.Name())
	if strings.Contains(string(content), "....") {
		t.Fatalf("Debug level should not have ....\n%s", string(content))
	}

	d.Start("--log-level=error")
	d.Stop()
	content, _ = ioutil.ReadFile(d.logFile.Name())
	if strings.Contains(string(content), "....") {
		t.Fatalf("Error level should not have ....\n%s", string(content))
	}

	d.Start("--log-level=info")
	d.Stop()
	content, _ = ioutil.ReadFile(d.logFile.Name())
	if !strings.Contains(string(content), "....") {
		t.Fatalf("Info level should have ....\n%s", string(content))
	}

	logDone("daemon - test dots on INFO")
}

func TestDaemonUnixSockCleanedUp(t *testing.T) {
	d := NewDaemon(t)
	dir, err := ioutil.TempDir("", "socket-cleanup-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	sockPath := filepath.Join(dir, "docker.sock")
	if err := d.Start("--host", "unix://"+sockPath); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(sockPath); err != nil {
		t.Fatal("socket does not exist")
	}

	if err := d.Stop(); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(sockPath); err == nil || !os.IsNotExist(err) {
		t.Fatal("unix socket is not cleaned up")
	}

	logDone("daemon - unix socket is cleaned up")
}
