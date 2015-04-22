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
	"time"

	"github.com/docker/libtrust"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestDaemonRestartWithRunningContainersPorts(c *check.C) {
	d := NewDaemon(c)
	if err := d.StartWithBusybox(); err != nil {
		c.Fatalf("Could not start daemon with busybox: %v", err)
	}
	defer d.Stop()

	if out, err := d.Cmd("run", "-d", "--name", "top1", "-p", "1234:80", "--restart", "always", "busybox:latest", "top"); err != nil {
		c.Fatalf("Could not run top1: err=%v\n%s", err, out)
	}
	// --restart=no by default
	if out, err := d.Cmd("run", "-d", "--name", "top2", "-p", "80", "busybox:latest", "top"); err != nil {
		c.Fatalf("Could not run top2: err=%v\n%s", err, out)
	}

	testRun := func(m map[string]bool, prefix string) {
		var format string
		for cont, shouldRun := range m {
			out, err := d.Cmd("ps")
			if err != nil {
				c.Fatalf("Could not run ps: err=%v\n%q", err, out)
			}
			if shouldRun {
				format = "%scontainer %q is not running"
			} else {
				format = "%scontainer %q is running"
			}
			if shouldRun != strings.Contains(out, cont) {
				c.Fatalf(format, prefix, cont)
			}
		}
	}

	testRun(map[string]bool{"top1": true, "top2": true}, "")

	if err := d.Restart(); err != nil {
		c.Fatalf("Could not restart daemon: %v", err)
	}

	testRun(map[string]bool{"top1": true, "top2": false}, "After daemon restart: ")

}

func (s *DockerSuite) TestDaemonRestartWithVolumesRefs(c *check.C) {
	d := NewDaemon(c)
	if err := d.StartWithBusybox(); err != nil {
		c.Fatal(err)
	}
	defer d.Stop()

	if out, err := d.Cmd("run", "-d", "--name", "volrestarttest1", "-v", "/foo", "busybox"); err != nil {
		c.Fatal(err, out)
	}
	if err := d.Restart(); err != nil {
		c.Fatal(err)
	}
	if _, err := d.Cmd("run", "-d", "--volumes-from", "volrestarttest1", "--name", "volrestarttest2", "busybox", "top"); err != nil {
		c.Fatal(err)
	}
	if out, err := d.Cmd("rm", "-fv", "volrestarttest2"); err != nil {
		c.Fatal(err, out)
	}
	v, err := d.Cmd("inspect", "--format", "{{ json .Volumes }}", "volrestarttest1")
	if err != nil {
		c.Fatal(err)
	}
	volumes := make(map[string]string)
	json.Unmarshal([]byte(v), &volumes)
	if _, err := os.Stat(volumes["/foo"]); err != nil {
		c.Fatalf("Expected volume to exist: %s - %s", volumes["/foo"], err)
	}

}

func (s *DockerSuite) TestDaemonStartIptablesFalse(c *check.C) {
	d := NewDaemon(c)
	if err := d.Start("--iptables=false"); err != nil {
		c.Fatalf("we should have been able to start the daemon with passing iptables=false: %v", err)
	}
	d.Stop()

}

// Issue #8444: If docker0 bridge is modified (intentionally or unintentionally) and
// no longer has an IP associated, we should gracefully handle that case and associate
// an IP with it rather than fail daemon start
func (s *DockerSuite) TestDaemonStartBridgeWithoutIPAssociation(c *check.C) {
	d := NewDaemon(c)
	// rather than depending on brctl commands to verify docker0 is created and up
	// let's start the daemon and stop it, and then make a modification to run the
	// actual test
	if err := d.Start(); err != nil {
		c.Fatalf("Could not start daemon: %v", err)
	}
	if err := d.Stop(); err != nil {
		c.Fatalf("Could not stop daemon: %v", err)
	}

	// now we will remove the ip from docker0 and then try starting the daemon
	ipCmd := exec.Command("ip", "addr", "flush", "dev", "docker0")
	stdout, stderr, _, err := runCommandWithStdoutStderr(ipCmd)
	if err != nil {
		c.Fatalf("failed to remove docker0 IP association: %v, stdout: %q, stderr: %q", err, stdout, stderr)
	}

	if err := d.Start(); err != nil {
		warning := "**WARNING: Docker bridge network in bad state--delete docker0 bridge interface to fix"
		c.Fatalf("Could not start daemon when docker0 has no IP address: %v\n%s", err, warning)
	}

	// cleanup - stop the daemon if test passed
	if err := d.Stop(); err != nil {
		c.Fatalf("Could not stop daemon: %v", err)
	}

}

