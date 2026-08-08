package launcher

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/moby/moby/v2/internal/extensions"
	echov1 "github.com/moby/moby/v2/internal/extensions/internal/launcher/echo/v1"
	echopb "github.com/moby/moby/v2/internal/extensions/internal/launcher/echo/v1/protogen"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// exeName returns name as an extension binary file name for the current OS: an
// .exe on Windows, the bare name elsewhere.
func exeName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// TestBinaries checks the scan: only executables whose file name is a valid
// extension id are returned (subdirectories, non-executable files, and stray
// executables with a non-id name are skipped), and a missing directory yields
// none rather than an error.
func TestBinaries(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, exeName("org.example.one.v1"))
	assert.NilError(t, os.WriteFile(exe, []byte("x"), 0o755))
	assert.NilError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644))
	assert.NilError(t, os.Mkdir(filepath.Join(dir, "sub"), 0o755))

	// A stray executable whose name is not a valid extension id is not launched:
	// the directory is trusted, but it is not a dumping ground for arbitrary
	// binaries. "helper" has no reverse-DNS/version shape, so it is skipped.
	assert.NilError(t, os.WriteFile(filepath.Join(dir, exeName("helper")), []byte("x"), 0o755))

	bins, err := Binaries(context.Background(), dir)
	assert.NilError(t, err)
	assert.DeepEqual(t, bins, []string{exe})

	missing, err := Binaries(context.Background(), filepath.Join(dir, "does-not-exist"))
	assert.NilError(t, err)
	assert.Check(t, is.Len(missing, 0))
}

// TestBinariesRefusesWorldWritable checks the root-exec safety filter: a
// world-writable binary is skipped, and a world-writable directory yields none.
func TestBinariesRefusesWorldWritable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("world-writable bit is not meaningful on Windows")
	}
	dir := t.TempDir()
	good := filepath.Join(dir, "org.example.good.v1")
	bad := filepath.Join(dir, "org.example.bad.v1")
	assert.NilError(t, os.WriteFile(good, []byte("x"), 0o755))
	assert.NilError(t, os.WriteFile(bad, []byte("x"), 0o755))
	assert.NilError(t, os.Chmod(bad, 0o757)) // o+w, set past umask: not trusted

	bins, err := Binaries(context.Background(), dir)
	assert.NilError(t, err)
	assert.DeepEqual(t, bins, []string{good})

	// A world-writable directory is refused wholesale.
	wwDir := t.TempDir()
	assert.NilError(t, os.WriteFile(filepath.Join(wwDir, "org.example.x.v1"), []byte("x"), 0o755))
	assert.NilError(t, os.Chmod(wwDir, 0o777))
	bins, err = Binaries(context.Background(), wwDir)
	assert.NilError(t, err)
	assert.Check(t, is.Len(bins, 0))
}

// TestBinariesRefusesUntrustedOwner checks the ownership filter: a binary owned
// by a uid that is neither root nor the daemon's own user could be rewritten by
// that user and then run as the daemon, so it is skipped. Creating a file owned
// by another user needs root, so this runs only as root.
func TestBinariesRefusesUntrustedOwner(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ownership is not enforced on Windows")
	}
	if os.Geteuid() != 0 {
		t.Skip("changing a file's owner requires root")
	}
	dir := t.TempDir()
	good := filepath.Join(dir, "org.example.good.v1")
	bad := filepath.Join(dir, "org.example.bad.v1")
	assert.NilError(t, os.WriteFile(good, []byte("x"), 0o755))
	assert.NilError(t, os.WriteFile(bad, []byte("x"), 0o755))
	assert.NilError(t, os.Chown(bad, 65534, 65534)) // nobody: not root, not us

	bins, err := Binaries(context.Background(), dir)
	assert.NilError(t, err)
	assert.DeepEqual(t, bins, []string{good})
}

// TestLaunchOutOfProcess exercises the whole out-of-process path end to end:
// it builds the testdata extension, launches it over the real stdio handshake,
// reads its declaration via Describe, and calls its echo point through the
// generated ClientProvider over the gRPC unix socket.
func TestLaunchOutOfProcess(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and launches a helper binary")
	}
	// An extension is an executable named after its id, in an extensions dir.
	const id = "org.example.exthook.v1"
	bin := filepath.Join(t.TempDir(), id)
	build := exec.Command("go", "build", "-o", bin, "./testdata/exthook")
	out, err := build.CombinedOutput()
	assert.NilError(t, err, "build extension: %s", out)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	launched, err := Launcher{RuntimeDir: t.TempDir()}.Launch(ctx, bin)
	assert.NilError(t, err)
	defer func() { assert.NilError(t, launched.Close(context.Background())) }()

	assert.Equal(t, launched.ID, extensions.ExtensionID(id))
	assert.Check(t, is.Len(launched.Points, 1))
	assert.Equal(t, launched.Points[0].ID, echov1.Point.ID())

	client := echopb.ClientProvider(launched.Conn).Impl.(echov1.EchoServer)

	resp, err := client.Echo(ctx, &echov1.EchoRequest{Message: "ping"})
	assert.NilError(t, err, "non-empty message should be echoed")
	assert.Equal(t, resp.Message, "ping")

	_, err = client.Echo(ctx, &echov1.EchoRequest{})
	assert.Check(t, is.ErrorContains(err, "message must not be empty"))
}
