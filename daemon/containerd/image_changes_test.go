package containerd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/mount"
	"github.com/containerd/containerd/v2/core/snapshots"
	"github.com/containerd/containerd/v2/defaults"
	"github.com/moby/go-archive"
	"github.com/moby/moby/v2/daemon/container"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

type viewRecordingSnapshotter struct {
	snapshots.Snapshotter
	events *[]string
}

func (s *viewRecordingSnapshotter) Stat(_ context.Context, key string) (snapshots.Info, error) {
	return snapshots.Info{Name: key, Parent: "parent"}, nil
}

func (s *viewRecordingSnapshotter) View(context.Context, string, string, ...snapshots.Opt) ([]mount.Mount, error) {
	return nil, nil
}

func (s *viewRecordingSnapshotter) Remove(context.Context, string) error {
	*s.events = append(*s.events, "remove view")
	return nil
}

type fakeRWLayer struct {
	root   string
	events *[]string
}

func (l *fakeRWLayer) Mount(string) (string, error) { return l.root, nil }

func (l *fakeRWLayer) Unmount() error {
	*l.events = append(*l.events, "unmount rwlayer")
	return nil
}

func (l *fakeRWLayer) Metadata() (map[string]string, error) { return nil, nil }

func TestChanges(t *testing.T) {
	errMount := errors.New("mount failed")

	tests := []struct {
		name     string
		mountErr error
		wantErr  error
	}{
		{name: "success"},
		{name: "mount fails", mountErr: errMount, wantErr: errMount},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var events []string
			sn := &viewRecordingSnapshotter{events: &events}
			c8dCli, err := containerd.New("", containerd.WithServices(
				containerd.WithSnapshotters(map[string]snapshots.Snapshotter{defaults.DefaultSnapshotter: sn}),
				containerd.WithLeasesService(noopLeasesManager{}),
			))
			assert.NilError(t, err)
			i := &ImageService{
				client:          c8dCli,
				snapshotter:     defaults.DefaultSnapshotter,
				refCountMounter: &fakeMounter{root: t.TempDir(), err: tc.mountErr, events: &events},
			}

			root := t.TempDir()
			assert.NilError(t, os.WriteFile(filepath.Join(root, "added"), nil, 0o644))
			ctr := &container.Container{
				ID:      "ctr",
				Driver:  defaults.DefaultSnapshotter,
				RWLayer: &fakeRWLayer{root: root, events: &events},
			}

			changes, err := i.Changes(t.Context(), ctr)
			assert.Check(t, is.ErrorIs(err, tc.wantErr))
			if tc.wantErr == nil {
				assert.Check(t, is.DeepEqual(changes, []archive.Change{{Path: filepath.FromSlash("/added"), Kind: archive.ChangeAdd}}))
			}
			assert.Check(t, is.DeepEqual(events, []string{"readonly temp mount", "unmount rwlayer", "remove view"}))
		})
	}
}
