package container // import "github.com/docker/docker/integration/container"

import (
	"strings"
	"testing"

	"github.com/docker/docker/integration/internal/container"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/icmd"
)

func TestDefaultUsernsPrivs(t *testing.T) {
	ctx := setupTest(t)

	apiClient := testEnv.APIClient()

	// Make sure that 2 privileged containers have the same user namespace
	hostNs1Res := container.RunAttach(ctx, t, apiClient, container.WithPrivileged(true), container.WithCmd("readlink", "/proc/self/ns/user"))
	assert.Equal(t, hostNs1Res.ExitCode, 0)
	hostns1 := strings.TrimSpace(hostNs1Res.Stdout.String())
	assert.Assert(t, hostns1 != "", "user namespace should not be empty")

	hostNs2Res := container.RunAttach(ctx, t, apiClient, container.WithPrivileged(true), container.WithCmd("readlink", "/proc/self/ns/user"))
	assert.Equal(t, hostNs2Res.ExitCode, 0)
	hostns2 := strings.TrimSpace(hostNs1Res.Stdout.String())
	assert.Assert(t, hostns2 != "", "user namespace should not be empty")

	assert.Equal(t, hostns1, hostns2, "privileged user namespaces should be the same")

	if testEnv.IsLocalDaemon() {
		// Make sure the privileged container has the same user namespace as the host
		res := icmd.RunCommand("readlink", "/proc/self/ns/user")
		res.Assert(t, icmd.Success)

		out := strings.TrimSpace(res.Combined())
		assert.NilError(t, res.Error, string(out))
		assert.Equal(t, hostns1, out, "privileged user namespace should be the same as the host")
	}

	res := container.RunAttach(ctx, t, apiClient, container.WithCmd("readlink", "/proc/self/ns/user"))
	assert.Equal(t, res.ExitCode, 0, res.Stderr)
	cUserns := strings.TrimSpace(res.Stdout.String())
	assert.Assert(t, cUserns != "", "user namespace should not be empty")
	assert.Assert(t, cUserns != hostns1, "user namespace should not be the same as the host")

	cmd := `
set -e
mkdir /test1
mkdir /test2
touch /test1/hello
mount --bind /test1 /test2
[ -f /test2/hello ]
`

	// TODO: For some reason this is failing in the test env but works just fine when running manually.
	res = container.RunAttach(ctx, t, apiClient, container.WithCmd("sh", "-c", cmd))
	assert.Equal(t, res.ExitCode, 0, res.Stderr)
}
