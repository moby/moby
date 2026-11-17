package security

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moby/go-archive"
	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/daemon/config"
	"github.com/moby/moby/v2/integration/internal/build"
	"github.com/moby/moby/v2/integration/internal/container"
	"github.com/moby/moby/v2/internal/testutil"
	"github.com/moby/moby/v2/internal/testutil/daemon"
	"github.com/moby/moby/v2/internal/testutil/environment"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

const syscallTestImage = "syscall-test:latest"

// ensureSyscallTest builds the syscall-test image from contrib/syscall-test
// unless it already exists.
func ensureSyscallTest(ctx context.Context, t *testing.T) {
	t.Helper()
	apiClient := testEnv.APIClient()

	if testEnv.HasExistingImage(t, syscallTestImage) {
		return
	}

	buildCtx, err := archive.TarWithOptions("../../contrib/syscall-test", &archive.TarOptions{})
	assert.NilError(t, err)
	defer buildCtx.Close()

	resp, err := apiClient.ImageBuild(ctx, buildCtx, client.ImageBuildOptions{Tags: []string{syscallTestImage}})
	assert.NilError(t, err)
	defer resp.Body.Close()
	build.GetImageIDFromBody(t, resp.Body)

	// Protect the tags and digests of the new image so that it survives the
	// environment cleanup at the end of the test and can be reused.
	environment.ProtectImages(ctx, t, testEnv)

	// The Dockerfile ignores failures of the 32-bit build, so check that it
	// produced a binary where it is expected to.
	if isAmd64() {
		res := container.RunAttach(ctx, t, apiClient,
			container.WithImage(syscallTestImage),
			container.WithCmd("test", "-x", "/usr/bin/exit32-test"),
		)
		assert.Assert(t, res.ExitCode == 0, "%s was built without /usr/bin/exit32-test", syscallTestImage)
	}
}

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

// TestRunSeccompProfileDenyUnshare checks that a custom profile denying
// unshare makes it fail with operation not permitted.
func TestRunSeccompProfileDenyUnshare(t *testing.T) {
	skipIfNoSeccomp(t)
	skipIfNoAppArmor(t)
	ctx := setupTest(t)

	const profile = `{
	"defaultAction": "SCMP_ACT_ALLOW",
	"syscalls": [
		{
			"name": "unshare",
			"action": "SCMP_ACT_ERRNO"
		}
	]
}`
	res := runContainer(ctx, t,
		container.WithImage("debian:trixie-slim"),
		container.WithSecurityOpt("apparmor=unconfined"),
		container.WithSecurityOpt("seccomp="+profile),
		container.WithCmd("unshare", "-p", "-m", "-f", "-r", "mount", "-t", "proc", "none", "/proc"),
	)
	assert.Check(t, is.Equal(res.ExitCode, 1))
	assert.Check(t, is.Contains(res.Stderr.String(), "Operation not permitted"))
}

// TestRunSeccompProfileDenyChmod checks that a custom profile denying the
// chmod syscalls makes chmod fail with operation not permitted.
func TestRunSeccompProfileDenyChmod(t *testing.T) {
	skipIfNoSeccomp(t)
	ctx := setupTest(t)

	const profile = `{
	"defaultAction": "SCMP_ACT_ALLOW",
	"syscalls": [
		{
			"name": "chmod",
			"action": "SCMP_ACT_ERRNO"
		},
		{
			"name":"fchmod",
			"action": "SCMP_ACT_ERRNO"
		},
		{
			"name": "fchmodat",
			"action":"SCMP_ACT_ERRNO"
		}
	]
}`
	res := runContainer(ctx, t,
		container.WithImage("busybox"),
		container.WithSecurityOpt("seccomp="+profile),
		container.WithCmd("chmod", "400", "/etc/hostname"),
	)
	assert.Check(t, is.Equal(res.ExitCode, 1))
	assert.Check(t, is.Contains(res.Stderr.String(), "Operation not permitted"))
}

// TestRunSeccompProfileDenyUnshareUserns checks that a custom profile denying
// unshare of a userns makes it fail with operation not permitted.
func TestRunSeccompProfileDenyUnshareUserns(t *testing.T) {
	skipIfNoSeccomp(t)
	skipIfNoAppArmor(t)
	ctx := setupTest(t)

	// from sched.h
	profile := fmt.Sprintf(`{
	"defaultAction": "SCMP_ACT_ALLOW",
	"syscalls": [
		{
			"name": "unshare",
			"action": "SCMP_ACT_ERRNO",
			"args": [
				{
					"index": 0,
					"value": %d,
					"op": "SCMP_CMP_EQ"
				}
			]
		}
	]
}`, uint64(0x10000000))

	res := runContainer(ctx, t,
		container.WithImage("debian:trixie-slim"),
		container.WithSecurityOpt("apparmor=unconfined"),
		container.WithSecurityOpt("seccomp="+profile),
		container.WithCmd("unshare", "--map-root-user", "--user", "sh", "-c", "whoami"),
	)
	assert.Check(t, is.Equal(res.ExitCode, 1))
	assert.Check(t, is.Contains(res.Stderr.String(), "Operation not permitted"))
}