func (s *DockerSuite) TestDaemonIptablesClean(c *check.C) {

	d := NewDaemon(c)
	if err := d.StartWithBusybox(); err != nil {
		c.Fatalf("Could not start daemon with busybox: %v", err)
	}
	defer d.Stop()

	if out, err := d.Cmd("run", "-d", "--name", "top", "-p", "80", "busybox:latest", "top"); err != nil {
		c.Fatalf("Could not run top: %s, %v", out, err)
	}

	// get output from iptables with container running
	ipTablesSearchString := "tcp dpt:80"
	ipTablesCmd := exec.Command("iptables", "-nvL")
	out, _, err := runCommandWithOutput(ipTablesCmd)
	if err != nil {
		c.Fatalf("Could not run iptables -nvL: %s, %v", out, err)
	}

	if !strings.Contains(out, ipTablesSearchString) {
		c.Fatalf("iptables output should have contained %q, but was %q", ipTablesSearchString, out)
	}

	if err := d.Stop(); err != nil {
		c.Fatalf("Could not stop daemon: %v", err)
	}

	// get output from iptables after restart
	ipTablesCmd = exec.Command("iptables", "-nvL")
	out, _, err = runCommandWithOutput(ipTablesCmd)
	if err != nil {
		c.Fatalf("Could not run iptables -nvL: %s, %v", out, err)
	}

	if strings.Contains(out, ipTablesSearchString) {
		c.Fatalf("iptables output should not have contained %q, but was %q", ipTablesSearchString, out)
	}

}

func (s *DockerSuite) TestDaemonIptablesCreate(c *check.C) {

	d := NewDaemon(c)
	if err := d.StartWithBusybox(); err != nil {
		c.Fatalf("Could not start daemon with busybox: %v", err)
	}
	defer d.Stop()

	if out, err := d.Cmd("run", "-d", "--name", "top", "--restart=always", "-p", "80", "busybox:latest", "top"); err != nil {
		c.Fatalf("Could not run top: %s, %v", out, err)
	}

	// get output from iptables with container running
	ipTablesSearchString := "tcp dpt:80"
	ipTablesCmd := exec.Command("iptables", "-nvL")
	out, _, err := runCommandWithOutput(ipTablesCmd)
	if err != nil {
		c.Fatalf("Could not run iptables -nvL: %s, %v", out, err)
	}

	if !strings.Contains(out, ipTablesSearchString) {
		c.Fatalf("iptables output should have contained %q, but was %q", ipTablesSearchString, out)
	}

	if err := d.Restart(); err != nil {
		c.Fatalf("Could not restart daemon: %v", err)
	}

	// make sure the container is not running
	runningOut, err := d.Cmd("inspect", "--format='{{.State.Running}}'", "top")
	if err != nil {
		c.Fatalf("Could not inspect on container: %s, %v", out, err)
	}
	if strings.TrimSpace(runningOut) != "true" {
		c.Fatalf("Container should have been restarted after daemon restart. Status running should have been true but was: %q", strings.TrimSpace(runningOut))
	}

	// get output from iptables after restart
	ipTablesCmd = exec.Command("iptables", "-nvL")
	out, _, err = runCommandWithOutput(ipTablesCmd)
	if err != nil {
		c.Fatalf("Could not run iptables -nvL: %s, %v", out, err)
	}

	if !strings.Contains(out, ipTablesSearchString) {
		c.Fatalf("iptables output after restart should have contained %q, but was %q", ipTablesSearchString, out)
	}

}

