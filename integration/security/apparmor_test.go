package security

import (
	"slices"
	"strings"
	"testing"

	"github.com/moby/moby/v2/integration/internal/container"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestAppArmorDeniesPtrace(t *testing.T) {
	skipIfNoAppArmor(t)
	skip.If(t, testEnv.IsUserNamespace(), "not applicable with user namespace remapping")
	skip.If(t, testEnv.IsRootless(), "container root cannot trace the host's pid 1 in rootless mode")
	ctx := setupTest(t)

	// The default profile allows processes in the container to trace each
	// other, so trace pid 1 of the host's PID namespace, which is outside of
	// the profile. CAP_SYS_PTRACE is added so that only AppArmor can deny it,
	// which the unconfined run confirms. busybox readlink exits with 0 even
	// when it fails, so check whether it printed the namespace.
	ops := []func(*container.TestContainerConfig){
		container.WithImage("busybox"),
		container.WithPIDMode("host"),
		container.WithCapability("SYS_PTRACE"),
		container.WithCmd("readlink", "/proc/1/ns/net"),
	}

	res := runContainer(ctx, t, append(ops, container.WithSecurityOpt("apparmor=unconfined"))...)
	assert.Assert(t, is.Contains(combined(res), "net:["), "ptrace of the host's pid 1 failed without AppArmor")

	res = runContainer(ctx, t, ops...)
	assert.Check(t, !strings.Contains(combined(res), "net:["), "ptrace was not successfully restricted by AppArmor: %s", combined(res))
}

func TestAppArmorTraceSelf(t *testing.T) {
	skipIfNoAppArmor(t)
	ctx := setupTest(t)

	res := runContainer(ctx, t,
		container.WithImage("busybox"),
		container.WithCmd("readlink", "/proc/1/ns/net"),
	)
	assert.Check(t, is.Equal(res.ExitCode, 0), "ptrace of self failed: %s", combined(res))
}

func TestAppArmorDeniesChmodProc(t *testing.T) {
	skipIfNoAppArmor(t)
	skip.If(t, testEnv.IsUserNamespace(), "not applicable with user namespace remapping")
	ctx := setupTest(t)

	res := runContainer(ctx, t,
		container.WithImage("busybox"),
		container.WithCmd("chmod", "744", "/proc/cpuinfo"),
	)
	if res.ExitCode == 0 {
		// The mode of /proc files is shared with the host, so repair it.
		_ = runContainer(ctx, t,
			container.WithImage("busybox"),
			container.WithCmd("chmod", "444", "/proc/cpuinfo"),
		)
		t.Fatal("AppArmor was unsuccessful in prohibiting chmod of /proc/* files")
	}
}

func TestRunReadFilteredProc(t *testing.T) {
	skipIfNoAppArmor(t)
	skip.If(t, testEnv.IsUserNamespace(), "not applicable with user namespace remapping")
	ctx := setupTest(t)

	// These are the /proc files the docker-default profile denies reading.
	// The container is privileged so that masked paths don't hide them.
	for _, path := range []string{"/proc/sysrq-trigger", "/proc/kcore"} {
		t.Run(path, func(t *testing.T) {
			res := runContainer(ctx, t,
				container.WithImage("busybox"),
				container.WithPrivileged(true),
				container.WithSecurityOpt("apparmor=docker-default"),
				container.WithCmd("sh", "-c", "exec 3<"+path),
			)
			assert.Check(t, res.ExitCode != 0, "opening %s for read should have been denied", path)
			assert.Check(t, is.Contains(combined(res), "Permission denied"))
		})
	}
}

func TestRunWriteFilteredProc(t *testing.T) {
	skipIfNoAppArmor(t)
	skip.If(t, testEnv.IsUserNamespace(), "not applicable with user namespace remapping")
	ctx := setupTest(t)

	for _, path := range []string{
		// modprobe and core_pattern should both be denied by generic
		// policy of denials for /proc/sys/kernel. These files have been
		// picked to be checked as they are particularly sensitive to writes
		"/proc/sys/kernel/modprobe",
		"/proc/sys/kernel/core_pattern",
		"/proc/sysrq-trigger",
		"/proc/kcore",
	} {
		t.Run(path, func(t *testing.T) {
			res := runContainer(ctx, t,
				container.WithImage("busybox"),
				container.WithPrivileged(true),
				container.WithSecurityOpt("apparmor=docker-default"),
				container.WithCmd("sh", "-c", "exec 3>"+path),
			)
			assert.Check(t, res.ExitCode != 0, "opening %s for write should have been denied", path)
		})
	}
}

func TestRunUnshareProc(t *testing.T) {
	skipIfNoAppArmor(t)
	skip.If(t, testEnv.IsUserNamespace(), "not applicable with user namespace remapping")
	ctx := setupTest(t)

	tests := []struct {
		doc      string
		ops      []func(*container.TestContainerConfig)
		cmd      []string
		expected []string
	}{
		{
			doc:      "mount-proc",
			cmd:      []string{"unshare", "-p", "-m", "-f", "-r", "--mount-proc=/proc", "mount"},
			expected: []string{"permission denied", "operation not permitted"},
		},
		{
			doc:      "mount proc",
			cmd:      []string{"unshare", "-p", "-m", "-f", "-r", "mount", "-t", "proc", "none", "/proc"},
			expected: []string{"mount: cannot mount none", "permission denied", "operation not permitted"},
		},
		{
			// Ensure it still fails when running privileged with the default policy.
			doc: "privileged",
			ops: []func(*container.TestContainerConfig){
				container.WithPrivileged(true),
				container.WithSecurityOpt("apparmor=docker-default"),
			},
			cmd:      []string{"unshare", "-p", "-m", "-f", "-r", "mount", "-t", "proc", "none", "/proc"},
			expected: []string{"mount: cannot mount none", "permission denied", "operation not permitted"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			ops := append([]func(*container.TestContainerConfig){
				container.WithImage("debian:trixie-slim"),
				container.WithSecurityOpt("seccomp=unconfined"),
				container.WithCmd(tc.cmd...),
			}, tc.ops...)
			res := runContainer(ctx, t, ops...)
			out := strings.ToLower(combined(res))
			assert.Check(t, res.ExitCode != 0, out)
			assert.Check(t, containsAny(out, tc.expected...), "expected one of %q, got: %s", tc.expected, out)
		})
	}
}

func TestRunApparmorProcDirectory(t *testing.T) {
	skipIfNoAppArmor(t)
	ctx := setupTest(t)

	for _, path := range []string{"/proc/1/cgroup", "/proc/1/attr/current"} {
		t.Run(path, func(t *testing.T) {
			// running w seccomp unconfined tests the apparmor profile
			res := runContainer(ctx, t,
				container.WithImage("busybox"),
				container.WithSecurityOpt("seccomp=unconfined"),
				container.WithCmd("chmod", "777", path),
			)
			assert.Check(t, is.Equal(res.ExitCode, 1))
			assert.Check(t, containsAny(combined(res), "Permission denied", "Operation not permitted"), "expected chmod 777 %s to fail, got: %s", path, combined(res))
		})
	}
}

func containsAny(s string, substrs ...string) bool {
	return slices.ContainsFunc(substrs, func(sub string) bool {
		return strings.Contains(s, sub)
	})
}
