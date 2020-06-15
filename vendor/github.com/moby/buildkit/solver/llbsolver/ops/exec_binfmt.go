package ops

import (
	"context"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/platforms"
	"github.com/docker/docker/pkg/idtools"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/binfmt_misc"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	copy "github.com/tonistiigi/fsutil/copy"
)

const qemuMountName = "/dev/.buildkit_qemu_emulator"

var qemuArchMap = map[string]string{
	"arm64":   "aarch64",
	"amd64":   "x86_64",
	"riscv64": "riscv64",
	"arm":     "arm",
	"s390x":   "s390x",
	"ppc64le": "ppc64le",
}

type emulator struct {
	path  string
	idmap *idtools.IdentityMapping
}

func (e *emulator) Mount(ctx context.Context, readonly bool) (snapshot.Mountable, error) {
	return &staticEmulatorMount{path: e.path, idmap: e.idmap}, nil
}

type staticEmulatorMount struct {
	path  string
	idmap *idtools.IdentityMapping
}

func (m *staticEmulatorMount) Mount() ([]mount.Mount, func() error, error) {
	tmpdir, err := ioutil.TempDir("", "buildkit-qemu-emulator")
	if err != nil {
		return nil, nil, err
	}
	var ret bool
	defer func() {
		if !ret {
			os.RemoveAll(tmpdir)
		}
	}()

	var uid, gid int
	if m.idmap != nil {
		root := m.idmap.RootPair()
		uid = root.UID
		gid = root.GID
	}
	if err := copy.Copy(context.TODO(), filepath.Dir(m.path), filepath.Base(m.path), tmpdir, qemuMountName, func(ci *copy.CopyInfo) {
		m := 0555
		ci.Mode = &m
	}, copy.WithChown(uid, gid)); err != nil {
		return nil, nil, err
	}

	ret = true
	return []mount.Mount{{
			Type:    "bind",
			Source:  filepath.Join(tmpdir, qemuMountName),
			Options: []string{"ro", "bind"},
		}}, func() error {
			return os.RemoveAll(tmpdir)
		}, nil

}
func (m *staticEmulatorMount) IdentityMapping() *idtools.IdentityMapping {
	return m.idmap
}

func getEmulator(p *pb.Platform, idmap *idtools.IdentityMapping) (*emulator, error) {
	all := binfmt_misc.SupportedPlatforms(false)
	m := make(map[string]struct{}, len(all))

	for _, p := range all {
		m[p] = struct{}{}
	}

	pp := platforms.Normalize(specs.Platform{
		Architecture: p.Architecture,
		OS:           p.OS,
		Variant:      p.Variant,
	})

	if _, ok := m[platforms.Format(pp)]; ok {
		return nil, nil
	}

	a, ok := qemuArchMap[pp.Architecture]
	if !ok {
		a = pp.Architecture
	}

	fn, err := exec.LookPath("buildkit-qemu-" + a)
	if err != nil {
		return nil, errors.Errorf("no emulator available for %v", pp.OS)
	}

	return &emulator{path: fn}, nil
}