func (s *DockerSuite) TestDaemonLoggingLevel(c *check.C) {
	d := NewDaemon(c)

	if err := d.Start("--log-level=bogus"); err == nil {
		c.Fatal("Daemon should not have been able to start")
	}

	d = NewDaemon(c)
	if err := d.Start("--log-level=debug"); err != nil {
		c.Fatal(err)
	}
	d.Stop()
	content, _ := ioutil.ReadFile(d.logFile.Name())
	if !strings.Contains(string(content), `level=debug`) {
		c.Fatalf(`Missing level="debug" in log file:\n%s`, string(content))
	}

	d = NewDaemon(c)
	if err := d.Start("--log-level=fatal"); err != nil {
		c.Fatal(err)
	}
	d.Stop()
	content, _ = ioutil.ReadFile(d.logFile.Name())
	if strings.Contains(string(content), `level=debug`) {
		c.Fatalf(`Should not have level="debug" in log file:\n%s`, string(content))
	}

	d = NewDaemon(c)
	if err := d.Start("-D"); err != nil {
		c.Fatal(err)
	}
	d.Stop()
	content, _ = ioutil.ReadFile(d.logFile.Name())
	if !strings.Contains(string(content), `level=debug`) {
		c.Fatalf(`Missing level="debug" in log file using -D:\n%s`, string(content))
	}

	d = NewDaemon(c)
	if err := d.Start("--debug"); err != nil {
		c.Fatal(err)
	}
	d.Stop()
	content, _ = ioutil.ReadFile(d.logFile.Name())
	if !strings.Contains(string(content), `level=debug`) {
		c.Fatalf(`Missing level="debug" in log file using --debug:\n%s`, string(content))
	}

	d = NewDaemon(c)
	if err := d.Start("--debug", "--log-level=fatal"); err != nil {
		c.Fatal(err)
	}
	d.Stop()
	content, _ = ioutil.ReadFile(d.logFile.Name())
	if !strings.Contains(string(content), `level=debug`) {
		c.Fatalf(`Missing level="debug" in log file when using both --debug and --log-level=fatal:\n%s`, string(content))
	}

}

func (s *DockerSuite) TestDaemonAllocatesListeningPort(c *check.C) {
	listeningPorts := [][]string{
		{"0.0.0.0", "0.0.0.0", "5678"},
		{"127.0.0.1", "127.0.0.1", "1234"},
		{"localhost", "127.0.0.1", "1235"},
	}

	cmdArgs := []string{}
	for _, hostDirective := range listeningPorts {
		cmdArgs = append(cmdArgs, "--host", fmt.Sprintf("tcp://%s:%s", hostDirective[0], hostDirective[2]))
	}

	d := NewDaemon(c)
	if err := d.StartWithBusybox(cmdArgs...); err != nil {
		c.Fatalf("Could not start daemon with busybox: %v", err)
	}
	defer d.Stop()

	for _, hostDirective := range listeningPorts {
		output, err := d.Cmd("run", "-p", fmt.Sprintf("%s:%s:80", hostDirective[1], hostDirective[2]), "busybox", "true")
		if err == nil {
			c.Fatalf("Container should not start, expected port already allocated error: %q", output)
		} else if !strings.Contains(output, "port is already allocated") {
			c.Fatalf("Expected port is already allocated error: %q", output)
		}
	}

}

