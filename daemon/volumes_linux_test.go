package daemon

import (
	"path/filepath"
	"testing"

	"github.com/docker/docker/api/types/mount"
)

func TestBindDaemonRoot(t *testing.T) {
	t.Parallel()
	d := &Daemon{root: "/a/b/c/daemon"}
	for _, test := range []struct {
		desc      string
		opts      *mount.BindOptions
		needsProp bool
		err       bool
	}{
		{desc: "nil propagation settings", opts: nil, needsProp: true, err: false},
		{desc: "empty propagation settings", opts: &mount.BindOptions{}, needsProp: true, err: false},
		{desc: "private propagation", opts: &mount.BindOptions{Propagation: mount.PropagationPrivate}, err: true},
		{desc: "rprivate propagation", opts: &mount.BindOptions{Propagation: mount.PropagationRPrivate}, err: true},
		{desc: "slave propagation", opts: &mount.BindOptions{Propagation: mount.PropagationSlave}, err: true},
		{desc: "rslave propagation", opts: &mount.BindOptions{Propagation: mount.PropagationRSlave}, err: false, needsProp: false},
		{desc: "shared propagation", opts: &mount.BindOptions{Propagation: mount.PropagationShared}, err: true},
		{desc: "rshared propagation", opts: &mount.BindOptions{Propagation: mount.PropagationRSlave}, err: false, needsProp: false},
	} {
		t.Run(test.desc, func(t *testing.T) {
			test := test
			for desc, source := range map[string]string{
				"source is root":    d.root,
				"source is subpath": filepath.Join(d.root, "a", "b"),
				"source is parent":  filepath.Dir(d.root),
				"source is /":       "/",
			} {
				t.Run(desc, func(t *testing.T) {
					mount := mount.Mount{
						Type:        mount.TypeBind,
						Source:      source,
						BindOptions: test.opts,
					}
					needsProp, err := d.validateBindDaemonRoot(mount)
					if (err != nil) != test.err {
						t.Fatalf("expected err=%v, got: %v", test.err, err)
					}
					if test.err {
						return
					}
					if test.needsProp != needsProp {
						t.Fatalf("expected needsProp=%v, got: %v", test.needsProp, needsProp)
					}
				})
			}
		})
	}
}
