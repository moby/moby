//go:build linux

package remote

import (
	"testing"

	"github.com/opencontainers/runtime-spec/specs-go"
	"gotest.tools/v3/assert"
)

func TestGetIOUser(t *testing.T) {
	tests := []struct {
		name    string
		userns  bool
		process *specs.Process
		wantUID int
		wantGID int
	}{
		{
			name:    "user namespace maps process user",
			userns:  true,
			process: &specs.Process{User: specs.User{UID: 65532, GID: 0}},
			wantUID: 165532,
			wantGID: 100000,
		},
		{
			name:    "user namespace maps root process",
			userns:  true,
			process: &specs.Process{User: specs.User{UID: 0, GID: 0}},
			wantUID: 100000,
			wantGID: 100000,
		},
		{
			name:    "without user namespace keeps host ownership",
			process: &specs.Process{User: specs.User{UID: 65532, GID: 1000}},
			wantUID: 0,
			wantGID: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			spec := &specs.Spec{
				Process: tc.process,
				Linux: &specs.Linux{
					UIDMappings: []specs.LinuxIDMapping{{ContainerID: 0, HostID: 100000, Size: 65536}},
					GIDMappings: []specs.LinuxIDMapping{{ContainerID: 0, HostID: 100000, Size: 65536}},
				},
			}
			if tc.userns {
				spec.Linux.Namespaces = []specs.LinuxNamespace{{Type: specs.UserNamespace}}
			}
			uid, gid := getIOUser(spec)
			assert.Equal(t, uid, tc.wantUID)
			assert.Equal(t, gid, tc.wantGID)
		})
	}
}
