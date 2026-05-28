package container

import (
	"context"
	_ "embed"
	"slices"
	"strings"
	"testing"

	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/integration/internal/container"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

var (
	//go:embed testdata/af_alg.c
	afALGSource string

	//go:embed testdata/af_vsock.c
	afVSOCKSource string

	//go:embed testdata/socketcall.c
	socketcallSource string
)

// compileAndExecSocketDenied writes a C source file into the container,
// compiles it with the given compiler command, runs the binary as uid 1000,
// and asserts that socket creation fails.
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
		func(ec *client.ExecCreateOptions) {
			ec.User = "1000"
		},
	)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(res.ExitCode, 1), "expected %s socket program to fail", name)

	out := strings.ToLower(res.Combined())
	assert.Check(t, is.Contains(out, "socket"), "expected socket-related error message")
	// Seccomp returns EPERM ("not permitted"), AppArmor returns EACCES
	// ("permission denied"). Accept either.
	denied := strings.Contains(out, "not permitted") || strings.Contains(out, "permission denied")
	assert.Check(t, denied, "expected EPERM or EACCES, got: %s", res.Combined())
}

// TestExecSocketDenied verifies that AF_ALG and AF_VSOCK sockets cannot be
// created inside a container. AF_ALG is blocked by the default seccomp profile
// (via socket arg filtering) and by the default AppArmor profile (via
// "deny network alg"). AF_VSOCK is blocked by seccomp only.
func TestExecSocketDenied(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	cID := container.Run(ctx, t, apiClient, container.WithImage("debian:trixie-slim"), container.WithCmd("sleep", "infinity"))

	// Install build dependencies as root.
	res := container.ExecT(ctx, t, apiClient, cID, []string{
		"sh", "-c", "apt-get update && apt-get install -y --no-install-recommends gcc libc-dev linux-libc-dev",
	})
	res.AssertSuccess(t)

	gcc := []string{"gcc"}

	arch := testEnv.DaemonInfo.Architecture
	isAmd64 := arch == "amd64" || arch == "x86_64"

	t.Run("AF_ALG", func(t *testing.T) {
		compileAndExecSocketDenied(ctx, t, apiClient, cID, "AF_ALG", afALGSource, gcc)
	})
	t.Run("AF_VSOCK", func(t *testing.T) {
		compileAndExecSocketDenied(ctx, t, apiClient, cID, "AF_VSOCK", afVSOCKSource, gcc)
	})

	// Test socketcall(2) via int $0x80 to invoke the ia32 compat syscall
	// path from a native 64-bit binary. MAP_32BIT is used to place the
	// args array below 4 GB since the ia32 compat path truncates all
	// registers to 32 bits.
	//
	// The socketcall binary is compiled with -DSOCK_FAMILY and -DSOCK_TYPE
	// to set the address family and socket type at compile time.
	t.Run("socketcall_int80", func(t *testing.T) {
		skip.If(t, !isAmd64, "int $0x80 ia32 compat only available on amd64")
		// Seccomp cannot filter socketcall arguments (the address family
		// is behind a userspace pointer). Only an LSM (AppArmor or
		// SELinux) can deny AF_ALG via the security_socket_create hook.
		hasLSM := slices.Contains(testEnv.DaemonInfo.SecurityOptions, "name=apparmor") ||
			slices.Contains(testEnv.DaemonInfo.SecurityOptions, "name=selinux")
		skip.If(t, !hasLSM, "socketcall filtering requires AppArmor or SELinux")

		srcPath := "/tmp/socketcall.c"
		res := container.ExecT(ctx, t, apiClient, cID, []string{
			"sh", "-c", "cat > " + srcPath + " << 'CEOF'\n" + socketcallSource + "\nCEOF",
		})
		res.AssertSuccess(t)

		// AF_ALG (38) via socketcall must be denied by the LSM
		// (AppArmor's "deny network alg" or SELinux's alg_socket deny),
		// which catches it at the security_socket_create hook even
		// though seccomp cannot filter socketcall args.
		t.Run("AF_ALG", func(t *testing.T) {
			binPath := "/tmp/socketcall_af_alg"
			res := container.ExecT(ctx, t, apiClient, cID, append(gcc,
				"-DSOCK_FAMILY=AF_ALG", "-DSOCK_TYPE=SOCK_SEQPACKET",
				"-include", "linux/if_alg.h",
				srcPath, "-o", binPath,
			))
			res.AssertSuccess(t)

			res, err := container.Exec(ctx, apiClient, cID, []string{binPath},
				func(ec *client.ExecCreateOptions) {
					ec.User = "1000"
				},
			)
			assert.NilError(t, err)
			assert.Check(t, is.Equal(res.ExitCode, 1), "expected AF_ALG socketcall to fail, got: %s", res.Combined())
			assert.Check(t, is.Contains(strings.ToLower(res.Combined()), "permission denied"))
		})

		// AF_INET via socketcall must still work to ensure the deny
		// rule is targeted and does not break legitimate usage.
		t.Run("AF_INET", func(t *testing.T) {
			binPath := "/tmp/socketcall_af_inet"
			res := container.ExecT(ctx, t, apiClient, cID, append(gcc,
				"-DSOCK_FAMILY=AF_INET", "-DSOCK_TYPE=SOCK_STREAM",
				srcPath, "-o", binPath,
			))
			res.AssertSuccess(t)

			res, err := container.Exec(ctx, apiClient, cID, []string{binPath},
				func(ec *client.ExecCreateOptions) {
					ec.User = "1000"
				},
			)
			assert.NilError(t, err)
			assert.Check(t, is.Equal(res.ExitCode, 0), "expected AF_INET socketcall to succeed, got: %s", res.Combined())
		})
	})
}
