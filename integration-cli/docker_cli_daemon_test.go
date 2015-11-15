// +build daemon,!windows

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/libnetwork/iptables"
	"github.com/docker/libtrust"
	"github.com/go-check/check"
)

func (s *DockerDaemonSuite) TestDaemonRestartWithRunningContainersPorts(c *check.C) {
	c.Assert(s.d.StartWithBusybox(), checker.IsNil, check.Commentf("Could not start daemon with busybox"))

	out, err := s.d.Cmd("run", "-d", "--name", "top1", "-p", "1234:80", "--restart", "always", "busybox:latest", "top")
	c.Assert(err, checker.IsNil, check.Commentf("Could not run top1: %s", out))

	// --restart=no by default
	out, err = s.d.Cmd("run", "-d", "--name", "top2", "-p", "80", "busybox:latest", "top")
	c.Assert(err, checker.IsNil, check.Commentf("Could not run top2: %s", out))

	testRun := func(m map[string]bool, prefix string) {
		var format string
		for cont, shouldRun := range m {
			out, err := s.d.Cmd("ps")
			c.Assert(err, checker.IsNil, check.Commentf("Could not run ps: %q", out))
			if shouldRun {
				format = "%s container %q is not running"
			} else {
				format = "%s container %q is running"
			}
			c.Assert(strings.Contains(out, cont), checker.Equals, shouldRun, check.Commentf(format, prefix, cont))
		}
	}

	testRun(map[string]bool{"top1": true, "top2": true}, "")

	c.Assert(s.d.Restart(), checker.IsNil, check.Commentf("Could not restart daemon"))

	testRun(map[string]bool{"top1": true, "top2": false}, "After daemon restart: ")
}

func (s *DockerDaemonSuite) TestDaemonRestartWithVolumesRefs(c *check.C) {
	c.Assert(s.d.StartWithBusybox(), checker.IsNil, check.Commentf("Could not start daemon with busybox"))

	out, err := s.d.Cmd("run", "-d", "--name", "volrestarttest1", "-v", "/foo", "busybox")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	c.Assert(s.d.Restart(), checker.IsNil)

	out, err = s.d.Cmd("run", "-d", "--volumes-from", "volrestarttest1", "--name", "volrestarttest2", "busybox", "top")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	out, err = s.d.Cmd("rm", "-fv", "volrestarttest2")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	out, err = s.d.Cmd("inspect", "-f", "{{json .Mounts}}", "volrestarttest1")
	c.Assert(err, checker.IsNil)

	_, err = inspectMountPointJSON(out, "/foo")
	c.Assert(err, checker.IsNil, check.Commentf("Expected volume to exist: /foo"))
}

// #11008
func (s *DockerDaemonSuite) TestDaemonRestartUnlessStopped(c *check.C) {
	c.Assert(s.d.StartWithBusybox(), checker.IsNil, check.Commentf("Could not start daemon with busybox"))

	out, err := s.d.Cmd("run", "-d", "--name", "top1", "--restart", "always", "busybox:latest", "top")
	c.Assert(err, checker.IsNil, check.Commentf("run top1: %v", out))

	out, err = s.d.Cmd("run", "-d", "--name", "top2", "--restart", "unless-stopped", "busybox:latest", "top")
	c.Assert(err, checker.IsNil, check.Commentf("run top2: %v", out))

	testRun := func(m map[string]bool, prefix string) {
		var format string
		for name, shouldRun := range m {
			out, err := s.d.Cmd("ps")
			c.Assert(err, checker.IsNil, check.Commentf("run ps: %v", out))
			if shouldRun {
				format = "%s container %q is not running"
			} else {
				format = "%s container %q is running"
			}
			c.Assert(strings.Contains(out, name), checker.Equals, shouldRun, check.Commentf(format, prefix, name))
		}
	}

	// both running
	testRun(map[string]bool{"top1": true, "top2": true}, "")

	out, err = s.d.Cmd("stop", "top1")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	out, err = s.d.Cmd("stop", "top2")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	// both stopped
	testRun(map[string]bool{"top1": false, "top2": false}, "")

	err = s.d.Restart()
	c.Assert(err, checker.IsNil)

	// restart=always running
	testRun(map[string]bool{"top1": true, "top2": false}, "After daemon restart: ")

	out, err = s.d.Cmd("start", "top2")
	c.Assert(err, checker.IsNil, check.Commentf("start top2: %v", out))

	err = s.d.Restart()
	c.Assert(err, checker.IsNil)

	// both running
	testRun(map[string]bool{"top1": true, "top2": true}, "After second daemon restart: ")

}

func (s *DockerDaemonSuite) TestDaemonStartIptablesFalse(c *check.C) {
	c.Assert(s.d.Start("--iptables=false"), checker.IsNil, check.Commentf("we should have been able to start the daemon with passing iptables=false"))
}

// Issue #8444: If docker0 bridge is modified (intentionally or unintentionally) and
// no longer has an IP associated, we should gracefully handle that case and associate
// an IP with it rather than fail daemon start
func (s *DockerDaemonSuite) TestDaemonStartBridgeWithoutIPAssociation(c *check.C) {
	// rather than depending on brctl commands to verify docker0 is created and up
	// let's start the daemon and stop it, and then make a modification to run the
	// actual test
	c.Assert(s.d.Start(), checker.IsNil, check.Commentf("Could not start daemon"))
	c.Assert(s.d.Stop(), checker.IsNil, check.Commentf("Could not stop daemon"))

	// now we will remove the ip from docker0 and then try starting the daemon
	ipCmd := exec.Command("ip", "addr", "flush", "dev", "docker0")
	stdout, stderr, _, err := runCommandWithStdoutStderr(ipCmd)
	c.Assert(err, checker.IsNil, check.Commentf("failed to remove docker0 IP association: stdout: %q, stderr: %q", stdout, stderr))

	warning := "**WARNING: Docker bridge network in bad state--delete docker0 bridge interface to fix"
	c.Assert(s.d.Start(), checker.IsNil, check.Commentf("Could not start daemon when docker0 has no IP address: %s", warning))
}

func (s *DockerDaemonSuite) TestDaemonIptablesClean(c *check.C) {
	c.Assert(s.d.StartWithBusybox(), checker.IsNil, check.Commentf("Could not start daemon with busybox"))

	out, err := s.d.Cmd("run", "-d", "--name", "top", "-p", "80", "busybox:latest", "top")
	c.Assert(err, checker.IsNil, check.Commentf("Could not run top: %s", out))

	// get output from iptables with container running
	ipTablesSearchString := "tcp dpt:80"
	ipTablesCmd := exec.Command("iptables", "-nvL")
	out, _, err = runCommandWithOutput(ipTablesCmd)
	c.Assert(err, checker.IsNil, check.Commentf("Could not run iptables -nvL: %s", out))

	c.Assert(out, checker.Contains, ipTablesSearchString)

	c.Assert(s.d.Stop(), checker.IsNil, check.Commentf("could not stop daemon"))

	// get output from iptables after restart
	ipTablesCmd = exec.Command("iptables", "-nvL")
	out, _, err = runCommandWithOutput(ipTablesCmd)
	c.Assert(err, checker.IsNil, check.Commentf("Could not run iptables -nvL: %s", out))

	c.Assert(out, checker.Not(checker.Contains), ipTablesSearchString)
}