// #9629
func (s *DockerSuite) TestDaemonVolumesBindsRefs(c *check.C) {
	d := NewDaemon(c)

	if err := d.StartWithBusybox(); err != nil {
		c.Fatal(err)
	}
	defer d.Stop()

	tmp, err := ioutil.TempDir(os.TempDir(), "")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	if err := ioutil.WriteFile(tmp+"/test", []byte("testing"), 0655); err != nil {
		c.Fatal(err)
	}

	if out, err := d.Cmd("create", "-v", tmp+":/foo", "--name=voltest", "busybox"); err != nil {
		c.Fatal(err, out)
	}

	if err := d.Restart(); err != nil {
		c.Fatal(err)
	}

	if out, err := d.Cmd("run", "--volumes-from=voltest", "--name=consumer", "busybox", "/bin/sh", "-c", "[ -f /foo/test ]"); err != nil {
		c.Fatal(err, out)
	}

}

func (s *DockerSuite) TestDaemonKeyGeneration(c *check.C) {
	// TODO: skip or update for Windows daemon
	os.Remove("/etc/docker/key.json")
	d := NewDaemon(c)
	if err := d.Start(); err != nil {
		c.Fatalf("Could not start daemon: %v", err)
	}
	d.Stop()

	k, err := libtrust.LoadKeyFile("/etc/docker/key.json")
	if err != nil {
		c.Fatalf("Error opening key file")
	}
	kid := k.KeyID()
	// Test Key ID is a valid fingerprint (e.g. QQXN:JY5W:TBXI:MK3X:GX6P:PD5D:F56N:NHCS:LVRZ:JA46:R24J:XEFF)
	if len(kid) != 59 {
		c.Fatalf("Bad key ID: %s", kid)
	}

}

func (s *DockerSuite) TestDaemonKeyMigration(c *check.C) {
	// TODO: skip or update for Windows daemon
	os.Remove("/etc/docker/key.json")
	k1, err := libtrust.GenerateECP256PrivateKey()
	if err != nil {
		c.Fatalf("Error generating private key: %s", err)
	}
	if err := os.MkdirAll(filepath.Join(os.Getenv("HOME"), ".docker"), 0755); err != nil {
		c.Fatalf("Error creating .docker directory: %s", err)
	}
	if err := libtrust.SaveKey(filepath.Join(os.Getenv("HOME"), ".docker", "key.json"), k1); err != nil {
		c.Fatalf("Error saving private key: %s", err)
	}

	d := NewDaemon(c)
	if err := d.Start(); err != nil {
		c.Fatalf("Could not start daemon: %v", err)
	}
	d.Stop()

	k2, err := libtrust.LoadKeyFile("/etc/docker/key.json")
	if err != nil {
		c.Fatalf("Error opening key file")
	}
	if k1.KeyID() != k2.KeyID() {
		c.Fatalf("Key not migrated")
	}

}

// Simulate an older daemon (pre 1.3) coming up with volumes specified in containers
//	without corresponding volume json
func (s *DockerSuite) TestDaemonUpgradeWithVolumes(c *check.C) {
	d := NewDaemon(c)

	graphDir := filepath.Join(os.TempDir(), "docker-test")
	defer os.RemoveAll(graphDir)
	if err := d.StartWithBusybox("-g", graphDir); err != nil {
		c.Fatal(err)
	}
	defer d.Stop()

	tmpDir := filepath.Join(os.TempDir(), "test")
	defer os.RemoveAll(tmpDir)

	if out, err := d.Cmd("create", "-v", tmpDir+":/foo", "--name=test", "busybox"); err != nil {
		c.Fatal(err, out)
	}

	if err := d.Stop(); err != nil {
		c.Fatal(err)
	}

	// Remove this since we're expecting the daemon to re-create it too
	if err := os.RemoveAll(tmpDir); err != nil {
		c.Fatal(err)
	}

	configDir := filepath.Join(graphDir, "volumes")

	if err := os.RemoveAll(configDir); err != nil {
		c.Fatal(err)
	}

	if err := d.Start("-g", graphDir); err != nil {
		c.Fatal(err)
	}

	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		c.Fatalf("expected volume path %s to exist but it does not", tmpDir)
	}

	dir, err := ioutil.ReadDir(configDir)
	if err != nil {
		c.Fatal(err)
	}
	if len(dir) == 0 {
		c.Fatalf("expected volumes config dir to contain data for new volume")
	}

	// Now with just removing the volume config and not the volume data
	if err := d.Stop(); err != nil {
		c.Fatal(err)
	}

	if err := os.RemoveAll(configDir); err != nil {
		c.Fatal(err)
	}

	if err := d.Start("-g", graphDir); err != nil {
		c.Fatal(err)
	}

	dir, err = ioutil.ReadDir(configDir)
	if err != nil {
		c.Fatal(err)
	}

	if len(dir) == 0 {
		c.Fatalf("expected volumes config dir to contain data for new volume")
	}

}

