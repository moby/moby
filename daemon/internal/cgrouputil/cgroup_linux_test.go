//go:build linux

package cgrouputil

import (
	"os"
	"testing"

	"gotest.tools/v3/assert"
)

func TestIsScopePath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"/system.slice/docker.service", false},
		{"/system.slice", false},
		{"/user.slice/user-1000.slice", false},
		{"/slurm/uid_1001/job_123", false},
		{"/docker/abc123", false},
		{"/user.slice/user-1000.slice/session-3.scope", true},
		{"/system.slice/slurmstepd.scope/job_123/step_0", true},
		{"/system.slice/slurmstepd.scope", true},
		// ".scope" as substring but not as segment suffix must not match.
		{"/foo.scope-bar/baz", false},
		{"", false},
		{"/", false},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			assert.Equal(t, IsScopePath(tc.path), tc.want)
		})
	}
}

func TestVerifyPIDOwnerSelf(t *testing.T) {
	uid := uint32(os.Getuid())
	assert.NilError(t, VerifyPIDOwner(int32(os.Getpid()), uid))
	assert.ErrorContains(t, VerifyPIDOwner(int32(os.Getpid()), uid^1), "not peer uid")
}

func TestVerifyPIDOwnerErrors(t *testing.T) {
	assert.ErrorContains(t, VerifyPIDOwner(0, 0), "invalid pid")
	assert.ErrorContains(t, VerifyPIDOwner(-1, 0), "invalid pid")
	// Very likely nonexistent PID; asserts the /proc open error path.
	assert.ErrorContains(t, VerifyPIDOwner(1<<30, 0), "no such file or directory")
}
