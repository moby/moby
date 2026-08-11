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

func exeName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func TestBinaries(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, exeName("org.example.one.v1"))
	assert.NilError(t, os.WriteFile(exe, []byte("x"), 0o755))
	assert.NilError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644))
	assert.NilError(t, os.Mkdir(filepath.Join(dir, "sub"), 0o755))

	assert.NilError(t, os.WriteFile(filepath.Join(dir, exeName("helper")), []byte("x"), 0o755))

	bins, err := Binaries(context.Background(), dir)
	assert.NilError(t, err)
	assert.DeepEqual(t, bins, []string{exe})

	missing, err := Binaries(context.Background(), filepath.Join(dir, "does-not-exist"))
	assert.NilError(t, err)
	assert.Check(t, is.Len(missing, 0))
}

// TestBinariesRefusesWorldWritable verifies the root-exec safety filter.
func TestBinariesRefusesWorldWritable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("world-writable bit is not meaningful on Windows")
	}
	dir := t.TempDir()
	good := filepath.Join(dir, "org.example.good.v1")
	bad := filepath.Join(dir, "org.example.bad.v1")
	assert.NilError(t, os.WriteFile(good, []byte("x"), 0o755))
	assert.NilError(t, os.WriteFile(bad, []byte("x"), 0o755))
	assert.NilError(t, os.Chmod(bad, 0o757)) // o+w

	bins, err := Binaries(context.Background(), dir)
	assert.NilError(t, err)
	assert.DeepEqual(t, bins, []string{good})

	wwDir := t.TempDir()
	assert.NilError(t, os.WriteFile(filepath.Join(wwDir, "org.example.x.v1"), []byte("x"), 0o755))
	assert.NilError(t, os.Chmod(wwDir, 0o777))
	bins, err = Binaries(context.Background(), wwDir)
	assert.NilError(t, err)
	assert.Check(t, is.Len(bins, 0))
}

// TestBinariesRefusesUntrustedOwner verifies the ownership filter.
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

func TestLaunchOutOfProcess(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and launches a helper binary")
	}
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

func TestStopProcessSignalledExitIsNotAnError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no signal semantics to assert on Windows")
	}
	cmd := exec.Command("sleep", "60")
	assert.NilError(t, cmd.Start())
	wait := make(chan error, 1)
	go func() { wait <- cmd.Wait() }()

	assert.NilError(t, stopProcess(context.Background(), cmd, wait, 5*time.Second))
}

func TestStopProcessAfterSelfExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("zombies are a unix concept")
	}
	cmd := exec.Command("sleep", "0.05")
	assert.NilError(t, cmd.Start())
	wait := make(chan error, 1)
	go func() { wait <- cmd.Wait() }()
	time.Sleep(500 * time.Millisecond) // let it exit and be reaped

	done := make(chan error, 1)
	go func() { done <- stopProcess(context.Background(), cmd, wait, time.Second) }()
	select {
	case err := <-done:
		assert.NilError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("stopProcess blocked: the child was never reaped")
	}
}

func TestServicesRequireDeclaringThePoint(t *testing.T) {
	const declared = extensions.PointID("org.mobyproject.extension.declared.v1")
	const undeclared = extensions.PointID("org.mobyproject.extension.service.grpc.v0")

	l := &Launched{
		ID:     "org.example.ext.v1",
		Points: []LaunchedPoint{{ID: declared}},
		ProviderServices: map[extensions.PointID][]string{
			undeclared: {"example.Service"},
		},
	}
	assert.ErrorContains(t, validateDeclaredServices("org.example.ext.v1", l),
		"without declaring it")

	l.ProviderServices = map[extensions.PointID][]string{declared: {"example.Service"}}
	assert.NilError(t, validateDeclaredServices("org.example.ext.v1", l))
}
