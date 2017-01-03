package stack

import (
	"testing"

	composetypes "github.com/aanand/compose-file/types"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/pkg/testutil/assert"
)

func TestConvertVolumeToMountAnonymousVolume(t *testing.T) {
	stackVolumes := map[string]composetypes.VolumeConfig{}
	namespace := namespace{name:"foo"}
	expected := mount.Mount{
		Type:   mount.TypeVolume,
		Target: "/foo/bar",
	}
	mnt, err := convertVolumeToMount("/foo/bar", stackVolumes, namespace)
	assert.NilError(t, err)
	assert.DeepEqual(t, mnt, expected)
}