func (s *DockerDaemonSuite) TestDaemonIptablesCreate(c *check.C) {
	c.Assert(s.d.StartWithBusybox(), checker.IsNil, check.Commentf("Could not start daemon with busybox"))

	out, err := s.d.Cmd("run", "-d", "--name", "top", "--restart=always", "-p", "80", "busybox:latest", "top")
	c.Assert(err, checker.IsNil, check.Commentf("Could not run top: %s", out))

	// get output from iptables with container running
	ipTablesSearchString := "tcp dpt:80"
	ipTablesCmd := exec.Command("iptables", "-nvL")
	out, _, err = runCommandWithOutput(ipTablesCmd)
	c.Assert(err, checker.IsNil, check.Commentf("Could not run iptables -nvL: %s", out))

	c.Assert(out, checker.Contains, ipTablesSearchString)

	c.Assert(s.d.Restart(), checker.IsNil)

	// make sure the container is not running
	runningOut, err := s.d.Cmd("inspect", "--format='{{.State.Running}}'", "top")
	c.Assert(err, checker.IsNil, check.Commentf("Could not inspect on container: %s", out))

	c.Assert(strings.TrimSpace(runningOut), checker.Equals, "true", check.Commentf("Container should have been restarted after daemon restart. Status running should have been true."))

	// get output from iptables after restart
	ipTablesCmd = exec.Command("iptables", "-nvL")
	out, _, err = runCommandWithOutput(ipTablesCmd)
	c.Assert(err, checker.IsNil, check.Commentf("Could not run iptables -nvL: %s", out))

	c.Assert(out, checker.Contains, ipTablesSearchString)
}

// TestDaemonIPv6Enabled checks that when the daemon is started with --ipv6=true that the docker0 bridge
// has the fe80::1 address and that a container is assigned a link-local address
func (s *DockerSuite) TestDaemonIPv6Enabled(c *check.C) {
	testRequires(c, IPv6)

	setupV6(c)

	d := NewDaemon(c)

	c.Assert(d.StartWithBusybox("--ipv6"), checker.IsNil)
	defer d.Stop()

	iface, err := net.InterfaceByName("docker0")
	c.Assert(err, checker.IsNil, check.Commentf("Error getting docker0 interface"))

	addrs, err := iface.Addrs()
	c.Assert(err, checker.IsNil, check.Commentf("Error getting addresses for docker0 interface"))

	var found bool
	expected := "fe80::1/64"

	for i := range addrs {
		if addrs[i].String() == expected {
			found = true
		}
	}

	c.Assert(found, checker.True, check.Commentf("Bridge does not have an IPv6 Address"))

	out, err := d.Cmd("run", "-itd", "--name=ipv6test", "busybox:latest")
	c.Assert(err, checker.IsNil, check.Commentf("Could not run container: %s", out))

	out, err = d.Cmd("inspect", "--format", "'{{.NetworkSettings.Networks.bridge.LinkLocalIPv6Address}}'", "ipv6test")
	out = strings.Trim(out, " \r\n'")

	c.Assert(err, checker.IsNil, check.Commentf("Error inspecting container: %s", out))

	ip := net.ParseIP(out)
	c.Assert(ip, checker.NotNil, check.Commentf("Container should have a link-local IPv6 address"))

	out, err = d.Cmd("inspect", "--format", "'{{.NetworkSettings.Networks.bridge.GlobalIPv6Address}}'", "ipv6test")
	out = strings.Trim(out, " \r\n'")

	c.Assert(err, checker.IsNil, check.Commentf("Error inspecting container: %s", out))

	ip = net.ParseIP(out)
	c.Assert(ip, checker.IsNil, check.Commentf("Container should not have a global IPv6 address: %v", out))

	teardownV6(c)

}

// TestDaemonIPv6FixedCIDR checks that when the daemon is started with --ipv6=true and a fixed CIDR
// that running containers are given a link-local and global IPv6 address
func (s *DockerSuite) TestDaemonIPv6FixedCIDR(c *check.C) {
	testRequires(c, IPv6)

	setupV6(c)

	d := NewDaemon(c)

	c.Assert(d.StartWithBusybox("--ipv6", "--fixed-cidr-v6='2001:db8:1::/64'"), checker.IsNil, check.Commentf("Could not start daemon with busybox"))
	defer d.Stop()

	out, err := d.Cmd("run", "-itd", "--name=ipv6test", "busybox:latest")
	c.Assert(err, checker.IsNil, check.Commentf("Could not run container: %s", out))

	out, err = d.Cmd("inspect", "--format", "'{{.NetworkSettings.Networks.bridge.LinkLocalIPv6Address}}'", "ipv6test")
	c.Assert(err, checker.IsNil, check.Commentf("Error inspecting container: %s", out))

	out = strings.Trim(out, " \r\n'")
	ip := net.ParseIP(out)
	c.Assert(ip, checker.NotNil, check.Commentf("Container should have a link-local IPv6 address"))

	out, err = d.Cmd("inspect", "--format", "'{{.NetworkSettings.Networks.bridge.GlobalIPv6Address}}'", "ipv6test")
	out = strings.Trim(out, " \r\n'")

	c.Assert(err, checker.IsNil, check.Commentf("Error inspecting container: %s", out))

	ip = net.ParseIP(out)
	c.Assert(ip, checker.NotNil, check.Commentf("Container should have a global IPv6 address"))

	teardownV6(c)
}

// TestDaemonIPv6FixedCIDRAndMac checks that when the daemon is started with ipv6 fixed CIDR
// the running containers are given a an IPv6 address derived from the MAC address and the ipv6 fixed CIDR
func (s *DockerSuite) TestDaemonIPv6FixedCIDRAndMac(c *check.C) {
	setupV6(c)

	d := NewDaemon(c)

	err := d.StartWithBusybox("--ipv6", "--fixed-cidr-v6='2001:db8:1::/64'")
	c.Assert(err, checker.IsNil)
	defer d.Stop()

	out, err := d.Cmd("run", "-itd", "--name=ipv6test", "--mac-address", "AA:BB:CC:DD:EE:FF", "busybox")
	c.Assert(err, checker.IsNil)

	out, err = d.Cmd("inspect", "--format", "'{{.NetworkSettings.Networks.bridge.GlobalIPv6Address}}'", "ipv6test")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.Trim(out, " \r\n'"), checker.Equals, "2001:db8:1::aabb:ccdd:eeff")

	teardownV6(c)
}

func (s *DockerDaemonSuite) TestDaemonLogLevelWrong(c *check.C) {
	c.Assert(s.d.Start("--log-level=bogus"), check.NotNil, check.Commentf("Daemon shouldn't start with wrong log level"))
}

func (s *DockerSuite) TestDaemonStartWithBackwardCompatibility(c *check.C) {

	var validCommandArgs = [][]string{
		{"--selinux-enabled", "-l", "info"},
		{"--insecure-registry", "daemon"},
	}

	var invalidCommandArgs = [][]string{
		{"--selinux-enabled", "--storage-opt"},
		{"-D", "-b"},
		{"--config", "/tmp"},
	}

	for _, args := range validCommandArgs {
		d := NewDaemon(c)
		d.Command = "--daemon"
		c.Assert(d.Start(args...), checker.IsNil, check.Commentf("Daemon should have started successfully with --daemon %v", args))
		c.Assert(d.Stop(), checker.IsNil, check.Commentf("Daemon should have stopped successfully with --daemon %v", args))
	}

	for _, args := range invalidCommandArgs {
		d := NewDaemon(c)
		if err := d.Start(args...); err == nil {
			d.Stop()
			c.Fatalf("Daemon should have failed to start with %v", args)
		}
	}
}

