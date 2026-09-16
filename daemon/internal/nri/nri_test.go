package nri

import (
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
	tests := []struct {
		name       string
		entrypoint []string
		cmd        []string
		expArgs    []string
	}{
		{
			name:       "entrypoint override",
			entrypoint: []string{"echo"},
			cmd:        []string{"hello"},
			expArgs:    []string{"echo", "hello"},
		},
		{
			name:       "entrypoint with args plus cmd",
			entrypoint: []string{"/usr/bin/tool", "--mode"},
			cmd:        []string{"input"},
			expArgs:    []string{"/usr/bin/tool", "--mode", "input"},
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
			_, nriCtr, err := containerToNRI(ctr)
			assert.NilError(t, err)
			assert.Check(t, is.DeepEqual(nriCtr.Args, tc.expArgs))
			// The NRI args must match the process args placed in the OCI spec,
			// which are constructed as append([]string{c.Path}, c.Args...).
			assert.Check(t, is.DeepEqual(nriCtr.Args, append([]string{ctr.Path}, ctr.Args...)))
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
