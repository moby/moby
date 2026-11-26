package security

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/moby/moby/client/pkg/security"
	"github.com/moby/moby/v2/integration/internal/container"
	"gotest.tools/v3/skip"
)

func hasSecurityOption(name string) bool {
	return slices.ContainsFunc(security.DecodeOptions(testEnv.DaemonInfo.SecurityOptions), func(o security.Option) bool {
		return o.Name == name
	})
}

func skipIfNoSeccomp(t *testing.T) {
	t.Helper()
	skip.If(t, !hasSecurityOption("seccomp"), "seccomp is not enabled in the daemon")
}

func skipIfNoAppArmor(t *testing.T) {
	t.Helper()
	skip.If(t, !hasSecurityOption("apparmor"), "AppArmor is not enabled in the daemon")
}

func skipIfNoUserNamespaces(t *testing.T) {
	t.Helper()
	skip.If(t, testEnv.IsRemoteDaemon(), "requires a local daemon")
	skip.If(t, !testEnv.IsUserNamespaceInKernel(), "kernel does not support user namespaces")
	skip.If(t, testEnv.IsUserNamespace(), "not applicable with user namespace remapping")
}

// skipIfUnprivilegedUsernsDisabled skips the test on kernels (e.g. Debian)
// where unprivileged user namespace creation is disabled via sysctl.
func skipIfUnprivilegedUsernsDisabled(t *testing.T) {
	t.Helper()
	content, err := os.ReadFile("/proc/sys/kernel/unprivileged_userns_clone")
	skip.If(t, err == nil && strings.Contains(string(content), "0"), "unprivileged user namespaces are disabled")
}

func isAmd64() bool {
	arch := testEnv.DaemonInfo.Architecture
	return arch == "amd64" || arch == "x86_64"
}

// runContainer runs a container to completion and returns its result.
func runContainer(ctx context.Context, t *testing.T, ops ...func(*container.TestContainerConfig)) container.RunResult {
	t.Helper()
	return container.RunAttach(ctx, t, testEnv.APIClient(), ops...)
}

func combined(res container.RunResult) string {
	return res.Stdout.String() + res.Stderr.String()
}