// GH#11320 - verify that the daemon exits on failure properly
// Note that this explicitly tests the conflict of {-b,--bridge} and {--bip} options as the means
// to get a daemon init failure; no other tests for -b/--bip conflict are therefore required
func (s *DockerSuite) TestDaemonExitOnFailure(c *check.C) {
	d := NewDaemon(c)
	defer d.Stop()

	//attempt to start daemon with incorrect flags (we know -b and --bip conflict)
	if err := d.Start("--bridge", "nosuchbridge", "--bip", "1.1.1.1"); err != nil {
		//verify we got the right error
		if !strings.Contains(err.Error(), "Daemon exited and never started") {
			c.Fatalf("Expected daemon not to start, got %v", err)
		}
		// look in the log and make sure we got the message that daemon is shutting down
		runCmd := exec.Command("grep", "Error starting daemon", d.LogfileName())
		if out, _, err := runCommandWithOutput(runCmd); err != nil {
			c.Fatalf("Expected 'Error starting daemon' message; but doesn't exist in log: %q, err: %v", out, err)
		}
	} else {
		//if we didn't get an error and the daemon is running, this is a failure
		d.Stop()
		c.Fatal("Conflicting options should cause the daemon to error out with a failure")
	}

}

func (s *DockerSuite) TestDaemonUlimitDefaults(c *check.C) {
	testRequires(c, NativeExecDriver)
	d := NewDaemon(c)

	if err := d.StartWithBusybox("--default-ulimit", "nofile=42:42", "--default-ulimit", "nproc=1024:1024"); err != nil {
		c.Fatal(err)
	}
	defer d.Stop()

	out, err := d.Cmd("run", "--ulimit", "nproc=2048", "--name=test", "busybox", "/bin/sh", "-c", "echo $(ulimit -n); echo $(ulimit -p)")
	if err != nil {
		c.Fatal(out, err)
	}

	outArr := strings.Split(out, "\n")
	if len(outArr) < 2 {
		c.Fatalf("got unexpected output: %s", out)
	}
	nofile := strings.TrimSpace(outArr[0])
	nproc := strings.TrimSpace(outArr[1])

	if nofile != "42" {
		c.Fatalf("expected `ulimit -n` to be `42`, got: %s", nofile)
	}
	if nproc != "2048" {
		c.Fatalf("exepcted `ulimit -p` to be 2048, got: %s", nproc)
	}

	// Now restart daemon with a new default
	if err := d.Restart("--default-ulimit", "nofile=43"); err != nil {
		c.Fatal(err)
	}

	out, err = d.Cmd("start", "-a", "test")
	if err != nil {
		c.Fatal(err)
	}

	outArr = strings.Split(out, "\n")
	if len(outArr) < 2 {
		c.Fatalf("got unexpected output: %s", out)
	}
	nofile = strings.TrimSpace(outArr[0])
	nproc = strings.TrimSpace(outArr[1])

	if nofile != "43" {
		c.Fatalf("expected `ulimit -n` to be `43`, got: %s", nofile)
	}
	if nproc != "2048" {
		c.Fatalf("exepcted `ulimit -p` to be 2048, got: %s", nproc)
	}

}

