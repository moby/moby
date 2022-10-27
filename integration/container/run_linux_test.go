package container // import "github.com/docker/docker/integration/container"

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/integration/internal/container"
	net "github.com/docker/docker/integration/internal/network"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/docker/testutil/daemon"
	"golang.org/x/sys/unix"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

func TestNISDomainname(t *testing.T) {
	// Older versions of the daemon would concatenate hostname and domainname,
	// so hostname "foobar" and domainname "baz.cyphar.com" would produce
	// `foobar.baz.cyphar.com` as hostname.
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.40"), "skip test from new feature")
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")

	// Rootless supports custom Hostname but doesn't support custom Domainname
	//  OCI runtime create failed: container_linux.go:349: starting container process caused "process_linux.go:449: container init caused \
	//  "write sysctl key kernel.domainname: open /proc/sys/kernel/domainname: permission denied\"": unknown.
	skip.If(t, testEnv.IsRootless, "rootless mode doesn't support setting Domainname (TODO: https://github.com/moby/moby/issues/40632)")

	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	const (
		hostname   = "foobar"
		domainname = "baz.cyphar.com"
	)

	cID := container.Run(ctx, t, client, func(c *container.TestContainerConfig) {
		c.Config.Hostname = hostname
		c.Config.Domainname = domainname
	})

	poll.WaitOn(t, container.IsInState(ctx, client, cID, "running"), poll.WithDelay(100*time.Millisecond))

	inspect, err := client.ContainerInspect(ctx, cID)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(hostname, inspect.Config.Hostname))
	assert.Check(t, is.Equal(domainname, inspect.Config.Domainname))

	// Check hostname.
	res, err := container.Exec(ctx, client, cID,
		[]string{"cat", "/proc/sys/kernel/hostname"})
	assert.NilError(t, err)
	assert.Assert(t, is.Len(res.Stderr(), 0))
	assert.Equal(t, 0, res.ExitCode)
	assert.Check(t, is.Equal(hostname, strings.TrimSpace(res.Stdout())))

	// Check domainname.
	res, err = container.Exec(ctx, client, cID,
		[]string{"cat", "/proc/sys/kernel/domainname"})
	assert.NilError(t, err)
	assert.Assert(t, is.Len(res.Stderr(), 0))
	assert.Equal(t, 0, res.ExitCode)
	assert.Check(t, is.Equal(domainname, strings.TrimSpace(res.Stdout())))
}

func TestHostnameDnsResolution(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")

	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	const (
		hostname = "foobar"
	)

	// using user defined network as we want to use internal DNS
	netName := "foobar-net"
	net.CreateNoError(context.Background(), t, client, netName, net.WithDriver("bridge"))

	cID := container.Run(ctx, t, client, func(c *container.TestContainerConfig) {
		c.Config.Hostname = hostname
		c.HostConfig.NetworkMode = containertypes.NetworkMode(netName)
	})

	poll.WaitOn(t, container.IsInState(ctx, client, cID, "running"), poll.WithDelay(100*time.Millisecond))

	inspect, err := client.ContainerInspect(ctx, cID)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(hostname, inspect.Config.Hostname))

	// Clear hosts file so ping will use DNS for hostname resolution
	res, err := container.Exec(ctx, client, cID,
		[]string{"sh", "-c", "echo 127.0.0.1 localhost | tee /etc/hosts && ping -c 1 foobar"})
	assert.NilError(t, err)
	assert.Check(t, is.Equal("", res.Stderr()))
	assert.Equal(t, 0, res.ExitCode)
}

func TestUnprivilegedPortsAndPing(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	skip.If(t, testEnv.IsRootless, "rootless mode doesn't support setting net.ipv4.ping_group_range and net.ipv4.ip_unprivileged_port_start")

	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	cID := container.Run(ctx, t, client, func(c *container.TestContainerConfig) {
		c.Config.User = "1000:1000"
	})

	poll.WaitOn(t, container.IsInState(ctx, client, cID, "running"), poll.WithDelay(100*time.Millisecond))

	// Check net.ipv4.ping_group_range.
	res, err := container.Exec(ctx, client, cID, []string{"cat", "/proc/sys/net/ipv4/ping_group_range"})
	assert.NilError(t, err)
	assert.Assert(t, is.Len(res.Stderr(), 0))
	assert.Equal(t, 0, res.ExitCode)
	assert.Equal(t, `0	2147483647`, strings.TrimSpace(res.Stdout()))

	// Check net.ipv4.ip_unprivileged_port_start.
	res, err = container.Exec(ctx, client, cID, []string{"cat", "/proc/sys/net/ipv4/ip_unprivileged_port_start"})
	assert.NilError(t, err)
	assert.Assert(t, is.Len(res.Stderr(), 0))
	assert.Equal(t, 0, res.ExitCode)
	assert.Equal(t, "0", strings.TrimSpace(res.Stdout()))
}

