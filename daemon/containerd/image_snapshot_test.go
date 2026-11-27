package containerd

import (
	"context"
	"errors"
	"testing"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/mount"
	"github.com/containerd/containerd/v2/core/snapshots"
	"github.com/containerd/containerd/v2/defaults"
	"github.com/moby/moby/v2/daemon/snapshotter"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// fakeMounter runs temporary mount callbacks on an empty directory.
type fakeMounter struct {
	snapshotter.Mounter
	root   string
	err    error
	events *[]string
}

func (m *fakeMounter) WithTempMount(_ context.Context, _ []mount.Mount, f func(root string) error) error {
	*m.events = append(*m.events, "temp mount")
	if m.err != nil {
		return m.err
	}
	return f(m.root)
}

func (m *fakeMounter) WithReadonlyTempMount(_ context.Context, _ []mount.Mount, f func(root string) error) error {
	*m.events = append(*m.events, "readonly temp mount")
	if m.err != nil {
		return m.err
	}
	return f(m.root)
}

type commitRecordingSnapshotter struct {
	snapshots.Snapshotter
	events *[]string
}

func (s *commitRecordingSnapshotter) Prepare(context.Context, string, string, ...snapshots.Opt) ([]mount.Mount, error) {
	return nil, nil
}

func (s *commitRecordingSnapshotter) Commit(_ context.Context, name, _ string, _ ...snapshots.Opt) error {
	*s.events = append(*s.events, "commit "+name)
	return nil
}

func TestPrepareInitLayer(t *testing.T) {
	errMount := errors.New("mount failed")
	errSetup := errors.New("setup failed")

	tests := []struct {
		name       string
		mountErr   error
		setupErr   error
		wantErr    error
		wantEvents []string
	}{
		{
			name:       "success",
			wantEvents: []string{"temp mount", "setup", "commit ctr-init"},
		},
		{
			name:       "mount fails",
			mountErr:   errMount,
			wantErr:    errMount,
			wantEvents: []string{"temp mount"},
		},
		{
			name:       "setup fails",
			setupErr:   errSetup,
			wantErr:    errSetup,
			wantEvents: []string{"temp mount", "setup"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var events []string
			sn := &commitRecordingSnapshotter{events: &events}
			c8dCli, err := containerd.New("", containerd.WithServices(
				containerd.WithSnapshotters(map[string]snapshots.Snapshotter{defaults.DefaultSnapshotter: sn}),
			))
			assert.NilError(t, err)
			i := &ImageService{
				client:          c8dCli,
				snapshotter:     defaults.DefaultSnapshotter,
				refCountMounter: &fakeMounter{root: t.TempDir(), err: tc.mountErr, events: &events},
			}

			err = i.prepareInitLayer(t.Context(), "ctr", "parent", func(string) error {
				events = append(events, "setup")
				return tc.setupErr
			})
			assert.Check(t, is.ErrorIs(err, tc.wantErr))
			assert.Check(t, is.DeepEqual(events, tc.wantEvents))
		})
	}
}