func (s *DockerSuite) TestDaemonStartWithDaemonCommand(c *check.C) {

	type kind int

	const (
		common kind = iota
		daemon
	)

	var flags = []map[kind][]string{
		{common: {"-l", "info"}, daemon: {"--selinux-enabled"}},
		{common: {"-D"}, daemon: {"--selinux-enabled", "-r"}},
		{common: {"-D"}, daemon: {"--restart"}},
		{common: {"--debug"}, daemon: {"--log-driver=json-file", "--log-opt=max-size=1k"}},
	}

	var invalidGlobalFlags = [][]string{
		//Invalid because you cannot pass daemon flags as global flags.
		{"--selinux-enabled", "-l", "info"},
		{"-D", "-r"},
		{"--config", "/tmp"},
	}

	// `docker daemon -l info --selinux-enabled`
	// should NOT error out
	for _, f := range flags {
		d := NewDaemon(c)
		args := append(f[common], f[daemon]...)
		c.Assert(d.Start(args...), checker.IsNil, check.Commentf("Daemon should have started successfully with %v", args))
		c.Assert(d.Stop(), checker.IsNil, check.Commentf("Daemon should have stopped successfully with %v", args))
	}

	// `docker -l info daemon --selinux-enabled`
	// should error out
	for _, f := range flags {
		d := NewDaemon(c)
		d.GlobalFlags = f[common]
		if err := d.Start(f[daemon]...); err == nil {
			d.Stop()
			c.Fatalf("Daemon should have failed to start with docker %v daemon %v", d.GlobalFlags, f[daemon])
		}
	}

	for _, f := range invalidGlobalFlags {
		cmd := exec.Command(dockerBinary, append(f, "daemon")...)
		errch := make(chan error)
		var err error
		go func() {
			errch <- cmd.Run()
		}()
		select {
		case <-time.After(time.Second):
			cmd.Process.Kill()
		case err = <-errch:
		}
		c.Assert(err, checker.NotNil, check.Commentf("Daemon should have failed to start with docker %v daemon", f))
	}
}

func (s *DockerDaemonSuite) TestDaemonLogLevelDebug(c *check.C) {
	c.Assert(s.d.Start("--log-level=debug"), checker.IsNil)

	content, _ := ioutil.ReadFile(s.d.logFile.Name())
	c.Assert(string(content), checker.Contains, `level=debug`, check.Commentf(`Should have level="debug" in log file using --log-level=debug`))
}

func (s *DockerDaemonSuite) TestDaemonLogLevelFatal(c *check.C) {
	// we creating new daemons to create new logFile
	c.Assert(s.d.Start("--log-level=fatal"), checker.IsNil)

	content, _ := ioutil.ReadFile(s.d.logFile.Name())
	c.Assert(string(content), checker.Not(checker.Contains), `level=debug`, check.Commentf(`Should not have level="debug" in log file`))
}

func (s *DockerDaemonSuite) TestDaemonFlagD(c *check.C) {
	c.Assert(s.d.Start("-D"), checker.IsNil)

	content, _ := ioutil.ReadFile(s.d.logFile.Name())
	c.Assert(string(content), checker.Contains, `level=debug`, check.Commentf(`Should have level="debug" in log file using -D`))
}

func (s *DockerDaemonSuite) TestDaemonFlagDebug(c *check.C) {
	c.Assert(s.d.Start("--debug"), checker.IsNil)

	content, _ := ioutil.ReadFile(s.d.logFile.Name())
	c.Assert(string(content), checker.Contains, `level=debug`, check.Commentf(`Should have level="debug" in log file using --debug`))
}

func (s *DockerDaemonSuite) TestDaemonFlagDebugLogLevelFatal(c *check.C) {
	c.Assert(s.d.Start("--debug", "--log-level=fatal"), checker.IsNil)

	content, _ := ioutil.ReadFile(s.d.logFile.Name())
	c.Assert(string(content), checker.Contains, `level=debug`, check.Commentf(`Should have level="debug" in log file when using both --debug and --log-level=fatal`))
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

	c.Assert(s.d.StartWithBusybox(cmdArgs...), check.IsNil, check.Commentf("Could not start daemon with busybox"))

	for _, hostDirective := range listeningPorts {
		output, err := s.d.Cmd("run", "-p", fmt.Sprintf("%s:%s:80", hostDirective[1], hostDirective[2]), "busybox", "true")
		c.Assert(err, checker.NotNil, check.Commentf("Container should not start, expected port already allocated error: %q", output))
		c.Assert(output, checker.Contains, "port is already allocated")
	}
}

func (s *DockerDaemonSuite) TestDaemonKeyGeneration(c *check.C) {
	// TODO: skip or update for Windows daemon
	os.Remove("/etc/docker/key.json")
	c.Assert(s.d.Start(), checker.IsNil, check.Commentf("Could not start daemon"))
	c.Assert(s.d.Stop(), checker.IsNil)

	k, err := libtrust.LoadKeyFile("/etc/docker/key.json")
	c.Assert(err, checker.IsNil, check.Commentf("Error opening key file"))

	kid := k.KeyID()
	// Test Key ID is a valid fingerprint (e.g. QQXN:JY5W:TBXI:MK3X:GX6P:PD5D:F56N:NHCS:LVRZ:JA46:R24J:XEFF)
	c.Assert(kid, checker.HasLen, 59, check.Commentf("Bad key ID: %s", kid))
}

func (s *DockerDaemonSuite) TestDaemonKeyMigration(c *check.C) {
	// TODO: skip or update for Windows daemon
	os.Remove("/etc/docker/key.json")
	k1, err := libtrust.GenerateECP256PrivateKey()
	c.Assert(err, checker.IsNil, check.Commentf("Error generating private key"))

	err = os.MkdirAll(filepath.Join(os.Getenv("HOME"), ".docker"), 0755)
	c.Assert(err, checker.IsNil, check.Commentf("Error creating .docker directory"))

	err = libtrust.SaveKey(filepath.Join(os.Getenv("HOME"), ".docker", "key.json"), k1)
	c.Assert(err, checker.IsNil, check.Commentf("Error saving private key"))

	c.Assert(s.d.Start(), checker.IsNil, check.Commentf("Could not start daemon"))
	c.Assert(s.d.Stop(), checker.IsNil, check.Commentf("Could not stop daemon"))

	k2, err := libtrust.LoadKeyFile("/etc/docker/key.json")
	c.Assert(err, checker.IsNil, check.Commentf("Error opening key file"))

	c.Assert(k1.KeyID(), checker.Equals, k2.KeyID(), check.Commentf("Key not migrated"))
}

