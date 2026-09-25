package daemon

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/checkpoint"
	"github.com/moby/moby/v2/daemon/container"
	"github.com/moby/moby/v2/daemon/server/backend"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestValidateCheckpointID(t *testing.T) {
	type testCase struct {
		name         string
		checkpointID string
		valid        bool
	}
	tests := []testCase{
		{name: "restricted name", checkpointID: "a1", valid: true},
		{name: "space", checkpointID: "space name"},
		{name: "unicode", checkpointID: "unicode-世界"},
		{name: "dots", checkpointID: "name.with-dots", valid: true},
		{name: "empty", checkpointID: ""},
		{name: "dot", checkpointID: "."},
		{name: "dot-dot", checkpointID: ".."},
		{name: "traversal", checkpointID: "../outside"},
		{name: "nested traversal", checkpointID: "foo/../outside"},
		{name: "separator", checkpointID: "foo/bar"},
		{name: "windows separator", checkpointID: `foo\bar`},
		{name: "NUL", checkpointID: "contains\x00nul"},
		{name: "absolute path", checkpointID: filepath.Join(string(filepath.Separator), "absolute")},
	}
	if runtime.GOOS == "windows" {
		tests = append(tests,
			testCase{name: "dot trailing space", checkpointID: ". "},
			testCase{name: "dot-dot trailing space", checkpointID: ".. "},
			testCase{name: "trailing dot", checkpointID: "name."},
			testCase{name: "trailing space", checkpointID: "name "},
		)
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateCheckpointID(tc.checkpointID)
			if tc.valid {
				assert.NilError(t, err)
				return
			}
			assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
		})
	}
}

func TestCheckpointOperationsValidateBeforeContainerLookup(t *testing.T) {
	daemon := &Daemon{}
	tests := []struct {
		name      string
		operation func() error
	}{
		{
			name: "create",
			operation: func() error {
				return daemon.CheckpointCreate("missing", checkpoint.CreateRequest{CheckpointID: "../outside"})
			},
		},
		{
			name: "delete",
			operation: func() error {
				return daemon.CheckpointDelete("missing", backend.CheckpointDeleteOptions{CheckpointID: "../outside"})
			},
		},
		{
			name: "restore",
			operation: func() error {
				return daemon.ContainerStart(context.Background(), "missing", "../outside", "")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.operation()
			assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
			assert.Check(t, is.ErrorContains(err, `invalid checkpoint ID "../outside"`))
		})
	}
}

func TestCheckpointListMissingDefaultDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ctr := container.NewBaseContainer("container-id", root)
	store := container.NewMemoryStore()
	store.Add(ctr.ID, ctr)
	daemon := &Daemon{containers: store}

	checkpoints, err := daemon.CheckpointList(ctr.ID, backend.CheckpointListOptions{})
	assert.NilError(t, err)
	assert.Assert(t, is.Len(checkpoints, 0))
}
