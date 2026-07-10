package container

import (
	"context"
	_ "embed"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/container"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

var (
	//go:embed testdata/af_alg.c
	afALGSource string

	//go:embed testdata/af_vsock.c
	afVSOCKSource string

	//go:embed testdata/af_alg_socketcall.c
	afALGSocketcallSource string
)

// compileAndExecSocketDenied writes a C source file into the container,
// compiles it with the given compiler command, runs the binary as uid 1000,
// and asserts that socket creation fails with the expected error.
func compileAndExecSocketDenied(ctx context.Context, t *testing.T, apiClient client.APIClient, cID string, name string, src string, cc []string, expectedErr string) {
	t.Helper()

	binPath := "/tmp/" + name
	srcPath := binPath + ".c"

	res := container.ExecT(ctx, t, apiClient, cID, []string{
		"sh", "-c", "cat > " + srcPath + " << 'CEOF'\n" + src + "\nCEOF",
	})
	res.AssertSuccess(t)

	compileCmd := append(cc, srcPath, "-o", binPath)
	res = container.ExecT(ctx, t, apiClient, cID, compileCmd)
	res.AssertSuccess(t)

	res, err := container.Exec(ctx, apiClient, cID, []string{binPath},
		func(ec *types.ExecConfig) {
			ec.User = "1000"
		},
	)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(res.ExitCode, 1), "expected %s socket program to fail", name)

	out := strings.ToLower(res.Combined())
	assert.Check(t, is.Contains(out, "socket"), "expected socket-related error message")
	assert.Check(t, is.Contains(out, expectedErr), "expected %s, got: %s", expectedErr, res.Combined())
}

// TestExecSocketDenied verifies that AF_ALG and AF_VSOCK sockets cannot be
// created inside a container. These address families are blocked by the
// default seccomp profile.
func TestExecSocketDenied(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	cID := container.Run(ctx, t, apiClient, container.WithImage("debian:bookworm-slim"), container.WithCmd("sleep", "infinity"))

	// Install build dependencies as root.
	res := container.ExecT(ctx, t, apiClient, cID, []string{
		"sh", "-c", "apt-get update && apt-get install -y --no-install-recommends gcc libc-dev linux-libc-dev",
	})
	res.AssertSuccess(t)

	gcc := []string{"gcc"}

	arch := testEnv.DaemonInfo.Architecture
	isAmd64 := arch == "amd64" || arch == "x86_64"

	t.Run("AF_ALG", func(t *testing.T) {
		compileAndExecSocketDenied(ctx, t, apiClient, cID, "AF_ALG", afALGSource, gcc, "not permitted")
	})
	t.Run("AF_VSOCK", func(t *testing.T) {
		compileAndExecSocketDenied(ctx, t, apiClient, cID, "AF_VSOCK", afVSOCKSource, gcc, "not permitted")
	})

	// Test AF_ALG via the socketcall(2) multiplexer using int $0x80 to
	// invoke the ia32 compat syscall path from a native 64-bit binary.
	// MAP_32BIT is used to place the args array below 4 GB, since the
	// ia32 compat path truncates all registers to 32 bits.
	t.Run("AF_ALG_socketcall_int80", func(t *testing.T) {
		skip.If(t, !isAmd64, "int $0x80 ia32 compat only available on amd64")

		compileAndExecSocketDenied(ctx, t, apiClient, cID, "AF_ALG_socketcall_int80", afALGSocketcallSource, gcc, "not implemented")
	})

	// Test AF_ALG with a real i386 binary cross-compiled from amd64. glibc
	// on i386 routes socket() through the socketcall(2) multiplexer, which
	// is a different seccomp path than the native socket(2) syscall.
	t.Run("AF_ALG_socketcall_i386", func(t *testing.T) {
		skip.If(t, !isAmd64, "i386 cross-compilation only available on amd64")

		res := container.ExecT(ctx, t, apiClient, cID, []string{
			"sh", "-c", "apt-get install -y --no-install-recommends gcc-i686-linux-gnu libc6-dev-i386-cross linux-libc-dev-i386-cross",
		})
		res.AssertSuccess(t)

		compileAndExecSocketDenied(ctx, t, apiClient, cID, "AF_ALG_socketcall_i386", afALGSource,
			[]string{"i686-linux-gnu-gcc", "-static"}, "not implemented",
		)
	})
}