// TestRunSeccompProfileDenyCloneUserns checks that the default profile makes
// creating a userns fail with operation not permitted.
func TestRunSeccompProfileDenyCloneUserns(t *testing.T) {
	skipIfNoSeccomp(t)
	skip.If(t, testEnv.IsRemoteDaemon(), "requires a local daemon")
	ctx := setupTest(t)
	ensureSyscallTest(ctx, t)

	res := runContainer(ctx, t,
		container.WithImage(syscallTestImage),
		container.WithCmd("userns-test", "id"),
	)
	assert.Check(t, is.Equal(res.ExitCode, 1))
	assert.Check(t, is.Contains(res.Stderr.String(), "clone failed: Operation not permitted"))
}

// TestRunSeccompUnconfinedCloneUserns checks that an unconfined container can
// create a userns.
func TestRunSeccompUnconfinedCloneUserns(t *testing.T) {
	skipIfNoSeccomp(t)
	skipIfNoUserNamespaces(t)
	skipIfUnprivilegedUsernsDisabled(t)
	ctx := setupTest(t)
	ensureSyscallTest(ctx, t)

	res := runContainer(ctx, t,
		container.WithImage(syscallTestImage),
		container.WithSecurityOpt("seccomp=unconfined"),
		container.WithCmd("userns-test", "id"),
	)
	assert.Check(t, is.Equal(res.ExitCode, 0), combined(res))
	assert.Check(t, is.Contains(res.Stdout.String(), "nobody"))
}

// TestRunSeccompAllowPrivCloneUserns checks that a privileged container can
// create a userns.
func TestRunSeccompAllowPrivCloneUserns(t *testing.T) {
	skipIfNoSeccomp(t)
	skipIfNoUserNamespaces(t)
	ctx := setupTest(t)
	ensureSyscallTest(ctx, t)

	res := runContainer(ctx, t,
		container.WithImage(syscallTestImage),
		container.WithPrivileged(true),
		container.WithCmd("userns-test", "id"),
	)
	assert.Check(t, is.Equal(res.ExitCode, 0), combined(res))
	assert.Check(t, is.Contains(res.Stdout.String(), "nobody"))
}

// TestRunSeccompProfileAllow32Bit checks that 32 bit code can run on x86_64
// with the default profile.
func TestRunSeccompProfileAllow32Bit(t *testing.T) {
	skipIfNoSeccomp(t)
	skip.If(t, testEnv.IsRemoteDaemon(), "requires a local daemon")
	skip.If(t, !isAmd64(), "32-bit code is only supported on amd64")
	ctx := setupTest(t)
	ensureSyscallTest(ctx, t)

	res := runContainer(ctx, t,
		container.WithImage(syscallTestImage),
		container.WithCmd("exit32-test"),
	)
	assert.Check(t, is.Equal(res.ExitCode, 0), combined(res))
}

// TestRunSeccompAllowSetrlimit checks that the default profile doesn't break
// ulimit, which uses setrlimit.
func TestRunSeccompAllowSetrlimit(t *testing.T) {
	skipIfNoSeccomp(t)
	ctx := setupTest(t)

	res := runContainer(ctx, t,
		container.WithImage("debian:trixie-slim"),
		container.WithCmd("bash", "-c", "ulimit -v 1048510"),
	)
	assert.Check(t, is.Equal(res.ExitCode, 0), combined(res))
}

