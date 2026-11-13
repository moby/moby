package osl

import (
	"fmt"
	"testing"

	"golang.org/x/sys/unix"
	"gotest.tools/v3/assert"
)

func TestRestoreNetNSFailureExpected(t *testing.T) {
	const detached = "/run/user/1000/dockerd-rootless/netns"
	tests := []struct {
		name          string
		err           error
		detachedNetNS string
		expected      bool
	}{
		{name: "detached netns, EPERM", err: unix.EPERM, detachedNetNS: detached, expected: true},
		{name: "detached netns, wrapped EPERM", err: fmt.Errorf("setns: %w", unix.EPERM), detachedNetNS: detached, expected: true},
		{name: "detached netns, other error", err: unix.EINVAL, detachedNetNS: detached, expected: false},
		{name: "no detached netns, EPERM", err: unix.EPERM, expected: false},
		{name: "no detached netns, other error", err: unix.EBADF, expected: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, restoreNetNSFailureExpected(tc.err, tc.detachedNetNS), tc.expected)
		})
	}
}