// GH#11320 - verify that the daemon exits on failure properly
// Note that this explicitly tests the conflict of {-b,--bridge} and {--bip} options as the means
// to get a daemon init failure; no other tests for -b/--bip conflict are therefore required
func (s *DockerDaemonSuite) TestDaemonExitOnFailure(c *check.C) {
	//attempt to start daemon with incorrect flags (we know -b and --bip conflict)
	if err := s.d.Start("--bridge", "nosuchbridge", "--bip", "1.1.1.1"); err != nil {
		//verify we got the right error
		c.Assert(err.Error(), checker.Contains, "Daemon exited and never started")
		// look in the log and make sure we got the message that daemon is shutting down
		runCmd := exec.Command("grep", "Error starting daemon", s.d.LogfileName())
		out, _, err := runCommandWithOutput(runCmd)
		c.Assert(err, checker.IsNil, check.Commentf("Expected 'Error starting daemon' message; but doesn't exist in log: %q", out))
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
	c.Assert(err, checker.IsNil, check.Commentf(out))
	defer deleteInterface(c, bridgeName)

	err = d.StartWithBusybox("--bridge", bridgeName)
	c.Assert(err, checker.IsNil)

	ipTablesSearchString := bridgeIPNet.String()
	ipTablesCmd := exec.Command("iptables", "-t", "nat", "-nvL")
	out, _, err = runCommandWithOutput(ipTablesCmd)
	c.Assert(err, checker.IsNil)

	c.Assert(out, checker.Contains, ipTablesSearchString)

	_, err = d.Cmd("run", "-d", "--name", "ExtContainer", "busybox", "top")
	c.Assert(err, checker.IsNil)

	containerIP := d.findContainerIP("ExtContainer")
	ip := net.ParseIP(containerIP)
	c.Assert(bridgeIPNet.Contains(ip), checker.True,
		check.Commentf("Container IP-Address must be in the same subnet range : %s",
			containerIP))
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
	c.Assert(err, checker.IsNil, check.Commentf(out))

	flushCmd := exec.Command("iptables", "-t", "nat", "--flush")
	out, _, err = runCommandWithOutput(flushCmd)
	c.Assert(err, checker.IsNil, check.Commentf(out))

	flushCmd = exec.Command("iptables", "--flush")
	out, _, err = runCommandWithOutput(flushCmd)
	c.Assert(err, checker.IsNil, check.Commentf(out))
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
	c.Assert(err, checker.IsNil)
	defer d.Restart()

	ifconfigSearchString := ip.String()
	ifconfigCmd := exec.Command("ifconfig", defaultNetworkBridge)
	out, _, _, err := runCommandWithStdoutStderr(ifconfigCmd)
	c.Assert(err, checker.IsNil)

	c.Assert(out, checker.Contains, ifconfigSearchString)

	ipTablesSearchString := bridgeIPNet.String()
	ipTablesCmd := exec.Command("iptables", "-t", "nat", "-nvL")
	out, _, err = runCommandWithOutput(ipTablesCmd)
	c.Assert(err, checker.IsNil)

	c.Assert(out, checker.Contains, ipTablesSearchString)

	out, err = d.Cmd("run", "-d", "--name", "test", "busybox", "top")
	c.Assert(err, checker.IsNil)

	containerIP := d.findContainerIP("test")
	ip = net.ParseIP(containerIP)
	c.Assert(bridgeIPNet.Contains(ip), checker.True,
		check.Commentf("Container IP-Address must be in the same subnet range : %s",
			containerIP))
	deleteInterface(c, defaultNetworkBridge)
}

func (s *DockerDaemonSuite) TestDaemonRestartWithBridgeIPChange(c *check.C) {
	c.Assert(s.d.Start(), checker.IsNil, check.Commentf("Could not start daemon"))
	defer s.d.Restart()
	c.Assert(s.d.Stop(), checker.IsNil, check.Commentf("Could not stop daemon"))

	// now we will change the docker0's IP and then try starting the daemon
	bridgeIP := "192.169.100.1/24"
	_, bridgeIPNet, _ := net.ParseCIDR(bridgeIP)

	ipCmd := exec.Command("ifconfig", "docker0", bridgeIP)
	stdout, stderr, _, err := runCommandWithStdoutStderr(ipCmd)
	c.Assert(err, checker.IsNil, check.Commentf("failed to change docker0's IP association: stdout: %q, stderr: %q", stdout, stderr))

	c.Assert(s.d.Start("--bip", bridgeIP), checker.IsNil, check.Commentf("Could not start daemon"))

	//check if the iptables contains new bridgeIP MASQUERADE rule
	ipTablesSearchString := bridgeIPNet.String()
	ipTablesCmd := exec.Command("iptables", "-t", "nat", "-nvL")
	out, _, err := runCommandWithOutput(ipTablesCmd)
	c.Assert(err, checker.IsNil, check.Commentf("Could not run iptables -nvL: %s", out))
	c.Assert(out, checker.Contains, ipTablesSearchString, check.Commentf("iptables output should have contained new MASQUERADE rule"))
}

func (s *DockerDaemonSuite) TestDaemonBridgeFixedCidr(c *check.C) {
	d := s.d

	bridgeName := "external-bridge"
	bridgeIP := "192.169.1.1/24"

	out, err := createInterface(c, "bridge", bridgeName, bridgeIP)
	c.Assert(err, checker.IsNil, check.Commentf(out))
	defer deleteInterface(c, bridgeName)

	args := []string{"--bridge", bridgeName, "--fixed-cidr", "192.169.1.0/30"}
	err = d.StartWithBusybox(args...)
	c.Assert(err, checker.IsNil)
	defer d.Restart()

	for i := 0; i < 4; i++ {
		cName := "Container" + strconv.Itoa(i)
		out, err := d.Cmd("run", "-d", "--name", cName, "busybox", "top")
		// If there is an error, we only allow "no available IPV4 addresses"
		if err != nil {
			c.Assert(out, checker.Contains, "no available IPv4 addresses", check.Commentf("could not run a Container : %s", err.Error()))
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
	c.Assert(err, checker.IsNil)
	defer d.Restart()

	expectedMessage := fmt.Sprintf("default via %s dev", bridgeIP)
	out, err := d.Cmd("run", "busybox", "ip", "-4", "route", "list", "0/0")
	c.Assert(out, checker.Contains, expectedMessage, check.Commentf("Implicit default gateway should be bridge IP %s, but default route was '%s'",
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
	c.Assert(err, checker.IsNil)
	defer d.Restart()

	expectedMessage := fmt.Sprintf("default via %s dev", gatewayIP)
	out, err := d.Cmd("run", "busybox", "ip", "-4", "route", "list", "0/0")
	c.Assert(out, checker.Contains, expectedMessage, check.Commentf("Explicit default gateway should be %s, but default route was '%s'",
		gatewayIP, strings.TrimSpace(out)))
	deleteInterface(c, defaultNetworkBridge)
}

func (s *DockerDaemonSuite) TestDaemonDefaultGatewayIPv4ExplicitOutsideContainerSubnet(c *check.C) {
	defaultNetworkBridge := "docker0"
	deleteInterface(c, defaultNetworkBridge)

	// Program a custom default gateway outside of the container subnet, daemon should accept it and start
	err := s.d.StartWithBusybox("--bip", "172.16.0.10/16", "--fixed-cidr", "172.16.1.0/24", "--default-gateway", "172.16.0.254")
	c.Assert(err, checker.IsNil)

	deleteInterface(c, defaultNetworkBridge)
	s.d.Restart()
}

func (s *DockerDaemonSuite) TestDaemonDefaultNetworkInvalidClusterConfig(c *check.C) {
	testRequires(c, DaemonIsLinux, SameHostDaemon)

	// Start daemon without docker0 bridge
	defaultNetworkBridge := "docker0"
	deleteInterface(c, defaultNetworkBridge)

	d := NewDaemon(c)
	discoveryBackend := "consul://consuladdr:consulport/some/path"
	err := d.Start(fmt.Sprintf("--cluster-store=%s", discoveryBackend))
	c.Assert(err, checker.IsNil)

	// Start daemon with docker0 bridge
	ifconfigCmd := exec.Command("ifconfig", defaultNetworkBridge)
	_, err = runCommand(ifconfigCmd)
	c.Assert(err, check.IsNil)

	err = d.Restart(fmt.Sprintf("--cluster-store=%s", discoveryBackend))
	c.Assert(err, checker.IsNil)

	d.Stop()
}

func (s *DockerDaemonSuite) TestDaemonIP(c *check.C) {
	d := s.d

	ipStr := "192.170.1.1/24"
	ip, _, _ := net.ParseCIDR(ipStr)
	args := []string{"--ip", ip.String()}
	err := d.StartWithBusybox(args...)
	c.Assert(err, checker.IsNil)
	defer d.Restart()

	out, err := d.Cmd("run", "-d", "-p", "8000:8000", "busybox", "top")
	c.Assert(err, check.NotNil,
		check.Commentf("Running a container must fail with an invalid --ip option"))
	c.Assert(out, checker.Contains, "Error starting userland proxy")

	ifName := "dummy"
	out, err = createInterface(c, "dummy", ifName, ipStr)
	c.Assert(err, checker.IsNil, check.Commentf(out))
	defer deleteInterface(c, ifName)

	_, err = d.Cmd("run", "-d", "-p", "8000:8000", "busybox", "top")
	c.Assert(err, checker.IsNil)

	ipTablesCmd := exec.Command("iptables", "-t", "nat", "-nvL")
	out, _, err = runCommandWithOutput(ipTablesCmd)
	c.Assert(err, checker.IsNil)

	regex := fmt.Sprintf("DNAT.*%s.*dpt:8000", ip.String())
	matched, _ := regexp.MatchString(regex, out)
	c.Assert(matched, checker.True,
		check.Commentf("iptables output should have contained %q, but was %q", regex, out))
}

func (s *DockerDaemonSuite) TestDaemonICCPing(c *check.C) {
	d := s.d

	bridgeName := "external-bridge"
	bridgeIP := "192.169.1.1/24"

	out, err := createInterface(c, "bridge", bridgeName, bridgeIP)
	c.Assert(err, checker.IsNil, check.Commentf(out))
	defer deleteInterface(c, bridgeName)

	args := []string{"--bridge", bridgeName, "--icc=false"}
	err = d.StartWithBusybox(args...)
	c.Assert(err, checker.IsNil)
	defer d.Restart()

	ipTablesCmd := exec.Command("iptables", "-nvL", "FORWARD")
	out, _, err = runCommandWithOutput(ipTablesCmd)
	c.Assert(err, checker.IsNil)

	regex := fmt.Sprintf("DROP.*all.*%s.*%s", bridgeName, bridgeName)
	matched, _ := regexp.MatchString(regex, out)
	c.Assert(matched, checker.True,
		check.Commentf("iptables output should have contained %q, but was %q", regex, out))

	// Pinging another container must fail with --icc=false
	pingContainers(c, d, true)

	ipStr := "192.171.1.1/24"
	ip, _, _ := net.ParseCIDR(ipStr)
	ifName := "icc-dummy"

	createInterface(c, "dummy", ifName, ipStr)

	// But, Pinging external or a Host interface must succeed
	pingCmd := fmt.Sprintf("ping -c 1 %s -W 1", ip.String())
	runArgs := []string{"--rm", "busybox", "sh", "-c", pingCmd}
	_, err = d.Cmd("run", runArgs...)
	c.Assert(err, checker.IsNil)
}

func (s *DockerDaemonSuite) TestDaemonICCLinkExpose(c *check.C) {
	d := s.d

	bridgeName := "external-bridge"
	bridgeIP := "192.169.1.1/24"

	out, err := createInterface(c, "bridge", bridgeName, bridgeIP)
	c.Assert(err, checker.IsNil, check.Commentf(out))
	defer deleteInterface(c, bridgeName)

	args := []string{"--bridge", bridgeName, "--icc=false"}
	err = d.StartWithBusybox(args...)
	c.Assert(err, checker.IsNil)
	defer d.Restart()

	ipTablesCmd := exec.Command("iptables", "-nvL", "FORWARD")
	out, _, err = runCommandWithOutput(ipTablesCmd)
	c.Assert(err, checker.IsNil)

	regex := fmt.Sprintf("DROP.*all.*%s.*%s", bridgeName, bridgeName)
	matched, _ := regexp.MatchString(regex, out)
	c.Assert(matched, checker.True,
		check.Commentf("iptables output should have contained %q, but was %q", regex, out))

	out, err = d.Cmd("run", "-d", "--expose", "4567", "--name", "icc1", "busybox", "nc", "-l", "-p", "4567")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	out, err = d.Cmd("run", "--link", "icc1:icc1", "busybox", "nc", "icc1", "4567")
	c.Assert(err, checker.IsNil, check.Commentf(out))
}

func (s *DockerDaemonSuite) TestDaemonLinksIpTablesRulesWhenLinkAndUnlink(c *check.C) {
	bridgeName := "external-bridge"
	bridgeIP := "192.169.1.1/24"

	out, err := createInterface(c, "bridge", bridgeName, bridgeIP)
	c.Assert(err, checker.IsNil, check.Commentf(out))
	defer deleteInterface(c, bridgeName)

	err = s.d.StartWithBusybox("--bridge", bridgeName, "--icc=false")
	c.Assert(err, checker.IsNil)
	defer s.d.Restart()

	_, err = s.d.Cmd("run", "-d", "--name", "child", "--publish", "8080:80", "busybox", "top")
	c.Assert(err, checker.IsNil)
	_, err = s.d.Cmd("run", "-d", "--name", "parent", "--link", "child:http", "busybox", "top")
	c.Assert(err, checker.IsNil)

	childIP := s.d.findContainerIP("child")
	parentIP := s.d.findContainerIP("parent")

	sourceRule := []string{"-i", bridgeName, "-o", bridgeName, "-p", "tcp", "-s", childIP, "--sport", "80", "-d", parentIP, "-j", "ACCEPT"}
	destinationRule := []string{"-i", bridgeName, "-o", bridgeName, "-p", "tcp", "-s", parentIP, "--dport", "80", "-d", childIP, "-j", "ACCEPT"}

	c.Assert(iptables.Exists("filter", "DOCKER", sourceRule...), checker.True, check.Commentf("Iptables rules not found"))
	c.Assert(iptables.Exists("filter", "DOCKER", destinationRule...), checker.True, check.Commentf("Iptables rules not found"))

	s.d.Cmd("rm", "--link", "parent/http")
	c.Assert(iptables.Exists("filter", "DOCKER", sourceRule...), checker.False, check.Commentf("Iptables rules should be removed when unlink"))
	c.Assert(iptables.Exists("filter", "DOCKER", destinationRule...), checker.False, check.Commentf("Iptables rules should be removed when unlink"))

	s.d.Cmd("kill", "child")
	s.d.Cmd("kill", "parent")
}

func (s *DockerDaemonSuite) TestDaemonUlimitDefaults(c *check.C) {
	testRequires(c, DaemonIsLinux)

	c.Assert(s.d.StartWithBusybox("--default-ulimit", "nofile=42:42", "--default-ulimit", "nproc=1024:1024"), checker.IsNil)

	out, err := s.d.Cmd("run", "--ulimit", "nproc=2048", "--name=test", "busybox", "/bin/sh", "-c", "echo $(ulimit -n); echo $(ulimit -p)")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	outArr := strings.Split(out, "\n")
	c.Assert(len(outArr), checker.GreaterOrEqualThan, 2, check.Commentf("got unexpected output: %s", out))

	nofile := strings.TrimSpace(outArr[0])
	nproc := strings.TrimSpace(outArr[1])

	c.Assert(nofile, checker.Equals, "42", check.Commentf("expected `ulimit -n` to be `42`"))
	c.Assert(nproc, checker.Equals, "2048", check.Commentf("exepcted `ulimit -p` to be 2048"))

	// Now restart daemon with a new default
	c.Assert(s.d.Restart("--default-ulimit", "nofile=43"), checker.IsNil)

	out, err = s.d.Cmd("start", "-a", "test")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	outArr = strings.Split(out, "\n")
	c.Assert(len(outArr), checker.GreaterOrEqualThan, 2, check.Commentf("got unexpected output: %s", out))

	nofile = strings.TrimSpace(outArr[0])
	nproc = strings.TrimSpace(outArr[1])

	c.Assert(nofile, checker.Equals, "43", check.Commentf("expected `ulimit -n` to be `43`"))
	c.Assert(nproc, checker.Equals, "2048", check.Commentf("exepcted `ulimit -p` to be 2048"))
}

// #11315
func (s *DockerDaemonSuite) TestDaemonRestartRenameContainer(c *check.C) {
	c.Assert(s.d.StartWithBusybox(), checker.IsNil, check.Commentf("Could not start daemon with busybox"))

	out, err := s.d.Cmd("run", "--name=test", "busybox")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	out, err = s.d.Cmd("rename", "test", "test2")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	c.Assert(s.d.Restart(), checker.IsNil)

	out, err = s.d.Cmd("start", "test2")
	c.Assert(err, checker.IsNil, check.Commentf(out))
}

func (s *DockerDaemonSuite) TestDaemonLoggingDriverDefault(c *check.C) {
	c.Assert(s.d.StartWithBusybox(), checker.IsNil, check.Commentf("Could not start daemon with busybox"))

	out, err := s.d.Cmd("run", "-d", "busybox", "echo", "testline")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	id := strings.TrimSpace(out)

	out, err = s.d.Cmd("wait", id)
	c.Assert(err, checker.IsNil, check.Commentf(out))

	logPath := filepath.Join(s.d.root, "containers", id, id+"-json.log")

	_, err = os.Stat(logPath)
	c.Assert(err, checker.IsNil)

	f, err := os.Open(logPath)
	c.Assert(err, checker.IsNil)

	var res struct {
		Log    string    `json:"log"`
		Stream string    `json:"stream"`
		Time   time.Time `json:"time"`
	}
	c.Assert(json.NewDecoder(f).Decode(&res), checker.IsNil, check.Commentf("Fail to decode %v", f))

	c.Assert(res.Log, checker.Equals, "testline\n", check.Commentf("Unexpected log line"))
	c.Assert(res.Stream, checker.Contains, "stdout", check.Commentf("Unexpected stream"))

	c.Assert(time.Now(), checker.IsAfter, res.Time, check.Commentf("Log time in future"))
}

func (s *DockerDaemonSuite) TestDaemonLoggingDriverDefaultOverride(c *check.C) {
	c.Assert(s.d.StartWithBusybox(), checker.IsNil, check.Commentf("Could not start daemon with busybox"))

	out, err := s.d.Cmd("run", "-d", "--log-driver=none", "busybox", "echo", "testline")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	id := strings.TrimSpace(out)

	out, err = s.d.Cmd("wait", id)
	c.Assert(err, checker.IsNil, check.Commentf(out))

	logPath := filepath.Join(s.d.root, "containers", id, id+"-json.log")

	_, err = os.Stat(logPath)
	c.Assert(err, checker.NotNil, check.Commentf("%s shouldn't exits", logPath))
	c.Assert(os.IsNotExist(err), checker.True, check.Commentf("%s shouldn't exits, err:%v", logPath, err))
}

func (s *DockerDaemonSuite) TestDaemonLoggingDriverNone(c *check.C) {
	c.Assert(s.d.StartWithBusybox("--log-driver=none"), checker.IsNil, check.Commentf("Could not start daemon"))

	out, err := s.d.Cmd("run", "-d", "busybox", "echo", "testline")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	id := strings.TrimSpace(out)
	out, err = s.d.Cmd("wait", id)
	c.Assert(err, checker.IsNil, check.Commentf(out))

	logPath := filepath.Join(s.d.folder, "graph", "containers", id, id+"-json.log")

	_, err = os.Stat(logPath)
	c.Assert(err, checker.NotNil, check.Commentf("%s shouldn't exits", logPath))
	c.Assert(os.IsNotExist(err), checker.True, check.Commentf("%s shouldn't exits, err:%v", logPath, err))
}

func (s *DockerDaemonSuite) TestDaemonLoggingDriverNoneOverride(c *check.C) {
	c.Assert(s.d.StartWithBusybox("--log-driver=none"), checker.IsNil, check.Commentf("Could not start daemon"))

	out, err := s.d.Cmd("run", "-d", "--log-driver=json-file", "busybox", "echo", "testline")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	id := strings.TrimSpace(out)

	s.d.Cmd("wait", id)
	c.Assert(err, checker.IsNil, check.Commentf(out))

	logPath := filepath.Join(s.d.root, "containers", id, id+"-json.log")

	_, err = os.Stat(logPath)
	c.Assert(err, checker.IsNil)

	f, err := os.Open(logPath)
	c.Assert(err, checker.IsNil)

	var res struct {
		Log    string    `json:"log"`
		Stream string    `json:"stream"`
		Time   time.Time `json:"time"`
	}

	c.Assert(json.NewDecoder(f).Decode(&res), checker.IsNil, check.Commentf("Fail to decode %v", f))
	c.Assert(res.Log, checker.Contains, "testline")

	c.Assert(res.Stream, checker.Equals, "stdout")
	c.Assert(time.Now(), checker.IsAfter, res.Time, check.Commentf("Log time in future"))
}

func (s *DockerDaemonSuite) TestDaemonLoggingDriverNoneLogsError(c *check.C) {
	c.Assert(s.d.StartWithBusybox("--log-driver=none"), checker.IsNil, check.Commentf("Could not start daemon"))

	out, err := s.d.Cmd("run", "-d", "busybox", "echo", "testline")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	id := strings.TrimSpace(out)
	out, err = s.d.Cmd("logs", id)
	c.Assert(err, checker.IsNil, check.Commentf("Logs should fail with 'none' driver"))
	c.Assert(out, checker.Contains, `"logs" command is supported only for "json-file" and "journald" logging drivers (got: none)`,
		check.Commentf("There should be an error about none not being a recognized log driver"))
}

func (s *DockerDaemonSuite) TestDaemonDots(c *check.C) {
	c.Assert(s.d.StartWithBusybox(), checker.IsNil, check.Commentf("Could not start daemon with busybox"))

	// Now create 4 containers
	_, err := s.d.Cmd("create", "busybox")
	c.Assert(err, checker.IsNil, check.Commentf("Error creating container"))
	_, err = s.d.Cmd("create", "busybox")
	c.Assert(err, checker.IsNil, check.Commentf("Error creating container"))
	_, err = s.d.Cmd("create", "busybox")
	c.Assert(err, checker.IsNil, check.Commentf("Error creating container"))
	_, err = s.d.Cmd("create", "busybox")
	c.Assert(err, checker.IsNil, check.Commentf("Error creating container"))

	s.d.Stop()

	s.d.Start("--log-level=debug")
	s.d.Stop()
	content, _ := ioutil.ReadFile(s.d.logFile.Name())
	c.Assert(string(content), checker.Not(checker.Contains), "....", check.Commentf("Debug level should have ....\n"))

	s.d.Start("--log-level=error")
	s.d.Stop()
	content, _ = ioutil.ReadFile(s.d.logFile.Name())
	c.Assert(string(content), checker.Not(checker.Contains), "....", check.Commentf("Error level should have ....\n"))

	s.d.Start("--log-level=info")
	s.d.Stop()
	content, _ = ioutil.ReadFile(s.d.logFile.Name())
	c.Assert(string(content), checker.Contains, "....", check.Commentf("Info level should have ....\n"))
}

func (s *DockerDaemonSuite) TestDaemonUnixSockCleanedUp(c *check.C) {
	dir, err := ioutil.TempDir("", "socket-cleanup-test")
	c.Assert(err, checker.IsNil)
	defer os.RemoveAll(dir)

	sockPath := filepath.Join(dir, "docker.sock")
	c.Assert(s.d.Start("--host", "unix://"+sockPath), checker.IsNil)

	_, err = os.Stat(sockPath)
	c.Assert(err, checker.IsNil, check.Commentf("socket does not exist"))

	c.Assert(s.d.Stop(), checker.IsNil)

	_, err = os.Stat(sockPath)
	c.Assert(err, checker.NotNil, check.Commentf("unix socket is not cleaned up"))
	c.Assert(os.IsNotExist(err), checker.True, check.Commentf("unix socket is not cleaned up"))
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
	c.Assert(s.d.Start(), checker.IsNil, check.Commentf("Failed to start daemon"))

	c.Assert(s.d.Stop(), checker.IsNil, check.Commentf("Could not stop daemon"))

	config := &Config{}
	bytes, err := ioutil.ReadFile("/etc/docker/key.json")
	c.Assert(err, checker.IsNil, check.Commentf("Error reading key.json file"))

	// byte[] to Data-Struct
	c.Assert(json.Unmarshal(bytes, &config), checker.IsNil, check.Commentf("Error Unmarshal"))

	//replace config.Kid with the fake value
	config.Kid = "VSAJ:FUYR:X3H2:B2VZ:KZ6U:CJD5:K7BX:ZXHY:UZXT:P4FT:MJWG:HRJ4"

	// NEW Data-Struct to byte[]
	newBytes, err := json.Marshal(&config)
	c.Assert(err, checker.IsNil, check.Commentf("Error Marshal"))

	// write back
	err = ioutil.WriteFile("/etc/docker/key.json", newBytes, 0400)
	c.Assert(err, checker.IsNil, check.Commentf("Error ioutil.WriteFile"))

	defer os.Remove("/etc/docker/key.json")

	c.Assert(s.d.Start(), checker.NotNil, check.Commentf("It should not be successful to start daemon with wrong key"))

	content, _ := ioutil.ReadFile(s.d.logFile.Name())

	c.Assert(string(content), checker.Contains, "Public Key ID does not match", check.Commentf("Missing KeyID message from daemon logs"))
}

func (s *DockerDaemonSuite) TestDaemonRestartKillWait(c *check.C) {
	c.Assert(s.d.StartWithBusybox(), checker.IsNil, check.Commentf("Could not start daemon with busybox"))

	out, err := s.d.Cmd("run", "-id", "busybox", "/bin/cat")
	c.Assert(err, checker.IsNil, check.Commentf("Could not run /bin/cat : %s", out))

	containerID := strings.TrimSpace(out)

	out, err = s.d.Cmd("kill", containerID)
	c.Assert(err, checker.IsNil, check.Commentf("Could not kill %s, %s", containerID, out))

	c.Assert(s.d.Restart(), checker.IsNil, check.Commentf("Could not restart daemon"))

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
		c.Assert(err, checker.IsNil)
	}
}

// TestHttpsInfo connects via two-way authenticated HTTPS to the info endpoint
func (s *DockerDaemonSuite) TestHttpsInfo(c *check.C) {
	const (
		testDaemonHTTPSAddr = "tcp://localhost:4271"
	)

	c.Assert(s.d.Start("--tlsverify", "--tlscacert", "fixtures/https/ca.pem", "--tlscert", "fixtures/https/server-cert.pem",
		"--tlskey", "fixtures/https/server-key.pem", "-H", testDaemonHTTPSAddr), checker.IsNil,
		check.Commentf("Could not start daemon with busybox"))

	daemonArgs := []string{"--host", testDaemonHTTPSAddr, "--tlsverify", "--tlscacert", "fixtures/https/ca.pem", "--tlscert", "fixtures/https/client-cert.pem", "--tlskey", "fixtures/https/client-key.pem"}
	out, err := s.d.CmdWithArgs(daemonArgs, "info")
	c.Assert(err, checker.IsNil, check.Commentf(out))
}

// TestTlsVerify verifies that --tlsverify=false turns on tls
func (s *DockerDaemonSuite) TestTlsVerify(c *check.C) {
	out, err := exec.Command(dockerBinary, "daemon", "--tlsverify=false").CombinedOutput()
	c.Assert(err, checker.NotNil, check.Commentf("Daemon should not have started due to missing certs: %s", string(out)))
	c.Assert(string(out), checker.Contains, "Could not load X509 key pair", check.Commentf("Daemon should not have started due to missing certs"))
}

// TestHttpsInfoRogueCert connects via two-way authenticated HTTPS to the info endpoint
// by using a rogue client certificate and checks that it fails with the expected error.
func (s *DockerDaemonSuite) TestHttpsInfoRogueCert(c *check.C) {
	const (
		errBadCertificate   = "remote error: bad certificate"
		testDaemonHTTPSAddr = "tcp://localhost:4271"
	)

	c.Assert(s.d.Start("--tlsverify", "--tlscacert", "fixtures/https/ca.pem", "--tlscert", "fixtures/https/server-cert.pem",
		"--tlskey", "fixtures/https/server-key.pem", "-H", testDaemonHTTPSAddr),
		checker.IsNil,
		check.Commentf("Could not start daemon with busybox"))

	daemonArgs := []string{"--host", testDaemonHTTPSAddr, "--tlsverify", "--tlscacert", "fixtures/https/ca.pem", "--tlscert", "fixtures/https/client-rogue-cert.pem", "--tlskey", "fixtures/https/client-rogue-key.pem"}
	out, err := s.d.CmdWithArgs(daemonArgs, "info")
	c.Assert(err, checker.NotNil, check.Commentf("Daemon should not have started"))
	c.Assert(out, checker.Contains, errBadCertificate)
}

// TestHttpsInfoRogueServerCert connects via two-way authenticated HTTPS to the info endpoint
// which provides a rogue server certificate and checks that it fails with the expected error
func (s *DockerDaemonSuite) TestHttpsInfoRogueServerCert(c *check.C) {
	const (
		errCaUnknown             = "x509: certificate signed by unknown authority"
		testDaemonRogueHTTPSAddr = "tcp://localhost:4272"
	)
	c.Assert(s.d.Start("--tlsverify", "--tlscacert", "fixtures/https/ca.pem", "--tlscert", "fixtures/https/server-rogue-cert.pem",
		"--tlskey", "fixtures/https/server-rogue-key.pem", "-H", testDaemonRogueHTTPSAddr),
		checker.IsNil,
		check.Commentf("Could not start daemon with busybox"))

	daemonArgs := []string{"--host", testDaemonRogueHTTPSAddr, "--tlsverify", "--tlscacert", "fixtures/https/ca.pem", "--tlscert", "fixtures/https/client-rogue-cert.pem", "--tlskey", "fixtures/https/client-rogue-key.pem"}
	out, err := s.d.CmdWithArgs(daemonArgs, "info")
	c.Assert(err, checker.NotNil, check.Commentf("Daemon should not have started"))
	c.Assert(out, checker.Contains, errCaUnknown)
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
		c.Assert(err, checker.IsNil)
	}

	args = append(dargs, "rm", "-f", "container1")
	dockerCmd(c, args...)
}

func (s *DockerDaemonSuite) TestDaemonRestartWithSocketAsVolume(c *check.C) {
	c.Assert(s.d.StartWithBusybox(), checker.IsNil)

	socket := filepath.Join(s.d.folder, "docker.sock")

	out, err := s.d.Cmd("run", "-d", "--restart=always", "-v", socket+":/sock", "busybox")
	c.Assert(err, checker.IsNil, check.Commentf("Output: %s", out))
	c.Assert(s.d.Restart(), checker.IsNil)
}

func (s *DockerDaemonSuite) TestCleanupMountsAfterCrash(c *check.C) {
	c.Assert(s.d.StartWithBusybox(), checker.IsNil)

	out, err := s.d.Cmd("run", "-d", "busybox", "top")
	c.Assert(err, checker.IsNil, check.Commentf("Output: %s", out))
	id := strings.TrimSpace(out)
	c.Assert(s.d.cmd.Process.Signal(os.Kill), checker.IsNil)
	c.Assert(s.d.Start(), checker.IsNil)
	mountOut, err := ioutil.ReadFile("/proc/self/mountinfo")
	c.Assert(err, checker.IsNil, check.Commentf("Output: %s", mountOut))

	comment := check.Commentf("%s is still mounted from older daemon start:\nDaemon root repository %s\n%s", id, s.d.folder, mountOut)
	c.Assert(string(mountOut), checker.Not(checker.Contains), id, comment)
}

func (s *DockerDaemonSuite) TestRunContainerWithBridgeNone(c *check.C) {
	testRequires(c, DaemonIsLinux, NotUserNamespace)
	c.Assert(s.d.StartWithBusybox("-b", "none"), checker.IsNil)

	out, err := s.d.Cmd("run", "--rm", "busybox", "ip", "l")
	c.Assert(err, checker.IsNil, check.Commentf("Output: %s", out))
	c.Assert(out, checker.Not(checker.Contains), "eth0",
		check.Commentf("There shouldn't be eth0 in container in default(bridge) mode when bridge network is disabled: %s", out))

	out, err = s.d.Cmd("run", "--rm", "--net=bridge", "busybox", "ip", "l")
	c.Assert(err, checker.IsNil, check.Commentf("Output: %s", out))
	c.Assert(out, checker.Not(checker.Contains), "eth0",
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

func (s *DockerDaemonSuite) TestDaemonRestartWithContainerRunning(c *check.C) {
	c.Assert(s.d.StartWithBusybox(), checker.IsNil, check.Commentf("Could not start daemon with busybox"))
	out, err := s.d.Cmd("run", "-ti", "-d", "--name", "test", "busybox")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	c.Assert(s.d.Restart(), checker.IsNil)

	// Container 'test' should be removed without error
	out, err = s.d.Cmd("rm", "test")
	c.Assert(err, checker.IsNil, check.Commentf(out))
}

func (s *DockerDaemonSuite) TestDaemonRestartCleanupNetns(c *check.C) {
	c.Assert(s.d.StartWithBusybox(), checker.IsNil, check.Commentf("Could not start daemon with busybox"))
	out, err := s.d.Cmd("run", "--name", "netns", "-d", "busybox", "top")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	// Get sandbox key via inspect
	out, err = s.d.Cmd("inspect", "--format", "'{{.NetworkSettings.SandboxKey}}'", "netns")
	c.Assert(err, checker.IsNil, check.Commentf("Error inspecting container: %s", out))

	fileName := strings.Trim(out, " \r\n'")

	out, err = s.d.Cmd("stop", "netns")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	// Test if the file still exists
	out, _, err = runCommandWithOutput(exec.Command("stat", "-c", "%n", fileName))
	out = strings.TrimSpace(out)
	c.Assert(err, checker.IsNil, check.Commentf("Output: %s", out))
	c.Assert(out, checker.Equals, fileName, check.Commentf("Output: %s", out))

	// Remove the container and restart the daemon
	out, err = s.d.Cmd("rm", "netns")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	c.Assert(s.d.Restart(), checker.IsNil)

	// Test again and see now the netns file does not exist
	out, _, err = runCommandWithOutput(exec.Command("stat", "-c", "%n", fileName))
	out = strings.TrimSpace(out)
	c.Assert(err, checker.NotNil, check.Commentf("Output: %s", out))
}

// tests regression detailed in #13964 where DOCKER_TLS_VERIFY env is ignored
func (s *DockerDaemonSuite) TestDaemonNoTlsCliTlsVerifyWithEnv(c *check.C) {
	host := "tcp://localhost:4271"
	c.Assert(s.d.Start("-H", host), checker.IsNil)
	cmd := exec.Command(dockerBinary, "-H", host, "info")
	cmd.Env = []string{"DOCKER_TLS_VERIFY=1", "DOCKER_CERT_PATH=fixtures/https"}
	out, _, err := runCommandWithOutput(cmd)
	c.Assert(err, checker.NotNil, check.Commentf("%s", out))
	c.Assert(out, checker.Contains, "error occurred trying to connect")

}

func setupV6(c *check.C) {
	// Hack to get the right IPv6 address on docker0, which has already been created
	err := exec.Command("ip", "addr", "add", "fe80::1/64", "dev", "docker0").Run()
	c.Assert(err, checker.IsNil, check.Commentf("Could not set up host for IPv6 tests"))
}

func teardownV6(c *check.C) {
	err := exec.Command("ip", "addr", "del", "fe80::1/64", "dev", "docker0").Run()
	c.Assert(err, checker.IsNil, check.Commentf("Could not perform teardown for IPv6 tests"))
}

func (s *DockerDaemonSuite) TestDaemonRestartWithContainerWithRestartPolicyAlways(c *check.C) {
	c.Assert(s.d.StartWithBusybox(), checker.IsNil)

	out, err := s.d.Cmd("run", "-d", "--restart", "always", "busybox", "top")
	c.Assert(err, checker.IsNil)
	id := strings.TrimSpace(out)

	_, err = s.d.Cmd("stop", id)
	c.Assert(err, checker.IsNil)
	_, err = s.d.Cmd("wait", id)
	c.Assert(err, checker.IsNil)

	out, err = s.d.Cmd("ps", "-q")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Equals, "")

	c.Assert(s.d.Restart(), checker.IsNil)

	out, err = s.d.Cmd("ps", "-q")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Equals, id[:12])
}

func (s *DockerDaemonSuite) TestDaemonWideLogConfig(c *check.C) {
	c.Assert(s.d.StartWithBusybox("--log-driver=json-file", "--log-opt=max-size=1k"), checker.IsNil)

	out, err := s.d.Cmd("run", "-d", "--name=logtest", "busybox", "top")
	c.Assert(err, checker.IsNil, check.Commentf("Output: %s, err: %v", out, err))
	out, err = s.d.Cmd("inspect", "-f", "{{ .HostConfig.LogConfig.Config }}", "logtest")
	c.Assert(err, checker.IsNil, check.Commentf("Output: %s", out))
	cfg := strings.TrimSpace(out)
	c.Assert(cfg, checker.Equals, "map[max-size:1k]", check.Commentf("Unexpected log-opt"))
}

func (s *DockerDaemonSuite) TestDaemonRestartWithPausedContainer(c *check.C) {
	c.Assert(s.d.StartWithBusybox(), checker.IsNil, check.Commentf("Could not start daemon with busybox"))
	out, err := s.d.Cmd("run", "-i", "-d", "--name", "test", "busybox", "top")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	out, err = s.d.Cmd("pause", "test")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	c.Assert(s.d.Restart(), checker.IsNil)

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
		c.Assert(err, checker.IsNil)
	}
}

func (s *DockerDaemonSuite) TestDaemonRestartRmVolumeInUse(c *check.C) {
	c.Assert(s.d.StartWithBusybox(), checker.IsNil)

	out, err := s.d.Cmd("create", "-v", "test:/foo", "busybox")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	c.Assert(s.d.Restart(), checker.IsNil)

	out, err = s.d.Cmd("volume", "rm", "test")
	c.Assert(err, checker.NotNil, check.Commentf("should not be able to remove in use volume after daemon restart"))
	c.Assert(out, checker.Contains, "in use")
}

func (s *DockerDaemonSuite) TestDaemonRestartLocalVolumes(c *check.C) {
	c.Assert(s.d.Start(), checker.IsNil)

	_, err := s.d.Cmd("volume", "create", "--name", "test")
	c.Assert(err, checker.IsNil)
	c.Assert(s.d.Restart(), checker.IsNil)

	_, err = s.d.Cmd("volume", "inspect", "test")
	c.Assert(err, checker.IsNil)
}

func (s *DockerDaemonSuite) TestDaemonCorruptedLogDriverAddress(c *check.C) {
	for _, driver := range []string{
		"syslog",
		"gelf",
	} {
		args := []string{"--log-driver=" + driver, "--log-opt", driver + "-address=corrupted:42"}
		c.Assert(s.d.Start(args...), checker.NotNil, check.Commentf(fmt.Sprintf("Expected daemon not to start with invalid %s-address provided", driver)))
		expected := fmt.Sprintf("Failed to set log opts: %s-address should be in form proto://address", driver)
		runCmd := exec.Command("grep", expected, s.d.LogfileName())
		out, _, err := runCommandWithOutput(runCmd)
		c.Assert(err, checker.IsNil, check.Commentf("Expected %q message; but doesn't exist in log: %q", expected, out))
	}
}

func (s *DockerDaemonSuite) TestDaemonCorruptedFluentdAddress(c *check.C) {
	c.Assert(s.d.Start("--log-driver=fluentd", "--log-opt", "fluentd-address=corrupted:c"), checker.NotNil)
	expected := "Failed to set log opts: invalid fluentd-address corrupted:c: "
	runCmd := exec.Command("grep", expected, s.d.LogfileName())
	out, _, err := runCommandWithOutput(runCmd)
	c.Assert(err, checker.IsNil, check.Commentf("Expected %q message; but doesn't exist in log: %q", expected, out))
}

func (s *DockerDaemonSuite) TestDaemonStartWithoutHost(c *check.C) {
	s.d.useDefaultHost = true
	defer func() {
		s.d.useDefaultHost = false
	}()
	c.Assert(s.d.Start(), checker.IsNil)
}

func (s *DockerDaemonSuite) TestDaemonStartWithDefalutTlsHost(c *check.C) {
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
