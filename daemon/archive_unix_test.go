//go:build !windows

package daemon

import (
	"testing"

	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/v2/daemon/container"
	volumemounts "github.com/moby/moby/v2/daemon/volume/mounts"
	"gotest.tools/v3/assert"
)

func TestCheckNotIDMappedPath(t *testing.T) {
	ctr := &container.Container{MountPoints: map[string]*volumemounts.MountPoint{
		"/mapped": {
			Destination: "/mapped",
			Type:        mount.TypeBind,
			Spec: mount.Mount{
				Type:        mount.TypeBind,
				Target:      "/mapped",
				BindOptions: &mount.BindOptions{IDMapping: &mount.IDMapping{Source: mount.IDMappingSourceMatchUser}},
			},
		},
		"/plain": {
			Destination: "/plain",
			Type:        mount.TypeBind,
			Spec:        mount.Mount{Type: mount.TypeBind, Target: "/plain", BindOptions: &mount.BindOptions{}},
		},
		"/nooptions": {
			Destination: "/nooptions",
			Type:        mount.TypeBind,
			Spec:        mount.Mount{Type: mount.TypeBind, Target: "/nooptions"},
		},
	}}

	assert.ErrorContains(t, checkNotIDMappedPath(ctr, "/mapped"), "id-mapped")
	assert.ErrorContains(t, checkNotIDMappedPath(ctr, "/mapped/sub/dir"), "id-mapped")
	assert.NilError(t, checkNotIDMappedPath(ctr, "/plain/file"))
	assert.NilError(t, checkNotIDMappedPath(ctr, "/nooptions"))
	assert.NilError(t, checkNotIDMappedPath(ctr, "/elsewhere"))
}
