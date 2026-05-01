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
)

// compileAndExecSocketDenied writes a C source file into the container,
// compiles it with the given compiler command, runs the binary as uid 1000,
// and asserts that socket creation fails with a permission or
// address-family error (not EFAULT or other unrelated failures).
func compileAndExecSocketDenied(ctx context.Context, t *testing.T, apiClient client.APIClient, cID string, name string, src string, cc []string) {
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

	// Seccomp blocks return either EPERM ("operation not permitted") or
	// EAFNOSUPPORT ("address family not supported"). Make sure the failure
	// is from seccomp, not from a bogus pointer (EFAULT) or other issue.
	permErr := strings.Contains(out, "not permitted") || strings.Contains(out, "not supported")
	assert.Check(t, permErr, "expected EPERM or EAFNOSUPPORT, got: %s", res.Combined())
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

	t.Run("AF_ALG", func(t *testing.T) {
		compileAndExecSocketDenied(ctx, t, apiClient, cID, "AF_ALG", afALGSource, gcc)
	})
	t.Run("AF_VSOCK", func(t *testing.T) {
		compileAndExecSocketDenied(ctx, t, apiClient, cID, "AF_VSOCK", afVSOCKSource, gcc)
	})
}
