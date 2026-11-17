package container

import (
	"archive/tar"
	"bytes"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"

	mounttypes "github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/integration/internal/build"
	"github.com/moby/moby/v2/integration/internal/container"
	"github.com/moby/moby/v2/internal/testutil"
	"github.com/moby/moby/v2/internal/testutil/daemon"
	"github.com/moby/moby/v2/internal/testutil/fakecontext"
	"golang.org/x/sys/unix"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

// TestCopyToContainerKeepsBindMountReadOnly checks that a read-only bind mount
// is never writable while docker cp extracts into the container, even if a
// process with write access to an ancestor of the mount point races the setup.
//
// The ancestors /vol/a and /vol/b live in a writable bind mount. The test
// swaps them in a tight loop, so a path resolved after the mount was created
// can lead to the decoy mount at /vol/a/ro instead of the read-only mount at
// /vol/b/ro.
func TestCopyToContainerKeepsBindMountReadOnly(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	skip.If(t, testEnv.IsRemoteDaemon(), "requires access to the bind-mount source")
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	volDir := testutil.TempDir(t)
	dirA := filepath.Join(volDir, "a")
	dirB := filepath.Join(volDir, "b")
	decoyDir := testutil.TempDir(t)
	roDir := testutil.TempDir(t)

	// A rootless daemon needs write access to the control directory, the
	// decoy, and either mountpoint if it is exposed by the swap.
	for _, dir := range []string{volDir, decoyDir, filepath.Join(dirA, "ro"), filepath.Join(dirB, "ro")} {
		assert.NilError(t, os.MkdirAll(dir, 0o755))
		assert.NilError(t, os.Chmod(dir, 0o777))
	}

	// Mounts are applied in lexical order within a depth, so the decoy
	// is mounted before the read-only bind.
	cid := container.Create(ctx, t, apiClient,
		container.WithBindRaw(volDir+":/vol"),
		container.WithBindRaw(decoyDir+":/vol/a/ro"),
		container.WithBindRaw(roDir+":/vol/b/ro:ro"),
	)

	const content = "from docker cp"
	copyFile := func(dest, name string) error {
		t.Helper()
		var buf bytes.Buffer
		tw := tar.NewWriter(&buf)
		assert.NilError(t, tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(content))}))
		_, err := io.WriteString(tw, content)
		assert.NilError(t, err)
		assert.NilError(t, tw.Close())

		_, err = apiClient.CopyToContainer(ctx, cid, client.CopyToContainerOptions{
			DestinationPath: dest,
			Content:         &buf,
		})
		return err
	}

	// A working copy into the writable bind shows that failures below come
	// from the read-only mount, not from a broken setup.
	assert.NilError(t, copyFile("/vol/", "control"))
	got, err := os.ReadFile(filepath.Join(volDir, "control"))
	assert.NilError(t, err)
	assert.Equal(t, string(got), content)

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
				_ = unix.Renameat2(unix.AT_FDCWD, dirA, unix.AT_FDCWD, dirB, unix.RENAME_EXCHANGE)
			}
		}
	}()
	defer func() {
		close(stop)
		<-done
	}()

	var readOnly, redirected int
	for range 200 {
		// Extract from the writable parent so it is the mount, not the API's
		// destination-path check, that rejects the write. Depending on which
		// directory is at /vol/b when the path is resolved, the file lands in
		// the decoy or hits the read-only mount.
		if err := copyFile("/vol/b/", "ro/file"); err != nil {
			assert.ErrorContains(t, err, unix.EROFS.Error())
			readOnly++
		} else {
			redirected++
		}

		_, err := os.Stat(filepath.Join(roDir, "file"))
		assert.ErrorIs(t, err, os.ErrNotExist)
	}
	assert.Check(t, readOnly > 0, "no copy reached the read-only mount")
	assert.Check(t, redirected > 0, "no copy reached the decoy mount; the swap had no effect")
}

// TestCopyWithAbsoluteSymlinkedMountTarget was introduced as a regression test
// for https://github.com/moby/moby/issues/52653.
//
// The security fix in GHSA-vp62-88p7-qqf5 switched openContainerFS to use
// os.Root for the mount-destination operations.
// os.Root refuses to follow absolute symlinks, but distro images commonly ship
// /var/run as an absolute symlink to /run.
// As a result, any container with a bind mount whose target traversed such a
// symlink (e.g. -v /host/sock:/var/run/docker.sock) made `docker cp` fail.
func TestCopyWithAbsoluteSymlinkedMountTarget(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	// Build an image with an absolute in-container symlink along the mount
	// target path.
	// Stock distro images expose this shape via /var/run -> /run, but we set
	// up our own /sockets -> /root pair so the test does not depend on any
	// particular base image's layout.
	buildCtx := fakecontext.New(t, "",
		fakecontext.WithDockerfile(`FROM busybox
RUN touch /root/nil && ln -s /root /sockets
`),
	)
	defer buildCtx.Close()
	imgID := build.Do(ctx, t, apiClient, buildCtx, client.ImageBuildOptions{})

	// Use testutil.TempDir so the rootless daemon can access the bind-mount
	// source: t.TempDir() creates a 0700 parent that the fake-root user
	// cannot stat.
	srcDir := testutil.TempDir(t)
	assert.NilError(t, os.WriteFile(filepath.Join(srcDir, "sock"), nil, 0o644))

	cid := container.Create(ctx, t, apiClient,
		container.WithImage(imgID),
		container.WithMount(mounttypes.Mount{
			Type:   mounttypes.TypeBind,
			Source: filepath.Join(srcDir, "sock"),
			Target: "/sockets/docker.sock",
		}),
	)

	_, err := apiClient.CopyToContainer(ctx, cid, client.CopyToContainerOptions{
		DestinationPath: "/sockets/",
		Content:         bytes.NewReader(nil),
	})
	assert.NilError(t, err)
}