// #11315
func (s *DockerSuite) TestDaemonRestartRenameContainer(c *check.C) {
	d := NewDaemon(c)
	if err := d.StartWithBusybox(); err != nil {
		c.Fatal(err)
	}
	defer d.Stop()

	if out, err := d.Cmd("run", "--name=test", "busybox"); err != nil {
		c.Fatal(err, out)
	}

	if out, err := d.Cmd("rename", "test", "test2"); err != nil {
		c.Fatal(err, out)
	}

	if err := d.Restart(); err != nil {
		c.Fatal(err)
	}

	if out, err := d.Cmd("start", "test2"); err != nil {
		c.Fatal(err, out)
	}

}

func (s *DockerSuite) TestDaemonLoggingDriverDefault(c *check.C) {
	d := NewDaemon(c)

	if err := d.StartWithBusybox(); err != nil {
		c.Fatal(err)
	}
	defer d.Stop()

	out, err := d.Cmd("run", "-d", "busybox", "echo", "testline")
	if err != nil {
		c.Fatal(out, err)
	}
	id := strings.TrimSpace(out)

	if out, err := d.Cmd("wait", id); err != nil {
		c.Fatal(out, err)
	}
	logPath := filepath.Join(d.folder, "graph", "containers", id, id+"-json.log")

	if _, err := os.Stat(logPath); err != nil {
		c.Fatal(err)
	}
	f, err := os.Open(logPath)
	if err != nil {
		c.Fatal(err)
	}
	var res struct {
		Log    string    `json:"log"`
		Stream string    `json:"stream"`
		Time   time.Time `json:"time"`
	}
	if err := json.NewDecoder(f).Decode(&res); err != nil {
		c.Fatal(err)
	}
	if res.Log != "testline\n" {
		c.Fatalf("Unexpected log line: %q, expected: %q", res.Log, "testline\n")
	}
	if res.Stream != "stdout" {
		c.Fatalf("Unexpected stream: %q, expected: %q", res.Stream, "stdout")
	}
	if !time.Now().After(res.Time) {
		c.Fatalf("Log time %v in future", res.Time)
	}
}

func (s *DockerSuite) TestDaemonLoggingDriverDefaultOverride(c *check.C) {
	d := NewDaemon(c)

	if err := d.StartWithBusybox(); err != nil {
		c.Fatal(err)
	}
	defer d.Stop()

	out, err := d.Cmd("run", "-d", "--log-driver=none", "busybox", "echo", "testline")
	if err != nil {
		c.Fatal(out, err)
	}
	id := strings.TrimSpace(out)

	if out, err := d.Cmd("wait", id); err != nil {
		c.Fatal(out, err)
	}
	logPath := filepath.Join(d.folder, "graph", "containers", id, id+"-json.log")

	if _, err := os.Stat(logPath); err == nil || !os.IsNotExist(err) {
		c.Fatalf("%s shouldn't exits, error on Stat: %s", logPath, err)
	}
}

func (s *DockerSuite) TestDaemonLoggingDriverNone(c *check.C) {
	d := NewDaemon(c)

	if err := d.StartWithBusybox("--log-driver=none"); err != nil {
		c.Fatal(err)
	}
	defer d.Stop()

	out, err := d.Cmd("run", "-d", "busybox", "echo", "testline")
	if err != nil {
		c.Fatal(out, err)
	}
	id := strings.TrimSpace(out)
	if out, err := d.Cmd("wait", id); err != nil {
		c.Fatal(out, err)
	}

	logPath := filepath.Join(d.folder, "graph", "containers", id, id+"-json.log")

	if _, err := os.Stat(logPath); err == nil || !os.IsNotExist(err) {
		c.Fatalf("%s shouldn't exits, error on Stat: %s", logPath, err)
	}
}

