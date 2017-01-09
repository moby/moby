// +build linux

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	icmd "github.com/docker/docker/pkg/integration/cmd"
	"github.com/docker/docker/pkg/mount"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/go-units"
	"github.com/docker/libnetwork/iptables"
	"github.com/docker/libtrust"
	"github.com/go-check/check"
	"github.com/kr/pty"
)

// TestLegacyDaemonCommand test starting docker daemon using "deprecated" docker daemon
// command. Remove this test when we remove this.
func (s *DockerDaemonSuite) TestLegacyDaemonCommand(c *check.C) {
	cmd := exec.Command(dockerBinary, "daemon", "--storage-driver=vfs", "--debug")
	err := cmd.Start()
	c.Assert(err, checker.IsNil, check.Commentf("could not start daemon using 'docker daemon'"))

	c.Assert(cmd.Process.Kill(), checker.IsNil)
}

func (s *DockerDaemonSuite) TestDaemonRestartWithRunningContainersPorts(c *check.C) {
	if err := s.d.StartWithBusybox(); err != nil {
		c.Fatalf("Could not start daemon with busybox: %v", err)
	}

	if out, err := s.d.Cmd("run", "-d", "--name", "top1", "-p", "1234:80", "--restart", "always", "busybox:latest", "top"); err != nil {
		c.Fatalf("Could not run top1: err=%v\n%s", err, out)
	}
	// --restart=no by default
	if out, err := s.d.Cmd("run", "-d", "--name", "top2", "-p", "80", "busybox:latest", "top"); err != nil {
		c.Fatalf("Could not run top2: err=%v\n%s", err, out)
	}

	testRun := func(m map[string]bool, prefix string) {
		var format string
		for cont, shouldRun := range m {
			out, err := s.d.Cmd("ps")
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

	if err := s.d.Restart(); err != nil {
		c.Fatalf("Could not restart daemon: %v", err)
	}
	testRun(map[string]bool{"top1": true, "top2": false}, "After daemon restart: ")
}

func (s *DockerDaemonSuite) TestDaemonRestartWithVolumesRefs(c *check.C) {
	if err := s.d.StartWithBusybox(); err != nil {
		c.Fatal(err)
	}

	if out, err := s.d.Cmd("run", "--name", "volrestarttest1", "-v", "/foo", "busybox"); err != nil {
		c.Fatal(err, out)
	}

	if err := s.d.Restart(); err != nil {
		c.Fatal(err)
	}

	if _, err := s.d.Cmd("run", "-d", "--volumes-from", "volrestarttest1", "--name", "volrestarttest2", "busybox", "top"); err != nil {
		c.Fatal(err)
	}

	if out, err := s.d.Cmd("rm", "-fv", "volrestarttest2"); err != nil {
		c.Fatal(err, out)
	}

	out, err := s.d.Cmd("inspect", "-f", "{{json .Mounts}}", "volrestarttest1")
	c.Assert(err, check.IsNil)

	if _, err := inspectMountPointJSON(out, "/foo"); err != nil {
		c.Fatalf("Expected volume to exist: /foo, error: %v\n", err)
	}
}

// #11008
func (s *DockerDaemonSuite) TestDaemonRestartUnlessStopped(c *check.C) {
	err := s.d.StartWithBusybox()
	c.Assert(err, check.IsNil)

	out, err := s.d.Cmd("run", "-d", "--name", "top1", "--restart", "always", "busybox:latest", "top")
	c.Assert(err, check.IsNil, check.Commentf("run top1: %v", out))

	out, err = s.d.Cmd("run", "-d", "--name", "top2", "--restart", "unless-stopped", "busybox:latest", "top")
	c.Assert(err, check.IsNil, check.Commentf("run top2: %v", out))

	testRun := func(m map[string]bool, prefix string) {
		var format string
		for name, shouldRun := range m {
			out, err := s.d.Cmd("ps")
			c.Assert(err, check.IsNil, check.Commentf("run ps: %v", out))
			if shouldRun {
				format = "%scontainer %q is not running"
			} else {
				format = "%scontainer %q is running"
			}
			c.Assert(strings.Contains(out, name), check.Equals, shouldRun, check.Commentf(format, prefix, name))
		}
	}

	// both running
	testRun(map[string]bool{"top1": true, "top2": true}, "")

	out, err = s.d.Cmd("stop", "top1")
	c.Assert(err, check.IsNil, check.Commentf(out))

	out, err = s.d.Cmd("stop", "top2")
	c.Assert(err, check.IsNil, check.Commentf(out))

	// both stopped
	testRun(map[string]bool{"top1": false, "top2": false}, "")

	err = s.d.Restart()
	c.Assert(err, check.IsNil)

	// restart=always running
	testRun(map[string]bool{"top1": true, "top2": false}, "After daemon restart: ")

	out, err = s.d.Cmd("start", "top2")
	c.Assert(err, check.IsNil, check.Commentf("start top2: %v", out))

	err = s.d.Restart()
	c.Assert(err, check.IsNil)

	// both running
	testRun(map[string]bool{"top1": true, "top2": true}, "After second daemon restart: ")

}

func (s *DockerDaemonSuite) TestDaemonRestartOnFailure(c *check.C) {
	err := s.d.StartWithBusybox()
	c.Assert(err, check.IsNil)

	out, err := s.d.Cmd("run", "-d", "--name", "test1", "--restart", "on-failure:3", "busybox:latest", "false")
	c.Assert(err, check.IsNil, check.Commentf("run top1: %v", out))

	// wait test1 to stop
	hostArgs := []string{"--host", s.d.sock()}
	err = waitInspectWithArgs("test1", "{{.State.Running}} {{.State.Restarting}}", "false false", 10*time.Second, hostArgs...)
	c.Assert(err, checker.IsNil, check.Commentf("test1 should exit but not"))

	// record last start time
	out, err = s.d.Cmd("inspect", "-f={{.State.StartedAt}}", "test1")
	c.Assert(err, checker.IsNil, check.Commentf("out: %v", out))
	lastStartTime := out

	err = s.d.Restart()
	c.Assert(err, check.IsNil)

	// test1 shouldn't restart at all
	err = waitInspectWithArgs("test1", "{{.State.Running}} {{.State.Restarting}}", "false false", 0, hostArgs...)
	c.Assert(err, checker.IsNil, check.Commentf("test1 should exit but not"))

	// make sure test1 isn't restarted when daemon restart
	// if "StartAt" time updates, means test1 was once restarted.
	out, err = s.d.Cmd("inspect", "-f={{.State.StartedAt}}", "test1")
	c.Assert(err, checker.IsNil, check.Commentf("out: %v", out))
	c.Assert(out, checker.Equals, lastStartTime, check.Commentf("test1 shouldn't start after daemon restarts"))
}

func (s *DockerDaemonSuite) TestDaemonStartIptablesFalse(c *check.C) {
	if err := s.d.Start("--iptables=false"); err != nil {
		c.Fatalf("we should have been able to start the daemon with passing iptables=false: %v", err)
	}
}

// Make sure we cannot shrink base device at daemon restart.
func (s *DockerDaemonSuite) TestDaemonRestartWithInvalidBasesize(c *check.C) {
	testRequires(c, Devicemapper)
	c.Assert(s.d.Start(), check.IsNil)

	oldBasesizeBytes := s.d.getBaseDeviceSize(c)
	var newBasesizeBytes int64 = 1073741824 //1GB in bytes

	if newBasesizeBytes < oldBasesizeBytes {
		err := s.d.Restart("--storage-opt", fmt.Sprintf("dm.basesize=%d", newBasesizeBytes))
		c.Assert(err, check.IsNil, check.Commentf("daemon should not have started as new base device size is less than existing base device size: %v", err))
	}
	c.Assert(s.d.Stop(), check.IsNil)
}

// Make sure we can grow base device at daemon restart.
func (s *DockerDaemonSuite) TestDaemonRestartWithIncreasedBasesize(c *check.C) {
	testRequires(c, Devicemapper)
	c.Assert(s.d.Start(), check.IsNil)

	oldBasesizeBytes := s.d.getBaseDeviceSize(c)

	var newBasesizeBytes int64 = 53687091200 //50GB in bytes

	if newBasesizeBytes < oldBasesizeBytes {
		c.Skip(fmt.Sprintf("New base device size (%v) must be greater than (%s)", units.HumanSize(float64(newBasesizeBytes)), units.HumanSize(float64(oldBasesizeBytes))))
	}

	err := s.d.Restart("--storage-opt", fmt.Sprintf("dm.basesize=%d", newBasesizeBytes))
	c.Assert(err, check.IsNil, check.Commentf("we should have been able to start the daemon with increased base device size: %v", err))

	basesizeAfterRestart := s.d.getBaseDeviceSize(c)
	newBasesize, err := convertBasesize(newBasesizeBytes)
	c.Assert(err, check.IsNil, check.Commentf("Error in converting base device size: %v", err))
	c.Assert(newBasesize, check.Equals, basesizeAfterRestart, check.Commentf("Basesize passed is not equal to Basesize set"))
	c.Assert(s.d.Stop(), check.IsNil)
}

// Issue #8444: If docker0 bridge is modified (intentionally or unintentionally) and
// no longer has an IP associated, we should gracefully handle that case and associate
// an IP with it rather than fail daemon start
func (s *DockerDaemonSuite) TestDaemonStartBridgeWithoutIPAssociation(c *check.C) {
	// rather than depending on brctl commands to verify docker0 is created and up
	// let's start the daemon and stop it, and then make a modification to run the
	// actual test
	if err := s.d.Start(); err != nil {
		c.Fatalf("Could not start daemon: %v", err)
	}
	if err := s.d.Stop(); err != nil {
		c.Fatalf("Could not stop daemon: %v", err)
	}

	// now we will remove the ip from docker0 and then try starting the daemon
	ipCmd := exec.Command("ip", "addr", "flush", "dev", "docker0")
	stdout, stderr, _, err := runCommandWithStdoutStderr(ipCmd)
	if err != nil {
		c.Fatalf("failed to remove docker0 IP association: %v, stdout: %q, stderr: %q", err, stdout, stderr)
	}

	if err := s.d.Start(); err != nil {
		warning := "**WARNING: Docker bridge network in bad state--delete docker0 bridge interface to fix"
		c.Fatalf("Could not start daemon when docker0 has no IP address: %v\n%s", err, warning)
	}
}

func (s *DockerDaemonSuite) TestDaemonIptablesClean(c *check.C) {
	if err := s.d.StartWithBusybox(); err != nil {
		c.Fatalf("Could not start daemon with busybox: %v", err)
	}

	if out, err := s.d.Cmd("run", "-d", "--name", "top", "-p", "80", "busybox:latest", "top"); err != nil {
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

	if err := s.d.Stop(); err != nil {
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

func (s *DockerDaemonSuite) TestDaemonIptablesCreate(c *check.C) {
	if err := s.d.StartWithBusybox(); err != nil {
		c.Fatalf("Could not start daemon with busybox: %v", err)
	}

	if out, err := s.d.Cmd("run", "-d", "--name", "top", "--restart=always", "-p", "80", "busybox:latest", "top"); err != nil {
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

	if err := s.d.Restart(); err != nil {
		c.Fatalf("Could not restart daemon: %v", err)
	}

	// make sure the container is not running
	runningOut, err := s.d.Cmd("inspect", "--format={{.State.Running}}", "top")
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

// TestDaemonIPv6Enabled checks that when the daemon is started with --ipv6=true that the docker0 bridge
// has the fe80::1 address and that a container is assigned a link-local address
func (s *DockerDaemonSuite) TestDaemonIPv6Enabled(c *check.C) {
	testRequires(c, IPv6)

	setupV6(c)
	defer teardownV6(c)

	if err := s.d.StartWithBusybox("--ipv6"); err != nil {
		c.Fatal(err)
	}

	iface, err := net.InterfaceByName("docker0")
	if err != nil {
		c.Fatalf("Error getting docker0 interface: %v", err)
	}

	addrs, err := iface.Addrs()
	if err != nil {
		c.Fatalf("Error getting addresses for docker0 interface: %v", err)
	}

	var found bool
	expected := "fe80::1/64"

	for i := range addrs {
		if addrs[i].String() == expected {
			found = true
			break
		}
	}

	if !found {
		c.Fatalf("Bridge does not have an IPv6 Address")
	}

	if out, err := s.d.Cmd("run", "-itd", "--name=ipv6test", "busybox:latest"); err != nil {
		c.Fatalf("Could not run container: %s, %v", out, err)
	}

	out, err := s.d.Cmd("inspect", "--format", "'{{.NetworkSettings.Networks.bridge.LinkLocalIPv6Address}}'", "ipv6test")
	out = strings.Trim(out, " \r\n'")

	if err != nil {
		c.Fatalf("Error inspecting container: %s, %v", out, err)
	}

	if ip := net.ParseIP(out); ip == nil {
		c.Fatalf("Container should have a link-local IPv6 address")
	}

	out, err = s.d.Cmd("inspect", "--format", "'{{.NetworkSettings.Networks.bridge.GlobalIPv6Address}}'", "ipv6test")
	out = strings.Trim(out, " \r\n'")

	if err != nil {
		c.Fatalf("Error inspecting container: %s, %v", out, err)
	}

	if ip := net.ParseIP(out); ip != nil {
		c.Fatalf("Container should not have a global IPv6 address: %v", out)
	}
}

// TestDaemonIPv6FixedCIDR checks that when the daemon is started with --ipv6=true and a fixed CIDR
// that running containers are given a link-local and global IPv6 address
func (s *DockerDaemonSuite) TestDaemonIPv6FixedCIDR(c *check.C) {
	// IPv6 setup is messing with local bridge address.
	testRequires(c, SameHostDaemon)
	setupV6(c)
	defer teardownV6(c)

	err := s.d.StartWithBusybox("--ipv6", "--fixed-cidr-v6=2001:db8:2::/64", "--default-gateway-v6=2001:db8:2::100")
	c.Assert(err, checker.IsNil, check.Commentf("Could not start daemon with busybox: %v", err))

	out, err := s.d.Cmd("run", "-itd", "--name=ipv6test", "busybox:latest")
	c.Assert(err, checker.IsNil, check.Commentf("Could not run container: %s, %v", out, err))

	out, err = s.d.Cmd("inspect", "--format", "{{.NetworkSettings.Networks.bridge.GlobalIPv6Address}}", "ipv6test")
	out = strings.Trim(out, " \r\n'")

	c.Assert(err, checker.IsNil, check.Commentf(out))

	ip := net.ParseIP(out)
	c.Assert(ip, checker.NotNil, check.Commentf("Container should have a global IPv6 address"))

	out, err = s.d.Cmd("inspect", "--format", "{{.NetworkSettings.Networks.bridge.IPv6Gateway}}", "ipv6test")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	c.Assert(strings.Trim(out, " \r\n'"), checker.Equals, "2001:db8:2::100", check.Commentf("Container should have a global IPv6 gateway"))
}

// TestDaemonIPv6FixedCIDRAndMac checks that when the daemon is started with ipv6 fixed CIDR
// the running containers are given an IPv6 address derived from the MAC address and the ipv6 fixed CIDR
func (s *DockerDaemonSuite) TestDaemonIPv6FixedCIDRAndMac(c *check.C) {
	// IPv6 setup is messing with local bridge address.
	testRequires(c, SameHostDaemon)
	setupV6(c)
	defer teardownV6(c)

	err := s.d.StartWithBusybox("--ipv6", "--fixed-cidr-v6=2001:db8:1::/64")
	c.Assert(err, checker.IsNil)

	out, err := s.d.Cmd("run", "-itd", "--name=ipv6test", "--mac-address", "AA:BB:CC:DD:EE:FF", "busybox")
	c.Assert(err, checker.IsNil)

	out, err = s.d.Cmd("inspect", "--format", "{{.NetworkSettings.Networks.bridge.GlobalIPv6Address}}", "ipv6test")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.Trim(out, " \r\n'"), checker.Equals, "2001:db8:1::aabb:ccdd:eeff")
}

func (s *DockerDaemonSuite) TestDaemonLogLevelWrong(c *check.C) {
	c.Assert(s.d.Start("--log-level=bogus"), check.NotNil, check.Commentf("Daemon shouldn't start with wrong log level"))
}

func (s *DockerDaemonSuite) TestDaemonLogLevelDebug(c *check.C) {
	if err := s.d.Start("--log-level=debug"); err != nil {
		c.Fatal(err)
	}
	content, _ := ioutil.ReadFile(s.d.logFile.Name())
	if !strings.Contains(string(content), `level=debug`) {
		c.Fatalf(`Missing level="debug" in log file:\n%s`, string(content))
	}
}

func (s *DockerDaemonSuite) TestDaemonLogLevelFatal(c *check.C) {
	// we creating new daemons to create new logFile
	if err := s.d.Start("--log-level=fatal"); err != nil {
		c.Fatal(err)
	}
	content, _ := ioutil.ReadFile(s.d.logFile.Name())
	if strings.Contains(string(content), `level=debug`) {
		c.Fatalf(`Should not have level="debug" in log file:\n%s`, string(content))
	}
}

func (s *DockerDaemonSuite) TestDaemonFlagD(c *check.C) {
	if err := s.d.Start("-D"); err != nil {
		c.Fatal(err)
	}
	content, _ := ioutil.ReadFile(s.d.logFile.Name())
	if !strings.Contains(string(content), `level=debug`) {
		c.Fatalf(`Should have level="debug" in log file using -D:\n%s`, string(content))
	}
}

func (s *DockerDaemonSuite) TestDaemonFlagDebug(c *check.C) {
	if err := s.d.Start("--debug"); err != nil {
		c.Fatal(err)
	}
	content, _ := ioutil.ReadFile(s.d.logFile.Name())
	if !strings.Contains(string(content), `level=debug`) {
		c.Fatalf(`Should have level="debug" in log file using --debug:\n%s`, string(content))
	}
}

func (s *DockerDaemonSuite) TestDaemonFlagDebugLogLevelFatal(c *check.C) {
	if err := s.d.Start("--debug", "--log-level=fatal"); err != nil {
		c.Fatal(err)
	}
	content, _ := ioutil.ReadFile(s.d.logFile.Name())
	if !strings.Contains(string(content), `level=debug`) {
		c.Fatalf(`Should have level="debug" in log file when using both --debug and --log-level=fatal:\n%s`, string(content))
	}
}

func (s *DockerDaemonSuite) TestDaemonAllocatesListeningPort(c *check.C) {
	listeningPorts := [][]string{
		{"0.0.0.0", "0.0.0.0", "5678"},
		{"127.0.0.1", "127.0.0.1", "1234"},
		{"localhost", "127.0.0.1", "1235"},
	}

	cmdArgs := make([]string, 0, len(listeningPorts)*2)
	for _, hostDirective := range listeningPorts {
		cmdArgs = append(cmdArgs, "--host", fmt.Sprintf("tcp://%s:%s", hostDirective[0], hostDirective[2]))
	}

	if err := s.d.StartWithBusybox(cmdArgs...); err != nil {
		c.Fatalf("Could not start daemon with busybox: %v", err)
	}

	for _, hostDirective := range listeningPorts {
		output, err := s.d.Cmd("run", "-p", fmt.Sprintf("%s:%s:80", hostDirective[1], hostDirective[2]), "busybox", "true")
		if err == nil {
			c.Fatalf("Container should not start, expected port already allocated error: %q", output)
		} else if !strings.Contains(output, "port is already allocated") {
			c.Fatalf("Expected port is already allocated error: %q", output)
		}
	}
}

func (s *DockerDaemonSuite) TestDaemonKeyGeneration(c *check.C) {
	// TODO: skip or update for Windows daemon
	os.Remove("/etc/docker/key.json")
	if err := s.d.Start(); err != nil {
		c.Fatalf("Could not start daemon: %v", err)
	}
	s.d.Stop()

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

func (s *DockerDaemonSuite) TestDaemonKeyMigration(c *check.C) {
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

	if err := s.d.Start(); err != nil {
		c.Fatalf("Could not start daemon: %v", err)
	}
	s.d.Stop()

	k2, err := libtrust.LoadKeyFile("/etc/docker/key.json")
	if err != nil {
		c.Fatalf("Error opening key file")
	}
	if k1.KeyID() != k2.KeyID() {
		c.Fatalf("Key not migrated")
	}
}

// GH#11320 - verify that the daemon exits on failure properly
// Note that this explicitly tests the conflict of {-b,--bridge} and {--bip} options as the means
// to get a daemon init failure; no other tests for -b/--bip conflict are therefore required
func (s *DockerDaemonSuite) TestDaemonExitOnFailure(c *check.C) {
	//attempt to start daemon with incorrect flags (we know -b and --bip conflict)
	if err := s.d.Start("--bridge", "nosuchbridge", "--bip", "1.1.1.1"); err != nil {
		//verify we got the right error
		if !strings.Contains(err.Error(), "Daemon exited") {
			c.Fatalf("Expected daemon not to start, got %v", err)
		}
		// look in the log and make sure we got the message that daemon is shutting down
		runCmd := exec.Command("grep", "Error starting daemon", s.d.LogFileName())
		if out, _, err := runCommandWithOutput(runCmd); err != nil {
			c.Fatalf("Expected 'Error starting daemon' message; but doesn't exist in log: %q, err: %v", out, err)
		}
	} else {
		//if we didn't get an error and the daemon is running, this is a failure
		c.Fatal("Conflicting options should cause the daemon to error out with a failure")
	}
}

func (s *DockerDaemonSuite) TestDaemonBridgeExternal(c *check.C) {
	d := s.d
	err := d.Start("--bridge", "nosuchbridge")
	c.Assert(err, check.NotNil, check.Commentf("--bridge option with an invalid bridge should cause the daemon to fail"))
	defer d.Restart()

	bridgeName := "external-bridge"
	bridgeIP := "192.169.1.1/24"
	_, bridgeIPNet, _ := net.ParseCIDR(bridgeIP)

	out, err := createInterface(c, "bridge", bridgeName, bridgeIP)
	c.Assert(err, check.IsNil, check.Commentf(out))
	defer deleteInterface(c, bridgeName)

	err = d.StartWithBusybox("--bridge", bridgeName)
	c.Assert(err, check.IsNil)

	ipTablesSearchString := bridgeIPNet.String()
	ipTablesCmd := exec.Command("iptables", "-t", "nat", "-nvL")
	out, _, err = runCommandWithOutput(ipTablesCmd)
	c.Assert(err, check.IsNil)

	c.Assert(strings.Contains(out, ipTablesSearchString), check.Equals, true,
		check.Commentf("iptables output should have contained %q, but was %q",
			ipTablesSearchString, out))

	_, err = d.Cmd("run", "-d", "--name", "ExtContainer", "busybox", "top")
	c.Assert(err, check.IsNil)

	containerIP := d.findContainerIP("ExtContainer")
	ip := net.ParseIP(containerIP)
	c.Assert(bridgeIPNet.Contains(ip), check.Equals, true,
		check.Commentf("Container IP-Address must be in the same subnet range : %s",
			containerIP))
}

func (s *DockerDaemonSuite) TestDaemonBridgeNone(c *check.C) {
	// start with bridge none
	d := s.d
	err := d.StartWithBusybox("--bridge", "none")
	c.Assert(err, check.IsNil)
	defer d.Restart()

	// verify docker0 iface is not there
	out, _, err := runCommandWithOutput(exec.Command("ifconfig", "docker0"))
	c.Assert(err, check.NotNil, check.Commentf("docker0 should not be present if daemon started with --bridge=none"))
	c.Assert(strings.Contains(out, "Device not found"), check.Equals, true)

	// verify default "bridge" network is not there
	out, err = d.Cmd("network", "inspect", "bridge")
	c.Assert(err, check.NotNil, check.Commentf("\"bridge\" network should not be present if daemon started with --bridge=none"))
	c.Assert(strings.Contains(out, "No such network"), check.Equals, true)
}

func createInterface(c *check.C, ifType string, ifName string, ipNet string) (string, error) {
	args := []string{"link", "add", "name", ifName, "type", ifType}
	ipLinkCmd := exec.Command("ip", args...)
	out, _, err := runCommandWithOutput(ipLinkCmd)
	if err != nil {
		return out, err
	}

	ifCfgCmd := exec.Command("ifconfig", ifName, ipNet, "up")
	out, _, err = runCommandWithOutput(ifCfgCmd)
	return out, err
}

func deleteInterface(c *check.C, ifName string) {
	ifCmd := exec.Command("ip", "link", "delete", ifName)
	out, _, err := runCommandWithOutput(ifCmd)
	c.Assert(err, check.IsNil, check.Commentf(out))

	flushCmd := exec.Command("iptables", "-t", "nat", "--flush")
	out, _, err = runCommandWithOutput(flushCmd)
	c.Assert(err, check.IsNil, check.Commentf(out))

	flushCmd = exec.Command("iptables", "--flush")
	out, _, err = runCommandWithOutput(flushCmd)
	c.Assert(err, check.IsNil, check.Commentf(out))
}

func (s *DockerDaemonSuite) TestDaemonBridgeIP(c *check.C) {
	// TestDaemonBridgeIP Steps
	// 1. Delete the existing docker0 Bridge
	// 2. Set --bip daemon configuration and start the new Docker Daemon
	// 3. Check if the bip config has taken effect using ifconfig and iptables commands
	// 4. Launch a Container and make sure the IP-Address is in the expected subnet
	// 5. Delete the docker0 Bridge
	// 6. Restart the Docker Daemon (via deferred action)
	//    This Restart takes care of bringing docker0 interface back to auto-assigned IP

	defaultNetworkBridge := "docker0"
	deleteInterface(c, defaultNetworkBridge)

	d := s.d

	bridgeIP := "192.169.1.1/24"
	ip, bridgeIPNet, _ := net.ParseCIDR(bridgeIP)

	err := d.StartWithBusybox("--bip", bridgeIP)
	c.Assert(err, check.IsNil)
	defer d.Restart()

	ifconfigSearchString := ip.String()
	ifconfigCmd := exec.Command("ifconfig", defaultNetworkBridge)
	out, _, _, err := runCommandWithStdoutStderr(ifconfigCmd)
	c.Assert(err, check.IsNil)

	c.Assert(strings.Contains(out, ifconfigSearchString), check.Equals, true,
		check.Commentf("ifconfig output should have contained %q, but was %q",
			ifconfigSearchString, out))

	ipTablesSearchString := bridgeIPNet.String()
	ipTablesCmd := exec.Command("iptables", "-t", "nat", "-nvL")
	out, _, err = runCommandWithOutput(ipTablesCmd)
	c.Assert(err, check.IsNil)

	c.Assert(strings.Contains(out, ipTablesSearchString), check.Equals, true,
		check.Commentf("iptables output should have contained %q, but was %q",
			ipTablesSearchString, out))

	out, err = d.Cmd("run", "-d", "--name", "test", "busybox", "top")
	c.Assert(err, check.IsNil)

	containerIP := d.findContainerIP("test")
	ip = net.ParseIP(containerIP)
	c.Assert(bridgeIPNet.Contains(ip), check.Equals, true,
		check.Commentf("Container IP-Address must be in the same subnet range : %s",
			containerIP))
	deleteInterface(c, defaultNetworkBridge)
}

func (s *DockerDaemonSuite) TestDaemonRestartWithBridgeIPChange(c *check.C) {
	if err := s.d.Start(); err != nil {
		c.Fatalf("Could not start daemon: %v", err)
	}
	defer s.d.Restart()
	if err := s.d.Stop(); err != nil {
		c.Fatalf("Could not stop daemon: %v", err)
	}

	// now we will change the docker0's IP and then try starting the daemon
	bridgeIP := "192.169.100.1/24"
	_, bridgeIPNet, _ := net.ParseCIDR(bridgeIP)

	ipCmd := exec.Command("ifconfig", "docker0", bridgeIP)
	stdout, stderr, _, err := runCommandWithStdoutStderr(ipCmd)
	if err != nil {
		c.Fatalf("failed to change docker0's IP association: %v, stdout: %q, stderr: %q", err, stdout, stderr)
	}

	if err := s.d.Start("--bip", bridgeIP); err != nil {
		c.Fatalf("Could not start daemon: %v", err)
	}

	//check if the iptables contains new bridgeIP MASQUERADE rule
	ipTablesSearchString := bridgeIPNet.String()
	ipTablesCmd := exec.Command("iptables", "-t", "nat", "-nvL")
	out, _, err := runCommandWithOutput(ipTablesCmd)
	if err != nil {
		c.Fatalf("Could not run iptables -nvL: %s, %v", out, err)
	}
	if !strings.Contains(out, ipTablesSearchString) {
		c.Fatalf("iptables output should have contained new MASQUERADE rule with IP %q, but was %q", ipTablesSearchString, out)
	}
}

func (s *DockerDaemonSuite) TestDaemonBridgeFixedCidr(c *check.C) {
	d := s.d

	bridgeName := "external-bridge"
	bridgeIP := "192.169.1.1/24"

	out, err := createInterface(c, "bridge", bridgeName, bridgeIP)
	c.Assert(err, check.IsNil, check.Commentf(out))
	defer deleteInterface(c, bridgeName)

	args := []string{"--bridge", bridgeName, "--fixed-cidr", "192.169.1.0/30"}
	err = d.StartWithBusybox(args...)
	c.Assert(err, check.IsNil)
	defer d.Restart()

	for i := 0; i < 4; i++ {
		cName := "Container" + strconv.Itoa(i)
		out, err := d.Cmd("run", "-d", "--name", cName, "busybox", "top")
		if err != nil {
			c.Assert(strings.Contains(out, "no available IPv4 addresses"), check.Equals, true,
				check.Commentf("Could not run a Container : %s %s", err.Error(), out))
		}
	}
}

func (s *DockerDaemonSuite) TestDaemonBridgeFixedCidr2(c *check.C) {
	d := s.d

	bridgeName := "external-bridge"
	bridgeIP := "10.2.2.1/16"

	out, err := createInterface(c, "bridge", bridgeName, bridgeIP)
	c.Assert(err, check.IsNil, check.Commentf(out))
	defer deleteInterface(c, bridgeName)

	err = d.StartWithBusybox("--bip", bridgeIP, "--fixed-cidr", "10.2.2.0/24")
	c.Assert(err, check.IsNil)
	defer s.d.Restart()

	out, err = d.Cmd("run", "-d", "--name", "bb", "busybox", "top")
	c.Assert(err, checker.IsNil, check.Commentf(out))
	defer d.Cmd("stop", "bb")

	out, err = d.Cmd("exec", "bb", "/bin/sh", "-c", "ifconfig eth0 | awk '/inet addr/{print substr($2,6)}'")
	c.Assert(out, checker.Equals, "10.2.2.0\n")

	out, err = d.Cmd("run", "--rm", "busybox", "/bin/sh", "-c", "ifconfig eth0 | awk '/inet addr/{print substr($2,6)}'")
	c.Assert(err, checker.IsNil, check.Commentf(out))
	c.Assert(out, checker.Equals, "10.2.2.2\n")
}

func (s *DockerDaemonSuite) TestDaemonBridgeFixedCIDREqualBridgeNetwork(c *check.C) {
	d := s.d

	bridgeName := "external-bridge"
	bridgeIP := "172.27.42.1/16"

	out, err := createInterface(c, "bridge", bridgeName, bridgeIP)
	c.Assert(err, check.IsNil, check.Commentf(out))
	defer deleteInterface(c, bridgeName)

	err = d.StartWithBusybox("--bridge", bridgeName, "--fixed-cidr", bridgeIP)
	c.Assert(err, check.IsNil)
	defer s.d.Restart()

	out, err = d.Cmd("run", "-d", "busybox", "top")
	c.Assert(err, check.IsNil, check.Commentf(out))
	cid1 := strings.TrimSpace(out)
	defer d.Cmd("stop", cid1)
}

func (s *DockerDaemonSuite) TestDaemonDefaultGatewayIPv4Implicit(c *check.C) {
	defaultNetworkBridge := "docker0"
	deleteInterface(c, defaultNetworkBridge)

	d := s.d

	bridgeIP := "192.169.1.1"
	bridgeIPNet := fmt.Sprintf("%s/24", bridgeIP)

	err := d.StartWithBusybox("--bip", bridgeIPNet)
	c.Assert(err, check.IsNil)
	defer d.Restart()

	expectedMessage := fmt.Sprintf("default via %s dev", bridgeIP)
	out, err := d.Cmd("run", "busybox", "ip", "-4", "route", "list", "0/0")
	c.Assert(strings.Contains(out, expectedMessage), check.Equals, true,
		check.Commentf("Implicit default gateway should be bridge IP %s, but default route was '%s'",
			bridgeIP, strings.TrimSpace(out)))
	deleteInterface(c, defaultNetworkBridge)
}

func (s *DockerDaemonSuite) TestDaemonDefaultGatewayIPv4Explicit(c *check.C) {
	defaultNetworkBridge := "docker0"
	deleteInterface(c, defaultNetworkBridge)

	d := s.d

	bridgeIP := "192.169.1.1"
	bridgeIPNet := fmt.Sprintf("%s/24", bridgeIP)
	gatewayIP := "192.169.1.254"

	err := d.StartWithBusybox("--bip", bridgeIPNet, "--default-gateway", gatewayIP)
	c.Assert(err, check.IsNil)
	defer d.Restart()

	expectedMessage := fmt.Sprintf("default via %s dev", gatewayIP)
	out, err := d.Cmd("run", "busybox", "ip", "-4", "route", "list", "0/0")
	c.Assert(strings.Contains(out, expectedMessage), check.Equals, true,
		check.Commentf("Explicit default gateway should be %s, but default route was '%s'",
			gatewayIP, strings.TrimSpace(out)))
	deleteInterface(c, defaultNetworkBridge)
}

func (s *DockerDaemonSuite) TestDaemonDefaultGatewayIPv4ExplicitOutsideContainerSubnet(c *check.C) {
	defaultNetworkBridge := "docker0"
	deleteInterface(c, defaultNetworkBridge)

	// Program a custom default gateway outside of the container subnet, daemon should accept it and start
	err := s.d.StartWithBusybox("--bip", "172.16.0.10/16", "--fixed-cidr", "172.16.1.0/24", "--default-gateway", "172.16.0.254")
	c.Assert(err, check.IsNil)

	deleteInterface(c, defaultNetworkBridge)
	s.d.Restart()
}

func (s *DockerDaemonSuite) TestDaemonDefaultNetworkInvalidClusterConfig(c *check.C) {
	testRequires(c, DaemonIsLinux, SameHostDaemon)

	// Start daemon without docker0 bridge
	defaultNetworkBridge := "docker0"
	deleteInterface(c, defaultNetworkBridge)

	discoveryBackend := "consul://consuladdr:consulport/some/path"
	err := s.d.Start(fmt.Sprintf("--cluster-store=%s", discoveryBackend))
	c.Assert(err, checker.IsNil)

	// Start daemon with docker0 bridge
	result := icmd.RunCommand("ifconfig", defaultNetworkBridge)
	c.Assert(result, icmd.Matches, icmd.Success)

	err = s.d.Restart(fmt.Sprintf("--cluster-store=%s", discoveryBackend))
	c.Assert(err, checker.IsNil)
}

func (s *DockerDaemonSuite) TestDaemonIP(c *check.C) {
	d := s.d

	ipStr := "192.170.1.1/24"
	ip, _, _ := net.ParseCIDR(ipStr)
	args := []string{"--ip", ip.String()}
	err := d.StartWithBusybox(args...)
	c.Assert(err, check.IsNil)
	defer d.Restart()

	out, err := d.Cmd("run", "-d", "-p", "8000:8000", "busybox", "top")
	c.Assert(err, check.NotNil,
		check.Commentf("Running a container must fail with an invalid --ip option"))
	c.Assert(strings.Contains(out, "Error starting userland proxy"), check.Equals, true)

	ifName := "dummy"
	out, err = createInterface(c, "dummy", ifName, ipStr)
	c.Assert(err, check.IsNil, check.Commentf(out))
	defer deleteInterface(c, ifName)

	_, err = d.Cmd("run", "-d", "-p", "8000:8000", "busybox", "top")
	c.Assert(err, check.IsNil)

	ipTablesCmd := exec.Command("iptables", "-t", "nat", "-nvL")
	out, _, err = runCommandWithOutput(ipTablesCmd)
	c.Assert(err, check.IsNil)

	regex := fmt.Sprintf("DNAT.*%s.*dpt:8000", ip.String())
	matched, _ := regexp.MatchString(regex, out)
	c.Assert(matched, check.Equals, true,
		check.Commentf("iptables output should have contained %q, but was %q", regex, out))
}

func (s *DockerDaemonSuite) TestDaemonICCPing(c *check.C) {
	testRequires(c, bridgeNfIptables)
	d := s.d

	bridgeName := "external-bridge"
	bridgeIP := "192.169.1.1/24"

	out, err := createInterface(c, "bridge", bridgeName, bridgeIP)
	c.Assert(err, check.IsNil, check.Commentf(out))
	defer deleteInterface(c, bridgeName)

	args := []string{"--bridge", bridgeName, "--icc=false"}
	err = d.StartWithBusybox(args...)
	c.Assert(err, check.IsNil)
	defer d.Restart()

	ipTablesCmd := exec.Command("iptables", "-nvL", "FORWARD")
	out, _, err = runCommandWithOutput(ipTablesCmd)
	c.Assert(err, check.IsNil)

	regex := fmt.Sprintf("DROP.*all.*%s.*%s", bridgeName, bridgeName)
	matched, _ := regexp.MatchString(regex, out)
	c.Assert(matched, check.Equals, true,
		check.Commentf("iptables output should have contained %q, but was %q", regex, out))

	// Pinging another container must fail with --icc=false
	pingContainers(c, d, true)

	ipStr := "192.171.1.1/24"
	ip, _, _ := net.ParseCIDR(ipStr)
	ifName := "icc-dummy"

	createInterface(c, "dummy", ifName, ipStr)

	// But, Pinging external or a Host interface must succeed
	pingCmd := fmt.Sprintf("ping -c 1 %s -W 1", ip.String())
	runArgs := []string{"run", "--rm", "busybox", "sh", "-c", pingCmd}
	_, err = d.Cmd(runArgs...)
	c.Assert(err, check.IsNil)
}

func (s *DockerDaemonSuite) TestDaemonICCLinkExpose(c *check.C) {
	d := s.d

	bridgeName := "external-bridge"
	bridgeIP := "192.169.1.1/24"

	out, err := createInterface(c, "bridge", bridgeName, bridgeIP)
	c.Assert(err, check.IsNil, check.Commentf(out))
	defer deleteInterface(c, bridgeName)

	args := []string{"--bridge", bridgeName, "--icc=false"}
	err = d.StartWithBusybox(args...)
	c.Assert(err, check.IsNil)
	defer d.Restart()

	ipTablesCmd := exec.Command("iptables", "-nvL", "FORWARD")
	out, _, err = runCommandWithOutput(ipTablesCmd)
	c.Assert(err, check.IsNil)

	regex := fmt.Sprintf("DROP.*all.*%s.*%s", bridgeName, bridgeName)
	matched, _ := regexp.MatchString(regex, out)
	c.Assert(matched, check.Equals, true,
		check.Commentf("iptables output should have contained %q, but was %q", regex, out))

	out, err = d.Cmd("run", "-d", "--expose", "4567", "--name", "icc1", "busybox", "nc", "-l", "-p", "4567")
	c.Assert(err, check.IsNil, check.Commentf(out))

	out, err = d.Cmd("run", "--link", "icc1:icc1", "busybox", "nc", "icc1", "4567")
	c.Assert(err, check.IsNil, check.Commentf(out))
}

func (s *DockerDaemonSuite) TestDaemonLinksIpTablesRulesWhenLinkAndUnlink(c *check.C) {
	bridgeName := "external-bridge"
	bridgeIP := "192.169.1.1/24"

	out, err := createInterface(c, "bridge", bridgeName, bridgeIP)
	c.Assert(err, check.IsNil, check.Commentf(out))
	defer deleteInterface(c, bridgeName)

	err = s.d.StartWithBusybox("--bridge", bridgeName, "--icc=false")
	c.Assert(err, check.IsNil)
	defer s.d.Restart()

	_, err = s.d.Cmd("run", "-d", "--name", "child", "--publish", "8080:80", "busybox", "top")
	c.Assert(err, check.IsNil)
	_, err = s.d.Cmd("run", "-d", "--name", "parent", "--link", "child:http", "busybox", "top")
	c.Assert(err, check.IsNil)

	childIP := s.d.findContainerIP("child")
	parentIP := s.d.findContainerIP("parent")

	sourceRule := []string{"-i", bridgeName, "-o", bridgeName, "-p", "tcp", "-s", childIP, "--sport", "80", "-d", parentIP, "-j", "ACCEPT"}
	destinationRule := []string{"-i", bridgeName, "-o", bridgeName, "-p", "tcp", "-s", parentIP, "--dport", "80", "-d", childIP, "-j", "ACCEPT"}
	if !iptables.Exists("filter", "DOCKER", sourceRule...) || !iptables.Exists("filter", "DOCKER", destinationRule...) {
		c.Fatal("Iptables rules not found")
	}

	s.d.Cmd("rm", "--link", "parent/http")
	if iptables.Exists("filter", "DOCKER", sourceRule...) || iptables.Exists("filter", "DOCKER", destinationRule...) {
		c.Fatal("Iptables rules should be removed when unlink")
	}

	s.d.Cmd("kill", "child")
	s.d.Cmd("kill", "parent")
}

func (s *DockerDaemonSuite) TestDaemonUlimitDefaults(c *check.C) {
	testRequires(c, DaemonIsLinux)

	if err := s.d.StartWithBusybox("--default-ulimit", "nofile=42:42", "--default-ulimit", "nproc=1024:1024"); err != nil {
		c.Fatal(err)
	}

	out, err := s.d.Cmd("run", "--ulimit", "nproc=2048", "--name=test", "busybox", "/bin/sh", "-c", "echo $(ulimit -n); echo $(ulimit -p)")
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
	if err := s.d.Restart("--default-ulimit", "nofile=43"); err != nil {
		c.Fatal(err)
	}

	out, err = s.d.Cmd("start", "-a", "test")
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
func (s *DockerDaemonSuite) TestDaemonRestartRenameContainer(c *check.C) {
	if err := s.d.StartWithBusybox(); err != nil {
		c.Fatal(err)
	}

	if out, err := s.d.Cmd("run", "--name=test", "busybox"); err != nil {
		c.Fatal(err, out)
	}

	if out, err := s.d.Cmd("rename", "test", "test2"); err != nil {
		c.Fatal(err, out)
	}

	if err := s.d.Restart(); err != nil {
		c.Fatal(err)
	}

	if out, err := s.d.Cmd("start", "test2"); err != nil {
		c.Fatal(err, out)
	}
}

func (s *DockerDaemonSuite) TestDaemonLoggingDriverDefault(c *check.C) {
	if err := s.d.StartWithBusybox(); err != nil {
		c.Fatal(err)
	}

	out, err := s.d.Cmd("run", "--name=test", "busybox", "echo", "testline")
	c.Assert(err, check.IsNil, check.Commentf(out))
	id, err := s.d.getIDByName("test")
	c.Assert(err, check.IsNil)

	logPath := filepath.Join(s.d.root, "containers", id, id+"-json.log")

	if _, err := os.Stat(logPath); err != nil {
		c.Fatal(err)
	}
	f, err := os.Open(logPath)
	if err != nil {
		c.Fatal(err)
	}
	defer f.Close()

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

func (s *DockerDaemonSuite) TestDaemonLoggingDriverDefaultOverride(c *check.C) {
	if err := s.d.StartWithBusybox(); err != nil {
		c.Fatal(err)
	}

	out, err := s.d.Cmd("run", "--name=test", "--log-driver=none", "busybox", "echo", "testline")
	if err != nil {
		c.Fatal(out, err)
	}
	id, err := s.d.getIDByName("test")
	c.Assert(err, check.IsNil)

	logPath := filepath.Join(s.d.root, "containers", id, id+"-json.log")

	if _, err := os.Stat(logPath); err == nil || !os.IsNotExist(err) {
		c.Fatalf("%s shouldn't exits, error on Stat: %s", logPath, err)
	}
}

func (s *DockerDaemonSuite) TestDaemonLoggingDriverNone(c *check.C) {
	if err := s.d.StartWithBusybox("--log-driver=none"); err != nil {
		c.Fatal(err)
	}

	out, err := s.d.Cmd("run", "--name=test", "busybox", "echo", "testline")
	if err != nil {
		c.Fatal(out, err)
	}
	id, err := s.d.getIDByName("test")
	c.Assert(err, check.IsNil)

	logPath := filepath.Join(s.d.folder, "graph", "containers", id, id+"-json.log")

	if _, err := os.Stat(logPath); err == nil || !os.IsNotExist(err) {
		c.Fatalf("%s shouldn't exits, error on Stat: %s", logPath, err)
	}
}

func (s *DockerDaemonSuite) TestDaemonLoggingDriverNoneOverride(c *check.C) {
	if err := s.d.StartWithBusybox("--log-driver=none"); err != nil {
		c.Fatal(err)
	}

	out, err := s.d.Cmd("run", "--name=test", "--log-driver=json-file", "busybox", "echo", "testline")
	if err != nil {
		c.Fatal(out, err)
	}
	id, err := s.d.getIDByName("test")
	c.Assert(err, check.IsNil)

	logPath := filepath.Join(s.d.root, "containers", id, id+"-json.log")

	if _, err := os.Stat(logPath); err != nil {
		c.Fatal(err)
	}
	f, err := os.Open(logPath)
	if err != nil {
		c.Fatal(err)
	}
	defer f.Close()

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

func (s *DockerDaemonSuite) TestDaemonLoggingDriverNoneLogsError(c *check.C) {
	c.Assert(s.d.StartWithBusybox("--log-driver=none"), checker.IsNil)

	out, err := s.d.Cmd("run", "--name=test", "busybox", "echo", "testline")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	out, err = s.d.Cmd("logs", "test")
	c.Assert(err, check.NotNil, check.Commentf("Logs should fail with 'none' driver"))
	expected := `"logs" command is supported only for "json-file" and "journald" logging drivers (got: none)`
	c.Assert(out, checker.Contains, expected)
}

func (s *DockerDaemonSuite) TestDaemonUnixSockCleanedUp(c *check.C) {
	dir, err := ioutil.TempDir("", "socket-cleanup-test")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(dir)

	sockPath := filepath.Join(dir, "docker.sock")
	if err := s.d.Start("--host", "unix://"+sockPath); err != nil {
		c.Fatal(err)
	}

	if _, err := os.Stat(sockPath); err != nil {
		c.Fatal("socket does not exist")
	}

	if err := s.d.Stop(); err != nil {
		c.Fatal(err)
	}

	if _, err := os.Stat(sockPath); err == nil || !os.IsNotExist(err) {
		c.Fatal("unix socket is not cleaned up")
	}
}

func (s *DockerDaemonSuite) TestDaemonWithWrongkey(c *check.C) {
	type Config struct {
		Crv string `json:"crv"`
		D   string `json:"d"`
		Kid string `json:"kid"`
		Kty string `json:"kty"`
		X   string `json:"x"`
		Y   string `json:"y"`
	}

	os.Remove("/etc/docker/key.json")
	if err := s.d.Start(); err != nil {
		c.Fatalf("Failed to start daemon: %v", err)
	}

	if err := s.d.Stop(); err != nil {
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

	defer os.Remove("/etc/docker/key.json")

	if err := s.d.Start(); err == nil {
		c.Fatalf("It should not be successful to start daemon with wrong key: %v", err)
	}

	content, _ := ioutil.ReadFile(s.d.logFile.Name())

	if !strings.Contains(string(content), "Public Key ID does not match") {
		c.Fatalf("Missing KeyID message from daemon logs: %s", string(content))
	}
}

func (s *DockerDaemonSuite) TestDaemonRestartKillWait(c *check.C) {
	if err := s.d.StartWithBusybox(); err != nil {
		c.Fatalf("Could not start daemon with busybox: %v", err)
	}

	out, err := s.d.Cmd("run", "-id", "busybox", "/bin/cat")
	if err != nil {
		c.Fatalf("Could not run /bin/cat: err=%v\n%s", err, out)
	}
	containerID := strings.TrimSpace(out)

	if out, err := s.d.Cmd("kill", containerID); err != nil {
		c.Fatalf("Could not kill %s: err=%v\n%s", containerID, err, out)
	}

	if err := s.d.Restart(); err != nil {
		c.Fatalf("Could not restart daemon: %v", err)
	}

	errchan := make(chan error)
	go func() {
		if out, err := s.d.Cmd("wait", containerID); err != nil {
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

// TestHTTPSInfo connects via two-way authenticated HTTPS to the info endpoint
func (s *DockerDaemonSuite) TestHTTPSInfo(c *check.C) {
	const (
		testDaemonHTTPSAddr = "tcp://localhost:4271"
	)

	if err := s.d.Start("--tlsverify", "--tlscacert", "fixtures/https/ca.pem", "--tlscert", "fixtures/https/server-cert.pem",
		"--tlskey", "fixtures/https/server-key.pem", "-H", testDaemonHTTPSAddr); err != nil {
		c.Fatalf("Could not start daemon with busybox: %v", err)
	}

	args := []string{
		"--host", testDaemonHTTPSAddr,
		"--tlsverify", "--tlscacert", "fixtures/https/ca.pem",
		"--tlscert", "fixtures/https/client-cert.pem",
		"--tlskey", "fixtures/https/client-key.pem",
		"info",
	}
	out, err := s.d.Cmd(args...)
	if err != nil {
		c.Fatalf("Error Occurred: %s and output: %s", err, out)
	}
}

// TestHTTPSRun connects via two-way authenticated HTTPS to the create, attach, start, and wait endpoints.
// https://github.com/docker/docker/issues/19280
func (s *DockerDaemonSuite) TestHTTPSRun(c *check.C) {
	const (
		testDaemonHTTPSAddr = "tcp://localhost:4271"
	)

	if err := s.d.StartWithBusybox("--tlsverify", "--tlscacert", "fixtures/https/ca.pem", "--tlscert", "fixtures/https/server-cert.pem",
		"--tlskey", "fixtures/https/server-key.pem", "-H", testDaemonHTTPSAddr); err != nil {
		c.Fatalf("Could not start daemon with busybox: %v", err)
	}

	args := []string{
		"--host", testDaemonHTTPSAddr,
		"--tlsverify", "--tlscacert", "fixtures/https/ca.pem",
		"--tlscert", "fixtures/https/client-cert.pem",
		"--tlskey", "fixtures/https/client-key.pem",
		"run", "busybox", "echo", "TLS response",
	}
	out, err := s.d.Cmd(args...)
	if err != nil {
		c.Fatalf("Error Occurred: %s and output: %s", err, out)
	}

	if !strings.Contains(out, "TLS response") {
		c.Fatalf("expected output to include `TLS response`, got %v", out)
	}
}

// TestTLSVerify verifies that --tlsverify=false turns on tls
func (s *DockerDaemonSuite) TestTLSVerify(c *check.C) {
	out, err := exec.Command(dockerdBinary, "--tlsverify=false").CombinedOutput()
	if err == nil || !strings.Contains(string(out), "Could not load X509 key pair") {
		c.Fatalf("Daemon should not have started due to missing certs: %v\n%s", err, string(out))
	}
}

// TestHTTPSInfoRogueCert connects via two-way authenticated HTTPS to the info endpoint
// by using a rogue client certificate and checks that it fails with the expected error.
func (s *DockerDaemonSuite) TestHTTPSInfoRogueCert(c *check.C) {
	const (
		errBadCertificate   = "bad certificate"
		testDaemonHTTPSAddr = "tcp://localhost:4271"
	)

	if err := s.d.Start("--tlsverify", "--tlscacert", "fixtures/https/ca.pem", "--tlscert", "fixtures/https/server-cert.pem",
		"--tlskey", "fixtures/https/server-key.pem", "-H", testDaemonHTTPSAddr); err != nil {
		c.Fatalf("Could not start daemon with busybox: %v", err)
	}

	args := []string{
		"--host", testDaemonHTTPSAddr,
		"--tlsverify", "--tlscacert", "fixtures/https/ca.pem",
		"--tlscert", "fixtures/https/client-rogue-cert.pem",
		"--tlskey", "fixtures/https/client-rogue-key.pem",
		"info",
	}
	out, err := s.d.Cmd(args...)
	if err == nil || !strings.Contains(out, errBadCertificate) {
		c.Fatalf("Expected err: %s, got instead: %s and output: %s", errBadCertificate, err, out)
	}
}

// TestHTTPSInfoRogueServerCert connects via two-way authenticated HTTPS to the info endpoint
// which provides a rogue server certificate and checks that it fails with the expected error
func (s *DockerDaemonSuite) TestHTTPSInfoRogueServerCert(c *check.C) {
	const (
		errCaUnknown             = "x509: certificate signed by unknown authority"
		testDaemonRogueHTTPSAddr = "tcp://localhost:4272"
	)
	if err := s.d.Start("--tlsverify", "--tlscacert", "fixtures/https/ca.pem", "--tlscert", "fixtures/https/server-rogue-cert.pem",
		"--tlskey", "fixtures/https/server-rogue-key.pem", "-H", testDaemonRogueHTTPSAddr); err != nil {
		c.Fatalf("Could not start daemon with busybox: %v", err)
	}

	args := []string{
		"--host", testDaemonRogueHTTPSAddr,
		"--tlsverify", "--tlscacert", "fixtures/https/ca.pem",
		"--tlscert", "fixtures/https/client-rogue-cert.pem",
		"--tlskey", "fixtures/https/client-rogue-key.pem",
		"info",
	}
	out, err := s.d.Cmd(args...)
	if err == nil || !strings.Contains(out, errCaUnknown) {
		c.Fatalf("Expected err: %s, got instead: %s and output: %s", errCaUnknown, err, out)
	}
}

func pingContainers(c *check.C, d *Daemon, expectFailure bool) {
	var dargs []string
	if d != nil {
		dargs = []string{"--host", d.sock()}
	}

	args := append(dargs, "run", "-d", "--name", "container1", "busybox", "top")
	dockerCmd(c, args...)

	args = append(dargs, "run", "--rm", "--link", "container1:alias1", "busybox", "sh", "-c")
	pingCmd := "ping -c 1 %s -W 1"
	args = append(args, fmt.Sprintf(pingCmd, "alias1"))
	_, _, err := dockerCmdWithError(args...)

	if expectFailure {
		c.Assert(err, check.NotNil)
	} else {
		c.Assert(err, check.IsNil)
	}

	args = append(dargs, "rm", "-f", "container1")
	dockerCmd(c, args...)
}

func (s *DockerDaemonSuite) TestDaemonRestartWithSocketAsVolume(c *check.C) {
	c.Assert(s.d.StartWithBusybox(), check.IsNil)

	socket := filepath.Join(s.d.folder, "docker.sock")

	out, err := s.d.Cmd("run", "--restart=always", "-v", socket+":/sock", "busybox")
	c.Assert(err, check.IsNil, check.Commentf("Output: %s", out))
	c.Assert(s.d.Restart(), check.IsNil)
}

// os.Kill should kill daemon ungracefully, leaving behind container mounts.
// A subsequent daemon restart shoud clean up said mounts.
func (s *DockerDaemonSuite) TestCleanupMountsAfterDaemonAndContainerKill(c *check.C) {
	c.Assert(s.d.StartWithBusybox(), check.IsNil)

	out, err := s.d.Cmd("run", "-d", "busybox", "top")
	c.Assert(err, check.IsNil, check.Commentf("Output: %s", out))
	id := strings.TrimSpace(out)
	c.Assert(s.d.cmd.Process.Signal(os.Kill), check.IsNil)
	mountOut, err := ioutil.ReadFile("/proc/self/mountinfo")
	c.Assert(err, check.IsNil, check.Commentf("Output: %s", mountOut))

	// container mounts should exist even after daemon has crashed.
	comment := check.Commentf("%s should stay mounted from older daemon start:\nDaemon root repository %s\n%s", id, s.d.folder, mountOut)
	c.Assert(strings.Contains(string(mountOut), id), check.Equals, true, comment)

	// kill the container
	runCmd := exec.Command(ctrBinary, "--address", "unix:///var/run/docker/libcontainerd/docker-containerd.sock", "containers", "kill", id)
	if out, ec, err := runCommandWithOutput(runCmd); err != nil {
		c.Fatalf("Failed to run ctr, ExitCode: %d, err: %v output: %s id: %s\n", ec, err, out, id)
	}

	// restart daemon.
	if err := s.d.Restart(); err != nil {
		c.Fatal(err)
	}

	// Now, container mounts should be gone.
	mountOut, err = ioutil.ReadFile("/proc/self/mountinfo")
	c.Assert(err, check.IsNil, check.Commentf("Output: %s", mountOut))
	comment = check.Commentf("%s is still mounted from older daemon start:\nDaemon root repository %s\n%s", id, s.d.folder, mountOut)
	c.Assert(strings.Contains(string(mountOut), id), check.Equals, false, comment)
}

// os.Interrupt should perform a graceful daemon shutdown and hence cleanup mounts.
func (s *DockerDaemonSuite) TestCleanupMountsAfterGracefulShutdown(c *check.C) {
	c.Assert(s.d.StartWithBusybox(), check.IsNil)

	out, err := s.d.Cmd("run", "-d", "busybox", "top")
	c.Assert(err, check.IsNil, check.Commentf("Output: %s", out))
	id := strings.TrimSpace(out)

	// Send SIGINT and daemon should clean up
	c.Assert(s.d.cmd.Process.Signal(os.Interrupt), check.IsNil)
	// Wait for the daemon to stop.
	c.Assert(<-s.d.wait, checker.IsNil)

	mountOut, err := ioutil.ReadFile("/proc/self/mountinfo")
	c.Assert(err, check.IsNil, check.Commentf("Output: %s", mountOut))

	comment := check.Commentf("%s is still mounted from older daemon start:\nDaemon root repository %s\n%s", id, s.d.folder, mountOut)
	c.Assert(strings.Contains(string(mountOut), id), check.Equals, false, comment)
}

func (s *DockerDaemonSuite) TestRunContainerWithBridgeNone(c *check.C) {
	testRequires(c, DaemonIsLinux, NotUserNamespace)
	c.Assert(s.d.StartWithBusybox("-b", "none"), check.IsNil)

	out, err := s.d.Cmd("run", "--rm", "busybox", "ip", "l")
	c.Assert(err, check.IsNil, check.Commentf("Output: %s", out))
	c.Assert(strings.Contains(out, "eth0"), check.Equals, false,
		check.Commentf("There shouldn't be eth0 in container in default(bridge) mode when bridge network is disabled: %s", out))

	out, err = s.d.Cmd("run", "--rm", "--net=bridge", "busybox", "ip", "l")
	c.Assert(err, check.IsNil, check.Commentf("Output: %s", out))
	c.Assert(strings.Contains(out, "eth0"), check.Equals, false,
		check.Commentf("There shouldn't be eth0 in container in bridge mode when bridge network is disabled: %s", out))
	// the extra grep and awk clean up the output of `ip` to only list the number and name of
	// interfaces, allowing for different versions of ip (e.g. inside and outside the container) to
	// be used while still verifying that the interface list is the exact same
	cmd := exec.Command("sh", "-c", "ip l | grep -E '^[0-9]+:' | awk -F: ' { print $1\":\"$2 } '")
	stdout := bytes.NewBuffer(nil)
	cmd.Stdout = stdout
	if err := cmd.Run(); err != nil {
		c.Fatal("Failed to get host network interface")
	}
	out, err = s.d.Cmd("run", "--rm", "--net=host", "busybox", "sh", "-c", "ip l | grep -E '^[0-9]+:' | awk -F: ' { print $1\":\"$2 } '")
	c.Assert(err, check.IsNil, check.Commentf("Output: %s", out))
	c.Assert(out, check.Equals, fmt.Sprintf("%s", stdout),
		check.Commentf("The network interfaces in container should be the same with host when --net=host when bridge network is disabled: %s", out))
}

func (s *DockerDaemonSuite) TestDaemonRestartWithContainerRunning(t *check.C) {
	if err := s.d.StartWithBusybox(); err != nil {
		t.Fatal(err)
	}
	if out, err := s.d.Cmd("run", "-d", "--name", "test", "busybox", "top"); err != nil {
		t.Fatal(out, err)
	}

	if err := s.d.Restart(); err != nil {
		t.Fatal(err)
	}
	// Container 'test' should be removed without error
	if out, err := s.d.Cmd("rm", "test"); err != nil {
		t.Fatal(out, err)
	}
}

func (s *DockerDaemonSuite) TestDaemonRestartCleanupNetns(c *check.C) {
	if err := s.d.StartWithBusybox(); err != nil {
		c.Fatal(err)
	}
	out, err := s.d.Cmd("run", "--name", "netns", "-d", "busybox", "top")
	if err != nil {
		c.Fatal(out, err)
	}

	// Get sandbox key via inspect
	out, err = s.d.Cmd("inspect", "--format", "'{{.NetworkSettings.SandboxKey}}'", "netns")
	if err != nil {
		c.Fatalf("Error inspecting container: %s, %v", out, err)
	}
	fileName := strings.Trim(out, " \r\n'")

	if out, err := s.d.Cmd("stop", "netns"); err != nil {
		c.Fatal(out, err)
	}

	// Test if the file still exists
	out, _, err = runCommandWithOutput(exec.Command("stat", "-c", "%n", fileName))
	out = strings.TrimSpace(out)
	c.Assert(err, check.IsNil, check.Commentf("Output: %s", out))
	c.Assert(out, check.Equals, fileName, check.Commentf("Output: %s", out))

	// Remove the container and restart the daemon
	if out, err := s.d.Cmd("rm", "netns"); err != nil {
		c.Fatal(out, err)
	}

	if err := s.d.Restart(); err != nil {
		c.Fatal(err)
	}

	// Test again and see now the netns file does not exist
	out, _, err = runCommandWithOutput(exec.Command("stat", "-c", "%n", fileName))
	out = strings.TrimSpace(out)
	c.Assert(err, check.Not(check.IsNil), check.Commentf("Output: %s", out))
}

// tests regression detailed in #13964 where DOCKER_TLS_VERIFY env is ignored
func (s *DockerDaemonSuite) TestDaemonTLSVerifyIssue13964(c *check.C) {
	host := "tcp://localhost:4271"
	c.Assert(s.d.Start("-H", host), check.IsNil)
	cmd := exec.Command(dockerBinary, "-H", host, "info")
	cmd.Env = []string{"DOCKER_TLS_VERIFY=1", "DOCKER_CERT_PATH=fixtures/https"}
	out, _, err := runCommandWithOutput(cmd)
	c.Assert(err, check.Not(check.IsNil), check.Commentf("%s", out))
	c.Assert(strings.Contains(out, "error during connect"), check.Equals, true)

}

func setupV6(c *check.C) {
	// Hack to get the right IPv6 address on docker0, which has already been created
	result := icmd.RunCommand("ip", "addr", "add", "fe80::1/64", "dev", "docker0")
	result.Assert(c, icmd.Expected{})
}

func teardownV6(c *check.C) {
	result := icmd.RunCommand("ip", "addr", "del", "fe80::1/64", "dev", "docker0")
	result.Assert(c, icmd.Expected{})
}

func (s *DockerDaemonSuite) TestDaemonRestartWithContainerWithRestartPolicyAlways(c *check.C) {
	c.Assert(s.d.StartWithBusybox(), check.IsNil)

	out, err := s.d.Cmd("run", "-d", "--restart", "always", "busybox", "top")
	c.Assert(err, check.IsNil)
	id := strings.TrimSpace(out)

	_, err = s.d.Cmd("stop", id)
	c.Assert(err, check.IsNil)
	_, err = s.d.Cmd("wait", id)
	c.Assert(err, check.IsNil)

	out, err = s.d.Cmd("ps", "-q")
	c.Assert(err, check.IsNil)
	c.Assert(out, check.Equals, "")

	c.Assert(s.d.Restart(), check.IsNil)

	out, err = s.d.Cmd("ps", "-q")
	c.Assert(err, check.IsNil)
	c.Assert(strings.TrimSpace(out), check.Equals, id[:12])
}

func (s *DockerDaemonSuite) TestDaemonWideLogConfig(c *check.C) {
	if err := s.d.StartWithBusybox("--log-opt=max-size=1k"); err != nil {
		c.Fatal(err)
	}
	name := "logtest"
	out, err := s.d.Cmd("run", "-d", "--log-opt=max-file=5", "--name", name, "busybox", "top")
	c.Assert(err, check.IsNil, check.Commentf("Output: %s, err: %v", out, err))

	out, err = s.d.Cmd("inspect", "-f", "{{ .HostConfig.LogConfig.Config }}", name)
	c.Assert(err, check.IsNil, check.Commentf("Output: %s", out))
	c.Assert(out, checker.Contains, "max-size:1k")
	c.Assert(out, checker.Contains, "max-file:5")

	out, err = s.d.Cmd("inspect", "-f", "{{ .HostConfig.LogConfig.Type }}", name)
	c.Assert(err, check.IsNil, check.Commentf("Output: %s", out))
	c.Assert(strings.TrimSpace(out), checker.Equals, "json-file")
}

func (s *DockerDaemonSuite) TestDaemonRestartWithPausedContainer(c *check.C) {
	if err := s.d.StartWithBusybox(); err != nil {
		c.Fatal(err)
	}
	if out, err := s.d.Cmd("run", "-i", "-d", "--name", "test", "busybox", "top"); err != nil {
		c.Fatal(err, out)
	}
	if out, err := s.d.Cmd("pause", "test"); err != nil {
		c.Fatal(err, out)
	}
	if err := s.d.Restart(); err != nil {
		c.Fatal(err)
	}

	errchan := make(chan error)
	go func() {
		out, err := s.d.Cmd("start", "test")
		if err != nil {
			errchan <- fmt.Errorf("%v:\n%s", err, out)
		}
		name := strings.TrimSpace(out)
		if name != "test" {
			errchan <- fmt.Errorf("Paused container start error on docker daemon restart, expected 'test' but got '%s'", name)
		}
		close(errchan)
	}()

	select {
	case <-time.After(5 * time.Second):
		c.Fatal("Waiting on start a container timed out")
	case err := <-errchan:
		if err != nil {
			c.Fatal(err)
		}
	}
}

func (s *DockerDaemonSuite) TestDaemonRestartRmVolumeInUse(c *check.C) {
	c.Assert(s.d.StartWithBusybox(), check.IsNil)

	out, err := s.d.Cmd("create", "-v", "test:/foo", "busybox")
	c.Assert(err, check.IsNil, check.Commentf(out))

	c.Assert(s.d.Restart(), check.IsNil)

	out, err = s.d.Cmd("volume", "rm", "test")
	c.Assert(err, check.NotNil, check.Commentf("should not be able to remove in use volume after daemon restart"))
	c.Assert(out, checker.Contains, "in use")
}

func (s *DockerDaemonSuite) TestDaemonRestartLocalVolumes(c *check.C) {
	c.Assert(s.d.Start(), check.IsNil)

	_, err := s.d.Cmd("volume", "create", "test")
	c.Assert(err, check.IsNil)
	c.Assert(s.d.Restart(), check.IsNil)

	_, err = s.d.Cmd("volume", "inspect", "test")
	c.Assert(err, check.IsNil)
}

func (s *DockerDaemonSuite) TestDaemonCorruptedLogDriverAddress(c *check.C) {
	c.Assert(s.d.Start("--log-driver=syslog", "--log-opt", "syslog-address=corrupted:42"), check.NotNil)
	expected := "Failed to set log opts: syslog-address should be in form proto://address"
	runCmd := exec.Command("grep", expected, s.d.LogFileName())
	if out, _, err := runCommandWithOutput(runCmd); err != nil {
		c.Fatalf("Expected %q message; but doesn't exist in log: %q, err: %v", expected, out, err)
	}
}

func (s *DockerDaemonSuite) TestDaemonCorruptedFluentdAddress(c *check.C) {
	c.Assert(s.d.Start("--log-driver=fluentd", "--log-opt", "fluentd-address=corrupted:c"), check.NotNil)
	expected := "Failed to set log opts: invalid fluentd-address corrupted:c: "
	runCmd := exec.Command("grep", expected, s.d.LogFileName())
	if out, _, err := runCommandWithOutput(runCmd); err != nil {
		c.Fatalf("Expected %q message; but doesn't exist in log: %q, err: %v", expected, out, err)
	}
}

func (s *DockerDaemonSuite) TestDaemonStartWithoutHost(c *check.C) {
	s.d.useDefaultHost = true
	defer func() {
		s.d.useDefaultHost = false
	}()
	c.Assert(s.d.Start(), check.IsNil)
}

func (s *DockerDaemonSuite) TestDaemonStartWithDefalutTLSHost(c *check.C) {
	s.d.useDefaultTLSHost = true
	defer func() {
		s.d.useDefaultTLSHost = false
	}()
	if err := s.d.Start(
		"--tlsverify",
		"--tlscacert", "fixtures/https/ca.pem",
		"--tlscert", "fixtures/https/server-cert.pem",
		"--tlskey", "fixtures/https/server-key.pem"); err != nil {
		c.Fatalf("Could not start daemon: %v", err)
	}

	// The client with --tlsverify should also use default host localhost:2376
	tmpHost := os.Getenv("DOCKER_HOST")
	defer func() {
		os.Setenv("DOCKER_HOST", tmpHost)
	}()

	os.Setenv("DOCKER_HOST", "")

	out, _ := dockerCmd(
		c,
		"--tlsverify",
		"--tlscacert", "fixtures/https/ca.pem",
		"--tlscert", "fixtures/https/client-cert.pem",
		"--tlskey", "fixtures/https/client-key.pem",
		"version",
	)
	if !strings.Contains(out, "Server") {
		c.Fatalf("docker version should return information of server side")
	}
}

func (s *DockerDaemonSuite) TestBridgeIPIsExcludedFromAllocatorPool(c *check.C) {
	defaultNetworkBridge := "docker0"
	deleteInterface(c, defaultNetworkBridge)

	bridgeIP := "192.169.1.1"
	bridgeRange := bridgeIP + "/30"

	err := s.d.StartWithBusybox("--bip", bridgeRange)
	c.Assert(err, check.IsNil)
	defer s.d.Restart()

	var cont int
	for {
		contName := fmt.Sprintf("container%d", cont)
		_, err = s.d.Cmd("run", "--name", contName, "-d", "busybox", "/bin/sleep", "2")
		if err != nil {
			// pool exhausted
			break
		}
		ip, err := s.d.Cmd("inspect", "--format", "'{{.NetworkSettings.IPAddress}}'", contName)
		c.Assert(err, check.IsNil)

		c.Assert(ip, check.Not(check.Equals), bridgeIP)
		cont++
	}
}

// Test daemon for no space left on device error
func (s *DockerDaemonSuite) TestDaemonNoSpaceLeftOnDeviceError(c *check.C) {
	testRequires(c, SameHostDaemon, DaemonIsLinux, Network)

	testDir, err := ioutil.TempDir("", "no-space-left-on-device-test")
	c.Assert(err, checker.IsNil)
	defer os.RemoveAll(testDir)
	c.Assert(mount.MakeRShared(testDir), checker.IsNil)
	defer mount.Unmount(testDir)

	// create a 2MiB image and mount it as graph root
	// Why in a container? Because `mount` sometimes behaves weirdly and often fails outright on this test in debian:jessie (which is what the test suite runs under if run from the Makefile)
	dockerCmd(c, "run", "--rm", "-v", testDir+":/test", "busybox", "sh", "-c", "dd of=/test/testfs.img bs=1M seek=2 count=0")
	out, _, err := runCommandWithOutput(exec.Command("mkfs.ext4", "-F", filepath.Join(testDir, "testfs.img"))) // `mkfs.ext4` is not in busybox
	c.Assert(err, checker.IsNil, check.Commentf(out))

	cmd := exec.Command("losetup", "-f", "--show", filepath.Join(testDir, "testfs.img"))
	loout, err := cmd.CombinedOutput()
	c.Assert(err, checker.IsNil)
	loopname := strings.TrimSpace(string(loout))
	defer exec.Command("losetup", "-d", loopname).Run()

	dockerCmd(c, "run", "--privileged", "--rm", "-v", testDir+":/test:shared", "busybox", "sh", "-c", fmt.Sprintf("mkdir -p /test/test-mount && mount -t ext4 -no loop,rw %v /test/test-mount", loopname))
	defer mount.Unmount(filepath.Join(testDir, "test-mount"))

	err = s.d.Start("--graph", filepath.Join(testDir, "test-mount"))
	defer s.d.Stop()
	c.Assert(err, check.IsNil)

	// pull a repository large enough to fill the mount point
	pullOut, err := s.d.Cmd("pull", "registry:2")
	c.Assert(err, checker.NotNil, check.Commentf(pullOut))
	c.Assert(pullOut, checker.Contains, "no space left on device")
}

// Test daemon restart with container links + auto restart
func (s *DockerDaemonSuite) TestDaemonRestartContainerLinksRestart(c *check.C) {
	err := s.d.StartWithBusybox()
	c.Assert(err, checker.IsNil)

	parent1Args := []string{}
	parent2Args := []string{}
	wg := sync.WaitGroup{}
	maxChildren := 10
	chErr := make(chan error, maxChildren)

	for i := 0; i < maxChildren; i++ {
		wg.Add(1)
		name := fmt.Sprintf("test%d", i)

		if i < maxChildren/2 {
			parent1Args = append(parent1Args, []string{"--link", name}...)
		} else {
			parent2Args = append(parent2Args, []string{"--link", name}...)
		}

		go func() {
			_, err = s.d.Cmd("run", "-d", "--name", name, "--restart=always", "busybox", "top")
			chErr <- err
			wg.Done()
		}()
	}

	wg.Wait()
	close(chErr)
	for err := range chErr {
		c.Assert(err, check.IsNil)
	}

	parent1Args = append([]string{"run", "-d"}, parent1Args...)
	parent1Args = append(parent1Args, []string{"--name=parent1", "--restart=always", "busybox", "top"}...)
	parent2Args = append([]string{"run", "-d"}, parent2Args...)
	parent2Args = append(parent2Args, []string{"--name=parent2", "--restart=always", "busybox", "top"}...)

	_, err = s.d.Cmd(parent1Args...)
	c.Assert(err, check.IsNil)
	_, err = s.d.Cmd(parent2Args...)
	c.Assert(err, check.IsNil)

	err = s.d.Stop()
	c.Assert(err, check.IsNil)
	// clear the log file -- we don't need any of it but may for the next part
	// can ignore the error here, this is just a cleanup
	os.Truncate(s.d.LogFileName(), 0)
	err = s.d.Start()
	c.Assert(err, check.IsNil)

	for _, num := range []string{"1", "2"} {
		out, err := s.d.Cmd("inspect", "-f", "{{ .State.Running }}", "parent"+num)
		c.Assert(err, check.IsNil)
		if strings.TrimSpace(out) != "true" {
			log, _ := ioutil.ReadFile(s.d.LogFileName())
			c.Fatalf("parent container is not running\n%s", string(log))
		}
	}
}

func (s *DockerDaemonSuite) TestDaemonCgroupParent(c *check.C) {
	testRequires(c, DaemonIsLinux)

	cgroupParent := "test"
	name := "cgroup-test"

	err := s.d.StartWithBusybox("--cgroup-parent", cgroupParent)
	c.Assert(err, check.IsNil)
	defer s.d.Restart()

	out, err := s.d.Cmd("run", "--name", name, "busybox", "cat", "/proc/self/cgroup")
	c.Assert(err, checker.IsNil)
	cgroupPaths := parseCgroupPaths(string(out))
	c.Assert(len(cgroupPaths), checker.Not(checker.Equals), 0, check.Commentf("unexpected output - %q", string(out)))
	out, err = s.d.Cmd("inspect", "-f", "{{.Id}}", name)
	c.Assert(err, checker.IsNil)
	id := strings.TrimSpace(string(out))
	expectedCgroup := path.Join(cgroupParent, id)
	found := false
	for _, path := range cgroupPaths {
		if strings.HasSuffix(path, expectedCgroup) {
			found = true
			break
		}
	}
	c.Assert(found, checker.True, check.Commentf("Cgroup path for container (%s) doesn't found in cgroups file: %s", expectedCgroup, cgroupPaths))
}

func (s *DockerDaemonSuite) TestDaemonRestartWithLinks(c *check.C) {
	testRequires(c, DaemonIsLinux) // Windows does not support links
	err := s.d.StartWithBusybox()
	c.Assert(err, check.IsNil)

	out, err := s.d.Cmd("run", "-d", "--name=test", "busybox", "top")
	c.Assert(err, check.IsNil, check.Commentf(out))

	out, err = s.d.Cmd("run", "--name=test2", "--link", "test:abc", "busybox", "sh", "-c", "ping -c 1 -w 1 abc")
	c.Assert(err, check.IsNil, check.Commentf(out))

	c.Assert(s.d.Restart(), check.IsNil)

	// should fail since test is not running yet
	out, err = s.d.Cmd("start", "test2")
	c.Assert(err, check.NotNil, check.Commentf(out))

	out, err = s.d.Cmd("start", "test")
	c.Assert(err, check.IsNil, check.Commentf(out))
	out, err = s.d.Cmd("start", "-a", "test2")
	c.Assert(err, check.IsNil, check.Commentf(out))
	c.Assert(strings.Contains(out, "1 packets transmitted, 1 packets received"), check.Equals, true, check.Commentf(out))
}

func (s *DockerDaemonSuite) TestDaemonRestartWithNames(c *check.C) {
	testRequires(c, DaemonIsLinux) // Windows does not support links
	err := s.d.StartWithBusybox()
	c.Assert(err, check.IsNil)

	out, err := s.d.Cmd("create", "--name=test", "busybox")
	c.Assert(err, check.IsNil, check.Commentf(out))

	out, err = s.d.Cmd("run", "-d", "--name=test2", "busybox", "top")
	c.Assert(err, check.IsNil, check.Commentf(out))
	test2ID := strings.TrimSpace(out)

	out, err = s.d.Cmd("run", "-d", "--name=test3", "--link", "test2:abc", "busybox", "top")
	test3ID := strings.TrimSpace(out)

	c.Assert(s.d.Restart(), check.IsNil)

	out, err = s.d.Cmd("create", "--name=test", "busybox")
	c.Assert(err, check.NotNil, check.Commentf("expected error trying to create container with duplicate name"))
	// this one is no longer needed, removing simplifies the remainder of the test
	out, err = s.d.Cmd("rm", "-f", "test")
	c.Assert(err, check.IsNil, check.Commentf(out))

	out, err = s.d.Cmd("ps", "-a", "--no-trunc")
	c.Assert(err, check.IsNil, check.Commentf(out))

	lines := strings.Split(strings.TrimSpace(out), "\n")[1:]

	test2validated := false
	test3validated := false
	for _, line := range lines {
		fields := strings.Fields(line)
		names := fields[len(fields)-1]
		switch fields[0] {
		case test2ID:
			c.Assert(names, check.Equals, "test2,test3/abc")
			test2validated = true
		case test3ID:
			c.Assert(names, check.Equals, "test3")
			test3validated = true
		}
	}

	c.Assert(test2validated, check.Equals, true)
	c.Assert(test3validated, check.Equals, true)
}

// TestDaemonRestartWithKilledRunningContainer requires live restore of running containers
func (s *DockerDaemonSuite) TestDaemonRestartWithKilledRunningContainer(t *check.C) {
	// TODO(mlaventure): Not sure what would the exit code be on windows
	testRequires(t, DaemonIsLinux)
	if err := s.d.StartWithBusybox(); err != nil {
		t.Fatal(err)
	}

	cid, err := s.d.Cmd("run", "-d", "--name", "test", "busybox", "top")
	defer s.d.Stop()
	if err != nil {
		t.Fatal(cid, err)
	}
	cid = strings.TrimSpace(cid)

	pid, err := s.d.Cmd("inspect", "-f", "{{.State.Pid}}", cid)
	t.Assert(err, check.IsNil)
	pid = strings.TrimSpace(pid)

	// Kill the daemon
	if err := s.d.Kill(); err != nil {
		t.Fatal(err)
	}

	// kill the container
	runCmd := exec.Command(ctrBinary, "--address", "unix:///var/run/docker/libcontainerd/docker-containerd.sock", "containers", "kill", cid)
	if out, ec, err := runCommandWithOutput(runCmd); err != nil {
		t.Fatalf("Failed to run ctr, ExitCode: %d, err: '%v' output: '%s' cid: '%s'\n", ec, err, out, cid)
	}

	// Give time to containerd to process the command if we don't
	// the exit event might be received after we do the inspect
	pidCmd := exec.Command("kill", "-0", pid)
	_, ec, _ := runCommandWithOutput(pidCmd)
	for ec == 0 {
		time.Sleep(1 * time.Second)
		_, ec, _ = runCommandWithOutput(pidCmd)
	}

	// restart the daemon
	if err := s.d.Start(); err != nil {
		t.Fatal(err)
	}

	// Check that we've got the correct exit code
	out, err := s.d.Cmd("inspect", "-f", "{{.State.ExitCode}}", cid)
	t.Assert(err, check.IsNil)

	out = strings.TrimSpace(out)
	if out != "143" {
		t.Fatalf("Expected exit code '%s' got '%s' for container '%s'\n", "143", out, cid)
	}

}

// os.Kill should kill daemon ungracefully, leaving behind live containers.
// The live containers should be known to the restarted daemon. Stopping
// them now, should remove the mounts.
func (s *DockerDaemonSuite) TestCleanupMountsAfterDaemonCrash(c *check.C) {
	testRequires(c, DaemonIsLinux)
	c.Assert(s.d.StartWithBusybox("--live-restore"), check.IsNil)

	out, err := s.d.Cmd("run", "-d", "busybox", "top")
	c.Assert(err, check.IsNil, check.Commentf("Output: %s", out))
	id := strings.TrimSpace(out)

	c.Assert(s.d.cmd.Process.Signal(os.Kill), check.IsNil)
	mountOut, err := ioutil.ReadFile("/proc/self/mountinfo")
	c.Assert(err, check.IsNil, check.Commentf("Output: %s", mountOut))

	// container mounts should exist even after daemon has crashed.
	comment := check.Commentf("%s should stay mounted from older daemon start:\nDaemon root repository %s\n%s", id, s.d.folder, mountOut)
	c.Assert(strings.Contains(string(mountOut), id), check.Equals, true, comment)

	// restart daemon.
	if err := s.d.Restart("--live-restore"); err != nil {
		c.Fatal(err)
	}

	// container should be running.
	out, err = s.d.Cmd("inspect", "--format={{.State.Running}}", id)
	c.Assert(err, check.IsNil, check.Commentf("Output: %s", out))
	out = strings.TrimSpace(out)
	if out != "true" {
		c.Fatalf("Container %s expected to stay alive after daemon restart", id)
	}

	// 'docker stop' should work.
	out, err = s.d.Cmd("stop", id)
	c.Assert(err, check.IsNil, check.Commentf("Output: %s", out))

	// Now, container mounts should be gone.
	mountOut, err = ioutil.ReadFile("/proc/self/mountinfo")
	c.Assert(err, check.IsNil, check.Commentf("Output: %s", mountOut))
	comment = check.Commentf("%s is still mounted from older daemon start:\nDaemon root repository %s\n%s", id, s.d.folder, mountOut)
	c.Assert(strings.Contains(string(mountOut), id), check.Equals, false, comment)
}

// TestDaemonRestartWithUnpausedRunningContainer requires live restore of running containers.
func (s *DockerDaemonSuite) TestDaemonRestartWithUnpausedRunningContainer(t *check.C) {
	// TODO(mlaventure): Not sure what would the exit code be on windows
	testRequires(t, DaemonIsLinux)
	if err := s.d.StartWithBusybox("--live-restore"); err != nil {
		t.Fatal(err)
	}

	cid, err := s.d.Cmd("run", "-d", "--name", "test", "busybox", "top")
	defer s.d.Stop()
	if err != nil {
		t.Fatal(cid, err)
	}
	cid = strings.TrimSpace(cid)

	pid, err := s.d.Cmd("inspect", "-f", "{{.State.Pid}}", cid)
	t.Assert(err, check.IsNil)

	// pause the container
	if _, err := s.d.Cmd("pause", cid); err != nil {
		t.Fatal(cid, err)
	}

	// Kill the daemon
	if err := s.d.Kill(); err != nil {
		t.Fatal(err)
	}

	// resume the container
	result := icmd.RunCommand(
		ctrBinary,
		"--address", "unix:///var/run/docker/libcontainerd/docker-containerd.sock",
		"containers", "resume", cid)
	t.Assert(result, icmd.Matches, icmd.Success)

	// Give time to containerd to process the command if we don't
	// the resume event might be received after we do the inspect
	waitAndAssert(t, defaultReconciliationTimeout, func(*check.C) (interface{}, check.CommentInterface) {
		result := icmd.RunCommand("kill", "-0", strings.TrimSpace(pid))
		return result.ExitCode, nil
	}, checker.Equals, 0)

	// restart the daemon
	if err := s.d.Start("--live-restore"); err != nil {
		t.Fatal(err)
	}

	// Check that we've got the correct status
	out, err := s.d.Cmd("inspect", "-f", "{{.State.Status}}", cid)
	t.Assert(err, check.IsNil)

	out = strings.TrimSpace(out)
	if out != "running" {
		t.Fatalf("Expected exit code '%s' got '%s' for container '%s'\n", "running", out, cid)
	}
	if _, err := s.d.Cmd("kill", cid); err != nil {
		t.Fatal(err)
	}
}

// TestRunLinksChanged checks that creating a new container with the same name does not update links
// this ensures that the old, pre gh#16032 functionality continues on
func (s *DockerDaemonSuite) TestRunLinksChanged(c *check.C) {
	testRequires(c, DaemonIsLinux) // Windows does not support links
	err := s.d.StartWithBusybox()
	c.Assert(err, check.IsNil)

	out, err := s.d.Cmd("run", "-d", "--name=test", "busybox", "top")
	c.Assert(err, check.IsNil, check.Commentf(out))

	out, err = s.d.Cmd("run", "--name=test2", "--link=test:abc", "busybox", "sh", "-c", "ping -c 1 abc")
	c.Assert(err, check.IsNil, check.Commentf(out))
	c.Assert(out, checker.Contains, "1 packets transmitted, 1 packets received")

	out, err = s.d.Cmd("rm", "-f", "test")
	c.Assert(err, check.IsNil, check.Commentf(out))

	out, err = s.d.Cmd("run", "-d", "--name=test", "busybox", "top")
	c.Assert(err, check.IsNil, check.Commentf(out))
	out, err = s.d.Cmd("start", "-a", "test2")
	c.Assert(err, check.NotNil, check.Commentf(out))
	c.Assert(out, check.Not(checker.Contains), "1 packets transmitted, 1 packets received")

	err = s.d.Restart()
	c.Assert(err, check.IsNil)
	out, err = s.d.Cmd("start", "-a", "test2")
	c.Assert(err, check.NotNil, check.Commentf(out))
	c.Assert(out, check.Not(checker.Contains), "1 packets transmitted, 1 packets received")
}

func (s *DockerDaemonSuite) TestDaemonStartWithoutColors(c *check.C) {
	testRequires(c, DaemonIsLinux, NotPpc64le)

	infoLog := "\x1b[34mINFO\x1b"

	p, tty, err := pty.Open()
	c.Assert(err, checker.IsNil)
	defer func() {
		tty.Close()
		p.Close()
	}()

	b := bytes.NewBuffer(nil)
	go io.Copy(b, p)

	// Enable coloring explicitly
	s.d.StartWithLogFile(tty, "--raw-logs=false")
	s.d.Stop()
	c.Assert(b.String(), checker.Contains, infoLog)

	b.Reset()

	// Disable coloring explicitly
	s.d.StartWithLogFile(tty, "--raw-logs=true")
	s.d.Stop()
	c.Assert(b.String(), check.Not(checker.Contains), infoLog)
}

func (s *DockerDaemonSuite) TestDaemonDebugLog(c *check.C) {
	testRequires(c, DaemonIsLinux, NotPpc64le)

	debugLog := "\x1b[37mDEBU\x1b"

	p, tty, err := pty.Open()
	c.Assert(err, checker.IsNil)
	defer func() {
		tty.Close()
		p.Close()
	}()

	b := bytes.NewBuffer(nil)
	go io.Copy(b, p)

	s.d.StartWithLogFile(tty, "--debug")
	s.d.Stop()
	c.Assert(b.String(), checker.Contains, debugLog)
}

func (s *DockerDaemonSuite) TestDaemonDiscoveryBackendConfigReload(c *check.C) {
	testRequires(c, SameHostDaemon, DaemonIsLinux)

	// daemon config file
	daemonConfig := `{ "debug" : false }`
	configFile, err := ioutil.TempFile("", "test-daemon-discovery-backend-config-reload-config")
	c.Assert(err, checker.IsNil, check.Commentf("could not create temp file for config reload"))
	configFilePath := configFile.Name()
	defer func() {
		configFile.Close()
		os.RemoveAll(configFile.Name())
	}()

	_, err = configFile.Write([]byte(daemonConfig))
	c.Assert(err, checker.IsNil)

	// --log-level needs to be set so that d.Start() doesn't add --debug causing
	// a conflict with the config
	err = s.d.Start("--config-file", configFilePath, "--log-level=info")
	c.Assert(err, checker.IsNil)

	// daemon config file
	daemonConfig = `{
	      "cluster-store": "consul://consuladdr:consulport/some/path",
	      "cluster-advertise": "192.168.56.100:0",
	      "debug" : false
	}`

	err = configFile.Truncate(0)
	c.Assert(err, checker.IsNil)
	_, err = configFile.Seek(0, os.SEEK_SET)
	c.Assert(err, checker.IsNil)

	_, err = configFile.Write([]byte(daemonConfig))
	c.Assert(err, checker.IsNil)

	err = s.d.reloadConfig()
	c.Assert(err, checker.IsNil, check.Commentf("error reloading daemon config"))

	out, err := s.d.Cmd("info")
	c.Assert(err, checker.IsNil)

	c.Assert(out, checker.Contains, fmt.Sprintf("Cluster Store: consul://consuladdr:consulport/some/path"))
	c.Assert(out, checker.Contains, fmt.Sprintf("Cluster Advertise: 192.168.56.100:0"))
}

// Test for #21956
func (s *DockerDaemonSuite) TestDaemonLogOptions(c *check.C) {
	err := s.d.StartWithBusybox("--log-driver=syslog", "--log-opt=syslog-address=udp://127.0.0.1:514")
	c.Assert(err, check.IsNil)

	out, err := s.d.Cmd("run", "-d", "--log-driver=json-file", "busybox", "top")
	c.Assert(err, check.IsNil, check.Commentf(out))
	id := strings.TrimSpace(out)

	out, err = s.d.Cmd("inspect", "--format='{{.HostConfig.LogConfig}}'", id)
	c.Assert(err, check.IsNil, check.Commentf(out))
	c.Assert(out, checker.Contains, "{json-file map[]}")
}

// Test case for #20936, #22443
func (s *DockerDaemonSuite) TestDaemonMaxConcurrency(c *check.C) {
	c.Assert(s.d.Start("--max-concurrent-uploads=6", "--max-concurrent-downloads=8"), check.IsNil)

	expectedMaxConcurrentUploads := `level=debug msg="Max Concurrent Uploads: 6"`
	expectedMaxConcurrentDownloads := `level=debug msg="Max Concurrent Downloads: 8"`
	content, _ := ioutil.ReadFile(s.d.logFile.Name())
	c.Assert(string(content), checker.Contains, expectedMaxConcurrentUploads)
	c.Assert(string(content), checker.Contains, expectedMaxConcurrentDownloads)
}

// Test case for #20936, #22443
func (s *DockerDaemonSuite) TestDaemonMaxConcurrencyWithConfigFile(c *check.C) {
	testRequires(c, SameHostDaemon, DaemonIsLinux)

	// daemon config file
	configFilePath := "test.json"
	configFile, err := os.Create(configFilePath)
	c.Assert(err, checker.IsNil)
	defer os.Remove(configFilePath)

	daemonConfig := `{ "max-concurrent-downloads" : 8 }`
	fmt.Fprintf(configFile, "%s", daemonConfig)
	configFile.Close()
	c.Assert(s.d.Start(fmt.Sprintf("--config-file=%s", configFilePath)), check.IsNil)

	expectedMaxConcurrentUploads := `level=debug msg="Max Concurrent Uploads: 5"`
	expectedMaxConcurrentDownloads := `level=debug msg="Max Concurrent Downloads: 8"`
	content, _ := ioutil.ReadFile(s.d.logFile.Name())
	c.Assert(string(content), checker.Contains, expectedMaxConcurrentUploads)
	c.Assert(string(content), checker.Contains, expectedMaxConcurrentDownloads)

	configFile, err = os.Create(configFilePath)
	c.Assert(err, checker.IsNil)
	daemonConfig = `{ "max-concurrent-uploads" : 7, "max-concurrent-downloads" : 9 }`
	fmt.Fprintf(configFile, "%s", daemonConfig)
	configFile.Close()

	syscall.Kill(s.d.cmd.Process.Pid, syscall.SIGHUP)

	time.Sleep(3 * time.Second)

	expectedMaxConcurrentUploads = `level=debug msg="Reset Max Concurrent Uploads: 7"`
	expectedMaxConcurrentDownloads = `level=debug msg="Reset Max Concurrent Downloads: 9"`
	content, _ = ioutil.ReadFile(s.d.logFile.Name())
	c.Assert(string(content), checker.Contains, expectedMaxConcurrentUploads)
	c.Assert(string(content), checker.Contains, expectedMaxConcurrentDownloads)
}

// Test case for #20936, #22443
func (s *DockerDaemonSuite) TestDaemonMaxConcurrencyWithConfigFileReload(c *check.C) {
	testRequires(c, SameHostDaemon, DaemonIsLinux)

	// daemon config file
	configFilePath := "test.json"
	configFile, err := os.Create(configFilePath)
	c.Assert(err, checker.IsNil)
	defer os.Remove(configFilePath)

	daemonConfig := `{ "max-concurrent-uploads" : null }`
	fmt.Fprintf(configFile, "%s", daemonConfig)
	configFile.Close()
	c.Assert(s.d.Start(fmt.Sprintf("--config-file=%s", configFilePath)), check.IsNil)

	expectedMaxConcurrentUploads := `level=debug msg="Max Concurrent Uploads: 5"`
	expectedMaxConcurrentDownloads := `level=debug msg="Max Concurrent Downloads: 3"`
	content, _ := ioutil.ReadFile(s.d.logFile.Name())
	c.Assert(string(content), checker.Contains, expectedMaxConcurrentUploads)
	c.Assert(string(content), checker.Contains, expectedMaxConcurrentDownloads)

	configFile, err = os.Create(configFilePath)
	c.Assert(err, checker.IsNil)
	daemonConfig = `{ "max-concurrent-uploads" : 1, "max-concurrent-downloads" : null }`
	fmt.Fprintf(configFile, "%s", daemonConfig)
	configFile.Close()

	syscall.Kill(s.d.cmd.Process.Pid, syscall.SIGHUP)

	time.Sleep(3 * time.Second)

	expectedMaxConcurrentUploads = `level=debug msg="Reset Max Concurrent Uploads: 1"`
	expectedMaxConcurrentDownloads = `level=debug msg="Reset Max Concurrent Downloads: 3"`
	content, _ = ioutil.ReadFile(s.d.logFile.Name())
	c.Assert(string(content), checker.Contains, expectedMaxConcurrentUploads)
	c.Assert(string(content), checker.Contains, expectedMaxConcurrentDownloads)

	configFile, err = os.Create(configFilePath)
	c.Assert(err, checker.IsNil)
	daemonConfig = `{ "labels":["foo=bar"] }`
	fmt.Fprintf(configFile, "%s", daemonConfig)
	configFile.Close()

	syscall.Kill(s.d.cmd.Process.Pid, syscall.SIGHUP)

	time.Sleep(3 * time.Second)

	expectedMaxConcurrentUploads = `level=debug msg="Reset Max Concurrent Uploads: 5"`
	expectedMaxConcurrentDownloads = `level=debug msg="Reset Max Concurrent Downloads: 3"`
	content, _ = ioutil.ReadFile(s.d.logFile.Name())
	c.Assert(string(content), checker.Contains, expectedMaxConcurrentUploads)
	c.Assert(string(content), checker.Contains, expectedMaxConcurrentDownloads)
}

func (s *DockerDaemonSuite) TestBuildOnDisabledBridgeNetworkDaemon(c *check.C) {
	err := s.d.StartWithBusybox("-b=none", "--iptables=false")
	c.Assert(err, check.IsNil)
	s.d.c.Logf("dockerBinary %s", dockerBinary)
	out, code, err := s.d.buildImageWithOut("busyboxs",
		`FROM busybox
                RUN cat /etc/hosts`, false)
	comment := check.Commentf("Failed to build image. output %s, exitCode %d, err %v", out, code, err)
	c.Assert(err, check.IsNil, comment)
	c.Assert(code, check.Equals, 0, comment)
}

// Test case for #21976
func (s *DockerDaemonSuite) TestDaemonDNSInHostMode(c *check.C) {
	testRequires(c, SameHostDaemon, DaemonIsLinux)

	err := s.d.StartWithBusybox("--dns", "1.2.3.4")
	c.Assert(err, checker.IsNil)

	expectedOutput := "nameserver 1.2.3.4"
	out, _ := s.d.Cmd("run", "--net=host", "busybox", "cat", "/etc/resolv.conf")
	c.Assert(out, checker.Contains, expectedOutput, check.Commentf("Expected '%s', but got %q", expectedOutput, out))
}

// Test case for #21976
func (s *DockerDaemonSuite) TestDaemonDNSSearchInHostMode(c *check.C) {
	testRequires(c, SameHostDaemon, DaemonIsLinux)

	err := s.d.StartWithBusybox("--dns-search", "example.com")
	c.Assert(err, checker.IsNil)

	expectedOutput := "search example.com"
	out, _ := s.d.Cmd("run", "--net=host", "busybox", "cat", "/etc/resolv.conf")
	c.Assert(out, checker.Contains, expectedOutput, check.Commentf("Expected '%s', but got %q", expectedOutput, out))
}

// Test case for #21976
func (s *DockerDaemonSuite) TestDaemonDNSOptionsInHostMode(c *check.C) {
	testRequires(c, SameHostDaemon, DaemonIsLinux)

	err := s.d.StartWithBusybox("--dns-opt", "timeout:3")
	c.Assert(err, checker.IsNil)

	expectedOutput := "options timeout:3"
	out, _ := s.d.Cmd("run", "--net=host", "busybox", "cat", "/etc/resolv.conf")
	c.Assert(out, checker.Contains, expectedOutput, check.Commentf("Expected '%s', but got %q", expectedOutput, out))
}

func (s *DockerDaemonSuite) TestRunWithRuntimeFromConfigFile(c *check.C) {
	conf, err := ioutil.TempFile("", "config-file-")
	c.Assert(err, check.IsNil)
	configName := conf.Name()
	conf.Close()
	defer os.Remove(configName)

	config := `
{
    "runtimes": {
        "oci": {
            "path": "docker-runc"
        },
        "vm": {
            "path": "/usr/local/bin/vm-manager",
            "runtimeArgs": [
                "--debug"
            ]
        }
    }
}
`
	ioutil.WriteFile(configName, []byte(config), 0644)
	err = s.d.StartWithBusybox("--config-file", configName)
	c.Assert(err, check.IsNil)

	// Run with default runtime
	out, err := s.d.Cmd("run", "--rm", "busybox", "ls")
	c.Assert(err, check.IsNil, check.Commentf(out))

	// Run with default runtime explicitly
	out, err = s.d.Cmd("run", "--rm", "--runtime=runc", "busybox", "ls")
	c.Assert(err, check.IsNil, check.Commentf(out))

	// Run with oci (same path as default) but keep it around
	out, err = s.d.Cmd("run", "--name", "oci-runtime-ls", "--runtime=oci", "busybox", "ls")
	c.Assert(err, check.IsNil, check.Commentf(out))

	// Run with "vm"
	out, err = s.d.Cmd("run", "--rm", "--runtime=vm", "busybox", "ls")
	c.Assert(err, check.NotNil, check.Commentf(out))
	c.Assert(out, checker.Contains, "/usr/local/bin/vm-manager: no such file or directory")

	// Reset config to only have the default
	config = `
{
    "runtimes": {
    }
}
`
	ioutil.WriteFile(configName, []byte(config), 0644)
	syscall.Kill(s.d.cmd.Process.Pid, syscall.SIGHUP)
	// Give daemon time to reload config
	<-time.After(1 * time.Second)

	// Run with default runtime
	out, err = s.d.Cmd("run", "--rm", "--runtime=runc", "busybox", "ls")
	c.Assert(err, check.IsNil, check.Commentf(out))

	// Run with "oci"
	out, err = s.d.Cmd("run", "--rm", "--runtime=oci", "busybox", "ls")
	c.Assert(err, check.NotNil, check.Commentf(out))
	c.Assert(out, checker.Contains, "Unknown runtime specified oci")

	// Start previously created container with oci
	out, err = s.d.Cmd("start", "oci-runtime-ls")
	c.Assert(err, check.NotNil, check.Commentf(out))
	c.Assert(out, checker.Contains, "Unknown runtime specified oci")

	// Check that we can't override the default runtime
	config = `
{
    "runtimes": {
        "runc": {
            "path": "my-runc"
        }
    }
}
`
	ioutil.WriteFile(configName, []byte(config), 0644)
	syscall.Kill(s.d.cmd.Process.Pid, syscall.SIGHUP)
	// Give daemon time to reload config
	<-time.After(1 * time.Second)

	content, _ := ioutil.ReadFile(s.d.logFile.Name())
	c.Assert(string(content), checker.Contains, `file configuration validation failed (runtime name 'runc' is reserved)`)

	// Check that we can select a default runtime
	config = `
{
    "default-runtime": "vm",
    "runtimes": {
        "oci": {
            "path": "docker-runc"
        },
        "vm": {
            "path": "/usr/local/bin/vm-manager",
            "runtimeArgs": [
                "--debug"
            ]
        }
    }
}
`
	ioutil.WriteFile(configName, []byte(config), 0644)
	syscall.Kill(s.d.cmd.Process.Pid, syscall.SIGHUP)
	// Give daemon time to reload config
	<-time.After(1 * time.Second)

	out, err = s.d.Cmd("run", "--rm", "busybox", "ls")
	c.Assert(err, check.NotNil, check.Commentf(out))
	c.Assert(out, checker.Contains, "/usr/local/bin/vm-manager: no such file or directory")

	// Run with default runtime explicitly
	out, err = s.d.Cmd("run", "--rm", "--runtime=runc", "busybox", "ls")
	c.Assert(err, check.IsNil, check.Commentf(out))
}

func (s *DockerDaemonSuite) TestRunWithRuntimeFromCommandLine(c *check.C) {
	err := s.d.StartWithBusybox("--add-runtime", "oci=docker-runc", "--add-runtime", "vm=/usr/local/bin/vm-manager")
	c.Assert(err, check.IsNil)

	// Run with default runtime
	out, err := s.d.Cmd("run", "--rm", "busybox", "ls")
	c.Assert(err, check.IsNil, check.Commentf(out))

	// Run with default runtime explicitly
	out, err = s.d.Cmd("run", "--rm", "--runtime=runc", "busybox", "ls")
	c.Assert(err, check.IsNil, check.Commentf(out))

	// Run with oci (same path as default) but keep it around
	out, err = s.d.Cmd("run", "--name", "oci-runtime-ls", "--runtime=oci", "busybox", "ls")
	c.Assert(err, check.IsNil, check.Commentf(out))

	// Run with "vm"
	out, err = s.d.Cmd("run", "--rm", "--runtime=vm", "busybox", "ls")
	c.Assert(err, check.NotNil, check.Commentf(out))
	c.Assert(out, checker.Contains, "/usr/local/bin/vm-manager: no such file or directory")

	// Start a daemon without any extra runtimes
	s.d.Stop()
	err = s.d.StartWithBusybox()
	c.Assert(err, check.IsNil)

	// Run with default runtime
	out, err = s.d.Cmd("run", "--rm", "--runtime=runc", "busybox", "ls")
	c.Assert(err, check.IsNil, check.Commentf(out))

	// Run with "oci"
	out, err = s.d.Cmd("run", "--rm", "--runtime=oci", "busybox", "ls")
	c.Assert(err, check.NotNil, check.Commentf(out))
	c.Assert(out, checker.Contains, "Unknown runtime specified oci")

	// Start previously created container with oci
	out, err = s.d.Cmd("start", "oci-runtime-ls")
	c.Assert(err, check.NotNil, check.Commentf(out))
	c.Assert(out, checker.Contains, "Unknown runtime specified oci")

	// Check that we can't override the default runtime
	s.d.Stop()
	err = s.d.Start("--add-runtime", "runc=my-runc")
	c.Assert(err, check.NotNil)

	content, _ := ioutil.ReadFile(s.d.logFile.Name())
	c.Assert(string(content), checker.Contains, `runtime name 'runc' is reserved`)

	// Check that we can select a default runtime
	s.d.Stop()
	err = s.d.StartWithBusybox("--default-runtime=vm", "--add-runtime", "oci=docker-runc", "--add-runtime", "vm=/usr/local/bin/vm-manager")
	c.Assert(err, check.IsNil)

	out, err = s.d.Cmd("run", "--rm", "busybox", "ls")
	c.Assert(err, check.NotNil, check.Commentf(out))
	c.Assert(out, checker.Contains, "/usr/local/bin/vm-manager: no such file or directory")

	// Run with default runtime explicitly
	out, err = s.d.Cmd("run", "--rm", "--runtime=runc", "busybox", "ls")
	c.Assert(err, check.IsNil, check.Commentf(out))
}

func (s *DockerDaemonSuite) TestDaemonRestartWithAutoRemoveContainer(c *check.C) {
	err := s.d.StartWithBusybox()
	c.Assert(err, checker.IsNil)

	// top1 will exist after daemon restarts
	out, err := s.d.Cmd("run", "-d", "--name", "top1", "busybox:latest", "top")
	c.Assert(err, checker.IsNil, check.Commentf("run top1: %v", out))
	// top2 will be removed after daemon restarts
	out, err = s.d.Cmd("run", "-d", "--rm", "--name", "top2", "busybox:latest", "top")
	c.Assert(err, checker.IsNil, check.Commentf("run top2: %v", out))

	out, err = s.d.Cmd("ps")
	c.Assert(out, checker.Contains, "top1", check.Commentf("top1 should be running"))
	c.Assert(out, checker.Contains, "top2", check.Commentf("top2 should be running"))

	// now restart daemon gracefully
	err = s.d.Restart()
	c.Assert(err, checker.IsNil)

	out, err = s.d.Cmd("ps", "-a")
	c.Assert(err, checker.IsNil, check.Commentf("out: %v", out))
	c.Assert(out, checker.Contains, "top1", check.Commentf("top1 should exist after daemon restarts"))
	c.Assert(out, checker.Not(checker.Contains), "top2", check.Commentf("top2 should be removed after daemon restarts"))
}

func (s *DockerDaemonSuite) TestDaemonRestartSaveContainerExitCode(c *check.C) {
	err := s.d.StartWithBusybox()
	c.Assert(err, checker.IsNil)

	containerName := "error-values"
	// Make a container with both a non 0 exit code and an error message
	out, err := s.d.Cmd("run", "--name", containerName, "busybox", "toto")
	c.Assert(err, checker.NotNil)

	// Check that those values were saved on disk
	out, err = s.d.Cmd("inspect", "-f", "{{.State.ExitCode}}", containerName)
	out = strings.TrimSpace(out)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Equals, "127")

	out, err = s.d.Cmd("inspect", "-f", "{{.State.Error}}", containerName)
	out = strings.TrimSpace(out)
	c.Assert(err, checker.IsNil)

	// now restart daemon
	err = s.d.Restart()
	c.Assert(err, checker.IsNil)

	// Check that those values are still around
	out, err = s.d.Cmd("inspect", "-f", "{{.State.ExitCode}}", containerName)
	out = strings.TrimSpace(out)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Equals, "127")

	out, err = s.d.Cmd("inspect", "-f", "{{.State.Error}}", containerName)
	out = strings.TrimSpace(out)
	c.Assert(err, checker.IsNil)
}

func (s *DockerDaemonSuite) TestDaemonBackcompatPre17Volumes(c *check.C) {
	testRequires(c, SameHostDaemon)
	d := s.d
	err := d.StartWithBusybox()
	c.Assert(err, checker.IsNil)

	// hack to be able to side-load a container config
	out, err := d.Cmd("create", "busybox:latest")
	c.Assert(err, checker.IsNil, check.Commentf(out))
	id := strings.TrimSpace(out)

	out, err = d.Cmd("inspect", "--type=image", "--format={{.ID}}", "busybox:latest")
	c.Assert(err, checker.IsNil, check.Commentf(out))
	c.Assert(d.Stop(), checker.IsNil)
	<-d.wait

	imageID := strings.TrimSpace(out)
	volumeID := stringid.GenerateNonCryptoID()
	vfsPath := filepath.Join(d.root, "vfs", "dir", volumeID)
	c.Assert(os.MkdirAll(vfsPath, 0755), checker.IsNil)

	config := []byte(`
		{
			"ID": "` + id + `",
			"Name": "hello",
			"Driver": "` + d.storageDriver + `",
			"Image": "` + imageID + `",
			"Config": {"Image": "busybox:latest"},
			"NetworkSettings": {},
			"Volumes": {
				"/bar":"/foo",
				"/foo": "` + vfsPath + `",
				"/quux":"/quux"
			},
			"VolumesRW": {
				"/bar": true,
				"/foo": true,
				"/quux": false
			}
		}
	`)

	configPath := filepath.Join(d.root, "containers", id, "config.v2.json")
	err = ioutil.WriteFile(configPath, config, 600)
	err = d.Start()
	c.Assert(err, checker.IsNil)

	out, err = d.Cmd("inspect", "--type=container", "--format={{ json .Mounts }}", id)
	c.Assert(err, checker.IsNil, check.Commentf(out))
	type mount struct {
		Name        string
		Source      string
		Destination string
		Driver      string
		RW          bool
	}

	ls := []mount{}
	err = json.NewDecoder(strings.NewReader(out)).Decode(&ls)
	c.Assert(err, checker.IsNil)

	expected := []mount{
		{Source: "/foo", Destination: "/bar", RW: true},
		{Name: volumeID, Destination: "/foo", RW: true},
		{Source: "/quux", Destination: "/quux", RW: false},
	}
	c.Assert(ls, checker.HasLen, len(expected))

	for _, m := range ls {
		var matched bool
		for _, x := range expected {
			if m.Source == x.Source && m.Destination == x.Destination && m.RW == x.RW || m.Name != x.Name {
				matched = true
				break
			}
		}
		c.Assert(matched, checker.True, check.Commentf("did find match for %+v", m))
	}
}

func (s *DockerDaemonSuite) TestDaemonWithUserlandProxyPath(c *check.C) {
	testRequires(c, SameHostDaemon, DaemonIsLinux)

	dockerProxyPath, err := exec.LookPath("docker-proxy")
	c.Assert(err, checker.IsNil)
	tmpDir, err := ioutil.TempDir("", "test-docker-proxy")
	c.Assert(err, checker.IsNil)

	newProxyPath := filepath.Join(tmpDir, "docker-proxy")
	cmd := exec.Command("cp", dockerProxyPath, newProxyPath)
	c.Assert(cmd.Run(), checker.IsNil)

	// custom one
	c.Assert(s.d.StartWithBusybox("--userland-proxy-path", newProxyPath), checker.IsNil)
	out, err := s.d.Cmd("run", "-p", "5000:5000", "busybox:latest", "true")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	// try with the original one
	c.Assert(s.d.Restart("--userland-proxy-path", dockerProxyPath), checker.IsNil)
	out, err = s.d.Cmd("run", "-p", "5000:5000", "busybox:latest", "true")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	// not exist
	c.Assert(s.d.Restart("--userland-proxy-path", "/does/not/exist"), checker.IsNil)
	out, err = s.d.Cmd("run", "-p", "5000:5000", "busybox:latest", "true")
	c.Assert(err, checker.NotNil, check.Commentf(out))
	c.Assert(out, checker.Contains, "driver failed programming external connectivity on endpoint")
	c.Assert(out, checker.Contains, "/does/not/exist: no such file or directory")
}

// Test case for #22471
func (s *DockerDaemonSuite) TestDaemonShutdownTimeout(c *check.C) {
	testRequires(c, SameHostDaemon)

	c.Assert(s.d.StartWithBusybox("--shutdown-timeout=3"), check.IsNil)

	_, err := s.d.Cmd("run", "-d", "busybox", "top")
	c.Assert(err, check.IsNil)

	syscall.Kill(s.d.cmd.Process.Pid, syscall.SIGINT)

	select {
	case <-s.d.wait:
	case <-time.After(5 * time.Second):
	}

	expectedMessage := `level=debug msg="start clean shutdown of all containers with a 3 seconds timeout..."`
	content, _ := ioutil.ReadFile(s.d.logFile.Name())
	c.Assert(string(content), checker.Contains, expectedMessage)
}

// Test case for #22471
func (s *DockerDaemonSuite) TestDaemonShutdownTimeoutWithConfigFile(c *check.C) {
	testRequires(c, SameHostDaemon)

	// daemon config file
	configFilePath := "test.json"
	configFile, err := os.Create(configFilePath)
	c.Assert(err, checker.IsNil)
	defer os.Remove(configFilePath)

	daemonConfig := `{ "shutdown-timeout" : 8 }`
	fmt.Fprintf(configFile, "%s", daemonConfig)
	configFile.Close()
	c.Assert(s.d.Start(fmt.Sprintf("--config-file=%s", configFilePath)), check.IsNil)

	configFile, err = os.Create(configFilePath)
	c.Assert(err, checker.IsNil)
	daemonConfig = `{ "shutdown-timeout" : 5 }`
	fmt.Fprintf(configFile, "%s", daemonConfig)
	configFile.Close()

	syscall.Kill(s.d.cmd.Process.Pid, syscall.SIGHUP)

	select {
	case <-s.d.wait:
	case <-time.After(3 * time.Second):
	}

	expectedMessage := `level=debug msg="Reset Shutdown Timeout: 5"`
	content, _ := ioutil.ReadFile(s.d.logFile.Name())
	c.Assert(string(content), checker.Contains, expectedMessage)
}

// Test case for 29342
func (s *DockerDaemonSuite) TestExecWithUserAfterLiveRestore(c *check.C) {
	testRequires(c, DaemonIsLinux)
	s.d.StartWithBusybox("--live-restore")

	out, err := s.d.Cmd("run", "-d", "--name=top", "busybox", "sh", "-c", "addgroup -S test && adduser -S -G test test -D -s /bin/sh && top")
	c.Assert(err, check.IsNil, check.Commentf("Output: %s", out))

	s.d.waitRun("top")

	out1, err := s.d.Cmd("exec", "-u", "test", "top", "id")
	// uid=100(test) gid=101(test) groups=101(test)
	c.Assert(err, check.IsNil, check.Commentf("Output: %s", out1))

	// restart daemon.
	s.d.Restart("--live-restore")

	out2, err := s.d.Cmd("exec", "-u", "test", "top", "id")
	c.Assert(err, check.IsNil, check.Commentf("Output: %s", out2))
	c.Assert(out1, check.Equals, out2, check.Commentf("Output: before restart '%s', after restart '%s'", out1, out2))

	out, err = s.d.Cmd("stop", "top")
	c.Assert(err, check.IsNil, check.Commentf("Output: %s", out))
}

func (s *DockerDaemonSuite) TestRemoveContainerAfterLiveRestore(c *check.C) {
	testRequires(c, DaemonIsLinux, overlayFSSupported, SameHostDaemon)
	s.d.StartWithBusybox("--live-restore", "--storage-driver", "overlay")
	out, err := s.d.Cmd("run", "-d", "--name=top", "busybox", "top")
	c.Assert(err, check.IsNil, check.Commentf("Output: %s", out))

	s.d.waitRun("top")

	// restart daemon.
	s.d.Restart("--live-restore", "--storage-driver", "overlay")

	out, err = s.d.Cmd("stop", "top")
	c.Assert(err, check.IsNil, check.Commentf("Output: %s", out))

	// test if the rootfs mountpoint still exist
	mountpoint, err := s.d.inspectFilter("top", ".GraphDriver.Data.MergedDir")
	c.Assert(err, check.IsNil)
	f, err := os.Open("/proc/self/mountinfo")
	c.Assert(err, check.IsNil)
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.Contains(line, mountpoint) {
			c.Fatalf("mountinfo should not include the mountpoint of stop container")
		}
	}

	out, err = s.d.Cmd("rm", "top")
	c.Assert(err, check.IsNil, check.Commentf("Output: %s", out))

}