// TestRunSeccompDefaultProfileAcct checks that acct(2) is only allowed by the
// default profile when CAP_SYS_PACCT is held.
//
// Kernels without BSD process accounting return ENOSYS, which also happens
// with seccomp=unconfined.
func TestRunSeccompDefaultProfileAcct(t *testing.T) {
	skipIfNoSeccomp(t)
	skip.If(t, testEnv.IsRemoteDaemon(), "requires a local daemon")
	skip.If(t, testEnv.IsUserNamespace(), "not applicable with user namespace remapping")
	// acct(2) checks CAP_SYS_PACCT in the initial user namespace, which a
	// rootless container never has, so it fails with EPERM regardless of the
	// profile.
	skip.If(t, testEnv.IsRootless(), "acct requires CAP_SYS_PACCT in the initial user namespace")
	ctx := setupTest(t)
	ensureSyscallTest(ctx, t)

	tests := []struct {
		doc      string
		ops      []func(*container.TestContainerConfig)
		expected string
	}{
		{
			doc:      "default",
			expected: "Operation not permitted",
		},
		{
			doc:      "cap-add sys_admin",
			ops:      []func(*container.TestContainerConfig){container.WithCapability("sys_admin")},
			expected: "Operation not permitted",
		},
		{
			doc:      "cap-add sys_pacct",
			ops:      []func(*container.TestContainerConfig){container.WithCapability("sys_pacct")},
			expected: "No such file or directory",
		},
		{
			doc:      "cap-add ALL",
			ops:      []func(*container.TestContainerConfig){container.WithCapability("ALL")},
			expected: "No such file or directory",
		},
		{
			doc: "cap-drop ALL cap-add sys_pacct",
			ops: []func(*container.TestContainerConfig){
				container.WithDropCapability("ALL"),
				container.WithCapability("sys_pacct"),
			},
			expected: "No such file or directory",
		},
		{
			doc: "cap-add ALL seccomp unconfined",
			ops: []func(*container.TestContainerConfig){
				container.WithCapability("ALL"),
				container.WithSecurityOpt("seccomp=unconfined"),
			},
			expected: "No such file or directory",
		},
	}
	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			ops := append([]func(*container.TestContainerConfig){
				container.WithImage(syscallTestImage),
				container.WithCmd("acct-test"),
			}, tc.ops...)
			res := runContainer(ctx, t, ops...)
			assert.Check(t, res.ExitCode != 0, combined(res))
			assert.Check(t, is.Contains(combined(res), tc.expected))
		})
	}
}

// TestRunSeccompDefaultProfileNS checks that the default profile only allows
// creating namespaces when CAP_SYS_ADMIN is held.
func TestRunSeccompDefaultProfileNS(t *testing.T) {
	skipIfNoSeccomp(t)
	skip.If(t, testEnv.IsRemoteDaemon(), "requires a local daemon")
	skip.If(t, testEnv.IsUserNamespace(), "not applicable with user namespace remapping")
	ctx := setupTest(t)
	ensureSyscallTest(ctx, t)

	tests := []struct {
		doc       string
		ops       []func(*container.TestContainerConfig)
		cmd       []string
		expected  string
		expectErr bool
	}{
		{
			doc:       "default",
			cmd:       []string{"ns-test", "echo", "hello0"},
			expected:  "Operation not permitted",
			expectErr: true,
		},
		{
			doc:      "cap-add sys_admin",
			ops:      []func(*container.TestContainerConfig){container.WithCapability("sys_admin")},
			cmd:      []string{"ns-test", "echo", "hello1"},
			expected: "hello1",
		},
		{
			doc: "cap-drop all cap-add sys_admin",
			ops: []func(*container.TestContainerConfig){
				container.WithDropCapability("all"),
				container.WithCapability("sys_admin"),
			},
			cmd:      []string{"ns-test", "echo", "hello2"},
			expected: "hello2",
		},
		{
			doc:      "cap-add ALL",
			ops:      []func(*container.TestContainerConfig){container.WithCapability("ALL")},
			cmd:      []string{"ns-test", "echo", "hello3"},
			expected: "hello3",
		},
		{
			doc: "cap-add ALL seccomp unconfined ns",
			ops: []func(*container.TestContainerConfig){
				container.WithCapability("ALL"),
				container.WithSecurityOpt("seccomp=unconfined"),
			},
			cmd:      []string{"ns-test", "echo", "hello4"},
			expected: "hello4",
		},
	}
	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			ops := append([]func(*container.TestContainerConfig){
				container.WithImage(syscallTestImage),
				container.WithCmd(tc.cmd...),
			}, tc.ops...)
			res := runContainer(ctx, t, ops...)
			if tc.expectErr {
				assert.Check(t, res.ExitCode != 0, combined(res))
			} else {
				assert.Check(t, is.Equal(res.ExitCode, 0), combined(res))
			}
			assert.Check(t, is.Contains(combined(res), tc.expected))
		})
	}
}

// TestRunSeccompWithDefaultProfile checks that the default profile can be
// successfully parsed when passed as a custom profile. It uses the vendored
// default.json, which matches the daemon's built-in profile, because
// seccomp.DefaultProfile is only available on Linux, and the test runner
// may not be. It uses unshare, which is blocked by the default profile.
func TestRunSeccompWithDefaultProfile(t *testing.T) {
	skipIfNoSeccomp(t)
	ctx := setupTest(t)

	b, err := os.ReadFile("../../vendor/github.com/moby/profiles/seccomp/default.json")
	assert.NilError(t, err)

	res := runContainer(ctx, t,
		container.WithImage("debian:trixie-slim"),
		container.WithSecurityOpt("seccomp="+string(b)),
		container.WithCmd("unshare", "--map-root-user", "--user", "sh", "-c", "whoami"),
	)
	assert.Check(t, res.ExitCode != 0, combined(res))
	assert.Check(t, is.Equal(strings.TrimSpace(combined(res)), "unshare: unshare failed: Operation not permitted"))
}

