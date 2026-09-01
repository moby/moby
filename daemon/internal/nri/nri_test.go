package nri

import (
	"slices"
	"testing"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/v2/daemon/container"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// newTestContainer returns a minimal container with the resolved process path
// and arguments set the same way daemon.newContainer resolves them from the
// merged Entrypoint/Cmd config. At least one of entrypoint or cmd must be
// non-empty, matching the precondition validated before container creation.
func newTestContainer(entrypoint, cmd []string) *container.Container {
	ctr := container.NewBaseContainer("test-id", "/tmp/test-root")
	ctr.Config = &containertypes.Config{
		Entrypoint: entrypoint,
		Cmd:        cmd,
	}
	ctr.HostConfig = &containertypes.HostConfig{}
	// Mirror daemon.getEntrypointAndArgs, which resolves the process path and
	// args before the container is passed to NRI.
	if len(entrypoint) == 0 {
		ctr.Path = cmd[0]
		ctr.Args = cmd[1:]
	} else {
		ctr.Path = entrypoint[0]
		ctr.Args = append(entrypoint[1:], cmd...)
	}
	return ctr
}

func TestContainerToNRIArgs(t *testing.T) {
	initEnabled := true
	initDisabled := false
	tests := []struct {
		name       string
		entrypoint []string
		cmd        []string
		init       *bool
		expArgs    []string
	}{
		{
			name:       "entrypoint override",
			entrypoint: []string{"echo"},
			cmd:        []string{"hello"},
			expArgs:    []string{"echo", "hello"},
		},
		{
			name:       "init enabled",
			entrypoint: []string{"/usr/bin/tool", "--mode"},
			cmd:        []string{"input", "output"},
			init:       &initEnabled,
			expArgs:    []string{"/usr/bin/tool", "--mode", "input", "output"},
		},
		{
			name:       "daemon init default possible",
			entrypoint: []string{"/usr/bin/tool", "--mode"},
			cmd:        []string{"input", "output"},
			expArgs:    []string{"/usr/bin/tool", "--mode", "input", "output"},
		},
		{
			name:       "init disabled",
			entrypoint: []string{"/usr/bin/tool", "--mode"},
			cmd:        []string{"input", "output"},
			init:       &initDisabled,
			expArgs:    []string{"/usr/bin/tool", "--mode", "input", "output"},
		},
		{
			name:    "cmd only",
			cmd:     []string{"/bin/sh", "-c", "true"},
			expArgs: []string{"/bin/sh", "-c", "true"},
		},
		{
			name:       "entrypoint only",
			entrypoint: []string{"/entrypoint.sh"},
			expArgs:    []string{"/entrypoint.sh"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctr := newTestContainer(tc.entrypoint, tc.cmd)
			ctr.HostConfig.Init = tc.init
			_, nriCtr, err := containerToNRI(ctr)
			assert.NilError(t, err)
			assert.Check(t, is.DeepEqual(nriCtr.Args, tc.expArgs))
			// NRI reports the resolved workload argv, not OCI-only wrappers.
			assert.Check(t, is.DeepEqual(nriCtr.Args, append([]string{ctr.Path}, ctr.Args...)))
			for _, initArg := range []string{"/sbin/docker-init", "--", "/usr/libexec/docker-init"} {
				assert.Check(t, !slices.Contains(nriCtr.Args, initArg), "NRI args unexpectedly include init argument %q: %v", initArg, nriCtr.Args)
			}
		})
	}
}

func TestContainerToNRIArgsNoAliasing(t *testing.T) {
	ctr := newTestContainer([]string{"echo"}, []string{"hello"})
	_, nriCtr, err := containerToNRI(ctr)
	assert.NilError(t, err)

	// Mutating the NRI args must not modify the container's resolved args.
	for i := range nriCtr.Args {
		nriCtr.Args[i] = "mutated"
	}
	assert.Check(t, is.Equal(ctr.Path, "echo"))
	assert.Check(t, is.DeepEqual(ctr.Args, []string{"hello"}))
}