func TestPrivilegedHostDevices(t *testing.T) {
	// Host devices are linux only. Also it creates host devices,
	// so needs to be same host.
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")

	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	const (
		devTest         = "/dev/test"
		devRootOnlyTest = "/dev/root-only/test"
	)

	// Create Null devices.
	if err := system.Mknod(devTest, unix.S_IFCHR|0600, int(system.Mkdev(1, 3))); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(devTest)
	if err := os.Mkdir(filepath.Dir(devRootOnlyTest), 0700); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(filepath.Dir(devRootOnlyTest))
	if err := system.Mknod(devRootOnlyTest, unix.S_IFCHR|0600, int(system.Mkdev(1, 3))); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(devRootOnlyTest)

	cID := container.Run(ctx, t, client, container.WithPrivileged(true))

	poll.WaitOn(t, container.IsInState(ctx, client, cID, "running"), poll.WithDelay(100*time.Millisecond))

	// Check test device.
	res, err := container.Exec(ctx, client, cID, []string{"ls", devTest})
	assert.NilError(t, err)
	assert.Equal(t, 0, res.ExitCode)
	assert.Check(t, is.Equal(strings.TrimSpace(res.Stdout()), devTest))

	// Check root-only test device.
	res, err = container.Exec(ctx, client, cID, []string{"ls", devRootOnlyTest})
	assert.NilError(t, err)
	if testEnv.IsRootless() {
		assert.Equal(t, 1, res.ExitCode)
		assert.Check(t, is.Contains(res.Stderr(), "No such file or directory"))
	} else {
		assert.Equal(t, 0, res.ExitCode)
		assert.Check(t, is.Equal(strings.TrimSpace(res.Stdout()), devRootOnlyTest))
	}
}

func TestRunConsoleSize(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.42"), "skip test from new feature")

	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	cID := container.Run(ctx, t, client,
		container.WithTty(true),
		container.WithImage("busybox"),
		container.WithCmd("stty", "size"),
		container.WithConsoleSize(57, 123),
	)

	poll.WaitOn(t, container.IsStopped(ctx, client, cID), poll.WithDelay(100*time.Millisecond))

	out, err := client.ContainerLogs(ctx, cID, types.ContainerLogsOptions{ShowStdout: true})
	assert.NilError(t, err)
	defer out.Close()

	var b bytes.Buffer
	_, err = io.Copy(&b, out)
	assert.NilError(t, err)

	assert.Equal(t, strings.TrimSpace(b.String()), "123 57")
}

func TestRunWithAlternativeContainerdShim(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")

	realShimPath, err := exec.LookPath("containerd-shim-runc-v2")
	assert.Assert(t, err)
	realShimPath, err = filepath.Abs(realShimPath)
	assert.Assert(t, err)

	// t.TempDir() can't be used here as the temporary directory returned by
	// that function cannot be accessed by the fake-root user for rootless
	// Docker. It creates a nested hierarchy of directories where the
	// outermost has permission 0700.
	shimDir, err := os.MkdirTemp("", t.Name())
	assert.Assert(t, err)
	t.Cleanup(func() {
		if err := os.RemoveAll(shimDir); err != nil {
			t.Errorf("shimDir RemoveAll cleanup: %v", err)
		}
	})
	assert.Assert(t, os.Chmod(shimDir, 0777))
	shimDir, err = filepath.Abs(shimDir)
	assert.Assert(t, err)
	assert.Assert(t, os.Symlink(realShimPath, filepath.Join(shimDir, "containerd-shim-realfake-v42")))

	d := daemon.New(t,
		daemon.WithEnvVars("PATH="+shimDir+":"+os.Getenv("PATH")),
		daemon.WithContainerdSocket(""), // A new containerd instance needs to be started which inherits the PATH env var defined above.
	)
	d.StartWithBusybox(t)
	defer d.Stop(t)

	client := d.NewClientT(t)
	ctx := context.Background()

	cID := container.Run(ctx, t, client,
		container.WithImage("busybox"),
		container.WithCmd("sh", "-c", `echo 'Hello, world!'`),
		container.WithRuntime("io.containerd.realfake.v42"),
	)

	poll.WaitOn(t, container.IsStopped(ctx, client, cID), poll.WithDelay(100*time.Millisecond))

	out, err := client.ContainerLogs(ctx, cID, types.ContainerLogsOptions{ShowStdout: true})
	assert.NilError(t, err)
	defer out.Close()

	var b bytes.Buffer
	_, err = stdcopy.StdCopy(&b, io.Discard, out)
	assert.NilError(t, err)

	assert.Equal(t, strings.TrimSpace(b.String()), "Hello, world!")

	d.Stop(t)
	d.Start(t, "--default-runtime="+"io.containerd.realfake.v42")

	cID = container.Run(ctx, t, client,
		container.WithImage("busybox"),
		container.WithCmd("sh", "-c", `echo 'Hello, world!'`),
	)

	poll.WaitOn(t, container.IsStopped(ctx, client, cID), poll.WithDelay(100*time.Millisecond))

	out, err = client.ContainerLogs(ctx, cID, types.ContainerLogsOptions{ShowStdout: true})
	assert.NilError(t, err)
	defer out.Close()

	b.Reset()
	_, err = stdcopy.StdCopy(&b, io.Discard, out)
	assert.NilError(t, err)

	assert.Equal(t, strings.TrimSpace(b.String()), "Hello, world!")
}