func TestRunSeccompJSONNewFormat(t *testing.T) {
	skipIfNoSeccomp(t)
	ctx := setupTest(t)

	const profile = `{
	"defaultAction": "SCMP_ACT_ALLOW",
	"syscalls": [
		{
			"names": ["chmod", "fchmod", "fchmodat"],
			"action": "SCMP_ACT_ERRNO"
		}
	]
}`
	res := runContainer(ctx, t,
		container.WithImage("busybox"),
		container.WithSecurityOpt("seccomp="+profile),
		container.WithCmd("chmod", "777", "."),
	)
	assert.Check(t, res.ExitCode != 0, combined(res))
	assert.Check(t, is.Contains(combined(res), "Operation not permitted"))
}

// TestRunSeccompInvalidJSON checks that invalid profiles are rejected.
func TestRunSeccompInvalidJSON(t *testing.T) {
	skipIfNoSeccomp(t)
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	tests := []struct {
		doc         string
		profile     string
		expectedErr string
	}{
		{
			doc: "name and names",
			profile: `{
	"defaultAction": "SCMP_ACT_ALLOW",
	"syscalls": [
		{
			"name": "chmod",
			"names": ["fchmod", "fchmodat"],
			"action": "SCMP_ACT_ERRNO"
		}
	]
}`,
			expectedErr: "use either 'name' or 'names'",
		},
		{
			doc: "architectures and archMap",
			profile: `{
	"archMap": [
		{
			"architecture": "SCMP_ARCH_X86_64",
			"subArchitectures": [
				"SCMP_ARCH_X86",
				"SCMP_ARCH_X32"
			]
		}
	],
	"architectures": [
		"SCMP_ARCH_X32"
	],
	"defaultAction": "SCMP_ACT_ALLOW",
	"syscalls": [
		{
			"names": ["chmod", "fchmod", "fchmodat"],
			"action": "SCMP_ACT_ERRNO"
		}
	]
}`,
			expectedErr: "use either 'architectures' or 'archMap'",
		},
	}
	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			cfg := container.NewTestConfig(
				container.WithImage("busybox"),
				container.WithSecurityOpt("seccomp="+tc.profile),
				container.WithCmd("chmod", "777", "."),
			)
			resp, err := container.CreateFromConfig(ctx, apiClient, cfg)
			if err == nil {
				_, err = apiClient.ContainerStart(ctx, resp.ID, client.ContainerStartOptions{})
			}
			assert.Check(t, is.ErrorContains(err, tc.expectedErr))
		})
	}
}

// TestRunWithDaemonDefaultSeccompProfile checks that a profile passed to the
// daemon replaces the built-in default one.
func TestRunWithDaemonDefaultSeccompProfile(t *testing.T) {
	skipIfNoSeccomp(t)
	skip.If(t, testEnv.IsRemoteDaemon, "cannot start daemon on remote test run")
	ctx := setupTest(t)

	d := daemon.New(t)
	d.StartWithBusybox(ctx, t)
	defer d.Stop(t)
	apiClient := d.NewClientT(t)

	// 1) verify I can run containers with the Docker default shipped profile which allows chmod
	res := container.RunAttach(ctx, t, apiClient, container.WithCmd("chmod", "777", "."))
	assert.Check(t, is.Equal(res.ExitCode, 0), combined(res))

	const profile = `{
	"defaultAction": "SCMP_ACT_ALLOW",
	"syscalls": [
		{
			"name": "chmod",
			"action": "SCMP_ACT_ERRNO"
		},
		{
			"name": "fchmodat",
			"action": "SCMP_ACT_ERRNO"
		}
	]
}`
	profilePath := filepath.Join(testutil.TempDir(t), "profile.json")
	assert.NilError(t, os.WriteFile(profilePath, []byte(profile), 0o644))

	// 2) restart the daemon and add a custom seccomp profile in which we deny chmod
	d.Restart(t, "--seccomp-profile="+profilePath)

	res = container.RunAttach(ctx, t, apiClient, container.WithCmd("chmod", "777", "."))
	assert.Check(t, res.ExitCode != 0, combined(res))
	assert.Check(t, is.Contains(combined(res), "Operation not permitted"))
}
