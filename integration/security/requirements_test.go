package security

import (
	"context"
	"slices"
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

func skipIfNoAppArmor(t *testing.T) {
	t.Helper()
	skip.If(t, !hasSecurityOption("apparmor"), "AppArmor is not enabled in the daemon")
}

// runContainer runs a container to completion and returns its result.
func runContainer(ctx context.Context, t *testing.T, ops ...func(*container.TestContainerConfig)) container.RunResult {
	t.Helper()
	return container.RunAttach(ctx, t, testEnv.APIClient(), ops...)
}

func combined(res container.RunResult) string {
	return res.Stdout.String() + res.Stderr.String()
}