func (s *DockerSuite) TestDaemonLoggingDriverNoneOverride(c *check.C) {
	d := NewDaemon(c)

	if err := d.StartWithBusybox("--log-driver=none"); err != nil {
		c.Fatal(err)
	}
	defer d.Stop()

	out, err := d.Cmd("run", "-d", "--log-driver=json-file", "busybox", "echo", "testline")
	if err != nil {
		c.Fatal(out, err)
	}
	id := strings.TrimSpace(out)

	if out, err := d.Cmd("wait", id); err != nil {
		c.Fatal(out, err)
	}
	logPath := filepath.Join(d.folder, "graph", "containers", id, id+"-json.log")

	if _, err := os.Stat(logPath); err != nil {
		c.Fatal(err)
	}
	f, err := os.Open(logPath)
	if err != nil {
		c.Fatal(err)
	}
	var res struct {
		Log    string    `json:"log"`
		Stream string    `json:"stream"`
		Time   time.Time `json:"time"`
	}
	if err := json.NewDecoder(f).Decode(&res); err != nil {
		c.Fatal(err)
	}
	if res.Log != "testline\n" {
		c.Fatalf("Unexpected log line: %q, expected: %q", res.Log, "testline\n")
	}
	if res.Stream != "stdout" {
		c.Fatalf("Unexpected stream: %q, expected: %q", res.Stream, "stdout")
	}
	if !time.Now().After(res.Time) {
		c.Fatalf("Log time %v in future", res.Time)
	}
}

func (s *DockerSuite) TestDaemonLoggingDriverNoneLogsError(c *check.C) {
	d := NewDaemon(c)

	if err := d.StartWithBusybox("--log-driver=none"); err != nil {
		c.Fatal(err)
	}
	defer d.Stop()

	out, err := d.Cmd("run", "-d", "busybox", "echo", "testline")
	if err != nil {
		c.Fatal(out, err)
	}
	id := strings.TrimSpace(out)
	out, err = d.Cmd("logs", id)
	if err == nil {
		c.Fatalf("Logs should fail with \"none\" driver")
	}
	if !strings.Contains(out, `\"logs\" command is supported only for \"json-file\" logging driver`) {
		c.Fatalf("There should be error about non-json-file driver, got %s", out)
	}
}

func (s *DockerSuite) TestDaemonDots(c *check.C) {
	d := NewDaemon(c)
	if err := d.StartWithBusybox(); err != nil {
		c.Fatal(err)
	}
	defer d.Stop()

	// Now create 4 containers
	if _, err := d.Cmd("create", "busybox"); err != nil {
		c.Fatalf("Error creating container: %q", err)
	}
	if _, err := d.Cmd("create", "busybox"); err != nil {
		c.Fatalf("Error creating container: %q", err)
	}
	if _, err := d.Cmd("create", "busybox"); err != nil {
		c.Fatalf("Error creating container: %q", err)
	}
	if _, err := d.Cmd("create", "busybox"); err != nil {
		c.Fatalf("Error creating container: %q", err)
	}

	d.Stop()

	d.Start("--log-level=debug")
	d.Stop()
	content, _ := ioutil.ReadFile(d.logFile.Name())
	if strings.Contains(string(content), "....") {
		c.Fatalf("Debug level should not have ....\n%s", string(content))
	}

	d.Start("--log-level=error")
	d.Stop()
	content, _ = ioutil.ReadFile(d.logFile.Name())
	if strings.Contains(string(content), "....") {
		c.Fatalf("Error level should not have ....\n%s", string(content))
	}

	d.Start("--log-level=info")
	d.Stop()
	content, _ = ioutil.ReadFile(d.logFile.Name())
	if !strings.Contains(string(content), "....") {
		c.Fatalf("Info level should have ....\n%s", string(content))
	}

}

