package security

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/moby/moby/v2/daemon/config"
	"github.com/moby/moby/v2/integration/internal/container"
	"github.com/moby/moby/v2/internal/testutil"
	"github.com/moby/moby/v2/internal/testutil/daemon"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestSeccomp(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	skip.If(t, testEnv.IsUserNamespace, "privileged test cases are incompatible with user namespaces")

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	const confined = `{
 "defaultAction": "SCMP_ACT_ALLOW",
 "syscalls": [ { "names": [ "chown", "chown32", "fchownat" ], "action": "SCMP_ACT_ERRNO" } ]
}
`
	type testCase struct {
		ops              []func(*container.TestContainerConfig)
		expectedExitCode int
	}
	testCases := []testCase{
		{
			ops:              nil,
			expectedExitCode: 0,
		},
		{
			ops:              []func(*container.TestContainerConfig){container.WithPrivileged(true)},
			expectedExitCode: 0,
		},
		{
			ops:              []func(*container.TestContainerConfig){container.WithSecurityOpt("seccomp=" + confined)},
			expectedExitCode: 1,
		},
		{
			// A custom profile should be still enabled, even when --privileged is set
			// https://github.com/moby/moby/issues/47499
			ops:              []func(*container.TestContainerConfig){container.WithPrivileged(true), container.WithSecurityOpt("seccomp=" + confined)},
			expectedExitCode: 1,
		},
	}
	for _, tc := range testCases {
		cID := container.Run(ctx, t, apiClient, tc.ops...)
		res, err := container.Exec(ctx, apiClient, cID, []string{"chown", "42", "/bin/true"})
		assert.NilError(t, err)
		assert.Equal(t, tc.expectedExitCode, res.ExitCode)
		if tc.expectedExitCode != 0 {
			assert.Check(t, is.Contains(res.Stderr(), "Operation not permitted"))
		}
	}
}

func TestConfigDaemonSeccompProfiles(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	skip.If(t, testEnv.IsRemoteDaemon, "cannot start daemon on remote test run")
	ctx := testutil.StartSpan(baseContext, t)

	d := daemon.New(t)
	defer d.Stop(t)

	tests := []struct {
		doc             string
		profile         string
		expectedProfile string
	}{
		{
			doc:             "empty profile set",
			profile:         "",
			expectedProfile: config.SeccompProfileDefault,
		},
		{
			doc:             "default profile",
			profile:         config.SeccompProfileDefault,
			expectedProfile: config.SeccompProfileDefault,
		},
		{
			doc:             "unconfined profile",
			profile:         config.SeccompProfileUnconfined,
			expectedProfile: config.SeccompProfileUnconfined,
		},
	}

	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			_ = testutil.StartSpan(ctx, t)

			d.Start(t, "--seccomp-profile="+tc.profile)
			info := d.Info(t)
			assert.Assert(t, is.Contains(info.SecurityOptions, "name=seccomp,profile="+tc.expectedProfile))
			d.Stop(t)

			cfg := filepath.Join(d.RootDir(), "daemon.json")
			err := os.WriteFile(cfg, []byte(`{"seccomp-profile": "`+tc.profile+`"}`), 0o644)
			assert.NilError(t, err)

			d.Start(t, "--config-file", cfg)
			info = d.Info(t)
			assert.Assert(t, is.Contains(info.SecurityOptions, "name=seccomp,profile="+tc.expectedProfile))
			d.Stop(t)
		})
	}
}