// TestCopyWithNestedBindMountedSocket is a regression test for a container
// with two overlapping bind mounts: one covering a directory, and one whose
// destination is a path inside that directory.
//
// openContainerFS replays the container's bind mounts parent-first, so by the
// time the inner mount is set up its destination resolves to the contents of
// the outer mount, and may be any kind of file. The security fix in
// GHSA-vp62-88p7-qqf5 started unconditionally opening that destination -- to
// create it, and again to pin its inode -- which fails with ENXIO when it is a
// socket, breaking docker cp in both directions for the whole container.
func TestCopyWithNestedBindMountedSocket(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	// Use testutil.TempDir so the rootless daemon can access the bind-mount
	// source: t.TempDir() creates a 0700 parent that the fake-root user
	// cannot stat.
	srcDir := testutil.TempDir(t)
	l, err := net.Listen("unix", filepath.Join(srcDir, "sock"))
	assert.NilError(t, err)
	defer l.Close()

	cid := container.Create(ctx, t, apiClient,
		container.WithMount(mounttypes.Mount{
			Type:   mounttypes.TypeBind,
			Source: srcDir,
			Target: "/mountpoint",
		}),
		container.WithMount(mounttypes.Mount{
			Type:   mounttypes.TypeBind,
			Source: filepath.Join(srcDir, "sock"),
			Target: "/mountpoint/sock",
		}),
	)

	res, err := apiClient.CopyFromContainer(ctx, cid, client.CopyFromContainerOptions{SourcePath: "/mountpoint"})
	assert.NilError(t, err)
	defer res.Content.Close()
	_, err = io.Copy(io.Discard, res.Content)
	assert.NilError(t, err)

	_, err = apiClient.CopyToContainer(ctx, cid, client.CopyToContainerOptions{
		DestinationPath: "/mountpoint/",
		Content:         bytes.NewReader(nil),
	})
	assert.NilError(t, err)
}

// TestCopyWithReadOnlyBindMountedSocket is a regression test for docker cp
// on a container with a read-only bind mount whose source is on a nosuid,nodev
// filesystem, such as an X11 socket shared with
// -v /tmp/.X11-unix/X0:/tmp/.X11-unix/X0:ro on a host where /tmp is a tmpfs.
func TestCopyWithReadOnlyBindMountedSocket(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	skip.If(t, testEnv.IsRemoteDaemon(), "cannot start daemon on remote test run")
	skip.If(t, testEnv.IsNonRootClient, "requires root and executing on the same host as the daemon")
	ctx := setupTest(t)

	// Use testutil.TempDir so the rootless daemon can access the bind-mount
	// source: t.TempDir() creates a 0700 parent that the fake-root user
	// cannot stat.
	srcDir := testutil.TempDir(t)
	assert.NilError(t, unix.Mount("tmpfs", srcDir, "tmpfs", unix.MS_NOSUID|unix.MS_NODEV, "mode=755"))
	t.Cleanup(func() { _ = unix.Unmount(srcDir, unix.MNT_DETACH) })

	l, err := net.Listen("unix", filepath.Join(srcDir, "X0"))
	assert.NilError(t, err)
	defer l.Close()

	// The daemon is started after the tmpfs is mounted, so that a rootless
	// daemon inherits it into its user namespace, with its flags locked.
	d := daemon.New(t)
	defer d.Cleanup(t)
	d.StartWithBusybox(ctx, t, "--iptables=false", "--ip6tables=false")
	defer d.Stop(t)
	apiClient := d.NewClientT(t)

	cid := container.Create(ctx, t, apiClient,
		container.WithMount(mounttypes.Mount{
			Type:     mounttypes.TypeBind,
			Source:   filepath.Join(srcDir, "X0"),
			Target:   "/tmp/.X11-unix/X0",
			ReadOnly: true,
		}),
	)

	res, err := apiClient.CopyFromContainer(ctx, cid, client.CopyFromContainerOptions{SourcePath: "/etc/hostname"})
	assert.NilError(t, err)
	defer res.Content.Close()
	_, err = io.Copy(io.Discard, res.Content)
	assert.NilError(t, err)

	_, err = apiClient.CopyToContainer(ctx, cid, client.CopyToContainerOptions{
		DestinationPath: "/tmp/",
		Content:         bytes.NewReader(nil),
	})
	assert.NilError(t, err)
}