func (s *DockerSuite) TestDaemonUnixSockCleanedUp(c *check.C) {
	d := NewDaemon(c)
	dir, err := ioutil.TempDir("", "socket-cleanup-test")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(dir)

	sockPath := filepath.Join(dir, "docker.sock")
	if err := d.Start("--host", "unix://"+sockPath); err != nil {
		c.Fatal(err)
	}
	defer d.Stop()

	if _, err := os.Stat(sockPath); err != nil {
		c.Fatal("socket does not exist")
	}

	if err := d.Stop(); err != nil {
		c.Fatal(err)
	}

	if _, err := os.Stat(sockPath); err == nil || !os.IsNotExist(err) {
		c.Fatal("unix socket is not cleaned up")
	}

}

func (s *DockerSuite) TestDaemonwithwrongkey(c *check.C) {
	type Config struct {
		Crv string `json:"crv"`
		D   string `json:"d"`
		Kid string `json:"kid"`
		Kty string `json:"kty"`
		X   string `json:"x"`
		Y   string `json:"y"`
	}

	os.Remove("/etc/docker/key.json")
	d := NewDaemon(c)
	if err := d.Start(); err != nil {
		c.Fatalf("Failed to start daemon: %v", err)
	}

	if err := d.Stop(); err != nil {
		c.Fatalf("Could not stop daemon: %v", err)
	}

	config := &Config{}
	bytes, err := ioutil.ReadFile("/etc/docker/key.json")
	if err != nil {
		c.Fatalf("Error reading key.json file: %s", err)
	}

	// byte[] to Data-Struct
	if err := json.Unmarshal(bytes, &config); err != nil {
		c.Fatalf("Error Unmarshal: %s", err)
	}

	//replace config.Kid with the fake value
	config.Kid = "VSAJ:FUYR:X3H2:B2VZ:KZ6U:CJD5:K7BX:ZXHY:UZXT:P4FT:MJWG:HRJ4"

	// NEW Data-Struct to byte[]
	newBytes, err := json.Marshal(&config)
	if err != nil {
		c.Fatalf("Error Marshal: %s", err)
	}

	// write back
	if err := ioutil.WriteFile("/etc/docker/key.json", newBytes, 0400); err != nil {
		c.Fatalf("Error ioutil.WriteFile: %s", err)
	}

	d1 := NewDaemon(c)
	defer os.Remove("/etc/docker/key.json")

	if err := d1.Start(); err == nil {
		d1.Stop()
		c.Fatalf("It should not be succssful to start daemon with wrong key: %v", err)
	}

	content, _ := ioutil.ReadFile(d1.logFile.Name())

	if !strings.Contains(string(content), "Public Key ID does not match") {
		c.Fatal("Missing KeyID message from daemon logs")
	}

}

func (s *DockerSuite) TestDaemonRestartKillWait(c *check.C) {
	d := NewDaemon(c)
	if err := d.StartWithBusybox(); err != nil {
		c.Fatalf("Could not start daemon with busybox: %v", err)
	}
	defer d.Stop()

	out, err := d.Cmd("run", "-id", "busybox", "/bin/cat")
	if err != nil {
		c.Fatalf("Could not run /bin/cat: err=%v\n%s", err, out)
	}
	containerID := strings.TrimSpace(out)

	if out, err := d.Cmd("kill", containerID); err != nil {
		c.Fatalf("Could not kill %s: err=%v\n%s", containerID, err, out)
	}

	if err := d.Restart(); err != nil {
		c.Fatalf("Could not restart daemon: %v", err)
	}

	errchan := make(chan error)
	go func() {
		if out, err := d.Cmd("wait", containerID); err != nil {
			errchan <- fmt.Errorf("%v:\n%s", err, out)
		}
		close(errchan)
	}()

	select {
	case <-time.After(5 * time.Second):
		c.Fatal("Waiting on a stopped (killed) container timed out")
	case err := <-errchan:
		if err != nil {
			c.Fatal(err)
		}
	}

}
